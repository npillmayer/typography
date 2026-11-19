# typography

Context policy (2026-11-19): typesetting, geometry, retained proofs, replay,
and document execution are synchronous and have no context argument. Contexts
are confined to streaming coordination and font discovery/loading. Streaming
callers create a local background context; consumer failure still cancels and
joins the producer before cleanup. Errors publish no partial result and preserve
previous immutable output. TUI layout jobs finish serially; stale results are
discarded, while font work remains cancellable. This supersedes earlier
cancellation requirements for typesetting in this document.

Typography turns styled text and preselected fonts into immutable paragraph
layouts, then provides a separate path for rendering retained glyph-outline
proofs. It supports CSS line heights, fixed inline objects and bounded flow across
frames. Applications own input normalization, font discovery, surrounding box
layout and paint selection.

Typography has two independent branches:

- [glyphing](glyphing/README.md) resolves selected fonts and itemizes text.
  [glyphing/paragraph](glyphing/paragraph/README.md) owns preparation,
  intrinsic measurement, breaking/freezing and vertical assembly.
- [render](render/README.md) consumes completed immutable layouts and separate
  paint. It builds retained glyph-outline proofs without reflow or font discovery.

The root package is documentation only. There are no compatibility aliases for
the former root resolver, paragraph or adapter/opentype APIs.

## Shared data

| Package | Ownership |
| --- | --- |
| [font](font/README.md) | Detached identity, retained-resource capability and font-source contract |
| [fontresource](fontresource/README.md) | Immutable bounded bytes, provenance and version-1 resource IDs |
| [style](style/README.md) | Document styles, typography values, paint and UTF-8 span projections |
| [layout](layout/README.md) | Completed immutable geometry and reusable VItem/ElasticVList data |

Shared packages import neither active branch. Glyphing imports no rendering
package; rendering imports no glyphing package. Generic code imports no concrete
backend. The module still uses khipu's frozen-result and font contracts; this is
not a new wire format or an independent copy of those contracts.

## Explicit OpenType composition

Import glyphing and glyphing/paragraph for execution. Use two backend imports:

    glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
    renderot "github.com/npillmayer/typography/render/adapter/opentype"

Prepare selected faces and itemize:

    glyphServices := glyphot.Services{}
    entry, err := glyphServices.Load(ctx, path, nil)
    if err != nil { return err }
    resolver, err := glyphing.NewStyleMapper(glyphing.SelectedFonts{
        Faces: []glyphing.FaceEntry{entry},
        Default: entry.Info.ResourceID, Size: size,
    }, glyphServices)
    if err != nil { return err }
    text, bindings, err := resolver.Itemize(processedText, spans)

Handle errors, then pass text and resolver to glyphing/paragraph. Construct fresh
streams with glyphot.NewStream and retain bindings for diagnostics.
Retain paint independently with style.ProjectPaint(spans).

After layout completes:

    drawing := renderot.New()
    proof, err := render.BuildParagraphProof(render.ParagraphInput{
        Frozen: result.Frozen, VList: result.VList,
        Text: processedText, Paint: paint,
    }, render.Services{Backend: drawing, Identity: drawing})

The glyphing adapter owns loading, coverage, metrics, scaling and shaping.
The rendering adapter owns an independent lazy outline-parser cache, path
conversion and visual ordering. It reads retained bytes through font.ResourceFace,
never a glyphing parser or source file. Its cache has explicit service lifetime,
not global scope. Both phases share stable resources and FontInstance bindings.

Font discovery and request policy remain in application fontselection. Only
explicit fallback resources participate in itemization.

## Preserved behavior

Glyphing retains the Latin/Hebrew profile and whole-grapheme nominal fallback.
Paint does not enter styled.Text. Equivalent typography coalesces across paint
boundaries while feature domains remain intact. ItemizeWithMetadata returns text,
font bindings and shaping metadata only; use style.ProjectPaint separately.

The [VList contract](doc/VLIST_CONTRACT.md) is CSSLineHeight only, with
tycli/documentlayout as the primary integration. Line boxes include their leading;
no additional interline/trailing gap is inserted. Compatibility assembly and frozen-only Assemble have been removed. Keep fixed atomic geometry, signed positions,
empty-line handling and the current CSS fractional operation order. VLists copy
geometry at publication/access and share frozen content. Rendering uses retained
placements without metric queries, reshaping, rebreaking or restacking.

`layout.Unbox(vlist)` recovers an immutable `ElasticVList` of full-height line
items without remeasurement. Each `VItem` retains frozen horizontal content and
local geometry independently of the VList. Surrounding block margins remain with
the application. Unboxing has no context argument; restacking, rendering of the
new items and stretch/shrink packing remain deferred.

`paragraph.FitFragment` selects a full-line prefix fitting an available height
for compatible equal-width layout, returning immutable placed/remainder fragments.
Its explicit reuse key retains no preparation or services. It performs no font
queries, binding or rebreaking.
See [equal-width height cuts](glyphing/paragraph/README.md#equal-width-height-cuts).

`Prepared.StartFlow` and `Continuation.Fit` add bounded NoHyphenation frame flow:
retain equal-width lines, or lock accepted breakpoints and fully assemble a
first-fit continuation at a new width. Keep preparation only until each paragraph
finishes, then retain its frozen fragments. See [frame flow](glyphing/paragraph/README.md#checkpoint-based-frame-flow).
`render.BuildFragmentProof` renders retained ranges without relayout; tytui's
documentlayout composes positioned fragments and fixed opaque interiors for
proof/SVG export. Tytui/documentlayout now supplies bounded single-paragraph frame scheduling and
`tycli --frames` export. P6 validates this opt-in, bounded profile; see the
[production cost report](doc/FLOW_COSTS.md) for measurements and remaining limits.

Replay is application-owned. Resource identities and CSS proof controls remain unchanged;
current replay is CSS-only version 5 and rejects earlier formats. This adds no page layout, font discovery, coverage
policy, rasterization or generalized paint model.

## Verification

Run go test ./..., go test -race ./..., go build ./... and go vet ./... in this
module and sibling tytui. Root dependency audits enforce the branch boundaries.
Unit tests stay with their owners; [integration](integration/README.md) contains cross-phase
tests. Real font fixtures remain in sibling opentype; no fonts are copied.

The [archived flow experiment](doc/history/flowexperiment/README.md) retains historical
E1–E3 evidence. Current production measurement commands and metric contracts are
in [the P6 harness guide](doc/flow-cost/README.md); historical evidence is not
regenerated as part of routine verification.
