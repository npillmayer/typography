# Bounded paragraph continuation experiment

Status: E1, bounded E2 and E3 implemented, 2026-11-18. E2 uses first-fit continuation
after either first-fit or Knuth–Plass prefixes, through temporary Go overlays.
E3 records costs and a decision report; the upstream bidi fix remains deferred.
The experiment stays isolated. Production P1–P6 are implemented across
khipu, typography and tytui: geometry reuse, checkpoint continuation, retained
rendering, bounded `Document.LayoutFlow` / `tycli --frames`, and production cost
verification. See [FLOW.md](../../doc/FLOW.md) and the
[production cost report](../../doc/FLOW_COSTS.md). Production packages do not
import this experiment. The overlay generator follows the shared P2 scan while keeping its
experimental controls out of normal builds; recorded E3 timings remain historical.
See [E1 evidence](RESULTS.md),
[E2 evidence and upstream bidi limits](E2_RESULTS.md), [E3 measurements and decision](E3_RESULTS.md),
and the underlying investigation in [FLOW.md](../../doc/FLOW.md).

The remaining sections retain the experiment's staged design and reproduction
instructions. Their references to proposed production work describe the experiment
at that time, not missing current APIs. E1–E3 result files and raw measurements
remain dated evidence; use the P6 harness for new production measurements.

## Decision and purpose

Paragraph-global breakpoint optimization is no longer a requirement for region
flow. Once a prefix is accepted into frame A, preserve its line decisions and
content; only the continuation may be rebroken for frame B. We may compare
quality with a whole-paragraph run, but equality with that run is not a success
criterion. No silent rollback of the accepted prefix is allowed.

The expected production workload is predominantly equal-width flow across two
frames. Treat the proposed 90% share as a workload assumption, not a measured
fact. The experiment must make that path independently efficient and quantify
the additional cost of narrower destinations. It must also determine whether a
safe continuation needs a small extension or a substantial redesign upstream.

This is an experiment in **reusing prepared logical material**, not recovering a
flexible khipu from Unbox. ElasticVList items are completed, measured lines.

## Package and dependency boundary

Use `typography/internal/flowexperiment` to keep the harness outside the stable
public API and outside shared layout data. No production package should import
it. Future tests may use glyphing, paragraph, layout and khipu, with synthetic
services first. Real-font cases may use the glyphing OpenType adapter and existing
offline fixtures. Do not import render, DOM, CSS core, fontfind or tytui here.
Tests that require both glyphing and rendering belong in integration; application
margin and document checks belong in tytui/documentlayout.

Files (future-stage files are added only with that stage):

| File | Purpose |
| --- | --- |
| `doc.go`, `README.md` | Package guidance and staged plan |
| `split.go` | E1 compatibility guard, two-frame split and explicit unplaced tail |
| `cases_test.go` | Deterministic text, style, frame and atomic fixtures |
| `split_test.go` | Equal-width split and no-service-call assertions |
| `boundary_test.go`, `audit_*_test.go` | Resolved-call audit and mutation regressions for operations without injectable counters |
| `guard_test.go` | Skip domains and suppressed-empty width validation |
| `e2_overlay.py` | Reproducible, checked overlay generation; no checkout edits |
| `testdata/e2/continuation_test.go.txt` | Two-frame harness and narrower/wider suffix cases |
| `testdata/e2/oracle_test.go.txt`, `kp_test.go.txt` | Independent breaks, bidi, features, heights, and a distinct KP prefix |
| `testdata/e2/lifetime_test.go.txt`, `admission_probe_test.go.txt` | Retry/ownership/policy tests and the known input-admission counterexample |
| `testdata/e2/{khipu,firstfit,paragraph}.go.txt` | Owning-package experimental seams, active only through the overlay |
| `ownership_test.go` | Retry, lifetime, detached slices and failure publication |
| `e3_overlay.py`, `testdata/e3/timing.go.txt` | Overlay-only phase clocks around owning operations |
| `testdata/e3/{fixture,bench}_test.go.txt` | Correctness admission, offline fonts, frame and fixed-suffix benchmarks |
| `e3_record.py`, `e3_analyze.py`, `e3_analyze_test.py` | Reproducible provenance, strict five-sample analysis and analyzer regressions |
| `RESULTS.md` | Observed E1 results, limitations and recommendation |
| `E2_RESULTS.md` | E2 correctness, operation counts, limits and next decision |
| `E3_RESULTS.md`, `testdata/e3/results/2026-11-18/` | Decision report, raw measurements, source/font hashes and distributions |

