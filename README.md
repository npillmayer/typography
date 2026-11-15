# typography

Paragraph typography integration with explicit services:

- Root: style mapping, script/grapheme itemization, bindings,
  pinned snapshots, IdentityProvider and FontSource.
- `style`: document values, UTF-8 spans and typography/paint projections.
- `fontresource`: bounded immutable bytes and fixed version-1 content/provenance IDs.
- `discovery`: neutral CSS requests, explicit generic policy, ordered families,
  session cache and detached resolution evidence; requires Searcher and FaceLoader.
- `adapter/fontfind`: controlled providers, exact-match attestations, private
  ResolverPipeline, enum conversion and resource snapshots; no default loader.
- `render`: neutral retained-outline/visual-order interface for frozen proof walkers.
- `adapter/opentype`: loader/face/scale/shaper, identity, retained outlines and
  generic VisualOrder implementation. No public parsed-font accessor.

Root LoadSettings requires FaceLoader and Coverage; discovery.New requires explicit
search/validation services. Neither installs a backend or imports an adapter.
No implicit font substitution or system/network discovery is enabled. Read/parse
failures terminate; successful cache hits retain diagnostic substitution evidence.
Providers must be offline, concurrency-safe and honor cancellation. The fontfind
ReadFontData allocation still precedes the snapshot size bound; only trusted
providers belong in that adapter. Services remain valid for the result lifetime.

Resources expose independent readers, never mutable bytes. Snapshot pins every
configured resource without invoking discovery or reopening paths. Resource request
codes preserve version-1 IDs and are distinct from document CSS numeric weights.
Tytui consumes these packages directly; only versioned font wire records remain
in its replay package. There are no tytui font/face/resource forwarding wrappers.

Glyph-boundary paint is the default: equivalent resolved typography coalesces
across paint changes, while original paint spans remain separate. One glyph gets
one color at its first logical source anchor; shared anchors share color and
independently anchored marks can differ. No clipping or conflict warnings.
Feature-bearing domains remain separate for khipu's existing clipping/rebasing.
Actual font/size/language/bidi boundaries remain significant. LegacyStyleBoundary
is available explicitly for old replay; document Style.Equals remains complete.
render.PaintSources maps frozen anchors and the supported explicit-soft-hyphen
replacements to original bytes, rejecting unknown generated transformations.

Development uses sibling cords, khipu, uax, schuko, opentype and fontfind checkouts.
Neutral tests use fake services; adapter tests use controlled providers and sibling
opentype font fixtures. Run `go test ./...`, `go test -race ./...`, `go build ./...`
and `go vet ./...`, then tytui suites including static pre-migration replay fixtures.

Tytui replay version 2 records the new glyph-boundary policy. Version-1 fixtures
keep legacy behavior and identities. Keep tests for ligatures, combining marks,
RTL/fallback/discretionaries, source maps and original feature-coordinate domains.

Rendering bypass: M owns `ProjectStyles`, producing `TypographySpan` values with
color-free `TypographyStyle` styles. The private run-domain identity preserves
relative feature origins and explicit legacy document boundaries without carrying
paint through styled.Text. Itemize/ItemizePolicy use the same projection and their
internal run styles contain no document Color field. ResolveStyle accepts projected
styles and legacy document Style inputs. Keep document equality and JSON unchanged.
The application retains `PaintSpan` values separately for rendering; tytui's
Result.PaintSpans returns a detached copy. ItemizeWithMetadata remains compatible.
No annotation package or replay version change is introduced.

For mixed inline input, use `Resolver.ItemizeInline(ctx, text, spans, atomics,
policy)`. Each `contracts.AtomicAnchor` selects one U+FFFC OBJECT REPLACEMENT
CHARACTER at its original UTF-8 byte offset, with a unique nonzero occurrence ID;
anchors must be strictly ordered by offset. The same anchors accompany the
returned styled text in khipu's `ParagraphInput`. Object dimensions are supplied
later through khipu binding; this itemizer does not measure or lay out objects.

Attached markers keep their bytes and scalar positions but receive no text style
or font binding. They separate grapheme, script and fallback processing, including
soft-hyphen neighbor selection. Object-only source spans bypass style/font
resolution; ordinary UTF-8/span validation still applies. Unattached U+FFFC remains
text. No anchors delegates to the existing text-only itemization path.

