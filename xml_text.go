package musicxml

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// validXMLText checks the XML 1.0 character range. encoding/xml silently
// replaces invalid input with U+FFFD, so check strings before writing them.
func validXMLText(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if char < 0x20 && char != '\t' && char != '\r' && char != '\n' ||
			char == 0xfffe || char == 0xffff {
			return false
		}
	}
	return true
}

type documentTextFrame struct {
	value reflect.Value
	field string
	index int
	next  int
}

// checkDocumentText walks the sealed generated model, including named string
// types and ordered choices. The only recursive model type is OpusDocument;
// callers must checkDocumentNesting first. Keep one frame per active ancestor
// rather than recursing, and build the path only when reporting an error.
func checkDocumentText(document Document) error {
	root := reflect.ValueOf(document)
	stack := []documentTextFrame{{value: root, field: root.Type().Elem().Name()}}
	for len(stack) != 0 {
		frame := &stack[len(stack)-1]
		switch frame.value.Kind() {
		case reflect.Pointer:
			if !frame.value.IsNil() {
				frame.value = frame.value.Elem()
				continue
			}
		case reflect.Struct:
			if frame.next < frame.value.NumField() {
				index := frame.next
				frame.next++
				field := frame.value.Type().Field(index)
				// All root names are fixed by XML tags, so XMLName is not
				// serialized. Ignore other non-serialized fields as well.
				if field.Name == "XMLName" || field.PkgPath != "" || field.Tag.Get("xml") == "-" {
					continue
				}
				stack = append(stack, documentTextFrame{
					value: frame.value.Field(index),
					field: field.Name,
				})
				continue
			}
		case reflect.Slice:
			if frame.next < frame.value.Len() {
				index := frame.next
				frame.next++
				stack = append(stack, documentTextFrame{
					value: frame.value.Index(index),
					index: index,
				})
				continue
			}
		case reflect.String:
			if !validXMLText(frame.value.String()) {
				return fmt.Errorf("%s: string cannot be represented as XML 1.0 text", documentTextPath(stack))
			}
		}
		stack = stack[:len(stack)-1]
	}
	return nil
}

func documentTextPath(stack []documentTextFrame) string {
	var path strings.Builder
	for position, frame := range stack {
		switch {
		case position == 0:
			path.WriteString(frame.field)
		case frame.field != "":
			path.WriteString(".")
			path.WriteString(frame.field)
		default:
			fmt.Fprintf(&path, "[%d]", frame.index)
		}
	}
	return path.String()
}
