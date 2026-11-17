# Unboxing and natural restacking

Implemented: 2026-11-17. Unboxing produces an `ElasticVList` of immutable
`VItem` values. Each item initially contains a line box with its full measured
line-height. The application separately retains the surrounding block's original
margins. Natural restacking is proposed below; its implementation, rendering
integration and stretch/shrink remain deferred.

The [CSS-only VList contract](VLIST_CONTRACT.md) remains in force.
Tycli/documentlayout is the primary integration. Existing fragment APIs remain
positioned views; this plan does not change their implementation or contract.

## Scope and current boundary

[layout.VList](../layout/vlist.go) retains completed paragraph geometry and frozen
horizontal content. Each line includes its strut and half-leading. Successive
line boxes abut, with no additional interline or trailing spacing.

Unboxing recovers these individual lines without loading fonts, querying
metrics, shaping, rebreaking or reconstructing line-height from source styles.
It is non-destructive: the original VList and any existing fragments stay usable.
Each recovered line has its own local origin and retains its original content
association; its position in a new arrangement is a separate decision.

The surrounding CSS block is distinct from the paragraph VList. The current
[document adapter](../../tytui/documentlayout/layout.go) returns the paragraph's
total extent to CSS core, which handles block margins, padding, borders and block
positioning. The VList does not store those margins. Therefore unboxing a bare
VList cannot recover them: the application must retain and supply them separately.
CSS core remains responsible for margin resolution and collapsing in a new context.

This implementation covers line boxes and surrounding-margin metadata. It introduces
no general box-and-glue sequence, recursive container model or packing engine.
Inline objects remain inside their frozen lines, with their measured contribution
and placements intact; they are not recursively unboxed.

## Typography API

The data API lives in the existing `typography/layout` package, in
[unbox.go](../layout/unbox.go). The declarations below summarize its interface;
method bodies are omitted. Existing `Box`, `VList`, `AtomicPlacement` and khipu
types retain their current meaning.

```go
// VItem is an immutable vertical-list item with local box geometry.
// The initial item form contains one already measured horizontal line.
// Its fields are private; valid values are produced only by Unbox.
type VItem struct {
	frozen          *khipu.FrozenKhipu
	horizontal      khipu.FrozenLine
	box             Box
	ascent, descent float64
	atomics         []AtomicPlacement
}

// ElasticVList is an ordered sequence of items awaiting vertical placement.
// Items have fixed geometry; no stretch/shrink API is exposed.
type ElasticVList struct {
	items []VItem
}

// Unbox recovers measured lines without changing the source or measuring again.
// Invalid input or geometry returns nil and an error, never partial output.
func Unbox(v *VList) (*ElasticVList, error)

func (v *ElasticVList) Len() int
func (v *ElasticVList) Item(i int) (VItem, bool)

func (item VItem) Box() Box
func (item VItem) Extents() (ascent, descent float64)

// Horizontal returns the immutable content store and its detached line record.
// The record's knot range addresses that store; no VList lookup is required.
func (item VItem) Horizontal() (*khipu.FrozenKhipu, khipu.FrozenLine)

// AtomicPlacements returns a detached slice in frozen physical traversal order.
// Coordinates are line-local; Line is zero for this single-line view.
func (item VItem) AtomicPlacements() []AtomicPlacement
```

The declarations assume an import of `github.com/npillmayer/khipu`.
A zero `VItem` is invalid; a zero `ElasticVList` is a valid empty sequence.
`Unbox` requires a valid, non-nil VList and validates the translated geometry
before publication. `Item` reports an out-of-range index with `false`; it does
not publish a partly initialized item.
`Len` treats a nil list as empty, and `Item` on a nil list returns false.

`Unbox` runs synchronously without cancellation or deadline handling. Once called,
it returns either a complete sequence or an error; callers cannot cancel it midway.
Cancellation contracts of existing layout and fragment APIs remain unchanged.

