package paragraph

import (
	"context"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// CSSLineHeight stacks baseline-aligned line boxes with a paragraph strut and
// half-leading. Unlike ProofCompatibility, it adds no interline/trailing kern.
const CSSLineHeight SpacingPolicy = "css-line-height-v1"

// LineHeightKind selects font metrics, an em multiplier or a resolved DU length.
type LineHeightKind string

const (
	// NormalHeight uses ascent, descent and nonnegative font line gap.
	NormalHeight LineHeightKind = "normal"
	// FactorHeight multiplies the receiving run's em size.
	FactorHeight LineHeightKind = "factor"
	// LengthHeight uses an already resolved DU length.
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
// affect only text line boxes, never shaping. All intersecting text ranges
// contribute even if their boundary falls within a ligature or combining
// sequence. Atomic markers use box geometry instead of text half-leading.
type LineHeightSpan struct {
	Start, End uint64
	Height     LineHeight
}

// LineLayout supplies paragraph and per-run line-height values separately from
// typography styles. The paragraph strut uses Streaming.Defaults.Font/EmSize.
// Run contributions use retained shaping metrics, including fallback faces;
// surviving spaces use resolved source styles. Discarded spaces do not contribute.
// Text uses baseline alignment; Prepared.Layout adds bottom-margin-edge atomic
// alignment. CSS edge computation and height-constrained packing are external.
// Keep these values stable during Run/Layout; each call copies the range slice,
// and results retain only geometry.
type LineLayout struct {
	Default LineHeight
	Runs    []LineHeightSpan `json:",omitempty"`
}

// Clone returns a value copy with a detached Runs slice.
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
func sourceMetrics(ctx context.Context, text styled.Text, cfg khipu.Config, excluded []khipu.ByteRange) ([]metricRun, error) {
	var runs []metricRun
	object := 0
	for run := range text.StyleRanges() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for object < len(excluded) && excluded[object].End <= run.Position {
			object++
		}
		end := run.Position
		for j := object; j < len(excluded) && excluded[j].Start <= end; j++ {
			end = max(end, excluded[j].End)
		}
		if end >= run.Position+run.Length {
			continue
		} // Entirely attached markers.
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
