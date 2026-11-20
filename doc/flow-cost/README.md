# Production flow cost harness (P6)

Reflow PR 2 note (2026-11-20): the retained timing reports predate the immutable
Khipu/Tikray migration. The phase helper now binds copied atomic advances and
freezes captured plans; raw historical evidence is unchanged. Migrated helper
correctness tests do not constitute a new performance measurement or release
baseline. Application integration has since passed the
[PR 3 correctness gate](../REFLOW_PR3.md). The timings remain historical.

The benchmarks live in `glyphing/paragraph/flow_cost*_test.go`; the measurement
code is compiled only for tests. No production code, dependency, public API or
historical E1–E3 evidence is replaced. The [decision report](../FLOW_COSTS.md)
explains the supported profile and interpretation.

From the typography module root:

```sh
go test ./glyphing/paragraph -run TestP6MeasurementAdmission -count=1
python3 doc/flow-cost/record.py doc/flow-cost/results/NEW-RUN
python3 doc/flow-cost/analyze_test.py
```

The recorder requires a new output directory, hashes working Go sources and font
fixtures (including uncommitted source), and runs **five sequential fresh Go test
processes**. Keep other verification/benchmark jobs stopped while sampling.
`-cpu=1 -benchtime=50ms -count=1 -benchmem` matches the bounded E3 repeated matrix.
Font loading uses sibling OpenType fixtures through the ordinary adapter; missing
fixtures fail instead of skipping. Preparation uses already itemized input and
loaded/warmed fonts, not cold font I/O. The command uses no overlay or build tag.

The 104 cases cover synthetic 100/1,000/10,000 words and Gentium Latin/pure Noto
Hebrew 100/1,000 words. Each has separate preparation and initial layout costs and
10/50/90% prefix cuts in the original A-width layout with equal-width reuse,
narrower continuation and equal/narrow
whole-paragraph relayout comparators. First-fit, NoHyphenation, 10 bp em, factor-1.2
CSS line height, zero skips, no space stretch/shrink, A=300 bp and B=225 bp are held
fixed. B has 30,000 bp of height. Relayout is a cost comparator, not a correctness
oracle: it may change the prefix. No 10,000-word real-font pilot is pooled into
these samples, and no arbitrary mixed-bidi admission claim follows from pure RTL.

A fixed-100-word-suffix series uses 100/1,000/10,000 synthetic `aa` words at A=145 bp,
B=108.75 bp. Checkpoint extraction is measured separately: it copies accepted decisions from
an already validated Tikray and records consumed atomic advances. A **separate component probe**, checked against actual Fit
output, times calls to production checkpoint, binding, seeded breaking, freezing
and CSS assembly/publication. Those clocks are not instrumentation inside Fit;
their sum is not its wall time. They exclude Fit's full validation/rebasing and
some orchestration. Allocation counts cover the whole probe, not individual phases.
Material counts are actual input/output cardinalities, not knot-loop visits.
A counted suffix-width schedule records breaker width requests (including any
unexpected prefix queries). Freezing consumes captured Tikray geometry, with no
width callback. Existing first-fit tests and the
production call-graph audit establish zero prefix search/zero reuse breaker work.

Synthetic wrappers count resolution, shaper factory, Shape, CheckFace, metric,
scale and conversion calls. The real adapter retains concrete faces untouched;
only factory/Shape/CheckFace are wrapped there. Equal-width synthetic calls seal
all services so accidental queries fail. Every warm case rejects new shaping or
CheckFace, failed/partial results and overflow. No artificial zero candidate-visit
counts are reported for uninstrumented APIs.

`analyze.py` rejects failed, incomplete, duplicate or inconsistent matrices and
retains raw five-process timing/allocation distributions, ranges and CVs. Workload
mixtures are **weighted estimates per process**, not additional mixed-dispatch
measurements: 90% equal + 10% continuation and 50/50, compared with the corresponding
whole-relayout mix. Preparation/initial layout is excluded from those incremental
costs. There is no fixed speedup gate; do not infer significance from overlapping
ranges or treat shared-host samples as a production latency guarantee.


## Evidence validation and original-cut labels

New frame rows report `original-cut-lines/op` and `original-cut-percent`. These
identify the original A-width cut used to choose frame height; they are not the
percentage of lines or source placed after whole-paragraph relayout. The dated
2026-11-18 evidence calls these fields `accepted-lines/op` and `accepted-percent`.
Keep those raw files and their summaries unchanged. The analyzer accepts that
complete historical pair, but rejects missing/ambiguous pairs and mixed schema
versions across a run. It preserves names when summarizing historical logs.

Every family requires its full metric inventory, including backend-appropriate
service counters, frame cut metrics, and component phase/work metrics. Warm runs
must report zero factories, Shape and CheckFace. All counted synthetic services
must remain zero for equal-width reuse and checkpoint creation; phase probes must
report zero prefix width queries. Preparation may call services and changed-width
assembly may query metrics. These checks run both when parsing and summarizing,
so consistently missing counters in all five samples do not evade validation.

Summary creation is exclusive: it never overwrites existing evidence. To validate
and reanalyze historical logs, choose a new output file, for example:

```sh
python3 doc/flow-cost/analyze.py doc/flow-cost/results/2026-11-18 /tmp/p6-reanalysis.json
```

The analyzer regressions cover each family, every required field, forbidden calls,
legacy/current labels, unchanged historical summary values and overwrite refusal.
