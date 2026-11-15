package paragraph_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/paragraph"
)

func cssParams(b paragraph.Breaker, height paragraph.LineHeight) paragraph.Params {
	p := params(b, 20*dimen.BP)
	p.Assembly = paragraph.Options{Policy: paragraph.CSSLineHeight}
	p.LineLayout = &paragraph.LineLayout{Default: height}
	return p
}
func factor(v float64) paragraph.LineHeight {
	return paragraph.LineHeight{Kind: paragraph.FactorHeight, Factor: v}
}
func length(v dimen.DU) paragraph.LineHeight {
	return paragraph.LineHeight{Kind: paragraph.LengthHeight, Length: v}
}

func TestLineHeightExactGeometryAndInheritance(t *testing.T) {
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		// Each line is one 10bp word. Ascender 8, descender 2, font gap 0.
		for _, tc := range []struct {
			h                paragraph.LineHeight
			height, baseline float64
		}{
			{paragraph.LineHeight{Kind: paragraph.NormalHeight}, 10, 8},
			{factor(1.5), 15, 10.5}, {length(20 * dimen.BP), 20, 13},
			{factor(.5), 5, 5.5}, {factor(0), 0, 3},
		} {
			out, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("aa aa")}, cssParams(b, tc.h), synthetic())
			if err != nil {
				t.Fatal(err)
			}
			if out.Frozen.LineCount() != 2 {
				t.Fatalf("fixture: %d lines", out.Frozen.LineCount())
			}
			if got := out.VList.Box(); got.Extent != 2*tc.height || got.Baseline != tc.baseline {
				t.Fatalf("%s %+v: box %+v", b, tc.h, got)
			}
			for i, l := range out.VList.Lines() {
				if l.Top != float64(i)*tc.height || l.Baseline != float64(i)*tc.height+tc.baseline || l.Height != tc.height || l.Gap != 0 {
					t.Fatalf("line %+v", l)
				}
			}
			if len(out.VList.Materials()) != 2 {
				t.Fatal("added compatibility kern")
			}
			for _, m := range out.VList.Materials() {
				if m.Kind != paragraph.LineBox || m.Extent != tc.height {
					t.Fatal("material", m)
				}
			}
		}
		// A factor is inherited as a factor; a computed percentage/length stays fixed.
		// Two font sizes share a line. The 10bp strut remains present for both.
		text := styled.TextFromString("ab")
		text, err := text.Style(sizeStyle{20 * dimen.BP, 17}, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		cfg := synthetic()
		cfg.Streaming.Resolver = resolver{}
		for _, tc := range []struct {
			h                paragraph.LineHeight
			baseline, height float64
		}{
			{factor(1.5), 21, 30},             // 20bp run receives 30bp line-height
			{length(15 * dimen.BP), 13.5, 18}, // inherited 150% of parent's 10bp = 15bp
			{factor(.5), 11, 10.5},            // negative leading can extend a baseline outside its own box
		} {
			p := cssParams(b, tc.h)
			p.Constraints.Width = 100 * dimen.BP
			out, err := paragraph.Run(t.Context(), paragraph.Input{Text: text}, p, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if out.VList.Box().Baseline != tc.baseline || out.VList.Box().Extent != tc.height {
				t.Fatalf("inheritance %+v got %+v", tc.h, out.VList.Box())
			}
		}
	}
}

func TestLineHeightRangesDoNotChangeHorizontalOutput(t *testing.T) {
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		input := paragraph.Input{Text: styled.TextFromString("abcd ef")}
		cfg := synthetic()
		p := cssParams(b, factor(1))
		p.Constraints.Width = 100 * dimen.BP
		before, err := paragraph.Run(t.Context(), input, p, cfg)
		if err != nil {
			t.Fatal(err)
		}
		// Interior range of one shaped word; no style or shaping boundary introduced.
		p.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 1, End: 2, Height: factor(3)}}
		out, err := paragraph.Run(t.Context(), input, p, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if out.VList.Box().Extent != 30 || out.VList.Box().Baseline != 18 {
			t.Fatal("range contribution", out.VList.Box())
		}
		if out.Frozen.KnotCount() != before.Frozen.KnotCount() || !reflect.DeepEqual(out.BreakPositions, before.BreakPositions) {
			t.Fatal("horizontal structure changed")
		}
		for k := 0; k < out.Frozen.KnotCount(); k++ {
			a, _ := out.Frozen.CopyText(k)
			c, _ := before.Frozen.CopyText(k)
			if !reflect.DeepEqual(a.Glyphs, c.Glyphs) || !reflect.DeepEqual(a.Clusters, c.Clusters) {
				t.Fatal("reshaped at layout boundary")
			}
		}
		p.LineLayout.Runs[0].Height = factor(100)
		if out.VList.Box().Extent != 30 {
			t.Fatal("result aliases layout input")
		}
		// Run override below default cannot reduce the paragraph strut.
		p.LineLayout.Default = factor(4)
		p.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 0, End: 7, Height: factor(1)}}
		out, err = paragraph.Run(t.Context(), input, p, cfg)
		if err != nil || out.VList.Box().Extent != 40 {
			t.Fatal("strut", out, err)
		}
	}
}

