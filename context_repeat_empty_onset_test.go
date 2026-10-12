package musicxml

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// An empty measure's encoded note roster cannot reproduce a source principal
// onset. This is a score-content proof, not a claim of acoustic silence: the
// source note's end may be unproved or extend beyond the nominal measure.
func TestContextRepeatEmptyTargetOnsetProof(t *testing.T) {
	attrs := `<attributes><divisions>4</divisions><time><beats>4</beats><beat-type>4</beat-type></time></attributes>`
	note := func(duration, prefix, suffix string) string {
		return `<note>` + prefix + `<pitch><step>C</step><octave>4</octave></pitch><duration>` + duration + `</duration>` + suffix + `<type>whole</type></note>`
	}
	principal := note("16", "", "")
	forward := `<forward><duration>16</duration></forward>`
	backup := `<backup><duration>16</duration></backup>`
	for _, tc := range []struct {
		name, source, target string
		noStop               bool
	}{
		{name: "whole-note-empty", source: principal},
		{name: "whole-note-forward", source: principal, target: forward},
		{name: "whole-note-absent-stop", source: principal, noStop: true},
		{name: "whole-note-forward-absent-stop", source: principal, target: forward, noStop: true},
		{name: "principal-and-chord-empty", source: principal + note("16", `<chord/>`, "")},
		{name: "principal-and-chord-forward", source: principal + note("16", `<chord/>`, ""), target: forward},
		{name: "overlong-source-empty", source: note("32", "", "")},
		{name: "overlong-source-forward", source: note("32", "", ""), target: forward},
		{name: "source-note-then-backup", source: principal + backup},
	} {
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/timewise%t/model%t", tc.name, tw, model), func(t *testing.T) {
					start := `<attributes><measure-style><measure-repeat type="start">1</measure-repeat></measure-style></attributes>`
					stop := `<attributes><measure-style><measure-repeat type="stop"/></measure-style></attributes>`
					measures := []string{attrs + tc.source, start + tc.target}
					if !tc.noStop {
						measures = append(measures, stop+principal)
					}
					report := independentContextReport(t, independentContextScore("", [][]string{measures}, tw), model)
					repeat := emptyOnsetMeasureRepeat(t, report)
					if got := repeat.NoteScopesComplete; got.State != validation.EvidenceKnown || !got.Value {
						t.Errorf("fixture does not prove a complete note universe: %+v", got)
					}
					if got := repeat.Complete; got.State != validation.EvidenceKnown || !got.Value {
						t.Errorf("fixture does not prove the repeat range: %+v", got)
					}
					wantStop := validation.EvidenceKnown
					if tc.noStop {
						wantStop = validation.EvidenceAbsent
					}
					if repeat.Stop.State != wantStop {
						t.Errorf("stop=%+v, want %s", repeat.Stop, wantStop)
					}
					if got := repeat.ContentEquivalent; got.State != validation.EvidenceKnown || got.Value {
						t.Errorf("empty roster cannot reproduce the source principal onset: got %+v, want Known(false)", got)
					}

					if strings.HasPrefix(tc.name, "whole-note") {
						notes := performanceTiming(report, "note")
						if len(notes) == 0 {
							t.Fatal("missing independent note timing")
						}
						onset, end := notes[0].ScoreOnset, notes[0].ScoreEnd
						if onset.State != validation.EvidenceKnown || onset.Value.MeasureOrdinal != 0 || onset.Value.Offset.String() != "0" || onset.Value.UnprovedFrame {
							t.Errorf("source onset changed: %+v", onset)
						}
						if end.State != validation.EvidenceKnown || end.Value.MeasureOrdinal != 0 || end.Value.Offset.String() != "4" || !end.Value.UnprovedFrame {
							t.Errorf("B7 proof changed uncontained B6 End: %+v", end)
						}
					}
					emptyOnsetContentFailure(t, report, true)
				})
			}
		}
	}
}

// Every selected onset must remain available and contained. In particular,
// rest-only or orphan-chord sources do not prove an ordinary principal onset,
// and unavailable target notes must never disappear into an empty roster.
func TestContextRepeatEmptyTargetOnsetProofBoundaries(t *testing.T) {
	attrs := `<attributes><divisions>4</divisions><time><beats>4</beats><beat-type>4</beat-type></time></attributes>`
	note := func(duration, prefix, suffix string) string {
		return `<note>` + prefix + `<pitch><step>C</step><octave>4</octave></pitch><duration>` + duration + `</duration>` + suffix + `<type>whole</type></note>`
	}
	principal := note("16", "", "")
	for _, tc := range []struct {
		name, source, target, last, staff string
		invalidDuration                   bool
	}{
		{name: "rest-only-source", source: `<note><rest/><duration>16</duration><type>whole</type></note>`},
		{name: "orphan-chord-source", source: note("16", `<chord/>`, "")},
		{name: "zero-duration-source", source: note("0", "", ""), invalidDuration: true},
		{name: "negative-duration-source", source: note("-1", "", ""), invalidDuration: true},
		{name: "missing-duration-source", source: `<note><pitch><step>C</step><octave>4</octave></pitch><type>whole</type></note>`},
		{name: "source-forward-before-note", source: `<forward><duration>16</duration></forward>` + principal},
		{name: "source-negative-onset", source: principal + `<backup><duration>20</duration></backup>` + note("4", "", "")},
		{name: "future-negative-onset", source: principal, last: `<backup><duration>16</duration></backup>` + principal},
		{name: "future-forward-before-note", source: principal, last: `<forward><duration>16</duration></forward>` + principal},
		{name: "unknown-staff-scope", source: principal, staff: ` number="1"`},
		{name: "rest-target-note", source: principal, target: `<note><rest/><duration>16</duration><type>whole</type></note>`},
		{name: "malformed-target-note", source: principal, target: `<note><pitch><step>C</step><octave>4</octave></pitch></note>`},
		{name: "grace-target-note", source: principal, target: `<note><grace/><pitch><step>C</step><octave>4</octave></pitch><type>quarter</type></note>`},
		{name: "tie-target-note", source: principal, target: note("16", "", `<tie type="start"/>`)},
	} {
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/timewise%t/model%t", tc.name, tw, model), func(t *testing.T) {
					start := `<attributes><measure-style` + tc.staff + `><measure-repeat type="start">1</measure-repeat></measure-style></attributes>`
					stop := `<attributes><measure-style` + tc.staff + `><measure-repeat type="stop"/></measure-style></attributes>`
					last := tc.last
					if last == "" {
						last = principal
					}
					measures := []string{attrs + tc.source, start + tc.target, stop + last}
					report := independentContextReport(t, independentContextScore("", [][]string{measures}, tw), model)
					repeat := emptyOnsetMeasureRepeat(t, report)
					if got := repeat.ContentEquivalent; got.State != validation.EvidenceUnknown && got.State != validation.EvidenceInvalid {
						t.Errorf("unproved source or target content must remain Unknown/Invalid, got %+v", got)
					}
					emptyOnsetContentFailure(t, report, false)
					if tc.invalidDuration {
						// Zero/negative MusicXML duration is XSD-invalid, not a
						// known duration that can support this onset theorem.
						emptyOnsetInvalidDuration(t, report, repeat.ContentEquivalent)
					}
				})
			}
		}
	}
}

