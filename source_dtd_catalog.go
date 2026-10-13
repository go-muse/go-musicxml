package musicxml

import (
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceInternalDTD selects the bounded internal-DTD source-infoset
// pipeline for MusicXML score and Opus roots. It applies actually read supported
// declarations before namespace expansion and XSD assessment. External DTDs are
// never fetched; unavailable declaration closure prevents acceptance. This is
// an explicit developmental source profile, not general XML/DTD conformance or
// a change to Decode and the existing source/model/package profiles.
const ProfileSourceInternalDTD validation.ProfileID = "musicxml-4.0/source-internal-dtd"

var sourceInternalDTDEngine = sync.OnceValues(func() (*validation.Engine, error) {
	return buildSourceXSDProfileEngine(true)
})
