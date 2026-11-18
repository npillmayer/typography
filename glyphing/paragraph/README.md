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
- `FitFragment` selects the longest whole-line prefix fitting an available height,
  with an explicit equal-layout reuse guard and an immutable remainder.
- `Prepared.StartFlow` creates a paragraph-bound continuation after initial layout.
  `Continuation.Fit` reuses equal-width geometry or locks accepted breakpoints and
  searches the suffix at a new width. Release working handles when flow finishes.


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
real-font integration also lives in [the glyphing adapter](../adapter/opentype/README.md),
[integration](../../integration/README.md) and sibling tytui. See the
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
unchanged. `render.BuildFragmentProof` consumes retained fragment placements;
target-height packing remains deferred. See
[the VList plan](../../doc/VLIST_PLAN.md).

## Equal-width height cuts

`FitFragment(ctx, fragment, sourceKey, targetKey, height)` implements production
stage P1 of [the flow plan](../../doc/FLOW.md). Height is an available content
height in fractional big points. It returns a detached `FragmentFit` containing
non-nil `Placed` and `Remainder` fragments, a `FitComplete`, `FitContinues` or
`FitNoProgress` status, and original frozen indexes of horizontally overfull
visible lines (including unplaced lines). Horizontal overflow is independent of
vertical completion. Empty input is complete; zero-height lines consume content.

`FragmentReuseKey` records width in DU, original left/right skip triples in
`[natural, minimum, maximum]` order, and nonzero caller-scoped revisions for
source, styles, fonts, line-height, atomics and policy. Capture it alongside the
initial layout request; it cannot be reconstructed from frozen geometry alone.
The target key must equal the source key. Changed inputs need new revisions and
a new layout; a false caller attestation or mutation of borrowed resources cannot
be detected here. Heights and origins do not belong in the key.

P1 requires a uniform-width frozen owner and checks **all** its target widths,
including suppressed-empty diagnostics, even when fitting a subfragment. It
does not admit heterogeneous-width continuation owners; use `Continuation.Fit`
for those retained width histories. Malformed source
keys/heights/fragments wrap `contracts.ErrIllegalArguments`; a differing target
key or inconsistent frozen target width returns `ErrIncompatibleFragment`.

```go
whole, err := paragraph.Fragment(ctx, result.VList)
if err != nil {
    return err
}
// key is the retained FragmentReuseKey for the initial request.
fit, err := paragraph.FitFragment(ctx, whole, key, key, availableHeight)
if err != nil {
    return err
}
// Retain fit.Placed; use fit.Remainder in the next compatible frame.
```

No preparation or service handle enters this operation. It does no itemization,
acceptance, metric queries, binding, shaping or breaking, and has no checkpoint
or paragraph-wide cache. It preserves full CSS line-height and source coordinates,
rebasing only the remainder. Fit uses strict authoritative ends without an overflow
epsilon; local baseline, depth and atomic coordinates retain strict finite-domain
checks without narrowing the existing arithmetic-equality tolerance. Errors or
cancellation return nil, preserve causes and leave earlier results reusable.

Completed fragments keep frozen content and stable font services alive without
requiring a working preparation. Applications retain preparation only while the
paragraph may still need different-width continuation/retry, then drop it and all
checkpoints when the paragraph has fully flowed. P1 neither creates nor owns those
handles. Surrounding margins stay application-owned. Khipu now supplies the P2
checkpoint and seeded first-fit primitives; P3's continuation/dispatch is described
below. P4 retained fragment rendering is available in `render` and application
proof composition; P5 adds bounded document/CLI scheduling outside this package.
P6 validates this bounded profile and records [production costs](../../doc/FLOW_COSTS.md).
Generalized flow and the upstream bidi fix remain deferred.

Owning tests migrate E1's partition/geometry oracles and sealed service traps.
A resolved-call audit follows the fit implementation and delegated private helpers;
mutation regressions reject hidden breakers, layout aliases and unknown callbacks.
The historical experiment and its measured evidence remain unchanged.

## Checkpoint-based frame flow

`Prepared.StartFlow(ctx, request)` creates a `Continuation` after one full layout
at the initial width. It requires `NoHyphenation`; the initial breaker may be FF
or KP. `token.Fit(ctx, Frame{Width: widthDU, Height: heightBP})` then places the
longest whole-line prefix in one frame. Zero widths/heights are valid. Every line
boundary is eligible; widows, orphans, target packing and elasticity are deferred.

