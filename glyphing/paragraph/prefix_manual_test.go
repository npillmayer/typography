package paragraph_test

import (
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

const manualPrefixText = "zz\nzz ab\u00adcd ef\u00adgh ij kl"

type prefixManualPolicy struct {
	calls     int
	failAt    int
	err       error
	pre, post string
}

func (h *prefixManualPolicy) Discretionaries(s string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.calls++
	if s != "\u00ad" {
		return nil, errors.New("manual policy received automatic request")
	}
	if h.calls == h.failAt {
		return nil, h.err
	}
	return []khipu.HyphenationFragment{{PreBreak: h.pre, PostBreak: h.post}}, nil
}

func manualPrefixes(t *testing.T, input paragraph.Input, policy *prefixManualPolicy) (*paragraph.Prepared, *fitServiceCalls) {
	t.Helper()
	c := new(fitServiceCalls)
	cfg := flowConfig(c)
	cfg.Hyphenator = policy
	p, e := paragraph.Prepare(input, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	return p, c
}

func manualPrefixRequest(w dimen.DU) paragraph.TrialRequest {
	r := paragraph.TrialRequest{Layout: flowRequest(paragraph.FirstFit)}
	r.Layout.Params.Constraints.Width = w * dimen.BP
	return r
}

func manualFragmentText(t *testing.T, f *layout.Fragment) []string {
	t.Helper()
	var out []string
	for _, l := range f.Lines() {
		var text strings.Builder
		for i := l.Horizontal.FirstKnot; i < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; i++ {
			k, _ := f.Frozen().Knot(i)
			if k.Material == khipu.ParagraphFillMaterial {
				continue
			}
			x, ok := f.Frozen().CopyText(i)
			if ok && x.IsReplacement {
				text.WriteString(x.ReplacementText)
			} else {
				r, e := f.Frozen().SourceReader(k.Source.Bytes)
				if e != nil {
					t.Fatal(e)
				}
				v, e := io.ReadAll(r)
				if e != nil {
					t.Fatal(e)
				}
				text.Write(v)
			}
		}
		out = append(out, strings.TrimSpace(strings.ReplaceAll(text.String(), "\u00ad", "")))
	}
	return out
}

func manualFragmentSnapshot(f *layout.Fragment) []any {
	out := []any{f.Box(), f.Lines(), f.AtomicPlacements()}
	for _, l := range f.Lines() {
		for i := l.Horizontal.FirstKnot; i < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; i++ {
			k, _ := f.Frozen().Knot(i)
			x, _ := f.Frozen().CopyText(i)
			out = append(out, k, x)
		}
	}
	return out
}

func checkManualPartition(t *testing.T, size uint64, fragments ...*layout.Fragment) {
	t.Helper()
	var end uint64
	fills := 0
	for _, f := range fragments {
		for _, l := range f.Lines() {
			if l.Horizontal.Source.Start != end {
				t.Fatal("source gap/overlap", end, l.Horizontal.Source)
			}
			end = l.Horizontal.Source.End
			for i := l.Horizontal.FirstKnot; i < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; i++ {
				k, _ := f.Frozen().Knot(i)
				if k.Material == khipu.ParagraphFillMaterial {
					fills++
					if end != size {
						t.Fatal("early fill")
					}
				}
			}
		}
	}
	if end != size || fills != 1 {
		t.Fatal("incomplete/duplicated material", end, fills)
	}
}

func TestManualPrefixEveryCutAndChangedWidth(t *testing.T) {
	h := &prefixManualPolicy{pre: "-"}
	p, c := manualPrefixes(t, fitTextInput(manualPrefixText), h)
	a := mustTrial(t, p, manualPrefixRequest(30))
	fit := mustPrefixes(t, a, 1000)
	all := selectPrefix(t, fit, 5)
	want := []string{"zz", "zz ab-", "cd ef-", "gh ij", "kl"}
	if got := manualFragmentText(t, all.Fragment()); !slices.Equal(got, want) {
		t.Fatal(got)
	}
	if h.calls != 2 || c.snapshot()[1] != 5 {
		t.Fatal("unexpected replacement work", h.calls, c.snapshot())
	}
	for n := 0; n <= 5; n++ {
		limited := mustPrefixes(t, a, float64(n)*10)
		if limited.Prefixes()[0].Lines != n {
			t.Fatal("height eligibility", n, limited.Prefixes())
		}
		first := selectPrefix(t, limited, n)
		original := manualFragmentSnapshot(first.Fragment())
		if n == 5 {
			if first.Checkpoint() != nil {
				t.Fatal("completed checkpoint")
			}
			continue
		}
		for _, w := range []dimen.DU{20, 30, 40} {
			r := manualPrefixRequest(w)
			r.Prefix = first.Checkpoint()
			r.Shape = trialShape(func(line int32) dimen.DU {
				if int(line) <= n {
					t.Fatal("accepted width queried", line)
				}
				return w * dimen.BP
			})
			b := mustTrial(t, p, r)
			before := c.snapshot()
			suffix := mustPrefixes(t, b, 1000)
			noFlowShaping(t, before, c.snapshot())
			last := selectPrefix(t, suffix, suffix.Prefixes()[0].Lines)
			if last.Info().From != first.Info().To || last.Info().Status != paragraph.FitComplete || last.Fragment().Range().Start != n {
				t.Fatal("invalid continuation association", n, last.Info())
			}
			checkManualPartition(t, uint64(len(manualPrefixText)), first.Fragment(), last.Fragment())
			got := append(manualFragmentText(t, first.Fragment()), manualFragmentText(t, last.Fragment())...)
			if w == 30 && !slices.Equal(got, want) {
				t.Fatal("same-width carry", n, got)
			}
			if n == 2 && w == 40 && !slices.Equal(got, []string{"zz", "zz ab-", "cd efgh", "ij kl"}) {
				t.Fatal("changed-width carry", got)
			}
			if !reflect.DeepEqual(original, manualFragmentSnapshot(first.Fragment())) {
				t.Fatal("accepted fragment changed")
			}
		}
	}
	if h.calls != 2 || c.snapshot()[1] != 5 {
		t.Fatal("reprepared warm material")
	}
	// Choosing an already assembled prefix is service-free even with carry.
	c.sealed.Store(true)
	for n := 0; n <= 5; n++ {
		selectPrefix(t, fit, n)
	}
}

func TestManualPrefixReplacementCoverageAndRepeatedResume(t *testing.T) {
	p, _ := manualPrefixes(t, fitTextInput(manualPrefixText), &prefixManualPolicy{pre: "-"})
	r := manualPrefixRequest(30)
	a := mustTrial(t, p, r)
	first := selectPrefix(t, mustPrefixes(t, a, 20), 2)
	if first.Info().To.Offset != 10 || first.Info().To.NextKnot != a.Plan().Lines()[1].End.Knot+1 {
		t.Fatal("not original split", first.Info())
	}
	r.Prefix = first.Checkpoint()
	b := mustTrial(t, p, r)
	second := selectPrefix(t, mustPrefixes(t, b, 10), 1)
	if second.Info().From != first.Info().To || second.Info().To.Offset != 17 {
		t.Fatal("second split", second.Info())
	}
	r.Prefix = second.Checkpoint()
	r.Layout.Params.Constraints.Width = 20 * dimen.BP
	tail := mustPrefixes(t, mustTrial(t, p, r), 1000)
	last := selectPrefix(t, tail, tail.Prefixes()[0].Lines)
	checkManualPartition(t, uint64(len(manualPrefixText)), first.Fragment(), second.Fragment(), last.Fragment())
	var replacements []khipu.ByteRange
	for _, s := range []*paragraph.FittedPrefix{first, second, last} {
		previous := s.Info().From.Offset
		for _, c := range s.Coverage() {
			if c.Source.Start < previous || c.Source.Start < c.Source.End && c.Source.End > s.Info().To.Offset {
				t.Fatal("replacement-local bytes escaped", c, s.Info())
			}
			previous = c.Source.Start
			if c.Kind == paragraph.ReplacementCoverage {
				replacements = append(replacements, c.Source)
			}
		}
	}
	if !slices.Equal(replacements, []khipu.ByteRange{{Start: 6, End: 10}, {Start: 10, End: 12}, {Start: 13, End: 17}, {Start: 17, End: 19}}) {
		t.Fatal(replacements)
	}
	// Nonempty owning ranges on either side of a selected SHY partition attach
	// to exactly the fragment containing that rendered replacement half.
	for _, tc := range []struct {
		source khipu.ByteRange
		owner  int
	}{{khipu.ByteRange{Start: 8, End: 10}, 0}, {khipu.ByteRange{Start: 10, End: 11}, 1}, {khipu.ByteRange{Start: 17, End: 18}, 2}} {
		count := 0
		for i, s := range []*paragraph.FittedPrefix{first, second, last} {
			for _, c := range s.Coverage() {
				if c.Source.Start < tc.source.End && tc.source.Start < c.Source.End {
					count++
					if i != tc.owner {
						t.Fatal("owning range moved", tc, i)
					}
				}
			}
		}
		if count != 1 {
			t.Fatal("lost/duplicated owning range", tc, count)
		}
	}
	copy := second.Coverage()
	copy[0].Source.End = 999
	if second.Coverage()[0].Source.End == 999 {
		t.Fatal("mutable coverage")
	}
}

func TestManualPrefixLazyFailuresAndRetry(t *testing.T) {
	for _, failureKind := range []string{"provider", "shaper"} {
		t.Run(failureKind, func(t *testing.T) {
			h := &prefixManualPolicy{pre: "-", err: errors.New("manual service failed")}
			c := new(fitServiceCalls)
			cfg := flowConfig(c)
			cfg.Hyphenator = h
			factory := cfg.Streaming.NewShaper
			failShape := false
			cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
				if failShape {
					return nil, h.err
				}
				return factory()
			}
			p, e := paragraph.Prepare(fitTextInput(manualPrefixText), khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
			if e != nil {
				t.Fatal(e)
			}
			wide := mustTrial(t, p, manualPrefixRequest(100))
			fit := mustPrefixes(t, wide, 1000)
			first := selectPrefix(t, fit, 1)
			old := manualFragmentSnapshot(first.Fragment())
			if h.calls != 0 || c.snapshot()[1] != 1 {
				t.Fatal("eager replacements")
			}
			if failureKind == "provider" {
				h.failAt = 2
			} else {
				failShape = true
			}
			r := manualPrefixRequest(30)
			r.Prefix = first.Checkpoint()
			bad, e := p.Trial(r)
			requireStage(t, bad, e, paragraph.StageBreak, h.err)
			if !reflect.DeepEqual(old, manualFragmentSnapshot(first.Fragment())) {
				t.Fatal("failure changed output")
			}
			selectPrefix(t, fit, 1)
			h.failAt = 0
			failShape = false
			b := mustTrial(t, p, r)
			f := mustPrefixes(t, b, 1000)
			last := selectPrefix(t, f, f.Prefixes()[0].Lines)
			if got := manualFragmentText(t, last.Fragment()); !slices.Equal(got, []string{"zz ab-", "cd ef-", "gh ij", "kl"}) {
				t.Fatal(got)
			}
			checkManualPartition(t, uint64(len(manualPrefixText)), first.Fragment(), last.Fragment())
			warm := c.snapshot()
			calls := h.calls
			mustTrial(t, p, r)
			noFlowShaping(t, warm, c.snapshot())
			if calls != h.calls {
				t.Fatal("retry rediscovered")
			}
		})
	}
}

func TestManualPrefixAtomicBindingsRemainPaired(t *testing.T) {
	input := fitTextInput("\uFFFC\nzz ab\u00adcd ef\u00adgh \uFFFC kl")
	input.Atomics = []contracts.AtomicAnchor{{ID: 7}, {ID: 9, ByteOffset: uint64(strings.LastIndex(input.Text.Raw().String(), "\uFFFC"))}}
	p, _ := manualPrefixes(t, input, &prefixManualPolicy{pre: "-"})
	r := manualPrefixRequest(30)
	r.Layout.Atomics = []paragraph.AtomicBox{atomicBox(7, 5, 8), atomicBox(9, 5, 8)}
	first := selectPrefix(t, mustPrefixes(t, mustTrial(t, p, r), 1000), 2)
	before := first.Atomics()
	snap := manualFragmentSnapshot(first.Fragment())
	if len(before) != 1 || before[0].Box.ID != 7 || before[0].Placement != first.Fragment().AtomicPlacements()[0] {
		t.Fatal(before)
	}
	r.Prefix = first.Checkpoint()
	r.Layout.Params.Constraints.Width = 40 * dimen.BP
	r.Layout.Atomics[1].BorderWidth = 15 * dimen.BP
	r.Layout.Atomics[1].BorderHeight = 30 * dimen.BP
	fit := mustPrefixes(t, mustTrial(t, p, r), 1000)
	last := selectPrefix(t, fit, fit.Prefixes()[0].Lines)
	if got := last.Atomics(); len(got) != 1 || got[0].Box != r.Layout.Atomics[1] || got[0].Placement != last.Fragment().AtomicPlacements()[0] {
		t.Fatal(got)
	}
	// Interiors are client-owned: these stable occurrence IDs associate each
	// retained placement with the corresponding immutable client payload.
	for _, change := range []func(*paragraph.AtomicBox){func(b *paragraph.AtomicBox) { b.BorderHeight++ }, func(b *paragraph.AtomicBox) { b.Margin.Left++; b.Margin.Right-- }} {
		badReq := r
		badReq.Layout.Atomics = slices.Clone(r.Layout.Atomics)
		change(&badReq.Layout.Atomics[0])
		bad, e := p.Trial(badReq)
		requireStage(t, bad, e, paragraph.StageBind, khipu.ErrIllegalArguments)
	}
	if !reflect.DeepEqual(before, first.Atomics()) || !reflect.DeepEqual(snap, manualFragmentSnapshot(first.Fragment())) {
		t.Fatal("consumed atomic changed")
	}
}

func TestManualPrefixUTF8BidiAndSilentHyphen(t *testing.T) {
	for _, tc := range []struct {
		source, pre, post string
		dir               bidi.Direction
	}{
		{"zz\néa\u00adβδ ef", "-", "", bidi.LeftToRight},
		{"אב\u00adגד הו זח", "-", "", bidi.RightToLeft},
		{"ab\u00adcd ef", "", "", bidi.LeftToRight},
		{"ab\u00adcd ef", "~", "!", bidi.LeftToRight},
	} {
		t.Run(tc.source+tc.pre, func(t *testing.T) {
			input := fitTextInput(tc.source)
			input.Direction = tc.dir
			p, _ := manualPrefixes(t, input, &prefixManualPolicy{pre: tc.pre, post: tc.post})
			r := manualPrefixRequest(15)
			a := mustTrial(t, p, r)
			f := mustPrefixes(t, a, 1000)
			cut := 0
			for i, l := range a.Plan().Lines() {
				if l.End.Kind == khipu.DiscretionaryBoundary {
					cut = i + 1
					break
				}
			}
			if cut == 0 {
				t.Fatal("fixture has no manual break")
			}
			first := selectPrefix(t, f, cut)
			if first.Info().To.Offset != uint64(strings.Index(tc.source, "\u00ad")+2) {
				t.Fatal("not original UTF-8 split", first.Info())
			}
			r.Prefix = first.Checkpoint()
			r.Layout.Params.Constraints.Width = 25 * dimen.BP
			tail := mustPrefixes(t, mustTrial(t, p, r), 1000)
			last := selectPrefix(t, tail, tail.Prefixes()[0].Lines)
			checkManualPartition(t, uint64(len(tc.source)), first.Fragment(), last.Fragment())
			for _, selected := range []*paragraph.FittedPrefix{first, last} {
				prev := selected.Info().From.Offset
				for _, c := range selected.Coverage() {
					if c.Source.Start < prev || c.Source.End > selected.Info().To.Offset {
						t.Fatal("invalid original coverage", c, selected.Info())
					}
					prev = c.Source.Start
				}
			}
		})
	}
}

func TestManualPrefixFullCheckpointHeightsAndProfile(t *testing.T) {
	p, _ := manualPrefixes(t, fitTextInput("ab\u00adcd ef"), &prefixManualPolicy{pre: "-"})
	r := manualPrefixRequest(15)
	r.Layout.Params.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 2, End: 4, Height: factor(3)}}
	a := mustTrial(t, p, r)
	if f := mustPrefixes(t, a, 29); len(f.Prefixes()) != 1 || f.Prefixes()[0].Status != paragraph.FitNoProgress {
		t.Fatal("replacement line-height lost", f.Prefixes())
	}
	first := selectPrefix(t, mustPrefixes(t, a, 30), 1)
	if first.Info().Height != 30 {
		t.Fatal(first.Info())
	}
	r.Prefix = mustCheckpoint(t, a, len(a.Plan().Lines()))
	r.Shape = trialShape(func(int32) dimen.DU { t.Fatal("completed prefix queried shape"); return 0 })
	complete := mustPrefixes(t, mustTrial(t, p, r), 0)
	last := selectPrefix(t, complete, 0)
	if len(complete.Prefixes()) != 1 || last.Info().Status != paragraph.FitComplete || last.Checkpoint() != nil || last.Info().From != last.Info().To {
		t.Fatal("full prefix did not finish", last.Info())
	}
	for _, n := range []int{-1, len(a.Plan().Lines()) + 1} {
		cp, e := a.Checkpoint(n)
		requireStage(t, cp, e, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	empty, _ := manualPrefixes(t, fitTextInput(""), &prefixManualPolicy{pre: "-"})
	if got := selectPrefix(t, mustPrefixes(t, mustTrial(t, empty, manualPrefixRequest(15)), 0), 0); got.Info().Status != paragraph.FitComplete || got.Checkpoint() != nil {
		t.Fatal("empty manual paragraph")
	}
	cfg := synthetic()
	cfg.Hyphenator = flowHyphenator{}
	automatic, e := paragraph.Prepare(fitTextInput("aa"), khipu.PrepareOptions{Hyphenation: khipu.AutomaticHyphenation}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	autoTrial := mustTrial(t, automatic, manualPrefixRequest(100))
	cp, e := autoTrial.Checkpoint(0)
	requireStage(t, cp, e, paragraph.StageInput, khipu.ErrUnsupportedPolicy)
	f, e := autoTrial.FitPrefixes(100)
	requireStage(t, f, e, paragraph.StageInput, khipu.ErrUnsupportedPolicy)
	// A service's explicitly unsupported manual feature remains an error, not a
	// successful empty/no-progress choice or a switch to NoHyphenation.
	unsupported, _ := manualPrefixes(t, fitTextInput("ab\u00adcd"), &prefixManualPolicy{failAt: 1, err: khipu.ErrUnsupportedPolicy})
	bad, e := unsupported.Trial(manualPrefixRequest(15))
	requireStage(t, bad, e, paragraph.StageBreak, khipu.ErrUnsupportedPolicy)
}
