package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

const layoutRound1Supports = `<identification><encoding><supports element="print" attribute="new-page" value="yes" type="yes"/><supports element="print" attribute="new-system" value="yes" type="yes"/></encoding></identification>`

// Each matrix entry independently exercises both authentic input adapters and
// both score hierarchies. Measures are grouped by ordinal; parts remain distinct.
func layoutRound1Reports(t *testing.T, header string, parts [][]string, widths []string, check func(*testing.T, validation.Report)) {
	t.Helper()
	for _, timewise := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			t.Run(fmt.Sprintf("timewise=%t/model=%t", timewise, model), func(t *testing.T) {
				var xml strings.Builder
				root := "score-partwise"
				if timewise {
					root = "score-timewise"
				}
				xml.WriteString("<" + root + ">" + header + "<part-list>")
				for p := range parts {
					fmt.Fprintf(&xml, `<score-part id="P%d"><part-name>Part</part-name></score-part>`, p+1)
				}
				xml.WriteString("</part-list>")
				measure := func(m int) {
					fmt.Fprintf(&xml, `<measure number="%d"`, m+1)
					if m < len(widths) && widths[m] != "" {
						fmt.Fprintf(&xml, ` width="%s"`, widths[m])
					}
					xml.WriteString(">")
				}
				if timewise {
					for m := range parts[0] {
						measure(m)
						for p := range parts {
							fmt.Fprintf(&xml, `<part id="P%d">%s</part>`, p+1, parts[p][m])
						}
						xml.WriteString("</measure>")
					}
				} else {
					for p, ms := range parts {
						fmt.Fprintf(&xml, `<part id="P%d">`, p+1)
						for m, body := range ms {
							measure(m)
							xml.WriteString(body + "</measure>")
						}
						xml.WriteString("</part>")
					}
				}
				xml.WriteString("</" + root + ">")
				var report validation.Report
				var err error
				if model {
					doc, e := Decode(strings.NewReader(xml.String()))
					if e != nil {
						t.Fatal(e)
					}
					report, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
				} else {
					report, err = InspectXML(context.Background(), strings.NewReader(xml.String()), SourceOptions{Profile: ProfileSourceContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				check(t, report)
			})
		}
	}
}

func TestLayoutRound1StaffLifetimeIndependentOfSystemPublisher(t *testing.T) {
	t.Parallel()
	layoutRound1Reports(t, `<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`, [][]string{{`<print><staff-layout><staff-distance>22</staff-distance></staff-layout></print>`, `<print><system-layout><system-distance>30</system-distance></system-layout></print>`}}, nil, func(t *testing.T, r validation.Report) {
		layoutMatrixValues(t, r, "staff-distance", lmk("22"), lmu())
		values := layoutFacts(r, "staff-distance")
		if values[1].Value.Reason != "staff-override-lifetime-unavailable" {
			t.Errorf("lost lifetime reason: %+v", values[1].Value)
		}
	})
}

func TestLayoutRound1UnknownVisiblePublisherDoesNotChooseDefaults(t *testing.T) {
	t.Parallel()
	layoutRound1Reports(t, `<defaults><system-layout><system-margins><left-margin>1</left-margin><right-margin>2</right-margin></system-margins><system-distance>20</system-distance></system-layout></defaults>`, [][]string{{`<attributes><staff-details print-object="invalid"/></attributes>`}, {`<print><system-layout><system-margins><left-margin>8</left-margin><right-margin>9</right-margin></system-margins><system-distance>73</system-distance></system-layout></print>`}}, nil, func(t *testing.T, r validation.Report) {
		for _, p := range []string{"system-distance", "system-left-margin", "system-right-margin"} {
			for _, row := range layoutFacts(r, p) {
				if row.Value.State == validation.EvidenceKnown || row.Value.State == validation.EvidenceAbsent {
					t.Errorf("%s default chosen with unknown publisher: %+v", p, row.Value)
				}
			}
		}
	})
}

func TestLayoutRound1MultiplePrintEvidenceNotAbsence(t *testing.T) {
	t.Parallel()
	layoutReports(t, `<print><staff-layout><staff-distance>22</staff-distance></staff-layout><measure-layout><measure-distance>7</measure-distance></measure-layout><part-name-display><display-text>New</display-text></part-name-display></print><print new-system="yes"/>`, `<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`, func(t *testing.T, r validation.Report) {
		layoutMatrixValues(t, r, "measure-distance", lmu())
		layoutMatrixValues(t, r, "staff-distance", lmu())
		names := layoutFacts(r, "name-display-onset")
		if len(names) != 1 {
			t.Fatalf("lost name update: %+v", names)
		}
		if names[0].Layout.System.State != validation.EvidenceUnknown {
			t.Errorf("ambiguous print onset: %+v", names[0].Layout)
		}
	})
}

func TestLayoutRound1ParallelBreakIsTransactional(t *testing.T) {
	t.Parallel()
	margins := `<page-layout><page-margins type="odd"><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins><page-margins type="even"><left-margin>20</left-margin><right-margin>20</right-margin><top-margin>20</top-margin><bottom-margin>20</bottom-margin></page-margins></page-layout>`
	header := layoutRound1Supports + `<defaults>` + margins + `<system-layout><system-distance>20</system-distance></system-layout><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`
	layoutRound1Reports(t, header, [][]string{{`<print><system-layout><system-distance>30</system-distance></system-layout><staff-layout><staff-distance>22</staff-distance></staff-layout></print>`, `<print new-page="yes"/>`}, {``, ``}}, nil, func(t *testing.T, r validation.Report) {
		for _, p := range []string{"page-left-margin", "system-distance"} {
			rows := layoutFacts(r, p)
			if len(rows) != 4 {
				t.Fatalf("%s: %d", p, len(rows))
			}
			for _, row := range rows[2:] {
				if row.Value.State != validation.EvidenceUnknown {
					t.Errorf("%s after disputed break: %+v", p, row.Value)
				}
			}
		}
		rows := layoutFacts(r, "staff-distance")
		if len(rows) != 4 || rows[2].Value.State != validation.EvidenceUnknown || rows[3].Value.State != validation.EvidenceKnown {
			t.Errorf("part-local uncertain override: %+v", rows)
		}
	})
}

func TestLayoutRound1AttributeLessSupportsIsIrrelevant(t *testing.T) {
	t.Parallel()
	header := strings.Replace(layoutRound1Supports, "<encoding>", `<encoding><supports element="print" type="yes"/>`, 1)
	layoutRound1Reports(t, header, [][]string{{`<print/>`, `<print/>`}}, nil, func(t *testing.T, r validation.Report) {
		for _, row := range layoutFacts(r, "page-height") {
			if row.Layout.Page.State != validation.EvidenceKnown || row.Layout.System.State != validation.EvidenceKnown {
				t.Errorf("irrelevant support contaminated scope: %+v", row.Layout)
			}
		}
	})
}

func TestLayoutRound1StaffSelectorGapIsPartAndSystemLocal(t *testing.T) {
	t.Parallel()
	header := layoutRound1Supports + `<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`
	layoutRound1Reports(t, header, [][]string{{`<print><staff-layout number="0"><staff-distance>22</staff-distance></staff-layout></print>`, `<print new-system="yes"/>`}, {``, `<print new-system="yes"/>`}}, nil, func(t *testing.T, r validation.Report) {
		rows := layoutFacts(r, "staff-distance")
		if len(rows) != 4 {
			t.Fatalf("rows %+v", rows)
		}
		if rows[0].Value.State != validation.EvidenceInvalid || rows[0].Value.Cause == nil {
			t.Errorf("selector cause: %+v", rows[0].Value)
		}
		for _, row := range rows[1:] {
			if row.Value.State != validation.EvidenceKnown || row.Value.Value.Decimal.String() != "12" {
				t.Errorf("selector gap spread: %+v", row)
			}
		}
	})
}

func TestLayoutRound1SparseStaffKeepsSelectorWithoutDefault(t *testing.T) {
	t.Parallel()
	layoutReports(t, `<print><staff-layout number="2"/></print>`, `<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`, func(t *testing.T, r validation.Report) {
		rows := layoutFacts(r, "staff-distance")
		if len(rows) != 2 {
			t.Fatalf("rows %+v", rows)
		}
		if rows[1].Layout.Staff.State != validation.EvidenceKnown || rows[1].Layout.Staff.Value.String() != "2" || rows[1].Value.State != validation.EvidenceAbsent {
			t.Errorf("empty observed selector lost: %+v", rows[1])
		}
	})
}

func TestLayoutRound1PartialSystemWidthCannotBeConsistent(t *testing.T) {
	t.Parallel()
	header := layoutRound1Supports + `<defaults><page-layout><page-height>100</page-height><page-width>100</page-width><page-margins><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins></page-layout><system-layout><system-margins><left-margin>0</left-margin><right-margin>0</right-margin></system-margins></system-layout></defaults>`
	layoutRound1Reports(t, header, [][]string{{``, ``, `<print new-system="yes"/>`, ``}, {``, ``, ``, ``}}, []string{"40", "40", "40", "40"}, func(t *testing.T, r validation.Report) {
		for _, row := range layoutFacts(r, "width-outcome") {
			if row.Value.State == validation.EvidenceKnown {
				t.Errorf("partial width proved consistent: %+v", row)
			}
		}
	})
}

func TestLayoutRound1ClosedNameHasExplicitAbsentPage(t *testing.T) {
	t.Parallel()
	layoutRound1Reports(t, layoutRound1Supports, [][]string{{``, `<print><part-name-display><display-text>Next</display-text></part-name-display></print>`}}, nil, func(t *testing.T, r validation.Report) {
		rows := layoutFacts(r, "name-display-onset")
		if len(rows) != 1 {
			t.Fatalf("rows %+v", rows)
		}
		if rows[0].Layout.Page.State != validation.EvidenceAbsent || rows[0].Layout.System.State != validation.EvidenceAbsent {
			t.Errorf("uninitialized closed name scope: %+v", rows[0].Layout)
		}
	})
}

func TestLayoutRound1WordFontFallbackHasConsumerOrigin(t *testing.T) {
	t.Parallel()
	layoutReports(t, `<direction><direction-type><words>Text</words></direction-type></direction>`, `<defaults><word-font font-family="A"/></defaults>`, func(t *testing.T, r validation.Report) {
		rows := layoutMatrixRows(r, "font-style")
		row := rows[len(rows)-1]
		if row.Value.State != validation.EvidenceUnknown || row.Layout.Provenance != validation.ContextApplicationChoice || row.Layout.Origin != row.Subject {
			t.Errorf("absent word-font component owns application choice: %+v", row)
		}
	})
}

func TestLayoutRound1FreshStaffOverrideRepairsItsOwnLifetime(t *testing.T) {
	t.Parallel()
	layoutRound1Reports(t, `<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout><staff-layout number="2"><staff-distance>13</staff-distance></staff-layout></defaults>`, [][]string{{`<print><staff-layout><staff-distance>22</staff-distance></staff-layout><staff-layout number="2"><staff-distance>23</staff-distance></staff-layout></print>`, `<print><staff-layout><staff-distance>32</staff-distance></staff-layout></print>`}}, nil, func(t *testing.T, r validation.Report) {
		layoutMatrixValues(t, r, "staff-distance", lmk("22"), lmk("23"), lmk("32"), lmu())
	})
}

func TestLayoutRound1InvalidStaffSelectorWithoutDefaultsStillPublishes(t *testing.T) {
	t.Parallel()
	layoutReports(t, `<print><staff-layout number="0"><staff-distance>22</staff-distance></staff-layout></print>`, "", func(t *testing.T, r validation.Report) {
		rows := layoutFacts(r, "staff-distance")
		if len(rows) != 1 {
			t.Fatalf("invalid selector vanished: %+v", rows)
		}
		if rows[0].Value.State != validation.EvidenceInvalid || rows[0].Value.Cause == nil || rows[0].Layout.Staff.State != validation.EvidenceInvalid {
			t.Errorf("invalid selector cause lost: %+v", rows[0])
		}
	})
}

func TestLayoutRound1PercussionReferenceSourceModel(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"", "<line>1</line>", "<line>5</line>"} {
		t.Run(line, func(t *testing.T) {
			layoutReports(t, `<attributes><divisions>1</divisions><clef><sign>percussion</sign>`+line+`</clef><staff-details><staff-lines>1</staff-lines></staff-details></attributes><note><unpitched><display-step>G</display-step><display-octave>4</display-octave></unpitched><duration>1</duration></note>`, "", func(t *testing.T, r validation.Report) { layoutMatrixValues(t, r, "display-staff-position", lmk("2")) })
		})
	}
}

