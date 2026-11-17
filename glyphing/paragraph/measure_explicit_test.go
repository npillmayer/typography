package paragraph_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/glyphing/paragraph"
)

type measureExplicitPolicy func(string, *khipu.Params) ([]khipu.HyphenationFragment, error)

func (f measureExplicitPolicy) Discretionaries(s string, p *khipu.Params) ([]khipu.HyphenationFragment, error) {
	return f(s, p)
}

func TestExplicitMeasurementLayoutReuse(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		t.Run(string(breaker), func(t *testing.T) {
			var calls measurementCalls
			cfg := measurementConfig(&calls)
			cfg.Hyphenator = measureExplicitPolicy(func(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
				calls.hyphenate++
				if word != "\u00ad" {
					t.Error("unexpected whole-word request", word)
				}
				return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
			})
			type snapshot struct {
				breaks           []uint64
				lines, materials any
				box              any
				frozenLines      []khipu.FrozenLine
				knots            []khipu.FrozenKnot
				texts            []khipu.FrozenText
			}
			capture := func(out *paragraph.Result) snapshot {
				s := snapshot{breaks: out.BreakPositions, lines: out.VList.Lines(), materials: out.VList.Materials(), box: out.VList.Box()}
				for i := 0; i < out.Frozen.LineCount(); i++ {
					line, _ := out.Frozen.Line(i)
					s.frozenLines = append(s.frozenLines, line)
				}
				for i := 0; i < out.Frozen.KnotCount(); i++ {
					k, _ := out.Frozen.Knot(i)
					tx, _ := out.Frozen.CopyText(i)
					s.knots = append(s.knots, k)
					s.texts = append(s.texts, tx)
				}
				return s
			}
			var cold snapshot
			for _, warm := range []bool{false, true} {
				p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("ab\u00adcd")}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
				if err != nil {
					t.Fatal(err)
				}
				base := calls
				measure := func() {
					t.Helper()
					metrics, resolves := calls.metrics, calls.resolve
					got, err := p.MeasureIntrinsic(t.Context(), khipu.MeasureRequest{})
					if err != nil || got != (khipu.IntrinsicWidths{MinContent: 15 * dimen.BP, MaxContent: 20 * dimen.BP}) {
						t.Fatal(got, err)
					}
					if calls.metrics != metrics || calls.resolve != resolves {
						t.Fatal("measurement resolved styles or queried metrics")
					}
				}
				if warm {
					measure()
				}
				var previous *paragraph.Result
				var retained snapshot
				for range 2 {
					out, err := p.Layout(t.Context(), paragraph.LayoutRequest{Params: params(breaker, 15*dimen.BP)})
					if err != nil {
						t.Fatal(err)
					}
					got := capture(out)
					if len(got.frozenLines) != 2 {
						t.Fatal("fixture failed to split")
					}
					if cold.breaks == nil {
						cold = got
					} else if !reflect.DeepEqual(cold, got) {
						t.Fatal("measurement changed frozen glyphs/geometry or vlist placements")
					}
					measure()
					if previous != nil && !reflect.DeepEqual(retained, capture(previous)) {
						t.Fatal("retained output changed")
					}
					previous, retained = out, got
					if calls.shape != base.shape+2 || calls.hyphenate != base.hyphenate+1 || calls.resolve != base.resolve {
						t.Fatal("repeated preparation services", base, calls)
					}
				}
			}
		})
	}
}

func TestExplicitMeasurementStageErrorsAndRetry(t *testing.T) {
	sentinel := errors.New("explicit provider or backend failed")
	for _, stage := range []string{"provider", "pre", "post", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			var calls measurementCalls
			cfg := measurementConfig(&calls)
			fail := false
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cfg.Hyphenator = measureExplicitPolicy(func(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
				calls.hyphenate++
				if fail && stage == "provider" {
					return nil, sentinel
				}
				if fail && stage == "cancel" {
					cancel()
				}
				return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
			})
			cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
				calls.shape++
				if fail && ((stage == "pre" && calls.shape == 2) || (stage == "post" && calls.shape == 3)) {
					return nil, sentinel
				}
				s, err := helpers.NewSyntheticStreamingShaper(8)
				return metricStream{s}, err
			}
			p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString("ab\u00adcd")}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			fail = true
			got, err := p.MeasureIntrinsic(ctx, khipu.MeasureRequest{})
			cause := sentinel
			if stage == "cancel" {
				cause = context.Canceled
			}
			requireMeasureError(t, got, err, paragraph.StageMeasure, cause)
			fail = false
			got, err = p.MeasureIntrinsic(t.Context(), khipu.MeasureRequest{})
			if err != nil || got.MinContent != 15*dimen.BP || calls.hyphenate != 2 {
				t.Fatal("retry", got, err, calls)
			}
		})
	}
	for _, text := range []string{"ab\u00adcd\u00adef", "ab\u00ad\u00adcd"} {
		var calls measurementCalls
		cfg := measurementConfig(&calls)
		p, err := paragraph.Prepare(t.Context(), paragraph.Input{Text: styled.TextFromString(text)}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		before := calls
		got, err := p.MeasureIntrinsic(t.Context(), khipu.MeasureRequest{})
		requireMeasureError(t, got, err, paragraph.StageMeasure, khipu.ErrUnsupportedPolicy)
		if calls != before {
			t.Fatal("unsupported multiplicity consulted services")
		}
	}
}
