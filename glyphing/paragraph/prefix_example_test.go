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

func ExampleTrial_FitPrefixes() {
	config := khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver:  helpers.DefaultStyleResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
	}}
	p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("aa bb cc dd ee ff")}, khipu.PrepareOptions{}, config)
	if err != nil {
		panic(err)
	}
	request := paragraph.TrialRequest{Layout: paragraph.LayoutRequest{
		Params: paragraph.Params{Constraints: paragraph.Constraints{Width: 40 * dimen.BP}, Breaker: paragraph.FirstFit,
			Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: 1}}},
		Horizontal: &paragraph.HorizontalLayout{},
	}}
	trial, err := p.Trial(request)
	if err != nil {
		panic(err)
	}
	fit, err := trial.FitPrefixes(20)
	if err != nil {
		panic(err)
	}
	fmt.Print("choices:")
	for _, choice := range fit.Prefixes() {
		fmt.Print(" ", choice.Lines)
	}
	fmt.Println()
	// Choose a shorter prefix even though both lines fit.
	selected, err := fit.Select(1)
	if err != nil {
		panic(err)
	}
	fmt.Println("selected:", selected.Info().Lines, selected.Info().Status == paragraph.FitContinues)
	request.Prefix = selected.Checkpoint()
	request.Layout.Params.Constraints.Width = 20 * dimen.BP
	suffixTrial, err := p.Trial(request)
	if err != nil {
		panic(err)
	}
	suffixFit, err := suffixTrial.FitPrefixes(100)
	if err != nil {
		panic(err)
	}
	last, err := suffixFit.Select(suffixFit.Prefixes()[0].Lines)
	if err != nil {
		panic(err)
	}
	fmt.Println("suffix:", last.Info().Lines, last.Checkpoint() == nil)
	// Keep selected.Fragment() and last.Fragment() for rendering. Release fit,
	// trials, selected checkpoints and preparation when resumption is finished.

	// Output:
	// choices: 2 1 0
	// selected: 1 true
	// suffix: 3 true
}
