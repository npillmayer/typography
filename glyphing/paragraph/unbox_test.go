package paragraph_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

func TestUnboxSelectedReplacementWithoutServices(t *testing.T) {
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
	if calls.shape == 0 || calls.metrics == 0 || calls.hyphenate == 0 {
		t.Fatal("fixture did not exercise services", calls)
	}
	var knots []khipu.FrozenKnot
	var texts []khipu.FrozenText
	replacement := false
	for k := 0; k < r.Frozen.KnotCount(); k++ {
		knot, _ := r.Frozen.Knot(k)
		text, ok := r.Frozen.CopyText(k)
		replacement = replacement || ok && text.IsReplacement
		knots, texts = append(knots, knot), append(texts, text)
	}
	if !replacement {
		t.Fatal("fixture has no selected replacement")
	}
	for range 3 {
		items, err := layout.Unbox(r.VList)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for i := 0; i < items.Len(); i++ {
			item, _ := items.Item(i)
			f, line := item.Horizontal()
			for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
				knot, _ := f.Knot(k)
				text, _ := f.CopyText(k)
				if knot != knots[k] || !reflect.DeepEqual(text, texts[k]) {
					t.Fatal("changed replacement/glyph/source coordinates")
				}
				seen++
			}
		}
		if seen != len(knots) || calls != before {
			t.Fatal("lost content or called services", calls, before)
		}
	}
}

func TestUnboxMixedSizesAndSpaceMetrics(t *testing.T) {
	text := styled.TextFromString("a b\naa")
	text, err := text.Style(sizeStyle{20 * dimen.BP, 17}, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	cfg := synthetic()
	cfg.Streaming.Resolver = resolver{}
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		params := cssParams(breaker, factor(1.5))
		params.Constraints.Width = 100 * dimen.BP
		r, err := paragraph.Run(paragraph.Input{Text: text}, params, cfg)
		if err != nil {
			t.Fatal(err)
		}
		items, err := layout.Unbox(r.VList)
		if err != nil || items.Len() != 2 {
			t.Fatal(items, err)
		}
		first, _ := items.Item(0)
		second, _ := items.Item(1)
		if first.Box().Extent != 30 || first.Box().Baseline != 21 || second.Box().Extent != 15 || second.Box().Baseline != 10.5 {
			t.Fatal("lost effective line-height", first.Box(), second.Box())
		}
	}
}
