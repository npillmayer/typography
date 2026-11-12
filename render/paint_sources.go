package render

import (
	"fmt"
	"github.com/npillmayer/khipu"
	"strings"
)

// PaintSources maps logical glyph anchors to original processed-text bytes.
// Repeated anchors deliberately share paint. The only supported generated-text
// policy here is explicit U+00AD -> '-' on a selected break; other source edits
// need their own mapping, not inferred glyph ownership.
func PaintSources(text string, knot khipu.FrozenKnot, snapshot khipu.FrozenText) (map[uint32]uint64, error) {
	var offsets []uint64
	if !snapshot.IsReplacement {
		for pos := range text {
			offsets = append(offsets, uint64(pos))
		}
	} else {
		a, b := knot.Source.Bytes.Start, knot.Source.Bytes.End
		if a > b || b > uint64(len(text)) {
			return nil, fmt.Errorf("invalid replacement source range")
		}
		source := text[a:b]
		last := strings.LastIndex(source, "\u00ad")
		var reconstructed strings.Builder
		for pos, c := range source {
			if c == '\u00ad' {
				if knot.Material != khipu.PreBreakMaterial || pos != last {
					continue
				}
				c = '-'
			}
			reconstructed.WriteRune(c)
			offsets = append(offsets, a+uint64(pos))
		}
		if reconstructed.String() != snapshot.ReplacementText {
			return nil, fmt.Errorf("unsupported generated-text paint mapping")
		}
	}
	out := make(map[uint32]uint64)
	for _, g := range snapshot.Glyphs {
		if uint64(g.Cluster) >= uint64(len(offsets)) {
			return nil, fmt.Errorf("glyph anchor %d outside paint source", g.Cluster)
		}
		out[g.Cluster] = offsets[g.Cluster]
	}
	return out, nil
}
