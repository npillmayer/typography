package paragraph_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"runtime"
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

func flowRequest(b paragraph.Breaker) paragraph.LayoutRequest {
	return paragraph.LayoutRequest{Params: paragraph.Params{Constraints: paragraph.Constraints{Width: 40 * dimen.BP}, Breaker: b, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: 1}}}, Horizontal: &paragraph.HorizontalLayout{}}
}
func flowConfig(c *fitServiceCalls) khipu.Config {
	params := khipu.DefaultParams()
	params.SpaceStretchPercent, params.SpaceShrinkPercent = 0, 0
	return khipu.Config{Params: &params, Streaming: &khipu.StreamingConfig{
		Resolver: fitCountedResolver{c}, Defaults: contracts.ResolvedRunStyle{Font: fitCountedFont{helpers.NewSyntheticFont(), c}, EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) {
			c.hit(1)
			s, e := helpers.NewSyntheticStreamingShaper(8)
			return fitCountedStream{s, c}, e
		},
	}}
}
func startFlow(t *testing.T, input paragraph.Input, req paragraph.LayoutRequest) (*paragraph.Prepared, *paragraph.Continuation, *fitServiceCalls) {
	t.Helper()
	c := new(fitServiceCalls)
	p, e := paragraph.Prepare(t.Context(), input, khipu.PrepareOptions{}, flowConfig(c))
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.StartFlow(t.Context(), req)
	if e != nil {
		t.Fatal(e)
	}
	return p, s, c
}
func frame(w dimen.DU, h float64) paragraph.Frame {
	return paragraph.Frame{Width: w * dimen.BP, Height: h}
}
func fitFlow(t *testing.T, c *paragraph.Continuation, f paragraph.Frame) *paragraph.FlowResult {
	t.Helper()
	r, e := c.Fit(t.Context(), f)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func flowEnds(f *layout.Fragment) []uint64 {
	var out []uint64
	for _, l := range f.Lines() {
		out = append(out, l.Horizontal.Source.End)
	}
	return out
}
func noFlowShaping(t *testing.T, before, after [7]int64) {
	t.Helper()
	for _, i := range []int{1, 2, 3} {
		if before[i] != after[i] {
			t.Fatal("base service called", i, before, after)
		}
	}
}
func sameFragment(t *testing.T, a, b *layout.Fragment) {
	t.Helper()
	if a.Frozen() != b.Frozen() || a.Range() != b.Range() || a.Box() != b.Box() || !reflect.DeepEqual(a.Lines(), b.Lines()) || !reflect.DeepEqual(a.Materials(), b.Materials()) || !reflect.DeepEqual(a.AtomicPlacements(), b.AtomicPlacements()) {
		t.Fatal("fragment changed")
	}
}

func TestFlowIndependentOracleAndDispatch(t *testing.T) {
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		_, s, c := startFlow(t, fitTextInput("aa bb cc dd ee ff gg hh ii jj kk ll"), flowRequest(b))
		c.sealed.Store(true)
		a := fitFlow(t, s, frame(40, 10))
		if !a.Reused || a.Breaker != b || a.Status != paragraph.FitContinues {
			t.Fatal(a)
		}
		c.sealed.Store(false)
		before := c.snapshot()
		for _, tc := range []struct {
			width dimen.DU
			want  []uint64
		}{{20, []uint64{12, 15, 18, 21, 24, 27, 30, 33, 35}}, {30, []uint64{15, 21, 27, 33, 35}}, {55, []uint64{21, 33, 35}}} {
			r := fitFlow(t, a.Continuation, frame(tc.width, 1000))
			if r.Reused || r.Continuation != nil || r.Status != paragraph.FitComplete || r.Breaker != paragraph.FirstFit || !slices.Equal(flowEnds(r.Placed), tc.want) {
				t.Fatal(tc, r, flowEnds(r.Placed))
			}
			noFlowShaping(t, before, c.snapshot())
		}
		sameFragment(t, a.Remainder, a.Continuation.Remainder())
	}
}

