package opentype

import (
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/typography/glyphing"
	"github.com/npillmayer/typography/style"
	"github.com/npillmayer/uax/bidi"
)

func TestItemizeInlineFeatureLigatureAndSourceCoordinates(t *testing.T) {
	entry, err := (Services{}).Load(t.Context(), progressFont, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{Faces: []glyphing.FaceEntry{entry}, Default: entry.Info.ResourceID, Size: 12 * dimen.BP}, Services{})
	if err != nil {
		t.Fatal(err)
	}
	// The test font's "test" feature forms fi. Only the second pair falls in
	// [3,5): the object occupies scalar 2 even though it is never shaped.
	defaults := r.Defaults()
	defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Enabled: true, Start: 3, End: 5}}
	anchors := []contracts.AtomicAnchor{{ID: 13, ByteOffset: 2}}

	for _, explicit := range []bool{false, true} {
		var spans []style.Span
		if explicit {
			spans = []style.Span{{Start: 0, End: 7, Style: style.Style{EmSize: defaults.EmSize, Language: "en"}}}
		}
		text, _, err := r.ItemizeInline("fi\uFFFCfi", spans, anchors)
		if err != nil {
			t.Fatal(err)
		}
		para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
		if err != nil {
			t.Fatal(err)
		}
		owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{Resolver: r, Defaults: defaults, NewShaper: func() (contracts.StreamingShaper, error) { return NewStream(8) }}})
		prepared, discovery, err := owner.PrepareParagraph(khipu.ParagraphInput{Paragraph: para, Atomics: anchors}, khipu.PrepareOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, engine := range []string{"firstfit", "knuthplass"} {
			knots, err := khipu.BindParagraph(prepared, []khipu.AtomicAdvance{{ID: 13, Advance: 4 * dimen.BP}})
			if err != nil {
				t.Fatal(err)
			}
			layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.BP))
			request := khipu.BreakRequest{Binding: knots, Discovery: discovery, Geometry: khipu.GeometryRequest{Shape: layout.LineShape, LeftSkip: layout.LeftSkip, RightSkip: layout.RightSkip}}
			var plan *khipu.Tikray
			if engine == "firstfit" {
				plan, err = firstfit.BreakParagraph(request, nil)
			} else {
				plan, err = knuthplass.BreakParagraph(request, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			frozen, err := khipu.Freeze(plan)
			if err != nil {
				t.Fatal(err)
			}
			var ids, clusters []uint32
			objects := 0
			for i := 0; i < frozen.KnotCount(); i++ {
				if id, ok := frozen.Atomic(i); ok {
					objects++
					knot, _ := frozen.Knot(i)
					if id != 13 || knot.Source.Bytes != (khipu.ByteRange{Start: 2, End: 5}) || knot.Source.ScalarStart != 2 {
						t.Fatal("object coordinates changed", id, knot)
					}
				}
				if tx, ok := frozen.CopyText(i); ok {
					for _, g := range tx.Glyphs {
						ids, clusters = append(ids, g.GlyphID), append(clusters, g.Cluster)
					}
				}
			}
			if objects != 1 || !slices.Equal(ids, []uint32{4, 5, 6}) || !slices.Equal(clusters, []uint32{0, 1, 3}) {
				t.Fatalf("%s %s explicit=%v: objects=%d glyphs=%v clusters=%v", "glyph-boundary", engine, explicit, objects, ids, clusters)
			}
		}
	}

	if defaults.Features[0].Start != 3 || defaults.Features[0].End != 5 {
		t.Fatal("feature defaults mutated")
	}
}
