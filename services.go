// Package typography resolves shared styles and itemizes text using injected
// font services. It does not install a parser, discovery provider, or shaper.
package typography

import (
	"context"
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
// Loader must retain the exact bytes represented by Info. Services and faces
// must remain safe for their intended use for the resolver/result lifetime.
type FaceEntry struct {
	Face     contracts.FontFace
	Info     Info
	Resource *fontresource.Resource
}

// FaceLoader loads exactly one selected path or resource, without substitution.
// The resolver validates selection before calling it. File I/O is loader-owned.
type FaceLoader interface {
	Load(context.Context, string, *fontresource.Resource) (FaceEntry, error)
}

// Coverage reports nominal cmap coverage; zero means missing. It does not promise
// successful shaping. Failures are returned, never treated as permission to fallback.
type Coverage interface {
	GlyphIndex(contracts.FontFace, rune) (uint32, error)
}
type Services struct {
	Loader   FaceLoader
	Coverage Coverage
}
type LoadFunc func(context.Context, string, *fontresource.Resource) (FaceEntry, error)

func (f LoadFunc) Load(ctx context.Context, path string, r *fontresource.Resource) (FaceEntry, error) {
	return f(ctx, path, r)
}

type CoverageFunc func(contracts.FontFace, rune) (uint32, error)

func (f CoverageFunc) GlyphIndex(face contracts.FontFace, r rune) (uint32, error) { return f(face, r) }

var _ contracts.StyleResolver = (*Resolver)(nil)
