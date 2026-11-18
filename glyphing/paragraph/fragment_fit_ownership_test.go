package paragraph_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

type fitExpiringContext struct {
	context.Context
	calls, limit int
}

func (c *fitExpiringContext) Err() error {
	c.calls++
	if c.limit > 0 && c.calls >= c.limit {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestFitRetryCancellationAndDetachedOwnership(t *testing.T) {
	v, key, c := fitFixture(t, fitTextInput("aa\naa\naa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	before := v.Lines()
	materials := v.Materials()
	box := v.Box()
	counts := c.snapshot()
	probe := &fitExpiringContext{Context: t.Context()}
	first, err := fitSplitEqual(probe, v, key, fitFrame{key, 10}, fitFrame{key, 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []int{1, probe.calls / 2, probe.calls} {
		ctx := &fitExpiringContext{Context: t.Context(), limit: at}
		got, err := fitSplitEqual(ctx, v, key, fitFrame{key, 10}, fitFrame{key, 10})
		if got != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("partial cancelled result", got, err, at)
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				out, err := fitSplitEqual(t.Context(), v, key, fitFrame{key, 10}, fitFrame{key, 10})
				if err != nil {
					t.Error(err)
					return
				}
				if !reflect.DeepEqual(out.a.Lines(), first.a.Lines()) || !reflect.DeepEqual(out.tail.Lines(), first.tail.Lines()) {
					t.Error("unstable geometry")
					return
				}
				clear(out.a.Lines())
				clear(out.b.Materials())
				clear(out.tail.AtomicPlacements())
				out.a = nil
				out.overfull = nil // Detached envelope.
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(v.Lines(), before) || !reflect.DeepEqual(v.Materials(), materials) || v.Box() != box || c.snapshot() != counts {
		t.Fatal("mutated source or queried services")
	}
	fitVerifyPartition(t, v, first)
}

func TestFitRetainedContentLifetime(t *testing.T) {
	out := func() *fitSplitResult {
		v, key, _ := fitFixture(t, fitTextInput("aa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
		out, err := fitSplitEqual(t.Context(), v, key, fitFrame{key, 10}, fitFrame{key, 10})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}()
	runtime.GC()
	found := false
	for _, l := range out.b.Lines() {
		for k := l.Horizontal.FirstKnot; k < l.Horizontal.FirstKnot+l.Horizontal.KnotCount; k++ {
			text, ok := out.b.Frozen().CopyText(k)
			if !ok || len(text.Glyphs) == 0 {
				continue
			}
			found = true
			id := text.Glyphs[0].GlyphID
			text.Glyphs[0].GlyphID++
			again, _ := out.b.Frozen().CopyText(k)
			if again.Glyphs[0].GlyphID != id {
				t.Fatal("mutable glyph alias")
			}
		}
	}
	if !found {
		t.Fatal("lost glyph content")
	}
}

func TestFitLateGeometryFailureAndDepthBounds(t *testing.T) {
	input := fitTextInput("aa\n\uFFFC")
	input.Atomics = []contracts.AtomicAnchor{{ID: 1, ByteOffset: 3}}
	v, key, _ := fitFixture(t, input, paragraph.FirstFit, 40*dimen.BP, 1, []paragraph.AtomicBox{{ID: 1, Alignment: paragraph.AlignBottomMarginEdge}})
	atoms := v.AtomicPlacements()
	atoms[0].Y = -dimen.Fil.Points() + .5
	bad, err := layout.NewVList(v.Frozen(), v.Box(), v.Lines(), v.Materials(), atoms)
	if err != nil {
		t.Fatal(err)
	}
	out, err := fitSplitEqual(t.Context(), bad, key, fitFrame{key, 10}, fitFrame{key, 10})
	if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal("published partial geometry", out, err)
	}
	if out, err = fitSplitEqual(t.Context(), v, key, fitFrame{key, 10}, fitFrame{key, 10}); err != nil {
		t.Fatal(err)
	}
	fitVerifyPartition(t, v, out)

	// A constructor-accepted arithmetic discrepancy must not turn into a local
	// depth at or beyond Fil. The same tolerance remains valid inside the domain.
	plain, key, _ := fitFixture(t, fitTextInput("aa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	lines, mats, b := plain.Lines(), plain.Materials(), plain.Box()
	for i := range lines {
		lines[i].Top = float64(i) * 100
		lines[i].Height = 100
		lines[i].Ascent, lines[i].Descent = 80, 20
		lines[i].Baseline = lines[i].Top + 80
		mats[i].Top, mats[i].Extent = lines[i].Top, 100
	}
	b.Extent, b.Baseline = 200, 80
	lines[1].Descent = math.Nextafter(dimen.Fil.Points(), 0)
	lines[1].Ascent = 100 - lines[1].Descent
	for _, offset := range []float64{5e-10, -5e-10} {
		lines[1].Baseline = lines[1].Top + lines[1].Ascent + offset
		source, err := layout.NewVList(plain.Frozen(), b, lines, mats, nil)
		if err != nil {
			t.Fatal("source tolerance changed", err)
		}
		out, err := fitSplitEqual(t.Context(), source, key, fitFrame{key, 100}, fitFrame{key, 100})
		if offset < 0 {
			if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
				t.Fatal(out, err)
			}
		} else if err != nil {
			t.Fatal("tightened arithmetic tolerance", err)
		}
	}
}
