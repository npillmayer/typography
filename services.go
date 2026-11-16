// Package typography resolves shared styles and itemizes text using preselected faces and injected
// coverage. It does not install a parser, discovery provider, or shaper.
package typography

import (
	"fmt"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography/fontresource"
)

// Info is detached identity and parser metadata, independent of a backend.
type Info struct {
	ResourceID                      string
	Origin                          fontresource.Origin
	Path, Family, Subfamily, SHA256 string
	Warnings                        []string
}

// FaceEntry binds an opaque khipu face to its immutable resource and identity.
// The face must represent exactly the retained resource bytes. Coverage and faces
// must remain safe for their intended use for the resolver/result lifetime.
type FaceEntry struct {
	Face     contracts.FontFace
	Info     Info
	Resource *fontresource.Resource
}

// Coverage reports nominal cmap coverage; zero means missing. CheckFace validates
// representation compatibility without queries. Neither check promises successful
// shaping. Failures are returned, never treated as permission to fallback.
type Coverage interface {
	contracts.FontAcceptor
	GlyphIndex(contracts.FontFace, rune) (uint32, error)
}

// CoverageFuncs adapts explicitly paired acceptance and nominal lookup functions.
// Both callbacks are required and must obey Coverage's lifetime/concurrency rules.
type CoverageFuncs struct {
	Check  func(contracts.FontFace) error
	Lookup func(contracts.FontFace, rune) (uint32, error)
}

// CheckFace validates the adapter and delegates the read-only acceptance check.
func (f CoverageFuncs) CheckFace(face contracts.FontFace) error {
	if f.Check == nil || f.Lookup == nil || nilFontService(face) {
		return fmt.Errorf("%w: incomplete coverage functions or nil face", contracts.ErrIllegalArguments)
	}
	return f.Check(face)
}

// GlyphIndex defensively checks acceptance before looking up nominal coverage.
func (f CoverageFuncs) GlyphIndex(face contracts.FontFace, r rune) (uint32, error) {
	if err := f.CheckFace(face); err != nil {
		return 0, err
	}
	return f.Lookup(face, r)
}

var _ contracts.StyleResolver = (*Resolver)(nil)
