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
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

func checkSplit(t *testing.T, v *layout.VList) {
	t.Helper()
	f, err := paragraph.Fragment(v)
	if err != nil {
		t.Fatal(err)
	}
	if f.Frozen() != v.Frozen() || f.Box() != v.Box() || !reflect.DeepEqual(f.Materials(), v.Materials()) || !reflect.DeepEqual(f.AtomicPlacements(), v.AtomicPlacements()) {
		t.Fatal("whole view changed geometry")
	}
	original := v.Lines()
	for i, l := range f.Lines() {
		if l.Line != original[i] || l.OriginalIndex != i {
			t.Fatal("whole line changed")
		}
	}
	for at := 1; at < len(original); at++ {
		a, b, err := paragraph.SplitFragment(f, at)
		if err != nil {
			t.Fatal(err)
		}
		if a.Range() != (layout.LineRange{End: at}) || b.Range() != (layout.LineRange{Start: at, End: len(original)}) {
			t.Fatal(a.Range(), b.Range())
		}
		if math.Abs(a.Box().Extent+b.Box().Extent-v.Box().Extent) > 1e-9 {
			t.Fatal("extent lost")
		}
		for _, part := range []*layout.Fragment{a, b} {
			origin := original[part.Range().Start].Top
			for i, l := range part.Lines() {
				old := original[l.OriginalIndex]
				if l.Index != i || l.Horizontal != old.Horizontal || l.Ascent != old.Ascent || l.Descent != old.Descent || l.Height != old.Height || math.Abs(l.Top+origin-old.Top) > 1e-9 || math.Abs(l.Baseline+origin-old.Baseline) > 1e-9 {
					t.Fatal("changed line", l, old)
				}
			}
			for _, p := range part.AtomicPlacements() {
				found := false
				for _, old := range v.AtomicPlacements() {
					if p.ID == old.ID {
						found = true
						if p.Line+part.Range().Start != old.Line || p.X != old.X || math.Abs(p.Y+origin-old.Y) > 1e-9 {
							t.Fatal("atomic translation", p, old)
						}
					}
				}
				if !found {
					t.Fatal("unknown atomic")
				}
			}
			if part.Frozen() != v.Frozen() {
				t.Fatal("copied horizontal owner")
			}
		}
		combined := a.Materials()
		for _, m := range b.Materials() {
			m.Top += original[at].Top
			if m.Kind == layout.LineBox {
				m.Child += at
			}
			combined = append(combined, m)
		}
		for i, m := range v.Materials() {
			got := combined[i]
			if got.Kind != m.Kind || got.Child != m.Child || got.Extent != m.Extent || math.Abs(got.Top-m.Top) > 1e-9 {
				t.Fatal("material lost", got, m)
			}
		}
		if len(b.Lines()) > 1 {
			_, tail, err := paragraph.SplitFragment(b, 1)
			if err != nil || tail.Range().Start != at+1 {
				t.Fatal(tail, err)
			}
		}
	}
	if !reflect.DeepEqual(original, v.Lines()) {
		t.Fatal("mutated original")
	}
}

func TestFragmentCSSPolicies(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, policy := range []paragraph.Params{params(breaker, 30*dimen.BP), cssParams(breaker, factor(.5)), cssParams(breaker, factor(0))} {
			r, err := paragraph.Run(paragraph.Input{Text: styled.TextFromString("aa\n\naa\naa")}, policy, synthetic())
			if err != nil {
				t.Fatal(err)
			}
			if len(r.VList.Lines()) < 3 {
				t.Fatal("fixture needs several lines")
			}
			checkSplit(t, r.VList)
		}
	}
	for _, text := range []string{"", "aa"} {
		r, err := paragraph.Run(paragraph.Input{Text: styled.TextFromString(text)}, cssParams(paragraph.FirstFit, factor(1)), synthetic())
		if err != nil {
			t.Fatal(err)
		}
		checkSplit(t, r.VList)
		f, err := paragraph.Fragment(r.VList)
		if err != nil {
			t.Fatal(err)
		}
		if text == "" && (len(f.Lines()) != 0 || f.Box() != (layout.Box{})) {
			t.Fatal("diagnostic line became content")
		}
		for _, at := range []int{-1, 0, len(f.Lines()), len(f.Lines()) + 1} {
			a, b, err := paragraph.SplitFragment(f, at)
			if a != nil || b != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
				t.Fatal(a, b, err)
			}
		}
	}
}

