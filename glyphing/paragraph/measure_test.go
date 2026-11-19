package paragraph_test

import (
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

type measurementCalls struct{ resolve, shape, hyphenate, metrics int }
type measurementResolver struct{ calls *measurementCalls }

func (r measurementResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	r.calls.resolve++
	return helpers.DefaultStyleResolver{}.ResolveStyle(s, d)
}

type measurementFont struct {
	contracts.FontFace
	calls *measurementCalls
}

func (f measurementFont) Metrics() (contracts.FontMetrics, error) {
	f.calls.metrics++
	return f.FontFace.Metrics()
}

type measurementHyphenator struct{ calls *measurementCalls }

func (h measurementHyphenator) Discretionaries(string, *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.calls.hyphenate++
	return nil, errors.New("unexpected discretionary discovery")
}

func measurementConfig(calls *measurementCalls) khipu.Config {
	cfg := synthetic()
	cfg.Streaming.Resolver = measurementResolver{calls}
	cfg.Streaming.Defaults.Font = measurementFont{cfg.Streaming.Defaults.Font, calls}
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
		calls.shape++
		s, err := helpers.NewSyntheticStreamingShaper(8)
		return metricStream{s}, err
	}
	cfg.Hyphenator = measurementHyphenator{calls}
	return cfg
}

func TestPreparedMeasureReuseAndRetainedResults(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		t.Run(string(breaker), func(t *testing.T) {
			var calls measurementCalls
			input := paragraph.Input{Text: styled.TextFromString("éa \uFFFC fi"), Atomics: []contracts.AtomicAnchor{{ID: 7, ByteOffset: 4}}}
			p, err := paragraph.Prepare(input, khipu.PrepareOptions{}, measurementConfig(&calls))
			if err != nil {
				t.Fatal(err)
			}
			preparedCalls := calls
			if calls.resolve == 0 || calls.shape == 0 {
				t.Fatal("uninstrumented preparation", calls)
			}
			request := khipu.MeasureRequest{Atomics: []khipu.AtomicIntrinsic{{ID: 7, MinAdvance: 10 * dimen.BP, MaxAdvance: 20 * dimen.BP}}}
			want := khipu.IntrinsicWidths{MinContent: 10 * dimen.BP, MaxContent: 50 * dimen.BP}
			measure := func() khipu.IntrinsicWidths {
				t.Helper()
				before := calls
				got, err := p.MeasureIntrinsic(request)
				if err != nil || got != want {
					t.Fatalf("measure = %v, %v; want %v", got, err, want)
				}
				if calls != before {
					t.Fatal("measurement consulted services", before, calls)
				}
				return got
			}
			firstWidths := measure()
			measure()
			var verify []func()
			for _, width := range []dimen.DU{100 * dimen.BP, 20 * dimen.BP} {
				out, err := p.Layout(paragraph.LayoutRequest{
					Params: mixedParams(breaker, width), Horizontal: &paragraph.HorizontalLayout{},
					Atomics: []paragraph.AtomicBox{atomicBox(7, 20, 12)},
				})
				if err != nil {
					t.Fatal(err)
				}
				if width == 100*dimen.BP && out.Frozen.LineCount() != 1 {
					t.Fatal("wide fixture wrapped")
				}
				if width == 20*dimen.BP && out.Frozen.LineCount() <= 1 {
					t.Fatal("narrow fixture did not wrap")
				}
				lines, placements, materials, box := out.VList.Lines(), out.VList.AtomicPlacements(), out.VList.Materials(), out.VList.Box()
				var knots []khipu.FrozenKnot
				var texts []khipu.FrozenText
				for i := 0; i < out.Frozen.KnotCount(); i++ {
					knot, _ := out.Frozen.Knot(i)
					knots = append(knots, knot)
					text, _ := out.Frozen.CopyText(i)
					texts = append(texts, text)
					if id, ok := out.Frozen.Atomic(i); ok && (id != 7 || knot.Source.Bytes != (khipu.ByteRange{Start: 4, End: 7}) || knot.Source.ScalarStart != 3) {
						t.Fatal("atomic source coordinates changed", knot)
					}
				}
				verify = append(verify, func() {
					t.Helper()
					if !reflect.DeepEqual(lines, out.VList.Lines()) || !reflect.DeepEqual(placements, out.VList.AtomicPlacements()) || !reflect.DeepEqual(materials, out.VList.Materials()) || box != out.VList.Box() {
						t.Fatal("retained vlist changed")
					}
					for i, before := range knots {
						after, _ := out.Frozen.Knot(i)
						text, _ := out.Frozen.CopyText(i)
						if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(texts[i], text) {
							t.Fatal("retained frozen geometry/glyphs changed")
						}
					}
				})
			}
			measure()
			request.Atomics[0].MinAdvance, request.Atomics[0].MaxAdvance = 0, 0
			want = khipu.IntrinsicWidths{MinContent: 10 * dimen.BP, MaxContent: 30 * dimen.BP}
			measure()
			if firstWidths != (khipu.IntrinsicWidths{MinContent: 10 * dimen.BP, MaxContent: 50 * dimen.BP}) {
				t.Fatal("retained widths changed")
			}
			request.Atomics[0].ID = 99
			got, err := p.MeasureIntrinsic(request)
			requireMeasureError(t, got, err, paragraph.StageMeasure, khipu.ErrIllegalArguments)
			request.Atomics[0].ID = 7
			measure()
			for _, check := range verify {
				check()
			}
			if calls.shape != preparedCalls.shape || calls.hyphenate != 0 || calls.metrics <= preparedCalls.metrics {
				t.Fatal("unexpected service lifetime", preparedCalls, calls)
			}
		})
	}
}

