package flowexperiment

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

func TestEqualWidthCuts(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		t.Run(string(breaker), func(t *testing.T) {
			v, key, c := fixture(t, textInput("aa\naa\naa"), breaker, 40*dimen.BP, 1, nil)
			if len(v.Lines()) != 3 || v.Box().Extent != 30 {
				t.Fatal("fixture geometry", v.Box())
			}
			before := c.snapshot()
			if before[0] == 0 || before[2] == 0 || before[4] == 0 {
				t.Fatal("uninstrumented services", before)
			}
			for _, tc := range []struct {
				name   string
				a, b   float64
				counts [3]int
				status outcome
			}{
				{"exact", 10, 20, [3]int{1, 2, 0}, complete}, {"slack", 15, 25, [3]int{1, 2, 0}, complete},
				{"all A", 30, 0, [3]int{3, 0, 0}, complete}, {"all B", 0, 30, [3]int{0, 3, 0}, complete},
				{"empty B", 10, 0, [3]int{1, 0, 2}, moreFrames}, {"B exhausted", 10, 10, [3]int{1, 1, 1}, moreFrames},
				{"neither fits", 9, 9, [3]int{0, 0, 3}, noProgress},
				{"strict boundary", math.Nextafter(10, 0), 30, [3]int{0, 3, 0}, complete},
			} {
				t.Run(tc.name, func(t *testing.T) {
					out, err := splitEqual(t.Context(), v, key, frame{key, tc.a}, frame{key, tc.b})
					if err != nil {
						t.Fatal(err)
					}
					if out.status != tc.status || out.a.Box().Extent > tc.a || out.b.Box().Extent > tc.b {
						t.Fatal("wrong fit/status", out)
					}
					fs := []*layout.Fragment{out.a, out.b, out.tail}
					for i, f := range fs {
						if f.Range().End-f.Range().Start != tc.counts[i] {
							t.Fatal("line count", i, f.Range())
						}
					}
					verifyPartition(t, v, out)
					delta := c.snapshot()
					for i := range delta {
						delta[i] -= before[i]
					}
					t.Logf("boundary lines examined=%d, interior splits=%d, additional service calls=%v", out.examinedLines, out.interiorSplits, delta)
					if delta != [7]int64{} {
						t.Fatal("service calls")
					}
				})
			}
		})
	}
}

// Independently compare every retained record with the original source, including
// line ranges, physical knot order, byte/scalar provenance and exact A geometry.
func verifyPartition(t *testing.T, v *layout.VList, out *splitResult) {
	t.Helper()
	lines, atoms := v.Lines(), v.AtomicPlacements()
	next, ai := 0, 0
	// The tail may be translated twice. Preserve the specified subtraction
	// order rather than assuming floating-point subtraction is associative.
	originA := 0.0
	if end := out.a.Range().End; end < len(lines) {
		originA = lines[end].Top
	}
	for part, f := range []*layout.Fragment{out.a, out.b, out.tail} {
		if f.Frozen() != v.Frozen() || f.Range().Start != next {
			t.Fatal("lost owner/range")
		}
		local := f.Lines()
		origin := 0.0
		if len(local) > 0 {
			origin = lines[next].Top
		}
		translate := func(y float64) float64 {
			if part == 2 {
				return (y - originA) - (origin - originA)
			}
			return y - origin
		}
		for i, l := range local {
			old := lines[next]
			want := old
			want.Index = i
			want.Top = translate(want.Top)
			want.Baseline = translate(want.Baseline)
			if l.Line != want || l.OriginalIndex != next {
				t.Fatalf("changed line %d: %+v != %+v", next, l, want)
			}
			m := f.Materials()[i]
			if m.Child != i || m.Top != l.Top || m.Extent != old.Height {
				t.Fatal("material mismatch")
			}
			for k := old.Horizontal.FirstKnot; k < old.Horizontal.FirstKnot+old.Horizontal.KnotCount; k++ {
				a, _ := f.Frozen().Knot(k)
				b, _ := v.Frozen().Knot(k)
				x, _ := f.Frozen().CopyText(k)
				y, _ := v.Frozen().CopyText(k)
				if a != b || !reflect.DeepEqual(x, y) {
					t.Fatal("content changed")
				}
			}
			next++
		}
		for _, a := range f.AtomicPlacements() {
			want := atoms[ai]
			want.Line -= f.Range().Start
			want.Y = translate(want.Y)
			if a != want {
				t.Fatal("atomic traversal/geometry", a, want)
			}
			ai++
		}
		if f.Range().End != next {
			t.Fatal("lost tail")
		}
	}
	if next != len(lines) || ai != len(atoms) {
		t.Fatal("lost lines or objects")
	}
}

func TestNaturalWrapAndFractionalRebasing(t *testing.T) {
	for _, breaker := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		v, key, _ := fixture(t, textInput("aa bb cc dd ee ff gg hh ii jj kk ll"), breaker, 40*dimen.BP, .731, nil)
		lines := v.Lines()
		if len(lines) < 3 {
			t.Fatal("fixture must wrap without forced breaks", len(lines))
		}
		out, err := splitEqual(t.Context(), v, key, frame{key, lines[0].Height}, frame{key, lines[1].Height})
		if err != nil {
			t.Fatal(err)
		}
		if out.status != moreFrames || len(out.a.Lines()) != 1 || len(out.b.Lines()) != 1 || out.interiorSplits != 2 {
			t.Fatal("wrong fractional cut", out)
		}
		verifyPartition(t, v, out)
	}
}

