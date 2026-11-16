package typography_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/typography"
)

func TestSelectedCoverageAcceptanceOrderAndFailure(t *testing.T) {
	for _, mode := range []string{"success", "reject unused rule", "cancel check", "structural error"} {
		t.Run(mode, func(t *testing.T) {
			fonts := selectedFixture(t)
			fonts.Faces[0], fonts.Faces[2] = fonts.Faces[2], fonts.Faces[0]
			if mode == "structural error" {
				fonts.Fallbacks = append(fonts.Fallbacks, "unknown")
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var names []string
			coverage := typography.CoverageFuncs{
				Check: func(face contracts.FontFace) error {
					f := face.(*fakeFace)
					names = append(names, f.name)
					if mode == "reject unused rule" && f.name == "rule" {
						return contracts.ErrUnsupportedFontFace
					}
					if mode == "cancel check" {
						cancel()
					}
					return nil
				},
				Lookup: func(contracts.FontFace, rune) (uint32, error) {
					t.Fatal("construction queried coverage")
					return 0, nil
				},
			}
			got, err := typography.NewResolver(ctx, fonts, coverage)
			switch mode {
			case "success":
				if err != nil || got == nil || !reflect.DeepEqual(names, []string{"primary", "rule", "fallback"}) || len(got.Snapshot().Fallbacks) != 2 {
					t.Fatal(got, err, names)
				}
			case "reject unused rule":
				if got != nil || !errors.Is(err, contracts.ErrUnsupportedFontFace) || !strings.Contains(err.Error(), fonts.Rules[0].ResourceID) || !reflect.DeepEqual(names, []string{"primary", "rule"}) {
					t.Fatal(got, err, names)
				}
			case "cancel check":
				if got != nil || !errors.Is(err, context.Canceled) || len(names) != 1 {
					t.Fatal(got, err, names)
				}
			case "structural error":
				if got != nil || err == nil || len(names) != 0 {
					t.Fatal(got, err, names)
				}
			}
		})
	}
}

func TestCoverageFunctionsRejectIncompleteAndForeignFaces(t *testing.T) {
	f := newServices()
	entry, err := f.Load(t.Context(), "primary", nil)
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(contracts.FontFace, rune) (uint32, error) {
		t.Fatal("invalid acceptance reached lookup")
		return 0, nil
	}
	for _, c := range []typography.CoverageFuncs{{}, {Check: f.CheckFace}, {Lookup: lookup}} {
		if err := c.CheckFace(entry.Face); !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(err)
		}
		if _, err := c.GlyphIndex(entry.Face, 'a'); !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(err)
		}
	}
	c := typography.CoverageFuncs{Check: f.CheckFace, Lookup: lookup}
	for _, face := range []contracts.FontFace{nil, (*fakeFace)(nil)} {
		if err := c.CheckFace(face); !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(err)
		}
	}
	c.Check = func(contracts.FontFace) error { return contracts.ErrUnsupportedFontFace }
	if _, err := c.GlyphIndex(entry.Face, 'a'); !errors.Is(err, contracts.ErrUnsupportedFontFace) {
		t.Fatal(err)
	}
}
