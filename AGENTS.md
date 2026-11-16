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

Root typography accepts preloaded SelectedFonts and injected Coverage through
NewResolver. It stores contracts.FontFace with neutral identity/resource metadata
and imports neither adapter/opentype, fontfind, DOM nor tytui. No loading callback
or default backend belongs here. Validate a closed entry table: one entry for each
referenced resource ID, no duplicates/missing/unreferenced entries. Canonical
snapshot/Info order is default, rules, fallbacks; retain role order and repeated
fallback references. Copy metadata/warnings and share only stable faces/resources.
Construction performs no I/O, parsing or coverage query. Keep mapping,
grapheme/script/fallback and snapshot policy here; test it with fake preloaded
faces. Tytui prepares fonts before layout; versioned wire DTOs stay in replay.
ItemizeWithMetadata preserves original paint spans alongside coalesced shaping.
Bidi context and original nonempty feature domains must remain intact.

Application package tytui/fontselection owns request policy/cache/evidence,
fontfind enum/resource conversion and controlled ResolverPipeline use. Typography
has no discovery or adapter/fontfind package. No implicit fallback or system/network
discovery is allowed. Keep provider/diagnostic ordering, exact-match attestations,
explicit substitution, terminal read/parse errors and cancellation/cache ownership
covered by application tests. The upstream read allocates before the resource bound;
providers must remain trusted and offline.
OpenType Services owns retained identity, outline-to-path conversion and VisualOrder.
Generic proof consumers use IdentityProvider/render.Backend, never concrete faces.
No backend services are serialized. Current replay controls protect output;
historical pre-step manifests remain unchanged as version-rejection cases.

Itemize and ItemizeInline always coalesce equivalent shaping across paint changes.
ProjectStyles takes text, spans, resolver and defaults, with no policy argument.
PaintPolicy, GlyphBoundary, LegacyStyleBoundary and ItemizePolicy are removed.
Shared anchors inherit cluster paint; distinct mark anchors may differ. PaintSources
maps the supported explicit-soft-hyphen transformations; unknown edits require
provenance. No paint enters styled.Text. Tytui replay accepts only version 4;
historical formats are rejected. Preserve current proof/placement controls and
run both modules' full suites.

## Styles and paint

The root resolver owns `ProjectStyles`, producing `TypographySpan` values with
color-free `TypographyStyle` styles. The private run-domain identity preserves
relative feature origins without carrying paint through styled.Text. Itemize and
ItemizeInline use the same projection rules and their
internal run styles contain no document Color field. ResolveStyle accepts projected
styles and legacy document Style inputs. Keep document equality and JSON unchanged.
The application retains `PaintSpan` values separately for rendering; tytui's
Result.PaintSpans returns a detached copy. ItemizeWithMetadata remains compatible.
Projection does not change document equality or replay formats.

Do not reintroduce tytui's removed fontbackend package or htmlinput style aliases.
OpenType Services.Load prepares FaceEntry values; Services implements Coverage.
Concrete type assertions stay inside the adapter. Do not reintroduce application
face/resource wrappers. Fontselection.Selection carries SelectedFonts and detached
application Provenance; the resolver's Snapshot is *typography.SelectedFonts.
Root path-bearing Settings/Rule, LoadSettings, FaceLoader/LoadFunc and the loader
Services bundle are retired. Replay owns explicit input/policy/descriptor/font
wire records; preserve numeric provenance codes, Fonts:null and current controls. Load validates every stored font but only passes referenced faces to the
resolver. Requested paths/descriptors are never executable on replay; retain the
requested-path provenance through load/save, without discovery.
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
Preserve current proof evidence and keep rendering colors outside assembly.
Owning tests cover synthetic baselines, mixed sizes, blank lines, overflow,
invalid metrics, cancellation and concurrent ownership. Tytui checks real
mixed-size/fallback placement/rendering, independent synthetic LTR/RTL positions,
source correspondence, input/source-removal stability and current-format resaves.
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
v4 records explicit assembly mode and placements for both backends; no services
or VList objects are serialized. Color bypass remains independent of layout. Owning tests cover exact geometry,
inheritance/strut, mixed fonts/sizes, zero/negative leading, spaces/replacements,
invalid values/metrics/ranges, cancellation and overflow. Tytui tests ligatures,
RTL/fallback/paint, rendering without metrics, and offline v4 resaves/tampering
alongside unchanged current controls.


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

Put general regressions in owning packages. Preserve current replay proof controls
and static historical rejection fixtures; never regenerate them to conceal drift. Keep numeric provenance codes and versioned DTOs in replay.
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

Replay simplification is complete (2026-11-16). The application supports only
experimental version 4; font identity encoding is unchanged and independent of
manifest-version support. Historical manifests are rejection cases, not executable
compatibility modes. Current glyph/placement controls and immutable resources
remain contracts. See [tytui offline replay](../tytui/doc/OFFLINE_REPLAY.md) for
case creation and deliberate evidence review. No automatic fixture update or
permanent format migration framework is provided.