func TestLayoutRound1ParallelBreakUnavailableAttributeIsOrderIndependent(t *testing.T) {
	t.Parallel()
	for _, attribute := range []string{"new-system", "new-page"} {
		t.Run(attribute, func(t *testing.T) {
			for _, invalidFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("invalid-first=%t", invalidFirst), func(t *testing.T) {
					header := layoutRound1Supports + `<defaults><system-layout><system-distance>20</system-distance></system-layout><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`
					parts := [][]string{{`<print><system-layout><system-distance>30</system-distance></system-layout><staff-layout><staff-distance>22</staff-distance></staff-layout></print>`, `<print/>`}, {``, `<print ` + attribute + `="bad"/>`}}
					if invalidFirst {
						parts[0][1], parts[1][1] = parts[1][1], parts[0][1]
					}
					layoutRound1Reports(t, header, parts, nil, func(t *testing.T, r validation.Report) {
						rows := layoutFacts(r, "system-distance")
						if len(rows) != 4 {
							t.Fatalf("rows %+v", rows)
						}
						for _, row := range rows[2:] {
							if row.Layout.System.State != validation.EvidenceInvalid || row.Layout.System.Cause == nil || row.Value.State != validation.EvidenceInvalid || row.Value.Cause == nil {
								t.Errorf("unavailable parallel break was ignored: %+v", row)
							}
						}
						staff := layoutFacts(r, "staff-distance")
						if staff[2].Value.State == validation.EvidenceKnown {
							t.Errorf("uncertain override lifetime was guessed: %+v", staff[2])
						}
					})
				})
			}
		})
	}
}
