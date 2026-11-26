package paragraph_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

func mustPrefixes(t *testing.T, trial *paragraph.Trial, height float64) *paragraph.PrefixFit {
	t.Helper()
	f, err := trial.FitPrefixes(height)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func selectPrefix(t *testing.T, fit *paragraph.PrefixFit, lines int) *paragraph.FittedPrefix {
	t.Helper()
	p, err := fit.Select(lines)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPrefixFitEveryCutAndChangedWidth(t *testing.T) {
	const source = "aa bb cc dd ee ff gg hh ii jj kk ll"
	calls := new(fitServiceCalls)
	p, err := paragraph.Prepare(fitTextInput(source), khipu.PrepareOptions{}, flowConfig(calls))
	if err != nil {
		t.Fatal(err)
	}
	a := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)})
	fit := mustPrefixes(t, a, 1000)
	choices := fit.Prefixes()
	if len(choices) != 5 {
		t.Fatalf("want four lines and zero, got %+v", choices)
	}
	before := calls.snapshot()
	// Every legal cut is independently selectable, including shorter prefixes.
	for i, info := range choices {
		if info.Lines != 4-i || info.Height != float64(4-i)*10 {
			t.Fatalf("order/height: %+v", info)
		}
		selected := selectPrefix(t, fit, info.Lines)
		if selected.Info() != info || selected.Fragment().Box().Extent != info.Height {
			t.Fatal("selection metadata")
		}
		if info.Status == paragraph.FitComplete {
			if selected.Checkpoint() != nil || info.To.Offset != uint64(len(source)) {
				t.Fatal("completion")
			}
			continue
		}
		if selected.Checkpoint() == nil {
			t.Fatal("missing continuation authority")
		}
		if info.Lines == 0 && (info.From != info.To || info.Status != paragraph.FitNoProgress) {
			t.Fatal("empty placement advances")
		}
		if info.Lines > 0 {
			if c, ok := info.From.Compare(info.To); !ok || c >= 0 {
				t.Fatal("logical progress")
			}
		}
		prefix := selected.Fragment()
		want := prefix.Lines()
		r := paragraph.TrialRequest{Layout: a.Inputs(), Prefix: selected.Checkpoint()}
		r.Layout.Params.Constraints.Width = 20 * dimen.BP
		b := mustTrial(t, p, r)
		remaining := mustPrefixes(t, b, 1000)
		last := selectPrefix(t, remaining, remaining.Prefixes()[0].Lines)
		if last.Info().Status != paragraph.FitComplete || last.Info().From != info.To || last.Info().To.Offset != uint64(len(source)) {
			t.Fatal("suffix coordinates")
		}
		if last.Fragment().Range().Start != info.Lines {
			t.Fatal("reoffered accepted prefix")
		}
		var end uint64
		for _, fragment := range []*layout.Fragment{prefix, last.Fragment()} {
			for _, line := range fragment.Lines() {
				if line.Horizontal.Source.Start != end {
					t.Fatal("source gap/overlap", line.Horizontal.Source, end)
				}
				end = line.Horizontal.Source.End
			}
		}
		if end != uint64(len(source)) || !reflect.DeepEqual(prefix.Lines(), want) {
			t.Fatal("changed accepted prefix")
		}
		// An independently retained selected checkpoint remains retryable.
		r.Layout.Params.Constraints.Width = 30 * dimen.BP
		mustPrefixes(t, mustTrial(t, p, r), 1000)
	}
	noFlowShaping(t, before, calls.snapshot())
	// Selection itself is geometry-only and must not ask any service at all.
	calls.sealed.Store(true)
	for _, info := range choices {
		selectPrefix(t, fit, info.Lines)
	}
	choices[0].Lines = -99
	if fit.Prefixes()[0].Lines != 4 {
		t.Fatal("mutable descriptors")
	}
}

func TestPrefixFitMultipleResumesAndCompletion(t *testing.T) {
	p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff gg hh ii jj"), khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	r := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)}
	var end paragraph.LogicalPosition
	var last *paragraph.Trial
	totalLines := 0
	for step := 0; step < 12; step++ {
		r.Layout.Params.Constraints.Width = dimen.DU(20+step%3*5) * dimen.BP
		last = mustTrial(t, p, r)
		fit := mustPrefixes(t, last, 1000)
		selected := selectPrefix(t, fit, 1)
		if step > 0 && selected.Info().From != end {
			t.Fatal("resume moved start")
		}
		end = selected.Info().To
		totalLines++
		if selected.Fragment().Range().Start != totalLines-1 {
			t.Fatal("suffix local line count")
		}
		if selected.Info().Status == paragraph.FitComplete {
			break
		}
		r.Prefix = selected.Checkpoint()
	}
	if end.Offset != 29 {
		t.Fatalf("incomplete %v", end)
	}
	// A full low-level checkpoint is also a valid, already-complete request.
	r.Prefix = mustCheckpoint(t, last, len(last.Plan().Lines()))
	fit := mustPrefixes(t, mustTrial(t, p, r), 0)
	if got := fit.Prefixes(); len(got) != 1 || got[0].Status != paragraph.FitComplete || got[0].From != got[0].To {
		t.Fatal(got)
	}
	if selected := selectPrefix(t, fit, 0); selected.Checkpoint() != nil || len(selected.Fragment().Lines()) != 0 {
		t.Fatal("already complete")
	}
}

