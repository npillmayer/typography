package paragraph

import (
	"cmp"
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography/layout"
)

// FittedAtomic certifies the accepted box dimensions/margins alongside exactly
// their local frozen placement. Rendering identities/interiors remain the
// client's responsibility; this package owns no application box trees.
type FittedAtomic struct {
	Box       AtomicBox
	Placement layout.AtomicPlacement
}

type CoverageKind uint8

const (
	GlyphCoverage CoverageKind = iota + 1
	SpaceCoverage
	AtomicCoverage
	// ReplacementCoverage covers one rendered pre/post-break half's original
	// source partition. Its generated clusters have replacement-local coordinates;
	// no per-character correspondence to original source is implied.
	ReplacementCoverage
)

// SourceCoverage identifies retained material in Input.Text coordinates, which
// may be processed text in the caller. Line is fragment-local. Glyph records
// cover whole indivisible shaping clusters, including ligatures; an overlapping
// nonempty source range belongs to the fragment containing that cluster, even
// when its boundary is inside the cluster. Never infer ownership from glyph or
// bidi visual order. Records are sorted in logical source order.
// Replacement records cover a whole original source partition indivisibly,
// independently of inserted, changed or removed bytes in the generated spelling.
//
// Retained spaces and atomic anchors are distinguished. Trimmed spaces, generated
// fill and glyphless clusters provide no rendered coverage; logical consumption
// remains in PrefixInfo.From/To. Empty-marker following/preceding affinity and
// original HTML-to-processed mappings stay with the client, without invented
// glyph positions or marker material here.
type SourceCoverage struct {
	Source khipu.ByteRange
	Line   int
	Kind   CoverageKind
	Atomic contracts.AtomicID
}

func prefixCoverage(f *layout.Fragment) []SourceCoverage {
	var out []SourceCoverage
	owner := f.Frozen()
	for _, line := range f.Lines() {
		for i := line.Horizontal.FirstKnot; i < line.Horizontal.FirstKnot+line.Horizontal.KnotCount; i++ {
			knot, _ := owner.Knot(i)
			if id, ok := owner.Atomic(i); ok {
				out = append(out, SourceCoverage{Source: knot.Source.Bytes, Line: line.Index, Kind: AtomicCoverage, Atomic: id})
			} else if text, ok := owner.CopyText(i); ok {
				out = appendTextCoverage(out, knot, text, line.Index)
			} else if knot.Kind == khipu.KTGlue && knot.Material == khipu.OriginalMaterial && knot.Source.Bytes.Start < knot.Source.Bytes.End {
				out = append(out, SourceCoverage{Source: knot.Source.Bytes, Line: line.Index, Kind: SpaceCoverage})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b SourceCoverage) int {
		if c := cmp.Compare(a.Source.Start, b.Source.Start); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Source.End, b.Source.End); c != 0 {
			return c
		}
		return cmp.Compare(a.Line, b.Line)
	})
	return out
}

func appendTextCoverage(out []SourceCoverage, knot khipu.FrozenKnot, text khipu.FrozenText, line int) []SourceCoverage {
	if text.IsReplacement {
		// Source provenance partitions the original textbox, even when spelling
		// shrinks, grows or is empty. Never treat replacement-local cluster bytes
		// as paragraph offsets. Silent material still advances logical positions,
		// but has no rendered coverage for activating a visible owning range.
		if len(text.Glyphs) > 0 && knot.Source.Bytes.Start < knot.Source.Bytes.End {
			out = append(out, SourceCoverage{Source: knot.Source.Bytes, Line: line, Kind: ReplacementCoverage})
		}
		return out
	}
	for _, cluster := range text.Clusters {
		if cluster.GlyphCount > 0 && cluster.Input.Bytes.Start < cluster.Input.Bytes.End {
			out = append(out, SourceCoverage{Source: cluster.Input.Bytes, Line: line, Kind: GlyphCoverage})
		}
	}
	return out
}
