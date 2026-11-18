package paragraph_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/typography/glyphing/paragraph"
)

// Only the latest sample is retained, never a growing collection of preparations.
var p6Sink any

func p6Fit(t testing.TB, c *paragraph.Continuation, f paragraph.Frame) *paragraph.FlowResult {
	t.Helper()
	r, e := c.Fit(t.Context(), f)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func p6Pair(t testing.TB, f *p6Fixture, height float64, narrow bool) *paragraph.FlowResult {
	t.Helper()
	a := p6Fit(t, f.seed, paragraph.Frame{Width: f.width, Height: height})
	if a.Continuation == nil {
		t.Fatal("benchmark cut must leave a suffix")
	}
	w := f.width
	if narrow {
		w = w * 3 / 4
	}
	b := p6Fit(t, a.Continuation, paragraph.Frame{Width: w, Height: 30000})
	if b.Continuation != nil || b.Status != paragraph.FitComplete || len(b.Overfull) != 0 || b.Reused == narrow {
		t.Fatal("invalid benchmark result")
	}
	return b
}
func p6Services(b *testing.B, before, after [7]int64, prepare, synthetic, equal bool) {
	for i := range before {
		delta := after[i] - before[i]
		if !prepare && ((i >= 1 && i <= 3) || equal) && delta != 0 {
			b.Fatalf("unexpected service %d: %d", i, delta)
		}
		if synthetic || (i >= 1 && i <= 3) {
			b.ReportMetric(float64(delta)/float64(b.N), []string{"resolutions/op", "factories/op", "shapes/op", "acceptance/op", "metrics/op", "scales/op", "conversions/op"}[i])
		}
	}
}
func BenchmarkP6Prepare(b *testing.B) {
	for _, backend := range []string{"synthetic", "gentium", "hebrew"} {
		for _, words := range p6Sizes(backend) {
			b.Run(p6Name(backend, words), func(b *testing.B) {
				input, cfg, calls := p6Input(b, backend, words, false)
				p6Sink = p6Prepare(b, input, cfg) // warm stable face caches, not paragraph state
				before := calls.snapshot()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					p6Sink = p6Prepare(b, input, cfg)
				}
				b.StopTimer()
				p6Services(b, before, calls.snapshot(), true, backend == "synthetic", false)
			})
		}
	}
}
func BenchmarkP6Frames(b *testing.B) {
	for _, backend := range []string{"synthetic", "gentium", "hebrew"} {
		for _, words := range p6Sizes(backend) {
			b.Run(p6Name(backend, words), func(b *testing.B) {
				f := newP6Fixture(b, backend, words, false)
				for _, percent := range []int{10, 50, 90} {
					cut, height := f.cut(percent)
					for _, op := range []string{"equal", "continue", "relayout-equal", "relayout-narrow"} {
						b.Run(fmt.Sprintf("P%d/%s", percent, op), func(b *testing.B) {
							// Admit the route before timing; failures and partial output are not samples.
							p6Pair(b, f, height, op == "continue" || op == "relayout-narrow")
							before := f.calls.snapshot()
							if op == "equal" {
								f.calls.sealed.Store(true)
								defer f.calls.sealed.Store(false)
							}
							b.ReportAllocs()
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								if op == "equal" || op == "continue" {
									p6Sink = p6Pair(b, f, height, op == "continue")
									continue
								}
								req := f.request
								if op == "relayout-narrow" {
									req.Params.Constraints.Width = f.width * 3 / 4
								}
								r, e := f.prepared.Layout(b.Context(), req)
								if e != nil {
									b.Fatal(e)
								}
								frag, e := paragraph.Fragment(b.Context(), r.VList)
								if e != nil {
									b.Fatal(e)
								}
								key := paragraph.FragmentReuseKey{Width: req.Params.Constraints.Width, Source: 1, Styles: 1, Fonts: 1, LineHeight: 1, Atomics: 1, Policy: 1}
								a, e := paragraph.FitFragment(b.Context(), frag, key, key, height)
								if e != nil {
									b.Fatal(e)
								}
								tail, e := paragraph.FitFragment(b.Context(), a.Remainder, key, key, 30000)
								if e != nil || tail.Status != paragraph.FitComplete || len(tail.Overfull) != 0 {
									b.Fatal("relayout", e)
								}
								p6Sink = tail
							}
							b.StopTimer()
							// The height cut was chosen from the original A-width layout.
							// Relayout comparators may have a different total line count.
							b.ReportMetric(float64(cut), "original-cut-lines/op")
							b.ReportMetric(100*float64(cut)/float64(len(f.seed.Remainder().Lines())), "original-cut-percent")
							p6Services(b, before, f.calls.snapshot(), false, backend == "synthetic", op == "equal")
						})
					}
				}
			})
		}
	}
}

