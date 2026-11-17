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
