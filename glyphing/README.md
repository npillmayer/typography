# Glyphing

Context policy (2026-11-19): typesetting, geometry, retained proofs, replay,
and document execution are synchronous and have no context argument. Contexts
are confined to streaming coordination and font discovery/loading. Streaming
callers create a local background context; consumer failure still cancels and
joins the producer before cleanup. Errors publish no partial result and preserve
previous immutable output. TUI layout jobs finish serially; stale results are
discarded, while font work remains cancellable. This supersedes earlier
cancellation requirements for typesetting in this document.

A typesetter translates styled text input into glyphs, i.e. graphic representations
of letters, ligatures, etc.  This translation is called "glyphing". This package
will map styles (e.g. *italic*) to fonts (e.g. an oblique font face). And it
will itemize input text into runs suitable for typesetting.

This package consumes preloaded faces and injected nominal coverage. It neither
loads/discovers fonts nor does it install a text-shaper, and has no rendering dependency.

## Select Fonts and map Styles

`NewStyleMapper(selectedFonts, coverage)` accepts a closed `SelectedFonts`
table: exactly one `FaceEntry` per referenced resource ID, with no missing,
duplicate or unreferenced entries. References include the default, ordered style
rules and explicit fallbacks, even when the current text never uses them.
`SelectedRule` weights are CSS numeric weights; they are distinct from the legacy
request codes used in resource identity. No implicit system fallback is added.

Construction performs structural validation, then `Coverage.CheckFace` once per
distinct canonical entry. It performs no I/O, parsing or glyph lookup. Acceptance
checks representation compatibility, not coverage or successful shaping. Snapshot
metadata is detached; faces/resources remain shared and stable. Canonical face
order is default, rules, fallbacks; role order and repeated fallback references
are preserved. Serialize operations sharing a `StyleMapper`.

The mapper implements `contracts.StyleResolver`. `Defaults()` supplies the default
font and size for paragraph execution; khipu constructs `FontInstance` values
from resolved runs. `ResolveStyle` accepts document `style.Style` values for
projection and owner defaults, projected `TypographyStyle` values, and the private
styles returned by itemization. Nil preserves the supplied defaults.

`TypographyStyle` implements `cords/styled.Style`, carrying typography without
paint and preserving feature-domain identity. Keep returned projected/itemized
values intact. Khipu receives opaque styles through cords and interprets them
through the injected resolver; it does not need the concrete glyphing style types.
Concrete loading/coverage/streaming services live in
[adapter/opentype](adapter/opentype/README.md).

## Itemize without losing Context

`Itemize` returns `styled.Text` and `[]FontTextRun`. Each run records the selected
resource and processed UTF-8 byte range. `ItemizeWithMetadata` adds shaping spans;
its `Itemization.Bindings` field also contains `FontTextRun` values. Keep returned
text intact: its private style identities preserve original feature domains.
Supply paragraph direction and isolate markup separately to paragraph execution.

`ItemizeInline` accepts ordered `AtomicAnchor` values identifying U+FFFC markers
at original byte offsets. Attached markers retain source coordinates but bypass
font binding and coverage; unattached U+FFFC remains ordinary text. Markers are
barriers for grapheme, script, fallback and soft-hyphen-neighbor processing.

Selection uses nominal glyph coverage for each complete grapheme. A missing glyph
may select an explicit fallback; a cmap error is terminal. Latin/Hebrew is the
current strong-script profile. Coverage does not establish suitable script/language
shaping support; script and language are passed to the shaper afterward.

`ProjectStyles` produces color-free typography spans; `CoalesceStyles` provides
coalesced document spans. Itemization coalesces equivalent shaping across paint
changes while retaining feature origins. Applications retain
`style.ProjectPaint(spans)` independently; paint is absent from itemization.

Pass the prepared text and mapper to [paragraph](paragraph/README.md) for
measurement, layout and checkpoint-based flow. Retain stable services for frozen
results; no rendering service is needed for layout.

## Verification

From the module root, run `go test ./glyphing` and `go test -race ./glyphing`.
Tests use fake preloaded faces for selection, acceptance, fallback, feature domains,
atomic markers, ownership and cancellation. Cross-branch proofs belong in
[integration](../integration/README.md); application discovery/replay policy stays
in sibling tytui. See the [module overview](../README.md) for full boundary checks.
