package validation

import (
	"os"
	"testing"
)

// Tests of this package evaluate plans and context readers directly on
// sessions whose synthetic profiles hold only XSD rules, which in production
// would release the typed XSD results of completed subjects. They keep them,
// as a profile with those plans would. Tests of the release itself turn the
// switch off, and the root package runs every shipped profile without it.
func TestMain(m *testing.M) {
	setKeepTypedXSD(true)
	os.Exit(m.Run())
}
