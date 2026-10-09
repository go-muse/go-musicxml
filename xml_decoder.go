package musicxml

import (
	"bufio"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func newXMLDecoder(reader io.Reader) (*xml.Decoder, error) {
	buffered := bufio.NewReader(reader)
	order, hasBOM, err := detectUTF16(buffered)
	if err != nil {
		return nil, err
	}

	var source io.Reader = buffered
	if order != nil {
		source = &utf16Reader{
			source: buffered,
			order:  order,
		}
	}

	lexical := newLexicalXMLTokenReader(source)
	decoder := lexical.decoder
	decoder.CharsetReader = func(
		charset string,
		input io.Reader,
	) (io.Reader, error) {
		if order != nil {
			// The input has already been transcoded from UTF-16. The byte
			// order mark, or the declaration check in the token reader for
			// documents without one, settles the encoding; the declared
			// charset is not applied a second time.
			return input, nil
		}
		switch normalizeCharset(charset) {
		case "iso88591", "latin1":
			return lexical.switchInput(&latin1Reader{
				source: bufio.NewReader(input),
			}), nil
		}

		return nil, fmt.Errorf(
			"musicxml: unsupported XML encoding %q",
			charset,
		)
	}

	var tokens xml.TokenReader = lexical
	// A byte order mark settles the encoding, whatever the declaration
	// says. Without one, the declaration is the only evidence of the byte
	// order and must agree with the detected one.
	if order != nil && !hasBOM {
		tokens = &utf16XMLTokenReader{
			source: tokens,
			order:  order,
		}
	}

	return xml.NewTokenDecoder(&wellFormedXMLTokenReader{
		source:              tokens,
		position:            decoder.InputPos,
		characterDataMarkup: func() bool { return lexical.characterDataMarkup },
	}), nil
}

// utf16XMLTokenReader checks the XML declaration of a UTF-16 document that
// has no byte order mark: the declaration must name the detected byte order.
type utf16XMLTokenReader struct {
	source    xml.TokenReader
	order     binary.ByteOrder
	readToken bool
}

func (r *utf16XMLTokenReader) Token() (xml.Token, error) {
	token, err := r.source.Token()
	if err != nil {
		return nil, err
	}
	if r.readToken {
		return token, nil
	}
	r.readToken = true

	instruction, ok := token.(xml.ProcInst)
	if !ok || instruction.Target != "xml" {
		return nil, fmt.Errorf("musicxml: UTF-16 XML without a byte order mark requires an encoding declaration")
	}

	// XML declaration pseudo-attributes have the same quoting and whitespace
	// syntax as attributes. Parse them separately because encoding/xml does
	// not expose the declared encoding and can skip CharsetReader.
	var declaration struct {
		Encoding string `xml:"encoding,attr"`
	}
	if err := xml.Unmarshal(
		[]byte("<xml "+string(instruction.Inst)+"/>"),
		&declaration,
	); err != nil {
		return nil, fmt.Errorf("musicxml: invalid UTF-16 XML declaration: %w", err)
	}

	switch normalizeCharset(declaration.Encoding) {
	case "utf16be":
		if r.order == binary.BigEndian {
			return token, nil
		}
	case "utf16le":
		if r.order == binary.LittleEndian {
			return token, nil
		}
	case "", "utf16":
		return nil, fmt.Errorf("musicxml: UTF-16 XML without a byte order mark requires an explicit UTF-16BE or UTF-16LE declaration")
	}

	return nil, fmt.Errorf(
		"musicxml: XML encoding %q does not match detected UTF-16 byte order %s",
		declaration.Encoding,
		r.order,
	)
}

func newDepthLimitedXMLDecoder(
	reader io.Reader,
	maximum int,
) (*xml.Decoder, error) {
	source, err := newXMLDecoder(reader)
	if err != nil {
		return nil, err
	}

	return xml.NewTokenDecoder(&depthLimitedXMLTokenReader{
		source:  rawXMLTokenReader{source},
		maximum: maximum,
	}), nil
}

// rawXMLTokenReader forwards lexical tokens to a wrapping xml.Decoder. Passing
// Decoder.Token here would resolve namespace URIs a second time: a URI such as
// "xlink" or "xml" could then be mistaken for a prefix. The wrapping decoder
// still checks matching start/end tags and resolves namespaces normally.
type rawXMLTokenReader struct {
	decoder *xml.Decoder
}

func (r rawXMLTokenReader) Token() (xml.Token, error) {
	return r.decoder.RawToken()
}

type depthLimitedXMLTokenReader struct {
	source  xml.TokenReader
	depth   int
	maximum int
}

func (r *depthLimitedXMLTokenReader) Token() (xml.Token, error) {
	token, err := r.source.Token()
	if err != nil {
		return nil, err
	}

	switch token.(type) {
	case xml.StartElement:
		if r.depth >= r.maximum {
			return nil, fmt.Errorf(
				"%w: maximum is %d elements",
				ErrXMLTooDeep,
				r.maximum,
			)
		}
		r.depth++

	case xml.EndElement:
		r.depth--
	}

	return token, nil
}

type latin1Reader struct {
	source  *bufio.Reader
	pending []byte
}

func (r *latin1Reader) Read(target []byte) (int, error) {
	if len(target) == 0 {
		return 0, nil
	}

	written := copy(target, r.pending)
	r.pending = r.pending[written:]
	if written == len(target) {
		return written, nil
	}

	for written < len(target) {
		value, err := r.source.ReadByte()
		if err != nil {
			return written, err
		}

		if value < utf8.RuneSelf {
			target[written] = value
			written++
			continue
		}

		var encoded [utf8.UTFMax]byte
		size := utf8.EncodeRune(encoded[:], rune(value))
		copied := copy(target[written:], encoded[:size])
		written += copied
		if copied < size {
			r.pending = append(r.pending, encoded[copied:size]...)
		}
	}

	return written, nil
}

func detectUTF16(
	reader *bufio.Reader,
) (binary.ByteOrder, bool, error) {
	prefix, err := reader.Peek(4)
	if len(prefix) >= 3 &&
		prefix[0] == 0xef &&
		prefix[1] == 0xbb &&
		prefix[2] == 0xbf {
		if _, err := reader.Discard(3); err != nil {
			return nil, false, err
		}

		return nil, false, nil
	}
	if err != nil && len(prefix) < 2 {
		return nil, false, nil
	}

	var order binary.ByteOrder
	switch {
	case prefix[0] == 0xfe && prefix[1] == 0xff:
		order = binary.BigEndian
	case prefix[0] == 0xff && prefix[1] == 0xfe:
		order = binary.LittleEndian
	default:
		// A BOM-less UTF-16 document must begin with an XML declaration
		// identifying its byte order. Do not guess from a bare root tag.
		if len(prefix) >= 4 {
			switch {
			case prefix[0] == 0 && prefix[1] == '<' &&
				prefix[2] == 0 && prefix[3] == '?':
				return binary.BigEndian, false, nil
			case prefix[0] == '<' && prefix[1] == 0 &&
				prefix[2] == '?' && prefix[3] == 0:
				return binary.LittleEndian, false, nil
			}
		}
		return nil, false, nil
	}

	if _, err := reader.Discard(2); err != nil {
		return nil, false, err
	}

	return order, true, nil
}

func normalizeCharset(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")

	return value
}

type utf16Reader struct {
	source  io.Reader
	order   binary.ByteOrder
	pending []byte
}

func (r *utf16Reader) Read(target []byte) (int, error) {
	if len(target) == 0 {
		return 0, nil
	}

	written := copy(target, r.pending)
	r.pending = r.pending[written:]
	if written == len(target) {
		return written, nil
	}

	for written < len(target) {
		value, err := r.readRune()
		if err != nil {
			return written, err
		}

		var encoded [utf8.UTFMax]byte
		size := utf8.EncodeRune(encoded[:], value)
		copied := copy(target[written:], encoded[:size])
		written += copied
		if copied < size {
			r.pending = append(r.pending, encoded[copied:size]...)
		}
	}

	return written, nil
}

func (r *utf16Reader) readRune() (rune, error) {
	first, err := r.readCodeUnit()
	if err != nil {
		return 0, err
	}

	switch {
	case 0xd800 <= first && first <= 0xdbff:
		second, err := r.readCodeUnit()
		if err != nil {
			if err == io.EOF {
				return 0, io.ErrUnexpectedEOF
			}

			return 0, err
		}
		if second < 0xdc00 || 0xdfff < second {
			return 0, fmt.Errorf(
				"musicxml: invalid UTF-16 surrogate pair %04X %04X",
				first,
				second,
			)
		}

		return utf16.DecodeRune(rune(first), rune(second)), nil

	case 0xdc00 <= first && first <= 0xdfff:
		return 0, fmt.Errorf(
			"musicxml: unexpected UTF-16 low surrogate %04X",
			first,
		)

	default:
		return rune(first), nil
	}
}

func (r *utf16Reader) readCodeUnit() (uint16, error) {
	var buffer [2]byte
	read, err := io.ReadFull(r.source, buffer[:])
	if err != nil {
		if err == io.ErrUnexpectedEOF && read == 0 {
			return 0, io.EOF
		}

		return 0, err
	}

	return r.order.Uint16(buffer[:]), nil
}
