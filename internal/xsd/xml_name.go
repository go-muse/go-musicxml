package xsd

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
	XSDNamePattern    = regexp.MustCompile(`^[` + XMLNameStartCharacters + `][` + XMLNameCharacters + `]*$`)
	XSDNMTOKENPattern = regexp.MustCompile(`^[` + XMLNameCharacters + `]+$`)
)

func ValidXSDName(value string) bool {
	return utf8.ValidString(value) && XSDNamePattern.MatchString(value)
}

func ValidXSDNCName(value string) bool {
	return !strings.ContainsRune(value, ':') && ValidXSDName(value)
}

func ValidXSDNMTOKEN(value string) bool {
	return utf8.ValidString(value) && XSDNMTOKENPattern.MatchString(value)
}

// This checks the QName lexical production only. Namespace bindings and type
// resolution are separate context-dependent operations.
func ValidXSDQName(value string) bool {
	prefix, local, qualified := strings.Cut(value, ":")
	if !qualified {
		return ValidXSDNCName(value)
	}
	return ValidXSDNCName(prefix) && ValidXSDNCName(local)
}
