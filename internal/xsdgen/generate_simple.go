package xsdgen

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/token"
	"go/types"
	"math"
	"strconv"
	"strings"
)

var (
	ErrInvalidPackageName          = errors.New("xsdgen: invalid package name")
	ErrUnsupportedEnumerationValue = errors.New(
		"xsdgen: unsupported enumeration value",
	)
)

// GenerateSimpleTypes generates one formatted Go source file containing all
// named XSD simple types in index.
func GenerateSimpleTypes(
	index *Index,
	packageName string,
) ([]byte, error) {
	if index == nil {
		return nil, ErrNilIndex
	}
	if !token.IsIdentifier(packageName) {
		return nil, fmt.Errorf(
			"%w %q",
			ErrInvalidPackageName,
			packageName,
		)
	}

	names, err := NewTypeNames(index)
	if err != nil {
		return nil, err
	}

	plans, err := PlanSimpleTypes(index)
	if err != nil {
		return nil, err
	}

	renderer := simpleTypeRenderer{
		index:      index,
		names:      names,
		plans:      make(map[*Declaration]*SimpleTypeDefinition, len(plans)),
		kindStates: make(map[*Declaration]resolveState, len(plans)),
		kinds:      make(map[*Declaration]GoTypeKind, len(plans)),
	}
	for index := range plans {
		renderer.plans[plans[index].Declaration] = plans[index].Definition
	}

	var source bytes.Buffer
	writeGeneratedHeader(&source, index)
	fmt.Fprintf(&source, "package %s\n\n", packageName)
	for _, plan := range plans {
		if renderer.numericXMLType(plan.Definition) != "" {
			source.WriteString("import \"encoding/xml\"\n\n")
			break
		}
	}

	for index := range plans {
		if err := renderer.renderPlan(&source, &plans[index]); err != nil {
			return nil, err
		}
	}

	result, err := format.Source(source.Bytes())
	if err != nil {
		return nil, fmt.Errorf(
			"xsdgen: format generated simple types: %w",
			err,
		)
	}

	return result, nil
}

type resolveState uint8

const (
	resolveStateVisiting resolveState = iota + 1
	resolveStateResolved
)

type simpleTypeRenderer struct {
	index      *Index
	names      *TypeNames
	plans      map[*Declaration]*SimpleTypeDefinition
	kindStates map[*Declaration]resolveState
	kinds      map[*Declaration]GoTypeKind
	// namedBuiltins makes fields of built-in integer types use the named types
	// of namedBuiltinTypes. Only the complex generation sets it, because only
	// it declares those types; other generations keep raw Go integers.
	namedBuiltins bool
	// takenNames holds the package-scope type names of this generation, so
	// that a named built-in type avoids them when there is no manifest.
	takenNames map[string]string
	// builtinNames maps the built-in types that fields reference by name to
	// their Go names.
	builtinNames map[string]string
}

func (r *simpleTypeRenderer) renderPlan(
	target *bytes.Buffer,
	plan *SimpleTypePlan,
) error {
	goTypeValue, err := r.definitionGoType(plan.Definition)
	if err != nil {
		return fmt.Errorf(
			"xsdgen: resolve Go type for %q: %w",
			plan.Declaration.Name.Local,
			err,
		)
	}

	fmt.Fprintf(
		target,
		"// %s represents the %q XSD simple type.\n",
		plan.GoName,
		plan.Declaration.Name.Local,
	)
	if plan.Declaration.SimpleType != nil {
		renderDocumentation(target, "", r.index.documentation(plan.Declaration.SimpleType.Annotation))
	}
	fmt.Fprintf(
		target,
		"type %s %s\n\n",
		plan.GoName,
		goTypeValue.Expression,
	)

	r.renderNumericXMLMethods(target, plan.GoName, plan.Definition)

	enumerations := inlineEnumerations(plan.Definition)
	if len(enumerations) == 0 {
		return nil
	}

	target.WriteString("const (\n")
	literals := make([]string, 0, len(enumerations))
	for _, enumeration := range enumerations {
		literal, err := enumerationLiteral(
			goTypeValue.Kind,
			enumeration.Value,
		)
		if err != nil {
			return fmt.Errorf(
				"xsdgen: render enumeration %q of %q: %w",
				enumeration.Value,
				plan.Declaration.Name.Local,
				err,
			)
		}

		if enumeration.Value == "" {
			fmt.Fprintf(
				target,
				"\t// %s is the empty value.\n",
				enumeration.GoName,
			)
		} else {
			fmt.Fprintf(
				target,
				"// %s is the %q value.\n",
				enumeration.GoName,
				enumeration.Value,
			)
		}
		literals = append(literals, literal)
		fmt.Fprintf(
			target,
			"\t%s %s = %s\n",
			enumeration.GoName,
			plan.GoName,
			literal,
		)
	}
	target.WriteString(")\n\n")

	// A union's constants cover only its enumerated members, so only a
	// restriction can tell known values apart.
	if plan.Definition.Form == SimpleTypeRestriction &&
		len(plan.Definition.Enumerations) == len(enumerations) {
		renderEnumerationHelpers(target, plan.GoName, enumerations, literals)
	}

	return nil
}

// constantValueKey identifies the Go constant a literal denotes, so that
// spellings such as 1 and 01, or 0 and -0.0, compare equal.
func constantValueKey(literal string) string {
	value, err := types.Eval(token.NewFileSet(), nil, token.NoPos, literal)
	if err != nil || value.Value == nil {
		return literal
	}
	return value.Value.ExactString()
}