Private style metadata retains original projected feature domains, including
marker scalars, and emitted text windows. `ResolveStyle` clips and rebases features
supplied through shaping defaults to each window, including script/fallback
splits. Keep the returned style values intact. Projection uses the existing paint
policy before slicing around objects; a separate object-only source span is a
domain barrier and is never resolved to coalesce surrounding text. Source, paint
and bidi associations remain with the caller. Input slices are not retained or
modified; errors return no partial itemization (invalid UTF-8 returns empty text).

Glyph coverage is the caller's responsibility. This API adds no coverage pass;
ordinary text still uses the existing itemizer checks and fallback behavior while
attached markers bypass them. Removing those checks and moving font preparation
elsewhere is a separate future task. Atomic-aware paragraph orchestration and
vertical assembly are available through `paragraph.Prepare` and `Prepared.Layout`.

Extraction step 7 is complete. Compose the OpenType implementation explicitly:

```go
services := opentype.Services{}
resolver, err := typography.LoadSettings(ctx, settings,
    typography.Services{Loader: services, Coverage: services})
```

Services implements FaceLoader, Coverage, IdentityProvider and render.Backend.
Use adapter/opentype.NewStream for shaping. Discovery.New takes the same injected
loader and a Searcher such as adapter/fontfind.New's controlled provider adapter.
Application file selection can use discovery.Resolution{Settings: settings};
descriptor resolution returns that same detached settings/provenance envelope.
The resolver's Snapshot remains typography.Settings with pinned resources.

General parser/stream/resource/discovery regressions live here. The feature-range
and nonzero-cluster-base baseline moved from tytui into adapter/opentype; tytui
retains DOM-to-resolver, frozen glyph, TUI and replay integration tests. A dependency
audit prevents generic packages from importing DOM, discovery or concrete adapters.
The rendering bypass and legacy replay policies are unchanged.

`paragraph.Assemble(ctx, frozen, paragraph.Options{Policy:
paragraph.ProofCompatibility, DefaultEmSize: size})` now assembles immutable frozen
lines into a boxed natural vertical list. `Box`, `Lines` and `Materials` return
value geometry; `Frozen` retains the read-only source. Geometry uses fractional
big points with downward-positive Y and a first-line reference baseline. The named
policy preserves tytui proof metrics, minimum 0.2em gap and trailing gap. Logical
width encloses target/occupied widths; glyph ink is not measured. DefaultEmSize
supplies 0.8/0.2/0.2em fallback metrics for lines without measured extents.

This first slice has no rendering, DOM, discovery or concrete font dependency;
callers supply already frozen output. There is no target-height packing, glue
stretch/shrink or CSS line-height interpretation. Coordinates must remain below
`dimen.Fil` expressed in big points; non-finite metrics, invalid input and overflow
are errors. Tytui now retains assembly output in completed results and its glyph
renderer consumes placements without querying line metrics. Result.VList returns
a detached handle; font/paint inspection input cannot reflow an existing proof.
Replay rebuilds placements under the compatibility policy without changing v1/v2
wire records or evidence. Rendering colors remain on their separate path.


