# Archived flow experiment

Retired from `internal/flowexperiment` on 2026-11-18 after production stages
P1–P6 superseded E1–E3. This directory preserves the experiment's evidence and
supporting sources; it is not an active Go package or a supported benchmark
harness. Current APIs, paragraph lifecycle and bounded release claims are in
[FLOW.md](../../FLOW.md). Use the [production cost harness](../../flow-cost/README.md)
and [P6 report](../../FLOW_COSTS.md) for current measurements.

## Historical evidence

- [Experiment specification](EXPERIMENT.md)
- [E1 equal-width results](RESULTS.md)
- [E2 continuation results and upstream bidi limitations](E2_RESULTS.md)
- [E3 measurements and decision](E3_RESULTS.md)
- [Original E3 raw samples and environment](testdata/e3/results/2026-11-18/README.md)

Historical commands, timings, paths, source hashes and release statements describe
the experiment at the time of recording. They are not current validation or
production guarantees. The upstream bidi fix remains deferred; the archived
[admission probe](testdata/e2/admission_probe_test.go.txt) is a reproducer of a
known limitation, not a passing production acceptance case.

## Source preservation and reproduction

[MANIFEST.json](MANIFEST.json) maps all 38 original files to their archive paths,
with original and archived SHA-256 hashes and the source commit. Raw measurement
logs, JSON metadata, source code and scripts are byte-for-byte preserved. Only
relative Markdown link targets were adjusted. The original package README is
now `EXPERIMENT.md`.

Go files and Python helpers have `.txt` suffixes so ordinary package discovery
and verification do not execute this retired experiment. Existing overlay
templates retain their `.go.txt` names. To reproduce historical work, restore
the original paths and filenames from the manifest in a separate checkout with
the recorded dependency revisions, then follow the historical instructions.
The overlay scripts assume their original location and historical source shape;
they are not maintained against current production code. Do not regenerate or
overwrite archived evidence during routine verification.

## Regression coverage retained in owning packages

Most experiment regressions had already moved into production tests. Retirement
also preserves the delegated-helper audit mutation explicitly, checking both
`Fragment` and the `FitFragment` call graph.

| Experimental coverage | Active regression coverage |
| --- | --- |
| Equal-width cuts, exact fit, full heights, signed/zero geometry, atomics and invalid requests | [fragment_fit_split_test.go](../../../glyphing/paragraph/fragment_fit_split_test.go), [fragment_fit_guard_test.go](../../../glyphing/paragraph/fragment_fit_guard_test.go) |
| Detached ownership, retained lifetime, cancellation and late geometry failures | [fragment_fit_ownership_test.go](../../../glyphing/paragraph/fragment_fit_ownership_test.go) |
| Zero service/breaker calls, aliases, callbacks and delegated helpers | [fragment_fit_boundary_test.go](../../../glyphing/paragraph/fragment_fit_boundary_test.go), [flow_boundary_test.go](../../../glyphing/paragraph/flow_boundary_test.go) |
| Checkpoint ownership, fixed prefix, independent oracle, failures and retry | [khipu checkpoint tests](../../../../khipu/checkpoint_test.go), [first-fit continuation tests](../../../../khipu/linebreak/firstfit/continue_test.go) |
| A/B/C width history, equal-width reuse, empty/uninitialized inputs and lifecycle | [flow_test.go](../../../glyphing/paragraph/flow_test.go), [prepared_lifetime_test.go](../../../glyphing/paragraph/prepared_lifetime_test.go) |
| Retained bidi context, feature domains, styles, atomics and skips | [flow_context_test.go](../../../glyphing/paragraph/flow_context_test.go), [flow_test.go](../../../glyphing/paragraph/flow_test.go) |
| Cost accounting and analysis | [production benchmarks](../../../glyphing/paragraph/flow_cost_test.go), [P6 analyzer regressions](../../flow-cost/analyze_test.py) |

The archived E2 mixed-RTL rejection/stall cases remain documented limitations;
successful retained-context tests do not establish general bidi conformance.
Historical E3 analyzer tests and overlay instrumentation remain as supporting
source snapshots; the production P6 harness owns today's metric and zero-call gates.
