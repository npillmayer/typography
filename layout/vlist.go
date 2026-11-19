// Package layout holds completed immutable paragraph geometry, not layout algorithms.
package layout

import (
	"fmt"
	"math"
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// Line refers to exactly one retained frozen line, without changing horizontal
// order, skips, fit diagnostics or source coordinates. X is the line-box origin
// (zero); the content pen begins at X + Horizontal.LeftSkip.Points(). All float
// fields use fractional big points. Ascent/Descent are effective CSS half-leading
// extents (possibly negative); Height is their sum. These are not glyph ink bounds.
type Line struct {
	Index                                     int
	X, Top, Baseline, Ascent, Descent, Height float64
	Horizontal                                khipu.FrozenLine
}

// VList owns immutable placement slices and retains immutable frozen content.
// Accessors return detached geometry; Frozen shares only khipu's read-only result
// and its retained immutable font/scale services. No caller input is reread later.
type VList struct {
	box      Box
	lines    []Line
	material []Material
	frozen   *khipu.FrozenKhipu
	atomics  []AtomicPlacement
}

// AtomicPlacements returns detached border origins in line/frozen-knot order.
func (v *VList) AtomicPlacements() []AtomicPlacement { return slices.Clone(v.atomics) }

// Box returns logical paragraph geometry by value.
func (v *VList) Box() Box { return v.box }

// Lines returns a detached placement slice in frozen line order.
func (v *VList) Lines() []Line { return slices.Clone(v.lines) }

// Materials returns a detached slice of placed CSS line boxes.
func (v *VList) Materials() []Material { return slices.Clone(v.material) }

// Frozen returns the retained read-only content, sharing its stable font services.
func (v *VList) Frozen() *khipu.FrozenKhipu { return v.frozen }

// AtomicPlacement is a border origin in fractional big points, Y downward.
// Line is zero-based. Placements follow line/frozen-knot order, not numeric X:
// zero advances may coincide and negative advances may move the pen backward.
type AtomicPlacement struct {
	ID   contracts.AtomicID
	Line int
	X, Y float64
}

// NewVList publishes completed CSS geometry. It validates finite geometry, tiled
// line boxes and frozen associations, then copies all slices. No font queries or
// layout occur here. Each material must share its line's exact Top and Height.
// Empty placements may accompany a diagnostic frozen line for suppressed empty
// source. The frozen content and its font instances remain shared and immutable.
func NewVList(frozen *khipu.FrozenKhipu, box Box, lines []Line, materials []Material, atomics []AtomicPlacement) (*VList, error) {
	bad := func() (*VList, error) {
		return nil, fmt.Errorf("%w: invalid completed layout", contracts.ErrIllegalArguments)
	}
	if frozen == nil || !nonnegative(box.Width) || !nonnegative(box.Extent) || !finite(box.Baseline) {
		return bad()
	}
	if len(lines) == 0 {
		if frozen.SourceLen() != 0 || box != (Box{}) || len(materials) != 0 || len(atomics) != 0 {
			return bad()
		}
	} else {
		if len(lines) != frozen.LineCount() || !box.HasBaseline || box.Baseline != lines[0].Baseline {
			return bad()
		}
	}
	local := make([]FragmentLine, len(lines))
	for i, line := range lines {
		local[i] = FragmentLine{Line: line, OriginalIndex: i}
	}
	if _, err := NewFragment(frozen, LineRange{End: len(lines)}, box, local, materials, atomics); err != nil {
		return nil, err
	}
	return &VList{box: box, lines: slices.Clone(lines), material: slices.Clone(materials), frozen: frozen, atomics: slices.Clone(atomics)}, nil
}
func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) < dimen.Fil.Points()
}
func nonnegative(v float64) bool { return finite(v) && v >= 0 }
