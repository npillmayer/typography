package render

import (
	"context"
	"fmt"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography"
	"github.com/npillmayer/typography/paragraph"
	"golang.org/x/image/colornames"
	"math"
	"regexp"
	"strings"
)

// Services supplies retained identity and outlines; no backend is installed here.
// Keep these services usable for the duration of proof construction.
type Services struct {
	Backend  Backend
	Identity typography.IdentityProvider
}

// ParagraphInput borrows completed immutable placements and frozen content.
// Text and Paint address the same processed UTF-8 stream used for layout.
// Each glyph receives the paint at its retained logical source anchor.
// Keep the input and service implementations stable during this call.
type ParagraphInput struct {
	Frozen *khipu.FrozenKhipu
	VList  *paragraph.VList
	Text   string
	Paint  []typography.PaintSpan
}

// Glyph holds physical drawing order and original source correspondence.
// X/Y use big points; Scale converts font units to big points.
// Path uses font units with downward Y.
type Glyph struct {
	Line, Knot       int
	SourceByte       uint64 // original UTF-8 anchor
	GlyphID, Cluster uint32
	X, Y, Scale      float64
	Color, Path      string
	FaceSHA256       string
}

// Line contains logical line geometry in big points with downward-positive Y.
// Width is occupied width; TargetWidth is the requested line width, not ink bounds.
type Line struct{ X, Baseline, Top, Height, Width, TargetWidth float64 }

// ParagraphProof owns detached glyph and line slices. Bounds include the origin,
// logical line rectangles and outline control points in big points; they are
// conservative proof bounds, not tight glyph-ink bounds.
type ParagraphProof struct {
	Glyphs                 []Glyph
	Lines                  []Line
	MinX, MinY, MaxX, MaxY float64
}

var hexColor = regexp.MustCompile(`^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$`)

func proofColor(spans []typography.PaintSpan, pos uint64) (string, error) {
	color := "black"
	for _, s := range spans {
		if s.Start <= pos && pos < s.End {
			color = strings.ToLower(strings.TrimSpace(s.Paint.Color))
			break
		}
	}
	if color == "transparent" || hexColor.MatchString(color) {
		return color, nil
	}
	if _, ok := colornames.Map[color]; ok {
		return color, nil
	}
	return "", fmt.Errorf("unsupported proof color %q: use a named or hex color", color)
}

// BuildParagraphProof never reshapes, rebreaks, reloads a font file, or reorders knots.
// Vertical placement comes from the retained vlist; rendering never queries line
// metrics or derives baselines from mutable Input.
// Knots are already physical LTR. VisualOrder converts each detached logical
// glyph fragment to drawing order, preserving its supplied advances/offsets.
func BuildParagraphProof(ctx context.Context, input ParagraphInput, services Services) (*ParagraphProof, error) {
	if ctx == nil || input.Frozen == nil || input.VList == nil || input.VList.Frozen() != input.Frozen || services.Backend == nil || services.Identity == nil {
		return nil, fmt.Errorf("glyph proof requires retained placements, matching frozen content and rendering services")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p := &ParagraphProof{}
	for _, placement := range input.VList.Lines() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := p.appendLine(ctx, input, services, placement); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, v := range []float64{p.MinX, p.MinY, p.MaxX, p.MaxY} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("non-finite outline bounds")
		}
	}
	return p, nil
}

// include expands conservative bounds from their initial origin. Preserve the
// min/max order, including non-finite values checked after the complete walk.
func (p *ParagraphProof) include(x, y float64) {
	p.MinX = math.Min(p.MinX, x)
	p.MaxX = math.Max(p.MaxX, x)
	p.MinY = math.Min(p.MinY, y)
	p.MaxY = math.Max(p.MaxY, y)
}

