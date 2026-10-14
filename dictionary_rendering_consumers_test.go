package musicxml

import (
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type dictionaryLocalExpectation struct {
	property  validation.ContextProperty
	role      string
	input     validation.Evidence[validation.Atomic]
	effective validation.Evidence[validation.Atomic]
}

func dictionaryLocalKnown(text string) validation.Evidence[validation.Atomic] {
	return validation.Evidence[validation.Atomic]{State: validation.EvidenceKnown, Value: validation.Atomic{Kind: validation.AtomicString, Text: text}}
}

func dictionaryLocalUnknown(reason validation.Code) validation.Evidence[validation.Atomic] {
	return validation.Evidence[validation.Atomic]{State: validation.EvidenceUnknown, Reason: reason}
}

// These assertions exercise the dictionary profiles, independently of the
// older context-profile rendering tests. No full semantic dictionary exists:
// canonical-name availability must not promote local advice to a proof.
func assertDictionaryLocalSourceModel(t *testing.T, text string, canonicalCount int, want []dictionaryLocalExpectation) {
	t.Helper()
	for _, target := range []struct {
		name    string
		model   bool
		profile validation.ProfileID
	}{{"source", false, ProfileSourceDictionaries}, {"model", true, ProfileModelDictionaries}} {
		for _, data := range []struct {
			name      string
			available bool
		}{{"without-data", false}, {"with-data", true}} {
			t.Run(target.name+"/"+data.name, func(t *testing.T) {
				report, err := dictionaryInspectTest(t, text, target.model, data.available)
				if err != nil {
					t.Fatal(err)
				}
				if report.Profile.ID != target.profile {
					t.Fatalf("profile = %s, want %s", report.Profile.ID, target.profile)
				}
				if report.Conformance == validation.ConformanceViolated || report.Terminal != nil {
					t.Fatalf("local advice became a violation or terminal failure: %s", localFindings(report))
				}
				wantConformance := validation.ConformanceConformant
				if !data.available && canonicalCount != 0 {
					wantConformance = validation.ConformanceUndetermined
				}
				if report.Conformance != wantConformance {
					t.Fatalf("local advice changed the independent canonical obligation: conformance=%s, want %s; %s", report.Conformance, wantConformance, localFindings(report))
				}
				canonical := dictionaryResults(report, validation.ContextDictionaryCanonical)
				if len(canonical) != canonicalCount {
					t.Fatalf("canonical occurrence count = %d, want %d", len(canonical), canonicalCount)
				}
				for _, row := range canonical {
					if data.available {
						if row.Result.State != validation.EvidenceKnown || !row.Result.Value {
							t.Fatalf("fixture must independently prove canonical membership: %+v", row)
						}
					} else if row.Result.State != validation.EvidenceUnknown {
						t.Fatalf("absent dictionary manufactured membership: %+v", row)
					}
				}
				rows := append(dictionaryResults(report, validation.ContextDictionaryRendering), dictionaryResults(report, validation.ContextDictionaryAdvice)...)
				if len(rows) != len(want) {
					t.Fatalf("local occurrence count = %d, want %d: %+v", len(rows), len(want), rows)
				}
				for _, expected := range want {
					found := 0
					for _, row := range dictionaryResults(report, expected.property) {
						if row.Role != expected.role {
							continue
						}
						found++
						if row.Input != expected.input || row.Effective != expected.effective {
							t.Errorf("%s: input=%+v effective=%+v, want input=%+v effective=%+v", row.Role, row.Input, row.Effective, expected.input, expected.effective)
						}
						if row.Applicability.State != validation.EvidenceKnown || !row.Applicability.Value || row.Result.State != validation.EvidenceAbsent {
							t.Errorf("%s: local facts manufactured membership or lost applicability: %+v", row.Role, row)
						}
					}
					if found != 1 {
						t.Errorf("%s/%s occurrence count = %d, want 1", expected.property, expected.role, found)
					}
				}
			})
		}
	}
}

func TestDictionarySymbolDefaultsAndUnavailableAdviceSourceModel(t *testing.T) {
	absent := validation.Evidence[validation.Atomic]{State: validation.EvidenceAbsent}
	for _, tc := range []struct {
		name, attributes, justify, enclosure string
	}{{"defaults", "", "left", "none"}, {"explicit", ` justify="right" enclosure="rectangle"`, "right", "rectangle"}} {
		t.Run(tc.name, func(t *testing.T) {
			justify, enclosure := absent, absent
			if tc.attributes != "" {
				justify, enclosure = dictionaryLocalKnown(tc.justify), dictionaryLocalKnown(tc.enclosure)
			}
			text := contextFixtureXML(`<direction><direction-type><symbol`+tc.attributes+`>restQuarter</symbol></direction-type></direction>`, false)
			assertDictionaryLocalSourceModel(t, text, 1, []dictionaryLocalExpectation{
				{validation.ContextDictionaryRendering, "symbol-justify", justify, dictionaryLocalKnown(tc.justify)},
				{validation.ContextDictionaryRendering, "symbol-enclosure", enclosure, dictionaryLocalKnown(tc.enclosure)},
				{validation.ContextDictionaryAdvice, "symbol-specialized-markup", dictionaryLocalKnown("restQuarter"), dictionaryLocalUnknown("smufl-specialized-markup-dictionary-unavailable")},
			})
		})
	}
}

func TestDictionaryGlyphLimitedPositivesAndUnavailableAdviceSourceModel(t *testing.T) {
	for _, tc := range []struct {
		name, typ, glyph string
		target, match    validation.Evidence[validation.Atomic]
	}{
		{"quarter-rest-positive", "quarter-rest", "restQuarterOld", dictionaryLocalKnown("quarter-rest"), dictionaryLocalKnown("yes")},
		{"clef-positive", "g-clef-ottava-bassa", "gClef8vb", dictionaryLocalKnown("g-clef-ottava-bassa"), dictionaryLocalKnown("yes")},
		{"octave-positive", "octave-shift-up-8", "ottava", dictionaryLocalKnown("octave-shift-up-8"), dictionaryLocalKnown("yes")},
		{"unresolved-match", "quarter-rest", "gClef8vb", dictionaryLocalKnown("quarter-rest"), dictionaryLocalUnknown("smufl-semantic-dictionary-unavailable")},
		{"open-type", "application-private", "restQuarter", dictionaryLocalUnknown("application-specific-glyph-type"), dictionaryLocalUnknown("smufl-semantic-dictionary-unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Replace(contextFixtureXML("", false), "<part-list>", `<defaults><appearance><glyph type="`+tc.typ+`">`+tc.glyph+`</glyph></appearance></defaults><part-list>`, 1)
			assertDictionaryLocalSourceModel(t, text, 1, []dictionaryLocalExpectation{
				{validation.ContextDictionaryAdvice, "glyph-semantic-target", dictionaryLocalKnown(tc.typ), tc.target},
				{validation.ContextDictionaryAdvice, "glyph-type-match", dictionaryLocalKnown(tc.glyph), tc.match},
			})
		})
	}
}

func TestDictionaryArrowLocalDescriptionAndUnavailableAdviceSourceModel(t *testing.T) {
	absent := validation.Evidence[validation.Atomic]{State: validation.EvidenceAbsent}
	for _, tc := range []struct {
		name, attributes, children, direction, style, circular, head string
	}{
		{"head", ` smufl="arrowBlackUp"`, `<arrow-direction>up</arrow-direction><arrow-style>filled</arrow-style><arrowhead/>`, "up", "filled", "", "yes"},
		{"unresolved-description", ` smufl="restQuarter"`, `<arrow-direction>down</arrow-direction>`, "down", "", "", "no"},
		{"circular", "", `<circular-arrow>clockwise</circular-arrow>`, "", "", "clockwise", "no"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := func(s string) validation.Evidence[validation.Atomic] {
				if s == "" {
					return absent
				}
				return dictionaryLocalKnown(s)
			}
			input, match, count := absent, absent, 0
			if tc.attributes != "" {
				name := "arrowBlackUp"
				if tc.name == "unresolved-description" {
					name = "restQuarter"
				}
				input, match, count = dictionaryLocalKnown(name), dictionaryLocalUnknown("smufl-arrow-description-dictionary-unavailable"), 1
			}
			text := contextFixtureXML(`<note><rest/><duration>1</duration><notations><technical><arrow`+tc.attributes+`>`+tc.children+`</arrow></technical></notations></note>`, false)
			assertDictionaryLocalSourceModel(t, text, count, []dictionaryLocalExpectation{
				{validation.ContextDictionaryRendering, "arrow-direction", value(tc.direction), value(tc.direction)},
				{validation.ContextDictionaryRendering, "arrow-style", value(tc.style), value(tc.style)},
				{validation.ContextDictionaryRendering, "circular-arrow", value(tc.circular), value(tc.circular)},
				{validation.ContextDictionaryRendering, "arrow-head-only", dictionaryLocalKnown(tc.head), dictionaryLocalKnown(tc.head)},
				{validation.ContextDictionaryAdvice, "arrow-glyph-match", input, match},
			})
		})
	}
}

func TestDictionaryInvertedMordentLocalLongDefaultSourceModel(t *testing.T) {
	for _, tc := range []struct {
		name, attributes, effective string
	}{{"default", "", "no"}, {"explicit-yes", ` long="yes"`, "yes"}, {"explicit-no", ` long="no"`, "no"}} {
		t.Run(tc.name, func(t *testing.T) {
			input := validation.Evidence[validation.Atomic]{State: validation.EvidenceAbsent}
			if tc.attributes != "" {
				input = dictionaryLocalKnown(tc.effective)
			}
			text := contextFixtureXML(`<note><rest/><duration>1</duration><notations><ornaments><inverted-mordent`+tc.attributes+`/></ornaments></notations></note>`, false)
			assertDictionaryLocalSourceModel(t, text, 0, []dictionaryLocalExpectation{
				{validation.ContextDictionaryRendering, "mordent-long", input, dictionaryLocalKnown(tc.effective)},
			})
		})
	}
}
