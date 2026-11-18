# glyphing/adapter/opentype

OpenType loading, nominal coverage and shaping through khipu/glyphing contracts.
Services supplies Load, GlyphIndex and CheckFace. Loader implements FontLoader;
`NewStream(capacity)` constructs a fresh streaming shaper with positive channel
capacity. `Services.Load(ctx, path, resource)` requires exactly one of a file path
or retained resource and returns a `glyphing.FaceEntry` for `SelectedFonts`.

Face retains immutable resource bytes, a private OpenType parser, cached metrics
and one-em scale. Lazy queries and shaping share its lock. Face.Scale multiplies
otquery.FontScale(font, 1) by physical DU em size; khipu binds it in FontInstance.
The adapter unwraps ShapeParams.Font.Face() and emits signed font-unit records.

Normal parsing remains mandatory. No public parsed-font accessor, discovery,
outline state or rendering service belongs here. Face implements font.ResourceFace
with detached font.Info and retained Resource. Rendering uses that capability.

Streams start no goroutines. Drain concurrently with Shape, finish streams and
check terminal errors. Preserve scalar clusters/features and cancellation.
`Stream.CheckFace` and `Services.CheckFace` accept initialized adapter `*Face`
values using shared private validation. These cheap, read-only checks perform no
I/O, parsing, metrics, glyph queries or stream-state changes. **CheckFace stays
entirely in this adapter:** it is not forwarded to `opentype/otshape`. Only actual
shaping passes the private concrete `*ot.Font` to that shaper. Direct `Shape` and
`GlyphIndex` calls also validate the face. Acceptance is not glyph, script/language
or feature certification.

Khipu checks actual resolved runs and fresh lazy replacement streams. It owns
producer/drainer orchestration and cleanup; ordinary paragraph callers supply the
stream factory rather than driving streams themselves. When using a stream
directly, join active production before finishing it; `Finish` on a busy stream
returns an error without closing output.

Run `go test ./glyphing/adapter/opentype` and
`go test -race ./glyphing/adapter/opentype` from the module root. Tests use the
normal parser and offline fixtures in sibling opentype; no fonts are copied or
downloaded. Cross-phase tests live in [integration](../../../integration/README.md).
See the [module overview](../../../README.md).
