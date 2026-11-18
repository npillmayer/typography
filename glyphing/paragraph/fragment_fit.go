package paragraph

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/layout"
)

// FragmentReuseKey attests the inputs of a retained, uniform-width layout.
// LeftSkip and RightSkip hold original [natural, minimum, maximum] widths in DU,
// not the allocated widths of frozen skips. All revision IDs must be nonzero;
// callers must change the corresponding ID whenever an input changes and keep
// borrowed resources stable. IDs are scoped to the caller, not content hashes.
// Frozen geometry can verify Width, but cannot verify the other attestations.
// This value retains no preparation, font service or checkpoint.
type FragmentReuseKey struct {
	Width                                              dimen.DU
	LeftSkip, RightSkip                                [3]dimen.DU
	Source, Styles, Fonts, LineHeight, Atomics, Policy uint64
}

// ErrIncompatibleFragment means retained layout cannot be reused for the request.
var ErrIncompatibleFragment = errors.New("paragraph: incompatible fragment layout")

// FragmentFitStatus describes vertical progress, independently of horizontal fit.
type FragmentFitStatus uint8

const (
	// FitComplete means no visible lines remain, including for empty input.
	FitComplete FragmentFitStatus = iota
	// FitContinues means some lines were placed and some remain.
	FitContinues
	// FitNoProgress means no line fit and the nonempty remainder is unchanged.
	FitNoProgress
)

// FragmentFit is a detached result envelope. Placed and Remainder are non-nil
// immutable fragments sharing the input's frozen owner. Overfull lists original
// frozen line indexes wider than the target, including lines in the remainder;
// suppressed-empty diagnostics are not visible lines. Mutating this envelope or
// Overfull cannot affect the input or another result.
type FragmentFit struct {
	Placed, Remainder *layout.Fragment
	Status            FragmentFitStatus
	Overfull          []int
}

// FitFragment places the longest whole-line prefix fitting height (fractional big
// points) without metrics, binding, shaping or breaking. All boundaries are eligible.
// source attests the retained layout; target must equal it exactly. The complete
// frozen owner must have source.Width, even for a suppressed-empty fragment. This
// entry supports uniform-width owners; Continuation.Fit handles retained width history.
//
// Height must be finite, nonnegative and below dimen.Fil. Zero-height lines count
// as progress. Full CSS line heights, signed extents and source coordinates are
// preserved; the remainder is rebased once. No margins, ink clipping, stretching
// or target-height packing are implied. Existing arithmetic tolerances do not
// permit overflow of height or the finite coordinate domain.
//
// Invalid inputs wrap contracts.ErrIllegalArguments. Incompatibility returns
// ErrIncompatibleFragment. Cancellation preserves its cause. Every failure returns
// nil and leaves prior results reusable. No working preparation is retained, so
// completed fragments remain usable after the paragraph's preparation is released.
func FitFragment(ctx context.Context, f *layout.Fragment, source, target FragmentReuseKey, height float64) (*FragmentFit, error) {
	if ctx == nil {
		return nil, fitInvalid("nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f == nil || f.Frozen() == nil || !validFragmentReuseKey(source) || !fitBounded(height) || height < 0 {
		return nil, fitInvalid("invalid fragment, source key or height")
	}
	if source != target {
		return nil, ErrIncompatibleFragment
	}
	// Reject invalid derived input coordinates even if a cut could hide them
	// inside two individually bounded output boxes.
	if err := validateFitLocal(ctx, f); err != nil {
		return nil, err
	}
	for i := 0; i < f.Frozen().LineCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, ok := f.Frozen().Line(i)
		if !ok || line.TargetWidth != source.Width {
			return nil, ErrIncompatibleFragment
		}
	}
	return fitRetained(ctx, f, source.Width, height)
}

// fitFlowFragment checks a privately retained width history, allowing a frozen
// owner containing earlier widths while its unplaced tail has one uniform width.
// No preparation or service handle enters this geometry-only boundary.
func fitFlowFragment(ctx context.Context, f *layout.Fragment, history []dimen.DU, width dimen.DU, height float64) (*FragmentFit, error) {
	for i := 0; i < f.Frozen().LineCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		want := width
		if i < len(history) {
			want = history[i]
		}
		line, ok := f.Frozen().Line(i)
		if !ok || line.TargetWidth != want {
			return nil, ErrIncompatibleFragment
		}
	}
	return fitRetained(ctx, f, width, height)
}

