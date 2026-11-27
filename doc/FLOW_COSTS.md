> This document records design or measurements from an earlier execution API.
> For the supported preparation, trial, checkpoint and prefix-refitting APIs,
> use the [paragraph guide](../glyphing/paragraph/README.md). Historical timings
> are not measurements of the current implementation.

# P6 production flow costs and release decision

Reflow PR 2 note (2026-11-20): the recorded timings below predate the immutable
Khipu/Tikray migration. The phase helper now binds copied atomic advances and
freezes captured plans; raw historical evidence is unchanged. Migrated helper
correctness tests do not constitute a new performance measurement or release
baseline. Application integration has since passed the
[PR 3 correctness gate](REFLOW_PR3.md). The timings remain historical.

Measured 2026-11-18 against the current local working trees. P6 completes
verification of the opt-in bounded frame-flow profile; it does not claim general
pagination or arbitrary mixed-bidi support. No production algorithm was changed
for these measurements. Historical E1–E3 evidence and replay fixtures are unchanged.

## Decision and supported profile

Retain the geometry-only equal-width route as the frequent path. It performs no
post-layout service or breaker work. Keep checkpoint-based first-fit continuation
for width changes, accepting full binding, freezing and CSS assembly initially.
There is no fixed speedup gate and no promise of suffix-proportional total cost.
The measurements support these existing choices, not a new cache or algorithm.

The application entry is `documentlayout.Document.LayoutFlow`; `tycli --frames`
exposes it explicitly. One selected root paragraph plus fixed pixel-width opaque
children is admitted. Frames are dumb, finite content rectangles; original
paragraph styles, paint, bidi context, line height and fixed atomic interiors stay
unchanged. Accepted prefix breakpoints and A/B/C width history remain locked.
Each destination is tried once, no-progress leaves source intact, and exhaustion
retains an explicit frozen remainder. CLI incomplete flow fails without replacing
output. Original surrounding margins remain application metadata, never new
per-frame gaps. The first frame must match the source CSS callback content width
at DU precision, after owner edges.

Process paragraphs in turn. Keep preparation and all intermediate checkpoints only
while the current paragraph is unfinished or retryable. The bounded application
invocation releases them on completion, exhaustion or failure; published fragments
remain usable independently. Incomplete application snapshots are inspection
results, not resumable working handles. Stable retained font/source services still
obey their original lifetime contract. No global preparation cache or flow replay
format was added; paragraph replay remains CSS-only v5.

**Restriction:** synchronous upstream bidi preparation can reject or stall some
mixed RTL input before flow starts. It is not interruptible by a context deadline.
Pure Hebrew samples and admitted correctness cases do not establish arbitrary
mixed-bidi support. That fix remains separate; no string blacklist, hidden skip
or timeout goroutine was introduced. Multiple-paragraph scheduling, width-dependent
opaque children, widow/orphan rollback, generalized pagination, elasticity and
flow replay remain deferred.

## Method and evidence

[Harness and reproduction commands](flow-cost/README.md),
[raw samples and environment](flow-cost/results/2026-11-18/README.md), and
[all distributions and workload estimates](flow-cost/results/2026-11-18/summary.json).

Five sequential fresh Go test processes ran 104 cases each, with `-cpu=1`,
`-benchtime=50ms`, `-benchmem`, `-count=1`, no overlays/build tags/race instrumentation,
and no concurrent verification jobs. The environment records Go/OS versions,
module HEADs plus hashes of working Go sources, and offline font hashes; the
working-tree hashes matter because this development pipeline is uncommitted.
Other host applications, CPU frequency and affinity were not controlled.

Environment: `go version go1.25.6 darwin/arm64`, `macOS-26.3-arm64-arm-64bit`. CPU is recorded in each raw log.


Preparation, initial layout, complete two-frame operations, and fixed-suffix
components are separate measurements. `Prepare` gets already itemized input and
preloaded/warmed stable font services; disk I/O, parsing, discovery and coverage
are excluded. Each fixture loads fonts through the normal adapter from sibling
OpenType test data. Missing fixtures fail; no network or relaxed parser is used.

