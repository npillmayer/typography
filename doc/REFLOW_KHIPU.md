# Stage 1: immutable Khipu and independent reflow decisions

Design specification, 2026-11-19. The khipu contracts are implemented by PR 1
of [REFLOW_PLAN.md](REFLOW_PLAN.md); typography and application adoption remain
PR 2 and later work. Go sketches describe contracts and illustrative private
organization, not an exact transcription of implementation fields. Migration
tables below compare the pre-PR-1 representations with the new model.

Public reflow APIs are synchronous and context-free. Only streaming shaping and
font loading use contexts; internal stream failure cleanup remains mandatory.
See [khipu usage](../../khipu/README.md) and
[PR 1 migration/verification](../../khipu/doc/REFLOW_MIGRATION.md).

## Decisions

1. `Khipu` is the immutable liquid paragraph stored through region registers.
   Its knot properties use private SoA planes. It consolidates the reusable
   material of `PreparedParagraph` and today's `Khipu`; there is no second
   retained AoS knot-template representation.
2. A paragraph incarnation fixes source, mapped styles, bidi/shaping context,
   fonts, spacing capacities and discovery policy. Changes create a new
   incarnation. Frame geometry, atomic realizations and breaker selection do not.
3. Atomic advances live in an immutable `AtomicBinding`. Selected break decisions
   live in an immutable `Tikray`, the retained result of one paragraph trial.
   Neither changes Khipu. Decisions vary between Tikrays; a published Tikray never
   becomes the mutable workspace for another attempt.
4. Lazy discovery uses a separate synchronized `Discovery` facility scoped to one
   incarnation. Completed immutable batches are reusable; selected choices are
   never cached there. Register values do not contain mutable discovery maps.
5. Breaker workspaces are exclusive to one invocation. Plans and frozen results
   never refer to recyclable workspace storage.
6. Freezing consumes a plan's decisions and captured geometry. It does not read
   “the latest selections,” call a width provider again, or discover alternatives.
7. `css-box-layout` owns persistent register collections. Client selection of a
   successor version is separate from paragraph construction, freezing and
   discovery. Page-decision synchronization is not decided here.

The specification preserves existing source coordinates, glyph feature domains,
font contracts, full CSS line-height and geometry validation. It does not enable
previously unsupported bidi, hyphenation or atomic fragmentation behavior.

## 1. Identity, construction and immutable base material

An identity is an opaque comparable capability, scoped to the process. It is
neither a content hash, a wire ID nor a pointer to the whole Khipu. Separately
constructing identical text produces distinct incarnations; copying a handle
preserves identity. Zero identities/handles are invalid for execution.

```go
// Proposed package khipu. Nonzero-size tokens ensure distinct live allocations.
type IncarnationID struct { token *incarnationToken }
type incarnationToken struct { marker byte }

type Khipu struct {
    id       IncarnationID
    source   *paragraphSource
    policy   materialPolicy

    w, minW, maxW []dimen.DU
    penalty       []Penalty
    pos           []uint64
    length        []uint16
    kind          []KnotType
    flags         []KnotFlags
    boxRefs       []shapedBoxID
    atomicSlot    []int // -1 for non-atomic; otherwise index in atomicIDs
    provenance    knotProvenance

    atomicIDs []contracts.AtomicID // occurrence order; unique, nonzero
    shaping   *baseShaping
}

type knotProvenance struct {
    byteStart, byteEnd, scalarStart, scalarEnd []uint64
    breakByteEnd, breakScalarEnd              []uint64
    breakPenalty                             []Penalty
    breakWordBoundary, breakSoftHyphen        []bool
}

type paragraphSource struct {
    paragraph *styled.Paragraph
    // Independently allocated holder; never points into Khipu/Prepared/workspace.
}

type baseShaping struct {
    boxes      []shapedBox
    glyphs     []contracts.ShapedGlyph
    clusters   []storedGlyphCluster
    contexts   []preparedRun
    boundaries []sourceBoundary
}

type materialPolicy struct {
    hyphenation HyphenationMode
    language    language.Tag
    script      language.Script
    bidiDir     bidi.Direction
    hyphenPenalty int
    spaceStretchPercent, spaceShrinkPercent float64
    paragraphFill WidthTriple
}
```

The eight core knot planes and every knot-indexed association/provenance plane
have exactly the same length. `atomicIDs` has one entry per occurrence, not per
knot. The generated paragraph trailer remains a distinct final knot with the
existing empty source span, forced break and fill semantics. Empty source still
has the canonical horizontal representation; CSS assembly retains its existing
suppressed-empty behavior.

`pos`/`length` preserve current absolute byte positions and the existing uint16
per-knot length bound. Provenance preserves original paragraph-relative byte and
scalar ranges, including break boundaries outside visible glyph-bearing spans.
No coordinate narrowing, text rewriting, new knot partition or penalty change is
part of this migration. Compilation validates all plane lengths, associations,
boundaries, ranges and finite domains before publishing.

The SoA requirement applies to retained knot properties. Immutable glyph,
cluster, shaping-context and Unicode-boundary records remain their own record
arrays. A temporary `KnotCore` value returned by an accessor is not another
retained AoS paragraph. Construction may use private transient candidate records;
those are released after compilation.

Atomic slots have no usable width until bound. Their private width cells are
canonical zero placeholders, not zero-width promises. Clients cannot read them
as ready-to-break material. Non-atomic widths retain the current fixed/elastic
semantics, including generated fill sentinels. Ordinary bounds and atomic
advances remain strictly within their existing finite domains.

