package flowexperiment

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

func TestSkipBoundDomain(t *testing.T) {
	v, key, c := fixture(t, textInput("aa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	before, calls := v.Lines(), c.snapshot()
	for _, tc := range []struct {
		name  string
		skip  [3]dimen.DU
		valid bool
	}{
		{"zero", [3]dimen.DU{}, true},
		{"signed finite", [3]dimen.DU{-1, -dimen.Fil + 1, dimen.Fil - 1}, true},
		{"fil", [3]dimen.DU{0, -1, dimen.Fil}, true},
		{"fill", [3]dimen.DU{0, 0, dimen.Fill}, true},
		{"filll", [3]dimen.DU{0, 0, dimen.Filll}, true},
		{"minimum boundary", [3]dimen.DU{0, -dimen.Fil, 0}, false},
		{"minimum infinity", [3]dimen.DU{0, -dimen.Infinity, 0}, false},
		{"positive minimum", [3]dimen.DU{0, dimen.Fil, dimen.Filll}, false},
		{"maximum infinity", [3]dimen.DU{0, 0, dimen.Infinity}, false},
		{"natural boundary", [3]dimen.DU{dimen.Fil, 0, dimen.Filll}, false},
		{"minimum above natural", [3]dimen.DU{0, 1, 1}, false},
		{"maximum below natural", [3]dimen.DU{0, -1, -1}, false},
	} {
		for _, side := range []string{"left", "right"} {
			t.Run(tc.name+"/"+side, func(t *testing.T) {
				k := key
				geometry := khipu.Params{LineShape: linebreak.RectangularParShape(key.width)}
				core := khipu.KnotCore{W: tc.skip[0], MinW: tc.skip[1], MaxW: tc.skip[2]}
				if side == "left" {
					k.left, geometry.LeftSkip = tc.skip, core
				} else {
					k.right, geometry.RightSkip = tc.skip, core
				}
				// The owning khipu validator is an independent domain oracle. This
				// tests key structure, not the truth of a caller's skip attestation.
				if err := geometry.ValidateGeometry(); (err == nil) != tc.valid {
					t.Fatal("incorrect fixture domain", err)
				}
				out, err := splitEqual(t.Context(), v, k, frame{k, 10}, frame{k, 10})
				if tc.valid {
					if err != nil {
						t.Fatal(err)
					}
					verifyPartition(t, v, out)
				} else if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
					t.Fatal("accepted malformed key", out, err)
				}
			})
		}
	}
	if c.snapshot() != calls || !reflect.DeepEqual(before, v.Lines()) {
		t.Fatal("guard queried services or mutated source")
	}
}

func TestSuppressedEmptyWidthGuard(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		v, key, c := fixture(t, textInput(""), breaker, 40*dimen.BP, 1, nil)
		if len(v.Lines()) != 0 || v.Frozen().LineCount() == 0 {
			t.Fatal("missing diagnostic owner")
		}
		diagnostic, _ := v.Frozen().Line(0)
		calls := c.snapshot()
		wrong := key
		wrong.width = 50 * dimen.BP
		out, err := splitEqual(t.Context(), v, wrong, frame{wrong, 0}, frame{wrong, 0})
		if out != nil || !errors.Is(err, errIncompatible) {
			t.Fatal("accepted wrong diagnostic width", out, err)
		}
		out, err = splitEqual(t.Context(), v, key, frame{key, 0}, frame{key, 0})
		if err != nil {
			t.Fatal(err)
		}
		verifyPartition(t, v, out)
		for _, f := range []*layout.Fragment{out.a, out.b, out.tail} {
			if f.Box() != (layout.Box{}) || len(f.Lines()) != 0 || len(f.Materials()) != 0 || len(f.AtomicPlacements()) != 0 {
				t.Fatal("diagnostics became visible geometry")
			}
		}
		after, _ := v.Frozen().Line(0)
		if out.status != complete || len(out.overfull) != 0 || diagnostic != after || c.snapshot() != calls {
			t.Fatal("changed diagnostic ownership/status or queried services")
		}
	}
}
