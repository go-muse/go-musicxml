package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func spanRound3Score(hierarchy string, bodies ...string) string {
	var b strings.Builder
	b.WriteString(`<score-` + hierarchy + ` version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list>`)
	if hierarchy == "partwise" {
		b.WriteString(`<part id="P1">`)
	}
	for _, body := range bodies {
		b.WriteString(`<measure number="1">`)
		if hierarchy == "timewise" {
			b.WriteString(`<part id="P1">`)
		}
		b.WriteString(body)
		if hierarchy == "timewise" {
			b.WriteString(`</part>`)
		}
		b.WriteString(`</measure>`)
	}
	if hierarchy == "partwise" {
		b.WriteString(`</part>`)
	}
	b.WriteString(`</score-` + hierarchy + `>`)
	return b.String()
}

func TestSpanRound3RefusedStubKeepsIdentityRunOrderIndependent(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	stub := timelineNote("1", "", `<notations><tied number="1" type="start"/><tied number="1" type="stop"/></notations>`)
	start := timelineNote("1", "", `<notations><tied number="1" type="start"/></notations>`)
	stop := timelineNote("1", "", `<notations><tied number="1" type="stop"/></notations>`)
	for _, hierarchy := range []string{"partwise", "timewise"} {
		for _, axis := range []string{"before", "after", "inside", "reverse-before", "reverse-after", "reverse-inside"} {
			t.Run(hierarchy+"/"+axis, func(t *testing.T) {
				var body string
				switch axis {
				case "before":
					body = stub + start + stop
				case "after":
					body = start + stop + stub
				case "inside":
					body = start + stub + stop
				case "reverse-before":
					body = stub + stop + start
				case "reverse-after":
					body = stop + start + stub
				case "reverse-inside":
					body = stop + stub + start
				}
				body = `<attributes><divisions>1</divisions></attributes>` + body
				spanBothWithEngine(t, engine, profiles, spanRound3Score(hierarchy, body), false, "", func(t *testing.T, r validation.Report) {
					es := spanResults(r, validation.SpanTied)
					if len(es) != 4 {
						t.Fatal(es)
					}
					for i, e := range es {
						known := false
						if (e.Association.State == validation.EvidenceKnown) != known {
							t.Errorf("order-dependent chain at endpoint %d: %+v", i, e)
						}
					}
				})
			})
		}
	}
}

func TestSpanRound3UnnamedLyricAmbiguityPositionIndependent(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	for _, hierarchy := range []string{"partwise", "timewise"} {
		for _, axis := range []string{"before", "between", "after", "stop"} {
			t.Run(hierarchy+"/"+axis, func(t *testing.T) {
				start := timelineNote("1", "", `<lyric><text>a</text><extend type="start"/></lyric>`)
				stop := timelineNote("1", "", `<lyric><extend type="stop"/></lyric>`)
				duplicate := timelineNote("1", "", `<lyric><text>b</text></lyric><lyric><text>c</text></lyric>`)
				var body string
				switch axis {
				case "before":
					body = duplicate + start + stop
				case "between":
					body = start + duplicate + stop
				case "after":
					body = start + stop + duplicate
				case "stop":
					body = start + timelineNote("1", "", `<lyric><extend type="stop"/></lyric><lyric><text>c</text></lyric>`)
				}
				body = `<attributes><divisions>1</divisions></attributes>` + body
				spanBothWithEngine(t, engine, profiles, spanRound3Score(hierarchy, body), false, "", func(t *testing.T, r validation.Report) {
					es := spanResults(r, validation.SpanLyricExtend)
					if len(es) != 2 {
						t.Fatal(es)
					}
					known := false
					for _, e := range es {
						if (e.Association.State == validation.EvidenceKnown) != known {
							t.Errorf("lyric candidate interval: %+v", e)
						}
						if !known && (e.Association.Reason != "span-lyric-line-ambiguous" || len(e.Members) != 0) {
							t.Errorf("missing local ambiguity: %+v", e)
						}
					}
				})
			})
		}
	}
}

func TestSpanRound3UnnamedAmbiguityPreservesExplicitIdentities(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	for _, hierarchy := range []string{"partwise", "timewise"} {
		for _, attribute := range []string{`number="1"`, `number="01"`, `name="verse"`, `name=""`, `number="1" name="verse"`} {
			for _, placement := range []string{"before", "inside", "after"} {
				t.Run(hierarchy+"/"+attribute+"/"+placement, func(t *testing.T) {
					start := timelineNote("1", "", `<lyric><text>a</text><extend type="start"/></lyric><lyric `+attribute+`><text>b</text><extend type="start"/></lyric>`)
					stop := timelineNote("1", "", `<lyric><extend type="stop"/></lyric><lyric `+attribute+`><extend type="stop"/></lyric>`)
					duplicate := timelineNote("1", "", `<lyric><text>x</text></lyric><lyric><text>y</text></lyric>`)
					body := start + duplicate + stop
					if placement == "before" {
						body = duplicate + start + stop
					}
					if placement == "after" {
						body = start + stop + duplicate
					}
					source := spanRound3Score(hierarchy, `<attributes><divisions>1</divisions></attributes>`+body)
					spanBothWithEngine(t, engine, profiles, source, false, "", func(t *testing.T, r validation.Report) {
						es := spanResults(r, validation.SpanLyricExtend)
						if len(es) != 4 {
							t.Fatal(es)
						}
						for i, e := range es {
							if i%2 == 0 {
								if e.Association.State != validation.EvidenceUnknown || e.Association.Reason != "span-lyric-line-ambiguous" || len(e.Members) != 0 {
									t.Errorf("unnamed ambiguity missing: %+v", e)
								}
							} else if e.Association.State != validation.EvidenceKnown || len(e.Members) != 2 {
								t.Errorf("explicit identity contaminated: %+v", e)
							}
						}
					})
				})
			}
		}
	}
}