`paragraphSource` borrows the stable styled paragraph under the existing source
and style lifetime contract. Khipu's API never exposes a mutable paragraph or
style setter. Font faces/scales remain stable shared services; “immutable Khipu”
does not imply copying arbitrary service implementations. The holder is separate
so frozen output can retain source without retaining Khipu or a constructor.

`baseShaping` retains all existing box source/context/range associations, glyph
records, cluster mappings, run-relative features and Unicode boundary metadata.
Feature slices are copied at publication. Base records are never extended for
lazy alternatives: replacement batches have independent immutable storage.

The recipe used for discovery must match `materialPolicy` and resolved run
contexts. Geometry fields from today's `Params` do not enter this policy.
`paragraphFill` is compiled material; changing it creates another incarnation.
Provider/factory identities are bound at construction through the discovery
recipe, not compared deeply on every trial. Their behavior must remain stable;
mutating a supplied provider violates the incarnation contract.

### Proposed constructor and access surface

```go
// Khipukamayuq remains a configured construction executor, not material identity.
func (q *Khipukamayuq) PrepareParagraph(
    input ParagraphInput, options PrepareOptions,
) (*Khipu, *Discovery, error)

type KnotTemplate struct {
    Core         KnotCore
    Atomic       contracts.AtomicID
    MetricsKnown bool
}

func (k *Khipu) ID() IncarnationID
func (k *Khipu) KnotCount() int
func (k *Khipu) Knot(index int) (KnotTemplate, bool)
func (k *Khipu) AtomicIDs() []contracts.AtomicID
func (k *Khipu) HasExplicitDiscretionary(index int) bool
```

Preparation resolves/shapes/compiles once and returns both handles or neither.
It constructs an empty, nonnil discovery facility even for `NoHyphenation`.
The constructor initializes the discovery gate and copies inputs it owns, keeping
the existing borrowing contracts
for source/styles/services. It performs no target-width layout. Existing streaming
CheckFace, draining, terminal-error and cleanup guarantees remain in force.
OpenType acceptance stays only in the glyphing adapter and is not forwarded to
`opentype/otshape`; see [FONT_SHAPER.md](FONT_SHAPER.md).

`Knot` returns a detached value: for an atomic slot, `Atomic != 0`,
`MetricsKnown == false`, and `Core`'s zero widths are unusable placeholders.
Other metadata is meaningful. `AtomicIDs` returns a copy. No getter returns a
writable plane, map or feature slice. Internal hot loops may use scalar accessors
or private read-only views; public cross-package breakers must not receive raw
mutable slices as a performance shortcut. Accessor overhead is measured later.

`HasExplicitDiscretionary` reads retained U+00AD source metadata only. It returns
false for nil/uninitialized material, invalid indexes, non-textboxes and
`NoHyphenation`. It neither queries a provider/cache nor promises an eligible
variant. Preserve first-fit's use of this predicate: an overflowing first word
with an explicit marker may request discovery even without an earlier feasible
breakpoint; ordinary first words must not trigger eager automatic discovery.

Remove public material mutation (`SetKnotFlags`, `Unskip`, append operations and
plane assignments) from the published type. Construction helpers may retain
private equivalents before publication. Manually constructed metric test inputs
need a separate validated builder/test fixture path; they must not masquerade as
shaped production Khipus or acquire freeze provenance through a boolean flag.

## 2. Atomic bindings and realization ownership

```go
type AtomicBinding struct {
    material *Khipu
    advances []dimen.DU // canonical atomic-slot order, copied on construction
}

func BindParagraph(k *Khipu,
    advances []AtomicAdvance) (*AtomicBinding, error)
func (b *AtomicBinding) Incarnation() IncarnationID
func (b *AtomicBinding) KnotCount() int
func (b *AtomicBinding) Knot(index int) (KnotCore, bool)
func (b *AtomicBinding) AtomicAdvances() []AtomicAdvance
func (b *AtomicBinding) HasExplicitDiscretionary(index int) bool
```

Supply exactly one finite signed advance per ID, in any input order. Reject
unknown, duplicate or missing IDs before publishing. A text-only binding takes
an empty list and does not allocate full knot copies. `Knot` combines immutable
base metadata with the bound atomic width, setting `W == MinW == MaxW` for atoms.
A real zero advance is valid and distinguishable from an unbound slot.
`KnotCount` and `HasExplicitDiscretionary` delegate to the immutable material.
Together with `Knot`, they give sibling breaker packages the read-only scan
surface without exposing the binding's private material pointer or planes.

A binding is immutable, contains no frame width and performs no shaping,
discovery or breaking. It shares the base SoA; it is not a second full Khipu.
Equivalent bindings have the same incarnation and canonical advance values;
pointer equality is a fast positive check, not the sole equivalence definition.
No content hashing is required. A break plan retains its exact binding, so callers
cannot substitute a different one when freezing.

The complete atomic realization remains in CSS/application integration: border
box, margins, alignment and matching nested interior output. Each new trial lays
out atomics under retained content properties and its own constraints. Typography
copies geometry into that trial's assembly request and derives advances from it.
An application trial envelope retains the matching interior result. Equal advances
do not prove equal heights/interiors. No CSS or DOM types enter `AtomicBinding`.
Realization caching remains deferred; binding reuse never licenses skipping the
new-trial realization contract without an explicit compatible optimization.

## 3. Discovery caching: a concrete ownership choice

Choose a **separate synchronized memoization facility**, not persistent mutation
of Khipu and not a mutable object inside the functional register collection.
The client keeps a session-scoped association from incarnation to `Discovery`.
Prepared orchestration handles may own that association while active. Trial plans
and frozen results do not retain the facility.

