// Package style defines document style values shared by content adapters and
// typography resolvers. It has no DOM, discovery, or concrete font dependency.
package style

import (
	"fmt"
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
	"unicode/utf8"
)

// Style preserves the existing document vocabulary. All fields are values;
// EmSize is a computed size in scaled big points, never an unevaluated expression.
// Family is the existing family-list representation, not a parsed font handle.
// Equality includes paint; projection does not authorize merging shaping runs.
type Style struct {
	Family                                          string
	EmSize                                          dimen.DU
	Weight                                          int
	Slant, Color, Language, WhiteSpace, UnicodeBidi string
	Direction                                       bidi.Direction
}

var _ styled.Style = Style{}

func (s Style) Equals(other styled.Style) bool { o, ok := other.(Style); return ok && s == o }
func (s Style) String() string {
	return fmt.Sprintf("%s %.3g bp weight=%d %s color=%s lang=%s dir=%s white-space=%s", s.Family, s.EmSize.Points(), s.Weight, s.Slant, s.Color, s.Language, s.Direction, s.WhiteSpace)
}

// Typography is the non-paint projection, including preserved paragraph/style
// semantics. It is not a resolved shaping key: faces, features and bidi context
// must still be resolved before considering run equivalence.
type Typography struct {
	Family                                   string
	EmSize                                   dimen.DU
	Weight                                   int
	Slant, Language, WhiteSpace, UnicodeBidi string
	Direction                                bidi.Direction
}
type Paint struct{ Color string }

func (s Style) Typography() Typography {
	return Typography{s.Family, s.EmSize, s.Weight, s.Slant, s.Language, s.WhiteSpace, s.UnicodeBidi, s.Direction}
}
func (s Style) Paint() Paint { return Paint{Color: s.Color} }

// Span uses half-open UTF-8 byte coordinates in processed paragraph text.
type Span struct {
	Start, End uint64
	Style      Style
}

// ValidateSpans checks complete ordered coverage and byte boundaries. It does not
// implement CSS/paragraph eligibility or decide grapheme-level paint policy.
// Nil means no supplied styles and is accepted. Non-nil empty covers empty text.
func ValidateSpans(text string, spans []Span) error {
	if !utf8.ValidString(text) {
		return fmt.Errorf("text must be UTF-8")
	}
	if spans == nil {
		return nil
	}
	var end uint64
	for _, s := range spans {
		if s.Start != end || s.End <= s.Start || s.End > uint64(len(text)) || (s.End < uint64(len(text)) && !utf8.RuneStart(text[s.End])) || s.Style.EmSize <= 0 || s.Style.EmSize >= dimen.Fil {
			return fmt.Errorf("invalid style span [%d,%d)", s.Start, s.End)
		}
		end = s.End
	}
	if end != uint64(len(text)) {
		return fmt.Errorf("style spans do not cover text")
	}
	return nil
}
