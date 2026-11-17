package paragraph

import (
	"context"
	"fmt"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/uax/bidi"
)

// Prepared retains shaped horizontal material and resolved bidi data. Serialize
// all operations on one handle; it has no concurrent-layout or transfer promise.
// Source/style/service objects remain borrowed under their stable-lifetime
// contracts. Request/configuration slices are copied; markup is not retained.
// Measurement and layout failures cannot change earlier results or invalidate
// this preparation. Measurement does not require final-layout geometry. Explicit
// replacement batches are memoized under the same serialized ownership contract
// and imported lazily into each fresh layout binding.
type Prepared struct {
	owner   *khipu.Khipukamayuq
	content *khipu.PreparedParagraph
	text    styled.Text
	config  khipu.Config
	runs    []bidi.Segment
	anchors []contracts.AtomicAnchor
}

// Prepare resolves bidi once and shapes text around atomic occurrences without
// requiring object geometry or a target width. It neither itemizes nor validates
// glyph coverage. The returned private styles retain their original identities.
// Zero config uses khipu's synthetic backend; CSSLineHeight later requires explicit
// streaming defaults. Hyphenation policy is fixed by options for every layout.
func Prepare(ctx context.Context, input Input, options khipu.PrepareOptions, config khipu.Config) (*Prepared, error) {
	fail := func(stage Stage, err error) (*Prepared, error) { return nil, &Error{stage, err} }
	if ctx == nil {
		return fail(StageInput, invalidInput("nil context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(StageInput, err)
	}
	if input.Direction != bidi.LeftToRight && input.Direction != bidi.RightToLeft {
		return fail(StageInput, invalidInput("direction"))
	}
	p := &Prepared{text: input.Text, config: cloneConfig(config), anchors: slices.Clone(input.Atomics)}
	para, err := styled.ParagraphFromText(&p.text, 0, p.text.Raw().Len(), input.Direction, input.Markup)
	if err != nil {
		return fail(StageParagraph, err)
	}
	for run := range para.BidiLevels().LogicalSegments() {
		p.runs = append(p.runs, run)
	}
	if err := ctx.Err(); err != nil {
		return fail(StageParagraph, err)
	}
	p.owner = khipu.NewKhipukamayuq(p.config)
	options.AtomicBoundaries = slices.Clone(options.AtomicBoundaries)
	p.content, err = p.owner.PrepareParagraph(ctx, khipu.ParagraphInput{Paragraph: para, Atomics: p.anchors}, options)
	if err != nil {
		return fail(StagePrepare, err)
	}
	return p, nil
}

// HorizontalLayout overrides both side skips completely, including explicit
// zeros. Only W/MinW/MaxW matter. Width remains Params.Constraints.Width.
type HorizontalLayout struct {
	LeftSkip, RightSkip khipu.KnotCore
}

// LayoutRequest supplies one final border box per occurrence, in any ID order.
// All primitive dimensions use DU. It is consumed without retaining its slices.
// Nil Horizontal preserves the chosen breaker's historical skip profile.
type LayoutRequest struct {
	Params     Params
	Horizontal *HorizontalLayout
	Atomics    []AtomicBox
}

// Layout validates geometry, derives advances, binds fresh working material,
// breaks, freezes and assembles a detached result. It does not reshape the base
// paragraph. Lazy discretionary shaping remains owner-controlled. Cancellation
// is checked between stages; synchronous collaborators retain their limits.
func (p *Prepared) Layout(ctx context.Context, request LayoutRequest) (*Result, error) {
	fail := func(stage Stage, err error) (*Result, error) { return nil, &Error{stage, err} }
	if ctx == nil {
		return fail(StageInput, invalidInput("nil context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(StageInput, err)
	}
	if p == nil || p.owner == nil || p.content == nil {
		return fail(StageInput, invalidInput("nil or uninitialized preparation"))
	}
	params, err := validateParams(request.Params, p.text, p.config)
	if err != nil {
		return fail(StageInput, err)
	}
	geometry := layoutGeometry(params, request.Horizontal)
	if err := geometry.ValidateGeometry(); err != nil {
		return fail(StageInput, fmt.Errorf("%w: %w", ErrInvalidInput, err))
	}
	boxes, advances, err := validateAtomics(ctx, p.anchors, request.Atomics)
	if err != nil {
		return fail(StageBind, err)
	}
	bound, err := p.owner.BindParagraph(ctx, p.content, advances)
	if err != nil {
		return fail(StageBind, err)
	}
	return finishLayout(ctx, p.owner, bound, params, geometry, p.text, p.config, p.runs, boxes)
}

// Clone value envelopes, not arbitrary services or style implementations.
func cloneConfig(c khipu.Config) khipu.Config {
	if c.Streaming != nil {
		s := *c.Streaming
		s.Defaults.Features = slices.Clone(s.Defaults.Features)
		c.Streaming = &s
	}
	if c.Params != nil {
		p := *c.Params
		c.Params = &p
	}
	if c.ParFillSkip != nil {
		p := *c.ParFillSkip
		c.ParFillSkip = &p
	}
	return c
}