// Fixed suffix: 100 synthetic aa words remain while the accepted prefix grows.
// These component calls do not include the public Fit validation/rebasing cost;
// that complete cost is measured above through production entry points.
func BenchmarkP6FixedSuffix(b *testing.B) {
	for _, words := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("N%d", words), func(b *testing.B) {
			f := newP6Fixture(b, "synthetic", words, true)
			cut := (words - 100) / 10
			height := 0.0
			if cut > 0 {
				l := f.seed.Remainder().Lines()[cut-1]
				height = l.Top + l.Height
			}
			a := p6Fit(b, f.seed, paragraph.Frame{Width: f.width, Height: height})
			if a.Continuation == nil || f.seed.Remainder().Frozen().SourceLen()-a.Remainder.Lines()[0].Horizontal.Source.Start != 299 {
				b.Fatal("fixed suffix not 100 words")
			}
			for _, op := range []string{"checkpoint", "phases"} {
				b.Run(op, func(b *testing.B) {
					var sum paragraph.P6PhaseSample
					before := f.calls.snapshot()
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if op == "checkpoint" {
							r, e := paragraph.P6Checkpoint(b.Context(), a.Continuation)
							if e != nil {
								b.Fatal(e)
							}
							p6Sink = r
						} else {
							r, m, e := paragraph.P6Phases(b.Context(), a.Continuation, f.width*3/4)
							if e != nil {
								b.Fatal(e)
							}
							p6Sink = r
							sum.Checkpoint += m.Checkpoint
							sum.Bind += m.Bind
							sum.Break += m.Break
							sum.Freeze += m.Freeze
							sum.Assembly += m.Assembly
							sum.BreakWidthQueries = m.BreakWidthQueries
							sum.PrefixWidthQueries = m.PrefixWidthQueries
							sum.BoundKnots = m.BoundKnots
							sum.PrefixKnots = m.PrefixKnots
							sum.FrozenKnots = m.FrozenKnots
							sum.AssembledLines = m.AssembledLines
						}
					}
					b.StopTimer()
					if op == "phases" {
						for k, v := range map[string]float64{"checkpoint-ns/op": float64(sum.Checkpoint), "bind-ns/op": float64(sum.Bind), "break-ns/op": float64(sum.Break), "freeze-ns/op": float64(sum.Freeze), "assembly-ns/op": float64(sum.Assembly)} {
							b.ReportMetric(v/float64(b.N), k)
						}
						for k, v := range map[string]int{"break-width-queries/op": sum.BreakWidthQueries, "prefix-width-queries/op": sum.PrefixWidthQueries, "bound-knots/op": sum.BoundKnots, "prefix-knots/op": sum.PrefixKnots, "frozen-knots/op": sum.FrozenKnots, "assembled-lines/op": sum.AssembledLines} {
							b.ReportMetric(float64(v), k)
						}
					}
					b.ReportMetric(float64(words-100), "prefix-words/op")
					b.ReportMetric(100, "suffix-words/op")
					p6Services(b, before, f.calls.snapshot(), false, true, op == "checkpoint")
				})
			}
		})
	}
}

// Separate component probes must reproduce the actual production continuation.
func TestP6MeasurementAdmission(t *testing.T) {
	for _, backend := range []string{"synthetic", "gentium", "hebrew"} {
		t.Run(backend, func(t *testing.T) {
			f := newP6Fixture(t, backend, 100, false)
			for _, percent := range []int{10, 50, 90} {
				cut, h := f.cut(percent)
				a := p6Fit(t, f.seed, paragraph.Frame{Width: f.width, Height: h})
				prefix := a.Placed.Lines()
				if len(prefix) != cut {
					t.Fatal("wrong prefix")
				}
				before := f.calls.snapshot()
				b := p6Pair(t, f, h, true)
				probe, m, e := paragraph.P6Phases(t.Context(), a.Continuation, f.width*3/4)
				if e != nil || m.BoundKnots <= 0 || m.FrozenKnots <= 0 || m.AssembledLines <= cut || m.BreakWidthQueries == 0 || m.PrefixWidthQueries != 0 {
					t.Fatal(m, e)
				}
				if probe.Box() != b.Placed.Box() || !reflect.DeepEqual(probe.Lines(), b.Placed.Lines()) || !reflect.DeepEqual(probe.Materials(), b.Placed.Materials()) {
					t.Fatal("component probe differs from production")
				}
				noFlowShaping(t, before, f.calls.snapshot())
				f.calls.sealed.Store(true)
				same := p6Pair(t, f, h, false)
				f.calls.sealed.Store(false)
				if same.Placed.Frozen() != a.Placed.Frozen() || !reflect.DeepEqual(prefix, a.Placed.Lines()) {
					t.Fatal("ownership/prefix changed")
				}
				end := a.Placed.Lines()[cut-1].Horizontal.Source.End
				for _, line := range b.Placed.Lines() {
					if line.Horizontal.Source.Start != end {
						t.Fatal("source gap")
					}
					end = line.Horizontal.Source.End
				}
				if end != f.seed.Remainder().Frozen().SourceLen() {
					t.Fatal("lost tail")
				}
			}
		})
	}
}

// Initial layout is reported separately from the post-layout frame operations.
func BenchmarkP6Initial(b *testing.B) {
	for _, backend := range []string{"synthetic", "gentium", "hebrew"} {
		for _, words := range p6Sizes(backend) {
			b.Run(p6Name(backend, words), func(b *testing.B) {
				f := newP6Fixture(b, backend, words, false)
				before := f.calls.snapshot()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					r, e := f.prepared.StartFlow(b.Context(), f.request)
					if e != nil {
						b.Fatal(e)
					}
					p6Sink = r
				}
				b.StopTimer()
				p6Services(b, before, f.calls.snapshot(), false, backend == "synthetic", false)
			})
		}
	}
}