Keep experimental helpers unexported. Do not add an executable, new font model,
renderer, replay schema or full document scheduler to demonstrate the idea.

## Bounded input and policy

The principal experiment uses one fully prepared paragraph and two resolved,
rectangular content frames. Width and remaining height are explicit; the harness
does not calculate margins, padding or borders. Use the current CSS full-height
line boxes, stable font/style services, horizontal writing, Latin/Hebrew text,
NoHyphenation and fixed atomic border boxes/margins.

Start with deterministic first-fit. Then test an existing Knuth–Plass prefix
with a separately optimized suffix; it is acceptable for the suffix to use a
different policy if the choice is explicit in the result. Keep boundary scoring
and paragraph-start/end behavior explicit rather than presenting a suffix as a
fresh paragraph. Do not optimize frame fullness at the expense of the prefix.

Exclude auto-sized objects, floats, footnotes, nested fragmentainers, different
style contexts across frames, automatic hyphenation, widow/orphan rollback,
column balancing, arbitrary parshape offsets and vertical elasticity. Include a
small explicit-soft-hyphen extension only after the NoHyphenation stages pass;
report it separately, not as a prerequisite for proving the simpler case.

For feasibility fixtures, cap a paragraph at 64 source tokens and 16 atomic
occurrences. Use benchmark sizes of 100, 1,000 and 10,000 words separately.
No unbounded schedule search or iterative “try until it fits” algorithm is part
of this experiment. One optional three-frame smoke case tests continuation
composition; it does not expand the principal two-frame scope.

## Three stages with explicit stopping points

### E1: equal-width fast path using existing APIs

Implemented by the private `splitEqual(ctx, vlist, sourceKey, frameA, frameB)`.
The caller supplies a completed layout and content-frame heights in fractional
big points. All interior line boundaries are permitted; no widow/orphan or
keep-together policy is implied. The result always retains three non-nil immutable
fragments: A, B and the unplaced tail. `complete` means no tail remains;
`moreFrames` means some content was placed but a tail remains; `noProgress` means
neither frame accepted any remaining line. Zero-height lines count as progress.
Horizontal overfull lines are reported separately by original line index,
including those in the tail. Height success does not promise horizontal or ink fit.

`reuseKey` is a caller attestation, not a hash computed from the VList. It includes
width, original skip width/minimum/maximum triples, and nonzero revision IDs for
source, styles, fonts, line-height, atomic geometry and paragraph policy. The
caller must advance the appropriate revision on every layout-affecting change
and keep borrowed services stable. Both frames must match the source key exactly;
the splitter additionally verifies every frozen target width, including retained
diagnostic lines when visible output is suppressed. Diagnostics remain unplaced.
Skip natural/minimum widths must be finite; maxima may use Fil/Fill/Filll, but
never Infinity, matching khipu's width-triple domain. A false attestation
about other inputs cannot be detected from retained geometry. Unequal keys fail
with `errIncompatible`; invalid requests/geometry wrap `ErrIllegalArguments`.
Cancellation preserves its cause. Errors return no result, including late errors;
earlier layouts and split results remain reusable.

The algorithm and required evidence are:

1. Lay out the whole paragraph once at A's width, with the chosen breaker.
2. Find the last permitted line boundary fitting A's remaining height using
   authoritative line-box ends. Keep zero-height lines distinct and apply strict
   finite-domain checks. Do not add a new epsilon to claim that an overflow fits.
3. Retain A's prefix and rebase B's remainder using existing positioned fragments.
   Preserve full heights, extents, horizontal records and the frozen owner.
4. Test B's height independently. If the remainder exceeds it, return a distinct
   “more frames required” experimental outcome; never drop the tail.

Handle cuts at zero and at the end explicitly: `SplitFragment` accepts only
interior boundaries. Empty input, everything fitting A and nothing fitting A are
normal test cases. No successful nonempty split is guaranteed for oversized lines.
If neither frame can accept the next indivisible line, report no progress or
diagnosed overflow according to the selected test policy; never loop.

The fast-path guard is **compatible line layout**, not width equality alone:
same effective width/skips, styles/fonts, line-height, atomic geometry and paragraph
policy. Frame origins may differ. Changes to layout-affecting inputs invalidate
reuse even when the numeric widths agree. Future widow/orphan rules can alter the
split boundary without necessarily requiring horizontal rebreaking.