// appendLine uses retained vertical placement and traverses physical knots once.
// All knots advance the horizontal pen, including glue, fill and atomic boxes.
func (p *ParagraphProof) appendLine(ctx context.Context, input ParagraphInput, services Services, placement paragraph.Line) error {
	li, line := placement.Index, placement.Horizontal
	baseline := placement.Baseline
	p.Lines = append(p.Lines, Line{placement.X, baseline, placement.Top, placement.Height, line.Width.Points(), line.TargetWidth.Points()})
	p.include(placement.X, placement.Top)
	p.include(placement.X+math.Max(line.Width.Points(), line.TargetWidth.Points()), placement.Top+placement.Height)
	x := placement.X + line.LeftSkip.Points()
	for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
		knot, _ := input.Frozen.Knot(k)
		t, ok := input.Frozen.CopyText(k)
		if ok {
			origin := Glyph{Line: li, Knot: k, X: x, Y: baseline}
			if err := p.appendText(ctx, input, services, knot, t, origin); err != nil {
				return err
			}
		}
		x += knot.W.Points()
	}
	return nil
}

// appendText resolves identity and paint before ordering a detached fragment.
// origin supplies its line/knot indexes and initial pen position. Glyph advances
// affect only this fragment; the enclosing line advances by the frozen knot width.
func (p *ParagraphProof) appendText(ctx context.Context, input ParagraphInput, services Services, knot khipu.FrozenKnot, t khipu.FrozenText, origin Glyph) error {
	if !t.Context.Params.Font.Valid() {
		return fmt.Errorf("invalid retained font instance")
	}
	identity, err := services.Identity.Identify(t.Context.Params.Font.Face())
	if err != nil {
		return err
	}
	// Validate paint before invoking ordering/outline services, preserving failure precedence.
	if _, err := proofColor(input.Paint, knot.Source.Bytes.Start); err != nil {
		return err
	}
	sources, err := PaintSources(input.Text, knot, t)
	if err != nil {
		return err
	}
	scale := t.Context.Params.Font.Scale().ToDU(1) / float64(dimen.BP)
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
		return fmt.Errorf("invalid retained glyph scale")
	}
	penX, penY := origin.X, origin.Y
	// CopyText owns these slices; in-place ordering leaves frozen data intact.
	visual, err := services.Backend.VisualOrder(t.Glyphs, t.Context.Params.Direction)
	if err != nil {
		return fmt.Errorf("glyph proof line %d knot %d: %w", origin.Line, origin.Knot, err)
	}
	for _, g := range visual {
		if err := ctx.Err(); err != nil {
			return err
		}
		source := sources[g.Cluster]
		color, err := proofColor(input.Paint, source)
		if err != nil {
			return err
		}
		gx, gy := penX+float64(g.XOffset)*scale, penY-float64(g.YOffset)*scale
		glyph := Glyph{Line: origin.Line, Knot: origin.Knot, SourceByte: source, GlyphID: g.GlyphID, Cluster: g.Cluster, X: gx, Y: gy, Scale: scale, Color: color, FaceSHA256: identity.SHA256}
		if err := p.appendGlyph(services.Backend, t.Context.Params.Font.Face(), glyph); err != nil {
			return err
		}
		penX += float64(g.XAdvance) * scale
		penY -= float64(g.YAdvance) * scale
	}
	return nil
}

// appendGlyph validates the physical origin before querying retained outlines,
// then records conservative control-point bounds and the detached glyph value.
func (p *ParagraphProof) appendGlyph(backend Backend, face contracts.FontFace, glyph Glyph) error {
	if math.IsNaN(glyph.X) || math.IsNaN(glyph.Y) || math.IsInf(glyph.X, 0) || math.IsInf(glyph.Y, 0) {
		return fmt.Errorf("non-finite glyph origin")
	}
	outline, err := backend.Outline(face, glyph.GlyphID)
	if err != nil {
		return err
	}
	for _, point := range outline.Points {
		p.include(glyph.X+point.X*glyph.Scale, glyph.Y+point.Y*glyph.Scale)
	}
	glyph.Path = outline.Path
	p.Glyphs = append(p.Glyphs, glyph)
	return nil
}