// renderEnumerationHelpers renders IsKnown and Values for a restriction
// whose constants are its complete value space.
func renderEnumerationHelpers(
	target *bytes.Buffer,
	goName string,
	enumerations []EnumerationPlan,
	literals []string,
) {
	names := make([]string, len(enumerations))
	// Distinct lexical values such as 1 and 01 can share a Go value, which
	// a switch may list only once.
	var cases []string
	seen := make(map[string]bool, len(literals))
	for index, enumeration := range enumerations {
		names[index] = enumeration.GoName
		value := constantValueKey(literals[index])
		if !seen[value] {
			seen[value] = true
			cases = append(cases, enumeration.GoName)
		}
	}

	fmt.Fprintf(
		target,
		"// IsKnown reports whether value is exactly one of the %s constants;\n"+
			"// XML whitespace is not collapsed first.\n",
		goName,
	)
	fmt.Fprintf(target, "func (value %s) IsKnown() bool {\n", goName)
	target.WriteString("\tswitch value {\n")
	fmt.Fprintf(target, "\tcase %s:\n", strings.Join(cases, ", "))
	target.WriteString("\t\treturn true\n")
	target.WriteString("\t}\n")
	target.WriteString("\treturn false\n")
	target.WriteString("}\n\n")

	fmt.Fprintf(
		target,
		"// Values returns a new slice of the %s constants in schema order.\n",
		goName,
	)
	fmt.Fprintf(target, "func (%s) Values() []%s {\n", goName, goName)
	fmt.Fprintf(
		target,
		"\treturn []%s{%s}\n",
		goName,
		strings.Join(names, ", "),
	)
	target.WriteString("}\n\n")
}

func (r *simpleTypeRenderer) definitionGoType(
	definition *SimpleTypeDefinition,
) (GoType, error) {
	if definition == nil {
		return GoType{}, ErrInvalidSimpleType
	}

	switch definition.Form {
	case SimpleTypeRestriction:
		if definition.Base == nil {
			return GoType{}, ErrInvalidSimpleType
		}

		return r.memberGoType(*definition.Base)

	case SimpleTypeUnion, SimpleTypeList:
		// The lexical representation preserves every union alternative and
		// the whitespace-separated value of an XSD list without guessing.
		return goType("string", GoTypeString), nil

	default:
		return GoType{}, fmt.Errorf(
			"%w: unknown form %q",
			ErrInvalidSimpleType,
			definition.Form,
		)
	}
}

func (r *simpleTypeRenderer) memberGoType(
	member SimpleTypeMember,
) (GoType, error) {
	switch {
	case member.Declaration != nil && member.Inline == nil:
		declaration := member.Declaration
		if declaration.Kind == DeclarationBuiltinSimpleType {
			return BuiltinGoType(declaration)
		}

		name, found := r.names.Lookup(declaration)
		if !found {
			return GoType{}, fmt.Errorf(
				"%w: missing Go name for %s",
				ErrInvalidSimpleType,
				describeDeclaration(declaration),
			)
		}

		kind, err := r.declarationKind(declaration)
		if err != nil {
			return GoType{}, err
		}

		return goType(name, kind), nil

	case member.Declaration == nil && member.Inline != nil:
		return r.definitionGoType(member.Inline)

	default:
		return GoType{}, ErrInvalidSimpleType
	}
}

func (r *simpleTypeRenderer) declarationKind(
	declaration *Declaration,
) (GoTypeKind, error) {
	switch r.kindStates[declaration] {
	case resolveStateResolved:
		return r.kinds[declaration], nil
	case resolveStateVisiting:
		return "", fmt.Errorf(
			"%w: cyclic simple type %s",
			ErrInvalidSimpleType,
			formatExpandedName(declaration.Name),
		)
	}

	definition, found := r.plans[declaration]
	if !found {
		return "", fmt.Errorf(
			"%w: missing plan for %s",
			ErrInvalidSimpleType,
			describeDeclaration(declaration),
		)
	}

	r.kindStates[declaration] = resolveStateVisiting

	value, err := r.definitionGoType(definition)
	if err != nil {
		delete(r.kindStates, declaration)
		return "", err
	}

	r.kindStates[declaration] = resolveStateResolved
	r.kinds[declaration] = value.Kind

	return value.Kind, nil
}

func inlineEnumerations(
	definition *SimpleTypeDefinition,
) []EnumerationPlan {
	if definition == nil {
		return nil
	}

	result := append(
		[]EnumerationPlan(nil),
		definition.Enumerations...,
	)

	for _, member := range definition.Members {
		if member.Inline != nil {
			result = append(
				result,
				inlineEnumerations(member.Inline)...,
			)
		}
	}

	return result
}

func enumerationLiteral(
	kind GoTypeKind,
	value string,
) (string, error) {
	switch kind {
	case GoTypeString:
		return strconv.Quote(value), nil

	case GoTypeBoolean:
		switch value {
		case "true", "1":
			return "true", nil
		case "false", "0":
			return "false", nil
		default:
			return "", unsupportedEnumeration(kind, value)
		}

	case GoTypeSignedInteger:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", unsupportedEnumeration(kind, value)
		}

		return strconv.FormatInt(parsed, 10), nil

	case GoTypeUnsignedInteger:
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return "", unsupportedEnumeration(kind, value)
		}

		return strconv.FormatUint(parsed, 10), nil

	case GoTypeFloat:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			return "", unsupportedEnumeration(kind, value)
		}

		return strconv.FormatFloat(parsed, 'g', -1, 64), nil

	case GoTypeBytes:
		return "", unsupportedEnumeration(kind, value)

	default:
		return "", unsupportedEnumeration(kind, value)
	}
}

func unsupportedEnumeration(
	kind GoTypeKind,
	value string,
) error {
	return fmt.Errorf(
		"%w %q for %s",
		ErrUnsupportedEnumerationValue,
		value,
		kind,
	)
}
