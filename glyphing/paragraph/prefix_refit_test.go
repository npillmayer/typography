package paragraph_test

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

func refit(t *testing.T, f *paragraph.PrefixFit, lines int, height float64) *paragraph.PrefixFit {
	t.Helper()
	next, err := f.Refit(lines, height)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestPrefixRefitIndependentWidthOracles(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		calls := new(fitServiceCalls)
		p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff gg hh ii jj kk ll"), khipu.PrepareOptions{}, flowConfig(calls))
		if err != nil {
			t.Fatal(err)
		}
		request := flowRequest(breaker)
		initial := mustTrial(t, p, paragraph.TrialRequest{Layout: request})
		fit := mustPrefixes(t, initial, 1000)
		original := fit.Remainder()
		calls.sealed.Store(true)
		short := refit(t, fit, 0, 10)
		first := selectPrefix(t, short, 1)
		tail := refit(t, short, 1, 0)
		sameFragment(t, original, fit.Remainder())
		calls.sealed.Store(false)
		before := calls.snapshot()
		for _, tc := range []struct {
			width dimen.DU
			ends  []uint64
		}{
			{20, []uint64{12, 15, 18, 21, 24, 27, 30, 33, 35}},
			{30, []uint64{15, 21, 27, 33, 35}},
			{55, []uint64{21, 33, 35}},
		} {
			// A zero-line selection after refitting certifies the entire consumed prefix.
			cp := selectPrefix(t, tail, 0).Checkpoint()
			request = initial.Inputs()
			request.Params.Constraints.Width = tc.width * dimen.BP
			request.Params.Breaker = paragraph.FirstFit
			trial := mustTrial(t, p, paragraph.TrialRequest{Layout: request, Prefix: cp})
			choices := mustPrefixes(t, trial, 1000)
			last := selectPrefix(t, choices, choices.Prefixes()[0].Lines)
			if last.Checkpoint() != nil || !slices.Equal(flowEnds(last.Fragment()), tc.ends) {
				t.Fatal(tc, last.Info(), flowEnds(last.Fragment()))
			}
			firstLine, _ := last.Fragment().Frozen().Line(0)
			if !reflect.DeepEqual(firstLine, first.Fragment().Lines()[0].Horizontal) {
				t.Fatal("accepted prefix changed")
			}
			noFlowShaping(t, before, calls.snapshot())
		}
	}
}

func TestPrefixRefitMixedWidthsAndZeroServiceReuse(t *testing.T) {
	calls := new(fitServiceCalls)
	p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff gg hh ii jj kk ll mm nn oo pp"), khipu.PrepareOptions{}, flowConfig(calls))
	if err != nil {
		t.Fatal(err)
	}
	request := flowRequest(paragraph.FirstFit)
	trial := mustTrial(t, p, paragraph.TrialRequest{Layout: request})
	fit := mustPrefixes(t, trial, 10)
	fragments := []*layout.Fragment{selectPrefix(t, fit, 1).Fragment()}
	fit = refit(t, fit, 1, 0)
	widths := []dimen.DU{40}
	for _, width := range []dimen.DU{30, 30, 20, 55, 55} {
		height := 10.0
		if len(widths) == 5 {
			height = 1000
		}
		before := calls.snapshot()
		if width == widths[len(widths)-1] {
			calls.sealed.Store(true)
			fit = refit(t, fit, 0, height)
		} else {
			cp := selectPrefix(t, fit, 0).Checkpoint()
			request.Params.Constraints.Width = width * dimen.BP
			trial = mustTrial(t, p, paragraph.TrialRequest{Layout: request, Prefix: cp})
			fit = mustPrefixes(t, trial, height)
		}
		selected := selectPrefix(t, fit, fit.Prefixes()[0].Lines)
		fragments = append(fragments, selected.Fragment())
		next := refit(t, fit, selected.Info().Lines, 0)
		if calls.sealed.Load() && calls.snapshot() != before {
			t.Fatal("equal-width service call")
		}
		calls.sealed.Store(false)
		noFlowShaping(t, before, calls.snapshot())
		for _, line := range selected.Fragment().Lines() {
			widths = append(widths, line.Horizontal.TargetWidth/dimen.BP)
		}
		fit = next
	}
	var end uint64
	fills := 0
	for _, f := range fragments {
		for _, l := range f.Lines() {
			if l.Horizontal.Source.Start != end {
				t.Fatal("source gap or overlap")
			}
			end = l.Horizontal.Source.End
			for k := l.Horizontal.FirstKnot; k < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; k++ {
				knot, _ := f.Frozen().Knot(k)
				if knot.Material == khipu.ParagraphFillMaterial {
					fills++
				}
			}
		}
	}
	if end != fragments[0].Frozen().SourceLen() || fills != 1 || fit.Prefixes()[0].Status != paragraph.FitComplete {
		t.Fatal("incomplete source/fill", end, fills)
	}
	last := fragments[len(fragments)-1]
	for i, want := range []dimen.DU{40, 30, 30, 20, 55} {
		line, _ := last.Frozen().Line(i)
		if line.TargetWidth != want*dimen.BP {
			t.Fatal("lost width history", i, line)
		}
	}
}

