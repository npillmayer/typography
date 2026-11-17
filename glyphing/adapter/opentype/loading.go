package opentype

import (
	"context"
	"fmt"

	"github.com/npillmayer/khipu/contracts"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
	"github.com/npillmayer/typography/glyphing"
)

// Load prepares a selected face without installing a backend in the generic
// resolver. Select exactly one path or retained resource.
func (s Services) Load(ctx context.Context, path string, resource *fontresource.Resource) (glyphing.FaceEntry, error) {
	if ctx == nil {
		return glyphing.FaceEntry{}, fmt.Errorf("nil load context")
	}
	if err := ctx.Err(); err != nil {
		return glyphing.FaceEntry{}, err
	}
	if (path == "") == (resource == nil) {
		return glyphing.FaceEntry{}, fmt.Errorf("select exactly one font file or resource")
	}
	var face *Face
	var err error
	if resource != nil {
		face, err = LoadResource(resource)
	} else {
		face, err = LoadFile(path)
	}
	if err != nil {
		return glyphing.FaceEntry{}, err
	}
	if err = ctx.Err(); err != nil {
		return glyphing.FaceEntry{}, err
	}
	info, err := fontmeta.Identify(face)
	if err != nil {
		return glyphing.FaceEntry{}, err
	}
	return glyphing.FaceEntry{Face: face, Info: info, Resource: face.Resource()}, nil
}

// GlyphIndex accepts only a nonnil OpenType Face and performs nominal lookup
// under its lock. Foreign faces are rejected; zero denotes missing coverage.
func (Services) GlyphIndex(value contracts.FontFace, r rune) (uint32, error) {
	face, err := acceptedFace(value)
	if err != nil {
		return 0, fmt.Errorf("OpenType coverage: %w", err)
	}
	return face.GlyphIndex(r)
}

var _ glyphing.Coverage = Services{}

// Services supplies font loading and nominal coverage for glyphing only.
type Services struct{}
