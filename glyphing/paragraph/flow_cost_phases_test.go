package paragraph

// Test-only component measurements use the same production primitives as reflow.
// They do not instrument or replace production calls, and are not public API.
import (
	"time"

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
	continuationWidths
	queries, prefixQueries int
}

func (w *p6WidthCounter) LineLength(line int32) dimen.DU {
	w.queries++
	if int(line) <= len(w.history) {
		w.prefixQueries++
	}
	return w.continuationWidths.LineLength(line)
}

func P6Checkpoint(c *Continuation) (any, error) {
	return c.state.prepared.owner.Checkpoint(c.state.prepared.content, c.path, len(c.history))
}

// P6Phases is a separately measured decomposition, not timings inside Fit.
// Cardinalities describe actual input/output material, not loop-visit counters.
func P6Phases(c *Continuation, width dimen.DU) (*layout.Fragment, P6PhaseSample, error) {
	var m P6PhaseSample
	s, p := c.state, c.state.prepared
	start := time.Now()
	cp, err := p.owner.Checkpoint(p.content, c.path, len(c.history))
	m.Checkpoint = time.Since(start)
	if err != nil {
		return nil, m, err
	}
	start = time.Now()
	bound, err := p.owner.BindParagraph(p.content, s.advances)
	m.Bind = time.Since(start)
	if err != nil {
		return nil, m, err
	}
	m.BoundKnots = len(bound.Kind)
	if len(c.history) > 0 {
		m.PrefixKnots = c.path[len(c.history)-1] + 1
	}
	geometry := s.geometry
	shape := &p6WidthCounter{continuationWidths: continuationWidths{c.history, width}}
	geometry.LineShape = shape
	start = time.Now()
	path, err := firstfit.ContinueParagraph(bound, geometry, cp)
	m.Break = time.Since(start)
	m.BreakWidthQueries, m.PrefixWidthQueries = shape.queries, shape.prefixQueries
	geometry.LineShape = shape.continuationWidths
	if err != nil {
		return nil, m, err
	}
	start = time.Now()
	frozen, err := p.owner.Freeze(bound, path, geometry)
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
