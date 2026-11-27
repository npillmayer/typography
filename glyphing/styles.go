package glyphing

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography/style"
)

// TypographyStyle adapts paint-free document typography to styled.Style.
// The private domain preserves relative feature origins across shaping fragments.
// Keep projected values intact when storing or passing styled text.
type TypographyStyle struct {
	Typography style.Typography
	domain     uint64
}

// Equals compares typography and the private run domain, including boundaries
// needed to preserve feature origins.
func (s TypographyStyle) Equals(other styled.Style) bool {
	o, ok := other.(TypographyStyle)
	return ok && s == o
}

// String returns diagnostic typography and run-domain information.
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
func ProjectStyles(text string, spans []style.Span, resolver contracts.StyleResolver, defaults contracts.ResolvedRunStyle) ([]TypographySpan, error) {
	if err := style.ValidateSpans(text, spans); err != nil {
		return nil, err
	}

	if spans == nil {
		return nil, nil
	}
	if resolver == nil {
		return nil, fmt.Errorf("style projection requires a resolver")
	}

	var err error
	spans, err = CoalesceStyles(text, spans, resolver, defaults)
	if err != nil {
		return nil, err
	}

	out := make([]TypographySpan, 0, len(spans))
	var domain uint64
	for _, span := range spans {
		resolved, err := resolver.ResolveStyle(span.Style, defaults)
		if err != nil {
			return nil, err
		}
		if len(resolved.Features) > 0 {
			domain = span.Start + 1
		} else {
			domain = 0
		}
		out = append(out, TypographySpan{span.Start, span.End, TypographyStyle{span.Style.Typography(), domain}})
	}
	return out, nil
}
