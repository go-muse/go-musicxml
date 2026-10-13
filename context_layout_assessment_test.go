package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"os"
	"os/exec"
	"path/filepath"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			parts := map[validation.NodeRef]bool{}
			for _, v := range values {
				part := v.Layout.Part
				if part.State != validation.EvidenceKnown || part.Value.Node == 0 || parts[part.Value] {
					t.Fatalf("ambiguous part identity %+v", part)
				}
				parts[part.Value] = true
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
	t.Parallel()
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
	t.Parallel()
	layoutReports(t, `<print/>`, `<defaults><page-layout><page-height>100</page-height><page-width>100</page-width><page-margins><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins></page-layout><system-layout><system-margins><left-margin>0</left-margin><right-margin>0</right-margin></system-margins><system-distance>30</system-distance><system-dividers><left-divider/><right-divider/></system-dividers></system-layout></defaults>`, func(t *testing.T, r validation.Report) {
		v := layoutFacts(r, "left-divider-y")
		if len(v) != 1 || v[0].Layout.Conversion.Value.String() != "15" || v[0].Layout.Applicability.Value {
			t.Fatalf("divider %+v", v)
		}
	})
}
func TestLayoutScalarDefaultsUseSpecificFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, header, property, text string
		state                              validation.EvidenceState
	}{

		{"image-valign", `<direction><direction-type><image source="image.png" type="image/png"/></direction-type></direction>`, "", "image-valign", "", validation.EvidenceUnknown},
		{"editorial-parentheses", `<direction><direction-type><words>x</words></direction-type><level>ed</level></direction>`, "", "editorial-parentheses", "", validation.EvidenceUnknown},
		{"unplayed-string", `<harmony><root><root-step>C</root-step></root><kind>major</kind><frame><frame-strings>6</frame-strings><frame-frets>4</frame-frets><frame-note><string>1</string><fret>0</fret></frame-note></frame></harmony>`, "", "unplayed-string", "", validation.EvidenceUnknown},
		{"numeral-root", `<harmony><numeral><numeral-root>1</numeral-root><numeral-key><numeral-fifths>0</numeral-fifths><numeral-mode>major</numeral-mode></numeral-key></numeral><kind>major</kind></harmony>`, "", "numeral-root-text", "", validation.EvidenceUnknown},
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
	t.Parallel()
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
func TestLayoutFormattingAndGeometryAgainstPinnedSchema(t *testing.T) {
	t.Parallel()
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal(err)
		}
		t.Skip("xmllint unavailable")
	}
	for _, tw := range []bool{false, true} {
		for _, tc := range []struct {
			body  string
			valid bool
		}{
			{`<direction><direction-type><words font-family="A" font-size="12.5" xml:lang="">x</words><words justify="right">y</words></direction-type></direction>`, true},
			{`<direction><direction-type><words justify="bad">x</words></direction-type></direction>`, false},
			{`<print new-page="no" page-number="arbitrary"><page-layout><page-height>-100.5</page-height><page-width>0</page-width></page-layout></print>`, true},
			{`<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><slur type="start" bezier-x="0" bezier-offset="-2"/></notations></note>`, true},
			{`<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><lyric number="01"><text>la</text><elision smufl="lyricsElisionNarrow"/><text>la</text></lyric></note>`, true},
		} {
			source := contextFixtureXML(tc.body, tw)
			report, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
			if err != nil {
				t.Fatal(err)
			}
			invalid := false
			for _, f := range report.Findings {
				if strings.HasPrefix(string(f.Rule.ID), "MX40-XSD-") && f.Outcome == validation.OutcomeFail {
					invalid = true
				}
			}
			if invalid == tc.valid {
				t.Fatalf("unexpected XSD result: %s", localFindings(report))
			}
			assertXMLLintOutcome(t, xmllint, filepath.Join("schema", "musicxml-4.0", "musicxml.xsd"), filepath.Join("schema", "musicxml-4.0", "catalog.xml"), source, tc.valid)
		}
	}
}
func TestLayoutHiddenSpaceAndVisiblePublisherAreSeparate(t *testing.T) {
	t.Parallel()
	for _, cfg := range []struct{ object, spacing string }{{"no", "yes"}, {"no", "no"}, {"no", "invalid"}, {"yes", "yes"}, {"invalid", "yes"}} {
		spacing, object := cfg.spacing, cfg.object
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				header := `<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults><part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>`
				a := `<attributes><staff-details print-object="` + object + `" print-spacing="` + spacing + `"/></attributes>`
				b := `<print><system-layout><system-distance>73</system-distance></system-layout></print>`
				source := `<score-partwise>` + header + `<part id="P1"><measure number="1">` + a + `</measure></part><part id="P2"><measure number="1">` + b + `</measure></part></score-partwise>`
				if tw {
					source = `<score-timewise>` + header + `<measure number="1"><part id="P1">` + a + `</part><part id="P2">` + b + `</part></measure></score-timewise>`
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
				staff := layoutFacts(r, "staff-distance")
				if len(staff) != 2 {
					t.Fatal("missing staff scopes")
				}
				if ((spacing == "invalid" || object == "invalid") && staff[1].Layout.Applicability.State == validation.EvidenceKnown) || (spacing != "invalid" && object != "invalid" && (staff[1].Layout.Applicability.State != validation.EvidenceKnown || staff[1].Layout.Applicability.Value != (object == "yes" || spacing == "yes"))) {
					t.Errorf("spacing=%s tw=%v model=%v P2=%+v", spacing, tw, model, staff[1].Layout)
				}
				system := layoutFacts(r, "system-distance")
				if len(system) != 2 || (object == "no" && (system[0].Value.Value.Decimal.String() != "73" || system[1].Value.Value.Decimal.String() != "73")) || (object != "no" && (system[0].Value.State == validation.EvidenceKnown || system[1].Value.State == validation.EvidenceKnown)) {
					t.Errorf("wrong first visible publisher %+v", system)
				}
			}
		}
	}
}
func TestLayoutSystemSelectorsAndAdvisoriesRemainContext(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ body, property, want string }{
		{`<direction system="none"><direction-type><words>text</words></direction-type></direction>`, "system", "none"},
		{`<direction system="also-top"><direction-type><words>text</words></direction-type></direction>`, "system", "also-top"},
		{`<print><measure-numbering system="also-bottom">system</measure-numbering></print>`, "system", "also-bottom"},
		{`<direction><direction-type><words font-size="small">text</words></direction-type></direction>`, "font-size", "small"},
		{`<harmony><root><root-step>C</root-step></root><kind use-symbols="yes" text="maj">major</kind></harmony>`, "kind-text-placement", "after-symbol"},
	} {
		layoutReports(t, tc.body, "", func(t *testing.T, r validation.Report) {
			values := layoutFacts(r, tc.property)
			found := false
			for _, v := range values {
				if v.Value.State == validation.EvidenceKnown && v.Value.Value.Text == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s=%s", tc.property, tc.want)
			}
			for _, f := range r.Findings {
				if strings.HasPrefix(string(f.Rule.ID), "MX40-CONTEXT-layout") && f.Outcome == validation.OutcomeFail {
					t.Fatalf("advisory strengthened %+v", f)
				}
			}
		})
	}
}
func TestLayoutZeroScalingAndLyricAlignmentAreIndependent(t *testing.T) {
	t.Parallel()
	layoutReports(t, `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><lyric><text>la</text></lyric></note>`, `<defaults><scaling><millimeters>7</millimeters><tenths>0</tenths></scaling></defaults>`, func(t *testing.T, r validation.Report) {
		scaling := layoutFacts(r, "millimeters-per-tenth")
		if len(scaling) != 1 || scaling[0].Layout.Conversion.State != validation.EvidenceUnknown || scaling[0].Layout.Conversion.Reason != "zero-scaling-denominator" {
			t.Fatalf("zero scaling %+v", scaling)
		}
		for _, tc := range []struct{ property, want string }{{"justify", "center"}, {"halign", "center"}, {"valign", "baseline"}} {
			found := false
			for _, v := range layoutFacts(r, tc.property) {
				if v.Value.State == validation.EvidenceKnown && v.Value.Value.Text == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("lyric %s", tc.property)
			}
		}
		for _, f := range r.Findings {
			if f.Outcome == validation.OutcomeFail && strings.HasPrefix(string(f.Rule.ID), "MX40-CONTEXT-") {
				t.Fatalf("zero scaling invented a defect %+v", f)
			}
		}
	})
}
func TestLayoutCreditInheritanceAndPhysicalPage(t *testing.T) {
	t.Parallel()
	layoutReports(t, `<print new-page="yes" page-number="99"/>`, `<credit><credit-words font-family="CreditFont" default-x="5" default-y="10">first</credit-words><credit-words>second</credit-words></credit>`, func(t *testing.T, r validation.Report) {
		page := layoutFacts(r, "credit-page")
		if len(page) != 1 || page[0].Value.Value.Integer.String() != "1" {
			t.Fatalf("printed label changed physical page %+v", page)
		}
		inherited := false
		for _, v := range layoutFacts(r, "font-family") {
			if v.Value.Value.Text == "CreditFont" && v.Layout.Provenance == validation.ContextInherited {
				inherited = true
			}
		}
		if !inherited {
			t.Fatal("credit inheritance missing")
		}
		lowerLeft := false
		absentPosition := false
		for _, v := range layoutFacts(r, "default-x") {
			if v.Layout.Basis == "physical-page-lower-left" {
				lowerLeft = true
				if v.Value.State == validation.EvidenceAbsent {
					absentPosition = true
				}
			}
		}
		if !lowerLeft || !absentPosition {
			t.Fatal("credit positional inheritance or origin wrong")
		}
	})
}
func TestLayoutWidthComparisonFiveAxes(t *testing.T) {
	t.Parallel()
	defaults := `<identification><encoding><supports element="print" attribute="new-page" value="yes" type="yes"/><supports element="print" attribute="new-system" value="yes" type="yes"/></encoding></identification><defaults><page-layout><page-height>100</page-height><page-width>100</page-width><page-margins><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins></page-layout><system-layout><system-margins><left-margin>0</left-margin><right-margin>0</right-margin></system-margins></system-layout></defaults>`
	for _, tc := range []struct {
		name, width string
		known       bool
	}{{"positive", "80", true}, {"negative", "70", false}, {"boundary", "80.000", true}, {"missing_fact", "", false}, {"interaction", "-80", false}} {
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				source := contextFixtureXML(`<print/>`, tw)
				source = strings.Replace(source, "<part-list>", defaults+"<part-list>", 1)
				if tc.width != "" {
					source = strings.Replace(source, `<measure number="A">`, `<measure number="A" width="`+tc.width+`">`, 1)
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
				values := layoutFacts(r, "width-outcome")
				if len(values) != 1 {
					t.Fatalf("%s %+v", tc.name, values)
				}
				if (values[0].Value.State == validation.EvidenceKnown) != tc.known {
					t.Errorf("%s tw%v model%v %+v", tc.name, tw, model, values[0])
				}
			}
		}
	}
}
func TestLayoutBlankPagesUsePhysicalParity(t *testing.T) {
	t.Parallel()
	header := `<defaults><page-layout><page-margins type="odd"><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins><page-margins type="even"><left-margin>20</left-margin><right-margin>20</right-margin><top-margin>20</top-margin><bottom-margin>20</bottom-margin></page-margins></page-layout></defaults>`
	layoutReports(t, `<print new-page="yes" blank-page="1" page-number="99"/>`, header, func(t *testing.T, r validation.Report) {
		v := layoutFacts(r, "page-left-margin")
		if len(v) != 1 || v[0].Value.State != validation.EvidenceKnown || v[0].Value.Value.Decimal.String() != "20" {
			t.Fatalf("physical parity %+v", v)
		}
	})
}
func TestLayoutGroupDisplayPriorityAndExplicitOverride(t *testing.T) {
	t.Parallel()
	for _, abbrev := range []bool{false, true} {
		name, display := "group-name", "group-name-display"
		if abbrev {
			name, display = "group-abbreviation", "group-abbreviation-display"
		}
		for _, mode := range []string{"absent", "present", "empty", "other"} {
			present := mode == "present" || mode == "empty"
			for _, tw := range []bool{false, true} {
				for _, model := range []bool{false, true} {
					body := `<part-group type="start" number="1"><` + name + ` font-family="Deprecated">Group</` + name + `>`
					if mode == "present" {
						body += `<` + display + `><display-text font-family="Preferred">Shown</display-text></` + display + `>`
					} else if mode == "empty" {
						body += `<` + display + `/>`
					} else if mode == "other" {
						other := "group-abbreviation-display"
						if abbrev {
							other = "group-name-display"
						}
						otherXML := `<` + other + `><display-text>Other</display-text></` + other + `>`
						if abbrev {
							body = strings.Replace(body, `<group-abbreviation`, otherXML+`<group-abbreviation`, 1)
						} else {
							body += otherXML
						}
					}
					body += `</part-group>`
					source := contextFixtureXML(`<print/>`, tw)
					source = strings.Replace(source, `<part-list>`, `<part-list>`+body, 1)
					source = strings.Replace(source, `</part-list>`, `<part-group type="stop" number="1"/></part-list>`, 1)
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
					values := layoutFacts(r, "display-active")
					if len(values) != 1 || values[0].Layout.Applicability.State != validation.EvidenceKnown || values[0].Layout.Applicability.Value == present {
						t.Fatalf("display=%v values=%+v", present, values)
					}
				}
			}
		}
	}
}

