# Font-face compatibility through CheckFace

> Package update: this document includes historical API locations.
> See [current ownership](README.md) for the glyphing/render split.
> Old root, paragraph and combined-adapter APIs have been removed.

Status: implemented on 2026-11-16; validation is recorded below. Revised on 2026-11-16. This plan supersedes
its earlier proposal to propagate a font type parameter through the pipeline.
The [selected-font boundary](FONT_INTF.md) remains in place; historical replay
promises in that document are superseded by the
[current replay contract](../tytui/doc/OFFLINE_REPLAY.md).

## Objective and scope

Let a shaping or coverage implementation report whether it can consume a given
`contracts.FontFace`, and reject incompatible combinations before the respective
operation starts. Use an exported `CheckFace(contracts.FontFace) error` method.
A backend may accept several face implementations, provided it has a real bridge
for each. Typography and khipu remain independent of concrete font types.

This is runtime validation, not compile-time proof of pairing. Go checks that
implementations provide the method; the implementation is responsible for an
honest compatibility answer. Keep defensive checks at direct backend entry
points as well as pipeline preflight.

Keep existing nongeneric configs, style resolvers, selected fonts, prepared
handles, frozen results and tytui result fields. Preserve khipu's synthetic
zero-config defaults. Do not introduce a completed-result facade, parameterize
retained owners/caches, or redesign identity and rendering interfaces. This plan
covers box layout through shaping and coverage; renderer compatibility remains
a separate responsibility. Implementing a HarfBuzz backend is not required.

## Current conversion points

The [font contract](../khipu/contracts/font_api.go) exposes metrics and scaling.
Resolved run styles carry that interface and an em size. Khipu constructs an
immutable FontInstance after resolution; ShapeParams.Font retains that binding.
Khipu does not extract a concrete OpenType font.

The actual conversion happens in typography's
[OpenType stream adapter](adapter/opentype/stream.go): `Stream.Shape` asserts
`p.Font.Face().(*Face)` and passes the face's private `font *ot.Font` to `otshape.Params`.
The adapter's [coverage service](adapter/opentype/loading.go) independently
asserts the same face implementation before querying a glyph. The synthetic
shaper has its own type check. Today these failures can be discovered only when
the backend operation is attempted.

Keep all concrete assertions inside the corresponding adapters. The
[OpenType face](adapter/opentype/font.go) continues to own parsing, retained
bytes, metrics and the lock shared by shaping, lazy queries and outlines. No
public parsed-font accessor is needed.

## Shared acceptance contract

Add the following contract in `khipu/contracts`:

```go
// FontAcceptor checks whether a consumer can use a font face.
// CheckFace is a cheap, read-only capability check, not a font query.
// It rejects nil, typed-nil, unsupported and unusable face instances.
// Success does not guarantee glyph coverage or successful shaping of any text.
// It performs no I/O, parsing, shaping or stream-state changes.
// Calls are safe concurrently; capabilities and accepted faces remain stable
// for the lifetime of the configured services.
type FontAcceptor interface {
    CheckFace(FontFace) error
}
```

Embed `FontAcceptor` in `contracts.StreamingShaper`, leaving its other methods
unchanged. Reuse it in typography's existing coverage interface:

```go
type Coverage interface {
    contracts.FontAcceptor
    GlyphIndex(contracts.FontFace, rune) (uint32, error)
}
```

Make the method mandatory. An optional type assertion that skips validation for
older implementations would leave the original hole in the pipeline. Update all
production implementations, test doubles and examples during the migration.

`CheckFace` checks a particular instance, not just its Go type name. A typed-nil
pointer or an uninitialized face must fail even if the type is supported. Do
not use backend-name strings, reflection-based type registries or pointer
identity as a substitute for the adapter's acceptance logic.

The method must not query glyphs or metrics, load resources, initialize a bridge,
consume input, change the cluster base, close a channel, or record a terminal
stream error. It may inspect already prepared immutable backend state. Repeated
checks are harmless, including checks on a stream that has been finished;
acceptance describes the backend's capability, not the stream's readiness.
`Shape` still enforces busy/closed state and all per-run argument checks.

