package paragraph_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

func fixedSkip(n int) khipu.KnotCore {
	w := dimen.DU(n) * dimen.BP
	return khipu.KnotCore{W: w, MinW: w, MaxW: w}
}

func atomicBox(id contracts.AtomicID, width, height int) paragraph.AtomicBox {
	return paragraph.AtomicBox{ID: id, BorderWidth: dimen.DU(width) * dimen.BP, BorderHeight: dimen.DU(height) * dimen.BP, Alignment: paragraph.AlignBottomMarginEdge}
}

func mixedParams(b paragraph.Breaker, width dimen.DU) paragraph.Params {
	p := cssParams(b, paragraph.LineHeight{Kind: paragraph.NormalHeight})
	p.Constraints.Width = width
	return p
}

func requireStage(t *testing.T, result any, err error, stage paragraph.Stage, cause error) {
	t.Helper()
	var e *paragraph.Error
	if (result != nil && !reflect.ValueOf(result).IsNil()) || !errors.As(err, &e) || e.Stage != stage || !errors.Is(err, cause) {
		t.Fatalf("result=%v error=%v; want %s wrapping %v", result, err, stage, cause)
	}
}

func TestPreparedReuseAndDetachedResults(t *testing.T) {
	text := styled.TextFromString("ab \uFFFC cd \uFFFC ef")
	text, err := text.Style(sizeStyle{20 * dimen.BP, 17}, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	anchors := []contracts.AtomicAnchor{{ID: 9, ByteOffset: 3}, {ID: 2, ByteOffset: 10}}
	cfg := synthetic()
	cfg.Streaming.Resolver = resolver{}
	created := 0
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) { created++; return helpers.NewSyntheticStreamingShaper(8) }
	cfg.Params = new(khipu.Params)
	*cfg.Params = khipu.DefaultParams()
	cfg.ParFillSkip = &khipu.KnotCore{MaxW: dimen.Fill}
	markupCalls, prepared := 0, false
	input := paragraph.Input{Text: text, Atomics: anchors, Markup: func(at uint64) int {
		if prepared {
			t.Fatal("markup retained beyond Prepare")
		}
		markupCalls++
		if at == 7 {
			return bidi.MarkupRLI
		}
		if at == 9 {
			return bidi.MarkupPDI
		}
		return 0
	}}
	options := khipu.PrepareOptions{AtomicBoundaries: []khipu.AtomicBoundary{{ByteOffset: 6, Action: khipu.ProhibitBoundary}}}
	p, err := paragraph.Prepare(t.Context(), input, options, cfg)
	if err != nil {
		t.Fatal(err)
	}
	prepared = true
	if markupCalls == 0 || created != 1 {
		t.Fatal(markupCalls, created)
	}
	anchors[0].ID = 100
	options.AtomicBoundaries[0].Action = 99
	input.Text = styled.TextFromString("changed")
	cfg.Streaming.Defaults.EmSize = 0
	cfg.Streaming.Resolver = nil
	cfg.Params.SpaceStretchPercent = -1
	cfg.ParFillSkip.W = dimen.Fil
	request := paragraph.LayoutRequest{Params: mixedParams(paragraph.FirstFit, 100*dimen.BP), Horizontal: &paragraph.HorizontalLayout{}, Atomics: []paragraph.AtomicBox{atomicBox(2, 8, 12), atomicBox(9, 6, 4)}}
	first, err := p.Layout(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	lines, placements, box := first.VList.Lines(), first.VList.AtomicPlacements(), first.VList.Box()
	runs := slices.Clone(first.BidiRuns)
	var glyphs []khipu.FrozenText
	for k := 0; k < first.Frozen.KnotCount(); k++ {
		tx, _ := first.Frozen.CopyText(k)
		glyphs = append(glyphs, tx)
	}
	first.BidiRuns[0].To = 999
	first.BreakPositions[0] = 999
	copy := first.VList.AtomicPlacements()
	copy[0].X = 999
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, width := range []dimen.DU{25 * dimen.BP, 80 * dimen.BP, 0} {
			request.Params = mixedParams(breaker, width)
			request.Atomics[0].BorderWidth = 20 * dimen.BP
			request.Atomics[0].BorderHeight = 25 * dimen.BP
			out, err := p.Layout(t.Context(), request)
			if err != nil {
				t.Fatal(breaker, width, err)
			}
			if out.Frozen == first.Frozen || out.Frozen.SourceLen() != text.Raw().Len() || len(out.VList.AtomicPlacements()) != 2 || !reflect.DeepEqual(out.BidiRuns, runs) {
				t.Fatal("reused mutable output")
			}
		}
	}
	if created != 1 || !reflect.DeepEqual(lines, first.VList.Lines()) || !reflect.DeepEqual(placements, first.VList.AtomicPlacements()) || box != first.VList.Box() {
		t.Fatal("reshaped or changed an old result", created)
	}
	for k, want := range glyphs {
		got, _ := first.Frozen.CopyText(k)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("changed frozen glyphs")
		}
	}
}

