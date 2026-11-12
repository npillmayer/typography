package render

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"testing"
)

func TestPaintSourcesAnchorsAndDiscretionaries(t *testing.T) {
	glyphs := []contracts.ShapedGlyph{{Cluster: 0}, {Cluster: 0}, {Cluster: 2}}
	m, err := PaintSources("אְב", khipu.FrozenKnot{}, khipu.FrozenText{Glyphs: glyphs})
	if err != nil || m[0] != 0 || m[2] != 4 {
		t.Fatalf("repeated anchor paint: %v %v", m, err)
	}
	knot := khipu.FrozenKnot{Source: khipu.SourceRange{Bytes: khipu.ByteRange{Start: 0, End: 8}}, Material: khipu.PreBreakMaterial}
	snap := khipu.FrozenText{IsReplacement: true, ReplacementText: "éé-", Glyphs: []contracts.ShapedGlyph{{Cluster: 0}, {Cluster: 1}, {Cluster: 2}}}
	m, err = PaintSources("é\u00adé\u00adab", knot, snap)
	if err != nil || m[0] != 0 || m[1] != 4 || m[2] != 6 {
		t.Fatalf("prebreak marker mapping %v %v", m, err)
	}
	knot.Source.Bytes = khipu.ByteRange{Start: 8, End: 10}
	knot.Material = khipu.PostBreakMaterial
	snap.ReplacementText = "ab"
	snap.Glyphs = snap.Glyphs[:2]
	m, err = PaintSources("é\u00adé\u00adab", knot, snap)
	if err != nil || m[0] != 8 || m[1] != 9 {
		t.Fatalf("postbreak mapping %v %v", m, err)
	}
	snap.ReplacementText = "unknown"
	if _, err = PaintSources("é\u00adé\u00adab", knot, snap); err == nil {
		t.Fatal("invented generated-text provenance")
	}
}
