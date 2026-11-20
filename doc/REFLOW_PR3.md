# Reflow PR 3: application migration baseline

Completed 2026-11-20 for [REFLOW_PLAN.md](REFLOW_PLAN.md). PRs 1–3 now pass the
complete khipu/typography/tytui migration gate. **No downstream API breakage from
these PRs remains.** PR 4 and all later work remain deferred; this does not add
public paragraph trials, region registers, a page solver or concurrent execution.

## Changes

The production tytui/tycli consumers already call typography's unchanged public
paragraph APIs. The remaining direct khipu client was the regression helper in
`tytui/pipeline/builder_test.go`. It now receives material and Discovery from
EncodeParagraph, binds the text-only material, passes a BreakRequest to first-fit
and freezes the returned Tikray with package-level Freeze. All original byte,
line-count and retained font-size assertions are preserved.

Application guidance now describes the immutable engine contracts, invocation-local
preparation, captured checkpoint geometry and the existing client entry points.
Typography's plan and architecture status link to this baseline. The PR 2 record
retains the earlier failed downstream checks as historical evidence.

No production layout, rendering, font-selection or replay implementation needed
changes. No module dependencies, fixture bytes, proof controls, replay versions
or historical benchmark results were changed. No new cost measurement or
performance release claim is made.

## Preserved behavior and verification

The following commands passed in **each** of khipu, typography and tytui:

```sh
go test ./... -count=1
go test -race ./... -count=1
go build ./...
go vet ./...
```

The existing suites retain their assertions and cover:

- Immutable material/bindings, independent Tikrays, discovery caching,
  captured side skips and checkpoint compatibility in khipu.
- Typography generic dependency and transitive geometry-only call audits,
  synthetic and real-font adapter regressions, A/B/C width history, distinct KP
  prefixes, source/feature domains, atomic placement and trial/failure isolation.
- GC of preparation, material, Discovery and Tikrays while frozen fragments
  remain alive, and release of application preparation/tokens on completion,
  frame exhaustion and failure.
- Nested fixed/auto atomic scheduling and CSS geometry, retained proofs without
  remeasurement, fragment coordinate transforms and surrounding-margin retention.
- Zero additional services/breaking on compatible equal-width reuse; changed
  widths retain paragraph styles/bidi and perform no base shaping or CheckFace.
- Tycli frame dispatch and preservation of prior output/zero stdout on failures.
- Replay-v5 offline load/resave and tampering controls, current glyph/placement
  evidence, and rejection of historical versions without regenerating fixtures.

The focused pipeline suite also passed with the staged helper before applying it.
The CSS core regression command `go test ./layout -count=1` passed in sibling
css-box-layout; its contracts and production code were not changed. Formatting
and whitespace checks passed for the modified files.

The upstream bidi limitation, NoHyphenation restriction on continuation, full
freezing/CSS assembly for changed widths and serialized application operations
remain deliberate limits. A passing migration baseline does not broaden them.

## Reproduction identities

Validation used the modules' existing sibling replacements. Git revisions alone
do not contain the uncommitted changes explicitly listed here.

| Checkout | HEAD | Local state during validation |
| --- | --- | --- |
| typography | `4be6d883d1ee962245c8d6ccb630d707cfff3e86` | PR 2 implementation committed; this PR adds documentation/status updates only |
| khipu | `43b6fa9c3c027faa5ba084a0d5ea340f3fc1f226` | Clean |
| tytui | `e840259f9f63d6a1045d5158db709b18600aa5de` | This PR's migrated builder test and documentation updates |
| cords | `58b65a5bc6e7dfe74c338f8f197276c5c9c94867` | Existing go.mod change adds sibling schuko replacement |
| opentype | `a351aa0e7e7626ae3427f60498b7afa648d4c173` | Clean |
| uax | `c92d5c4593263cec69aeffd7ecc0257fe77c2d8f` | Clean |
| schuko | `287adb3427da6b042e3224be3f6ef52b68ff0551` | Clean |
| css-box-layout | `c687d89896f8693aa9fc0d7360b106f1a2b8b892` | Existing documentation edit in doc/Architecture-Interfaces.md |
| fontfind | `1b6583bd29f20139f9a57112c11faa52eb8e5757` | Clean |
| w3c-dom | `c326be477fc655c581803d626a8fc922c0b9aafa` | Clean |
| tree | `167ff46c52dbede92dbc905be30704f76e7af3dc` | Clean |

Local AGENTS guidance was updated alongside the repository documentation. No
commits or pushes were made by this task.
