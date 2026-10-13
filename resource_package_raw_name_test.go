package musicxml

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPackageRawLegacyNameAdviceSurvivesPathRefusal(t *testing.T) {
	for _, name := range []string{"ascii.dat", "répertoire.dat", "legacy-\x82.dat"} {
		wire := packageGraphBytes(`<opus/>`, "main.musicxml", "", assessmentZIPFixture{name: name, body: "unread"})
		b, err := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
		legacy := !utf8.ValidString(name)
		if legacy && (err == nil || errors.Is(err, validation.ErrInvalidConfiguration)) {
			t.Fatal("lost transport policy distinction", err)
		}
		if !legacy && err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range b.PackageReport().Findings {
			if f.Reason == "legacy-entry-name-encoding" && f.Outcome == validation.OutcomeFail {
				found = true
				if !utf8.ValidString(f.Location.Path) || f.Location.Entry != "" || !strings.Contains(f.Location.Path, "6c65676163792d822e646174") {
					t.Fatal("unsafe or invented diagnostic identity", f.Location)
				}
			}
		}
		if found != legacy || b.PackageReport().Counts.Fail != 0 {
			t.Fatal("legacy advice lost/promoted", b.PackageReport())
		}
	}
}
