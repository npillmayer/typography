# CSS line-box VList contract

Context policy (2026-11-19): typesetting, geometry, retained proofs, replay,
and document execution are synchronous and have no context argument. Contexts
are confined to streaming coordination and font discovery/loading. Streaming
callers create a local background context; consumer failure still cancels and
joins the producer before cleanup. Errors publish no partial result and preserve
previous immutable output. TUI layout jobs finish serially; stale results are
discarded, while font work remains cancellable. This supersedes earlier
cancellation requirements for typesetting in this document.

Implemented: 2026-11-17. CSSLineHeight is the sole paragraph assembly policy.
ProofCompatibility, DefaultEmSize, Line.Gap and frozen-only Assemble are removed.
Tytui paragraph callers now supply explicit CSS normal line-height by default;
tycli/documentlayout's existing CSS geometry remains unchanged. Replay is
CSS-only version 5, with versions 1–4 rejected before schema/font processing.

Existing line-range fragments remain positioned views, not an unboxed sequence.
Unboxing into immutable VItem/ElasticVList data is implemented as described in
[VLIST_PLAN.md](VLIST_PLAN.md). Restacking and target-height packing remain deferred.

## Scope and consumers

A paragraph VList is the completed natural vertical arrangement of already
broken horizontal line boxes. It owns vertical geometry and retains immutable
frozen horizontal content. It is not the containing CSS block, a page, an ink
bounding box or an elastic list waiting to be packed.

Current `tycli` and tytui document layout already select `CSSLineHeight` through
[documentlayout](../../tytui/documentlayout/layout.go). Their current CSS geometry
is the baseline to preserve during migration. Paragraph-mode spacing is not a
compatibility requirement. Remaining paragraph clients must adopt CSS line boxes
or be retired; preserving the TUI paragraph mode must not dictate library design.

This contract describes our existing supported CSS-inspired subset, not complete
CSS inline formatting. General vertical-align, empty inline-element boxes,
fragmentation inside inline objects and page layout remain outside it.

## A line box contains its leading

Each horizontal line has logical extents above and below its baseline. These
are effective line-box extents, not necessarily font ascent/descent or glyph ink.

The line-height inputs are computed values supplied by the caller:

- `normal`: scaled font ascent + descent + nonnegative font line gap.
- A unitless factor: multiply the receiving run's em size.
- A resolved length: use the fixed length independently of run size.

For each text contribution, half of the difference between its requested
line-height and its font ascent-plus-descent is placed on either side of the
baseline. Leading may be negative. The paragraph strut contributes on every
nonempty line, using explicit default font/size and paragraph line-height.
The line takes the maximum effective extent above and below the baseline across
the strut, text contributions and supported atomic-box contributions.

Glyph-bearing text uses its retained font metrics, including fallback fonts.
Surviving source spaces use prepared source styles; generated fill and silent
discretionary material add no text extent. Atomic border boxes and resolved
margins contribute according to the existing bottom-margin-edge alignment,
without text half-leading or marker font queries.

CSS parsing, inheritance, percentage resolution and whitespace processing remain
with the input/application layer. Factor inheritance retains the multiplier;
resolved lengths stay fixed. A frozen paragraph alone does not retain all inputs
needed to reconstruct these line boxes, especially source-style metrics for spaces.

## Natural vertical arrangement

For each line, let A and D be its effective above/below-baseline extents, and H
its logical height. The placement relationships are:

```text
H[i]        = A[i] + D[i]
Top[0]      = 0
Baseline[i] = Top[i] + A[i]
Top[i+1]    = Top[i] + H[i]
```

Preserve the existing CSS assembler's fractional operation order. The equations
describe the geometry; they do not require floating-point reassociation.

There is no additional interline or trailing spacing. Paragraph assembly emits
one LineBox material per line and no Kern or Glue material. The final extent is
the bottom of the final line box. It includes that line's own leading, not an
extra trailing gap. Line.Gap is removed;
`Line.Height` equals A + D, and the corresponding material has the same top and
height. The compatibility policy's 0.2em minimum and trailing kern are not part
of this contract.

A and D may be signed, but H is finite and nonnegative. Zero-height lines are
valid and remain distinct, ordered content even when their tops coincide.
Baselines can lie outside their logical line boxes. Glyph ink and inline-object
ink may overflow logical boxes; no implicit clipping or ink measurement occurs.

The VList's Box has:

- Width equal to the maximum of retained occupied and target line widths.
- Extent equal to the completed natural vertical extent.
- Baseline equal to the first line's baseline relative to the VList top.
- No baseline and zero geometry when there is no laid-out content.

Suppressed empty source may still retain a diagnostic frozen line; that does not
create a visible line box or strut. Preserved whitespace and explicit blank lines
are not equivalent to empty source. Block padding, borders, margins, margin
collapsing and alignment inside a containing block are outside this VList.

## Ownership, identity and validation

Keep all completed geometry immutable. Constructors copy caller-owned slices;
accessors return detached geometry. Frozen horizontal content and its stable
font/scale services remain shared under their existing lifetime contracts.

