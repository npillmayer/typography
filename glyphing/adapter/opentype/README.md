# glyphing/adapter/opentype

OpenType loading, nominal coverage and shaping through khipu/glyphing contracts.
Services supplies Load, GlyphIndex and CheckFace. Loader implements FontLoader;
NewStream constructs a fresh streaming shaper.

Face retains immutable resource bytes, a private OpenType parser, cached metrics
and one-em scale. Lazy queries and shaping share its lock. Face.Scale multiplies
otquery.FontScale(font, 1) by physical DU em size; khipu binds it in FontInstance.
The adapter unwraps ShapeParams.Font.Face() and emits signed font-unit records.

Normal parsing remains mandatory. No public parsed-font accessor, discovery,
outline state or rendering service belongs here. Face implements font.ResourceFace
with detached font.Info and retained Resource. Rendering uses that capability.

Streams start no goroutines. Drain concurrently with Shape, finish streams and
check terminal errors. Preserve scalar clusters/features and cancellation.
Acceptance is representation checking, not glyph or feature certification.

Cross-phase tests live in integration; fixtures remain in sibling opentype.
See the [module overview](../../../README.md).
