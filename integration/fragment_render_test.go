package integration_test

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/typography/render"
	"github.com/npillmayer/typography/style"
	xbidi "golang.org/x/text/unicode/bidi"
)

type sealedStream struct {
	stream
	sealed *bool
}

func (s sealedStream) CheckFace(f contracts.FontFace) error {
	if *s.sealed {
		panic("rendering called CheckFace")
	}
	return s.stream.CheckFace(f)
}
func (s sealedStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	if *s.sealed {
		panic("rendering shaped text")
	}
	return s.stream.Shape(ctx, p, r, o)
}

type orderCounter struct {
	backend
	orders int
}

func (b *orderCounter) VisualOrder(g []contracts.ShapedGlyph, d xbidi.Direction) ([]contracts.ShapedGlyph, error) {
	b.orders++
	return b.backend.VisualOrder(g, d)
}

// Return no strong preparation/continuation references. Both the ordinary owner
// and completed flow fragments must remain renderable after preparation dies.
func fragmentRenderFixture(t *testing.T, width dimen.DU) (render.ParagraphInput, []*layout.Fragment, weak.Pointer[paragraph.Prepared]) {
	t.Helper()
	const text = "aa bb cc dd ee ff gg hh"
	face := &guardedFace{}
	sealed := false
	params := khipu.DefaultParams()
	params.SpaceStretchPercent, params.SpaceShrinkPercent = 0, 0
	config := khipu.Config{Params: &params, Streaming: &khipu.StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: face, EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) {
			if sealed {
				panic("rendering opened a stream")
			}
			s, err := helpers.NewSyntheticStreamingShaper(8)
			return sealedStream{stream{s}, &sealed}, err
		},
	}}
	p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString(text)}, khipu.PrepareOptions{}, config)
	if err != nil {
		t.Fatal(err)
	}
	req := paragraph.LayoutRequest{Params: paragraph.Params{Constraints: paragraph.Constraints{Width: 40 * dimen.BP}, Breaker: paragraph.FirstFit, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: 1}}}, Horizontal: &paragraph.HorizontalLayout{}}
	full, err := p.Layout(req)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := p.Trial(paragraph.TrialRequest{Layout: req})
	if err != nil {
		t.Fatal(err)
	}
	choices, err := trial.FitPrefixes(10)
	if err != nil {
		t.Fatal(err)
	}
	a, err := choices.Select(1)
	if err != nil {
		t.Fatal(err)
	}
	var tail *paragraph.PrefixFit
	if width == 40 {
		tail, err = choices.Refit(1, 1000)
	} else {
		req.Params.Constraints.Width = width * dimen.BP
		trial, err = p.Trial(paragraph.TrialRequest{Layout: req, Prefix: a.Checkpoint()})
		if err == nil {
			tail, err = trial.FitPrefixes(1000)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	b, err := tail.Select(tail.Prefixes()[0].Lines)
	if err != nil || b.Checkpoint() != nil {
		t.Fatal("completion", err)
	}
	sealed, face.deny = true, true
	return render.ParagraphInput{Frozen: full.Frozen, VList: full.VList, Text: text, Paint: []style.PaintSpan{{Start: 0, End: 3, Paint: style.Paint{Color: "red"}}, {Start: 3, End: uint64(len(text)), Paint: style.Paint{Color: "navy"}}}}, []*layout.Fragment{a.Fragment(), b.Fragment()}, weak.Make(p)
}

func TestFragmentProofFlowAndCollectedPreparation(t *testing.T) {
	for _, width := range []dimen.DU{40, 30} {
		input, fragments, prepared := fragmentRenderFixture(t, width)
		for i := 0; i < 5; i++ {
			runtime.GC()
		}
		if prepared.Value() != nil {
			t.Fatal("proof inputs retain preparation")
		}
		b := &orderCounter{}
		services := render.Services{Backend: b, Identity: b}
		full, err := render.BuildParagraphProof(input, services)
		if err != nil {
			t.Fatal(err)
		}
		glyphCount := 0
		for i, f := range fragments {
			b.orders = 0
			proof, err := render.BuildFragmentProof(render.FragmentInput{Fragment: f, Text: input.Text, Paint: input.Paint}, services)
			if err != nil {
				t.Fatal(err)
			}
			wantOrders := 0
			for _, line := range f.Lines() {
				for k := line.Horizontal.FirstKnot; k < line.Horizontal.FirstKnot+line.Horizontal.KnotCount; k++ {
					if _, ok := f.Frozen().CopyText(k); ok {
						wantOrders++
					}
				}
			}
			if b.orders != wantOrders {
				t.Fatal("directional fragment ordered more than once", b.orders, wantOrders)
			}
			for _, g := range proof.Glyphs {
				want := full.Glyphs[glyphCount]
				if g.GlyphID != want.GlyphID || g.SourceByte != want.SourceByte || g.Cluster != want.Cluster || g.Color != want.Color || g.Path != want.Path || g.Scale != want.Scale {
					t.Fatal("lost glyph/source/paint", g, want)
				}
				if width == 40 {
					line := f.Range().Start + g.Line
					dy := input.VList.Lines()[f.Range().Start].Top
					if g.Line != want.Line-f.Range().Start || g.X != want.X || g.Y+dy != want.Y || line != want.Line {
						t.Fatal("equal-width proof geometry", g, want)
					}
				}
				glyphCount++
			}
			if width == 30 && i == 1 {
				// Locked A ends at byte 9; B fits two words per line, then one.
				if len(proof.Lines) != 3 {
					t.Fatal("narrow suffix", proof.Lines)
				}
				for j, l := range proof.Lines {
					if l.TargetWidth != 30 || l.Top != float64(j)*10 || l.Baseline != float64(j)*10+8 || l.Height != 10 {
						t.Fatal("narrow geometry", l)
					}
				}
				for j, g := range proof.Glyphs {
					if g.Line != j/4 || g.X != []float64{0, 5, 15, 20}[j%4] || g.Y != float64(j/4)*10+8 {
						t.Fatal("independent narrow glyph geometry", g, j)
					}
				}
			}
			original := *proof
			proof.Glyphs = nil
			again, err := render.BuildFragmentProof(render.FragmentInput{Fragment: f, Text: input.Text, Paint: input.Paint}, services)
			if err != nil || !reflect.DeepEqual(&original, again) {
				t.Fatal("render changed retained output", err)
			}
		}
		if glyphCount != len(full.Glyphs) {
			t.Fatal("prefix repeated or suffix lost", glyphCount)
		}
		if width == 30 && fragments[0].Frozen() == fragments[1].Frozen() {
			t.Fatal("fixture needs multiple owners")
		}
		runtime.KeepAlive(fragments)
	}
}

func TestFragmentProofBidiEmptyAndFailures(t *testing.T) {
	input, face := proofInput(t)
	f, err := paragraph.Fragment(input.VList)
	if err != nil {
		t.Fatal(err)
	}
	face.deny = true
	b := &backend{}
	services := render.Services{Backend: b, Identity: b}
	want, err := render.BuildParagraphProof(input, services)
	if err != nil {
		t.Fatal(err)
	}
	arg := render.FragmentInput{Fragment: f, Text: input.Text, Paint: input.Paint}
	got, err := render.BuildFragmentProof(arg, services)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("bidi/paint changed", err)
	}
	empty, err := layout.NewFragment(f.Frozen(), layout.LineRange{}, layout.Box{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b.calls = 0
	p, err := render.BuildFragmentProof(render.FragmentInput{Fragment: empty, Text: input.Text}, services)
	if err != nil || len(p.Lines) != 0 || len(p.Glyphs) != 0 || b.calls != 0 {
		t.Fatal("empty fragment rendered owner", err)
	}
	sentinel := errors.New("late outline")
	{
		b := &backend{fail: sentinel}
		want := sentinel
		p, err := render.BuildFragmentProof(arg, render.Services{Backend: b, Identity: b})
		if p != nil || !errors.Is(err, want) || b.calls != 2 {
			t.Fatal("late failure published proof", p, err, b.calls)
		}
	}
	for _, f := range []*layout.Fragment{nil, {}} {
		if p, err := render.BuildFragmentProof(render.FragmentInput{Fragment: f}, services); p != nil || err == nil {
			t.Fatal("invalid fragment accepted")
		}
	}
}
