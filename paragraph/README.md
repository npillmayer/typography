# paragraph

Headless preparation, intrinsic measurement, line breaking/freezing and natural
vertical assembly using injected khipu contracts. Input is already prepared
`styled.Text`: preserve its private style identities and original UTF-8 coordinates.
This package does not load/discover fonts, normalize text, carry paint, or parse CSS.

## Entry points

- `Run` is the text-only convenience path with fresh state per call; it rejects
  atomic anchors at `StageInput`.
- `Prepare` resolves bidi and shapes base text once. `Prepared.Layout` binds fresh
  material for each width/atomic geometry request without reshaping base text.
- `Prepared.MeasureIntrinsic` measures retained horizontal material without
  vertical metrics or final object heights.
- `Assemble` accepts already frozen text and supports `ProofCompatibility` only.
  Atomic content needs `Prepared.Layout` with geometry.

Given prepared `input`, injected `config`, and a positive finite `emSize`:

```go
prepared, err := paragraph.Prepare(ctx, input,
    khipu.PrepareOptions{Hyphenation: khipu.NoHyphenation}, config)
if err != nil {
    return err
}
result, err := prepared.Layout(ctx, paragraph.LayoutRequest{
    Params: paragraph.Params{
        Constraints: paragraph.Constraints{Width: 200 * dimen.BP},
        Breaker: paragraph.FirstFit,
        Assembly: paragraph.Options{
            Policy: paragraph.ProofCompatibility, DefaultEmSize: emSize,
        },
    },
})
// Handle err before consuming result.Frozen and result.VList.
```

For mixed input, provide strictly ordered `Input.Atomics` identifying U+FFFC byte
offsets and exactly one `AtomicBox` per ID in each layout request. Border sizes
already include padding/borders; resolved margins may be signed. Only
`AlignBottomMarginEdge` is supported. Placements follow physical traversal, never
sorted X order. Zero widths/sizes are valid; negative completed line widths are not.
Explicit horizontal skips override both defaults, including zeros.

## Geometry and lifetime

Input dimensions use DU; published geometry uses fractional big points with Y
downward. Validate finite domains below `dimen.Fil` without narrowing/wrapping.
Logical boxes are separate from glyph ink, which may overflow. Compatibility
spacing preserves fractional operation order, the 0.2em minimum gap and trailing
kern. `CSSLineHeight` instead requires `LineLayout` and explicit streaming
font/size/resolver defaults for its strut. Normal, factor and resolved length
values produce signed half-leading extents, with no compatibility gap. CSS
parsing/inheritance and height-constrained packing stay external.

Serialize operations on each prepared handle. Borrowed source/styles/services
must remain stable; copied configuration/request slices do not deep-clone services.
Failures preserve causes/stages, publish no partial result, and leave previous
results unchanged. VList accessors detach geometry; `Frozen` shares read-only
content and stable font/scale services.

Intrinsic requests supply one signed advance range per atomic ID, including
horizontal margins. Results are finite nonnegative DU. `NoHyphenation` and bounded
`ExplicitHyphenation` are supported: at most one eligible soft-hyphen position and
one permitting variant per textbox. Automatic or successive/multiple splitting
is unsupported. Complete lazy replacement batches are shared with later layouts;
failed batches remain retryable. Entry failures use `StageInput`; evaluation and
later cancellation use `StageMeasure`.

Run `go test ./paragraph` and `go test -race ./paragraph`. Tests cover geometry,
metrics, CSS struts/spaces, atomic traversal, measurement, cancellation and ownership;
real-font integration also lives in `adapter/opentype` and sibling tytui. See the
[integration guide](../README.md) for service composition and replay compatibility.
