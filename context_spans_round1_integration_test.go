package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestSpanRound1TieStubAndChainBoundaries(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	for _, tc := range []struct {
		name, middle string
		outer, known bool
	}{
		{"isolated-stub", `<tied type="start"/><tied type="stop"/>`, false, true},
		{"ambiguous-stub-or-chain", `<tied type="start"/><tied type="stop"/>`, true, false},
		{"ordinary-chain", `<tied type="stop"/><tied type="start"/>`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := timelineNote("1", "", `<notations>`+tc.middle+`</notations>`)
			if tc.outer {
				body = timelineNote("1", "", `<notations><tied type="start"/></notations>`) + body + timelineNote("1", "", `<notations><tied type="stop"/></notations>`)
			}
			body = `<attributes><divisions>1</divisions></attributes>` + body
			spanBothWithEngine(t, engine, profiles, (localFixture{body: body}).xml(), false, "", func(t *testing.T, r validation.Report) {
				es := spanResults(r, validation.SpanTied)
				for _, e := range es {
					if (e.Association.State == validation.EvidenceKnown) != tc.known {
						t.Fatalf("unproved tie interpretation: %+v", es)
					}
				}
				if tc.outer && tc.known {
					if es[0].Members[1] != es[1].Members[1] || es[2].Members[0] != es[3].Members[0] || es[0].Members[0] == es[2].Members[0] {
						t.Fatalf("wrong chain: %+v", es)
					}
				}
			})
		})
	}
}

func TestSpanRound1AbsentLyricIdentityAndCompetition(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	for _, tc := range []struct {
		name, extraStart, extraStop string
		known                       bool
	}{
		{"unnamed", "", "", true},
		{"named-and-unnamed", `<lyric number="1"><text>b</text><extend type="start"/></lyric>`, `<lyric number="1"><extend type="stop"/></lyric>`, true},
		{"duplicate-unnamed", `<lyric><text>b</text><extend type="start"/></lyric>`, `<lyric><extend type="stop"/></lyric>`, false},
		{"competing-unnamed-without-extend", `<lyric><text>b</text></lyric>`, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<lyric><text>a</text><extend type="start"/></lyric>`+tc.extraStart) + timelineNote("1", "", `<lyric><extend type="stop"/></lyric>`+tc.extraStop)
			spanBothWithEngine(t, engine, profiles, (localFixture{body: body}).xml(), false, "", func(t *testing.T, r validation.Report) {
				es := spanResults(r, validation.SpanLyricExtend)
				for _, e := range es {
					if (e.Association.State == validation.EvidenceKnown) != tc.known {
						t.Fatalf("lyric identity %+v", es)
					}
					if e.Number.State != validation.EvidenceAbsent {
						t.Fatal("invented verse number", e)
					}
				}
			})
		})
	}
}

func TestSpanRound1VerticalSeparatesNoteCategories(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	principal := timelineNote("1", "", `<notations><arpeggiate/></notations>`)
	for _, tc := range []struct{ name, first string }{
		{"grace", `<note><grace/><pitch><step>C</step><octave>4</octave></pitch><type>eighth</type><notations><arpeggiate/></notations></note>`},
		{"cue", `<note><cue/><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>eighth</type><notations><arpeggiate/></notations></note><backup><duration>1</duration></backup>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<attributes><divisions>1</divisions></attributes>` + tc.first + principal
			spanBothWithEngine(t, engine, profiles, (localFixture{body: body}).xml(), false, "", func(t *testing.T, r validation.Report) {
				es := spanResults(r, validation.SpanArpeggiate)
				if len(es) != 2 {
					t.Fatal(es)
				}
				for _, e := range es {
					if e.Association.State != validation.EvidenceKnown || len(e.Members) != 1 {
						t.Fatalf("note categories merged: %+v", es)
					}
				}
			})
		})
	}
}

func TestSpanRound1BeamLevelsHaveNoStartStopAdvice(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	body := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<beam number="1">begin</beam><beam number="2">begin</beam>`) + timelineNote("1", "", `<beam number="1">end</beam><beam number="2">end</beam>`)
	spanBothWithEngine(t, engine, profiles, (localFixture{body: body}).xml(), false, "", func(t *testing.T, r validation.Report) {
		if advice := spanProperty(r, validation.ContextPropertySpanSameNoteOrderRecommended); len(advice) != 0 {
			t.Fatal("irrelevant same-note advice", advice)
		}
	})
}

// The real adapters also gate canonical part identity on incomplete enumeration.
// The provider-level hierarchy test separately exercises a supplied, valid timeline.
func TestSpanRound1HierarchyGapKeepsAdapterIdentityUnavailable(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	header := `<part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>`
	bodies := []string{`<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<notations><slur type="start"/></notations>`), timelineNote("1", "", ""), timelineNote("1", "", `<notations><slur type="stop"/></notations>`)}
	for _, hierarchy := range []string{"partwise", "timewise"} {
		t.Run(hierarchy, func(t *testing.T) {
			var b strings.Builder
			b.WriteString(`<score-` + hierarchy + ` version="4.0">` + header)
			if hierarchy == "timewise" {
				for _, body := range bodies {
					b.WriteString(`<measure number="1"><part id="P1">` + body + `</part><part id="P2">` + body + `</part></measure>`)
				}
			} else {
				for _, part := range []string{"P1", "P2"} {
					b.WriteString(`<part id="` + part + `">`)
					for _, body := range bodies {
						b.WriteString(`<measure number="1">` + body + `</measure>`)
					}
					b.WriteString(`</part>`)
				}
			}
			b.WriteString(`</score-` + hierarchy + `>`)
			spanBothWithEngine(t, engine, profiles, b.String(), false, "measure", func(t *testing.T, r validation.Report) {
				es := spanResults(r, validation.SpanSlur)
				if len(es) != 4 {
					t.Fatal(es)
				}
				for _, e := range es {
					if e.Association.State != validation.EvidenceUnsupported || e.Association.Reason != "identity-scope-incomplete" || len(e.Members) != 0 {
						t.Fatalf("lost measure gap: %+v", e)
					}
				}
			})
		})
	}
}

func TestSpanRound1SingletonInvalidNumberRemainsCausal(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	source := (localFixture{body: timelineNote("1", "", `<notations><tied type="let-ring" number="0"/></notations>`)}).xml()
	spanBothWithEngine(t, engine, profiles, source, true, "", func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanTied)
		if len(es) != 1 {
			t.Fatal(es)
		}
		e := es[0]
		if e.Number.State != validation.EvidenceInvalid || e.Number.Cause == nil {
			t.Fatal("missing scalar cause", e)
		}
		cause := *e.Number.Cause
		found := false
		for _, d := range r.Findings {
			if d.Rule == cause.Rule && d.Subject == cause.Subject && d.Outcome == validation.OutcomeFail {
				found = true
			}
		}
		if !found {
			t.Fatal("dangling scalar cause", e)
		}
		if e.Association.State != validation.EvidenceKnown || !e.Association.Value || len(e.Members) != 1 {
			t.Fatal("singleton incorrectly needs number", e)
		}
	})
}
