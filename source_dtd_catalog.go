package musicxml

import (
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceInternalDTD selects the bounded internal-DTD source-infoset
// pipeline for MusicXML score and Opus roots. It applies actually read supported
// declarations before namespace expansion and XSD assessment, including a DOCTYPE
// external subset read from the bundled official MusicXML DTDs or a caller
// DTDSource. External DTDs are never fetched; unavailable declaration closure
// prevents acceptance. This is
// an explicit developmental source profile, not general XML/DTD conformance or
// a change to Decode and the existing source/model/package profiles.
const ProfileSourceInternalDTD SourceProfile = "musicxml-4.0/source-internal-dtd"

const profileSourceInternalDTD = validation.ProfileID(ProfileSourceInternalDTD)

var sourceInternalDTDEngine = sync.OnceValues(func() (*validation.Engine, error) {
	return buildSourceXSDProfileEngine(true)
})
