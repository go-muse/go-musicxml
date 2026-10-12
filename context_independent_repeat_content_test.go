package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func independentRepeatContent(t *testing.T, report validation.Report, kind validation.RepeatStyleKind) *validation.RepeatContextResult {
	t.Helper()
	for _, fact := range report.Context {
		if fact.Repeat != nil && fact.Repeat.Kind == kind && fact.Repeat.Type.State == validation.EvidenceKnown && fact.Repeat.Type.Value == "start" {
			if fact.Repeat.Complete.State != validation.EvidenceKnown || !fact.Repeat.Complete.Value || fact.Repeat.Stop.State != validation.EvidenceKnown {
				t.Fatalf("fixture does not prove a repeat range: %+v", fact.Repeat)
			}
			return fact.Repeat
		}
	}
	t.Fatal("missing graphical repeat start")
	return nil
}

// The comparison is musical content, not equal raw presence. Natural C without
// alter and C with explicit zero semitones are the same pitch. Raw evidence
// remains distinct; this test requests only normalized content correspondence.
func TestIndependentContextRepeatNaturalAlterEquivalence(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, alter := range []string{"0", "1"} {
				t.Run(fmt.Sprintf("timewise%t/model%t/alter%s", tw, model, alter), func(t *testing.T) {
					body := `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note><attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes><note><pitch><step>C</step><alter>` + alter + `</alter><octave>4</octave></pitch><duration>1</duration></note><attributes><measure-style><beat-repeat type="stop"/></measure-style></attributes>`
					report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
					got := independentRepeatContent(t, report, validation.RepeatStyleBeat).ContentEquivalent
					if alter == "0" {
						if got.State != validation.EvidenceKnown || !got.Value {
							t.Errorf("equivalent natural pitch remains unavailable: %+v", got)
						}
					} else if got.State == validation.EvidenceKnown && got.Value {
						t.Errorf("one-semitone difference incorrectly equal: %+v", got)
					}
				})
			}
		}
	}
}

// measure-style@number restricts the repeated content to that staff. Different
// explicitly other-staff material cannot hide equality in the selected staff;
// a note with unavailable staff identity cannot be silently dropped.
func TestIndependentContextRepeatNumberedStaffContent(t *testing.T) {
	note := func(step, staff string) string {
		selector := ""
		if staff != "" {
			selector = `<staff>` + staff + `</staff>`
		}
		return `<note><pitch><step>` + step + `</step><octave>4</octave></pitch><duration>1</duration>` + selector + `</note>`
	}
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, staff := range []string{"2", ""} {
				t.Run(fmt.Sprintf("timewise%t/model%t/other-staff%s", tw, model, staff), func(t *testing.T) {
					bodies := [][]string{{
						`<attributes><divisions>1</divisions><staves>2</staves></attributes>` + note("C", "1") + `<backup><duration>1</duration></backup>` + note("E", staff),
						`<attributes><measure-style number="1"><measure-repeat type="start">1</measure-repeat></measure-style></attributes>` + note("C", "1") + `<backup><duration>1</duration></backup>` + note("F", staff),
						`<attributes><measure-style number="1"><measure-repeat type="stop"/></measure-style></attributes>` + note("D", "1"),
					}}
					report := independentContextReport(t, independentContextScore("", bodies, tw), model)
					got := independentRepeatContent(t, report, validation.RepeatStyleMeasure).ContentEquivalent
					if staff == "2" {
						if got.State != validation.EvidenceKnown || !got.Value {
							t.Errorf("proved selected-staff content remains unavailable: %+v", got)
						}
					} else if got.State == validation.EvidenceKnown && got.Value {
						t.Errorf("unavailable staff identity dropped from content: %+v", got)
					}
				})
			}
		}
	}
}
