package musicxml

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// These are adapter fixtures: each source is independently inspected, and each
// decoded current model is independently assessed. No result is obtained by
// encoding a model, and no target label substitutes for exercising an adapter.
type layoutMatrixValue struct {
	state validation.EvidenceState
	text  string
}

func lmk(s string) layoutMatrixValue { return layoutMatrixValue{validation.EvidenceKnown, s} }
func lmu() layoutMatrixValue         { return layoutMatrixValue{state: validation.EvidenceUnknown} }
func lmi() layoutMatrixValue         { return layoutMatrixValue{state: validation.EvidenceInvalid} }
func lma() layoutMatrixValue         { return layoutMatrixValue{state: validation.EvidenceAbsent} }

// The required score-part/part-name is fixture scaffolding, not a test
// subject. Identify it through the observer's concrete path instead of
// assuming a node number or dropping an arbitrary first result.
func layoutMatrixRows(r validation.Report, property string) []validation.ContextResult {
	scaffolding := make(map[validation.NodeRef]bool)
	for _, row := range r.Context {
		for _, loc := range row.Origins {
			if strings.HasSuffix(loc.Path, "/part-list/score-part/part-name") || strings.HasSuffix(loc.Path, ".ScorePart.PartName") {
				scaffolding[row.Subject] = true
			}
		}
	}
	var out []validation.ContextResult
	for _, row := range layoutFacts(r, property) {
		if !scaffolding[row.Subject] {
			out = append(out, row)
		}
	}
	return out
}
func layoutMatrixValues(t *testing.T, r validation.Report, property string, want ...layoutMatrixValue) {
	t.Helper()
	rows := layoutMatrixRows(r, property)
	if len(rows) != len(want) {
		t.Fatalf("%s: got %d rows, want %d: %+v", property, len(rows), len(want), rows)
	}
	for i, row := range rows {
		got := row.Value.Value.Text
		switch row.Value.Value.Kind {
		case validation.AtomicInteger:
			got = row.Value.Value.Integer.String()
		case validation.AtomicDecimal:
			got = row.Value.Value.Decimal.String()
		}
		if row.Value.State != want[i].state || (want[i].state == validation.EvidenceKnown && got != want[i].text) {
			t.Errorf("%s[%d]: got %+v (%q), want %+v", property, i, row.Value, got, want[i])
		}
		if want[i].state == validation.EvidenceInvalid && row.Value.Cause == nil {
			t.Errorf("%s[%d]: invalid prerequisite lost causal reference", property, i)
		}
	}
}
func layoutMatrixNoDefect(t *testing.T, r validation.Report) {
	t.Helper()
	for _, f := range r.Findings {
		if strings.HasPrefix(string(f.Rule.ID), "MX40-CONTEXT-layout") && f.Outcome == validation.OutcomeFail {
			t.Errorf("context/advice invented a normative defect: %+v", f)
		}
	}
}
func layoutMatrixWords(attrs string) string {
	return `<direction><direction-type><words ` + attrs + `>text</words></direction-type></direction>`
}
func layoutMatrixNote(child string) string {
	return `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration>` + child + `</note>`
}
func layoutMatrixHarmony(kind string) string {
	return `<harmony><root><root-step>C</root-step></root>` + kind + `</harmony>`
}

func TestLayoutMatrixFormattingFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		check, axis, body, property string
		want                        []layoutMatrixValue
	}{
		{"valign-app-default", "positive", layoutMatrixWords(`valign="top"`), "valign", []layoutMatrixValue{lmk("top")}},
		{"valign-app-default", "negative", layoutMatrixWords(`valign="sideways"`), "valign", []layoutMatrixValue{lmi()}},
		{"valign-app-default", "boundary", layoutMatrixWords(`valign="baseline"`), "valign", []layoutMatrixValue{lmk("baseline")}},
		{"valign-app-default", "missing_fact", layoutMatrixWords(""), "valign", []layoutMatrixValue{lmu()}},
		{"valign-app-default", "interaction", layoutMatrixWords(`valign="bottom"`) + layoutMatrixWords(""), "valign", []layoutMatrixValue{lmk("bottom"), lmu()}},
		{"font-app-default", "positive", layoutMatrixWords(`font-family="Handwritten"`), "font-family", []layoutMatrixValue{lmk("Handwritten")}},
		{"font-app-default", "negative", layoutMatrixWords(`font-style="sideways"`), "font-style", []layoutMatrixValue{lmi()}},
		{"font-app-default", "boundary", layoutMatrixWords(`font-family=""`), "font-family", []layoutMatrixValue{lmi()}},
		{"font-app-default", "missing_fact", layoutMatrixWords(""), "font-family", []layoutMatrixValue{lmu()}},
		{"font-app-default", "interaction", layoutMatrixWords(`font-family="One"`) + layoutMatrixWords(""), "font-family", []layoutMatrixValue{lmk("One"), lmu()}},
		{"halign-fallback", "positive", layoutMatrixWords(`justify="right"`), "halign", []layoutMatrixValue{lmk("right")}},
		{"halign-fallback", "negative", layoutMatrixWords(`justify="bad"`), "halign", []layoutMatrixValue{lmi()}},
		{"halign-fallback", "boundary", layoutMatrixWords(`justify="right" halign="left"`), "halign", []layoutMatrixValue{lmk("left")}},
		{"halign-fallback", "missing_fact", `<print><part-name-display><display-text>Part</display-text></part-name-display></print>`, "halign", []layoutMatrixValue{lmu()}},
		{"halign-fallback", "interaction", layoutMatrixWords(`justify="bad" halign="center"`), "halign", []layoutMatrixValue{lmk("center")}},
		{"justify-alignment", "positive", layoutMatrixWords(`justify="right"`), "justify", []layoutMatrixValue{lmk("right")}},
		{"justify-alignment", "negative", layoutMatrixWords(`justify="bad" halign="center"`), "justify", []layoutMatrixValue{lmi()}},
		{"justify-alignment", "boundary", layoutMatrixWords(`justify="left" halign="right"`), "justify", []layoutMatrixValue{lmk("left")}},
		{"justify-alignment", "missing_fact", `<print><part-name-display><display-text>Part</display-text></part-name-display></print>`, "justify", []layoutMatrixValue{lmu()}},
		{"justify-alignment", "interaction", layoutMatrixWords(`justify="right"`) + layoutMatrixWords(""), "justify", []layoutMatrixValue{lmk("right"), lmk("left")}},
		{"line-height-default", "positive", layoutMatrixWords(""), "line-height", []layoutMatrixValue{lmk("normal")}},
		{"line-height-default", "negative", layoutMatrixWords(`line-height="high"`), "line-height", []layoutMatrixValue{lmi()}},
		{"line-height-default", "boundary", layoutMatrixWords(`line-height="0"`), "line-height", []layoutMatrixValue{lmk("0")}},
		{"line-height-default", "missing_fact", `<direction><direction-type><words line-height="bad">one</words><words>two</words></direction-type></direction>`, "line-height", []layoutMatrixValue{lmi(), lmi()}},
		{"line-height-default", "interaction", `<direction><direction-type><words line-height="bad">one</words><words line-height="125">two</words></direction-type></direction>`, "line-height", []layoutMatrixValue{lmi(), lmk("125")}},
		{"text-rotation-sign", "positive", layoutMatrixWords(`rotation="30"`), "rotation", []layoutMatrixValue{lmk("30")}},
		{"text-rotation-sign", "negative", layoutMatrixWords(`rotation="181"`), "rotation", []layoutMatrixValue{lmi()}},
		{"text-rotation-sign", "boundary", layoutMatrixWords(`rotation="-180"`), "rotation", []layoutMatrixValue{lmk("-180")}},
		{"text-rotation-sign", "missing_fact", layoutMatrixWords(""), "rotation", []layoutMatrixValue{lmu()}},
		{"text-rotation-sign", "interaction", layoutMatrixWords(`rotation="-30" justify="right" halign="left"`) + layoutMatrixWords(`rotation="0"`), "rotation", []layoutMatrixValue{lmk("-30"), lmk("0")}},
		{"css-size-guidelines", "positive", layoutMatrixWords(`font-size="medium"`), "font-size", []layoutMatrixValue{lmk("medium")}},
		{"css-size-guidelines", "negative", layoutMatrixWords(`font-size="gigantic"`), "font-size", []layoutMatrixValue{lmi()}},
		{"css-size-guidelines", "boundary", layoutMatrixWords(`font-size="12.5"`), "font-size", []layoutMatrixValue{lmk("12.5")}},
		{"css-size-guidelines", "missing_fact", layoutMatrixWords(""), "font-size", []layoutMatrixValue{lmu()}},
		{"css-size-guidelines", "interaction", `<direction><direction-type><words font-size="small">one</words><words font-size="13">two</words></direction-type></direction>`, "font-size", []layoutMatrixValue{lmk("small"), lmk("13")}},
	}
	for _, tc := range cases {
		t.Run(tc.check+"/"+tc.axis, func(t *testing.T) {
			layoutReports(t, tc.body, "", func(t *testing.T, r validation.Report) {
				layoutMatrixValues(t, r, tc.property, tc.want...)
				layoutMatrixNoDefect(t, r)
				if tc.property == "rotation" {
					for _, v := range layoutFacts(r, tc.property) {
						if v.Layout.Basis != "positive-clockwise-about-alignment-point" {
							t.Errorf("rotation basis %+v", v.Layout)
						}
					}
				}
				if tc.property == "font-size" {
					for _, v := range layoutFacts(r, tc.property) {
						if v.Value.State == validation.EvidenceKnown {
							numeric := v.Value.Value.Kind == validation.AtomicDecimal
							if (v.Layout.Unit == validation.ContextUnitPoints) != numeric {
								t.Errorf("keyword/numeric size conflated: %+v", v)
							}
						}
					}
				}
				if tc.property == "line-height" {
					for _, v := range layoutFacts(r, tc.property) {
						if v.Value.State == validation.EvidenceKnown && v.Value.Value.Text == "normal" && v.Layout.Conversion.State == validation.EvidenceKnown {
							t.Errorf("normal magnitude invented: %+v", v)
						}
					}
				}
			})
		})
	}
}

