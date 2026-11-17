package glyphing_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/fontresource"
	"github.com/npillmayer/typography/glyphing"
)

func selectedFixture(t *testing.T) glyphing.SelectedFonts {
	t.Helper()
	f := newServices()
	var entries []glyphing.FaceEntry
	for _, name := range []string{"primary", "rule", "fallback"} {
		e, err := f.Load(t.Context(), name, nil)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	return glyphing.SelectedFonts{Faces: entries, Default: entries[0].Info.ResourceID, Size: 12 * dimen.BP, Rules: []glyphing.SelectedRule{{Weight: 700, ResourceID: entries[1].Info.ResourceID}}, Fallbacks: []string{entries[2].Info.ResourceID, entries[2].Info.ResourceID}}
}
func TestSelectedFontsCanonicalOrderAndOwnership(t *testing.T) {
	s := selectedFixture(t)
	want := []string{s.Default, s.Rules[0].ResourceID, s.Fallbacks[0]}
	s.Faces[0], s.Faces[2] = s.Faces[2], s.Faces[0]
	coverage := glyphing.CoverageFuncs{Check: newServices().CheckFace, Lookup: func(contracts.FontFace, rune) (uint32, error) { t.Fatal("constructor queried coverage"); return 0, nil }}
	r, err := glyphing.NewResolver(t.Context(), s, coverage)
	if err != nil {
		t.Fatal(err)
	}
	snap := r.Snapshot()
	var ids []string
	for i, e := range snap.Faces {
		ids = append(ids, e.Info.ResourceID)
		if e.Info.ResourceID != r.Infos()[i].ResourceID {
			t.Fatal("Infos order")
		}
	}
	if !reflect.DeepEqual(ids, want) || len(snap.Fallbacks) != 2 {
		t.Fatalf("canonical roles: %+v", snap)
	}
	s.Faces[0].Info.Warnings[0] = "caller"
	s.Rules[0].Weight = 1
	s.Fallbacks[0] = "caller"
	snap.Faces[0].Info.Warnings[0] = "snapshot"
	snap.Faces[0].Info.Path = "snapshot"
	snap.Rules[0].ResourceID = "snapshot"
	snap.Fallbacks[0] = "snapshot"
	again := r.Snapshot()
	if again.Faces[0].Info.Warnings[0] != "test warning" || again.Faces[0].Info.Path != "primary" || again.Rules[0].Weight != 700 || again.Rules[0].ResourceID != want[1] || again.Fallbacks[0] != want[2] {
		t.Fatal("selection aliases mutable metadata")
	}
	if again.Faces[0].Face != snap.Faces[0].Face || again.Faces[0].Resource != snap.Faces[0].Resource {
		t.Fatal("immutable handles not shared")
	}
}
func TestSelectedFontsRejectInvalidEntriesAndReferences(t *testing.T) {
	cases := map[string]func(*glyphing.SelectedFonts){
		"size zero": func(s *glyphing.SelectedFonts) { s.Size = 0 }, "size infinite": func(s *glyphing.SelectedFonts) { s.Size = dimen.Fil },
		"missing default": func(s *glyphing.SelectedFonts) { s.Default = "" }, "unknown rule": func(s *glyphing.SelectedFonts) { s.Rules[0].ResourceID = "missing" }, "unknown fallback": func(s *glyphing.SelectedFonts) { s.Fallbacks[0] = "missing" },
		"duplicate": func(s *glyphing.SelectedFonts) { s.Faces = append(s.Faces, s.Faces[0]) }, "unreferenced": func(s *glyphing.SelectedFonts) { s.Rules = nil },
		"nil face": func(s *glyphing.SelectedFonts) { s.Faces[0].Face = nil }, "typed nil face": func(s *glyphing.SelectedFonts) { s.Faces[0].Face = (*fakeFace)(nil) }, "nil resource": func(s *glyphing.SelectedFonts) { s.Faces[0].Resource = nil },
		"ID": func(s *glyphing.SelectedFonts) { s.Faces[0].Info.ResourceID = "wrong" }, "hash": func(s *glyphing.SelectedFonts) { s.Faces[0].Info.SHA256 = "wrong" }, "origin": func(s *glyphing.SelectedFonts) { s.Faces[0].Info.Origin.Provider = "wrong" }, "path": func(s *glyphing.SelectedFonts) { s.Faces[0].Info.Path = "wrong" },
		"weight": func(s *glyphing.SelectedFonts) { s.Rules[0].Weight = 1001 }, "slant": func(s *glyphing.SelectedFonts) { s.Rules[0].Slant = "sideways" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := selectedFixture(t)
			mutate(&s)
			if r, err := glyphing.NewResolver(t.Context(), s, newServices()); r != nil || err == nil {
				t.Fatal("published invalid selection")
			}
		})
	}
	for _, coverage := range []glyphing.Coverage{nil, (*fakeServices)(nil), glyphing.CoverageFuncs{}} {
		if r, err := glyphing.NewResolver(t.Context(), selectedFixture(t), coverage); r != nil || err == nil {
			t.Fatal("accepted unusable coverage")
		}
	}
}
func TestSelectedFontsCancellationAndCoverageFailure(t *testing.T) {
	s := selectedFixture(t)
	f := newServices()
	if r, err := glyphing.NewResolver(nil, s, f); r != nil || err == nil {
		t.Fatal("nil context")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if r, err := glyphing.NewResolver(ctx, s, f); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation", err)
	}
	r, err := glyphing.NewResolver(t.Context(), s, f)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.Itemize(nil, "", nil); err == nil {
		t.Fatal("nil itemization context")
	}
	if _, _, err = r.Itemize(ctx, "", nil); !errors.Is(err, context.Canceled) {
		t.Fatal("empty itemization ignored cancellation")
	}
	f.coverageErr = errors.New("coverage failed")
	if _, err = r.CheckText(t.Context(), "a", nil); !errors.Is(err, f.coverageErr) {
		t.Fatal("coverage error hidden", err)
	}
	// A configured rule face is not an implicit fallback merely because it is in the table.
	s.Fallbacks = nil
	s.Faces = s.Faces[:2]
	r, err = glyphing.NewResolver(t.Context(), s, newServices())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CheckText(t.Context(), "é", nil); err == nil {
		t.Fatal("table order used as fallback")
	}
}

func TestSelectedFontsKeepDistinctOriginsForSameBytes(t *testing.T) {
	s := selectedFixture(t)
	// Resource identity includes provenance; equality of font bytes never merges roles.
	entry := s.Faces[0]
	reader := entry.Resource.Open()
	defer reader.Close()
	origin := entry.Info.Origin
	origin.Location = "another origin"
	resource, err := fontresource.Read(reader, origin)
	if err != nil {
		t.Fatal(err)
	}
	entry.Resource = resource
	entry.Info.ResourceID = resource.ID()
	entry.Info.Origin = origin
	entry.Info.Path = origin.Location
	s.Faces = append(s.Faces, entry)
	s.Fallbacks = append(s.Fallbacks, resource.ID())
	r, err := glyphing.NewResolver(t.Context(), s, newServices())
	if err != nil {
		t.Fatal(err)
	}
	snap := r.Snapshot()
	if len(snap.Faces) != 4 || snap.Faces[0].Info.SHA256 != snap.Faces[3].Info.SHA256 || snap.Faces[0].Info.ResourceID == snap.Faces[3].Info.ResourceID {
		t.Fatal("content equality collapsed provenance identity")
	}
}
