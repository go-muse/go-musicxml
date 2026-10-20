package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type localFixture struct{ header, prefix, suffix, body, partID string }

func (f localFixture) xml() string {
	id := f.partID
	if id == "" {
		id = "P1"
	}
	return `<score-partwise version="4.0" xmlns:xlink="http://www.w3.org/1999/xlink">` + f.header + `<part-list><score-part id="P1">` + f.prefix + `<part-name>Piano</part-name><score-instrument id="I1"><instrument-name>Piano</instrument-name></score-instrument><player id="PL1"><player-name>First</player-name></player>` + f.suffix + `</score-part><score-part id="P2"><part-name>Second</part-name><score-instrument id="I2"><instrument-name>Second</instrument-name></score-instrument><player id="PL2"><player-name>Second</player-name></player></score-part></part-list><part id="` + id + `"><measure number="1">` + f.body + `</measure></part></score-partwise>`
}
func localNote(tail string) string { return `<note><rest/><duration>1</duration>` + tail + `</note>` }
func localDirection(content string) string {
	return `<direction><direction-type>` + content + `</direction-type></direction>`
}
func localConcert() string { return `<defaults><concert-score/></defaults>` }
func localFindings(report validation.Report) string {
	var b strings.Builder
	for _, f := range report.Findings {
		fmt.Fprintf(&b, "%s %s %s %s cause=%v; ", f.Rule.ID, f.Outcome, f.Reason, f.Location.Path, f.Cause)
	}
	return b.String()
}
func assessLocalFixture(t *testing.T, f localFixture, rule string, violated bool) {
	t.Helper()
	source := f.xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("fixture decode: %v", err)
	}
	for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
		t.Run(string(target), func(t *testing.T) {
			var report validation.Report
			if target == validation.TargetSource {
				report, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceLocal})
			} else {
				report, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelLocal})
			}
			if err != nil {
				t.Fatalf("operation: %v; %s", err, localFindings(report))
			}
			if !report.InputComplete || !report.AssessmentComplete || report.BlockingProblems != 0 {
				t.Fatalf("incomplete: %+v; %s", report.Counts, localFindings(report))
			}
			if !violated {
				if !report.Satisfied() {
					t.Fatalf("positive fixture: %s", localFindings(report))
				}
				return
			}
			if report.Conformance != validation.ConformanceViolated || report.Satisfied() {
				t.Fatalf("negative fixture accepted: %+v", report.Counts)
			}
			found := false
			for _, finding := range report.Findings {
				if finding.Outcome != validation.OutcomeFail {
					continue
				}
				if strings.HasPrefix(string(finding.Rule.ID), "MX40-XSD-") {
					t.Errorf("fixture has unintended XSD failure: %s", localFindings(report))
				}
				if strings.HasPrefix(string(finding.Rule.ID), "MX40-LOCAL-"+rule) {
					found = true
					if finding.Rule.Revision != 1 || finding.Location.Target != target || finding.Location.Path == "" {
						t.Errorf("missing identity/location: %+v", finding)
					}
				}
			}
			if !found {
				t.Fatalf("missing clause %s: %s", rule, localFindings(report))
			}
		})
	}
}

