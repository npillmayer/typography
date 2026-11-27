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
| Lay out text once | `Prepare`, then `Prepared.Layout` |
| Measure or lay out the same text at several widths, or include inline objects | `Prepare`, then `Prepared.MeasureIntrinsic` / `Prepared.Layout` |
| Retain independent candidates and assemble one later | `Prepared.Trial`, `Trial.Plan`, `Trial.Assemble` |
| Lock accepted lines and search a changed suffix | `Trial.Checkpoint`, then `Prepared.Trial` with `Prefix` |
| Inspect all fitting line prefixes and choose one | `Trial.FitPrefixes`, `PrefixFit.Prefixes`, `PrefixFit.Select` |
| Reuse fitted geometry at another height | `PrefixFit.Refit` |
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
prepared, err := paragraph.Prepare(input, khipu.PrepareOptions{Hyphenation: khipu.NoHyphenation}, config)
if err != nil { return err }
result, err := prepared.Layout(paragraph.LayoutRequest{Params: params})
if err != nil { return err }
fmt.Printf("%d lines, %.2f bp high\n",
    len(result.VList.Lines()), result.VList.Box().Extent)
```

Other backends supply their own compatible resolver, default face and shaper
factory through the same contracts. Khipu manages streaming and cleanup.
Use the same preparation path for text and attached inline objects.
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
replacement text may be shaped lazily and complete discoveries are shared
with subsequent measurement/layout calls. Selected breaks remain private to
each layout trial. `NoHyphenation` is the simplest choice
and is required for frame flow.

`LayoutRequest.Horizontal == nil` uses the chosen breaker's default side skips.
To override both, provide `&paragraph.HorizontalLayout{LeftSkip: ..., RightSkip: ...}`
with `khipu.KnotCore` values using `W`, `MinW`, `MaxW`. An empty `HorizontalLayout`
explicitly sets both skips to zero.

## Retain independent paragraph trials

`Layout` searches and assembles immediately. Use `Trial` to retain decisions
before paying for freezing and CSS assembly. It accepts the same layout inputs:

```go
request := paragraph.LayoutRequest{Params: params}
a, err := prepared.Trial(paragraph.TrialRequest{Layout: request})
if err != nil { return err }
request.Params.Constraints.Width = 180 * dimen.BP
b, err := prepared.Trial(paragraph.TrialRequest{Layout: request})
if err != nil { return err } // A would also survive this failure.
fmt.Println(len(a.Plan().Lines()), len(b.Plan().Lines()))
chosen, err := a.Assemble() // B has not overwritten A.
if err != nil { return err }
fmt.Println(chosen.VList.Box())
```

Each immutable `Trial` owns its selected `khipu.Tikray` and copied line-height,
side-skip and atomic-box inputs. `Inputs()` returns a detached snapshot. `Plan()`
exposes immutable decisions, per-line policy and diagnostics; `khipu.Freeze(plan)`
is available when only horizontal output is wanted. `Assemble()` always freezes
and assembles the complete paragraph, without searching or shaping again. It may
query stable metrics/resolver services and may fail without invalidating the trial.
See [Tikray versus paragraph.Trial](../../doc/PARAGRAPH_DESIGN.md#tikray-versus-paragraphtrial)
for the distinction between breaking decisions and their assembly wrapper.

`TrialRequest.Shape` optionally supplies a `khipu.LineShape` instead of a rectangle.
Its one-based line widths are captured during search, and the callback is not
retained. For shaped or continued paragraphs, `Plan().Lines()` records actual
widths/skips; `Inputs()` describes the requested suffix settings. Optional
`FirstFit` / `KnuthPlass` parameters let each trial choose its breaking policy;
only the arm matching `Layout.Params.Breaker` may be supplied. Hyphenation mode
and source styles remain fixed by preparation.

For a compatible `NoHyphenation` or `ExplicitHyphenation` trial, lock already
accepted complete lines:

```go
checkpoint, err := a.Checkpoint(1) // Requires at least one line in A.
if err != nil { return err }
suffix := a.Inputs()
suffix.Params.Breaker = paragraph.FirstFit
suffix.Params.Constraints.Width = 160 * dimen.BP
continued, err := prepared.Trial(paragraph.TrialRequest{
    Layout: suffix, Prefix: checkpoint,
})
if err != nil { return err }
result, err := continued.Assemble() // Complete paragraph, including locked line 1.
if err != nil { return err }
fmt.Println(result.BreakPositions)
```

The prefix retains endpoints, widths, skips and policy; only the suffix is
searched. The preparation must be the same incarnation. A nonempty checkpoint
requires unchanged `LineLayout` values (conservatively including suffix-only
overrides), and identical consumed `AtomicBox` values, including height and all
margins. Unconsumed atomic boxes may change. Keep matching application-owned
atomic interiors yourself; typography cannot certify their render identity.
Only first-fit continuation is supported; the original prefix may come from KP.
Checkpoint counts range from zero through all lines. Zero locks no geometry;
invalid/foreign checkpoints fail without publishing a candidate.

Trials have no mutable selection or rollback state: keep the candidate you need
and release the others. Serialize calls using shared services. Keep preparation
only while more searches/measurement are possible; release it and checkpoints
when the paragraph finishes or is abandoned. A trial needs neither preparation
nor discovery for later assembly, and frozen output retains neither trials nor
checkpoints. Frame scheduling belongs to the caller. For unchanged widths and
atomic geometry, use `PrefixFit.Refit` to reuse assembled geometry without service
or breaker calls. `Trial` always requests a search.

## Choose a fitting paragraph prefix

Use `Trial.FitPrefixes` when a receiving area has a finite height and the caller
may choose fewer lines than the longest fitting prefix. Preparation may use
`khipu.NoHyphenation` or `khipu.ExplicitHyphenation`:

```go
trial, err := prepared.Trial(paragraph.TrialRequest{Layout: request})
if err != nil { return err }
fit, err := trial.FitPrefixes(120) // fractional big points
if err != nil { return err }
for _, choice := range fit.Prefixes() {
    fmt.Println(choice.Lines, choice.Height, choice.Status, choice.To.Offset)
}
// Choose a returned local line count; shorter fitting prefixes are allowed.
selected, err := fit.Select(fit.Prefixes()[0].Lines)
if err != nil { return err }
fragment := selected.Fragment() // immutable, renderable independently
if selected.Info().Status == paragraph.FitContinues {
    suffix := trial.Inputs()
    suffix.Params.Breaker = paragraph.FirstFit
    suffix.Params.Constraints.Width = 160 * dimen.BP
    continued, err := prepared.Trial(paragraph.TrialRequest{
        Layout: suffix, Prefix: selected.Checkpoint(),
    })
    if err != nil { return err }
    next, err := continued.FitPrefixes(180)
    if err != nil { return err }
    _ = next // its line counts exclude all previously accepted lines
}
_ = fragment
```

`Prefixes()` returns detached descriptors, longest first, including an explicit
zero-line `FitNoProgress` choice last. Empty or already-completed paragraphs have
one zero-line `FitComplete` choice. Full CSS line boxes must fit exactly, with
no overflow tolerance. Zero-height lines can advance. Heights must be finite,
nonnegative and below `dimen.Fil.Points()`. Horizontal overflow is reported by
`Overfull` and the occupied `Width`; height fitting never clips it. A no-progress
checkpoint can be retried with different receiving constraints.

`FitPrefixes` assembles once per call using the trial's captured decisions;
it does not reshape or search. `Select` performs no service calls. It pairs the
selected fragment with its exact checkpoint and copied `FittedAtomic` records
containing each accepted box and matching local placement. A complete selection
has no checkpoint. Consumed atomic geometry stays locked; change unconsumed
`LayoutRequest.Atomics` before a resumed trial when its receiving width changes.
Application-owned atomic interiors must stay paired with these accepted values.

`Info().From`/`To` describe logical consumption in `Input.Text` byte coordinates,
qualified by preparation identity. `NextKnot` distinguishes zero-byte boundaries;
these values support comparison, not resumption. `Coverage()` separately reports
rendered glyph-cluster, retained-space and atomic ranges in logical source order.
An authored range overlapping a ligature belongs to the fragment containing the
whole cluster, even if that range starts inside it. `ReplacementCoverage` reports
each rendered pre/post-break half as one indivisible original source partition.
Its generated cluster coordinates are local to replacement text and must not be
treated as original offsets or a per-character source map. Trimmed spaces,
generated fill and empty/silent replacement output have no rendered coverage;
logical consumption and checkpoint carry still retain their source provenance. Keep original source-to-processed mappings and
empty-marker following/preceding affinity in the calling application; bidi
visual order is not logical progression.

Release the choice set and trial after selection. Retain the checkpoint while
resumption is needed, and keep the fragment alone for later rendering. Frozen
fragments retain no trial, preparation, discovery, shape callback or checkpoint.
Calls remain synchronous and serialized under the borrowed-service contract.
Invalid heights, non-fitting counts, foreign checkpoints, changed consumed
atomics and unsupported hyphenation fail with existing stage/cause errors.
See the runnable [prefix-selection example](prefix_example_test.go) and
[manual-continuation example](prefix_manual_example_test.go).

For manual input, supply an explicit `Config.Hyphenator` and prepare with
`PrepareOptions{Hyphenation: khipu.ExplicitHyphenation}`. A selected soft hyphen
locks its exact replacement pair in the checkpoint; the post-break half starts
the next suffix once, including zero-width material. `Info().To.Offset` records
the original source split, not generated spelling length. Equal or changed
suffix widths can use `Prepared.Trial`/`FitPrefixes`/`Select`; unchanged geometry
can also use `PrefixFit.Refit` without discovering any new alternatives.
Later authored alternatives may invoke lazy replacement services; base text and
accepted alternatives are never prepared again. Failed services publish no trial
and leave previous choices/checkpoints usable for retry. Unsupported policy
errors remain distinguishable from a successful `FitNoProgress`.

Automatic continuation is unsupported. The carried half is not recursively split.

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

See the executable [height-refitting example](prefix_refit_example_test.go).

Start with one trial at the first receiver's width, then choose its longest
fitting prefix. Keep the choice set for height-only reuse:

```go
trial, err := prepared.Trial(paragraph.TrialRequest{Layout: request})
if err != nil { return err }
choices, err := trial.FitPrefixes(120)
if err != nil { return err }
first, err := choices.Select(choices.Prefixes()[0].Lines)
if err != nil { return err }