func requireMeasureError(t *testing.T, got khipu.IntrinsicWidths, err error, stage paragraph.Stage, cause error) {
	t.Helper()
	var staged *paragraph.Error
	if got != (khipu.IntrinsicWidths{}) || !errors.As(err, &staged) || staged.Stage != stage || !errors.Is(err, cause) {
		t.Fatalf("widths=%v error=%v; want %s wrapping %v", got, err, stage, cause)
	}
}

func TestPreparedMeasureErrorsAndRecovery(t *testing.T) {
	p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("\uFFFC"), Atomics: []contracts.AtomicAnchor{{ID: 1}}}, khipu.PrepareOptions{}, synthetic())
	if err != nil {
		t.Fatal(err)
	}
	valid := khipu.MeasureRequest{Atomics: []khipu.AtomicIntrinsic{{ID: 1, MinAdvance: -10, MaxAdvance: 20}}}
	for _, bad := range []*paragraph.Prepared{nil, {}} {
		got, err := bad.MeasureIntrinsic(valid)
		requireMeasureError(t, got, err, paragraph.StageInput, khipu.ErrIllegalArguments)
		if !errors.Is(err, paragraph.ErrInvalidInput) {
			t.Fatal("lost paragraph input cause", err)
		}
	}
	for _, atoms := range [][]khipu.AtomicIntrinsic{
		nil, {{ID: 0}}, {{ID: 2}}, {{ID: 1}, {ID: 1}}, {{ID: 1, MinAdvance: 20, MaxAdvance: 10}}, {{ID: 1, MaxAdvance: dimen.Fil}},
	} {
		got, err := p.MeasureIntrinsic(khipu.MeasureRequest{Atomics: atoms})
		requireMeasureError(t, got, err, paragraph.StageMeasure, khipu.ErrIllegalArguments)
	}
	got, err := p.MeasureIntrinsic(valid)
	if err != nil || got != (khipu.IntrinsicWidths{MaxContent: 20}) {
		t.Fatal("retry or signed normalization failed", got, err)
	}
	for _, mode := range []khipu.HyphenationMode{khipu.AutomaticHyphenation} {
		var calls measurementCalls
		p, err := paragraph.Prepare(paragraph.Input{Text: styled.TextFromString("ab\u00adcd")}, khipu.PrepareOptions{Hyphenation: mode}, measurementConfig(&calls))
		if err != nil {
			t.Fatal(err)
		}
		before := calls
		got, err := p.MeasureIntrinsic(khipu.MeasureRequest{})
		requireMeasureError(t, got, err, paragraph.StageMeasure, khipu.ErrUnsupportedPolicy)
		if calls != before {
			t.Fatal("unsupported measurement consulted services")
		}
		// Unsupported measurement must not poison an otherwise valid layout.
		if _, err := p.Layout(paragraph.LayoutRequest{Params: params(paragraph.FirstFit, 100*dimen.BP)}); err != nil {
			t.Fatal(err)
		}
	}
}
