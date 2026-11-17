package opentype

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

func TestPreparedInlineFeatureDomainsAndReusableLayout(t *testing.T) {
	entry, err := (Services{}).Load(t.Context(), progressFont, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := glyphing.NewResolver(t.Context(), glyphing.SelectedFonts{Faces: []glyphing.FaceEntry{entry}, Default: entry.Info.ResourceID, Size: 12 * dimen.BP}, Services{})
	if err != nil {
		t.Fatal(err)
	}

	anchors := []contracts.AtomicAnchor{{ID: 42, ByteOffset: 2}}
	text, _, err := r.ItemizeInline(t.Context(), "fi\uFFFCfi", nil, anchors)
	if err != nil {
		t.Fatal(err)
	}
	created := 0
	cfg := khipu.Config{Streaming: &khipu.StreamingConfig{Resolver: r, Defaults: r.Defaults(), NewShaper: func() (contracts.StreamingShaper, error) { created++; return NewStream(8) }}}
	cfg.Streaming.Defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Start: 3, End: 5, Enabled: true}}
	p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: text, Atomics: anchors}, khipu.PrepareOptions{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Assembly must use the captured defaults as well as shaping. Mutation
	// of the caller's value envelope after Prepare must affect neither.
	cfg.Streaming.Defaults.Features[0].Start = 99
	cfg.Streaming.Defaults.Font = nil
	anchors[0].ID = 99
	measurement := khipu.MeasureRequest{Atomics: []khipu.AtomicIntrinsic{{ID: 42, MinAdvance: 4 * dimen.BP, MaxAdvance: 4 * dimen.BP}}}
	widths, err := p.MeasureIntrinsic(t.Context(), measurement)
	if err != nil {
		t.Fatal(err)
	}
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, width := range []dimen.DU{100 * dimen.BP, 0} {
			out, err := p.Layout(t.Context(), paragraph.LayoutRequest{
				Params:     paragraph.Params{Constraints: paragraph.Constraints{Width: width}, Breaker: breaker, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.NormalHeight}}},
				Horizontal: &paragraph.HorizontalLayout{},
				Atomics:    []paragraph.AtomicBox{{ID: 42, BorderWidth: 4 * dimen.BP, BorderHeight: 10 * dimen.BP, Alignment: paragraph.AlignBottomMarginEdge}},
			})
			if err != nil {
				t.Fatal(err)
			}
			var ids, clusters []uint32
			for i := 0; i < out.Frozen.KnotCount(); i++ {
				if tx, ok := out.Frozen.CopyText(i); ok {
					for _, g := range tx.Glyphs {
						ids = append(ids, g.GlyphID)
						clusters = append(clusters, g.Cluster)
					}
				}
			}
			if !slices.Equal(ids, []uint32{4, 5, 6}) || !slices.Equal(clusters, []uint32{0, 1, 3}) {
				t.Fatal("private feature domains or source anchors lost", ids, clusters)
			}
			placements := out.VList.AtomicPlacements()
			if len(placements) != 1 || placements[0].ID != 42 {
				t.Fatal("atomic occurrence lost", placements)
			}
			if got, err := p.MeasureIntrinsic(t.Context(), measurement); err != nil || got != widths {
				t.Fatal("measurement changed after layout", got, widths, err)
			}
		}
	}
	if created != 1 {
		t.Fatal("base paragraph reshaped", created)
	}

}