func TestLayoutMatrixUseSiteDefaults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ check, element, container, defaultEnclosure string }{
		{"words-defaults", "words", "direction", "none"}, {"rehearsal-defaults", "rehearsal", "direction", "square"}, {"name-display-defaults", "display-text", "name", "none"},
	} {
		wrap := func(attrs string) string {
			child := `<` + tc.element + ` ` + attrs + `>Text</` + tc.element + `>`
			if tc.container == "name" {
				return `<print><part-name-display>` + child + `</part-name-display></print>`
			}
			return `<direction><direction-type>` + child + `</direction-type></direction>`
		}
		for _, v := range []struct {
			axis, attrs, enclosure, language string
			invalid                          bool
		}{
			{"positive", "", tc.defaultEnclosure, "it", false},
			{"negative", `enclosure="invalid" xml:lang="fr"`, "", "fr", true},
			{"boundary", `enclosure="none" xml:lang=""`, "none", "", false},
			{"interaction", `enclosure="circle" xml:lang="de" justify="right"`, "circle", "de", false},
		} {
			t.Run(tc.check+"/"+v.axis, func(t *testing.T) {
				layoutReports(t, wrap(v.attrs), "", func(t *testing.T, r validation.Report) {
					enclosure := lmk(v.enclosure)
					if v.invalid {
						enclosure = lmi()
					}
					layoutMatrixValues(t, r, "enclosure", enclosure)
					layoutMatrixValues(t, r, "language", lmk(v.language))
					if tc.container == "direction" {
						justify := "left"
						if v.axis == "interaction" {
							justify = "right"
						}
						layoutMatrixValues(t, r, "justify", lmk(justify))
					}
				})
			})
		}
		// A predecessor from another wrapper cannot establish the carried format.
		// Name displays have no inherited font choice when score defaults are absent.
		t.Run(tc.check+"/missing_fact", func(t *testing.T) {
			body := wrap("")
			if tc.container == "direction" {
				body = `<direction><direction-type><words font-family="Earlier">one</words></direction-type><direction-type><` + tc.element + `>two</` + tc.element + `></direction-type></direction>`
			}
			layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
				want := []layoutMatrixValue{lmu()}
				if tc.container == "direction" {
					want = []layoutMatrixValue{lmk("Earlier"), lmu()}
				}
				layoutMatrixValues(t, r, "font-family", want...)
			})
		})
	}
}

