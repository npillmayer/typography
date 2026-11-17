package layout_test

import (
	"errors"
	"math"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

func unboxConfig() khipu.Config {
	return khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver:  helpers.DefaultStyleResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
	}}
}

func unboxParams(breaker paragraph.Breaker, width dimen.DU, factor float64) paragraph.Params {
	return paragraph.Params{Constraints: paragraph.Constraints{Width: width}, Breaker: breaker,
		Assembly:   paragraph.Options{Policy: paragraph.CSSLineHeight},
		LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: factor}},
	}
}

func TestUnboxFullLineHeightAndEmptyOutput(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, tc := range []struct {
			text                            string
			factor, height, ascent, descent float64
		}{
			{"aa\n\naa", 1, 10, 8, 2},
			{"aa\n\naa", .25, 2.5, 4.25, -1.75},
			{"aa\n\naa", 0, 0, 3, -3},
			{"", 1, 0, 0, 0},
		} {
			for _, width := range []dimen.DU{0, 40 * dimen.BP} {
				r, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString(tc.text)}, unboxParams(breaker, width, tc.factor), unboxConfig())
				if err != nil {
					t.Fatal(err)
				}
				before, materials, box := r.VList.Lines(), r.VList.Materials(), r.VList.Box()
				list, err := layout.Unbox(r.VList)
				if err != nil || list == nil || list.Len() != len(before) {
					t.Fatal(list, err)
				}
				if tc.text == "" && (list.Len() != 0 || r.Frozen.LineCount() == 0) {
					t.Fatal("diagnostic line became an item")
				}
				for i, old := range before {
					item, ok := list.Item(i)
					f, h := item.Horizontal()
					a, d := item.Extents()
					b := item.Box()
					if !ok || f != r.Frozen || h != old.Horizontal || b.Extent != tc.height || a != tc.ascent || d != tc.descent || b.Baseline != tc.ascent || !b.HasBaseline {
						t.Fatalf("%s factor=%v item %d: box=%+v extents=%v,%v", breaker, tc.factor, i, b, a, d)
					}
					if b.Width != math.Max(h.Width.Points(), h.TargetWidth.Points()) {
						t.Fatal("lost overfull/target width")
					}
				}
				for _, i := range []int{-1, list.Len(), list.Len() + 1} {
					if _, ok := list.Item(i); ok {
						t.Fatal("accepted invalid index", i)
					}
				}
				if !reflect.DeepEqual(before, r.VList.Lines()) || !reflect.DeepEqual(materials, r.VList.Materials()) || box != r.VList.Box() {
					t.Fatal("changed original placement")
				}
			}
		}
	}
	for _, v := range []*layout.VList{nil, new(layout.VList)} {
		if got, err := layout.Unbox(v); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
	for _, list := range []*layout.ElasticVList{nil, new(layout.ElasticVList)} {
		if list.Len() != 0 {
			t.Fatal("nonempty zero list")
		}
		if _, ok := list.Item(0); ok {
			t.Fatal("zero list contains an item")
		}
	}
}

func TestUnboxLocalDepthDomain(t *testing.T) {
	r, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("aa\naa")},
		unboxParams(paragraph.FirstFit, 40*dimen.BP, 1), unboxConfig())
	if err != nil {
		t.Fatal(err)
	}
	lines, materials, box := r.VList.Lines(), r.VList.Materials(), r.VList.Box()
	if len(lines) != 2 {
		t.Fatal("fixture needs two lines", len(lines))
	}
	for i := range lines {
		lines[i].Top, lines[i].Height = float64(i)*100, 100
		lines[i].Ascent, lines[i].Descent = 80, 20
		lines[i].Baseline = lines[i].Top + 80
		materials[i].Top, materials[i].Extent = lines[i].Top, lines[i].Height
	}
	box.Extent, box.Baseline = 200, 80
	limit := dimen.Fil.Points()
	lines[1].Descent = math.Nextafter(limit, 0)
	lines[1].Ascent = lines[1].Height - lines[1].Descent
	for _, tc := range []struct {
		name           string
		baselineOffset float64
		wantError      bool
	}{
		{"just inside", 0, false},
		{"within arithmetic tolerance", 5e-10, false},
		{"at limit", -(limit - lines[1].Descent), true},
		{"beyond limit", -5e-10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// These offsets obey the source's 1e-9 arithmetic tolerance. The
			// retained descent stays finite, but the newly derived depth may not.
			lines[1].Baseline = lines[1].Top + lines[1].Ascent + tc.baselineOffset
			v, err := layout.NewVList(r.Frozen, box, lines, materials, nil)
			if err != nil {
				t.Fatal("source geometry must remain accepted", err)
			}
			depth := lines[1].Height - (lines[1].Baseline - lines[1].Top)
			if (depth >= limit) != tc.wantError {
				t.Fatalf("fixture depth %.17g does not exercise limit %.17g", depth, limit)
			}
			list, err := layout.Unbox(v)
			if tc.wantError {
				if list != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
					t.Fatal("published partial/out-of-domain items", list, err)
				}
			} else {
				if err != nil || list == nil || list.Len() != 2 {
					t.Fatal("rejected valid local geometry", list, err)
				}
				item, _ := list.Item(1)
				ascent, descent := item.Extents()
				if item.Box().Depth() != depth || ascent != lines[1].Ascent || descent != lines[1].Descent {
					t.Fatal("changed translated geometry or retained extents")
				}
			}
			if !reflect.DeepEqual(v.Lines(), lines) || !reflect.DeepEqual(v.Materials(), materials) || v.Box() != box {
				t.Fatal("unboxing changed the source geometry")
			}
		})
	}
}