func TestEmptyZeroHeightAndOverflow(t *testing.T) {
	for _, tc := range []struct {
		text   string
		factor float64
		width  dimen.DU
		lines  int
	}{
		{"", 1, 40 * dimen.BP, 0}, {"aa\n\naa", 0, 40 * dimen.BP, 3}, {"aa", 1, 0, 1},
	} {
		v, key, _ := fixture(t, textInput(tc.text), paragraph.FirstFit, tc.width, tc.factor, nil)
		out, err := splitEqual(t.Context(), v, key, frame{key, 0}, frame{key, 100})
		if err != nil {
			t.Fatal(err)
		}
		verifyPartition(t, v, out)
		if out.status != complete || len(v.Lines()) != tc.lines {
			t.Fatal(out.status, len(v.Lines()))
		}
		if tc.factor == 0 && out.a.Range().End != 3 {
			t.Fatal("zero-height lines not consumed")
		}
		if tc.text == "" && v.Frozen().LineCount() == 0 {
			t.Fatal("missing diagnostic fixture")
		}
		if tc.width == 0 && len(out.overfull) != 1 {
			t.Fatal("hidden horizontal overflow")
		}
	}
}

func TestMixedHeightsAndAtomics(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		input := textInput("a b\n\uFFFC\uFFFC\naa\n\uFFFC")
		var err error
		input.Text, err = input.Text.Style(sized(20*dimen.BP), 1, 3)
		if err != nil {
			t.Fatal(err)
		}
		input.Direction = dir
		input.Atomics = []contracts.AtomicAnchor{{ID: 7, ByteOffset: 4}, {ID: 2, ByteOffset: 7}, {ID: 9, ByteOffset: 14}}
		boxes := []paragraph.AtomicBox{
			{ID: 7, Margin: paragraph.AtomicMargins{Right: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge},
			{ID: 2, BorderWidth: 4 * dimen.BP, BorderHeight: 30 * dimen.BP, Margin: paragraph.AtomicMargins{Left: -2 * dimen.BP}, Alignment: paragraph.AlignBottomMarginEdge},
			{ID: 9, Alignment: paragraph.AlignBottomMarginEdge},
		}
		v, key, _ := fixture(t, input, paragraph.FirstFit, 100*dimen.BP, 1, boxes)
		if v.Lines()[0].Height != 20 || v.Lines()[1].Height != 32 {
			t.Fatal("unexpected mixed fixture", v.Lines())
		}
		out, err := splitEqual(t.Context(), v, key, frame{key, 20}, frame{key, 100})
		if err != nil {
			t.Fatal(err)
		}
		verifyPartition(t, v, out)
		if len(out.a.Lines()) != 1 || out.b.Lines()[0].Height != 32 {
			t.Fatal("lost full height")
		}
	}
	// Signed effective descent and preserved blank lines.
	v, key, _ := fixture(t, textInput("aa\n\naa"), paragraph.FirstFit, 40*dimen.BP, .25, nil)
	out, err := splitEqual(t.Context(), v, key, frame{key, 2.5}, frame{key, 5})
	if err != nil {
		t.Fatal(err)
	}
	verifyPartition(t, v, out)
	if out.b.Lines()[0].Descent >= 0 {
		t.Fatal("lost negative leading")
	}
	// Mixed bidi punctuation and a textual isolate crossing the region cut.
	input := paragraph.Input{Text: styled.TextFromString("abc \u2067אב 12\nגד\u2069 (xyz)"), Direction: bidi.LeftToRight}
	v, key, _ = fixture(t, input, paragraph.FirstFit, 200*dimen.BP, 1, nil)
	out, err = splitEqual(t.Context(), v, key, frame{key, 10}, frame{key, 100})
	if err != nil {
		t.Fatal(err)
	}
	verifyPartition(t, v, out)
}

func TestGuardAndInvalidRequests(t *testing.T) {
	v, key, _ := fixture(t, textInput("aa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	changes := []func(*reuseKey){func(k *reuseKey) { k.width++ }, func(k *reuseKey) { k.left[0]++ }, func(k *reuseKey) { k.right[2]++ }, func(k *reuseKey) { k.source++ }, func(k *reuseKey) { k.styles++ }, func(k *reuseKey) { k.fonts++ }, func(k *reuseKey) { k.lineHeight++ }, func(k *reuseKey) { k.atomics++ }, func(k *reuseKey) { k.policy++ }}
	for _, change := range changes {
		other := key
		change(&other)
		for _, frames := range [][2]frame{{{other, 10}, {key, 10}}, {{key, 10}, {other, 10}}} {
			out, err := splitEqual(t.Context(), v, key, frames[0], frames[1])
			if out != nil || !errors.Is(err, errIncompatible) {
				t.Fatal("guard", out, err)
			}
		}
	}
	wrong := key
	wrong.width++
	if out, err := splitEqual(t.Context(), v, wrong, frame{wrong, 10}, frame{wrong, 10}); out != nil || !errors.Is(err, errIncompatible) {
		t.Fatal("false source width", out, err)
	}
	for _, h := range []float64{-1, math.NaN(), math.Inf(1), dimen.Fil.Points()} {
		out, err := splitEqual(t.Context(), v, key, frame{key, 10}, frame{key, h})
		if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(out, err)
		}
	}
	for _, bad := range []*layout.VList{nil, new(layout.VList)} {
		out, err := splitEqual(t.Context(), bad, key, frame{key, 10}, frame{key, 10})
		if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(out, err)
		}
	}
	out, err := splitEqual(nil, v, key, frame{key, 10}, frame{key, 10})
	if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err = splitEqual(ctx, v, key, frame{key, 10}, frame{key, 10})
	if out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(out, err)
	}
}
