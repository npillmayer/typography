package render

import (
	"context"
	"fmt"
	"github.com/npillmayer/khipu"
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
// GlyphBoundary is the default; LegacyStyleBoundary supports existing replay.
// Keep the input and service implementations stable during this call.
type ParagraphInput struct {
	Frozen      *khipu.FrozenKhipu
	VList       *paragraph.VList
	Text        string
	Paint       []typography.PaintSpan
	PaintPolicy typography.PaintPolicy
}

// Glyph holds physical drawing order and original source correspondence.
// X/Y and Scale use big points; Path uses font units with downward Y.
type Glyph struct {
	Line, Knot       int
	SourceByte       uint64 // original UTF-8 anchor; legacy paint uses the knot's start byte
	GlyphID, Cluster uint32
	X, Y, Scale      float64
	Color, Path      string
	FaceSHA256       string
}
type Line struct{ X, Baseline, Top, Height, Width, TargetWidth float64 }
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
	include := func(x, y float64) {
		p.MinX = math.Min(p.MinX, x)
		p.MaxX = math.Max(p.MaxX, x)
		p.MinY = math.Min(p.MinY, y)
		p.MaxY = math.Max(p.MaxY, y)
	}
	for _, placement := range input.VList.Lines() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		li, line := placement.Index, placement.Horizontal
		baseline := placement.Baseline
		p.Lines = append(p.Lines, Line{placement.X, baseline, placement.Top, placement.Height, line.Width.Points(), line.TargetWidth.Points()})
		include(placement.X, placement.Top)
		include(placement.X+math.Max(line.Width.Points(), line.TargetWidth.Points()), placement.Top+placement.Height)
		x := placement.X + line.LeftSkip.Points()
		for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
			knot, _ := input.Frozen.Knot(k)
			t, ok := input.Frozen.CopyText(k)
			if ok {
				identity, err := services.Identity.Identify(t.Context.Params.Font)
				if err != nil {
					return nil, err
				}
				color, err := proofColor(input.Paint, knot.Source.Bytes.Start)
				if err != nil {
					return nil, err
				}
				var sources map[uint32]uint64
				if input.PaintPolicy != typography.LegacyStyleBoundary {
					sources, err = PaintSources(input.Text, knot, t)
					if err != nil {
						return nil, err
					}
				}
				scale := t.Context.Scale.ToDU(1) / float64(dimen.BP)
				if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
					return nil, fmt.Errorf("invalid retained glyph scale")
				}
				penX, penY := x, baseline
				// CopyText owns these slices; in-place ordering leaves frozen data intact.
				visual, err := services.Backend.VisualOrder(t.Glyphs, t.Context.Params.Direction)
				if err != nil {
					return nil, fmt.Errorf("glyph proof line %d knot %d: %w", li, k, err)
				}
				for _, g := range visual {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					source := knot.Source.Bytes.Start
					if sources != nil {
						source = sources[g.Cluster]
						color, err = proofColor(input.Paint, source)
						if err != nil {
							return nil, err
						}
					}
					gx, gy := penX+float64(g.XOffset)*scale, penY-float64(g.YOffset)*scale
					if math.IsNaN(gx) || math.IsNaN(gy) || math.IsInf(gx, 0) || math.IsInf(gy, 0) {
						return nil, fmt.Errorf("non-finite glyph origin")
					}
					outline, err := services.Backend.Outline(t.Context.Params.Font, g.GlyphID)
					if err != nil {
						return nil, err
					}
					for _, point := range outline.Points {
						include(gx+point.X*scale, gy+point.Y*scale)
					}
					p.Glyphs = append(p.Glyphs, Glyph{Line: li, Knot: k, SourceByte: source, GlyphID: g.GlyphID, Cluster: g.Cluster, X: gx, Y: gy, Scale: scale, Color: color, Path: outline.Path, FaceSHA256: identity.SHA256})
					penX += float64(g.XAdvance) * scale
					penY -= float64(g.YAdvance) * scale
				}
			}
			x += knot.W.Points()
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
