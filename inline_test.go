package typography_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/typography"
	"github.com/npillmayer/typography/style"
	"github.com/npillmayer/uax/bidi"
)

func TestItemizeInlineObjectOnlyBypassesResolutionAndCoverage(t *testing.T) {
	f := newServices()
	r := load(t, f, fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	f.coverageErr = errors.New("coverage must not be called")
	s := documentStyle()
	s.Language = "not a language!" // Resolving this style would fail.

	text := "\uFFFC\uFFFC"
	spans := []style.Span{{Start: 0, End: 6, Style: s}}
	anchors := []contracts.AtomicAnchor{{ID: 9}, {ID: 2, ByteOffset: 3}}
	got, bindings, err := r.ItemizeInline(t.Context(), text, spans, anchors)
	if err != nil || len(bindings) != 0 || got.Raw().String() != text {
		t.Fatal(got, bindings, err)
	}
	for run := range got.StyleRanges() {
		if run.Style != nil {
			t.Fatal("marker acquired a text style")
		}
	}
	if spans[0].Style != s || anchors[0].ID != 9 {
		t.Fatal("caller inputs mutated")
	}

}

func TestItemizeInlineMarkerBarriers(t *testing.T) {
	for _, tc := range []struct {
		text           string
		offset         uint64
		ranges         [][2]uint64
		fonts, scripts []string
	}{
		{"a\uFFFC\u0301a", 1, [][2]uint64{{0, 1}, {4, 6}, {6, 7}}, []string{"primary", "fallback", "primary"}, []string{"Latn", "Latn", "Latn"}},
		{"é\u00ad\uFFFC\u00ada", 4, [][2]uint64{{0, 4}, {7, 10}}, []string{"fallback", "primary"}, []string{"Latn", "Latn"}},
		{"א\uFFFC ", 2, [][2]uint64{{0, 2}, {5, 6}}, []string{"fallback", "primary"}, []string{"Hebr", "Latn"}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			f := newServices()
			svc := f.services()
			var queried []rune
			svc.Coverage = typography.CoverageFunc(func(face contracts.FontFace, c rune) (uint32, error) {
				queried = append(queried, c)
				return f.GlyphIndex(face, c)
			})
			r, err := loadResolver(t.Context(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Fallbacks: []string{"fallback"}}, svc)
			if err != nil {
				t.Fatal(err)
			}
			out, b, err := r.ItemizeInline(t.Context(), tc.text, nil, []contracts.AtomicAnchor{{ID: 1, ByteOffset: tc.offset}})
			if err != nil || len(b) != len(tc.ranges) {
				t.Fatal(b, err)
			}
			if slices.Contains(queried, '\uFFFC') {
				t.Fatal("object sent to coverage service")
			}
			for i, v := range b {
				if v.Start != tc.ranges[i][0] || v.End != tc.ranges[i][1] || v.Path != tc.fonts[i] || v.Script != tc.scripts[i] {
					t.Fatal(b)
				}
			}
			if out.Raw().String() != tc.text {
				t.Fatal("source changed")
			}
			// The unstyled marker is a source gap between text styles, not a new
			// font-backed run. Paint cannot leak into the retained text styles.
			for run := range out.StyleRanges() {
				if run.Style != nil {
					assertColorFree(t, run.Style)
				}
			}
		})
	}
}

