package typography

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography/style"
)

// TypographyStyle is M's projection of a document style for the typesetter.
// It contains no paint. The private domain preserves relative feature origins
// and, only for legacy replay, original document-style boundaries.
type TypographyStyle struct {
	Typography style.Typography
	domain     uint64
}

func (s TypographyStyle) Equals(other styled.Style) bool {
	o, ok := other.(TypographyStyle)
	return ok && s == o
}
func (s TypographyStyle) String() string { return fmt.Sprintf("%+v domain=%d", s.Typography, s.domain) }

// TypographySpan addresses processed paragraph UTF-8 bytes, like style.Span.
type TypographySpan struct {
	Start, End uint64
	Style      TypographyStyle
}

// ProjectStyles defines which document values reach the typesetter. Callers keep
// rendering properties separately. Nil spans preserve unstyled/default input.
// Projection owns both coalescing and required run boundaries; callers must use
// the returned Style values, not rebuild them from their Typography fields.
func ProjectStyles(text string, spans []style.Span, resolver contracts.StyleResolver, defaults contracts.ResolvedRunStyle, policy PaintPolicy) ([]TypographySpan, error) {
	if err := style.ValidateSpans(text, spans); err != nil {
		return nil, err
	}
	if policy != "" && policy != GlyphBoundary && policy != LegacyStyleBoundary {
		return nil, fmt.Errorf("unknown paint policy %q", policy)
	}
	if spans == nil {
		return nil, nil
	}
	if resolver == nil {
		return nil, fmt.Errorf("style projection requires a resolver")
	}
	legacy := policy == LegacyStyleBoundary
	if !legacy {
		var err error
		spans, err = CoalesceStyles(text, spans, resolver, defaults)
		if err != nil {
			return nil, err
		}
	}
	out := make([]TypographySpan, 0, len(spans))
	var domain uint64
	for i, span := range spans {
		resolved, err := resolver.ResolveStyle(span.Style, defaults)
		if err != nil {
			return nil, err
		}
		if legacy {
			// Identical adjacent source styles historically shared a run.
			if i > 0 && spans[i-1].Style != span.Style {
				domain = span.Start + 1
			}
		} else if len(resolved.Features) > 0 {
			domain = span.Start + 1
		} else {
			domain = 0
		}
		out = append(out, TypographySpan{span.Start, span.End, TypographyStyle{span.Style.Typography(), domain}})
	}
	return out, nil
}
