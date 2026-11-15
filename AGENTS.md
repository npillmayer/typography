# Typography integration

## Dependencies and services

Keep shared styles and immutable resources independent of DOM, fontfind and
concrete OpenType types. Reuse khipu/contracts rather than inventing another
abstract font/scale model. Resource request Slant/Weight codes preserve the legacy
identity encoding; they are not document CSS weights. Preserve version-1 hash
formatting, wire values, source coordinates, and immutable ownership.

adapter/opentype owns normal parsing, retained bytes, metrics, scaling, streaming
and outlines. No parsed-font accessor belongs in public boundaries. Lazy queries,
shaping and outlines share the face lock. Preserve synchronous production with
concurrent draining, scalar features/clusters, terminal errors and cancellation.
No relaxed test-font parsing in production.

The agreed glyph-boundary policy permits coalescing equivalent resolved styles
across paint changes. One glyph gets one color; no clipping/conflict warnings. Keep resolution, discovery adaptation and retained consumer services separate.
Put general regressions
in owning packages and verify compatibility through sibling tytui. Run ordinary
and race suites, build and vet in both modules when changing these boundaries.
Adapter tests use sibling opentype font fixtures. Do not commit or push for the user.

Root typography requires injected FaceLoader/Coverage services, stores only
contracts.FontFace with neutral identity/resource metadata, and imports neither
adapter/opentype, fontfind, DOM nor tytui. There is no default backend constructor.
Keep mapping, grapheme/script/fallback and snapshot policy here; test it with fake
services. Tytui composes these services directly; versioned font wire DTOs stay in replay.
ItemizeWithMetadata preserves original paint spans alongside coalesced shaping.
Bidi context and original nonempty feature domains must remain intact.

Neutral discovery owns request policy/cache/evidence and requires Searcher plus an
injected loader. Adapter/fontfind owns provider enum/resource conversion and
ResolverPipeline; no implicit fallback or system/network discovery. Keep provider
and diagnostic ordering, exact-match attestations, explicit substitution and
cancellation/cache ownership covered by owning tests.
OpenType Services owns retained identity, outline-to-path conversion and VisualOrder.
Generic proof consumers use IdentityProvider/render.Backend, never concrete faces.
No backend services are serialized. Tytui's static pre-step-5 manifests protect
version-1 compatibility; do not regenerate them to accommodate accidental changes.

Itemize defaults to GlyphBoundary; ItemizePolicy retains LegacyStyleBoundary for
version-1 replay. Never redefine document equality to drop paint. Shared anchors
inherit cluster paint, distinct mark anchors may differ. PaintSources maps only the
known explicit-soft-hyphen generated-text policy; unknown edits require provenance.
Tytui defaults to replay version 2 for glyph-boundary compatibility layouts, uses
version 3 for explicit line-height and preserves version-1 legacy behavior.
Keep the pre-step6-paint fixture unchanged and run both modules' full suites.

## Styles and paint

The root resolver owns `ProjectStyles`, producing `TypographySpan` values with
color-free `TypographyStyle` styles. The private run-domain identity preserves
relative feature origins and explicit legacy document boundaries without carrying
paint through styled.Text. Itemize/ItemizePolicy use the same projection and their
internal run styles contain no document Color field. ResolveStyle accepts projected
styles and legacy document Style inputs. Keep document equality and JSON unchanged.
The application retains `PaintSpan` values separately for rendering; tytui's
Result.PaintSpans returns a detached copy. ItemizeWithMetadata remains compatible.
Projection does not change document equality or replay formats.

Do not reintroduce tytui's removed fontbackend package or htmlinput style aliases.
OpenType Services implements typography.FaceLoader and Coverage; type assertions
stay inside the adapter. Do not reintroduce application face/resource wrappers.
Discovery.Resolution carries application selection provenance, while the resolver's pinned
Snapshot is typography.Settings. Font wire records/conversions live only in
replay/wire.go; preserve numeric provenance codes and unchanged v1/v2 fixtures.
Run the generic-package dependency audit and adapter service regressions alongside
both modules' ordinary/race/build/vet checks. Keep annotation APIs outside this boundary.

