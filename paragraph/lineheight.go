package paragraph

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// CSSLineHeight stacks baseline-aligned line boxes with a paragraph strut and
// half-leading. Unlike ProofCompatibility, it adds no interline/trailing kern.
const CSSLineHeight SpacingPolicy = "css-line-height-v1"

type LineHeightKind string

const (
	NormalHeight LineHeightKind = "normal"
	FactorHeight LineHeightKind = "factor"
	LengthHeight LineHeightKind = "length"
)

// LineHeight is a computed layout value. Normal uses ascent + descent + the
// nonnegative font line gap. Factor multiplies the receiving run's em size;
// Length is an already computed absolute DU length (including CSS percentages).
// Zero factor/length is valid; negative/nonfinite values and unused fields fail.
// CSS parsing, inheritance and percentage resolution belong to the input adapter.
type LineHeight struct {
	Kind   LineHeightKind
	Factor float64  `json:",omitempty"`
	Length dimen.DU `json:",omitempty"`
}

// LineHeightSpan overrides Default on a nonempty half-open UTF-8 byte range.
// Ranges are ordered and nonoverlapping; gaps inherit Default. These boundaries
// affect only line boxes, never shaping. All intersecting ranges contribute even
// if their boundary falls within a ligature or combining sequence.
type LineHeightSpan struct {
	Start, End uint64
	Height     LineHeight
}

// LineLayout supplies paragraph and per-run line-height values separately from
// typography styles. The paragraph strut uses Streaming.Defaults.Font/EmSize.
// Run contributions use retained shaping metrics, including fallback faces;
// surviving spaces use resolved source styles. Discarded spaces do not contribute.
// Baseline alignment only; no vertical-align, block padding/margins or packing.
// Callers keep these values stable until Run returns; results retain only geometry.
type LineLayout struct {
	Default LineHeight
	Runs    []LineHeightSpan `json:",omitempty"`
}

func (l LineLayout) Clone() *LineLayout {
	l.Runs = append([]LineHeightSpan(nil), l.Runs...)
	return &l
}

func (h LineHeight) validate() error {
	switch h.Kind {
	case NormalHeight:
		if h.Factor == 0 && h.Length == 0 {
			return nil
		}
	case FactorHeight:
		if !math.IsNaN(h.Factor) && !math.IsInf(h.Factor, 0) && h.Factor >= 0 && h.Length == 0 {
			return nil
		}
	case LengthHeight:
		if h.Factor == 0 && h.Length >= 0 && h.Length < dimen.Fil {
			return nil
		}
	}
	return fmt.Errorf("invalid line-height value %+v", h)
}

func validateLineLayout(l *LineLayout, text styled.Text, cfg khipu.Config) error {
	if l == nil || cfg.Streaming == nil || nilService(cfg.Streaming.Resolver) || nilService(cfg.Streaming.Defaults.Font) || cfg.Streaming.Defaults.EmSize <= 0 || cfg.Streaming.Defaults.EmSize >= dimen.Fil {
		return fmt.Errorf("line-height policy requires layout values and explicit streaming resolver/font defaults")
	}
	if err := l.Default.validate(); err != nil {
		return err
	}
	var end uint64
	for _, s := range l.Runs {
		if s.Start < end || s.End <= s.Start || s.End > text.Raw().Len() {
			return fmt.Errorf("invalid line-height range [%d,%d)", s.Start, s.End)
		}
		for _, p := range []uint64{s.Start, s.End} {
			if p < text.Raw().Len() {
				var b [1]byte
				_, err := io.ReadFull(text.Raw().BoundedReader(p, p+1), b[:])
				if err != nil || !utf8.RuneStart(b[0]) {
					return fmt.Errorf("line-height range is not on UTF-8 boundaries")
				}
			}
		}
		if err := s.Height.validate(); err != nil {
			return err
		}
		end = s.End
	}
	return nil
}

type fontExtents struct{ a, d, g, em float64 }

func cssMetrics(face contracts.FontFace, scale contracts.FontScale, size dimen.DU) (fontExtents, error) {
	if nilService(face) || nilService(scale) || size <= 0 || size >= dimen.Fil {
		return fontExtents{}, fmt.Errorf("invalid line-height font/scale/size")
	}
	m, err := face.Metrics()
	if err != nil {
		return fontExtents{}, err
	}
	unit := scale.ToDU(1)
	if m.UnitsPerEm == 0 || !dimension(unit/float64(dimen.BP)) || unit <= 0 {
		return fontExtents{}, fmt.Errorf("invalid line-height metrics/scale")
	}
	e := fontExtents{scale.ToDU(m.Ascent) / float64(dimen.BP), -scale.ToDU(m.Descent) / float64(dimen.BP), scale.ToDU(m.LineGap) / float64(dimen.BP), size.Points()}
	if !dimension(e.a) || !dimension(e.d) || !dimension(math.Abs(e.g)) || !dimension(e.a+e.d) {
		return fontExtents{}, fmt.Errorf("invalid line-height font extents")
	}
	return e, nil
}
func cssFaceMetrics(s contracts.ResolvedRunStyle) (fontExtents, error) {
	if nilService(s.Font) || s.EmSize <= 0 || s.EmSize >= dimen.Fil {
		return fontExtents{}, fmt.Errorf("missing line-height font")
	}
	scale, err := s.Font.Scale(s.EmSize)
	if err != nil {
		return fontExtents{}, err
	}
	return cssMetrics(s.Font, scale, s.EmSize)
}
func (h LineHeight) extents(e fontExtents) (float64, float64, error) {
	var height float64
	switch h.Kind {
	case NormalHeight:
		height = e.a + e.d + math.Max(0, e.g)
	case FactorHeight:
		height = h.Factor * e.em
	case LengthHeight:
		height = h.Length.Points()
	default:
		return 0, 0, fmt.Errorf("invalid line-height kind")
	}
	leading := (height - e.a - e.d) / 2
	a := e.a + leading
	d := height - a // preserves a zero-height box exactly even with fractional metrics
	if !dimension(height) || !dimension(math.Abs(a)) || !dimension(math.Abs(d)) {
		return 0, 0, fmt.Errorf("line-height outside finite dimension range")
	}
	return a, d, nil
}

