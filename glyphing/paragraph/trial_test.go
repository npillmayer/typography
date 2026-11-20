package paragraph_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

type trialShape func(int32) dimen.DU

func (s trialShape) LineLength(i int32) dimen.DU { return s(i) }

func mustTrial(t *testing.T, p *paragraph.Prepared, r paragraph.TrialRequest) *paragraph.Trial {
	t.Helper()
	a, err := p.Trial(r)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func mustAssembleTrial(t *testing.T, a *paragraph.Trial) *paragraph.Result {
	t.Helper()
	r, err := a.Assemble()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func trialOutput(r *paragraph.Result) []any {
	out := []any{r.VList.Box(), r.VList.Lines(), r.VList.Materials(), r.VList.AtomicPlacements(), r.BidiRuns, r.BreakPositions}
	for i := 0; i < r.Frozen.LineCount(); i++ {
		line, _ := r.Frozen.Line(i)
		out = append(out, line)
	}
	for i := 0; i < r.Frozen.KnotCount(); i++ {
		knot, _ := r.Frozen.Knot(i)
		text, _ := r.Frozen.CopyText(i)
		out = append(out, knot, text)
	}
	return out
}

func TestTrialIsolationAndCapturedInputs(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		t.Run(string(breaker), func(t *testing.T) {
			calls := new(fitServiceCalls)
			p, err := paragraph.Prepare(fitTextInput("aa bb cc dd ee ff gg hh ii jj"), khipu.PrepareOptions{}, flowConfig(calls))
			if err != nil {
				t.Fatal(err)
			}
			before := calls.snapshot()
			r := paragraph.TrialRequest{Layout: flowRequest(breaker)}
			r.Layout.Params.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 0, End: 2, Height: factor(1.2)}}
			shapeCalls, sealed := 0, false
			r.Shape = trialShape(func(i int32) dimen.DU {
				if sealed || i < 1 {
					t.Fatal("shape queried outside search", i)
				}
				shapeCalls++
				return 35 * dimen.BP
			})
			if breaker == paragraph.FirstFit {
				r.FirstFit = &firstfit.Parameters{MinHyphenGain: dimen.BP}
			} else {
				r.KnuthPlass = knuthplass.NewDefaultParameters()
				r.KnuthPlass.LinePenalty = 17
			}
			a := mustTrial(t, p, r)
			sealed = true
			if shapeCalls == 0 || calls.snapshot() != before {
				t.Fatal("NoHyphenation trial called services or skipped shape", before, calls.snapshot())
			}
			plan, policy := a.Plan().Lines(), a.Plan().Policies()
			want := trialOutput(mustAssembleTrial(t, a))
			r.Layout.Params.LineLayout.Runs[0].Height = factor(8)
			r.Layout.Horizontal.LeftSkip = fixedSkip(10)
			if r.FirstFit != nil {
				r.FirstFit.MinHyphenGain = 100 * dimen.BP
			} else {
				r.KnuthPlass.LinePenalty = 999
			}
			copy := a.Inputs()
			copy.Params.LineLayout.Default = factor(99)
			copy.Params.LineLayout.Runs[0].End = 99
			copy.Horizontal.RightSkip = fixedSkip(90)
			b := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)}
			b.Layout.Params.Constraints.Width = 20 * dimen.BP
			mustAssembleTrial(t, mustTrial(t, p, b))
			b.Shape = trialShape(func(int32) dimen.DU { return -1 })
			failed, err := p.Trial(b)
			requireStage(t, failed, err, paragraph.StageBreak, khipu.ErrIllegalArguments)
			b.Shape = nil
			b.Layout.Params.Constraints.Width = -1
			failed, err = p.Trial(b)
			requireStage(t, failed, err, paragraph.StageInput, paragraph.ErrInvalidInput)
			b.Layout = flowRequest(paragraph.FirstFit)
			b.Layout.Params.Constraints.Width = 5 * dimen.BP
			b.Layout.Params.LineLayout.Default = length(dimen.Fil / 2)
			badAssembly := mustTrial(t, p, b)
			out, err := badAssembly.Assemble()
			requireStage(t, out, err, paragraph.StageAssemble, khipu.ErrIllegalArguments)
			if !reflect.DeepEqual(want, trialOutput(mustAssembleTrial(t, a))) ||
				!reflect.DeepEqual(plan, a.Plan().Lines()) || !reflect.DeepEqual(policy, a.Plan().Policies()) {
				t.Fatal("later outcome or input mutation changed retained trial")
			}
			noFlowShaping(t, before, calls.snapshot())
			if _, err := khipu.Freeze(a.Plan()); err != nil {
				t.Fatal("retained plan is not independently freezable", err)
			}
		})
	}
}

