package integration_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

func TestLayoutPublicationDetachesInputs(t *testing.T) {
	input, face := proofInput(t)
	face.deny = true
	original := input.VList
	lines, materials, atoms := original.Lines(), original.Materials(), original.AtomicPlacements()
	v, err := layout.NewVList(input.Frozen, original.Box(), lines, materials, atoms)
	if err != nil {
		t.Fatal(err)
	}
	lines[0].Baseline++
	materials[0].Extent++
	if !reflect.DeepEqual(v.Lines(), original.Lines()) || !reflect.DeepEqual(v.Materials(), original.Materials()) {
		t.Fatal("publication retained mutable inputs")
	}
	detached := v.Lines()
	detached[0].Top++
	if !reflect.DeepEqual(v.Lines(), original.Lines()) || v.Frozen() != input.Frozen {
		t.Fatal("accessor changed layout")
	}
	// Validate records without asking the retained face for metrics.
	for _, change := range []func(*layout.Box, []layout.Line, []layout.Material){
		func(b *layout.Box, _ []layout.Line, _ []layout.Material) { b.Width = math.Inf(1) },
		func(b *layout.Box, _ []layout.Line, _ []layout.Material) { b.Baseline++ },
		func(_ *layout.Box, l []layout.Line, _ []layout.Material) { l[0].Index++ },
		func(_ *layout.Box, l []layout.Line, _ []layout.Material) { l[0].Horizontal.FirstKnot++ },
		func(_ *layout.Box, l []layout.Line, _ []layout.Material) { l[0].Ascent = math.NaN() },
		func(_ *layout.Box, _ []layout.Line, m []layout.Material) { m[0].Child = 99 },
	} {
		b, l, m := original.Box(), original.Lines(), original.Materials()
		change(&b, l, m)
		if v, err := layout.NewVList(input.Frozen, b, l, m, nil); err == nil || v != nil {
			t.Fatal("published invalid geometry")
		}
	}
	if _, err := layout.NewVList(nil, layout.Box{}, nil, nil, nil); err == nil {
		t.Fatal("nil frozen")
	}
	if _, err := layout.NewVList(input.Frozen, layout.Box{}, nil, nil, nil); err == nil {
		t.Fatal("suppressed nonempty content")
	}
	empty, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("")}, paragraph.Params{
		Constraints: paragraph.Constraints{Width: 100 * dimen.BP}, Breaker: paragraph.FirstFit,
		Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.NormalHeight}},
	}, khipu.Config{Streaming: &khipu.StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP}, NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) }}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := layout.NewVList(empty.Frozen, layout.Box{}, nil, nil, nil); err != nil {
		t.Fatal("empty diagnostic placements", err)
	}
}
