package paragraph

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

var compat = Options{Policy: ProofCompatibility, DefaultEmSize: 10 * dimen.BP}

func freeze(t *testing.T, value string, width dimen.DU, kp bool) *khipu.FrozenKhipu {
	t.Helper()
	return freezeStyled(t, styled.TextFromString(value), width, kp)
}

type sizeStyle dimen.DU

func (s sizeStyle) Equals(v styled.Style) bool { o, ok := v.(sizeStyle); return ok && s == o }
func (s sizeStyle) String() string             { return "synthetic size" }

type sizeResolver struct{}

func (sizeResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if s != nil {
		d.EmSize = dimen.DU(s.(sizeStyle))
	}
	return d, nil
}

func freezeStyled(t *testing.T, text styled.Text, width dimen.DU, kp bool) *khipu.FrozenKhipu {
	t.Helper()
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
		Resolver:  sizeResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP, Spacing: contracts.SpacingElastic},
	}})
	encoded, err := owner.EncodeParagraph(t.Context(), para)
	if err != nil {
		t.Fatal(err)
	}
	params := firstfit.DefaultLayout(linebreak.RectangularParShape(width))
	params.LeftSkip = khipu.KnotCore{W: dimen.BP, MinW: dimen.BP, MaxW: dimen.BP}
	params.RightSkip = khipu.KnotCore{W: 2 * dimen.BP, MinW: 2 * dimen.BP, MaxW: 2 * dimen.BP}
	var breaks []int
	if kp {
		breaks, err = knuthplass.BreakParagraph(encoded, params, knuthplass.NewDefaultParameters())
	} else {
		breaks, err = firstfit.BreakParagraph(encoded, params, firstfit.NewDefaultParameters())
	}
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := owner.Freeze(t.Context(), encoded, breaks, params)
	if err != nil {
		t.Fatal(err)
	}
	return frozen
}

func TestNaturalAssemblyFromBothBreakers(t *testing.T) {
	for _, kp := range []bool{false, true} {
		f := freeze(t, "aa\naa", 30*dimen.BP, kp)
		before, _ := f.CopyText(0)
		v, err := Assemble(t.Context(), f, compat)
		if err != nil {
			t.Fatal(err)
		}
		if f.LineCount() != 2 {
			t.Fatalf("fixture: %d lines", f.LineCount())
		}
		// Synthetic 10bp metrics are 8bp ascent, 2bp descent; the minimum gap is
		// 2bp. These expectations are independent of the measured output.
		if got := v.Box(); got != (Box{Width: 30, Extent: 24, Baseline: 8, HasBaseline: true}) || got.Height() != 8 || got.Depth() != 16 {
			t.Fatalf("box %+v", got)
		}
		for i, l := range v.Lines() {
			original, _ := f.Line(i)
			if l.Index != i || l.X != 0 || l.Top != float64(i*12) || l.Baseline != float64(i*12+8) || l.Height != 12 || l.Ascent != 8 || l.Descent != 2 || l.Gap != 2 || !reflect.DeepEqual(l.Horizontal, original) {
				t.Fatalf("line %+v", l)
			}
			if l.Horizontal.LeftSkip != dimen.BP || l.Horizontal.RightSkip != 2*dimen.BP {
				t.Fatal("lost skips")
			}
		}
		want := []Material{{LineBox, 0, 0, 10}, {Kern, -1, 10, 2}, {LineBox, 1, 12, 10}, {Kern, -1, 22, 2}}
		if !reflect.DeepEqual(v.Materials(), want) {
			t.Fatalf("material %+v", v.Materials())
		}
		lines, material := v.Lines(), v.Materials()
		lines[0].Baseline = 999
		material[0].Extent = 999
		if v.Lines()[0].Baseline != 8 || v.Materials()[0].Extent != 10 || v.Frozen() != f {
			t.Fatal("placement ownership")
		}
		after, _ := f.CopyText(0)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("assembly mutated frozen text")
		}
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				other, e := Assemble(t.Context(), f, compat)
				if e != nil || !reflect.DeepEqual(other.Lines(), v.Lines()) {
					t.Error("concurrent assembly", e)
				}
			})
		}
		wg.Wait()
	}
}

func TestEmptyBlankAndOverfull(t *testing.T) {
	v, err := Assemble(t.Context(), &khipu.FrozenKhipu{}, compat)
	if err != nil || v.Box() != (Box{}) || len(v.Lines()) != 0 || len(v.Materials()) != 0 {
		t.Fatal("empty box", v, err)
	}
	for _, kp := range []bool{false, true} {
		blank := freeze(t, "\n", 30*dimen.BP, kp)
		options := Options{Policy: ProofCompatibility, DefaultEmSize: 20 * dimen.BP}
		v, err = Assemble(t.Context(), blank, options)
		if err != nil || len(v.Lines()) == 0 {
			t.Fatal("blank fixture", err)
		}
		for _, l := range v.Lines() {
			if l.Ascent != 16 || l.Descent != 4 || l.Gap != 4 {
				t.Fatalf("fallback metrics %+v", l)
			}
		}
		f := freeze(t, "aaaa", 10*dimen.BP, kp)
		v, err = Assemble(t.Context(), f, compat)
		if err != nil {
			t.Fatal(err)
		}
		l := v.Lines()[0]
		if l.Horizontal.Fit != khipu.FitOverfull || l.Horizontal.Residual >= 0 || v.Box().Width != l.Horizontal.Width.Points() {
			t.Fatal("overflow lost", l, v.Box())
		}
	}
}

