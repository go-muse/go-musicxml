package musicxml

import (
	"encoding/xml"
	"fmt"
	"io"
)

// Encode writes a MusicXML root element.
//
// Encode does not add an XML declaration. Strings that cannot be represented
// in XML 1.0 are rejected before writing any output.
func Encode(
	writer io.Writer,
	document Document,
) error {
	if writer == nil {
		return ErrNilWriter
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

	if err := xml.NewEncoder(writer).Encode(document); err != nil {
		return fmt.Errorf("musicxml: encode document: %w", err)
	}

	return nil
}
