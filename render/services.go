// Package render builds detached paragraph and line-range fragment proofs from
// retained frozen content and vertical placements, and maps glyph anchors to
// original paint sources.
// Its backend boundary supplies retained outlines and visual order;
// proof construction does not load fonts, measure lines or repeat layout.
package render

import (
	"github.com/npillmayer/khipu/contracts"
	"golang.org/x/text/unicode/bidi"
)

// Point is an outline control point in font units with downward-positive Y.
type Point struct{ X, Y float64 }

// Outline is an SVG path in font units with downward Y, plus its control points
// for conservative proof bounds. It contains no backend parser types.
type Outline struct {
	Path   string
	Points []Point
}
// Backend supplies retained outlines and directional glyph ordering without
// loading fonts or measuring lines. Keep face/scale services stable for the proof
// lifetime; implementations must honor the caller-owned slice contract below.
type Backend interface {
	Outline(contracts.FontFace, uint32) (Outline, error)
	// VisualOrder receives one complete directional fragment in logical order.
	// The caller owns the slice; implementations may reorder it in place.
	VisualOrder([]contracts.ShapedGlyph, bidi.Direction) ([]contracts.ShapedGlyph, error)
}