```go
type AlternativeID struct {
    Incarnation IncarnationID
    Source      int    // original textbox knot index
    Variant     uint16 // stable nonzero normalized variant within that source
}

type discoveryRecipe struct {
    hyphenator Hyphenator
    newShaper func() (contracts.StreamingShaper, error)
    bufferOptions contracts.ShapeBufferOptions
    // Fixed material policy and resolved run contexts come from material.
}

type Discovery struct {
    material *Khipu
    recipe   discoveryRecipe
    gate     sync.Mutex // serializes synchronous discovery misses
    mu       sync.Mutex
    batches  map[int]*AlternativeSet
}

type AlternativeSet struct {
    incarnation IncarnationID
    source      int
    positions   int // explicit-position count needed by intrinsic admission
    alternatives []*alternative
}

type alternative struct {
    id        AlternativeID
    preCore, postCore KnotCore
    split     SourceSplit
    pre, post *replacementMaterial
}

type replacementMaterial struct {
    input   replacementInput
    params  runParameters
    glyphs  []contracts.ShapedGlyph
    clusters []storedGlyphCluster
    widths  candidateWidths
}

func (d *Discovery) Alternatives(sourceKnot int) (*AlternativeSet, error)
type Alternative struct {
    ID AlternativeID
    PreBreak, PostBreak KnotCore
    SourceSplit SourceSplit
}
func (a *AlternativeSet) IDs() []AlternativeID
func (a *AlternativeSet) Candidates() []Alternative
```

`discoveryRecipe` captures the fixed hyphenator and streaming factory/options
from construction; resolved policy and contexts are read from its immutable
Khipu. The recipe is private and never replaced during the incarnation. Calls using shared collaborators still
obey their serialization requirements, including across different facilities.
The gate is not a claim that a shared provider is safe across incarnations.

A layout call to `Alternatives` returns an immutable cached set. On a miss,
acquire the discovery gate, recheck the cache, discover and normalize all inputs,
shape the whole batch privately, then publish under the cache mutex. Release the
gate on every outcome. Concurrent requests reuse successful publication; failures
publish nothing and a waiting caller may retry. There is no detached producer.
A successfully published batch may survive later failure or rejection of a trial.

Discovery misses are serialized per incarnation. Completed reads and independent
search work can be concurrent after service audits. Public preparation, binding,
discovery, measurement, breaking, freezing and checkpoint APIs are synchronous
and take no context. Contexts belong only at streaming-shaper and font-loader
boundaries; preserve internal stream cancellation, draining and producer joining
on errors. No caller cancellation of CPU search or discovery-lock waiting is
promised. This does not repair the upstream bidi limitation.

A present empty set means successful empty discovery; an absent key means unknown.
Do not cache errors. No partial pre/post pair is published. Intrinsic measurement
uses a private admission path through the same gate/cache, with checks **before
shaping on a miss**, as specified in Section 6. It must not call the layout
`Alternatives` path unconditionally and then reject an already shaped batch.
Admission failure publishes no entry and leaves ordinary layout free to discover
the complete batch later. Automatic and explicit discovery use the incarnation's
fixed mode; changing discovery rules creates a new incarnation. Trial scoring and
eligibility never change the cache's universe or populate a filtered negative.

Alternative IDs derive from incarnation, source occurrence and a **normalized**
variant, not the order in which trials request discovery or worker arena offsets.
Provider input IDs and published IDs have different contracts:

- Automatic discovery preserves valid nonzero provider IDs. Validate the whole
  response and apply the existing empty-half filtering; reserve all retained
  nonzero IDs, then assign zero IDs the lowest unused positive IDs in response
  order. Reject duplicate retained nonzero IDs and variant-count overflow.
- Explicit-marker discovery ignores provider `Variant` and `SourceSplit` fields.
  Khipu supplies the original marker splits and numbers the expanded alternatives
  in source-position order, then provider-material order, starting at one. Do not
  merge this namespace with automatic alternatives for the same textbox; explicit
  opportunities retain their existing precedence.
- Published alternatives must have unique nonzero IDs. Never reject a valid
  provider response merely because its input IDs are zero, and never sort the
  candidates by assigned ID: response/material order remains the tie-break order.

Preserve the existing validation/filtering sequence in
[automatic](../../khipu/discretionary_automatic.go) and
[explicit](../../khipu/discretionary_explicit.go) preparation. A nondeterministic
provider violates the fixed-incarnation contract. Unknown source splits must not
be invented from replacement lengths; keep existing production provenance
admission checks. Providers requiring mutable Khipu access must migrate to
read-only source/context input; no callback may install selected decisions as a
discovery side effect.

`Candidates` returns detached metric/provenance snapshots for breaker evaluation;
internal pre/post cores retain the current candidate penalties, flags and source
length semantics. Glyph/replacement references remain private. Sets and
replacements expose copies/value accessors, not writable slices. Plans
retain selected immutable replacement objects directly, so clearing an unneeded
session cache cannot invalidate them. No eviction policy is introduced: discard
the facility when no active work needs it. Frozen output retains only the immutable
replacement/source records it needs, never `Discovery` or its recipe/services for
future shaping. The representation may retain unused immutable records initially,
but must not retain a working cache through an accidental backlink.

## 4. Break requests, independent plans and captured geometry

