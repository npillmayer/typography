# Font input boundary: move discovery upstream

> Package update: this document includes historical API locations.
> See [current ownership](README.md) for the glyphing/render split.
> Old root, paragraph and combined-adapter APIs have been removed.

Status: implemented and verified in typography and sibling tytui on 2026-11-15.
This document retains the agreed migration rationale and acceptance criteria.
Current API guidance is in README.md.

Verification passed in both modules: `go test ./... -count=1`,
`go test -race ./... -count=1`, `go build ./...`, and `go vet ./...`. These include
the dependency audits, adapter service regressions, selection ownership/cache and
file-lifetime checks, and replay v1/v2/v3 compatibility. Protected static replay
fixtures were unchanged. Typography no longer depends on fontfind. No changes to
khipu, fontfind or css-box-layout were needed; nothing was committed or pushed.

## Objective and recommended boundary

Typography should receive an explicit set of selected fonts, a default font,
style-to-font rules and an ordered fallback list. It should select among those
fonts while preparing text, but should neither locate additional fonts nor own
provider policy, generic-family mappings or discovery caches.

Move application font selection into a new `tytui/fontselection` package, using
fontfind directly. Remove `typography/discovery` and `typography/adapter/fontfind`
after their consumers and tests have migrated. Keep `adapter/opentype`: parsing,
coverage, shaping and retained outlines are separate from locating fonts.

For a clear final boundary, pass **already loaded `FaceEntry` values** into the
root resolver. Reuse `contracts.FontFace`, `Info` and immutable `fontresource.Resource`;
do not introduce another abstract font/scale model or application face wrapper.
Loading selected files/resources moves to application setup, which can continue
to call the existing OpenType adapter directly.

Removing discovery alone would be a smaller change: existing `LoadSettings`
already accepts explicitly chosen paths/resources. This plan also replaces that
loading constructor so the promised “vector of fonts” boundary is explicit and
verifiable. Parsing stays in the OpenType adapter, not in tytui.

## Pre-migration implementation and coupling

- [Root loading](resolver.go) loads the default, each mapping rule, path fallbacks,
  then resource fallbacks. The resolver retains faces keyed by resource identity.
  Itemization already performs coverage fallback only within that configured set.
- `discovery` owns ordered family requests, explicit
  generic mappings, successful-result caching, diagnostics and parse validation.
  The former `adapter/fontfind` translates provider values,
  disables implicit fallback, checks match attestations and snapshots bytes.
- Tytui's [font editor](../tytui/fonts.go) and
  [request setup](../tytui/font_requests.go) produce `discovery.Resolution`.
  Even an explicit file selection uses this discovery-named envelope.
- [Paragraph pipeline input](../tytui/pipeline/pipeline.go) carries that envelope;
  `Run` calls `LoadSettings`, itemizes, and retains a pinned `Settings` snapshot.
  [Document setup](../tytui/document_workbench.go) similarly loads fonts before
  supplying a resolver/shaper factory to `documentlayout`.
- [Replay](../tytui/replay/replay.go) already loads stored bytes without invoking
  providers. Its `Selection.Policy` and `Selection.Descriptors`, however, directly
  use discovery runtime types. Font identity/binding/diagnostic wire records are
  already separated in [wire.go](../tytui/replay/wire.go).

The migration is primarily an ownership and input-API change. It is not a change
to glyph coverage, shaping, breaking, line-height or painting algorithms.

## Responsibility after migration

| Owner | Responsibility |
| --- | --- |
| `fontfind` | Existing provider mechanisms and locator pipeline; no dependency on typography is added |
| `tytui/fontselection` | Application requests, permitted providers, generic mappings, exact/substitution policy, diagnostics, session cache and selection orchestration |
| `typography/adapter/opentype` | Read/parse explicitly selected fonts, retain bytes, provide coverage/shaping/identity/outlines; no discovery |
| Root `typography` | Validate supplied entries/references, map styles, choose grapheme coverage fallbacks, preserve feature domains/source coordinates, retain snapshots |
| `typography/fontresource` | Immutable bytes and stable provenance/content identity; provenance is data, not permission to search |
| `paragraph` / khipu | Existing preparation, measurement, shaping/breaking/freezing and assembly |
| `render` | Existing retained proof construction through services |
| `tytui/replay` | Versioned wire records, content-store validation and offline reconstruction |

