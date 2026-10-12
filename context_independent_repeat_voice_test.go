package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// An explicit except-voice applies to the graphical beat-repeat scope. The
// named voice can differ without changing the repeated content of other known
// voices. Missing voice identity remains unavailable rather than excluded.
func TestIndependentContextRepeatExceptVoiceContent(t *testing.T) {
	note := func(step, voice string) string {
		v := ""
		if voice != "" {
			v = `<voice>` + voice + `</voice>`
		}
		return `<note><pitch><step>` + step + `</step><octave>4</octave></pitch><duration>1</duration>` + v + `</note>`
	}
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, voice := range []string{"2", ""} {
				t.Run(fmt.Sprintf("timewise%t/model%t/excluded-voice%s", tw, model, voice), func(t *testing.T) {
					body := `<attributes><divisions>1</divisions></attributes>` + note("C", "1") + `<backup><duration>1</duration></backup>` + note("E", voice) + `<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type><except-voice>2</except-voice></beat-repeat></measure-style></attributes>` + note("C", "1") + `<backup><duration>1</duration></backup>` + note("F", voice) + `<attributes><measure-style><beat-repeat type="stop"/></measure-style></attributes>`
					report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
					got := independentRepeatContent(t, report, validation.RepeatStyleBeat).ContentEquivalent
					if voice == "2" {
						if got.State != validation.EvidenceKnown || !got.Value {
							t.Errorf("proved nonexcepted voice remains unavailable: %+v", got)
						}
					} else if got.State == validation.EvidenceKnown && got.Value {
						t.Errorf("unavailable voice identity excluded: %+v", got)
					}
				})
			}
		}
	}
}