```go
type BreakRequest struct {
    Binding   *AtomicBinding
    Discovery *Discovery
    Geometry  GeometryRequest
    Prefix    *BreakCheckpoint // nil starts at the paragraph beginning
}
func (r BreakRequest) Validate() error

type GeometryRequest struct {
    Shape LineShape
    LeftSkip, RightSkip KnotCore // only W/MinW/MaxW used
}

type BoundaryKind uint8
const (
    OrdinaryBoundary BoundaryKind = iota + 1
    DiscretionaryBoundary
)
type BreakBoundary struct {
    Kind        BoundaryKind
    Knot        int // inclusive ordinary endpoint, or replaced textbox
    Alternative AlternativeID // zero for ordinary
}
type WidthTriple struct { Natural, Minimum, Maximum dimen.DU }
type LineGeometry struct {
    TargetWidth dimen.DU
    LeftSkip, RightSkip WidthTriple
}
type LineDecision struct {
    End      BreakBoundary
    Geometry LineGeometry
}

type Tikray struct {
    binding      *AtomicBinding
    lines        []LineDecision
    replacements map[AlternativeID]*alternative // selected only; immutable
    policies     []PolicyRecord
    linePolicy   []int // one policy record index per line
    diagnostics  BreakDiagnostics
}

func (p *Tikray) Incarnation() IncarnationID
func (p *Tikray) Lines() []LineDecision
func (p *Tikray) Policies() []PolicyRecord
func (p *Tikray) BreakpointPositions() ([]uint64, error)
func (p *Tikray) Diagnostics() BreakDiagnostics
func (p *Tikray) PolicyForLine(index int) (PolicyRecord, bool)
func Freeze(plan *Tikray) (*FrozenKhipu, error)
func NewTikray(request BreakRequest,
    decisions []LineDecision, policy PolicyRecord, diagnostics BreakDiagnostics,
) (*Tikray, error)
```

`Tikray` names the immutable paragraph trial plan. Its private binding, decisions,
selected replacements, captured geometry, policy and diagnostics belong to that
attempt. Accessors return detached values/slices. Provisional choices, width
memoization and search arrays stay in a private per-invocation workspace; they
are not a mutable phase of a published Tikray. There is no shared "current
Tikray" on Khipu or Discovery, and creating one does not commit a page candidate.

`BreakpointPositions` maps this plan's complete decisions to original paragraph-relative
UTF-8 endpoints, without discovery. It replaces the old Khipu-plus-path query.

`Diagnostics` returns the retained `BreakDiagnostics` by value. A nil receiver
or uninitialized zero-value Tikray returns `BreakDiagnostics{}`. `HasScore` and
`HasPass` distinguish unavailable information from a valid zero score/pass;
these flags do not certify that a Tikray is initialized or executable.

`PolicyForLine` takes a **zero-based index into `Lines()`**, including locked
prefix lines; it is not a fragment-local index or a one-based shape line number.
For a valid index it returns the resolved `PolicyRecord` selected through
`linePolicy`, by value, and `true`. A negative index, an index at or beyond the
line count, a nil receiver or an uninitialized zero-value Tikray returns
`PolicyRecord{}, false`, without panicking, clamping or choosing a default.
Thus a continued Tikray exposes the original KP policy for its accepted prefix
and the first-fit policy for its newly searched suffix.

Both accessors read retained data only, with no service, discovery or breaker
calls. They expose no pointers or writable storage from the Tikray. Editing a
returned record cannot affect later inspection, freezing or checkpoint creation.
The private policy mapping remains encapsulated; `Policies()` alone is not a
per-line attribution list.

`BreakRequest.Validate` checks initialized binding/discovery handles,
matching incarnation, geometry inputs and any checkpoint's binding compatibility
before a sibling breaker searches. It uses the same prefix validator as
`SeedFor` below, without copying a seed, scanning prefix knots or querying Shape,
discovery or font services. Checking geometry inputs validates the shape handle
and skip triples; target widths are validated when first queried during search.
It does not choose an algorithm or admit unsupported seeded KP execution.

`NewTikray` is the shared, validated publication seam for the sibling breaker
packages. It revalidates the request, copies all envelopes/slices, verifies the
exact incarnation and binding, resolves selected alternative IDs from already
published batches (or certified prefix references), and retains those references.
It must not discover missing alternatives. Invalid kinds, fabricated IDs, foreign variants, prohibited
breaks, skipped mandatory boundaries and inconsistent prefix geometry fail.
The publisher validates captured geometry without calling `Shape` again; it
certifies path legality and dependency consistency, not optimality or whether a
custom caller truthfully reports its search score. Plans represent a complete
legal path through the paragraph-final trailer; partial search output is workspace
state, never a successful plan. Required nonnil handles and incarnation matches
are checked before search/measurement, even with NoHyphenation; this does not
require any discovery query on that path.

```go
type Algorithm uint8
const (
    FirstFitAlgorithm Algorithm = iota + 1
    KnuthPlassAlgorithm
)
type FirstFitSettings struct { MinHyphenGain dimen.DU }
type KnuthPlassSettings struct {
    PreTolerance, Tolerance, LinePenalty int64
    FirstHyphenDemerits, FinalHyphenDemerits int64
    EmergencyStretch dimen.DU
}
type PolicyRecord struct {
    Algorithm Algorithm
    FirstFit FirstFitSettings
    KnuthPlass KnuthPlassSettings
}
type BreakDiagnostics struct {
    HasScore bool
    Score int64
    HasPass bool
    Pass int
}
```

Diagnostics contain only information actually produced by the breaker. Physical
fit diagnostics remain on frozen lines; do not invent them from search scores.
For a continued plan, Score/Pass describe the suffix search if available, never
a newly optimized score for the locked complete paragraph. Diagnostics are not a
page-level acceptance judgment or a timing probe.

`PolicyRecord`'s inactive arm must be zero. Copy resolved defaults, not nil/default
placeholders. Paragraph fill is material; providers come from `Discovery`; neither
is a breaker option. Emergency stretch remains scoring-only, never physical width.
`linePolicy` preserves an earlier KP prefix followed by first-fit without falsely
attributing all lines to one algorithm. The shared publisher takes the suffix
policy from its argument and prefix policies from the checkpoint. Publication
requires exactly one valid policy-record index per line; `PolicyForLine` reads
that certified mapping. No search graph or mutable custom policy callback is
retained.

