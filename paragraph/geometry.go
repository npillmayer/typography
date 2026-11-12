// Package paragraph lays out prepared styled text with injected khipu services
// and assembles frozen horizontal lines into a boxed vertical list. It does not
// interpret CSS, discover fonts or carry rendering paint.
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

// Height and Depth are signed extents above/below the reference baseline. Extent is
// authoritative: floating-point subtraction/addition need not round identically.
func (b Box) Height() float64 { return b.Baseline }
func (b Box) Depth() float64  { return b.Extent - b.Baseline }

type MaterialKind uint8

const (
	LineBox MaterialKind = iota
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
