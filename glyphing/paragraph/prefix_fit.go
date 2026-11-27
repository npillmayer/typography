package paragraph

import (
	"cmp"
	"math"
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/typography/layout"
)

// LogicalPosition is paragraph-relative inspection data, not continuation
// authority. Offset counts bytes in Input.Text (after any application-owned
// whitespace processing); NextKnot distinguishes zero-byte material boundaries.
// Incarnation qualifies the coordinates independently of width and line count.
// At a discretionary split, Offset is the original source split and NextKnot is
// past the original textbox. Pending replacement material stays in the checkpoint;
// these coordinates alone never describe or authorize a resumable suffix.
type LogicalPosition struct {
	Incarnation khipu.IncarnationID
	Offset      uint64
	NextKnot    int
}

// Compare orders positions only within the same nonzero preparation identity.
// Changing or comparing these values cannot manufacture a TrialCheckpoint.
func (p LogicalPosition) Compare(q LogicalPosition) (int, bool) {
	if p.Incarnation == (khipu.IncarnationID{}) || p.Incarnation != q.Incarnation {
		return 0, false
	}
	if c := cmp.Compare(p.Offset, q.Offset); c != 0 {
		return c, true
	}
	return cmp.Compare(p.NextKnot, q.NextKnot), true
}

// PrefixInfo describes one eligible suffix prefix. Lines is local to the
// unaccepted suffix, not the whole plan. Width/Height are fractional big points,
// including full CSS line boxes, and Overfull reports horizontal overflow
// against the captured line targets. Height fitting does not clip that overflow.
type PrefixInfo struct {
	Lines         int
	Status        FragmentFitStatus
	From, To      LogicalPosition
	Width, Height float64
	Overfull      bool
}

// PrefixFit retains a trial and its assembled suffix for repeated selection.
// It is immutable and has no callbacks or search workspace, but its Trial still
// borrows stable assembly services. Release it after choosing; a selected frozen
// fragment retains neither this object nor the trial. Zero is invalid.
type PrefixFit struct {
	trial     *Trial
	tail      *layout.Fragment
	choices   []PrefixInfo
	startLine int
	positions []LogicalPosition
}

// FitPrefixes freezes/assembles this trial once and enumerates every whole-line
// prefix of its unaccepted suffix fitting height. Choices are in descending
// logical progression, with an explicit zero-line FitNoProgress choice last.
// Empty/already-completed paragraphs have one zero-line FitComplete choice.
// A prefix inherited through TrialRequest.Prefix is never offered again.
//
// Height is finite, nonnegative and strictly below dimen.Fil in big points.
// Authoritative line bottoms determine eligibility, without an overflow epsilon;
// zero-height lines count as progress. NoHyphenation and ExplicitHyphenation are
// supported. There are no shaping, discovery or line-breaking calls here, and
// no context or page policy. Assembly may query the stable borrowed
// metrics/resolver services.
// Errors publish no partial choice set and preserve earlier output.
func (t *Trial) FitPrefixes(height float64) (*PrefixFit, error) {
	if t.Plan() == nil || !fitBounded(height) || height < 0 {
		return nil, &Error{StageInput, invalidInput("invalid trial or prefix height")}
	}
	if t.hyphenation != khipu.NoHyphenation && t.hyphenation != khipu.ExplicitHyphenation {
		return nil, &Error{StageInput, khipu.ErrUnsupportedPolicy}
	}
	result, err := t.Assemble()
	if err != nil {
		return nil, err
	}
	whole, err := Fragment(result.VList)
	if err != nil {
		return nil, &Error{StageAssemble, err}
	}
	if err := validateFitLocal(whole); err != nil {
		return nil, &Error{StageAssemble, err}
	}
	decisions := t.plan.Lines()
	positions := make([]LogicalPosition, len(decisions)+1)
	positions[0].Incarnation = t.plan.Incarnation()
	for i, d := range decisions {
		positions[i+1] = LogicalPosition{t.plan.Incarnation(), result.BreakPositions[i], d.End.Knot + 1}
	}
	fit := &PrefixFit{trial: t, tail: whole, startLine: t.startLine, positions: positions}
	if t.startLine > 0 && whole.Range().End > 0 {
		_, fit.tail, err = splitPrefix(whole, t.startLine)
		if err != nil {
			return nil, &Error{StageAssemble, err}
		}
	}
	return fit.enumerate(height)
}

