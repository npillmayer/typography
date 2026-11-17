package style

// PaintSpan retains document paint over a half-open processed UTF-8 byte range,
// independently of coalesced shaping boundaries.
type PaintSpan struct {
	Start, End uint64
	Paint      Paint
}

// ProjectPaint detaches the document paint projection in processed UTF-8 bytes.
// Callers validate document spans before projection; no shaping or coalescing is
// performed here. Original paint boundaries are retained.
func ProjectPaint(spans []Span) []PaintSpan {
	if spans == nil {
		return nil
	}
	out := make([]PaintSpan, len(spans))
	for i, s := range spans {
		out[i] = PaintSpan{Start: s.Start, End: s.End, Paint: s.Style.Paint()}
	}
	return out
}