func TestPrefixFitHeightZeroEmptyAndOverfull(t *testing.T) {
	p, err := paragraph.Prepare(fitTextInput("aa\naa\naa"), khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	r := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)}
	a := mustTrial(t, p, r)
	for _, tc := range []struct {
		height float64
		max    int
	}{{0, 0}, {math.Nextafter(10, 0), 0}, {10, 1}, {20, 2}, {30, 3}} {
		fit := mustPrefixes(t, a, tc.height)
		if got := fit.Prefixes(); len(got) != tc.max+1 || got[0].Lines != tc.max {
			t.Fatal(tc, got)
		}
		for _, info := range fit.Prefixes() {
			if selectPrefix(t, fit, info.Lines).Fragment().Box().Extent > tc.height {
				t.Fatal("overflow")
			}
		}
		if bad, err := fit.Select(tc.max + 1); bad != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("non-fitting selection", bad, err)
		}
	}
	r.Layout.Params.LineLayout.Default = length(0)
	fit := mustPrefixes(t, mustTrial(t, p, r), 0)
	if len(fit.Prefixes()) != 4 || fit.Prefixes()[0].Status != paragraph.FitComplete {
		t.Fatal("zero-height lines lost")
	}
	for _, info := range fit.Prefixes() {
		if info.Lines > 0 {
			if c, ok := info.From.Compare(info.To); !ok || c >= 0 {
				t.Fatal("zero-height no progression")
			}
		}
	}
	empty, err := paragraph.Prepare(fitTextInput(""), khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	fit = mustPrefixes(t, mustTrial(t, empty, r), 0)
	zero := selectPrefix(t, fit, 0)
	if len(fit.Prefixes()) != 1 || zero.Info().Status != paragraph.FitComplete || zero.Checkpoint() != nil || zero.Fragment().Box() != (layout.Box{}) {
		t.Fatal("empty acquired material")
	}
	r.Layout.Params.Constraints.Width = 0
	r.Layout.Params.LineLayout.Default = factor(1)
	fit = mustPrefixes(t, mustTrial(t, p, r), 1000)
	if !fit.Prefixes()[0].Overfull || fit.Prefixes()[0].Width <= 0 {
		t.Fatal("overfull diagnostics hidden")
	}
}

func TestPrefixFitAtomicAssociationAndRebinding(t *testing.T) {
	input := fitTextInput("\uFFFC aa bb cc dd \uFFFC ee ff gg")
	input.Atomics = []contracts.AtomicAnchor{{ID: 7}, {ID: 9, ByteOffset: 16}}
	p, err := paragraph.Prepare(input, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	r := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)}
	r.Layout.Params.Constraints.Width = 25 * dimen.BP
	r.Layout.Atomics = []paragraph.AtomicBox{atomicBox(9, 8, 10), atomicBox(7, 8, 10)}
	fit := mustPrefixes(t, mustTrial(t, p, r), 1000)
	first := selectPrefix(t, fit, 1)
	want := first.Atomics()
	if len(want) != 1 || want[0].Box.ID != 7 || want[0].Placement != first.Fragment().AtomicPlacements()[0] {
		t.Fatal("prefix atomic pairing", want)
	}
	r.Prefix = first.Checkpoint()
	r.Layout.Atomics[0].BorderWidth = 13 * dimen.BP
	r.Layout.Atomics[0].BorderHeight = 30 * dimen.BP
	r.Layout.Params.Constraints.Width = 20 * dimen.BP
	suffix := mustPrefixes(t, mustTrial(t, p, r), 1000)
	last := selectPrefix(t, suffix, suffix.Prefixes()[0].Lines)
	got := last.Atomics()
	if len(got) != 1 || got[0].Box.ID != 9 || got[0].Box != r.Layout.Atomics[0] || got[0].Placement != last.Fragment().AtomicPlacements()[0] {
		t.Fatal("suffix atomic pairing", got)
	}
	got[0].Box.BorderHeight = 99
	if !reflect.DeepEqual(first.Atomics(), want) || last.Atomics()[0].Box.BorderHeight != 30*dimen.BP {
		t.Fatal("mutable bindings")
	}
	r.Layout.Atomics[1].BorderHeight++
	bad, err := p.Trial(r)
	requireStage(t, bad, err, paragraph.StageBind, khipu.ErrIllegalArguments)
	if !reflect.DeepEqual(first.Atomics(), want) {
		t.Fatal("failed rebind changed prefix")
	}
	coverage := first.Coverage()
	if coverage[0].Kind != paragraph.AtomicCoverage || coverage[0].Atomic != 7 || coverage[0].Source != (khipu.ByteRange{End: 3}) {
		t.Fatal("atomic source mapping", coverage)
	}
}

