package paragraph

import (
	"context"
	"errors"
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
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
// its callback and style values must stay stable until Run returns. No DOM types
// or source locators enter this envelope.
type Input struct {
	Text      styled.Text
	Direction bidi.Direction
	Markup    bidi.OutOfLineBidiMarkup
}

// Constraints are the available inline space. Height-constrained packing is not
// supported. Width is positive finite DU, not pixels or terminal columns.
type Constraints struct{ Width dimen.DU }

type Breaker string

const (
	FirstFit   Breaker = "firstfit"
	KnuthPlass Breaker = "knuth-plass"
)

// Params separates parent constraints and breaking policy from paragraph
// assembly defaults. Per-run typography/layout stays in the prepared input and
// its resolver. LineLayout supplies CSS-inspired line-height separately from shaping. Vertical
// alignment other than baseline is not implemented.
// Both breaker and assembly policy are explicit. CSSLineHeight requires LineLayout
// and explicit Streaming defaults/resolver; ProofCompatibility requires DefaultEmSize.
type Params struct {
	Constraints Constraints
	Breaker     Breaker
	Assembly    Options
	LineLayout  *LineLayout // Required only with CSSLineHeight; kept out of shaper styles.
}

type Stage string

const (
	StageInput     Stage = "input"
	StageParagraph Stage = "paragraph"
	StageEncode    Stage = "encode"
	StageBreak     Stage = "break"
	StageFreeze    Stage = "freeze"
	StageAssemble  Stage = "assemble"
)

var ErrInvalidInput = errors.New("invalid paragraph input")

type Error struct {
	Stage Stage
	Err   error
}

func (e *Error) Error() string { return fmt.Sprintf("paragraph %s: %v", e.Stage, e.Err) }
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
	if params.Constraints.Width <= 0 || params.Constraints.Width >= dimen.Fil || (params.Breaker != FirstFit && params.Breaker != KnuthPlass) || (input.Direction != bidi.LeftToRight && input.Direction != bidi.RightToLeft) {
		return fail(StageInput, fmt.Errorf("%w: direction, width or breaker", ErrInvalidInput))
	}
	if params.Assembly.Policy == CSSLineHeight {
		if params.Assembly.DefaultEmSize != 0 {
			return fail(StageInput, fmt.Errorf("%w: CSS strut uses Streaming.Defaults; omit compatibility DefaultEmSize", ErrInvalidInput))
		}
		if err := validateLineLayout(params.LineLayout, input.Text, config); err != nil {
			return fail(StageInput, fmt.Errorf("%w: %v", ErrInvalidInput, err))
		}
		params.LineLayout = params.LineLayout.Clone()
	} else {
		if params.LineLayout != nil {
			return fail(StageInput, fmt.Errorf("%w: line-height values require CSSLineHeight", ErrInvalidInput))
		}
		if err := validateOptions(params.Assembly); err != nil {
			return fail(StageInput, fmt.Errorf("%w: %v", ErrInvalidInput, err))
		}
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
	if err := ctx.Err(); err != nil {
		return fail(StageBreak, err)
	}
	shape := linebreak.RectangularParShape(params.Constraints.Width)
	var geometry khipu.Params
	var breaks []int
	switch params.Breaker {
	case FirstFit:
		geometry = firstfit.DefaultLayout(shape)
		options := firstfit.NewDefaultParameters()
		options.DiscretionaryProvider = owner
		breaks, err = firstfit.BreakParagraph(encoded, geometry, options)
	case KnuthPlass:
		geometry = knuthplass.DefaultLayout(shape)
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
		vlist, err = assembleLineHeight(ctx, frozen, params.LineLayout, text, config)
	} else {
		vlist, err = Assemble(ctx, frozen, params.Assembly)
	}
	if err != nil {
		return fail(StageAssemble, err)
	}
	return &Result{Frozen: frozen, VList: vlist, BreakPositions: positions, BidiRuns: runs, EncodedKnots: len(encoded.Kind), BreakCount: len(breaks)}, nil
}
