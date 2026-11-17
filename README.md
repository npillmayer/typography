# typography

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
| [layout](layout/README.md) | Completed immutable lines, boxes, material and atomic placements |

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
    resolver, err := glyphing.NewStyleMapper(ctx, glyphing.SelectedFonts{
        Faces: []glyphing.FaceEntry{entry},
        Default: entry.Info.ResourceID, Size: size,
    }, glyphServices)
    if err != nil { return err }
    text, bindings, err := resolver.Itemize(ctx, processedText, spans)

Handle errors, then pass text and resolver to glyphing/paragraph. Construct fresh
streams with glyphot.NewStream and retain bindings for diagnostics.
Retain paint independently with style.ProjectPaint(spans).

After layout completes:

    drawing := renderot.New()
    proof, err := render.BuildParagraphProof(ctx, render.ParagraphInput{
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

Paragraph assembly preserves both policies, fixed atomic geometry, signed
positions, empty-line handling and fractional operation order. VLists copy
geometry at publication/access and share frozen content. Rendering uses retained
placements without metric queries, reshaping, rebreaking or restacking.

Replay is application-owned. Resource IDs, wire values, glyph records and proof
controls remain unchanged. This adds no page layout, font discovery, coverage
policy, rasterization or generalized paint model.

## Verification

Run go test ./..., go test -race ./..., go build ./... and go vet ./... in this
module and sibling tytui. Root dependency audits enforce the branch boundaries.
Unit tests stay with their owners; [integration](integration/) contains cross-phase
tests. Real font fixtures remain in sibling opentype; no fonts are copied.
