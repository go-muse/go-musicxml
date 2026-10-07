package musicxml

import (
	"net"
	"net/url"
	"strings"
	"unicode/utf8"
)

// validXSDAnyURI checks the lexical space of xs:anyURI, after whiteSpace
// normalization. XSD 1.0 Part 2, section 3.2.17, applies the escaping procedure
// from XLink 1.0 section 5.4 before checking the URI reference. net/url alone
// both rejects some legal escaped authorities and leaves query/opaque escapes
// and fragment delimiters unchecked.
// https://www.w3.org/TR/xmlschema-2/#anyURI
// https://www.w3.org/TR/2001/REC-xlink-20010627/#link-locators
func validXSDAnyURI(value string) bool {
	if !utf8.ValidString(value) || strings.Count(value, "#") > 1 {
		return false
	}

	var prepared strings.Builder
	prepared.Grow(len(value))
	const hex = "0123456789ABCDEF"
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '%' {
			if index+2 >= len(value) || !validationURIHex(value[index+1]) ||
				!validationURIHex(value[index+2]) {
				return false
			}
			prepared.WriteString(value[index : index+3])
			index += 2
		} else if character <= 0x20 || character >= 0x7f ||
			strings.ContainsRune(`<>"{}|\^`+"`", rune(character)) {
			prepared.WriteByte('%')
			prepared.WriteByte(hex[character>>4])
			prepared.WriteByte(hex[character&15])
		} else {
			prepared.WriteByte(character)
		}
	}

	reference := prepared.String()
	if start, end := validationURIAuthority(reference); start >= 0 {
		if !validValidationURIAuthority(reference[start:end]) {
			return false
		}
		// RFC 2396 permits registry-based authorities, including colons and
		// escaped octets. Go parses every authority as a server host/port,
		// so hide this independently checked component from its URL parser.
		reference = reference[:start] + "x" + reference[end:]
	}
	parsed, err := url.Parse(reference)
	if err != nil {
		return false
	}
	if parsed.Scheme != "" {
		body, _, _ := strings.Cut(reference[len(parsed.Scheme)+1:], "#")
		if body == "" {
			return false
		}
		if body[0] != '/' {
			// opaque_part = uric_no_slash *uric. RFC 2732 adds brackets
			// to uric via reserved, but not to the first-character rule.
			// A leading '?' is also opaque, despite Go splitting it as a query.
			return body[0] != '[' && body[0] != ']'
		}
	}

	// RFC 2396's path-segment grammar excludes literal square brackets.
	// RFC 2732 permits them in an IPv6 authority, not in a hierarchical path.
	// Inspect the lexical path, since parsed.Path has decoded %5B and %5D.
	path := reference
	if parsed.Scheme != "" {
		path = path[len(parsed.Scheme)+1:]
	}
	if start, end := validationURIAuthority(path); start >= 0 {
		path = path[end:]
	}
	if index := strings.IndexAny(path, "?#"); index >= 0 {
		path = path[:index]
	}
	return !strings.ContainsAny(path, "[]")
}

// validValidationURIAuthority receives text after XLink escaping and global
// percent-escape checks. Without brackets, the remaining characters all belong
// to RFC 2396's reg_name; the empty authority is permitted by server. Brackets
// instead require RFC 2732's [IPv6address], with optional userinfo and port.
func validValidationURIAuthority(authority string) bool {
	if !strings.ContainsAny(authority, "[]") {
		return true
	}
	host := authority
	if index := strings.IndexByte(authority, '@'); index >= 0 {
		if strings.ContainsAny(authority[:index], "[]") {
			return false
		}
		host = authority[index+1:]
	}
	if !strings.HasPrefix(host, "[") {
		return false
	}
	address, port, closed := strings.Cut(host[1:], "]")
	if !closed || !validValidationIPv6(address) {
		return false
	}
	if port == "" {
		return true
	}
	if port[0] != ':' {
		return false
	}
	// RFC 2396 port is *digit, so an explicitly empty port is legal.
	for _, digit := range port[1:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// validValidationIPv6 checks RFC 2373's IPv6 representation, which RFC 2732
// incorporates. Its embedded IPv4 octets permit 1-3 decimal digits, including
// leading zeros. net.ParseIP deliberately rejects those zeros, so remove them
// from a checked temporary representation without changing the original URI.
func validValidationIPv6(address string) bool {
	if !strings.ContainsRune(address, ':') {
		return false
	}
	if strings.ContainsRune(address, '.') {
		colon := strings.LastIndexByte(address, ':')
		octets := strings.SplitN(address[colon+1:], ".", 5)
		if len(octets) != 4 {
			return false
		}
		for index, octet := range octets {
			if len(octet) == 0 || len(octet) > 3 {
				return false
			}
			for _, digit := range octet {
				if digit < '0' || digit > '9' {
					return false
				}
			}
			octets[index] = strings.TrimLeft(octet, "0")
			if octets[index] == "" {
				octets[index] = "0"
			}
		}
		address = address[:colon+1] + strings.Join(octets, ".")
	}
	// Keep net.ParseIP's address-width and octet-range checks.
	return net.ParseIP(address) != nil
}

func validationURIAuthority(value string) (start, end int) {
	start = 0
	if index := strings.IndexAny(value, ":/?#"); index >= 0 && value[index] == ':' {
		start = index + 1
	}
	if !strings.HasPrefix(value[start:], "//") {
		return -1, -1
	}
	start += 2
	end = len(value)
	if index := strings.IndexAny(value[start:], "/?#"); index >= 0 {
		end = start + index
	}
	return start, end
}

func validationURIHex(value byte) bool {
	return '0' <= value && value <= '9' ||
		'A' <= value && value <= 'F' ||
		'a' <= value && value <= 'f'
}
