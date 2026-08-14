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
	order, utf16Encoded, err := detectUTF16(buffered)
	if err != nil {
		return nil, err
	}

	var source io.Reader = buffered
	if utf16Encoded {
		source = &utf16Reader{
			source: buffered,
			order:  order,
		}
	}

	decoder := xml.NewDecoder(source)
	decoder.CharsetReader = func(
		charset string,
		input io.Reader,
	) (io.Reader, error) {
		switch normalizeCharset(charset) {
		case "utf16", "utf16be", "utf16le":
			if utf16Encoded {
				return input, nil
			}

			switch normalizeCharset(charset) {
			case "utf16be":
				return &utf16Reader{
					source: input,
					order:  binary.BigEndian,
				}, nil

			case "utf16le":
				return &utf16Reader{
					source: input,
					order:  binary.LittleEndian,
				}, nil
			}

		case "iso88591", "latin1":
			return &latin1Reader{
				source: bufio.NewReader(input),
			}, nil
		}

		return nil, fmt.Errorf(
			"musicxml: unsupported XML encoding %q",
			charset,
		)
	}

	return decoder, nil
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
		source:  source,
		maximum: maximum,
	}), nil
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
	prefix, err := reader.Peek(2)
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
