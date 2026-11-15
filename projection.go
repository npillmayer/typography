package typography

import (
	"context"
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/style"
	"golang.org/x/text/language"
)

// ShapingKey records resolved typography before bidi/style intersection. It is
// not permission to merge runs: bidi context and feature origins still matter.
// Features is detached and uses scalar coordinates relative to its original run.
type ShapingKey struct {
	ResourceID string
	EmSize     dimen.DU
	Script     language.Script
	Language   language.Tag
	Features   []contracts.ShapeFeature
	Spacing    contracts.SpacingMode
}
// ShapingSpan associates a resolved key with a half-open processed UTF-8 byte
// range. Start is also the origin of the key's scalar feature domain.
type ShapingSpan struct {
	Start, End uint64 // processed UTF-8 bytes; Start is the feature coordinate origin
	Key        ShapingKey
}
// PaintSpan retains document paint over a half-open processed UTF-8 byte range,
// independently of coalesced shaping boundaries.
type PaintSpan struct {
	Start, End uint64
	Paint      style.Paint
}
// Itemization combines prepared styled text, selected font bindings, shaping
// metadata and original paint spans. Metadata slices are caller-owned; text retains
// private style identities needed by subsequent resolution.
type Itemization struct {
	Text     styled.Text
	Bindings []Binding
	Shaping  []ShapingSpan
	Paint    []PaintSpan
}

// ItemizeWithMetadata exposes coalesced shaping and unchanged document paint
// spans. Plain input has no document paint spans. The caller
// supplies paragraph direction/isolates to bidi construction separately.
func (r *Resolver) ItemizeWithMetadata(ctx context.Context, text string, spans []style.Span) (*Itemization, error) {
	out, bindings, err := r.Itemize(ctx, text, spans)
	if err != nil {
		return nil, err
	}
	result := &Itemization{Text: out, Bindings: bindings}
	for _, b := range bindings {
		v, _, err := out.StyleAt(b.Start)
		if err != nil {
			return nil, err
		}
		resolved, err := r.ResolveStyle(v, r.Defaults())
		if err != nil {
			return nil, err
		}
		result.Shaping = append(result.Shaping, ShapingSpan{Start: b.Start, End: b.End, Key: ShapingKey{ResourceID: b.ResourceID, EmSize: resolved.EmSize, Script: resolved.Script, Language: resolved.Language, Features: resolved.Features, Spacing: resolved.Spacing}})
	}
	for _, s := range spans {
		result.Paint = append(result.Paint, PaintSpan{Start: s.Start, End: s.End, Paint: s.Style.Paint()})
	}
	return result, nil
}
