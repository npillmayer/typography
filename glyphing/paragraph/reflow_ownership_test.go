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
// succeeds or fails. The public trial API is deferred; exercise our private seam.
func TestRetainedTikraySurvivesLaterLayouts(t *testing.T) {
	p, request := reflowPreparation(t)
	b, err := p.bindLayout(request)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := breakLayout(breakRequest(b.bound, p.discovery, b.geometry), b.params.Breaker)
	if err != nil {
		t.Fatal(err)
	}
	assemble := func() []any {
		r, err := finishWithPlan(plan, b.bound.KnotCount(), b.params, p.text, p.config, p.runs, b.boxes)
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
	c, err := p.StartFlow(request)
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.Fit(Frame{Width: 40 * dimen.BP, Height: 10})
	if err != nil || a.Continuation == nil {
		t.Fatal("expected initial remainder", err)
	}
	b, err := a.Continuation.Fit(Frame{Width: 25 * dimen.BP, Height: 10})
	if err != nil || b.Continuation == nil {
		t.Fatal("expected continued remainder", err)
	}
	done, err := b.Continuation.Fit(Frame{Width: 25 * dimen.BP, Height: 1000})
	if err != nil || done.Continuation != nil || !done.Reused {
		t.Fatal("expected equal-width completion", err)
	}
	return reflowWeakHandles{weak.Make(p), weak.Make(p.content), weak.Make(p.discovery), weak.Make(c.plan), weak.Make(b.Continuation.plan)},
		[]*layout.Fragment{a.Placed, b.Placed, done.Placed}
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