Seven datasets repeat E3's bounded matrix: synthetic 100/1,000/10,000 words;
Gentium Latin and Noto Hebrew 100/1,000 words. First-fit, NoHyphenation, 10 bp em,
CSS factor-1.2 line height, zero skips and no space stretch/shrink are held fixed.
A=300 bp, B=225 bp, and B's height=30,000 bp. Prefix fractions 10/50/90% are rounded
to interior line boundaries of the original A-width layout; those original-cut
percentages are in the logs (the small Hebrew case has only five lines, realizing
20/40/80%). The old expensive large-real pilot
is not rerun or pooled into this five-process matrix.

The dated logs use `accepted-lines/op` and `accepted-percent` for the original
cut, including relayout comparators. They do **not** report the fraction of a
newly relaid-out paragraph placed into A: synthetic N100/P50 narrow relayout
places 5 of 13 new lines (38.5%), while the original cut was 5 of 10 (50%).
New runs use `original-cut-lines/op` and `original-cut-percent` to make this
distinction explicit. Historical logs, hashes and summaries remain unchanged.

`equal` calls production `Continuation.Fit` for both frames; `continue` adds a
width change at B, including lazy checkpoint creation and full result assembly.
Both begin with one already completed `StartFlow`. `relayout-equal/narrow` call
`Prepared.Layout` for the entire paragraph and then `Fragment`/`FitFragment` for
both destinations. Those are cost comparators, not correctness oracles: they can
change A's accepted prefix. Timed routines reject failures, incomplete output and
overflow rather than counting a fast error as a sample.

## Complete two-frame operations

Median [minimum, maximum] microseconds at the nominal midpoint cut:

| Dataset / words | Equal reuse | Continuation | Relayout equal | Relayout narrow |
| --- | ---: | ---: | ---: | ---: |
| synthetic / 100 | 4.8 [4.7, 4.8] | 139.2 [138.1, 142.6] | 150.6 [149.7, 151.0] | 157.1 [155.7, 158.0] |
| synthetic / 1,000 | 37.6 [37.6, 38.3] | 1,307.3 [1,304.2, 1,355.1] | 1,456.1 [1,446.5, 1,486.3] | 1,518.5 [1,518.1, 1,578.6] |
| synthetic / 10,000 | 274.8 [274.2, 296.4] | 11,659.4 [11,527.1, 19,164.4] | 13,201.4 [13,118.7, 13,471.5] | 13,841.5 [13,803.6, 14,253.3] |
| gentium / 100 | 4.5 [4.2, 5.1] | 135.9 [126.8, 144.5] | 146.5 [138.9, 159.6] | 155.4 [149.8, 162.8] |
| gentium / 1,000 | 28.7 [28.3, 29.3] | 1,198.8 [1,177.3, 1,376.6] | 1,313.7 [1,303.2, 1,512.7] | 1,411.8 [1,406.2, 2,682.4] |
| hebrew / 100 | 3.3 [3.3, 3.3] | 113.0 [112.4, 113.2] | 123.2 [123.0, 126.8] | 129.2 [129.0, 151.6] |
| hebrew / 1,000 | 20.8 [20.6, 23.1] | 1,077.2 [1,042.1, 1,146.4] | 1,255.3 [1,196.1, 1,394.7] | 1,308.0 [1,264.4, 2,024.0] |

Small differences with overlapping ranges are not evidence of a reliable speedup.
For example, the synthetic 10,000-word continuation has a 19.2 ms outlier and
Gentium 1,000-word narrow relayout reaches 2.68 ms. Raw ranges and CVs are retained;
no samples were discarded. The strong equal-width cost distinction is consistent
across all measured cuts and backends. It still scans/copies geometry and retains
a complete frozen owner, so it is not constant-time.

Median allocation traffic at the midpoint (MiB / allocations per operation):

