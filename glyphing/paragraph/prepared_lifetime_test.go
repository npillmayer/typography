package paragraph_test

import (
	"io"
	"reflect"
	"runtime"
	"testing"
	"weak"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/typography/glyphing/paragraph"
	"github.com/npillmayer/typography/layout"
)

const lifetimeSource = "aa bb cc dd ee ff gg hh"

// Return only a weak preparation reference and frozen output. No caller stack
// frame, fixture closure or unfinished token may keep the preparation alive.
func frozenLifetimeFixture(t *testing.T, mode string) (weak.Pointer[paragraph.Prepared], []*layout.Fragment) {
	t.Helper()
	calls := new(fitServiceCalls)
	p, err := paragraph.Prepare(fitTextInput(lifetimeSource), khipu.PrepareOptions{}, flowConfig(calls))
	if err != nil {
		t.Fatal(err)
	}
	req := flowRequest(paragraph.FirstFit)
	if mode == "ordinary layout" {
		result, err := p.Layout(req)
		if err != nil {
			t.Fatal(err)
		}
		fragment, err := paragraph.Fragment(result.VList)
		if err != nil {
			t.Fatal(err)
		}
		return weak.Make(p), []*layout.Fragment{fragment}
	}
	initial, err := p.StartFlow(req)
	if err != nil {
		t.Fatal(err)
	}
	first := fitFlow(t, initial, frame(40, 10))
	if first.Continuation == nil {
		t.Fatal("fixture needs an unfinished paragraph")
	}
	target := frame(40, 1000)
	if mode == "changed width" {
		target = frame(30, 1000)
	}
	last := fitFlow(t, first.Continuation, target)
	if last.Status != paragraph.FitComplete || last.Continuation != nil {
		t.Fatal("fixture did not complete")
	}
	return weak.Make(p), []*layout.Fragment{first.Placed, last.Placed}
}

func TestFrozenFragmentsReleasePreparation(t *testing.T) {
	for _, mode := range []string{"ordinary layout", "equal width", "changed width"} {
		t.Run(mode, func(t *testing.T) {
			prepared, fragments := frozenLifetimeFixture(t, mode)
			before := make([][]layout.FragmentLine, len(fragments))
			for i, f := range fragments {
				before[i] = f.Lines()
			}
			// Weak pointers do not keep the preparation alive. Force full GC
			// cycles without calling Value between them (which would temporarily
			// restore a strong reference). Keep every frozen owner rooted below.
			for i := 0; i < 5; i++ {
				runtime.GC()
			}
			if prepared.Value() != nil {
				t.Error("frozen fragments still retain working preparation")
			}
			var end uint64
			glyphs := 0
			for i, f := range fragments {
				if !reflect.DeepEqual(before[i], f.Lines()) {
					t.Fatal("retained geometry changed")
				}
				reader, err := f.Frozen().SourceReader(khipu.ByteRange{End: f.Frozen().SourceLen()})
				if err != nil {
					t.Fatal(err)
				}
				text, err := io.ReadAll(reader)
				if err != nil || string(text) != lifetimeSource {
					t.Fatal("lost retained source", string(text), err)
				}
				for _, line := range f.Lines() {
					if line.Horizontal.Source.Start != end {
						t.Fatal("source gap or overlap")
					}
					end = line.Horizontal.Source.End
					for k := line.Horizontal.FirstKnot; k < line.Horizontal.FirstKnot+line.Horizontal.KnotCount; k++ {
						if text, ok := f.Frozen().CopyText(k); ok {
							glyphs += len(text.Glyphs)
						}
					}
				}
			}
			if end != uint64(len(lifetimeSource)) || glyphs != 16 {
				t.Fatal("lost retained glyphs/source coverage", glyphs, end)
			}
			runtime.KeepAlive(fragments)
		})
	}
}
