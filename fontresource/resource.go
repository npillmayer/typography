// Package fontresource owns immutable font bytes and provenance. It does not
// parse fonts, discover resources, or import concrete font/provider types.
package fontresource

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
)

// Slant and Weight are request values independent of discovery libraries.
// Numeric values are fixed for version-1 identity encoding; Weight is not a CSS
// numeric weight. Document styles use CSS weights, which adapters must convert.
type Slant int
type Weight int

const (
	Normal Slant = iota
	Italic
	Oblique
)
const (
	Thin       Weight = -3
	ExtraLight Weight = -2
	Light      Weight = -1
	Regular    Weight = 0
	Medium     Weight = 1
	SemiBold   Weight = 2
	Bold       Weight = 3
	ExtraBold  Weight = 4
	Black      Weight = 5
)

type Request struct {
	Pattern string
	Style   Slant
	Weight  Weight
}
type Origin struct {
	Provider, Name, Location string
	Request                  Request
}

const MaxBytes = 64 << 20

// Resource owns a snapshot. Copies may share private immutable bytes safely.
// A zero Resource is an empty snapshot; parsing validity belongs to an adapter.
type Resource struct {
	data   []byte
	origin Origin
}

func Read(src io.Reader, origin Origin) (*Resource, error) {
	if src == nil {
		return nil, fmt.Errorf("font reader is nil")
	}
	data, err := io.ReadAll(io.LimitReader(src, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read font resource: %w", err)
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("font exceeds 64 MiB")
	}
	return &Resource{data: data, origin: origin}, nil
}

// Open exposes only a reader, never a mutable byte slice or buffer.
func (r *Resource) Open() io.ReadCloser {
	return io.NopCloser(struct{ io.Reader }{bytes.NewReader(r.data)})
}
func (r *Resource) Origin() Origin { return r.origin }
func (r *Resource) SHA256() string { return fmt.Sprintf("%x", sha256.Sum256(r.data)) }

// ID is the version-1 provenance identity. It hashes Go-quoted provider, name,
// location, pattern, decimal slant/weight, and the lower-case content SHA256,
// joined with |. Do not change this encoding without a versioned migration.
func (r *Resource) ID() string {
	o := r.origin
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(fmt.Sprintf("%q|%q|%q|%q|%d|%d|%s", o.Provider, o.Name, o.Location, o.Request.Pattern, o.Request.Style, o.Request.Weight, r.SHA256()))))
}
