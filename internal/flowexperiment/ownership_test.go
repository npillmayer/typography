package flowexperiment

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

type expiringContext struct {
	context.Context
	calls, limit int
}

func (c *expiringContext) Err() error {
	c.calls++
	if c.limit > 0 && c.calls >= c.limit {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestRetryCancellationAndDetachedOwnership(t *testing.T) {
	v, key, c := fixture(t, textInput("aa\naa\naa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
	before := v.Lines()
	materials := v.Materials()
	box := v.Box()
	counts := c.snapshot()
	probe := &expiringContext{Context: t.Context()}
	first, err := splitEqual(probe, v, key, frame{key, 10}, frame{key, 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []int{1, probe.calls / 2, probe.calls} {
		ctx := &expiringContext{Context: t.Context(), limit: at}
		got, err := splitEqual(ctx, v, key, frame{key, 10}, frame{key, 10})
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
				out, err := splitEqual(t.Context(), v, key, frame{key, 10}, frame{key, 10})
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
	verifyPartition(t, v, first)
}

func TestRetainedContentLifetime(t *testing.T) {
	out := func() *splitResult {
		v, key, _ := fixture(t, textInput("aa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
		out, err := splitEqual(t.Context(), v, key, frame{key, 10}, frame{key, 10})
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

func TestLateGeometryFailureAndDepthBounds(t *testing.T) {
	input := textInput("aa\n\uFFFC")
	input.Atomics = []contracts.AtomicAnchor{{ID: 1, ByteOffset: 3}}
	v, key, _ := fixture(t, input, paragraph.FirstFit, 40*dimen.BP, 1, []paragraph.AtomicBox{{ID: 1, Alignment: paragraph.AlignBottomMarginEdge}})
	atoms := v.AtomicPlacements()
	atoms[0].Y = -dimen.Fil.Points() + .5
	bad, err := layout.NewVList(v.Frozen(), v.Box(), v.Lines(), v.Materials(), atoms)
	if err != nil {
		t.Fatal(err)
	}
	out, err := splitEqual(t.Context(), bad, key, frame{key, 10}, frame{key, 10})
	if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal("published partial geometry", out, err)
	}
	if out, err = splitEqual(t.Context(), v, key, frame{key, 10}, frame{key, 10}); err != nil {
		t.Fatal(err)
	}
	verifyPartition(t, v, out)

	// A constructor-accepted arithmetic discrepancy must not turn into a local
	// depth at or beyond Fil. The same tolerance remains valid inside the domain.
	plain, key, _ := fixture(t, textInput("aa\naa"), paragraph.FirstFit, 40*dimen.BP, 1, nil)
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
		out, err := splitEqual(t.Context(), source, key, frame{key, 100}, frame{key, 100})
		if offset < 0 {
			if out != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
				t.Fatal(out, err)
			}
		} else if err != nil {
			t.Fatal("tightened arithmetic tolerance", err)
		}
	}
}