func unboxAtomicFixture(t *testing.T, direction bidi.Direction) *layout.VList {
	t.Helper()
	p, err := paragraph.Prepare(t.Context(), paragraph.Input{
		Text: styled.TextFromString("\uFFFC\uFFFC\naa\n\uFFFC"), Direction: direction,
		Atomics: []contracts.AtomicAnchor{{ID: 7, ByteOffset: 0}, {ID: 2, ByteOffset: 3}, {ID: 9, ByteOffset: 10}},
	}, khipu.PrepareOptions{}, unboxConfig())
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Layout(t.Context(), paragraph.LayoutRequest{
		Params: unboxParams(paragraph.FirstFit, 40*dimen.BP, 1),
		Atomics: []paragraph.AtomicBox{
			{ID: 7, Margin: paragraph.AtomicMargins{Right: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge},
			{ID: 2, BorderWidth: 4 * dimen.BP, BorderHeight: 6 * dimen.BP, Margin: paragraph.AtomicMargins{Left: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge},
			{ID: 9, Alignment: paragraph.AlignBottomMarginEdge},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r.VList
}

func TestUnboxAtomicTraversalOwnershipAndFailure(t *testing.T) {
	for _, direction := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		v := unboxAtomicFixture(t, direction)
		old := v.AtomicPlacements()
		list, err := layout.Unbox(v)
		if err != nil {
			t.Fatal(err)
		}
		at := 0
		for i, l := range v.Lines() {
			item, _ := list.Item(i)
			for _, a := range item.AtomicPlacements() {
				want := old[at]
				want.Line, want.X, want.Y = 0, want.X-l.X, want.Y-l.Top
				if a != want {
					t.Fatal("reordered or mis-rebased atomic", a, want)
				}
				at++
			}
			clear(item.AtomicPlacements())
		}
		if at != len(old) || !reflect.DeepEqual(old, v.AtomicPlacements()) {
			t.Fatal("lost or changed atomics")
		}
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 20 {
					again, err := layout.Unbox(v)
					if err != nil {
						t.Error(err)
						return
					}
					for i := 0; i < list.Len(); i++ {
						a, _ := list.Item(i)
						b, _ := again.Item(i)
						if a.Box() != b.Box() || !reflect.DeepEqual(a.AtomicPlacements(), b.AtomicPlacements()) {
							t.Error("changed reusable items")
						}
						clear(b.AtomicPlacements())
					}
				}
			})
		}
		wg.Wait()
		// Valid paragraph coordinates may exceed the finite domain after translation.
		badAtoms := v.AtomicPlacements()
		badAtoms[len(badAtoms)-1].Y = -dimen.Fil.Points() + .5
		bad, err := layout.NewVList(v.Frozen(), v.Box(), v.Lines(), v.Materials(), badAtoms)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := layout.Unbox(bad); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal("published partial/out-of-domain items", got, err)
		}
		if !reflect.DeepEqual(old, v.AtomicPlacements()) {
			t.Fatal("failure mutated old result")
		}
	}
}

func TestVItemRetainsContentWithoutList(t *testing.T) {
	item := func() layout.VItem {
		v := unboxAtomicFixture(t, bidi.LeftToRight)
		list, err := layout.Unbox(v)
		if err != nil {
			t.Fatal(err)
		}
		item, _ := list.Item(1)
		return item
	}()
	runtime.GC()
	f, line := item.Horizontal()
	if f == nil || line.KnotCount == 0 {
		t.Fatal("lost horizontal content")
	}
	found := false
	for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
		if text, ok := f.CopyText(k); ok {
			found = true
			if len(text.Glyphs) == 0 {
				t.Fatal("lost glyphs")
			}
			text.Glyphs[0].GlyphID = 0
			again, _ := f.CopyText(k)
			if again.Glyphs[0].GlyphID == 0 {
				t.Fatal("mutable frozen glyph alias")
			}
		}
	}
	if !found {
		t.Fatal("fixture needs retained text")
	}
}
