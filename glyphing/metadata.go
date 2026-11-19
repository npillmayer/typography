package glyphing

import (
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

// Itemization combines prepared styled text, selected font bindings and shaping
// metadata. Metadata slices are caller-owned; text retains
// private style identities needed by subsequent resolution.
type Itemization struct {
	Text     styled.Text
	Bindings []FontTextRun
	Shaping  []ShapingSpan
}

// ItemizeWithMetadata exposes coalesced shaping metadata. The caller retains
// document paint separately using style.ProjectPaint, and
// supplies paragraph direction/isolates to bidi construction separately.
func (r *StyleMapper) ItemizeWithMetadata(text string, spans []style.Span) (*Itemization, error) {
	out, bindings, err := r.Itemize(text, spans)
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

	return result, nil
}
