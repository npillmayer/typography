package paragraph

import (
	"errors"
	"fmt"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/typography/layout"
	"github.com/npillmayer/uax/bidi"
)

// Input is one already prepared paragraph. Text retains its exact styles and
// UTF-8 byte coordinates: Run neither normalizes whitespace nor projects styles
// or itemizes again. Use M's Itemize/ProjectStyles before this boundary, or supply
// equivalent typography-only styles with a matching resolver. Paint stays with
// the rendering consumer. Markup supplies zero-byte isolates in processed text;
// its callback is consumed by Run/Prepare and is not retained. Source styles and
// services must remain stable for prepared and frozen results. Atomics select
// U+FFFC occurrences; mixed input uses Prepare/Layout. No DOM types enter here.
type Input struct {
	Text      styled.Text
	Direction bidi.Direction
	Markup    bidi.OutOfLineBidiMarkup
	Atomics   []contracts.AtomicAnchor
}

// Constraints are the available inline space. Use Trial.FitPrefixes for finite-
// height prefix selection. Width is nonnegative finite DU, not pixels or terminal
// columns; target-height stretching/packing is not supported.
type Constraints struct{ Width dimen.DU }

// Breaker selects the paragraph line-breaking algorithm; no zero default is implied.
type Breaker string

const (
	// FirstFit selects greedy first-fit line breaking.
	FirstFit Breaker = "firstfit"
	// KnuthPlass selects Knuth-Plass paragraph optimization.
	KnuthPlass Breaker = "knuth-plass"
)

// Params separates parent constraints and breaking policy from paragraph
// assembly defaults. Per-run typography/layout stays in the prepared input and
// its resolver. LineLayout supplies CSS-inspired line-height separately from shaping. Vertical
// alignment for atomic boxes is supplied separately through LayoutRequest.
// Both breaker and assembly policy are explicit. CSSLineHeight requires LineLayout
// and explicit Streaming defaults/resolver.
type Params struct {
	Constraints Constraints
	Breaker     Breaker
	Assembly    Options
	LineLayout  *LineLayout // Required only with CSSLineHeight; kept out of shaper styles.
}

// Stage identifies the operation that failed, independently of its underlying cause.
type Stage string

const (
	// StageInput identifies input/parameter validation.
	StageInput Stage = "input"
	// StageParagraph identifies bidi paragraph construction.
	StageParagraph Stage = "paragraph"
	// StageEncode identifies text-only Run encoding.
	StageEncode Stage = "encode"
	// StagePrepare identifies reusable horizontal preparation.
	StagePrepare Stage = "prepare"
	// StageMeasure identifies intrinsic evaluation after entry validation.
	StageMeasure Stage = "measure"
	// StageBind identifies binding prepared material to atomic advances.
	StageBind Stage = "bind"
	// StageBreak identifies line breaking and breakpoint extraction.
	StageBreak Stage = "break"
	// StageFreeze identifies freezing selected lines.
	StageFreeze Stage = "freeze"
	// StageAssemble identifies vertical assembly and atomic placement.
	StageAssemble Stage = "assemble"
)

// ErrInvalidInput marks invalid paragraph input or parameter values.
// Use errors.Is through Error to test it.
var ErrInvalidInput = errors.New("invalid paragraph input")

// Error records the failed paragraph stage and preserves the original cause.
// Failed operations publish no partial result.
type Error struct {
	Stage Stage
	Err   error
}

// Error formats the paragraph stage and cause.
func (e *Error) Error() string { return fmt.Sprintf("paragraph %s: %v", e.Stage, e.Err) }

// Unwrap exposes the cause for errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Err }

// Result retains frozen content and its boxed vlist. Counts, break endpoints and
// bidi runs supply diagnostics without prescribing application display strings.
// All ranges are paragraph-relative UTF-8 bytes; glyph clusters remain scalars.
// Frozen and VList share only immutable content/font services. Public slices are
// detached from the engine; changing them cannot alter frozen output/placements.
type Result struct {
	Frozen                   *khipu.FrozenKhipu
	VList                    *layout.VList
	BreakPositions           []uint64
	BidiRuns                 []bidi.Segment
	EncodedKnots, BreakCount int
}

