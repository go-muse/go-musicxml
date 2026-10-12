package validation_test

import (
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// The same-byte source and direct-model paths exercise both score hierarchies.
// A sufficient window index must not turn a mismatch or a crossing event into
// either equivalence or a necessary-rule failure.
func TestRepeatWindowAdaptersSequentialBeatsAndConservativeControls(t *testing.T) {
	note := func(pitch, duration string) string {
		return `<note><pitch><step>` + pitch + `</step><octave>4</octave></pitch><duration>` + duration + `</duration></note>`
	}
	pattern := note("C", "1") + note("D", "1") + note("E", "1")
	chord := note("C", "3") + `<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>3</duration></note>`
	for _, tc := range []struct {
		name, source, targets, reason string
		known                         bool
	}{
		{"sequential", pattern, strings.Repeat(pattern, 8), "", true},
		{"later-mismatch", pattern, strings.Repeat(pattern, 7) + note("F", "1") + note("D", "1") + note("E", "1"), "context-repeat-content-equivalence-unresolved", false},
		{"crossing-event", pattern, note("C", "4") + note("D", "2"), "context-repeat-cross-boundary-event-unresolved", false},
		{"equal-chord-fallback", chord, strings.Repeat(chord, 3), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<attributes><divisions>3</divisions></attributes>` + tc.source + `<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>` + tc.targets + `<attributes><measure-style><beat-repeat type="stop"/></measure-style></attributes>`
			repeatAssessBoth(t, body, func(t *testing.T, report validation.Report) {
				found := false
				for _, c := range report.Context {
					if c.Repeat == nil || c.Repeat.Kind != validation.RepeatStyleBeat || c.Repeat.Type.Value != "start" {
						continue
					}
					found = true
					got := c.Repeat.ContentEquivalent
					if tc.known {
						if got.State != validation.EvidenceKnown || !got.Value {
							t.Fatalf("equivalent sequence lost: %+v", got)
						}
					} else if got.State != validation.EvidenceUnknown || string(got.Reason) != tc.reason {
						t.Fatalf("conservative control changed: %+v", got)
					}
				}
				if !found {
					t.Fatal("missing beat-repeat evidence")
				}
			})
		})
	}
}