During search, each workspace memoizes the first validated width returned for a
one-based line number. Scoring uses that captured value consistently. `Shape`
remains borrowed and stable only for the duration of the call. Selected widths
and skips are recorded in the plan, then the callback is released. Freezing
accepts no independent Params/shape argument and never queries `Shape`. Thus
changing the caller's schedule after a trial cannot alter that candidate. A
seeded search imports accepted widths from its checkpoint without querying the
prefix schedule. General height-dependent frame solving remains outside khipu.

Proposed breaker boundaries, in their existing packages:

```go
// firstfit and knuthplass each retain their own typed algorithm Parameters.
func BreakParagraph(request khipu.BreakRequest,
    params *Parameters) (*khipu.Tikray, error)
// firstfit only initially: requires request.Prefix != nil.
func ContinueParagraph(request khipu.BreakRequest,
    params *Parameters) (*khipu.Tikray, error)
```

`BreakParagraph` starts a fresh paragraph and requires a nil Prefix;
`firstfit.ContinueParagraph` requires a nonnil checkpoint. A seeded KP request
returns `ErrUnsupportedPolicy` until an explicit continuation contract is added.
Both algorithms own fresh private search state and publish through the common
validator. Nil algorithm parameters keep existing effective defaults. Neither
writes Khipu, binding or shared selections. Errors return nil plans;
successful discovery can remain cached. Search and publication are synchronous;
there is no public context or mid-call cancellation contract.

Freezing performs existing trimming, replacement substitution, line-local bidi
ordering and width allocation using captured geometry. It neither shapes nor
queries a hyphenator. Preserve physical traversal, source provenance, fill,
negative/zero geometry and diagnostic outcomes. Frozen output owns its planes
and retains stable source/shaping records, not the Tikray, binding, Khipu,
checkpoint or Discovery. Avoid interior pointers into these working handles.

## 5. Continuations, prefix validation and replacement carry

```go
type ResumePosition struct {
    NextKnot int
    Carry    AlternativeID // zero, or pending post-break replacement
}

type BreakCheckpoint struct {
    material     *Khipu
    prefix       []LineDecision
    replacements map[AlternativeID]*alternative
    policies     []PolicyRecord
    linePolicy   []int
    prefixAtomics []AtomicAdvance
    resume       ResumePosition
}

func Checkpoint(plan *Tikray,
    acceptedLines int) (*BreakCheckpoint, error)

// Detached breaker-owned snapshot, not an independently trusted certificate.
type ContinuationSeed struct {
    Prefix    []LineDecision // copied accepted decisions, including widths/skips
    Resume    ResumePosition
    PostBreak KnotCore // carried metrics/flags; meaningful iff Resume.Carry != zero
}
func (s ContinuationSeed) AcceptedLines() int // len(s.Prefix)
func (c *BreakCheckpoint) SeedFor(binding *AtomicBinding) (ContinuationSeed, error)
```

Only a validated successful plan creates a checkpoint; arbitrary byte offsets or
caller-supplied endpoint lists cannot. Zero accepted lines and the entire path
are legal. Copy the accepted prefix, its geometry/policy records and selected
replacement references; do not retain the entire plan or a chain of checkpoints.
The checkpoint intentionally retains its incarnation while continuation is needed.

`SeedFor` is the replacement for today's `PrefixFor` and the public read-only
seam used by `firstfit.ContinueParagraph`. It rejects nil/zero
handles, foreign incarnations and changed consumed atomic advances. Validation uses the checkpoint's already certified records
and retained atomic IDs/values, not a new scan/search of prefix knots. It performs
no width query, discovery, shaping, CheckFace or metric service call.

On success, the seed owns a copy of the prefix and value copies of the resume
position and pending post-break `KnotCore`, including its trimming/content flags.
`AcceptedLines()` is the prefix length, so the next queried line number is
`AcceptedLines()+1`. Carry presence comes from `Resume.Carry`, never from a
nonzero advance or nonempty glyph sequence. Its immutable glyph/provenance
records stay private in the checkpoint; no cache lookup is needed to seed search.
Zero accepted lines yield `NextKnot == 0` with no carry. A fully accepted path
yields `NextKnot == binding.KnotCount()` with no carry and needs no suffix search.
Errors return a zero seed, never a partial prefix.

The breaker may modify its detached seed/workspace without changing the
checkpoint. A seed is not accepted as a substitute for `BreakRequest.Prefix`:
`NewTikray` rechecks the complete decisions against that original immutable
checkpoint, including prefix geometry and selected replacement identity. Seed
edits therefore cannot rewrite a locked prefix or manufacture carry provenance.
Prefix policy records are imported privately by the publisher, not reconstructed
by the breaker. Full freezing/assembly may still revisit prefix material afterward.

After an ordinary endpoint k, resume at k+1 with no carry. After a discretionary
endpoint at textbox k, resume at k+1 **with that alternative's post-break material
pending**. The next line begins with the retained post-break replacement, not
the original textbox and not a newly shaped source substring. Empty replacements
still have identity/provenance and must be consumed correctly. A complete prefix
has no pending carry and resumes past the final trailer.

Selected replacements carry original source association and independent local
replacement coordinates/features. Retain whole-paragraph bidi context and existing
replacement materialization rules; do not reset isolates/direction at a frame
boundary or infer a source split from replacement lengths. Ordinary and
replacement transitions must make logical progress. General repeated subdivision
of a post-break replacement is unsupported; return `ErrUnsupportedPolicy` rather
than manufacture a knot offset. Existing upstream feature/replacement limits
remain admission limits until explicitly extended.