func TestPreparedAtomicGeometryOracles(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, tc := range []struct {
			text   string
			offset uint64
			x      float64
		}{{"\uFFFC", 0, 6}, {"aa \uFFFC", 3, 21}} {
			text := styled.TextFromString(tc.text)
			// This private style is intentionally unusable by the resolver. An
			// attached object needs neither a text font nor text half-leading.
			text, err := text.Style(sizeStyle{0, 999}, tc.offset, tc.offset+3)
			if err != nil {
				t.Fatal(err)
			}
			cfg := synthetic()
			cfg.Streaming.Resolver = resolver{}
			p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: text, Atomics: []contracts.AtomicAnchor{{ID: 1, ByteOffset: tc.offset}}}, khipu.PrepareOptions{}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			box := atomicBox(1, 20, 12)
			box.Margin = paragraph.AtomicMargins{Top: 3 * dimen.BP, Bottom: 2 * dimen.BP, Left: 4 * dimen.BP, Right: 5 * dimen.BP}
			request := paragraph.LayoutRequest{Params: mixedParams(breaker, 100*dimen.BP), Horizontal: &paragraph.HorizontalLayout{LeftSkip: fixedSkip(2), RightSkip: fixedSkip(3)}, Atomics: []paragraph.AtomicBox{box}}
			request.Params.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: tc.offset, End: tc.offset + 3, Height: length(200 * dimen.BP)}}
			out, err := p.Layout(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			line := out.VList.Lines()[0]
			if line.Baseline != 17 || line.Height != 19 || line.Ascent != 17 || line.Descent != 2 || line.Horizontal.LeftSkip != 2*dimen.BP || line.Horizontal.RightSkip != 3*dimen.BP {
				t.Fatal(line)
			}
			want := []layout.AtomicPlacement{{ID: 1, Line: 0, X: tc.x, Y: 3}}
			if !reflect.DeepEqual(out.VList.AtomicPlacements(), want) {
				t.Fatal(out.VList.AtomicPlacements(), want)
			}
			for i := 0; i < out.Frozen.KnotCount(); i++ {
				if _, ok := out.Frozen.Atomic(i); ok {
					knot, _ := out.Frozen.Knot(i)
					if knot.W != 29*dimen.BP {
						t.Fatal("edges counted incorrectly", knot)
					}
				}
			}

		}
	}
}

func TestPreparedAtomicSignedAndVisualOrder(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, direction := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
			p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("\uFFFC\uFFFC\uFFFC"), Direction: direction, Atomics: []contracts.AtomicAnchor{{ID: 9}, {ID: 2, ByteOffset: 3}, {ID: 7, ByteOffset: 6}}}, khipu.PrepareOptions{}, synthetic())
			if err != nil {
				t.Fatal(err)
			}
			boxes := []paragraph.AtomicBox{atomicBox(7, 0, 0), atomicBox(2, 0, 2), atomicBox(9, 20, 0)}
			boxes[1].Margin.Right = -5 * dimen.BP
			boxes[1].Margin.Top = -6 * dimen.BP // Signed above-baseline contribution -4bp.
			out, err := p.Layout(t.Context(), paragraph.LayoutRequest{Params: mixedParams(breaker, 100*dimen.BP), Horizontal: &paragraph.HorizontalLayout{}, Atomics: boxes})
			if err != nil {
				t.Fatal(err)
			}
			want := []layout.AtomicPlacement{{ID: 9, X: 0, Y: 8}, {ID: 2, X: 20, Y: 6}, {ID: 7, X: 15, Y: 8}}
			if direction == bidi.RightToLeft {
				// The 85bp paragraph fill precedes the objects in physical RTL order.
				want = []layout.AtomicPlacement{{ID: 7, X: 85, Y: 8}, {ID: 2, X: 85, Y: 6}, {ID: 9, X: 80, Y: 8}}
			}
			if !reflect.DeepEqual(out.VList.AtomicPlacements(), want) || out.VList.Box().Baseline != 8 || out.VList.Box().Extent != 10 {
				t.Fatal(direction, out.VList.AtomicPlacements(), out.VList.Box())
			}
		}
	}
}

