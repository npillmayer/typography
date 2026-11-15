# Compile-time font-face and shaper pairing

Status: proposed migration, not implemented. Based on the sibling khipu,
typography and tytui checkouts inspected on 2026-11-15, including the completed
[selected-font migration](FONT_INTF.md).

## Objective and recommendation

Make an incompatible font-face/shaper combination fail to compile. Keep khipu
and typography's generic packages independent of concrete OpenType types, and
preserve layout behavior and replay versions 1, 2 and 3.

Use one shared type parameter, `F contracts.FontFace`, throughout the path that
selects, resolves, retains and shapes faces. Instantiate it with
`*opentypeadapter.Face` or `*helpers.SyntheticFont` at application composition.
The OpenType adapter continues to keep its parsed `*ot.Font` private.

Carry `F` through reusable preparation, lazy replacement shaping and frozen
content. Also type the coverage, identity and outline services that consume those
faces. This avoids moving today's runtime mismatch from shaping into rendering.
Numeric layout records and serialized replay records remain nongeneric.

This is a coordinated Go source API migration across all three modules. Do not
claim source compatibility with existing nongeneric composite literals. Runtime
backend selection remains possible: tytui chooses between two independently
type-checked execution paths and combines only their completed inspection/output
interfaces.

## Current behavior and the unchecked boundaries

The current [font contract](../khipu/contracts/font_api.go) exposes metrics and
scaling. It does not associate a face with a particular shaping implementation.
`ShapeParams.Font` and `ResolvedRunStyle.Font` both hold `contracts.FontFace`.
`StreamingConfig` independently accepts a style resolver, defaults and a shaper
factory, so incompatible implementations can be assembled successfully.

The OpenType [face](adapter/opentype/font.go) retains `font *ot.Font`. Its
[stream adapter](adapter/opentype/stream.go) asserts `p.Font.(*Face)` before
constructing `otshape.Params{Font: face.font}`. Coverage, identity and outline
services make equivalent assertions. Khipu's synthetic shaper also checks the
concrete font type at runtime.

Changing only that `Shape` signature is insufficient. Faces survive in:

- Khipu's prepared runs, shaping arena, encoded paragraphs and owner environment.
- Prepared paragraphs and cached/lazily shaped discretionary replacements used
  by measurement, binding and both line breakers.
- Frozen text contexts, which retain shaping parameters and a scale.
- Typography's selected table, resolver defaults, projection/coalescing helpers,
  paragraph preparation, assembled vlists and retained proofs.
- Tytui's paragraph results, document invocation caches, proof services and replay
  evidence collection.

Two application details need explicit treatment. Khipu's zero `Config` currently
installs synthetic services; a generic implementation cannot construct a
synthetic face for arbitrary `F`. Tytui's `pipeline.Run` currently changes one
configuration variable from synthetic to OpenType, and returns one concrete
`Result` type containing nongeneric frozen data. Both patterns must change.

## Contract design

The following signatures are proposed. Existing fields and method behavior not
shown here remain unchanged.

```go
// In khipu/contracts. FontFace and FontScale retain their present methods.
type FontLoader[F FontFace] interface {
    LoadFont(io.Reader) (F, error)
}

type ResolvedRunStyle[F FontFace] struct {
    Font F
    // Existing size, language, script, spacing and feature fields.
}

type StyleResolver[F FontFace] interface {
    ResolveStyle(styled.Style, ResolvedRunStyle[F]) (ResolvedRunStyle[F], error)
}

type ShapeParams[F FontFace] struct {
    Font F
    // Existing direction, script, language and feature fields.
}

type StreamingShaper[F FontFace] interface {
    Shape(context.Context, ShapeParams[F], RuneReader, ShapeBufferOptions) error
    Glyphs() <-chan ShapedGlyph
    Err() error
    SetInitialCluster(uint32) error
    Finish() error
}

// In khipu.
type StreamingConfig[F contracts.FontFace] struct {
    NewShaper     func() (contracts.StreamingShaper[F], error)
    Resolver      contracts.StyleResolver[F]
    Defaults      contracts.ResolvedRunStyle[F]
    BufferOptions contracts.ShapeBufferOptions
}
```