## Paragraph execution and assembly

paragraph.Assemble owns headless frozen-line measurement
and natural boxed-vlist assembly. Its ProofCompatibility policy preserves the
existing tytui proof's fractional big-point operation order, metric maxima, 0.2em
minimum gap and trailing space. DefaultEmSize is an explicit fallback value.
Keep first-baseline geometry and fixed Kern material detached from frozen-content
references; no paint, backend or DOM types enter assembly. Reject non-finite and
out-of-domain geometry rather than narrowing/wrapping integer DU. Empty frozen
output has no baseline; glyph ink may overflow logical boxes. No target-height or
stretch/shrink semantics are implemented. Tytui retains vlist placements and uses them without querying line metrics.
Preserve existing v1/v2 proof evidence and keep rendering colors outside assembly.
Owning tests cover synthetic baselines, mixed sizes, blank lines, overflow,
invalid metrics, cancellation and concurrent ownership. Tytui checks real
mixed-size/fallback placement/rendering, independent synthetic LTR/RTL positions,
source correspondence, input/source-removal stability and unchanged legacy resaves.
Keep compatibility spacing defaults unchanged.

paragraph.Run owns text-only bidi preparation and encode/break/freeze
orchestration, followed by policy-specific assembly. It rejects atomic anchors
at StageInput; use Prepare and Prepared.Layout for mixed input. Input is prepared
styled.Text plus Direction and zero-byte Markup; Params separates Constraints,
Breaker and Assembly defaults. khipu.Config supplies existing service contracts.
Do not import DOM/discovery/adapters or add paint/layout-style reconstruction here.
The root resolver prepares text before paragraph execution; preserve private feature domains and source coordinates.
Each invocation owns fresh encoding/breaking state; never return elastic owners or
partial results. Errors preserve causes and stages. Whitespace normalization and
DOM source maps remain with the input adapter/application. The former tytui internal/layout package
is removed; no forwarding implementation should be reintroduced.


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
height packing are unsupported. Prepared layout accepts fixed atomic border
boxes (already including padding/borders), resolved margins and bottom-margin-edge
alignment; do not confuse this with general CSS box computation. Rendering colors and ink bounds remain separate.
Tytui's headless Input.LineLayout opts in; TUI defaults stay compatible. Replay
v1/v2 omit the new fields and retain old evidence. Explicit line-height cases use
v3 with typed input and numeric placements, no services or VList objects. Both
paint policies remain independent of layout. Owning tests cover exact geometry,
inheritance/strut, mixed fonts/sizes, zero/negative leading, spaces/replacements,
invalid values/metrics/ranges, cancellation and overflow. Tytui tests ligatures,
RTL/fallback/paint, rendering without metrics, and offline v3 resaves/tampering
alongside unchanged legacy fixtures.


## Inline objects and reusable preparation

Resolver.ItemizeInline accepts strictly ordered AtomicAnchor values selecting
U+FFFC at original UTF-8 byte offsets, with unique nonzero IDs. Attached markers
retain bytes/scalar positions but receive no text style or font binding. They
separate grapheme/script/fallback processing and soft-hyphen neighbor selection.
Object-only spans bypass font resolution but retain ordinary input validation;
unattached U+FFFC remains text. No anchors uses the text-only path.

Preserve private projected feature domains, marker scalar positions and emitted
text windows; ResolveStyle clips/rebases shaping defaults per window. Object-only
source spans are domain barriers. Keep returned styles intact. Source/paint/bidi
associations stay with the caller; input slices are neither retained nor modified.
Errors publish no partial itemization. Coverage remains caller-owned: existing
text itemizer checks remain, while attached markers bypass them. No new coverage
pass or relocation of font preparation belongs in paragraph preparation.

paragraph.Prepare resolves bidi and shapes base text once without target width
or object geometry. Prepared.Layout binds fresh material, breaks, freezes and
assembles for each request without reshaping base text. Hyphenation policy is
fixed at preparation. Serialize all operations on one prepared handle and shared
resolver; keep borrowed source/style/service objects stable. Configuration and
request slices are copied/consumed, not arbitrary service/style objects cloned.
Failures return no partial result, leave preparation reusable and cannot mutate
previous results. Detached result slices must not alter retained vlists.

