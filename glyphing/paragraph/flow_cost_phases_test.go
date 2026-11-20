package paragraph

// Test-only component measurements use the same production primitives as reflow.
// They do not instrument or replace production calls, and are not public API.
import (
	"time"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/typography/layout"
)

type P6PhaseSample struct {
	Checkpoint, Bind, Break, Freeze, Assembly            time.Duration
	BoundKnots, FrozenKnots, AssembledLines, PrefixKnots int
	BreakWidthQueries, PrefixWidthQueries                int
}

type p6WidthCounter struct {
	history                []dimen.DU
	width                  dimen.DU
	queries, prefixQueries int
}

func (w *p6WidthCounter) LineLength(line int32) dimen.DU {
	w.queries++
	if int(line) <= len(w.history) {
		w.prefixQueries++
	}
	return w.width
}

func P6Checkpoint(c *Continuation) (any, error) {
	return khipu.Checkpoint(c.plan, len(c.history))
}

// P6Phases is a separately measured decomposition, not timings inside Fit.
// Cardinalities describe actual input/output material, not loop-visit counters.
func P6Phases(c *Continuation, width dimen.DU) (*layout.Fragment, P6PhaseSample, error) {
	var m P6PhaseSample
	s, p := c.state, c.state.prepared
	start := time.Now()
	cp, err := khipu.Checkpoint(c.plan, len(c.history))
	m.Checkpoint = time.Since(start)
	if err != nil {
		return nil, m, err
	}
	start = time.Now()
	bound, err := khipu.BindParagraph(p.content, s.advances)
	m.Bind = time.Since(start)
	if err != nil {
		return nil, m, err
	}
	m.BoundKnots = bound.KnotCount()
	if len(c.history) > 0 {
		m.PrefixKnots = c.plan.Lines()[len(c.history)-1].End.Knot + 1
	}
	geometry := s.geometry
	shape := &p6WidthCounter{history: c.history, width: width}
	geometry.LineShape = shape
	request := breakRequest(bound, p.discovery, geometry)
	request.Prefix = cp
	start = time.Now()
	plan, err := firstfit.ContinueParagraph(request, nil)
	m.Break = time.Since(start)
	m.BreakWidthQueries, m.PrefixWidthQueries = shape.queries, shape.prefixQueries
	if err != nil {
		return nil, m, err
	}
	start = time.Now()
	frozen, err := khipu.Freeze(plan)
	m.Freeze = time.Since(start)
	if err != nil {
		return nil, m, err
	}
	m.FrozenKnots = frozen.KnotCount()
	start = time.Now()
	v, err := assembleLineHeight(frozen, s.params.LineLayout, p.text, p.config, s.boxes)
	if err == nil {
		err = placeAtomics(v, s.boxes)
	}
	if err != nil {
		return nil, m, err
	}
	published, err := v.publish()
	m.Assembly = time.Since(start)
	if err != nil {
		return nil, m, err
	}
	m.AssembledLines = len(published.Lines())
	whole, err := Fragment(published)
	if err != nil {
		return nil, m, err
	}
	if len(c.history) > 0 {
		_, whole, err = SplitFragment(whole, len(c.history))
	}
	return whole, m, err
}