// Run prepares bidi levels, encodes, breaks, freezes, then assembles a boxed vlist.
// It accepts text-only input; nonempty Atomics fails at StageInput because Run
// has no object geometry. Use Prepare/Layout for mixed or reusable layout.
// It creates fresh immutable material and an independent trial, then freezes
// before returning. No material, discovery or trial handle is published. Config uses
// khipu's existing service contracts (including hyphenation). CSS assembly
// requires explicit streaming font/size/resolver defaults. Streaming is supplied
// by the caller; its factory must return fresh exclusive shapers. Config.Params governs encoder
// policy; line geometry is the selected breaker's rectangular default profile.
// CSSLineHeight uses Streaming.Defaults as its font/size strut.
//
// Services/styles must obey their immutable-lifetime contracts and remain usable
// for retained results. Run does not acquire fonts or rendering services. Errors
// return nil results and preserve backend causes. Execution is synchronous;
// streaming coordination cancels and joins producers internally on failure.
func Run(input Input, params Params, config khipu.Config) (*Result, error) {
	fail := func(stage Stage, err error) (*Result, error) { return nil, &Error{stage, err} }

	if len(input.Atomics) != 0 || (input.Direction != bidi.LeftToRight && input.Direction != bidi.RightToLeft) {
		return fail(StageInput, invalidInput("Run requires text-only input and a valid direction"))
	}
	var err error
	params, err = validateParams(params, input.Text, config)
	if err != nil {
		return fail(StageInput, err)
	}
	// Copy the persistent text handle; never rebuild its projected/private styles.
	text := input.Text
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), input.Direction, input.Markup)
	if err != nil {
		return fail(StageParagraph, err)
	}
	var runs []bidi.Segment
	for run := range para.BidiLevels().LogicalSegments() {
		runs = append(runs, run)
	}

	owner := khipu.NewKhipukamayuq(config)
	encoded, discovery, err := owner.EncodeParagraph(para)
	if err != nil {
		return fail(StageEncode, err)
	}
	bound, err := khipu.BindParagraph(encoded, nil)
	if err != nil {
		return fail(StageBind, err)
	}
	return finishLayout(bound, discovery, params, layoutGeometry(params, nil), text, config, runs, nil)
}

// finishLayout creates an independent trial and publishes only a complete result.
func finishLayout(bound *khipu.AtomicBinding, discovery *khipu.Discovery, params Params, geometry khipu.Params, text styled.Text, config khipu.Config, runs []bidi.Segment, atomics map[contracts.AtomicID]AtomicBox) (*Result, error) {
	plan, err := breakLayout(breakRequest(bound, discovery, geometry), params.Breaker)
	if err != nil {
		return nil, err
	}
	return finishWithPlan(plan, bound.KnotCount(), params, text, config, runs, atomics)
}

func breakRequest(bound *khipu.AtomicBinding, discovery *khipu.Discovery, geometry khipu.Params) khipu.BreakRequest {
	return khipu.BreakRequest{Binding: bound, Discovery: discovery, Geometry: khipu.GeometryRequest{
		Shape: geometry.LineShape, LeftSkip: geometry.LeftSkip, RightSkip: geometry.RightSkip,
	}}
}

func breakLayout(request khipu.BreakRequest, breaker Breaker) (*khipu.Tikray, error) {
	return searchTrial(request, breaker, nil, nil)
}

// finishWithPlan is shared by ordinary layout and checkpoint continuation.
// Freezing consumes only the trial's captured geometry and selected alternatives.
func finishWithPlan(plan *khipu.Tikray, encodedKnots int, params Params, text styled.Text, config khipu.Config, runs []bidi.Segment, atomics map[contracts.AtomicID]AtomicBox) (*Result, error) {
	fail := func(stage Stage, err error) (*Result, error) { return nil, &Error{stage, err} }
	positions, err := plan.BreakpointPositions()
	if err != nil {
		return fail(StageBreak, err)
	}
	frozen, err := khipu.Freeze(plan)
	if err != nil {
		return fail(StageFreeze, err)
	}
	vlist, err := assembleLineHeight(frozen, params.LineLayout, text, config, atomics)
	if err == nil {
		err = placeAtomics(vlist, atomics)
	}
	if err != nil {
		return fail(StageAssemble, err)
	}
	published, err := vlist.publish()
	if err != nil {
		return fail(StageAssemble, err)
	}
	return &Result{Frozen: frozen, VList: published, BreakPositions: positions, BidiRuns: slices.Clone(runs), EncodedKnots: encodedKnots, BreakCount: len(positions)}, nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %w: %s", ErrInvalidInput, contracts.ErrIllegalArguments, message)
}

func validateParams(params Params, text styled.Text, config khipu.Config) (Params, error) {
	if params.Constraints.Width < 0 || params.Constraints.Width >= dimen.Fil || (params.Breaker != FirstFit && params.Breaker != KnuthPlass) {
		return params, invalidInput("width or breaker")
	}
	if params.Assembly.Policy != CSSLineHeight {
		return params, invalidInput("assembly requires CSSLineHeight")
	}
	if err := validateLineLayout(params.LineLayout, text, config); err != nil {
		return params, fmt.Errorf("%w: %w: %w", ErrInvalidInput, contracts.ErrIllegalArguments, err)
	}
	params.LineLayout = params.LineLayout.Clone()
	return params, nil
}

func layoutGeometry(params Params, horizontal *HorizontalLayout) khipu.Params {
	shape := linebreak.RectangularParShape(params.Constraints.Width)
	geometry := firstfit.DefaultLayout(shape)
	if params.Breaker == KnuthPlass {
		geometry = knuthplass.DefaultLayout(shape)
	}
	if horizontal != nil {
		geometry.LeftSkip, geometry.RightSkip = horizontal.LeftSkip, horizontal.RightSkip
	}
	return geometry
}