func TestPreparedZeroWidthAndTextOnlyCompatibility(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, value := range []string{"", "aa aa aa", "ab\u00adcd"} {
			cfg := synthetic()
			cfg.Hyphenator = softHyphens{}
			input := paragraph.Input{Text: styled.TextFromString(value)}
			prepared, err := paragraph.Prepare(t.Context(), input, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			for _, width := range []dimen.DU{0, 15 * dimen.BP, 100 * dimen.BP} {
				p := params(breaker, width)
				got, err := prepared.Layout(t.Context(), paragraph.LayoutRequest{Params: p})
				if err != nil {
					t.Fatal(err)
				}
				want, err := paragraph.Run(t.Context(), input, p, cfg)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got.BreakPositions, want.BreakPositions) || !reflect.DeepEqual(got.VList.Lines(), want.VList.Lines()) || got.VList.Box() != want.VList.Box() || len(got.VList.AtomicPlacements()) != 0 {
					t.Fatal("legacy text geometry changed", breaker, value, width)
				}
			}
		}
		p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("\uFFFC"), Atomics: []contracts.AtomicAnchor{{ID: 1}}}, khipu.PrepareOptions{}, synthetic())
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{0, 4} {
			out, err := p.Layout(t.Context(), paragraph.LayoutRequest{Params: mixedParams(breaker, 0), Atomics: []paragraph.AtomicBox{atomicBox(1, width, 0)}})
			if err != nil || len(out.VList.AtomicPlacements()) != 1 || out.VList.Box().Extent != 10 {
				t.Fatal(out, err)
			}
			line, _ := out.Frozen.Line(0)
			if width > 0 && line.Fit != khipu.FitOverfull {
				t.Fatal("lost overfull diagnostic", line)
			}
		}
	}
}

func TestPreparedStageErrorsAndRecovery(t *testing.T) {
	input := paragraph.Input{Text: styled.TextFromString("\uFFFC"), Atomics: []contracts.AtomicAnchor{{ID: 1}}}
	p, err := paragraph.Prepare(t.Context(), input, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	request := paragraph.LayoutRequest{Params: mixedParams(paragraph.FirstFit, 100*dimen.BP), Atomics: []paragraph.AtomicBox{atomicBox(1, 2, 3)}}
	out, err := paragraph.Run(t.Context(), input, request.Params, synthetic())
	requireStage(t, out, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	bad, err := paragraph.Prepare(nil, input, khipu.PrepareOptions{}, synthetic())
	requireStage(t, bad, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	input.Atomics[0].ByteOffset = 1
	bad, err = paragraph.Prepare(t.Context(), input, khipu.PrepareOptions{}, synthetic())
	requireStage(t, bad, err, paragraph.StagePrepare, khipu.ErrIllegalArguments)
	input.Atomics[0].ByteOffset = 0
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err = p.Layout(ctx, request)
	requireStage(t, out, err, paragraph.StageInput, context.Canceled)
	out, err = p.Layout(nil, request)
	requireStage(t, out, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	var nilPrepared *paragraph.Prepared
	out, err = nilPrepared.Layout(t.Context(), request)
	requireStage(t, out, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	out, err = new(paragraph.Prepared).Layout(t.Context(), request)
	requireStage(t, out, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	for _, change := range []func(*paragraph.LayoutRequest){
		func(r *paragraph.LayoutRequest) { r.Atomics = nil },
		func(r *paragraph.LayoutRequest) { r.Atomics = append(r.Atomics, r.Atomics[0]) },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].ID = 9 },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].ID = 0 },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].Alignment = 0 },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].Alignment = 99 },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].BorderWidth = -1 },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].BorderHeight = dimen.Fil },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].Margin.Left = -dimen.Fil },
		func(r *paragraph.LayoutRequest) { r.Atomics[0].Margin.Right = dimen.Fill },
		func(r *paragraph.LayoutRequest) {
			r.Atomics[0].BorderWidth = dimen.Fil - 1
			r.Atomics[0].Margin.Left = 2
		},
		func(r *paragraph.LayoutRequest) {
			r.Atomics[0].BorderHeight = dimen.Fil - 1
			r.Atomics[0].Margin.Top = 2
		},
	} {
		r := request
		r.Atomics = slices.Clone(request.Atomics)
		change(&r)
		out, err = p.Layout(t.Context(), r)
		requireStage(t, out, err, paragraph.StageBind, khipu.ErrIllegalArguments)
	}
	for _, change := range []func(*paragraph.LayoutRequest){
		func(r *paragraph.LayoutRequest) { r.Params.Constraints.Width = -1 },
		func(r *paragraph.LayoutRequest) { r.Params.Breaker = "bad" },
		func(r *paragraph.LayoutRequest) {
			r.Horizontal = &paragraph.HorizontalLayout{LeftSkip: khipu.KnotCore{W: 2, MinW: 3, MaxW: 4}}
		},
	} {
		r := request
		change(&r)
		out, err = p.Layout(t.Context(), r)
		requireStage(t, out, err, paragraph.StageInput, khipu.ErrIllegalArguments)
	}
	negative := request
	negative.Atomics = []paragraph.AtomicBox{atomicBox(1, 0, 0)}
	negative.Atomics[0].Margin.Left = -dimen.BP
	out, err = p.Layout(t.Context(), negative)
	requireStage(t, out, err, paragraph.StageFreeze, khipu.ErrUnsupportedPolicy)
	sentinel := errors.New("backend failed")
	cfg := synthetic()
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) { return nil, sentinel }
	bad, err = paragraph.Prepare(t.Context(), input, khipu.PrepareOptions{}, cfg)
	requireStage(t, bad, err, paragraph.StagePrepare, sentinel)
	ctx, cancel = context.WithCancel(t.Context())
	input.Markup = func(uint64) int { cancel(); return 0 }
	bad, err = paragraph.Prepare(ctx, input, khipu.PrepareOptions{}, synthetic())
	requireStage(t, bad, err, paragraph.StageParagraph, context.Canceled)
	out, err = p.Layout(t.Context(), request)
	if err != nil || len(out.VList.AtomicPlacements()) != 1 {
		t.Fatal("preparation unusable after failed layouts", out, err)
	}
}