func TestLineHeightSpaceAndDiscretionaryContributions(t *testing.T) {
	cfg := synthetic()
	cfg.Streaming.Resolver = resolver{}
	text := styled.TextFromString("a b")
	text, err := text.Style(sizeStyle{30 * dimen.BP, 17}, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		p := cssParams(b, factor(1))
		p.Constraints.Width = 100 * dimen.BP
		out, err := paragraph.Run(t.Context(), paragraph.Input{Text: text}, p, cfg)
		if err != nil || out.VList.Box().Extent != 30 {
			t.Fatal("surviving styled space", out, err)
		}
		// A discarded trailing space must not enlarge a line.
		trailing := styled.TextFromString("a ")
		trailing, err = trailing.Style(sizeStyle{30 * dimen.BP, 17}, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		out, err = paragraph.Run(t.Context(), paragraph.Input{Text: trailing}, p, cfg)
		if err != nil || out.VList.Box().Extent != 10 {
			t.Fatal("trimmed space", out, err)
		}
		cfg := synthetic()
		cfg.Hyphenator = softHyphens{}
		p = cssParams(b, factor(1))
		p.Constraints.Width = 15 * dimen.BP
		p.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 2, End: 4, Height: factor(3)}}
		out, err = paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("ab\u00adcd")}, p, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.VList.Lines()) != 2 || out.VList.Lines()[0].Height != 30 || out.VList.Lines()[1].Height != 10 {
			t.Fatal("replacement correspondence", out.VList.Lines())
		}
	}
}

func TestLineHeightValidationAndEmpty(t *testing.T) {
	in := paragraph.Input{Text: styled.TextFromString("אב")}
	cfg := synthetic()
	for _, change := range []func(*paragraph.Params){
		func(p *paragraph.Params) { p.LineLayout = nil },
		func(p *paragraph.Params) { p.Assembly.DefaultEmSize = dimen.BP },
		func(p *paragraph.Params) { p.LineLayout.Default = factor(-1) },
		func(p *paragraph.Params) { p.LineLayout.Default = factor(math.Inf(1)) },
		func(p *paragraph.Params) { p.LineLayout.Default = factor(math.NaN()) },
		func(p *paragraph.Params) { p.LineLayout.Default = length(-1) },
		func(p *paragraph.Params) {
			p.LineLayout.Default = paragraph.LineHeight{Kind: paragraph.NormalHeight, Factor: 1}
		},
		func(p *paragraph.Params) {
			p.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 1, End: 2, Height: factor(1)}}
		},
		func(p *paragraph.Params) {
			p.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 0, End: 2, Height: factor(1)}, {Start: 0, End: 4, Height: factor(2)}}
		},
		func(p *paragraph.Params) {
			p.LineLayout.Runs = []paragraph.LineHeightSpan{{Start: 0, End: 5, Height: factor(1)}}
		},
	} {
		p := cssParams(paragraph.FirstFit, factor(1))
		change(&p)
		out, err := paragraph.Run(t.Context(), in, p, cfg)
		if out != nil || !errors.Is(err, paragraph.ErrInvalidInput) {
			t.Fatalf("accepted invalid policy %+v: %v", p, err)
		}
	}
	p := cssParams(paragraph.FirstFit, factor(1))
	if out, err := paragraph.Run(t.Context(), in, p, khipu.Config{}); out != nil || err == nil {
		t.Fatal("missing explicit services")
	}
	p.LineLayout.Default = factor(1e100)
	if out, err := paragraph.Run(t.Context(), in, p, cfg); out != nil || err == nil {
		t.Fatal("overflow")
	}
	p = cssParams(paragraph.FirstFit, factor(1))
	out, err := paragraph.Run(t.Context(), paragraph.Input{}, p, cfg)
	if err != nil || out.VList.Box().Extent != 10 || !out.VList.Box().HasBaseline {
		t.Fatal("empty", out, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if out, err := paragraph.Run(ctx, in, p, cfg); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation")
	}
}

// Metrics differ from the synthetic shaper's metrics, without coupling P to a
// concrete font implementation. Normal consumes the font gap, not a 0.2em floor.
type gapFont struct {
	contracts.FontFace
	gap int32
}

func (f gapFont) Metrics() (contracts.FontMetrics, error) {
	m, e := f.FontFace.Metrics()
	m.LineGap = f.gap
	return m, e
}
func TestLineHeightNormalFontGap(t *testing.T) {
	for _, tc := range []struct {
		gap              int32
		height, baseline float64
	}{{400, 14, 10}, {-400, 10, 8}} {
		cfg := synthetic()
		cfg.Streaming.Defaults.Font = gapFont{cfg.Streaming.Defaults.Font, tc.gap}
		cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
			s, e := helpers.NewSyntheticStreamingShaper(8)
			return metricStream{s}, e
		}
		// Empty source has no lines; use a blank line with only strut metrics.
		p := cssParams(paragraph.FirstFit, paragraph.LineHeight{Kind: paragraph.NormalHeight})
		out, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("\n")}, p, cfg)
		if err != nil || out.VList.Lines()[0].Height != tc.height || out.VList.Lines()[0].Baseline != tc.baseline {
			t.Fatal("normal gap", out, err)
		}
	}
}