func TestLayoutMatrixApplicationChoices(t *testing.T) {
	t.Parallel()
	frame := func(attrs string) string {
		return `<harmony><root><root-step>C</root-step></root><kind>major</kind><frame ` + attrs + `><frame-strings>6</frame-strings><frame-frets>4</frame-frets><frame-note><string>1</string><fret>0</fret></frame-note></frame></harmony>`
	}
	numeral := func(attrs string) string {
		return `<harmony><numeral><numeral-root ` + attrs + `>1</numeral-root><numeral-key><numeral-fifths>0</numeral-fifths><numeral-mode>major</numeral-mode></numeral-key></numeral><kind>major</kind></harmony>`
	}
	breath := func(value string) string {
		return layoutMatrixNote(`<notations><articulations><breath-mark>` + value + `</breath-mark></articulations></notations>`)
	}
	for _, tc := range []struct {
		check, axis, body, property string
		want                        []layoutMatrixValue
	}{
		{"image-valign-app-default", "positive", `<direction><direction-type><image source="i.png" type="image/png" valign="top"/></direction-type></direction>`, "image-valign", []layoutMatrixValue{lmk("top")}},
		{"image-valign-app-default", "negative", `<direction><direction-type><image source="i.png" type="image/png" valign="baseline"/></direction-type></direction>`, "image-valign", []layoutMatrixValue{lmi()}},
		{"image-valign-app-default", "boundary", `<direction><direction-type><image source="i.png" type="image/png" valign="middle"/></direction-type></direction>`, "image-valign", []layoutMatrixValue{lmk("middle")}},
		{"image-valign-app-default", "missing_fact", `<direction><direction-type><image source="i.png" type="image/png"/></direction-type></direction>`, "image-valign", []layoutMatrixValue{lmu()}},
		{"image-valign-app-default", "interaction", layoutMatrixWords(`valign="baseline"`) + `<direction><direction-type><image source="i.png" type="image/png"/></direction-type></direction>`, "image-valign", []layoutMatrixValue{lmu()}},
		{"unplayed-string-default", "positive", frame(`unplayed="x"`), "unplayed-string", []layoutMatrixValue{lmk("x")}},
		{"unplayed-string-default", "boundary", frame(`unplayed=""`), "unplayed-string", []layoutMatrixValue{lmk("")}},
		{"unplayed-string-default", "missing_fact", frame(""), "unplayed-string", []layoutMatrixValue{lmu()}},
		{"unplayed-string-default", "interaction", frame(`unplayed="x"`) + frame(""), "unplayed-string", []layoutMatrixValue{lmk("x"), lmu()}},
		{"numeral-root-display", "positive", numeral(`text="I"`), "numeral-root-text", []layoutMatrixValue{lmk("I")}},
		{"numeral-root-display", "boundary", numeral(`text=""`), "numeral-root-text", []layoutMatrixValue{lmk("")}},
		{"numeral-root-display", "missing_fact", numeral(""), "numeral-root-text", []layoutMatrixValue{lmu()}},
		{"numeral-root-display", "interaction", numeral(`text="one"`) + numeral(""), "numeral-root-text", []layoutMatrixValue{lmk("one"), lmu()}},
		{"empty-breath-mark", "positive", breath("comma"), "breath-mark-symbol", []layoutMatrixValue{lmk("comma")}},
		{"empty-breath-mark", "negative", breath("star"), "breath-mark-symbol", []layoutMatrixValue{lmi()}},
		{"empty-breath-mark", "boundary", breath("tick"), "breath-mark-symbol", []layoutMatrixValue{lmk("tick")}},
		{"empty-breath-mark", "missing_fact", breath(""), "breath-mark-symbol", []layoutMatrixValue{lmu()}},
		{"empty-breath-mark", "interaction", breath("comma") + breath(""), "breath-mark-symbol", []layoutMatrixValue{lmk("comma"), lmu()}},
		{"kind-text-symbol-default", "positive", layoutMatrixHarmony(`<kind use-symbols="yes" text="maj">major</kind>`), "kind-text-placement", []layoutMatrixValue{lmk("after-symbol")}},
		{"kind-text-symbol-default", "negative", layoutMatrixHarmony(`<kind use-symbols="invalid">major</kind>`), "kind-text-placement", []layoutMatrixValue{lmi()}},
		{"kind-text-symbol-default", "boundary", layoutMatrixHarmony(`<kind use-symbols="no" text="">major</kind>`), "kind-text-placement", []layoutMatrixValue{lma()}},
		{"kind-text-symbol-default", "missing_fact", layoutMatrixHarmony(`<kind>major</kind>`), "kind-text-placement", []layoutMatrixValue{lmu()}},
		{"kind-text-symbol-default", "interaction", layoutMatrixHarmony(`<kind use-symbols="yes">major</kind>`) + layoutMatrixHarmony(`<kind>minor</kind>`), "kind-text-placement", []layoutMatrixValue{lmk("after-symbol"), lmu()}},
	} {
		t.Run(tc.check+"/"+tc.axis, func(t *testing.T) {
			layoutReports(t, tc.body, "", func(t *testing.T, r validation.Report) {
				layoutMatrixValues(t, r, tc.property, tc.want...)
				layoutMatrixNoDefect(t, r)
			})
		})
	}
}

func TestLayoutMatrixIndependentEditorialAndKindFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		check      string
		properties []string
		body       func(string) string
		attrs      []string
	}{
		{"editorial-display-default", []string{"editorial-parentheses", "editorial-bracket", "editorial-size"}, func(a string) string {
			return `<direction><direction-type><words>Text</words></direction-type><level ` + a + `>editorial</level></direction>`
		}, []string{`parentheses="yes" bracket="no" size="cue"`, `parentheses="invalid" bracket="no" size="cue"`, `parentheses="no" bracket="yes" size="full"`, "", `parentheses="yes"`}},
		{"kind-format-defaults", []string{"kind-stack-degrees", "kind-parentheses-degrees", "kind-bracket-degrees"}, func(a string) string { return layoutMatrixHarmony(`<kind ` + a + `>major</kind>`) }, []string{`stack-degrees="yes" parentheses-degrees="no" bracket-degrees="yes"`, `stack-degrees="invalid" parentheses-degrees="no" bracket-degrees="yes"`, `stack-degrees="no" parentheses-degrees="yes" bracket-degrees="no"`, "", `stack-degrees="yes"`}},
	} {
		for i, axis := range []string{"positive", "negative", "boundary", "missing_fact", "interaction"} {
			t.Run(tc.check+"/"+axis, func(t *testing.T) {
				layoutReports(t, tc.body(tc.attrs[i]), "", func(t *testing.T, r validation.Report) {
					want := [][]layoutMatrixValue{{lmk("yes"), lmk("no"), lmk("yes")}, {lmi(), lmk("no"), lmk("yes")}, {lmk("no"), lmk("yes"), lmk("no")}, {lmu(), lmu(), lmu()}, {lmk("yes"), lmu(), lmu()}}[i]
					if tc.check == "editorial-display-default" && i < 3 {
						size := "cue"
						if i == 2 {
							size = "full"
						}
						want[2] = lmk(size)
					}
					for j, p := range tc.properties {
						layoutMatrixValues(t, r, p, want[j])
					}
				})
			})
		}
	}
}

