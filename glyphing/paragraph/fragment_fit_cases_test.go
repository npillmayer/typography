package paragraph_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

type fitServiceCalls struct {
	sealed atomic.Bool
	counts [7]atomic.Int64 // resolution, factory, shaping, acceptance, metrics, scale, conversion
}

func (c *fitServiceCalls) hit(i int) {
	c.counts[i].Add(1)
	if c.sealed.Load() {
		panic("service called after initial layout")
	}
}
func (c *fitServiceCalls) snapshot() (n [7]int64) {
	for i := range n {
		n[i] = c.counts[i].Load()
	}
	return
}

type fitCountedFont struct {
	contracts.FontFace
	calls *fitServiceCalls
}

func (f fitCountedFont) Metrics() (contracts.FontMetrics, error) {
	f.calls.hit(4)
	return f.FontFace.Metrics()
}
func (f fitCountedFont) Scale(em dimen.DU) (contracts.FontScale, error) {
	f.calls.hit(5)
	s, e := f.FontFace.Scale(em)
	return fitCountedScale{s, f.calls}, e
}

type fitCountedScale struct {
	contracts.FontScale
	calls *fitServiceCalls
}

func (s fitCountedScale) ToDU(v int32) float64 { s.calls.hit(6); return s.FontScale.ToDU(v) }

type fitSized dimen.DU

func (s fitSized) Equals(o styled.Style) bool { v, ok := o.(fitSized); return ok && v == s }
func (s fitSized) String() string             { return "fit size" }

type fitCountedResolver struct{ calls *fitServiceCalls }

func (r fitCountedResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	r.calls.hit(0)
	if em, ok := s.(fitSized); ok {
		d.EmSize = dimen.DU(em)
		return d, nil
	}
	return helpers.DefaultStyleResolver{}.ResolveStyle(s, d)
}

type fitCountedStream struct {
	*helpers.SyntheticStreamingShaper
	calls *fitServiceCalls
}

func (s fitCountedStream) CheckFace(f contracts.FontFace) error {
	s.calls.hit(3)
	if v, ok := f.(fitCountedFont); ok {
		f = v.FontFace
	}
	return s.SyntheticStreamingShaper.CheckFace(f)
}
func (s fitCountedStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	s.calls.hit(2)
	p.Font, _ = contracts.NewFontInstance(helpers.NewSyntheticFont(), p.Font.EmSize())
	return s.SyntheticStreamingShaper.Shape(ctx, p, r, o)
}
func fitFixture(t *testing.T, input paragraph.Input, breaker paragraph.Breaker, width dimen.DU, factor float64, boxes []paragraph.AtomicBox) (*layout.VList, fitReuseKey, *fitServiceCalls) {
	t.Helper()
	c := new(fitServiceCalls)
	cfg := khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver: fitCountedResolver{c}, Defaults: contracts.ResolvedRunStyle{Font: fitCountedFont{helpers.NewSyntheticFont(), c}, EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) {
			c.hit(1)
			s, e := helpers.NewSyntheticStreamingShaper(8)
			return fitCountedStream{s, c}, e
		},
	}}
	p, err := paragraph.Prepare(t.Context(), input, khipu.PrepareOptions{Hyphenation: khipu.NoHyphenation}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Layout(t.Context(), paragraph.LayoutRequest{Params: paragraph.Params{
		Constraints: paragraph.Constraints{Width: width}, Breaker: breaker, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight},
		LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: factor}},
	}, Horizontal: &paragraph.HorizontalLayout{}, Atomics: boxes})
	if err != nil {
		t.Fatal(err)
	}
	c.sealed.Store(true)
	return r.VList, fitReuseKey{width: width, source: 1, styles: 1, fonts: 1, lineHeight: 1, atomics: 1, policy: 1}, c
}
func fitTextInput(s string) paragraph.Input { return paragraph.Input{Text: styled.TextFromString(s)} }
