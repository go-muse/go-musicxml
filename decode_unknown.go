package musicxml

import "reflect"

// discardUnknownXMLContent removes zero choice wrappers left by encoding/xml
// after a generated ordered-content decoder skips an unknown child element.
func discardUnknownXMLContent(value any) {
	discardUnknownXMLValue(reflect.ValueOf(value))
}

func discardUnknownXMLValue(value reflect.Value) {
	if !value.IsValid() {
		return
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !value.IsNil() {
			discardUnknownXMLValue(value.Elem())
		}

	case reflect.Struct:
		valueType := value.Type()
		for index := 0; index < value.NumField(); index++ {
			field := value.Field(index)
			if !field.CanSet() {
				continue
			}

			definition := valueType.Field(index)
			if definition.Tag.Get("xml") == ",any" &&
				field.Kind() == reflect.Slice &&
				field.Type().Elem().Kind() == reflect.Struct {
				discardUnknownXMLSlice(field)
				continue
			}

			discardUnknownXMLValue(field)
		}

	case reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			discardUnknownXMLValue(value.Index(index))
		}
	}
}

func discardUnknownXMLSlice(value reflect.Value) {
	writeIndex := 0
	for readIndex := 0; readIndex < value.Len(); readIndex++ {
		item := value.Index(readIndex)
		if item.IsZero() {
			continue
		}

		discardUnknownXMLValue(item)
		if writeIndex != readIndex {
			value.Index(writeIndex).Set(item)
		}
		writeIndex++
	}
	if writeIndex == 0 {
		value.Set(reflect.Zero(value.Type()))
		return
	}

	zero := reflect.Zero(value.Type().Elem())
	for index := writeIndex; index < value.Len(); index++ {
		value.Index(index).Set(zero)
	}
	value.SetLen(writeIndex)
}
