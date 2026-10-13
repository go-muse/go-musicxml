package musicxml

// These spans are lexical fast paths in the same cursor, not a second scanner.
// Each is at most one already-accounted refill. Delimiters, non-ASCII, CR and
// invalid XML controls remain with the authoritative rune/normalization path.
type sourceASCIIMode uint8

const (
	sourceASCIIName sourceASCIIMode = iota
	sourceASCIIText
	sourceASCIIQuoted
	sourceASCIISpace
)

func (c *sourceXMLCursor) asciiSpan(mode sourceASCIIMode, quote byte, maximum int) []byte {
	if !c.initialized || c.hasCached || c.hasRawCached || c.rawErr != nil || c.prefixAt < c.prefixN || c.wide {
		return nil
	}
	data := c.data[c.at:c.n]
	if len(data) > maximum {
		data = data[:maximum]
	}
	i := 0
	for ; i < len(data); i++ {
		b := data[i]
		allowed := false
		switch mode {
		case sourceASCIIName:
			allowed = b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == ':' || b == '-' || b == '.'
		case sourceASCIIText:
			allowed = (b >= 32 && b < 128 || b == '\t' || b == '\n') && b != '<' && b != '&' && b != ']' && b != '>'
		case sourceASCIIQuoted:
			allowed = b >= 32 && b < 128 && b != quote && b != '&' && b != '<'
		case sourceASCIISpace:
			allowed = b == ' ' || b == '\t' || b == '\n'
		}
		if !allowed {
			break
		}
	}
	return data[:i]
}

func (c *sourceXMLCursor) consumeASCII(data []byte) {
	c.at += len(data)
	c.rawOffset += int64(len(data))
	c.position.ByteOffset = c.rawOffset
	for _, b := range data {
		c.position.advanceCoordinates(rune(b))
	}
}

func (r *sourceXMLTokenReader) appendASCII(b *sourceXMLBuilder, mode sourceASCIIMode, quote byte) (bool, error) {
	if err := r.cursor.contextErr(); err != nil {
		return false, err
	}
	maximum := sourceXMLCursorBytes
	if r.account != nil {
		remaining := r.account.Limits().MaxScalarBytes - int64(len(b.data))
		if remaining < int64(maximum) {
			maximum = int(remaining) + 1
		}
	}
	span := r.asciiSpan(mode, quote, maximum)
	if len(span) == 0 {
		return false, nil
	}
	// Admit storage and scalar length before copying or advancing source position.
	if err := b.grow(int64(len(b.data)) + int64(len(span))); err != nil {
		return false, err
	}
	b.data = append(b.data, span...)
	r.cursor.consumeASCII(span)
	return true, nil
}

// The physical fast path cannot consume caller bytes under a replacement frame.
func (r *sourceXMLTokenReader) asciiSpan(mode sourceASCIIMode, quote byte, maximum int) []byte {
	if r.entity != nil {
		return nil
	}
	return r.cursor.asciiSpan(mode, quote, maximum)
}
