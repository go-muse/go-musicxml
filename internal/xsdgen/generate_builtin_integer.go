package xsdgen

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
)

// namedBuiltinType is a built-in integer type that fields reference through a
// generated named Go type instead of a raw Go integer.
type namedBuiltinType struct {
	GoType
	// codec is the private text adapter that decodes every XSD lexical form of
	// the type, or "" when encoding/xml already accepts them all.
	codec string
	doc   string
}

// namedBuiltinTypes keeps the structs that hold these integers free of XML
// methods: a named type decodes its own lexical forms, such as "+1" and "-0",
// which strconv.ParseUint rejects, so no shadow wire struct is needed. Derived
// simple types keep their raw Go base types.
var namedBuiltinTypes = map[string]namedBuiltinType{
	"positiveInteger": {
		GoType: goType("PositiveInteger", GoTypeUnsignedInteger),
		codec:  "xmlUnsigned64",
		doc:    "an xs:positiveInteger value",
	},
	"nonNegativeInteger": {
		GoType: goType("NonNegativeInteger", GoTypeUnsignedInteger),
		codec:  "xmlUnsigned64",
		doc:    "an xs:nonNegativeInteger value",
	},
	"integer": {
		GoType: goType("Integer", GoTypeSignedInteger),
		doc:    "an xs:integer value",
	},
}

// namedBuiltin returns the named type of a built-in declaration, if it has one.
func namedBuiltin(declaration *Declaration) (namedBuiltinType, bool) {
	if declaration == nil || declaration.Kind != DeclarationBuiltinSimpleType || declaration.Name.Namespace != Namespace {
		return namedBuiltinType{}, false
	}
	value, found := namedBuiltinTypes[declaration.Name.Local]
	return value, found
}

// builtinKey identifies a built-in type in the name manifest.
func builtinKey(declaration *Declaration) string {
	return "XMLSchema builtin " + declaration.Name.Local
}

// fieldGoType returns the Go type of a field whose type is declaration: the
// named type of a built-in integer, named through the manifest, or the type
// memberGoType derives.
func (r *simpleTypeRenderer) fieldGoType(declaration *Declaration) (GoType, error) {
	named, found := namedBuiltin(declaration)
	if !found || !r.namedBuiltins {
		return r.memberGoType(SimpleTypeMember{Declaration: declaration})
	}
	if name, assigned := r.builtinNames[declaration.Name.Local]; assigned {
		value := named.GoType
		value.Expression = name
		return value, nil
	}
	var name string
	if r.index != nil && r.index.names != nil {
		var err error
		if name, err = r.index.names.Assign(builtinKey(declaration), named.Expression); err != nil {
			return GoType{}, err
		}
	} else {
		// Without a manifest, avoid the names of this generation's types and
		// the package's enumeration constants. Names are then unique only
		// within this generation: a package built from several generations
		// (element, document) needs a shared manifest (-names).
		name = named.Expression
		for suffix := 2; r.takenNames[name] != "" || r.constantNames[name]; suffix++ {
			name = named.Expression + strconv.Itoa(suffix)
		}
	}
	if r.takenNames != nil {
		r.takenNames[name] = "built-in type " + declaration.Name.Local
	}
	if r.builtinNames == nil {
		r.builtinNames = make(map[string]string)
	}
	r.builtinNames[declaration.Name.Local] = name
	value := named.GoType
	value.Expression = name
	return value, nil
}

// builtinKind returns the kind of a named built-in type that a field of this
// generation references by goType.
func (r *simpleTypeRenderer) builtinKind(goType string) (GoTypeKind, bool) {
	for local, name := range r.builtinNames {
		if name == goType {
			return namedBuiltinTypes[local].Kind, true
		}
	}
	return "", false
}

// renderNamedBuiltins renders the named built-in types that fields of this
// generation reference. Only the complex generation renders them; the element
// and document generations of the same package reference them.
func (r *simpleTypeRenderer) renderNamedBuiltins(target *bytes.Buffer) {
	locals := make([]string, 0, len(r.builtinNames))
	for local := range r.builtinNames {
		locals = append(locals, local)
	}
	sort.Strings(locals)
	for _, local := range locals {
		named, name := namedBuiltinTypes[local], r.builtinNames[local]
		base := "uint64"
		if named.Kind == GoTypeSignedInteger {
			base = "int64"
		}
		fmt.Fprintf(target, "// %s is %s in a field whose type is the\n", name, named.doc)
		fmt.Fprintf(target, "// built-in type itself. Decoding accepts every XSD lexical form that fits\n")
		fmt.Fprintf(target, "// %s, such as \"+1\"; validation, not decoding, checks the range.\n", base)
		fmt.Fprintf(target, "type %s %s\n\n", name, base)
		if named.codec == "" {
			continue
		}
		fmt.Fprintf(target, "// UnmarshalText decodes an XSD integer lexical form, such as \"+1\" or \"-0\".\n")
		fmt.Fprintf(target, "func (value *%s) UnmarshalText(text []byte) error {\n", name)
		fmt.Fprintf(target, "return (*%s)(value).UnmarshalText(text)\n}\n\n", named.codec)
	}
}

// enumerationConstantNames returns the Go names of the enumeration constants
// that the simple generation emits for plans, collected with the renderer's
// own inlineEnumerations so that the two sets stay identical.
func enumerationConstantNames(plans []SimpleTypePlan) map[string]bool {
	names := make(map[string]bool)
	for index := range plans {
		for _, enumeration := range inlineEnumerations(plans[index].Definition) {
			names[enumeration.GoName] = true
		}
	}
	return names
}
