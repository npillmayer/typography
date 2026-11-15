// Package opentype adapts offline OpenType fonts to khipu's public contracts.
package opentype

import (
	"fmt"
	"github.com/npillmayer/typography/fontresource"
	"golang.org/x/image/font/sfnt"
	"io"
	"os"
	"sync"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/opentype"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
)

// Face retains private immutable bytes and serializes access to lazy backend data.
// Use Info to obtain detached identity and parser diagnostics.
type Face struct {
	mu        sync.Mutex
	font      *ot.Font
	data      []byte
	info      Info
	resource  *fontresource.Resource
	metrics   contracts.FontMetrics
	unitScale float64
	outline   *sfnt.Font
}
// Info is detached identity and parser metadata for a retained OpenType face.
type Info struct {
	ResourceID                      string
	Origin                          fontresource.Origin
	Path, Family, Subfamily, SHA256 string
	Warnings                        []string
}

// Info returns identity by value with a detached warning slice.
func (f *Face) Info() Info { i := f.info; i.Warnings = append([]string(nil), i.Warnings...); return i }

// Loader implements contracts.FontLoader using normal OpenType parsing and
// bounded retained snapshots. Use Services for typography.FaceLoader injection.
type Loader struct{}

var _ contracts.FontLoader = Loader{}

// LoadFont snapshots src with empty provenance and parses it with normal parser
// policy. It does not close src; read/parse failures return no face.
func (Loader) LoadFont(src io.Reader) (contracts.FontFace, error) {
	resource, err := fontresource.Read(src, fontresource.Origin{})
	if err != nil {
		return nil, err
	}
	face, err := LoadResource(resource)
	if err != nil {
		return nil, err
	}
	return face, nil
}

// LoadResource parses a retained immutable snapshot with the normal parser policy.
func LoadResource(resource *fontresource.Resource) (*Face, error) {
	if resource == nil {
		return nil, fmt.Errorf("nil font resource")
	}
	reader := resource.Open()
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	font, err := ot.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse OpenType: %w", err)
	}
	if font == nil {
		return nil, fmt.Errorf("parser returned no font")
	}
	if errs := font.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("OpenType parser diagnostics: %v (critical: %v)", errs, font.CriticalErrors())
	}
	family, subfamily := opentype.FamilyName(font)
	f := &Face{font: font, data: data, resource: resource, info: Info{Family: family, Subfamily: subfamily, SHA256: resource.SHA256(), ResourceID: resource.ID(), Origin: resource.Origin(), Path: resource.Origin().Location}}
	for _, w := range font.Warnings() {
		f.info.Warnings = append(f.info.Warnings, fmt.Sprint(w))
	}
	m := otquery.FontMetrics(font)
	if m.UnitsPerEm <= 0 {
		return nil, fmt.Errorf("invalid units per em")
	}
	f.metrics = contracts.FontMetrics{UnitsPerEm: uint32(m.UnitsPerEm), Ascent: int32(m.Ascent), Descent: int32(m.Descent), LineGap: int32(m.LineGap)}
	f.unitScale, err = otquery.FontScale(font, 1)
	if err != nil {
		return nil, err
	}
	return f, nil
}
// LoadFile snapshots a regular file with file-path provenance, closes the file
// and applies normal parser policy. Subsequent operations use retained bytes.
func LoadFile(path string) (*Face, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", path, err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("font %q is not a regular file", path)
	}
	resource, err := fontresource.Read(file, fontresource.Origin{Provider: "file", Location: path})
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", path, err)
	}
	face, err := LoadResource(resource)
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", path, err)
	}
	return face, nil
}

// Resource returns the immutable snapshot retained at load time, without file I/O.
func (f *Face) Resource() *fontresource.Resource { return f.resource }

// Metrics returns cached font-unit metrics by value. Nil or uninitialized faces
// are rejected; no lazy parser query is performed.
func (f *Face) Metrics() (contracts.FontMetrics, error) {
	if f == nil || f.font == nil {
		return contracts.FontMetrics{}, fmt.Errorf("nil OpenType face")
	}
	return f.metrics, nil
}

// Scale converts font units to DU without rounding to integer dimensions.
type Scale float64

// ToDU converts font units to fractional DU.
func (s Scale) ToDU(units int32) float64 { return float64(s) * float64(units) }
// Scale returns a font-unit-to-DU conversion for a positive size below dimen.Fil.
// Nil or uninitialized faces and invalid sizes are rejected.
func (f *Face) Scale(size dimen.DU) (contracts.FontScale, error) {
	if f == nil || f.font == nil || size <= 0 || size >= dimen.Fil {
		return nil, fmt.Errorf("invalid face or font size")
	}
	return Scale(f.unitScale * float64(size)), nil
}
// GlyphIndex queries nominal cmap coverage under the shared face lock.
// Zero means missing coverage. The receiver must be a loaded, nonnil face.
func (f *Face) GlyphIndex(r rune) (uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, err := otquery.GlyphIndex(f.font, r)
	return uint32(g), err
}
