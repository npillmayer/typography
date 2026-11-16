# typography

Typography is a Go library for turning styled text into laid-out paragraphs.
It connects preselected fonts and text preparation to khipu's shaping and
line-breaking machinery, then assembles the resulting lines into reusable
paragraph geometry. Applications can use it to handle font fallback, mixed
scripts, line spacing and fixed inline objects, measure paragraph widths, and
build drawing proofs from completed layouts. Font and shaping services are
supplied explicitly, with OpenType adapters included; document parsing, CSS
computation and the user interface remain with the application.

Paragraph typography integration with explicit services:

- Root: style mapping, script/grapheme itemization, bindings,
  pinned snapshots, IdentityProvider and FontSource.
- [`style`](style/README.md): document values, UTF-8 spans and typography/paint projections.
- [`fontresource`](fontresource/README.md): bounded immutable bytes and fixed version-1 content/provenance IDs.
- [`paragraph`](paragraph/README.md): reusable preparation, intrinsic measurement, breaking/freezing and
  natural vertical assembly for text and fixed atomic inline boxes.
- [`render`](render/README.md): neutral retained-outline/visual-order services, paint-source mapping
  and retained paragraph proof construction.
- [`adapter/opentype`](adapter/opentype/README.md): loader/face/scale/shaper, identity, retained outlines and
  generic VisualOrder implementation. No public parsed-font accessor.

Root `NewResolver` accepts already loaded `FaceEntry` values, explicit default,
style-rule and fallback references, and an injected `Coverage` service. It does
no loading, parsing or discovery. Applications select and load fonts before
entering the engine; tytui owns its policy, providers and cache in `fontselection`.
Only explicitly configured fallback references may be tried. Faces and coverage
services remain valid for the result lifetime.

Resources expose independent readers, never mutable bytes. Snapshot pins every
configured resource without invoking discovery or reopening paths. Resource request
codes preserve version-1 IDs and are distinct from document CSS numeric weights.
Tytui consumes these packages directly; only versioned font wire records remain
in its replay package. There are no tytui font/face/resource forwarding wrappers.

Glyph-boundary paint is the sole behavior: equivalent resolved typography coalesces
across paint changes, while original paint spans remain separate. One glyph gets
one color at its first logical source anchor; shared anchors share color and
independently anchored marks can differ. No clipping or conflict warnings.
Feature-bearing domains remain separate for khipu's existing clipping/rebasing.
Actual font/size/language/bidi boundaries remain significant; document
Style.Equals remains complete. There is no paint-policy selector or legacy mode.
render.PaintSources maps frozen anchors and the supported explicit-soft-hyphen
replacements to original bytes, rejecting unknown generated transformations.

Development uses sibling cords, khipu, uax, schuko and opentype checkouts.
Neutral tests use fake preloaded faces; adapter tests use sibling OpenType font
fixtures. Tytui additionally needs fontfind for its application selection tests. Run `go test ./...`, `go test -race ./...`, `go build ./...`
and `go vet ./...`, then tytui suites including current replay controls and historical rejection fixtures.

Tytui replay supports experimental version 4 only. It verifies placements in both
vertical modes; historical formats are rejection cases. Current SVG/placement
controls and font identities remain unchanged. Keep tests for ligatures, combining
marks, RTL/fallback/discretionaries, source maps and feature-coordinate domains.

## Styles, paint and inline itemization

The root resolver owns `ProjectStyles`, producing `TypographySpan` values with
color-free `TypographyStyle` styles. The private run-domain identity preserves
relative feature origins without carrying paint through styled.Text. Itemize and
ItemizeInline use the same projection rules and their
internal run styles contain no document Color field. ResolveStyle accepts projected
styles and legacy document Style inputs. Keep document equality and JSON unchanged.
The application retains `PaintSpan` values separately for rendering; tytui's
Result.PaintSpans returns a detached copy. ItemizeWithMetadata remains compatible.
Typography projection does not change document equality or replay formats.

