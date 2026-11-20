package paragraph_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

func mustCheckpoint(t *testing.T, a *paragraph.Trial, n int) *paragraph.TrialCheckpoint {
	t.Helper()
	c, err := a.Checkpoint(n)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTrialCheckpointAtomicCompatibility(t *testing.T) {
	input := paragraph.Input{Text: styled.TextFromString("\uFFFC aa bb cc dd \uFFFC ee ff gg"),
		Atomics: []contracts.AtomicAnchor{{ID: 7}, {ID: 9, ByteOffset: 16}}}
	p, err := paragraph.Prepare(input, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	r := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)}
	r.Layout.Params.Constraints.Width = 25 * dimen.BP
	r.Layout.Atomics = []paragraph.AtomicBox{atomicBox(9, 8, 10), atomicBox(7, 8, 10)}
	a := mustTrial(t, p, r)
	before := mustAssembleTrial(t, a)
	if len(before.VList.Lines()) < 3 || before.VList.AtomicPlacements()[1].Line < 1 {
		t.Fatal("test requires a prefix atomic and a suffix atomic")
	}
	checkpoint := mustCheckpoint(t, a, 1)
	for _, change := range []func(*paragraph.AtomicBox){
		func(b *paragraph.AtomicBox) { b.BorderHeight++ },
		func(b *paragraph.AtomicBox) { b.Margin.Top++ },
		func(b *paragraph.AtomicBox) { b.Margin.Bottom++ },
		func(b *paragraph.AtomicBox) { b.Margin.Left++; b.Margin.Right-- },
		func(b *paragraph.AtomicBox) { b.BorderWidth++ },
	} {
		request := paragraph.TrialRequest{Layout: a.Inputs(), Prefix: checkpoint}
		for i := range request.Layout.Atomics {
			if request.Layout.Atomics[i].ID == 7 {
				change(&request.Layout.Atomics[i])
			}
		}
		bad, err := p.Trial(request)
		requireStage(t, bad, err, paragraph.StageBind, khipu.ErrIllegalArguments)
	}
	r.Layout = a.Inputs()
	r.Prefix = checkpoint
	for i := range r.Layout.Atomics {
		if r.Layout.Atomics[i].ID == 9 {
			r.Layout.Atomics[i].BorderWidth = 13 * dimen.BP
			r.Layout.Atomics[i].BorderHeight = 30 * dimen.BP
		}
	}
	r.Layout.Params.Constraints.Width = 20 * dimen.BP
	b := mustTrial(t, p, r)
	after := mustAssembleTrial(t, b)
	if before.VList.Lines()[0] != after.VList.Lines()[0] ||
		before.VList.AtomicPlacements()[0] != after.VList.AtomicPlacements()[0] ||
		before.BreakPositions[0] != after.BreakPositions[0] ||
		a.Plan().Lines()[0] != b.Plan().Lines()[0] {
		t.Fatal("changed suffix geometry changed accepted prefix")
	}
	// Caller mutation after search must not affect this candidate's realization.
	want := trialOutput(after)
	r.Layout.Atomics[1].BorderHeight = dimen.Fil - 1
	if !reflect.DeepEqual(want, trialOutput(mustAssembleTrial(t, b))) {
		t.Fatal("atomic request storage retained")
	}
	r.Layout = a.Inputs()
	r.Layout.Params.LineLayout.Default = factor(2)
	bad, err := p.Trial(r)
	requireStage(t, bad, err, paragraph.StageBind, khipu.ErrIllegalArguments)
	r.Prefix = mustCheckpoint(t, a, 0)
	r.Layout.Atomics[0].BorderHeight += dimen.BP
	mustAssembleTrial(t, mustTrial(t, p, r)) // An empty prefix locks no geometry.
	other, err := paragraph.Prepare(input, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	bad, err = other.Trial(r)
	requireStage(t, bad, err, paragraph.StageBind, khipu.ErrIllegalArguments)
	for _, n := range []int{-1, len(a.Plan().Lines()) + 1} {
		cp, err := a.Checkpoint(n)
		requireStage(t, cp, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	r.Layout = a.Inputs()
	r.Layout.Params.Breaker = paragraph.KnuthPlass
	bad, err = p.Trial(r)
	requireStage(t, bad, err, paragraph.StageBreak, khipu.ErrUnsupportedPolicy)
}

func TestTrialCheckpointWidthPolicyHistoryAndReuse(t *testing.T) {
	calls := new(fitServiceCalls)
	p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff gg hh ii jj kk ll mm nn"), khipu.PrepareOptions{}, flowConfig(calls))
	if err != nil {
		t.Fatal(err)
	}
	a := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.KnuthPlass)})
	original := a.Plan().Lines()
	if len(original) < 3 {
		t.Fatal("short fixture")
	}
	before := calls.snapshot()
	for _, step := range []struct{ accepted, width, skip int }{{1, 25, 1}, {2, 30, 2}} {
		prefix := slices.Clone(a.Plan().Lines()[:step.accepted])
		policies := make([]khipu.PolicyRecord, step.accepted)
		for i := range policies {
			policies[i], _ = a.Plan().PolicyForLine(i)
		}
		r := paragraph.TrialRequest{Layout: a.Inputs(), Prefix: mustCheckpoint(t, a, step.accepted),
			FirstFit: &firstfit.Parameters{MinHyphenGain: dimen.DU(step.skip) * dimen.BP}}
		r.Layout.Params.Breaker = paragraph.FirstFit
		r.Layout.Horizontal.LeftSkip = fixedSkip(step.skip)
		widthCalls := 0
		r.Shape = trialShape(func(i int32) dimen.DU {
			if int(i) <= step.accepted {
				t.Fatal("queried locked prefix width", i)
			}
			widthCalls++
			return dimen.DU(step.width) * dimen.BP
		})
		a = mustTrial(t, p, r)
		if widthCalls == 0 || !reflect.DeepEqual(prefix, a.Plan().Lines()[:step.accepted]) {
			t.Fatal("did not preserve prefix/search suffix")
		}
		for i, want := range policies {
			got, ok := a.Plan().PolicyForLine(i)
			if !ok || got != want {
				t.Fatal("changed prefix policy")
			}
		}
	}
	if calls.snapshot() != before {
		t.Fatal("continuation searched with services", before, calls.snapshot())
	}
	// Accepting the entire paragraph queries neither widths nor services.
	full := paragraph.TrialRequest{Layout: a.Inputs(), Prefix: mustCheckpoint(t, a, len(a.Plan().Lines())),
		Shape: trialShape(func(int32) dimen.DU { t.Fatal("full prefix queried width"); return 0 })}
	completed := mustTrial(t, p, full)
	if !reflect.DeepEqual(completed.Plan().Lines(), a.Plan().Lines()) || calls.snapshot() != before {
		t.Fatal("full prefix changed")
	}
	mustAssembleTrial(t, completed)
	uniform := mustTrial(t, p, paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)})
	result := mustAssembleTrial(t, uniform)
	fragment, err := paragraph.Fragment(result.VList)
	if err != nil {
		t.Fatal(err)
	}
	// Existing equal-width geometry operations remain usable on a trial result.
	calls.sealed.Store(true)
	key := paragraph.FragmentReuseKey{Width: 40 * dimen.BP, Source: 1, Styles: 1, Fonts: 1, LineHeight: 1, Atomics: 1, Policy: 1}
	_, err = paragraph.FitFragment(fragment, key, key, 10)
	if err != nil {
		t.Fatal(err)
	}
}
