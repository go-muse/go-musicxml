package musicxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// EncodeOptions controls the textual form written by EncodeWithOptions. The
// zero value writes exactly the bytes Encode writes.
type EncodeOptions struct {
	// XMLDeclaration writes the standard XML declaration
	// <?xml version="1.0" encoding="UTF-8"?> and a line feed before the root
	// element, as xml.Header does.
	XMLDeclaration bool

	// Indent, when non-empty, starts every nested element on a new line
	// indented by one more copy of Indent, like xml.Encoder.Indent. It may
	// contain only spaces and tabs. Character data is never re-indented.
	Indent string
}

// Encode writes a MusicXML root element.
//
// Encode does not add an XML declaration. Strings that cannot be represented
// in XML 1.0 are rejected before writing any output. Use EncodeWithOptions for
// an XML declaration or indented output with the same checks.
func Encode(
	writer io.Writer,
	document Document,
) error {
	return EncodeWithOptions(writer, document, EncodeOptions{})
}

// EncodeWithOptions writes a MusicXML root element like Encode, using options
// for the XML declaration and indentation.
//
// Prefer it to configuring an xml.Encoder directly: direct encoding/xml use
// skips the cycle, nesting and XML 1.0 text checks that Encode performs before
// writing any output. Invalid options match ErrInvalidEncodeOptions and are
// reported before the document is inspected.
func EncodeWithOptions(
	writer io.Writer,
	document Document,
	options EncodeOptions,
) error {
	if nilValue(writer) {
		return ErrNilWriter
	}
	if err := options.validate(); err != nil {
		return err
	}

	switch value := document.(type) {
	case nil:
		return ErrNilDocument

	case *OpusDocument:
		if value == nil {
			return ErrNilDocument
		}

	case *ScorePartwise:
		if value == nil {
			return ErrNilDocument
		}

	case *ScoreTimewise:
		if value == nil {
			return ErrNilDocument
		}

	default:
		return fmt.Errorf(
			"%w: %T",
			ErrUnsupportedDocument,
			document,
		)
	}
	if err := checkDocumentNesting(document); err != nil {
		return err
	}

	if err := checkDocumentText(document); err != nil {
		return fmt.Errorf("musicxml: encode document: %w", err)
	}

	if options.XMLDeclaration {
		if _, err := io.WriteString(writer, xml.Header); err != nil {
			return fmt.Errorf("musicxml: encode document: %w", err)
		}
	}

	encoder := xml.NewEncoder(writer)
	if options.Indent != "" {
		encoder.Indent("", options.Indent)
	}
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("musicxml: encode document: %w", err)
	}

	return nil
}

func (o EncodeOptions) validate() error {
	if strings.Trim(o.Indent, " \t") != "" {
		return fmt.Errorf(
			"%w: Indent must contain only spaces and tabs",
			ErrInvalidEncodeOptions,
		)
	}

	return nil
}
