# typography

Paragraph typography integration with explicit services:

- Root: style mapping, script/grapheme itemization, coverage policy, bindings,
  pinned snapshots, IdentityProvider and FontSource.
- `style`: document values, UTF-8 spans and typography/paint projections.
- `fontresource`: bounded immutable bytes and fixed version-1 content/provenance IDs.
- `discovery`: neutral CSS requests, explicit generic policy, ordered families,
  session cache and detached resolution evidence; requires Searcher and FaceLoader.
- `adapter/fontfind`: controlled providers, exact-match attestations, private
  ResolverPipeline, enum conversion and resource snapshots; no default loader.
- `render`: neutral retained-outline/visual-order interface for frozen proof walkers.
- `adapter/opentype`: loader/face/scale/shaper, identity, retained outlines and
  generic VisualOrder implementation. No public parsed-font accessor.

Root LoadSettings requires FaceLoader and Coverage; discovery.New requires explicit
search/validation services. Neither installs a backend or imports an adapter.
No implicit font substitution or system/network discovery is enabled. Read/parse
failures terminate; successful cache hits retain diagnostic substitution evidence.
Providers must be offline, concurrency-safe and honor cancellation. The fontfind
ReadFontData allocation still precedes the snapshot size bound; only trusted
providers belong in that adapter. Services remain valid for the result lifetime.

Resources expose independent readers, never mutable bytes. Snapshot pins every
configured resource without invoking discovery or reopening paths. Resource request
codes preserve version-1 IDs and are distinct from document CSS numeric weights.
Tytui consumes these packages directly; only versioned font wire records remain
in its replay package. There are no tytui font/face/resource forwarding wrappers.

Glyph-boundary paint is the default: equivalent resolved typography coalesces
across paint changes, while original paint spans remain separate. One glyph gets
one color at its first logical source anchor; shared anchors share color and
independently anchored marks can differ. No clipping or conflict warnings.
Feature-bearing domains remain separate for khipu's existing clipping/rebasing.
Actual font/size/language/bidi boundaries remain significant. LegacyStyleBoundary
is available explicitly for old replay; document Style.Equals remains complete.
render.PaintSources maps frozen anchors and the supported explicit-soft-hyphen
replacements to original bytes, rejecting unknown generated transformations.

Development uses sibling cords, khipu, uax, schuko, opentype and fontfind checkouts.
Neutral tests use fake services; adapter tests use controlled providers and sibling
opentype font fixtures. Run `go test ./...`, `go test -race ./...`, `go build ./...`
and `go vet ./...`, then tytui suites including static pre-migration replay fixtures.

Tytui replay version 2 records the new glyph-boundary policy. Version-1 fixtures
keep legacy behavior and identities. Keep tests for ligatures, combining marks,
RTL/fallback/discretionaries, source maps and original feature-coordinate domains.

Rendering bypass: M owns `ProjectStyles`, producing `TypographySpan` values with
color-free `TypographyStyle` styles. The private run-domain identity preserves
relative feature origins and explicit legacy document boundaries without carrying
paint through styled.Text. Itemize/ItemizePolicy use the same projection and their
internal run styles contain no document Color field. ResolveStyle accepts projected
styles and legacy document Style inputs. Keep document equality and JSON unchanged.
The application retains `PaintSpan` values separately for rendering; tytui's
Result.PaintSpans returns a detached copy. ItemizeWithMetadata remains compatible.
No annotation package or replay version change is introduced.

Extraction step 7 is complete. Compose the OpenType implementation explicitly:

```go
services := opentype.Services{}
resolver, err := typography.LoadSettings(ctx, settings,
    typography.Services{Loader: services, Coverage: services})
```

Services implements FaceLoader, Coverage, IdentityProvider and render.Backend.
Use adapter/opentype.NewStream for shaping. Discovery.New takes the same injected
loader and a Searcher such as adapter/fontfind.New's controlled provider adapter.
Application file selection can use discovery.Resolution{Settings: settings};
descriptor resolution returns that same detached settings/provenance envelope.
The resolver's Snapshot remains typography.Settings with pinned resources.