func TestLayoutWordFontDoesNotCrossNotationDomains(t *testing.T) {
	t.Parallel()
	body := `<direction><direction-type><dynamics><p/></dynamics></direction-type></direction><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type><notehead>normal</notehead></note><direction><direction-type><symbol>segno</symbol></direction-type></direction><direction><direction-type><words>dolce</words></direction-type></direction>`
	layoutReports(t, body, `<defaults><music-font font-size="20"/><word-font font-family="Words"/></defaults>`, func(t *testing.T, r validation.Report) {
		facts := layoutFacts(r, "font-family")
		if len(facts) != 8 {
			t.Fatalf("font domains: expected eight exact consumers, got %d", len(facts))
		}
		// Document order: two publishers, part name, dynamics, note, notehead,
		// symbol, words. The note and its symbol must not acquire a word font.
		for _, i := range []int{0, 3, 4, 5, 6} {
			if facts[i].Value.State == validation.EvidenceKnown {
				t.Errorf("non-word domain %d acquired %+v", i, facts[i].Value)
			}
		}
		for _, i := range []int{1, 2, 7} {
			if facts[i].Value.State != validation.EvidenceKnown || facts[i].Value.Value.Text != "Words" {
				t.Errorf("word domain %d: %+v", i, facts[i].Value)
			}
		}
	})
}

