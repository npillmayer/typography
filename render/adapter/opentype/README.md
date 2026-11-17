# render/adapter/opentype

New returns concurrency-safe *Services implementing render.Backend and
font.IdentityProvider. A zero-valued pointer works too; do not copy a service
after use. Its cache lifetime is the service lifetime, with no global state.

Outline validates font.ResourceFace identity and lazily parses retained bytes
with sfnt. It never opens source files or queries a glyphing parser, metrics or
scale. The renderer owns the parser/cache lock. Immutable resource identity is
validated once per session; face metadata is checked against it on every call.
Failed parses are not cached.
Paths and detached control points use font units, Y downward. The proof walker
applies frozen FontInstance scaling.

VisualOrder uses OpenType's public directional-fragment helper on caller-owned
slices. Identify returns detached retained font.Info. Unsupported capabilities,
malformed resources and invalid glyphs fail explicitly. Service construction
initializes no shaper and loads no font.
