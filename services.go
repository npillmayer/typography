// Package typography resolves shared styles and itemizes text using preselected faces and injected
// coverage. It does not install a parser, discovery provider, or shaper.
package typography

import (
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

// Coverage reports nominal cmap coverage; zero means missing. It does not promise
// successful shaping. Failures are returned, never treated as permission to fallback.
type Coverage interface {
	GlyphIndex(contracts.FontFace, rune) (uint32, error)
}

// CoverageFunc adapts a nominal glyph lookup function to Coverage.
type CoverageFunc func(contracts.FontFace, rune) (uint32, error)

// GlyphIndex delegates to f; zero denotes missing nominal coverage.
func (f CoverageFunc) GlyphIndex(face contracts.FontFace, r rune) (uint32, error) { return f(face, r) }

var _ contracts.StyleResolver = (*Resolver)(nil)
