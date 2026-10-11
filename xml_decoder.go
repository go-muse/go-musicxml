package musicxml

import (
	"context"
	"encoding/xml"
	"io"
	"strings"
)

func newXMLDecoder(reader io.Reader) (*xml.Decoder, error) {
	source, err := newSourceXMLTokenReader(reader, sourceXMLConfig{Context: context.Background()})
	if err != nil {
		return nil, err
	}
	return xml.NewTokenDecoder(source), nil
}

func newDepthLimitedXMLDecoder(reader io.Reader, maximum int) (*xml.Decoder, error) {
	source, err := newSourceXMLTokenReader(reader, sourceXMLConfig{Context: context.Background(), MaxDepth: maximum})
	if err != nil {
		return nil, err
	}
	return xml.NewTokenDecoder(source), nil
}

// rawXMLTokenReader forwards lexical tokens to a wrapping xml.Decoder. Passing
// Decoder.Token here would resolve namespace URIs a second time: a URI such as
// "xlink" or "xml" could then be mistaken for a prefix. The wrapping decoder
// still checks matching start/end tags and resolves namespaces normally.
// This compatibility helper remains for the legacy validation parser and tests.
type rawXMLTokenReader struct{ decoder *xml.Decoder }

func (r rawXMLTokenReader) Token() (xml.Token, error) { return r.decoder.RawToken() }

func normalizeCharset(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	return strings.ReplaceAll(value, "_", "")
}
