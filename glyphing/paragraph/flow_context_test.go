package paragraph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/uax/bidi"
)

type flowHyphenator struct{}

func (flowHyphenator) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	panic("flow must reject hyphenation")
}
func TestFlowAdmission(t *testing.T) {
	for _, mode := range []khipu.HyphenationMode{khipu.ExplicitHyphenation, khipu.AutomaticHyphenation} {
		cfg := flowConfig(new(fitServiceCalls))
		cfg.Hyphenator = flowHyphenator{}
		p, e := paragraph.Prepare(t.Context(), fitTextInput("ab\u00adcd"), khipu.PrepareOptions{Hyphenation: mode}, cfg)
		if e != nil {
			t.Fatal(e)
		}
		if s, e := p.StartFlow(t.Context(), flowRequest(paragraph.FirstFit)); s != nil || !errors.Is(e, khipu.ErrUnsupportedPolicy) {
			t.Fatal(s, e)
		}
	}
	for _, p := range []*paragraph.Prepared{nil, {}} {
		if s, e := p.StartFlow(t.Context(), flowRequest(paragraph.FirstFit)); s != nil || !errors.Is(e, contracts.ErrIllegalArguments) {
			t.Fatal(s, e)
		}
	}
	p, _, _ := startFlow(t, fitTextInput("aa bb"), flowRequest(paragraph.FirstFit))
	if s, e := p.StartFlow(nil, flowRequest(paragraph.FirstFit)); s != nil || e == nil {
		t.Fatal(s, e)
	}
	probe := &fitExpiringContext{Context: t.Context()}
	if _, e := p.StartFlow(probe, flowRequest(paragraph.FirstFit)); e != nil {
		t.Fatal(e)
	}
	for at := 1; at <= probe.calls; at++ {
		ctx := &fitExpiringContext{Context: t.Context(), limit: at}
		if s, e := p.StartFlow(ctx, flowRequest(paragraph.FirstFit)); s != nil || !errors.Is(e, context.Canceled) {
			t.Fatal(at, s, e)
		}
	}
}

type flowFeatureStream struct{ fitCountedStream }

func (s flowFeatureStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	p.Features = nil
	return s.fitCountedStream.Shape(ctx, p, r, o)
}
func TestFlowRetainsMarkupFeaturesAndSizes(t *testing.T) {
	c := new(fitServiceCalls)
	cfg := flowConfig(c)
	cfg.Streaming.Defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'l', 'i', 'g', 'a'}, Enabled: true, Value: 1, Start: 2, End: 10}}
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
		c.hit(1)
		s, e := helpers.NewSyntheticStreamingShaper(8)
		return flowFeatureStream{fitCountedStream{s, c}}, e
	}
	input := fitTextInput("aa bb cc dd ee ff gg hh ii jj")
	p, e := paragraph.Prepare(t.Context(), input, khipu.PrepareOptions{}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.StartFlow(t.Context(), flowRequest(paragraph.FirstFit))
	if e != nil {
		t.Fatal(e)
	}
	a := fitFlow(t, s, frame(40, 10))
	before := c.snapshot()
	b := fitFlow(t, a.Continuation, frame(20, 1000))
	noFlowShaping(t, before, c.snapshot())
	found := false
	for _, l := range b.Placed.Lines() {
		for i := l.Horizontal.FirstKnot; i < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; i++ {
			tx, ok := b.Placed.Frozen().CopyText(i)
			if ok && len(tx.Context.Params.Features) > 0 {
				found = true
				if tx.Context.Params.Features[0].Start != 2 || tx.Context.Params.Features[0].End != 10 || tx.Context.Input.Bytes.Start != 0 {
					t.Fatal("feature domain rebased")
				}
			}
		}
	}
	if !found {
		t.Fatal("missing feature fixture")
	}
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		input := fitTextInput("aa bb \uFFFC אב גד הו זח טי כל מנ")
		input.Atomics = []contracts.AtomicAnchor{{ID: 1, ByteOffset: 6}}
		calls := 0
		input.Markup = func(at uint64) int {
			calls++
			if at == 10 {
				return bidi.MarkupRLI
			}
			if at == input.Text.Raw().Len() {
				return bidi.MarkupPDI
			}
			return 0
		}
		req := flowRequest(breaker)
		req.Horizontal = nil
		req.Params.Constraints.Width = 60 * dimen.BP
		req.Atomics = []paragraph.AtomicBox{{ID: 1, BorderWidth: 5 * dimen.BP, BorderHeight: 22 * dimen.BP, Alignment: paragraph.AlignBottomMarginEdge}}
		_, s, c := startFlow(t, input, req)
		n := calls
		a := fitFlow(t, s, frame(60, s.Remainder().Lines()[0].Height))
		before := c.snapshot()
		_ = fitFlow(t, a.Continuation, frame(30, 1000))
		noFlowShaping(t, before, c.snapshot())
		if calls != n {
			t.Fatal("bidi/markup rerun")
		}
	}
	input = fitTextInput("aa\ncc dd ee ff gg hh")
	input.Text, e = input.Text.Style(fitSized(20*dimen.BP), 3, input.Text.Raw().Len())
	if e != nil {
		t.Fatal(e)
	}
	req := flowRequest(paragraph.FirstFit)
	req.Params.Constraints.Width = 80 * dimen.BP
	_, s, c = startFlow(t, input, req)
	a = fitFlow(t, s, frame(80, 10))
	before = c.snapshot()
	b = fitFlow(t, a.Continuation, frame(40, 20))
	noFlowShaping(t, before, c.snapshot())
	if len(b.Placed.Lines()) != 1 || b.Placed.Lines()[0].Height != 20 {
		t.Fatal("lost space/size metrics")
	}
}