Use errors with useful context rather than a boolean. Add a shared
`contracts.ErrUnsupportedFontFace` sentinel for unsupported representations;
invalid instances retain `ErrIllegalArguments`. Preserve existing `errors.Is`
classifications where direct calls already expose them, including synthetic
shaper argument errors. Wrap causes with the operation and run/resource context;
callers must not need to parse error text. Successful acceptance does not promise
support for every script, feature, buffer option or Unicode character.

### Multiple supported representations

The OpenType adapter accepts its own initialized `*opentype.Face`. A hypothetical
HarfBuzz wrapper could accept its native face plus an OpenType adapter face and
another library's face. Its private conversion helper defines that supported
set, and both `CheckFace` and actual shaping use the same acceptance rules.

Supporting an additional type requires a real conversion path, not merely a
positive type switch. A bridge must use the same font instance, face index and
variation state, preserve glyph IDs, and emit positions in the font units
expected by the retained face's `FontScale`. Normalize a backend's different
position units inside the wrapper. Coverage must describe the same font.

The OpenType adapter's retained immutable `Resource()` can supply bytes to an
external bridge without exposing `*ot.Font`. Expensive parsing/conversion belongs
in explicit backend preparation before selection, with any reusable state owned
by that backend. `CheckFace` only verifies that the required state is already
available. It must not substitute another font or silently discover/load one.
Neither khipu nor root typography needs a switch over backend types.

## Coverage checks during resolver construction

Extend `NewResolver(ctx, selected, coverage)` as follows:

1. Retain all existing structural validation: context, size, nonnil services and
   faces, resource identities/metadata, duplicates, roles and the closed entry
   table. No missing or unreferenced entries are accepted.
2. After that validation, call `coverage.CheckFace` for each distinct selected
   entry in canonical default/rules/fallback order. Check all configured entries,
   including rules or fallback candidates that the current text may never use.
   Repeated fallback references retain their role order but need only one check
   per resource entry.
3. Check cancellation between calls and before publication. On failure return no
   resolver; wrap the acceptance error with the resource ID and coverage context.

This is capability validation only. Construction still performs no I/O, parsing,
`GlyphIndex`, metric or character-coverage query. Existing itemization coverage
queries remain in their owning stage; attached atomic markers still bypass them.
Do not move font preparation or add a second text coverage scan.

`GlyphIndex` must retain defensive face validation for callers that bypass
`NewResolver`. The OpenType service should use the same private face validator
as its `CheckFace` method.

The existing one-function `CoverageFunc` cannot express acceptance. Replace it
with an explicitly paired function adapter (for example `CoverageFuncs` with
`Check` and `Lookup` callbacks), or use small concrete implementations at its
call sites. Missing callbacks must fail safely. Do not keep it satisfying
`Coverage` by adding a blanket `CheckFace` that returns nil. A truly generic
coverage implementation can explicitly accept every valid face it actually
supports. Update fake services and constructor tests accordingly.

## Shaping checks on the actual stream

Khipu owns the authoritative shaping preflight. Typography must not acquire a
shaper dependency in its resolver simply to check compatibility.

In [streamMixedCandidateParagraph](../khipu/stream_candidates.go), resolve the
runs with `prepareMixedRuns`, then check every actual run face against the
supplied stream before entering `consumeShaper`. Complete all checks before
launching the producer, setting cluster bases or emitting any glyphs. This also
covers the text-only entry point through `streamCandidateParagraph`.

Check resolved faces, including mappings and fallbacks, rather than just the
configured default. Check the actual factory-created stream; validating one
prototype is insufficient if a later factory result has different capabilities.
A simple check per run is sufficient. Do not use `map[contracts.FontFace]` for
deduplication: implementations need not be comparable.

`prepareMixedRuns` already obtains scales. This placement promises rejection
before shaping, not before all calls to the face's generic methods. It does not
reorder style resolution, scalar feature clipping or scale validation. Atomic-only
content creates no text-run face to check; do not add font queries for markers.

