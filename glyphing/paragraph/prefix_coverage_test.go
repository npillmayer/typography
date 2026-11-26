package paragraph

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"testing"
)

func TestReplacementCoverageUsesOriginalPartition(t *testing.T) {
	// Replacement bytes/clusters can be unrelated to original source; including
	// empty spelling or nonempty spelling with zero output glyphs is deliberate.
	for _, tc := range []struct {
		text   string
		glyphs int
	}{{"inserted-long-text", 2}, {"x", 1}, {"", 0}, {"silent", 0}} {
		k := khipu.FrozenKnot{Source: khipu.SourceRange{Bytes: khipu.ByteRange{Start: 17, End: 21}}, Material: khipu.PostBreakMaterial}
		x := khipu.FrozenText{IsReplacement: true, ReplacementText: tc.text, Glyphs: make([]contracts.ShapedGlyph, tc.glyphs), Clusters: []khipu.FrozenCluster{{Input: khipu.SourceRange{Bytes: khipu.ByteRange{End: 1}}, GlyphCount: tc.glyphs}}}
		got := appendTextCoverage(nil, k, x, 3)
		if tc.glyphs == 0 {
			if len(got) != 0 {
				t.Fatal("silent output invented coverage", got)
			}
			continue
		}
		if len(got) != 1 || got[0] != (SourceCoverage{Source: k.Source.Bytes, Line: 3, Kind: ReplacementCoverage}) {
			t.Fatal(got)
		}
	}
}
