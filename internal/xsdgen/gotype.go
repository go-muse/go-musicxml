package xsdgen

import (
	"errors"
	"fmt"
)

var ErrUnsupportedBuiltinType = errors.New(
	"xsdgen: unsupported built-in simple type",
)

// GoTypeKind identifies the Go representation category of an XSD simple type.
type GoTypeKind string

const (
	GoTypeString          GoTypeKind = "string"
	GoTypeBoolean         GoTypeKind = "boolean"
	GoTypeSignedInteger   GoTypeKind = "signed integer"
	GoTypeUnsignedInteger GoTypeKind = "unsigned integer"
	GoTypeFloat           GoTypeKind = "float"
	GoTypeBytes           GoTypeKind = "bytes"
)

// GoType describes a Go representation and the category needed to render
// constants for it.
type GoType struct {
	Expression string
	Kind       GoTypeKind
}

// BuiltinGoType returns the Go representation of an XML Schema built-in
// simple type.
func BuiltinGoType(declaration *Declaration) (GoType, error) {
	if declaration == nil ||
		declaration.Kind != DeclarationBuiltinSimpleType ||
		declaration.Name.Namespace != Namespace {
		description := "<nil>"
		if declaration != nil {
			description = describeDeclaration(declaration)
		}

		return GoType{}, fmt.Errorf(
			"%w: %s",
			ErrUnsupportedBuiltinType,
			description,
		)
	}

	value, found := builtinGoTypes[declaration.Name.Local]
	if !found {
		return GoType{}, fmt.Errorf(
			"%w: %s",
			ErrUnsupportedBuiltinType,
			formatExpandedName(declaration.Name),
		)
	}

	return value, nil
}

func goType(expression string, kind GoTypeKind) GoType {
	return GoType{
		Expression: expression,
		Kind:       kind,
	}
}

var builtinGoTypes = map[string]GoType{
	"ENTITIES":           goType("string", GoTypeString),
	"ENTITY":             goType("string", GoTypeString),
	"ID":                 goType("string", GoTypeString),
	"IDREF":              goType("string", GoTypeString),
	"IDREFS":             goType("string", GoTypeString),
	"NCName":             goType("string", GoTypeString),
	"NMTOKEN":            goType("string", GoTypeString),
	"NMTOKENS":           goType("string", GoTypeString),
	"NOTATION":           goType("string", GoTypeString),
	"Name":               goType("string", GoTypeString),
	"QName":              goType("string", GoTypeString),
	"anySimpleType":      goType("string", GoTypeString),
	"anyURI":             goType("string", GoTypeString),
	"base64Binary":       goType("[]byte", GoTypeBytes),
	"boolean":            goType("bool", GoTypeBoolean),
	"byte":               goType("int8", GoTypeSignedInteger),
	"date":               goType("string", GoTypeString),
	"dateTime":           goType("string", GoTypeString),
	"decimal":            goType("float64", GoTypeFloat),
	"double":             goType("float64", GoTypeFloat),
	"duration":           goType("string", GoTypeString),
	"float":              goType("float32", GoTypeFloat),
	"gDay":               goType("string", GoTypeString),
	"gMonth":             goType("string", GoTypeString),
	"gMonthDay":          goType("string", GoTypeString),
	"gYear":              goType("string", GoTypeString),
	"gYearMonth":         goType("string", GoTypeString),
	"hexBinary":          goType("[]byte", GoTypeBytes),
	"int":                goType("int32", GoTypeSignedInteger),
	"integer":            goType("int64", GoTypeSignedInteger),
	"language":           goType("string", GoTypeString),
	"long":               goType("int64", GoTypeSignedInteger),
	"negativeInteger":    goType("int64", GoTypeSignedInteger),
	"nonNegativeInteger": goType("uint64", GoTypeUnsignedInteger),
	"nonPositiveInteger": goType("int64", GoTypeSignedInteger),
	"normalizedString":   goType("string", GoTypeString),
	"positiveInteger":    goType("uint64", GoTypeUnsignedInteger),
	"short":              goType("int16", GoTypeSignedInteger),
	"string":             goType("string", GoTypeString),
	"time":               goType("string", GoTypeString),
	"token":              goType("string", GoTypeString),
	"unsignedByte":       goType("uint8", GoTypeUnsignedInteger),
	"unsignedInt":        goType("uint32", GoTypeUnsignedInteger),
	"unsignedLong":       goType("uint64", GoTypeUnsignedInteger),
	"unsignedShort":      goType("uint16", GoTypeUnsignedInteger),
}