On failure preserve the cause and add the run/source context. Return no encoded
paragraph or partially prepared result. Typography's `Run` and `Prepare` retain
their existing stage wrappers; no new paragraph stage or alternate fallback
behavior is needed.

## Lazy replacement shaping and stream ownership

Initial paragraph checks are insufficient for discretionary replacements. Both
intrinsic measurement and later layout may request lazy replacement shaping from
a fresh stream after base text has already been prepared.

In [streamReplacement](../khipu/discretionary_stream.go):

1. Preserve existing replacement validation, feature copying and cancellation.
2. Create the stream and reject a nil or typed-nil factory result.
3. Call that stream's `CheckFace` with the replacement's actual face before
   `consumeShaper`, `SetInitialCluster` or `Shape`.
4. On acceptance, use the existing synchronous producer/concurrent drain path,
   terminal-error handling, cancellation and producer join.

Apply this to both pre-break and post-break fragments, including empty fragments
that the existing path sends to a backend. Do not treat the base paragraph's
acceptance or another factory instance's acceptance as a permanent certificate.

Preserve complete-pair publication. The pre-break side may already have shaped
when the post-break check fails, but neither side may escape as a partial pair
or be published as a successful cached replacement. Failed attempts remain
retryable, successful empty batches remain cached, and prior layouts and
measurements remain unchanged. Reusing a complete cached replacement needs no
new stream check because it performs no new shaping. Do not eagerly shape
alternatives or reshape base text to achieve preflight.

Make ownership of fresh streams explicit on every pre-production exit. Extend
the `Finish` contract to require that finishing an unused idle stream is safe,
idempotent, releases stream-owned resources and does not emit glyphs or require
a drainer. The current OpenType and synthetic implementations already close
idle streams; audit all implementations and test doubles against this guarantee.

The preflight owner must finish an acquired but unused stream after an acceptance
error, cancellation or other preparation failure. Include early failures inside
`consumeShaper` before its producer launches. Choose a single cleanup owner or
an explicit handoff so callers do not race or inconsistently duplicate cleanup.
After launch, preserve cancel/join before final idle cleanup, and never call
`Finish` concurrently with active `Shape`. Keep acceptance failures identifiable
if cleanup also fails; preserve both causes where appropriate. Never wait for a
producer that was not launched.

`CheckFace` itself does not poison a stream. Direct callers may check a rejected
face and then use a supported one. Pipeline owners finish rejected streams
because they abandon that invocation. Direct `Shape` calls still validate the
face inside the existing terminal-error lifecycle.

## Adapter and application changes

For the OpenType backend, `CheckFace` is handled only by
`typography/adapter/opentype`. It must not be forwarded to `opentype/otshape`,
and `otshape` must not acquire this interface-compatibility contract. The adapter
validates the opaque `contracts.FontFace`; only actual shaping extracts its
private concrete `*ot.Font` and passes that to the OpenType shaper. The shaper's
existing concrete-font and shaping validation remains its own responsibility.

In typography's OpenType adapter, introduce one private helper that validates
and returns an initialized `*Face`. Use it from `Stream.CheckFace`,
`Services.CheckFace`, `Shape` and `GlyphIndex`, retaining operation-specific
context. Do not move `Shape` validation outside its terminal-error handling.
Keep the shared face lock, normal parser, retained bytes, scalar feature/cluster
semantics, output ordering and cancellation unchanged.

Khipu's synthetic shaper gets equivalent shared validation. Preserve support for
both `helpers.SyntheticFont` values and nonnil pointers; do not narrow the
accepted representations or change zero-config behavior. Update other shapers
and coverage implementations with truthful acceptance checks.

Tytui's `fontselection` and replay setup continue to prepare selected fonts,
then construct the resolver with the coverage service. Their existing resolver
construction now rejects incompatible coverage. Paragraph and document execution
receive mandatory shaping checks through khipu, including reusable preparation,
lazy measurement and later layout. Keep error causes visible through application
wrappers and preserve cancellation and detached result ownership.

