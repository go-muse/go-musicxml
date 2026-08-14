package musicxml

const (
	// MusicXMLVersion is the MusicXML schema version implemented by this
	// package.
	MusicXMLVersion = "4.0"

	// Version is the MusicXML schema version implemented by this package.
	//
	// Deprecated: use MusicXMLVersion.
	Version = MusicXMLVersion
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