func TestItemizeInlinePaintAndLiteralMarker(t *testing.T) {
	f := newServices()
	r := load(t, f, fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	a, b := documentStyle(), documentStyle()
	b.Color = "blue"
	object := a
	object.Language = "invalid language!"
	text := "fi\uFFFCfi"
	spans := []style.Span{{Start: 0, End: 1, Style: a}, {Start: 1, End: 2, Style: b}, {Start: 2, End: 5, Style: object}, {Start: 5, End: 6, Style: a}, {Start: 6, End: 7, Style: b}}

	_, bindings, err := r.ItemizeInline(t.Context(), text, spans, []contracts.AtomicAnchor{{ID: 1, ByteOffset: 2}})
	want := 2

	if err != nil || len(bindings) != want {
		t.Fatal(bindings, err)
	}

	// Literal U+FFFC still follows the existing text itemizer. Only the first
	// occurrence is attached; it must not exempt the second from that path.
	_, _, err = r.ItemizeInline(t.Context(), "\uFFFC\uFFFC", nil, []contracts.AtomicAnchor{{ID: 1}})
	if err == nil || !strings.Contains(err.Error(), "missing glyph") {
		t.Fatal(err)
	}
	f.entries["primary"].Face.(*fakeFace).chars += "\uFFFC"
	_, bindings, err = r.ItemizeInline(t.Context(), "\uFFFC\uFFFC", nil, []contracts.AtomicAnchor{{ID: 1}})
	if err != nil || len(bindings) != 1 || bindings[0].Start != 3 || bindings[0].End != 6 {
		t.Fatal(bindings, err)
	}
}

func TestItemizeInlineValidationAndCancellation(t *testing.T) {
	f := newServices()
	r := load(t, f, fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	for _, anchors := range [][]contracts.AtomicAnchor{
		{{ID: 0, ByteOffset: 2}}, {{ID: 1, ByteOffset: 2}, {ID: 1, ByteOffset: 5}}, {{ID: 1, ByteOffset: 5}, {ID: 2, ByteOffset: 2}},
		{{ID: 1, ByteOffset: 2}, {ID: 2, ByteOffset: 2}}, {{ID: 1, ByteOffset: 0}}, {{ID: 1, ByteOffset: 1}}, {{ID: 1, ByteOffset: 3}}, {{ID: 1, ByteOffset: 8}}, {{ID: 1, ByteOffset: ^uint64(0)}},
	} {
		out, b, err := r.ItemizeInline(t.Context(), "é\uFFFC\uFFFC", nil, anchors)
		if !errors.Is(err, contracts.ErrIllegalArguments) || b != nil {
			t.Fatal(anchors, out, b, err)
		}
	}
	anchors := []contracts.AtomicAnchor{{ID: 1}}
	for _, text := range []string{"\xff\uFFFC", "界"} {
		if _, _, err := r.ItemizeInline(t.Context(), text, nil, anchors); !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(err)
		}
	}
	if _, _, err := r.ItemizeInline(nil, "\uFFFC", nil, anchors); !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := r.ItemizeInline(ctx, "\uFFFC", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := r.ItemizeInline(ctx, "\uFFFC", nil, anchors); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// A genuine style boundary inside a text grapheme is still invalid even
	// when some other part of the paragraph contains an attached object.
	a, b := documentStyle(), documentStyle()
	b.EmSize *= 2
	spans := []style.Span{{Start: 0, End: 1, Style: a}, {Start: 1, End: 6, Style: b}}
	if _, _, err := r.ItemizeInline(t.Context(), "a\u0301\uFFFC", spans, []contracts.AtomicAnchor{{ID: 1, ByteOffset: 3}}); err == nil || !strings.Contains(err.Error(), "grapheme") {
		t.Fatal(err)
	}
	// Cancellation triggered by the last coverage query must publish nothing.
	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	svc := f.services()
	svc.Coverage = typography.CoverageFunc(func(face contracts.FontFace, c rune) (uint32, error) { cancel(); return f.GlyphIndex(face, c) })
	r, err := loadResolver(t.Context(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP}, svc)
	if err != nil {
		t.Fatal(err)
	}
	out, bindings, err := r.ItemizeInline(ctx, "\uFFFCa", nil, anchors)
	if !errors.Is(err, context.Canceled) || bindings != nil {
		t.Fatal(bindings, err)
	}
	for run := range out.StyleRanges() {
		if run.Style != nil {
			t.Fatal("partial itemization published")
		}
	}
}

func TestItemizeInlineFeatureDomains(t *testing.T) {
	f := newServices()
	r := load(t, f, fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Fallbacks: []string{"fallback"}})
	text := "éa\uFFFCfiאב"
	features := []contracts.ShapeFeature{
		{Tag: [4]byte{'t', 'e', 's', 't'}, Start: 1, End: 6, Enabled: true},
		{Tag: [4]byte{'o', 'b', 'j', 't'}, Start: 2, End: 3, Enabled: true},
		{Tag: [4]byte{'a', 'l', 'l', ' '}, Start: -1, End: -1, Enabled: false},
		{Tag: [4]byte{'t', 'a', 'i', 'l'}, Start: 4, End: 7, Enabled: true, Value: 2},
	}

	spans := []style.Span{{Start: 0, End: uint64(len(text)), Style: documentStyle()}}
	anchors := []contracts.AtomicAnchor{{ID: 7, ByteOffset: 3}}
	out, bindings, err := r.ItemizeInline(t.Context(), text, spans, anchors)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 4 {
		t.Fatal(bindings)
	}
	defaults := r.Defaults()
	defaults.Features = slices.Clone(features)
	// Expected intersections in each emitted style span: é, a, fi, אב.
	want := [][][3]int{{{2, 0, 1}}, {{0, 0, 1}, {2, 0, 1}}, {{0, 0, 2}, {2, 0, 2}, {3, 1, 2}}, {{0, 0, 1}, {2, 0, 2}, {3, 0, 2}}}
	for i, b := range bindings {
		s, _, err := out.StyleAt(b.Start)
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.ResolveStyle(s, defaults)
		if err != nil {
			t.Fatal(err)
		}
		var expected []contracts.ShapeFeature
		for _, entry := range want[i] {
			f := features[entry[0]]
			f.Start, f.End = entry[1], entry[2]
			expected = append(expected, f)
		}
		if !reflect.DeepEqual(got.Features, expected) {
			t.Fatalf("%v span %d: %+v want %+v", "glyph-boundary", i, got.Features, expected)
		}
		if len(got.Features) > 0 {
			got.Features[0].Start = 99
		}
	}
	if !reflect.DeepEqual(features, defaults.Features) {
		t.Fatal("feature defaults mutated")
	}
	spans[0].Style.Language = "invalid!"
	anchors[0].ID = 99
	s, _, _ := out.StyleAt(6)
	if _, err := r.ResolveStyle(s, defaults); err != nil {
		t.Fatal("retained caller span slice", err)
	}
	defaults.Features = []contracts.ShapeFeature{{Start: 4, End: 2}}
	if _, err := r.ResolveStyle(s, defaults); !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal(err)
	}

}

