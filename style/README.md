# style

Document style values and typography/paint projections shared by input adapters
and the root resolver. This package has no DOM, discovery or concrete font dependency.

`Style.Equals` compares every field, including color. `Style.Typography()` and
`Style.Paint()` return value projections; equal typography alone does not permit
merging shaping runs. Use the root `ProjectStyles` policy for that decision.

`Span` uses half-open UTF-8 byte ranges in processed paragraph text. `ValidateSpans`
requires complete ordered coverage, UTF-8 boundaries and positive finite em sizes.
Nil spans mean no supplied styles; nonnil empty spans cover only empty text.
Sizes use DU (65536 DU per big point), not pixels. Values contain no mutable font handles.

Validation and projection checks live in `style_test.go`; run `go test ./style`
from the module root. See the [integration guide](../README.md) for itemization
and separate paint ownership.

PaintSpan and ProjectPaint retain original paint ranges independently of shaping
metadata. Neither projection nor span storage imports a renderer.