// Remainder returns the immutable unaccepted fragment used by this choice set.
// The fragment retains no trial, preparation or assembly callbacks.
func (f *PrefixFit) Remainder() *layout.Fragment {
	if f == nil {
		return nil
	}
	return f.tail
}

// Refit accepts an eligible local line count and enumerates the remaining tail
// at a new height using only retained geometry. Pass zero to change height
// without advancing. It preserves captured widths/skips, including mixed width
// histories; changed widths or atomic geometry require a checkpoint and a new
// Prepared.Trial instead. No binding, checkpointing, shaping, breaking, assembly
// or service calls occur. The original choice set stays reusable. The returned
// set shares the same trial and immutable position data, never its predecessor.
// Heights follow FitPrefixes; invalid counts/heights return no result.
func (f *PrefixFit) Refit(lines int, height float64) (*PrefixFit, error) {
	if f == nil || f.trial == nil || !fitBounded(height) || height < 0 {
		return nil, &Error{StageInput, invalidInput("invalid prefix refit")}
	}
	if slices.IndexFunc(f.choices, func(p PrefixInfo) bool { return p.Lines == lines }) < 0 {
		return nil, &Error{StageInput, invalidInput("prefix does not fit")}
	}
	_, tail, err := splitPrefix(f.tail, lines)
	if err != nil {
		return nil, &Error{StageAssemble, err}
	}
	next := &PrefixFit{trial: f.trial, tail: tail, startLine: f.startLine + lines, positions: f.positions}
	if len(f.tail.Lines()) == 0 && f.choices[0].Status == FitComplete {
		next.startLine = len(f.positions) - 1
	}
	return next.enumerate(height)
}

func (f *PrefixFit) enumerate(height float64) (*PrefixFit, error) {
	if err := validateFitLocal(f.tail); err != nil {
		return nil, &Error{StageAssemble, err}
	}
	from := f.positions[f.startLine]
	lines := f.tail.Lines()
	if len(lines) == 0 {
		// Suppressed-empty assembly retains a diagnostic frozen line, not a
		// visible line box. Preserve its complete logical consumption.
		f.choices = []PrefixInfo{{Status: FitComplete, From: from, To: f.positions[len(f.positions)-1]}}
		return f, nil
	}
	f.choices = append(f.choices, PrefixInfo{Status: FitNoProgress, From: from, To: from})
	width, overfull := 0.0, false
	for i, line := range lines {
		end := line.Top + line.Height
		if i == len(lines)-1 {
			end = math.Max(end, f.tail.Box().Extent)
		}
		if !fitBounded(end) {
			return nil, &Error{StageAssemble, fitInvalid("prefix end outside dimension domain")}
		}
		if end > height {
			break
		}
		width = math.Max(width, math.Max(line.Horizontal.Width.Points(), line.Horizontal.TargetWidth.Points()))
		overfull = overfull || line.Horizontal.Width > line.Horizontal.TargetWidth
		status := FitContinues
		if i == len(lines)-1 {
			status = FitComplete
		}
		f.choices = append(f.choices, PrefixInfo{Lines: i + 1, Status: status, From: from,
			To: f.positions[f.startLine+i+1], Width: width, Height: end, Overfull: overfull})
	}
	slices.Reverse(f.choices)
	return f, nil
}

// Prefixes returns detached descriptors, in farther-progress-first order.
// Editing them has no effect on selection, eligibility or continuation.
func (f *PrefixFit) Prefixes() []PrefixInfo {
	if f == nil {
		return nil
	}
	return slices.Clone(f.choices)
}

