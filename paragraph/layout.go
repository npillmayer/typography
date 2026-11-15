package paragraph

import (
	"context"
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

// Constraints are the available inline space. Height-constrained packing is not
// supported. Width is nonnegative finite DU, not pixels or terminal columns.
type Constraints struct{ Width dimen.DU }

// Breaker selects the paragraph line-breaking algorithm; no zero default is implied.
type Breaker string

const (
	// FirstFit selects greedy first-fit line breaking.
	FirstFit   Breaker = "firstfit"
	// KnuthPlass selects Knuth-Plass paragraph optimization.
	KnuthPlass Breaker = "knuth-plass"
)

// Params separates parent constraints and breaking policy from paragraph
// assembly defaults. Per-run typography/layout stays in the prepared input and
// its resolver. LineLayout supplies CSS-inspired line-height separately from shaping. Vertical
// alignment for atomic boxes is supplied separately through LayoutRequest.
// Both breaker and assembly policy are explicit. CSSLineHeight requires LineLayout
// and explicit Streaming defaults/resolver; ProofCompatibility requires DefaultEmSize.
type Params struct {
	Constraints Constraints
	Breaker     Breaker
	Assembly    Options
	LineLayout  *LineLayout // Required only with CSSLineHeight; kept out of shaper styles.
}

// Stage identifies the operation that failed, independently of its underlying cause.
type Stage string

const (
	// StageInput identifies input/parameter validation and entry cancellation.
	StageInput     Stage = "input"
	// StageParagraph identifies bidi paragraph construction.
	StageParagraph Stage = "paragraph"
	// StageEncode identifies text-only Run encoding.
	StageEncode    Stage = "encode"
	// StagePrepare identifies reusable horizontal preparation.
	StagePrepare   Stage = "prepare"
	// StageMeasure identifies intrinsic evaluation after entry validation.
	StageMeasure   Stage = "measure"
	// StageBind identifies binding prepared material to atomic advances.
	StageBind      Stage = "bind"
	// StageBreak identifies line breaking and breakpoint extraction.
	StageBreak     Stage = "break"
	// StageFreeze identifies freezing selected lines.
	StageFreeze    Stage = "freeze"
	// StageAssemble identifies vertical assembly and atomic placement.
	StageAssemble  Stage = "assemble"
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
	VList                    *VList
	BreakPositions           []uint64
	BidiRuns                 []bidi.Segment
	EncodedKnots, BreakCount int
}

// Run prepares bidi levels, encodes, breaks, freezes, then assembles a boxed vlist.
// It accepts text-only input; nonempty Atomics fails at StageInput because Run
// has no object geometry. Use Prepare/Layout for mixed or reusable layout.
// It creates a fresh owner for every invocation and freezes before returning or
// running another breaker. No elastic paragraph/owner is published. Config uses
// khipu's existing service contracts (including hyphenation); a zero Config uses
// its synthetic backend. Non-nil Streaming is fully supplied by the caller and
// its factory must return fresh exclusive shapers. Config.Params governs encoder
// policy; line geometry is the selected breaker's rectangular default profile.
// For ProofCompatibility supply a matching Assembly.DefaultEmSize for metricless
// lines. CSSLineHeight instead uses Streaming.Defaults as its font/size strut.
//
// Services/styles must obey their immutable-lifetime contracts and remain usable
// for retained results. Run does not acquire fonts or rendering services. Errors
// return nil results and preserve cancellation/backend causes. Cancellation is
// checked at stage boundaries and within encoding/freezing/assembly; synchronous
// bidi preparation and breaking cannot be interrupted mid-call.
func Run(ctx context.Context, input Input, params Params, config khipu.Config) (*Result, error) {
	fail := func(stage Stage, err error) (*Result, error) { return nil, &Error{stage, err} }
	if ctx == nil {
		return fail(StageInput, fmt.Errorf("%w: nil context", ErrInvalidInput))
	}
	if err := ctx.Err(); err != nil {
		return fail(StageInput, err)
	}
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
	if err := ctx.Err(); err != nil {
		return fail(StageParagraph, err)
	}
	owner := khipu.NewKhipukamayuq(config)
	encoded, err := owner.EncodeParagraph(ctx, para)
	if err != nil {
		return fail(StageEncode, err)
	}
	return finishLayout(ctx, owner, encoded, params, layoutGeometry(params, nil), text, config, runs, nil)
}

// finishLayout owns fresh bound material and publishes only a complete result.
func finishLayout(ctx context.Context, owner *khipu.Khipukamayuq, encoded *khipu.Khipu, params Params, geometry khipu.Params, text styled.Text, config khipu.Config, runs []bidi.Segment, atomics map[contracts.AtomicID]AtomicBox) (*Result, error) {
	fail := func(stage Stage, err error) (*Result, error) { return nil, &Error{stage, err} }
	if err := ctx.Err(); err != nil {
		return fail(StageBreak, err)
	}
	var err error
	var breaks []int
	switch params.Breaker {
	case FirstFit:
		options := firstfit.NewDefaultParameters()
		options.DiscretionaryProvider = owner
		breaks, err = firstfit.BreakParagraph(encoded, geometry, options)
	case KnuthPlass:
		options := knuthplass.NewDefaultParameters()
		options.DiscretionaryProvider = owner
		breaks, err = knuthplass.BreakParagraph(encoded, geometry, options)
	}
	if err != nil {
		return fail(StageBreak, err)
	}
	if err := ctx.Err(); err != nil {
		return fail(StageBreak, err)
	}
	positions, err := encoded.BreakpointPositions(breaks)
	if err != nil {
		return fail(StageBreak, err)
	}
	frozen, err := owner.Freeze(ctx, encoded, breaks, geometry)
	if err != nil {
		return fail(StageFreeze, err)
	}
	if err := ctx.Err(); err != nil {
		return fail(StageFreeze, err)
	}
	var vlist *VList
	if params.Assembly.Policy == CSSLineHeight {
		vlist, err = assembleLineHeight(ctx, frozen, params.LineLayout, text, config, atomics)
	} else {
		vlist, err = assembleCompatibility(ctx, frozen, params.Assembly, atomics)
	}
	if err == nil {
		err = placeAtomics(ctx, vlist, atomics)
	}
	if err != nil {
		return fail(StageAssemble, err)
	}
	return &Result{Frozen: frozen, VList: vlist, BreakPositions: positions, BidiRuns: slices.Clone(runs), EncodedKnots: len(encoded.Kind), BreakCount: len(breaks)}, nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %w: %s", ErrInvalidInput, contracts.ErrIllegalArguments, message)
}

func validateParams(params Params, text styled.Text, config khipu.Config) (Params, error) {
	if params.Constraints.Width < 0 || params.Constraints.Width >= dimen.Fil || (params.Breaker != FirstFit && params.Breaker != KnuthPlass) {
		return params, invalidInput("width or breaker")
	}
	if params.Assembly.Policy == CSSLineHeight {
		if params.Assembly.DefaultEmSize != 0 {
			return params, invalidInput("CSS strut uses Streaming.Defaults; omit compatibility DefaultEmSize")
		}
		if err := validateLineLayout(params.LineLayout, text, config); err != nil {
			return params, fmt.Errorf("%w: %w: %w", ErrInvalidInput, contracts.ErrIllegalArguments, err)
		}
		params.LineLayout = params.LineLayout.Clone()
	} else {
		if params.LineLayout != nil {
			return params, invalidInput("line-height values require CSSLineHeight")
		}
		if err := validateOptions(params.Assembly); err != nil {
			return params, fmt.Errorf("%w: %w: %w", ErrInvalidInput, contracts.ErrIllegalArguments, err)
		}
	}
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
