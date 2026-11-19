# Region registers and speculative reflow

Design proposal, 2026-11-19. **Not implemented.** This plan establishes the
ownership and decision boundaries needed before general page layout and page
breaking. It does not select a page-scoring algorithm, change existing APIs or
promise concurrent use of current paragraph handles. The reusable liquid
paragraph will be named **`Khipu`**, with immutable SoA base material. One trial's
retained paragraph decisions form an immutable **`Tikray`**. Defining these
replacements for today's `PreparedParagraph` and mutable `Khipu` state is Stage 1,
now specified in [REFLOW_KHIPU.md](REFLOW_KHIPU.md). Implementation remains deferred.

The central model is **immutable Khipu → independent Tikrays and page candidates →
adoption of a successor register collection**. Frame-set trials share stable
paragraph information but never share mutable selected decisions or tentative
register consumption.

See [paragraph architecture](PARAGRAPH_DESIGN.md) for the current pipeline,
[FLOW.md](FLOW.md) for implemented bounded continuation, and
[VLIST_CONTRACT.md](VLIST_CONTRACT.md) for geometry invariants that remain binding.

## Scenario and fixed assumptions

The typesetter maintains registers for regions such as body text, asides and
footnotes. Each register contains an ordered sequence of unfrozen paragraphs.
Register content is independent of particular frames and frame widths.

A page candidate supplies a frame set. Every frame belongs to a region; the
application supplies an explicit traversal order within each region. A page may
have multiple frames per region, with substantially different widths. Frames
come from templates or constraint solving. Rejected candidates can be replaced
by entirely different frame sets, starting from the same committed content state.

Source text, styles, fonts, shaping features and material-generation parameters
remain fixed during this iteration. Frames supply geometry, not new paragraph
styles. Break algorithms and permitted trial selection parameters may vary.
The definition of satisfactory filling belongs to the page builder: exhausted
frames, incomplete content, overflow and poor quality need not be execution errors.

Register data structures belong to `css-box-layout` and follow its functional
paradigm: the client owns an immutable version of the collection of all registers.
Register access is concurrency-safe through functional state threading. Concurrent
trial execution is a separate goal; initially serialize execution where current
collaborators require it. Synchronization of page decisions remains undecided.

## Current baseline and required changes

| Current behavior | Implication for this design |
| --- | --- |
| `PreparedParagraph` already contains compiled base knots and shaping records; atomic slots are unbound. | It is the closest existing representation to reusable material, not completed layout output. |
| `BindParagraph` copies knots into an isolated `Khipu`, supplies advances and shares immutable shaping records. | Fresh binding is an isolation strategy; it performs no shaping or discretionary discovery. It need not be the permanent strategy for every trial. |
| `Khipu` holds both reusable discoveries and the latest `SelectedDiscretionaries`. | Breakpoints alone do not preserve a trial. Selected decisions must become a self-contained result usable by freezing. |
| `Continuation` is immutable and earlier tokens remain reusable; equal-width fitting uses retained geometry. | This already provides a bounded form of branching and the fast path to preserve. |
| Changed-width continuation uses first-fit, fixes other layout inputs and fully binds/freezes/assembles. | Arbitrary breaker changes and varying atomic realizations require additional contracts. No suffix-proportional cost is promised. |
| `BreakCheckpoint` supports only `NoHyphenation`. | General continuation must represent replacement carry and any required policy history, not just ordinary knot endpoints. |
| `documentlayout.LayoutFlow` supports one paragraph with fixed-width opaque children and releases working state on every exit. | Its published frozen remainder is not a resumable register cursor. A general multi-register trial session is new work. |
| Prepared owners, mappers and shared services currently require serialized operations. | Moving selections alone does not make trials concurrent-safe. |

Source contracts: [binding](../../khipu/paragraph_input.go),
[Khipu](../../khipu/khipu.go), [freezing](../../khipu/frozen.go),
[checkpoints](../../khipu/checkpoint.go),
[continuation](../glyphing/paragraph/flow.go), and
[application flow](../../tytui/documentlayout/flow.go).

## Region registers and progress

