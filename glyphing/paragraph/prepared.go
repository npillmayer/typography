package paragraph

import (
	"fmt"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/uax/bidi"
)

// Prepared retains immutable Khipu material, matching Discovery and resolved
// bidi data. Serialize
// all operations on one handle; it has no concurrent-layout or transfer promise.
// Source/style/service objects remain borrowed under their stable-lifetime
// contracts. Request/configuration slices are copied; markup is not retained.
// Measurement and layout failures cannot change earlier results or invalidate
// this preparation. Measurement does not require final-layout geometry. Explicit
// replacement batches are memoized under the same serialized ownership contract
// and selected independently by each immutable layout trial.
type Prepared struct {
	content     *khipu.Khipu
	discovery   *khipu.Discovery
	text        styled.Text
	config      khipu.Config
	runs        []bidi.Segment
	anchors     []contracts.AtomicAnchor
	hyphenation khipu.HyphenationMode
}

// Prepare resolves bidi once and shapes text around atomic occurrences without
// requiring object geometry or a target width. It neither itemizes nor validates
// glyph coverage. The returned private styles retain their original identities.
// Zero config uses khipu's synthetic backend; CSSLineHeight later requires explicit
// streaming defaults. Hyphenation policy is fixed by options for every layout.
func Prepare(input Input, options khipu.PrepareOptions, config khipu.Config) (*Prepared, error) {
	fail := func(stage Stage, err error) (*Prepared, error) { return nil, &Error{stage, err} }

	if input.Direction != bidi.LeftToRight && input.Direction != bidi.RightToLeft {
		return fail(StageInput, invalidInput("direction"))
	}
	p := &Prepared{text: input.Text, config: cloneConfig(config), anchors: slices.Clone(input.Atomics), hyphenation: options.Hyphenation}
	// ParagraphFromText retains its text handle, also shared by frozen output.
	// An interior pointer (&p.text) would keep all working preparation alive.
	// Give it an independent handle while sharing the immutable cord and styles.
	text := p.text
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), input.Direction, input.Markup)
	if err != nil {
		return fail(StageParagraph, err)
	}
	for run := range para.BidiLevels().LogicalSegments() {
		p.runs = append(p.runs, run)
	}

	owner := khipu.NewKhipukamayuq(p.config)
	options.AtomicBoundaries = slices.Clone(options.AtomicBoundaries)
	p.content, p.discovery, err = owner.PrepareParagraph(khipu.ParagraphInput{Paragraph: para, Atomics: p.anchors}, options)
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

// Layout validates geometry, derives advances, binds copied atomic advances,
// breaks, freezes and assembles a detached result. It does not reshape the base
// paragraph. Lazy discretionary shaping is shared through discovery. Cancellation
// is confined to internal streaming coordination.
func (p *Prepared) Layout(request LayoutRequest) (*Result, error) {
	b, err := p.bindLayout(request)
	if err != nil {
		return nil, err
	}
	return finishLayout(b.bound, p.discovery, b.params, b.geometry, p.text, p.config, p.runs, b.boxes)
}

type boundLayout struct {
	params   Params
	geometry khipu.Params
	boxes    map[contracts.AtomicID]AtomicBox
	advances []khipu.AtomicAdvance
	bound    *khipu.AtomicBinding
}

func (p *Prepared) bindLayout(request LayoutRequest) (*boundLayout, error) {
	fail := func(stage Stage, err error) (*boundLayout, error) { return nil, &Error{stage, err} }

	if p == nil || p.discovery == nil || p.content == nil {
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
	boxes, advances, err := validateAtomics(p.anchors, request.Atomics)
	if err != nil {
		return fail(StageBind, err)
	}
	bound, err := khipu.BindParagraph(p.content, advances)
	if err != nil {
		return fail(StageBind, err)
	}
	return &boundLayout{params, geometry, boxes, advances, bound}, nil
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
