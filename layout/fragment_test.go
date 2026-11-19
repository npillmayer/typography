package layout_test

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

func fixture(t *testing.T) *layout.Fragment {
	t.Helper()
	r, err := paragraph.Run(paragraph.Input{Text: styled.TextFromString("aa\naa\naa")}, paragraph.Params{
		Constraints: paragraph.Constraints{Width: 30 * dimen.BP}, Breaker: paragraph.FirstFit,
		Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.NormalHeight}},
	}, khipu.Config{Streaming: &khipu.StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP}, NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) }}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := paragraph.Fragment(r.VList)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFragmentValidationAndOwnership(t *testing.T) {
	f := fixture(t)
	span, box := f.Range(), f.Box()
	lines, materials, atomics := f.Lines(), f.Materials(), f.AtomicPlacements()
	copy, err := layout.NewFragment(f.Frozen(), span, box, lines, materials, atomics)
	if err != nil {
		t.Fatal(err)
	}
	clear(lines)
	clear(materials)
	if !reflect.DeepEqual(copy.Lines(), f.Lines()) || !reflect.DeepEqual(copy.Materials(), f.Materials()) {
		t.Fatal("retained caller slices")
	}
	clear(copy.Lines())
	clear(copy.Materials())
	if !reflect.DeepEqual(copy.Lines(), f.Lines()) {
		t.Fatal("accessor aliases owner")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				clear(copy.Lines())
				clear(copy.Materials())
				clear(copy.AtomicPlacements())
				if copy.Frozen() != f.Frozen() || copy.Box() != box || copy.Range() != span {
					t.Error("changed immutable state")
				}
			}
		})
	}
	wg.Wait()
	for _, mutate := range []struct {
		name string
		fn   func(*layout.LineRange, *layout.Box, []layout.FragmentLine, *[]layout.Material)
	}{
		{"negative range", func(s *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, _ *[]layout.Material) { s.Start = -1 }},
		{"beyond owner", func(s *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, _ *[]layout.Material) { s.End++ }},
		{"range count", func(s *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, _ *[]layout.Material) { s.Start++ }},
		{"width", func(_ *layout.LineRange, b *layout.Box, _ []layout.FragmentLine, _ *[]layout.Material) { b.Width++ }},
		{"extent", func(_ *layout.LineRange, b *layout.Box, _ []layout.FragmentLine, _ *[]layout.Material) { b.Extent++ }},
		{"infinity", func(_ *layout.LineRange, b *layout.Box, _ []layout.FragmentLine, _ *[]layout.Material) {
			b.Extent = math.Inf(1)
		}},
		{"baseline", func(_ *layout.LineRange, b *layout.Box, _ []layout.FragmentLine, _ *[]layout.Material) {
			b.HasBaseline = false
		}},
		{"local index", func(_ *layout.LineRange, _ *layout.Box, l []layout.FragmentLine, _ *[]layout.Material) {
			l[1].Index = 0
		}},
		{"original index", func(_ *layout.LineRange, _ *layout.Box, l []layout.FragmentLine, _ *[]layout.Material) {
			l[1].OriginalIndex = 0
		}},
		{"horizontal", func(_ *layout.LineRange, _ *layout.Box, l []layout.FragmentLine, _ *[]layout.Material) {
			l[1].Horizontal.FirstKnot++
		}},
		{"nan", func(_ *layout.LineRange, _ *layout.Box, l []layout.FragmentLine, _ *[]layout.Material) {
			l[1].Ascent = math.NaN()
		}},
		{"line geometry", func(_ *layout.LineRange, _ *layout.Box, l []layout.FragmentLine, _ *[]layout.Material) {
			l[1].Baseline++
		}},
		{"out of domain", func(_ *layout.LineRange, _ *layout.Box, l []layout.FragmentLine, _ *[]layout.Material) {
			l[1].Baseline = dimen.Fil.Points()
		}},
		{"missing material", func(_ *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, m *[]layout.Material) {
			*m = (*m)[:len(*m)-1]
		}},
		{"duplicate child", func(_ *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, m *[]layout.Material) {
			(*m)[2].Child = 0
		}},
		{"overlap", func(_ *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, m *[]layout.Material) { (*m)[1].Top-- }},
		{"hole", func(_ *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, m *[]layout.Material) { (*m)[1].Top++ }},
		{"wrong extent", func(_ *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, m *[]layout.Material) {
			(*m)[0].Extent++
		}},
		{"negative extent", func(_ *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, m *[]layout.Material) {
			(*m)[1].Extent = -1
		}},
		{"reordered materials", func(_ *layout.LineRange, _ *layout.Box, _ []layout.FragmentLine, m *[]layout.Material) {
			(*m)[0], (*m)[1] = (*m)[1], (*m)[0]
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			s, b, l, m := span, box, f.Lines(), f.Materials()
			mutate.fn(&s, &b, l, &m)
			got, err := layout.NewFragment(f.Frozen(), s, b, l, m, nil)
			if got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
				t.Fatal(got, err)
			}
		})
	}
	for _, at := range []int{0, 1, span.End} {
		empty, err := layout.NewFragment(f.Frozen(), layout.LineRange{Start: at, End: at}, layout.Box{}, nil, nil, nil)
		if err != nil || empty.Box().HasBaseline || len(empty.Lines()) != 0 {
			t.Fatal(empty, err)
		}
	}
}

// Publication anchors must be identical, even when the mismatch is smaller
// than the tolerance for recomposed arithmetic identities. Otherwise rebasing
// a later line could turn an accepted material top negative.
func TestCSSPlacementAnchorsAndMaterialKinds(t *testing.T) {
	f := fixture(t)
	for _, delta := range []float64{-5e-10, 5e-10} {
		lines, materials := f.Lines(), f.Materials()
		materials[1].Top += delta
		if got, err := layout.NewFragment(f.Frozen(), f.Range(), f.Box(), lines, materials, nil); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("accepted independently rounded fragment anchor", got, err)
		}
		whole := make([]layout.Line, len(lines))
		for i, l := range lines {
			whole[i] = l.Line
		}
		if got, err := layout.NewVList(f.Frozen(), f.Box(), whole, materials, nil); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("accepted independently rounded vlist anchor", got, err)
		}
	}
	materials := f.Materials()
	materials[1].Kind, materials[1].Child = layout.Kern, -1
	if got, err := layout.NewFragment(f.Frozen(), f.Range(), f.Box(), f.Lines(), materials, nil); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal("accepted non-line material", got, err)
	}
	left, _, err := paragraph.SplitFragment(f, 2)
	if err != nil {
		t.Fatal(err)
	}
	a, b, err := paragraph.SplitFragment(left, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []*layout.Fragment{a, b} {
		line := part.Lines()[0]
		old := f.Lines()[line.OriginalIndex]
		if line.Horizontal != old.Horizontal || line.Top != 0 || line.Baseline != old.Baseline-old.Top ||
			part.Materials()[0].Top != line.Top || part.Materials()[0].Extent != line.Height {
			t.Fatal("repeated split lost CSS placement identity")
		}
	}
}