Preserve horizontal widths/skips, physical knot order, selected replacements,
original UTF-8/scalar coordinates, feature domains and atomic IDs. Paint stays
separate. Atomic origins follow line/frozen traversal, not sorting by X; signed
horizontal advances and coincident origins remain valid.

Coordinates are fractional big points with Y downward. Validate finite primitive
values, completed sums and published coordinates within the existing dimension
domain; never narrow/wrap through integer DU. Keep the nonnegative extent rules
distinct from signed baseline and atomic-origin rules.

The full paragraph VList retains its complete frozen-line association, except
for suppressed empty output. Its published line tops and LineBox material tops
must use the same authoritative placement values. Do not independently round
equivalent copies of a placement and then rely on tolerance to reconcile them.
Unboxing preserves stored line-height and effective extents and validates new
local coordinates. Future repacking must specify numerical consistency before
allowing tolerance at boundaries; acceptance must not create unexplained failures
on a subsequent valid split.

Failures publish no partial result and cannot mutate previous
results. Rendering consumes completed placements without querying line metrics,
restacking, reshaping, rebreaking or reopening font sources.

## Boundary with unboxing and packing

A completed VList and an unboxed sequence are different abstractions. The current
`layout.Material` records absolute placement. The current `layout.Fragment`
retains positioned line ranges. Neither is the agreed unboxed representation.

For CSS paragraph output, layout.Unbox recovers VItems whose intrinsic vertical
geometry already includes leading. Items retain frozen content directly and
translate baselines/atomic origins into local coordinates, without manufacturing
interline glue. Surrounding margins are retained separately by the application.
Unbox is synchronous without a context argument; it validates input and translated
geometry and publishes no partial output on failure. Existing fragment/layout
cancellation contracts remain unchanged. A future general vertical item sequence can
support boxes, explicit kerns and glue, but its type, ownership and packing
semantics require a separate design. This contract does not remove the general
concept of kern/glue or claim that every future vertical list contains only lines.

Putting a natural VList in a taller container can use container positioning.
Distributing additional height between lines requires explicit extra spacing and
a packing policy; it is not an implicit change to CSS line-height. Page splitting,
page-edge spacing and target-height packing remain deferred. Do not extend the
current fragment implementation as a substitute for a packing policy.

## Migration record

1. Make CSS assembly the sole paragraph assembly path. Remove the compatibility
   spacing algorithm and its metricless fallback-size policy. Retire or redesign
   the frozen-only `Assemble` entry point: do not silently reconstruct missing
   source styles or invent strut defaults. Choose the final simplified API in
   the implementation step; the explicit CSS Options/LineLayout inputs remain.
2. Update remaining paragraph callers to supply explicit CSS line-height and
   streaming defaults, or retire those callers. Keep `tycli` and documentlayout
   output unchanged. Paragraph mode remains available with CSS normal defaults; it no longer
   requires a separate assembly algorithm.
3. Align VList data, validators, comments and tests with the one line-box contract.
   Review compatibility-only fields such as Gap rather than preserving redundant
   geometry for an obsolete mode. Revisit fragments after the unboxed sequence
   design. Publication now requires exact line/material anchors, resolving the
   tolerance/rebasing defect without repairing caller geometry.
4. Deliberately retire compatibility replay behavior under the application's
   [replay change procedure](../../tytui/doc/OFFLINE_REPLAY.md). Old evidence must
   not be reinterpreted as CSS output or automatically regenerated. Historical
   cases may become explicit rejection cases. Version 5 replaces support for versions 1–4; font identity/provenance and immutable resources remain
   independent contracts. Old replay fixtures remain unchanged rejection cases. The eight existing CSS
   SVG/placement controls remain executable and unchanged; the eight compatibility
   controls remain historical evidence.
5. Preserve current CSS document controls and verify struts, mixed fonts/sizes,
   spaces, zero/negative leading, blank/empty content, atomics, RTL, ownership,
   cancellation, finite bounds and rendering without metrics. Run ordinary/race
   suites, build, vet, dependency audits and adapter regressions in typography
   and tytui, including tycli/documentlayout integration, when code is migrated.

This decision supersedes requirements to preserve ProofCompatibility in earlier
guidance and in the [fragmentation plan](VLIST_PLAN.md). Unrelated font, shaping,
source-coordinate, ownership and dependency invariants continue to apply.


## Validation completed (2026-11-17)

Passed in typography and sibling tytui:

- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go build ./...`
- `go vet ./...`

Typography suites include the generic dependency/branch audit and both adapter
packages. Tytui suites include tycli/documentlayout geometry controls, the eight
unchanged CSS replay SVG/placement controls, version-5 normal/factor CSS resaves,
resource/source-removal cases, retired-mode/version rejection and strict rejection
of the removed Gap field. No JSON/SVG fixture evidence was modified.

Also passed `go test ./... -count=1` in sibling khipu and css-box-layout.
Publication regressions reject independently rounded line/material anchors and
non-line material and verify repeated CSS splitting. This does not implement
unboxed nodes, renderer fragment support or target-height packing.
