package opentype

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

type queryTrapFace struct{}

func (*queryTrapFace) Metrics() (contracts.FontMetrics, error)     { panic("acceptance queried metrics") }
func (*queryTrapFace) Scale(dimen.DU) (contracts.FontScale, error) { panic("acceptance queried scale") }

func TestFaceAcceptanceAndDirectValidation(t *testing.T) {
	face := fixture(t)
	stream, _ := NewStream(8)
	if err := stream.SetInitialCluster(7); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		face contracts.FontFace
		want error
	}{
		{face, nil}, {nil, contracts.ErrIllegalArguments}, {(*Face)(nil), contracts.ErrIllegalArguments},
		{&Face{}, contracts.ErrIllegalArguments}, {new(queryTrapFace), contracts.ErrUnsupportedFontFace},
		{(*queryTrapFace)(nil), contracts.ErrIllegalArguments}, {helpers.NewSyntheticFont(), contracts.ErrUnsupportedFontFace},
	} {
		for _, consumer := range []contracts.FontAcceptor{stream, Services{}} {
			if err := consumer.CheckFace(tc.face); !errors.Is(err, tc.want) {
				t.Fatalf("%T: %v", tc.face, err)
			}
		}
		if tc.want != nil {
			if _, err := (Services{}).GlyphIndex(tc.face, 'a'); !errors.Is(err, tc.want) {
				t.Fatal("direct coverage", err)
			}
			direct, _ := NewStream(1)
			err := direct.Shape(t.Context(), contracts.ShapeParams{Font: tc.face}, strings.NewReader("a"), contracts.ShapeBufferOptions{})
			if !errors.Is(err, tc.want) || !errors.Is(direct.Err(), tc.want) {
				t.Fatal("lost terminal lifecycle", err, direct.Err())
			}
			if _, ok := <-direct.Glyphs(); ok {
				t.Fatal("invalid face emitted glyph")
			}
		}
	}
	if stream.Err() != nil || stream.next != 7 || stream.running || stream.closed || len(stream.out) != 0 {
		t.Fatal("check changed stream")
	}
	// Query/shaping locks are not needed by acceptance; it reads preparation state.
	face.mu.Lock()
	err := stream.CheckFace(face)
	face.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				if err := stream.CheckFace(face); err != nil {
					t.Error(err)
				}
				if err := (Services{}).CheckFace(face); err != nil {
					t.Error(err)
				}
			}
		})
	}
	err = stream.Shape(t.Context(), contracts.ShapeParams{Font: face}, strings.NewReader("a"), contracts.ShapeBufferOptions{})
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if g := <-stream.Glyphs(); g.Cluster != 7 {
		t.Fatal("preflight changed cluster base", g)
	}
	if err := stream.Finish(); err != nil {
		t.Fatal(err)
	}
	if err := stream.CheckFace(face); err != nil {
		t.Fatal("acceptance depends on closed state", err)
	}
	if err := stream.Finish(); err != nil {
		t.Fatal("idle cleanup not idempotent", err)
	}
}