The application flow becomes:

```text
file/resource requests or family requests
    -> tytui/fontselection + fontfind where needed
    -> OpenType load of the selected bytes
    -> selected FaceEntry vector + mapping/fallback references + provenance
    -> typography.NewStyleMapper
    -> itemization -> paragraph/khipu -> retained rendering
```

Replay enters at “OpenType load of the selected bytes”, using only its local
content store. Discovery provenance may be displayed or resaved, never executed.

## Proposed typography input API

The implemented input API:

```go
// FaceEntry, Info and Coverage are existing types/contracts.
// Every reference below is a FaceEntry.Info.ResourceID, not a filename.
type SelectedFonts struct {
    Faces     []FaceEntry
    Default   string
    Rules     []SelectedRule
    Fallbacks []string
    Size      dimen.DU
}

type SelectedRule struct {
    Family     string
    Weight     int
    Slant      string
    ResourceID string
}

func NewStyleMapper(ctx context.Context, fonts SelectedFonts, coverage Coverage) (*StyleMapper, error)
func (r *StyleMapper) Snapshot() *SelectedFonts
```

This is a selected set with explicit roles, not a search catalog. Specify these
contracts:

1. `Faces` contains one entry per resource ID referenced by the default, rules or
   fallback list, including fonts in rules/fallbacks unused by the current text.
   The application builder deduplicates this table; the constructor rejects
   duplicate IDs, missing references and unreferenced table entries. Distinct
   provenance IDs remain distinct even when their content hashes match.
2. Preserve ordered, first-match style rules, their existing wildcard/matching
   semantics, and ordered fallback references, including repeated references.
   No fallback is inferred from table order. Existing path fallbacks precede
   existing resource fallbacks when translating an old request.
3. Preserve first-use identity ordering: default, rule order, fallback order,
   deduplicated by resource ID. `Infos` and snapshots use this canonical order,
   regardless of the supplied table order; never derive order from map iteration.
4. Require a valid default, positive finite size and a usable coverage service.
   Validate each nonnil face/resource and the existing metadata consistency rules:
   ID, SHA256, origin and `Info.Path == Origin.Location`. Keep rule validation.
   Metadata paths are descriptive only; the resolver must not open them.
5. `NewStyleMapper` performs no provider calls, file I/O, font parsing, shaping or
   coverage preflight. The loader remains responsible for the face matching the
   retained bytes; the generic constructor cannot inspect a concrete parser.
   Preserve context checks during validation and before publication.
6. Copy input slices and warning slices. Share only immutable resources and
   opaque faces under their existing stable-lifetime contracts. `Snapshot` returns
   detached selection/metadata slices; it never reloads a font. Keep all configured
   roles, not just faces appearing in emitted bindings.
7. Only explicit fallback references may be tried when a grapheme is uncovered.
   Cmap errors remain terminal and missing coverage remains an error. Do not
   relocate existing coverage checks or add dynamic discovery during itemization.

`Info`, `FontTextRun`, `IdentityProvider`, `FontSource` and the existing font/scale
contracts remain. `SelectedFonts` is a runtime value, not a JSON or replay DTO.
No parsed-font accessor is introduced.

Retire root `LoadSettings`, path-bearing `Settings`/`Rule`, `FaceLoader`,
`LoadFunc`, and the loader/coverage `Services` bundle after migration. Retain
`Coverage`/`CoverageFunc`. OpenType `Services.Load` remains a direct application
entry point returning `FaceEntry`; remove only its assertion against the retired
root loader interface. Khipu's separate `contracts.FontLoader` and the adapter's
`Loader.LoadFont` are unaffected. A temporary compatibility constructor may bridge
old callers during migration, but must not survive as a second permanent API.

## Application selection in tytui

Create `tytui/fontselection` with request records and a selection envelope:

```go
// Illustrative application-owned aggregate, not a face/backend wrapper.
type Selection struct {
    Fonts      typography.SelectedFonts
    Provenance Provenance
}
```

`Provenance` owns discovery policy, original descriptor requests, ordered
selection diagnostics, and requested file paths (default, per-rule and fallback).
Keep it separate from the executable font references. File/resource request
records replace the path-bearing part of `typography.Settings`; existing
fontresource and FaceEntry values are reused directly.

