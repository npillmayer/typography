package style_test

import (
	"encoding/json"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/style"
)

func TestDocumentEqualityAndProjections(t *testing.T) {
	red := style.Style{Family: "serif", EmSize: 12 * dimen.BP, Weight: 400, Slant: "normal", Color: "red", Language: "en", WhiteSpace: "normal", UnicodeBidi: "normal"}
	blue := red
	blue.Color = "blue"
	if red.Equals(blue) || red.Equals(nil) || !red.Equals(red) {
		t.Fatal("document equality lost paint or type identity")
	}
	if red.Typography() != blue.Typography() || red.Paint() == blue.Paint() {
		t.Fatal("incorrect projections")
	}
	larger := red
	larger.EmSize++
	if larger.Typography() == red.Typography() {
		t.Fatal("typography lost size")
	}
	text := styled.TextFromString("ab")
	var err error
	text, err = text.Style(red, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	text, err = text.Style(blue, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	a, _, err := text.StyleAt(0)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := text.StyleAt(1)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Equals(red) || !b.Equals(blue) {
		t.Fatal("paint spans coalesced")
	}
}

func TestSpanValidation(t *testing.T) {
	s := style.Style{EmSize: 12 * dimen.BP}
	cases := []struct {
		name, text string
		spans      []style.Span
		valid      bool
	}{
		{"plain", "éa", nil, true}, {"empty", "", []style.Span{}, true},
		{"covered", "éa", []style.Span{{0, 2, s}, {2, 3, s}}, true},
		{"scalar split", "éa", []style.Span{{0, 1, s}, {1, 3, s}}, false},
		{"gap", "éa", []style.Span{{0, 2, s}}, false},
		{"overlap", "éa", []style.Span{{0, 2, s}, {0, 3, s}}, false},
		{"empty span", "éa", []style.Span{{0, 0, s}, {0, 3, s}}, false},
		{"empty list", "éa", []style.Span{}, false},
		{"overflow", "éa", []style.Span{{0, ^uint64(0), s}}, false},
		{"zero size", "a", []style.Span{{0, 1, style.Style{}}}, false},
		{"invalid UTF8", "\xff", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := style.ValidateSpans(tc.text, tc.spans); (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestLegacyJSONShape(t *testing.T) {
	// Keep the existing exported field names/order and numeric unit/direction form.
	want := `{"Start":0,"End":2,"Style":{"Family":"serif","EmSize":0,"Weight":400,"Slant":"normal","Color":"red","Language":"en","WhiteSpace":"normal","UnicodeBidi":"normal","Direction":0}}`
	var span style.Span
	if err := json.Unmarshal([]byte(want), &span); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(span)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("wire format changed: %s", data)
	}
}
