package xsdgen

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

var (
	ErrNilReader       = errors.New("xsdgen: nil reader")
	ErrTrailingContent = errors.New("xsdgen: content after schema")
)

func Parse(reader io.Reader) (*Schema, error) {
	if reader == nil {
		return nil, ErrNilReader
	}

	decoder := xml.NewDecoder(reader)

	var schema Schema
	if err := decoder.Decode(&schema); err != nil {
		return nil, fmt.Errorf("xsdgen: decode schema: %w", err)
	}

	if err := readTail(decoder); err != nil {
		return nil, err
	}

	return &schema, nil
}

func readTail(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("xsdgen: read schema tail: %w", err)
		}

		switch value := token.(type) {
		case xml.CharData:
			if len(bytes.TrimSpace(value)) == 0 {
				continue
			}
		case xml.Comment, xml.ProcInst:
			continue
		}

		return fmt.Errorf("%w: %T", ErrTrailingContent, token)
	}
}