`FontLoader` remains optional adapter infrastructure, not a loading callback in
typography's resolver. Discovery and loading stay upstream of selected-font input.

The OpenType implementation becomes concrete at its own boundary:

```go
func (s *Stream) Shape(
    ctx context.Context,
    p contracts.ShapeParams[*Face],
    src contracts.RuneReader,
    options contracts.ShapeBufferOptions,
) error

var _ contracts.StreamingShaper[*Face] = (*Stream)(nil)
```

`Shape` can then use `p.Font.font` directly after validating the face value. No
assertion from `contracts.FontFace` to `*Face` is necessary. Compile-time assertions
also cover the typed loader, resolver, coverage and retained rendering services.

### Exact scope of the guarantee

`StreamingShaper[*FaceA]` and `StreamingShaper[*FaceB]` have different method
signatures. A factory producing the former cannot populate a configuration for
the latter. It also cannot be widened to `StreamingShaper[contracts.FontFace]`:
Go does not make these parameterized interfaces covariant.

Keep the concrete face parameter through every shaping-capable API. Do not add
aliases instantiated with `contracts.FontFace`, `any` fields for live faces, or
an adapter that restores the old assertion-based `Shape` entry point. Generic
constraints alone do not forbid a caller from deliberately implementing a new
type-erasing backend; the guarantee covers the provided typed contracts and
implementations, not arbitrary casts or dishonest third-party implementations.

Compile-time pairing does not prove that a face is initialized, its retained
bytes match its metadata, a font covers some text, a scale belongs to the correct
face instance, or a factory returns fresh streams. Keep the existing runtime
validation, immutable ownership, cancellation and stream-lifecycle contracts.
Do not introduce a second generic scale model: `FontScale` only converts units
and need not be unwrapped by a backend.

### Go-specific implementation rules

- Methods cannot introduce their own independent type parameters. Put `F` on a
  receiver type that retains faces, or use a generic package-level function.
- Go function return types are not covariant either. A `NewStream` returning
  `*Stream` still needs a small typed factory closure returning
  `contracts.StreamingShaper[*Face]` when assigned to `NewShaper`.
- Do not add `comparable` to the font constraint merely to implement equality.
  Current projection conservatively retains boundaries for noncomparable faces.
  Preserve that behavior, feature-domain checks and pointer/value identity rules.
- Generic `F` cannot generally be compared with `nil`. Preserve nil and typed-nil
  validation using a narrow internal helper. Boxing for validation/comparability
  inspection is acceptable; recovering a backend face to dispatch shaping is not.
- Do not replace public parsed-font encapsulation with an `OTFont()` accessor,
  backend-name string, registration token or unchecked handle.

## Khipu changes

### Carry the face type through ownership and lazy work

Parameterize the following connected structures and their constructors/methods:

| Area | Changes |
| --- | --- |
| Configuration/ownership | `Config[F]`, `StreamingConfig[F]`, `Khipukamayuq[F]`, private `typEnv[F]` |
| Run preparation | `preparedRun[F]`, `runParameters[F]`, style resolution and run-production helpers |
| Retained shaping | Face-bearing contexts in `shapingArena[F]`, candidate paragraphs and replacement batches |
| Reusable execution | `PreparedParagraph[F]`, `Khipu[F]`, encode/prepare/bind/measure methods and their cache associations |
| Freezing | `FrozenKhipu[F]`, `FrozenText[F]`, `FrozenTextContext[F]`; `CopyText` preserves `F` |
| Breakers | Entry points and internal holders receiving `Khipu[F]`; typed discretionary providers and the parameter records that contain them |
| Clients | Helpers, debug writers, CLI, examples and test fixtures using those types |

The first-fit and Knuth-Plass `Parameters` currently contain
`DiscretionaryProvider`, whose method receives `*Khipu`. These parameter records,
provider interfaces and relevant default constructors therefore also acquire
`F`; they cannot simply remain nongeneric while `Khipu` changes. Keep geometry,
scoring and candidate value records nongeneric wherever they contain no typed
owner or font-bearing data. Do not redesign either breaking algorithm.

