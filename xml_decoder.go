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

func newDepthLimitedXMLDecoder(ctx context.Context, reader io.Reader, maximum int) (*xml.Decoder, error) {
	source, err := newSourceXMLTokenReader(reader, sourceXMLConfig{Context: ctx, MaxDepth: maximum})
	if err != nil {
		return nil, err
	}
	return xml.NewTokenDecoder(source), nil
}

func normalizeCharset(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	return strings.ReplaceAll(value, "_", "")
}
