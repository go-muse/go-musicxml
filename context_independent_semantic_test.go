package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func independentContextReport(t *testing.T, source string, model bool) validation.Report {
	t.Helper()
	var report validation.Report
	var err error
	if model {
		doc, e := Decode(strings.NewReader(source))
		if e != nil {
			t.Fatal(e)
		}
		report, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
	} else {
		report, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceContext})
	}
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func independentContextScore(header string, bodies [][]string, timewise bool) string {
	head := header + `<part-list>`
	for p := range bodies {
		head += fmt.Sprintf(`<score-part id="P%d"><part-name>Part</part-name></score-part>`, p+1)
	}
	head += `</part-list>`
	if timewise {
		out := `<score-timewise version="4.0">` + head
		for m := range bodies[0] {
			out += fmt.Sprintf(`<measure number="%d">`, m+1)
			for p := range bodies {
				out += fmt.Sprintf(`<part id="P%d">%s</part>`, p+1, bodies[p][m])
			}
			out += `</measure>`
		}
		return out + `</score-timewise>`
	}
	out := `<score-partwise version="4.0">` + head
	for p := range bodies {
		out += fmt.Sprintf(`<part id="P%d">`, p+1)
		for m, body := range bodies[p] {
			out += fmt.Sprintf(`<measure number="%d">%s</measure>`, m+1, body)
		}
		out += `</part>`
	}
	return out + `</score-partwise>`
}

// MusicXML4.0 print-attributes: blank-page inserts physical blank pages before
// the new page. Odd/even default margins depend on physical page, once per page,
// and cannot alternate among simultaneous parts.
func TestIndependentContextBlankPageParity(t *testing.T) {
	header := `<identification><encoding><supports element="print" attribute="new-page" value="yes" type="yes"/><supports element="print" attribute="new-system" value="yes" type="yes"/></encoding></identification><defaults><page-layout><page-height>100</page-height><page-width>100</page-width><page-margins type="odd"><left-margin>11</left-margin><right-margin>11</right-margin><top-margin>11</top-margin><bottom-margin>11</bottom-margin></page-margins><page-margins type="even"><left-margin>22</left-margin><right-margin>22</right-margin><top-margin>22</top-margin><bottom-margin>22</bottom-margin></page-margins></page-layout></defaults>`
	for _, parts := range []int{1, 2} {
		bodies := make([][]string, parts)
		for p := range bodies {
			bodies[p] = []string{`<attributes><staves>1</staves></attributes>`, `<print new-page="yes" blank-page="1"/>`, `<print/>`, `<print new-page="yes"/>`}
		}
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("parts%d/timewise%t/model%t", parts, tw, model), func(t *testing.T) {
					report := independentContextReport(t, independentContextScore(header, bodies, tw), model)
					facts := layoutFacts(report, "page-left-margin")
					if len(facts) != 4*parts {
						t.Fatalf("got %d margins", len(facts))
					}
					for i, fact := range facts {
						want := "11"
						if i/parts == 3 {
							want = "22"
						}
						if fact.Value.State != validation.EvidenceKnown || fact.Value.Value.Decimal.String() != want {
							t.Errorf("measure%d part%d margin=%+v want %s", i/parts+1, i%parts+1, fact.Value, want)
						}
					}
				})
			}
		}
	}
}

// A direction-associated dynamics placement is ignored even if its containing
// direction has no explicit placement (MusicXML4.0 dynamics documentation).
func TestIndependentContextDynamicsPlacementIgnored(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, parent := range []string{"", ` placement="below"`} {
				t.Run(fmt.Sprintf("timewise%t/model%t/parent%s", tw, model, parent), func(t *testing.T) {
					body := `<direction` + parent + `><direction-type><dynamics placement="above"><p/></dynamics></direction-type></direction>`
					report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
					facts := layoutFacts(report, "placement")
					if len(facts) != 1 {
						t.Fatalf("got %d facts", len(facts))
					}
					got := facts[0]
					if got.Layout.Raw.State != validation.EvidenceKnown || got.Layout.Raw.Value.Text != "above" {
						t.Fatalf("raw placement lost: %+v", got)
					}
					if parent == "" {
						if got.Value.State == validation.EvidenceKnown {
							t.Errorf("ignored placement promoted: %+v", got.Value)
						}
					} else if got.Value.State != validation.EvidenceKnown || got.Value.Value.Text != "below" {
						t.Errorf("parent placement lost: %+v", got.Value)
					}
				})
			}
		}
	}
}

// The default word font cannot supply missing music-font properties. The
// defaults prose expressly leaves missing defaults to the application.
func TestIndependentContextFontDefaultDomains(t *testing.T) {
	header := `<defaults><music-font font-size="20"/><word-font font-family="Words"/></defaults>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			t.Run(fmt.Sprintf("timewise%t/model%t", tw, model), func(t *testing.T) {
				body := `<direction><direction-type><words>dolce</words></direction-type></direction>`
				report := independentContextReport(t, independentContextScore(header, [][]string{{body}}, tw), model)
				facts := layoutFacts(report, "font-family")
				if len(facts) != 4 {
					t.Fatalf("expected music-font,word-font,part-name,words; got %d: %+v", len(facts), facts)
				}
				if facts[0].Value.State == validation.EvidenceKnown {
					t.Errorf("music-font family inherited from word-font: %+v", facts[0])
				}
				if facts[len(facts)-1].Value.State != validation.EvidenceKnown || facts[len(facts)-1].Value.Value.Text != "Words" {
					t.Errorf("word-font should apply to words: %+v", facts[len(facts)-1])
				}
			})
		}
	}
}