General parser/stream/resource/discovery regressions live here. The feature-range
and nonzero-cluster-base baseline moved from tytui into adapter/opentype; tytui
retains DOM-to-resolver, frozen glyph, TUI and replay integration tests. A dependency
audit prevents generic packages from importing DOM, discovery or concrete adapters.
The rendering bypass and legacy replay policies are unchanged.

`paragraph.Assemble(ctx, frozen, paragraph.Options{Policy:
paragraph.ProofCompatibility, DefaultEmSize: size})` now assembles immutable frozen
lines into a boxed natural vertical list. `Box`, `Lines` and `Materials` return
value geometry; `Frozen` retains the read-only source. Geometry uses fractional
big points with downward-positive Y and a first-line reference baseline. The named
policy preserves tytui proof metrics, minimum 0.2em gap and trailing gap. Logical
width encloses target/occupied widths; glyph ink is not measured. DefaultEmSize
supplies 0.8/0.2/0.2em fallback metrics for lines without measured extents.

This first slice has no rendering, DOM, discovery or concrete font dependency;
callers supply already frozen output. There is no target-height packing, glue
stretch/shrink or CSS line-height interpretation. Coordinates must remain below
`dimen.Fil` expressed in big points; non-finite metrics, invalid input and overflow
are errors. Tytui now retains assembly output in completed results and its glyph
renderer consumes placements without querying line metrics. Result.VList returns
a detached handle; font/paint inspection input cannot reflow an existing proof.
Replay rebuilds placements under the compatibility policy without changing v1/v2
wire records or evidence. Rendering colors remain on their separate path.


`paragraph.Run(ctx, input, params, config)` now provides prepared styled paragraph
→ boxed vlist. Input carries styled.Text, base Direction and zero-byte Markup;
Params separates Constraints.Width, Breaker and Assembly defaults. Config supplies
khipu shaper/resolver/hyphenation contracts (zero uses khipu's synthetic backend).
M's Itemize/ProjectStyles runs before P; input style identities are preserved. P
never re-itemizes or reconstructs styles, normalizes whitespace or loads fonts.
Result retains Frozen/VList, paragraph-relative break endpoints, bidi runs and
counts; errors carry the failed stage and return no partial output. Every call
gets a fresh owner and freezes before assembly. Services must stay valid for
retained results. Typography-only text and separate renderer paint remain the
application contract. The compatibility assembly policy preserves replay v1/v2.


Paragraph-engine step 4 adds `CSSLineHeight` to Run with `Params.LineLayout`:
normal, unitless factor or resolved DU length, plus optional nonoverlapping UTF-8
LineHeightSpan overrides. Values bypass styled.Text and shaping. Explicit
Streaming.Defaults font/size supplies the strut. Normal uses scaled ascent +
descent + max(0,font gap); half-leading is split about the baseline, including
negative leading. Effective extents are signed; baselines may lie outside boxes.
CSS mode stacks LineBox material without compatibility's 0.2em floor or trailing
kern. Glyphs use frozen metrics/fallback; surviving spaces use prepared source
style metrics. Source styles are scanned without flattening the cord. Keep
ProofCompatibility unchanged, and reject CSS values under that policy. Frozen-only
Assemble remains compatibility-only; CSS Run needs source styles for spaces.

The narrow layout records are not a general annotation API. CSS parsing,
percentage/relative-length computation and inheritance remain the input adapter's
responsibility. Factor inheritance preserves a multiplier; lengths stay fixed.
No vertical-align, inline objects, empty-element boxes, padding, margins or bounded
height packing are implemented. Rendering colors and ink bounds remain separate.
Tytui's headless Input.LineLayout opts in; TUI defaults stay compatible. Replay
v1/v2 omit the new fields and retain old evidence. Explicit line-height cases use
v3 with typed input and numeric placements, no services or VList objects. Both
paint policies remain independent of layout. Owning tests cover exact geometry,
inheritance/strut, mixed fonts/sizes, zero/negative leading, spaces/replacements,
invalid values/metrics/ranges, cancellation and overflow. Tytui tests ligatures,
RTL/fallback/paint, rendering without metrics, and offline v3 resaves/tampering
alongside unchanged legacy fixtures. Full tests/races/build/vet pass in both modules.