// Numeric source spellings that cannot survive decoding get distinct current-
// model invalid values. This is current-model evidence, not source equivalence.
func TestLayoutMatrixNonFiniteCurrentGeometry(t *testing.T) {
	t.Parallel()
	for _, tw := range []bool{false, true} {
		for _, target := range []string{"source", "model"} {
			t.Run(fmt.Sprintf("%s/timewise=%v", target, tw), func(t *testing.T) {
				source := contextFixtureXML(layoutMatrixWords(`default-x="0" relative-y="0" rotation="0"`), tw)
				var r validation.Report
				var err error
				if target == "source" {
					source = strings.Replace(source, `default-x="0"`, `default-x="NaN"`, 1)
					r, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
				} else {
					doc, e := Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					var words *FormattedTextID
					switch d := doc.(type) {
					case *ScorePartwise:
						words = d.Part[0].Measure[0].Content[0].Direction.DirectionType[0].Content[0].Words
					case *ScoreTimewise:
						words = d.Measure[0].Part[0].Content[0].Direction.DirectionType[0].Content[0].Words
					}
					*words.DefaultX = Tenths(math.NaN())
					r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				layoutMatrixValues(t, r, "default-x", lmi())
				layoutMatrixValues(t, r, "relative-y", lmk("0"))
				layoutMatrixValues(t, r, "rotation", lmk("0"))
			})
		}
	}
}

func TestLayoutMatrixCoordinatesAndRelations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		check, axis, body, header, property, basis string
		want                                       []layoutMatrixValue
	}{
		{"global-tenths", "positive", layoutMatrixWords(`default-x="10"`), "", "default-x", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lmk("10")}},
		{"global-tenths", "negative", layoutMatrixWords(`default-x="-1.5"`), "", "default-x", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lmk("-1.5")}},
		{"global-tenths", "boundary", layoutMatrixWords(`default-x="0"`), "", "default-x", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lmk("0")}},
		{"global-tenths", "missing_fact", layoutMatrixWords(""), "", "default-x", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lma()}},
		{"global-tenths", "interaction", `<attributes><staff-details><staff-size>50</staff-size></staff-details></attributes>` + layoutMatrixWords(`default-x="10"`), "", "default-x", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lmk("10")}},
		{"position-origins", "positive", layoutMatrixWords(`default-y="10"`), "", "default-y", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lmk("10")}},
		{"position-origins", "negative", layoutMatrixWords(`relative-y="10"`), "", "default-y", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lma()}},
		{"position-origins", "boundary", `<print/>`, `<credit><credit-words default-y="0">Credit</credit-words></credit>`, "default-y", "physical-page-lower-left", []layoutMatrixValue{lmk("0")}},
		{"position-origins", "missing_fact", layoutMatrixWords(""), "", "default-y", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lma()}},
		{"position-origins", "interaction", `<direction><direction-type><words default-y="12">one</words><words>two</words></direction-type></direction>`, "", "default-y", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lmk("12"), lma()}},
		{"position-resolution", "positive", layoutMatrixWords(`default-x="12" relative-x="2"`), "", "relative-x", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("2")}},
		{"position-resolution", "negative", layoutMatrixWords(`relative-x="2"`), "", "default-x", "current-measure-start-x-staff-top-y", []layoutMatrixValue{lma()}},
		{"position-resolution", "boundary", layoutMatrixWords(`default-x="0" relative-x="0"`), "", "relative-x", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("0")}},
		{"position-resolution", "missing_fact", layoutMatrixWords(""), "", "relative-x", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lma()}},
		{"position-resolution", "interaction", `<direction placement="above" directive="yes"><direction-type><words default-x="12" relative-x="2">text</words></direction-type><offset>3</offset></direction>`, "", "relative-x", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("2")}},
		{"relative-position-basis", "positive", `<direction placement="above"><direction-type><words relative-y="3">text</words></direction-type></direction>`, "", "relative-y", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("3")}},
		{"relative-position-basis", "negative", `<direction placement="below"><direction-type><words relative-y="-3">text</words></direction-type></direction>`, "", "relative-y", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("-3")}},
		{"relative-position-basis", "boundary", layoutMatrixWords(`relative-y="0"`), "", "relative-y", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("0")}},
		{"relative-position-basis", "missing_fact", layoutMatrixWords(`relative-x="3"`), "", "relative-x", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("3")}},
		{"relative-position-basis", "interaction", `<direction directive="yes"><direction-type><words relative-x="3">first</words><words relative-x="-2">second</words></direction-type><offset>4</offset></direction>`, "", "relative-x", "default-position-plus-placement-offset-directive", []layoutMatrixValue{lmk("3"), lmk("-2")}},
	} {
		t.Run(tc.check+"/"+tc.axis, func(t *testing.T) {
			layoutReports(t, tc.body, tc.header, func(t *testing.T, r validation.Report) {
				layoutMatrixValues(t, r, tc.property, tc.want...)
				for _, v := range layoutMatrixRows(r, tc.property) {
					if v.Layout.Unit != validation.ContextUnitTenths || v.Layout.Basis != tc.basis {
						t.Errorf("unit/origin %+v", v.Layout)
					}
					if v.Layout.Conversion.State == validation.EvidenceKnown {
						t.Errorf("coordinate invented unprovided absolute geometry: %+v", v.Layout)
					}
				}
				layoutMatrixNoDefect(t, r)
			})
		})
	}
}

func TestLayoutMatrixBezierControlPoints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, attrs   string
		first, second bool
	}{
		{"positive", `bezier-x="2" bezier-offset="3" bezier-x2="4" bezier-offset2="5"`, false, false},
		{"negative", `bezier-offset="3" bezier-offset2="5"`, true, true},
		{"boundary", `bezier-x="0" bezier-offset="0" bezier-offset2="0"`, false, true},
		{"missing_fact", `bezier-y="2" bezier-offset="3" bezier-y2="4" bezier-offset2="5"`, true, true},
		{"interaction", `bezier-x2="-2" bezier-offset="3" bezier-offset2="5"`, true, false},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutReports(t, layoutMatrixNote(`<notations><slur type="start" `+tc.attrs+`/></notations>`), "", func(t *testing.T, r validation.Report) {
				for _, v := range []struct {
					property string
					active   bool
				}{{"bezier-offset", tc.first}, {"bezier-offset2", tc.second}} {
					rows := layoutFacts(r, v.property)
					if len(rows) != 1 || rows[0].Layout.Applicability.State != validation.EvidenceKnown || rows[0].Layout.Applicability.Value != v.active || rows[0].Layout.Unit != validation.ContextUnitDivisions {
						t.Errorf("%s: %+v", v.property, rows)
					}
				}
				for _, property := range []string{"bezier-x", "bezier-x2", "bezier-y", "bezier-y2"} {
					for _, v := range layoutFacts(r, property) {
						if v.Layout.Unit != validation.ContextUnitTenths {
							t.Errorf("%s unit %+v", property, v.Layout)
						}
					}
				}
			})
		})
	}
}

