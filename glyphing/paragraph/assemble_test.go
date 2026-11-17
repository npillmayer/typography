package paragraph

import (
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

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