func TestAssemblyValidation(t *testing.T) {
	f := freeze(t, "aa\naa", 30*dimen.BP, false)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if v, err := Assemble(ctx, f, compat); v != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", v, err)
	}
	for _, tc := range []struct {
		ctx context.Context
		f   *khipu.FrozenKhipu
		o   Options
	}{
		{nil, f, compat}, {t.Context(), nil, compat}, {t.Context(), f, Options{}},
		{t.Context(), f, Options{ProofCompatibility, 0}}, {t.Context(), f, Options{ProofCompatibility, dimen.Fil}},
		{t.Context(), f, Options{"unknown", dimen.BP}},
	} {
		if v, err := Assemble(tc.ctx, tc.f, tc.o); v != nil || err == nil {
			t.Fatal("accepted invalid assembly", tc)
		}
	}
	// Blank-line defaults are individually finite, but their accumulated extent
	// exceeds the same finite DU coordinate domain. Never wrap or return partial output.
	f = freeze(t, "\n\n", 30*dimen.BP, false)
	if v, err := Assemble(t.Context(), f, Options{ProofCompatibility, 20000 * dimen.BP}); v != nil || err == nil {
		t.Fatal("accepted vertical overflow", v, err)
	}
}

type metricFace struct {
	m     contracts.FontMetrics
	err   error
	scale contracts.FontScale
}

func (f *metricFace) Metrics() (contracts.FontMetrics, error) { return f.m, f.err }
func (f *metricFace) Scale(dimen.DU) (contracts.FontScale, error) {
	if f.scale == nil {
		panic("assembly must use retained scale")
	}
	scale := f.scale
	f.scale = nil
	return scale, nil
}
func metricInstance(face *metricFace, scale contracts.FontScale) (contracts.FontInstance, error) {
	face.scale = scale
	return contracts.NewFontInstance(face, 10*dimen.BP)
}
func mustMetricInstance(face *metricFace, scale contracts.FontScale) contracts.FontInstance {
	i, err := metricInstance(face, scale)
	if err != nil {
		panic(err)
	}
	return i
}

type metricScale func(int32) float64

func (f metricScale) ToDU(n int32) float64 { return f(n) }

func TestContractMetricsValidation(t *testing.T) {
	sentinel := errors.New("metrics unavailable")
	face := &metricFace{m: contracts.FontMetrics{UnitsPerEm: 1000, Ascent: 700, Descent: -300, LineGap: 400}}
	base := khipu.FrozenText{Context: khipu.FrozenTextContext{Params: contracts.ShapeParams{Font: mustMetricInstance(face, metricScale(func(n int32) float64 { return float64(n) * float64(dimen.BP) / 100 }))}}}
	a, d, g, err := measure(base)
	if err != nil || a != 7 || d != 3 || g != 4 {
		t.Fatal("signed metrics", a, d, g, err)
	}
	face.m.LineGap = -400
	_, _, g, err = measure(base)
	if err != nil || g != 2 {
		t.Fatal("negative line gap", g, err)
	}
	face.err = sentinel
	if _, _, _, err = measure(base); !errors.Is(err, sentinel) {
		t.Fatal("lost metrics error", err)
	}
	face.err = nil
	for _, value := range []float64{math.NaN(), math.Inf(1), -1, 0, 1e20} {
		instance, err := metricInstance(face, metricScale(func(int32) float64 { return value }))
		if err == nil {
			bad := base
			bad.Context.Params.Font = instance
			if _, _, _, err = measure(bad); err == nil {
				t.Fatalf("accepted scale %v", value)
			}
		}
	}
	bad := base
	bad.Context.Params.Font = mustMetricInstance(face, metricScale(func(n int32) float64 {
		if n == 1 {
			return 1
		}
		return math.NaN()
	}))
	if _, _, _, err = measure(bad); err == nil {
		t.Fatal("accepted nonfinite metric conversion")
	}
	bad.Context.Params.Font = contracts.FontInstance{}
	if _, _, _, err = measure(bad); err == nil {
		t.Fatal("accepted zero instance")
	}
	face.m.UnitsPerEm = 0
	if _, _, _, err = measure(base); err == nil {
		t.Fatal("accepted zero UPEM")
	}
}

func TestMixedSizeBaselines(t *testing.T) {
	text := styled.TextFromString("ab\nc")
	text, err := text.Style(sizeStyle(20*dimen.BP), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, kp := range []bool{false, true} {
		frozen := freezeStyled(t, text, 100*dimen.BP, kp)
		v, err := Assemble(t.Context(), frozen, compat)
		if err != nil {
			t.Fatal(err)
		}
		lines := v.Lines()
		if len(lines) != 2 {
			t.Fatalf("fixture lines %d", len(lines))
		}
		// First line uses the larger 20bp run; second returns to the 10bp default.
		if lines[0].Ascent != 16 || lines[0].Descent != 4 || lines[0].Gap != 4 || lines[0].Baseline != 16 || lines[0].Height != 24 || lines[1].Top != 24 || lines[1].Baseline != 32 || lines[1].Height != 12 || v.Box().Extent != 36 {
			t.Fatalf("mixed sizes %+v box %+v", lines, v.Box())
		}
	}
}
