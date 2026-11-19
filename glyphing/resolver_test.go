package glyphing_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
	"github.com/npillmayer/typography/glyphing"
	"github.com/npillmayer/typography/style"
)

type fakeFace struct {
	contracts.FontFace
	name, chars string
}
type fakeServices struct {
	calls       []string
	entries     map[string]glyphing.FaceEntry
	coverageErr error
}

func newServices() *fakeServices { return &fakeServices{entries: map[string]glyphing.FaceEntry{}} }
func (f *fakeServices) Load(ctx context.Context, path string, res *fontresource.Resource) (glyphing.FaceEntry, error) {
	if res != nil {
		path = res.Origin().Location
	}
	f.calls = append(f.calls, path)
	if path == "broken" {
		return glyphing.FaceEntry{}, errors.New("load failed")
	}
	if res == nil {
		res, _ = fontresource.Read(strings.NewReader(path), fontresource.Origin{Provider: "fake", Location: path})
	}
	chars := "aefix "
	if path != "primary" {
		chars += "éאבְ\u0301"
	}
	face := &fakeFace{FontFace: helpers.NewSyntheticFont(), name: path, chars: chars}
	entry := glyphing.FaceEntry{Face: face, Resource: res, Info: fontmeta.Info{ResourceID: res.ID(), SHA256: res.SHA256(), Origin: res.Origin(), Path: path, Family: path, Warnings: []string{"test warning"}}}
	f.entries[path] = entry
	return entry, nil
}
func (f *fakeServices) GlyphIndex(face contracts.FontFace, c rune) (uint32, error) {
	if f.coverageErr != nil {
		return 0, f.coverageErr
	}
	if strings.ContainsRune(face.(*fakeFace).chars, c) {
		return 1, nil
	}
	return 0, nil
}
func (f *fakeServices) services() testServices {
	return testServices{Loader: f, Coverage: f}
}
func documentStyle() style.Style {
	return style.Style{Family: "serif", EmSize: 18 * dimen.BP, Weight: 700, Slant: "normal", Color: "red", Language: "en", WhiteSpace: "normal", UnicodeBidi: "normal"}
}
func load(t *testing.T, f *fakeServices, s fixtureSettings) *glyphing.StyleMapper {
	t.Helper()
	r, err := loadResolver(context.Background(), s, f.services())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMappingAndSnapshotOwnership(t *testing.T) {
	f := newServices()
	s := fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Rules: []fixtureRule{{Family: "serif", Weight: 700, Path: "first"}, {Weight: 700, Path: "second"}}, Fallbacks: []string{"fallback"}}
	r := load(t, f, s)
	s.Rules[0].Path = "mutated"
	s.Fallbacks[0] = "mutated"
	css := documentStyle()
	css.Family = `"Other", 'SeRiF'`
	defaults := r.Defaults()
	defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'t', 'e', 's', 't'}, Enabled: true, Start: 2, End: 4}}
	resolved, err := r.ResolveStyle(css, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Font.(*fakeFace).name != "first" || resolved.EmSize != 18*dimen.BP {
		t.Fatalf("first match/size %+v", resolved)
	}
	resolved.Features[0].Start = 99
	if defaults.Features[0].Start != 2 {
		t.Fatal("features alias caller")
	}
	// Neither original callback metadata nor returned metadata may mutate the resolver.
	e := f.entries["first"]
	e.Info.Warnings[0] = "caller mutation"
	infos := r.Infos()
	for _, i := range infos {
		if i.Warnings[0] != "test warning" {
			t.Fatal("loader metadata aliased")
		}
	}
	infos[0].Warnings[0] = "mutated"
	if r.Infos()[0].Warnings[0] != "test warning" {
		t.Fatal("Infos aliases metadata")
	}
	before := len(f.calls)
	snap := r.Snapshot()
	if len(f.calls) != before || len(snap.Faces) != 4 || len(snap.Rules) != 2 || snap.Rules[1].ResourceID != f.entries["second"].Info.ResourceID || snap.Fallbacks[0] != f.entries["fallback"].Info.ResourceID {
		t.Fatalf("incomplete snapshot %+v", snap)
	}

	snap.Rules[0].Weight = 0
	if r.Snapshot().Rules[0].Weight != 700 {
		t.Fatal("snapshot aliases policy")
	}
	replay, err := glyphing.NewStyleMapper(*r.Snapshot(), f)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.CheckText("éa", nil)
	b, _ := replay.CheckText("éa", nil)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("snapshot binding identity changed: %+v %+v", a, b)
	}
}

