# glyphing

Font/style resolution and grapheme/script itemization for typesetting.
This package consumes preloaded faces and injected coverage. It neither discovers
nor parses fonts and has no rendering dependency.

NewStyleMapper validates a closed SelectedFonts table and explicit default, ordered
style-rule and fallback resource IDs. Every configured face is represented once;
unused rules remain part of the snapshot. Backend acceptance performs no glyph
queries. Snapshots detach metadata and share stable faces/resources.

Itemize selects one face covering each grapheme and preserves feature domains,
bidi metadata and UTF-8 coordinates. ItemizeInline excludes attached U+FFFC
markers from coverage while retaining their bytes; unattached markers remain
text. Missing coverage differs from cmap errors. Latin/Hebrew is the current
strong-script profile.

ProjectStyles and CoalesceStyles determine shaping-equivalent spans without
paint. ItemizeWithMetadata adds bindings and shaping metadata, not paint.
Applications retain style.ProjectPaint separately.

Pass text and resolver to [paragraph](paragraph/README.md). Concrete services are
in [adapter/opentype](adapter/opentype/README.md). FontInstance construction stays
in khipu after style resolution. No render services are required for layout.
See the [module overview](../README.md).
