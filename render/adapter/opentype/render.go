// Package opentype renders retained OpenType resources without a glyphing adapter.
package opentype

import (
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/opentype/otshape"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
	"github.com/npillmayer/typography/render"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/bidi"
)

// Services owns an outline-parser cache for one rendering session. Use New or
// a nonnil pointer to a zero-value Services struct, such as &Services{} or
// new(Services). A nil *Services is not supported. Do not copy after use.
// Methods are concurrency-safe.
// Releasing the service releases its cache; no global state or file I/O is used.
type Services struct {
	mu         sync.Mutex
	fonts      map[string]*sfnt.Font
	identities map[*fontresource.Resource]fontmeta.Info
}

// New creates independent rendering services. No fonts are parsed eagerly.
func New() *Services { return &Services{} }

// Identify obtains detached retained identity without loading or metric queries.
func (s *Services) Identify(value contracts.FontFace) (fontmeta.Info, error) {
	if s == nil {
		return fontmeta.Info{}, fmt.Errorf("nil rendering service")
	}
	f, ok := value.(fontmeta.ResourceFace)
	if !ok {
		return fontmeta.Identify(value)
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan, reflect.Interface:
		if v.IsNil() {
			return fontmeta.Identify(value)
		}
	}
	r := f.Resource()
	if r == nil {
		return fontmeta.Identify(value)
	}
	i := f.Info()
	s.mu.Lock()
	defer s.mu.Unlock()
	expected, found := s.identities[r]
	if !found {
		var err error
		expected, err = fontmeta.Identify(value)
		if err != nil {
			return fontmeta.Info{}, err
		}
		if s.identities == nil {
			s.identities = make(map[*fontresource.Resource]fontmeta.Info)
		}
		// Immutable resources need hashing only once per rendering session.
		// Keep resource identity only; face labels/warnings are returned afresh.
		s.identities[r] = fontmeta.Info{ResourceID: expected.ResourceID, SHA256: expected.SHA256, Origin: expected.Origin, Path: expected.Path}
	}
	if i.ResourceID != expected.ResourceID || i.SHA256 != expected.SHA256 || i.Origin != expected.Origin || i.Path != expected.Path {
		return fontmeta.Info{}, fmt.Errorf("font identity does not match retained resource")
	}
	i.Warnings = slices.Clone(i.Warnings)
	return i, nil
}

func (s *Services) outline(value contracts.FontFace, gid uint32) (sfnt.Segments, error) {
	if s == nil || gid > 65535 {
		return nil, fmt.Errorf("invalid outline service/glyph")
	}
	info, err := s.Identify(value)
	if err != nil {
		return nil, err
	}
	resource := value.(fontmeta.ResourceFace).Resource()
	// Identity includes provenance; equal bytes with distinct origins stay distinct.
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fonts[info.ResourceID]
	if f == nil {
		reader := resource.Open()
		data, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		f, err = sfnt.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("outline parser: %w", err)
		}
		if s.fonts == nil {
			s.fonts = make(map[string]*sfnt.Font)
		}
		s.fonts[info.ResourceID] = f
	}
	var buffer sfnt.Buffer
	segments, err := f.LoadGlyph(&buffer, sfnt.GlyphIndex(gid), fixed.I(int(f.UnitsPerEm())), nil)
	if err != nil {
		return nil, fmt.Errorf("outline glyph %d: %w", gid, err)
	}
	return append(sfnt.Segments(nil), segments...), nil
}

// Outline converts a retained OpenType glyph outline into an SVG path and
// control points in font units with downward-positive Y. Foreign/nil faces are
// rejected. Outline parsing is owned by this rendering service and never accesses
// the glyphing backend or its locks.
func (s *Services) Outline(value contracts.FontFace, gid uint32) (render.Outline, error) {
	segments, err := s.outline(value, gid)
	if err != nil {
		return render.Outline{}, err
	}
	var path strings.Builder
	var points []render.Point
	for _, seg := range segments {
		count := 0
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			path.WriteString("M")
			count = 1
		case sfnt.SegmentOpLineTo:
			path.WriteString("L")
			count = 1
		case sfnt.SegmentOpQuadTo:
			path.WriteString("Q")
			count = 2
		case sfnt.SegmentOpCubeTo:
			path.WriteString("C")
			count = 3
		default:
			return render.Outline{}, fmt.Errorf("unsupported outline operation")
		}
		for a := 0; a < count; a++ {
			px, py := float64(seg.Args[a].X)/64, float64(seg.Args[a].Y)/64
			fmt.Fprintf(&path, "%.6g %.6g ", px, py)
			points = append(points, render.Point{X: px, Y: py})
		}
	}
	return render.Outline{Path: path.String(), Points: points}, nil
}

// VisualOrder delegates directional-fragment ordering to the OpenType backend.
// The glyph slice must be caller-owned; consumers must not reorder frozen knots.
func (*Services) VisualOrder(g []contracts.ShapedGlyph, d bidi.Direction) ([]contracts.ShapedGlyph, error) {
	return otshape.VisualOrder(g, d, true)
}

var _ fontmeta.IdentityProvider = (*Services)(nil)
var _ render.Backend = (*Services)(nil)
