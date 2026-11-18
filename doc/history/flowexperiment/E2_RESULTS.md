# E2 fixed-prefix continuation evidence

Date: 2026-11-18. Bounded E2 is implemented using temporary Go overlays.
Production integration and vertical elasticity remain deferred. Subsequent E3
cost measurements and the decision are recorded in [E3_RESULTS.md](E3_RESULTS.md).

## Finding and admitted scope

For the tested NoHyphenation paragraphs, a continuation can retain A's accepted
lines and rebreak the suffix at B's width without preparing bidi or shaping base
text again. The resumed first-fit search matches a constrained reference and
independent small breakpoint oracles. This also works after a Knuth–Plass prefix:
the suffix policy is explicitly first-fit, with no new paragraph-global optimum
claim. A distinct KP fixture prevents this comparison from accidentally testing
only prefixes that both breakers would have chosen.

This establishes bounded semantic feasibility, **not an inexpensive production
continuation**. The candidate still binds and freezes the complete logical path,
performs line-local bidi reordering and assembles a complete CSS VList. It keeps
the original A fragment and takes B/tail from the new complete frozen owner.
VList's full-owner invariant remains intact; source/feature coordinates remain
relative to the original paragraph. No suffix text is extracted or re-itemized.

Inputs are stable prepared text/services, fixed atomic geometry, horizontal
Latin/Hebrew layout, and resolved rectangular content frames. The retained seed
owns copies of the request slices, line-height policy and skips. Only B's width
changes on continuation. Explicit and automatic hyphenation fail admission
before the initial layout; no replacement carry is guessed. Width-dependent
objects, KP suffix scoring, discretionary continuation and generalized scheduling
are outside this implementation. Existing preparation rejection and progress
limitations are recorded below and excluded from successful E2 evidence.

## Work actually avoided

For `aa bb cc dd ee ff gg hh ii jj kk ll`, with synthetic 10 bp words,
5 bp spaces, no shrink/stretch, A = 40 bp and B = 20 bp, there are 24 logical
knots. The initial path has four lines. Counts below are per continuation and
hold with either initial breaker for this fixture:

| Accepted lines | Resumed search, prefix/total | Reference search, prefix/total | Binding, prefix/total | Materialization, prefix/total | CSS lines assembled |
| --- | --- | --- | --- | --- | --- |
| 1 | 0 / 18 | 6 / 24 | 6 / 24 | 6 / 24 | 10 |
| 2 | 0 / 12 | 12 / 24 | 12 / 24 | 12 / 24 | 8 |
| 3 | 0 / 6 | 18 / 24 | 18 / 24 | 18 / 24 | 6 |

The counters observe the owning binding and logical-materialization loops; they
are not estimates from line counts. Boundary-index validation separately checks
one entry per accepted line. Creating a checkpoint scans prefix knots to validate
permitted boundaries and reject skipped forced breaks; that one-time scan is
outside these continuation counts. The two-frame test harness creates a checkpoint
on each invocation, so its total work is not prefix-free. Geometry validation,
freezing validation, reordering and copying are additional work, not included in
the search counter. No allocation or timing conclusion follows from this table.

Service counters verify zero additional shaper factories, Shape calls and face
acceptance calls. CSS assembly still resolves source styles and queries metrics,
scales and conversions, including for the prefix. Retained zero-byte bidi markup
is not called again. When all visible lines fit A, continuation returns the
immutable initial result with detached diagnostics and zero phase work.

## Correctness evidence

- The independent first-fit oracle starts at byte endpoints `[9,18,27,35]`.
  After accepting one line, B = 20 gives
  `[9,12,15,18,21,24,27,30,33,35]`, B = 30 gives
  `[9,15,21,27,33,35]`, and B = 55 gives `[9,21,33,35]`.
  Both reference and resumed paths must match these explicit expectations.
- A separate shrink/stretch fixture initially yields FF
  `[9,15,23,31,34]` and KP `[9,20,29,34]`. Retaining two KP lines and using
  first-fit at B = 30 preserves those KP decisions and their allocations.
- Widths at 50%, 75%, 125% and 137.5% of A, cuts at zero/interior/end, both
  initial breakers, explicit zero skips and retained default skips are covered.