func TestItemizeInlineNonzeroFeatureOrigin(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	a, b := documentStyle(), documentStyle()
	b.EmSize *= 2
	text := "xxfi\uFFFCfi"
	out, bindings, err := r.ItemizeInline(t.Context(), text, []style.Span{{Start: 0, End: 2, Style: a}, {Start: 2, End: 9, Style: b}}, []contracts.AtomicAnchor{{ID: 1, ByteOffset: 4}})
	if err != nil || len(bindings) != 3 {
		t.Fatal(bindings, err)
	}
	defaults := r.Defaults()
	defaults.Features = []contracts.ShapeFeature{{Start: 3, End: 5, Enabled: true}}
	for i, b := range bindings {
		s, _, _ := out.StyleAt(b.Start)
		got, err := r.ResolveStyle(s, defaults)
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			if len(got.Features) != 0 {
				t.Fatal("feature escaped original domain", got.Features)
			}
		} else if len(got.Features) != 1 || got.Features[0].Start != 0 || got.Features[0].End != 2 {
			t.Fatal("feature origin reset", got.Features)
		}
	}
}

func TestItemizeInlinePaintFeatureDomains(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP})
	a, b := documentStyle(), documentStyle()
	b.Color = "blue"
	// The first source span contains fi and an object. Itemization coalesces
	// the following paint-only change before splitting around the object;
	// The feature origin includes the object scalar before the second pair.
	spans := []style.Span{{Start: 0, End: 5, Style: a}, {Start: 5, End: 7, Style: b}}
	defaults := r.Defaults()
	defaults.Features = []contracts.ShapeFeature{{Start: 3, End: 5, Enabled: true}}

	out, _, err := r.ItemizeInline(t.Context(), "fi\uFFFCfi", spans, []contracts.AtomicAnchor{{ID: 1, ByteOffset: 2}})
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ := out.StyleAt(5)
	resolved, err := r.ResolveStyle(s, defaults)
	if err != nil {
		t.Fatal(err)
	}

	if len(resolved.Features) != 1 || resolved.Features[0].Start != 0 || resolved.Features[0].End != 2 {
		t.Fatal("coalesced domain lost", resolved.Features)
	}

}

func TestItemizeInlineTextOnlyCompatibility(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Fallbacks: []string{"fallback"}})
	for _, text := range []string{"", "fié\u00adאב"} {

		want, wb, err := r.Itemize(t.Context(), text, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, gb, err := r.ItemizeInline(t.Context(), text, nil, nil)
		if err != nil || got.Raw().String() != want.Raw().String() || !reflect.DeepEqual(gb, wb) {
			t.Fatal(gb, wb, err)
		}
		for _, b := range wb {
			ws, _, _ := want.StyleAt(b.Start)
			gs, _, _ := got.StyleAt(b.Start)
			if !ws.Equals(gs) {
				t.Fatal("text-only style identity changed")
			}
		}

	}
}

