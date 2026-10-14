package xsd

// binaryLexicalLength checks the XSD 1.0 Second Edition binary lexical spaces
// and counts decoded octets without allocating decoded storage. Whitespace has
// already been normalized for the primitive; only XML whitespace is admitted.
// https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#base64Binary
func binaryLexicalLength(builtin, value string, c *Context) (uint64, bool) {
	c.scalar(int64(len(value)))
	if builtin == "hexBinary" {
		for i := range len(value) {
			c.step(1)
			b := value[i]
			if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F') {
				return 0, false
			}
		}
		return uint64(len(value) / 2), len(value)%2 == 0
	}
	var quartet [4]byte
	used := 0
	var length uint64
	ended := false
	for i := range len(value) {
		c.step(1)
		b := value[i]
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		if ended {
			return 0, false
		}
		var digit byte
		switch {
		case b >= 'A' && b <= 'Z':
			digit = b - 'A'
		case b >= 'a' && b <= 'z':
			digit = b - 'a' + 26
		case b >= '0' && b <= '9':
			digit = b - '0' + 52
		case b == '+':
			digit = 62
		case b == '/':
			digit = 63
		case b == '=':
			digit = 64
		default:
			return 0, false
		}
		quartet[used] = digit
		used++
		if used != 4 {
			continue
		}
		if quartet[0] == 64 || quartet[1] == 64 {
			return 0, false
		}
		switch {
		case quartet[2] == 64:
			if quartet[3] != 64 || quartet[1]&15 != 0 {
				return 0, false
			}
			length++
			ended = true
		case quartet[3] == 64:
			if quartet[2]&3 != 0 {
				return 0, false
			}
			length += 2
			ended = true
		default:
			length += 3
		}
		used = 0
	}
	c.step(0)
	return length, used == 0
}