Move the policy and provider logic from the two old typography packages into this
application package. Use fontfind's `locate.NewResolverPipeline` with an explicit
private registry/provider list. Do not recreate a general `Searcher` abstraction
merely to preserve the old layering. Controlled callbacks or a small private load
function seam are sufficient for application tests. Keep generic fontfind work
upstream if needed later, but this migration requires no fontfind API change and
must not add a fontfind-to-typography dependency cycle.

Preserve the existing selection guarantees:

- Provider/family order, explicit generic mappings, match attestations and opt-in
  substitution. A returned font with an error is not an exact match.
- The explicit no-fallback registry: ordinary fontfind fallback behavior must not
  silently replace this policy when the adapter package disappears.
- Exact diagnostic order, stages, request provenance and substitution evidence,
  including evidence returned on failure and on cache hits.
- Read/parse failures terminate selection; failed attempts are not cached.
  Preserve cancellation while waiting for a session, during provider work and
  before publishing a completed selection.
- Successful cache entries retain immutable selected resources and detached
  evidence under one fixed policy. Keep caching session-scoped; do not introduce
  a new global face cache or change cache-key normalization in this migration.
- The existing trusted-provider restriction: fontfind's `ReadFontData` allocates
  before the snapshot size bound. Removing the adapter does not remove that limit.

For the initial migration, keep the current successful-resource cache and parse
validation behavior. Build loaded entries for each layout invocation from the
pinned resources; avoiding repeated parse work can be a separate optimization.
This preserves fresh invocation ownership while eliminating loading inside the
root resolver. Keep parser warnings and diagnostics stable across both steps.

### Tytui call sites

| Area | Required change |
| --- | --- |
| `font_requests.go` | Use application-owned request/policy types and selection session; preserve the packaged catalog, substitution toggle and generic mapping |
| `fonts.go` | Keep editable requests distinct from completed selections; F5 validates asynchronously and reports the same diagnostics; preserve cancellation/stale-result handling |
| `pipeline.Input.Fonts` | Replace `*discovery.Resolution` with `*fontselection.Selection`; nil still selects synthetic, nonnil never silently falls back to synthetic |
| `pipeline.Run` | Construct the resolver from loaded entries and coverage; preserve itemization, streaming setup, retained render services and successful diagnostic transcript |
| `pipeline.Result` | Replace `FontSnapshot` with the selected-font snapshot; clone application provenance/diagnostics independently from mutable input; keep runtime faces/services out of serialization |
| `document_workbench.go` | Select/load fonts in the worker, construct the new resolver, and retain snapshot/provenance for inspection and export |
| `document_view.go` and other font views | Display selected entries/references and separate provenance instead of assuming snapshot path fields |
| `documentlayout` | Keep `Config.Resolver` and `NewShaper`; no locator/provider is passed to layout callbacks; update setup helpers and fixtures' generating tools only as needed |
| `replay` | Convert application provenance through explicit wire DTOs and reconstruct loaded selections offline |
| Examples and tests | Replace old Settings/Resolution construction and imports; keep expected geometry, bindings, diagnostic evidence and static manifests unchanged |

Resolve file requests at the same application operation boundary as today:
preview may validate, while a new run loads its own selected snapshot. Do not
silently pin an editable pathname forever at F5 preview time. Completed results
remain stable after file deletion/replacement, and replay never consults that
path. Headless callers now explicitly select/load before calling `pipeline.Run`.
This is a deliberate source-API migration; keep UI font-stage reporting and
underlying error causes when moving selection failures out of `pipeline.Run`.

The shared workbench scheduler and serialized resolver/prepared-handle rules
remain. No `tytui/fontbackend`, application face implementation, forwarding
paragraph engine, cross-invocation preparation cache, or document replay format
is introduced.

## Replay compatibility strategy

**Keep replay versions 1, 2 and 3 unchanged.** Changing runtime ownership or Go
type names does not require a new wire format.

### Isolate wire records before removing discovery types

Extend `replay/wire.go` with replay-owned policy/request/descriptor records that
reproduce the current JSON fields, types, ordering and omission behavior of
`DiscoveryPolicy`, `FontRequest`, `DescriptorRule` and `DescriptorSettings`.
Change `Selection.Policy` and `Selection.Descriptors` to those records, with
explicit conversion functions. Continue using the existing wire identity,
origin, request, binding and diagnostic records. Do not alias runtime selection
types into wire structs.

