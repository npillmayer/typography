# glyphing/paragraph

Headless preparation, intrinsic measurement, line breaking/freezing and natural
vertical assembly using injected khipu contracts. Input is already prepared
`styled.Text`: preserve its private style identities and original UTF-8 coordinates.
This package does not load/discover fonts, normalize text, carry paint, or parse CSS.

Assembly implements [CSS-only VList assembly](../../doc/VLIST_CONTRACT.md),
with sibling tycli/documentlayout as the primary integration. ProofCompatibility, DefaultEmSize and the frozen-only Assemble API are removed.

## Current entry points

- `Run` is the text-only convenience path with fresh state per call; it rejects
  atomic anchors at `StageInput`.
- `Prepare` resolves bidi and shapes base text once. `Prepared.Layout` binds fresh
  material for each width/atomic geometry request without reshaping base text.
- `Prepared.MeasureIntrinsic` measures retained horizontal material without
  vertical metrics or final object heights.
- `Fragment` makes an immutable whole-VList view; `SplitFragment` partitions it
  at an explicit interior line boundary without metrics, shaping or rebreaking.


Given prepared `input` and injected `config` with an explicit streaming resolver,
default font and positive finite em size:

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
            Policy: paragraph.CSSLineHeight,
        },
        LineLayout: &paragraph.LineLayout{
            Default: paragraph.LineHeight{Kind: paragraph.NormalHeight},
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
Logical boxes are separate from glyph ink, which may overflow.
`CSSLineHeight` requires `LineLayout` and explicit streaming font/size/resolver
defaults for its strut. Normal, factor and resolved length values produce signed
half-leading extents inside each line box. There is no extra interline or trailing
gap. CSS parsing/inheritance and height-constrained packing stay external.
There is no metricless fallback-size policy. Supply explicit CSS inputs even for
synthetic and empty paragraphs; frozen content alone lacks source styles for spaces.

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

Run `go test ./glyphing/paragraph` and `go test -race ./glyphing/paragraph`. Tests cover geometry,
metrics, CSS struts/spaces, atomic traversal, measurement, cancellation and ownership;
real-font integration also lives in `adapter/opentype` and sibling tytui. See the
[integration guide](../../README.md) for service composition and replay compatibility.

Result.VList is a shared layout.VList. Private assembly state is validated and
copied on publication; geometry types live in typography/layout.


## Existing vertical fragmentation

These APIs retain positioned geometry; they are not the unboxed sequence needed
for reuse as individual items. Use `layout.Unbox(result.VList)` to recover an
immutable ElasticVList of full-height VItems without shaping or metric queries.
Surrounding block margins are application-owned. Restacking remains deferred.
Line/material placement anchors are now identical at publication; independently
rounded copies are rejected before splitting.

`Fragment(ctx, result.VList)` preserves whole-paragraph geometry exactly.
`SplitFragment(ctx, fragment, at)` returns two immutable fragments, where at is
the first line of the second fragment in local indexes. Endpoints, empty input
and invalid geometry fail with `contracts.ErrIllegalArguments`; cancellation
preserves its cause. Both split results are nil on any failure.

The split retains every CSS line box, including its leading. It rebases the second fragment's tops, baselines and atomic Y
origins together. CSS leading remains inside its original line box. Local line
indexes change; original line identities, horizontal content, knot/source
coordinates and physical order do not. Splitting an already split fragment is
supported. Results and accessor slices are detached; the frozen owner remains
shared and immutable.

This is an explicit geometry operation, not automatic pagination. It neither
clips ink nor fragments inline objects. Available horizontal width must remain
unchanged. Rendering integration and target-height packing remain deferred; see
[the VList plan](../../doc/VLIST_PLAN.md).
