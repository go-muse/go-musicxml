package xsdgen

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strconv"
)

// attributeEnumerationPrefixes names the constants of anonymous global
// attribute enumerations by the attribute's namespace and its usual prefix.
var attributeEnumerationPrefixes = map[string]struct{ goName, xmlPrefix string }{
	"http://www.w3.org/1999/xlink":         {"XLink", "xlink"},
	"http://www.w3.org/XML/1998/namespace": {"XML", "xml"},
}

// renderAttributeEnumerations renders untyped string constants for the values
// of global attributes in the xlink and xml namespaces whose type is an
// anonymous enumeration, such as xlink:show. The fields that carry them stay
// strings, so the constants are untyped.
func renderAttributeEnumerations(target *bytes.Buffer, index *Index) error {
	var declarations []*Declaration
	for _, declaration := range index.attributes {
		if _, found := attributeEnumerationPrefixes[declaration.Name.Namespace]; found &&
			declaration.Attribute != nil && declaration.Attribute.SimpleType != nil {
			declarations = append(declarations, declaration)
		}
	}
	slices.SortFunc(declarations, func(a, b *Declaration) int {
		return cmp.Or(
			cmp.Compare(a.Name.Namespace, b.Name.Namespace),
			cmp.Compare(a.Name.Local, b.Name.Local),
		)
	})

	for _, declaration := range declarations {
		prefix := attributeEnumerationPrefixes[declaration.Name.Namespace]
		local, err := GoTypeName(declaration.Name.Local)
		if err != nil {
			return err
		}
		planner := simpleTypePlanner{
			index:            index,
			file:             declaration.File,
			ownerName:        declaration.Name.Local,
			constantPrefix:   prefix.goName + local,
			enumerationNames: make(map[string]string),
		}
		definition, err := planner.plan(declaration.Attribute.SimpleType)
		if err != nil {
			return fmt.Errorf(
				"xsdgen: plan enumeration of attribute %s: %w",
				formatExpandedName(declaration.Name),
				err,
			)
		}
		if len(definition.Enumerations) == 0 {
			continue
		}

		attribute := prefix.xmlPrefix + ":" + declaration.Name.Local
		fmt.Fprintf(
			target,
			"// Values of the %s attribute. The fields that carry it are strings.\n",
			attribute,
		)
		target.WriteString("const (\n")
		for _, enumeration := range definition.Enumerations {
			fmt.Fprintf(
				target,
				"\t// %s is the %q value of %s.\n",
				enumeration.GoName,
				enumeration.Value,
				attribute,
			)
			fmt.Fprintf(
				target,
				"\t%s = %s\n",
				enumeration.GoName,
				strconv.Quote(enumeration.Value),
			)
		}
		target.WriteString(")\n\n")
	}

	return nil
}