func TestSpanRound3UnnamedAmbiguityStaysInPart(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	clean := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<lyric><text>a</text><extend type="start"/></lyric>`) + timelineNote("1", "", `<lyric><extend type="stop"/></lyric>`)
	duplicate := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<lyric><text>x</text></lyric><lyric><text>y</text></lyric>`)
	for _, hierarchy := range []string{"partwise", "timewise"} {
		for _, reversed := range []bool{false, true} {
			t.Run(hierarchy+"/"+map[bool]string{false: "first-clean", true: "last-clean"}[reversed], func(t *testing.T) {
				parts := []string{clean, duplicate}
				if reversed {
					parts = []string{duplicate, clean}
				}
				var b strings.Builder
				b.WriteString(`<score-` + hierarchy + ` version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>`)
				if hierarchy == "timewise" {
					b.WriteString(`<measure number="1">`)
				}
				for i, body := range parts {
					id := []string{"P1", "P2"}[i]
					b.WriteString(`<part id="` + id + `">`)
					if hierarchy == "partwise" {
						b.WriteString(`<measure number="1">`)
					}
					b.WriteString(body)
					if hierarchy == "partwise" {
						b.WriteString(`</measure>`)
					}
					b.WriteString(`</part>`)
				}
				if hierarchy == "timewise" {
					b.WriteString(`</measure>`)
				}
				b.WriteString(`</score-` + hierarchy + `>`)
				spanBothWithEngine(t, engine, profiles, b.String(), false, "", func(t *testing.T, r validation.Report) {
					es := spanResults(r, validation.SpanLyricExtend)
					if len(es) != 2 {
						t.Fatal(es)
					}
					for _, e := range es {
						if e.Association.State != validation.EvidenceKnown || len(e.Members) != 2 {
							t.Fatal("unrelated part contaminated", e)
						}
					}
				})
			})
		}
	}
}

func TestSpanRound3MalformedLyricIdentityPreservesCause(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	for _, hierarchy := range []string{"partwise", "timewise"} {
		t.Run(hierarchy, func(t *testing.T) {
			body := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<lyric><text>a</text><extend type="start"/></lyric>`) + timelineNote("1", "", `<lyric number=""><text>x</text></lyric>`) + timelineNote("1", "", `<lyric><extend type="stop"/></lyric>`)
			spanBothWithEngine(t, engine, profiles, spanRound3Score(hierarchy, body), true, "", func(t *testing.T, r validation.Report) {
				es := spanResults(r, validation.SpanLyricExtend)
				if len(es) != 2 {
					t.Fatal(es)
				}
				for _, e := range es {
					if e.Association.State != validation.EvidenceInvalid || e.Association.Cause == nil || len(e.Members) != 0 {
						t.Fatal("malformed identity cause lost", e)
					}
					found := false
					for _, f := range r.Findings {
						if f.Rule == e.Association.Cause.Rule && f.Subject == e.Association.Cause.Subject && f.Outcome == validation.OutcomeFail {
							found = true
						}
					}
					if !found {
						t.Fatal("dangling cause", e)
					}
				}
			})
		})
	}
}

func TestSpanRound3ExplicitDuplicateRemainsEndpointLocal(t *testing.T) {
	engine, profiles := spanTestEngine(t, true)
	for _, hierarchy := range []string{"partwise", "timewise"} {
		for _, where := range []string{"middle", "endpoint"} {
			t.Run(hierarchy+"/"+where, func(t *testing.T) {
				start := timelineNote("1", "", `<lyric number="1"><text>a</text><extend type="start"/></lyric>`)
				middle := timelineNote("1", "", `<lyric number="1"><text>x</text></lyric><lyric number="1"><text>y</text></lyric>`)
				stopExtra := ""
				if where == "endpoint" {
					stopExtra = `<lyric number="1"><text>z</text></lyric>`
					middle = ""
				}
				stop := timelineNote("1", "", `<lyric number="1"><extend type="stop"/></lyric>`+stopExtra)
				spanBothWithEngine(t, engine, profiles, spanRound3Score(hierarchy, `<attributes><divisions>1</divisions></attributes>`+start+middle+stop), false, "", func(t *testing.T, r validation.Report) {
					es := spanResults(r, validation.SpanLyricExtend)
					if len(es) != 2 {
						t.Fatal(es)
					}
					for _, e := range es {
						if (e.Association.State == validation.EvidenceKnown) != (where == "middle") {
							t.Fatal("explicit duplicate scope changed", e)
						}
					}
				})
			})
		}
	}
}