Required result: after the initial layout, zero font-metric queries, shaping,
itemization, horizontal binding and breaker calls. Geometry traversal/copies are
allowed. E1 reports boundary lines examined and interior splits; delegated
validation also traverses lines/knots and copies geometry. Allocation and timing
measurements are recorded in E3. Runtime service traps and a resolved-call source audit
verify the avoided work; see RESULTS.md for their scope.
Existing fragment rebasing may round coordinates; preserve
A's retained geometry exactly, compare B's positions against the specified
translation, and leave arithmetic tolerances unchanged. The unplaced tail may
undergo two translations; comparisons preserve their subtraction order.

**Stop/go:** establish this path first. If it fails ownership or geometry checks,
fix the owning invariant before attempting different-width continuation.

### E2: correctness of a fixed-prefix continuation

The bounded implementation below follows this plan with **first-fit for every
suffix**, retaining the seed's skips/line-height/atomic geometry and original
paragraph fill. It does not implement a separately optimized KP suffix. A fixture
whose accepted KP boundaries differ from first-fit verifies that changing the
suffix policy does not replace the prefix.

The constrained reference scans all locked prefix knots before entering the
existing first-fit suffix loop. The resumed candidate seeds the same certified
boundary list and enters that loop immediately after the last accepted knot.
Fresh trim/carry state is valid only under the admitted NoHyphenation policy;
explicit and automatic hyphenation are rejected before the initial layout.
Neither implementation slices text, edits Khipu planes or resets bidi context.

Checkpoint creation validates the original prefix against its preparation and
costs a prefix scan. Subsequent calls validate owner/seed identity and copy/check
boundary indexes; the resumed breakpoint search visits no prefix knots.
Both paths still bind, freeze, reorder and assemble the **complete paragraph**.
Per-invocation counters measure binding and logical-materialization visits,
including the prefix; assembly reports its completed line count. Metrics/style
queries for CSS assembly are expected. Base shaping and face acceptance stay at
zero additional calls. These are semantic/operation observations, not E3 timings.

The output holds the original A fragment and new B/tail fragments with their own
complete frozen owner. Line indexes belong to the respective owner; original
source coordinates remain paragraph-relative and unchanged. B's height is tested
after rebreaking, with an explicit unplaced tail, no-progress and overflow report.
No application margins, paint or renderer enter the experiment.

Only FlowStart initializes a seed. Nil and zero-value seeds have no initial
result and fail the frame harness with an input error before geometry/service
work. A valid empty paragraph remains initialized. These checks do not weaken
the seed/preparation/checkpoint identity guards or cancellation precedence.

To run E2 from this module:

```sh
python3 internal/flowexperiment/e2_overlay.py /tmp/flow-e2
go test -overlay /tmp/flow-e2/overlay.json ./internal/flowexperiment -run '^TestE2' -count=1 -v
```