type metricRun struct {
	start, end uint64
	metrics    fontExtents
}

// Source styles are consulted only for surviving non-text material (spaces).
// Actual glyphs always use their frozen font and scale, including replacements.
func sourceMetrics(ctx context.Context, text styled.Text, cfg khipu.Config) ([]metricRun, error) {
	var runs []metricRun
	for run := range text.StyleRanges() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s, err := cfg.Streaming.Resolver.ResolveStyle(run.Style, cfg.Streaming.Defaults)
		if err != nil {
			return nil, err
		}
		m, err := cssFaceMetrics(s)
		if err != nil {
			return nil, err
		}
		runs = append(runs, metricRun{run.Position, run.Position + run.Length, m})
	}
	return runs, nil
}

func assembleLineHeight(ctx context.Context, f *khipu.FrozenKhipu, l *LineLayout, text styled.Text, cfg khipu.Config) (*VList, error) {
	strut, err := cssFaceMetrics(cfg.Streaming.Defaults)
	if err != nil {
		return nil, err
	}
	sa, sd, err := l.Default.extents(strut)
	if err != nil {
		return nil, err
	}
	runs, err := sourceMetrics(ctx, text, cfg)
	if err != nil {
		return nil, err
	}
	v := &VList{frozen: f}
	top := 0.0
	for i := 0; i < f.LineCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, _ := f.Line(i)
		a, d := sa, sd
		contribute := func(start, end uint64, m fontExtents) error {
			add := func(h LineHeight) error {
				x, y, e := h.extents(m)
				if e != nil {
					return e
				}
				a = math.Max(a, x)
				d = math.Max(d, y)
				return nil
			}
			at := start
			j := sort.Search(len(l.Runs), func(j int) bool { return l.Runs[j].End > start })
			for ; j < len(l.Runs) && l.Runs[j].Start < end; j++ {
				s := l.Runs[j]
				if s.Start > at {
					if e := add(l.Default); e != nil {
						return e
					}
				}
				if e := add(s.Height); e != nil {
					return e
				}
				at = min(end, s.End)
			}
			if at < end {
				return add(l.Default)
			}
			return nil
		}
		for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			knot, _ := f.Knot(k)
			src := knot.Source.Bytes
			if src.End <= src.Start {
				continue
			} // generated paragraph fill
			tx, ok := f.CopyText(k)
			if ok {
				if len(tx.Glyphs) == 0 {
					continue
				} // silent discretionary material
				m, e := cssMetrics(tx.Context.Params.Font, tx.Context.Scale, tx.Context.EmSize)
				if e != nil {
					return nil, e
				}
				if e = contribute(src.Start, src.End, m); e != nil {
					return nil, e
				}
			} else {
				// Glue has no retained text context; locate its prepared source font(s).
				j := sort.Search(len(runs), func(j int) bool { return runs[j].end > src.Start })
				for ; j < len(runs) && runs[j].start < src.End; j++ {
					r := runs[j]
					if e := contribute(max(src.Start, r.start), min(src.End, r.end), r.metrics); e != nil {
						return nil, e
					}
				}
			}
		}
		height := a + d
		baseline := top + a
		end := top + height
		width := math.Max(line.Width.Points(), line.TargetWidth.Points())
		if !dimension(height) || !dimension(end) || !dimension(math.Abs(baseline)) || !dimension(width) || line.Width < 0 || line.TargetWidth < 0 {
			return nil, fmt.Errorf("line-height line %d: geometry outside finite dimension range", i)
		}
		v.lines = append(v.lines, Line{Index: i, Top: top, Baseline: baseline, Ascent: a, Descent: d, Height: height, Horizontal: line})
		v.material = append(v.material, Material{Kind: LineBox, Child: i, Top: top, Extent: height})
		v.box.Width = math.Max(v.box.Width, width)
		if i == 0 {
			v.box.Baseline = baseline
			v.box.HasBaseline = true
		}
		top = end
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v.box.Extent = top
	return v, nil
}