func TestFragmentAtomicsRTLAndOverflow(t *testing.T) {
	for _, direction := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("\uFFFC\uFFFC\naa\n\uFFFC"), Direction: direction, Atomics: []contracts.AtomicAnchor{{ID: 7, ByteOffset: 0}, {ID: 2, ByteOffset: 3}, {ID: 9, ByteOffset: 10}}}, khipu.PrepareOptions{}, synthetic())
		if err != nil {
			t.Fatal(err)
		}
		boxes := []paragraph.AtomicBox{atomicBox(7, 0, 0), atomicBox(2, 4, 6), atomicBox(9, 0, 0)}
		boxes[1].Margin.Left = -2 * dimen.BP
		boxes[0].Margin.Right = -2 * dimen.BP
		r, err := p.Layout(paragraph.LayoutRequest{Params: mixedParams(paragraph.FirstFit, 30*dimen.BP), Atomics: boxes})
		if err != nil {
			t.Fatal(err)
		}
		checkSplit(t, r.VList)
		f, err := paragraph.Fragment(r.VList)
		if err != nil {
			t.Fatal(err)
		}
		orig := f.AtomicPlacements()
		clear(f.AtomicPlacements())
		if !reflect.DeepEqual(orig, f.AtomicPlacements()) {
			t.Fatal("atomic slice alias")
		}
		for i := range orig {
			if orig[i].Line > 0 {
				orig[i].Y = -dimen.Fil.Points() + .5
			}
		}
		overflow, err := layout.NewFragment(f.Frozen(), f.Range(), f.Box(), f.Lines(), f.Materials(), orig)
		if err != nil {
			t.Fatal(err)
		}
		a, b, err := paragraph.SplitFragment(overflow, 1)
		if a != nil || b != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("translated out-of-domain atomic accepted", err)
		}
		// Missing, swapped, extra and invalid origins cannot be published.
		for _, bad := range [][]layout.AtomicPlacement{orig[:1], append(orig, orig[0]), {{ID: 99, Line: 0}}} {
			if got, err := layout.NewFragment(f.Frozen(), f.Range(), f.Box(), f.Lines(), f.Materials(), bad); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
				t.Fatal(got, err)
			}
		}
	}
}

func TestFragmentReplacementsNoServiceCalls(t *testing.T) {
	var calls measurementCalls
	cfg := measurementConfig(&calls)
	cfg.Hyphenator = measureExplicitPolicy(func(_ string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
		calls.hyphenate++
		return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
	})
	p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("ab\u00adcd\naa")}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Layout(paragraph.LayoutRequest{Params: params(paragraph.FirstFit, 15*dimen.BP)})
	if err != nil {
		t.Fatal(err)
	}
	before := calls
	replacement := false
	for k := 0; k < r.Frozen.KnotCount(); k++ {
		tx, ok := r.Frozen.CopyText(k)
		replacement = replacement || ok && tx.IsReplacement
	}
	if !replacement {
		t.Fatal("fixture has no selected replacement")
	}
	checkSplit(t, r.VList)
	if calls != before {
		t.Fatal("split queried services", before, calls)
	}
}

func TestFragmentRejectsMalformedVListAndNilInput(t *testing.T) {
	r, err := paragraph.Run(paragraph.Input{Text: styled.TextFromString("aa aa")}, params(paragraph.FirstFit, 15*dimen.BP), synthetic())
	if err != nil {
		t.Fatal(err)
	}
	m := r.VList.Materials()
	m[1].Top++
	// CSS publication rejects mismatched material/line placement anchors.
	v, err := layout.NewVList(r.Frozen, r.VList.Box(), r.VList.Lines(), m, nil)
	if v != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal("accepted mismatched placement", v, err)
	}
	for _, v := range []*layout.VList{nil, new(layout.VList)} {
		if f, err := paragraph.Fragment(v); f != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(f, err)
		}
	}
}