func TestFlowWidthHistoryAndHeterogeneousReuse(t *testing.T) {
	_, s, c := startFlow(t, fitTextInput("aa bb cc dd ee ff gg hh ii jj kk ll mm nn oo pp"), flowRequest(paragraph.FirstFit))
	a := fitFlow(t, s, frame(40, 10))
	b := fitFlow(t, a.Continuation, frame(30, 10))
	if !slices.Equal(flowEnds(b.Placed), []uint64{15}) {
		t.Fatal(flowEnds(b.Placed))
	}
	// Reuse B's unplaced tail although its frozen owner begins at width A.
	c.sealed.Store(true)
	reused := fitFlow(t, b.Continuation, frame(30, 10))
	if !reused.Reused || reused.Placed.Frozen() != b.Placed.Frozen() {
		t.Fatal("heterogeneous tail rebroken")
	}
	c.sealed.Store(false)
	next := fitFlow(t, reused.Continuation, frame(20, 10))
	for i, w := range []dimen.DU{40, 30, 30, 20} {
		line, _ := next.Placed.Frozen().Line(i)
		if line.TargetWidth != w*dimen.BP {
			t.Fatal("lost width history", i, line)
		}
	}
	// A wider request is still a changed-width search; a following equal frame reuses it.
	wide := fitFlow(t, next.Continuation, frame(55, 10))
	c.sealed.Store(true)
	done := fitFlow(t, wide.Continuation, frame(55, 1000))
	if !done.Reused || done.Continuation != nil {
		t.Fatal(done)
	}
	var end uint64
	fills := 0
	for _, result := range []*paragraph.FlowResult{a, b, reused, next, wide, done} {
		for _, line := range result.Placed.Lines() {
			if line.Horizontal.Source.Start != end {
				t.Fatal("source gap or overlap", line.Horizontal.Source, end)
			}
			end = line.Horizontal.Source.End
			for i := line.Horizontal.FirstKnot; i < line.Horizontal.FirstKnot+line.Horizontal.KnotCount; i++ {
				knot, _ := result.Placed.Frozen().Knot(i)
				if knot.Material == khipu.ParagraphFillMaterial {
					fills++
					if result != done {
						t.Fatal("early final fill")
					}
				}
			}
		}
	}
	if end != done.Placed.Frozen().SourceLen() || fills != 1 {
		t.Fatal("incomplete source or fill", end, fills)
	}
	sameFragment(t, a.Placed, fitFlow(t, s, frame(40, 10)).Placed)
}