func emptyOnsetMeasureRepeat(t *testing.T, report validation.Report) *validation.RepeatContextResult {
	t.Helper()
	if !report.InputComplete || report.Terminal != nil {
		t.Fatalf("fixture assessment did not complete its input: complete=%t terminal=%+v", report.InputComplete, report.Terminal)
	}
	var found *validation.RepeatContextResult
	for _, fact := range report.Context {
		if fact.Repeat == nil || fact.Repeat.Kind != validation.RepeatStyleMeasure || fact.Repeat.Type.State != validation.EvidenceKnown || fact.Repeat.Type.Value != "start" {
			continue
		}
		if found != nil {
			t.Fatal("fixture has multiple measure-repeat starts")
		}
		found = fact.Repeat
	}
	if found == nil {
		t.Fatal("missing measure-repeat start")
	}
	return found
}

func emptyOnsetContentFailure(t *testing.T, report validation.Report, want bool) {
	t.Helper()
	found := false
	for _, finding := range report.Findings {
		if finding.Reason == "context-repeat-encoded-content-missing" {
			found = true
			if finding.Outcome != validation.OutcomeFail {
				t.Errorf("content-missing finding must fail: %+v", finding)
			}
		}
	}
	if found != want {
		t.Errorf("content-missing consumer failure present=%t, want %t", found, want)
	}
}

func emptyOnsetInvalidDuration(t *testing.T, report validation.Report, content validation.Evidence[bool]) {
	t.Helper()
	var cause *validation.DecisionRef
	for _, finding := range report.Findings {
		durationPath := strings.Contains(finding.Location.Path, "/duration") || strings.HasSuffix(finding.Location.Path, ".Duration")
		if finding.Outcome == validation.OutcomeFail && strings.Contains(string(finding.Rule.ID), "-XSD-") && finding.Reason == "minExclusive" && durationPath {
			cause = &validation.DecisionRef{Rule: finding.Rule, Subject: finding.Subject}
		}
	}
	if cause == nil {
		t.Errorf("invalid duration lost its XSD failure: %+v", report.Findings)
	}
	if content.State == validation.EvidenceInvalid && (content.Cause == nil || cause == nil || *content.Cause != *cause) {
		t.Errorf("invalid content evidence lost its causal duration prerequisite: content=%+v, want cause=%+v", content, cause)
	}
}

func TestContextRepeatEmptyTargetOnsetProofPatternAndStop(t *testing.T) {
	attrs := `<attributes><divisions>4</divisions></attributes>`
	note := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>16</duration></note>`
	rest := `<note><rest/><duration>16</duration></note>`
	start := `<attributes><measure-style><measure-repeat type="start">2</measure-repeat></measure-style></attributes>`
	stop := `<attributes><measure-style><measure-repeat type="stop"/></measure-style></attributes>`
	for _, tc := range []struct {
		name     string
		measures []string
		state    validation.EvidenceState
		equal    bool
	}{
		{"first-pattern-source", []string{attrs + note, rest, start, stop}, validation.EvidenceKnown, false},
		{"second-pattern-source", []string{attrs + rest, note, start + rest, "", stop}, validation.EvidenceKnown, false},
		{"rest-source-does-not-borrow-next", []string{attrs + rest, note, start, stop}, validation.EvidenceUnknown, false},
		{"empty-stop-measure-is-excluded", []string{attrs + note, note, start + note, note, stop}, validation.EvidenceKnown, true},
	} {
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/timewise%t/model%t", tc.name, tw, model), func(t *testing.T) {
					report := independentContextReport(t, independentContextScore("", [][]string{tc.measures}, tw), model)
					got := emptyOnsetMeasureRepeat(t, report).ContentEquivalent
					if got.State != tc.state || got.State == validation.EvidenceKnown && got.Value != tc.equal {
						t.Errorf("pattern source/stop changed: %+v want %s/%t", got, tc.state, tc.equal)
					}
					emptyOnsetContentFailure(t, report, tc.state == validation.EvidenceKnown && !tc.equal)
				})
			}
		}
	}
}