func TestPreparedAtomicNegativeExtentsAndFiniteBounds(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		cfg := synthetic()
		cfg.Streaming.Defaults.Font = asymmetricFont{cfg.Streaming.Defaults.Font}
		p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("\uFFFC"), Atomics: []contracts.AtomicAnchor{{ID: 1}}}, khipu.PrepareOptions{}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		box := atomicBox(1, 0, 0)
		box.Margin.Top = -dimen.BP
		request := paragraph.LayoutRequest{Params: cssParams(breaker, factor(0)), Atomics: []paragraph.AtomicBox{box}}
		out, err := p.Layout(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		// The strut contributes (-2,+2), the object (-1,0). Union is (-1,+2),
		// not (0,+2): no per-object clamp may erase its negative contribution.
		if out.VList.Box().Baseline != -1 || out.VList.Box().Extent != 1 || out.VList.AtomicPlacements()[0].Y != -1 {
			t.Fatal(out.VList.Box(), out.VList.AtomicPlacements())
		}
		request.Params = mixedParams(breaker, 100*dimen.BP)
		request.Horizontal = &paragraph.HorizontalLayout{LeftSkip: fixedSkip(2)}
		box = atomicBox(1, 1, 1)
		box.Margin.Left, box.Margin.Right = dimen.Fil-1, -(dimen.Fil - 1)
		request.Atomics = []paragraph.AtomicBox{box}
		out, err = p.Layout(t.Context(), request)
		requireStage(t, out, err, paragraph.StageAssemble, khipu.ErrIllegalArguments) // Published X overflows; advance is only 1bp.
		request.Horizontal = &paragraph.HorizontalLayout{}
		request.Atomics[0].BorderWidth = dimen.Fil - 1
		// Intermediate border+left exceeds int32, but the right margin brings
		// the completed advance back into the finite domain. Border X is legal.
		out, err = p.Layout(t.Context(), request)
		if err != nil || out.VList.AtomicPlacements()[0].X != (dimen.Fil-1).Points() {
			t.Fatal(out, err)
		}
	}
}

type failingDiscretionaries struct{ err error }

func (h failingDiscretionaries) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	return nil, h.err
}

func TestPreparedBreakFailureAndRecovery(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		sentinel := errors.New("discretionary provider failed")
		cfg := synthetic()
		cfg.Hyphenator = failingDiscretionaries{sentinel}
		p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("ab\u00adcd")}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		request := paragraph.LayoutRequest{Params: params(breaker, 100*dimen.BP)}
		first, err := p.Layout(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		before := first.VList.Lines()
		request.Params.Constraints.Width = 15 * dimen.BP
		out, err := p.Layout(t.Context(), request)
		requireStage(t, out, err, paragraph.StageBreak, sentinel)
		request.Params.Constraints.Width = 100 * dimen.BP
		out, err = p.Layout(t.Context(), request)
		if err != nil || !reflect.DeepEqual(before, out.VList.Lines()) || !reflect.DeepEqual(before, first.VList.Lines()) {
			t.Fatal("failed binding contaminated preparation", err)
		}
	}
}