func fitRetained(ctx context.Context, f *layout.Fragment, width dimen.DU, height float64) (*FragmentFit, error) {
	if err := validateFitLocal(ctx, f); err != nil {
		return nil, err
	}
	out := new(FragmentFit)
	for _, l := range f.Lines() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if l.Horizontal.Width > width {
			out.Overfull = append(out.Overfull, l.OriginalIndex)
		}
	}
	var err error
	out.Placed, out.Remainder, err = fitPrefix(ctx, f, height)
	if err != nil {
		return nil, err
	}
	for _, part := range []*layout.Fragment{out.Placed, out.Remainder} {
		if err := validateFitLocal(ctx, part); err != nil {
			return nil, err
		}
	}
	if out.Remainder.Range().Start != out.Remainder.Range().End {
		out.Status = FitContinues
		if out.Placed.Range().Start == out.Placed.Range().End {
			out.Status = FitNoProgress
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func validFragmentReuseKey(k FragmentReuseKey) bool {
	if k.Width < 0 || k.Width >= dimen.Fil || k.Source == 0 || k.Styles == 0 || k.Fonts == 0 || k.LineHeight == 0 || k.Atomics == 0 || k.Policy == 0 {
		return false
	}
	for _, skip := range [][3]dimen.DU{k.LeftSkip, k.RightSkip} {
		// Only maxima admit fill orders; natural/minimum widths are finite.
		if skip[0] <= -dimen.Fil || skip[0] >= dimen.Fil || skip[1] <= -dimen.Fil || skip[1] >= dimen.Fil ||
			skip[1] > skip[0] || skip[0] > skip[2] || skip[2] > dimen.Filll {
			return false
		}
	}
	return true
}

func fitPrefix(ctx context.Context, f *layout.Fragment, height float64) (*layout.Fragment, *layout.Fragment, error) {
	lines := f.Lines()
	cut := 0
	for i, l := range lines {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		// Use authoritative ends, not a new sum of rounded heights. A retained
		// box may be slightly taller under arithmetic-equality tolerance.
		end := l.Top + l.Height
		if i == len(lines)-1 {
			end = math.Max(end, f.Box().Extent)
		}
		if !fitBounded(end) {
			return nil, nil, fitInvalid("line end outside dimension domain")
		}
		if end > height {
			break
		}
		cut++
	}
	span := f.Range()
	if cut == 0 {
		empty, err := layout.NewFragment(ctx, f.Frozen(), layout.LineRange{Start: span.Start, End: span.Start}, layout.Box{}, nil, nil, nil)
		return empty, f, err
	}
	if cut == len(lines) {
		empty, err := layout.NewFragment(ctx, f.Frozen(), layout.LineRange{Start: span.End, End: span.End}, layout.Box{}, nil, nil, nil)
		return f, empty, err
	}
	first, rest, err := SplitFragment(ctx, f, cut)
	if err != nil {
		return nil, nil, err
	}
	if first.Box().Extent > height {
		return nil, nil, fitInvalid("split exceeds available height")
	}
	return first, rest, nil
}

// Publication validates placed geometry. Derived local geometry also needs
// strict domain checks, without tightening arithmetic-equality tolerances.
func validateFitLocal(ctx context.Context, f *layout.Fragment) error {
	if !fitBounded(f.Box().Depth()) {
		return fitInvalid("fragment depth outside dimension domain")
	}
	lines := f.Lines()
	for _, l := range lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		baseline := l.Baseline - l.Top
		if !fitBounded(baseline) || !fitBounded(l.Height-baseline) {
			return fitInvalid("local line geometry outside dimension domain")
		}
	}
	for _, a := range f.AtomicPlacements() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !fitBounded(a.Y - lines[a.Line].Top) {
			return fitInvalid("local atomic geometry outside dimension domain")
		}
	}
	return nil
}

func fitBounded(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) < dimen.Fil.Points()
}

func fitInvalid(s string) error {
	return fmt.Errorf("%w: paragraph fragment fit: %s", contracts.ErrIllegalArguments, s)
}
