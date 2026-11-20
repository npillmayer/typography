package paragraph

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
)

func (c *Continuation) reflow(width dimen.DU) (*Continuation, error) {
	s := c.state
	p := s.prepared
	checkpoint, err := khipu.Checkpoint(c.plan, len(c.history))
	if err != nil {
		return nil, &Error{StageBreak, err}
	}
	bound, err := khipu.BindParagraph(p.content, s.advances)
	if err != nil {
		return nil, &Error{StageBind, err}
	}
	geometry := s.geometry
	// The checkpoint supplies all accepted widths/skips; only the suffix
	// queries this new frame width. No callback retains prior flow tokens.
	geometry.LineShape = linebreak.RectangularParShape(width)
	request := breakRequest(bound, p.discovery, geometry)
	request.Prefix = checkpoint
	plan, err := firstfit.ContinueParagraph(request, nil)
	if err != nil {
		return nil, &Error{StageBreak, err}
	}
	params := s.params
	params.Constraints.Width = width
	params.Breaker = FirstFit
	result, err := finishWithPlan(plan, bound.KnotCount(), params, p.text, p.config, p.runs, s.boxes)
	if err != nil {
		return nil, err
	}
	whole, err := Fragment(result.VList)
	if err != nil {
		return nil, &Error{StageAssemble, err}
	}
	if err = validateFitLocal(whole); err != nil {
		return nil, &Error{StageAssemble, err}
	}
	tail := whole
	if len(c.history) > 0 {
		_, tail, err = SplitFragment(whole, len(c.history))
		if err != nil {
			return nil, &Error{StageAssemble, err}
		}
	}
	return &Continuation{state: s, remainder: tail, plan: plan, history: c.history, width: width, breaker: FirstFit}, nil
}
