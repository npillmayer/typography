# render

Retained paragraph proofs and paint-source mapping through neutral services.
There is no glyphing dependency.

    drawing := renderot.New() // render/adapter/opentype
    proof, err := render.BuildParagraphProof(ctx, render.ParagraphInput{
        Frozen: result.Frozen, VList: result.VList,
        Text: processedText, Paint: style.ProjectPaint(spans),
    }, render.Services{Backend: drawing, Identity: drawing})

BuildParagraphProof consumes layout.VList and matching frozen content. It never
queries line metrics, reshapes, rebreaks or restacks. Outline parsers can lazily
read retained bytes, never discover fonts or reopen original files.

`BuildFragmentProof(ctx, FragmentInput{Fragment: fragment, Text: processedText,
Paint: paint}, services)` uses the same retained walker for just that fragment's
line range. Text and paint still address the complete original paragraph.
Horizontal records come from `FragmentLine.OriginalIndex`; vertical coordinates
come from the local placement. `Glyph.Line` indexes the proof's local lines;
knot indexes and UTF-8 source anchors stay in the original frozen owner.
An empty fragment produces an empty proof without outline/order calls.

Render a sequence of flow fragments separately, even when they have different
frozen owners. The application translates each proof by its frame plus local
paragraph origin, and draws only its retained `AtomicPlacements`. Do not render
the whole intermediate owner, which would repeat accepted prefix lines. Finished
fragments need no preparation or continuation handle; source/font services remain
stable under their existing contracts. Tytui/documentlayout provides positioned
fragment proof/SVG composition, including retained fixed opaque child interiors.
Its `Document.LayoutFlow` and `tycli --frames` APIs now schedule a bounded single
paragraph through supplied frames. This proof API only renders given placements;
it does not choose frames or cuts. See [application flow guidance](../../tytui/documentlayout/README.md#bounded-paragraph-frame-flow-p5)
and the [production cost report](../doc/FLOW_COSTS.md).

Text/paint ranges use processed UTF-8 bytes. PaintSources maps scalar anchors to
source bytes including supported soft-hyphen replacements. Shared anchors share
paint; unknown generated transformations are rejected. There is no paint-policy
selector or legacy paint mode.

Placements use fractional big points, Y downward. Outlines use font units;
FontInstance supplies conversion. Bounds include logical rectangles and outline
control points, not tight ink. VisualOrder applies once to detached fragments;
knots remain physical.

Inputs/services stay stable during rendering; outputs are detached and failures
publish no partial proof. Document transforms, atomic drawing and SVG serialization
remain consumer work. Cross-phase proofs live in integration; render adapter
tests operate without a glyphing backend.


Run `go test ./render` and `go test -race ./render` from the module root.
[Integration tests](../integration/README.md) cover both branches, retained
fragment output after preparation collection, and real-font proof consistency.
The [OpenType renderer](adapter/opentype/README.md) supplies outlines/identity;
alternative backends implement the same neutral service contracts.
