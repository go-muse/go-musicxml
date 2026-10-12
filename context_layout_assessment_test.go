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
func TestLayoutStaffDefaultsApplyAcrossParts(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			header := `<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults><part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>`
			source := `<score-partwise>` + header + `<part id="P1"><measure number="1"><print/></measure></part><part id="P2"><measure number="1"><print/></measure></part></score-partwise>`
			if tw {
				source = `<score-timewise>` + header + `<measure number="1"><part id="P1"><print/></part><part id="P2"><print/></part></measure></score-timewise>`
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
			values := layoutFacts(r, "staff-distance")
			if len(values) != 2 {
				t.Fatalf("got %d staff scopes", len(values))
			}
			for i, v := range values {
				if v.Value.Value.Decimal.String() != "12" || v.Layout.Staff.Value.String() != "1" || v.Layout.Applicability.State != validation.EvidenceKnown || v.Layout.Applicability.Value != (i == 1) {
					t.Errorf("tw%v model%v staff%d=%+v payload%+v", tw, model, i, v, v.Layout)
				}
			}
		}
	}
}
func TestLayoutPageMarginsAndNegativeSystemInsets(t *testing.T) {
	layoutReports(t, `<print><system-layout><system-margins><left-margin>-5</left-margin><right-margin>3</right-margin></system-margins></system-layout></print>`, `<defaults><page-layout><page-height>100</page-height><page-width>80</page-width><page-margins><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins></page-layout></defaults>`, func(t *testing.T, r validation.Report) {
		for _, tc := range []struct{ property, want string }{{"effective-left-margin", "5"}, {"effective-right-margin", "13"}} {
			v := layoutFacts(r, tc.property)
			if len(v) != 1 || v[0].Layout.Conversion.State != validation.EvidenceKnown || v[0].Layout.Conversion.Value.String() != tc.want {
				t.Fatalf("%s got %+v", tc.property, v)
			}
		}
	})
}
func TestLayoutSystemDividerMidpointAndWidthMismatch(t *testing.T) {
	layoutReports(t, `<print/>`, `<defaults><page-layout><page-height>100</page-height><page-width>100</page-width><page-margins><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins></page-layout><system-layout><system-margins><left-margin>0</left-margin><right-margin>0</right-margin></system-margins><system-distance>30</system-distance><system-dividers><left-divider/><right-divider/></system-dividers></system-layout></defaults>`, func(t *testing.T, r validation.Report) {
		v := layoutFacts(r, "left-divider-y")
		if len(v) != 1 || v[0].Layout.Conversion.Value.String() != "15" || v[0].Layout.Applicability.Value {
			t.Fatalf("divider %+v", v)
		}
	})
}
func TestLayoutScalarDefaultsUseSpecificFields(t *testing.T) {
	tests := []struct {
		name, body, header, property, text string
		state                              validation.EvidenceState
	}{
		{"valign", `<direction><direction-type><words>text</words></direction-type></direction>`, "", "valign", "", validation.EvidenceUnknown},
		{"font-app", `<direction><direction-type><words>text</words></direction-type></direction>`, "", "font-family", "", validation.EvidenceUnknown},
		{"line-height", `<direction><direction-type><words>text</words></direction-type></direction>`, "", "line-height", "normal", validation.EvidenceKnown},
		{"words-enclosure", `<direction><direction-type><words>text</words></direction-type></direction>`, "", "enclosure", "none", validation.EvidenceKnown},
		{"words-language", `<direction><direction-type><words>text</words></direction-type></direction>`, "", "language", "it", validation.EvidenceKnown},
		{"rehearsal-enclosure", `<direction><direction-type><rehearsal>A</rehearsal></direction-type></direction>`, "", "enclosure", "square", validation.EvidenceKnown},
		{"rehearsal-justify", `<direction><direction-type><rehearsal>A</rehearsal></direction-type></direction>`, "", "justify", "left", validation.EvidenceKnown},
		{"note-lyric-defaults", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><lyric><text>la</text></lyric></note>`, "", "lyric-placement", "below", validation.EvidenceKnown},
		{"name-display", `<print><part-name-display><display-text>part</display-text></part-name-display></print>`, "", "enclosure", "none", validation.EvidenceKnown},
		{"accidental-language", `<print><part-name-display><accidental-text>sharp</accidental-text></part-name-display></print>`, "", "language", "it", validation.EvidenceKnown},
		{"kind-degrees", `<harmony><root><root-step>C</root-step></root><kind>major</kind></harmony>`, "", "kind-stack-degrees", "", validation.EvidenceUnknown},
		{"kind-symbol", `<harmony><root><root-step>C</root-step></root><kind>major</kind></harmony>`, "", "kind-use-symbols", "", validation.EvidenceUnknown},
		{"breath-symbol", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><articulations><breath-mark/></articulations></notations></note>`, "", "breath-mark-symbol", "", validation.EvidenceUnknown},
		{"tap-text", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><tap hand="left">T</tap></technical></notations></note>`, "", "display", "T", validation.EvidenceKnown},
		{"tap-fallback", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><tap hand="left"/></technical></notations></note>`, "", "display", "left", validation.EvidenceKnown},
		{"elision-fallback", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><lyric><text>l</text><elision smufl="lyricsElisionNarrow"/><text>a</text></lyric></note>`, "", "display", "lyricsElisionNarrow", validation.EvidenceKnown},
		{"arrowhead", `<direction><direction-type><words>text</words></direction-type></direction><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><arrow><arrow-direction>up</arrow-direction><arrowhead/></arrow></technical></notations></note>`, "", "geometry", "head-only", validation.EvidenceKnown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			layoutReports(t, tc.body, tc.header, func(t *testing.T, r validation.Report) {
				v := layoutFacts(r, tc.property)
				if len(v) == 0 {
					t.Fatalf("missing %s", tc.property)
				}
				got := v[len(v)-1]
				if got.Value.State != tc.state || got.Value.Value.Text != tc.text {
					t.Fatalf("got %+v", got)
				}
			})
		})
	}
}
func TestLayoutHarmonyAndMetronomeComponentInheritance(t *testing.T) {
	for _, body := range []string{`<harmony font-family="Parent" font-style="italic"><root><root-step font-family="Child">C</root-step></root><kind>major</kind></harmony>`, `<direction><direction-type><metronome font-family="Parent" font-style="italic"><beat-unit>quarter</beat-unit><per-minute font-family="Child">120ish</per-minute></metronome></direction-type></direction>`} {
		layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
			families := layoutFacts(r, "font-family")
			styles := layoutFacts(r, "font-style")
			found := false
			for _, v := range families {
				if v.Value.State == validation.EvidenceKnown && v.Value.Value.Text == "Child" {
					found = true
					matched := false
					for _, s := range styles {
						if s.Subject == v.Subject && s.Value.Value.Text == "italic" {
							matched = true
						}
					}
					if !matched {
						t.Fatal("component inheritance lost")
					}
				}
			}
			if !found {
				t.Fatal("explicit child missing")
			}
		})
	}
}
