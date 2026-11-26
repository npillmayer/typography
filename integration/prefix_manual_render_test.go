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

type prefixProofHyphens struct{}

func (prefixProofHyphens) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

const manualProofText = "co\u00adoperate more text for another frame"

func manualProofFragments(t *testing.T) ([]*layout.Fragment, weak.Pointer[paragraph.Prepared]) {
	t.Helper()
	face, e := glyphot.LoadFile("../../opentype/testdata/fonts/GentiumPlus-R.ttf")
	if e != nil {
		t.Fatal(e)
	}
	streams := 0
	cfg := khipu.Config{Hyphenator: prefixProofHyphens{}, Streaming: &khipu.StreamingConfig{Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: face, EmSize: 10 * dimen.BP, Script: language.MustParseScript("Latn"), Language: language.English}, NewShaper: func() (contracts.StreamingShaper, error) { streams++; return glyphot.NewStream(8) }}}
	p, e := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString(manualProofText)}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
	if e != nil {
		t.Fatal(e)
	}
	r := paragraph.TrialRequest{Layout: paragraph.LayoutRequest{Params: paragraph.Params{Constraints: paragraph.Constraints{Width: 18 * dimen.BP}, Breaker: paragraph.FirstFit, Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: 1}}}, Horizontal: &paragraph.HorizontalLayout{}}}
	a, e := p.Trial(r)
	if e != nil {
		t.Fatal(e)
	}
	f, e := a.FitPrefixes(10)
	if e != nil {
		t.Fatal(e)
	}
	first, e := f.Select(1)
	if e != nil {
		t.Fatal(e)
	}
	if first.Info().To.Offset != 4 || a.Plan().Lines()[0].End.Kind != khipu.DiscretionaryBoundary {
		t.Fatal("fixture did not select SHY", first.Info())
	}
	if cs := first.Coverage(); len(cs) != 1 || cs[0].Kind != paragraph.ReplacementCoverage || cs[0].Source != (khipu.ByteRange{End: 4}) {
		t.Fatal(cs)
	}
	warm := streams
	r.Prefix = first.Checkpoint()
	r.Layout.Params.Constraints.Width = 45 * dimen.BP
	b, e := p.Trial(r)
	if e != nil {
		t.Fatal(e)
	}
	tail, e := b.FitPrefixes(1000)
	if e != nil {
		t.Fatal(e)
	}
	last, e := tail.Select(tail.Prefixes()[0].Lines)
	if e != nil {
		t.Fatal(e)
	}
	if last.Info().From != first.Info().To || last.Info().Status != paragraph.FitComplete || streams != warm {
		t.Fatal("lost carry or reshaped base")
	}
	if cs := last.Coverage(); len(cs) == 0 || cs[0].Source != (khipu.ByteRange{Start: 4, End: 11}) || cs[0].Kind != paragraph.ReplacementCoverage {
		t.Fatal(cs)
	}
	return []*layout.Fragment{first.Fragment(), last.Fragment()}, weak.Make(p)
}

func TestManualPrefixRealFontRetainedRendering(t *testing.T) {
	fragments, p := manualProofFragments(t)
	for range 5 {
		runtime.GC()
	}
	if p.Value() != nil {
		t.Fatal("fragments retain preparation")
	}
	backend := renderot.New()
	services := render.Services{Backend: backend, Identity: backend}
	var end uint64
	hyphens := 0
	for i, f := range fragments {
		arg := render.FragmentInput{Fragment: f, Text: manualProofText}
		proof, e := render.BuildFragmentProof(arg, services)
		if e != nil {
			t.Fatal(e)
		}
		if len(proof.Glyphs) == 0 {
			t.Fatal("no retained glyph output")
		}
		for _, g := range proof.Glyphs {
			if g.SourceByte == 2 {
				hyphens++
				if i != 0 {
					t.Fatal("accepted hyphen replayed")
				}
			}
		}
		for _, l := range f.Lines() {
			if l.Horizontal.Source.Start != end {
				t.Fatal("source gap/overlap")
			}
			end = l.Horizontal.Source.End
		}
		again, e := render.BuildFragmentProof(arg, services)
		if e != nil || !reflect.DeepEqual(proof, again) {
			t.Fatal("retained rendering changed", e)
		}
	}
	if end != uint64(len(manualProofText)) || hyphens != 1 {
		t.Fatal("lost/duplicated source or selected hyphen", end, hyphens)
	}
	runtime.KeepAlive(fragments)
}
