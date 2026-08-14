package musicxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// Decode reads one uncompressed MusicXML root document.
//
// Supported roots are score-partwise, score-timewise, and opus from the
// no-namespace MusicXML 4.0 schemas. Decode does not call Validate.
func Decode(reader io.Reader) (Document, error) {
	if reader == nil {
		return nil, ErrNilReader
	}

	decoder, err := newXMLDecoder(reader)
	if err != nil {
		return nil, fmt.Errorf(
			"musicxml: initialize XML decoder: %w",
			err,
		)
	}

	start, err := readRoot(decoder)
	if err != nil {
		return nil, err
	}
	if start.Name.Space != "" {
		return nil, &UnsupportedRootError{Name: start.Name}
	}

	var document Document
	switch start.Name.Local {
	case "opus":
		document = &OpusDocument{}
	case "score-partwise":
		document = &ScorePartwise{}
	case "score-timewise":
		document = &ScoreTimewise{}
	default:
		return nil, &UnsupportedRootError{Name: start.Name}
	}

	if err := decoder.DecodeElement(document, &start); err != nil {
		return nil, fmt.Errorf("musicxml: decode document: %w", err)
	}
	if err := readDocumentTail(decoder); err != nil {
		return nil, err
	}

	return document, nil
}

func readRoot(decoder *xml.Decoder) (xml.StartElement, error) {
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return xml.StartElement{}, ErrEmptyDocument
		}
		if err != nil {
			return xml.StartElement{}, fmt.Errorf(
				"musicxml: read document root: %w",
				err,
			)
		}

		switch value := token.(type) {
		case xml.StartElement:
			return value, nil
		case xml.CharData:
			if len(bytes.TrimSpace(value)) == 0 {
				continue
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
			continue
		}

		return xml.StartElement{}, fmt.Errorf(
			"musicxml: unexpected %T before root element",
			token,
		)
	}
}

func readDocumentTail(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf(
				"musicxml: read document tail: %w",
				err,
			)
		}

		switch value := token.(type) {
		case xml.CharData:
			if len(bytes.TrimSpace(value)) == 0 {
				continue
			}

		case xml.Comment, xml.ProcInst:
			continue
		}

		return fmt.Errorf(
			"musicxml: unexpected %T after root element",
			token,
		)
	}
}
