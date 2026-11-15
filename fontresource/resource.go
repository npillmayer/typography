// Package fontresource owns immutable font bytes and provenance. It does not
// parse fonts, discover resources, or import concrete font/provider types.
package fontresource

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
)

// Slant is a request code independent of discovery libraries. Its numeric
// values are fixed for version-1 identity encoding.
type Slant int
// Weight is the legacy request identity code, not a numeric CSS weight.
// Its fixed values must remain compatible with version-1 resource IDs.
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

// Request identifies a provider query using legacy slant/weight codes.
// It records provenance; it does not perform discovery.
type Request struct {
	Pattern string
	Style   Slant
	Weight  Weight
}
// Origin records provider, resource name, location and original request.
// All fields participate in the version-1 resource identity.
type Origin struct {
	Provider, Name, Location string
	Request                  Request
}

// MaxBytes is the maximum retained font snapshot size: 64 MiB.
const MaxBytes = 64 << 20

// Resource owns a snapshot. Copies may share private immutable bytes safely.
// A zero Resource is an empty snapshot; parsing validity belongs to an adapter.
type Resource struct {
	data   []byte
	origin Origin
}

// Read copies at most MaxBytes into an immutable snapshot with the given origin.
// It reads at most MaxBytes+1 bytes to detect oversize input and returns no resource
// on read failure, nil input or overflow. It does not parse fonts or close src.
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
// Origin returns the provenance record by value.
func (r *Resource) Origin() Origin { return r.origin }
// SHA256 returns the lowercase hexadecimal hash of retained content only.
// Use ID for the versioned identity that also includes provenance.
func (r *Resource) SHA256() string { return fmt.Sprintf("%x", sha256.Sum256(r.data)) }

// ID is the version-1 provenance identity. It hashes Go-quoted provider, name,
// location, pattern, decimal slant/weight, and the lower-case content SHA256,
// joined with |. Do not change this encoding without a versioned migration.
func (r *Resource) ID() string {
	o := r.origin
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(fmt.Sprintf("%q|%q|%q|%q|%d|%d|%s", o.Provider, o.Name, o.Location, o.Request.Pattern, o.Request.Style, o.Request.Weight, r.SHA256()))))
}