| Dataset / words | Equal reuse | Continuation | Relayout narrow |
| --- | ---: | ---: | ---: |
| synthetic / 100 | 0.014 / 27 | 0.308 / 1,417 | 0.298 / 1,461 |
| synthetic / 1,000 | 0.140 / 30 | 3.051 / 12,658 | 2.984 / 13,472 |
| synthetic / 10,000 | 1.364 / 33 | 31.866 / 124,664 | 31.198 / 133,089 |
| gentium / 100 | 0.013 / 26 | 0.304 / 1,294 | 0.325 / 1,413 |
| gentium / 1,000 | 0.114 / 30 | 3.037 / 11,558 | 3.198 / 12,586 |
| hebrew / 100 | 0.008 / 25 | 0.233 / 868 | 0.237 / 866 |
| hebrew / 1,000 | 0.058 / 29 | 2.279 / 6,986 | 2.387 / 7,495 |

## Workload estimates

These are paired, per-process **weighted incremental cost estimates**, not extra
benchmarks or measured workload frequencies. 90/10 means 90% equal-width pairs
and 10% narrower B; 50/50 weights both equally. Preparation and initial layout are
excluded. The comparator is the same weighted mix of whole-paragraph relayout.
Ratios are medians of paired ratios, not ratios of the displayed medians.

| Dataset / words | 90/10 retained / relayout (µs) | Ratio | 50/50 retained / relayout (µs) | Ratio |
| --- | ---: | ---: | ---: | ---: |
| synthetic / 100 | 18.2 / 151.2 | 8.31× | 72.0 / 153.8 | 2.14× |
| synthetic / 1,000 | 164.6 / 1,462.4 | 8.86× | 672.4 / 1,487.3 | 2.21× |
| synthetic / 10,000 | 1,412.7 / 13,262.8 | 9.40× | 5,966.8 / 13,522.2 | 2.26× |
| gentium / 100 | 17.7 / 148.1 | 8.40× | 70.2 / 151.2 | 2.16× |
| gentium / 1,000 | 146.1 / 1,323.5 | 9.17× | 614.0 / 1,362.8 | 2.25× |
| hebrew / 100 | 14.3 / 123.8 | 8.68× | 58.1 / 126.1 | 2.17× |
| hebrew / 1,000 | 127.2 / 1,260.5 | 10.17× | 549.4 / 1,281.6 | 2.38× |

## Checkpoint, search and complete material work

Exactly 100 synthetic `aa` words (299 source bytes) remain after 0/900/9,900
prefix words; A=145 bp and B=108.75 bp. Checkpoint certification is measured
separately and scans the complete legal path, even though seeded search does not
search the accepted prefix. Its median cost grows from 0.44 to 4.29 to 45.35 µs;
allocations are 1/2/2 (32/800/8,224 bytes).

The separate test-only component probe calls the actual production primitives.
It is compared with production continuation output by `TestP6MeasurementAdmission`.
Its clocks are not instrumentation inside `Fit`: complete Fit costs above also
include orchestration, validation and rebasing. Phase sums are not the API wall
time. Freezing includes line-local bidi, materialization and width allocation;
assembly includes metrics, atomic placement and publication.

| Prefix words | Bind µs | Search µs | Freeze µs | Assembly µs | Bound knots | Frozen knots | Assembled lines |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 0 | 6.5 | 40.5 | 67.6 | 28.4 | 200 | 186 | 15 |
| 900 | 52.1 | 43.4 | 629.4 | 270.2 | 2,000 | 1,896 | 105 |
| 9,900 | 345.3 | 47.8 | 5,089.3 | 2,571.2 | 20,000 | 18,996 | 1,005 |

Every fixed-suffix case makes 214 breaker width queries, with **zero prefix width
queries**, while total material and assembly grow with the prefix. These are
actual width-schedule calls and input/output cardinalities, not invented counts
of internal candidate or materialization-loop visits. The owning first-fit
regressions independently protect zero prefix search. This confirms the intended
trade-off: resumable suffix search with full reconstruction, not suffix-only cost.

## Preparation and initial layout

Median fresh preparation [min, max] and initial `StartFlow`, microseconds:

