package glyphing

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
)

// FaceEntry binds an opaque khipu face to its immutable resource and identity.
// The face must represent exactly the retained resource bytes. Coverage and faces
// must remain safe for their intended use for the resolver/result lifetime.
type FaceEntry struct {
	Face     contracts.FontFace
	Info     fontmeta.Info
	Resource *fontresource.Resource
}

// SelectedFonts is a closed set of loaded faces with explicit selection roles.
// References are resource IDs, never paths. Faces contains exactly one entry per
// referenced ID, including rules/fallbacks unused by the current text. Table order
// does not imply fallback order. Faces/resources remain stable for result lifetimes.
type SelectedFonts struct {
	Faces     []FaceEntry
	Default   string
	Rules     []SelectedRule
	Fallbacks []string
	Size      dimen.DU
}

// SelectedRule selects a retained resource for the first matching document style.
// Empty family/slant and zero weight are wildcards; Weight is a CSS numeric weight.
type SelectedRule struct {
	Family     string
	Weight     int
	Slant      string
	ResourceID string
}

// Clone detaches selection and warning slices while sharing immutable resources
// and opaque faces. A nil receiver returns nil.
func (s *SelectedFonts) Clone() *SelectedFonts {
	if s == nil {
		return nil
	}
	c := *s
	c.Faces = slices.Clone(s.Faces)
	c.Rules = slices.Clone(s.Rules)
	c.Fallbacks = slices.Clone(s.Fallbacks)
	for i := range c.Faces {
		c.Faces[i].Info.Warnings = slices.Clone(c.Faces[i].Info.Warnings)
	}
	return &c
}

// NewResolver validates already loaded fonts without loading, discovery, shaping
// or coverage queries. It canonicalizes faces in default/rule/fallback order,
// copies mutable metadata, and checks coverage acceptance for every selected face.
// Acceptance checks perform no glyph queries. Only explicit
// fallback references are eligible. Serialize operations sharing this resolver.
func NewResolver(ctx context.Context, fonts SelectedFonts, coverage Coverage) (*Resolver, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if fonts.Size <= 0 || fonts.Size >= dimen.Fil {
		return nil, fmt.Errorf("font size must be positive and finite")
	}
	if nilFontService(coverage) {
		return nil, fmt.Errorf("font coverage service is required")
	}
	r := &Resolver{selected: SelectedFonts{Default: fonts.Default, Size: fonts.Size, Rules: slices.Clone(fonts.Rules), Fallbacks: slices.Clone(fonts.Fallbacks)}, faces: make(map[string]FaceEntry), coverage: coverage, defaultID: fonts.Default}
	for _, entry := range fonts.Faces {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if nilFontService(entry.Face) || entry.Resource == nil {
			return nil, fmt.Errorf("incomplete selected font entry")
		}
		if entry.Info.ResourceID != entry.Resource.ID() || entry.Info.SHA256 != entry.Resource.SHA256() || entry.Info.Origin != entry.Resource.Origin() || entry.Info.Path != entry.Info.Origin.Location {
			return nil, fmt.Errorf("selected font identity does not match retained resource")
		}
		if _, exists := r.faces[entry.Info.ResourceID]; exists {
			return nil, fmt.Errorf("duplicate selected resource ID %q", entry.Info.ResourceID)
		}
		entry.Info.Warnings = slices.Clone(entry.Info.Warnings)
		r.faces[entry.Info.ResourceID] = entry
	}
	seen := map[string]bool{}
	use := func(id string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, ok := r.faces[id]
		if !ok {
			return fmt.Errorf("unresolved selected font %q", id)
		}
		if !seen[id] {
			seen[id] = true
			r.selected.Faces = append(r.selected.Faces, entry)
			r.infos = append(r.infos, entry.Info)
		}
		return nil
	}
	if err := use(fonts.Default); err != nil {
		return nil, err
	}
	for i, rule := range fonts.Rules {
		if rule.Weight < 0 || rule.Weight > 1000 || (rule.Slant != "" && rule.Slant != "normal" && rule.Slant != "italic" && rule.Slant != "oblique") {
			return nil, fmt.Errorf("invalid font mapping row %d", i+1)
		}
		if err := use(rule.ResourceID); err != nil {
			return nil, fmt.Errorf("font mapping row %d: %w", i+1, err)
		}
		r.ruleIDs = append(r.ruleIDs, rule.ResourceID)
	}
	for _, id := range fonts.Fallbacks {
		if err := use(id); err != nil {
			return nil, err
		}
		r.fallbackIDs = append(r.fallbackIDs, id)
	}
	if len(seen) != len(r.faces) {
		return nil, fmt.Errorf("unreferenced selected font entry")
	}
	for _, entry := range r.selected.Faces {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := coverage.CheckFace(entry.Face); err != nil {
			return nil, fmt.Errorf("coverage for selected resource %q: %w", entry.Info.ResourceID, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

func nilFontService(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

// Snapshot returns all configured roles and faces in canonical reference order.
// Metadata slices are detached; stable faces/resources are shared, never reloaded.
func (r *Resolver) Snapshot() *SelectedFonts { return r.selected.Clone() }
