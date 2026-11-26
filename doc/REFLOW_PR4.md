# Reflow PR 4: independent paragraph trials

Extension implemented 2026-11-26 (TYPE-SHIP-01): `Trial.FitPrefixes(height)` now
describes all fitting whole-line suffix prefixes, and `PrefixFit.Select` freezes
the chosen fragment/checkpoint/atomic association. Authoritative line geometry,
zero-height progress and explicit NoProgress are preserved. Logical byte/knot
positions and detached rendered-cluster coverage support consumer event mapping
without DOM or page dependencies. See the
[usage contract](../glyphing/paragraph/README.md#choose-a-fitting-paragraph-prefix).
TYPE-SHIP-02 (2026-11-26) extends checkpoints and prefix selection to
ExplicitHyphenation through the same API. Selected replacements and original
source splits survive equal/changed-width resumes; suffix alternatives can be
shaped lazily without re-preparing the base paragraph. ReplacementCoverage uses
one indivisible original partition per rendered replacement half, never its
replacement-local cluster offsets. Empty/silent halves retain logical carry
without rendered coverage. Provider/shaper errors preserve earlier choices and
checkpoints for retry. Automatic continuation and seeded KP remain unsupported;
StartFlow retains its separate NoHyphenation restriction. The application fitting
bridge remains separate work. Original milestone evidence below remains dated;
it is not a claim of page-runtime completion.

Implemented 2026-11-20 for [REFLOW_PLAN.md](REFLOW_PLAN.md). This completes the
paragraph trial API of Stage 3. It does not implement a page builder, region
registers, frame scheduling/scoring, cross-region coordination or page-level
commit/rollback. Those designs remain undecided. Later stages, concurrent
execution and the upstream bidi fix remain deferred.

## Client contract

`Prepared.Trial(TrialRequest)` binds fresh atomic advances and searches the
retained immutable Khipu using its shared Discovery. It returns an immutable
`Trial`: a `khipu.Tikray` paired with the copied inputs required for CSS assembly.
The wrapper does not introduce a second representation of breaking decisions.
`Prepared.Layout` remains immediate trial creation followed by assembly.

| Entry point | Contract |
| --- | --- |
| `TrialRequest.Layout` | Existing width, breaker, CSS line-height, horizontal skips and complete atomic-box table. Each trial can supply independent values. |
| `TrialRequest.Shape` | Optional `khipu.LineShape`, consumed during search with one-based paragraph line numbers. Captured plan geometry replaces this callback afterwards. The rectangular width field must still be in its valid domain. |
| `TrialRequest.FirstFit` / `KnuthPlass` | Optional parameters for the selected breaker only; nil selects its defaults. Resolved policy is retained per line in the Tikray. |
| `Trial.Plan()` | Immutable Tikray, independently usable with `khipu.Freeze`. Nil for a nil/zero Trial. |
| `Trial.Inputs()` | Detached layout/assembly snapshot, explicit side skips and atomic boxes in material order. Custom/continued per-line widths and skips are authoritative in `Plan().Lines()`, not the suffix request snapshot. Zero for a nil/zero Trial. |
| `Trial.Assemble()` | Full freeze and CSS assembly with retained inputs. No new search, discovery, shaping or CheckFace; stable metric/resolver services are still used. Repeated calls return independent output. |
| `Trial.Checkpoint(n)` | Certifies zero through all complete lines, retaining their decisions and assembly compatibility constraints. NoHyphenation and ExplicitHyphenation. |
| `TrialRequest.Prefix` | Continues the same preparation incarnation from that certificate, retaining the prefix exactly and searching only the first-fit suffix. The prefix may have been produced by KP. |

Input validation still precedes publication. Invalid trials/counts fail at
StageInput; checkpoint incompatibility fails at StageBind; search/freezing/assembly
failures retain their respective stages and underlying causes. Every failure
returns no partial candidate/result and leaves existing trials reusable.
Operations are synchronous and context-free; only internal streaming coordination
uses cancellation and producer cleanup. There is no new concurrency promise.

For examples, see the [paragraph user guide](../glyphing/paragraph/README.md#retain-independent-paragraph-trials).

## Prefix compatibility and atomic realizations

The khipu checkpoint verifies incarnation and consumed advances. Typography also
checks the **complete consumed AtomicBox**: border width/height, all margins and
alignment. Equal advance alone does not certify the same baseline, atomic origin
or line height. Unconsumed atomic boxes may change. Atomic interiors and their
render identity remain application-owned and must be kept with the matching
trial; typography accepts already resolved geometry and does not lay them out.

A nonempty checkpoint conservatively requires equality of the entire LineLayout,
including overrides confined to suffix text. This prevents prefix CSS geometry
drift without introducing a partial-style compatibility protocol. A zero-line
checkpoint locks no line-height or atomic geometry. All checkpoints still require
the same incarnation. Changed suffix widths, skips and first-fit settings are
allowed; accepted decisions and their policy attribution remain unchanged.
Automatic continuation and seeded KP remain unsupported.

## Ownership, lifecycle and unchanged flow

Trial owns copied layout envelopes, line-height spans, atomic values, bidi runs
and its immutable plan. Source/styles/font services obey the existing stable
borrowed-resource contract. Only resolver/defaults are retained for assembly;
shaper factories, hyphenators, discovery and shape callbacks are not retained
through the assembly configuration.

Trial does not retain Prepared or Discovery. TrialCheckpoint keeps material,
copied accepted decisions, line-height and consumed atomic boxes, without an
originating trial/preparation/cache or checkpoint-history chain. Published frozen
results and fragments retain none of these working handles. Keeping a trial
alive intentionally keeps its material and selected replacements alive.

Process paragraphs in turn. Keep preparation while further searches or intrinsic
measurements are possible. Retain only useful candidates/checkpoints; dropping
a candidate is sufficient to discard it. At paragraph completion or abandonment,
release preparation and checkpoints; keep frozen fragments. A deliberately
retained Trial can still assemble after its preparation has been collected.
This is paragraph ownership, not a page transaction protocol.

Existing `StartFlow`/`Continuation.Fit` orchestration is preserved. Equal-width
Fit still uses only frozen geometry and never inspects a plan, binds, checkpoints,
searches or calls services. Changed widths still freeze/assemble the complete
plan before publishing the suffix fragment. `Prepared.Trial` requests a search;
it does not infer equal-width reuse. Uniform-width trial results can use the
existing `Fragment`/`FitFragment` path with the required compatibility attestations.

## Manual continuation verification

[Manual prefix tests](../glyphing/paragraph/prefix_manual_test.go) cover every
line cut, exact heights, source splits, rendered replacement ownership, UTF-8/bidi,
repeated width changes, suffix atomic geometry and full-prefix completion.
Injected provider/shaper and unsupported-policy errors return no trial; completed
discoveries stay reusable. Selection calls no services.
[Lifetime tests](../glyphing/paragraph/prefix_manual_lifetime_test.go) and the
[real-font proof](../integration/prefix_manual_render_test.go) retain fragments
after releasing preparation, discovery, trials and checkpoints. The latter
renders the selected hyphen exactly once after changed-width completion.
Application-owned atomic interiors are associated by stable occurrence ID and
validated accepted geometry; their rendering identity stays with the adapter.

## Verification and downstream impact

The following passed in **each of typography, khipu and tytui**:

```sh
go test ./... -count=1
go test -race ./... -count=1
go build ./...
go vet ./...
```

New regressions cover candidate A after successful, rejected and failed B;
discovery failure/retry and cached selected replacements; copied configuration,
shape and atomic inputs; nil/zero handles; checkpoint incarnation/admission;
same-advance changes to consumed atomic geometry; suffix-only changes; A/B/C
width/skip/policy history; zero/full prefixes; absence of service calls during
NoHyphenation searches; and collection of preparation/discovery while a trial
survives, trials while checkpoints survive, and all working handles while frozen
output survives.

Existing suites cover source/bidi/feature domains, CSS geometry, failure recovery,
real-font adapters, nested atomic scheduling, retained proofs, replay v5 and
historical-version rejection. Generic dependency audits and the transitive
equal-width/geometry-only call audits pass. The initial sandboxed paragraph test
could not run the dependency audits because of Go-cache permissions; rerunning
with the required cache access passed. Formatting and `git diff --check` pass.

**No downstream API breakage is expected or observed.** Tytui/tycli needed no
source changes. CSS core contracts are unchanged; its behavior is exercised by
the tytui document tests, without a separate CSS-module check in this PR. No
replay/proof fixture or historical measurement evidence was regenerated. No new
cost measurements or performance release claim is made.

## Reproduction identities

Validation used existing sibling replacements. The typography checkout includes
this uncommitted PR 4 implementation, regressions and documentation; HEAD alone
does not include those changes.

| Checkout | HEAD | Local state during validation |
| --- | --- | --- |
| typography | `b83e58d60e31ce3fdfc79d9dd9ccfeedbc37ba6f` | This PR's code, tests and documentation |
| khipu | `43b6fa9c3c027faa5ba084a0d5ea340f3fc1f226` | Clean |
| tytui | `24deb63aedca92c8233acaf88f2f4deae279f2ff` | Clean |
| cords | `58b65a5bc6e7dfe74c338f8f197276c5c9c94867` | Existing go.mod sibling schuko replacement |
| opentype | `a351aa0e7e7626ae3427f60498b7afa648d4c173` | Clean |
| uax | `c92d5c4593263cec69aeffd7ecc0257fe77c2d8f` | Clean |
| schuko | `287adb3427da6b042e3224be3f6ef52b68ff0551` | Clean |
| css-box-layout | `c687d89896f8693aa9fc0d7360b106f1a2b8b892` | Existing doc/Architecture-Interfaces.md edit |
| fontfind | `1b6583bd29f20139f9a57112c11faa52eb8e5757` | Clean |
| w3c-dom | `c326be477fc655c581803d626a8fc922c0b9aafa` | Clean |
| tree | `167ff46c52dbede92dbc905be30704f76e7af3dc` | Clean |

Local AGENTS guidance was updated. No commits or pushes were made.
