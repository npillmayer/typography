package glyphing_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing"
	"github.com/npillmayer/typography/style"
)

func assertColorFree(t *testing.T, value any) {
	t.Helper()
	var inspect func(reflect.Type)
	inspect = func(typ reflect.Type) {
		if typ.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Name == "Color" || field.Type == reflect.TypeOf(style.Paint{}) {
				t.Fatalf("typesetter style retains paint: %T (%s)", value, field.Name)
			}
			inspect(field.Type)
		}
	}
	inspect(reflect.TypeOf(value))
}

func TestRenderingBypassProjection(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	red := documentStyle()
	blue := red
	blue.Color = "blue"
	spans := []style.Span{{Start: 0, End: 1, Style: red}, {Start: 1, End: 2, Style: blue}}

	t.Run("glyph-boundary", func(t *testing.T) {
		projected, err := glyphing.ProjectStyles("fi", spans, r, r.Defaults())
		if err != nil {
			t.Fatal(err)
		}
		want := 1

		if len(projected) != want || projected[0].Style.Typography != red.Typography() {
			t.Fatalf("projection: %+v", projected)
		}
		for _, span := range projected {
			assertColorFree(t, span.Style)
			resolved, err := r.ResolveStyle(span.Style, r.Defaults())
			if err != nil || resolved.EmSize != red.EmSize {
				t.Fatalf("projection lost typography: %+v %v", resolved, err)
			}
		}
		text, bindings, err := r.Itemize("fi", spans)
		if err != nil || len(bindings) != want {
			t.Fatalf("itemization: %+v %v", bindings, err)
		}
		for _, pos := range []uint64{0, 1} {
			value, _, err := text.StyleAt(pos)
			if err != nil {
				t.Fatal(err)
			}
			assertColorFree(t, value)
		}
	})

	// Identical source styles also coalesce.
	spans[1].Style = red
	_, bindings, err := r.Itemize("fi", spans)
	if err != nil || len(bindings) != 1 {
		t.Fatalf("identical styles split: %+v %v", bindings, err)
	}
}

func TestRenderingBypassKeepsFeatureDomainsWithoutColor(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	red := documentStyle()
	blue := red
	blue.Color = "blue"
	spans := []style.Span{{Start: 0, End: 4, Style: red}, {Start: 4, End: 8, Style: blue}}
	defaults := r.Defaults()
	defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Enabled: true, Start: 2, End: 4}}
	projected, err := glyphing.ProjectStyles("fifififi", spans, r, defaults)
	if err != nil || len(projected) != 2 {
		t.Fatalf("feature domains merged: %+v %v", projected, err)
	}
	if projected[0].Style.Equals(projected[1].Style) {
		t.Fatal("style storage would merge relative feature domains")
	}
	for _, span := range projected {
		assertColorFree(t, span.Style)
	}
	if spans[0].Style.Color != "red" || spans[1].Style.Color != "blue" || defaults.Features[0].Start != 2 {
		t.Fatal("projection mutated document paint or feature defaults")
	}
	// Removing a color difference must not remove a feature-domain boundary.
	spans[1].Style.Color = "red"
	again, err := glyphing.ProjectStyles("fifififi", spans, r, defaults)
	if err != nil || !reflect.DeepEqual(projected, again) {
		t.Fatalf("color determined feature-domain identity: %+v %v", again, err)
	}
}