type inlineRecordingStream struct {
	contracts.StreamingShaper
	inputs []string
	bases  []uint32
	params []contracts.ShapeParams
}

func (s *inlineRecordingStream) SetInitialCluster(base uint32) error {
	s.bases = append(s.bases, base)
	return s.StreamingShaper.SetInitialCluster(base)
}
func (s *inlineRecordingStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	var text strings.Builder
	for {
		c, _, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		text.WriteRune(c)
	}
	p.Features = slices.Clone(p.Features)
	s.params = append(s.params, p)
	s.inputs = append(s.inputs, text.String())
	// Transport fixture: retain submitted parameters, then use synthetic glyphs.
	p.Font = helpers.NewSyntheticFont()
	p.Features = nil
	return s.StreamingShaper.Shape(ctx, p, strings.NewReader(text.String()), o)
}

func TestItemizeInlineKhipuCoordinatesAndFreeze(t *testing.T) {
	r := load(t, newServices(), fixtureSettings{DefaultPath: "primary", Size: 12 * dimen.BP, Fallbacks: []string{"fallback"}})
	text := "éa\uFFFCfiאב"
	anchors := []contracts.AtomicAnchor{{ID: 7, ByteOffset: 3}}
	out, _, err := r.ItemizeInline(t.Context(), text, nil, anchors)
	if err != nil {
		t.Fatal(err)
	}
	para, err := styled.ParagraphFromText(&out, 0, out.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	defaults := r.Defaults()
	defaults.Features = []contracts.ShapeFeature{{Start: 1, End: 6, Enabled: true}}
	var stream *inlineRecordingStream
	owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{Resolver: r, Defaults: defaults, NewShaper: func() (contracts.StreamingShaper, error) {
		s, err := helpers.NewSyntheticStreamingShaper(0)
		stream = &inlineRecordingStream{StreamingShaper: s}
		return stream, err
	}}})
	p, err := owner.PrepareParagraph(t.Context(), khipu.ParagraphInput{Paragraph: para, Atomics: anchors}, khipu.PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stream.inputs, []string{"é", "a", "fi", "אב"}) || !slices.Equal(stream.bases, []uint32{0, 1, 3, 5}) {
		t.Fatal(stream.inputs, stream.bases)
	}
	for i, wantEnd := range []int{0, 1, 2, 1} {
		features := stream.params[i].Features
		if wantEnd == 0 {
			if len(features) != 0 {
				t.Fatal("feature leaked before its source range", features)
			}
		} else if len(features) != 1 || features[0].Start != 0 || features[0].End != wantEnd {
			t.Fatal("shaper received wrong feature window", i, features)
		}
	}
	for _, engine := range []string{"firstfit", "knuthplass"} {
		k, err := owner.BindParagraph(t.Context(), p, []khipu.AtomicAdvance{{ID: 7, Advance: 0}})
		if err != nil {
			t.Fatal(err)
		}
		layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.BP))
		var breaks []int
		if engine == "firstfit" {
			breaks, err = firstfit.BreakParagraph(k, layout, nil)
		} else {
			breaks, err = knuthplass.BreakParagraph(k, layout, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		f, err := owner.Freeze(t.Context(), k, breaks, layout)
		if err != nil {
			t.Fatal(err)
		}
		atoms := 0
		var clusters []uint32
		for i := 0; i < f.KnotCount(); i++ {
			if id, ok := f.Atomic(i); ok {
				atoms++
				knot, _ := f.Knot(i)
				if id != 7 || knot.Source.Bytes != (khipu.ByteRange{Start: 3, End: 6}) || knot.Source.ScalarStart != 2 {
					t.Fatal(knot, id)
				}
			}
			if tx, ok := f.CopyText(i); ok {
				for _, g := range tx.Glyphs {
					clusters = append(clusters, g.Cluster)
				}
			}
		}
		slices.Sort(clusters)
		if atoms != 1 || !slices.Equal(clusters, []uint32{0, 1, 3, 4, 5, 6}) {
			t.Fatal("lost original coordinates", atoms, clusters)
		}
	}
}