An application may additionally check every selected face against a shaper at
composition time for earlier diagnostics. This is optional and must finish any
probe stream it creates. It cannot replace checks on actual execution streams.
Do not silently change font-selection-only previews into shaping operations or
reject unused shaper candidates there as part of this migration.

No changes are needed to `Result.Frozen`, vlist/proof data types, document callback
signatures or runtime backend selection. Retained identities and rendering remain
with their existing services; do not imply that a successful layout preflight
certifies an arbitrary renderer.

## Current replay contract

Follow [tytui offline replay](../tytui/doc/OFFLINE_REPLAY.md), its
[regression inventory](../tytui/doc/REPLAY_REGRESSION_INVENTORY.md) and
[fixture notes](../tytui/replay/testdata/README.md). Adding runtime acceptance
checks changes neither successful output nor wire data and requires no version
bump or migration reader.

- Read and write experimental version 4 only. Versions 1–3 and unknown versions
  fail before current-schema decoding, font acquisition or pipeline execution.
  Historical manifests remain unchanged rejection cases, not executable modes.
- Preserve the required `Input.Assembly` and `Evidence.Layout` for both assembly
  policies and both synthetic/OpenType backends. Compatibility mode has absent
  or null `LineLayout`; CSS mode requires it. Retained result state determines
  assembly. No paint-policy selector or legacy shaping branch returns.
- Keep current glyphs, paint/source correspondence, breaks, frozen geometry,
  placements and OpenType proof evidence unchanged. Preserve the 16 current
  SVG/placement controls and version-4 load/save/load cases.
- Retain immutable resources, selected ordering, numeric provenance codes,
  `Fonts:null`, and version-1 font identity/hash encoding. That identity version
  is independent of the manifest version. Save uses retained bytes; Load checks
  all stored fonts but passes only referenced entries to the resolver.
- Keep requested paths, rule/fallback paths, descriptors, policies and diagnostics
  through load/save as inert provenance. Never open original paths, rediscover
  fonts or substitute missing blobs during replay. Diagnostic prose is retained
  information rather than independently compared layout evidence.
- Do not serialize face objects, coverage/shaping services, acceptance results,
  backend bridge caches or VList objects. Load reconstructs runtime services and
  reruns their normal checks. JSON field ordering alone is not a contract.

A future HarfBuzz replay backend would need its own explicit replay design; this
plan adds no new backend discriminator or cross-shaper equivalence promise.
There is no permanent format migration framework or indefinite behavior promise.
Intentional future algorithm changes require explicit evidence review; an
incompatible format/semantic-contract change replaces supported versions.
Unexpected output drift during this change is a regression, not a reason to
regenerate fixtures or increment the version.

## Implementation sequence

1. Add `FontAcceptor`, error classification and the unused-stream `Finish`
   guarantee in khipu/contracts. Update its package documentation, synthetic
   implementation, examples and shaper test doubles.
2. Add actual-run preflight and explicit stream cleanup in khipu, including both
   base encoding entry points and lazy replacements. Keep existing ownership,
   publication, feature domains and shaping algorithms intact.
3. Extend typography coverage, replace one-function adapters, and add constructor
   acceptance checks. Implement OpenType shaping/coverage acceptance with shared
   private validation. Update package READMEs and API comments.
4. Update tytui service/test implementations and integration expectations for
   earlier compatibility failures. Keep existing pipeline/result/replay APIs and
   successful evidence unchanged.
5. Run cross-module regression checks and review the final diff for accidental
   rendering, discovery, generic-type or wire-format changes.

These are coordinated source changes: adding mandatory interface methods breaks
implementations until updated. Compiler failures identify missing methods, but
runtime regression tests establish pairing behavior. Do not retain a permissive
legacy path merely to make old implementations compile.

## Acceptance tests and required checks

Owning package tests must cover:

- Supported faces, foreign types, nil interfaces, typed-nil and uninitialized
  faces; meaningful wrapped errors and preserved `errors.Is` causes. A fake
  multi-representation backend demonstrates that acceptance is not one-to-one.
