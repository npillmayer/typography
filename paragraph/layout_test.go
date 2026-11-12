package paragraph_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography/paragraph"
	"github.com/npillmayer/uax/bidi"
)

func params(b paragraph.Breaker, w dimen.DU) paragraph.Params {
	return paragraph.Params{Constraints: paragraph.Constraints{Width: w}, Breaker: b, Assembly: paragraph.Options{Policy: paragraph.ProofCompatibility, DefaultEmSize: 10 * dimen.BP}}
}
func synthetic() khipu.Config {
	return khipu.Config{Streaming: &khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(8) },
		Resolver:  helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP, Spacing: contracts.SpacingElastic},
	}}
}

type sizeStyle struct {
	size     dimen.DU
	identity int
}

func (s sizeStyle) Equals(v styled.Style) bool { o, ok := v.(sizeStyle); return ok && s == o }
func (s sizeStyle) String() string             { return "opaque typography" }

type resolver struct{}

func (resolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	if s != nil {
		v, ok := s.(sizeStyle)
		if !ok || v.identity != 17 {
			return d, errors.New("style reconstructed")
		}
		d.EmSize = v.size
	}
	return d, nil
}

func TestPreparedStylesAndNonDOMMarkup(t *testing.T) {
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		text := styled.TextFromString("ab אב 12")
		text, err := text.Style(sizeStyle{20 * dimen.BP, 17}, 0, 2)
		if err != nil {
			t.Fatal(err)
		}
		events := map[uint64]int{3: bidi.MarkupRLI, text.Raw().Len(): bidi.MarkupPDI}
		calls := 0
		in := paragraph.Input{Text: text, Direction: bidi.LeftToRight, Markup: func(at uint64) int { calls++; return events[at] }}
		cfg := synthetic()
		cfg.Streaming.Resolver = resolver{}
		out, err := paragraph.Run(t.Context(), in, params(b, 100*dimen.BP), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if calls == 0 || len(out.BidiRuns) < 2 || out.EncodedKnots == 0 || len(out.BreakPositions) != out.BreakCount {
			t.Fatal("missing preparation/diagnostics")
		}
		if out.VList.Frozen() != out.Frozen || out.Frozen.SourceLen() != uint64(len("ab אב 12")) || out.VList.Lines()[0].Baseline != 16 {
			t.Fatal("styles/markup lost")
		}
		reader, err := out.Frozen.SourceReader(khipu.ByteRange{Start: 0, End: out.Frozen.SourceLen()})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(reader)
		if err != nil || string(raw) != "ab אב 12" {
			t.Fatal("zero-byte markup changed source", string(raw), err)
		}
		before := out.VList.Lines()
		events[3] = bidi.MarkupLRI
		in.Text = styled.TextFromString("changed")
		out.BidiRuns = nil
		out.BreakPositions[0] = 0
		if !reflect.DeepEqual(before, out.VList.Lines()) {
			t.Fatal("result aliases input/diagnostics")
		}
	}
}

func TestFreshOwnersAndRetainedResults(t *testing.T) {
	in := paragraph.Input{Text: styled.TextFromString("aa aa aa aa aa"), Direction: bidi.RightToLeft}
	cfg := synthetic()
	var created atomic.Int32
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) {
		created.Add(1)
		return helpers.NewSyntheticStreamingShaper(8)
	}
	first, err := paragraph.Run(t.Context(), in, params(paragraph.FirstFit, 25*dimen.BP), cfg)
	if err != nil {
		t.Fatal(err)
	}
	lines := first.VList.Lines()
	var glyphs []khipu.FrozenText
	for k := 0; k < first.Frozen.KnotCount(); k++ {
		tx, _ := first.Frozen.CopyText(k)
		glyphs = append(glyphs, tx)
	}
	var wg sync.WaitGroup
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		wg.Go(func() {
			out, e := paragraph.Run(t.Context(), in, params(b, 45*dimen.BP), cfg)
			if e != nil || out.Frozen == first.Frozen {
				t.Error("fresh invocation failed", e)
			}
		})
	}
	wg.Wait()
	if created.Load() < 3 || !reflect.DeepEqual(lines, first.VList.Lines()) {
		t.Fatal("owner/frozen state shared")
	}
	for k, want := range glyphs {
		got, _ := first.Frozen.CopyText(k)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("later breaker changed frozen glyphs")
		}
	}
}

type softHyphens struct{}

func (softHyphens) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	if word == "\u00ad" {
		return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
	}
	return nil, nil
}
func TestInjectedHyphenationAndEmptyInput(t *testing.T) {
	for _, b := range []paragraph.Breaker{paragraph.FirstFit, paragraph.KnuthPlass} {
		cfg := synthetic()
		cfg.Hyphenator = softHyphens{}
		out, err := paragraph.Run(t.Context(), paragraph.Input{Text: styled.TextFromString("ab\u00adcd")}, params(b, 15*dimen.BP), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if out.Frozen.LineCount() != 2 || out.VList.Box().Extent != 24 {
			t.Fatal("hyphenation not composed", out.VList.Box())
		}
		replacement := false
		for k := 0; k < out.Frozen.KnotCount(); k++ {
			tx, ok := out.Frozen.CopyText(k)
			replacement = replacement || (ok && tx.IsReplacement)
		}
		if !replacement {
			t.Fatal("missing replacement provenance")
		}
		empty, err := paragraph.Run(t.Context(), paragraph.Input{}, params(b, 30*dimen.BP), khipu.Config{})
		if err != nil || empty.VList == nil || empty.Frozen.SourceLen() != 0 {
			t.Fatal("empty paragraph", err)
		}
	}
}

func TestRunValidationCancellationAndServiceErrors(t *testing.T) {
	input := paragraph.Input{Text: styled.TextFromString("aa")}
	p := params(paragraph.FirstFit, 30*dimen.BP)
	check := func(ctx context.Context, in paragraph.Input, p paragraph.Params, cfg khipu.Config, stage paragraph.Stage, cause error) {
		t.Helper()
		out, err := paragraph.Run(ctx, in, p, cfg)
		var e *paragraph.Error
		if out != nil || !errors.As(err, &e) || e.Stage != stage || !errors.Is(err, cause) {
			t.Fatalf("result=%v error=%v want stage=%s cause=%v", out, err, stage, cause)
		}
	}
	check(nil, input, p, synthetic(), paragraph.StageInput, paragraph.ErrInvalidInput)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	check(ctx, input, p, synthetic(), paragraph.StageInput, context.Canceled)
	for _, change := range []func(*paragraph.Input, *paragraph.Params){
		func(_ *paragraph.Input, p *paragraph.Params) { p.Constraints.Width = 0 },
		func(_ *paragraph.Input, p *paragraph.Params) { p.Breaker = "unknown" },
		func(_ *paragraph.Input, p *paragraph.Params) { p.Assembly = paragraph.Options{} },
		func(i *paragraph.Input, _ *paragraph.Params) { i.Direction = bidi.Direction(99) },
	} {
		i, q := input, p
		change(&i, &q)
		check(t.Context(), i, q, synthetic(), paragraph.StageInput, paragraph.ErrInvalidInput)
	}
	sentinel := errors.New("shaper failure")
	cfg := synthetic()
	cfg.Streaming.NewShaper = func() (contracts.StreamingShaper, error) { return nil, sentinel }
	check(t.Context(), input, p, cfg, paragraph.StageEncode, sentinel)
	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	input.Markup = func(uint64) int { cancel(); return 0 }
	check(ctx, input, p, synthetic(), paragraph.StageParagraph, context.Canceled)
}
