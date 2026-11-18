package paragraph

import (
	"context"
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/layout"
)

// Frame supplies resolved content dimensions only: Width in DU and Height in
// fractional big points. Both are nonnegative and strictly below dimen.Fil.
// Frame styles, origins and surrounding margins belong to the application.
type Frame struct {
	Width  dimen.DU
	Height float64
}

// Continuation is an immutable, preparation-bound unfinished paragraph. Only
// StartFlow and Fit initialize it; its zero value is invalid. Operations sharing
// preparation/services must be serialized. Styles/fonts/services remain borrowed
// and stable; changing them requires a fresh preparation and flow. Layout values
// and request slices are copied. Older tokens remain reusable after success or
// failure, so callers must drop them too when completing or abandoning a paragraph.
// No continuation, checkpoint or service is serializable.
type Continuation struct {
	state     *flowState
	remainder *layout.Fragment
	path      []int
	history   []dimen.DU // target widths of every accepted logical line
	width     dimen.DU
	breaker   Breaker // policy that produced the retained suffix
}

type flowState struct {
	prepared *Prepared
	params   Params
	geometry khipu.Params
	boxes    map[contracts.AtomicID]AtomicBox
	advances []khipu.AtomicAdvance
}

// FlowResult publishes one frame's immutable placed fragment and an explicit
// frozen remainder. Continuation is nonnil only when content remains. Retain each
// Placed fragment independently: successive frames may use different frozen owners.
// Mutating this envelope or Overfull does not mutate any token or earlier result.
// Overfull indexes refer to this call's frozen owner and include the remainder.
// Breaker describes the suffix policy; Reused means no binding/breaking/assembly
// occurred. A changed-width suffix always uses FirstFit, even after a KP prefix.
type FlowResult struct {
	Placed, Remainder *layout.Fragment
	Continuation      *Continuation
	Status            FragmentFitStatus
	Overfull          []int
	Breaker           Breaker
	Reused            bool
}

// StartFlow lays out a NoHyphenation preparation once at the requested initial
// width. All other layout inputs are fixed for this flow. It shares ordinary
// layout's validation, breaking, freezing and CSS assembly. No base text is
// reshaped. Use Fit for each frame; release this initial token and all intermediate
// tokens/preparation when the paragraph fully flows. Completed frozen fragments
// need no working handle. There is no cross-paragraph cache or Close operation.
func (p *Prepared) StartFlow(ctx context.Context, request LayoutRequest) (*Continuation, error) {
	if ctx == nil {
		return nil, &Error{StageInput, invalidInput("nil context")}
	}
	if err := ctx.Err(); err != nil {
		return nil, &Error{StageInput, err}
	}
	if p == nil || p.owner == nil || p.content == nil {
		return nil, &Error{StageInput, invalidInput("uninitialized preparation")}
	}
	if p.hyphenation != khipu.NoHyphenation {
		return nil, &Error{StageInput, khipu.ErrUnsupportedPolicy}
	}
	b, err := p.bindLayout(ctx, request)
	if err != nil {
		return nil, err
	}
	path, err := breakLayout(ctx, p.owner, b.bound, b.params, b.geometry)
	if err != nil {
		return nil, err
	}
	result, err := finishWithBreaks(ctx, p.owner, b.bound, b.params, b.geometry, p.text, p.config, p.runs, b.boxes, path)
	if err != nil {
		return nil, err
	}
	f, err := Fragment(ctx, result.VList)
	if err != nil {
		return nil, &Error{StageAssemble, err}
	}
	// Validate even suppressed-empty owners and derived local geometry at entry.
	if _, err = fitFlowFragment(ctx, f, nil, b.params.Constraints.Width, 0); err != nil {
		return nil, &Error{StageAssemble, err}
	}
	return &Continuation{state: &flowState{p, b.params, b.geometry, b.boxes, b.advances}, remainder: f, path: slices.Clone(path), width: b.params.Constraints.Width, breaker: b.params.Breaker}, nil
}

// Remainder returns the immutable retained tail, or nil for an uninitialized token.
// Holding this fragment alone does not keep a preparation/checkpoint alive.
func (c *Continuation) Remainder() *layout.Fragment {
	if c == nil || c.state == nil {
		return nil
	}
	return c.remainder
}

// Fit places the longest whole-line prefix into a frame. Equal-width requests
// use only retained geometry, including after heterogeneous A/B width history.
// A width change binds afresh, locks all accepted boundaries with a khipu
// checkpoint, searches only the suffix, then freezes/assembles the complete path.
// The original paragraph styles and bidi context remain in use. Every boundary
// is eligible; no widows/orphans, target-height packing or rendering is implied.
// Errors/cancellation return nil with a stage and cause, preserving older tokens
// and fragments. No-progress is a successful explicit remainder, not an error.
func (c *Continuation) Fit(ctx context.Context, frame Frame) (*FlowResult, error) {
	if ctx == nil {
		return nil, &Error{StageInput, invalidInput("nil context")}
	}
	if err := ctx.Err(); err != nil {
		return nil, &Error{StageInput, err}
	}
	if c == nil || c.state == nil || c.remainder == nil || frame.Width < 0 || frame.Width >= dimen.Fil || !fitBounded(frame.Height) || frame.Height < 0 {
		return nil, &Error{StageInput, invalidInput("continuation or frame")}
	}
	current := c
	reused := true
	if frame.Width != c.width && c.remainder.Range().Start != c.remainder.Range().End {
		var err error
		current, err = c.reflow(ctx, frame.Width)
		if err != nil {
			return nil, err
		}
		reused = false
	}
	fit, err := fitFlowFragment(ctx, current.remainder, current.history, current.width, frame.Height)
	if err != nil {
		return nil, &Error{StageAssemble, err}
	}
	out := &FlowResult{Placed: fit.Placed, Remainder: fit.Remainder, Status: fit.Status, Overfull: fit.Overfull, Breaker: current.breaker, Reused: reused}
	if fit.Status != FitComplete {
		history := slices.Clone(current.history)
		for _, line := range fit.Placed.Lines() {
			if err := ctx.Err(); err != nil {
				return nil, &Error{StageAssemble, err}
			}
			history = append(history, line.Horizontal.TargetWidth)
		}
		out.Continuation = &Continuation{state: current.state, remainder: fit.Remainder, path: current.path, history: history, width: current.width, breaker: current.breaker}
	}
	if err := ctx.Err(); err != nil {
		return nil, &Error{StageAssemble, err}
	}
	return out, nil
}