func TestLayoutMatrixDirectiveAndPageApplicability(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, attrs string
		state       validation.EvidenceState
		active      bool
	}{
		{"positive", "", validation.EvidenceKnown, true}, {"negative", `justify="right"`, validation.EvidenceKnown, false}, {"boundary", `default-x="0"`, validation.EvidenceKnown, false}, {"missing_fact", `halign="invalid"`, validation.EvidenceInvalid, false}, {"interaction", `halign="left" relative-x="3"`, validation.EvidenceKnown, false},
	} {
		t.Run("directive-position-precedence/"+tc.axis, func(t *testing.T) {
			body := `<direction directive="yes"><direction-type><words ` + tc.attrs + `>text</words></direction-type><offset>4</offset></direction>`
			layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
				rows := layoutFacts(r, "directive")
				if len(rows) != 1 {
					t.Fatal(rows)
				}
				v := rows[0]
				if v.Layout.Applicability.State != tc.state || v.Layout.Applicability.Value != tc.active || v.Value.Value.Text != "yes" || v.Layout.Basis != "time-signature-or-first-notational-element" {
					t.Fatalf("directive %+v %+v", v, v.Layout)
				}
				if tc.state == validation.EvidenceInvalid && v.Layout.Applicability.Cause == nil {
					t.Fatal("invalid controlling attribute lost cause")
				}
			})
		})
	}
	for _, tc := range []struct {
		axis, attrs, children string
		state                 validation.EvidenceState
		active, spacing       bool
	}{
		{"positive", `new-page="yes" page-number="chapter-A" blank-page="2"`, "", validation.EvidenceKnown, true, true},
		{"negative", `new-page="no" page-number="chapter-A" blank-page="2"`, "", validation.EvidenceKnown, false, true},
		{"boundary", `new-page="yes" page-number="0" blank-page="0"`, "", validation.EvidenceKnown, true, true},
		{"missing_fact", `new-page="invalid" page-number="A"`, "", validation.EvidenceInvalid, false, true},
		{"interaction", `new-page="no" staff-spacing="20"`, `<staff-layout><staff-distance>12</staff-distance></staff-layout>`, validation.EvidenceKnown, false, false},
	} {
		t.Run("print-page-conditions/"+tc.axis, func(t *testing.T) {
			layoutReports(t, `<print `+tc.attrs+`>`+tc.children+`</print>`, "", func(t *testing.T, r validation.Report) {
				for _, property := range []string{"page-number", "blank-page"} {
					rows := layoutFacts(r, property)
					if len(rows) != 1 || rows[0].Layout.Applicability.State != tc.state || rows[0].Layout.Applicability.Value != tc.active {
						t.Errorf("%s %+v", property, rows)
					}
				}
				rows := layoutFacts(r, "staff-spacing")
				if len(rows) != 1 || rows[0].Layout.Applicability.Value != tc.spacing {
					t.Errorf("deprecated staff spacing %+v", rows)
				}
			})
		})
	}
}

func TestLayoutMatrixDynamicsPlacement(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, body string
		want       []layoutMatrixValue
	}{
		{"positive", `<direction placement="above"><direction-type><dynamics placement="below"><p/></dynamics></direction-type></direction>`, []layoutMatrixValue{lmk("above")}},
		{"negative", `<direction placement="invalid"><direction-type><dynamics placement="below"><p/></dynamics></direction-type></direction>`, []layoutMatrixValue{lmi()}},
		{"boundary", `<direction><direction-type><dynamics placement="below"><p/></dynamics></direction-type></direction>`, []layoutMatrixValue{lmu()}},
		{"missing_fact", `<direction><direction-type><dynamics><p/></dynamics></direction-type></direction>`, []layoutMatrixValue{lmu()}},
		{"interaction", `<direction placement="above"><direction-type><dynamics placement="below"><p/></dynamics></direction-type></direction>` + layoutMatrixNote(`<notations><dynamics placement="below"><f/></dynamics></notations>`), []layoutMatrixValue{lmk("above"), lmk("below")}},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutReports(t, tc.body, "", func(t *testing.T, r validation.Report) { layoutMatrixValues(t, r, "placement", tc.want...) })
		})
	}
}

func TestLayoutMatrixTextAlternativePrecedence(t *testing.T) {
	t.Parallel()
	for _, owner := range []string{"elision", "tap"} {
		for _, tc := range []struct {
			axis, text, alternative string
			want                    layoutMatrixValue
			active                  bool
		}{
			{"positive", "literal", "valid", lmk("literal"), false}, {"negative", "literal", "invalid", lmk("literal"), false}, {"boundary", "", "valid", lmk(""), true}, {"missing_fact", "", "", lmu(), true}, {"interaction", " ", "valid", lmk(" "), false},
		} {
			t.Run(owner+"-precedence/"+tc.axis, func(t *testing.T) {
				attr := ""
				name, value := "smufl", "lyricsElisionNarrow"
				if owner == "tap" {
					name, value = "hand", "left"
				}
				if tc.alternative == "invalid" {
					value = "invalid"
				}
				if tc.alternative != "" {
					attr = ` ` + name + `="` + value + `"`
				}
				element := `<` + owner + attr + `>` + tc.text + `</` + owner + `>`
				child := `<notations><technical>` + element + `</technical></notations>`
				if owner == "elision" {
					child = `<lyric><text>one</text>` + element + `<text>two</text></lyric>`
				}
				layoutReports(t, layoutMatrixNote(child), "", func(t *testing.T, r validation.Report) {
					want := tc.want
					if tc.axis == "boundary" {
						want = lmk(value)
					}
					layoutMatrixValues(t, r, "display", want)
					rows := layoutFacts(r, name)
					if len(rows) != 1 || rows[0].Layout.Applicability.State != validation.EvidenceKnown || rows[0].Layout.Applicability.Value != tc.active {
						t.Errorf("%s alternative %+v", name, rows)
					}
				})
			})
		}
	}
}

func TestLayoutMatrixLyricSelectorComponents(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, defaults, lyric, property string
		want                            layoutMatrixValue
	}{
		{"font/positive", `<lyric-font name="v" number="01" font-family="A"/>`, `<lyric name="v" number="01"><text>x</text></lyric>`, "font-family", lmk("A")},
		{"font/negative", `<lyric-font name="v" number="1" font-family="A"/>`, `<lyric name="v" number="01"><text>x</text></lyric>`, "font-family", lmu()},
		{"font/boundary", `<lyric-font font-family="A"/><lyric-font name="v" font-style="italic"/>`, `<lyric name="v"><text>x</text></lyric>`, "font-family", lmk("A")},
		{"font/missing_fact", `<lyric-font number="" font-family="A"/>`, `<lyric number="1"><text>x</text></lyric>`, "font-family", lmi()},
		{"font/interaction", `<lyric-font font-family="A"/><lyric-font name="v" font-family="B"/>`, `<lyric name="v"><text font-family="C">x</text></lyric>`, "font-family", lmk("C")},
		{"language/positive", `<lyric-language name="v" number="01" xml:lang="fr"/>`, `<lyric name="v" number="01"><text>x</text></lyric>`, "language", lmk("fr")},
		{"language/negative", `<lyric-language name="v" number="1" xml:lang="fr"/>`, `<lyric name="v" number="01"><text>x</text></lyric>`, "language", lmu()},
		{"language/boundary", `<lyric-language xml:lang="fr"/>`, `<lyric><text xml:lang="">x</text></lyric>`, "language", lmk("")},
		{"language/missing_fact", `<lyric-language xml:lang="fr"/><lyric-language name="v" xml:lang="de"/>`, `<lyric name="v"><text>x</text></lyric>`, "language", lmu()},
		{"language/interaction", `<lyric-language xml:lang="fr"/><lyric-language name="v" xml:lang="de"/>`, `<lyric name="v"><text xml:lang="it">x</text></lyric>`, "language", lmk("it")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutReports(t, layoutMatrixNote(tc.lyric), `<defaults>`+tc.defaults+`</defaults>`, func(t *testing.T, r validation.Report) {
				rows := layoutFacts(r, tc.property)
				if len(rows) == 0 {
					t.Fatal("missing lyric result")
				}
				got := rows[len(rows)-1]
				if got.Value.State != tc.want.state || (tc.want.state == validation.EvidenceKnown && got.Value.Value.Text != tc.want.text) {
					t.Fatalf("lyric: %+v want %+v", got.Value, tc.want)
				}
				if tc.want.state == validation.EvidenceInvalid && got.Value.Cause == nil {
					t.Fatal("lost selector cause")
				}
			})
		})
	}
}