type metricStream struct {
	*helpers.SyntheticStreamingShaper
}

func (s metricStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	p.Font = helpers.NewSyntheticFont()
	return s.SyntheticStreamingShaper.Shape(ctx, p, r, o)
}

func TestLineHeightAccumulatedOverflowAndCancellation(t *testing.T) {
	cfg := synthetic()
	p := cssParams(paragraph.FirstFit, length(dimen.Fil/5*2))
	p.Constraints.Width = 5 * dimen.BP
	out, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("a a a")}, p, cfg)
	var e *paragraph.Error
	if out != nil || !errors.As(err, &e) || e.Stage != paragraph.StageAssemble {
		t.Fatal("accumulated height overflow", out, err)
	}
	// Cancellation while measuring the strut must not publish partial placements.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cfg.Streaming.Defaults.Font = cancelFont{FontFace: cfg.Streaming.Defaults.Font, cancel: cancel}
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
		s, e := helpers.NewSyntheticStreamingShaper(8)
		return metricStream{s}, e
	}
	p = cssParams(paragraph.FirstFit, factor(1))
	out, err = paragraph.Run(ctx, paragraph.Input{Text: styled.TextFromString("aa")}, p, cfg)
	if out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled result", out, err)
	}
}

type cancelFont struct {
	contracts.FontFace
	cancel func()
}

func (f cancelFont) Metrics() (contracts.FontMetrics, error) { f.cancel(); return f.FontFace.Metrics() }

func TestLineHeightDifferentFontExtents(t *testing.T) {
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		cfg := synthetic()
		cfg.Streaming.Resolver = fallbackMetricResolver{asymmetricFont{cfg.Streaming.Defaults.Font}}
		cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
			s, e := helpers.NewSyntheticStreamingShaper(8)
			return metricStream{s}, e
		}
		text := styled.TextFromString("ab")
		text, err := text.Style(sizeStyle{10 * dimen.BP, 17}, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		p := cssParams(b, factor(1))
		p.Constraints.Width = 100 * dimen.BP
		out, err := paragraph.Run(t.Context(), paragraph.Input{Text: text}, p, cfg)
		if err != nil {
			t.Fatal(err)
		}
		// Default/strut is 8 above, 2 below; second face is 3 above, 7 below.
		// The line takes maxima independently, not just the largest font/line-height.
		if got := out.VList.Box(); got.Baseline != 8 || got.Extent != 15 {
			t.Fatal("fallback metric union", got)
		}
	}
}

type asymmetricFont struct{ contracts.FontFace }

func (f asymmetricFont) Metrics() (contracts.FontMetrics, error) {
	m, e := f.FontFace.Metrics()
	m.Ascent = 300
	m.Descent = -700
	return m, e
}

type fallbackMetricResolver struct{ face contracts.FontFace }

func (r fallbackMetricResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if s != nil {
		d.Font = r.face
	}
	return d, nil
}

// A taller run must receive the default line height in every uncovered portion
// of its source range, even when overrides inside the same shaped word shrink it.
func TestLineHeightOverrideGapsKeepDefaultContribution(t *testing.T) {
	text := styled.TextFromString("abcd")
	text, err := text.Style(sizeStyle{20 * dimen.BP, 17}, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		for _, tc := range []struct {
			name             string
			ranges           [][2]uint64
			height, baseline float64
		}{
			{"leading", [][2]uint64{{1, 4}}, 40, 26},
			{"interior", [][2]uint64{{0, 1}, {2, 4}}, 40, 26},
			{"trailing", [][2]uint64{{0, 3}}, 40, 26},
			{"fully covered", [][2]uint64{{0, 2}, {2, 4}}, 20, 13},
		} {
			t.Run(string(b)+"/"+tc.name, func(t *testing.T) {
				cfg := synthetic()
				cfg.Streaming.Resolver = resolver{}
				p := cssParams(b, factor(2))
				p.Constraints.Width = 100 * dimen.BP
				for _, r := range tc.ranges {
					p.LineLayout.Runs = append(p.LineLayout.Runs, paragraph.LineHeightSpan{Start: r[0], End: r[1], Height: factor(0)})
				}
				out, err := paragraph.Run(t.Context(), paragraph.Input{Text: text}, p, cfg)
				if err != nil {
					t.Fatal(err)
				}
				lines := out.VList.Lines()
				if len(lines) != 1 || lines[0].Height != tc.height || lines[0].Baseline != tc.baseline || lines[0].Gap != 0 {
					t.Fatalf("line geometry = %+v; want height %g, baseline %g, no gap", lines, tc.height, tc.baseline)
				}
				if box := out.VList.Box(); box.Extent != tc.height || box.Baseline != tc.baseline || !box.HasBaseline {
					t.Fatalf("box geometry = %+v", box)
				}
				if material := out.VList.Materials(); len(material) != 1 || material[0].Kind != paragraph.LineBox || material[0].Extent != tc.height {
					t.Fatalf("vertical material = %+v", material)
				}
			})
		}
	}
}
