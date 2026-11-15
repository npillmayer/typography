package typography

import (
	"context"
	"fmt"
	"strings"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/fontresource"
	typestyle "github.com/npillmayer/typography/style"
	"golang.org/x/text/language"
)

// Binding records the chosen face and exact processed-text byte range.
type Binding struct {
	Start, End   uint64
	Path, SHA256 string
	ResourceID   string
	Origin       fontresource.Origin
	EmSize       dimen.DU
	Script       string
}

// Resolver maps document styles and itemizes text using explicitly loaded faces.
// Construct it with NewResolver and serialize operations sharing a resolver.
// Injected services and retained faces must remain valid for all retained results.
type Resolver struct {
	selected    SelectedFonts
	coverage    Coverage
	faces       map[string]FaceEntry
	infos       []Info
	defaultID   string
	ruleIDs     []string
	fallbackIDs []string
}

// Infos returns detached identity records, including detached warning slices,
// in default/rule/fallback reference order with duplicate resource identities removed.
func (r *Resolver) Infos() []Info {
	infos := make([]Info, len(r.infos))
	for i, v := range r.infos {
		infos[i] = v
		infos[i].Warnings = append([]string(nil), v.Warnings...)
	}
	return infos
}

// Defaults returns the selected default face and size with English language,
// Latin script and elastic spacing. The face is retained, not cloned.
func (r *Resolver) Defaults() contracts.ResolvedRunStyle {
	return contracts.ResolvedRunStyle{Font: r.faces[r.defaultID].Face, EmSize: r.selected.Size, Language: language.English, Script: language.MustParseScript("Latn"), Spacing: contracts.SpacingElastic}
}
func familyMatches(list, want string) bool {
	for _, family := range strings.Split(list, ",") {
		if strings.EqualFold(strings.Trim(strings.TrimSpace(family), "\"'"), want) {
			return true
		}
	}
	return false
}

// ResolveStyle accepts projected TypographyStyle, legacy document style.Style,
// and private styles returned by itemization. Nil preserves defaults. Document
// paint is ignored; other style implementations are rejected. Feature slices are
// detached, and itemized feature windows clip/rebase defaults without losing their
// original scalar domains. Keep itemized style values intact when passing them on.
func (r *Resolver) ResolveStyle(value styled.Style, defaults contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if item, ok := value.(runStyle); ok {
		var css styled.Style
		if !item.Plain {
			css = item.Input
		}
		result, err := r.ResolveStyle(css, defaults)
		if err != nil {
			return result, err
		}
		entry, exists := r.faces[item.ResourceID]
		if !exists {
			return result, fmt.Errorf("unknown itemized font %q", item.ResourceID)
		}
		result.Font = entry.Face
		result.Script = language.MustParseScript(item.Script)
		if item.features.active {
			result.Features, err = item.features.intersect(result.Features)
			if err != nil {
				return result, err
			}
		}
		return result, nil
	}
	result := defaults
	result.Features = append([]contracts.ShapeFeature(nil), defaults.Features...)
	if value == nil {
		return result, nil
	}
	var css typestyle.Typography
	switch v := value.(type) {
	case typestyle.Style: // Compatibility input; document paint is ignored.
		css = v.Typography()
	case TypographyStyle:
		css = v.Typography
	default:
		return result, fmt.Errorf("unknown style %T", value)
	}
	path := r.styleID(css)
	result.Font = r.faces[path].Face
	result.EmSize = css.EmSize
	var err error
	result.Language, err = language.Parse(css.Language)
	if err != nil {
		return result, err
	}
	result.Script, _ = result.Language.Script()
	result.Spacing = contracts.SpacingElastic
	if css.WhiteSpace == "pre-wrap" {
		result.Spacing = contracts.SpacingFixed
	}
	return result, nil
}

// CheckText validates itemization and coverage without retaining the styled text.
func (r *Resolver) CheckText(ctx context.Context, text string, spans []typestyle.Span) ([]Binding, error) {
	_, bindings, err := r.Itemize(ctx, text, spans)
	return bindings, err
}

func (r *Resolver) styleID(css typestyle.Typography) string {
	path := r.defaultID
	for i, rule := range r.selected.Rules {
		if (rule.Family == "" || familyMatches(css.Family, rule.Family)) && (rule.Weight == 0 || rule.Weight == css.Weight) && (rule.Slant == "" || rule.Slant == css.Slant) {
			path = r.ruleIDs[i]
			break
		}
	}
	return path
}
