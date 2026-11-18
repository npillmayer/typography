# render/adapter/opentype

`New` returns concurrency-safe `*Services` implementing `render.Backend` and
`font.IdentityProvider`. A nonnil pointer to a zero-value `Services` struct,
such as `&opentype.Services{}` or `new(opentype.Services)`, also works;
a nil `*Services` does not. Do not copy a service after use. Its cache lifetime
is the service lifetime, with no global state.

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


Use these services with `render.BuildParagraphProof` or `render.BuildFragmentProof`;
no separate parser or cache is needed per fragment. The renderer supports
`font.ResourceFace`, not just a concrete glyphing adapter face. Representation
acceptance for shaping/coverage belongs to the glyphing adapter's `CheckFace`;
rendering relies on retained resource identity and its own capabilities.

Run `go test ./render/adapter/opentype` and
`go test -race ./render/adapter/opentype` from the module root. Owning tests require
no glyphing backend; real cross-branch scenarios live in
[integration](../../../integration/README.md). See [render](../../README.md) for
coordinate, paint, cancellation and immutable-output contracts.
