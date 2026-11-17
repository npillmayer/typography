package paragraph

import (
	"context"
	"fmt"
	"math"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography/layout"
)

// Fragment makes a whole-VList immutable view without recomputing geometry.
// It retains the frozen owner, including diagnostic content for suppressed empty
// source. Custom material arrangements unsupported by layout.NewFragment fail.
func Fragment(ctx context.Context, v *layout.VList) (*layout.Fragment, error) {
	if ctx == nil || v == nil {
		return nil, fmt.Errorf("%w: fragment requires context and vlist", contracts.ErrIllegalArguments)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lines := v.Lines()
	local := make([]layout.FragmentLine, len(lines))
	for i, l := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		local[i] = layout.FragmentLine{Line: l, OriginalIndex: i}
	}
	return layout.NewFragment(ctx, v.Frozen(), layout.LineRange{End: len(lines)}, v.Box(), local, v.Materials(), v.AtomicPlacements())
}

// SplitFragment partitions at an interior, fragment-local line boundary.
// All CSS line boxes, including their leading, are preserved. The second
// fragment is rebased by its first line's former top. Frozen
// content, source coordinates and horizontal order remain unchanged; no metrics,
// shaping or rendering services are called. Both results are nil on failure.
func SplitFragment(ctx context.Context, f *layout.Fragment, at int) (*layout.Fragment, *layout.Fragment, error) {
	if ctx == nil || f == nil || f.Frozen() == nil || at <= 0 || at >= f.Range().End-f.Range().Start {
		return nil, nil, fmt.Errorf("%w: split requires an interior line boundary", contracts.ErrIllegalArguments)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	lines, materials, atomics := f.Lines(), f.Materials(), f.AtomicPlacements()
	// Partition by material order, not Top: zero-height lines can coincide.
	cut := 0
	for cut < len(materials) && !(materials[cut].Kind == layout.LineBox && materials[cut].Child == at) {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		cut++
	}
	acut := 0
	for acut < len(atomics) && atomics[acut].Line < at {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		acut++
	}
	first, err := fragmentPart(ctx, f, 0, lines[:at], materials[:cut], atomics[:acut])
	if err != nil {
		return nil, nil, err
	}
	second, err := fragmentPart(ctx, f, at, lines[at:], materials[cut:], atomics[acut:])
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return first, second, nil
}

// fragmentPart consumes private detached slices. Extent follows the final
// retained material, avoiding a new sum of rounded line heights.
func fragmentPart(ctx context.Context, owner *layout.Fragment, offset int, lines []layout.FragmentLine, materials []layout.Material, atomics []layout.AtomicPlacement) (*layout.Fragment, error) {
	origin := lines[0].Top
	box := layout.Box{HasBaseline: true}
	for i := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		l := &lines[i]
		l.Index = i
		l.Top -= origin
		l.Baseline -= origin
		box.Width = math.Max(box.Width, math.Max(l.Horizontal.Width.Points(), l.Horizontal.TargetWidth.Points()))
	}
	for i := range materials {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		materials[i].Top -= origin
		if materials[i].Kind == layout.LineBox {
			materials[i].Child -= offset
		}
	}
	for i := range atomics {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		atomics[i].Line -= offset
		atomics[i].Y -= origin
	}
	box.Baseline = lines[0].Baseline
	last := materials[len(materials)-1]
	box.Extent = last.Top + last.Extent
	start := owner.Range().Start + offset
	return layout.NewFragment(ctx, owner.Frozen(), layout.LineRange{Start: start, End: start + len(lines)}, box, lines, materials, atomics)
}
