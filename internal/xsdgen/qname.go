package xsdgen

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const XMLNamespace = "http://www.w3.org/XML/1998/namespace"

var (
	ErrInvalidQName     = errors.New("xsdgen: invalid QName")
	ErrUndeclaredPrefix = errors.New("xsdgen: undeclared namespace prefix")
)

// NamespaceBindings maps prefixes to namespace names.
//
// The empty prefix contains the default namespace, when one is declared.
type NamespaceBindings map[string]string

// QName is an expanded XML qualified name together with its source prefix.
type QName struct {
	Namespace string
	Local     string
	Prefix    string
}

// String returns the lexical QName used in the source schema.
func (q QName) String() string {
	if q.Prefix == "" {
		return q.Local
	}

	return q.Prefix + ":" + q.Local
}

// XMLName returns the expanded name used by encoding/xml.
func (q QName) XMLName() xml.Name {
	return xml.Name{
		Space: q.Namespace,
		Local: q.Local,
	}
}

// ResolveQName resolves a lexical QName using the schema-root namespace
// bindings.
func (s *Schema) ResolveQName(value string) (QName, error) {
	if s == nil {
		return QName{}, fmt.Errorf("%w: nil schema", ErrInvalidQName)
	}

	return s.Namespaces.ResolveQName(value)
}

// ResolveQNames resolves an XML Schema list of lexical QNames.
func (s *Schema) ResolveQNames(value string) ([]QName, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: nil schema", ErrInvalidQName)
	}

	return s.Namespaces.ResolveQNames(value)
}

// ResolveQName resolves one lexical QName.
func (b NamespaceBindings) ResolveQName(value string) (QName, error) {
	prefix, local, err := splitQName(value)
	if err != nil {
		return QName{}, err
	}

	namespace, found := b[prefix]
	if prefix == "xml" {
		namespace = XMLNamespace
		found = true
	}
	if prefix != "" && !found {
		return QName{}, fmt.Errorf(
			"%w %q in %q",
			ErrUndeclaredPrefix,
			prefix,
			value,
		)
	}

	return QName{
		Namespace: namespace,
		Local:     local,
		Prefix:    prefix,
	}, nil
}

// ResolveQNames resolves one or more whitespace-separated lexical QNames.
func (b NamespaceBindings) ResolveQNames(value string) ([]QName, error) {
	values := strings.Fields(value)
	result := make([]QName, len(values))

	for index, lexical := range values {
		resolved, err := b.ResolveQName(lexical)
		if err != nil {
			return nil, fmt.Errorf(
				"xsdgen: resolve QName at index %d: %w",
				index,
				err,
			)
		}

		result[index] = resolved
	}

	return result, nil
}

func namespaceBindings(start xml.StartElement) NamespaceBindings {
	result := NamespaceBindings{
		"xml": XMLNamespace,
	}

	for _, attribute := range start.Attr {
		switch {
		case attribute.Name.Space == "xmlns":
			result[attribute.Name.Local] = attribute.Value
		case attribute.Name.Space == "" &&
			attribute.Name.Local == "xmlns":
			result[""] = attribute.Value
		}
	}

	return result
}

func splitQName(value string) (string, string, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", "", fmt.Errorf("%w %q", ErrInvalidQName, value)
	}

	index := strings.IndexByte(value, ':')
	if index < 0 {
		if !validNCName(value) {
			return "", "", fmt.Errorf("%w %q", ErrInvalidQName, value)
		}

		return "", value, nil
	}

	if index == 0 ||
		index == len(value)-1 ||
		strings.IndexByte(value[index+1:], ':') >= 0 {
		return "", "", fmt.Errorf("%w %q", ErrInvalidQName, value)
	}

	prefix := value[:index]
	local := value[index+1:]
	if !validNCName(prefix) || !validNCName(local) {
		return "", "", fmt.Errorf("%w %q", ErrInvalidQName, value)
	}

	return prefix, local, nil
}

func validNCName(value string) bool {
	first, size := utf8.DecodeRuneInString(value)
	if first == utf8.RuneError && size == 1 {
		return false
	}
	if !validNCNameStart(first) {
		return false
	}

	for _, valueRune := range value[size:] {
		if !validNCNameCharacter(valueRune) {
			return false
		}
	}

	return true
}

func validNCNameStart(value rune) bool {
	switch {
	case value >= 'A' && value <= 'Z':
		return true
	case value == '_':
		return true
	case value >= 'a' && value <= 'z':
		return true
	case value >= 0xC0 && value <= 0xD6:
		return true
	case value >= 0xD8 && value <= 0xF6:
		return true
	case value >= 0xF8 && value <= 0x2FF:
		return true
	case value >= 0x370 && value <= 0x37D:
		return true
	case value >= 0x37F && value <= 0x1FFF:
		return true
	case value >= 0x200C && value <= 0x200D:
		return true
	case value >= 0x2070 && value <= 0x218F:
		return true
	case value >= 0x2C00 && value <= 0x2FEF:
		return true
	case value >= 0x3001 && value <= 0xD7FF:
		return true
	case value >= 0xF900 && value <= 0xFDCF:
		return true
	case value >= 0xFDF0 && value <= 0xFFFD:
		return true
	case value >= 0x10000 && value <= 0xEFFFF:
		return true
	default:
		return false
	}
}

func validNCNameCharacter(value rune) bool {
	if validNCNameStart(value) {
		return true
	}

	switch {
	case value == '-' || value == '.':
		return true
	case value >= '0' && value <= '9':
		return true
	case value == 0xB7:
		return true
	case value >= 0x0300 && value <= 0x036F:
		return true
	case value >= 0x203F && value <= 0x2040:
		return true
	default:
		return false
	}
}