Continuation verifies incarnation, prefix endpoints and geometry, then checks
new binding advances for atomic IDs actually consumed by accepted prefix lines.
Different advances for suffix-only atoms are allowed. A change to a consumed
atomic advance requires an earlier checkpoint/restart. The resumed complete plan
uses the new binding, whose prefix values have been checked, and copies locked
prefix decisions exactly. Incarnation identity alone is insufficient.

Vertical/interior compatibility is stronger than advance equality. Typography
and the application must separately retain and compare accepted prefix line-height
inputs and full atomic realizations; changing a prefix object's height/interior
cannot reuse its old positioned fragment merely because the advance matches.
A new frame-set trial reconsiders tentative prefixes from its input collection;
previously committed output is never silently changed.

The first continuation implementation remains first-fit, preserving an initial
KP prefix without resuming its optimization search. Current first-fit settings
need no extra previous-line fitness state beyond logical position/carry and
accepted-line count. Any future continuation policy that needs fitness, hyphen
streak or other history must add a validated policy-specific summary or reject
that checkpoint; no inference from missing history is permitted. General KP
continuation is not promised by the proposed Tikray API.

Migration first preserves the existing NoHyphenation gate. Enable hyphenating
continuation only after carry, provenance, mandatory-boundary and failure-recovery
regressions pass. The representation is defined here; general support is not
claimed before those implementations and checks exist.

## 6. Intrinsic measurement and typography integration

```go
func MeasureIntrinsic(k *Khipu, discovery *Discovery,
    request MeasureRequest) (IntrinsicWidths, error)
```

Measurement reads immutable SoA templates and supplied `AtomicIntrinsic` ranges;
it does not construct a fixed binding or alter a plan. Preserve signed span
normalization, no generated fill/skips, no vertical metrics and current
NoHyphenation/bounded ExplicitHyphenation admission. Complete replacement batches
are shared with breaking. Automatic/multiple-split measurement remains unsupported.

Preserve the current intrinsic admission and error ordering, including its
zero-call rejection paths. On a cache miss, the private measurement discovery
path shares Discovery's synchronous gate and cache recheck, then:

1. Count eligible explicit source positions. More than one returns
   `ErrUnsupportedPolicy` before any hyphenator, shaper or CheckFace call. Zero
   positions produce a successful empty batch without those calls.
2. Prepare and validate the complete unshaped provider response, including
   variant normalization. When replacements exist, resolve/validate their feature
   parameters before the permitting-variant check, preserving the current
   feature-error precedence.
3. If more than one variant permits breaking, return `ErrUnsupportedPolicy`
   before creating any replacement stream or calling CheckFace/shaping.
4. Otherwise shape every variant, including prohibited ones, and publish only
   the complete successful batch (or a successful empty batch).

Admission rejection discards transient inputs and publishes no cache entry,
including no filtered negative or separately cached unshaped response. Errors
remain retryable and a later ordinary layout can discover the full batch. On a
cache hit, apply the same position/variant admission rules to the complete
immutable set without service calls or cache modification. A shape failure must
not replace an admission error that should have been returned before shaping.
Retain the service-count and error-precedence expectations in
[intrinsic_explicit_test.go](../../khipu/intrinsic_explicit_test.go).

Typography's `Prepared` may remain as the client orchestration handle, retaining
`*Khipu`, `*Discovery`, stable source/default resolver data and copied anchors/bidi
context as needed. It is not another horizontal material representation. A future
paragraph trial envelope pairs a Tikray with copied LineLayout, strut settings
and AtomicBox values and, after successful assembly, immutable paragraph output.
Application envelopes pair that output with source/paint and atomic interiors.
No concrete backend, DOM or CSS types enter generic material or plans.

`Run` remains one-shot text-only execution; reusable layout constructs a binding,
gets a plan, freezes it, then assembles CSS geometry. Initial migration can retain
full freezing/assembly, including the accepted prefix. `Continuation.Fit` retains
its geometry-only path; a requested different breaking policy cannot silently
reuse a plan selected under another policy. Atomic relayout costs in new page
trials are distinct from the zero-service/breaker-work retained fitting operation.

The immutable collection of registers lives in css-box-layout. It references
material through the agreed typed integration boundary, not a mutable executor.
Discovery facilities and service gates are session/execution dependencies, not
mutable register fields. Each trial returns output paired with a successor
collection; the client chooses which version to adopt. Page constraints,
cross-region associations and decision synchronization remain outside Stage 1.

## 7. Field-by-field migration

The tables cover the stored fields in the current sources, not only public API
names. All destinations above are proposed; current code remains unchanged.

### PreparedParagraph and preparedMaterial

| Current field | Destination / disposition |
| --- | --- |
| `PreparedParagraph.*preparedMaterial` | Consolidate into new Khipu; remove embedding and duplicate paragraph representation. |
| `PreparedParagraph.owner` | Remove material-to-executor ownership. Constructor configuration supplies discovery recipe; incarnation replaces worker identity checks. |
| `PreparedParagraph.atomicCount` | Derive from `len(Khipu.atomicIDs)`; validate bindings against exact IDs. |
| `PreparedParagraph.hyphenation` | `Khipu.policy.hyphenation`; fixed per incarnation. |
| `PreparedParagraph.explicit` | `Discovery.batches`; complete immutable AlternativeSets, independent of selected plans. |
| `preparedMaterial.paragraph` | `Khipu.source.paragraph`, in the independently allocated source holder. |
| `preparedMaterial.knots` | Private `w`, `minW`, `maxW`, `penalty`, `pos`, `length`, `kind`, `flags` planes. Remove retained `[]KnotCore`. |
| `preparedMaterial.boxRefs` | `Khipu.boxRefs`; immutable association to base shaped boxes. |
| `preparedMaterial.shaping` | Split into `Khipu.shaping`, provenance planes and atomic-slot associations as below. |