Audit initial shaping **and** `shapeDiscretionaryInput`/`streamReplacement`.
Intrinsic measurement can trigger lazy replacement shaping; freezing is not the
first point at which work becomes lazy. Prepared caches, replacement contexts,
newly bound paragraphs and factories must all retain the same `F`.

Keep face-free contracts nongeneric: glyph and feature records, rune readers,
buffer options, font metrics/scales, source ranges, atomic IDs/anchors/geometry,
hyphenation text requests, `Params`, `KnotCore`, frozen line/knot/cluster values,
break decisions and width summaries. Pure arithmetic helpers need no `F` even
when their callers are generic.

### Synthetic backend and zero configuration

Standardize the supplied synthetic shaping path on `*helpers.SyntheticFont`,
matching `NewSyntheticFont`. Give its shaper a corresponding typed `Shape`
signature. Existing synthetic value-form calls must migrate explicitly to the
chosen pointer form; do not retain a runtime union of backend types.

Make `DefaultStyleResolver[F]` a typed pass-through resolver. Add a clearly named
`khipu.SyntheticConfig()` returning `Config[*helpers.SyntheticFont]`, preserving
the current font, 10 pt default size, elastic spacing, fresh stream factory,
buffering and disabled lazy hyphenation defaults. Other configuration defaults
retain their current semantics.

A zero `Config[F]` must no longer silently select a synthetic backend. Require a
complete streaming configuration for generic execution; keep the established
validation/error staging for incomplete explicit services. Update zero-config
callers, paragraph examples and the CLI to call `SyntheticConfig`. Do not use a
runtime type switch on `F` to restore implicit defaults. This changes source
setup, not synthetic advances, layout defaults or replay evidence.

## Typography changes

### Selected fonts, resolution and projection

Introduce `FaceEntry[F]`, `SelectedFonts[F]`, `Resolver[F]`, `Coverage[F]` and
`CoverageFunc[F]`. The constructor becomes:

```go
func NewResolver[F contracts.FontFace](
    ctx context.Context,
    fonts SelectedFonts[F],
    coverage Coverage[F],
) (*Resolver[F], error)
```

`SelectedRule`, `Info`, `Binding`, resource IDs, resources, provenance codes and
paint/document style records stay nongeneric. Keep the closed-table validation,
canonical default/rule/fallback order, repeated role references, detached warning
slices and shared immutable resource/face ownership from FONT_INTF.

`Defaults` and `ResolveStyle` preserve `F`. Parameterize `ProjectStyles`,
`CoalesceStyles` and their resolved-style comparison helpers. Their output
typography/paint span records need no face parameter: they carry style values and
private domain metadata, not a choice of backend. Audit private itemized styles
and any future face-bearing additions; never hide a live face in `styled.Style`
and assert its backend later. Cords' `styled.Text` API need not change.

### Paragraphs and retained rendering

Parameterize `paragraph.Run`, `Prepare`, `Prepared`, `Result`, `VList` and
`Assemble` wherever they accept or retain typed configurations/frozen content.
Keep `Input`, layout requests/options, geometry, line-height spans, placements and
stage errors nongeneric. A `VList[F]` retains its matching `FrozenKhipu[F]`; preserve
the retained-content correspondence check used by proof construction.

Use `IdentityProvider[F]`, `render.Backend[F]`, `render.Services[F]` and
`render.ParagraphInput[F]`. `Identify` and `Outline` receive `F`; `VisualOrder`
continues to consume detached glyph records without a font argument. Keep
`ParagraphProof`, glyph/path records and line geometry nongeneric.

The existing private proof-walking methods on nongeneric `ParagraphProof` cannot
gain method type parameters. Convert face-bearing walkers to generic free
functions, or a private `proofBuilder[F]` receiver returning `ParagraphProof`.
This is a signature/ownership change only: preserve traversal and floating-point
operation order. Metrics-only helpers may accept the existing `FontFace` query
interface if they never return/recover a backend face or feed it to shaping;
prefer keeping `F` where it is already available.