`Manifest.Input` currently embeds `pipeline.Input` and saves `Fonts: null`.
Introduce a replay-only input DTO mirroring that JSON shape before changing the
runtime Fonts field. Keep the literal `Fonts` field and reject non-null unpinned
font input on load; using only `json:"-"` would change the saved shape. Preserve
existing tags, source/style data and v1/v2 omission of line-height fields. This
also prevents future loaded faces/services from leaking into manifests.

Characterize nil versus empty slices/maps using the existing writer and Clone
behavior before implementing conversions. Preserve legacy provenance integers,
including unknown codes round-tripped by `TestWireProvenanceCodesAndFieldNames`;
never reinterpret provenance Weight as a CSS weight or validate historic codes
against the current discovery request domain.

### Saving

Walk pinned selected references in the existing order: default, rules, fallbacks.
Deduplicate the font table by resource ID while storing content by SHA256. Keep
same-bytes/different-origin resources as distinct identities. Pin unused configured
rules/fallbacks too. Preserve font table order, binding ranges, diagnostic order,
requested-path fields, descriptor evidence and the existing numeric units.

Read only retained resource bytes; do not rerun discovery or reopen a source path.
The current Save reparses retained resources to validate identity: retain that
validation behavior initially using the existing OpenType loader. Any later
optimization to use cached metadata alone requires its own compatibility review.
Never serialize faces, coverage services, shapers, locators, VList objects or a
new selected-font runtime structure.

### Loading

1. Keep strict JSON/version/policy checks, size bounds and trailing-data rejection.
2. Read font blobs from the supplied store, verify hash and byte count, recreate
   each immutable resource with the recorded origin, and validate parsed identity.
3. Resolve all default/rule/fallback IDs, preserving role order and repeated
   references. Reject duplicates in the font table and unresolved references as
   today. Validate every stored entry, but build the selected FaceEntry vector
   only from referenced IDs: the current loader tolerates unreferenced stored
   entries, so the new constructor's stricter table contract must not accidentally
   reject an otherwise accepted legacy manifest.
4. Restore display provenance through wire conversions. Neither requested paths
   nor family descriptors may trigger lookup, even when the stored blob is absent.
5. Call the migrated pipeline with the selected fonts; compare paragraph text,
   break positions, bindings and full frozen/proof evidence exactly as today.

Keep the version-specific behavior:

| Version | Required behavior |
| --- | --- |
| 1 | Legacy style boundaries; restore omitted paint policy to legacy at runtime and omit it again on save; compatibility spacing |
| 2 | Glyph-boundary paint; compatibility spacing; no line-height/placement wire fields |
| 3 | Explicit line-height and numeric placement evidence; either explicit paint policy remains supported |

Keep the version-1 resource-ID formula, quoting, separators, decimal request
codes and lowercase content hashes unchanged in `fontresource`. Paths in origins
are historical identity inputs; relocating a blob to a content store must not
rewrite those origins to the new storage path.

### Resave qualification

The current loader does not reconstruct executable paths from
`RequestedPath`/`RequestedFallbackPaths`/per-rule `RequestedPath`, and existing
legacy resave tests compare versions, policies, fonts, bindings and evidence,
not the complete Selection object. Do not promise byte-identical whole-manifest
resaves: revisions and GoVersion are also external metadata.

Before changing the writer, add characterization tests for selection provenance
on load/resave, including nil/empty values. Preserve current normalized resave
behavior in this migration. If retaining original requested-path strings in a
separate display record would change resaved JSON, treat that as a distinct
follow-up decision; never restore them as paths eligible for loading. New saves
must continue to record the application's current request provenance.

Do not regenerate static pre-step-5 or pre-step6-paint fixtures, alter v1/v2/v3
expected evidence, or relax tamper checks to make the migration pass.

## Implementation sequence

1. **Capture behavior.** Inventory remaining consumers and record the current
   full-suite baseline. Add focused characterization coverage for selection order,
   duplicate role references, unused configured fonts, provenance conversion and
   selection load/resave. Preserve the existing static fixtures as external controls.
2. **Freeze remaining replay DTOs.** Separate policy/descriptors/input from runtime
   types while the current implementation still exists. Require identical wire
   output for equivalent input and unchanged offline replay before proceeding.
