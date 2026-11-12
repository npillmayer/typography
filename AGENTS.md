# Typography integration

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
across paint changes. One glyph gets one color; no clipping/conflict warnings. Resolution, discovery adaptation and retained consumer services are extracted. Put general regressions
in owning packages and verify compatibility through sibling tytui. Run ordinary
and race suites, build and vet in both modules when changing these boundaries.
Adapter tests use sibling opentype font fixtures. Do not commit or push for the user.

Root typography requires injected FaceLoader/Coverage services, stores only
contracts.FontFace with neutral identity/resource metadata, and imports neither
adapter/opentype, fontfind, DOM nor tytui. There is no default backend constructor.
Keep mapping, grapheme/script/fallback and snapshot policy here; test it with fake
services. Tytui now composes these services directly; versioned font wire DTOs stay in replay.
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
Tytui writes replay version 2 for new layouts and preserves version-1 old behavior.
Keep the pre-step6-paint fixture unchanged and run both modules' full suites.

Rendering bypass: M owns `ProjectStyles`, producing `TypographySpan` values with
color-free `TypographyStyle` styles. The private run-domain identity preserves
relative feature origins and explicit legacy document boundaries without carrying
paint through styled.Text. Itemize/ItemizePolicy use the same projection and their
internal run styles contain no document Color field. ResolveStyle accepts projected
styles and legacy document Style inputs. Keep document equality and JSON unchanged.
The application retains `PaintSpan` values separately for rendering; tytui's
Result.PaintSpans returns a detached copy. ItemizeWithMetadata remains compatible.
No annotation package or replay version change is introduced.

Step 7 removed tytui's fontbackend package and htmlinput style aliases. OpenType
Services now also implements typography.FaceLoader and Coverage; type assertions
stay inside the adapter. Do not reintroduce application face/resource wrappers.
Discovery.Resolution carries application selection provenance, while M's pinned
Snapshot is typography.Settings. Font wire records/conversions live only in
replay/wire.go; preserve numeric provenance codes and unchanged v1/v2 fixtures.
Run the generic-package dependency audit and adapter service regressions alongside
both modules' ordinary/race/build/vet checks. No annotation API or boxed-vlist
implementation was added by this cleanup.

Paragraph-engine step 1: paragraph.Assemble owns headless frozen-line measurement
and natural boxed-vlist assembly. Its ProofCompatibility policy preserves the
existing tytui proof's fractional big-point operation order, metric maxima, 0.2em
minimum gap and trailing space. DefaultEmSize is an explicit fallback value.
Keep first-baseline geometry and fixed Kern material detached from frozen-content
references; no paint, backend or DOM types enter assembly. Reject non-finite and
out-of-domain geometry rather than narrowing/wrapping integer DU. Empty frozen
output has no baseline; glyph ink may overflow logical boxes. No target-height or
stretch/shrink semantics are implemented. Tytui renderer migration (step 2) is
complete: it retains vlist placements and uses them without querying line metrics.
Preserve existing v1/v2 proof evidence and keep rendering colors outside assembly.
Owning tests cover synthetic baselines, mixed sizes, blank lines, overflow,
invalid metrics, cancellation and concurrent ownership. Tytui checks real
mixed-size/fallback placement/rendering, independent synthetic LTR/RTL positions,
source correspondence, input/source-removal stability and unchanged legacy resaves.
Both modules pass full tests, races, build and vet after the renderer migration.
The reusable paragraph entry point is implemented in step 3; step 4 adds
explicit CSS-inspired line-height. No default spacing semantics changed.

Paragraph-engine step 3: paragraph.Run owns bidi preparation and the former
tytui encode/break/freeze orchestration, followed by Assemble. Input is prepared
styled.Text plus Direction and zero-byte Markup; Params separates Constraints,
Breaker and Assembly defaults. khipu.Config supplies existing service contracts.
Do not import DOM/discovery/adapters or add paint/layout-style reconstruction here.
M prepares text before P; preserve private feature domains and source coordinates.
Each invocation owns fresh encoding/breaking state; never return elastic owners or
partial results. Errors preserve causes and stages. Whitespace normalization and
DOM source maps remain with X/application. The former tytui internal/layout package
is removed; no forwarding implementation should be reintroduced.


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
No vertical-align, inline objects, empty-element boxes, padding, margins or bounded
height packing are implemented. Rendering colors and ink bounds remain separate.
Tytui's headless Input.LineLayout opts in; TUI defaults stay compatible. Replay
v1/v2 omit the new fields and retain old evidence. Explicit line-height cases use
v3 with typed input and numeric placements, no services or VList objects. Both
paint policies remain independent of layout. Owning tests cover exact geometry,
inheritance/strut, mixed fonts/sizes, zero/negative leading, spaces/replacements,
invalid values/metrics/ranges, cancellation and overflow. Tytui tests ligatures,
RTL/fallback/paint, rendering without metrics, and offline v3 resaves/tampering
alongside unchanged legacy fixtures. Full tests/races/build/vet pass in both modules.
