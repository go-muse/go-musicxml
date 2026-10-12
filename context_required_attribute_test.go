package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestContextRequiredAttributeCauseAcrossFamilies(t *testing.T) {
	for _, tc := range []struct {
		name, body, attribute, valid string
		span                         validation.SpanFamily
		navigation                   validation.NavigationKind
	}{
		{name: "slur", body: `<note><rest/><duration>1</duration><notations><slur%s/></notations></note>`, attribute: "type", valid: "start", span: validation.SpanSlur},
		{name: "tied", body: `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><tied%s/></notations></note>`, attribute: "type", valid: "start", span: validation.SpanTied},
		{name: "bracket", body: `<direction><direction-type><bracket line-end="none"%s/></direction-type></direction>`, attribute: "type", valid: "start", span: validation.SpanBracket},
		{name: "repeat", body: `<barline><repeat%s/></barline>`, attribute: "direction", valid: "backward", navigation: validation.NavigationBackward},
	} {
		for _, value := range []string{"", "bad", tc.valid} {
			attr := ""
			if value != "" {
				attr = ` ` + tc.attribute + `="` + value + `"`
			}
			for _, timewise := range []bool{false, true} {
				for _, model := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/timewise%t/model%t", tc.name, value, timewise, model), func(t *testing.T) {
						report := independentContextReport(t, independentContextScore("", [][]string{{fmt.Sprintf(tc.body, attr)}}, timewise), model)
						count := 0
						for _, fact := range report.Context {
							var got validation.Evidence[string]
							if tc.span != "" && fact.Span != nil && fact.Span.Family == tc.span {
								count++
								got = fact.Span.Role
								if tc.span == validation.SpanSlur || tc.span == validation.SpanBracket {
									if fact.Span.Number.State != validation.EvidenceKnown || fact.Span.Number.Value.Integer.String() != "1" {
										t.Errorf("optional number default contaminated: %+v", fact.Span.Number)
									}
								}
							} else if tc.navigation != "" && fact.Navigation != nil && fact.Navigation.Kind == tc.navigation {
								count++
								got = fact.Navigation.Direction
								if fact.Navigation.Times.State != validation.EvidenceAbsent {
									t.Errorf("optional repeat times contaminated: %+v", fact.Navigation.Times)
								}
							} else {
								continue
							}
							if value == "" || value == "bad" {
								if got.State != validation.EvidenceInvalid || got.Cause == nil {
									t.Errorf("required attribute lost causal invalidity: %+v", got)
								}
							} else if got.State != validation.EvidenceKnown || got.Value != value {
								t.Errorf("valid required attribute: %+v", got)
							}
						}
						if count != 1 {
							t.Errorf("got %d matching context rows", count)
						}
					})
				}
			}
		}
	}
}

// Required current-model string fields have a present zero value. In particular,
// empty ending-number is valid: source omission must not be reconstructed from it.
func TestContextRequiredEndingNumberPreservesCurrentValue(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, attribute := range []string{"", ` number=""`, ` number="1"`} {
				t.Run(fmt.Sprintf("%s/timewise%t/model%t", attribute, timewise, model), func(t *testing.T) {
					report := independentContextReport(t, independentContextScore("", [][]string{{`<barline><ending type="start"` + attribute + `/></barline>`}}, timewise), model)
					count := 0
					for _, fact := range report.Context {
						if fact.Navigation == nil || fact.Navigation.Kind != validation.NavigationEnding {
							continue
						}
						count++
						got := fact.Navigation.EndingNumber
						if attribute == "" && !model {
							if got.State != validation.EvidenceInvalid || got.Cause == nil {
								t.Errorf("source missing required number: %+v", got)
							}
						} else {
							want := ""
							if attribute == ` number="1"` {
								want = "1"
							}
							if got.State != validation.EvidenceKnown || got.Value != want {
								t.Errorf("present ending-number: %+v", got)
							}
						}
						if fact.Navigation.Direction.State != validation.EvidenceKnown || fact.Navigation.Direction.Value != "start" {
							t.Errorf("valid sibling attribute contaminated: %+v", fact.Navigation.Direction)
						}
					}
					if count != 1 {
						t.Fatalf("got %d ending rows", count)
					}
				})
			}
		}
	}
}