3. **Add selected-font construction.** Implement `SelectedFonts`, `SelectedRule`,
   detached cloning/snapshot and `NewStyleMapper`. Keep a temporary old-entry bridge
   if necessary, and compare old/new itemization with fake services and real fonts.
   Avoid changing the recently refactored itemization/assembly/render algorithms.
4. **Move selection policy to tytui.** Move discovery/provider regressions with
   their owner, introduce application request/provenance records, and use fontfind
   directly. Preserve terminal errors, explicit fallback policy, cache lifetime
   and diagnostic transcripts. This package may temporarily use the old root API.
5. **Switch all consumers together.** Migrate paragraph and document setup, F5
   preview, headless callers, replay reconstruction/saving, tests and proof tools
   to selected fonts. Run end-to-end checks before deleting the old surface.
6. **Remove the obsolete surface.** Delete `typography/discovery` and
   `typography/adapter/fontfind`, root loader-based construction and transitional
   aliases/bridges. Remove typography's direct fontfind requirement/replacement
   and tidy dependencies; inspect indirect dependencies rather than forcing their
   removal. Tytui retains its direct fontfind dependency.
7. **Update the contract and verify.** Update both modules' AGENTS.md and README
   files, package godoc and examples. Replace instructions assigning discovery to
   typography or requiring root FaceLoader injection with the new boundary.
   Keep all other documented invariants. Complete the checks below before declaring
   the migration finished; no commit or push is part of this plan.

The changes to AGENTS.md in step 7 are proposed replacements for its current
architecture rules, not claims that the current code already follows this plan.
Removing public packages/constructors is a Go source compatibility break even
though replay is compatible. Coordinate all known sibling users in the same
migration and document replacement calls for other consumers. If external release
compatibility is required, deprecate temporarily before removal; do not reverse
the dependency by making typography forward to tytui.

## Verification and acceptance criteria

Typography's owning tests must cover selected-entry validation, detached metadata,
reference validation/order, canonical identity ordering, immutable snapshot
ownership and preservation of mapping/grapheme/script/fallback behavior. Exercise
both paint policies, inline-object barriers, scalar feature domains, cancellation,
terminal coverage errors and whole-grapheme fallback. A constructor test should
use fake preloaded faces and coverage that fails if queried, proving construction
does not preflight coverage or require a font loader.

Move discovery tests to `tytui/fontselection` and retain provider ordering,
exact/substitution evidence, successful-cache ownership, failure retry, queued
cancellation, explicit generic policy, read/parse errors and input mutation cases.
Keep adapter loader/retained-byte/stream/outline tests in typography. Do not move
font parsing or metric tests into tytui.

Preserve and extend these application controls:

- `TestPreStep5VersionOneFixtures`, `TestLegacyPlacementEvidenceSurvivesResave`,
  `TestPaintPolicyVersions` and `TestLineHeightReplayVersion3`.
- `TestWireProvenanceCodesAndFieldNames`, `TestDescriptorReplayBypassesDiscovery`,
  offline round trips, rendering-bypass/source correspondence and retained placement
  tests. Add zero-discovery-call assertions for both load and resave.
- Tampered/missing blobs, invalid identities/references, unknown provenance-code
  preservation, requested-path removal and completed-result lifetime after file
  deletion. A missing blob must fail even if a provider could locate a replacement.
- Paragraph and document proof equivalence, both breakers, RTL/fallback, ligatures,
  combining marks and explicit soft hyphens. Preserve the document NoHyphenation
  profile and all existing source/style/paint behavior.

After the final switch run these in **both typography and tytui**, using their
actual sibling replacement checkouts:

```sh
go test ./... -count=1
go test -race ./... -count=1
go build ./...
go vet ./...
```

Keep `TestExtractedPackageDependencyBoundaries`, tytui's application dependency
audit, and adapter service regressions in those runs. Extend the audits to ensure
no typography production package imports fontfind or tytui/fontselection, and
no tytui caller imports the removed typography discovery packages. Verify generic
paragraph/render code still imports neither discovery nor concrete adapters.

If implementation unexpectedly requires khipu, fontfind or css-box-layout changes,
run their relevant owning checks as well; no changes there are planned. Report
actual commands/results and any pre-existing failures instead of reusing an older
passing-suite claim.

Completion means typography accepts preselected fonts without a discovery or
loading callback; tytui preserves its current selection experience; replay v1/v2/v3
loads and resaves offline with unchanged protected evidence; and no redundant
adapter/forwarding layer remains in typography.