func TestFlowDistinctKPPrefix(t *testing.T) {
	c := new(fitServiceCalls)
	cfg := flowConfig(c)
	cfg.Params.SpaceStretchPercent = 50
	cfg.Params.SpaceShrinkPercent = 33.3
	p, e := paragraph.Prepare(t.Context(), fitTextInput("aaaa a a aa aa aaaa aa aaa a a aaa"), khipu.PrepareOptions{}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.StartFlow(t.Context(), flowRequest(paragraph.KnuthPlass))
	if e != nil {
		t.Fatal(e)
	}
	if !slices.Equal(flowEnds(s.Remainder()), []uint64{9, 20, 29, 34}) {
		t.Fatal(flowEnds(s.Remainder()))
	}
	a := fitFlow(t, s, frame(40, 20))
	before := c.snapshot()
	b := fitFlow(t, a.Continuation, frame(30, 1000))
	noFlowShaping(t, before, c.snapshot())
	for i, l := range a.Placed.Lines() {
		other, _ := b.Placed.Frozen().Line(i)
		if !reflect.DeepEqual(l.Horizontal, other) {
			t.Fatal("KP prefix changed")
		}
		for j := l.Horizontal.FirstKnot; j < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; j++ {
			oldKnot, _ := a.Placed.Frozen().Knot(j)
			newKnot, _ := b.Placed.Frozen().Knot(j)
			oldText, _ := a.Placed.Frozen().CopyText(j)
			newText, _ := b.Placed.Frozen().CopyText(j)
			if !reflect.DeepEqual(oldKnot, newKnot) || !reflect.DeepEqual(oldText, newText) {
				t.Fatal("KP glyph prefix changed", j)
			}
		}
	}
}

func TestFlowBidiAtomicsAndFullHeights(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		input := fitTextInput("aa\n\uFFFC\uFFFC aa bb cc dd ee\n\uFFFC")
		input.Direction = dir
		input.Atomics = []contracts.AtomicAnchor{{ID: 7, ByteOffset: 3}, {ID: 2, ByteOffset: 6}, {ID: 9, ByteOffset: 25}}
		req := flowRequest(paragraph.FirstFit)
		req.Atomics = []paragraph.AtomicBox{{ID: 7, Margin: paragraph.AtomicMargins{Right: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge}, {ID: 2, BorderWidth: 4 * dimen.BP, BorderHeight: 30 * dimen.BP, Margin: paragraph.AtomicMargins{Left: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge}, {ID: 9, Alignment: paragraph.AlignBottomMarginEdge}}
		_, s, c := startFlow(t, input, req)
		a := fitFlow(t, s, frame(40, 10))
		before := c.snapshot()
		req.Atomics[0].BorderHeight = dimen.Infinity
		req.Params.LineLayout.Default.Factor = 99
		req.Horizontal.LeftSkip.W = dimen.Infinity
		b := fitFlow(t, a.Continuation, frame(30, 32))
		if len(b.Placed.Lines()) != 1 || b.Placed.Lines()[0].Height != 32 || len(b.Placed.AtomicPlacements()) != 2 {
			t.Fatal("lost atomic height", b)
		}
		noFlowShaping(t, before, c.snapshot())
		c.sealed.Store(true)
		_ = fitFlow(t, b.Continuation, frame(30, 1000))
	}
	for _, factor := range []float64{0, .25, .731, 1} {
		input := fitTextInput("abc \u2067אב 12\nגד הו\u2069 (xyz) aa bb cc dd ee ff")
		req := flowRequest(paragraph.FirstFit)
		req.Params.LineLayout.Default.Factor = factor
		_, s, c := startFlow(t, input, req)
		a := fitFlow(t, s, frame(40, 10*factor))
		if a.Continuation == nil {
			if factor != 0 {
				t.Fatal("unexpected completion")
			}
			continue
		}
		before := c.snapshot()
		b := fitFlow(t, a.Continuation, frame(30, 1000))
		noFlowShaping(t, before, c.snapshot())
		if b.Placed.Lines()[0].Height != 10*factor {
			t.Fatal("lost line height")
		}
	}
}

type flowFailResolver struct {
	fitCountedResolver
	fail  *bool
	cause error
}

func (r flowFailResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if *r.fail {
		return d, r.cause
	}
	return r.fitCountedResolver.ResolveStyle(s, d)
}

func TestFlowFailuresCancellationAndLifecycle(t *testing.T) {
	c := new(fitServiceCalls)
	cfg := flowConfig(c)
	failed := false
	cause := errors.New("late assembly failure")
	cfg.Streaming.Resolver = flowFailResolver{fitCountedResolver{c}, &failed, cause}
	p, e := paragraph.Prepare(t.Context(), fitTextInput("aa bb cc dd ee ff gg hh ii jj kk ll"), khipu.PrepareOptions{}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.StartFlow(t.Context(), flowRequest(paragraph.FirstFit))
	if e != nil {
		t.Fatal(e)
	}
	a := fitFlow(t, s, frame(40, 10))
	token := a.Continuation
	original := token.Remainder()
	for _, w := range []dimen.DU{40, 30} {
		probe := &fitExpiringContext{Context: t.Context()}
		if _, e := token.Fit(probe, frame(w, 10)); e != nil {
			t.Fatal(e)
		}
		for at := 1; at <= probe.calls; at++ {
			ctx := &fitExpiringContext{Context: t.Context(), limit: at}
			out, e := token.Fit(ctx, frame(w, 10))
			if out != nil || !errors.Is(e, context.Canceled) {
				t.Fatal("partial cancellation", at, e)
			}
		}
		sameFragment(t, original, token.Remainder())
	}
	failed = true
	out, e := token.Fit(t.Context(), frame(30, 10))
	var stage *paragraph.Error
	if out != nil || !errors.Is(e, cause) || !errors.As(e, &stage) || stage.Stage != paragraph.StageAssemble {
		t.Fatal("lost late failure", e)
	}
	failed = false
	before := c.snapshot()
	done := fitFlow(t, token, frame(30, 1000))
	noFlowShaping(t, before, c.snapshot())
	if done.Continuation != nil {
		t.Fatal("completed result owns preparation")
	}
	clear(done.Overfull)
	clear(done.Placed.Lines())
	clear(done.Placed.AtomicPlacements())
	p = nil
	s = nil
	token = nil
	a = nil
	runtime.GC()
	count := 0
	for _, l := range done.Placed.Lines() {
		for i := l.Horizontal.FirstKnot; i < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; i++ {
			tx, ok := done.Placed.Frozen().CopyText(i)
			if ok {
				count += len(tx.Glyphs)
			}
		}
	}
	if count == 0 {
		t.Fatal("lost content")
	}
}

func TestFlowInvalidAndEmptyFrames(t *testing.T) {
	_, s, _ := startFlow(t, fitTextInput("aa bb cc dd ee ff"), flowRequest(paragraph.FirstFit))
	for _, f := range []paragraph.Frame{{Width: -1}, {Width: dimen.Fil}, {Height: -1}, {Height: math.NaN()}, {Height: math.Inf(1)}, {Height: dimen.Fil.Points()}} {
		if out, e := s.Fit(t.Context(), f); out != nil || !errors.Is(e, contracts.ErrIllegalArguments) {
			t.Fatal(out, e)
		}
	}
	for _, c := range []*paragraph.Continuation{nil, {}} {
		if c.Remainder() != nil {
			t.Fatal("zero token")
		}
		if out, e := c.Fit(t.Context(), frame(40, 10)); out != nil || !errors.Is(e, contracts.ErrIllegalArguments) {
			t.Fatal(out, e)
		}
	}
	if out, e := s.Fit(nil, frame(40, 10)); out != nil || e == nil {
		t.Fatal(out, e)
	}
	no := fitFlow(t, s, frame(30, 0))
	if no.Status != paragraph.FitNoProgress || no.Continuation == nil {
		t.Fatal(no)
	}
	over := fitFlow(t, no.Continuation, frame(0, 1000))
	if len(over.Overfull) == 0 || over.Status != paragraph.FitComplete {
		t.Fatal(over)
	}
	_, empty, c := startFlow(t, fitTextInput(""), flowRequest(paragraph.FirstFit))
	c.sealed.Store(true)
	done := fitFlow(t, empty, frame(20, 0))
	if done.Status != paragraph.FitComplete || done.Continuation != nil || done.Placed.Range().End != 0 || done.Placed.Frozen().LineCount() == 0 {
		t.Fatal(done)
	}
}
