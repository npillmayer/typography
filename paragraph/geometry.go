// Package paragraph prepares styled text with injected khipu services for
// reusable intrinsic measurement and layout, including fixed atomic inline boxes.
// It breaks/freezes horizontal material and assembles natural vertical lists.
// Resolved CSS-inspired line-height values are supported; CSS parsing/cascade,
// font discovery, rendering paint and height-constrained packing remain external.
package paragraph

// Box describes logical geometry in fractional big points (1/72 inch), with Y
// increasing downward. Extent includes all vertical material, including trailing
// spacing. Width encloses both target and occupied line widths, not glyph ink.
// Baseline is the first line's baseline measured from the top. An empty box has
// zero dimensions and HasBaseline=false. Under CSSLineHeight, negative leading
// can place Baseline outside [0, Extent]. No conversion to integer DU occurs.
type Box struct {
	Width, Extent, Baseline float64
	HasBaseline             bool
}

// Height returns the signed extent above the reference baseline. Extent is
// authoritative: floating-point subtraction/addition need not round identically.
func (b Box) Height() float64 { return b.Baseline }
// Depth returns the signed extent below the reference baseline. Extent is
// authoritative when floating-point recomposition would round differently.
func (b Box) Depth() float64  { return b.Extent - b.Baseline }

// MaterialKind distinguishes placed line boxes from fixed vertical kerns.
type MaterialKind uint8

const (
	// LineBox is a placed frozen line referenced by Material.Child.
	LineBox MaterialKind = iota
	// Kern is fixed vertical spacing with no stretch or shrink.
	Kern
)

// Material is an already placed item in a natural vertical list. Child indexes
// Lines for LineBox and is -1 for Kern. Kern is fixed spacing; this API offers no
// stretch/shrink or target-height packing. Geometry carries no glyph payload.
type Material struct {
	Kind        MaterialKind
	Child       int
	Top, Extent float64
}
