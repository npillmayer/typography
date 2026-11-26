package paragraph

import (
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/layout"
)

type prefixShape struct {
	width   dimen.DU
	padding [32]byte // Avoid tiny-allocator grouping in weak-pointer GC assertions.
}

func (s *prefixShape) LineLength(int32) dimen.DU { return s.width }

type prefixWeak struct {
	trialWeakHandles
	fit   weak.Pointer[PrefixFit]
	shape weak.Pointer[prefixShape]
}

func retainedPrefix(t *testing.T, complete bool) (prefixWeak, *FittedPrefix) {
	t.Helper()
	p, r := reflowPreparation(t)
	shape := &prefixShape{width: 40 * dimen.BP}
	a, err := p.Trial(TrialRequest{Layout: r, Shape: shape})
	if err != nil {
		t.Fatal(err)
	}
	f, err := a.FitPrefixes(1000)
	if err != nil {
		t.Fatal(err)
	}
	n := 1
	if complete {
		n = f.Prefixes()[0].Lines
	}
	out, err := f.Select(n)
	if err != nil {
		t.Fatal(err)
	}
	return prefixWeak{trialWeakHandles: trialWeakHandles{
		prepared: weak.Make(p), discovery: weak.Make(p.discovery), material: weak.Make(p.content), trial: weak.Make(a), plan: weak.Make(a.plan), checkpoint: weak.Make(out.checkpoint),
	}, fit: weak.Make(f), shape: weak.Make(shape)}, out
}

func TestPrefixSelectionReleasesTrialWorkspace(t *testing.T) {
	for _, complete := range []bool{false, true} {
		refs, out := retainedPrefix(t, complete)
		for range 5 {
			runtime.GC()
		}
		if refs.prepared.Value() != nil || refs.discovery.Value() != nil || refs.trial.Value() != nil || refs.plan.Value() != nil || refs.fit.Value() != nil || refs.shape.Value() != nil {
			t.Fatalf("selection retains workspace: prepared=%v discovery=%v trial=%v plan=%v fit=%v shape=%v", refs.prepared.Value() != nil, refs.discovery.Value() != nil, refs.trial.Value() != nil, refs.plan.Value() != nil, refs.fit.Value() != nil, refs.shape.Value() != nil)
		}
		if complete && refs.material.Value() != nil {
			t.Fatal("completed selection retains material")
		}
		if !complete && refs.material.Value() == nil {
			t.Fatal("checkpoint lost material incarnation")
		}
		if len(out.Fragment().Lines()) == 0 || len(out.Coverage()) == 0 {
			t.Fatal("lost frozen output")
		}
		runtime.KeepAlive(out)
	}
}

func prefixFragmentsOnly(t *testing.T) (prefixWeak, *layout.Fragment) {
	t.Helper()
	refs, selected := retainedPrefix(t, false)
	return refs, selected.Fragment()
}

func TestPrefixFragmentReleasesAllWorkingHandles(t *testing.T) {
	refs, out := prefixFragmentsOnly(t)
	for range 5 {
		runtime.GC()
	}
	if refs.prepared.Value() != nil || refs.material.Value() != nil || refs.discovery.Value() != nil || refs.trial.Value() != nil || refs.plan.Value() != nil || refs.checkpoint.Value() != nil || refs.fit.Value() != nil || refs.shape.Value() != nil {
		t.Fatal("frozen fragment retains execution state")
	}
	if out.Frozen().SourceLen() == 0 || len(out.Lines()) != 1 {
		t.Fatal("lost retained fragment")
	}
	runtime.KeepAlive(out)
}
