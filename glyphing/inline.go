package glyphing

import (
	"fmt"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	typestyle "github.com/npillmayer/typography/style"
)

// ItemizeInline itemizes text around opaque inline occurrences. Anchors must
// have unique nonzero IDs, strictly increasing byte offsets, and select exactly
// one U+FFFC OBJECT REPLACEMENT CHARACTER each. Unattached U+FFFC remains text.
// Original bytes/scalar positions are preserved. Attached markers stay unstyled
// and have no FontTextRun; pass the same anchors to khipu's paragraph preparation.
// Text spans retain their projected feature domains across object, script and
// fallback splits. ResolveStyle clips late-supplied features to each fragment;
// callers must retain the returned styles rather than reconstructing them.
// Source/paint/bidi associations remain with the caller. Request slices are not
// retained or modified. On error, only unstyled original text and nil bindings
// are returned (empty text for invalid UTF-8); no partial itemization is published.
//
// Glyph coverage is the caller's responsibility. This adds no validation pass:
// ordinary text uses the existing itemizer's coverage/fallback behavior, pending
// its separate relocation. Attached markers bypass style resolution and coverage.
// With no anchors this delegates to Itemize for text-only compatibility.
func (r *StyleMapper) ItemizeInline(text string, spans []typestyle.Span,
	atomics []contracts.AtomicAnchor) (styled.Text, []FontTextRun, error) {
	if !utf8.ValidString(text) {
		return styled.TextFromString(""), nil, fmt.Errorf("%w: invalid UTF-8", contracts.ErrIllegalArguments)
	}
	if len(atomics) == 0 {
		out, bindings, err := r.Itemize(text, spans)
		if err != nil {
			return styled.TextFromString(text), nil, err
		}
		return out, bindings, nil
	}
	out := styled.TextFromString(text)

	if r == nil {
		return out, nil, fmt.Errorf("%w: nil resolver", contracts.ErrIllegalArguments)
	}
	if err := typestyle.ValidateSpans(text, spans); err != nil {
		return out, nil, fmt.Errorf("%w: %w", contracts.ErrIllegalArguments, err)
	}

	if err := validateAtomicAnchors(text, atomics); err != nil {
		return out, nil, err
	}
	projected, err := r.projectInlineSpans(text, spans, atomics)
	if err != nil {
		return out, nil, err
	}
	result, bindings, err := r.itemizeSpans(text, projected, spans == nil)
	if err != nil {
		return out, nil, err
	}
	return result, bindings, nil
}

// This validates attachments, not glyph availability. UTF-8 is checked by the
// style envelope first. Bound the subtraction before forming offset+3.
func validateAtomicAnchors(text string, atomics []contracts.AtomicAnchor) error {
	seen := make(map[contracts.AtomicID]bool, len(atomics))
	for i, atom := range atomics {
		if atom.ID == 0 || seen[atom.ID] || (i > 0 && atom.ByteOffset <= atomics[i-1].ByteOffset) ||
			atom.ByteOffset > uint64(len(text)) || uint64(len(text))-atom.ByteOffset < 3 || text[atom.ByteOffset:atom.ByteOffset+3] != "\uFFFC" {
			return fmt.Errorf("%w: invalid atomic anchor at byte %d", contracts.ErrIllegalArguments, atom.ByteOffset)
		}
		seen[atom.ID] = true
	}
	return nil
}

