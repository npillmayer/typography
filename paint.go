package typography

import (
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography/style"
	"reflect"
)

// PaintPolicy selects shaping compatibility, not a change to document equality.
type PaintPolicy string

const (
	// GlyphBoundary coalesces equivalent typography across paint changes.
	GlyphBoundary       PaintPolicy = "glyph-boundary-v1"
	// LegacyStyleBoundary preserves document style boundaries for old replay.
	LegacyStyleBoundary PaintPolicy = "style-boundary-v1"
)

// CoalesceStyles resolves adjacent spans before grouping them for shaping.
// Paint is retained separately by callers. Bidi direction/isolation metadata is
// still significant; actual bidi intersections are performed by cords/khipu.
// Features are not part of document Style. Feature-bearing spans keep their
// boundaries so their original scalar coordinate origins remain intact.
func CoalesceStyles(text string, spans []style.Span, resolver contracts.StyleResolver, defaults contracts.ResolvedRunStyle) ([]style.Span, error) {
	if err := style.ValidateSpans(text, spans); err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}
	out := make([]style.Span, 0, len(spans))
	var previous contracts.ResolvedRunStyle
	for _, span := range spans {
		resolved, err := resolver.ResolveStyle(span.Style, defaults)
		if err != nil {
			return nil, err
		}
		// Compare opaque font identity without traversing backend state.
		equivalent := len(out) > 0 && sameShaping(previous, resolved)
		if equivalent {
			last := &out[len(out)-1]
			equivalent = last.Style.Direction == span.Style.Direction && last.Style.UnicodeBidi == span.Style.UnicodeBidi
			if equivalent {
				last.End = span.End
				continue
			}
		}
		if len(resolved.Features) == 0 {
			span.Style.Color = ""
		}
		out = append(out, span)
		previous = resolved
	}
	return out, nil
}

func sameShaping(a, b contracts.ResolvedRunStyle) bool {
	// Non-comparable faces and feature-bearing spans conservatively retain their
	// boundaries. Equal relative feature ranges at different starts are not the
	// same feature domain; cords/khipu must retain and rebase those original spans.
	if len(a.Features) > 0 || len(b.Features) > 0 {
		return false
	}
	if a.Font != nil || b.Font != nil {
		if a.Font == nil || b.Font == nil || reflect.TypeOf(a.Font) != reflect.TypeOf(b.Font) || !reflect.TypeOf(a.Font).Comparable() {
			return false
		}
		if a.Font != b.Font {
			return false
		}
	}
	a.Font = nil
	b.Font = nil
	return reflect.DeepEqual(a, b)
}