func TestBoundedLocalProductionClausesBothTargets(t *testing.T) {
	type test struct {
		name, rule string
		fixture    localFixture
		fail       bool
	}
	var tests []test
	add := func(name, rule, body string, fail bool) {
		tests = append(tests, test{name, rule, localFixture{body: body}, fail})
	}
	add("accordion-present", "accordion-at-least-one", localDirection(`<accordion-registration><accordion-high/></accordion-registration>`), false)
	add("accordion-empty", "accordion-at-least-one", localDirection(`<accordion-registration/>`), true)
	add("accordion-middle-present", "accordion-at-least-one", localDirection(`<accordion-registration><accordion-middle>1</accordion-middle></accordion-registration>`), false)
	add("accordion-low-present", "accordion-at-least-one", localDirection(`<accordion-registration><accordion-low/></accordion-registration>`), false)
	add("accordion-all-three", "accordion-at-least-one", localDirection(`<accordion-registration><accordion-high/><accordion-middle>3</accordion-middle><accordion-low/></accordion-registration>`), false)
	add("accordion-omitted", "accordion-at-least-one", "", false)
	add("beam-default-distinct", "beam-number-distinct", localNote(`<beam>begin</beam><beam number="2">begin</beam>`), false)
	add("beam-default-duplicate", "beam-number-distinct", localNote(`<beam>begin</beam><beam number="1">end</beam>`), true)
	add("beam-level-eight", "beam-number-distinct", localNote(`<beam number="8">begin</beam>`), false)
	add("beam-equivalent-lexical-values", "beam-number-distinct", localNote(`<beam number="01">begin</beam><beam number="1">end</beam>`), true)
	add("bezier-no-second-controls", "bezier-second-control", localNote(`<notations><slur type="start"/><tied type="continue"/></notations>`), false)
	for _, attr := range []string{"bezier-x2", "bezier-y2", "bezier-offset2"} {
		add("slur-continue-"+attr, "bezier-second-control", localNote(`<notations><slur type="continue" `+attr+`="0"/></notations>`), false)
		add("slur-start-"+attr, "bezier-second-control", localNote(`<notations><slur type="start" `+attr+`="0"/></notations>`), true)
		add("tied-continue-"+attr, "bezier-second-control", localNote(`<notations><tied type="continue" `+attr+`="0"/></notations>`), true)
	}
	forPart := `<attributes><for-part><part-transpose><chromatic>2</chromatic></part-transpose></for-part></attributes>`
	tests = append(tests, test{"for-part-with-marker", "for-part-requires-concert", localFixture{header: localConcert(), body: forPart}, false}, test{"for-part-without-marker", "for-part-requires-concert", localFixture{body: forPart}, true})
	add("key-cancel-present", "key-octave-cancel-exists", `<attributes><key><cancel>2</cancel><fifths>0</fifths><key-octave number="1" cancel="yes">4</key-octave></key></attributes>`, false)
	add("key-cancel-missing", "key-octave-cancel-exists", `<attributes><key><fifths>0</fifths><key-octave number="1" cancel="yes">4</key-octave></key></attributes>`, true)
	add("key-cancel-no", "key-octave-cancel-exists", `<attributes><key><fifths>0</fifths><key-octave number="1" cancel="no">4</key-octave></key></attributes>`, false)
	for _, value := range []string{"", "equals", " equals", "Equals", "approximately"} {
		body := localDirection(`<metronome><metronome-note><metronome-type>quarter</metronome-type></metronome-note><metronome-relation>` + value + `</metronome-relation><metronome-note><metronome-type>half</metronome-type></metronome-note></metronome>`)
		add("metronome-"+value, "metronome-relation-value", body, value != "" && value != "equals")
	}
	for _, attr := range []string{"dacapo", "forward-repeat"} {
		for _, value := range []string{"yes", "no"} {
			add(attr+"-"+value, "sound-"+attr+"-yes", `<sound `+attr+`="`+value+`"/>`, value == "no")
		}
	}
	add("sound-omitted-flags", "sound-dacapo-yes", `<sound/>`, false)
	for _, attr := range []string{"damper-pedal", "soft-pedal", "sostenuto-pedal"} {
		for _, value := range []string{"yes", "no", "0", "100", "0.125", "100.001", "-0.001"} {
			add(attr+"-"+value, "sound-pedal-range", `<sound `+attr+`="`+value+`"/>`, value == "100.001" || value == "-0.001")
		}
	}
	for _, child := range []string{`<chromatic>0</chromatic>`, `<diatonic>0</diatonic><chromatic>0</chromatic><octave-change>1</octave-change><double/>`, `<chromatic>1</chromatic>`, `<diatonic>1</diatonic><chromatic>0</chromatic>`} {
		fail := strings.Contains(child, `<chromatic>1`) || strings.Contains(child, `<diatonic>1`)
		tests = append(tests, test{"concert-" + child, "concert-score-transpose", localFixture{header: localConcert(), body: `<attributes><transpose>` + child + `</transpose></attributes>`}, fail})
	}
	add("written-transpose-outside-marker", "concert-score-transpose", `<attributes><transpose><chromatic>2</chromatic></transpose></attributes>`, false)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assessLocalFixture(t, tt.fixture, tt.rule, tt.fail) })
	}
}