### OpenType adapter

`Services.Load` returns `FaceEntry[*Face]`; `Loader.LoadFont` returns `*Face`.
Coverage, identity and outline methods receive `*Face` directly. Replace foreign
backend rejection tests with compile-fail tests, while retaining runtime tests
for nil/uninitialized faces and invalid resources. The shaper still accesses its
private `*ot.Font` only inside this package and under the shared face lock.

Do not modify normal parsing, retained bytes, lazy-query synchronization, scalar
feature/cluster translation, synchronous production with concurrent draining,
terminal failure handling or outline conversion. No opentype module API change
is expected: its shaper still receives `otshape.Params` with `*ot.Font`.

## Tytui composition and runtime backend choice

### Selection and paragraph execution

`fontselection.Selection.Fonts` becomes
`typography.SelectedFonts[*opentypeadapter.Face]`. It is an application package
already using the adapter, so this concrete instantiation belongs here. Editable
requests, provenance, provider policy and resource-cache behavior remain unchanged.
Preview and each new run continue to prepare their own selected faces.

Keep the public `pipeline.Input` application-specific: Fonts is the concrete
OpenType selection or nil for synthetic. Dispatch early into two typed paths:

```text
Fonts == nil -> runWithBackend[*helpers.SyntheticFont](...)
Fonts != nil -> runWithBackend[*opentypeadapter.Face](...)
```

Use a private generic helper for shared orchestration, with typed resolver,
defaults, shaper factory and retained services. Preserve the input validation and
successful diagnostic transcript. Do not store both configurations in
`Config[contracts.FontFace]`, or introduce an `any`-based shaper dispatcher.

### One TUI result type without erasing live shaping contracts

Recommend retaining a nongeneric application `pipeline.Result` facade, backed by
a private `retainedResult[F]` holding typed frozen content, placements and services.
Publish it only after complete layout. Its nongeneric private interface may expose
operations such as detached inspection, glyph proof generation and numeric
placement access; it must not accept a font, shaper, resolver or replacement
configuration from the caller.

Each operation executes inside the typed implementation, where the original
`F` and services remain paired. For inspection/replay, expose detached run records
containing source ranges, features, glyphs, clusters, size and neutral identity,
not `contracts.FontFace`. Proof generation remains lazy over retained data, not
eagerly moved into layout or implemented by reshaping.

This deliberately changes public result access:

- Replace the current broadly exposed `Frozen`/`VList` access with completed
  inspection/placement accessors suitable for either backend. Typed internal
  results remain available to typed helpers/tests.
- Keep OpenType `FontSnapshot` concretely typed, or provide a detached typed
  accessor; synthetic results have no selected-font snapshot.
- Move independently assignable `Identity`/`Renderer` fields into the typed
  retained result. Maintain rendering-without-metrics and injected-service tests
  through typed construction helpers, rather than permitting mismatched services.
- Preserve detached Input, paint spans, provenance, geometry and diagnostics.
  Migrate the TUI, history/export and replay callers together; no fake untyped
  frozen-result compatibility object should return live faces.

This is safe type erasure of a **completed operation**, not of separately
recombinable font/shaper values. It does not introduce a font/face wrapper or
restore the removed fontbackend/paragraph forwarding packages.

### Document layout

Parameterize `documentlayout.Config[F]`, private `invocation[F]` and
`preparedGroup[F]`, plus face-bearing `GroupResult[F]`/`Result[F]` and proof entry
points. Preserve invocation-local prepared reuse for intrinsic measurement and
final layout.

`Document` is built before fonts are chosen and contains no backend face. Keep it
nongeneric. Replace its current `Document.Layout` method with a package function
such as `documentlayout.Layout[F](ctx, document, request, config)`; do not propose
an unsupported generic method on `Document`. The TUI's document snapshot can be
concretely instantiated with `*opentypeadapter.Face`; owning tests use their own
fake face types.

