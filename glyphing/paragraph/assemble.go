package paragraph

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/layout"
)

// SpacingPolicy selects compatibility spacing or resolved CSS-inspired line boxes.
type SpacingPolicy string

// ProofCompatibility reproduces tytui's original proof geometry, including its
// minimum 0.2em gap and trailing gap. It is neither CSS normal nor TeX baseline glue.
const ProofCompatibility SpacingPolicy = "proof-compatibility-v1"

// Options selects assembly policy and compatibility fallback size.
// CSSLineHeight uses explicit streaming defaults instead of DefaultEmSize.
type Options struct {
	Policy SpacingPolicy // Required; unknown policies are rejected.
	// DefaultEmSize supplies fallback metrics when a line's measured ascent plus
	// descent is zero (including lines without text): 0.8em, 0.2em, 0.2em gap.
	// It must be positive and below dimen.Fil, even for empty output.
	DefaultEmSize dimen.DU
}

// assembly is private mutable construction state, never exposed to rendering.
type assembly struct {
	box      layout.Box
	lines    []layout.Line
	material []layout.Material
	frozen   *khipu.FrozenKhipu
	atomics  []layout.AtomicPlacement
}

func (v *assembly) publish() (*layout.VList, error) {
	return layout.NewVList(v.frozen, v.box, v.lines, v.material, v.atomics)
}

// Assemble measures contract metrics and places frozen lines once, without
// rendering, shaping, breaking, loading resources or inspecting concrete faces.
// Accumulation follows the legacy proof's float64 big-point operation order.
// Invalid metrics and coordinates outside [0, dimen.Fil) in DU are errors; no
// saturation or narrowing to integer dimensions is performed. Ink overflow is intentionally not measured here.
// Cancellation is checked between lines/knots; font query calls are synchronous.
// A zero-line frozen result is an empty box; nil input is an error.
// This frozen-only entry supports ProofCompatibility. CSSLineHeight uses Run or
// Prepared.Layout, which retain prepared source styles for surviving spaces.
// Atom-containing input requires Prepared.Layout with border geometry; this
// frozen-only entry rejects it instead of silently omitting vertical extents.
func Assemble(ctx context.Context, frozen *khipu.FrozenKhipu, options Options) (*layout.VList, error) {
	if ctx == nil || frozen == nil {
		return nil, fmt.Errorf("paragraph assembly requires context and frozen content")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	for i := 0; i < frozen.KnotCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, ok := frozen.Atomic(i); ok {
			return nil, fmt.Errorf("%w: atomic assembly requires border geometry", khipu.ErrUnsupportedPolicy)
		}
	}
	v, err := assembleCompatibility(ctx, frozen, options, nil)
	if err != nil {
		return nil, err
	}
	return v.publish()
}

func assembleCompatibility(ctx context.Context, frozen *khipu.FrozenKhipu, options Options, atomics map[contracts.AtomicID]AtomicBox) (*assembly, error) {
	v := &assembly{frozen: frozen}
	top := 0.0
	for i := 0; i < frozen.LineCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		horizontal, _ := frozen.Line(i)
		ascent, descent, gap := 0.0, 0.0, 0.0
		atomicAbove, hasAtomic := math.Inf(-1), false
		for k := horizontal.FirstKnot; k < horizontal.FirstKnot+horizontal.KnotCount; k++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if id, ok := frozen.Atomic(k); ok {
				box, exists := atomics[id]
				if !exists {
					return nil, fmt.Errorf("%w: missing atomic box", contracts.ErrIllegalArguments)
				}
				atomicAbove = math.Max(atomicAbove, float64(box.above())/float64(dimen.BP))
				hasAtomic = true
				continue
			}
			text, ok := frozen.CopyText(k)
			if !ok {
				continue
			}
			a, d, g, err := measure(text)
			if err != nil {
				return nil, fmt.Errorf("paragraph line %d knot %d: %w", i, k, err)
			}
			ascent = math.Max(ascent, a)
			descent = math.Max(descent, d)
			gap = math.Max(gap, g)
		}
		if ascent+descent == 0 {
			ascent = options.DefaultEmSize.Points() * .8
			descent = options.DefaultEmSize.Points() * .2
			gap = options.DefaultEmSize.Points() * .2
		}
		// Preserve compatibility's text metrics/fallback, then combine signed
		// atomic contributions. Objects receive no text leading or font gap.
		if hasAtomic {
			ascent, descent = math.Max(ascent, atomicAbove), math.Max(descent, 0)
		}
		baseline := top + ascent
		height := ascent + descent + gap
		end := top + height
		width := math.Max(horizontal.Width.Points(), horizontal.TargetWidth.Points())
		if !dimension(end) || !dimension(baseline) || !dimension(width) || horizontal.Width < 0 || horizontal.TargetWidth < 0 {
			return nil, fmt.Errorf("%w: paragraph line %d: geometry outside finite dimension range", contracts.ErrIllegalArguments, i)
		}
		v.lines = append(v.lines, layout.Line{Index: i, Top: top, Baseline: baseline, Ascent: ascent, Descent: descent, Gap: gap, Height: height, Horizontal: horizontal})
		v.material = append(v.material,
			layout.Material{Kind: layout.LineBox, Child: i, Top: top, Extent: ascent + descent},
			layout.Material{Kind: layout.Kern, Child: -1, Top: top + (ascent + descent), Extent: gap})
		v.box.Width = math.Max(v.box.Width, width)
		if i == 0 {
			v.box.Baseline, v.box.HasBaseline = baseline, true
		}
		top = end
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v.box.Extent = top
	return v, nil
}

func dimension(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < dimen.Fil.Points()
}

func nilService(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

// measure preserves signed descent and legacy clamping of negative font extents.
// Negative LineGap is permitted; the 0.2em minimum still applies.
func measure(text khipu.FrozenText) (ascent, descent, gap float64, err error) {
	c := text.Context
	if !c.Params.Font.Valid() {
		return 0, 0, 0, fmt.Errorf("invalid retained font, scale or em size")
	}
	m, err := c.Params.Font.Face().Metrics()
	if err != nil {
		return 0, 0, 0, err
	}
	unit := c.Params.Font.Scale().ToDU(1)
	if m.UnitsPerEm == 0 || math.IsNaN(unit) || math.IsInf(unit, 0) || unit <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid font metrics or scale")
	}
	a := c.Params.Font.Scale().ToDU(m.Ascent) / float64(dimen.BP)
	d := -c.Params.Font.Scale().ToDU(m.Descent) / float64(dimen.BP)
	g := c.Params.Font.Scale().ToDU(m.LineGap) / float64(dimen.BP)
	for _, value := range []float64{a, d, g} {
		if !dimension(math.Abs(value)) {
			return 0, 0, 0, fmt.Errorf("font metric outside finite dimension range")
		}
	}
	return math.Max(0, a), math.Max(0, d), math.Max(g, c.Params.Font.EmSize().Points()*.2), nil
}

func validateOptions(options Options) error {
	if options.Policy != ProofCompatibility || options.DefaultEmSize <= 0 || options.DefaultEmSize >= dimen.Fil {
		return fmt.Errorf("invalid paragraph assembly policy or default size")
	}
	return nil
}
