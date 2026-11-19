## glyphing/

Context policy (2026-11-19): typesetting, geometry, retained proofs, replay,
and document execution are synchronous and have no context argument. Contexts
are confined to streaming coordination and font discovery/loading. Streaming
callers create a local background context; consumer failure still cancels and
joins the producer before cleanup. Errors publish no partial result and preserve
previous immutable output. TUI layout jobs finish serially; stale results are
discarded, while font work remains cancellable. This supersedes earlier
cancellation requirements for typesetting in this document.
# Paragraph

Package "paragraph" orchestrates the translation of paragraphs of text into
lines of glyphs. It governs preparation, intrinsic measurement, line
breaking/freezing and natural vertical assembly using injected *khipu* contracts
(see module [npillmayer/khipu](https://github.com/npillmayer/khipu) ).
A khipu is akin to TeX's *hlist*, which we will break into line-boxes
and then stack into a *vlist*/*vbox*.

Input to this package is an already prepared `styled.Text`: we preserve its private
style identities and original UTF-8 coordinates.
This package does not load/discover fonts, normalize text, carry paint, or parse CSS.
Styles are not (necessarily) CSS styles, but as far as this package is concerned,
opaque character properties.

Assembly implements [VList assembly](../../doc/VLIST_CONTRACT.md).

## Top-level Entry Points

| Task | Function(s) |
| --- | --- |
| Lay out text once | `Run` |
| Measure or lay out the same text at several widths, or include inline objects | `Prepare`, then `Prepared.MeasureIntrinsic` / `Prepared.Layout` |
| Flow a paragraph through frames, possibly of different widths | `Prepared.StartFlow`, then `Continuation.Fit` |
| Divide an existing layout without rebreaking | `Fragment`, then `SplitFragment` or `FitFragment` |

## Lay out Text once

Start with itemized text and a matching style resolver. The
[glyphing guide](../README.md) describes `StyleMapper`; the
[module example](../../README.md#explicit-opentype-composition) shows font loading
and itemization. Keep the returned `styled.Text` intact.

The snippets below are function bodies returning `error`. They assume a nonnil
itemized `text`, and its `mapper` backed by the OpenType adapter. Import
`paragraph` from `github.com/npillmayer/typography/glyphing/paragraph`, `glyphot`
from `github.com/npillmayer/typography/glyphing/adapter/opentype`, and `khipu`,
`khipu/contracts`, `khipu/dimen`, `uax/bidi` from `github.com/npillmayer/`.
The examples also use `fmt` and `github.com/npillmayer/typography/layout`.

```go
config := khipu.Config{Streaming: &khipu.StreamingConfig{
    Resolver: mapper,
    Defaults: mapper.Defaults(), // Explicit default face and positive em size.
    NewShaper: func() (contracts.StreamingShaper, error) {
        return glyphot.NewStream(32) // Fresh stream on every call.
    },
}}
input := paragraph.Input{Text: text, Direction: bidi.LeftToRight}
params := paragraph.Params{
    Constraints: paragraph.Constraints{Width: 240 * dimen.BP},
    Breaker: paragraph.FirstFit,
    Assembly: paragraph.Options{Policy: paragraph.CSSLineHeight},
    LineLayout: &paragraph.LineLayout{
        Default: paragraph.LineHeight{Kind: paragraph.NormalHeight},
    },
}
result, err := paragraph.Run(input, params, config)
if err != nil { return err }
fmt.Printf("%d lines, %.2f bp high\n",
    len(result.VList.Lines()), result.VList.Box().Extent)
```

Other backends supply their own compatible resolver, default face and shaper
factory through the same contracts. Khipu manages streaming and cleanup.
`Run` accepts text only; use `Prepare` for attached inline objects.
Use `bidi.RightToLeft` for an RTL paragraph; `Input.Markup` optionally supplies
zero-byte bidi isolates at processed-text byte positions.

Choose `FirstFit` for greedy breaks or `KnuthPlass` for paragraph optimization.
Both the breaker and `CSSLineHeight` policy must be explicit. Widths and object
dimensions use `dimen.DU`; `240 * dimen.BP` means 240 big points. Returned float
coordinates use fractional big points, with Y increasing downward. Zero widths
are valid; dimensions must stay within the finite domain below `dimen.Fil`.

`Result.VList` provides `Box()`, `Lines()` and `AtomicPlacements()`;
`Result.Frozen` retains horizontal glyph content. `BreakPositions` reports UTF-8
byte endpoints and `BidiRuns` supplies diagnostics. Geometry is logical, so ink
may overflow it. To draw the result, use
[render.BuildParagraphProof](../../render/README.md) with separately retained paint
and rendering services.

### Set Line Height

Always supply `LineLayout`, including for empty text. Its default can be:

| Value | Meaning |
| --- | --- |
| `LineHeight{Kind: NormalHeight}` | Font ascent + descent + nonnegative font gap |
| `LineHeight{Kind: FactorHeight, Factor: 1.2}` | 1.2 times each receiving run's em size |
| `LineHeight{Kind: LengthHeight, Length: 14 * dimen.BP}` | A resolved absolute line height |

Names in this table are in `paragraph`. Optional `LineLayout.Runs` entries are
`LineHeightSpan{Start, End, Height}` overrides on ordered, nonoverlapping UTF-8
byte ranges; gaps use the default. Resolve CSS percentages/inheritance before
calling. Line boxes include their leading; there is no extra interline/trailing
gap. A tall inline object can enlarge a line beyond the requested text height.

## Prepare once and lay out repeatedly

Use `Prepare` when the same text needs measurement, resizing or frame flow.
It shapes base text and resolves bidi without requiring a target width.

```go
prepared, err := paragraph.Prepare(input,
    khipu.PrepareOptions{Hyphenation: khipu.NoHyphenation}, config)
if err != nil { return err }
request := paragraph.LayoutRequest{Params: params}
for _, width := range []dimen.DU{240 * dimen.BP, 180 * dimen.BP} {
    request.Params.Constraints.Width = width
    resized, err := prepared.Layout(request)
    if err != nil { return err }
    fmt.Printf("width %v: %d lines\n", width, len(resized.VList.Lines()))
}
```

Each layout returns independent immutable output without reshaping base text.
Hyphenation is fixed at preparation: enabled modes require `config.Hyphenator`;
replacement text may be shaped lazily. `NoHyphenation` is the simplest choice
and is required for frame flow.

`LayoutRequest.Horizontal == nil` uses the chosen breaker's default side skips.
To override both, provide `&paragraph.HorizontalLayout{LeftSkip: ..., RightSkip: ...}`
with `khipu.KnotCore` values using `W`, `MinW`, `MaxW`. An empty `HorizontalLayout`
explicitly sets both skips to zero.

## Measure intrinsic Widths

Measurement needs preparation but no target width, line-height or object heights:

```go
widths, err := prepared.MeasureIntrinsic(khipu.MeasureRequest{})
if err != nil { return err }
fmt.Printf("min-content %v; max-content %v\n", widths.MinContent, widths.MaxContent)
```

The result is in DU: min-content takes supported optional breaks, max-content
suppresses them; both respect mandatory breaks. For mixed input, supply exactly
one `khipu.AtomicIntrinsic` per attached ID in `MeasureRequest.Atomics`, with
`MinAdvance` and `MaxAdvance` including horizontal margins. These signed,
ordered bounds describe object widths, not stretch/shrink capacities.

Measurement supports `NoHyphenation` and bounded `ExplicitHyphenation` (one
eligible soft-hyphen position and at most one permitting variant per textbox).
Unsupported policies return `khipu.ErrUnsupportedPolicy`; no approximate fallback
is substituted. Successful lazy replacements are shared with later layouts.

## Include inline Objects

Represent each object by U+FFFC and a unique nonzero ID at its UTF-8 byte offset.
Use `ItemizeInline` so the marker is excluded from font selection, then pass the
same strictly ordered anchors to `Prepare`:

```go
anchors := []contracts.AtomicAnchor{{ID: 1, ByteOffset: 2}}
mixedText, _, err := mapper.ItemizeInline("A \uFFFC B", nil, anchors)
if err != nil { return err }
mixed, err := paragraph.Prepare(paragraph.Input{
    Text: mixedText, Direction: bidi.LeftToRight, Atomics: anchors,
}, khipu.PrepareOptions{Hyphenation: khipu.NoHyphenation}, config)
if err != nil { return err }
withObject, err := mixed.Layout(paragraph.LayoutRequest{
    Params: params,
    Atomics: []paragraph.AtomicBox{{
        ID: 1, BorderWidth: 30 * dimen.BP, BorderHeight: 20 * dimen.BP,
        Margin: paragraph.AtomicMargins{Left: 2 * dimen.BP},
        Alignment: paragraph.AlignBottomMarginEdge,
    }},
})
if err != nil { return err }
fmt.Println(withObject.VList.AtomicPlacements())
```

Supply exactly one `AtomicBox` per ID on every layout, in any ID order. Border
sizes include padding/borders and are nonnegative; resolved margins may be
signed. Only `AlignBottomMarginEdge` is supported: the bottom margin edge meets
the line baseline. Returned placements are border origins in physical traversal
order; do not sort them by X. The application lays out and paints object interiors.

## Checkpoint-based Frame Flow

Prepare with `NoHyphenation`, then start a flow at the first frame's width:

```go
frames := []paragraph.Frame{
    {Width: 240 * dimen.BP, Height: 120}, // Height is in big points, not DU.
    {Width: 180 * dimen.BP, Height: 180},
}
flowRequest := paragraph.LayoutRequest{Params: params}
flowRequest.Params.Constraints.Width = frames[0].Width
token, err := prepared.StartFlow(flowRequest)
if err != nil { return err }
var placed []*layout.Fragment
for _, frame := range frames {
    step, err := token.Fit(frame)
    if err != nil { return err } // The input token remains retryable.
    placed = append(placed, step.Placed)
    token = step.Continuation
    if token == nil { break }
}
fmt.Printf("%d frames visited; complete: %t\n", len(placed), token == nil)
```

A nonnil final token means unfinished content: retain it for more frames or retry.
`FitNoProgress` means no line fits; offer more height or stop rather than retrying
forever. `FitContinues` means some content was placed; `FitComplete` ends the flow.
`Overfull` reports horizontally overfull line indexes in that call's frozen owner,
including unplaced lines; vertical completion does not imply horizontal fit.

Keep each `Placed` fragment independently and add the frame's origin when drawing
it with `render.BuildFragmentProof`. Equal widths reuse retained lines (`Reused`
is true). A width change preserves accepted breakpoints and rebreaks the suffix
using first-fit, even after an initial Knuth–Plass layout. Paragraph styles, bidi
context, line-height, skips and object boxes stay fixed; frame styles do not alter
text. Changed layout inputs require a new flow, and source/style/font changes
require new preparation. `token.Remainder()` gives the current frozen tail.

Process paragraphs in turn. After completion or abandonment, drop the preparation
and **all** old tokens; retain only frozen fragments and the stable services they
need. There is no `Close` or implicit cross-paragraph cache. Frame scheduling,
surrounding margins, widows/orphans and fragmentation inside objects belong to
the application; this API places whole lines without height packing.

## Equal-Width Height Cuts

This is something akin to TeX's `\vsplit`<*n*>` to `<*dimen*>.
For an existing `VList`, use `Fragment(result.VList)` to get a whole view.
`SplitFragment(whole, at)` splits before local line `at`, which must be an
interior boundary (`0 < at < len(whole.Lines())`). Both halves retain full line
heights; the second is rebased to local Y=0. You can split either half again.

To select a cut by height without retaining preparation, use `FitFragment`.
Capture a `FragmentReuseKey` alongside the original layout request. This example
uses explicit zero side skips so the key's skip triples are also zero:

```go
fixedRequest := paragraph.LayoutRequest{
    Params: params, Horizontal: &paragraph.HorizontalLayout{},
}
fixed, err := prepared.Layout(fixedRequest)
if err != nil { return err }
key := paragraph.FragmentReuseKey{
    Width: fixedRequest.Params.Constraints.Width,
    Source: 1, Styles: 1, Fonts: 1, LineHeight: 1, Atomics: 1, Policy: 1,
}
whole, err := paragraph.Fragment(fixed.VList)
if err != nil { return err }
fit, err := paragraph.FitFragment(whole, key, key, 120)
if err != nil { return err }
fmt.Println(fit.Status, fit.Placed.Box(), fit.Remainder.Box())
```

The nonzero IDs are caller-managed revisions; change the relevant ID when an
input changes. Nonzero skips must be recorded as their original
`[natural, minimum, maximum]` DU triples. Source and target keys must match exactly;
they attest inputs that geometry alone cannot verify. Mismatch returns
`ErrIncompatibleFragment`. Use `Continuation.Fit` for owners containing different
line widths. Both fitting APIs share the completion statuses described above.
`FragmentFit.Overfull` also includes unplaced lines.

Neither splitting nor fitting calls shaping, metrics or the breaker. For individual
full-height line items instead, see `layout.Unbox` in the
[layout guide](../../layout/README.md). Surrounding margins remain application-owned.

## Ownership and Errors

Serialize operations sharing a `Prepared`, mapper or services. Keep borrowed
source styles and font services stable for prepared and frozen results. Request
slices are consumed/copied, and geometry accessors return detached slices; this
does not deep-copy service implementations. Earlier results remain unchanged
when another operation succeeds or fails.

Check errors before consuming results: failures publish no partial output and
remain retryable. Use `errors.As(err, &stageErr)` with `var stageErr *paragraph.Error`
to inspect the stage of preparation/layout/measurement/flow failures, and
`errors.Is` for underlying causes. Standalone fragment operations return errors
without the paragraph-stage wrapper. Operations run synchronously to completion and preserve backend error causes. Known mixed-bidi preparation rejection/stall cases limit
supported input; see [the documented limits](../../doc/FLOW.md#bidi-and-shaping).

For architecture and design rationale, see
[paragraph building and glyphing](../../doc/PARAGRAPH_DESIGN.md).