The generator patches copies of the owning first-fit loop and paragraph finishing
path in the output directory, adding source seams and tests through Go's overlay
map. Checked patch anchors fail on upstream drift. Generated files must stay
outside typography and khipu checkouts. The `.go.txt` inputs are inert in ordinary
builds; exported `Flow*` names exist only in overlay builds and are not proposed
production APIs. Use the same overlay for full checks in typography, khipu and
tytui. No tag, unsafe access, dependency replacement or permanent sibling edit is
needed. `TestE2MarkupAdmissionProbe` currently skips one documented ordinary
preparation failure; it does not count that input as a successful continuation.
The review also found mixed RTL inputs that stall in ordinary bidi preparation
despite a context deadline. See the [upstream limitations](E2_RESULTS.md#upstream-bidi-limitations)
for exact inputs and bounded reproduction guidance. The bidi fix is deferred;
continuation cancellation does not imply interruptible initial preparation.

Use the same initial A-width result to choose the cut. Keep the entire original
logical preparation alive, including paragraph bidi context, source/feature
domains and original paragraph-ending material. Freeze the accepted prefix as an
immutable reference. Rebreak only the suffix under B's geometry and policy.

The report distinguishes two implementations:

- **Constrained reference:** allow a complete logical traversal while enforcing
  every selected prefix boundary and preserving its break semantics. Use original
  paragraph context and a stable width schedule: A for the accepted line count,
  B thereafter. This can validate output, but is not evidence of suffix-only cost.
- **Resumed candidate:** begin breaking at an owner-validated logical checkpoint,
  carrying original line count, edge rules and any relevant continuation state.
  Count how much prefix material is actually visited, rebound or materialized.
  Retaining the old prefix in the output while recomputing it internally is not
  the desired performance result.

Neither entry point is public in default builds. Stage E2 uses a bounded
owning-package spike, not a fake suffix built from source substrings. The
experimental seams are a constrained-break request, a validated start
checkpoint, and access to final candidate line geometry through the existing CSS
assembler. Khipu's materialization currently requires a complete path ending at
the trailer; record separately whether the first prototype still materializes
the full path even when its search resumes only on the suffix.

Keep any experimental access to private state in its owning khipu/paragraph
package, exercised with temporary Go overlays or a narrowly scoped experimental
build tag. Store reproducible overlay inputs and commands with the experiment
alongside the implementation. Do not use unsafe access, copy the breaker into this package,
edit exported Khipu planes as a shortcut, or expose production APIs just to make
the harness compile. If the seams demand a broad rewrite, stop and document that
result before extending the experiment. The current seams are overlay-only.

Use explicit expected breakpoints for small synthetic examples, plus the
constrained reference, as correctness oracles. For first-fit the resumed output
should match that reference under identical policies. For a KP suffix, define
its initial search/scoring state first; compare only runs using that same state.
An unconstrained global KP run is a quality comparator, never an oracle for
committed-prefix equality. An independently enumerated tiny case should guard
against two implementations sharing the same boundary mistake.

For this stage B is usually tall enough for the whole new suffix, so height
overflow does not conceal rebreaking defects. Add insufficient-height cases
separately. Keep the original A prefix and the new B material as separate
experimental fragment records; do not weaken VList's full-frozen-owner invariant.
Publishing before the entire suffix is known is outside this first experiment.

**Stop/go:** require correct ordinary boundaries, bidi and paragraph edge behavior
before proceeding to performance claims or the explicit-discretionary extension.

### E3: costs, boundaries and the decision report

Implemented with the E3 overlay, which includes E2 and adds phase clocks and
benchmarks without editing production sources. [E3_RESULTS.md](E3_RESULTS.md)
records the decision: pursue equal-width application integration design, while
treating complete-path narrower continuation as an explicit slower possibility.
Resumed search avoids prefix knots, but binding, freezing and assembly remain
proportional to the whole paragraph. Production integration lives outside this experiment and is now implemented.

The repeated matrix contains 121 cases, each sampled in five fresh sequential
processes: synthetic 100/1,000/10,000 words, plus offline Gentium Latin and Noto
Hebrew at 100/1,000 words. A separate successful single-iteration real-font
10,000-word pilot was expensive enough to bound the repeated real-font set;
its results are retained separately, not averaged or hidden. The optional
`-flow-e3-large-real` flag enables those large cases. The standard analyzer
intentionally accepts only the stated 121-case matrix.

Frame benchmarks include cut search, checkpoint creation, complete publication
and B/tail placement. Fixed-suffix benchmarks keep exactly 100 words after
increasing prefixes; they measure checkpoint creation separately from core
continuation. Fresh paragraph preparation is measured with pre-itemized text and
preloaded, warmed services; it does not include loading/parsing/itemization.
Untimed admission compares complete frozen results and the accepted prefix before
frame measurements. E1 keeps its service traps and typed no-breaker audit.

To generate the overlay and run admission plus all experiment regressions:

```sh
python3 internal/flowexperiment/e3_overlay.py /tmp/flow-e3
go test -overlay /tmp/flow-e3/overlay.json ./internal/flowexperiment -count=1
```

For timing, use a separate process for each sample, without concurrent checks:

```sh
go test -overlay /tmp/flow-e3/overlay.json ./internal/flowexperiment -run '^$' -bench '^BenchmarkE3' -benchmem -benchtime=50ms -count=1 -cpu=1 -timeout=10m
```

See the [evidence README](testdata/e3/results/2026-11-18/README.md) for recording
provenance, saving five logs and deriving distributions and workload mixtures.
The requirements below govern interpretation and future repetitions.

Measure the equal-width split, constrained reference, resumed candidate and a
whole-paragraph relayout baseline. Separate preparation from the operation after
the cut. A relayout baseline is a cost comparator, not an equivalent output policy.
Use the same services and options and report cold preparation separately from
warm reusable preparation. Include prefix fractions near 10%, 50% and 90%.

Run each benchmark with allocations reported and at least five independent
samples on a recorded Go version/OS/CPU. Report dispersion, not a lone timing.
Use a synthetic backend for exact counters and a small offline real-font set for
representative shaping costs. Aggregate a stated 90/10 equal/different-width mix,
but also publish each path independently and a 50/50 sensitivity calculation.
Do not hide failures or fallback runs inside averages.

With a fixed-size suffix and increasing prefix length, determine separately
whether breaker work, binding, freezing and assembly still scale with the whole
paragraph. A prototype can establish semantic feasibility while failing the
intended suffix-only cost model. Record that distinction explicitly.

## Case matrix and evidence to retain

| Family | Minimum cases | Evidence |
| --- | --- | --- |
| Frequent path | Equal widths; exact fit, slack, empty A/B, all in A, B exhausted | Shared frozen owner; zero post-layout service/breaker calls; no lost tail |
| Width change | B at 50%, 75% and 125% of A; early/middle/late cut | Fixed prefix; new suffix endpoints and allocation at B's width |
| Height variation | Mixed sizes, surviving spaces, blank/zero-height lines, tall and zero-sized atomics | Full-height oracle; accepted extent and no-progress outcome |
| Direction/context | LTR, RTL, mixed Hebrew/Latin/numbers, neutrals/brackets, isolate spanning cut | Logical coverage and fresh line-local ordering, with original paragraph context |
| Paragraph edges | Interior cut, true end, leading/trailing whitespace, forced line break | No extra final fill in A or new paragraph-start treatment in B |
| Object boundaries | Fixed opaque box before/after cut; negative advance and coincident origins | Atomic IDs/order; no object split; placements detached |
| Failure/lifetime | Wrong owner/checkpoint, cancelled attempt, injected failure, repeated retry | No partial publication or prefix mutation; earlier results still usable |
| Guard/termination | Equal width with changed line-height or object size; line too large; optional third frame | Fast-path refusal, explicit exhaustion/no-progress, bounded work |
| Later extension | One selected soft hyphen at cut, including an empty replacement half | Matched pre/post carry, original source mapping, no duplicate hyphen |

Record original byte/scalar coverage, glyph IDs/advances/features and frozen fit
diagnostics alongside geometric extents. Glyph or knot counts alone do not prove
source preservation: discarded whitespace, zero-byte generated fill and
replacement provenance need their own checks. No new paragraph or bidi reset is
allowed at a region boundary.

For each case record the cut, accepted line count/height, prefix semantic and
geometry comparison, suffix breakpoints, overflow status, service-call counts,
visited logical material, allocations and the implementation route used.
Logical height fit does not promise ink containment or horizontal fit. Any
overfull output must be reported even when the call succeeds.

Surrounding margins remain application-owned. The harness receives content-frame
geometry only. Application verification in tytui must separately demonstrate that
the owner's original margins/source/paint remain retained, are not copied onto
each fragment, and are not mutated by failed or repeated flow attempts.

## How results determine the next step

| Result | Decision |
| --- | --- |
| E1 exact retained content, sound rebasing and zero service calls | Build production equal-width dispatch around frozen fragmentation; no parshape expansion needed for this path |
| E2 matches constrained semantics for the admitted cases | A prepared-continuation contract is viable; propose only the owner APIs the experiment actually needed |
| E2 works only after full traversal/materialization | Semantics proven, performance not proven; cost the upstream range/binding work before promising cheap continuation |
| Bidi/paragraph-edge discrepancies remain | Do not generalize the API; isolate the missing context and keep that case unsupported |
| Variable heights work with fixed objects | Candidate measurement and geometric cuts suffice for this bounded policy; no global frame optimizer is needed |
| Only width-dependent object cases fail | Keep the simpler path; design CSS/application object re-resolution separately |
| Fast path is cheap but rare path expensive | Evaluate the measured workload mix; prefer an explicit slower fallback over burdening every split |
| Explicit discretionaries require substantial extra state | Ship no-hyphenation scope first if useful; keep the extension separate rather than guessing source offsets |

Correctness, immutability and honest overflow reporting are hard gates. Timing
results inform an engineering choice; this plan invents no millisecond SLA or
required speedup before a baseline exists. The final recommendation must say
which inputs are supported, which work is actually avoided, what remains linear
in the prefix, and what public API changes would be required.

## Execution and completion

Implement and review E1, then E2, then E3. Do not start a generalized page scheduler
or broaden scripts/policies when a bounded case fails. Preserve raw benchmark
output and commands; add a dated RESULTS.md with passed/failed/unsupported cases,
counterexamples, the smallest necessary upstream change and a go/no-go conclusion.
No results should be recorded as passing before those runs occur.

Once executable behavior or shared seams are added, run ordinary/race suites,
build and vet in typography and tytui, including dependency audits and adapter
regressions, and relevant khipu/css-box-layout checks for changed boundaries.
Run the same checks with any experimental tag/overlay enabled; default builds
must remain unchanged. Keep replay v5 and static proof controls intact. The
E1 harness needs no experimental tag, overlay or upstream seam. E2 and E3 use the
reproducible overlays above. E3 adds measured costs and recommendations; these
bounded stages do not establish production readiness or general bidi support.
