# E1: equal-width retained-line splitting

Executed 2026-11-18 with Go 1.25.6, darwin/arm64. E1 passes its bounded correctness
gate using existing fragment APIs. No upstream hook or public API change was
needed for E1. Bounded E2 continuation now has separate [evidence and limits](E2_RESULTS.md).
Subsequent E3 cost comparisons are recorded in [E3_RESULTS.md](E3_RESULTS.md).

## Observed behavior

The synthetic fixture is prepared once and laid out once. Subsequent operations
use only its immutable VList. Both first-fit and Knuth–Plass initial layouts pass
the following three-line example (40 bp target width, 10 bp full line-height).
The counts include every original line exactly once across A, B and the tail.

| A height / B height (bp) | Lines A / B / tail | Outcome | Boundary lines examined | Interior splits |
| --- | --- | --- | --- | --- |
| 10 / 20 | 1 / 2 / 0 | complete | 4 | 1 |
| 15 / 25 | 1 / 2 / 0 | complete | 4 | 1 |
| 30 / 0 | 3 / 0 / 0 | complete | 3 | 0 |
| 0 / 30 | 0 / 3 / 0 | complete | 4 | 0 |
| 10 / 0 | 1 / 0 / 2 | more frames | 3 | 1 |
| 10 / 10 | 1 / 1 / 1 | more frames | 4 | 2 |
| 9 / 9 | 0 / 0 / 3 | no progress | 2 | 0 |
| next float below 10 / 30 | 0 / 3 / 0 | complete | 4 | 0 |

The boundary scan may examine a rejected line again for B; its counter is not a
count of unique source lines. There are at most two interior splits. These are
operation counts, not allocation or timing measurements. Whole-fragment
publication and delegated splitting also validate lines/knots and copy geometry;
their work is not included in the boundary counter. Total work remains linear
in the retained paragraph records, even for a short accepted prefix. There is no
claim of constant-time splitting or measured speedup.

Additional passing cases and assertions:

- Natural wrapping with both breakers and fractional 7.31 bp line-height, with
  two successive splits. A retains exact placements; B and the tail follow the
  existing ordered rebasing operations. Arithmetic tolerances are unchanged.
- Empty visible output retains its frozen diagnostic owner. Three zero-height
  lines all fit a zero-height A without losing their distinct identities.
- Mixed sizes and surviving spaces produce a 20 bp line; fixed atomic content
  produces a 32 bp line. Both keep full heights after splitting. Blank lines,
  negative leading, signed atomic advances and zero-sized atomics are preserved.
- LTR, RTL and mixed Hebrew/Latin/numbers with punctuation and a textual isolate
  across the cut retain the same horizontal records and physical knot order.
  No new bidi analysis or fresh paragraph interpretation occurs.
- Every fragment shares the original frozen owner, preserving glyph records,
  byte/scalar anchors, fit records and source associations. Line/atomic indexes
  become local; original line identities and knot coordinates remain unchanged.
- A zero-width frame reports horizontal overfull content separately. Vertical
  fit never silently becomes a claim of horizontal or ink containment.
- Changing any compatibility-key component for either frame refuses reuse.
  A falsely declared source width is also detected against frozen target widths,
  including suppressed-empty diagnostic lines with both breakers. A valid retry
  preserves the diagnostic owner without creating placements or overflow reports.
  Invalid heights, absent sources and nil context fail without a result.
- Both skip triples reject infinite/out-of-domain minima, Infinity maxima and
  inconsistent bounds. Zero, signed finite bounds and all three fill maxima remain
  valid. The tests compare the domain with khipu's own geometry validator; they
  do not claim to verify the truth of a caller's input attestation.
- Entry, mid-operation and final cancellation publish no partial result.
  Late rebasing/domain failures leave the original usable. Constructor-accepted
  arithmetic discrepancies remain accepted within the domain; derived depth at
  or beyond `dimen.Fil` is refused without tightening equality tolerances.
- Eight concurrent readers each repeat the split ten times. Mutating detached
  accessor slices/envelopes does not affect the source or earlier results. Frozen
  glyph content remains usable after the source VList variable is released and
  garbage collection runs.

## Zero additional services and breaker work

After initial layout the fixture seals its service wrappers. Any subsequent
style resolution, shaper creation, shaping, face acceptance, font metrics,
font scaling or scale conversion panics. The normal cases log the actual counter
delta: `[0 0 0 0 0 0 0]` for all seven categories. Failure and ownership cases
also run against sealed services.

Itemization, horizontal binding and the breakers have no injectable call counter
in the existing API. Their exclusion is verified by `TestE1GeometryOnlyCallBoundary`.
Following review, the audit checks every E1 implementation file and initializer,
using resolved Go function/method identities. It follows approved paragraph,
layout and frozen-accessor bodies, recursively checking private helpers across
files. Only the listed standard operations and context.Err are opaque leaves.
Unresolved callbacks fail the audit; locally bound, unmodified closure literals
are inspected. Function aliases and method values are checked at their references.

In-memory mutation regressions reject an extra-file breaker helper, a helper
introduced under paragraph.Fragment, a paragraph.Run alias, a Prepared.Layout
method value and an unknown callback. A pure helper in another file still passes.
Parse/type failures cannot count as successful detection of a forbidden call.
These tests close the reviewed gap in the original file/name-only audit.

The current path reaches only geometry publication and immutable frozen accessors,
never preparation, binding or a breaker. This is structural verification of zero
calls, not a fabricated runtime breaker counter. The harness starts from prepared
styled input, so it does not itself exercise a StyleMapper itemization pass.

## Limits and next decision

E1 supported proceeding to the separately authorized E2 experiment, now recorded
in [E2_RESULTS.md](E2_RESULTS.md). E1 itself establishes
that compatible equal-width flow can reuse completed lines without extending
parshape or reopening logical material. It does not establish different-width
continuation semantics, real-font shaping quality, discretionary replacement
carry or production performance.

The reuse key is an explicit caller attestation. Only target width can be checked
against the frozen layout; revisions for styles, selected fonts, line-height,
atomic geometry and paragraph policy must be truthful and kept current by the
owner. No automatic input fingerprint or production dispatch is provided.

All line boundaries are eligible under this bounded policy. There are no
widow/orphan rules, elastic packing, third-frame scheduling, renderer integration
or replay changes. An oversized next line can leave a frame unused; the caller
receives a retained tail or no-progress outcome instead of silent loss or a loop.

Surrounding margins/source/paint remain application-owned and are never passed
to this helper. Existing tytui tests remain passing, including margin retention
and retained proofs; no new application flow integration has been exercised.
Such integration needs its own ownership and scheduling tests before production
use. In particular, E1 has not implemented fragment rendering or restacking.

## Checks run

Re-run after all three E1 review fixes on 2026-11-18. Focused evidence:
`go test ./internal/flowexperiment -count=1 -v` passed, including the audit mutation
tests and both guard regressions. The following commands all passed in both
typography and sibling tytui:

```sh
go test ./... -count=1
go test -race ./... -count=1
go build ./...
go vet ./...
```

These include typography's generic dependency/package-arm audits and OpenType
adapter regressions, plus tytui's document, tycli and replay-v5/control suites.
No fixtures were regenerated. No shared khipu or CSS boundary was changed, so no
separate khipu/css-box-layout suite was run. E1 needs no build tag or overlay.
