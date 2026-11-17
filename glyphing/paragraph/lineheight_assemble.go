package paragraph

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/typography/layout"
)

// assembleLineHeight measures the strut, source styles and frozen lines in that
// order. Only a fully assembled, cancellation-checked vlist is published.
func assembleLineHeight(ctx context.Context, f *khipu.FrozenKhipu, l *LineLayout, text styled.Text, cfg khipu.Config, atomics map[contracts.AtomicID]AtomicBox) (*assembly, error) {
	// The caller supplies CSS-processed text. An empty source has no text,
	// preserved whitespace/newline, or atomic content, so it creates no line.
	// Keep the frozen diagnostic result, but do not turn its final fill into a strut.
	if text.Raw().Len() == 0 && len(atomics) == 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return &assembly{frozen: f}, nil
	}
	strut, err := cssFaceMetrics(cfg.Streaming.Defaults)
	if err != nil {
		return nil, err
	}
	sa, sd, err := l.Default.extents(strut)
	if err != nil {
		return nil, err
	}
	runs, err := lineHeightSourceMetrics(ctx, f, text, cfg)
	if err != nil {
		return nil, err
	}
	v := &assembly{frozen: f}
	top := 0.0
	for i := 0; i < f.LineCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, _ := f.Line(i)
		extents, err := measureLineHeight(ctx, f, line, l, runs, atomics, lineHeightExtents{sa, sd})
		if err != nil {
			return nil, err
		}
		top, err = appendLineHeight(v, i, line, top, extents)
		if err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v.box.Extent = top
	return v, nil
}

// lineHeightSourceMetrics excludes attached marker-only styles before resolving
// source metrics for surviving spaces. Frozen glyphs retain their own metrics.
func lineHeightSourceMetrics(ctx context.Context, f *khipu.FrozenKhipu, text styled.Text, cfg khipu.Config) ([]metricRun, error) {
	var excluded []khipu.ByteRange
	for i := 0; i < f.KnotCount(); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, ok := f.Atomic(i); ok {
			knot, _ := f.Knot(i)
			excluded = append(excluded, knot.Source.Bytes)
		}
	}
	sort.Slice(excluded, func(i, j int) bool { return excluded[i].Start < excluded[j].Start })
	return sourceMetrics(ctx, text, cfg, excluded)
}

// lineHeightExtents holds signed effective distances above and below the baseline.
// Each line starts with the strut and takes maxima in frozen traversal order.
type lineHeightExtents struct{ ascent, descent float64 }

// includeText applies every intersecting override and the default in uncovered
// source intervals. Layout ranges do not split shaping or reset source origins.
func (e *lineHeightExtents) includeText(l *LineLayout, start, end uint64, m fontExtents) error {
	add := func(h LineHeight) error {
		x, y, err := h.extents(m)
		if err != nil {
			return err
		}
		e.ascent = math.Max(e.ascent, x)
		e.descent = math.Max(e.descent, y)
		return nil
	}
	at := start
	j := sort.Search(len(l.Runs), func(j int) bool { return l.Runs[j].End > start })
	for ; j < len(l.Runs) && l.Runs[j].Start < end; j++ {
		s := l.Runs[j]
		if s.Start > at {
			if e := add(l.Default); e != nil {
				return e
			}
		}
		if e := add(s.Height); e != nil {
			return e
		}
		at = min(end, s.End)
	}
	if at < end {
		return add(l.Default)
	}
	return nil
}

// measureLineHeight combines atomic geometry, frozen glyph metrics and surviving
// source spaces. Generated fill and silent discretionary text do not contribute.
func measureLineHeight(ctx context.Context, f *khipu.FrozenKhipu, line khipu.FrozenLine, l *LineLayout, runs []metricRun, atomics map[contracts.AtomicID]AtomicBox, extents lineHeightExtents) (lineHeightExtents, error) {
	for k := line.FirstKnot; k < line.FirstKnot+line.KnotCount; k++ {
		if err := ctx.Err(); err != nil {
			return lineHeightExtents{}, err
		}
		if id, ok := f.Atomic(k); ok {
			box, exists := atomics[id]
			if !exists {
				return lineHeightExtents{}, fmt.Errorf("%w: missing atomic box", contracts.ErrIllegalArguments)
			}
			extents.ascent = math.Max(extents.ascent, float64(box.above())/float64(dimen.BP))
			extents.descent = math.Max(extents.descent, 0)
			continue
		}
		knot, _ := f.Knot(k)
		src := knot.Source.Bytes
		if src.End <= src.Start {
			continue
		} // generated paragraph fill
		tx, ok := f.CopyText(k)
		if ok {
			if len(tx.Glyphs) == 0 {
				continue
			} // silent discretionary material
			m, e := cssMetrics(tx.Context.Params.Font)
			if e != nil {
				return lineHeightExtents{}, e
			}
			if e = extents.includeText(l, src.Start, src.End, m); e != nil {
				return lineHeightExtents{}, e
			}
		} else {
			// Glue has no retained text context; locate its prepared source font(s).
			j := sort.Search(len(runs), func(j int) bool { return runs[j].end > src.Start })
			for ; j < len(runs) && runs[j].start < src.End; j++ {
				r := runs[j]
				if e := extents.includeText(l, max(src.Start, r.start), min(src.End, r.end), r.metrics); e != nil {
					return lineHeightExtents{}, e
				}
			}
		}
	}
	return extents, nil
}

// appendLineHeight validates completed geometry before retaining a line box.
// Keep the float64 operation order and signed baseline; no compatibility kern
// or integer-DU rounding belongs in CSS line-height placement.
func appendLineHeight(v *assembly, i int, line khipu.FrozenLine, top float64, extents lineHeightExtents) (float64, error) {
	height := extents.ascent + extents.descent
	baseline := top + extents.ascent
	end := top + height
	width := math.Max(line.Width.Points(), line.TargetWidth.Points())
	if !dimension(height) || !dimension(end) || !dimension(math.Abs(baseline)) || !dimension(width) || line.Width < 0 || line.TargetWidth < 0 {
		return 0, fmt.Errorf("%w: line-height line %d: geometry outside finite dimension range", contracts.ErrIllegalArguments, i)
	}
	v.lines = append(v.lines, layout.Line{Index: i, Top: top, Baseline: baseline, Ascent: extents.ascent, Descent: extents.descent, Height: height, Horizontal: line})
	v.material = append(v.material, layout.Material{Kind: layout.LineBox, Child: i, Top: top, Extent: height})
	v.box.Width = math.Max(v.box.Width, width)
	if i == 0 {
		v.box.Baseline = baseline
		v.box.HasBaseline = true
	}
	return end, nil
}
