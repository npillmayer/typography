package layout_test

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

// These text fixtures select manual opportunities only when a test supplies a
// marker service; production callers choose their own preparation policy.
func layoutText(input paragraph.Input, params paragraph.Params, config khipu.Config) (*paragraph.Result, error) {
	options := khipu.PrepareOptions{}
	if config.Hyphenator != nil {
		options.Hyphenation = khipu.ExplicitHyphenation
	}
	prepared, err := paragraph.Prepare(input, options, config)
	if err != nil {
		return nil, err
	}
	return prepared.Layout(paragraph.LayoutRequest{Params: params})
}