### Current Khipu

| Current field | Destination / disposition |
| --- | --- |
| `Khipu.owner` | Remove; plans/bindings certify incarnation and dependencies rather than a mutable worker. |
| `Khipu.paragraph` | Shared independent `paragraphSource` holder. |
| `Khipu.prepared` | Remove explicit-cache backlink; execution receives Discovery, selected plans retain replacement records. |
| `Khipu.boundMaterial` | Remove AoS-template backlink; binding/checkpoint retain the new Khipu and its identity. |
| `Khipu.W` | Private `Khipu.w`; atomic placeholders overridden only through AtomicBinding. |
| `Khipu.MinW` | Private `Khipu.minW`; atomics use bound fixed advance. |
| `Khipu.MaxW` | Private `Khipu.maxW`; atomics use bound fixed advance; preserve glue fill semantics. |
| `Khipu.Penalty` | Private `Khipu.penalty`; changing compiled penalties means new incarnation. |
| `Khipu.Pos` | Private `Khipu.pos`; preserve absolute source-byte positions. |
| `Khipu.Len` | Private `Khipu.length`; preserve source-byte lengths and bounds. |
| `Khipu.Kind` | Private `Khipu.kind`; no public editing. |
| `Khipu.Flags` | Private `Khipu.flags`; preserve trimming independent of penalty. |
| `Khipu.boxRefs` | Immutable `Khipu.boxRefs`; replacement references no longer appended here. |
| `Khipu.shaping` | Immutable base records/provenance; separate immutable replacement storage. |
| `Khipu.hyphenationPolicy` | Remove legacy gate. Every constructed incarnation has explicit policy; no ungated execution path. |
| `Khipu.hyphenation` | `Khipu.policy.hyphenation`. |
| `Khipu.DiscretionaryCandidates` | Discovery alternatives and read-only workspace views; no public mutable map. |
| `Khipu.SelectedDiscretionaries` | `Tikray.lines[].End` plus retained selected replacement references. |
| `Khipu.discretionaryEmpty` | Present empty `AlternativeSet` in `Discovery.batches`; absent remains unqueried. |

### Shaping/provenance and replacement records

| Current field | Destination / disposition |
| --- | --- |
| `shapingArena.boxes` | Immutable `baseShaping.boxes`; selected replacement boxes retained separately, then remapped when freezing if required. |
| `shapingArena.glyphs` | Immutable `baseShaping.glyphs`; replacement glyphs belong to replacementMaterial. |
| `shapingArena.clusters` | Immutable `baseShaping.clusters`; replacement clusters retain local input coordinates. |
| `shapingArena.contexts` | Immutable `baseShaping.contexts`, preserving FontInstance, features, script/language/direction and original run domains. |
| `shapingArena.sources` | Flatten `knotSource.source` into byte/scalar provenance planes; `atomic` becomes atomicSlot/atomicIDs; `breakAfter` becomes the five break provenance planes. |
| `shapingArena.boundaries` | Immutable `baseShaping.boundaries`, retaining paragraph-wide Unicode/soft-hyphen analysis. |
| `shapedBox.source/context/glyphs/clusters/replacement` | Preserve all associations; base boxes remain immutable. Replacement input and separate contexts stay with replacementMaterial; frozen internal box IDs may be remapped without changing source or glyph semantics. |
| `storedGlyphCluster.source/glyphs` | Preserve ranges in their original coordinate domain. |
| `preparedRun.source/params` | Preserve resolved run context and copied feature domains; never reconstruct from paint. |
| `explicitBatch.positions/replacements` | `AlternativeSet.positions/alternatives`; complete-batch publication with intrinsic admission before shaping on a miss. |
| `explicitReplacement.variant/penalty/pair` | Stable AlternativeID, preCore penalty, pre/post cores and replacement records, and SourceSplit. |
| `DiscretionaryCandidate.Variant/PreBreak/PostBreak/SourceSplit/preBox/postBox` | Stable ID, immutable pre/post metrics/provenance and owned replacement references; no worker-local arena IDs in public plans. |
| `DiscretionarySelection.Source/Variant` | Discretionary BreakBoundary's source knot and AlternativeID. |

### Adjacent handles and API changes

| Current operation/state | Proposed contract |
| --- | --- |
| `PrepareParagraph` | Returns Khipu and Discovery; no PreparedParagraph. Construction owns all publication validation. |
| `EncodeParagraph` / `publishCandidateParagraph` | Route one-shot construction through the same immutable compiler and explicit policy. Translate current supported legacy configuration deliberately; do not keep a hidden mutable Khipu representation. |
| `BindParagraph` | Returns immutable AtomicBinding, not a copied full Khipu. |
| Breaker `[]int` result plus mutable selections | Immutable Tikray result, effective policy snapshot and captured line geometry. |
| `Freeze(k, breaks, params)` | `Freeze(plan)`; no current selections/provider/width callback consulted. |
| `MeasureIntrinsic(prepared, request)` | Immutable Khipu + compatible Discovery + ranges; same width and policy semantics. |
| `BreakCheckpoint.prepared/prefix` | Khipu incarnation plus copied prefix decisions/geometry, replacement references, consumed atomic advances and explicit resume carry. |
| `Checkpoint(prepared, completePath, acceptedLines)` | `Checkpoint(validatedPlan, acceptedLines)`; validate and preserve decisions, not a bare integer path. |
| `PrefixFor(binding)` | `BreakCheckpoint.SeedFor(binding)` validates compatibility and returns detached prefix geometry, accepted-line count, resume position and carry metrics; no prefix search or width/service query. |
| `HasExplicitDiscretionary` / breaker plane scans | Metadata-only predicates on Khipu/binding plus binding `KnotCount`/`Knot` accessors; preserve lazy first-word rescue without exposing mutable storage. |
| `paragraph.Prepared.owner/content` | Orchestration keeps Khipu/Discovery, not an execution owner that defines material identity. Source/default services remain stable. |
| `Continuation.path/history/state` | Use certified plans/checkpoints and copied trial layout inputs; retain accepted fragments independently. No chain of prior workspaces or plans. |
| Root construction/test helpers that mutate planes | Private builders before publication or explicit metric-only test fixtures; audit every caller, not aliases preserving mutation. |