A typed invocation can still implement CSS core's nongeneric callback interfaces:
those callbacks exchange geometry, not fonts or shapers. No CSS core generics,
DOM changes, cross-invocation caches or document replay format are required.

## Replay compatibility

Keep versions 1, 2 and 3 and all existing wire records unchanged. Go type
parameters and backend implementation names are not serialized identifiers.
The input/policy/descriptor/font DTO separation implemented in FONT_INTF is the
starting point, not something to undo.

### Saving and evidence

- Preserve `Input.Fonts: null`, field names/order/tags, nil/empty normalization,
  numeric provenance codes (including unknown codes) and resource-ID hashing.
- Read selected snapshots through the typed OpenType result. Pin default, rules
  and fallbacks in the current canonical order, retaining repeated references and
  unused configured roles. Distinct origins sharing bytes remain distinct IDs.
- Retain normal reparsing of resource bytes during Save. Never reopen original
  paths or call discovery. Do not serialize `F`, face pointers, factories,
  services, prepared owners or vlists.
- Migrate `evidence` to the completed result's detached run/geometry accessors.
  Keep the existing wire `Run`, `Evidence` and numeric placement fields. Existing
  face-free khipu glyph/feature/frozen geometry values need no generic wire form.
- Keep synthetic runs' identity fields and omitted glyph proof as today.
  Avoid eagerly invoking a service or populating a formerly absent field simply
  because a common result interface offers that operation.

### Loading and resaving

1. Perform the existing strict JSON, version, policy, size and hash validation.
2. For font cases, load store bytes through the normal OpenType adapter into
   `FaceEntry[*Face]`. Validate every stored record, including unreferenced ones;
   include only referenced entries in `SelectedFonts[*Face]`.
3. Restore application provenance as data. Requested paths and descriptors must
   never select a backend or trigger loading/discovery.
4. Use the existing presence/absence of font selection to enter the typed
   OpenType/synthetic pipeline branch. No new backend wire discriminator is needed.
5. Compare paragraph text, breaks, bindings, glyphs, source/feature coordinates,
   proof and numeric placement evidence exactly as before.
6. Preserve normalized requested-path removal on load/resave, including nil/empty
   behavior. Revisions and GoVersion remain external metadata; do not promise
   byte-identical entire manifests.

Version 1 retains legacy style boundaries and compatibility spacing; version 2
retains glyph-boundary paint and compatibility spacing; version 3 retains explicit
line-height, either supported paint policy and numeric placement evidence.
Do not regenerate pre-step-5, pre-step6-paint or other protected fixtures to make
the migration pass. Do not alter metric/rounding order to simplify generic code.

## Implementation sequence

1. **Capture the current baseline.** Run ordinary/race/build/vet in all three
   modules. Inventory production consumers, examples, debug/CLI utilities and
   sibling imports of affected APIs. Capture representative synthetic/OpenType
   evidence, including lazy replacements and document layouts. Preserve current
   fixtures and the uncommitted FONT_INTF work; do not reset checkouts.
2. **Prove the typed contracts.** Add a small positive/negative compilation
   harness for two distinct fake face types. Settle the concrete synthetic type,
   explicit synthetic configuration and function/receiver signatures before
   propagating types. Record the deliberate Go source changes.
3. **Migrate khipu as one dependency layer.** Change contracts, helpers, owner/run
   storage, preparation/measurement/replacements, breakers and frozen output.
   Migrate the CLI/debug/examples and run khipu's full checks. Temporary scaffolding
   is allowed only during the change; no permanent untyped shaping bridge remains.
4. **Migrate typography.** Type selected entries/resolver/projection, OpenType
   services, paragraph handles and retained proof services. Preserve metadata-only
   types and all algorithms. Add dependency and compile-fail coverage, then run
   typography's full checks against the migrated khipu checkout.
5. **Migrate tytui composition and consumers together.** Specialize fontselection,
   split synthetic/OpenType execution, introduce the completed-result facade,
   migrate document callbacks/results, TUI inspection/export and replay evidence.
   Keep asynchronous font-stage error reporting and editable-file semantics.