func TestPreparedAssemblyCancellationAndMultipleLines(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		cfg := synthetic()
		ctx, cancel := context.WithCancel(t.Context())
		cfg.Streaming.Defaults.Font = cancelFont{FontFace: cfg.Streaming.Defaults.Font, cancel: cancel}
		p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("\uFFFC\uFFFC"), Atomics: []contracts.AtomicAnchor{{ID: 8}, {ID: 3, ByteOffset: 3}}}, khipu.PrepareOptions{}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		request := paragraph.LayoutRequest{Params: mixedParams(breaker, 10*dimen.BP), Horizontal: &paragraph.HorizontalLayout{}, Atomics: []paragraph.AtomicBox{atomicBox(3, 10, 20), atomicBox(8, 10, 10)}}
		out, err := p.Layout(ctx, request)
		requireStage(t, out, err, paragraph.StageAssemble, context.Canceled)
		// The font keeps cancelling only the previous request's context. A new
		// request must succeed without reshaping or retaining the failed vlist.
		out, err = p.Layout(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		want := []layout.AtomicPlacement{{ID: 8, Line: 0, X: 0, Y: 0}, {ID: 3, Line: 1, X: 0, Y: 12}}
		if !reflect.DeepEqual(out.VList.AtomicPlacements(), want) || out.VList.Box().Extent != 34 {
			t.Fatal(out.VList.AtomicPlacements(), out.VList.Box())
		}
		lines := out.VList.Lines()
		if len(lines) != 2 || lines[0].Baseline != 10 || lines[1].Baseline != 32 {
			t.Fatal(lines)
		}
		request.Atomics[0].BorderHeight = dimen.Fil / 2
		request.Atomics[1].BorderHeight = dimen.Fil / 2
		failed, err := p.Layout(t.Context(), request)
		requireStage(t, failed, err, paragraph.StageAssemble, khipu.ErrIllegalArguments)
		if !reflect.DeepEqual(out.VList.Lines(), lines) {
			t.Fatal("failed assembly changed retained result")
		}
	}
}

func TestPreparedExplicitZeroSkips(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("aa aa aa")}, khipu.PrepareOptions{}, synthetic())
		if err != nil {
			t.Fatal(err)
		}
		request := paragraph.LayoutRequest{Params: params(breaker, 30*dimen.BP)}
		profile, err := p.Layout(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.Horizontal = &paragraph.HorizontalLayout{}
		explicit, err := p.Layout(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := profile.Frozen.Line(0)
		b, _ := explicit.Frozen.Line(0)
		if a.RightSkip <= 0 || b.RightSkip != 0 || b.LeftSkip != 0 {
			t.Fatal("explicit zero skips replaced by profile", a, b)
		}
	}
}

func TestPreparedMultibyteMixedDirectionCoordinates(t *testing.T) {
	const source = "éa\uFFFCfiאב"
	for _, direction := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString(source), Direction: direction, Atomics: []contracts.AtomicAnchor{{ID: 7, ByteOffset: 3}}}, khipu.PrepareOptions{}, synthetic())
		if err != nil {
			t.Fatal(err)
		}
		for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
			for _, width := range []dimen.DU{100 * dimen.BP, 5 * dimen.BP} {
				out, err := p.Layout(t.Context(), paragraph.LayoutRequest{Params: mixedParams(breaker, width), Atomics: []paragraph.AtomicBox{atomicBox(7, 4, 8)}})
				if err != nil {
					t.Fatal(err)
				}
				var clusters []uint32
				for i := 0; i < out.Frozen.KnotCount(); i++ {
					if id, ok := out.Frozen.Atomic(i); ok {
						knot, _ := out.Frozen.Knot(i)
						if id != 7 || knot.Source.Bytes != (khipu.ByteRange{Start: 3, End: 6}) || knot.Source.ScalarStart != 2 {
							t.Fatal(knot, id)
						}
					}
					if tx, ok := out.Frozen.CopyText(i); ok {
						for _, g := range tx.Glyphs {
							clusters = append(clusters, g.Cluster)
						}
					}
				}
				slices.Sort(clusters)
				if !slices.Equal(clusters, []uint32{0, 1, 3, 4, 5, 6}) || len(out.VList.AtomicPlacements()) != 1 {
					t.Fatal("source coordinates changed", clusters)
				}
				reader, err := out.Frozen.SourceReader(khipu.ByteRange{End: uint64(len(source))})
				if err != nil {
					t.Fatal(err)
				}
				bytes, err := io.ReadAll(reader)
				if err != nil || string(bytes) != source {
					t.Fatal("source changed", string(bytes), err)
				}
			}
		}
	}
}
