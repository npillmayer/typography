package paragraph

import (
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
)

// A stable schedule includes every accepted A/B/... line, not just the initial
// width. The backing history is private immutable storage for this invocation.
type continuationWidths struct {
	history []dimen.DU
	tail    dimen.DU
}

func (w continuationWidths) LineLength(line int32) dimen.DU {
	if int(line) <= len(w.history) {
		return w.history[line-1]
	}
	return w.tail
}

func (c *Continuation) reflow(width dimen.DU) (*Continuation, error) {
	s := c.state
	p := s.prepared
	checkpoint, err := p.owner.Checkpoint(p.content, c.path, len(c.history))
	if err != nil {
		return nil, &Error{StageBreak, err}
	}
	bound, err := p.owner.BindParagraph(p.content, s.advances)
	if err != nil {
		return nil, &Error{StageBind, err}
	}
	geometry := s.geometry
	geometry.LineShape = continuationWidths{c.history, width}
	path, err := firstfit.ContinueParagraph(bound, geometry, checkpoint)
	if err != nil {
		return nil, &Error{StageBreak, err}
	}
	params := s.params
	params.Constraints.Width = width
	params.Breaker = FirstFit
	result, err := finishWithBreaks(p.owner, bound, params, geometry, p.text, p.config, p.runs, s.boxes, path)
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
	return &Continuation{state: s, remainder: tail, path: path, history: c.history, width: width, breaker: FirstFit}, nil
}
