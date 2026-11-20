# Reflow PR 2: typography adoption

Historical PR 2 completion snapshot. [PR 3](REFLOW_PR3.md) subsequently resolved
the downstream failures recorded below and established the passing cross-module
baseline; the original outcomes and checkout identities are retained here.

Implemented 2026-11-20. This is PR 2 of [REFLOW_PLAN.md](REFLOW_PLAN.md), not a
GitHub PR identifier. PR 3 application migration and the later public trial API,
region registers, page solver and concurrency work remain deferred.

## Implementation

- `Run` and `Prepare` construct immutable Khipu material plus matching Discovery.
  Prepared retains those handles, source/configuration and bidi data, without
  retaining the construction executor.
- Each layout binds copied atomic advances, searches an independent Tikray and
  freezes its captured geometry/alternatives. CSS assembly and atomic placement
  still use the copied request inputs and retained source styles. Public paragraph
  signatures and staged error handling are unchanged.
- Intrinsic measurement uses the same Khipu/Discovery pair; successful lazy
  replacement batches remain shared, with failures retryable and selected
  decisions confined to individual trials.
- A continuation retains its latest complete Tikray. Changed widths create a
  checkpoint lazily and search only the suffix. Accepted widths/skips come from
  the checkpoint, not a reconstructed shape callback. Full freezing and CSS
  assembly remain deliberate; no suffix-proportional cost claim is made.
- Equal-width fitting retains its geometry-only implementation and transitive
  call audit. It does no checkpointing, binding, discovery, breaking or assembly.
  Tokens share stable state without chaining prior plans or tokens.
- Preparation, Discovery, material and plans remain live only while the caller
  retains working handles. Completion returns no continuation. Frozen results
  retain their source/glyph resources independently, and the caller releases all
  earlier tokens and preparation before advancing to the next paragraph.

Direct khipu adapter tests and the test-only P6 phase helper use the new APIs.
The helper's correctness test still matches production continuation. Historical
timings, archived experiment sources and replay/proof controls were not changed;
no production cost measurements were rerun or new performance claims made.

## Regression coverage

The existing suite preserves exact CSS geometry, signed/zero atomic dimensions,
physical traversal, RTL and source/feature domains, intrinsic discovery retry,
failure recovery, A/B/C width history, a distinct KP prefix, immutable earlier
fragments and zero service/breaker work on equal-width reuse.

New tests additionally retain a Tikray and assemble it again after a later layout
succeeds or is rejected. Weak-reference GC checks prove both initial and continued
Tikrays, Khipu, Discovery and Prepared can be collected while frozen fragments
from changed-width and equal-width completion remain alive and source-contiguous.
The existing lifetime test also reads retained glyphs and source text after GC.

## Validation and dependency identity

Typography passed `go test ./... -count=1`, `go test -race ./... -count=1`,
`go build ./...`, `go vet ./...` and `go mod tidy -diff`. These suites include
generic dependency audits, geometry-only call audits, adapter regressions and
cross-branch rendering tests. The P6 analyzer regression suite also passed.
Graphviz regenerated the architecture SVG; its rendering was inspected.

The sibling checkouts used by local replacements were:

| Module | Revision / local state |
| --- | --- |
| typography before PR 2 | `fcb347d225742dc35cbb0aea3b42b735badd13df`; validation includes this uncommitted PR 2 change |
| khipu | `9a0d506d07477f3cf60611703f2c29cd26edc76d`, clean; includes the NewTikray side-skip fix |
| cords | `58b65a5bc6e7dfe74c338f8f197276c5c9c94867`; existing `go.mod` modification adds the sibling schuko replacement |
| opentype | `a351aa0e7e7626ae3427f60498b7afa648d4c173`, clean |
| uax | `c92d5c4593263cec69aeffd7ecc0257fe77c2d8f`, clean |
| schuko | `287adb3427da6b042e3224be3f6ef52b68ff0551`, clean |
| tytui | `e840259f9f63d6a1045d5158db709b18600aa5de`, clean |

Khipu ordinary/race suites, build and vet were also run against the upstream
checkout. CSS core contracts were not changed.

## Remaining downstream breakage: PR 3

Tytui `go build ./...` passes. Its ordinary and race runs pass the root,
documentlayout, fontselection, htmlinput, paragraphtext, replay and tycli packages,
including their existing output controls. Full ordinary/race suites and vet are
**blocked**, not passing, because `pipeline/builder_test.go` still uses the former
khipu API:

- Line 43 expects two `EncodeParagraph` results instead of material/discovery/error.
- Line 48 passes material and separate geometry to `firstfit.BreakParagraph`.
- Line 52 calls the removed executor method `owner.Freeze`.

PR 3 must migrate that helper to binding, BreakRequest/Tikray and package-level
Freeze, retain its assertions and run the complete application verification gate.
No tytui source or fixtures were changed here. The passing subsets do not establish
the fully migrated cross-module baseline reserved for PR 3.
