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

// Settings is a detached request. Rules use first-match order; zero/empty fields
// are wildcards. Select exactly one default file or immutable resource.
// Unmatched styles use that default, never synthetic. Path fallbacks precede resources.
type Settings struct {
	DefaultPath       string
	DefaultResource   *fontresource.Resource
	FallbackResources []*fontresource.Resource
	Size              dimen.DU
	Rules             []Rule
	Fallbacks         []string
}
// Rule selects a font for the first matching family, CSS weight and slant.
// Empty family/slant and zero weight are wildcards. Select exactly one Path or
// Resource; family matching is case-insensitive against the style family list.
type Rule struct {
	Family      string
	Weight      int
	Slant, Path string
	Resource    *fontresource.Resource
}

// Clone copies the settings and their slices, sharing immutable resources.
// A nil receiver returns nil.
func (s *Settings) Clone() *Settings {
	if s == nil {
		return nil
	}
	c := *s
	c.FallbackResources = append([]*fontresource.Resource(nil), s.FallbackResources...)
	c.Fallbacks = append([]string(nil), s.Fallbacks...)
	c.Rules = append([]Rule(nil), s.Rules...)
	return &c
}

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
// Construct it with LoadSettings and serialize operations sharing a resolver.
// Injected services and retained faces must remain valid for all retained results.
type Resolver struct {
	settings    Settings
	faces       map[string]FaceEntry
	services    Services
	infos       []Info
	defaultID   string
	ruleIDs     []string
	fallbackIDs []string
}

// LoadSettings loads all configured selections through the required Loader and
// Coverage services; it installs no backend or discovery fallback. Size must be
// positive and below dimen.Fil. Each selection must specify exactly one path or
// resource. Retained resource identities are validated and substitution is rejected.
// Settings slices are copied; resources and services remain shared. A nil context,
// cancellation or any load/validation failure returns no resolver.
func LoadSettings(ctx context.Context, s Settings, services Services) (*Resolver, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil context")
	}
	if s.Size <= 0 || s.Size >= dimen.Fil {
		return nil, fmt.Errorf("font size must be positive and finite")
	}
	if services.Loader == nil || services.Coverage == nil {
		return nil, fmt.Errorf("font loader and coverage services are required")
	}
	r := &Resolver{settings: *s.Clone(), faces: make(map[string]FaceEntry), services: services}
	load := func(path string, resource *fontresource.Resource) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if (strings.TrimSpace(path) == "") == (resource == nil) {
			return "", fmt.Errorf("select exactly one font file or resource")
		}
		entry, err := services.Loader.Load(ctx, path, resource)
		if err != nil {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if entry.Face == nil || entry.Resource == nil {
			return "", fmt.Errorf("font loader returned incomplete entry")
		}
		if entry.Info.ResourceID != entry.Resource.ID() || entry.Info.SHA256 != entry.Resource.SHA256() || entry.Info.Origin != entry.Resource.Origin() || entry.Info.Path != entry.Info.Origin.Location {
			return "", fmt.Errorf("font loader identity does not match retained resource")
		}
		if resource != nil && resource.ID() != entry.Info.ResourceID {
			return "", fmt.Errorf("font loader substituted the selected resource")
		}
		entry.Info.Warnings = append([]string(nil), entry.Info.Warnings...)
		id := entry.Info.ResourceID
		if _, exists := r.faces[id]; !exists {
			r.faces[id] = entry
			r.infos = append(r.infos, entry.Info)
		}
		return id, nil
	}
	var err error
	r.defaultID, err = load(s.DefaultPath, s.DefaultResource)
	if err != nil {
		return nil, err
	}
	for i, rule := range s.Rules {
		if rule.Weight < 0 || rule.Weight > 1000 || (rule.Slant != "" && rule.Slant != "normal" && rule.Slant != "italic" && rule.Slant != "oblique") {
			return nil, fmt.Errorf("invalid font mapping row %d", i+1)
		}
		id, err := load(rule.Path, rule.Resource)
		if err != nil {
			return nil, fmt.Errorf("font mapping row %d: %w", i+1, err)
		}
		r.ruleIDs = append(r.ruleIDs, id)
	}
	for _, path := range s.Fallbacks {
		id, err := load(path, nil)
		if err != nil {
			return nil, err
		}
		r.fallbackIDs = append(r.fallbackIDs, id)
	}
	for _, resource := range s.FallbackResources {
		id, err := load("", resource)
		if err != nil {
			return nil, err
		}
		r.fallbackIDs = append(r.fallbackIDs, id)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}
// Infos returns detached identity records, including detached warning slices,
// in first-load order with duplicate resource identities removed.
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
	return contracts.ResolvedRunStyle{Font: r.faces[r.defaultID].Face, EmSize: r.settings.Size, Language: language.English, Script: language.MustParseScript("Latn"), Spacing: contracts.SpacingElastic}
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

// Snapshot pins every configured face, including unused rules/fallbacks. It never
// reopens original paths and preserves provenance for offline replay.
func (r *Resolver) Snapshot() *Settings {
	s := r.settings.Clone()
	resource := func(id string) *fontresource.Resource { return r.faces[id].Resource }
	s.DefaultPath = ""
	s.DefaultResource = resource(r.defaultID)
	for i := range s.Rules {
		s.Rules[i].Path = ""
		s.Rules[i].Resource = resource(r.ruleIDs[i])
	}
	s.Fallbacks = nil
	s.FallbackResources = nil
	for _, id := range r.fallbackIDs {
		s.FallbackResources = append(s.FallbackResources, resource(id))
	}
	return s
}

func (r *Resolver) styleID(css typestyle.Typography) string {
	path := r.defaultID
	for i, rule := range r.settings.Rules {
		if (rule.Family == "" || familyMatches(css.Family, rule.Family)) && (rule.Weight == 0 || rule.Weight == css.Weight) && (rule.Slant == "" || rule.Slant == css.Slant) {
			path = r.ruleIDs[i]
			break
		}
	}
	return path
}
