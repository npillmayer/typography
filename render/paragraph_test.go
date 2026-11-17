package render_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography"
	"github.com/npillmayer/typography/paragraph"
	"github.com/npillmayer/typography/render"
	"github.com/npillmayer/typography/style"
	"github.com/npillmayer/uax/bidi"
	xbidi "golang.org/x/text/unicode/bidi"
)

type guardedFace struct {
	helpers.SyntheticFont
	deny bool
}

func (f *guardedFace) Metrics() (contracts.FontMetrics, error) {
	if f.deny {
		return contracts.FontMetrics{}, errors.New("metrics queried during rendering")
	}
	return f.SyntheticFont.Metrics()
}

type stream struct {
	*helpers.SyntheticStreamingShaper
}

func (s stream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	p.Font, _ = contracts.NewFontInstance(helpers.NewSyntheticFont(), p.Font.EmSize())
	return s.SyntheticStreamingShaper.Shape(ctx, p, r, o)
}

type backend struct {
	calls     int
	fail      error
	cancel    context.CancelFunc
	nonfinite bool
}

func (b *backend) Identify(contracts.FontFace) (typography.Info, error) {
	return typography.Info{SHA256: "test"}, nil
}
func (b *backend) VisualOrder(g []contracts.ShapedGlyph, d xbidi.Direction) ([]contracts.ShapedGlyph, error) {
	if d == xbidi.RightToLeft {
		slices.Reverse(g)
	}
	return g, nil
}
func (b *backend) Outline(contracts.FontFace, uint32) (render.Outline, error) {
	b.calls++
	if b.calls == 2 {
		if b.cancel != nil {
			b.cancel()
		}
		if b.fail != nil {
			return render.Outline{}, b.fail
		}
	}
	x := 1.0
	if b.nonfinite {
		x = math.NaN()
	}
	return render.Outline{Path: "M0 0 L1 1", Points: []render.Point{{}, {X: x, Y: 1}}}, nil
}

func proofInput(t *testing.T) (render.ParagraphInput, *guardedFace) {
	t.Helper()
	f := &guardedFace{}
	p, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("aאב"), Direction: bidi.LeftToRight}, paragraph.Params{
		Constraints: paragraph.Constraints{Width: 100 * dimen.BP}, Breaker: paragraph.FirstFit, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.NormalHeight}},
	}, khipu.Config{Streaming: &khipu.StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: f, EmSize: 10 * dimen.BP}, NewShaper: func() (contracts.StreamingShaper, error) {
		s, e := helpers.NewSyntheticStreamingShaper(8)
		return stream{s}, e
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return render.ParagraphInput{Frozen: p.Frozen, VList: p.VList, Text: "aאב", Paint: []typography.PaintSpan{{Start: 0, End: 1, Paint: style.Paint{Color: "red"}}, {Start: 1, End: 5, Paint: style.Paint{Color: "navy"}}}}, f
}

func TestRetainedProofCoordinatesAndLifetime(t *testing.T) {
	input, f := proofInput(t)
	f.deny = true
	b := &backend{}
	services := render.Services{Backend: b, Identity: b}
	p, err := render.BuildParagraphProof(t.Context(), input, services)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Glyphs) != 3 || p.Lines[0].Baseline != 8 {
		t.Fatal(p)
	}
	// Synthetic 10bp yields 5bp cells. RTL glyph fragments reverse, byte anchors
	// remain original UTF-8 coordinates, and paint follows those anchors.
	for i, w := range []struct {
		gid    uint32
		source uint64
		color  string
	}{{'a' + 1, 0, "red"}, {'ב' + 1, 3, "navy"}, {'א' + 1, 1, "navy"}} {
		g := p.Glyphs[i]
		if g.GlyphID != w.gid || g.SourceByte != w.source || g.X != float64(i)*5 || g.Y != 8 || g.Color != w.color {
			t.Fatalf("glyph %d: %+v", i, g)
		}
	}
	again, err := render.BuildParagraphProof(t.Context(), input, services)
	if err != nil || !reflect.DeepEqual(p, again) {
		t.Fatal("mutated retained glyphs", err)
	}
	p.Glyphs[0].X = 999
	if again.Glyphs[0].X == 999 {
		t.Fatal("proof results share storage")
	}
}

