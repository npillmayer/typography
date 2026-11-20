package paragraph

import (
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

// TrialRequest describes an independent paragraph search. Layout supplies copied
// assembly inputs and atomic realizations. Shape, when nonnil, replaces the
// rectangular width; it is consumed during search and never retained. Its line
// numbers are one-based, including any locked prefix. Captured Plan geometry is
// authoritative afterwards. Layout.Params.Constraints.Width must still be valid.
//
// Nil algorithm parameters select defaults. Only the arm matching Params.Breaker
// may be supplied. Prefix optionally locks complete lines from this preparation;
// continuation supports only NoHyphenation and a first-fit suffix. Suffix widths,
// skips, first-fit settings and unconsumed atomic boxes may change. A nonempty
// prefix requires identical LineLayout values and consumed atomic boxes.
type TrialRequest struct {
	Layout     LayoutRequest
	Shape      khipu.LineShape
	FirstFit   *firstfit.Parameters
	KnuthPlass *knuthplass.Parameters
	Prefix     *TrialCheckpoint
}

// Trial pairs immutable breaking decisions with their copied CSS assembly
// inputs. It retains source/style/font services, but not Prepared, Discovery,
// shape callbacks or search workspaces. Assemble remains usable after any later
// trial succeeds or fails. Serialize operations using shared borrowed services;
// this API does not promise concurrent execution. Drop unwanted trials explicitly
// by releasing their references; no rollback operation mutates shared material.
type Trial struct {
	plan         *khipu.Tikray
	inputs       LayoutRequest
	text         styled.Text
	assembly     khipu.Config
	runs         []bidi.Segment
	boxes        map[contracts.AtomicID]AtomicBox
	atomicKnots  []trialAtomic
	encodedKnots int
}

type trialAtomic struct {
	knot int
	box  AtomicBox
}

// Trial searches fresh bound material without freezing or CSS assembly. Base
// text is not reshaped; lazy replacement discoveries are shared and retryable.
// Failure publishes no partial trial and cannot invalidate earlier candidates.
// This is paragraph orchestration, not frame scheduling or page commit/rollback.
func (p *Prepared) Trial(request TrialRequest) (*Trial, error) {
	b, err := p.bindLayout(request.Layout)
	if err != nil {
		return nil, err
	}
	if (b.params.Breaker != FirstFit && request.FirstFit != nil) ||
		(b.params.Breaker != KnuthPlass && request.KnuthPlass != nil) {
		return nil, &Error{StageInput, invalidInput("inactive breaker parameters")}
	}
	r := breakRequest(b.bound, p.discovery, b.geometry)
	if request.Shape != nil {
		r.Geometry.Shape = request.Shape
	}
	if request.Prefix != nil {
		if err := request.Prefix.validate(b); err != nil {
			return nil, &Error{StageBind, err}
		}
		r.Prefix = request.Prefix.prefix
	}
	plan, err := searchTrial(r, b.params.Breaker, request.FirstFit, request.KnuthPlass)
	if err != nil {
		return nil, err
	}
	// Assembly needs only the stable resolver and defaults. In particular do not
	// keep the preparation's shaper factory, hyphenator or discovery through config.
	streaming := &khipu.StreamingConfig{Resolver: p.config.Streaming.Resolver, Defaults: p.config.Streaming.Defaults}
	streaming.Defaults.Features = slices.Clone(streaming.Defaults.Features)
	t := &Trial{plan: plan, text: p.text, assembly: khipu.Config{Streaming: streaming},
		runs: slices.Clone(p.runs), boxes: b.boxes, encodedKnots: b.bound.KnotCount(),
		inputs: LayoutRequest{Params: b.params, Horizontal: &HorizontalLayout{b.geometry.LeftSkip, b.geometry.RightSkip}}}
	if len(b.boxes) != 0 {
		for i := 0; i < p.content.KnotCount(); i++ {
			knot, _ := p.content.Knot(i)
			if knot.Atomic != 0 {
				box := b.boxes[knot.Atomic]
				t.atomicKnots = append(t.atomicKnots, trialAtomic{i, box})
				t.inputs.Atomics = append(t.inputs.Atomics, box)
			}
		}
	}
	return t, nil
}

func searchTrial(r khipu.BreakRequest, breaker Breaker, ff *firstfit.Parameters, kp *knuthplass.Parameters) (*khipu.Tikray, error) {
	var plan *khipu.Tikray
	var err error
	switch breaker {
	case FirstFit:
		if r.Prefix == nil {
			plan, err = firstfit.BreakParagraph(r, ff)
		} else {
			plan, err = firstfit.ContinueParagraph(r, ff)
		}
	case KnuthPlass:
		plan, err = knuthplass.BreakParagraph(r, kp)
	}
	if err != nil {
		return nil, &Error{StageBreak, err}
	}
	return plan, nil
}

// Plan returns immutable decisions, including captured geometry and per-line
// policy. It returns nil for a nil or uninitialized Trial. Retaining the returned
// Tikray alone permits khipu.Freeze, but does not retain CSS assembly inputs.
func (t *Trial) Plan() *khipu.Tikray {
	if t == nil {
		return nil
	}
	return t.plan
}

// Inputs returns detached assembly/layout inputs with explicit resolved side
// skips and atomics in material order, or zero for a nil/uninitialized Trial.
// For custom shapes or continuation, Plan's per-line widths/skips take precedence
// over this request's suffix geometry. Algorithm settings are recorded in Plan.
func (t *Trial) Inputs() LayoutRequest {
	if t.Plan() == nil {
		return LayoutRequest{}
	}
	r := t.inputs
	r.Params.LineLayout = r.Params.LineLayout.Clone()
	h := *r.Horizontal
	r.Horizontal = &h
	r.Atomics = slices.Clone(r.Atomics)
	return r
}

// Assemble freezes the captured plan and assembles the full paragraph. Repeated
// calls produce independent results without searching or shaping. Stable borrowed
// metrics/resolver services remain necessary. Errors publish no partial result;
// nil/uninitialized trials fail at StageInput. Results do not retain the Trial.
func (t *Trial) Assemble() (*Result, error) {
	if t.Plan() == nil {
		return nil, &Error{StageInput, invalidInput("nil or uninitialized trial")}
	}
	return finishWithPlan(t.plan, t.encodedKnots, t.inputs.Params, t.text, t.assembly, t.runs, t.boxes)
}