A register has stable identity and an ordered sequence of paragraph entries.
Its flow material is liquid: it can be tried in any destination frame without
becoming attached to that frame or its CSS ancestry. The authored surrounding
box can influence initial style mapping; after style mapping and glyphing, no
new CSS cascade, inheritance or frame styles alter the paragraph incarnation.

Retain **content layout properties**, not an attachment to the original
surrounding box. Those properties accompany the material at the appropriate
CSS/typography layer and are applied during each trial. Applying a retained
layout rule under new geometric constraints can yield different geometry without
restyling the text. No selected frame, width or final atomic realization belongs
permanently to the Khipu. Nested atomics are laid out for each new trial.

Source and paint associations remain outside generic horizontal material where
appropriate. A body reference's association with a footnote is a content
relationship, not a destination-frame attachment. Such cross-region relationships
can live in an application-owned association table; this plan does not require
them inside Khipu or inside every register entry.

The client threads a version of the **whole register collection** through layout.
Content and progress are logically separate, but both can be represented in that
immutable collection. A trial takes collection S and derives a successor SA; it
never changes S, destructively pops entries or updates a shared current paragraph.
Alternative trials derive their own successors from the same S. Construction of
entries and preparation may be lazy, but must respect immutable published values;
no unprepared placeholder may later be mutated in place.

A position within a paragraph identifies a legal logical continuation. It is
not a frozen visual line index or an arbitrary source-byte cut. Future hyphenating
continuation must capture selected replacement carry and preserve the original
paragraph-wide bidi/shaping context. If a selection policy needs previous-line
history, that requirement must be explicit and validated. A cursor need not
retain an entire algorithm's search graph, and changing algorithms need not resume
paragraph-global optimization.

```text
Immutable register collection S (all regions and positions)
                       |
              +--------+--------+
              |                 |
        frame-set trial A  frame-set trial B
              |                 |
        candidate A + SA   candidate B + SB
              |
        client accepts A: adopts SA and A's corresponding output
        S and SB are unchanged; no state restoration is needed
```

## Reusable material, bindings and trial plans

The following are logical responsibilities. Stage 1 defines proposed Go contracts
in [the Khipu specification](REFLOW_KHIPU.md); these are not implemented APIs.
They do not require separate objects for every row. The base Khipu uses SoA.

| Responsibility | Contents | Ownership |
| --- | --- | --- |
| New `Khipu` | SoA base knots, glyphs/clusters, source associations, resolved run contexts, bidi data, unbound atomic identities and fixed incarnation policy | Immutable liquid paragraph shared across trials |
| Discovery facility | Complete immutable discretionary alternatives and successful empty findings | Scoped to the incarnation, outside Khipu's immutable base and selected decisions |
| Atomic realization | Resolved box geometry and matching retained interior output | Produced for each new trial; compatible caching is deferred optimization |
| Atomic binding | Khipu incarnation identity and concrete advances for its atomic slots | Immutable trial input; no mutation of Khipu or requirement for another complete knot representation |
| Workspace | Breaker search arrays, provisional choices, accumulators and scratch buffers | Exclusive to an active attempt; recyclable afterward |
| `Tikray` (paragraph trial plan) | Selected endpoints/variants, binding and geometry dependencies, policy and diagnostics; certifies continuation checkpoints | Independently retained immutable result |
| Page candidate | Tikrays, frame placements, successor register collection and cross-region obligations | Independently retained immutable result of a complete candidate evaluation |

Tikray holds the decisions that vary **between** attempts. A private breaker
workspace computes those decisions and publishes a Tikray through `NewTikray`;
the published object is never edited for the next attempt. Shared memoized
discoveries remain outside it. `Freeze` consumes that Tikray's own decisions,
and `Checkpoint` derives a certified prefix from it. Neither operation commits
the page candidate or changes a register collection.

