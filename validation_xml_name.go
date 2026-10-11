package musicxml

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// XSD 1.0 datatype names use XML 1.0 Second Edition's productions, just like
// the XSD pattern escapes \i and \c. These predicates validate scalar values;
// they do not define the repertoire accepted by the XML document reader.
// https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#Name
var (
	validationXSDNamePattern    = regexp.MustCompile(`^[` + validationXMLNameStartCharacters + `][` + validationXMLNameCharacters + `]*$`)
	validationXSDNMTOKENPattern = regexp.MustCompile(`^[` + validationXMLNameCharacters + `]+$`)
)

func validXSDName(value string) bool {
	return utf8.ValidString(value) && validationXSDNamePattern.MatchString(value)
}

func validXSDNCName(value string) bool {
	return !strings.ContainsRune(value, ':') && validXSDName(value)
}

func validXSDNMTOKEN(value string) bool {
	return utf8.ValidString(value) && validationXSDNMTOKENPattern.MatchString(value)
}

// This checks the QName lexical production only. Namespace bindings and type
// resolution are separate context-dependent operations.
func validXSDQName(value string) bool {
	prefix, local, qualified := strings.Cut(value, ":")
	if !qualified {
		return validXSDNCName(value)
	}
	return validXSDNCName(prefix) && validXSDNCName(local)
}