func TestPrefixRefitBidiAtomicsAndExactHeights(t *testing.T) {
	for _, direction := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		calls := new(fitServiceCalls)
		input := fitTextInput("aa\n\uFFFC\uFFFC aa bb cc dd ee\n\uFFFC")
		input.Direction = direction
		input.Atomics = []contracts.AtomicAnchor{{ID: 7, ByteOffset: 3}, {ID: 2, ByteOffset: 6}, {ID: 9, ByteOffset: 25}}
		p, err := paragraph.Prepare(input, khipu.PrepareOptions{}, flowConfig(calls))
		if err != nil {
			t.Fatal(err)
		}
		request := flowRequest(paragraph.FirstFit)
		request.Atomics = []paragraph.AtomicBox{
			{ID: 7, Margin: paragraph.AtomicMargins{Right: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge},
			{ID: 2, BorderWidth: 4 * dimen.BP, BorderHeight: 30 * dimen.BP, Margin: paragraph.AtomicMargins{Left: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge},
			{ID: 9, Alignment: paragraph.AlignBottomMarginEdge},
		}
		initial := mustTrial(t, p, paragraph.TrialRequest{Layout: request})
		fit := mustPrefixes(t, initial, 10)
		first := selectPrefix(t, fit, 1)
		request = initial.Inputs()
		request.Params.Constraints.Width = 30 * dimen.BP
		trial := mustTrial(t, p, paragraph.TrialRequest{Layout: request, Prefix: first.Checkpoint()})
		fit = mustPrefixes(t, trial, 32)
		calls.sealed.Store(true)
		tooShort := refit(t, fit, 0, math.Nextafter(32, 0))
		if tooShort.Prefixes()[0].Status != paragraph.FitNoProgress {
			t.Fatal("oversized atomic line fit")
		}
		selected := selectPrefix(t, fit, 1)
		if selected.Fragment().Lines()[0].Height != 32 || len(selected.Atomics()) != 2 {
			t.Fatal("atomic geometry lost")
		}
		tail := refit(t, fit, 1, 1000)
		done := selectPrefix(t, tail, tail.Prefixes()[0].Lines)
		if done.Checkpoint() != nil {
			t.Fatal("did not drain")
		}
	}
}

type refitFailResolver struct {
	fitCountedResolver
	failed *bool
	cause  error
}

func (r refitFailResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if *r.failed {
		return d, r.cause
	}
	return r.fitCountedResolver.ResolveStyle(s, d)
}
func TestPrefixRefitSurvivesLateFailure(t *testing.T) {
	calls := new(fitServiceCalls)
	config := flowConfig(calls)
	failed := false
	cause := errors.New("late metrics")
	config.Streaming.Resolver = refitFailResolver{fitCountedResolver{calls}, &failed, cause}
	p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff gg hh"), khipu.PrepareOptions{}, config)
	if err != nil {
		t.Fatal(err)
	}
	request := flowRequest(paragraph.FirstFit)
	initial := mustTrial(t, p, paragraph.TrialRequest{Layout: request})
	fit := mustPrefixes(t, initial, 10)
	selected := selectPrefix(t, fit, 1)
	request.Params.Constraints.Width = 30 * dimen.BP
	changed := mustTrial(t, p, paragraph.TrialRequest{Layout: request, Prefix: selected.Checkpoint()})
	failed = true
	if out, err := changed.FitPrefixes(1000); out != nil || !errors.Is(err, cause) {
		t.Fatal("late failure", err)
	}
	calls.sealed.Store(true)
	tail := refit(t, fit, 1, 1000)
	selectPrefix(t, tail, tail.Prefixes()[0].Lines)
	calls.sealed.Store(false)
	failed = false
	before := calls.snapshot()
	recovered := mustPrefixes(t, changed, 1000)
	if selectPrefix(t, recovered, recovered.Prefixes()[0].Lines).Checkpoint() != nil {
		t.Fatal("retry incomplete")
	}
	noFlowShaping(t, before, calls.snapshot())
}

func TestPrefixRefitValidationAndEmpty(t *testing.T) {
	for _, source := range []string{"aa bb cc dd", ""} {
		calls := new(fitServiceCalls)
		p, err := paragraph.Prepare(fitTextInput(source), khipu.PrepareOptions{}, flowConfig(calls))
		if err != nil {
			t.Fatal(err)
		}
		trial := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)})
		fit := mustPrefixes(t, trial, 0)
		calls.sealed.Store(true)
		for _, height := range []float64{-1, math.NaN(), math.Inf(1), dimen.Fil.Points()} {
			if got, err := fit.Refit(0, height); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
				t.Fatal(got, err)
			}
		}
		for _, n := range []int{-1, 1, 100} {
			if got, err := fit.Refit(n, 1000); got != nil || err == nil {
				t.Fatal("ineligible count", n, got, err)
			}
		}
		wide := refit(t, fit, 0, 1000)
		done := selectPrefix(t, wide, wide.Prefixes()[0].Lines)
		empty := refit(t, wide, done.Info().Lines, 0)
		if len(empty.Remainder().Lines()) != 0 || empty.Prefixes()[0].Status != paragraph.FitComplete {
			t.Fatal("not completed")
		}
		again := refit(t, empty, 0, 0)
		if selectPrefix(t, again, 0).Checkpoint() != nil {
			t.Fatal("empty checkpoint")
		}
	}
	for _, f := range []*paragraph.PrefixFit{nil, {}} {
		if f.Remainder() != nil {
			t.Fatal("zero remainder")
		}
		if got, err := f.Refit(0, 10); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
}

