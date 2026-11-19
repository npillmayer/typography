package paragraph_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing"
	fontot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/uax/bidi"
)

// Real faces stay concrete only inside the adapter. This instrumentation wraps
// services, never faces, and forwards the original ShapeParams unchanged.
type p6Stream struct {
	contracts.StreamingShaper
	calls *fitServiceCalls
}

func (s p6Stream) CheckFace(f contracts.FontFace) error {
	s.calls.hit(3)
	return s.StreamingShaper.CheckFace(f)
}
func (s p6Stream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	s.calls.hit(2)
	return s.StreamingShaper.Shape(ctx, p, r, o)
}

type p6Fixture struct {
	backend  string
	input    paragraph.Input
	config   khipu.Config
	request  paragraph.LayoutRequest
	prepared *paragraph.Prepared
	seed     *paragraph.Continuation

	calls *fitServiceCalls
	width dimen.DU
}

func p6Input(t testing.TB, backend string, words int, fixed bool) (paragraph.Input, khipu.Config, *fitServiceCalls) {
	t.Helper()
	calls := new(fitServiceCalls)
	vocabulary := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa"}
	if fixed {
		vocabulary = []string{"aa"}
	} else if backend == "hebrew" {
		// Pure Hebrew avoids the separately documented mixed-RTL admission/stall cases.
		vocabulary = []string{"אב", "גד", "הו", "זח", "טי"}
	}
	tokens := make([]string, words)
	for i := range tokens {
		tokens[i] = vocabulary[i%len(vocabulary)]
	}
	raw := strings.Join(tokens, " ")
	if backend == "synthetic" {
		return fitTextInput(raw), flowConfig(calls), calls
	}
	path := "../../../opentype/testdata/fonts/GentiumPlus-R.ttf"
	if backend == "hebrew" {
		path = "../../../opentype/testdata/fonts/NotoSansHebrew-Regular.ttf"
	} else if backend != "gentium" {
		t.Fatal("unknown backend", backend)
	}
	entry, err := (fontot.Services{}).Load(t.Context(), path, nil)
	if err != nil {
		t.Fatal(err) // Missing offline fixtures must never silently skip a sample.
	}
	mapper, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{
		Faces: []glyphing.FaceEntry{entry}, Default: entry.Info.ResourceID, Size: 10 * dimen.BP,
	}, fontot.Services{})
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := mapper.Itemize(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	params := khipu.DefaultParams()
	params.SpaceShrinkPercent, params.SpaceStretchPercent = 0, 0
	config := khipu.Config{Params: &params, Streaming: &khipu.StreamingConfig{
		Resolver: mapper, Defaults: mapper.Defaults(),
		NewShaper: func() (contracts.StreamingShaper, error) {
			calls.hit(1)
			s, err := fontot.NewStream(8)
			return p6Stream{s, calls}, err
		},
	}}
	in := paragraph.Input{Text: text}
	if backend == "hebrew" {
		in.Direction = bidi.RightToLeft
	}
	return in, config, calls
}

func p6Prepare(t testing.TB, input paragraph.Input, config khipu.Config) *paragraph.Prepared {
	t.Helper()
	p, err := paragraph.Prepare(input, khipu.PrepareOptions{Hyphenation: khipu.NoHyphenation}, config)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newP6Fixture(t testing.TB, backend string, words int, fixed bool) *p6Fixture {
	t.Helper()
	input, cfg, calls := p6Input(t, backend, words, fixed)
	p := p6Prepare(t, input, cfg)
	req := flowRequest(paragraph.FirstFit)
	req.Params.Constraints.Width = 300 * dimen.BP
	if fixed {
		req.Params.Constraints.Width = 145 * dimen.BP // Exactly ten synthetic "aa" words.
	}
	req.Params.LineLayout.Default.Factor = 1.2
	seed, err := p.StartFlow(req)
	if err != nil {
		t.Fatal(err)
	}
	return &p6Fixture{backend: backend, input: input, config: cfg, request: req, prepared: p, seed: seed, calls: calls, width: req.Params.Constraints.Width}
}

func p6Sizes(backend string) []int {
	if backend == "synthetic" {
		return []int{100, 1000, 10000}
	}
	return []int{100, 1000}
}
func p6Name(backend string, words int) string { return fmt.Sprintf("%s/N%d", backend, words) }
func (f *p6Fixture) cut(percent int) (int, float64) {
	lines := f.seed.Remainder().Lines()
	n := max(1, min(len(lines)-1, len(lines)*percent/100))
	return n, lines[n-1].Top + lines[n-1].Height
}