func TestPrefixFitBidiCoverageAndLogicalCoordinates(t *testing.T) {
	input := fitTextInput("ab אב גד ef הו זח ij")
	input.Direction = bidi.RightToLeft
	p, err := paragraph.Prepare(input, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	r := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)}
	fit := mustPrefixes(t, mustTrial(t, p, r), 1000)
	first := selectPrefix(t, fit, 1)
	coverage := first.Coverage()
	if len(coverage) == 0 {
		t.Fatal("no coverage")
	}
	var previous uint64
	for i, c := range coverage {
		if c.Line != 0 || c.Source.Start < c.Source.End && c.Source.End > first.Info().To.Offset || i > 0 && c.Source.Start < previous {
			t.Fatal("logical coverage", c)
		}
		previous = c.Source.Start
	}
	coverage[0].Source.End = 999
	if first.Coverage()[0].Source.End == 999 {
		t.Fatal("mutable coverage")
	}
	r.Prefix = first.Checkpoint()
	r.Layout.Params.Constraints.Width = 25 * dimen.BP
	suffix := mustPrefixes(t, mustTrial(t, p, r), 1000)
	if suffix.Prefixes()[0].From != first.Info().To || suffix.Prefixes()[0].To.Offset != input.Text.Raw().Len() {
		t.Fatal("bidi source conservation")
	}
	other, err := paragraph.Prepare(input, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	bad, err := other.Trial(r)
	requireStage(t, bad, err, paragraph.StageBind, khipu.ErrIllegalArguments)
	r.Prefix = nil
	otherFit := mustPrefixes(t, mustTrial(t, other, r), 1000)
	if _, ok := first.Info().To.Compare(otherFit.Prefixes()[0].To); ok {
		t.Fatal("foreign positions compared")
	}
}

func TestPrefixFitErrorsPreserveEarlierOutput(t *testing.T) {
	for _, a := range []*paragraph.Trial{nil, {}} {
		f, err := a.FitPrefixes(10)
		requireStage(t, f, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	for _, f := range []*paragraph.PrefixFit{nil, {}} {
		p, err := f.Select(0)
		requireStage(t, p, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	calls := new(fitServiceCalls)
	p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff"), khipu.PrepareOptions{}, flowConfig(calls))
	if err != nil {
		t.Fatal(err)
	}
	a := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)})
	fit := mustPrefixes(t, a, 1000)
	first := selectPrefix(t, fit, 1)
	want := first.Fragment().Lines()
	for _, h := range []float64{-1, math.NaN(), math.Inf(1), dimen.Fil.Points()} {
		f, err := a.FitPrefixes(h)
		requireStage(t, f, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	if bad, err := fit.Select(-1); bad != nil || !errors.Is(err, khipu.ErrIllegalArguments) {
		t.Fatal("negative count")
	}
	for _, mode := range []khipu.HyphenationMode{khipu.AutomaticHyphenation} {
		cfg := synthetic()
		cfg.Hyphenator = flowHyphenator{}
		p, err := paragraph.Prepare(fitTextInput("aa"), khipu.PrepareOptions{Hyphenation: mode}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		a := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)})
		f, err := a.FitPrefixes(1000)
		requireStage(t, f, err, paragraph.StageInput, khipu.ErrUnsupportedPolicy)
	}
	r := paragraph.TrialRequest{Layout: a.Inputs()}
	r.Layout.Params.LineLayout.Default = length(dimen.Fil / 2)
	r.Layout.Params.Constraints.Width = 1 * dimen.BP
	bad := mustTrial(t, p, r)
	f, err := bad.FitPrefixes(1000)
	requireStage(t, f, err, paragraph.StageAssemble, khipu.ErrIllegalArguments)
	if !reflect.DeepEqual(first.Fragment().Lines(), want) {
		t.Fatal("failed fit changed output")
	}
	selectPrefix(t, fit, 1)
}

type prefixFailureResolver struct{ failure *error }

func (r prefixFailureResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if *r.failure != nil {
		return d, *r.failure
	}
	return helpers.DefaultStyleResolver{}.ResolveStyle(s, d)
}

func TestPrefixFitAssemblyFailureCauseAndRetry(t *testing.T) {
	var injected error
	cfg := synthetic()
	cfg.Streaming.Resolver = prefixFailureResolver{&injected}
	p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff"), khipu.PrepareOptions{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	a := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)})
	fit := mustPrefixes(t, a, 1000)
	earlier := selectPrefix(t, fit, 1)
	want := earlier.Fragment().Lines()
	injected = errors.New("injected assembly service failure")
	bad, err := a.FitPrefixes(1000)
	requireStage(t, bad, err, paragraph.StageAssemble, injected)
	// Selection of an already assembled choice never reopens the resolver.
	selectPrefix(t, fit, 1)
	injected = nil
	mustPrefixes(t, a, 1000)
	if !reflect.DeepEqual(earlier.Fragment().Lines(), want) {
		t.Fatal("late failure changed retained selection")
	}
}