func TestPrefixRefitGeometryOnlyCallBoundary(t *testing.T) {
	w := newAuditWorld(t)
	p, err := w.load(paragraphPath)
	if err != nil {
		t.Fatal(err)
	}
	fn := p.functions["(*"+paragraphPath+".PrefixFit).Refit"]
	if fn == nil {
		t.Fatal("missing refit")
	}
	audit := &callAudit{world: w, seen: make(map[string]bool)}
	audit.inspect(p, fn.Body)
	if audit.err != nil || len(audit.issues) != 0 {
		t.Fatal(audit.issues, audit.err)
	}
}

func TestPrefixRefitManualReplacementAndCheckpoint(t *testing.T) {
	policy := &prefixManualPolicy{pre: "-", post: "Q"}
	p, calls := manualPrefixes(t, fitTextInput(manualPrefixText), policy)
	trial := mustTrial(t, p, manualPrefixRequest(30))
	choices := mustPrefixes(t, trial, 1000)
	// Accept the first two lines, including the first authored discretionary.
	first := selectPrefix(t, choices, 2)
	if first.Info().Status != paragraph.FitContinues {
		t.Fatal("fixture must continue")
	}
	before, discoveries := calls.snapshot(), policy.calls
	calls.sealed.Store(true)
	next := refit(t, choices, 2, 10)
	selected := selectPrefix(t, next, next.Prefixes()[0].Lines)
	if selected.Info().From != first.Info().To || selected.Info().Lines != 1 {
		t.Fatal("lost manual boundary")
	}
	rest := refit(t, next, selected.Info().Lines, 1000)
	sameWidth := selectPrefix(t, rest, rest.Prefixes()[0].Lines)
	if sameWidth.Checkpoint() != nil || policy.calls != discoveries || calls.snapshot() != before {
		t.Fatal("geometry reuse queried services")
	}
	calls.sealed.Store(false)
	// This checkpoint contains both the inherited and newly selected lines.
	request := manualPrefixRequest(20)
	request.Prefix = selected.Checkpoint()
	resumed := mustPrefixes(t, mustTrial(t, p, request), 1000)
	done := selectPrefix(t, resumed, resumed.Prefixes()[0].Lines)
	if done.Info().From != selected.Info().To || done.Checkpoint() != nil {
		t.Fatal("manual resumption repeated/lost source")
	}
}