Private slices and detached accessors prevent callers from mutating retained
geometry. A `VItem` has no backlink to a VList and stores no former line index.
It directly retains the immutable frozen content store and a detached horizontal
line record. The record alone would not keep the glyphs, source data and font
services available: its knot range indexes the shared frozen store. These content
coordinates remain unchanged and are distinct from positions in an ElasticVList.
See [khipu's frozen records](../../khipu/frozen_result.go).

Copying an item keeps that content alive independently of the original VList and
the ElasticVList handle. There is no mutable glyph copy, new font abstraction or
global identity registry. The list itself holds only its ordered items; items
carry no membership or placement identity. Application source/paint associations
remain outside the item, using the retained original content coordinates.

`ElasticVList` names the sequence before completed vertical placement. The
sequence is immutable and every item has fixed measured geometry.
The name does not introduce glue, capacities, mutation methods or target-height
packing; those semantics still require a later design.

## Full line-height and local geometry

For each recovered line:

- `Box.Extent` is the original `Line.Height`, including the complete effective
  line-height. `Extents` preserves the original signed ascent and descent.
- `Box.Width` follows the existing maximum of occupied and target line widths,
  preserving the horizontal skips and frozen line unchanged.
- `Box.Baseline` is the original baseline translated by subtracting the original
  line top. `HasBaseline` is true even for a zero-height line. No old paragraph
  `Top` is exposed as a placement in the recovered sequence.
- Atomic origins are translated by the same line origin, in both axes. Their
  source associations, IDs and physical traversal order are retained. Never sort
  by X or by coincident vertical coordinates.

All typography coordinates remain fractional big points with downward-positive
Y. Baselines and effective extents may be signed; line width and total height
remain finite and nonnegative. Logical boxes do not bound ink and imply no
clipping. Half-leading stays inside each line, including its negative-leading
cases; it is not converted into margins, kerns or glue.

Preserve authoritative stored heights and extents. Compute each coordinate
translation once; do not recompute height from rounded endpoints or replace a
retained extent with a newly subtracted value. A translated baseline can differ
slightly from the retained ascent because of floating-point subtraction. Apply
the existing 1e-9-big-point tolerance to source arithmetic identities only.
The VList constructor already checked those identities; Unbox validates finite
translated values, including derived `Box.Depth()`, without imposing a stricter
second identity test after rebasing.
Any later publication must use the same exact top/height anchors for a line and its placed
material; tolerance must not permit conflicting copies of placement data.

A natural restacking operation would advance by each complete line height
and translate its baseline and atomics together. The proposal below specifies
that operation without promising bitwise reproduction after floating-point rebasing.
Unboxing itself leaves the original completed geometry exactly unchanged.
Keep finite-domain validation and never narrow or wrap through integer DU.

Empty processed input produces an empty ElasticVList, even if the original frozen
result contains a diagnostic line. The diagnostic remains available through the
unchanged original result; an empty list need not retain it. Preserved blank lines
and zero-height lines remain individual, ordered items with their full content
associations.

## Application-owned surrounding margins

Retain margins once for the surrounding block, outside `VItem` and
`ElasticVList`. They are signed, uncollapsed resolved inputs, including explicit
zeros; they cannot be inferred from the paragraph's extent or a neighboring gap.
They do not become extra line height or get copied onto every recovered line.

The application API in [tytui/documentlayout](../../tytui/documentlayout/unbox.go)
uses these snapshots:

```go
// SurroundingMargins belongs to the enclosing CSS block, not to any one line.
type SurroundingMargins struct {
	OwnerID  box.BoxID
	Resolved box.Edges // top/right/bottom/left, in CSS pixels; before collapsing
}

// UnboxedParagraph is a detached application result record.
type UnboxedParagraph struct {
	Source  *Result              // immutable originating document result
	List    *layout.ElasticVList // immutable typography result
	Margins *SurroundingMargins  // detached copy; nil means not retained
}

// Both methods take an inline-group ID, as used by Result.Group.
func (r *Result) SurroundingMargins(id box.BoxID) (SurroundingMargins, bool)
func (r *Result) Unbox(id box.BoxID) (*UnboxedParagraph, error)
```

Here `box` is the existing css-box-layout/layout import. These types belong to
the application, so typography gains no CSS or DOM dependency. The exported
fields describe a detached result record, not mutable retained application state.
Changing the record or its margin copy must not change its source result, another
recovery or the shared item sequence. `Source` is application context for margins
and document associations; no VItem needs it to retain its horizontal content.

`LayoutInline` captures the enclosing owner's resolved margin values from the
final `InlineRequest.Used` view, after CSS auto sizing has finalized descendants.
It rejects missing/provisional/non-finite owner values. Each accepted group
retains a private value snapshot, published only with whole-document success.
No used-values table or callback view is retained. The original input table is
not used as a substitute for the final callback view.

`SurroundingMargins` returns a value copy and a presence flag. `Result.Unbox`
recovers the group's items and a detached margin copy, retaining `Source` for
document associations. Nil/unknown groups or invalid VLists fail without partial
output and preserve error causes. Unboxing is synchronous without cancellation.
Every group produced by `Document.Layout` has margins, including all-zero margins
and empty paragraphs. Missing metadata remains absent: false from the accessor
and nil in the envelope, rather than fabricated zeros. Neither a bare VList nor
a source result that never recorded margins can reconstruct them from geometry.

The source result retains the originating document and owner associations.
Resolved values are valid for that original resolution context. Changing the
containing width or other resolution inputs requires the application/CSS layer
to resolve new values from its source styles. The old snapshot stays unchanged;
the margin record alone is not a complete CSS recomputation API. Keep CSS-pixel
values at this boundary and convert explicitly when a future operation needs
typography big points, preserving existing rounding rules.

Padding and borders remain with the surrounding application box and its geometry.
They are neither transferred to every line nor removed by unboxing the paragraph.
Removing that surrounding box or flattening a document hierarchy is outside scope.
CSS core handles new block placement using retained or newly resolved inputs;
there is no requirement to reverse an old collapse calculation.

If a later operation splits the line sequence, the original surrounding margins
remain associated with their original owner. Whether top/bottom margins belong
on particular page fragments, are suppressed, or are reapplied needs a separate
policy. Recovery must not decide this implicitly or duplicate the owner's margins.

## Ownership and compatibility invariants

- No source VList, fragment, caller slice or previously returned result is mutated.
  Share only frozen content and stable font/scale services under their existing
  lifetime contracts. Copy all newly published mutable slices and metadata.
- Preserve original UTF-8/scalar coordinates, private feature domains, font
  identities, selected replacement provenance, frozen knot order and atomic IDs.
  Paint stays with the application; no repeated bidi conversion is introduced.
- Reuse performs no I/O, font loading, metric queries, shaping or line breaking.
  If horizontal widths or inline-object geometry must change, use retained
  paragraph preparation for a fresh layout; frozen content has no unfreeze step.
- Invalid input and geometry failures return no partial sequence and preserve
  error causes. Repeated recovery cannot affect an earlier result.
- Current `VList`, `Fragment`, rendering and CSS line-height contracts stay intact.
  A recovered sequence is not automatically an accepted new VList arrangement.
  Existing line-range fragments retain positioned geometry and remain distinct.
- Keep [replay version 5](../../tytui/doc/OFFLINE_REPLAY.md), font identity encoding,
  numeric provenance and current CSS/document controls unchanged. Versions 1–4
  remain rejection cases. Unboxing adds no serialized line handles, margins,
  services or VLists; future document replay needs its own explicit format decision.

## Proposed natural restacking (not implemented)

The first restacking operation should place immutable items consecutively from
top zero and publish a new `layout.VList`. Each item contributes its entire stored
line-height, including strut and half-leading. There is no requested target height,
extra spacing, margin insertion, stretching or shrinking. Restacking performs no
font queries, shaping, line breaking, bidi reordering or CSS resolution.

### Scope and frozen ownership

The current VList has one frozen owner and represents all its lines in their
original order, except for suppressed empty output. `NewVList`, fragment APIs and
proof consumers rely on that association. Preserve it for this first step:

- A nonempty input must contain exactly one item for each line of the same frozen
  paragraph, in frozen line order. Each retained horizontal record must match
  `frozen.Line(i)` and each item's frozen pointer must match the supplied owner.
- Reject mixed owners, omitted, duplicated or reordered lines, and invalid items.
  Do not concatenate frozen stores or rewrite knot indexes, atomic IDs or source
  coordinates to make such sequences fit the existing VList.
- Preserve every blank and zero-height line. Coincident tops do not identify
  duplicate items; sequence order is authoritative.

Today ElasticVList is produced only by Unbox and exposes no editing or slicing
API. This step supplies its natural return path to a completed VList. Restacking
subsets, reordered items or multiple paragraphs would require a separate decision
about the completed container and its content associations. Keep that extension
deferred; existing positioned fragments remain available for contiguous ranges.

Proposed API, in `typography/layout`:

```go
// Restack places complete paragraph items at their natural total height.
// It shares frozen content and publishes independently owned geometry.
// Invalid associations or geometry return nil and an error, never partial output.
func Restack(frozen *khipu.FrozenKhipu, items *ElasticVList) (*VList, error)
```

The explicit frozen argument follows NewVList's ownership boundary. For nonempty
input it identifies and checks the common content store. For empty input it keeps
the original suppressed diagnostic content: an empty ElasticVList intentionally
does not retain an owner from which Restack could recover it. Callers keep the
original `VList.Frozen()` or the application's `GroupResult.Paragraph.Frozen`.
Do not add a VList backlink or former line index to VItem, retain an owner merely
for empty ElasticVLists, or manufacture frozen content by running layout again.

Reject nil arguments. A non-nil zero ElasticVList is a valid empty sequence only
with an owner satisfying NewVList's existing empty-output rules. Publish `Box{}`,
no lines/material/atomics and the supplied frozen owner. An empty sequence must
not suppress a nonempty paragraph. The generic operation cannot prove the history
of an empty list; the application must pass the owner of the group being recovered.

Like Unbox, Restack should be synchronous and have no context parameter. It does
bounded work over retained lines and atomic associations, with no I/O or backend
calls. Existing preparation, fragment and rendering cancellation contracts stay
unchanged. Validation errors wrap `contracts.ErrIllegalArguments`; preserve causes
from publication and include the item index when identifying an invalid item.

### Placement and numerical acceptance

Use float64 fractional big points throughout. For item `i`, let `b` be its stored
Box, `a,d` its retained extents, and `top` the running end of the preceding line:

```text
top initially          = 0
Line.Index, Line.X     = i, 0
Line.Top               = top
Line.Height            = b.Extent
Line.Baseline          = top + b.Baseline
Line.Ascent, Descent   = a, d
Line.Horizontal        = item's retained horizontal record
Material               = LineBox, Child i, same exact Top and Height as Line
Atomic.Line            = i
Atomic.X, Atomic.Y     = local X, top + local Y
next top               = top + b.Extent
```

Compute each new placement once and use it in every corresponding record.
Advance in sequence order using stored heights, including zero heights. Preserve
atomic IDs and physical frozen traversal, including negative or coincident X.
Do not derive height from endpoints, substitute ascent for the local baseline,
recompute extents from fonts, or insert Kern/Glue material.

The completed Box uses the maximum item width, the final running end as Extent,
and the first published baseline with HasBaseline=true. Widths remain the maximum
of frozen occupied/target widths; horizontal skips and fit diagnostics are
unchanged. No width constraint or horizontal alignment option belongs in Restack.

Validation must distinguish arithmetic agreement from the finite dimension domain:

- Preserve the existing 1e-9-big-point arithmetic-equality tolerance in NewVList
  and NewFragment. Never widen or tighten it for restacking, nor introduce a new
  exact equality between the translated baseline and the retained ascent.
- Check item width/height, signed baseline and extents, and derived local
  `b.Depth()` against the existing strict domain: finite and magnitude less than
  `dimen.Fil.Points()`. Apply nonnegativity only to widths, heights and stack tops.
- Check each accumulated end, placed baseline and atomic origin, and the completed
  Box including `Box.Depth()`, before publication. Finite inputs do not guarantee
  finite-domain sums. Do not clamp values, narrow to DU or repair geometry.
- Also check the local values obtained by subtracting each newly published top
  again, including derived depth and atomic origins. A successful result must
  remain acceptable to Unbox; rounding must not publish an immediately unusable
  result. Share finite-domain validation internally where appropriate rather than
  drifting from Unbox's rules.
- Publish through NewVList after all additional checks. Its existing structural
  and arithmetic validation remains authoritative, including exact line/material
  placement anchors. Failure returns no VList and changes no source state.

Restack preserves stored heights, extents and horizontal content exactly; translated
coordinates are subject to float64 rounding. It is not a bitwise inverse of Unbox.
An accepted source can lie close enough to a numerical boundary that a new sum or
translation cannot be published under the unchanged rules. Report that error;
do not silently snap the baseline to ascent or relax a tolerance to force success.
Repeated calls with the same immutable inputs must produce the same result or
error. Test repeated unbox/restack cycles separately for drift and boundary errors.

### Immutable results and application margins

Allocate fresh line, material and atomic-placement slices; publish detached
geometry through NewVList. Share only the immutable frozen store and its stable
font/scale services. No ElasticVList item, previous VList, fragment or application
result is mutated. Result geometry must survive releasing the input list; both
old and new results remain independently readable, including concurrently.

Tytui should pass the frozen owner of the same retained group and keep Source,
group/owner identity, paint/source maps and SurroundingMargins alongside the new
VList in application-owned state. Copy margin metadata by value when publishing
a new envelope, preserving nil/missing versus explicit zeros. Restack itself takes
no margins and adds no CSS dependency to typography. Margins do not affect line
tops, paragraph height or baseline, and are not replicated onto individual items.

Keep signed uncollapsed margins in CSS pixels under the original OwnerID and
resolution context. CSS core applies them once when placing the enclosing block;
a changed containing context requires fresh CSS resolution. Padding, borders and
page-edge margin policy remain outside this operation. A new VList alone is not
a newly laid-out document: do not replace a retained group's VList inside an old
Result while reusing possibly inconsistent CSS document geometry. Publication of
new document placements and renderer integration need their own later step.

Replay remains version 5 with unchanged DTOs and evidence. Do not serialize the
ElasticVList, restacking handles, frozen services or surrounding-margin metadata.

### Implementation stages and verification (all deferred)

1. Add owning layout regressions for the accepted associations and numerical
   rules, then implement Restack using the existing immutable publication path.
   Add no sequence-editing API or new generic vertical node type.
2. Verify the application boundary in tytui/documentlayout using retained group
   owners and detached margin snapshots. Exercise the new VList independently;
   keep the existing document/rendering path and replay evidence unchanged.
3. Run the required checks and update API/package guidance only after the
   implementation is complete. Rendering integration and packing remain separate.

Required regressions for that future work:

- Independently expected natural geometry for mixed sizes, struts, surviving
  spaces, negative leading/baselines, overfull and zero widths, preserved blank
  lines, zero-height lines and suppressed empty output with retained diagnostics.
- Exact heights/extents, frozen owner/records, source coordinates, replacements,
  feature domains and LTR/RTL atomic order; no metric, shaping or breaking calls.
- Strict limits on local and completed depth, accumulated height, baselines and
  atomic coordinates; valid tolerated arithmetic discrepancies remain accepted.
  Include Unbox's near-limit depth regression, new translation failures and
  repeated cycles. Compare fractional coordinates under explicit rounding
  expectations, not by regenerating proof fixtures.
- Wrong/nil owners, nil lists and invalid items; missing, repeated, reordered or
  mixed-owner records if constructed in internal tests. No partial result after a
  late error. Repeat success/failure without changing any earlier result.
- Detached accessors, source/list lifetime independence and concurrent reads;
  application margins retain positive, negative, zero, missing, fractional and
  final auto-resolved values, without adding to natural height or mutating Source.

Run ordinary/race suites, build and vet in typography and tytui, including generic
dependency audits, adapter regressions and current replay/document controls.
Run relevant khipu/css-box-layout checks if implementation changes those boundaries.
These are future requirements, not checks performed for this documentation proposal.

## Deferred work and unboxing verification

Restacking/rendering integration, target-height packing, elasticity,
stretch/shrink, general box-and-glue lists and recursive container unboxing remain
deferred. So do automatic page breaking, widows/orphans, page-edge margin handling,
repeated border/padding decorations, floats and footnotes. There is no generic
item union, spacing-node API or new packing representation in this implementation.

The proposed restacking rules above are not implemented. Existing unboxing
regressions cover:

- Mixed fonts/sizes, struts, spaces, negative leading, signed baselines, empty
  paragraphs, preserved blank lines and distinct zero-height lines.
- Repeated unboxing, detached slices/margin records, item usability after releasing
  the original VList/list handles, and unchanged original VList/fragment geometry.
- Line-local baseline/atomic translation, exact placement anchors, finite bounds,
  LTR/RTL traversal, original source associations and metric-query bypass.
- Positive, negative and zero surrounding margins; missing versus explicit zero
  metadata; owner association and changed resolution contexts without mutation.
- Nil/invalid input and geometry failures without partial output, and unchanged
  tycli/document proofs and current replay evidence.

For implementation changes, run ordinary/race suites, build and vet in typography
and tytui, including dependency audits and adapter regressions. Run relevant
khipu/css-box-layout checks when those boundaries change. Preserve static evidence;
do not regenerate fixtures to conceal drift.

### Verification completed (2026-11-17)

Passed in typography and the installed sibling tytui checkout:

- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go build ./...`
- `go vet ./...`

The suites include generic dependency/branch audits, adapter regressions and
the unchanged replay/document proof controls. Focused unboxing regressions cover
local geometry, signed/zero heights, atomic traversal and overflow, detached
ownership, concurrent reads, retained content lifetime, mixed sizes, selected
replacements, real-font glyph/source/feature retention and service-query bypass.
Application regressions cover original collapsed-margin inputs, final nested-auto
resolution, resize stability, fractional CSS values, missing versus zero metadata,
failure publication and unchanged proofs. Existing document cancellation tests pass.

Also passed `go test ./... -count=1` in sibling khipu and css-box-layout.
Documentation links, API declarations, formatting and whitespace were checked.
No fixture evidence or replay schema was changed by this implementation.

## Historical implementation record

- Earlier stages 1–2 implemented immutable positioned line-range fragments and
  explicit interior line splits. Their ordinary/race/build/vet checks passed at
  that milestone; those APIs are not an implementation of this unboxing proposal.
- Review found a constructor-tolerance/rebasing inconsistency. CSS-only migration
  required identical line/material placement anchors and added regressions for
  the 5e-10-point discrepancy and repeated splitting. It also removed
  ProofCompatibility, DefaultEmSize, Line.Gap and frozen-only Assemble.
- CSS migration preserved tycli/documentlayout geometry and the eight existing
  CSS replay controls, moved replay to version 5 and retained older fixture bytes
  as historical evidence. Its verification is recorded in
  [VLIST_CONTRACT.md](VLIST_CONTRACT.md#validation-completed-2026-11-17).

Those checks describe the earlier milestones. The unboxing implementation and
its verification are separate from those historical results.
