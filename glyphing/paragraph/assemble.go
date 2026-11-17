package paragraph

import (
	"math"
	"reflect"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/layout"
)

// SpacingPolicy identifies the supported CSS line-box assembly contract.
type SpacingPolicy string

// Options requires the CSSLineHeight policy. Strut inputs belong to
// Streaming.Defaults; line-height values belong to Params.LineLayout.
type Options struct{ Policy SpacingPolicy }

// assembly is private mutable construction state, never exposed to rendering.
type assembly struct {
	box      layout.Box
	lines    []layout.Line
	material []layout.Material
	frozen   *khipu.FrozenKhipu
	atomics  []layout.AtomicPlacement
}

func (v *assembly) publish() (*layout.VList, error) {
	return layout.NewVList(v.frozen, v.box, v.lines, v.material, v.atomics)
}

func dimension(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < dimen.Fil.Points()
}

func nilService(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}
