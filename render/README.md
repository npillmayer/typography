# render

Retained paragraph proofs and paint-source mapping through neutral backend services.
`BuildParagraphProof` consumes completed frozen content and its matching VList;
it does not load fonts, query line metrics, reshape, rebreak or restack lines.

```go
proof, err := render.BuildParagraphProof(ctx, render.ParagraphInput{
    Frozen: result.Frozen, VList: result.VList,
    Text: processedText, Paint: paintSpans,
    PaintPolicy: typography.GlyphBoundary,
}, render.Services{Backend: backend, Identity: identity})
// Handle err before using proof; failures return no partial proof.
```

Text/paint ranges use the same processed UTF-8 stream as layout. Glyph clusters
remain scalars; `PaintSources` maps them to original byte anchors. Shared anchors
share paint. The supported generated-text policy is explicit soft-hyphen
replacement; unknown transformations need provenance and are rejected. Legacy
paint uses the frozen knot's start; glyph-boundary paint is the default.

Returned glyph/line slices are detached. Origins and line geometry use big points
with downward-positive Y. Outline paths/control points use font units; glyph
scale converts those units to big points. Bounds include the origin, logical line
rectangles and control points, so they are conservative proof bounds, not tight ink.

Keep borrowed inputs and backend/identity services stable during construction.
`Backend.VisualOrder` receives a detached directional fragment and may reorder it;
frozen knots stay in physical order. Late outline errors and cancellation return
no partial proof. CSS/DOM transforms and drawing atomic objects remain consumer work.

Run `go test ./render` for paint provenance, physical positions, ownership,
cancellation and metric-bypass regressions. Real-font consumers are checked in
sibling tytui. See the [integration guide](../README.md).
