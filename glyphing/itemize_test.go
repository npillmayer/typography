package glyphing_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/glyphing"
)

// Coverage order and failure precedence are observable service contracts: a
// grapheme must be covered by one face, and a cmap failure cannot try another.
func TestItemizeCoverageOrderAndErrors(t *testing.T) {
	cmapErr := errors.New("cmap unavailable")
	for _, tc := range []struct {
		name, text, failAt string
		queries            []string
		errText            string
	}{
		{"whole grapheme", "aa\u0301", "", []string{"primary:U+0061", "primary:U+0061", "primary:U+0301", "fallback:U+0061", "fallback:U+0301"}, ""},
		{"primary failure", "aa\u0301", "primary:U+0301", []string{"primary:U+0061", "primary:U+0061", "primary:U+0301"}, "cmap"},
		{"fallback failure", "aa\u0301", "fallback:U+0301", []string{"primary:U+0061", "primary:U+0061", "primary:U+0301", "fallback:U+0061", "fallback:U+0301"}, "cmap"},
		{"unsupported script", "éΩ", "", []string{"primary:U+00E9", "fallback:U+00E9"}, "unsupported script for U+03A9 at UTF-8 bytes [2,4)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newServices()
			svc := f.services()
			var queries []string
			svc.Coverage = glyphing.CoverageFuncs{Check: newServices().CheckFace, Lookup: func(face contracts.FontFace, c rune) (uint32, error) {
				query := fmt.Sprintf("%s:%U", face.(*fakeFace).name, c)
				queries = append(queries, query)
				if query == tc.failAt {
					return 0, cmapErr
				}
				return f.GlyphIndex(face, c)
			}}
			r, err := loadResolver(t.Context(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Fallbacks: []string{"fallback", "unused"}}, svc)
			if err != nil {
				t.Fatal(err)
			}
			out, bindings, err := r.Itemize(tc.text, nil)
			if !reflect.DeepEqual(queries, tc.queries) {
				t.Fatalf("coverage queries = %v, want %v", queries, tc.queries)
			}
			if tc.errText != "" {
				want := tc.errText
				if tc.failAt != "" {
					path := "primary"
					if tc.name == "fallback failure" {
						path = "fallback"
					}
					want = fmt.Sprintf("font %q cmap: %v", f.entries[path].Info.ResourceID, cmapErr)
					if !errors.Is(err, cmapErr) {
						t.Fatalf("lost cmap cause: %v", err)
					}
				}
				if err == nil || err.Error() != want || bindings != nil {
					t.Fatalf("bindings/error = %v, %v; want nil, %q", bindings, err, want)
				}
				for run := range out.StyleRanges() {
					if run.Style != nil {
						t.Fatal("failure published partial styles")
					}
				}
			} else if err != nil || len(bindings) != 2 || bindings[0].Start != 0 || bindings[0].End != 1 || bindings[0].Path != "primary" || bindings[1].Start != 1 || bindings[1].End != 4 || bindings[1].Path != "fallback" {
				t.Fatalf("grapheme bindings = %+v, error = %v", bindings, err)
			}
			if out.Raw().String() != tc.text {
				t.Fatal("source text changed")
			}
		})
	}
}