// Project original domains before slicing out objects. Resolving only spans
// with text prevents object-only styles from requiring a usable font/language.
// Coalescing follows the same shaping/paint rules as ProjectStyles; a skipped
// object-only span is a barrier, never a bridge between unrelated domains.
func (r *StyleMapper) projectInlineSpans(text string, spans []typestyle.Span, atomics []contracts.AtomicAnchor) ([]itemizationSpan, error) {
	var groups []TypographySpan
	if spans == nil {
		groups = []TypographySpan{{Start: 0, End: uint64(len(text))}}
	} else {
		var previous contracts.ResolvedRunStyle
		var previousStyle typestyle.Style
		ai := 0
		for _, span := range spans {
			for ai < len(atomics) && atomics[ai].ByteOffset < span.Start {
				ai++
			}
			end := span.Start
			for j := ai; j < len(atomics) && atomics[j].ByteOffset == end && end < span.End; j++ {
				end += 3
			}
			if end == span.End {
				continue
			}
			resolved, err := r.ResolveStyle(span.Style, r.Defaults())
			if err != nil {
				return nil, err
			}
			merge := len(groups) > 0 && groups[len(groups)-1].End == span.Start

			merge = merge && sameShaping(previous, resolved) && span.Style.Direction == previousStyle.Direction && span.Style.UnicodeBidi == previousStyle.UnicodeBidi

			if merge {
				groups[len(groups)-1].End = span.End
			} else {
				groups = append(groups, TypographySpan{Start: span.Start, End: span.End, Style: TypographyStyle{Typography: span.Style.Typography(), domain: span.Start + 1}})
			}
			previous, previousStyle = resolved, span.Style
		}
	}
	var result []itemizationSpan
	ai := 0
	for _, group := range groups {
		window := featureWindow{active: true, domainStart: group.Start, domainEnd: group.End}
		from := group.Start
		for ai < len(atomics) && atomics[ai].ByteOffset < from {
			ai++
		}
		for ai < len(atomics) && atomics[ai].ByteOffset < group.End {
			at := atomics[ai].ByteOffset
			if from < at {
				result = append(result, itemizationSpan{TypographySpan{Start: from, End: at, Style: group.Style}, window})
			}
			from = at + 3
			ai++
		}
		if from < group.End {
			result = append(result, itemizationSpan{TypographySpan{Start: from, End: group.End, Style: group.Style}, window})
		}
	}
	return result, nil
}

// featureWindow is comparable, so styles still have value identity. The byte
// domain is an identity; from/to/length are original-domain scalar coordinates.
// Resolution clips late-supplied features to the emitted style range. Khipu may
// subsequently clip that range further for bidi without resetting its origin.
type featureWindow struct {
	active                 bool
	domainStart, domainEnd uint64
	from, to, length       int
}

func setFeatureWindows(text string, items []textItem) {
	var positions map[uint64]int
	for _, item := range items {
		w := item.style.features
		if !w.active {
			continue
		}
		if positions == nil {
			positions = make(map[uint64]int)
		}
		for _, p := range []uint64{w.domainStart, w.domainEnd, item.start, item.end} {
			positions[p] = 0
		}
	}
	if positions == nil {
		return
	}
	scalar := 0
	for pos := range text {
		if _, ok := positions[uint64(pos)]; ok {
			positions[uint64(pos)] = scalar
		}
		scalar++
	}
	positions[uint64(len(text))] = scalar
	for i := range items {
		w := &items[i].style.features
		if !w.active {
			continue
		}
		origin := positions[w.domainStart]
		w.from, w.to, w.length = positions[items[i].start]-origin, positions[items[i].end]-origin, positions[w.domainEnd]-origin
	}
}

func (w featureWindow) intersect(features []contracts.ShapeFeature) ([]contracts.ShapeFeature, error) {
	var result []contracts.ShapeFeature
	for _, f := range features {
		if f.Start > 0 && f.End > 0 && f.Start > f.End {
			return nil, fmt.Errorf("%w: reversed feature range", contracts.ErrIllegalArguments)
		}
		start, end := 0, w.length
		if f.Start > 0 {
			start = min(f.Start, w.length)
		}
		if f.End > 0 {
			end = min(f.End, w.length)
		}
		start, end = max(start, w.from), min(end, w.to)
		if start >= end {
			continue
		} // [0,0) means whole input, never an empty overlap.
		f.Start, f.End = start-w.from, end-w.from
		result = append(result, f)
	}
	return result, nil
}
