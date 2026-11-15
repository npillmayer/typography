package render_test

import (
	"context"
	"errors"
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
	p.Font = helpers.NewSyntheticFont()
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
