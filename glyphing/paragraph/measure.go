package paragraph

import (
	"github.com/npillmayer/khipu"
)

// MeasureIntrinsic returns natural inline bounds from retained horizontal
// material. It requires no atomic heights, line-height policy, paragraph strut
// or rendering services. Atomic ranges are signed advances including resolved
// horizontal margins; supply exactly one range per occurrence, in any ID order.
// Khipu owns validation, supported break policies and signed normalization.
// Supports NoHyphenation and bounded ExplicitHyphenation preparations. Automatic
// measurement and multiple eligible split positions or permitting variants in
// one textbox return khipu.ErrUnsupportedPolicy.
//
// Serialize calls with other operations on this preparation and keep borrowed
// source/styles/services stable. Requests are consumed without retaining their
// slices. Explicit measurement may lazily discover and shape replacement pairs,
// shared with later layouts. Complete batches (including empty results) are
// memoized; failures are retryable. Base text is never reshaped, vertical metrics
// are not queried, and previously returned layout results remain unchanged.
//
// Invalid handles use StageInput. Evaluation failures (including request
// validation and unsupported policy) use StageMeasure. Errors preserve their causes, return zero
// widths and leave the preparation reusable.
func (p *Prepared) MeasureIntrinsic(request khipu.MeasureRequest) (khipu.IntrinsicWidths, error) {
	if p == nil || p.discovery == nil || p.content == nil {
		return khipu.IntrinsicWidths{}, &Error{StageInput, invalidInput("nil or uninitialized preparation")}
	}
	widths, err := khipu.MeasureIntrinsic(p.content, p.discovery, request)
	if err != nil {
		return khipu.IntrinsicWidths{}, &Error{StageMeasure, err}
	}
	return widths, nil
}
