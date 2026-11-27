package paragraph_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
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

// Admission tests must reject automatic policy before requesting alternatives.
type flowHyphenator struct{}

func (flowHyphenator) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	panic("unexpected discretionary discovery")
}
