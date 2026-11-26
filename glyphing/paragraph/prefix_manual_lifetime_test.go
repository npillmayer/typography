package paragraph

import (
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

type lifetimeManualPolicy struct{ padding [32]byte }

func (*lifetimeManualPolicy) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

func retainedManualPrefix(t *testing.T, complete bool) (prefixWeak, weak.Pointer[lifetimeManualPolicy], *FittedPrefix) {
	t.Helper()
	h := new(lifetimeManualPolicy)
	cfg := khipu.Config{Hyphenator: h, Streaming: &khipu.StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP}, NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) }}}
	p, e := Prepare(Input{Text: styled.TextFromString("ab\u00adcd ef")}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	shape := &prefixShape{width: 15 * dimen.BP}
	r := LayoutRequest{Params: Params{Constraints: Constraints{Width: 15 * dimen.BP}, Breaker: FirstFit, Assembly: Options{Policy: CSSLineHeight}, LineLayout: &LineLayout{Default: LineHeight{Kind: FactorHeight, Factor: 1}}}, Horizontal: &HorizontalLayout{}}
	a, e := p.Trial(TrialRequest{Layout: r, Shape: shape})
	if e != nil {
		t.Fatal(e)
	}
	fit, e := a.FitPrefixes(1000)
	if e != nil {
		t.Fatal(e)
	}
	n := 1
	if complete {
		n = fit.Prefixes()[0].Lines
	}
	out, e := fit.Select(n)
	if e != nil {
		t.Fatal(e)
	}
	return prefixWeak{trialWeakHandles: trialWeakHandles{prepared: weak.Make(p), discovery: weak.Make(p.discovery), material: weak.Make(p.content), trial: weak.Make(a), plan: weak.Make(a.plan), checkpoint: weak.Make(out.checkpoint)}, fit: weak.Make(fit), shape: weak.Make(shape)}, weak.Make(h), out
}

func TestManualPrefixReleasesDiscoveryAndTrials(t *testing.T) {
	for _, complete := range []bool{false, true} {
		refs, policy, out := retainedManualPrefix(t, complete)
		for range 5 {
			runtime.GC()
		}
		if refs.prepared.Value() != nil || refs.discovery.Value() != nil || refs.trial.Value() != nil || refs.plan.Value() != nil || refs.fit.Value() != nil || refs.shape.Value() != nil || policy.Value() != nil {
			t.Fatal("manual selection retains execution services")
		}
		if (refs.material.Value() == nil) != complete {
			t.Fatal("wrong checkpoint material lifetime")
		}
		if out.Fragment().Frozen().SourceLen() != 9 || len(out.Coverage()) == 0 || out.Coverage()[0].Kind != ReplacementCoverage {
			t.Fatal("lost selected replacement")
		}
		runtime.KeepAlive(out)
	}
}

func manualFragmentOnly(t *testing.T) (prefixWeak, *layout.Fragment) {
	refs, _, out := retainedManualPrefix(t, false)
	return refs, out.Fragment()
}

func TestManualPrefixFragmentReleasesCheckpoint(t *testing.T) {
	refs, out := manualFragmentOnly(t)
	for range 5 {
		runtime.GC()
	}
	if refs.prepared.Value() != nil || refs.material.Value() != nil || refs.discovery.Value() != nil || refs.trial.Value() != nil || refs.plan.Value() != nil || refs.checkpoint.Value() != nil || refs.fit.Value() != nil || refs.shape.Value() != nil {
		t.Fatal("manual fragment retains working state")
	}
	x, ok := out.Frozen().CopyText(out.Lines()[0].Horizontal.FirstKnot)
	if !ok || !x.IsReplacement || x.ReplacementText != "ab-" {
		t.Fatal("lost replacement storage", x)
	}
	runtime.KeepAlive(out)
}