func TestRetainedProofFailureAndCancellation(t *testing.T) {
	input, _ := proofInput(t)
	sentinel := errors.New("outline unavailable")
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		b := &backend{fail: sentinel}
		want := sentinel
		if cancelled {
			b.fail = nil
			b.cancel = cancel
			want = context.Canceled
		}
		p, err := render.BuildParagraphProof(ctx, input, render.Services{Backend: b, Identity: b})
		cancel()
		if p != nil || !errors.Is(err, want) {
			t.Fatal("partial proof escaped", p, err)
		}
	}
	b := &backend{nonfinite: true}
	if p, err := render.BuildParagraphProof(t.Context(), input, render.Services{Backend: b, Identity: b}); p != nil || err == nil {
		t.Fatal("non-finite outline accepted")
	}
	input.VList = nil
	if p, err := render.BuildParagraphProof(t.Context(), input, render.Services{Backend: b, Identity: b}); p != nil || err == nil {
		t.Fatal("missing placements reconstructed")
	}
}

// tracingBackend exposes service ordering, including late errors after a prior
// fragment has already contributed to the private proof under construction.
type tracingBackend struct {
	backend
	events []string
	failAt string
	err    error
}

func (b *tracingBackend) Identify(face contracts.FontFace) (typography.Info, error) {
	b.events = append(b.events, "identity")
	if b.failAt == "identity" {
		return typography.Info{}, b.err
	}
	return b.backend.Identify(face)
}

func (b *tracingBackend) VisualOrder(g []contracts.ShapedGlyph, d xbidi.Direction) ([]contracts.ShapedGlyph, error) {
	b.events = append(b.events, "visual")
	if b.failAt == "visual" && d == xbidi.RightToLeft {
		return nil, b.err
	}
	return b.backend.VisualOrder(g, d)
}

func (b *tracingBackend) Outline(face contracts.FontFace, gid uint32) (render.Outline, error) {
	b.events = append(b.events, "outline")
	return b.backend.Outline(face, gid)
}

func TestRetainedProofServiceOrderAndFailurePrecedence(t *testing.T) {
	sentinel := errors.New("service unavailable")
	for _, tc := range []struct {
		name, failAt string
		badPaint     bool
		events       []string
	}{
		{"success", "", false, []string{"identity", "visual", "outline", "identity", "visual", "outline", "outline"}},
		{"identity precedes paint", "identity", true, []string{"identity"}},
		{"paint precedes ordering", "", true, []string{"identity"}},
		{"late ordering failure", "visual", false, []string{"identity", "visual", "outline", "identity", "visual"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, face := proofInput(t)
			face.deny = true
			if tc.badPaint {
				input.Paint[0].Paint.Color = "invalid color"
			}
			b := &tracingBackend{failAt: tc.failAt, err: sentinel}
			p, err := render.BuildParagraphProof(t.Context(), input, render.Services{Backend: b, Identity: b})
			if !reflect.DeepEqual(b.events, tc.events) {
				t.Fatalf("service calls = %v, want %v", b.events, tc.events)
			}
			if tc.failAt != "" {
				if p != nil || !errors.Is(err, sentinel) {
					t.Fatalf("lost cause or published partial proof: %v, %v", p, err)
				}
				if tc.failAt == "visual" {
					line := input.VList.Lines()[0].Horizontal
					knotIndex := -1
					for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
						if tx, ok := input.Frozen.CopyText(k); ok && tx.Context.Params.Direction == xbidi.RightToLeft {
							knotIndex = k
							break
						}
					}
					want := fmt.Sprintf("glyph proof line 0 knot %d: %v", knotIndex, sentinel)
					if knotIndex < 0 || err.Error() != want {
						t.Fatalf("ordering error = %v, want %q", err, want)
					}
				}
			} else if tc.badPaint {
				if p != nil || err == nil || err.Error() != `unsupported proof color "invalid color": use a named or hex color` {
					t.Fatalf("paint error = %v, proof = %v", err, p)
				}
			} else if err != nil || p == nil || len(p.Glyphs) != 3 {
				t.Fatalf("proof = %v, error = %v", p, err)
			}
		})
	}
}

func (s stream) CheckFace(face contracts.FontFace) error {
	if f, ok := face.(*guardedFace); ok {
		if f == nil {
			return contracts.ErrIllegalArguments
		}
		return nil // This fixture shapes fixed synthetic cells for its metric wrapper.
	}
	return s.SyntheticStreamingShaper.CheckFace(face)
}
