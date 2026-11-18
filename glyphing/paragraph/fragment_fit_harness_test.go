package paragraph_test

import (
	"context"

	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

// Compose two production one-frame calls to retain E1's partition oracles.
// Preparation in fitFixture ends before this harness runs; services are sealed.
type fitReuseKey struct {
	width                                              dimen.DU
	left, right                                        [3]dimen.DU
	source, styles, fonts, lineHeight, atomics, policy uint64
}

func (k fitReuseKey) public() paragraph.FragmentReuseKey {
	return paragraph.FragmentReuseKey{Width: k.width, LeftSkip: k.left, RightSkip: k.right,
		Source: k.source, Styles: k.styles, Fonts: k.fonts, LineHeight: k.lineHeight, Atomics: k.atomics, Policy: k.policy}
}

type fitFrame struct {
	key    fitReuseKey
	height float64
}
type fitOutcome = paragraph.FragmentFitStatus

const (
	fitComplete   = paragraph.FitComplete
	fitMoreFrames = paragraph.FitContinues
	fitNoProgress = paragraph.FitNoProgress
)

var fitErrIncompatible = paragraph.ErrIncompatibleFragment

type fitSplitResult struct {
	a, b, tail     *layout.Fragment
	status         fitOutcome
	overfull       []int
	interiorSplits int
}

func fitSplitEqual(ctx context.Context, v *layout.VList, key fitReuseKey, a, b fitFrame) (*fitSplitResult, error) {
	f, err := paragraph.Fragment(ctx, v)
	if err != nil {
		return nil, err
	}
	x, err := paragraph.FitFragment(ctx, f, key.public(), a.key.public(), a.height)
	if err != nil {
		return nil, err
	}
	y, err := paragraph.FitFragment(ctx, x.Remainder, key.public(), b.key.public(), b.height)
	if err != nil {
		return nil, err
	}
	out := &fitSplitResult{a: x.Placed, b: y.Placed, tail: y.Remainder, overfull: x.Overfull}
	for _, part := range []*paragraph.FragmentFit{x, y} {
		if part.Status == paragraph.FitContinues {
			out.interiorSplits++
		}
	}
	if y.Status != paragraph.FitComplete {
		out.status = fitMoreFrames
		if len(x.Placed.Lines()) == 0 && len(y.Placed.Lines()) == 0 {
			out.status = fitNoProgress
		}
	}
	return out, nil
}