For mixed inline input, use `Resolver.ItemizeInline(ctx, text, spans, atomics)`. Each `contracts.AtomicAnchor` selects one U+FFFC OBJECT REPLACEMENT
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
attached markers bypass them. Removing those coverage checks would be a separate behavior change; font
selection/loading already happens before resolver construction. Atomic-aware paragraph orchestration and
vertical assembly are available through `paragraph.Prepare` and `Prepared.Layout`.

## Service composition

Compose the OpenType implementation explicitly:

```go
services := opentype.Services{}
entry, err := services.Load(ctx, path, nil) // Or (ctx, "", retainedResource).
if err != nil { return err }
resolver, err := typography.NewResolver(ctx, typography.SelectedFonts{
    Faces: []typography.FaceEntry{entry},
    Default: entry.Info.ResourceID,
    Size: size,
}, services)
// Handle err before using resolver.
```

`SelectedFonts` is a closed set, not a search catalog. Every default/rule/fallback
reference is a resource ID. Include exactly one entry per referenced ID, even for
unused rules; duplicates, missing references and unreferenced entries are errors.
Rule and fallback order (including repeated references) is preserved. Table order
has no selection meaning: `Infos` and `Snapshot` use default, rules, then fallbacks,
deduplicated by resource ID. Metadata and warning slices are copied; opaque faces
and immutable resources remain shared. Snapshot never reloads a font.

Services implements Coverage, IdentityProvider and render.Backend, and supplies
`Load` as a direct application entry point. Use `adapter/opentype.NewStream` for
shaping. Tytui callers prepare a `fontselection.Request` with `fontselection.Prepare`
before passing its `Selection` to `pipeline.Run`. Its `Provenance` stays separate
from executable references, and replay converts it through explicit wire records.

This replaces root `LoadSettings`, `Settings`, `Rule`, `FaceLoader`, `LoadFunc`
and the loader/coverage `Services` bundle. The old `discovery` and `adapter/fontfind`
packages have moved to tytui. This is a Go source API change. Replay now supports only experimental version 4;
resource identities and current evidence remain unchanged.

General parser/stream/resource/resolver regressions, including feature-range
and nonzero-cluster-base baselines, live here. Tytui owns selection/provider,
DOM-to-resolver, frozen-glyph, TUI and replay integration tests. Dependency audits
keep generic packages free of concrete backends and all typography production
packages free of fontfind and application selection imports.

## Paragraph assembly and text-only execution

`paragraph.Assemble(ctx, frozen, paragraph.Options{Policy:
paragraph.ProofCompatibility, DefaultEmSize: size})` assembles immutable frozen
lines into a boxed natural vertical list. `Box`, `Lines` and `Materials` return
value geometry; `Frozen` retains the read-only source. Geometry uses fractional
big points with downward-positive Y and a first-line reference baseline. The named
policy preserves tytui proof metrics, minimum 0.2em gap and trailing gap. Logical
width encloses target/occupied widths; glyph ink is not measured. DefaultEmSize
supplies 0.8/0.2/0.2em fallback metrics for lines without measured extents.

Frozen-only Assemble has no rendering, DOM, discovery or concrete font dependency;
callers supply already frozen text output. It supports only ProofCompatibility
and rejects atomic content without geometry. Target-height packing and vertical
glue stretch/shrink are unsupported; CSS line-height requires Run or Prepared.Layout.
Coordinates must remain below
`dimen.Fil` expressed in big points; non-finite metrics, invalid input and overflow
are errors. Tytui retains assembly output in completed results and its glyph
renderer consumes placements without querying line metrics. Result.VList returns
a detached handle; font/paint inspection input cannot reflow an existing proof.
Replay v4 rebuilds and verifies placements for both assembly policies. Rendering colors remain on their separate path.