func TestLayoutMatrixComponentInheritance(t *testing.T) {
	t.Parallel()
	for _, use := range []string{"harmony", "metronome"} {
		for _, tc := range []struct {
			axis, parent, child string
			want                layoutMatrixValue
		}{
			{"positive", `font-family="Parent"`, "", lmk("Parent")},
			{"negative", `font-style="bad"`, "", lmi()},
			{"boundary", `font-family="Parent"`, `font-family="Child"`, lmk("Child")},
			{"missing_fact", "", "", lmu()},
			{"interaction", `font-style="bad"`, `font-style="italic"`, lmk("italic")},
		} {
			t.Run(use+"/"+tc.axis, func(t *testing.T) {
				body := `<harmony ` + tc.parent + `><root><root-step>C</root-step></root><kind ` + tc.child + `>major</kind></harmony>`
				if use == "metronome" {
					body = `<direction><direction-type><metronome ` + tc.parent + `><beat-unit>quarter</beat-unit><per-minute ` + tc.child + `>60</per-minute></metronome></direction-type></direction>`
				}
				prop := "font-family"
				if tc.axis == "negative" || tc.axis == "interaction" {
					prop = "font-style"
				}
				layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
					rows := layoutMatrixRows(r, prop)
					if len(rows) < 2 {
						t.Fatal("missing parent/child fields")
					}
					got := rows[len(rows)-1]
					if got.Value.State != tc.want.state || (tc.want.state == validation.EvidenceKnown && got.Value.Value.Text != tc.want.text) {
						t.Fatalf("%+v want %+v", got.Value, tc.want)
					}
				})
			})
		}
	}
}

func TestLayoutMatrixSystemSelectors(t *testing.T) {
	t.Parallel()
	for _, owner := range []string{"direction", "measure-numbering"} {
		for _, tc := range []struct {
			axis, attr string
			want       layoutMatrixValue
		}{
			{"positive", `system="only-top"`, lmk("only-top")}, {"negative", `system="bogus"`, lmi()},
			{"boundary", `system="none"`, lmk("none")}, {"missing_fact", "", lma()}, {"interaction", `system="also-top"`, lmk("also-top")},
		} {
			t.Run(owner+"/"+tc.axis, func(t *testing.T) {
				body := `<direction ` + tc.attr + `><direction-type><words>x</words></direction-type></direction>`
				if owner == "measure-numbering" {
					body = `<print><measure-numbering ` + tc.attr + `>measure</measure-numbering></print>`
				}
				layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
					layoutMatrixValues(t, r, "system", tc.want)
					layoutMatrixNoDefect(t, r)
					if tc.axis == "boundary" {
						for _, v := range layoutMatrixRows(r, "system") {
							if v.Layout.Scope.Node == 0 {
								t.Error("containing part scope missing")
							}
						}
					}
				})
			})
		}
	}
}

func TestLayoutMatrixCreditPageAndComponents(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, page, first, second string
		pageWant, formatWant      layoutMatrixValue
	}{
		{"positive", "", `font-family="A"`, "", lmk("1"), lmk("A")},
		{"negative", `page="0"`, `font-style="bad"`, "", lmi(), lmi()},
		{"boundary", `page="2"`, `font-family="A" default-x="0"`, `font-family="B"`, lmk("2"), lmk("B")},
		{"missing_fact", "", "", "", lmk("1"), lmu()},
		{"interaction", `page="3"`, `font-family="A" default-x="10"`, `font-family="B" default-x="20"`, lmk("3"), lmk("B")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutReports(t, `<print new-page="yes" page-number="99"/>`, `<credit `+tc.page+`><credit-words `+tc.first+`>a</credit-words><credit-words `+tc.second+`>b</credit-words></credit>`, func(t *testing.T, r validation.Report) {
				layoutMatrixValues(t, r, "credit-page", tc.pageWant)
				prop := "font-family"
				if tc.axis == "negative" {
					prop = "font-style"
				}
				rows := layoutMatrixRows(r, prop)
				if len(rows) != 2 {
					t.Fatalf("credit rows %d", len(rows))
				}
				got := rows[1].Value
				if got.State != tc.formatWant.state || (got.State == validation.EvidenceKnown && got.Value.Text != tc.formatWant.text) {
					t.Errorf("credit component %+v", got)
				}
				pos := layoutMatrixRows(r, "default-x")
				var credits []validation.ContextResult
				for _, v := range pos {
					if v.Layout.Basis == "physical-page-lower-left" {
						credits = append(credits, v)
					}
				}
				if len(credits) != 2 {
					t.Fatal("credit coordinate basis missing")
				}
				if tc.axis == "boundary" && credits[1].Value.State != validation.EvidenceAbsent {
					t.Error("position inherited")
				}
			})
		})
	}
}

func TestLayoutMatrixScalingAndInsets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, mm, tenths, left, sys string
		ratio, inset                layoutMatrixValue
	}{
		{"positive", "7", "40", "10", "-5", lmk("7/40"), lmk("5")},
		{"negative", "7", "NaN", "NaN", "-5", lmi(), lmi()},
		{"boundary", "0", "40", "10", "-10", lmk("0"), lmk("0")},
		{"missing_fact", "7", "0", "", "", lmu(), lmu()},
		{"interaction", "7", "20", "10", "-15", lmk("7/20"), lmk("-5")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			margins := ""
			if tc.left != "" {
				margins = `<page-margins><left-margin>` + tc.left + `</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins>`
			}
			system := ""
			if tc.sys != "" {
				system = `<system-layout><system-margins><left-margin>` + tc.sys + `</left-margin><right-margin>0</right-margin></system-margins></system-layout>`
			}
			header := `<defaults><scaling><millimeters>` + tc.mm + `</millimeters><tenths>` + tc.tenths + `</tenths></scaling><page-layout>` + margins + `</page-layout>` + system + `</defaults>`
			layoutReports(t, `<print/>`, header, func(t *testing.T, r validation.Report) {
				for _, pair := range []struct {
					property string
					want     layoutMatrixValue
				}{{"millimeters-per-tenth", tc.ratio}, {"effective-left-margin", tc.inset}} {
					rows := layoutFacts(r, pair.property)
					if len(rows) != 1 {
						t.Fatalf("%s: %d", pair.property, len(rows))
					}
					got := rows[0].Layout.Conversion
					if got.State != pair.want.state || (got.State == validation.EvidenceKnown && got.Value.String() != pair.want.text) {
						t.Errorf("%s %+v want %+v", pair.property, got, pair.want)
					}
				}
				layoutMatrixNoDefect(t, r)
			})
		})
	}
}

func layoutMatrixMeasures(t *testing.T, header string, bodies []string, check func(*testing.T, validation.Report)) {
	t.Helper()
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			t.Run(fmt.Sprintf("timewise%t/model%t", tw, model), func(t *testing.T) {
				h := header + `<part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list>`
				source := `<score-partwise>` + h + `<part id="P1">`
				if tw {
					source = `<score-timewise>` + h
				}
				for i, body := range bodies {
					source += fmt.Sprintf(`<measure number="%d">`, i+1)
					if tw {
						source += `<part id="P1">`
					}
					source += body
					if tw {
						source += `</part>`
					}
					source += `</measure>`
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
				check(t, r)
			})
		}
	}
}

