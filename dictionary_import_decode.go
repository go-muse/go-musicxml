package musicxml

import (
	"bytes"
	"context"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

// dictionaryJSON is a fixed-depth scanner for hash-certified corpus bytes, not
// an alternate public JSON ingestion boundary. Its only scratch buffer is fixed;
// returned string bytes borrow that buffer until the next string is scanned.
type dictionaryJSON struct {
	ctx       context.Context
	raw       []byte
	at        int
	err       error
	scratch   [512]byte
	copied    int
	copyLimit int
}

func (c *dictionaryJSON) fail(reason string) {
	if c.err == nil {
		c.err = dictionaryImportError(reason)
	}
}
func (c *dictionaryJSON) space() {
	for c.at < len(c.raw) {
		switch c.raw[c.at] {
		case ' ', '\n', '\r', '\t':
			c.at++
		default:
			return
		}
	}
}
func (c *dictionaryJSON) want(b byte) {
	if c.err != nil {
		return
	}
	c.space()
	if c.at == len(c.raw) || c.raw[c.at] != b {
		c.fail("unexpected certified corpus token")
		return
	}
	c.at++
}
func (c *dictionaryJSON) next(end byte, first *bool) bool {
	if c.err != nil {
		return false
	}
	if err := c.ctx.Err(); err != nil {
		c.err = err
		return false
	}
	c.space()
	if c.at < len(c.raw) && c.raw[c.at] == end {
		c.at++
		return false
	}
	if !*first {
		c.want(',')
		c.space()
		if c.at < len(c.raw) && c.raw[c.at] == end {
			c.fail("trailing corpus separator")
		}
	}
	*first = false
	return c.err == nil
}
func (c *dictionaryJSON) eof() {
	c.space()
	if c.at != len(c.raw) {
		c.fail("trailing corpus bytes")
	}
}
func (c *dictionaryJSON) hex4() rune {
	var out rune
	for i := 0; i < 4; i++ {
		if c.at == len(c.raw) {
			c.fail("truncated JSON escape")
			return 0
		}
		b := c.raw[c.at]
		c.at++
		out <<= 4
		switch {
		case b >= '0' && b <= '9':
			out += rune(b - '0')
		case b >= 'a' && b <= 'f':
			out += rune(b - 'a' + 10)
		case b >= 'A' && b <= 'F':
			out += rune(b - 'A' + 10)
		default:
			c.fail("invalid JSON escape")
			return 0
		}
	}
	return out
}
func (c *dictionaryJSON) str(maximum int) []byte {
	c.want('"')
	if c.err != nil {
		return nil
	}
	n := 0
	for c.at < len(c.raw) && c.err == nil {
		b := c.raw[c.at]
		c.at++
		if b == '"' {
			if !utf8.Valid(c.scratch[:n]) {
				c.fail("invalid UTF-8 corpus string")
			}
			return c.scratch[:n]
		}
		if b < 0x20 {
			c.fail("unescaped JSON control")
			return nil
		}
		if b == '\\' {
			if c.at == len(c.raw) {
				c.fail("truncated JSON escape")
				return nil
			}
			b = c.raw[c.at]
			c.at++
			switch b {
			case '"', '\\', '/':
			case 'b':
				b = '\b'
			case 'f':
				b = '\f'
			case 'n':
				b = '\n'
			case 'r':
				b = '\r'
			case 't':
				b = '\t'
			case 'u':
				r := c.hex4()
				if r >= 0xD800 && r <= 0xDBFF {
					if c.at+2 > len(c.raw) || c.raw[c.at] != '\\' || c.raw[c.at+1] != 'u' {
						c.fail("unpaired JSON surrogate")
						return nil
					}
					c.at += 2
					tail := c.hex4()
					if tail < 0xDC00 || tail > 0xDFFF {
						c.fail("unpaired JSON surrogate")
						return nil
					}
					r = 0x10000 + (r-0xD800)*0x400 + (tail - 0xDC00)
				} else if r >= 0xDC00 && r <= 0xDFFF {
					c.fail("unpaired JSON surrogate")
					return nil
				}
				if c.err != nil {
					return nil
				}
				size := utf8.RuneLen(r)
				if size < 0 || n+size > maximum || n+size > len(c.scratch) {
					c.fail("excessive corpus string")
					return nil
				}
				n += utf8.EncodeRune(c.scratch[n:], r)
				continue
			default:
				c.fail("invalid JSON escape")
				continue
			}
		}
		if n >= maximum || n == len(c.scratch) {
			c.fail("excessive corpus string")
			return nil
		}
		c.scratch[n] = b
		n++
	}
	c.fail("unterminated corpus string")
	return nil
}
func (c *dictionaryJSON) owned(maximum int) string {
	b := c.str(maximum)
	if c.err != nil {
		return ""
	}
	if len(b) > c.copyLimit-c.copied {
		c.fail("excessive copied corpus text")
		return ""
	}
	c.copied += len(b)
	return string(b)
}
func (c *dictionaryJSON) point() uint32 {
	b := c.str(8)
	if c.err != nil {
		return 0
	}
	if len(b) < 6 || len(b) > 8 || b[0] != 'U' || b[1] != '+' || (len(b) > 6 && b[2] == '0') {
		c.fail("noncanonical corpus codepoint")
		return 0
	}
	var out uint32
	for _, v := range b[2:] {
		out <<= 4
		switch {
		case v >= '0' && v <= '9':
			out += uint32(v - '0')
		case v >= 'A' && v <= 'F':
			out += uint32(v - 'A' + 10)
		default:
			c.fail("noncanonical corpus codepoint")
		}
	}
	if out > 0x10FFFF || (out >= 0xD800 && out <= 0xDFFF) {
		c.fail("invalid corpus scalar")
	}
	return out
}
func (c *dictionaryJSON) field(mask *uint32, bit uint32) {
	if *mask&bit != 0 {
		c.fail("duplicate corpus field")
	}
	*mask |= bit
	c.want(':')
}

func decodeDictionaryComponent(ctx context.Context, a *validation.Account, id validation.DictionaryManifestID, raw []byte) (_ *dictionaryImported, err error) {
	if err = dictionaryImportStep(ctx, a, 1); err != nil {
		return nil, err
	}
	manifest, known := validation.LookupDictionaryManifest(id)
	if !known || int64(len(raw)) != manifest.Component.Bytes {
		return nil, dictionaryImportError("decoder requires the exact bounded component length")
	}
	var retained int64
	// Logical bounds include fixed cursor/control/slice headers (2048), generous
	// 64-byte row slots, 32-byte member/string slots, exact copied UTF-8 payload,
	// and 16 bytes of allocation rounding per copied string. Raw storage and the
	// core's simultaneous immutable copy have separate leases.
	switch id {
	case validation.DictionaryManifestSMuFL14Canonical:
		retained = 2048 + 2932*64 + 58476 + 2932*16
	case validation.DictionaryManifestSMuFL14Ranges:
		retained = 2048 + 131*64 + 2932*32 + 61570 + (131+2932)*16
	case validation.DictionaryManifestStandardSounds40:
		retained = 2048 + 894*32 + 15567 + 894*16
	case validation.DictionaryManifestIPAPositive2015Unicode13:
		retained = 2048 + 197*64 + 4233 + 197*3*16
	default:
		return nil, dictionaryImportError("unsupported corpus decoder")
	}
	if _, err = validation.AllocationSize(retained); err != nil {
		return nil, err
	}
	if err = a.Charge(validation.BudgetBufferedBytes, retained); err != nil {
		return nil, err
	}
	out := &dictionaryImported{account: a, retained: retained, data: validation.DictionaryImport{Manifest: id, State: validation.EvidenceKnown}}
	defer func() {
		if err != nil {
			out.close()
		}
	}()
	// This conservative logical envelope reserves six slots per raw byte plus
	// fixed overhead for scanning, UTF-8/escape inspection, copied text, token
	// comparisons and row/object operations. It is not a measured pass count;
	// changing it needs an independently established bound for every decoder.
	// No general tree, decoder buffers or parser recursion are admitted.
	if err = dictionaryImportStep(ctx, a, 6*int64(len(raw))+1024); err != nil {
		return nil, err
	}
	c := dictionaryJSON{ctx: ctx, raw: raw}
	switch id {
	case validation.DictionaryManifestSMuFL14Canonical:
		c.copyLimit = 58476
		out.data.SMuFL, err = decodeDictionaryGlyphs(&c)
	case validation.DictionaryManifestSMuFL14Ranges:
		c.copyLimit = 61570
		out.data.SMuFL, err = decodeDictionaryRanges(&c)
	case validation.DictionaryManifestStandardSounds40:
		out.data.Sounds, err = decodeDictionarySounds(ctx, raw)
	case validation.DictionaryManifestIPAPositive2015Unicode13:
		c.copyLimit = 4233
		out.data.IPA, err = decodeDictionaryIPA(&c)
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeDictionaryGlyphs(c *dictionaryJSON) (*validation.SMuFLDictionaryData, error) {
	out := &validation.SMuFLDictionaryData{Glyphs: make([]validation.SMuFLGlyph, 0, 2932)}
	c.want('{')
	first := true
	previous := ""
	for c.next('}', &first) {
		if len(out.Glyphs) == cap(out.Glyphs) {
			c.fail("excessive glyph count")
			break
		}
		name := c.owned(44)
		if name == "" || name <= previous {
			c.fail("duplicate or out-of-order certified glyph key")
			break
		}
		previous = name
		c.want(':')
		c.want('{')
		fields := true
		mask := uint32(0)
		var point uint32
		for c.next('}', &fields) {
			switch string(c.str(18)) {
			case "codepoint":
				c.field(&mask, 1)
				point = c.point()
			case "description":
				c.field(&mask, 2)
				c.str(97)
			case "alternateCodepoint":
				c.field(&mask, 4)
				c.point()
			default:
				c.fail("unknown glyph field")
			}
		}
		if mask&3 != 3 {
			c.fail("missing glyph fields")
		}
		out.Glyphs = append(out.Glyphs, validation.SMuFLGlyph{Name: name, Codepoint: point})
	}
	c.eof()
	if len(out.Glyphs) != 2932 {
		c.fail("incomplete glyph count")
	}
	return out, c.err
}
func decodeDictionaryRanges(c *dictionaryJSON) (*validation.SMuFLDictionaryData, error) {
	out := &validation.SMuFLDictionaryData{Ranges: make([]validation.SMuFLRange, 0, 131)}
	members := make([]string, 0, 2932)
	c.want('{')
	first := true
	previous := ""
	for c.next('}', &first) {
		if len(out.Ranges) == cap(out.Ranges) {
			c.fail("excessive range count")
			break
		}
		id := c.owned(63)
		if id == "" || id <= previous {
			c.fail("duplicate or out-of-order certified range key")
			break
		}
		previous = id
		c.want(':')
		c.want('{')
		fields := true
		mask := uint32(0)
		startMembers := len(members)
		r := validation.SMuFLRange{ID: id}
		for c.next('}', &fields) {
			switch string(c.str(11)) {
			case "description":
				c.field(&mask, 1)
				c.str(72)
			case "range_start":
				c.field(&mask, 2)
				r.Start = c.point()
			case "range_end":
				c.field(&mask, 4)
				r.End = c.point()
			case "glyphs":
				c.field(&mask, 8)
				c.want('[')
				names := true
				for c.next(']', &names) {
					if len(members) == cap(members) || len(members)-startMembers == 128 {
						c.fail("excessive range members")
						break
					}
					members = append(members, c.owned(44))
				}
			default:
				c.fail("unknown range field")
			}
		}
		if mask != 15 || r.Start > r.End || len(members) == startMembers {
			c.fail("incomplete or inverted range")
		}
		r.Names = members[startMembers:len(members):len(members)]
		out.Ranges = append(out.Ranges, r)
	}
	c.eof()
	if len(out.Ranges) != 131 || len(members) != 2932 {
		c.fail("incomplete range counts")
	}
	return out, c.err
}

func decodeDictionarySounds(ctx context.Context, raw []byte) (*validation.StandardSoundsDictionaryData, error) {
	const prefixBytes = 1139
	const declaration = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\r\n<!DOCTYPE sounds PUBLIC \"-//Recordare//DTD MusicXML 4.0 Sounds//EN\" \"http://www.musicxml.org/dtds/sounds.dtd\">\r\n"
	// The original, hash-certified prolog/comments remain in raw unchanged. This
	// skips only their reviewed literal prefix; it neither validates XML/DTD nor
	// consults a resolver. No entity/reference or alternate XML spelling is read.
	if len(raw) < prefixBytes || !bytes.HasPrefix(raw, []byte(declaration)) || !bytes.HasSuffix(raw[:prefixBytes], []byte("<sounds>\r\n")) {
		return nil, dictionaryImportError("unexpected certified sounds prefix")
	}
	out := &validation.StandardSoundsDictionaryData{IDs: make([]string, 0, 894)}
	at := prefixBytes
	copied := 0
	for len(out.IDs) < 894 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		const start = "   <sound id=\""
		if !bytes.HasPrefix(raw[at:], []byte(start)) {
			return nil, dictionaryImportError("unexpected literal sound record")
		}
		at += len(start)
		begin := at
		for at < len(raw) && raw[at] != '"' {
			b := raw[at]
			if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '.' || b == '-') {
				return nil, dictionaryImportError("nonliteral sound identifier")
			}
			at++
			if at-begin > 39 {
				return nil, dictionaryImportError("excessive sound identifier")
			}
		}
		if at == begin || !bytes.HasPrefix(raw[at:], []byte("\"/>\r\n")) {
			return nil, dictionaryImportError("unexpected sound terminator")
		}
		copied += at - begin
		if copied > 15567 {
			return nil, dictionaryImportError("excessive sound identifier bytes")
		}
		out.IDs = append(out.IDs, string(raw[begin:at]))
		at += 5
	}
	if !bytes.Equal(raw[at:], []byte("</sounds>\r\n")) || copied != 15567 {
		return nil, dictionaryImportError("incomplete sound corpus")
	}
	return out, nil
}
