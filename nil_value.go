package musicxml

import "reflect"

// nilValue reports whether value is nil or a typed nil, such as a nil
// *os.File in an io.Reader. A typed-nil reader, writer, provider or stream
// transfers no usable object, so entry points reject it like an untyped nil and
// never invoke its methods (including Close): they may dereference the nil value.
func nilValue(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