func TestLayoutMatrixScopedOverrides(t *testing.T) {
	t.Parallel()
	supports := `<identification><encoding><supports element="print" attribute="new-page" value="yes" type="yes"/><supports element="print" attribute="new-system" value="yes" type="yes"/></encoding></identification>`
	defaults := `<defaults><page-layout><page-height>100</page-height><page-width>80</page-width></page-layout><system-layout><system-distance>20</system-distance><top-system-distance>10</top-system-distance></system-layout></defaults>`
	first := `<print><page-layout><page-height>200</page-height><page-width>80</page-width></page-layout><system-layout><system-distance>30</system-distance></system-layout><measure-layout><measure-distance>7</measure-distance></measure-layout></print>`
	for _, tc := range []struct {
		axis, header, second string
		page, system         layoutMatrixValue
	}{
		{"positive", supports, `<print new-page="yes"/>`, lmk("100"), lmk("20")},
		{"negative", supports, `<print new-system="yes"/>`, lmk("200"), lmk("20")},
		{"boundary", supports, `<print new-page="no" new-system="no"/>`, lmk("200"), lmk("30")},
		{"missing_fact", "", `<print/>`, lmu(), lmu()},
		{"interaction", supports, `<print new-page="yes"><page-layout><page-height>300</page-height><page-width>80</page-width></page-layout><system-layout><system-distance>40</system-distance></system-layout></print>`, lmk("300"), lmk("40")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutMatrixMeasures(t, tc.header+defaults, []string{first, tc.second}, func(t *testing.T, r validation.Report) {
				layoutMatrixValues(t, r, "page-height", lmk("200"), tc.page)
				layoutMatrixValues(t, r, "top-system-distance", lmk("10"), lmk("10"))
				layoutMatrixValues(t, r, "system-distance", lmk("30"), tc.system)
				layoutMatrixValues(t, r, "measure-distance", lmk("7"), lma())
				for _, v := range layoutFacts(r, "measure-distance") {
					if v.Layout.Applicability.State != validation.EvidenceUnknown {
						t.Error("internal coda whitespace invented")
					}
				}
			})
		})
	}
}

func TestLayoutMatrixDividers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, distance, attrs string
		want                  layoutMatrixValue
	}{
		{"positive", `<system-distance>30</system-distance>`, "", lmk("15")},
		{"negative", `<system-distance>NaN</system-distance>`, "", lmi()},
		{"boundary", `<system-distance>0</system-distance>`, "", lmk("0")},
		{"missing_fact", "", "", lmu()},
		{"interaction", `<system-distance>30</system-distance>`, `default-y="8"`, lmk("8")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutMatrixMeasures(t, `<identification><encoding><supports element="print" attribute="new-page" value="yes" type="yes"/></encoding></identification><defaults><system-layout>`+tc.distance+`<system-dividers><left-divider `+tc.attrs+`/><right-divider/></system-dividers></system-layout></defaults>`, []string{`<print/>`, `<print new-system="yes"/>`}, func(t *testing.T, r validation.Report) {
				rows := layoutFacts(r, "left-divider-y")
				if len(rows) != 2 {
					t.Fatalf("divider rows %d", len(rows))
				}
				for i, v := range rows {
					got := v.Layout.Conversion
					if got.State != tc.want.state || (got.State == validation.EvidenceKnown && got.Value.String() != tc.want.text) {
						t.Errorf("divider %+v", got)
					}
					if v.Layout.Applicability.State != validation.EvidenceKnown || v.Layout.Applicability.Value != (i == 1) {
						t.Errorf("divider first-system applicability %+v", v.Layout.Applicability)
					}
				}
			})
		})
	}
}

func TestLayoutMatrixArrowheadAndAbsentOwners(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, content string
		count         int
	}{
		{"positive", `<arrow-direction>up</arrow-direction><arrowhead/>`, 1},
		{"negative", `<arrow-direction>up</arrow-direction>`, 0},
		{"boundary", `<arrow-direction>left</arrow-direction><arrow-style>filled</arrow-style><arrowhead/>`, 1},
		{"interaction", `<arrow-direction>down</arrow-direction><arrowhead/>`, 1},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutReports(t, layoutMatrixNote(`<notations><technical><arrow>`+tc.content+`</arrow></technical></notations>`), "", func(t *testing.T, r validation.Report) {
				rows := layoutFacts(r, "geometry")
				if len(rows) != tc.count {
					t.Fatalf("head-only rows %d want %d", len(rows), tc.count)
				}
				for _, v := range rows {
					if v.Value.State != validation.EvidenceKnown || v.Value.Value.Text != "head-only" {
						t.Errorf("geometry %+v", v.Value)
					}
				}
				layoutMatrixNoDefect(t, r)
			})
		})
	}
	t.Run("missing_fact", func(t *testing.T) {
		layoutReports(t, layoutMatrixNote(""), "", func(t *testing.T, r validation.Report) {
			if len(layoutFacts(r, "geometry")) != 0 {
				t.Fatal("invented arrow from absent notation")
			}
		})
	})
	for _, property := range []string{"unplayed-string", "numeral-root-text"} {
		t.Run(property+"/negative", func(t *testing.T) {
			layoutReports(t, layoutMatrixNote(""), "", func(t *testing.T, r validation.Report) {
				if len(layoutFacts(r, property)) != 0 {
					t.Fatal("default applied outside its owner")
				}
			})
		})
	}
}

func TestLayoutMatrixPageMarginSelection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, typ string
		print     bool
		want      layoutMatrixValue
	}{
		{"positive", "", false, lmk("11")}, {"negative", `type="even"`, false, lmu()},
		{"boundary", `type="odd"`, false, lmk("11")}, {"missing_fact", `type="invalid"`, false, lmi()},
		{"interaction", `type="even"`, true, lmk("11")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layout := `<page-layout><page-margins ` + tc.typ + `><left-margin>11</left-margin><right-margin>12</right-margin><top-margin>13</top-margin><bottom-margin>14</bottom-margin></page-margins></page-layout>`
			header := `<defaults>` + layout + `</defaults>`
			body := `<print/>`
			if tc.print {
				header = ""
				body = `<print>` + layout + `</print>`
			}
			layoutReports(t, body, header, func(t *testing.T, r validation.Report) { layoutMatrixValues(t, r, "page-left-margin", tc.want) })
		})
	}
}

