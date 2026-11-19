# layout

Context policy (2026-11-19): typesetting, geometry, retained proofs, replay,
and document execution are synchronous and have no context argument. Contexts
are confined to streaming coordination and font discovery/loading. Streaming
callers create a local background context; consumer failure still cancels and
joins the producer before cleanup. Errors publish no partial result and preserve
previous immutable output. TUI layout jobs finish serially; stale results are
discarded, while font work remains cancellable. This supersedes earlier
cancellation requirements for typesetting in this document.

Immutable paragraph geometry and reusable vertical items, with no layout or
rendering algorithms.

The [CSS-only VList contract](../doc/VLIST_CONTRACT.md) is implemented:
line boxes contain their leading and stack without extra interline/trailing
spacing. Completed VLists and fragments accept only LineBox material.

NewVList validates finite geometry and frozen line/atomic associations and copies
all input slices. Accessors return detached geometry and the shared immutable
frozen content. Empty placements can retain diagnostic frozen content for
suppressed empty source.

Coordinates are fractional big points, Y downward. Baselines and atomic origins
can be signed. Logical boxes do not bound glyph ink. Material records line-box placements. Atomic placements preserve physical traversal, not X sort.

Glyphing's paragraph assembler owns mutable construction state and publishes
only completed results. Rendering never rebuilds metrics or changes placements.
This package is not a page-layout engine or wire format.

## Unboxing

`Unbox(vlist)` returns an immutable `ElasticVList` of `VItem` values in source
line order. `Len` and `Item(i)` expose the sequence. Each item directly retains
the shared frozen content and a detached horizontal line record, with no backlink
to the VList or former line index. `Horizontal()` returns that store/record pair;
`Box()`, `Extents()` and `AtomicPlacements()` expose local geometry. Items remain
usable independently of the original VList and the list handle.

The full measured CSS line-height and signed effective extents are preserved.
Baselines and atomic origins are translated relative to each line's origin;
atomic slices are detached, keep physical order and use local Line=0. Empty
paragraphs recover no items; blank and zero-height lines remain distinct.

Unboxing validates input and translated coordinates without querying metrics,
shaping or breaking. It is synchronous, has no context argument and returns nil
with an error on failure, never partial items. A zero ElasticVList is empty; a
zero VItem is not a content item. The source VList and fragments stay unchanged.

The application retains surrounding block margins separately; a bare VList has
none to recover. Despite its name, ElasticVList currently has fixed item geometry
and no stretch/shrink, mutation, restacking, renderer or packing API. See
[VLIST_PLAN.md](../doc/VLIST_PLAN.md) for ownership and integration details.

## Line-range fragments

These existing APIs describe positioned views, not unboxed vertical items.
They remain distinct from ElasticVList, whose items have local origins.

`Fragment` is a separate immutable placement result sharing the entire frozen
paragraph. `NewFragment(frozen, range, box, lines, materials, atomics)`
validates a contiguous original line range and copies all geometry slices.
`FragmentLine.Index` is local; `OriginalIndex` identifies the frozen line.
Material children and atomic line indexes are local; knot/source indexes are not
rebased. Empty ranges have a zero box and no baseline.

Each line has exactly one material sharing its exact Top and Height. Independent
rounding of those anchors is rejected, fixing the previously accepted geometry
that could become negative on rebasing. Validation allows 1e-9 big points only
for recomposed arithmetic identities; finite-domain checks remain strict.
Line.Gap has been removed. Kern remains a material-kind identifier but is not
accepted in completed CSS lists; it provides no unboxing or glue API.

Use `paragraph.Fragment` for an exact whole-VList view and
`paragraph.SplitFragment` for explicit interior boundaries. These operations do
not change VList's full-paragraph contract. Fragments retain stable frozen
services and can retain the whole paragraph in memory. `render.BuildFragmentProof`
consumes these placements without restacking; working preparation is not retained.
For compatible uniform-width owners, `paragraph.FitFragment` selects the longest
whole-line prefix fitting an available height. `Prepared.StartFlow` and
`Continuation.Fit` handle width changes and heterogeneous width history in the
paragraph package; shared layout values contain no working preparation or tokens.
See [paragraph flow](../glyphing/paragraph/README.md#checkpoint-based-frame-flow).
Target-height packing and serialization remain deferred.

Run `go test ./layout` and `go test -race ./layout` from the module root. Owning
tests cover publication copies, strict placement anchors, signed/zero geometry,
fragment ranges, local depth validation, cancellation and unboxing ownership.