func TestLayoutParallelPrintPositionsUseMusicalOffsets(t *testing.T) {
	t.Parallel()
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, mode := range []string{"equal", "different", "rewound", "unavailable", "one-publisher"} {
				different := mode == "different" || mode == "unavailable"
				header := `<defaults><page-layout><page-height>100</page-height><page-width>80</page-width></page-layout></defaults><part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>`
				prefix := `<attributes><divisions>1</divisions></attributes>`
				note := `<note><rest/><duration>1</duration></note>`
				a := prefix + `<print new-page="yes"/>` + note
				b := a
				switch mode {
				case "different":
					b = prefix + note + `<print new-page="yes"/>`
				case "rewound":
					b = prefix + note + `<backup><duration>1</duration></backup><print new-page="yes"/>`
				case "unavailable":
					b = prefix + `<backup><duration>1</duration></backup><print new-page="yes"/>`
				case "one-publisher":
					b = prefix + note
				}
				source := `<score-partwise>` + header + `<part id="P1"><measure number="1">` + a + `</measure></part><part id="P2"><measure number="1">` + b + `</measure></part></score-partwise>`
				if tw {
					source = `<score-timewise>` + header + `<measure number="1"><part id="P1">` + a + `</part><part id="P2">` + b + `</part></measure></score-timewise>`
				}
				var report validation.Report
				var err error
				if model {
					doc, e := Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
				} else {
					report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				facts := layoutFacts(report, "page-height")
				if len(facts) != 2 {
					t.Fatalf("expected two parallel blocks, got %d", len(facts))
				}
				for _, f := range facts {
					if f.Value.State != validation.EvidenceKnown || f.Value.Value.Decimal.String() != "100" {
						t.Errorf("independent default lost: %+v", f.Value)
					}
					want := validation.EvidenceKnown
					if different {
						want = validation.EvidenceUnknown
					}
					if f.Layout.Page.State != want {
						t.Errorf("timewise=%t model=%t different=%t page=%+v", tw, model, different, f.Layout.Page)
					}
				}
			}
		}
	}
}