func TestLayoutMatrixLyricAlignmentDefaults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, attrs       string
		placement, halign layoutMatrixValue
	}{
		{"positive", "", lmk("below"), lmk("center")},
		{"negative", `placement="above" justify="left"`, lmk("above"), lmk("left")},
		{"boundary", `justify="right"`, lmk("below"), lmk("right")},
		{"missing_fact", `justify="invalid"`, lmk("below"), lmi()},
		{"interaction", `placement="invalid" justify="left"`, lmi(), lmk("left")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutReports(t, layoutMatrixNote(`<lyric `+tc.attrs+`><text>x</text></lyric>`), "", func(t *testing.T, r validation.Report) {
				layoutMatrixValues(t, r, "lyric-placement", tc.placement)
				rows := layoutMatrixRows(r, "halign")
				if len(rows) < 1 {
					t.Fatal("missing lyric alignment")
				}
				got := rows[0].Value
				if got.State != tc.halign.state || (got.State == validation.EvidenceKnown && got.Value.Text != tc.halign.text) {
					t.Errorf("lyric halign %+v", got)
				}
				vals := layoutMatrixRows(r, "valign")
				if len(vals) < 1 || vals[0].Value.Value.Text != "baseline" {
					t.Error("lyric baseline lost")
				}
			})
		})
	}
}

func TestLayoutMatrixStaffOverrideLifetime(t *testing.T) {
	t.Parallel()
	header := `<identification><encoding><supports element="print" attribute="new-page" value="yes" type="yes"/><supports element="print" attribute="new-system" value="yes" type="yes"/></encoding></identification><defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`
	for _, tc := range []struct {
		axis, second string
		want         layoutMatrixValue
	}{
		{"positive", `<print new-system="yes"/>`, lmk("12")},
		{"negative", `<print new-system="no"/>`, lmk("22")},
		{"boundary", `<print new-page="yes"/>`, lmk("12")},
		{"missing_fact", `<print new-system="invalid"/>`, lmi()},
		{"interaction", `<print new-system="yes"><staff-layout><staff-distance>32</staff-distance></staff-layout></print>`, lmk("32")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutMatrixMeasures(t, header, []string{`<print><staff-layout><staff-distance>22</staff-distance></staff-layout></print>`, tc.second}, func(t *testing.T, r validation.Report) {
				layoutMatrixValues(t, r, "staff-distance", lmk("22"), tc.want)
			})
		})
	}
}

func TestLayoutMatrixDefaultsStayComponentLocal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, defaults, attrs string
		want                  layoutMatrixValue
	}{
		{"positive", `<word-font font-family="Default"/>`, "", lmk("Default")},
		{"negative", `<music-font font-family="Music"/>`, "", lmu()},
		{"boundary", `<word-font font-style="italic"/>`, "", lmu()},
		{"missing_fact", "", "", lmu()},
		{"interaction", `<word-font font-family="Default"/>`, `font-family="Local"`, lmk("Local")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			layoutReports(t, layoutMatrixWords(tc.attrs), `<defaults>`+tc.defaults+`</defaults>`, func(t *testing.T, r validation.Report) {
				rows := layoutMatrixRows(r, "font-family")
				if len(rows) == 0 {
					t.Fatal("missing word font")
				}
				got := rows[len(rows)-1].Value
				if got.State != tc.want.state || (got.State == validation.EvidenceKnown && got.Value.Text != tc.want.text) {
					t.Errorf("font %+v want %+v", got, tc.want)
				}
			})
		})
	}
}

func TestLayoutMatrixEmptyLanguageClears(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, first, second string
		want                layoutMatrixValue
	}{
		{"positive", `xml:lang="fr"`, `xml:lang=""`, lmk("")},
		{"negative", `xml:lang="fr"`, "", lmk("fr")},
		{"boundary", `xml:lang=""`, "", lmk("")},
		{"missing_fact", `xml:lang="*"`, "", lmi()},
		{"interaction", `xml:lang="*"`, `xml:lang=""`, lmk("")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			body := `<direction><direction-type><words ` + tc.first + `>a</words><words ` + tc.second + `>b</words></direction-type></direction>`
			layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
				rows := layoutMatrixRows(r, "language")
				if len(rows) < 2 {
					t.Fatal("missing word language")
				}
				got := rows[len(rows)-1].Value
				if got.State != tc.want.state || (got.State == validation.EvidenceKnown && got.Value.Text != tc.want.text) {
					t.Errorf("language %+v want %+v", got, tc.want)
				}
			})
		})
	}
}

func TestLayoutMatrixInheritedTextDefaults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		axis, first, second string
		cross               bool
		dir, spacing        layoutMatrixValue
	}{
		{"positive", "", "", false, lmk("ltr"), lmk("normal")},
		{"negative", `dir="invalid" letter-spacing="bad"`, "", false, lmi(), lmi()},
		{"boundary", `dir="rtl" letter-spacing="0"`, "", false, lmk("rtl"), lmk("0")},
		// Missing local fields in a fresh wrapper use their own documented defaults.
		{"missing_fact", `dir="rtl" letter-spacing="2"`, "", true, lmk("ltr"), lmk("normal")},
		{"interaction", `dir="invalid" letter-spacing="bad"`, `dir="ltr" letter-spacing="normal"`, false, lmk("ltr"), lmk("normal")},
	} {
		t.Run(tc.axis, func(t *testing.T) {
			separator := ""
			if tc.cross {
				separator = `</direction-type><direction-type>`
			}
			body := `<direction><direction-type><words ` + tc.first + `>a</words>` + separator + `<words ` + tc.second + `>b</words></direction-type></direction>`
			layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
				for _, pair := range []struct {
					property string
					want     layoutMatrixValue
				}{{"dir", tc.dir}, {"letter-spacing", tc.spacing}} {
					rows := layoutMatrixRows(r, pair.property)
					if len(rows) != 2 {
						t.Fatalf("%s rows %d", pair.property, len(rows))
					}
					got := rows[1].Value
					value := got.Value.Text
					if got.Value.Kind == validation.AtomicDecimal {
						value = got.Value.Decimal.String()
					}
					if got.State != pair.want.state || (got.State == validation.EvidenceKnown && value != pair.want.text) {
						t.Errorf("%s %+v want %+v", pair.property, got, pair.want)
					}
				}
			})
		})
	}
}

func TestLayoutMatrixNameTextDefaults(t *testing.T) {
	t.Parallel()
	for _, element := range []string{"display-text", "accidental-text"} {
		for _, tc := range []struct {
			axis, attrs         string
			language, enclosure layoutMatrixValue
		}{
			{"positive", "", lmk("it"), lmk("none")},
			{"negative", `xml:lang="fr" enclosure="circle"`, lmk("fr"), lmk("circle")},
			{"boundary", `xml:lang="" enclosure="none"`, lmk(""), lmk("none")},
			{"missing_fact", `xml:lang="*" enclosure="invalid"`, lmi(), lmi()},
			{"interaction", `xml:lang="de" enclosure="square" font-family="Explicit"`, lmk("de"), lmk("square")},
		} {
			t.Run(element+"/"+tc.axis, func(t *testing.T) {
				value := "Name"
				if element == "accidental-text" {
					value = "sharp"
				}
				body := `<print><part-name-display><` + element + ` ` + tc.attrs + `>` + value + `</` + element + `></part-name-display></print>`
				layoutReports(t, body, "", func(t *testing.T, r validation.Report) {
					layoutMatrixValues(t, r, "language", tc.language)
					layoutMatrixValues(t, r, "enclosure", tc.enclosure)
				})
			})
		}
	}
}
