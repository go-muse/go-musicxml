package xsd

import (
	"fmt"
	"reflect"
)

var schemaDataType = reflect.TypeFor[SchemaData]()

// schemaMapName reads a map key of the shape of QName.
func schemaMapName(key reflect.Value) QName {
	return QName{Space: key.Field(0).String(), Local: key.Field(1).String()}
}

// checkSchemaShape reports whether value is a struct of the shape of
// SchemaData, so that the reflective graph copy can move it between families.
func checkSchemaShape(value reflect.Value) error {
	if !value.IsValid() {
		return fmt.Errorf("xsd: a nil schema graph does not have the shape of SchemaData")
	}
	if !sameSchemaShape(value.Type(), schemaDataType, newShapeMatch()) {
		return fmt.Errorf("xsd: %v does not have the shape of SchemaData", value.Type())
	}
	return nil
}

// shapeMatch records the type correspondence of a shape comparison.
type shapeMatch struct {
	seen          map[[2]reflect.Type]bool
	forward, back map[reflect.Type]reflect.Type
}

func newShapeMatch() *shapeMatch {
	return &shapeMatch{seen: map[[2]reflect.Type]bool{}, forward: map[reflect.Type]reflect.Type{}, back: map[reflect.Type]reflect.Type{}}
}

// sameSchemaShape compares field names and kinds at every level. Composite
// types must correspond one to one, because the copy assigns a pointer, map or
// slice made for one position to every position that shares it. Recursive
// types are assumed equal while they are being compared.
func sameSchemaShape(a, b reflect.Type, match *shapeMatch) bool {
	if a == nil || b == nil || a.Kind() != b.Kind() {
		return false
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Struct:
		if prior, ok := match.forward[a]; ok && prior != b {
			return false
		}
		if prior, ok := match.back[b]; ok && prior != a {
			return false
		}
		match.forward[a], match.back[b] = b, a
	}
	pair := [2]reflect.Type{a, b}
	if match.seen[pair] {
		return true
	}
	match.seen[pair] = true
	switch a.Kind() {
	case reflect.Pointer, reflect.Slice:
		return sameSchemaShape(a.Elem(), b.Elem(), match)
	case reflect.Map:
		return sameSchemaShape(a.Key(), b.Key(), match) && sameSchemaShape(a.Elem(), b.Elem(), match)
	case reflect.Struct:
		if a.NumField() != b.NumField() {
			return false
		}
		for i := range a.NumField() {
			fa, fb := a.Field(i), b.Field(i)
			if fa.Name != fb.Name || !fa.IsExported() || !sameSchemaShape(fa.Type, fb.Type, match) {
				return false
			}
		}
		return true
	case reflect.String, reflect.Bool, reflect.Uint8, reflect.Uint64:
		return true
	default:
		return false
	}
}