| Dataset / words | Fresh preparation | Initial layout |
| --- | ---: | ---: |
| synthetic / 100 | 4,308.0 [3,998.2, 5,272.7] | 150.4 [149.7, 159.4] |
| synthetic / 1,000 | 47,802.8 [45,738.5, 49,293.9] | 1,449.8 [1,442.9, 1,554.3] |
| synthetic / 10,000 | 1,168,548.8 [1,131,461.1, 1,208,642.3] | 13,191.7 [12,738.1, 13,686.2] |
| gentium / 100 | 62,170.0 [61,394.2, 65,453.5] | 140.1 [138.5, 153.5] |
| gentium / 1,000 | 1,382,529.2 [1,361,335.6, 1,484,263.2] | 1,345.0 [1,303.1, 1,945.7] |
| hebrew / 100 | 3,540.6 [3,516.6, 4,238.2] | 123.9 [122.4, 127.6] |
| hebrew / 1,000 | 37,303.4 [36,782.0, 79,808.2] | 1,204.7 [1,191.6, 1,215.6] |

Fresh preparation can dominate end-to-end time, particularly Gentium at 1,000
words (about 1.38 s and 1.20 GB allocation traffic per preparation here). Some
expensive samples run only one timed iteration; raw iteration counts are retained.
These measurements do not locate that cost within bidi/shaping and are not a
justification for retaining preparation after a paragraph is finished. They do
show why incremental frame ratios must not be presented as whole-application
speedups. No preparation or font-service optimization is included in P6.

## Correctness and release checks

The hard gates remain independent of speed:

- Compatible reuse makes no font/service calls and no binding, breaking or assembly.
  Synthetic services are sealed after initial layout; the transitive production
  call-graph audit covers the equal-width branch, including breaker exclusion.
- All warm measurements report zero new factories, Shape and CheckFace calls.
  Synthetic metrics/resolution/conversion counts remain explicit for assembly;
  real concrete faces are untouched and only their streaming calls are counted.
- Production tests retain fixed prefixes, source/bidi/paint context and A/B/C width
  history, check detached ownership, zero/negative geometry, no-progress and failure
  recovery, and prove preparation can be collected while frozen fragments survive.
- Application/CLI checks cover fixed/nested objects, once-only CSS callbacks,
  cancellation, working-table/token release, complete/incomplete frame scheduling,
  retained export after release, failed-export zero bytes and prior-output retention.
  Existing ordinary document/tycli output controls and replay v5 remain required.

No upstream bidi fix, generalized scheduler, replay format or historical control
regeneration is part of this release decision.

Validation run on 2026-11-18, after measurement collection:

| Module | `go test ./... -count=1` | `go test -race ./... -count=1` | `go build ./...` | `go vet ./...` |
| --- | --- | --- | --- | --- |
| typography | passed | passed | passed | passed |
| tytui | passed | passed | passed | passed |
| khipu | passed | passed | passed | passed |
| css-box-layout | passed | passed | passed | passed |

These runs include dependency audits, adapter/CheckFace regressions, P1/P3 call
boundaries and lifecycle tests, seeded-search regressions, retained rendering,
P5 documentlayout/tycli controls and replay v5 checks. `TestP6MeasurementAdmission`
also passed separately; `python3 doc/flow-cost/analyze_test.py` passed all three
analysis tests (incomplete/failed matrix rejection, five-process requirement,
weighting/dispersion). No passing result is inherited from an earlier stage.


### Measurement review corrections

The analyzer now requires each benchmark family's full metric schema, including
stream counters for real faces, all service counters for synthetic cases, original
cut labels for frame cases, and phase/work counts for component probes. Warm
operations must have zero factories/Shape/CheckFace; equal-width synthetic reuse
and checkpoint certification must have zero calls to every counted service.
Component probes must report zero prefix width queries. Missing metrics or failed
gates are rejected even when identical across all five processes. Nonzero service
work in preparation and metrics during changed-width assembly remain valid.

Historical evidence is read with its original complete cut-label pair, without
renaming or modifying stored values. Mixed/incomplete label pairs or mixed schema
versions within a run are rejected. Summary generation refuses to overwrite an
existing file; use an explicit new output path to reanalyze historical samples.
The historical tables above are not new measurements of the revised harness.
