package opentype

import (
	"fmt"
	"reflect"

	"github.com/npillmayer/khipu/contracts"
)

// acceptedFace inspects only immutable preparation state, never lazy font tables.
// Both preflight and direct backend entry points use these acceptance rules.
func acceptedFace(value contracts.FontFace) (*Face, error) {
	if value == nil {
		return nil, fmt.Errorf("%w: nil OpenType face", contracts.ErrIllegalArguments)
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		if v.IsNil() {
			return nil, fmt.Errorf("%w: nil face %T", contracts.ErrIllegalArguments, value)
		}
	}
	face, ok := value.(*Face)
	if !ok {
		return nil, fmt.Errorf("%w: OpenType consumer received %T", contracts.ErrUnsupportedFontFace, value)
	}
	if face.font == nil || face.resource == nil || len(face.data) == 0 {
		return nil, fmt.Errorf("%w: uninitialized OpenType face", contracts.ErrIllegalArguments)
	}
	return face, nil
}

// CheckFace reports whether this stream can shape the prepared face. It performs
// no queries or stream-state changes, even after the stream has finished.
func (*Stream) CheckFace(value contracts.FontFace) error {
	_, err := acceptedFace(value)
	return err
}

// CheckFace reports whether coverage can query the prepared face without doing
// any lookup, parsing or I/O. It is independent of stream and rendering state.
func (Services) CheckFace(value contracts.FontFace) error {
	_, err := acceptedFace(value)
	return err
}
