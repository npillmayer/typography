package paragraph

import (
	"errors"
	"reflect"
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/layout"
)

func reflowPreparation(t *testing.T) (*Prepared, LayoutRequest) {
	t.Helper()
	cfg := khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver:  helpers.DefaultStyleResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
	}}
	p, err := Prepare(Input{Text: styled.TextFromString("aa bb cc dd ee ff gg hh ii jj")}, khipu.PrepareOptions{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := LayoutRequest{Params: Params{Constraints: Constraints{Width: 40 * dimen.BP}, Breaker: FirstFit,
		Assembly: Options{Policy: CSSLineHeight}, LineLayout: &LineLayout{Default: LineHeight{Kind: FactorHeight, Factor: 1}}}, Horizontal: &HorizontalLayout{}}
	return p, r
}

func resultMaterial(r *Result) []any {
	out := []any{r.VList.Box(), r.VList.Lines(), r.VList.Materials(), r.VList.AtomicPlacements(), r.BreakPositions, r.EncodedKnots, r.BreakCount}
	for i := 0; i < r.Frozen.KnotCount(); i++ {
		knot, _ := r.Frozen.Knot(i)
		text, _ := r.Frozen.CopyText(i)
		out = append(out, knot, text)
	}
	return out
}

// A completed trial must remain independently assemblable after another trial
// succeeds or fails. Exercise the low-level seam alongside the public Trial tests.
func TestRetainedTikraySurvivesLaterLayouts(t *testing.T) {
	p, request := reflowPreparation(t)
	trial, err := p.Trial(TrialRequest{Layout: request})
	if err != nil {
		t.Fatal(err)
	}
	assemble := func() []any {
		r, err := trial.Assemble()
		if err != nil {
			t.Fatal(err)
		}
		return resultMaterial(r)
	}
	want := assemble()
	request.Params.Constraints.Width = 25 * dimen.BP
	if _, err := p.Layout(request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(assemble(), want) {
		t.Fatal("later successful layout changed retained trial")
	}
	request.Params.Constraints.Width = -1
	if r, err := p.Layout(request); r != nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatal("expected rejected layout", err)
	}
	if !reflect.DeepEqual(assemble(), want) {
		t.Fatal("rejected layout changed retained trial")
	}
}

type reflowWeakHandles struct {
	prepared  weak.Pointer[Prepared]
	material  weak.Pointer[khipu.Khipu]
	discovery weak.Pointer[khipu.Discovery]
	initial   weak.Pointer[khipu.Tikray]
	continued weak.Pointer[khipu.Tikray]
}

// Keep all strong working handles in a separate stack frame, returning only
// weak references and frozen output from both sides of a changed-width split.
func reflowLifetime(t *testing.T) (reflowWeakHandles, []*layout.Fragment) {
	t.Helper()
	p, request := reflowPreparation(t)
	first, err := p.Trial(TrialRequest{Layout: request})
	if err != nil {
		t.Fatal(err)
	}
	choices, err := first.FitPrefixes(10)
	if err != nil {
		t.Fatal(err)
	}
	a, err := choices.Select(1)
	if err != nil {
		t.Fatal(err)
	}
	request.Params.Constraints.Width = 25 * dimen.BP
	continued, err := p.Trial(TrialRequest{Layout: request, Prefix: a.Checkpoint()})
	if err != nil {
		t.Fatal(err)
	}
	choices, err = continued.FitPrefixes(10)
	if err != nil {
		t.Fatal(err)
	}
	b, err := choices.Select(1)
	if err != nil {
		t.Fatal(err)
	}
	tail, err := choices.Refit(1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	done, err := tail.Select(tail.Prefixes()[0].Lines)
	if err != nil || done.Info().Status != FitComplete || done.Checkpoint() != nil {
		t.Fatal("expected completion", err)
	}
	return reflowWeakHandles{weak.Make(p), weak.Make(p.content), weak.Make(p.discovery), weak.Make(first.plan), weak.Make(continued.plan)},
		[]*layout.Fragment{a.Fragment(), b.Fragment(), done.Fragment()}
}

func TestFrozenFragmentsReleaseMaterialDiscoveryAndTikrays(t *testing.T) {
	refs, fragments := reflowLifetime(t)
	for range 5 {
		runtime.GC()
	}
	if refs.prepared.Value() != nil || refs.material.Value() != nil || refs.discovery.Value() != nil || refs.initial.Value() != nil || refs.continued.Value() != nil {
		t.Fatal("completed fragments retain working handles")
	}
	var end uint64
	for _, f := range fragments {
		for _, line := range f.Lines() {
			if line.Horizontal.Source.Start != end {
				t.Fatal("lost source continuity")
			}
			end = line.Horizontal.Source.End
		}
	}
	if end != fragments[0].Frozen().SourceLen() {
		t.Fatal("lost retained source")
	}
	runtime.KeepAlive(fragments)
}

func refitPredecessor(t *testing.T) (weak.Pointer[PrefixFit], *PrefixFit) {
	t.Helper()
	p, request := reflowPreparation(t)
	trial, err := p.Trial(TrialRequest{Layout: request})
	if err != nil {
		t.Fatal(err)
	}
	original, err := trial.FitPrefixes(10)
	if err != nil {
		t.Fatal(err)
	}
	next, err := original.Refit(1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return weak.Make(original), next
}

func TestPrefixRefitDoesNotRetainPredecessor(t *testing.T) {
	predecessor, tail := refitPredecessor(t)
	for range 5 {
		runtime.GC()
	}
	if predecessor.Value() != nil {
		t.Fatal("refit retained its predecessor")
	}
	completed, err := tail.Select(tail.Prefixes()[0].Lines)
	if err != nil || completed.Checkpoint() != nil {
		t.Fatal("retained tail unusable", err)
	}
	runtime.KeepAlive(tail)
}