`paragraph.Run(ctx, input, params, config)` now provides prepared styled paragraph
→ boxed vlist. Input carries styled.Text, base Direction and zero-byte Markup;
Params separates Constraints.Width, Breaker and Assembly defaults. Config supplies
khipu shaper/resolver/hyphenation contracts (zero uses khipu's synthetic backend).
M's Itemize/ProjectStyles runs before P; input style identities are preserved. P
never re-itemizes or reconstructs styles, normalizes whitespace or loads fonts.
Result retains Frozen/VList, paragraph-relative break endpoints, bidi runs and
counts; errors carry the failed stage and return no partial output. Every call
gets a fresh owner and freezes before assembly. Services must stay valid for
retained results. Typography-only text and separate renderer paint remain the
application contract. The compatibility assembly policy preserves replay v1/v2.


Paragraph-engine step 4 adds `CSSLineHeight` to Run with `Params.LineLayout`:
normal, unitless factor or resolved DU length, plus optional nonoverlapping UTF-8
LineHeightSpan overrides. Values bypass styled.Text and shaping. Explicit
Streaming.Defaults font/size supplies the strut. Normal uses scaled ascent +
descent + max(0,font gap); half-leading is split about the baseline, including
negative leading. Effective extents are signed; baselines may lie outside boxes.
CSS mode stacks LineBox material without compatibility's 0.2em floor or trailing
kern. Glyphs use frozen metrics/fallback; surviving spaces use prepared source
style metrics. Source styles are scanned without flattening the cord. Keep
ProofCompatibility unchanged, and reject CSS values under that policy. Frozen-only
Assemble remains compatibility-only; CSS Run needs source styles for spaces.

The narrow layout records are not a general annotation API. CSS parsing,
percentage/relative-length computation and inheritance remain the input adapter's
responsibility. Factor inheritance preserves a multiplier; lengths stay fixed.
General vertical-align, empty-element boxes, CSS edge computation and bounded
height packing remain outside this engine. Prepared mixed layout supports the
fixed atomic boxes and resolved margins described below. Rendering colors and
ink bounds remain separate.
Tytui's headless Input.LineLayout opts in; TUI defaults stay compatible. Replay
v1/v2 omit the new fields and retain old evidence. Explicit line-height cases use
v3 with typed input and numeric placements, no services or VList objects. Both
paint policies remain independent of layout. Owning tests cover exact geometry,
inheritance/strut, mixed fonts/sizes, zero/negative leading, spaces/replacements,
invalid values/metrics/ranges, cancellation and overflow. Tytui tests ligatures,
RTL/fallback/paint, rendering without metrics, and offline v3 resaves/tampering
alongside unchanged legacy fixtures. Full tests/races/build/vet pass in both modules.

Pipeline integration stages 3B/3C add reusable paragraph preparation and atomic
vertical assembly. `Input.Atomics` extends the existing input record in place;
use keyed struct literals. `Prepare(ctx, input, options, config)` consumes bidi
markup and shapes base text once. `Prepared.Layout(ctx, LayoutRequest)` derives
atomic advances, binds fresh material, breaks, freezes and assembles. Serialize
operations on one prepared handle. Preserve source/style/service lifetimes;
configuration envelopes, feature slices and request slices are copied, while
arbitrary services/styles are not deep-cloned. Failed calls publish no partial
result, and neither later layouts nor mutated result slices change old vlists.

```go
prepared, err := paragraph.Prepare(ctx, input,
    khipu.PrepareOptions{Hyphenation: khipu.NoHyphenation}, config)
// Handle err, then reuse prepared for different widths and atomic dimensions.
result, err := prepared.Layout(ctx, paragraph.LayoutRequest{
    Params: paragraph.Params{
        Constraints: paragraph.Constraints{Width: width},
        Breaker: paragraph.FirstFit,
        Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight},
        LineLayout: &paragraph.LineLayout{
            Default: paragraph.LineHeight{Kind: paragraph.NormalHeight},
        },
    },
    Horizontal: &paragraph.HorizontalLayout{}, // explicit zero skips
    Atomics: boxes, // exactly one AtomicBox per anchor, in any ID order
})
```

An `AtomicBox` supplies nonnegative border dimensions, signed `AtomicMargins`
and `AlignBottomMarginEdge`. Border dimensions already contain padding/borders;
only margins are added to the advance and vertical contribution. Atomic geometry
unites with text and the paragraph strut; marker-only source styles are never
resolved for font metrics, and markers receive no text half-leading. The CSS
strut uses explicit captured `Streaming.Defaults`; compatibility assembly retains
its existing text metrics/fallback and gap before adding atomic extents.

`VList.AtomicPlacements()` returns detached border origins in fractional big
points, Y downward, ordered by line and frozen physical traversal. Walkers must
not sort by X: signed advances can move the pen backward and zero-sized objects
may coincide. Line indexes are zero-based. IDs preserve caller associations.

Widths may be zero. Explicit horizontal skips replace both breaker defaults,
including zeros, and are identical for breaking/freezing. Nil retains the default
profile. Geometry validates primitive values, completed sums and published
coordinates against `dimen.Fil`; negative completed line widths are unsupported,
while positive overfull lines remain successful with diagnostics. `StagePrepare`
and `StageBind` preserve causes alongside existing stages. `Run` remains the
text-only convenience path with historical error stages and rejects anchors at
`StageInput`. Frozen-only `Assemble` rejects atomic content without geometry.

No new glyph coverage validation, CSS document adapter,
general baseline alignment or replay format is introduced. Coverage relocation
remains future work. Owning tests cover repeated widths/dimensions, input/result
ownership, both breakers, signed/zero geometry, RTL/fill traversal, metric bypass,
cancellation/failure recovery and real-font feature domains. Typography and tytui
retain their full ordinary/race/build/vet and replay compatibility checks.

Pipeline integration stages 4B/4E expose ordinary and bounded explicit intrinsic
measurement on the same prepared handle:

```go
widths, err := prepared.MeasureIntrinsic(ctx, khipu.MeasureRequest{
    Atomics: []khipu.AtomicIntrinsic{
        {ID: 1, MinAdvance: 10 * dimen.BP, MaxAdvance: 20 * dimen.BP},
    },
})
// Handle err; widths.MinContent and widths.MaxContent are finite nonnegative DU.
```

Supply exactly one range per attached ID in any order, or an empty request for
text-only paragraphs. Endpoints are signed horizontal advances including resolved
margins, ordered and of magnitude below `dimen.Fil`. Khipu evaluates natural widths,
ordinary wrapping and edge trimming, excludes generated fill/skips/elasticity, and
normalizes complete signed spans. No atomic heights, alignment, line-height values,
strut or rendering services are required. `NoHyphenation` and bounded
`ExplicitHyphenation` are supported. Explicit measurement permits one eligible
soft-hyphen position and at most one break-permitting material variant per textbox.
Multiple positions/variants and `AutomaticHyphenation` return
`khipu.ErrUnsupportedPolicy`, without a fallback or successive splitting.

Measurement shares preparation's serialized-operation and stable-source contracts.
Explicit queries may discover and shape complete replacement pairs once per
textbox; layout requests reuse these batches at their usual lazy provider seam.
Successful empty responses are cached; failures remain retryable. Base text is not
reshaped, vertical metrics are not queried, and request storage is not retained.
Changing atomic ranges recomputes widths without reshaping replacements;
prior frozen/vlist results remain unchanged. Invalid handles, nil contexts and
cancellation already present at entry use `StageInput`; request validation,
evaluation and later cancellation use `StageMeasure`. All failures preserve
`errors.Is` causes, return zero widths and leave preparation reusable. Glyph
coverage remains caller-owned. CSS measurement callbacks and their invocation-local
preparation reuse table are implemented in `tytui/documentlayout`. CSS core owns
recursive intrinsic aggregation, width selection and descendant resolution; the
adapter measures/layouts only the requested normalized group and publishes a
complete document after success. The CSS profile uses NoHyphenation; bounded
explicit discretionary measurement stays available through this generic API.

## Retained paragraph proofs

`render.BuildParagraphProof(ctx, render.ParagraphInput{Frozen, VList, Text, Paint,
PaintPolicy}, render.Services{Backend, Identity})` walks completed immutable
paragraphs. It returns detached float64 big-point glyph/line geometry, outline
paths, font identities, colors and original processed UTF-8 `SourceByte` anchors.
`PaintPolicy` defaults to glyph-boundary paint; explicit legacy paint preserves
existing replay behavior. Both paragraph and document proof consumers in tytui
use this walker. CSS/DOM transforms remain in `tytui/documentlayout`.

Proof construction never measures lines, loads fonts, reshapes, rebreaks or stacks
lines. Visual ordering applies once to each detached directional glyph fragment;
frozen knot order stays physical. Retained font/scale services remain stable.
Errors/cancellation return no partial proof, including late outline failures.
Owning tests deny metric queries after layout and verify physical positions,
multibyte anchors, paint, detached ownership and cancellation. Tytui checks real
nested documents and the unchanged static paragraph replay evidence.


Pipeline stage 5 is verified across khipu, typography, tytui and css-box-layout
(2026-11-15). Tytui supplies actual nested scheduling/reuse tests, independent
signed geometry/rounding oracles, resize/failure lifetime sequences and retained
real-font fixed/auto proofs. Owning prepared-result and render tests remain here;
CSS types, document scheduling, normalization and publication stay outside this
module. See [the integration record](../khipu/doc/PIPELINE_INTEGRATION.md) and
[document proof fixtures](../tytui/documentlayout/testdata/README.md).

Ordinary/race suites, build and vet passed in all four modules, including generic
dependency audits, backend adapters and unchanged paragraph replay controls.
Independent document layouts prepare afresh; operations sharing a resolver or
prepared owner stay serialized, and retained font/scale services remain stable.
No cross-invocation cache, speculative final callback, automatic intrinsic
hyphenation or successive explicit splitting is added. The explicit-split limit
is not a claim of general CSS min-content/hyphenation conformance. Glyph coverage
stays caller-owned; relocation of the existing itemizer checks remains future work.
