# font

Neutral font identity and retained-source capabilities shared by both branches.
This package imports neither branch, font parsers nor discovery.

Info is detached identity, family/subfamily and parser warnings. IdentityProvider
identifies a retained contracts.FontFace. ResourceFace exposes exact immutable
fontresource bytes and matching Info; frozen glyph IDs must address that resource.
Accessors perform no loading or metric queries and return detached metadata.
Identify validates identity and copies warning slices without font queries.
Source supplies independent readers for retained bytes.

FontFace, FontScale and FontInstance remain in khipu/contracts. ResourceFace is
an optional capability used by the OpenType renderer, not a new requirement on
all synthetic or alternative backends.


Use [glyphing adapters](../glyphing/adapter/opentype/README.md) to prepare concrete
faces and [render services](../render/adapter/opentype/README.md) to consume their
retained resources. This package owns neither parser lifetime nor font acceptance.
Run `go test ./font` and `go test -race ./font` from the module root; tests cover
missing/mismatched resource identity and detached warning metadata.
