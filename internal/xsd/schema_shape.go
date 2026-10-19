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
	if !value.IsValid() || !sameSchemaShape(value.Type(), schemaDataType, map[[2]reflect.Type]bool{}) {
		return fmt.Errorf("xsd: %v does not have the shape of SchemaData", value.Type())
	}
	return nil
}

// sameSchemaShape compares field names and kinds at every level. Recursive
// types are assumed equal while they are being compared.
func sameSchemaShape(a, b reflect.Type, seen map[[2]reflect.Type]bool) bool {
	if a == nil || b == nil || a.Kind() != b.Kind() {
		return false
	}
	pair := [2]reflect.Type{a, b}
	if seen[pair] {
		return true
	}
	seen[pair] = true
	switch a.Kind() {
	case reflect.Pointer, reflect.Slice:
		return sameSchemaShape(a.Elem(), b.Elem(), seen)
	case reflect.Map:
		return sameSchemaShape(a.Key(), b.Key(), seen) && sameSchemaShape(a.Elem(), b.Elem(), seen)
	case reflect.Struct:
		if a.NumField() != b.NumField() {
			return false
		}
		for i := range a.NumField() {
			fa, fb := a.Field(i), b.Field(i)
			if fa.Name != fb.Name || !fa.IsExported() || !sameSchemaShape(fa.Type, fb.Type, seen) {
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
