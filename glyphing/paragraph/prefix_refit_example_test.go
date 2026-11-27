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

func ExamplePrefixFit_Refit() {
	config := khipu.Config{Streaming: &khipu.StreamingConfig{
		Resolver:  helpers.DefaultStyleResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
	}}
	prepared, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("aa bb cc dd ee ff")}, khipu.PrepareOptions{}, config)
	if err != nil {
		panic(err)
	}
	trial, err := prepared.Trial(paragraph.TrialRequest{Layout: paragraph.LayoutRequest{
		Params: paragraph.Params{Constraints: paragraph.Constraints{Width: 25 * dimen.BP}, Breaker: paragraph.FirstFit,
			Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight}, LineLayout: &paragraph.LineLayout{Default: paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: 1}}},
		Horizontal: &paragraph.HorizontalLayout{},
	}})
	if err != nil {
		panic(err)
	}
	choices, err := trial.FitPrefixes(10)
	if err != nil {
		panic(err)
	}
	first, err := choices.Select(choices.Prefixes()[0].Lines)
	if err != nil {
		panic(err)
	}
	// Keep the same widths and object geometry; only the receiving height changes.
	tail, err := choices.Refit(first.Info().Lines, 20)
	if err != nil {
		panic(err)
	}
	last, err := tail.Select(tail.Prefixes()[0].Lines)
	if err != nil {
		panic(err)
	}
	fmt.Println("first frame:", first.Info().Lines)
	fmt.Println("second frame:", last.Info().Lines)
	fmt.Println("complete:", last.Info().Status == paragraph.FitComplete)
	// Output:
	// first frame: 1
	// second frame: 2
	// complete: true
}
