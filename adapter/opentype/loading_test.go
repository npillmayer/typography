package opentype

import (
	"context"
	"errors"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography"
	"golang.org/x/text/language"
	"strings"
	"testing"
)

func TestInjectedLoadingAndCoverageServices(t *testing.T) {
	s := Services{}
	entry, err := s.Load(context.Background(), progressFont, nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Resource == nil || entry.Info.ResourceID != entry.Resource.ID() {
		t.Fatal("resource identity lost")
	}
	gid, err := s.GlyphIndex(entry.Face, 'a')
	if err != nil || gid != 1 {
		t.Fatalf("coverage: %d %v", gid, err)
	}
	pinned, err := s.Load(context.Background(), "", entry.Resource)
	if err != nil || pinned.Info.ResourceID != entry.Info.ResourceID {
		t.Fatal("pinned loading changed identity", err)
	}
	for _, face := range []contracts.FontFace{nil, (*Face)(nil), helpers.NewSyntheticFont()} {
		if _, err = s.GlyphIndex(face, 'a'); err == nil {
			t.Fatal("accepted foreign coverage face")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.Load(ctx, progressFont, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation", err)
	}
	if _, err = s.Load(nil, progressFont, nil); err == nil {
		t.Fatal("nil context")
	}
	if _, err = s.Load(context.Background(), progressFont, entry.Resource); err == nil {
		t.Fatal("ambiguous source")
	}
	if _, err = s.Load(context.Background(), "", nil); err == nil {
		t.Fatal("missing source")
	}
	r, err := typography.NewResolver(context.Background(), typography.SelectedFonts{Faces: []typography.FaceEntry{entry}, Default: entry.Info.ResourceID, Size: 12 * dimen.BP}, s)
	if err != nil {
		t.Fatal(err)
	}
	_, bindings, err := r.Itemize(context.Background(), "fi", nil)
	if err != nil || len(bindings) != 1 || bindings[0].ResourceID != entry.Info.ResourceID {
		t.Fatal("direct resolver composition", err)
	}
}

func TestExtractionContractFeatureCoordinatesWithClusterBase(t *testing.T) {
	stream, err := NewStream(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.SetInitialCluster(7); err != nil {
		t.Fatal(err)
	}
	params := contracts.ShapeParams{Font: fixture(t), Script: language.MustParseScript("Latn"), Language: language.English, Features: []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Enabled: true, Start: 2, End: 4}}}
	done := make(chan error, 1)
	go func() {
		err := stream.Shape(context.Background(), params, strings.NewReader("fifi"), contracts.ShapeBufferOptions{Flush: contracts.ShapeFlushCluster})
		if err == nil {
			err = stream.Finish()
		}
		done <- err
	}()
	var got []contracts.ShapedGlyph
	for g := range stream.Glyphs() {
		got = append(got, g)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// ShaperTestProgress's test feature ligates only the second fi. Feature ranges
	// remain run-relative scalars while emitted cluster anchors include the base.
	if len(got) != 3 {
		t.Fatalf("glyphs %+v", got)
	}
	for i, want := range []struct{ id, cluster uint32 }{{4, 7}, {5, 8}, {6, 9}} {
		if got[i].GlyphID != want.id || got[i].Cluster != want.cluster {
			t.Fatalf("glyph %d %+v", i, got[i])
		}
	}
	if params.Features[0].Start != 2 || params.Features[0].End != 4 {
		t.Fatal("feature input changed")
	}
}
