// Package render defines the small backend boundary used by frozen proof walkers.
package render

import (
	"github.com/npillmayer/khipu/contracts"
	"golang.org/x/text/unicode/bidi"
)

type Point struct{ X, Y float64 }

// Outline is an SVG path in font units with downward Y, plus its control points
// for conservative proof bounds. It contains no backend parser types.
type Outline struct {
	Path   string
	Points []Point
}
type Backend interface {
	Outline(contracts.FontFace, uint32) (Outline, error)
	// VisualOrder receives one complete directional fragment in logical order.
	// The caller owns the slice; implementations may reorder it in place.
	VisualOrder([]contracts.ShapedGlyph, bidi.Direction) ([]contracts.ShapedGlyph, error)
}
