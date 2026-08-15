package musicxml

import "encoding/xml"

const (
	// MusicXMLVersion is the MusicXML schema version implemented by this
	// package.
	MusicXMLVersion = "4.0"
)

// Document is a supported MusicXML root document.
type Document interface {
	isDocument()
}

// ScoreDocument is a supported MusicXML score root document.
type ScoreDocument interface {
	Document
	isScoreDocument()
}

func (*ScorePartwise) isDocument() {}

func (*ScorePartwise) isScoreDocument() {}

func (*ScoreTimewise) isDocument() {}

func (*ScoreTimewise) isScoreDocument() {}

func (*OpusDocument) isDocument() {}

// AsScorePartwise returns document as a partwise score when it has that root
// type.
func AsScorePartwise(document Document) (*ScorePartwise, bool) {
	value, ok := document.(*ScorePartwise)
	return value, ok && value != nil
}

// AsScoreTimewise returns document as a timewise score when it has that root
// type.
func AsScoreTimewise(document Document) (*ScoreTimewise, bool) {
	value, ok := document.(*ScoreTimewise)
	return value, ok && value != nil
}

// AsOpusDocument returns document as an opus when it has that root type.
func AsOpusDocument(document Document) (*OpusDocument, bool) {
	value, ok := document.(*OpusDocument)
	return value, ok && value != nil
}

func documentRootName(document Document) xml.Name {
	switch document.(type) {
	case *ScorePartwise:
		return xml.Name{Local: "score-partwise"}
	case *ScoreTimewise:
		return xml.Name{Local: "score-timewise"}
	case *OpusDocument:
		return xml.Name{Local: "opus"}
	default:
		return xml.Name{}
	}
}
