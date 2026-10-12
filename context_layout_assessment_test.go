package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func layoutReports(t *testing.T, body string, header string, check func(*testing.T, validation.Report)) {
	t.Helper()
	for _, tw := range []bool{false, true} {
		source := contextFixtureXML(body, tw)
		source = strings.Replace(source, "<part-list>", header+"<part-list>", 1)
		for _, model := range []bool{false, true} {
			name := "source-partwise"
			if model {
				name = "model-partwise"
			}
			if tw {
				name += "-timewise"
			}
			t.Run(name, func(t *testing.T) {
				var r validation.Report
				var err error
				if model {
					doc, e := Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
				} else {
					r, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				check(t, r)
			})
		}
	}
}
func layoutFacts(r validation.Report, property string) []validation.ContextResult {
	var out []validation.ContextResult
	for _, v := range r.Context {
		if string(v.Property) == "layout."+property {
			out = append(out, v)
		}
	}
	return out
}
func TestLayoutFormattingFiveAxes(t *testing.T) {
	for _, tc := range []struct {
		name, body, property, text string
		state                      validation.EvidenceState
	}{
		{"positive", `<direction><direction-type><words font-family="A">one</words><words>two</words></direction-type></direction>`, "font-family", "A", validation.EvidenceKnown},
		{"negative", `<direction><direction-type><words font-family="A">one</words><words font-family="B">two</words></direction-type></direction>`, "font-family", "B", validation.EvidenceKnown},
		{"boundary", `<direction><direction-type><words xml:lang="">one</words><words>two</words></direction-type></direction>`, "language", "", validation.EvidenceKnown},
		{"missing_fact", `<direction><direction-type><words font-family="A">one</words></direction-type><direction-type><words>two</words></direction-type></direction>`, "font-family", "", validation.EvidenceUnknown},
		{"interaction", `<direction><direction-type><words justify="right" halign="left">one</words></direction-type></direction>`, "halign", "left", validation.EvidenceKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layoutReports(t, tc.body, "", func(t *testing.T, r validation.Report) {
				v := layoutFacts(r, tc.property)
				if len(v) == 0 {
					t.Fatal("missing formatting fact")
				}
				got := v[len(v)-1]
				if got.Value.State != tc.state || got.Value.Value.Text != tc.text {
					t.Fatalf("got %+v", got)
				}
			})
		})
	}
}
func TestLayoutLyricPublishersFiveAxes(t *testing.T) {
	for _, tc := range []struct {
		name, defaults, lyric, property, text string
		state                                 validation.EvidenceState
	}{
		{"positive", `<lyric-font name="verse" font-family="A"/>`, `<lyric number="01" name="verse"><text>text</text></lyric>`, "font-family", "A", validation.EvidenceKnown},
		{"negative", `<lyric-font number="1" font-family="A"/>`, `<lyric number="01"><text>text</text></lyric>`, "font-family", "", validation.EvidenceUnknown},
		{"boundary", `<lyric-font font-family="A"/><lyric-font name="verse" font-style="italic"/>`, `<lyric name="verse"><text>text</text></lyric>`, "font-family", "A", validation.EvidenceKnown},
		{"missing_fact", `<lyric-font font-family="A"/><lyric-font name="verse" font-family="B"/>`, `<lyric name="verse"><text>text</text></lyric>`, "font-family", "", validation.EvidenceUnknown},
		{"interaction", `<lyric-font font-family="A"/><lyric-font name="verse" font-family="B"/>`, `<lyric name="verse"><text font-family="C">text</text></lyric>`, "font-family", "C", validation.EvidenceKnown},
		{"language", `<lyric-language number="01" xml:lang="fr"/>`, `<lyric number="01"><text>text</text></lyric>`, "language", "fr", validation.EvidenceKnown},
		{"empty-language", `<lyric-language xml:lang="fr"/>`, `<lyric><text xml:lang="">text</text></lyric>`, "language", "", validation.EvidenceKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layoutReports(t, `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration>`+tc.lyric+`</note>`, `<defaults>`+tc.defaults+`</defaults>`, func(t *testing.T, r validation.Report) {
				v := layoutFacts(r, tc.property)
				if len(v) == 0 {
					t.Fatal("no lyric value")
				}
				got := v[len(v)-1]
				if got.Value.State != tc.state || got.Value.Value.Text != tc.text {
					t.Fatalf("got %+v", got)
				}
			})
		})
	}
}
func TestLayoutPageSystemMeasureLifetimes(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			header := `<identification><encoding><supports element="print" attribute="new-page" value="yes" type="yes"/><supports element="print" attribute="new-system" value="yes" type="yes"/></encoding></identification><defaults><page-layout><page-height>100</page-height><page-width>80</page-width></page-layout><system-layout><system-distance>20</system-distance><top-system-distance>10</top-system-distance></system-layout></defaults><part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list>`
			bodies := []string{`<print><page-layout><page-height>200</page-height><page-width>160</page-width></page-layout><system-layout><system-distance>30</system-distance></system-layout><measure-layout><measure-distance>7</measure-distance></measure-layout></print>`, `<print new-system="yes"/>`, `<print new-page="yes"/>`}
			source := `<score-partwise>` + header + `<part id="P1">`
			if tw {
				source = `<score-timewise>` + header
			}
			for i, b := range bodies {
				if tw {
					source += `<measure number="` + string(rune('1'+i)) + `"><part id="P1">` + b + `</part></measure>`
				} else {
					source += `<measure number="` + string(rune('1'+i)) + `">` + b + `</measure>`
				}
			}
			if tw {
				source += `</score-timewise>`
			} else {
				source += `</part></score-partwise>`
			}
			var r validation.Report
			var err error
			if model {
				doc, e := Decode(strings.NewReader(source))
				if e != nil {
					t.Fatal(e)
				}
				r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
			} else {
				r, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				property string
				want     []string
			}{{"page-height", []string{"200", "200", "100"}}, {"system-distance", []string{"30", "20", "20"}}} {
				values := layoutFacts(r, tc.property)
				if len(values) != 3 {
					t.Errorf("%s: got %d want 3", tc.property, len(values))
					continue
				}
				for i, v := range values {
					if v.Value.State != validation.EvidenceKnown || v.Value.Value.Decimal.String() != tc.want[i] {
						t.Errorf("tw=%v model=%v %s[%d]=%+v", tw, model, tc.property, i, v)
					}
				}
			}
		}
	}
}
func TestLayoutGeometryPrecedenceFiveAxes(t *testing.T) {
	for _, tc := range []struct {
		name, body, property string
		active               bool
		state                validation.EvidenceState
	}{
		{"positive", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><slur type="start" bezier-x="2" bezier-offset="3"/></notations></note>`, "bezier-offset", false, validation.EvidenceKnown},
		{"negative", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><slur type="start" bezier-offset="3"/></notations></note>`, "bezier-offset", true, validation.EvidenceKnown},
		{"boundary", `<direction directive="yes"><direction-type><words default-x="0">text</words></direction-type></direction>`, "directive", false, validation.EvidenceKnown},
		{"missing_fact", `<direction directive="yes"><direction-type><words justify="bad">text</words></direction-type></direction>`, "directive", false, validation.EvidenceInvalid},
		{"interaction", `<print new-page="no" page-number="x"/>`, "page-number", false, validation.EvidenceKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layoutReports(t, tc.body, "", func(t *testing.T, r validation.Report) {
				v := layoutFacts(r, tc.property)
				if len(v) == 0 {
					t.Fatal("no geometry fact")
				}
				got := v[len(v)-1]
				if got.Layout == nil || got.Layout.Applicability.State != tc.state || got.Layout.Applicability.Value != tc.active {
					t.Fatalf("got %+v", got)
				}
			})
		})
	}
}