func TestPrefixRefitPreservesDistinctKPPrefix(t *testing.T) {
	calls := new(fitServiceCalls)
	config := flowConfig(calls)
	config.Params.SpaceStretchPercent, config.Params.SpaceShrinkPercent = 50, 33.3
	p, err := paragraph.Prepare(fitTextInput("aaaa a a aa aa aaaa aa aaa a a aaa"), khipu.PrepareOptions{}, config)
	if err != nil {
		t.Fatal(err)
	}
	trial := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.KnuthPlass)})
	fit := mustPrefixes(t, trial, 20)
	if !slices.Equal(flowEnds(fit.Remainder()), []uint64{9, 20, 29, 34}) {
		t.Fatal("KP oracle", flowEnds(fit.Remainder()))
	}
	first := selectPrefix(t, fit, 2)
	tail := refit(t, fit, 2, 0)
	request := trial.Inputs()
	request.Params.Breaker = paragraph.FirstFit
	request.Params.Constraints.Width = 30 * dimen.BP
	continued := mustTrial(t, p, paragraph.TrialRequest{Layout: request, Prefix: selectPrefix(t, tail, 0).Checkpoint()})
	choices := mustPrefixes(t, continued, 1000)
	last := selectPrefix(t, choices, choices.Prefixes()[0].Lines)
	for _, line := range first.Fragment().Lines() {
		other, _ := last.Fragment().Frozen().Line(line.OriginalIndex)
		if !reflect.DeepEqual(line.Horizontal, other) {
			t.Fatal("KP line changed")
		}
		for k := line.Horizontal.FirstKnot; k < line.Horizontal.FirstKnot+line.Horizontal.KnotCount; k++ {
			oldKnot, _ := first.Fragment().Frozen().Knot(k)
			newKnot, _ := last.Fragment().Frozen().Knot(k)
			oldText, _ := first.Fragment().Frozen().CopyText(k)
			newText, _ := last.Fragment().Frozen().CopyText(k)
			if !reflect.DeepEqual(oldKnot, newKnot) || !reflect.DeepEqual(oldText, newText) {
				t.Fatal("accepted KP glyph changed", k)
			}
		}
	}
}

func TestPrefixFitCompletedSuppressedEmptyCheckpoint(t *testing.T) {
	p, err := paragraph.Prepare(fitTextInput(""), khipu.PrepareOptions{}, flowConfig(new(fitServiceCalls)))
	if err != nil {
		t.Fatal(err)
	}
	request := flowRequest(paragraph.FirstFit)
	trial := mustTrial(t, p, paragraph.TrialRequest{Layout: request})
	checkpoint, err := trial.Checkpoint(len(trial.Plan().Lines()))
	if err != nil {
		t.Fatal(err)
	}
	resumed := mustTrial(t, p, paragraph.TrialRequest{Layout: request, Prefix: checkpoint})
	choices := mustPrefixes(t, resumed, 0)
	selected := selectPrefix(t, choices, 0)
	if selected.Info().Status != paragraph.FitComplete || selected.Checkpoint() != nil || len(selected.Fragment().Lines()) != 0 {
		t.Fatal("completed empty paragraph", selected.Info())
	}
}
