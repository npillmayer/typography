package paragraph_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

func TestFitFragmentDirect(t *testing.T) {
	v, key, _ := fitFixture(t, fitTextInput("aa\naa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	f, err := paragraph.Fragment(v)
	if err != nil {
		t.Fatal(err)
	}
	k := key.public()
	for _, tc := range []struct {
		height float64
		lines  int
		status paragraph.FragmentFitStatus
	}{
		{0, 0, paragraph.FitNoProgress}, {math.Nextafter(10, 0), 0, paragraph.FitNoProgress},
		{10, 1, paragraph.FitContinues}, {30, 3, paragraph.FitComplete},
	} {
		out, err := paragraph.FitFragment(f, k, k, tc.height)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Placed.Lines()) != tc.lines || out.Status != tc.status || out.Placed.Box().Extent > tc.height {
			t.Fatal(out)
		}
	}
	for _, bad := range []*layout.Fragment{nil, new(layout.Fragment)} {
		out, err := paragraph.FitFragment(bad, k, k, 10)
		if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(out, err)
		}
	}
	for _, field := range []string{"Source", "Styles", "Fonts", "LineHeight", "Atomics", "Policy"} {
		bad := k
		reflect.ValueOf(&bad).Elem().FieldByName(field).SetUint(0)
		out, err := paragraph.FitFragment(f, bad, bad, 10)
		if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(field, out, err)
		}
	}
	// Consume the same frozen owner over three calls; completed output holds no
	// prepared handle. Each range and each copy of diagnostics is independent.
	for i := 0; i < 3; i++ {
		out, err := paragraph.FitFragment(f, k, k, 10)
		if err != nil || out.Placed.Range() != (layout.LineRange{Start: i, End: i + 1}) {
			t.Fatal(out, err)
		}
		f = out.Remainder
	}
	out, err := paragraph.FitFragment(f, k, k, 0)
	if err != nil || out.Status != paragraph.FitComplete || out.Placed.Range() != (layout.LineRange{Start: 3, End: 3}) {
		t.Fatal(out, err)
	}
}

func TestFitRetainedExtentMustFitStrictly(t *testing.T) {
	v, key, _ := fitFixture(t, fitTextInput("aa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	f, err := paragraph.Fragment(v)
	if err != nil {
		t.Fatal(err)
	}
	box := f.Box()
	box.Extent += 5e-10 // Legal arithmetic discrepancy, not permission to overflow.
	f, err = layout.NewFragment(f.Frozen(), f.Range(), box, f.Lines(), f.Materials(), nil)
	if err != nil {
		t.Fatal("constructor tolerance changed", err)
	}
	for _, h := range []float64{10, box.Extent} {
		out, err := paragraph.FitFragment(f, key.public(), key.public(), h)
		if err != nil {
			t.Fatal(err)
		}
		if h == 10 && out.Status != paragraph.FitNoProgress || h == box.Extent && out.Status != paragraph.FitComplete {
			t.Fatal(out)
		}
	}
}

func TestFitDoesNotHideOutOfDomainInputDepth(t *testing.T) {
	v, key, _ := fitFixture(t, fitTextInput("aa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	lines, mats, box := v.Lines(), v.Materials(), v.Box()
	for i := range lines {
		lines[i].Top, lines[i].Height = float64(i)*100, 100
		lines[i].Ascent, lines[i].Descent = 80, 20
		lines[i].Baseline = lines[i].Top + 80
		mats[i].Top, mats[i].Extent = lines[i].Top, 100
	}
	lines[0].Ascent = -dimen.Fil.Points() + 150
	lines[0].Descent = 100 - lines[0].Ascent
	lines[0].Baseline = lines[0].Ascent
	box.Baseline, box.Extent = lines[0].Baseline, 200
	f, err := layout.NewFragment(v.Frozen(), layout.LineRange{End: 2}, box, []layout.FragmentLine{
		{Line: lines[0], OriginalIndex: 0}, {Line: lines[1], OriginalIndex: 1},
	}, mats, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Both split boxes have bounded depths; the source box's derived depth
	// exceeds Fil. Cutting must not silently conceal that invalid input.
	out, err := paragraph.FitFragment(f, key.public(), key.public(), 100)
	if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal(out, err)
	}
}
