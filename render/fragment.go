package render

import (
	"context"
	"fmt"

	"github.com/npillmayer/typography/layout"
	typestyle "github.com/npillmayer/typography/style"
)

// FragmentInput borrows an immutable positioned line range. Text and Paint
// address the complete original processed UTF-8 paragraph, not a sliced suffix.
// Keep these values and the rendering services stable during proof construction.
type FragmentInput struct {
	Fragment *layout.Fragment
	Text     string
	Paint    []typestyle.PaintSpan
}

// BuildFragmentProof renders only the fragment's published lines. It looks up
// horizontal content by OriginalIndex and uses retained fragment-local vertical
// geometry, without restacking, metrics, shaping, binding or breaking. Glyph.Line
// is local to the returned proof; knot indexes and source anchors stay original.
// VisualOrder runs once per detached directional glyph fragment. Atomic drawing
// and frame transforms remain application-owned; use Fragment.AtomicPlacements.
// Errors/cancellation return no partial proof. No working preparation is retained.
func BuildFragmentProof(ctx context.Context, input FragmentInput, services Services) (*ParagraphProof, error) {
	f := input.Fragment
	if ctx == nil || f == nil || f.Frozen() == nil || services.Backend == nil || services.Identity == nil {
		return nil, fmt.Errorf("fragment proof requires retained placements and rendering services")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lines := f.Lines()
	placements := make([]layout.Line, len(lines))
	for i, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		horizontal, ok := f.Frozen().Line(line.OriginalIndex)
		if !ok {
			return nil, fmt.Errorf("fragment proof has no original line %d", line.OriginalIndex)
		}
		placements[i] = line.Line
		placements[i].Horizontal = horizontal
	}
	return buildRetainedProof(ctx, ParagraphInput{Frozen: f.Frozen(), Text: input.Text, Paint: input.Paint}, services, placements)
}
