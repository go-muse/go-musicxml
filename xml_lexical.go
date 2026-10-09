package musicxml

import (
	"bufio"
	"encoding/xml"
	"io"
)

// lexicalXMLTokenReader retains the two lexical markers that encoding/xml
// otherwise loses when it returns references and CDATA as CharData. It does
// not parse XML or retain token/document bytes. The decoder remains responsible
// for token grammar, and wellFormedXMLTokenReader checks the markers only when
// a CharData token occurs outside all elements.
type lexicalXMLTokenReader struct {
	decoder             *xml.Decoder
	input               *xmlLexicalReader
	characterDataMarkup bool
}

func newLexicalXMLTokenReader(source io.Reader) *lexicalXMLTokenReader {
	input := newXMLLexicalReader(source, 0)
	return &lexicalXMLTokenReader{decoder: xml.NewDecoder(input), input: input}
}

func (r *lexicalXMLTokenReader) Token() (xml.Token, error) {
	start := r.decoder.InputOffset()
	token, err := r.decoder.RawToken()
	r.characterDataMarkup = false
	if _, ok := token.(xml.CharData); ok && err == nil {
		end := r.decoder.InputOffset()
		// InputOffset excludes the decoder's one-byte lookahead. A '<'
		// read ahead after literal text therefore lies at end, outside
		// this half-open token span. A '<' within CharData is CDATA markup;
		// an '&' within it is either a reference or part of that CDATA.
		r.characterDataMarkup = (r.input.angle >= start && r.input.angle < end) ||
			(r.input.ampersand >= start && r.input.ampersand < end)
	}
	return token, err
}

// switchInput installs an observer after charset conversion, using the inner
// decoder's UTF-8 byte offset. The old observer may remain upstream of the
// converter and read ahead; its source-byte offsets must no longer be used.
func (r *lexicalXMLTokenReader) switchInput(source io.Reader) io.Reader {
	r.input = newXMLLexicalReader(source, r.decoder.InputOffset())
	return r.input
}

// Implementing io.ByteReader keeps encoding/xml from buffering ahead of this
// observer. Buffering below it is bounded and does not affect observed offsets.
type xmlLexicalReader struct {
	source    *bufio.Reader
	offset    int64
	angle     int64
	ampersand int64
}

func newXMLLexicalReader(source io.Reader, offset int64) *xmlLexicalReader {
	return &xmlLexicalReader{
		source: bufio.NewReader(source), offset: offset, angle: -1, ampersand: -1,
	}
}

func (r *xmlLexicalReader) observe(value byte) {
	switch value {
	case '<':
		r.angle = r.offset
	case '&':
		r.ampersand = r.offset
	}
	r.offset++
}

func (r *xmlLexicalReader) ReadByte() (byte, error) {
	value, err := r.source.ReadByte()
	if err == nil {
		r.observe(value)
	}
	return value, err
}

func (r *xmlLexicalReader) Read(target []byte) (int, error) {
	n, err := r.source.Read(target)
	for _, value := range target[:n] {
		r.observe(value)
	}
	return n, err
}