- Exact accepted geometry, knots, glyph data, feature contexts and prefix atomic
  placement/order are compared with the initial result. Reference and resumed
  results agree on endpoints, bidi runs, frozen records, placements and fit
  diagnostics. Source intervals cover the original source once; generated
  paragraph fill occurs only on the true final line.
- RTL has an independent descending-source physical-word oracle in the new
  suffix. Mixed Hebrew/Latin/numbers, neutrals/brackets, an isolate spanning the
  cut, retained zero-byte markup, forced breaks and trimmed spaces are covered.
  The feature fixture verifies original scalar feature domains; its synthetic
  shaper does not establish real-font shaping quality.
- Mixed font sizes and surviving spaces, fractional and negative half-leading,
  blank/zero-height lines, tall/zero-sized atomics, signed advances and both text
  directions retain full CSS line-height. B's height is checked after rebreaking;
  excess lines form an explicit tail. No-progress and horizontal-overflow outcomes
  are distinct. A nextafter-below-height case prevents epsilon-based false fits.
- Owner/seed/preparation mismatches, malformed checkpoints, invalid widths and
  heights, nil inputs and unsupported policies fail without partial publication.
  Cancellation is injected at every observed context check of a successful
  resumed call. A late assembly failure preserves its cause/stage; repeated
  retries match the earlier successful result and do not reshape base text.
- `TestE2UninitializedSeed` covers the reviewed panic: nil and zero-value seeds
  now fail the frame harness with `ErrIllegalArguments`, no result and no service
  calls, in both reference and resumed modes. `Initial` returns nil for these
  seeds; checkpoint/continuation entries also reject them. Entry cancellation
  still takes precedence. Valid seeds remain reusable after rejection, and a
  successfully prepared empty paragraph still has a non-nil initial result.
- Mutating caller request storage, returned diagnostics and detached geometry
  cannot change retained results. Frozen glyph copies remain detached and usable
  after dropping preparation/checkpoint handles and collecting garbage.

## Upstream bidi limitations

### Preparation rejection

`TestE2MarkupAdmissionProbe` uses source
`aa bb \uFFFC אב גד הו 12 (xy) zz`, an atomic marker at byte 6, and zero-byte
RLI at byte 10 with PDI at source end. Ordinary `paragraph.Prepare` currently
returns `paragraph prepare: khipu: invalid logical bidi segment`, before either
initial layout or continuation. A separate temporary test-only overlay, with
**none of the E2 implementation seams**, reproduced the same failure against
the existing paragraph synthetic services.

The probe skips only this StagePrepare/error combination and explicitly reports
it as unsupported. Any different failure fails the test. If upstream preparation
starts accepting this input, the probe automatically checks reference/resumed
continuation. The skip is neither a correctness oracle nor a successful bidi case.
Textual mixed isolates and a simpler all-Hebrew zero-byte isolate with an atomic
marker do pass. Investigating the mixed-markup preparation failure is separate
upstream work; E2 does not rewrite bidi preparation or relax its validation.

### Preparation does not return for some mixed RTL input

The E2 review also reproduced a separate progress failure on 2026-11-18:
ordinary `paragraph.Prepare` with source `"גד 123 123 (x) "` (including the
trailing space), `Direction: bidi.RightToLeft`, NoHyphenation and no atomic
anchors or markup did not return before a three-second test timeout. A 100 ms
context deadline did not interrupt it. The shorter forced-break example
`"a bb \n גד 123 "` reproduced the same symptom.

These were isolated baseline tests with **none of the E2 implementation seams**.
The timeout stacks remained in `uax/bidi.(*parser).pass1` / `matchRulesLHS`,
called by `styled.ParagraphFromText` from `paragraph.Prepare`. They never reached
FlowStart, checkpoint creation or continuation. This is distinct from the
explicit preparation error above and is not an E2 continuation regression.
The observed timeout does not establish that every mixed RTL input stalls.

Preparation currently calls bidi resolution synchronously and checks cancellation
after that call returns; a deadline cannot interrupt the resolver. Consequently,
E2's successful continuation-cancellation tests do not establish a time bound for
preparing arbitrary input. Upstream bidi investigation and correction remain
deferred; E3 excludes these inputs. This known case supplies no successful E2 evidence.