This is a deliberate source-API migration across khipu, typography and application
integration. It does not change font identity encoding or replay version 5.
Replay still records application inputs and output controls, never these handles,
plans, incarnation tokens or discovery services. Preserve historical evidence.

## 8. Ownership, release and verification gate

Khipu has no execution-owner/cache backlink. Binding retains Khipu; Discovery
retains Khipu and its fixed recipe; plans retain bindings and selected replacement
records; checkpoints retain Khipu and a detached accepted prefix. None retains
worker scratch. Source/shaping records may be independently shared by frozen
output, whose graph must exclude these working handles. Identity tokens alone
do not retain their material.

Persistent register versions, candidate plans and checkpoints may prolong an
incarnation's life. Release obsolete versions and session cache associations when
no permitted reconsideration needs them. A completed trial is not commitment.
Dropping the discovery facility does not invalidate a plan; dropping all working
handles does not invalidate frozen output. There is no global cache, serialized
service handle or implicit permanent retention.

Before Stage 2 is accepted, require these concrete regressions:

| Case | Required result |
| --- | --- |
| Mutate input slices and accessor return values | Published Khipu, bindings, plans and checkpoints unchanged. |
| Tikray diagnostics/policy value copies and invalid lookups | Editing returned records leaves inspection/freezing/checkpoints unchanged; nil/zero receivers return zero diagnostics and `(PolicyRecord{}, false)`; negative and past-end line indexes return the same failed policy lookup without panic or fallback. |
| Available zero versus unavailable diagnostics | `Diagnostics` preserves HasScore/HasPass and their values, including valid zeros; continued-plan diagnostics retain suffix-only meaning. |
| Empty input; zero and signed atomic advances | Existing empty/fill/finite-domain behavior; unbound slot cannot be mistaken for bound zero. |
| Same text constructed twice; foreign binding/discovery/alternative | Different incarnation; incompatible combinations rejected before publication. |
| Change an executor but retain the exact material/discovery | No false worker-identity rejection; service execution restrictions still honored. |
| Trial A, then B success/failure/rejection, then Freeze(A) | Identical A output, without restoring selections; no extra shape/CheckFace/discovery. |
| Mutate width callback state after successful breaking | Freeze uses captured widths; callback is neither retained nor called. |
| Warm/cold discovery and reversed trial order | Same alternatives, tie outcomes and output; failure never cached as empty. |
| Automatic zero/mixed provider IDs; explicit ignored IDs | Deterministic normalization, duplicate/overflow rejection and original candidate tie order; no rejection of valid zero-ID input. |
| Unsupported intrinsic positions/variants on a cold cache | Multiple positions: zero provider/shape/CheckFace calls. Multiple permitting variants: zero shape/CheckFace calls; feature errors retain precedence; no cache entry, later layout remains valid. |
| Intrinsic admission on a warm complete batch | Same admissibility and widths, zero service calls, complete batch unchanged. |
| Concurrent same-source cache misses; failed first producer | Complete immutable publication or retry; no poisoned entry/partial replacement. |
| Read-only explicit-marker predicate and first-word overflow | No predicate service/cache calls; preserve first-word explicit rescue without eager automatic discovery. |
| Seed extraction, mutation, foreign bindings and failure | Detached prefix/carry values, no partial seed; `NewTikray` rejects edited locked decisions; no prefix search, width query or service call. |
| Prefix atomic advance changes versus suffix-only change | Reject incompatible accepted prefix; allow correctly rebound suffix. Typography rejects changed prefix vertical/interior geometry too. |
| Discretionary cut with empty/nonempty post material | Exact carry/provenance, no original-word duplication or loss; no bidi reset. Gate unsupported successive splitting explicitly. |
| Initial KP prefix, first-fit continuation, A/B/C widths | Prefix decisions/geometry/policy retained; `PolicyForLine` reports KP for prefix lines and first-fit for suffix lines; zero prefix width queries and search. |
| Shared snapshot branch rejected | Original functional register collection and all other candidates unchanged. |
| GC with only frozen fragments alive | Khipu, Discovery, bindings/plans/checkpoints collectible; retained source/glyph services still render. |
| Compatible equal-width fitting | No new service/breaker work; no accidental discovery dependency. |

Also preserve current adapter and generic-dependency audits, CSS geometry,
source/paint/RTL/feature controls and replay-v5 evidence. Run ordinary/race/build/vet
in owning and integration modules during implementation. The design itself has
only source-inventory, syntax and link checks; no new runtime behavior is tested
or claimed. Upstream bidi repair, general page solving, vertical elasticity,
concurrent service enablement and generalized atomic fragmentation remain deferred.