func TestTrialDiscoveryFailureRetryAndSelectedReplacements(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		failure := errors.New("discovery unavailable")
		failed, discoveries := true, 0
		var calls measurementCalls
		cfg := measurementConfig(&calls)
		cfg.Hyphenator = measureExplicitPolicy(func(s string, p *khipu.Params) ([]khipu.HyphenationFragment, error) {
			discoveries++
			if failed {
				return nil, failure
			}
			return softHyphens{}.Discretionaries(s, p)
		})
		p, err := paragraph.Prepare(fitTextInput("ab\u00adcd"), khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		r := paragraph.TrialRequest{Layout: paragraph.LayoutRequest{Params: params(breaker, 100*dimen.BP)}}
		a := mustTrial(t, p, r)
		wantA := trialOutput(mustAssembleTrial(t, a))
		r.Layout.Params.Constraints.Width = 15 * dimen.BP
		bad, err := p.Trial(r)
		requireStage(t, bad, err, paragraph.StageBreak, failure)
		if !reflect.DeepEqual(wantA, trialOutput(mustAssembleTrial(t, a))) {
			t.Fatal("discovery failure changed A")
		}
		failed = false
		selected := mustTrial(t, p, r)
		wantSelected := trialOutput(mustAssembleTrial(t, selected))
		if len(selected.Plan().Lines()) != 2 || discoveries < 2 {
			t.Fatal("did not retry failed discovery/select replacement", discoveries)
		}
		warm, shapes := discoveries, calls.shape
		r.Layout.Params.Constraints.Width = 100 * dimen.BP
		mustTrial(t, p, r)
		r.Layout.Params.Constraints.Width = 15 * dimen.BP
		mustTrial(t, p, r)
		if discoveries != warm || calls.shape != shapes ||
			!reflect.DeepEqual(wantSelected, trialOutput(mustAssembleTrial(t, selected))) ||
			!reflect.DeepEqual(wantA, trialOutput(mustAssembleTrial(t, a))) {
			t.Fatal("warm discovery or later selections changed retained candidates")
		}
		cp, err := selected.Checkpoint(0)
		requireStage(t, cp, err, paragraph.StageInput, khipu.ErrUnsupportedPolicy)
	}
}

func TestTrialInvalidHandlesAndPolicyArms(t *testing.T) {
	for _, a := range []*paragraph.Trial{nil, {}} {
		if a.Plan() != nil || !reflect.DeepEqual(a.Inputs(), paragraph.LayoutRequest{}) {
			t.Fatal("invalid trial returned data")
		}
		r, err := a.Assemble()
		requireStage(t, r, err, paragraph.StageInput, khipu.ErrIllegalArguments)
		cp, err := a.Checkpoint(0)
		requireStage(t, cp, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	for _, p := range []*paragraph.Prepared{nil, {}} {
		a, err := p.Trial(paragraph.TrialRequest{})
		requireStage(t, a, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("")}, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	r := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit), KnuthPlass: knuthplass.NewDefaultParameters()}
	a, err := p.Trial(r)
	requireStage(t, a, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	r.KnuthPlass, r.Prefix = nil, &paragraph.TrialCheckpoint{}
	a, err = p.Trial(r)
	requireStage(t, a, err, paragraph.StageBind, khipu.ErrIllegalArguments)
	r.Prefix = nil
	empty := mustAssembleTrial(t, mustTrial(t, p, r))
	if empty.VList.Box().Extent != 0 || len(empty.VList.Lines()) != 0 {
		t.Fatal("empty paragraph acquired a strut")
	}
}
