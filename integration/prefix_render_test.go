package integration_test

import (
	"reflect"
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/typography/render"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"golang.org/x/text/language"
)

const prefixProofText = "fi fi fi fi fi fi fi"

func prefixProofFixture(t *testing.T) ([]*layout.Fragment, []paragraph.SourceCoverage, weak.Pointer[paragraph.Prepared]) {
	t.Helper()
	face := fixture(t)
	streams := 0
	cfg := khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{},
		Defaults: contracts.ResolvedRunStyle{Font: face, EmSize: 10 * dimen.BP, Script: language.MustParseScript("Latn"), Language: language.English,
			Features: []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Enabled: true, Value: 1, Start: 0, End: 2}}},
		NewShaper: func() (contracts.StreamingShaper, error) { streams++; return glyphot.NewStream(8) },
	}}
	p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString(prefixProofText)}, khipu.PrepareOptions{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	preparedStreams := streams
	r := paragraph.TrialRequest{Layout: paragraph.LayoutRequest{Params: paragraph.Params{
		Constraints: paragraph.Constraints{Width: 25 * dimen.BP}, Breaker: paragraph.FirstFit, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight},
		LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: 1}},
	}, Horizontal: &paragraph.HorizontalLayout{}}}
	a, err := p.Trial(r)
	if err != nil {
		t.Fatal(err)
	}
	fit, err := a.FitPrefixes(1000)
	if err != nil {
		t.Fatal(err)
	}
	if fit.Prefixes()[0].Lines < 2 {
		t.Fatal("fixture requires a suffix")
	}
	first, err := fit.Select(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Coverage()) == 0 || first.Coverage()[0].Source != (khipu.ByteRange{End: 2}) || first.Coverage()[0].Kind != paragraph.GlyphCoverage {
		t.Fatalf("expected indivisible fi cluster: %+v", first.Coverage())
	}
	// A link over just the i (byte 1) overlaps that whole cluster; no glyph or
	// break is fabricated at its interior boundary.
	ownerFound := false
	for _, c := range first.Coverage() {
		if c.Kind == paragraph.GlyphCoverage && c.Source.Start < 2 && 1 < c.Source.End {
			ownerFound = true
		}
	}
	if !ownerFound {
		t.Fatal("ligature interior lost source ownership")
	}
	r.Prefix = first.Checkpoint()
	r.Layout.Params.Constraints.Width = 15 * dimen.BP
	b, err := p.Trial(r)
	if err != nil {
		t.Fatal(err)
	}
	next, err := b.FitPrefixes(1000)
	if err != nil {
		t.Fatal(err)
	}
	last, err := next.Select(next.Prefixes()[0].Lines)
	if err != nil {
		t.Fatal(err)
	}
	if last.Info().Status != paragraph.FitComplete || last.Info().From != first.Info().To || streams != preparedStreams {
		t.Fatal("suffix repeated preparation or lost source")
	}
	return []*layout.Fragment{first.Fragment(), last.Fragment()}, first.Coverage(), weak.Make(p)
}

func TestPrefixRealLigatureAndRetainedRendering(t *testing.T) {
	fragments, coverage, prepared := prefixProofFixture(t)
	for range 5 {
		runtime.GC()
	}
	if prepared.Value() != nil {
		t.Fatal("frozen prefix retains preparation")
	}
	backend := renderot.New()
	services := render.Services{Backend: backend, Identity: backend}
	var end uint64
	glyphs := 0
	for _, f := range fragments {
		arg := render.FragmentInput{Fragment: f, Text: prefixProofText}
		proof, err := render.BuildFragmentProof(arg, services)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range f.Lines() {
			if line.Horizontal.Source.Start != end {
				t.Fatal("source gap")
			}
			end = line.Horizontal.Source.End
		}
		for _, g := range proof.Glyphs {
			if g.GlyphID == 6 && g.SourceByte == 0 {
				glyphs++
			}
		}
		again, err := render.BuildFragmentProof(arg, services)
		if err != nil || !reflect.DeepEqual(proof, again) {
			t.Fatal("retained proof changed", err)
		}
	}
	if end != uint64(len(prefixProofText)) || glyphs != 1 || coverage[0].Source != (khipu.ByteRange{End: 2}) {
		t.Fatal("lost or duplicated ligature/material", end, glyphs)
	}
	runtime.KeepAlive(fragments)
}