func TestBoundedTypedReferencesBothTargets(t *testing.T) {
	for _, spec := range []struct {
		rule  string
		build func(string) localFixture
	}{
		{"midi-device-target", func(id string) localFixture {
			return localFixture{suffix: `<midi-device id="` + id + `">Device</midi-device>`}
		}},
		{"midi-instrument-target", func(id string) localFixture {
			return localFixture{suffix: `<midi-instrument id="` + id + `"><midi-channel>1</midi-channel><midi-bank>1</midi-bank><midi-program>1</midi-program></midi-instrument>`}
		}},
		{"instrument-change-target", func(id string) localFixture {
			return localFixture{body: `<sound><instrument-change id="` + id + `"/></sound>`}
		}},
		{"instrument-note-target", func(id string) localFixture { return localFixture{body: localNote(`<instrument id="` + id + `"/>`)} }},
		{"play-instrument-target", func(id string) localFixture { return localFixture{body: `<sound><play id="` + id + `"/></sound>`} }},
		{"instrument-link-target", func(id string) localFixture {
			return localFixture{prefix: `<part-link xlink:href="not-fetched.musicxml"><instrument-link id="` + id + `"/></part-link>`}
		}},
	} {
		for _, target := range []string{"I1", "I2", "PL1"} {
			t.Run(spec.rule+"-"+target, func(t *testing.T) { assessLocalFixture(t, spec.build(target), spec.rule, target != "I1") })
		}
	}
	for _, target := range []string{"PL1", "PL2", "I1"} {
		t.Run("assess-"+target, func(t *testing.T) {
			assessLocalFixture(t, localFixture{body: localNote(`<listen><assess type="yes" player="` + target + `"/></listen>`)}, "assess-player-target", target != "PL1")
		})
	}
	t.Run("part-wrong-kind", func(t *testing.T) { assessLocalFixture(t, localFixture{partID: "I1"}, "part-id-target", true) })
	t.Run("optional-omission", func(t *testing.T) {
		assessLocalFixture(t, localFixture{suffix: `<midi-device>Device</midi-device>`, body: `<sound><play/></sound>` + localNote(`<instrument id="I1"/><instrument id="I1"/><listen><assess type="yes"/></listen>`)}, "", false)
	})
}

// Child presence is independent of its XSD scalar validity. The pinned
// accordion-middle domain excludes zero; the local register-presence clause
// must not relabel that present-but-invalid child as a missing registration.
func TestAccordionInvalidPresentChildKeepsDistinctConsequence(t *testing.T) {
	source := (localFixture{body: localDirection(`<accordion-registration><accordion-middle>0</accordion-middle></accordion-registration>`)}).xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
		var report validation.Report
		if target == validation.TargetSource {
			report, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceLocal})
		} else {
			report, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelLocal})
		}
		if err != nil || report.Conformance != validation.ConformanceViolated || !report.AssessmentComplete {
			t.Fatalf("%s: %+v %v", target, report, err)
		}
		xsdFailure := false
		for _, finding := range report.Findings {
			if finding.Outcome != validation.OutcomeFail {
				continue
			}
			if finding.Rule.ID == "MX40-LOCAL-accordion-at-least-one" {
				t.Fatalf("%s: present child treated as absent", target)
			}
			if finding.Rule.ID == "MX40-XSD-musicxml-scalar" && finding.Reason == "datatype" {
				xsdFailure = true
			}
		}
		if !xsdFailure {
			t.Fatalf("%s: lost independent child domain failure", target)
		}
	}
}