`paragraph.Run(ctx, input, params, config)` provides prepared styled paragraph
→ boxed vlist. Input carries styled.Text, base Direction and zero-byte Markup;
Params separates Constraints.Width, Breaker and Assembly defaults. Config supplies
khipu shaper/resolver/hyphenation contracts (zero uses khipu's synthetic backend).
Root Itemize/ProjectStyles runs before paragraph execution; input style identities
are preserved. The paragraph engine never re-itemizes or reconstructs styles, normalizes whitespace or loads fonts.
Result retains Frozen/VList, paragraph-relative break endpoints, bidi runs and
counts; errors carry the failed stage and return no partial output. Every call
gets a fresh owner and freezes before assembly. Services must stay valid for
retained results. Typography-only text and separate renderer paint remain the
application contract. The compatibility assembly policy preserves existing paragraph proof geometry.


## CSS-inspired line height

Run and Prepared.Layout support `CSSLineHeight` with `Params.LineLayout`:
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
v4 records explicit assembly mode and placements for both backends; no services
or VList objects are serialized. Color bypass remains independent of layout. Owning tests cover exact geometry,
inheritance/strut, mixed fonts/sizes, zero/negative leading, spaces/replacements,
invalid values/metrics/ranges, cancellation and overflow. Tytui tests ligatures,
RTL/fallback/paint, rendering without metrics, and offline v4 resaves/tampering
alongside unchanged current controls.

## Reusable preparation and atomic layout

`Input.Atomics` carries inline anchors; use keyed struct literals. `Prepare(ctx, input, options, config)` consumes bidi
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

## Intrinsic measurement

The same prepared handle supports ordinary and bounded explicit intrinsic
measurement:

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

`render.BuildParagraphProof(ctx, render.ParagraphInput{Frozen, VList, Text, Paint}, render.Services{Backend, Identity})` walks completed immutable
paragraphs. It returns detached float64 big-point glyph/line geometry, outline
paths, font identities, colors and original processed UTF-8 `SourceByte` anchors.
Glyph color always comes from the retained logical source anchor through PaintSources.
No policy selector remains on ParagraphInput. Keep fonts and scale services stable
and paint separate from shaping; proof construction does not remeasure or reflow.

Proof construction never measures lines, loads fonts, reshapes, rebreaks or stacks
lines. Visual ordering applies once to each detached directional glyph fragment;
frozen knot order stays physical. Retained font/scale services remain stable.
Errors/cancellation return no partial proof, including late outline failures.
Owning tests deny metric queries after layout and verify physical positions,
multibyte anchors, paint, detached ownership and cancellation. Tytui checks real
nested documents and the unchanged static paragraph replay evidence.


## Development and integration checks

Run ordinary/race suites, build and vet in typography and sibling tytui when
changing shared boundaries, including the generic dependency audit, adapter
service regressions and unchanged static paragraph replay fixtures. Cross-module
pipeline changes also require the relevant khipu and css-box-layout checks.
See [the integration record](../khipu/doc/PIPELINE_INTEGRATION.md) and
[document proof fixtures](../tytui/documentlayout/testdata/README.md).

Tytui owns nested scheduling/reuse tests, independent signed geometry/rounding
oracles, resize/failure lifetime sequences and retained real-font fixed/auto
proofs. Prepared-result and render ownership tests live here. CSS types, document
scheduling, normalization and publication stay outside this module.

Independent document layouts prepare afresh; operations sharing a resolver or
prepared owner stay serialized, and retained font/scale services remain stable.
There is no cross-invocation cache or speculative final callback. Automatic
intrinsic hyphenation and successive explicit splitting are unsupported; bounded
explicit splitting is not general CSS min-content/hyphenation conformance. Glyph
coverage stays caller-owned; relocating existing itemizer checks is future work.

Replay simplification is complete (2026-11-16). The application supports only
experimental version 4; font identity encoding is unchanged and independent of
manifest-version support. Historical manifests are rejection cases, not executable
compatibility modes. Current glyph/placement controls and immutable resources
remain contracts. See [tytui offline replay](../tytui/doc/OFFLINE_REPLAY.md) for
case creation and deliberate evidence review. No automatic fixture update or
permanent format migration framework is provided.
