package layout

import (
	"fmt"
	"math"
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
)

// VItem is an immutable vertical-list item in local box coordinates. It currently
// contains one measured horizontal line with its full CSS line-height. It retains
// frozen content directly, without a VList backlink or former line index.
// Obtain valid items through Unbox; the zero value is not a content item.
type VItem struct {
	frozen          *khipu.FrozenKhipu
	horizontal      khipu.FrozenLine
	box             Box
	ascent, descent float64
	atomics         []AtomicPlacement
}

// Box returns detached logical geometry in fractional big points. Its baseline
// is relative to this item's top and may lie outside its nonnegative extent.
func (item VItem) Box() Box { return item.box }

// Extents returns the original signed effective ascent and descent, including
// half-leading. Subtracting the old line top to obtain Box().Baseline may introduce
// roundoff; the original extents and full line height are preserved independently.
func (item VItem) Extents() (ascent, descent float64) { return item.ascent, item.descent }

// Horizontal returns the shared immutable content store and a detached line
// record whose knot range addresses that store. No VList lookup is needed.
// The frozen content and its font/scale services obey their retained lifetimes.
func (item VItem) Horizontal() (*khipu.FrozenKhipu, khipu.FrozenLine) {
	return item.frozen, item.horizontal
}

// AtomicPlacements returns a detached slice of border origins relative to this
// item's top-left corner, in fractional big points with Y downward. Line is zero;
// IDs and frozen physical traversal order are unchanged, even at coincident or
// decreasing X positions.
func (item VItem) AtomicPlacements() []AtomicPlacement { return slices.Clone(item.atomics) }

// ElasticVList is an immutable ordered sequence of VItems awaiting vertical
// placement. Its zero value is empty. Items have fixed measured geometry: the
// name does not provide stretch/shrink, glue, mutation or target-height packing.
type ElasticVList struct {
	items []VItem
}

// Len returns the number of items. A nil list is empty.
func (v *ElasticVList) Len() int {
	if v == nil {
		return 0
	}
	return len(v.items)
}

// Item returns an immutable item by value, or false for an out-of-range index.
// The returned item keeps its content alive independently of this list.
func (v *ElasticVList) Item(i int) (VItem, bool) {
	if i < 0 || i >= v.Len() {
		return VItem{}, false
	}
	return v.items[i], true
}

// Unbox recovers measured line boxes without mutating v, measuring, shaping or
// breaking again. Full line-height, signed extents and frozen content coordinates
// are preserved; baselines and atomic origins become item-local. Empty output
// stays empty even when the frozen source retains a diagnostic line.
//
// A valid non-nil VList is required. Its constructors already validate source
// associations and geometry; Unbox additionally validates the translated values.
// Failure returns nil and an error wrapping contracts.ErrIllegalArguments, never
// partial items. The operation is synchronous and has no cancellation parameter.
func Unbox(v *VList) (*ElasticVList, error) {
	if v == nil || v.frozen == nil {
		return nil, fmt.Errorf("%w: unbox requires a valid vlist", contracts.ErrIllegalArguments)
	}
	result := &ElasticVList{items: make([]VItem, len(v.lines))}
	ai := 0
	for i, line := range v.lines {
		start := ai
		for ai < len(v.atomics) && v.atomics[ai].Line == i {
			ai++
		}
		item, err := unboxLine(v.frozen, line, v.atomics[start:ai])
		if err != nil {
			return nil, fmt.Errorf("unbox line %d: %w", i, err)
		}
		result.items[i] = item
	}
	return result, nil
}

func unboxLine(frozen *khipu.FrozenKhipu, line Line, atomics []AtomicPlacement) (VItem, error) {
	bad := func() (VItem, error) {
		return VItem{}, fmt.Errorf("%w: item geometry outside finite dimension range", contracts.ErrIllegalArguments)
	}
	box := Box{
		Width:       math.Max(line.Horizontal.Width.Points(), line.Horizontal.TargetWidth.Points()),
		Extent:      line.Height,
		Baseline:    line.Baseline - line.Top,
		HasBaseline: true,
	}
	if !nonnegative(box.Width) || !nonnegative(box.Extent) || !finite(box.Baseline) || !finite(line.Ascent) || !finite(line.Descent) {
		return bad()
	}
	// Rebasing and tolerated source roundoff can put the derived depth outside
	// the finite domain even when the retained descent is still inside it.
	if !finite(box.Depth()) {
		return bad()
	}
	// The source constructor validated the arithmetic identities before rebasing.
	// Do not turn subtraction roundoff into a stricter second identity check.
	local := slices.Clone(atomics)
	for i := range local {
		local[i].Line = 0
		local[i].X -= line.X
		local[i].Y -= line.Top
		if !finite(local[i].X) || !finite(local[i].Y) {
			return bad()
		}
	}
	return VItem{frozen: frozen, horizontal: line.Horizontal, box: box,
		ascent: line.Ascent, descent: line.Descent, atomics: local}, nil
}