// Same width and atomic geometry: consume the selected lines and fit the tail.
next, err := choices.Refit(first.Info().Lines, 180)
if err != nil { return err }
second, err := next.Select(next.Prefixes()[0].Lines)
if err != nil { return err }
fmt.Println(second.Info().Status, next.Remainder().Box())
```

`Refit(0, height)` changes only the available height. The accepted count must be
eligible in the original set, so a previous height limit cannot be bypassed when
advancing. Earlier choices remain reusable. `Remainder()` exposes the set's frozen
unaccepted range; it retains no working preparation or trial.

For a width or unconsumed-object change, select a checkpoint and pass it to a new
`Prepared.Trial` with first-fit suffix parameters. After refitting, `Select(0)`
certifies all previously consumed lines without accepting more. The checkpoint
preserves full width history, even when the initial prefix used Knuth–Plass.

`FitNoProgress` means no line fits; try another receiver or stop. `FitContinues`
means a remainder exists; `FitComplete` needs no checkpoint. Horizontal overflow
remains independent of vertical completion. Preserve frozen fragments separately
for rendering and add each frame's origin there. Release preparations, trials,
choice sets and checkpoints when finished. Frame scheduling and surrounding
margins stay with the caller; there is no height stretching or implicit cache.

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
`ErrIncompatibleFragment`. Use `PrefixFit.Refit` for captured geometry with different
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
