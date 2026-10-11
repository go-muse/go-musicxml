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
	return validXSDName(value, nil)
}

func validXSDName(value string, c *Context) bool {
	c.scalar(int64(len(value)))
	c.bytes(len(value))
	valid := utf8.ValidString(value)
	c.step(0)
	return valid && scalarPatternMatch(XSDNamePattern, value, c)
}

func ValidXSDNCName(value string) bool {
	return validXSDNCName(value, nil)
}

func validXSDNCName(value string, c *Context) bool {
	c.scalar(int64(len(value)))
	c.bytes(len(value))
	qualified := strings.ContainsRune(value, ':')
	c.step(0)
	return !qualified && validXSDName(value, c)
}

func ValidXSDNMTOKEN(value string) bool {
	return validXSDNMTOKEN(value, nil)
}

func validXSDNMTOKEN(value string, c *Context) bool {
	c.scalar(int64(len(value)))
	c.bytes(len(value))
	valid := utf8.ValidString(value)
	c.step(0)
	return valid && scalarPatternMatch(XSDNMTOKENPattern, value, c)
}

// This checks the QName lexical production only. Namespace bindings and type
// resolution are separate context-dependent operations.
func ValidXSDQName(value string) bool {
	return validXSDQName(value, nil)
}

func validXSDQName(value string, c *Context) bool {
	c.scalar(int64(len(value)))
	c.bytes(len(value))
	prefix, local, qualified := strings.Cut(value, ":")
	c.step(0)
	if !qualified {
		return validXSDNCName(value, c)
	}
	return validXSDNCName(prefix, c) && validXSDNCName(local, c)
}

// The regexp VM cannot be interrupted internally. Bound its lexical work and
// temporary workspace before entry, then observe cancellation after it returns.
func scalarPatternMatch(pattern *regexp.Regexp, value string, c *Context) bool {
	scalarPatternPrepare(pattern, value, c)
	matched := pattern.MatchString(value)
	c.step(0)
	return matched
}
func scalarPatternSubmatch(pattern *regexp.Regexp, value string, c *Context) []string {
	scalarPatternPrepare(pattern, value, c)
	c.scratch(c.size(c.sum(int64(pattern.NumSubexp()), 1), 32))
	match := pattern.FindStringSubmatch(value)
	c.step(0)
	return match
}
func scalarPatternPrepare(pattern *regexp.Regexp, value string, c *Context) {
	c.scalar(int64(len(value)))
	c.bytes(len(value))
	c.scratch(c.sum(256, c.size(int64(len(value)), 16)))
}
