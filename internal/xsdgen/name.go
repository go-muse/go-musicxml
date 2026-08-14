package xsdgen

import (
	"errors"
	"fmt"
	"go/token"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidGoName         = errors.New("xsdgen: invalid Go name")
	ErrInvalidGoNameOverride = errors.New(
		"xsdgen: invalid Go name override",
	)
	ErrGoNameCollision = errors.New("xsdgen: Go name collision")
)

// TypeNameOverride assigns a Go identifier to one user-defined XSD type.
//
// Prefix is ignored when matching Name; XSD declarations are identified by
// namespace URI and local name.
type TypeNameOverride struct {
	Name   QName
	GoName string
}

// GoNameCollisionError reports distinct XSD names that normalize to the same
// Go identifier.
type GoNameCollisionError struct {
	Name   string
	First  string
	Second string
}

func (e *GoNameCollisionError) Error() string {
	return fmt.Sprintf(
		"%v %q: %s and %s",
		ErrGoNameCollision,
		e.Name,
		e.First,
		e.Second,
	)
}

func (e *GoNameCollisionError) Unwrap() error {
	return ErrGoNameCollision
}

// TypeNames contains deterministic Go names for user-defined XSD types.
//
// Built-in XML Schema types are deliberately excluded. They are mapped to Go
// representations by the code-generation layer.
type TypeNames struct {
	byDeclaration map[*Declaration]string
	declarations  []*Declaration
}

// NewTypeNames creates names for all user-defined simple and complex types.
func NewTypeNames(index *Index) (*TypeNames, error) {
	return NewTypeNamesWithOverrides(index)
}

// NewTypeNamesWithOverrides creates names for all user-defined simple and
// complex types, applying explicit names before collision detection.
func NewTypeNamesWithOverrides(
	index *Index,
	overrides ...TypeNameOverride,
) (*TypeNames, error) {
	if index == nil {
		return nil, ErrNilIndex
	}

	overrideNames, err := validateTypeNameOverrides(index, overrides)
	if err != nil {
		return nil, err
	}

	declarations := userTypeDeclarations(index)
	result := &TypeNames{
		byDeclaration: make(map[*Declaration]string, len(declarations)),
		declarations:  declarations,
	}
	used := make(map[string]*Declaration, len(declarations))

	for _, declaration := range declarations {
		key := expandedName{
			namespace: declaration.Name.Namespace,
			local:     declaration.Name.Local,
		}
		name, overridden := overrideNames[key]
		if !overridden {
			name, err = GoTypeName(declaration.Name.Local)
			if err != nil {
				return nil, fmt.Errorf(
					"xsdgen: name %s from %q: %w",
					formatExpandedName(declaration.Name),
					declarationPath(declaration),
					err,
				)
			}
		}

		if existing, found := used[name]; found {
			return nil, &GoNameCollisionError{
				Name:   name,
				First:  describeDeclaration(existing),
				Second: describeDeclaration(declaration),
			}
		}

		used[name] = declaration
		result.byDeclaration[declaration] = name
	}

	return result, nil
}

func validateTypeNameOverrides(
	index *Index,
	overrides []TypeNameOverride,
) (map[expandedName]string, error) {
	result := make(map[expandedName]string, len(overrides))

	for _, override := range overrides {
		if !validNCName(override.Name.Local) ||
			!exportedGoIdentifier(override.GoName) {
			return nil, fmt.Errorf(
				"%w: %s=%q",
				ErrInvalidGoNameOverride,
				formatExpandedName(override.Name),
				override.GoName,
			)
		}

		declaration, found := index.LookupType(QName{
			Namespace: override.Name.Namespace,
			Local:     override.Name.Local,
		})
		if !found || declaration.Builtin() {
			return nil, &UnresolvedReferenceError{
				Space:   TypeSymbolSpace,
				Lexical: override.Name.String(),
				Name:    override.Name,
				From:    "Go name override",
			}
		}

		key := expandedName{
			namespace: override.Name.Namespace,
			local:     override.Name.Local,
		}
		if existing, found := result[key]; found {
			return nil, fmt.Errorf(
				"%w: duplicate override for %s (%q and %q)",
				ErrInvalidGoNameOverride,
				formatExpandedName(override.Name),
				existing,
				override.GoName,
			)
		}

		result[key] = override.GoName
	}

	return result, nil
}

func exportedGoIdentifier(value string) bool {
	if !token.IsIdentifier(value) {
		return false
	}

	first, _ := utf8.DecodeRuneInString(value)
	return unicode.IsUpper(first)
}