Clients inspect retained results through value-returning `Tikray.Diagnostics()`
and `Tikray.PolicyForLine(index)`. Policy indexes are zero-based in the complete
paragraph plan, so a locked KP prefix and a first-fit suffix remain distinguishable.
Invalid indexes or nil/zero Tikrays yield `PolicyRecord{}, false`; nil/zero
Tikrays yield zero diagnostics. Editing either returned record cannot change the
Tikray. Diagnostics preserve availability flags and describe only the suffix
search for a continued plan, not a newly optimized whole-paragraph score. See
[Tikray inspection contracts](REFLOW_KHIPU.md#4-break-requests-independent-plans-and-captured-geometry).

### Incarnation identity and the new Khipu

A Khipu represents one paragraph incarnation. Source, mapped styles, script,
language, fonts, shaping features, spacing capacities and discovery configuration
are guaranteed stable for its lifetime. A change to any of these produces a new
incarnation. Identity is established at construction, not repeatedly inferred by
hashing or comparing every property during trials. A word string is not identity.

The new Khipu consolidates the reusable roles currently divided between
`PreparedParagraph` and the base-material portion of `Khipu`. Preparation already
constructs knots; the new constructor should publish that material directly in
its immutable SoA representation. Remove writable public base planes and keep
trial geometry, selected alternatives and search state out of the published
Khipu. Typography can retain an orchestration wrapper, but must not introduce
another competing copy of the horizontal material.

Identify material by incarnation rather than by a particular mutable execution
worker. Binding, Tikray and checkpoint compatibility must retain their additional
geometry/decision checks: one incarnation can participate in many incompatible
trials. [Stage 1](REFLOW_KHIPU.md) specifies how these identities replace existing
owner checks without allowing foreign or mismatched decisions to be frozen.

Stage 1 chooses a separate synchronized incarnation-scoped discovery facility
supplied to execution, with cancellable serialized misses and immutable completed
batches. It is not a mutable candidate map in Khipu or the register collection.
Tikrays retain selected replacement records independently of the facility. See
[discovery ownership](REFLOW_KHIPU.md#3-discovery-caching-a-concrete-ownership-choice).

A Tikray must keep its endpoints and discretionary selections
together, identify its material and binding, and retain the selected replacement
records. It must capture stable widths/skips and relevant policy settings rather
than borrow a mutable frame solver's state. A later trial must not replace its
choices or invalidate its arena references. Freezing must consume the Tikray's own
decisions; the new Khipu has no latest-selected-decisions field.

A page candidate additionally records frame identities, region assignment,
traversal, origins, successor register collection and the paragraph fragments
placed there. Fragments from different frozen owners remain separate; do not
fabricate a single-owner VList.
All published coordinates retain the existing finite-domain, full-line-height
and source-coordinate contracts. Paint and retained content margins remain
application-owned.

The decisive invariant is: **after trial B succeeds, fails or is cancelled,
trial A remains usable without restoring mutable state by hand.**

## Atomic realizations

An atomic advance is associated with a geometric realization, not inherently
with one frame width. A fixed-size image can have the same advance across several
widths. An auto-sized inline block may acquire a different width, interior layout
and height under different constraints even though its styles are unchanged.

Keep atomic source/content identity in the immutable Khipu; an unbound slot is
not a zero-width object. For every new trial, lay out atomics, including nested
interiors, with the retained layout properties and that trial's constraints.
This does not reopen CSS style mapping. Resolve geometry using the
application's/CSS layout machinery. The parent binding receives `left margin + border-box width + right margin`; vertical assembly also
needs border height, vertical margins and alignment. The application retains the
corresponding interior output. Equal advances alone do not prove equivalent
vertical geometry or rendered content.

A binding associates exactly the required atomic IDs of an incarnation with
concrete advances. It leaves Khipu unchanged and must remain paired with the
realizations used for vertical assembly and rendering. Its identity is not just
the frame width.

Caching a compatible realization is a possible later optimization, not the
baseline trial contract. It requires equivalence of content, retained layout
properties and all relevant constraints, including nested dependencies. Do not
key such reuse solely by outer frame width or transfer an interior whose
geometry no longer matches the trial.

The current paragraph engine accepts fixed boxes per layout and does not compute
CSS edges or interiors. Preserve that boundary. The general sizing/caching
contract is proposed here; current bounded frame flow still fixes atomics for
the entire invocation.

## Memoized hyphenation and changing breaker policy

Separate **discovery** (“these alternatives exist”) from **selection** (“this
attempt uses this alternative”). Rejected trials may leave useful successful
discoveries cached. They must leave no selected choices in shared material.

The incarnation guarantee supplies the identity of source, script/language,
font/features, shaping context and discovery configuration for the entire
paragraph lifetime. Scope memoization to that incarnation and the source
occurrence/alternative within it; no repeated deep identity comparison is needed.
A changed discovery configuration creates a new incarnation. Immutable shaped
replacements can be shared within the incarnation while their stable resources
remain alive.

Publish complete successful batches, including successful empty results. Do not
cache errors/cancellation as absence, expose partially populated batches or let
worker-local arena IDs escape. “Not explored by this breaker” is not a negative
discovery result. Trial selection policies must operate on the incarnation's
fixed candidate universe, possibly with explicit eligibility filtering. Changing
the discovery universe requires a new incarnation. Results must not depend on
which trial warms the cache first.

Distinguish provider IDs from published alternative IDs. Automatic provider zero
IDs are assigned unused positive IDs deterministically; explicit-marker input
IDs are ignored and expanded alternatives are numbered in source/material order.
Published IDs are unique and nonzero, while candidate order still governs ties.
See the [normalization contract](REFLOW_KHIPU.md#3-discovery-caching-a-concrete-ownership-choice).

Complete-batch publication does not require shaping before an intrinsic policy
rejection. On a cache miss, reject multiple explicit positions before provider
calls and multiple permitting variants before shaping/CheckFace, preserving
feature-validation precedence. These rejections publish no cache entry; later
layout may discover the complete batch. Cache hits apply admission to the stored
complete set without service calls. See the
[intrinsic admission sequence](REFLOW_KHIPU.md#6-intrinsic-measurement-and-typography-integration).

Distinguish two policy classes:

- **Material-generation policy:** source/style interpretation, shaping inputs,
  spacing capacities and discovery rules. These remain fixed for the material.
- **Trial selection policy:** breaker algorithm and explicitly supported scoring
  or eligibility choices over that material. These are captured by the Tikray.

Switching first-fit/Knuth–Plass need not rebuild base material. Changing an input
already compiled into knots or candidates requires a new incarnation under this
design. Current khipu space stretch/shrink settings and some discretionary
penalties are compiled values;
current preparation fixes `HyphenationMode`. This proposal does not silently
make those settings late-bound.

The proposed `BreakCheckpoint.SeedFor(ctx, binding)` gives sibling breakers a
validated, detached prefix, accepted-line count, resume position and post-break
carry metrics. It checks incarnation and consumed atomic advances without prefix
search, width queries or discovery/service calls. `NewTikray` validates against
the original checkpoint, so edits to a detached seed cannot change the locked
prefix. Metadata-only `HasExplicitDiscretionary` accessors preserve first-word
explicit rescue without eager automatic discovery. See
[continuation contracts](REFLOW_KHIPU.md#5-continuations-prefix-validation-and-replacement-carry).

Hyphenating checkpoints remain a prerequisite for claiming general hyphenating
region flow. Stage 1 specifies their carry and compatibility contracts; migration
keeps the existing NoHyphenation gate until the carry regressions pass. Retaining
a Khipu or a list of endpoints alone is not sufficient.

## Trial evaluation, freezing, commit and rollback

1. Take the client's current immutable register collection S.
2. Obtain a candidate frame set and its explicit per-region order.
3. Obtain immutable Khipus for the required content, lay out atomics for this
   trial, bind their advances and produce immutable Tikrays in private workspaces.
4. Assemble the candidate's geometry and derive its successor register collection.
   Record incomplete filling, no-progress, overflow and quality diagnostics as
   explicit outcomes.
5. Let the page builder evaluate feasibility and preference across regions.
6. Accept one candidate, or discard it and try another from the same snapshot.

Execution success means a valid candidate was computed, not that it was chosen.
Errors/cancellation publish no partial candidate and leave committed state and
earlier candidates usable. A rejected candidate can have entirely valid output.

**Freezing is not commitment.** A trial may freeze and assemble output while its
register retains unfrozen material. Rollback discards tentative decisions/output;
it never needs to unfreeze lines. Full freezing and assembly are acceptable
initially. Any later shortcut for evaluating heights must agree with authoritative
assembly, including mixed sizes, signed extents and atomic geometry.

Commit means the client adopts the selected successor collection and its matching
output. Rollback means retaining S and discarding unchosen successors/output; no
register version is modified. Which worker may make that page decision, and how
competing decisions are synchronized, remain open. Do not prescribe a mutable
commit manager, locks or compare-and-swap as part of the register data model.
The decision protocol must still prevent an obsolete candidate from being
mistaken for a successor of a different input state. Explicit backtracking can
revisit an earlier retained version under application policy.

This operation spans interacting regions. For example, a body reference moved to
the next page can change footnote eligibility on the current page. A page candidate
must keep reference/placement obligations and register progress consistent.
Typography does not decide those cross-region rules.

Acceptance inside one candidate is only tentative. Its continuation may lock a
prefix for further work in that branch, but a competing frame set restarts from
the earlier snapshot. Incarnation identity alone cannot certify a retained prefix
if its relevant atomic geometry or other layout inputs changed.

## Reuse and bounded lifetime

Preserve geometry-only equal-width fitting as the frequent path. Reuse requires
compatible material, entry continuation, atomic realization, line-height, skips
and other relevant geometry/policy—not width equality alone. Reusing a valid
candidate does not rerun a different requested optimization policy implicitly;
the page builder must explicitly permit retaining its existing break decisions.

Width changes can reuse Khipus and compatible bindings while searching new
breaks. Height/origin changes alone can reuse completed lines when their realized
inputs remain compatible. The zero-service/breaker-work guarantee concerns the
retained paragraph geometry operation, not a whole new frame-set trial that
relayouts nested atomics. Initial changed-width evaluation may still bind,
freeze and assemble the complete combined path; this
plan makes no suffix-proportional performance claim. Binding is not necessarily
the dominant cost; retain the distinction between phase measurements in
[FLOW_COSTS.md](FLOW_COSTS.md) and future trial-loop measurements.

Retain working material only within the active reconsideration window. A paragraph
being fully placed in a trial does **not** make it releasable while another
candidate, permitted backtracking step or unfinished continuation can revisit it.
Once consumption is committed and no live branch needs it, release obsolete
register collection versions as well as Khipus, discovery facilities, bindings,
Tikrays/checkpoints and workspaces as applicable. An old persistent collection can
keep an incarnation alive even after the client has adopted its successor. Frozen
output keeps stable source/font/shaping data independently. Avoid a permanent
whole-document material cache; construct immutable Khipus on demand without
mutating old register versions.

This is a proposed extension of [FLOW.md's lifecycle](FLOW.md#paragraph-lifecycle)
for speculative multi-paragraph scheduling: completion must be judged at the
accepted frontier, not at the end of an arbitrary trial. Existing sequential
flow and `LayoutFlow` release behavior remain unchanged until an explicit new
session/lifetime API is implemented. Retrying after deliberate release requires
fresh preparation.

## Future concurrency

The persistent register collection provides concurrency-safe access without
mutation. That does not establish concurrency safety of discovery, font services
or execution workspaces. Design branch isolation now; enable concurrent execution
only after validating these collaborators. Workers share read-only material and
completed immutable discoveries, but own independent workspaces and Tikrays.
Published Tikrays cannot point into recyclable scratch storage or mutable slices
owned by another trial.

Memoization serializes misses through the incarnation's cancellable discovery
gate. A cancelled producer publishes no batch; a waiter can retry with its own
context. There is no detached producer or cached cancellation. Intrinsic admission
uses the same gate/cache without publishing policy rejections. Immutable results,
normalized alternative IDs and deterministic tie-breaking keep output independent
of scheduling and cache warmth.

Current owners, `StyleMapper` operations and retained services are not granted
new concurrency promises by this plan. Audit each collaborator; serialize access
where required or provide independent instances with compatible identities.
Internal locking may limit speedup even when trials are logically concurrent.
The client's page-decision synchronization remains to be specified separately
from functional register access.

## Ownership across modules

| Owner | Responsibility |
| --- | --- |
| khipu | New immutable SoA Khipu and incarnation identity, atomic bindings, discovery facility, immutable Tikrays, freezing from Tikrays and legal logical checkpoints with detached continuation seeds |
| typography/glyphing/paragraph | Stable source/bidi context, layout orchestration, CSS assembly, paragraph trial/continuation envelopes and compatible geometry reuse |
| typography/layout and render | Immutable geometry and retained rendering; no solver state, reshaping during rendering or hidden register advancement |
| css-box-layout | Persistent functional registers and versions of the collection of all regions; CSS layout applies retained content properties under trial constraints |
| CSS/application integration | Atomic realization, nested sizing, source/paint/margin correspondence and unit conversion |
| Page builder / application | Threads register collection versions; owns frame proposals/order, cross-region associations/constraints, scoring, selection of successors, page-decision synchronization, backtracking and retention window |

Register ownership in `css-box-layout` is settled. The final package location
and public Go APIs of the page builder and its decision synchronization are undecided.
Do not move DOM, discovery, CSS types or concrete font types into generic khipu or
paragraph boundaries. No new serialization/replay contract is proposed. Existing
replay-v5 controls and immutable ownership remain unchanged.

## Module-by-module PR sequence

The following is the proposed implementation sequence; no implementation or PR
is created by this document. PR numbers below identify work units, not repository
PR identifiers. PR 1 implements Stage 2; PRs 2–4 cover adoption, integration and
the independent paragraph trials of Stage 3. They do not implement the register
collections or page solver of Stage 4.

For this migration there are no external consumers requiring source backwards
compatibility. Change `khipu` first, then propagate its contracts into `typography`
and `tytui` (including tycli). Temporary downstream compilation failures caused
by that API change are permitted between migration PRs. Transitional aliases,
compatibility wrappers and parallel legacy public APIs are not required.
Source compatibility is separate from the behavioral contracts: preserve CSS
geometry, original source coordinates, immutable ownership, paragraph lifecycle,
replay v5 and the zero-service/breaker-work equal-width path.

| PR | Scope | Acceptance gate |
| --- | --- | --- |
| **1 — khipu: immutable material and Tikray** | Compile the immutable SoA Khipu; add incarnation identity, atomic bindings and Discovery; migrate first-fit and KP to immutable Tikrays and captured geometry; migrate freezing, intrinsic measurement and checkpoint/seed APIs. Include Diagnostics/PolicyForLine. Preserve existing ordinary hyphenation modes and retain the NoHyphenation restriction for continuation. | Khipu ordinary/race/build/vet checks pass. Prove A remains freezable after B succeeds, fails or is cancelled; validate binding identity, detached accessors, variant normalization, intrinsic early admission, cache retry/cancellation and seed ownership. Existing supported output remains equivalent. Typography and tytui may still fail to compile against the changed APIs. |
| **2 — typography: adopt the new contracts** | Migrate Run, Prepared, Layout, intrinsic measurement and existing StartFlow/Continuation.Fit orchestration to PR 1. Preserve the existing client behavior, CSS assembly and retained rendering boundaries. Keep the equal-width fast path geometry-only. | Typography ordinary/race/build/vet checks, dependency audits and adapter regressions pass against PR 1. Cover A/B/C width history, cancellation, atomic placement, source/feature domains and collection of working state while frozen fragments survive. Tytui may remain temporarily incompatible. |
| **3 — tytui/tycli: complete the migration** | Update application consumers and affected test helpers; adapt tests to new source APIs while retaining their assertions. Verify paragraph/document layout, nested atomics, proofs, CLI behavior and offline replay. Update current usage documentation. | Establish a fully passing migration baseline across khipu, typography and tytui: ordinary/race/build/vet plus relevant dependency audits, adapter and integration regressions. Preserve current CSS/proof/replay-v5 controls and historical rejection evidence; do not regenerate controls to conceal drift. No downstream API breakage from PRs 1–3 remains. |
| **4 — typography: expose independent paragraph trials** | Define and expose the client-facing trial orchestration API using reusable Khipu/Discovery, immutable Tikrays and matching copied assembly inputs. Allow independent constraints and atomic realizations; retain full freezing/assembly initially. Update application integration where needed. | Required owning and integration checks pass, preserving the PR 3 baseline. Prove retry/rollback isolation, freeze or assemble A after every outcome of B, changed-atomic prefix compatibility, cancellation recovery, bounded retention and unchanged equal-width reuse. Published results must not retain trial workspaces or preparation unnecessarily. |

PR 1 is substantial. Organize its review into focused commits for material and
binding, discovery, breaker decisions/freezing, and checkpoints. Its completed
state must pass khipu's gate; this sequence does not require intermediate commits
inside that PR to be independently publishable. PR 2 adopts the new khipu
contracts before PR 4 exposes a broader trial API, keeping the existing flow
behavior separately verifiable.

### Temporary breakage and verification policy

Each migration PR must identify its upstream revisions, remaining downstream
API failures and the follow-up PR scope that repairs them. Record the actual
sibling checkouts used for validation: local `replace` directives otherwise make
a passing result ambiguous. A check blocked by an unmigrated consumer is reported
as blocked, never passed. Expected downstream API failures do not excuse failures
inside the owning module or justify weakening behavioral assertions.

Run the owning module's ordinary and race suites, build and vet for each PR, plus
applicable dependency audits and adapter regressions. Run downstream checks as
soon as the corresponding consumers compile; PR 3 must run the complete migrated
pipeline checks before claiming migration completion. PR 4 must preserve that
baseline and run relevant cross-module checks for its new trial API. Include
css-box-layout checks whenever changes reach its contracts. A coordinated set of
PRs is also acceptable, but simultaneous updates across repositories are not a
requirement.

### Work after the migration series

Register collections and bounded page trials follow as a separate series in
css-box-layout and application integration. Define their typed integration and
page-candidate boundaries before implementation; the page-decision protocol is
still outside Stage 1's contracts. Hyphenating continuation gets its own PR and
carry/provenance/mandatory-boundary/cancellation acceptance gate before removing
the NoHyphenation restriction. It is not a prerequisite for migrating the current
bounded flow profile. Trial-loop cost measurements and any optimization follow
Stage 5, preserving historical measurement evidence; concurrent execution follows
Stage 6's ownership and service audits. The upstream bidi fix remains deferred.

## Staged work and acceptance gates

These stages are independent of FLOW.md's completed P1–P6 sequence. Stage 1's
design is documented; stages 2–6 and all implementation acceptance tests remain
deferred. Specification completion does not claim runtime validation.

1. **Define the new immutable SoA Khipu and its companion contracts — documented.**
   [REFLOW_KHIPU.md](REFLOW_KHIPU.md) is the reviewable specification and contains
   the field-by-field migration. Its scope is:

   - Every SoA plane, immutable shaping/source/bidi record, accessor and retained
     resource; distinguish unbound atomic slots from real zero advances.
   - Incarnation construction/identity and the fixed properties it guarantees;
     define new-incarnation rules without trial-time deep comparisons.
   - Atomic bindings and their pairing with complete trial realizations, without
     writing geometry back into Khipu or requiring duplicate base knots.
   - The incarnation-scoped synchronized discovery facility: complete immutable
     publication, normalized replacement IDs, early intrinsic admission,
     errors/cancellation and lifetime. Keep selected alternatives out of both
     Khipu and discovery state.
   - Independent immutable Tikrays containing endpoints, variants, binding identity,
     stable geometry/policy and references needed for freezing after later trials.
     Value-returning diagnostics and per-line policy accessors expose retained
     results with explicit invalid-index and nil/zero-receiver behavior.
   - Logical continuation and hyphenation carry, including `SeedFor`, metadata-only
     breaker accessors and compatibility rules replacing current worker-owner
     checks. Keep page solver internals outside this contract.
   - A field-by-field migration from `PreparedParagraph`, `preparedMaterial` and
     today's `Khipu`; identify what is consolidated, shared, moved or removed,
     and how construction, breaking, freezing, measurement and checkpoints change.

   The gate is a reviewable ownership/API specification with aliasing, identity,
   lifetime and branch-isolation cases. Naming and SoA are settled; exact Go fields
   and signatures are specified in the Stage 1 document and remain unimplemented.

2. **Implement the material/decision separation in khipu.** Construct the immutable
   Khipu, supply trial bindings and produce independent Tikrays; make freezing consume
   those Tikrays. Preserve existing algorithms/output while proving candidate A
   remains valid after all outcomes of B. This requires the Stage 1 design first.
3. **Add isolated paragraph trials.** Retain bidi/source context, bind atomic
   realizations, assemble Tikrays and preserve equal-width reuse. Start serialized
   with full materialization; do not claim general hyphenating continuation until
   its checkpoint gate passes.
4. **Add bounded multi-register page trials.** Use explicit frame sets and a
   bounded search window before introducing a general constraint/template solver.
   Validate coordinated commit/rollback and source-reference obligations.
5. **Measure and then optimize.** Measure repeated width changes, height-only
   trials, changing atomic geometry, policy switches, warm/cold discovery and
   memory retained by live/discarded candidates. Evaluate binding reuse and
   reduced materialization against output equivalence.
6. **Enable concurrency after ownership audits.** Exercise shared-cache misses,
   cancellations and service restrictions under the race detector; test the chosen
   page-decision protocol against obsolete candidate inputs.
   Compare concurrent results with serialized results before claiming support.

Required acceptance cases include:

- Every trial preserves its input register collection; alternative successors
  remain independent. Rejection requires no restoration of an earlier version.
- Published Khipu planes and records cannot be mutated through accessors, bindings,
  discovery or trials. Changed fixed inputs yield a different incarnation.
- A remains freezable/renderable after B succeeds, fails or is cancelled; its
  endpoints, selections, replacement records and bindings remain paired.
- Diagnostic/policy inspection returns independent values, preserves availability
  flags and mixed KP/first-fit attribution, and handles invalid indexes and
  nil/zero Tikrays without panic or fallback policy.
- Atomic realizations can differ without changing reusable material, and changed
  geometry cannot silently reuse an incompatible accepted prefix.
- Breaker switches preserve fixed shaping/material and use the requested policy.
- Cache warming and trial order cannot change alternatives or accepted output;
  errors are retryable and failed discovery never appears as a negative result.
- Provider zero/mixed IDs and explicit-marker IDs normalize deterministically
  without reordering candidate ties or admitting duplicate published IDs.
- Intrinsic policy rejections retain their early zero-call gates and error
  precedence; they do not poison the complete-batch cache used by layout.
- Detached continuation seeds cannot alter checkpoints/Tikrays; compatibility
  validation performs no prefix search, width query or discovery/service call.
- Footnote/aside dependencies are respected by the adopted successor collection
  and corresponding body placements.
- Repeated splits, A/B/C width histories, RTL/source associations and full CSS
  line-height survive independent Tikrays and continuation.
- Compatible retained paragraph fitting performs zero additional service or
  breaker work; new-trial atomic relayout costs are reported separately.
- Incarnation material and discovery state can be collected after the last
  legitimate register version/Tikray/checkpoint retaining them is released, while
  frozen output remains usable.
- Concurrent execution, when enabled, agrees with serialized evaluation; the
  eventual page-decision protocol cannot confuse successors of different input
  collections or corrupt earlier candidates.

Implementation must run the owning ordinary/race/build/vet suites and relevant
cross-module checks under the [PR verification policy](#temporary-breakage-and-verification-policy),
including the complete migration baseline at PR 3. This document adds no implementation or new validation
evidence. The upstream bidi preparation rejection/stall limitation remains
separate deferred work, not an accepted result or a reason to reset bidi context
at frame boundaries. General vertical elasticity and atomic-interior fragmentation
are also outside this plan.
