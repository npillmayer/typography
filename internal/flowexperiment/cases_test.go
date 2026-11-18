package flowexperiment

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

type serviceCalls struct {
	sealed atomic.Bool
	counts [7]atomic.Int64 // resolution, factory, shaping, acceptance, metrics, scale, conversion
}

func (c *serviceCalls) hit(i int) {
	c.counts[i].Add(1)
	if c.sealed.Load() {
		panic("service called after initial layout")
	}
}
func (c *serviceCalls) snapshot() (n [7]int64) {
	for i := range n {
		n[i] = c.counts[i].Load()
	}
	return
}

type countedFont struct {
	contracts.FontFace
	calls *serviceCalls
}

func (f countedFont) Metrics() (contracts.FontMetrics, error) {
	f.calls.hit(4)
	return f.FontFace.Metrics()
}
func (f countedFont) Scale(em dimen.DU) (contracts.FontScale, error) {
	f.calls.hit(5)
	s, e := f.FontFace.Scale(em)
	return countedScale{s, f.calls}, e
}

type countedScale struct {
	contracts.FontScale
	calls *serviceCalls
}

func (s countedScale) ToDU(v int32) float64 { s.calls.hit(6); return s.FontScale.ToDU(v) }

type sized dimen.DU

func (s sized) Equals(o styled.Style) bool { v, ok := o.(sized); return ok && v == s }
func (s sized) String() string             { return "experiment size" }

type countedResolver struct{ calls *serviceCalls }

func (r countedResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	r.calls.hit(0)
	if em, ok := s.(sized); ok {
		d.EmSize = dimen.DU(em)
		return d, nil
	}
	return helpers.DefaultStyleResolver{}.ResolveStyle(s, d)
}

type countedStream struct {
	*helpers.SyntheticStreamingShaper
	calls *serviceCalls
}

func (s countedStream) CheckFace(f contracts.FontFace) error {
	s.calls.hit(3)
	if v, ok := f.(countedFont); ok {
		f = v.FontFace
	}
	return s.SyntheticStreamingShaper.CheckFace(f)
}
func (s countedStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	s.calls.hit(2)
	p.Font, _ = contracts.NewFontInstance(helpers.NewSyntheticFont(), p.Font.EmSize())
	return s.SyntheticStreamingShaper.Shape(ctx, p, r, o)
}
func fixture(t *testing.T, input paragraph.Input, breaker paragraph.Breaker, width dimen.DU, factor float64, boxes []paragraph.AtomicBox) (*layout.VList, reuseKey, *serviceCalls) {
	t.Helper()
	c := new(serviceCalls)
	cfg := khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver: countedResolver{c}, Defaults: contracts.ResolvedRunStyle{Font: countedFont{helpers.NewSyntheticFont(), c}, EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) {
			c.hit(1)
			s, e := helpers.NewSyntheticStreamingShaper(8)
			return countedStream{s, c}, e
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
	return r.VList, reuseKey{width: width, source: 1, styles: 1, fonts: 1, lineHeight: 1, atomics: 1, policy: 1}, c
}
func textInput(s string) paragraph.Input { return paragraph.Input{Text: styled.TextFromString(s)} }