Reproduce it only in an isolated test process with an external bound, for example
`go test -run '^TestBidiPreparationStall$' -count=1 -timeout=3s`, whose test calls
`paragraph.Prepare` with the source/direction above and a 100 ms context deadline.
A goroutine timeout inside the same process would leave the resolver running.
No deliberately stalling test is added to the ordinary or E2 suites; the existing
admission probe skips only its specific rejection, not this progress failure.

## Isolated seams and reproduction

The generator copies owning source into a temporary directory and checks that
each patch anchor occurs exactly once. It does not edit sibling checkouts or
introduce default-build/public APIs. Inputs under `testdata/e2/*.go.txt` are inert
unless the overlay is selected.

| Owner | Overlay-only addition/change |
| --- | --- |
| khipu | Owner/preparation-bound immutable checkpoint; binding/materialization instrumentation |
| khipu/linebreak/firstfit | Reference/resumed entry into the existing loop; explicit boundary validation and cancellation |
| typography/glyphing/paragraph | Seed with copied request and private original knot boundaries; shared existing freeze/CSS publication tail |
| internal/flowexperiment | Two-frame orchestration, constrained comparisons, independent oracles and admission probe |

There is no copied breaker in the harness, mutable-plane editing, unsafe access,
new font abstraction or serialized service. Ordinary first-fit calls use the
same original loop with nil experimental control. Initial layouts use the
ordinary Prepared.Layout path. Application-owned margins, paint and source maps
remain in tytui; this experiment adds no application dispatcher or rendering
path and makes no new margin-fragmentation claim. Replay remains version 5 and
no proof/replay controls were regenerated.

From typography:

```sh
python3 internal/flowexperiment/e2_overlay.py /tmp/flow-e2
go test -overlay /tmp/flow-e2/overlay.json ./internal/flowexperiment -run '^TestE2' -count=1 -v
```

All checks below were rerun after the uninitialized-seed fix on 2026-11-18,
with the existing sibling replacements:

| Module | Ordinary tests | Race tests | Build | Vet |
| --- | --- | --- | --- | --- |
| typography, default and E2 overlay | pass | pass | pass | pass |
| tytui, default and E2 overlay | pass | pass | pass | pass |
| khipu, default and E2 overlay | pass | pass | pass | pass |
| css-box-layout, default | pass | pass | pass | pass |

For each module the commands were `go test ./... -count=1`,
`go test -race ./... -count=1`, `go build ./...` and `go vet ./...`.
The first three modules were also checked with
`-overlay /tmp/flow-e2-seed-fix/overlay.json` immediately after the Go subcommand,
generated using the same script above with `/tmp/flow-e2-seed-fix` as destination.
CSS core does not depend on the overlaid packages. Full suites include the
generic dependency audits, OpenType adapter regressions and current tytui
document/replay controls. The E2 suite has ten passing top-level
tests and one explicitly skipped admission probe, as explained above.
Generator syntax, formatting of generated Go files and whitespace checks also
passed. The new seed regression failed before the fix and passed afterward.
The review's separate baseline diagnostics reproduced the preparation failures;
they are not counted as passing checks. The stalling cases were not rerun as
part of this seed fix.

## Decision supported by E2

The tested ordinary-break continuation is viable with a small set of owner
seams: an immutable owner-bound checkpoint of a successful path, a stable width
schedule retaining original line numbers, and a continuation entry into the
existing breaker/freezing/assembly pipeline. These experimental names are not a
proposed public API. The preparation rejection and progress counterexamples
prevent a general bidi or end-to-end cancellation claim.

E2 identified full binding, materialization, assembly and checkpoint creation as
the costs to measure before promising inexpensive continuation. The subsequent
[E3 report](E3_RESULTS.md) now measures them. True work
proportional to suffix size would need upstream range binding/materialization
and a safe way to combine independently owned completed line geometry. Do not
weaken VList ownership or prematurely extend parshape to hide that work. E1
remains the independent equal-width path. The discretionary extension remains
deferred.
