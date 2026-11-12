package paragraph

import (
	"errors"
	"math"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

func TestCSSMetricsAndLeadingValidation(t *testing.T) {
	sentinel := errors.New("unavailable")
	f := &metricFace{m: contracts.FontMetrics{UnitsPerEm: 1000, Ascent: 751, Descent: -249, LineGap: 101}}
	scale := metricScale(func(n int32) float64 { return float64(n) * float64(dimen.BP) / 73 })
	m, err := cssMetrics(f, scale, 10*dimen.BP)
	if err != nil {
		t.Fatal(err)
	}
	a, d, err := (LineHeight{Kind: LengthHeight}).extents(m)
	if err != nil || a+d != 0 {
		t.Fatal("fractional zero-height box", a, d, err)
	}
	f.err = sentinel
	if _, err = cssMetrics(f, scale, 10*dimen.BP); !errors.Is(err, sentinel) {
		t.Fatal("metric cause lost", err)
	}
	f.err = nil
	for _, v := range []float64{math.NaN(), math.Inf(1), 0, -1, 1e30} {
		if _, err = cssMetrics(f, metricScale(func(int32) float64 { return v }), 10*dimen.BP); err == nil {
			t.Fatal("invalid scale", v)
		}
	}
	f.m.Ascent = -1
	if _, err = cssMetrics(f, scale, 10*dimen.BP); err == nil {
		t.Fatal("negative ascent accepted")
	}
	f.m.Ascent = 751
	f.m.UnitsPerEm = 0
	if _, err = cssMetrics(f, scale, 10*dimen.BP); err == nil {
		t.Fatal("zero units accepted")
	}
}
