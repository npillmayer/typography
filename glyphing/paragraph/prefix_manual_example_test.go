package paragraph_test

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

type exampleManualHyphens struct{}

func (exampleManualHyphens) Discretionaries(marker string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	if marker == "\u00ad" {
		return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
	}
	return nil, nil
}

func ExampleTrial_FitPrefixes_manual() {
	config := khipu.Config{Hyphenator: exampleManualHyphens{}, Streaming: &khipu.StreamingConfig{
		Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
	}}
	prepared, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("ab\u00adcd ef")}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, config)
	if err != nil {
		panic(err)
	}
	request := paragraph.TrialRequest{Layout: paragraph.LayoutRequest{Params: paragraph.Params{
		Constraints: paragraph.Constraints{Width: 15 * dimen.BP}, Breaker: paragraph.FirstFit,
		Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: 1}},
	}, Horizontal: &paragraph.HorizontalLayout{}}}
	trial, err := prepared.Trial(request)
	if err != nil {
		panic(err)
	}
	fit, err := trial.FitPrefixes(10)
	if err != nil {
		panic(err)
	}
	first, err := fit.Select(1)
	if err != nil {
		panic(err)
	}
	fmt.Println("accepted source bytes:", first.Info().From.Offset, first.Info().To.Offset)
	// The checkpoint carries "cd" after the accepted "ab-". Original byte
	// coordinates include both UTF-8 bytes of the authored soft hyphen.
	request.Prefix = first.Checkpoint()
	request.Layout.Params.Constraints.Width = 25 * dimen.BP
	trial, err = prepared.Trial(request)
	if err != nil {
		panic(err)
	}
	fit, err = trial.FitPrefixes(100)
	if err != nil {
		panic(err)
	}
	last, err := fit.Select(fit.Prefixes()[0].Lines)
	if err != nil {
		panic(err)
	}
	fmt.Println("remaining source bytes:", last.Info().From.Offset, last.Info().To.Offset)
	fmt.Println("complete:", last.Info().Status == paragraph.FitComplete)
	// Retain the two Fragment() values for rendering; release the preparation,
	// trials, choice sets and checkpoints after the paragraph is complete.
	// Output:
	// accepted source bytes: 0 4
	// remaining source bytes: 4 9
	// complete: true
}