```go
token, err := prepared.StartFlow(ctx, request)
if err != nil { return err }
for _, frame := range frames {
    result, err := token.Fit(ctx, frame)
    if err != nil { return err } // token remains retryable
    placed = append(placed, result.Placed)
    token = result.Continuation
    if token == nil { break } // complete, including suppressed-empty input
}
// A nonnil token is the explicit unfinished remainder; retain it for retry or
// more frames. On completion/abandonment, drop preparation and all old tokens.
```

`FlowResult.Status` uses `FitComplete`, `FitContinues` or `FitNoProgress`.
`Remainder` is an immutable frozen view; `Continuation` is nonnil only while
content remains. `Overfull` contains original frozen indexes for visible lines
in this call's input, including its remainder. `Breaker` identifies the retained
suffix's policy; `Reused` reports geometry-only reuse. Empty paragraphs complete
without relayout even if the frame width changes.

Frame input contains only resolved width and height. Source, styles, fonts,
line-height, skips and atomic boxes are fixed by the private flow state, giving
reuse an identity certificate by construction. The request's line-height slices,
atomic values and skips are copied; borrowed source/styles/services must stay
stable. Changing those inputs requires a new flow (and fresh preparation for
source/style/font changes); there is no silent update of accepted content.

Equal-width fitting uses P1's shared geometry core without metrics, binding,
shaping, CheckFace or breaking. It verifies the retained width history, so a
B-width tail can be reused even when its owner starts with A-width lines.
A width change certifies a P2 checkpoint from the successful full path, binds
fresh material, searches only the suffix using first-fit, and shares ordinary
layout's **complete** freezing and CSS assembly. Base shaping and bidi analysis
are retained; markup is never called again. Original styles still supply space
metrics. Each accepted A/B/... line keeps its own target width and original skips.
Repeated prefix metric work is accepted on this path.

Keep each returned `Placed` fragment exactly: later fragments may refer to a new
complete frozen owner. They form a sequence, never a synthetic single-owner VList.
The token retains only the current remainder/path and accepted width history,
not a chain of previous tokens or accepted fragments. Margins and frame origins
remain application-owned. Rendering of this sequence is implemented separately
by P4's `render.BuildFragmentProof` and tytui/documentlayout proof/export APIs.

All operations sharing preparation/services are serialized. Errors preserve
paragraph stages and underlying causes and publish no partial result. Earlier
tokens and fragments remain reusable. Cancellation is checked inside seeded
search, freezing, assembly and fitting; synchronous collaborators retain their
documented limits. The known upstream bidi preparation limitation is unchanged.

Process paragraphs in turn. An incomplete token intentionally retains preparation
for later width changes/retry. Completion returns frozen output with no working
continuation; drop **all** older tokens and the prepared handle then, before
advancing to the next paragraph. There is no implicit cache or `Close`; release
means dropping references for Go's GC. Frozen fragments retain stable source,
shaping and services independently. No new replay records or backend serialization
are introduced.


## Production flow measurements (P6)

The owning-package `BenchmarkP6*` benchmarks use `Prepare`, `StartFlow`, `Fit`,
`Layout` and `FitFragment` directly, without experiment imports or overlays.
Run the five-process recorder from the module root as described in
[the measurement harness](../../doc/flow-cost/README.md). Keep preparation and
initial layout separate from the post-layout two-frame comparison. Test-only
component probes time the actual checkpoint/bind/break/freeze/assembly primitives;
they introduce no production hooks or public diagnostic API. A regression compares
the component output with production continuation before accepting measurements.

[The P6 report](../../doc/FLOW_COSTS.md) records allocations, phase/material counts,
service-call gates and weighted 90/10 and 50/50 workloads. Full reconstruction on
width changes remains accepted; no suffix-proportional cost is promised. Existing
ownership, cancellation, GC and call-graph audits remain release gates, independent
of timing results. Arbitrary mixed-bidi input remains outside the supported profile
until the synchronous upstream preparation limitation is addressed.


New benchmark frame rows use `original-cut-lines/op` and `original-cut-percent`:
these describe the initial A-width layout used to select frame height, including
for whole-paragraph relayout comparators. Historical P6 labels are documented in
the cost report and are not rewritten. The analyzer validates per-family metrics
and zero-call gates, and refuses to overwrite historical summary files.
