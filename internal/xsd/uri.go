package xsd

import (
	"net/netip"
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
func ValidXSDAnyURI(value string) bool {
	return validXSDAnyURI(value, nil)
}

func validXSDAnyURI(value string, c *Context) bool {
	c.scalar(int64(len(value)))
	c.bytes(len(value))
	valid := utf8.ValidString(value)
	c.step(0)
	if !valid {
		return false
	}
	c.bytes(len(value))
	fragments := strings.Count(value, "#")
	c.step(0)
	if fragments > 1 {
		return false
	}

	var prepared strings.Builder
	if c != nil && c.run != nil {
		capacity := c.size(int64(len(value)), 3)
		c.scratch(c.builderAllowance(capacity))
		prepared.Grow(int(capacity))
	} else {
		prepared.Grow(len(value))
	}
	const hex = "0123456789ABCDEF"
	for index := 0; index < len(value); index++ {
		c.step(1)
		character := value[index]
		if character == '%' {
			if len(value)-index < 3 || !URIHex(value[index+1]) ||
				!URIHex(value[index+2]) {
				return false
			}
			c.step(2)
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
	if start, end := uriAuthority(reference, c); start >= 0 {
		if !validURIAuthority(reference[start:end], c) {
			return false
		}
		// RFC 2396 permits registry-based authorities, including colons and
		// escaped octets. Go parses every authority as a server host/port,
		// so hide this independently checked component from its URL parser.
		c.scratch(c.sum(int64(start), 1, int64(len(reference)-end)))
		c.bytes(start)
		c.bytes(len(reference) - end)
		reference = reference[:start] + "x" + reference[end:]
		c.step(0)
	}
	c.bytes(len(reference))
	c.scratch(c.sum(512, c.size(int64(len(reference)), 4)))
	parsed, err := url.Parse(reference)
	c.step(0)
	if err != nil {
		return false
	}
	if parsed.Scheme != "" {
		c.bytes(len(reference) - len(parsed.Scheme) - 1)
		body, _, _ := strings.Cut(reference[len(parsed.Scheme)+1:], "#")
		c.step(0)
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
	if start, end := uriAuthority(path, c); start >= 0 {
		path = path[end:]
	}
	c.bytes(len(path))
	if index := strings.IndexAny(path, "?#"); index >= 0 {
		path = path[:index]
	}
	c.step(0)
	c.bytes(len(path))
	valid = !strings.ContainsAny(path, "[]")
	c.step(0)
	return valid
}

// validValidationURIAuthority receives text after XLink escaping and global
// percent-escape checks. Without brackets, the remaining characters all belong
// to RFC 2396's reg_name; the empty authority is permitted by server. Brackets
// instead require RFC 2732's [IPv6address], with optional userinfo and port.
func ValidURIAuthority(authority string) bool {
	return validURIAuthority(authority, nil)
}

func validURIAuthority(authority string, c *Context) bool {
	c.bytes(len(authority))
	bracketed := strings.ContainsAny(authority, "[]")
	c.step(0)
	if !bracketed {
		return true
	}
	host := authority
	c.bytes(len(authority))
	if index := strings.IndexByte(authority, '@'); index >= 0 {
		c.bytes(index)
		if strings.ContainsAny(authority[:index], "[]") {
			return false
		}
		host = authority[index+1:]
	}
	if !strings.HasPrefix(host, "[") {
		return false
	}
	c.bytes(len(host) - 1)
	address, port, closed := strings.Cut(host[1:], "]")
	c.step(0)
	if !closed || !validIPv6(address, c) {
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
		c.step(1)
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
func ValidIPv6(address string) bool {
	return validIPv6(address, nil)
}

func validIPv6(address string, c *Context) bool {
	c.bytes(len(address))
	if !strings.ContainsRune(address, ':') {
		return false
	}
	c.bytes(len(address))
	if strings.ContainsRune(address, '.') {
		c.bytes(len(address))
		colon := strings.LastIndexByte(address, ':')
		c.bytes(len(address) - colon - 1)
		c.scratch(5 * 16)
		octets := strings.SplitN(address[colon+1:], ".", 5)
		c.step(0)
		if len(octets) != 4 {
			return false
		}
		for index, octet := range octets {
			c.step(1)
			if len(octet) == 0 || len(octet) > 3 {
				return false
			}
			for _, digit := range octet {
				c.step(1)
				if digit < '0' || digit > '9' {
					return false
				}
			}
			octets[index] = strings.TrimLeft(octet, "0")
			if octets[index] == "" {
				octets[index] = "0"
			}
		}
		c.bytes(len(address))
		c.scratch(c.size(int64(len(address)), 2))
		address = address[:colon+1] + strings.Join(octets, ".")
		c.step(0)
	}
	// Keep net.ParseIP's address-width and octet-range checks.
	c.bytes(len(address))
	c.scratch(c.sum(128, int64(len(address))))
	parsed, err := netip.ParseAddr(address)
	c.step(0)
	return err == nil && parsed.Is6() && parsed.Zone() == ""
}

func URIAuthority(value string) (start, end int) {
	return uriAuthority(value, nil)
}

func uriAuthority(value string, c *Context) (start, end int) {
	start = 0
	c.bytes(len(value))
	if index := strings.IndexAny(value, ":/?#"); index >= 0 && value[index] == ':' {
		start = index + 1
	}
	if !strings.HasPrefix(value[start:], "//") {
		return -1, -1
	}
	start += 2
	end = len(value)
	c.bytes(len(value) - start)
	if index := strings.IndexAny(value[start:], "/?#"); index >= 0 {
		end = start + index
	}
	c.step(0)
	return start, end
}

func URIHex(value byte) bool {
	return '0' <= value && value <= '9' ||
		'A' <= value && value <= 'F' ||
		'a' <= value && value <= 'f'
}