Supply exactly one AtomicBox per anchor in any ID order. Border dimensions are
nonnegative; margins and resulting advances may be signed. AlignBottomMarginEdge
is the supported alignment. Atomic extents unite with text/strut extents without
text half-leading or marker font-metric queries. AtomicPlacements returns detached
border origins in fractional big points, downward-positive Y, ordered by line
and frozen physical traversal. Never sort by X; negative advances and coincident
zero-sized objects are valid. Line indexes are zero-based.

Zero widths are valid. Explicit Horizontal skips replace both breaker defaults,
including zeros, consistently for breaking/freezing; nil retains defaults. Check
primitive geometry, completed sums and published coordinates against dimen.Fil
without narrowing/wrapping. Negative completed line widths are unsupported;
positive overfull lines succeed with diagnostics. Frozen-only Assemble rejects
atomic content without geometry. StagePrepare/StageBind preserve causes; Run
retains historical text-only stages.

## Intrinsic measurement

Prepared.MeasureIntrinsic uses retained horizontal material without vertical
metrics, line-height, struts or rendering. Supply one signed MinAdvance/MaxAdvance
range per atomic ID, ordered and bounded below dimen.Fil in magnitude, including
resolved horizontal margins; text-only requests are empty. Khipu owns natural
widths, ordinary wrapping, edge trimming and complete signed-span normalization;
exclude generated fills/skips/elasticity. Returned widths are finite nonnegative DU.

Support NoHyphenation and bounded ExplicitHyphenation only: one eligible soft
hyphen and at most one break-permitting variant per textbox. AutomaticHyphenation,
multiple positions/variants and successive splitting have no fallback; return
khipu.ErrUnsupportedPolicy. This is not general CSS hyphenation conformance.
Complete replacement pairs are shaped lazily and shared with later layouts;
successful empty batches are cached, failures retryable. Base text is not reshaped,
request storage is not retained, and prior results remain unchanged. Entry
validation/cancellation uses StageInput; evaluation and later cancellation use
StageMeasure. Preserve errors.Is causes and return zero widths on failure.

CSS callbacks and invocation-local preparation reuse belong to tytui/documentlayout;
CSS core owns recursive aggregation, width selection and descendant resolution.
The adapter handles only the requested normalized group and publishes complete
documents after success. Independent layouts prepare afresh. Do not add a
cross-invocation cache or speculative final callback. CSS uses NoHyphenation.

## Retained proofs and verification

render.BuildParagraphProof owns retained paragraph walking for paragraph/document
consumers. Return detached float64 big-point glyph/line geometry, paths, identities,
colors and original processed UTF-8 source anchors. Never measure lines, load
fonts, reshape, rebreak or restack during proof construction. Apply VisualOrder
once per detached directional glyph fragment; frozen knot order stays physical.
Errors/cancellation, including late outline failures, return no partial proof.
Keep font/scale services stable and CSS/DOM transforms in tytui/documentlayout.

Put general regressions in owning packages. Preserve static pre-step-5,
pre-step6-paint and v1/v2/v3 replay fixtures; never regenerate them to conceal
compatibility drift. Keep numeric provenance codes and versioned DTOs in replay.
No backend services or VList objects are serialized. Do not reintroduce removed
tytui internal/layout forwarding code or application face/resource wrappers.

For shared-boundary changes run ordinary/race suites, build and vet in typography
and sibling tytui, including generic dependency audits and adapter regressions.
Cross-module pipeline changes also need relevant khipu/css-box-layout checks.
Cover repeated layouts/measurements, ownership, cancellation/failure recovery,
signed/zero geometry, RTL/fill traversal, metric bypass and real-font feature
domains here; tytui owns nested scheduling, source correspondence, rounding,
resize/lifetime, retained document proofs and replay compatibility integration.
Report checks actually run; historical passing results are not current validation.
Do not commit or push for the user.
