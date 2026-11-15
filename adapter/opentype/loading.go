package opentype

import (
	"context"
	"fmt"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography"
	"github.com/npillmayer/typography/fontresource"
)

// Load implements the injected typography loader without installing a backend
// in the generic resolver. Select exactly one path or retained resource.
func (s Services) Load(ctx context.Context, path string, resource *fontresource.Resource) (typography.FaceEntry, error) {
	if ctx == nil {
		return typography.FaceEntry{}, fmt.Errorf("nil load context")
	}
	if err := ctx.Err(); err != nil {
		return typography.FaceEntry{}, err
	}
	if (path == "") == (resource == nil) {
		return typography.FaceEntry{}, fmt.Errorf("select exactly one font file or resource")
	}
	var face *Face
	var err error
	if resource != nil {
		face, err = LoadResource(resource)
	} else {
		face, err = LoadFile(path)
	}
	if err != nil {
		return typography.FaceEntry{}, err
	}
	if err = ctx.Err(); err != nil {
		return typography.FaceEntry{}, err
	}
	info, err := s.Identify(face)
	if err != nil {
		return typography.FaceEntry{}, err
	}
	return typography.FaceEntry{Face: face, Info: info, Resource: face.Resource()}, nil
}

// GlyphIndex accepts only a nonnil OpenType Face and performs nominal lookup
// under its lock. Foreign faces are rejected; zero denotes missing coverage.
func (Services) GlyphIndex(value contracts.FontFace, r rune) (uint32, error) {
	face, ok := value.(*Face)
	if !ok || face == nil {
		return 0, fmt.Errorf("unsupported coverage face %T", value)
	}
	return face.GlyphIndex(r)
}

var _ typography.FaceLoader = Services{}
var _ typography.Coverage = Services{}
