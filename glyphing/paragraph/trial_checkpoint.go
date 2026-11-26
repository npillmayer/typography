package paragraph

import (
	"slices"

	"github.com/npillmayer/khipu"
)

// TrialCheckpoint certifies complete prefix decisions and matching paragraph
// assembly inputs. It retains the material incarnation, copied prefix decisions
// and consumed atomic geometry, never the originating Trial/Prepared/Discovery
// or a checkpoint chain. It is immutable and reusable for independent suffix
// trials. NoHyphenation and ExplicitHyphenation are supported.
// A zero value is invalid.
type TrialCheckpoint struct {
	prefix        *khipu.BreakCheckpoint
	lineLayout    *LineLayout
	atomics       []AtomicBox
	acceptedLines int
}

// Checkpoint locks zero through all lines of this trial, without assembly or
// service calls. Invalid trials/counts fail at StageInput; unsupported hyphenation
// preserves khipu.ErrUnsupportedPolicy. A nonempty prefix conservatively locks
// the entire LineLayout, even overrides applying only to suffix text. Atomic
// boxes are locked only for consumed occurrences, including all margins and
// vertical dimensions. Application-owned atomic interiors must also remain
// consistent; this package cannot compare or certify their rendering identity.
func (t *Trial) Checkpoint(acceptedLines int) (*TrialCheckpoint, error) {
	prefix, err := khipu.Checkpoint(t.Plan(), acceptedLines)
	if err != nil {
		return nil, &Error{StageInput, err}
	}
	c := &TrialCheckpoint{prefix: prefix, acceptedLines: acceptedLines}
	if acceptedLines > 0 {
		c.lineLayout = t.inputs.Params.LineLayout.Clone()
		end := t.plan.Lines()[acceptedLines-1].End.Knot
		for _, a := range t.atomicKnots {
			if a.knot <= end {
				c.atomics = append(c.atomics, a.box)
			}
		}
	}
	return c, nil
}

func (c *TrialCheckpoint) validate(b *boundLayout) error {
	if c.prefix == nil {
		return invalidInput("uninitialized trial checkpoint")
	}
	if _, err := c.prefix.SeedFor(b.bound); err != nil {
		return err
	}
	if c.lineLayout != nil && (c.lineLayout.Default != b.params.LineLayout.Default ||
		!slices.Equal(c.lineLayout.Runs, b.params.LineLayout.Runs)) {
		return invalidInput("checkpoint line-height changed")
	}
	for _, box := range c.atomics {
		if b.boxes[box.ID] != box {
			return invalidInput("checkpoint atomic box changed")
		}
	}
	return nil
}
