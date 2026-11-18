package flowexperiment

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

// reuseKey is an experiment-only caller attestation, not a content hash. Revisions
// identify stable source/styles, selected font instances, line-height, atomic
// geometry and paragraph policy. Any change requires a new nonzero revision.
// Retained VLists cannot reconstruct those inputs. Skips retain their input
// width triples, rather than the allocated per-line frozen skip widths.
type reuseKey struct {
	width                                              dimen.DU
	left, right                                        [3]dimen.DU
	source, styles, fonts, lineHeight, atomics, policy uint64
}

type frame struct {
	key    reuseKey
	height float64 // Remaining content height in big points; no surrounding edges.
}

type outcome uint8

const (
	complete outcome = iota
	moreFrames
	noProgress
)

var errIncompatible = errors.New("flow experiment: incompatible retained layout")

// splitResult is a detached envelope. Tail retains everything not accepted by
// A or B; exhausted frames never discard source. All three fragments are non-nil
// and share the original frozen owner, including suppressed empty diagnostics.
type splitResult struct {
	a, b, tail                    *layout.Fragment
	status                        outcome
	overfull                      []int // Original line indexes whose occupied width exceeds the frame.
	examinedLines, interiorSplits int   // Explicit geometry work, not timing/allocation estimates.
}

// splitEqual has no preparation, service or breaker handle. It only partitions
// completed geometry. All line boundaries are permitted in the E1 policy.
func splitEqual(ctx context.Context, v *layout.VList, source reuseKey, a, b frame) (*splitResult, error) {
	if ctx == nil {
		return nil, illegal("nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if v == nil || v.Frozen() == nil || !validKey(source) || !bounded(a.height) || !bounded(b.height) || a.height < 0 || b.height < 0 {
		return nil, illegal("invalid source or frames")
	}
	if a.key != source || b.key != source {
		return nil, errIncompatible
	}
	// Visible placements omit suppressed-empty diagnostics. Check the complete
	// frozen owner too, without turning diagnostic lines into placed material.
	frozen := v.Frozen()
	for i := 0; i < frozen.LineCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, ok := frozen.Line(i)
		if !ok || line.TargetWidth != source.width {
			return nil, errIncompatible
		}
	}
	whole, err := paragraph.Fragment(ctx, v)
	if err != nil {
		return nil, err
	}
	out := &splitResult{}
	for _, l := range whole.Lines() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if l.Horizontal.Width > source.width {
			out.overfull = append(out.overfull, l.OriginalIndex)
		}
	}
	rest := whole
	out.a, rest, err = take(ctx, rest, a.height, out)
	if err != nil {
		return nil, fmt.Errorf("frame A: %w", err)
	}
	out.b, out.tail, err = take(ctx, rest, b.height, out)
	if err != nil {
		return nil, fmt.Errorf("frame B: %w", err)
	}
	for _, f := range []*layout.Fragment{out.a, out.b, out.tail} {
		if err := checkLocalGeometry(ctx, f); err != nil {
			return nil, err
		}
	}
	if out.tail.Range().Start != out.tail.Range().End {
		out.status = moreFrames
		if out.a.Range().Start == out.a.Range().End && out.b.Range().Start == out.b.Range().End {
			out.status = noProgress
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func validKey(k reuseKey) bool {
	if k.width < 0 || k.width >= dimen.Fil || k.source == 0 || k.styles == 0 || k.fonts == 0 || k.lineHeight == 0 || k.atomics == 0 || k.policy == 0 {
		return false
	}
	for _, skip := range [][3]dimen.DU{k.left, k.right} {
		// Match khipu's width-triple domain: natural/minimum widths are finite;
		// only maxima admit Fil/Fill/Filll. Infinity is never a fill order.
		if skip[0] <= -dimen.Fil || skip[0] >= dimen.Fil || skip[1] <= -dimen.Fil || skip[1] >= dimen.Fil ||
			skip[1] > skip[0] || skip[0] > skip[2] || skip[2] > dimen.Filll {
			return false
		}
	}
	return true
}

// take uses the authoritative placed ends, not a re-sum of rounded heights.
// The whole fragment's retained Box must fit too; its arithmetic tolerance must
// never become permission to exceed the requested height.
func take(ctx context.Context, f *layout.Fragment, height float64, work *splitResult) (*layout.Fragment, *layout.Fragment, error) {
	lines := f.Lines()
	cut := 0
	for i, l := range lines {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		work.examinedLines++
		end := l.Top + l.Height
		if i == len(lines)-1 {
			end = math.Max(end, f.Box().Extent)
		}
		if !bounded(end) {
			return nil, nil, illegal("line end outside dimension domain")
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
	work.interiorSplits++
	first, rest, err := paragraph.SplitFragment(ctx, f, cut)
	if err != nil {
		return nil, nil, err
	}
	if first.Box().Extent > height {
		return nil, nil, illegal("split exceeds frame height")
	}
	return first, rest, nil
}

// Fragment validates placed geometry. Also check derived depths and local
// coordinates before retaining a result; signed extents remain signed.
func checkLocalGeometry(ctx context.Context, f *layout.Fragment) error {
	if !bounded(f.Box().Depth()) {
		return illegal("fragment depth outside dimension domain")
	}
	lines := f.Lines()
	for _, l := range lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		baseline := l.Baseline - l.Top
		if !bounded(baseline) || !bounded(l.Height-baseline) {
			return illegal("local line geometry outside dimension domain")
		}
	}
	for _, a := range f.AtomicPlacements() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !bounded(a.Y - lines[a.Line].Top) {
			return illegal("local atomic geometry outside dimension domain")
		}
	}
	return nil
}

func bounded(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) < dimen.Fil.Points()
}
func illegal(s string) error {
	return fmt.Errorf("%w: flow experiment: %s", contracts.ErrIllegalArguments, s)
}
