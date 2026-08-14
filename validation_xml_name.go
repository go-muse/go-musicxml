package musicxml

import (
	"strings"
	"unicode/utf8"
)

func validXMLName(value string) bool {
	return validXMLNameValue(value, true)
}

func validXMLNCName(value string) bool {
	return validXMLNameValue(value, false)
}

func validXMLNameValue(value string, allowColon bool) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}

	for index, character := range value {
		if character == ':' && !allowColon {
			return false
		}
		if index == 0 {
			if !xmlNameStartCharacter(character) {
				return false
			}
			continue
		}
		if !xmlNameCharacter(character) {
			return false
		}
	}

	return true
}

func validXMLNMTOKEN(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !xmlNameCharacter(character) {
			return false
		}
	}

	return true
}

func validXMLQName(value string) bool {
	prefix, local, qualified := strings.Cut(value, ":")
	if !qualified {
		return validXMLNCName(value)
	}

	return validXMLNCName(prefix) && validXMLNCName(local)
}

func xmlNameStartCharacter(value rune) bool {
	return value == ':' ||
		value == '_' ||
		'A' <= value && value <= 'Z' ||
		'a' <= value && value <= 'z' ||
		0xC0 <= value && value <= 0xD6 ||
		0xD8 <= value && value <= 0xF6 ||
		0xF8 <= value && value <= 0x2FF ||
		0x370 <= value && value <= 0x37D ||
		0x37F <= value && value <= 0x1FFF ||
		0x200C <= value && value <= 0x200D ||
		0x2070 <= value && value <= 0x218F ||
		0x2C00 <= value && value <= 0x2FEF ||
		0x3001 <= value && value <= 0xD7FF ||
		0xF900 <= value && value <= 0xFDCF ||
		0xFDF0 <= value && value <= 0xFFFD ||
		0x10000 <= value && value <= 0xEFFFF
}

func xmlNameCharacter(value rune) bool {
	return xmlNameStartCharacter(value) ||
		value == '-' ||
		value == '.' ||
		'0' <= value && value <= '9' ||
		value == 0xB7 ||
		0x300 <= value && value <= 0x36F ||
		0x203F <= value && value <= 0x2040
}
