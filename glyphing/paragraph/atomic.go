package paragraph

import (
	"fmt"
	"math"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/layout"
)

// AtomicMargins contains resolved signed margins in DU, added to border dimensions.
// CSS margin computation belongs to the input adapter.
type AtomicMargins struct {
	Top, Right, Bottom, Left dimen.DU
}

// AtomicAlignment selects how a preboxed occurrence meets the line baseline.
// Only AlignBottomMarginEdge is supported; zero is invalid.
type AtomicAlignment uint8

// AlignBottomMarginEdge places the bottom margin edge on the line baseline.
// It does not model an inline-block's internal text baseline.
const AlignBottomMarginEdge AtomicAlignment = iota + 1

// AtomicBox describes a preboxed occurrence. Borders already include padding;
// only margins are added here. Border dimensions are nonnegative, margins signed.
// All dimensions and completed sums must have magnitude below dimen.Fil.
type AtomicBox struct {
	ID                        contracts.AtomicID
	BorderWidth, BorderHeight dimen.DU
	Margin                    AtomicMargins
	Alignment                 AtomicAlignment
}

func validateAtomics(anchors []contracts.AtomicAnchor, input []AtomicBox) (map[contracts.AtomicID]AtomicBox, []khipu.AtomicAdvance, error) {
	bad := func() (map[contracts.AtomicID]AtomicBox, []khipu.AtomicAdvance, error) {
		return nil, nil, fmt.Errorf("%w: atomic IDs, alignment or geometry", contracts.ErrIllegalArguments)
	}
	if len(input) != len(anchors) {
		return bad()
	}
	boxes := make(map[contracts.AtomicID]AtomicBox, len(input))
	advances := make([]khipu.AtomicAdvance, 0, len(input))
	for _, b := range input {
		if _, duplicate := boxes[b.ID]; duplicate || b.ID == 0 || b.Alignment != AlignBottomMarginEdge ||
			b.BorderWidth < 0 || b.BorderWidth >= dimen.Fil || b.BorderHeight < 0 || b.BorderHeight >= dimen.Fil {
			return bad()
		}
		for _, m := range []dimen.DU{b.Margin.Top, b.Margin.Right, b.Margin.Bottom, b.Margin.Left} {
			if !finiteSigned(int64(m)) {
				return bad()
			}
		}
		// Each primitive is int32; sums of three fit int64. Validate completed
		// quantities, allowing intermediate prefixes outside the DU domain.
		advance := int64(b.BorderWidth) + int64(b.Margin.Left) + int64(b.Margin.Right)
		if !finiteSigned(advance) || !finiteSigned(b.above()) {
			return bad()
		}
		boxes[b.ID] = b
		advances = append(advances, khipu.AtomicAdvance{ID: b.ID, Advance: dimen.DU(advance)})
	}
	for _, a := range anchors {
		if _, ok := boxes[a.ID]; !ok {
			return bad()
		}
	}
	return boxes, advances, nil
}

func finiteSigned(v int64) bool { return v > -int64(dimen.Fil) && v < int64(dimen.Fil) }
func (b AtomicBox) above() int64 {
	return int64(b.Margin.Top) + int64(b.BorderHeight) + int64(b.Margin.Bottom)
}

// placeAtomics traverses physical frozen order exactly once, including glue and
// fill in pen advancement. No second bidi reordering or coordinate sort occurs.
func placeAtomics(v *assembly, boxes map[contracts.AtomicID]AtomicBox) error {
	for _, line := range v.lines {
		pen := int64(line.Horizontal.LeftSkip)
		for i := line.Horizontal.FirstKnot; i < line.Horizontal.FirstKnot+line.Horizontal.KnotCount; i++ {
			knot, _ := v.frozen.Knot(i)
			if id, ok := v.frozen.Atomic(i); ok {
				box, exists := boxes[id]
				if !exists {
					return fmt.Errorf("%w: missing atomic box %d", contracts.ErrIllegalArguments, id)
				}
				xDU, err := addPen(pen, int64(box.Margin.Left))
				if err != nil {
					return err
				}
				x := line.X + float64(xDU)/float64(dimen.BP)
				y := line.Baseline - box.Margin.Bottom.Points() - box.BorderHeight.Points()
				if !dimension(math.Abs(x)) || !dimension(math.Abs(y)) {
					return fmt.Errorf("%w: atomic border origin outside finite domain", contracts.ErrIllegalArguments)
				}
				v.atomics = append(v.atomics, layout.AtomicPlacement{ID: id, Line: line.Index, X: x, Y: y})
			}
			// Intermediate positions may leave the DU domain. Only published
			// coordinates and completed line dimensions are domain-checked.
			var err error
			pen, err = addPen(pen, int64(knot.W))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func addPen(pen, delta int64) (int64, error) {
	if (delta > 0 && pen > math.MaxInt64-delta) || (delta < 0 && pen < math.MinInt64-delta) {
		return 0, fmt.Errorf("%w: atomic placement accumulation overflow", contracts.ErrIllegalArguments)
	}
	return pen + delta, nil
}