6. **Remove transitional APIs and document the contract.** No mixed untyped/typed
   production paths or aliases to `FontFace` remain. Update package godoc/READMEs,
   AGENTS guidance where present, API examples, FONT_INTF follow-up references and
   all known callers. Keep the module Go versions unless an actual feature requires
   a change; the design does not require a language upgrade.
7. **Validate the integrated migration.** Run the checks and acceptance matrix
   below on final code in all three modules. Report actual results and deviations.
   Do not commit or push as part of implementation.

The dependency-layer transitions will temporarily break downstream compilation.
Coordinate them in the same development change; do not publish an intermediate
module combination or preserve unsafe wrappers merely to keep each transient
checkout green. External consumers need a source migration/release note even
though replay data remains compatible.

## Verification and acceptance criteria

### Compile-time checks

Use compiler tests, not runtime assertions, to establish the new guarantee.
Keep intentionally invalid programs under testdata and compile them in a
controlled harness using the actual local modules/toolchain. Include a matching
positive case so missing dependencies or unrelated syntax errors cannot be
mistaken for successful rejection. Check the relevant type/signature diagnostic
without overfitting its full compiler wording.

Required negative cases:

- A synthetic face passed to OpenType `Shape`, and the reverse pairing.
- An OpenType stream/factory in a synthetic `StreamingConfig`, including an
  attempted widening to `StreamingShaper[contracts.FontFace]`.
- A resolver/default/selected table with a different `F` than its configuration
  or coverage implementation.
- A prepared/encoded paragraph passed to an owner or discretionary provider
  instantiated for another face type.
- A frozen result supplied to identity/outline services for another face type.
- A document configuration combining the wrong resolver and shaper factory.

Positive checks cover both concrete backends and at least two independent fake
backends through encode, lazy shaping, freeze and proof construction. Audit for
`any`/`FontFace` erasure on shaping-capable paths, not just explicit assertions in
the OpenType adapter. Generic dependency audits must continue to forbid DOM,
fontfind, tytui and concrete backend imports where currently forbidden.

### Behavioral and compatibility checks

Retain existing tests for scalar feature/cluster domains, source byte ranges,
mixed scripts, whole-grapheme fallback, both paint policies, atomic barriers,
stream cancellation/terminal errors, detached snapshots and retained bytes.
Test repeated preparation/layout/measurement, lazy replacement success/empty/error
cache behavior and fresh stream factories. Preserve signed/zero geometry,
first-baseline/line-height behavior and rendering without metric queries.

Keep tytui's replay controls, including `TestPreStep5VersionOneFixtures`,
`TestLegacyPlacementEvidenceSurvivesResave`, `TestPaintPolicyVersions`,
`TestLineHeightReplayVersion3`, `TestWireProvenanceCodesAndFieldNames`, descriptor
bypass and the FONT_INTF selection wire/resave/unreferenced-font tests. Verify
zero discovery calls on load **and** resave, missing/tampered blob rejection,
unused-role retention and original-file removal. Exercise synthetic and OpenType
results through the same TUI history/export/replay facade without changing proof
timing, errors or evidence.

Run in **khipu, typography and tytui** after migration:

```sh
go test ./... -count=1
go test -race ./... -count=1
go build ./...
go vet ./...
```

Compile-fail harnesses, dependency audits, examples and adapter service regressions
must be included. Check formatting and unchanged protected fixtures. No changes to
cords, opentype, fontfind or CSS core are planned; if the implementation requires
any, explain the dependency and run that module's owning checks too.

Completion means ordinary public composition cannot pair the supplied backends'
faces and shapers incorrectly, lazy work never loses that pairing, generic cores
remain backend-independent, and offline replay preserves v1/v2/v3 semantics and
protected evidence without a new format version.

## Planning validation

For this plan, the current source paths and consumers were inspected. A temporary
standalone Go compiler probe confirmed a matching generic configuration compiles,
while cross-face assignment and widening a concrete shaper to `Shaper[FontFace]`
fail with incompatible method signatures. This validates the proposed type
relationship only. No production implementation or module test suite was changed
or run for this planning task.
