package paragraph

import (
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/khipu"
)

type trialWeakHandles struct {
	prepared   weak.Pointer[Prepared]
	discovery  weak.Pointer[khipu.Discovery]
	material   weak.Pointer[khipu.Khipu]
	trial      weak.Pointer[Trial]
	plan       weak.Pointer[khipu.Tikray]
	checkpoint weak.Pointer[TrialCheckpoint]
}

// Separate frames avoid accidentally keeping tested handles live on the test stack.
func retainedTrial(t *testing.T) (trialWeakHandles, *Trial) {
	t.Helper()
	p, r := reflowPreparation(t)
	a, err := p.Trial(TrialRequest{Layout: r})
	if err != nil {
		t.Fatal(err)
	}
	return trialWeakHandles{prepared: weak.Make(p), discovery: weak.Make(p.discovery),
		material: weak.Make(p.content), trial: weak.Make(a), plan: weak.Make(a.plan)}, a
}

func TestTrialDoesNotRetainPreparationOrDiscovery(t *testing.T) {
	refs, a := retainedTrial(t)
	for range 5 {
		runtime.GC()
	}
	if refs.prepared.Value() != nil || refs.discovery.Value() != nil {
		t.Fatal("trial retains preparation or discovery")
	}
	if out, err := a.Assemble(); err != nil || out.VList.Box().Extent == 0 {
		t.Fatal("trial lost material needed for later assembly", err)
	}
	runtime.KeepAlive(a)
}

func retainedCheckpoint(t *testing.T) (trialWeakHandles, *TrialCheckpoint) {
	t.Helper()
	refs, a := retainedTrial(t)
	cp, err := a.Checkpoint(1)
	if err != nil {
		t.Fatal(err)
	}
	refs.checkpoint = weak.Make(cp)
	return refs, cp
}

func TestTrialCheckpointDoesNotRetainTrialOrDiscovery(t *testing.T) {
	refs, cp := retainedCheckpoint(t)
	for range 5 {
		runtime.GC()
	}
	if refs.prepared.Value() != nil || refs.discovery.Value() != nil || refs.trial.Value() != nil || refs.plan.Value() != nil {
		t.Fatal("checkpoint retains trial/preparation/discovery")
	}
	if refs.material.Value() == nil {
		t.Fatal("checkpoint lost its incarnation")
	}
	runtime.KeepAlive(cp)
}

func assembledTrial(t *testing.T) (trialWeakHandles, *Result) {
	t.Helper()
	refs, a := retainedTrial(t)
	cp, err := a.Checkpoint(1)
	if err != nil {
		t.Fatal(err)
	}
	refs.checkpoint = weak.Make(cp)
	out, err := a.Assemble()
	if err != nil {
		t.Fatal(err)
	}
	return refs, out
}

func TestTrialFrozenOutputReleasesWorkingHandles(t *testing.T) {
	refs, out := assembledTrial(t)
	for range 5 {
		runtime.GC()
	}
	if refs.prepared.Value() != nil || refs.discovery.Value() != nil || refs.material.Value() != nil ||
		refs.trial.Value() != nil || refs.plan.Value() != nil || refs.checkpoint.Value() != nil {
		t.Fatal("frozen output retains working state")
	}
	if out.Frozen.SourceLen() == 0 || len(out.VList.Lines()) == 0 {
		t.Fatal("lost retained output")
	}
	runtime.KeepAlive(out)
}