// FittedPrefix pairs exactly one immutable fragment with its checkpoint and
// accepted atomic bindings. Retain Fragment separately for rendering after
// releasing all working checkpoints. A complete choice has a nil checkpoint.
type FittedPrefix struct {
	info       PrefixInfo
	fragment   *layout.Fragment
	checkpoint *TrialCheckpoint
	atomics    []FittedAtomic
	coverage   []SourceCoverage
}

func (p *FittedPrefix) Info() PrefixInfo {
	if p == nil {
		return PrefixInfo{}
	}
	return p.info
}
func (p *FittedPrefix) Fragment() *layout.Fragment {
	if p == nil {
		return nil
	}
	return p.fragment
}
func (p *FittedPrefix) Checkpoint() *TrialCheckpoint {
	if p == nil {
		return nil
	}
	return p.checkpoint
}
func (p *FittedPrefix) Atomics() []FittedAtomic {
	if p == nil {
		return nil
	}
	return slices.Clone(p.atomics)
}
func (p *FittedPrefix) Coverage() []SourceCoverage {
	if p == nil {
		return nil
	}
	return slices.Clone(p.coverage)
}

// Select accepts an eligible local line count from Prefixes. Selection uses the
// set's private trial and geometry; callers cannot pair an unrelated fragment
// with a checkpoint. This performs no service calls and leaves all choices
// reusable. Invalid/non-fitting counts return no result. Resume via Prepared.Trial
// with the returned Checkpoint; only unconsumed atomic geometry may change.
func (f *PrefixFit) Select(lines int) (*FittedPrefix, error) {
	if f == nil || f.trial == nil {
		return nil, &Error{StageInput, invalidInput("invalid prefix fit")}
	}
	i := slices.IndexFunc(f.choices, func(p PrefixInfo) bool { return p.Lines == lines })
	if i < 0 {
		return nil, &Error{StageInput, invalidInput("prefix does not fit")}
	}
	out := &FittedPrefix{info: f.choices[i]}
	var err error
	out.fragment, _, err = splitPrefix(f.tail, lines)
	if err != nil {
		return nil, &Error{StageAssemble, err}
	}
	if err = validateFitLocal(out.fragment); err != nil {
		return nil, &Error{StageAssemble, err}
	}
	// The final fragment's authoritative envelope may be infinitesimally
	// taller than a recomputed line sum. Never publish an over-height selection.
	if out.fragment.Box().Extent > out.info.Height {
		return nil, &Error{StageAssemble, fitInvalid("selected prefix exceeds its declared height")}
	}
	if out.info.Status != FitComplete {
		out.checkpoint, err = f.trial.Checkpoint(f.startLine + lines)
		if err != nil {
			return nil, err
		}
	}
	for _, placement := range out.fragment.AtomicPlacements() {
		box, ok := f.trial.boxes[placement.ID]
		if !ok {
			return nil, &Error{StageAssemble, invalidInput("missing selected atomic binding")}
		}
		out.atomics = append(out.atomics, FittedAtomic{Box: box, Placement: placement})
	}
	out.coverage = prefixCoverage(out.fragment)
	return out, nil
}

// splitPrefix includes the two exterior cuts not accepted by SplitFragment.
func splitPrefix(f *layout.Fragment, n int) (*layout.Fragment, *layout.Fragment, error) {
	span := f.Range()
	if n < 0 || n > span.End-span.Start {
		return nil, nil, fitInvalid("prefix line count")
	}
	if n == 0 || n == span.End-span.Start {
		at := span.Start + n
		empty, err := layout.NewFragment(f.Frozen(), layout.LineRange{Start: at, End: at}, layout.Box{}, nil, nil, nil)
		if err != nil {
			return nil, nil, err
		}
		if n == 0 {
			return empty, f, nil
		}
		return f, empty, nil
	}
	return SplitFragment(f, n)
}
