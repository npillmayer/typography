package layout

// Box describes logical geometry in fractional big points (1/72 inch), with Y
// increasing downward. Extent is the bottom of the last CSS line box, with no extra
// trailing spacing. Width encloses both target and occupied line widths, not glyph ink.
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
func (b Box) Depth() float64 { return b.Extent - b.Baseline }

// MaterialKind distinguishes placed line boxes from fixed vertical kerns.
type MaterialKind uint8

const (
	// LineBox is a placed frozen line referenced by Material.Child.
	LineBox MaterialKind = iota
	// Kern identifies fixed vertical spacing. CSS VList/Fragment constructors
	// reject it; retaining the kind does not add a packing or unboxing API.
	Kern
)

// Material is an already placed vertical item, not an unboxed node. Completed
// CSS VLists/Fragments accept only LineBox, whose Child indexes their Lines.
// This API offers no stretch/shrink or target-height packing. Geometry carries
// no glyph payload.
type Material struct {
	Kind        MaterialKind
	Child       int
	Top, Extent float64
}
