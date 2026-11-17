package integration_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
	"golang.org/x/text/unicode/bidi"
)

func TestRetainedRenderServices(t *testing.T) {
	s := renderot.New()
	f := fixture(t)
	info, err := s.Identify(f)
	if err != nil || info.ResourceID != f.Info().ResourceID {
		t.Fatal("identity", err)
	}
	outline, err := s.Outline(f, 1)
	if err != nil || outline.Path == "" || len(outline.Points) == 0 {
		t.Fatal("outline", err)
	}
	if _, err = s.Identify(helpers.NewSyntheticFont()); err == nil {
		t.Fatal("accepted foreign identity")
	}
	if _, err = s.Outline(helpers.NewSyntheticFont(), 1); err == nil {
		t.Fatal("accepted foreign outline")
	}
	logical := []contracts.ShapedGlyph{{GlyphID: 1, Cluster: 0, XOffset: 5}, {GlyphID: 2, Cluster: 0, YOffset: 7}, {GlyphID: 3, Cluster: 1, XAdvance: 100}}
	detached := append([]contracts.ShapedGlyph(nil), logical...)
	visual, err := s.VisualOrder(detached, bidi.RightToLeft)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(visual, []contracts.ShapedGlyph{logical[2], logical[1], logical[0]}) {
		t.Fatal("repeated anchors or positioning changed")
	}
}
