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
