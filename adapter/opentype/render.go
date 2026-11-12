package opentype

import (
	"fmt"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/typography"
	"github.com/npillmayer/typography/render"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/unicode/bidi"
	"strings"
)

// Services supplies loading, coverage, retained identity and outlines through
// typography contracts. It owns no mutable state and installs no global backend.
type Services struct{}

func (Services) Identify(value contracts.FontFace) (typography.Info, error) {
	f, ok := value.(*Face)
	if !ok || f == nil {
		return typography.Info{}, fmt.Errorf("unsupported identity face %T", value)
	}
	i := f.Info()
	return typography.Info{ResourceID: i.ResourceID, Origin: i.Origin, Path: i.Path, Family: i.Family, Subfamily: i.Subfamily, SHA256: i.SHA256, Warnings: i.Warnings}, nil
}
func (Services) Outline(value contracts.FontFace, gid uint32) (render.Outline, error) {
	f, ok := value.(*Face)
	if !ok || f == nil {
		return render.Outline{}, fmt.Errorf("unsupported proof face %T", value)
	}
	segments, err := f.Outline(gid)
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
func (Services) VisualOrder(g []contracts.ShapedGlyph, d bidi.Direction) ([]contracts.ShapedGlyph, error) {
	return otshape.VisualOrder(g, d, true)
}

var _ typography.IdentityProvider = Services{}
var _ render.Backend = Services{}
