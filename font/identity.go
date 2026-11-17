// Package font defines retained identity and resource capabilities shared by
// glyphing and rendering. It performs no discovery, parsing or shaping.
package font

import (
	"fmt"
	"reflect"

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

// IdentityProvider identifies retained faces without reopening source paths.
type IdentityProvider interface {
	Identify(contracts.FontFace) (Info, error)
}

// ResourceFace promises that glyph IDs refer to the exact retained resource.
// Info must describe Resource, and return detached mutable metadata. No accessor
// may load files, query metrics or mutate state. This is an optional capability;
// FontFace itself remains backend-neutral and size-independent.
type ResourceFace interface {
	contracts.FontFace
	Resource() *fontresource.Resource
	Info() Info
}

// Identify validates retained identity without parsing or querying font tables.
func Identify(face contracts.FontFace) (Info, error) {
	if face == nil {
		return Info{}, fmt.Errorf("nil retained face")
	}
	v := reflect.ValueOf(face)
	switch v.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan, reflect.Interface:
		if v.IsNil() {
			return Info{}, fmt.Errorf("nil retained face")
		}
	}
	f, ok := face.(ResourceFace)
	if !ok {
		return Info{}, fmt.Errorf("face %T has no retained resource identity", face)
	}
	r := f.Resource()
	if r == nil {
		return Info{}, fmt.Errorf("missing retained font resource")
	}
	i := f.Info()
	if i.ResourceID != r.ID() || i.SHA256 != r.SHA256() || i.Origin != r.Origin() || i.Path != i.Origin.Location {
		return Info{}, fmt.Errorf("font identity does not match retained resource")
	}
	i.Warnings = append([]string(nil), i.Warnings...)
	return i, nil
}
