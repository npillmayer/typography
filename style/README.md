# style

Document style values and typography/paint projections shared by input adapters
and `glyphing.StyleMapper`. This package has no DOM, discovery or concrete font dependency.

`Style.Equals` compares every field, including color. `Style.Typography()` and
`Style.Paint()` return value projections; equal typography alone does not permit
merging shaping runs. Use `glyphing.ProjectStyles` and the itemizer for shaping projection and
coalescing; this shared package does not import glyphing.

`Span` uses half-open UTF-8 byte ranges in processed paragraph text. `ValidateSpans`
requires complete ordered coverage, UTF-8 boundaries and positive finite em sizes.
Nil spans mean no supplied styles; nonnil empty spans cover only empty text.
Sizes use DU (65536 DU per big point), not pixels. Values contain no mutable font handles.

Validation and projection checks live in `style_test.go`; run `go test ./style`
and `go test -race ./style`
from the module root. See the [integration guide](../README.md) for itemization
and separate paint ownership.

PaintSpan and ProjectPaint retain original paint ranges independently of shaping
metadata. Neither projection nor span storage imports a renderer.
