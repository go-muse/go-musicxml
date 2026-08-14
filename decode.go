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
	decoder, start, err := newDocumentDecoder(reader)
	if err != nil {
		return nil, err
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

	if err := decodeRootElement(decoder, start, document); err != nil {
		return nil, err
	}

	return document, nil
}

// DecodeScorePartwise reads one uncompressed score-partwise document.
//
// It returns ErrUnsupportedRoot when the input contains another root type and
// does not call Validate.
func DecodeScorePartwise(reader io.Reader) (*ScorePartwise, error) {
	document := &ScorePartwise{}
	if err := decodeExpectedDocument(
		reader,
		"score-partwise",
		document,
	); err != nil {
		return nil, err
	}

	return document, nil
}

// DecodeScoreTimewise reads one uncompressed score-timewise document.
//
// It returns ErrUnsupportedRoot when the input contains another root type and
// does not call Validate.
func DecodeScoreTimewise(reader io.Reader) (*ScoreTimewise, error) {
	document := &ScoreTimewise{}
	if err := decodeExpectedDocument(
		reader,
		"score-timewise",
		document,
	); err != nil {
		return nil, err
	}

	return document, nil
}

// DecodeOpusDocument reads one uncompressed opus document.
//
// It returns ErrUnsupportedRoot when the input contains another root type and
// does not call Validate.
func DecodeOpusDocument(reader io.Reader) (*OpusDocument, error) {
	document := &OpusDocument{}
	if err := decodeExpectedDocument(reader, "opus", document); err != nil {
		return nil, err
	}

	return document, nil
}

func decodeExpectedDocument(
	reader io.Reader,
	expectedRoot string,
	document Document,
) error {
	decoder, start, err := newDocumentDecoder(reader)
	if err != nil {
		return err
	}
	if start.Name.Local != expectedRoot {
		return &UnsupportedRootError{Name: start.Name}
	}

	return decodeRootElement(decoder, start, document)
}

func newDocumentDecoder(
	reader io.Reader,
) (*xml.Decoder, xml.StartElement, error) {
	if reader == nil {
		return nil, xml.StartElement{}, ErrNilReader
	}

	decoder, err := newXMLDecoder(reader)
	if err != nil {
		return nil, xml.StartElement{}, fmt.Errorf(
			"musicxml: initialize XML decoder: %w",
			err,
		)
	}

	start, err := readRoot(decoder)
	if err != nil {
		return nil, xml.StartElement{}, err
	}
	if start.Name.Space != "" {
		return nil, xml.StartElement{}, &UnsupportedRootError{
			Name: start.Name,
		}
	}

	return decoder, start, nil
}

func decodeRootElement(
	decoder *xml.Decoder,
	start xml.StartElement,
	document Document,
) error {
	if err := decoder.DecodeElement(document, &start); err != nil {
		return fmt.Errorf("musicxml: decode document: %w", err)
	}
	if err := readDocumentTail(decoder); err != nil {
		return err
	}

	return nil
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