// Lookup returns the Go name assigned to a user-defined XSD type.
func (n *TypeNames) Lookup(
	declaration *Declaration,
) (string, bool) {
	if n == nil {
		return "", false
	}

	name, found := n.byDeclaration[declaration]
	return name, found
}

// Declarations returns user-defined XSD types in deterministic expanded-name
// order.
func (n *TypeNames) Declarations() []*Declaration {
	if n == nil {
		return nil
	}

	result := make([]*Declaration, len(n.declarations))
	copy(result, n.declarations)

	return result
}

// GoTypeName converts an XSD NCName to an exported Go identifier.
func GoTypeName(value string) (string, error) {
	if !validNCName(value) {
		return "", fmt.Errorf("%w %q", ErrInvalidGoName, value)
	}

	name := joinGoWords(value)
	if name == "" {
		return "", fmt.Errorf("%w %q", ErrInvalidGoName, value)
	}

	first, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(first) {
		name = "X" + name
	}

	if !token.IsIdentifier(name) {
		return "", fmt.Errorf("%w %q from %q", ErrInvalidGoName, name, value)
	}

	return name, nil
}

// GoEnumerationName creates an exported constant name for one enumeration
// value. Prefix must be a valid Go identifier, normally the owning type name.
func GoEnumerationName(
	prefix string,
	value string,
) (string, error) {
	if !token.IsIdentifier(prefix) {
		return "", fmt.Errorf("%w prefix %q", ErrInvalidGoName, prefix)
	}

	suffix := "Empty"
	if value != "" {
		suffix = joinGoWords(value)
		if suffix == "" {
			return "", fmt.Errorf(
				"%w enumeration value %q",
				ErrInvalidGoName,
				value,
			)
		}
	}

	name := prefix + suffix
	if !token.IsIdentifier(name) {
		return "", fmt.Errorf(
			"%w %q from enumeration value %q",
			ErrInvalidGoName,
			name,
			value,
		)
	}

	return name, nil
}

func userTypeDeclarations(index *Index) []*Declaration {
	result := make([]*Declaration, 0, len(index.types))

	for _, declaration := range index.types {
		if declaration.Builtin() {
			continue
		}

		result = append(result, declaration)
	}

	sort.Slice(result, func(left int, right int) bool {
		leftName := result[left].Name
		rightName := result[right].Name

		if leftName.Namespace != rightName.Namespace {
			return leftName.Namespace < rightName.Namespace
		}
		if leftName.Local != rightName.Local {
			return leftName.Local < rightName.Local
		}

		return declarationPath(result[left]) <
			declarationPath(result[right])
	})

	return result
}

func joinGoWords(value string) string {
	words := strings.FieldsFunc(value, func(valueRune rune) bool {
		return !unicode.IsLetter(valueRune) &&
			!unicode.IsDigit(valueRune)
	})

	var result strings.Builder
	for _, word := range words {
		result.WriteString(goWord(word))
	}

	return result.String()
}

func goWord(value string) string {
	if initialism, found := goInitialisms[strings.ToLower(value)]; found {
		return initialism
	}

	first, size := utf8.DecodeRuneInString(value)
	if !unicode.IsLetter(first) {
		return value
	}

	return string(unicode.ToUpper(first)) + value[size:]
}

func describeDeclaration(declaration *Declaration) string {
	return fmt.Sprintf(
		"%s %s from %q",
		declaration.Kind,
		formatExpandedName(declaration.Name),
		declarationPath(declaration),
	)
}

var goInitialisms = map[string]string{
	"api":     "API",
	"ascii":   "ASCII",
	"css":     "CSS",
	"dd":      "DD",
	"html":    "HTML",
	"http":    "HTTP",
	"https":   "HTTPS",
	"id":      "ID",
	"idref":   "IDREF",
	"iso":     "ISO",
	"json":    "JSON",
	"midi":    "MIDI",
	"mm":      "MM",
	"nmtoken": "NMTOKEN",
	"pdf":     "PDF",
	"qname":   "QName",
	"rgb":     "RGB",
	"smufl":   "SMuFL",
	"svg":     "SVG",
	"uri":     "URI",
	"url":     "URL",
	"utf8":    "UTF8",
	"uuid":    "UUID",
	"w3c":     "W3C",
	"xlink":   "XLink",
	"xml":     "XML",
	"xsd":     "XSD",
	"yyyy":    "YYYY",
}
