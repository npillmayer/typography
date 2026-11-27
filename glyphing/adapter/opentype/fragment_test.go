package opentype

import (
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

func TestFragmentRealFontsAndOriginalSources(t *testing.T) {
	for _, tc := range []struct {
		file, text string
		direction  bidi.Direction
	}{
		{progressFont, "fi fi fi", bidi.LeftToRight},
		{"../../../../opentype/testdata/fonts/NotoSansHebrew-Regular.ttf", "אב אב אב", bidi.RightToLeft},
	} {
		entry, err := (Services{}).Load(t.Context(), tc.file, nil)
		if err != nil {
			t.Fatal(err)
		}
		mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{Faces: []glyphing.FaceEntry{entry}, Default: entry.Info.ResourceID, Size: 12 * dimen.BP}, Services{})
		if err != nil {
			t.Fatal(err)
		}
		text, _, err := mapper.Itemize(tc.text, nil)
		if err != nil {
			t.Fatal(err)
		}
		defaults := mapper.Defaults()
		if tc.direction == bidi.LeftToRight {
			defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Start: 3, End: 5, Enabled: true}}
		}
		for _, width := range []dimen.DU{0, 16 * dimen.BP} {
			created := 0
			cfg := khipu.Config{Streaming: &khipu.StreamingConfig{Resolver: mapper, Defaults: defaults, NewShaper: func() (contracts.StreamingShaper, error) { created++; return NewStream(8) }}}
			r, err := layoutText(paragraph.Input{Text: text, Direction: tc.direction}, paragraph.Params{Constraints: paragraph.Constraints{Width: width}, Breaker: paragraph.FirstFit, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.NormalHeight}}}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.VList.Lines()) < 2 {
				t.Fatal("fixture needs split")
			}
			var knots []khipu.FrozenKnot
			var texts []khipu.FrozenText
			for i := 0; i < r.Frozen.KnotCount(); i++ {
				k, _ := r.Frozen.Knot(i)
				tx, _ := r.Frozen.CopyText(i)
				knots = append(knots, k)
				texts = append(texts, tx)
			}
			before := created
			f, err := paragraph.Fragment(r.VList)
			if err != nil {
				t.Fatal(err)
			}
			left, right, err := paragraph.SplitFragment(f, 1)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for _, part := range []*layout.Fragment{left, right} {
				for _, line := range part.Lines() {
					orig := r.VList.Lines()[line.OriginalIndex]
					if line.Horizontal != orig.Horizontal || math.Abs(line.Baseline+r.VList.Lines()[part.Range().Start].Top-orig.Baseline) > 1e-9 {
						t.Fatal("line identity or position")
					}
					for i := line.Horizontal.FirstKnot; i < line.Horizontal.FirstKnot+line.Horizontal.KnotCount; i++ {
						k, _ := part.Frozen().Knot(i)
						tx, _ := part.Frozen().CopyText(i)
						if k != knots[i] || !reflect.DeepEqual(tx, texts[i]) {
							t.Fatal("source/glyph fragment changed")
						}
						seen++
					}
				}
			}
			if seen != len(knots) || created != before {
				t.Fatal("lost knots or reshaped")
			}
			items, err := layout.Unbox(r.VList)
			if err != nil {
				t.Fatal(err)
			}
			seen = 0
			for i := 0; i < items.Len(); i++ {
				item, _ := items.Item(i)
				frozen, line := item.Horizontal()
				old := r.VList.Lines()[i]
				if frozen != r.Frozen || line != old.Horizontal || item.Box().Extent != old.Height || item.Box().Baseline != old.Baseline-old.Top {
					t.Fatal("unboxing changed real-font geometry")
				}
				for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
					knot, _ := frozen.Knot(k)
					text, _ := frozen.CopyText(k)
					if knot != knots[k] || !reflect.DeepEqual(text, texts[k]) {
						t.Fatal("unboxing changed glyphs, features or original sources")
					}
					seen++
				}
			}
			if seen != len(knots) || created != before {
				t.Fatal("unboxing lost knots or reshaped")
			}
		}
	}
}
