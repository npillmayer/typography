package layout

import (
	"fmt"
	"math"
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
)

// LineRange is a half-open interval of original frozen paragraph line indexes.
type LineRange struct{ Start, End int }

// FragmentLine is a local placement with an explicit original paragraph index.
// Line.Index is local to the fragment. Horizontal knot indexes and source
// coordinates are unchanged and belong to the shared frozen paragraph.
type FragmentLine struct {
	Line
	OriginalIndex int
}

// Fragment retains a contiguous range of immutable horizontal lines and owns
// detached vertical geometry. It keeps the entire frozen paragraph alive.
// Its zero value is not a valid fragment; use NewFragment or paragraph.Fragment.
type Fragment struct {
	frozen    *khipu.FrozenKhipu
	span      LineRange
	box       Box
	lines     []FragmentLine
	materials []Material
	atomics   []AtomicPlacement
}

// Frozen returns the shared immutable paragraph, not a sliced or rebased owner.
func (f *Fragment) Frozen() *khipu.FrozenKhipu { return f.frozen }

// Range returns the original paragraph line interval.
func (f *Fragment) Range() LineRange { return f.span }

// Box returns completed local geometry in fractional big points.
func (f *Fragment) Box() Box { return f.box }

// Lines returns detached local placements with original line identities.
func (f *Fragment) Lines() []FragmentLine { return slices.Clone(f.lines) }

// Materials returns detached material; Child indexes the local line slice.
func (f *Fragment) Materials() []Material { return slices.Clone(f.materials) }

// AtomicPlacements returns detached origins in local line/frozen traversal order.
func (f *Fragment) AtomicPlacements() []AtomicPlacement { return slices.Clone(f.atomics) }

// NewFragment validates and copies completed geometry without font queries.
// Supported material is exactly one CSS LineBox per line, sharing its exact
// Top and Height. Lines tile the box from top zero; X must be zero.
// Empty ranges have zero geometry, even when retaining diagnostic frozen content.
// Geometry identities allow 1e-9 big points of rounding error from rebasing;
// finite-domain checks are strict. No geometry is normalized or repaired.
// Cancellation or invalid input returns no fragment.
func NewFragment(frozen *khipu.FrozenKhipu, span LineRange, box Box, lines []FragmentLine, materials []Material, atomics []AtomicPlacement) (*Fragment, error) {
	bad := func() (*Fragment, error) {
		return nil, fmt.Errorf("%w: invalid completed fragment", contracts.ErrIllegalArguments)
	}

	if frozen == nil || span.Start < 0 || span.End < span.Start || span.End > frozen.LineCount() || len(lines) != span.End-span.Start {
		return bad()
	}
	if !nonnegative(box.Width) || !nonnegative(box.Extent) || !finite(box.Baseline) {
		return bad()
	}
	if len(lines) == 0 {
		if box != (Box{}) || len(materials) != 0 || len(atomics) != 0 {
			return bad()
		}
	} else if !box.HasBaseline || box.Baseline != lines[0].Baseline || lines[0].Top != 0 {
		return bad()
	}
	mi, ai := 0, 0
	end, width, previousTop := 0.0, 0.0, 0.0
	for i, l := range lines {
		h, ok := frozen.Line(span.Start + i)
		if !ok || l.Index != i || l.OriginalIndex != span.Start+i || l.Horizontal != h || l.X != 0 || h.Width < 0 || h.TargetWidth < 0 {
			return bad()
		}
		for _, n := range []float64{l.Baseline, l.Ascent, l.Descent} {
			if !finite(n) {
				return bad()
			}
		}
		for _, n := range []float64{l.Top, l.Height, l.Top + l.Height, l.Ascent + l.Descent} {
			if !nonnegative(n) {
				return bad()
			}
		}
		if !finite(l.Top+l.Ascent) || !closeGeometry(l.Top, end) || !closeGeometry(l.Baseline, l.Top+l.Ascent) || !closeGeometry(l.Height, l.Ascent+l.Descent) {
			return bad()
		}
		if mi >= len(materials) {
			return bad()
		}
		m := materials[mi]
		if m.Top < previousTop || m.Kind != LineBox || m.Child != i || !validFragmentMaterial(m) || m.Top != l.Top || m.Extent != l.Height {
			return bad()
		}
		mi++
		end = m.Top + m.Extent
		previousTop = m.Top
		width = math.Max(width, math.Max(h.Width.Points(), h.TargetWidth.Points()))
		for k := h.FirstKnot; k < h.FirstKnot+h.KnotCount; k++ {
			if id, ok := frozen.Atomic(k); ok {
				if ai >= len(atomics) {
					return bad()
				}
				a := atomics[ai]
				if a.ID != id || a.Line != i || !finite(a.X) || !finite(a.Y) {
					return bad()
				}
				ai++
			}
		}
	}
	if !nonnegative(width) || mi != len(materials) || ai != len(atomics) || !closeGeometry(box.Extent, end) || !closeGeometry(box.Width, width) {
		return bad()
	}

	result := &Fragment{frozen: frozen, span: span, box: box, lines: slices.Clone(lines), materials: slices.Clone(materials), atomics: slices.Clone(atomics)}

	return result, nil
}

func closeGeometry(a, b float64) bool { return math.Abs(a-b) <= 1e-9 }
func validFragmentMaterial(m Material) bool {
	return nonnegative(m.Top) && nonnegative(m.Extent) && nonnegative(m.Top+m.Extent)
}