func TestGraphemesScriptsAndSoftHyphenContext(t *testing.T) {
	f := newServices()
	r := load(t, f, fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Fallbacks: []string{"fallback", "unused"}})
	for _, text := range []string{"é\u00adé", "\u00adé", "é\u00ad\u00ad"} {
		b, err := r.CheckText(text, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) != 1 || b[0].Start != 0 || b[0].End != uint64(len(text)) || b[0].Path != "fallback" || b[0].ResourceID != f.entries["fallback"].Info.ResourceID {
			t.Fatalf("lost discretionary context: %q %+v", text, b)
		}
	}
	b, err := r.CheckText("a\u0301a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 2 || b[0].End != 3 || b[0].Path != "fallback" || b[1].Start != 3 || b[1].Path != "primary" {
		t.Fatalf("grapheme/fallback %+v", b)
	}
	b, err = r.CheckText("aאב", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 2 || b[0].Script != "Latn" || b[1].Script != "Hebr" || b[1].End != 5 {
		t.Fatalf("script byte ranges %+v", b)
	}
	b, err = r.CheckText("\u00ad\u00ad", nil)
	if err != nil || len(b) != 1 || b[0].Path != "primary" {
		t.Fatalf("marker-only primary %+v %v", b, err)
	}
	for _, text := range []string{"Ω", "X"} {
		if _, err = r.CheckText(text, nil); err == nil {
			t.Fatalf("accepted unsupported/missing %q", text)
		}
	}
	css := documentStyle()
	larger := css
	larger.EmSize *= 2
	if _, err = r.CheckText("a\u0301", []style.Span{{Start: 0, End: 1, Style: css}, {Start: 1, End: 3, Style: larger}}); err == nil {
		t.Fatal("split grapheme")
	}
	f.coverageErr = errors.New("cmap failed")
	if _, err = r.CheckText("é", nil); !errors.Is(err, f.coverageErr) {
		t.Fatalf("coverage failure hidden: %v", err)
	}
}

func TestPaintProjectionMergesShapingOnly(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	red := documentStyle()
	blue := red
	blue.Color = "blue"
	spans := []style.Span{{Start: 0, End: 1, Style: red}, {Start: 1, End: 2, Style: blue}}
	paint := style.ProjectPaint(spans)
	result, err := r.ItemizeWithMetadata("aa", spans)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 || len(result.Shaping) != 1 || len(paint) != 2 {
		t.Fatalf("paint boundary merged: %+v", result)
	}
	if paint[0].Paint.Color != "red" || paint[1].Paint.Color != "blue" {
		t.Fatal("projection changed typography/paint")
	}
	a, _, _ := result.Text.StyleAt(0)
	b, _, _ := result.Text.StyleAt(1)
	if !a.Equals(b) || red.Equals(blue) {
		t.Fatal("shaping/document equality conflated")
	}
	spans[0].Style.Color = "green"
	if paint[0].Paint.Color != "red" {
		t.Fatal("paint aliases source")
	}
}

func TestSoftHyphensCrossPaintSpans(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Fallbacks: []string{"fallback"}})
	red := documentStyle()
	blue := red
	blue.Color = "blue"
	b, err := r.CheckText("é\u00ad", []style.Span{{Start: 0, End: 2, Style: red}, {Start: 2, End: 4, Style: blue}})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 || b[0].Path != "fallback" {
		t.Fatalf("paint broke soft-hyphen context: %+v", b)
	}
}
func TestCoalescingPreservesFeatureOrigins(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	a := documentStyle()
	b := a
	b.Color = "blue"
	spans := []style.Span{{Start: 0, End: 1, Style: a}, {Start: 1, End: 2, Style: b}}
	defaults := r.Defaults()
	defaults.Features = []contracts.ShapeFeature{{Tag: [4]byte{'l', 'i', 'g', 'a'}, Start: 1, End: 2, Enabled: true}}
	grouped, err := glyphing.CoalesceStyles("aa", spans, r, defaults)
	if err != nil || len(grouped) != 2 || defaults.Features[0].Start != 1 {
		t.Fatal("relative feature origins lost", err)
	}
	b.Family = "another request"
	b.Weight = 400
	spans[1].Style = b
	grouped, err = glyphing.CoalesceStyles("aa", spans, r, r.Defaults())
	if err != nil || len(grouped) != 1 {
		t.Fatal("equivalent mapped requests did not merge", err)
	}
	spans[1].Style.EmSize *= 2
	grouped, err = glyphing.CoalesceStyles("aa", spans, r, r.Defaults())
	if err != nil || len(grouped) != 2 {
		t.Fatal("size boundary merged", err)
	}
}

func (*fakeServices) CheckFace(face contracts.FontFace) error {
	f, ok := face.(*fakeFace)
	if !ok {
		return contracts.ErrUnsupportedFontFace
	}
	if f == nil || f.FontFace == nil {
		return contracts.ErrIllegalArguments
	}
	return nil
}