- CheckFace purity: no metrics, coverage queries, reads, parsing, glyph emission,
  cluster changes or terminal errors. Rejection followed by acceptance on the
  same directly used stream succeeds; concurrent checks are race-free.
- Resolver construction checks every distinct configured entry in canonical
  order, preserves repeated fallback roles, and returns no resolver on failure.
  Existing validation/cancellation and immutable snapshot behavior remain.
- A later incompatible resolved run fails before any base-paragraph shaping,
  even if earlier runs are compatible. Cover mapped/fallback faces, atomics and
  noncomparable face implementations. Direct Shape/GlyphIndex remain defensive.
- Lazy pre/post failures with a factory whose later stream accepts a different
  set of faces. Verify no partial pair/cache/result, retryability, successful
  empty caching, unchanged previous results and no reshaping of base text.
- Finish/cleanup after rejected preflight, preparation failure, cancellation and
  pre-launch consumer validation failure; no leaked resources or goroutines.
  Preserve bounded-channel draining, terminal failure and cancel/join behavior.
- Real OpenType shaping and coverage, synthetic value/pointer support, feature
  domains, RTL/fallback, placement and current retained proofs. Test unit/identity
  consistency for any real bridge introduced separately.

Run these commands independently in each of `khipu`, `typography` and `tytui`
after implementation:

```sh
go test ./...
go test -race ./...
go build ./...
go vet ./...
```

Include typography's generic-package dependency audits and OpenType adapter
service regressions, using sibling OpenType fixtures. Run relevant
`css-box-layout` integration checks for the cross-module pipeline change.
Tytui coverage must include pipeline/document preparation, repeated measurement
and layout, source correspondence, failure recovery and retained rendering.

Replay checks include current behavior and synthetic placement controls, CSS
assembly, color bypass, current-format resaves, strict format validation,
historical/unknown rejection before resources, provenance/identity/ownership,
requested-path resaves, unreferenced stored-font validation, discovery bypass,
offline resource removal, tampering and cancellation. Keep static historical
fixtures unchanged. Report checks actually run, not earlier passing results.

## Risk and review boundary

The change is substantially smaller than propagating a type parameter through
all retained content. The main risks are incomplete interface migration,
acceptance rules that disagree with actual operations, missed lazy paths, and
incorrect cleanup before producer launch. Mandatory contracts, shared backend
validators and lifecycle tests address those risks without changing layout
algorithms or result ownership.

Implementation follows the contract and sequence above. `CoverageFuncs` is the
paired callback adapter. Base and replacement owners finish streams on every
exit; cleanup after production follows cancel/join and preserves distinct causes.
Cleanup snapshots the terminal error before calling Finish. If a primary failure
already exists, repeated terminal causes are removed from the Finish error,
including wrapped/joined causes, while independent resource-release failures
remain visible. Internal producer cancellation therefore does not make a consumer
failure appear to be caller cancellation. Existing layout-time provider
cancellation limits remain unchanged.
Tytui's private selection test seam pairs its injected loader with explicit
coverage; the public Prepare API still installs OpenType services.

Validation completed on 2026-11-16 in the actual sibling checkouts:

| Module | Ordinary tests | Race tests | Build | Vet |
| --- | --- | --- | --- | --- |
| khipu | Passed | Passed | Passed | Passed |
| typography | Passed | Passed | Passed | Passed |
| tytui | Passed | Passed | Passed | Passed |
| css-box-layout | Passed | Passed | Passed | Passed |

Commands were `go test ./... -count=1`, `go test -race ./... -count=1`,
`go build ./...` and `go vet ./...` in each module. Typography's dependency
audit and OpenType service regressions, tytui's document integration and replay
controls, and the new acceptance/lifecycle regressions were included in those
suites. No replay source, manifest or proof fixture changed. Diff whitespace
checks passed in all three edited modules. No commits or pushes were made.

The same four-module checks were rerun successfully after the cleanup
error-reporting fix. New regressions first reproduced the failure on the old
implementation, then passed for base shaping and both lazy replacement sides,
with and without independent cleanup failures. They also cover wrapped/joined
errors, caller cancellation and terminal state changed during Finish.
