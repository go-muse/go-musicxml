package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestContextLocalGlyphSemanticsAcrossTargets(t *testing.T) {
	cases := []struct {
		name, defaults, body, property, want string
		state                                validation.EvidenceState
	}{
		{"known-glyph", "<glyph type=\"quarter-rest\">restQuarterOld</glyph>", "", "local.glyph-semantic-target", "quarter-rest", validation.EvidenceKnown},
		{"open-type", "<glyph type=\"application-private\">myGlyph</glyph>", "", "local.glyph-semantic-target", "", validation.EvidenceUnknown},
		{"known-match", "<glyph type=\"g-clef-ottava-bassa\">gClef8vb</glyph>", "", "local.glyph-type-match", "yes", validation.EvidenceKnown},
		{"unresolved-match", "<glyph type=\"quarter-rest\">gClef8vb</glyph>", "", "local.glyph-type-match", "", validation.EvidenceUnknown},
		{"missing-type", "<glyph>restQuarter</glyph>", "", "local.glyph-semantic-target", "", validation.EvidenceInvalid},
		{"symbol-name", "", `<direction><direction-type><symbol justify="right">metNoteQuarterUp</symbol></direction-type></direction>`, "local.symbol-glyph-name", "metNoteQuarterUp", validation.EvidenceKnown},
		{"symbol-advice", "", `<direction><direction-type><symbol>applicationGlyph</symbol></direction-type></direction>`, "local.symbol-specialized-markup", "", validation.EvidenceUnknown},
		{"arrow-direction", "", `<note><rest/><duration>1</duration><notations><technical><arrow smufl="handbellsSwingUp"><arrow-direction>up</arrow-direction><arrow-style>filled</arrow-style></arrow></technical></notations></note>`, "local.arrow-direction", "up", validation.EvidenceKnown},
		{"circular-description", "", `<note><rest/><duration>1</duration><notations><technical><arrow><circular-arrow>clockwise</circular-arrow></arrow></technical></notations></note>`, "local.circular-arrow", "clockwise", validation.EvidenceKnown},
		{"arrowhead", "", `<note><rest/><duration>1</duration><notations><technical><arrow smufl="arrowBlackUp"><arrow-direction>up</arrow-direction><arrowhead/></arrow></technical></notations></note>`, "local.arrow-head-only", "yes", validation.EvidenceKnown},
		{"arrow-shaft", "", `<note><rest/><duration>1</duration><notations><technical><arrow><arrow-direction>up</arrow-direction></arrow></technical></notations></note>`, "local.arrow-head-only", "no", validation.EvidenceKnown},
		{"arrow-invalid-choice", "", `<note><rest/><duration>1</duration><notations><technical><arrow><arrow-direction>up</arrow-direction><circular-arrow>clockwise</circular-arrow></arrow></technical></notations></note>`, "local.arrow-head-only", "", validation.EvidenceInvalid},
		{"arrow-no-glyph", "", `<note><rest/><duration>1</duration><notations><technical><arrow><arrow-direction>up</arrow-direction></arrow></technical></notations></note>`, "local.arrow-glyph-match", "", validation.EvidenceAbsent},
		{"notehead-standard", "", `<note><rest/><duration>1</duration><notehead>normal</notehead></note>`, "local.notehead-text-recommended", "", validation.EvidenceAbsent},
		{"notehead-invalid-name", "", `<note><rest/><duration>1</duration><notehead smufl="two words">other</notehead></note>`, "local.notehead-text-recommended", "", validation.EvidenceInvalid},
		{"notehead-range", "", `<note><rest/><duration>1</duration><notehead smufl="noteheadAWhole">other</notehead></note>`, "local.notehead-text-recommended", "", validation.EvidenceUnknown},
	}
	for _, tc := range cases {
		for _, timewise := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/partwise", true: "/timewise"}[timewise], func(t *testing.T) {
				xml := contextFixtureXML(tc.body, timewise)
				if tc.defaults != "" {
					xml = strings.Replace(xml, "<part-list>", "<defaults><appearance>"+tc.defaults+"</appearance></defaults><part-list>", 1)
				}
				doc, err := Decode(strings.NewReader(xml))
				if err != nil {
					t.Fatal(err)
				}
				for _, model := range []bool{false, true} {
					var r validation.Report
					if model {
						r, err = AssessModel(context.Background(), doc, ModelOptions{Context: validation.ContextFamilyAll, Profile: ProfileModelContext})
					} else {
						r, err = InspectXML(context.Background(), strings.NewReader(xml), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceContext})
					}
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, v := range r.Context {
						if string(v.Property) == tc.property {
							found = true
							expected := tc.state
							if model && tc.name == "missing-type" {
								expected = validation.EvidenceUnknown
							}
							if v.Value.State != expected || expected == validation.EvidenceKnown && v.Value.Value.Text != tc.want {
								t.Errorf("model%t %s: %+v; %s", model, tc.property, v.Value, localFindings(r))
							}
							if expected == validation.EvidenceInvalid && v.Value.Cause == nil {
								t.Error("invalid evidence lacks cause")
							}
						}
					}
					if (tc.name == "open-type" || tc.name == "symbol-advice" || tc.name == "unresolved-match") && !r.Satisfied() {
						t.Errorf("open-domain/advisory evidence became a hard obligation: %s", localFindings(r))
					}
					if !found {
						t.Errorf("model%t missing %s", model, tc.property)
					}
				}
			})
		}
	}
}
