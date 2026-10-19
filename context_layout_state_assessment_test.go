package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var cachedLayoutStateTestEngine = sync.OnceValues(buildLayoutStateTestEngine)

func layoutStateTestEngine(t *testing.T) *validation.Engine {
	t.Helper()
	engine, err := cachedLayoutStateTestEngine()
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func buildLayoutStateTestEngine() (*validation.Engine, error) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	schema := d.schemas[validation.AssemblyMusicXML]
	b := newLocalCatalogBuilder(schema)
	var owners []validation.ReferenceOwner
	for i, part := range b.namedElementUses("part") {
		attribute, e := b.typeAttribute(part.DeclaredType, "id")
		if e != nil {
			return nil, e
		}
		owners = append(owners, validation.ReferenceOwner{Declaration: part.Declaration, Attribute: attribute, Rule: validation.RuleKey{ID: validation.RuleID("MX40-LOCAL-part-id-target/use-" + strconv.Itoa(i+1)), Revision: 1}})
	}
	timeline := validation.RuleKey{ID: "test-layout-timeline", Revision: 1}
	timelinePlan, err := validationNewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: owners})
	if err != nil {
		return nil, err
	}
	layout := validation.RuleKey{ID: "test-layout-state", Revision: 1}
	layoutPlan, err := validationNewLayoutContextPlan(validation.ContextPlanSpec{Schema: schema, TimelineProvider: timeline})
	if err != nil {
		return nil, err
	}
	assembly := validation.AssemblyMusicXML
	for _, v := range []struct {
		key  validation.RuleKey
		plan validation.Plan
	}{{timeline, timelinePlan}, {layout, layoutPlan}} {
		rule := validation.RuleDefinition{Key: v.key, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(v.key.ID), Text: validation.RuleText{Title: "Layout state", Explanation: "Source-backed layout state fixture", Message: "Incomplete layout state"}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-clef-conditions"}, URL: localSchemaSource(2930), Locator: "Clef context"}}}
		if v.key == layout {
			rule.Requires = []validation.Dependency{{Kind: validation.DependencyRule, Rule: timeline, Relation: validation.RelationDocument}}
		}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: v.plan})
		for i := range d.profiles {
			if d.profiles[i].ID == ProfileSourceLocal || d.profiles[i].ID == ProfileModelLocal {
				d.profiles[i].Required = append(d.profiles[i].Required, v.key)
				d.profiles[i].Support = append(d.profiles[i].Support, validation.RuleSupport{Rule: v.key, State: validation.ImplementationAvailable})
			}
		}
	}
	catalog, err := newCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	engine, err := validation.NewEngine(catalog)
	if err != nil {
		return nil, err
	}
	return engine, nil
}
func layoutStateRows(report validation.Report, property string) []validation.ContextResult {
	var rows []validation.ContextResult
	for _, v := range report.Context {
		if string(v.Property) == "layout."+property {
			rows = append(rows, v)
		}
	}
	return rows
}
func layoutStateKnown(t *testing.T, report validation.Report, property, want string) {
	t.Helper()
	rows := layoutStateRows(report, property)
	if len(rows) != 1 {
		t.Fatalf("%s rows %d", property, len(rows))
	}
	value := rows[0].Value
	if value.State != validation.EvidenceKnown {
		t.Fatalf("%s %+v", property, value)
	}
	got := value.Value.Text
	if value.Value.Kind == validation.AtomicInteger {
		got = value.Value.Integer.String()
	}
	if got != want {
		t.Fatalf("%s got %q want %q", property, got, want)
	}
}
func TestLayoutStateSourceModelConditionalFacts(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	body := `<attributes><divisions>1</divisions><key print-object="no"><fifths>0</fifths></key><staves>2</staves><part-symbol top-staff="1" bottom-staff="1">bracket</part-symbol><clef number="1" additional="yes" size="cue" after-barline="yes"><sign>G</sign><line>2</line></clef><staff-details number="2" print-object="no" print-spacing="yes"><staff-type>alternate</staff-type><staff-lines>5</staff-lines><line-detail line="3" print-object="no"/><staff-size scaling="80">120</staff-size></staff-details></attributes><note><rest measure="yes"/><duration>1</duration><type>whole</type><stem relative-y="-100">up</stem><staff>1</staff></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, localContextFixture(body, tw), model)
			if !report.Satisfied() {
				t.Fatalf("fixture: %s", localFindings(report))
			}
			for property, want := range map[string]string{"clef-staff": "1", "clef-system-print": "no", "key-system-print": "no", "created-part-staff-scope": "", "hidden-line-coordinate": "included-in-clef-and-display-coordinates", "hidden-line-layout-count": "no", "staff-cutaway": "yes", "alternate-staff-playback": "no", "alternate-staff-source": "1", "staff-size-affects-margins": "no", "part-symbol": "bracket", "part-symbol-top": "1", "part-symbol-bottom": "1", "part-symbol-barline-scope-change": "yes", "measure-rest": "yes", "stemlet": "yes", "measure-number-text": "label"} {
				if want != "" {
					layoutStateKnown(t, report, property, want)
				}
			}
			for property, want := range map[string]string{"staff-space-scale": "120", "staff-notation-scale": "80"} {
				rows := layoutStateRows(report, property)
				if len(rows) != 1 || rows[0].Value.State != validation.EvidenceKnown || rows[0].Value.Value.Decimal.String() != want {
					t.Fatalf("%s %+v", property, rows)
				}
			}
			for property, want := range map[string]bool{"clef-line": false, "clef-size": true, "clef-after-barline": true} {
				rows := layoutStateRows(report, property)
				if len(rows) != 1 || rows[0].Layout.Applicability.State != validation.EvidenceKnown || rows[0].Layout.Applicability.Value != want {
					t.Fatalf("%s %+v", property, rows)
				}
			}
			rows := layoutStateRows(report, "stem-relative-y")
			if len(rows) != 1 || rows[0].Layout.Applicability.State != validation.EvidenceUnknown {
				t.Fatalf("stem default length unknown: %+v", rows)
			}
		}
	}
}
func TestLayoutStatePercussionAndDots(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	body := `<attributes><divisions>1</divisions><clef><sign>percussion</sign></clef></attributes><note><unpitched><display-step>G</display-step><display-octave>4</display-octave></unpitched><duration>1</duration><type>quarter</type><dot placement="above"/></note><note><unpitched><display-step>A</display-step><display-octave>4</display-octave></unpitched><duration>1</duration><type>quarter</type><dot placement="below"/></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, localContextFixture(body, tw), model)
			if !report.Satisfied() {
				t.Fatal(localFindings(report))
			}
			positions := layoutStateRows(report, "display-staff-position")
			dots := layoutStateRows(report, "dot-placement-active")
			if len(positions) != 2 || len(dots) != 2 {
				t.Fatalf("rows %+v %+v", positions, dots)
			}
			for i, want := range []string{"2", "3"} {
				if positions[i].Value.State != validation.EvidenceKnown || positions[i].Value.Value.Integer.String() != want {
					t.Fatalf("position %+v", positions[i])
				}
			}
			for i, want := range []string{"yes", "no"} {
				if dots[i].Value.State != validation.EvidenceKnown || dots[i].Value.Value.Text != want {
					t.Fatalf("dot %+v", dots[i])
				}
			}
		}
	}
}

func TestLayoutStateOmittedDefaultsAndCategories(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	body := `<attributes><divisions>1</divisions><staves>2</staves><clef><sign>G</sign><line>2</line></clef><staff-details number="1"><staff-type>cue</staff-type><staff-lines>1</staff-lines></staff-details><for-part><part-transpose><diatonic>1</diatonic><chromatic>2</chromatic></part-transpose></for-part></attributes><note><grace/><pitch><step>C</step><octave>4</octave></pitch><type>eighth</type><staff>1</staff></note><note><cue/><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type><staff>1</staff><notations><technical><string>1</string></technical></notations></note><note><grace/><cue/><unpitched/><type>eighth</type><staff>1</staff></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			source := localContextFixture(body, tw)
			source = strings.Replace(source, `<part-list>`, `<defaults><concert-score/></defaults><part-list>`, 1)
			report := localContextAssess(t, engine, source, model)
			if !report.Satisfied() {
				t.Fatal(localFindings(report))
			}
			for property, want := range map[string]string{"part-symbol": "brace", "part-symbol-top": "1", "part-symbol-bottom": "2", "part-symbol-barline-scope-change": "no", "created-part-staff-scope": "all-staves-of-created-part", "staff-size-range": "less-than-100-implementation-dependent", "staff-cutaway": "no", "unpitched-default-position": "middle-line-of-staff", "display-staff-position": "0", "string-number-basis": "one-is-highest-pitched-full-length-string"} {
				layoutStateKnown(t, report, property, want)
			}
			rows := layoutStateRows(report, "note-size-category")
			if len(rows) != 3 {
				t.Fatalf("categories %d", len(rows))
			}
			for i, want := range []string{"grace", "cue", "grace-cue"} {
				if rows[i].Value.State != validation.EvidenceKnown || rows[i].Value.Value.Text != want {
					t.Fatalf("category %+v", rows[i])
				}
			}
		}
	}
}
func TestLayoutStateImplicitAndNumberingPolicy(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, policy := range []string{"none", "measure", "system"} {
				for _, implicit := range []string{"no", "yes"} {
					body := `<print><measure-numbering>` + policy + `</measure-numbering></print><note><rest/><duration>1</duration></note>`
					source := localContextFixture(body, tw)
					source = strings.Replace(source, `number="label"`, `number="label" text="display A" implicit="`+implicit+`"`, 1)
					report := localContextAssess(t, engine, source, model)
					if !report.Satisfied() {
						t.Fatal(localFindings(report))
					}
					want := "yes"
					if implicit == "yes" || policy == "none" {
						want = "no"
					}
					layoutStateKnown(t, report, "measure-number-display", want)
					layoutStateKnown(t, report, "measure-number-text", "display A")
					layoutStateKnown(t, report, "measure-number-staff", "1")
					rows := layoutStateRows(report, "measure-number-text")
					if rows[0].Layout.Applicability.State != validation.EvidenceKnown || rows[0].Layout.Applicability.Value != (implicit == "no") {
						t.Fatalf("implicit applicability %+v", rows[0])
					}
				}
			}
		}
	}
}

func layoutStateManyMeasures(bodies []string, timewise bool) string {
	head := `<part-list><score-part id="P1"><part-name>Part</part-name></score-part></part-list>`
	if timewise {
		out := `<score-timewise version="4.0">` + head
		for _, body := range bodies {
			out += `<measure number="same label"><part id="P1">` + body + `</part></measure>`
		}
		return out + `</score-timewise>`
	}
	out := `<score-partwise version="4.0">` + head + `<part id="P1">`
	for _, body := range bodies {
		out += `<measure number="same label">` + body + `</measure>`
	}
	return out + `</part></score-partwise>`
}
func TestLayoutStateInheritedSymbolNumberingAndMidmeasureClef(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	first := `<attributes><divisions>1</divisions><staves>2</staves><part-symbol>bracket</part-symbol><clef number="1"><sign>G</sign><line>2</line></clef></attributes><print><measure-numbering>measure</measure-numbering></print><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><staff>1</staff></note>`
	second := `<attributes><key><fifths>1</fifths></key></attributes><note><cue/><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><staff>1</staff></note><attributes><clef number="1" after-barline="yes"><sign>F</sign><line>4</line></clef></attributes><note><rest><display-step>F</display-step><display-octave>3</display-octave></rest><duration>1</duration><staff>1</staff></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, layoutStateManyMeasures([]string{first, second}, tw), model)
			if !report.Satisfied() {
				t.Fatal(localFindings(report))
			}
			symbols := layoutStateRows(report, "part-symbol")
			if len(symbols) != 3 {
				t.Fatalf("symbols %d", len(symbols))
			}
			for i, row := range symbols {
				if row.Value.State != validation.EvidenceKnown || row.Value.Value.Text != "bracket" {
					t.Fatalf("symbol%d %+v", i, row)
				}
				if i > 0 && row.Layout.Provenance != validation.ContextInherited {
					t.Fatalf("inheritance %+v", row)
				}
			}
			numbering := layoutStateRows(report, "measure-number-display")
			if len(numbering) != 2 {
				t.Fatalf("numbering %d", len(numbering))
			}
			for _, row := range numbering {
				if row.Value.State != validation.EvidenceKnown || row.Value.Value.Text != "yes" {
					t.Fatalf("numbering %+v", row)
				}
			}
			clefs := layoutStateRows(report, "clef-after-barline")
			if len(clefs) != 2 || clefs[1].Layout.Applicability.State != validation.EvidenceKnown || clefs[1].Layout.Applicability.Value {
				t.Fatalf("midmeasure clef %+v", clefs)
			}
			layoutStateKnown(t, report, "display-staff-position", "6")
			categories := layoutStateRows(report, "note-size-category")
			if categories[1].Value.State != validation.EvidenceKnown || categories[1].Value.Value.Text != "cue" {
				t.Fatalf("typeless cue %+v", categories[1])
			}
		}
	}
}
func TestLayoutStateInvalidComponentsStayCausal(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	body := `<attributes><divisions>1</divisions><clef additional="not-valid"><sign>G</sign><line>2</line></clef><staff-details number="bad" print-object="not-valid"><staff-type>alternate</staff-type></staff-details></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type size="not-valid">quarter</type></note>`
	// Invalid integer attributes cannot be represented by generated model fields;
	// the source adapter alone retains their original lexical error. The invalid
	// enum/yes-no components are independently exercised in both target models.
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			source := localContextFixture(body, tw)
			if model {
				source = strings.Replace(source, `number="bad"`, `number="2"`, 1)
			}
			report := localContextAssess(t, engine, source, model)
			for _, property := range []string{"clef-line", "staff-cutaway", "note-size-category"} {
				rows := layoutStateRows(report, property)
				if len(rows) != 1 {
					t.Fatalf("%s rows%d", property, len(rows))
				}
				state, cause := rows[0].Value.State, rows[0].Value.Cause
				if property == "clef-line" {
					state, cause = rows[0].Layout.Applicability.State, rows[0].Layout.Applicability.Cause
				}
				if state != validation.EvidenceInvalid || cause == nil {
					t.Fatalf("%s %+v", property, rows[0])
				}
			}
			if !model {
				rows := layoutStateRows(report, "alternate-staff-source")
				if len(rows) != 1 || rows[0].Value.State != validation.EvidenceInvalid || rows[0].Value.Cause == nil {
					t.Fatalf("invalid staff %+v", rows)
				}
			}
		}
	}
}

func TestLayoutStateFutureTimingGapCannotPoisonPriorNumbering(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	first := `<attributes><divisions>1</divisions><staves>2</staves><part-symbol>bracket</part-symbol></attributes><attributes><key><fifths>0</fifths></key></attributes><print><measure-numbering>measure</measure-numbering></print>`
	second := `<backup><duration>0</duration></backup><print><measure-numbering>none</measure-numbering></print>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, layoutStateManyMeasures([]string{first, second}, tw), model)
			rows := layoutStateRows(report, "measure-number-display")
			if len(rows) != 2 {
				t.Fatalf("rows%d", len(rows))
			}
			for i, want := range []string{"yes", "no"} {
				if rows[i].Value.State != validation.EvidenceKnown || rows[i].Value.Value.Text != want {
					t.Fatalf("prefix%d %+v", i, rows[i])
				}
			}
			symbols := layoutStateRows(report, "part-symbol")
			if len(symbols) != 2 || symbols[1].Value.State != validation.EvidenceKnown || symbols[1].Value.Value.Text != "bracket" {
				t.Fatalf("prior symbol %+v", symbols)
			}
		}
	}
}

func TestLayoutStateUnconfinedLaterSymbolCannotProveInitialDefault(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	first := `<attributes><divisions>1</divisions><staves>2</staves></attributes><note><rest/><duration>1</duration></note>`
	for _, excursion := range []bool{false, true} {
		second := ""
		if excursion {
			second = `<backup><duration>2</duration></backup>`
		}
		second += `<attributes><part-symbol>bracket</part-symbol></attributes>`
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				report := localContextAssess(t, engine, layoutStateManyMeasures([]string{first, second}, tw), model)
				rows := layoutStateRows(report, "part-symbol")
				if len(rows) != 2 {
					t.Fatalf("rows%d", len(rows))
				}
				if excursion {
					if rows[0].Value.State == validation.EvidenceKnown {
						t.Fatalf("unconfined publisher ignored: %+v", rows[0])
					}
				} else if rows[0].Value.State != validation.EvidenceKnown || rows[0].Value.Value.Text != "brace" {
					t.Fatalf("contained publisher polluted default: %+v", rows[0])
				}
				if rows[1].Value.State != validation.EvidenceKnown || rows[1].Value.Value.Text != "bracket" {
					t.Fatalf("explicit local symbol lost %+v", rows[1])
				}
			}
		}
	}
}

func TestLayoutStatePerRecordBoundaryMatrix(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	yes, no := true, false
	cases := []struct {
		name, body, property, value string
		state                       validation.EvidenceState
		active                      *bool
		concert                     bool
	}{
		{"ordinary-clef-line", `<attributes><clef size="cue"><sign>G</sign><line>2</line></clef></attributes>`, "clef-line", "2", validation.EvidenceKnown, &yes, false},
		{"ordinary-clef-size-inactive", `<attributes><clef size="cue"><sign>G</sign><line>2</line></clef></attributes>`, "clef-size", "cue", validation.EvidenceKnown, &no, false},
		{"key-default-system-print", `<attributes><key><fifths>0</fifths></key></attributes>`, "key-system-print", "yes", validation.EvidenceKnown, nil, false},
		{"created-staff-two", `<attributes><for-part number="2"><part-transpose><chromatic>2</chromatic></part-transpose></for-part></attributes>`, "created-part-staff-scope", "2", validation.EvidenceKnown, nil, true},
		{"visible-line-count", `<attributes><staff-details><staff-lines>1</staff-lines><line-detail line="1"/></staff-details></attributes>`, "hidden-line-layout-count", "yes", validation.EvidenceKnown, nil, false},
		{"visible-line-hidden-rule-inactive", `<attributes><staff-details><staff-lines>1</staff-lines><line-detail line="1"/></staff-details></attributes>`, "hidden-line-coordinate", "included-in-clef-and-display-coordinates", validation.EvidenceKnown, &no, false},
		{"hidden-staff-no-space", `<attributes><staff-details print-object="no" print-spacing="no"/></attributes>`, "staff-cutaway", "no", validation.EvidenceKnown, nil, false},
		{"notation-scale-no-numeric-default", `<attributes><staff-details><staff-size>60</staff-size></staff-details></attributes>`, "staff-notation-scale", "", validation.EvidenceUnknown, nil, false},
		{"alternate-top-has-no-prior", `<attributes><staff-details number="1"><staff-type>alternate</staff-type></staff-details></attributes>`, "alternate-staff-source", "", validation.EvidenceUnknown, nil, false},
		{"regular-default-playback", `<attributes><staff-details/></attributes>`, "alternate-staff-playback", "yes", validation.EvidenceKnown, nil, false},
		{"dot-missing-clef", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><dot/></note>`, "dot-placement-active", "", validation.EvidenceUnknown, nil, false},
		{"not-measure-rest", `<note><rest measure="no"/><duration>1</duration></note>`, "measure-rest", "no", validation.EvidenceKnown, nil, false},
		{"stem-none-position-inactive", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><stem relative-y="-1">none</stem></note>`, "stem-relative-y", "-1", validation.EvidenceKnown, &no, false},
		{"stem-zero-offset-active", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><stem relative-y="0">up</stem></note>`, "stem-relative-y", "0", validation.EvidenceKnown, &yes, false},
		{"pitched-stem-not-stemlet", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><stem>down</stem></note>`, "stemlet", "no", validation.EvidenceKnown, nil, false},
		{"explicit-unpitched-no-middle-default", `<note><unpitched><display-step>C</display-step><display-octave>4</display-octave></unpitched><duration>1</duration></note>`, "unpitched-default-position", "", validation.EvidenceAbsent, &no, false},
		{"scordatura-string-reference", `<direction><direction-type><scordatura><accord string="1"><tuning-step>E</tuning-step><tuning-octave>4</tuning-octave></accord></scordatura></direction-type></direction>`, "string-number-basis", "one-is-highest-pitched-full-length-string", validation.EvidenceKnown, nil, false},
		{"large-note-category", `<note><rest/><duration>1</duration><type size="large">quarter</type></note>`, "note-size-category", "large", validation.EvidenceKnown, nil, false},
		{"full-grace-not-scaled", `<note><grace/><rest/><type size="full">quarter</type></note>`, "note-size-category", "", validation.EvidenceAbsent, nil, false},
		{"explicit-cue-overrides-grace-cue", `<note><grace/><cue/><rest/><type size="cue">quarter</type></note>`, "note-size-category", "grace", validation.EvidenceKnown, nil, false},
		{"numbering-none-disables-range", `<print><measure-numbering multiple-rest-range="yes">none</measure-numbering></print>`, "measure-number-multiple-rest-range", "yes", validation.EvidenceKnown, &no, false},
		{"numbering-none-disables-always", `<print><measure-numbering multiple-rest-always="yes">none</measure-numbering></print>`, "measure-number-multiple-rest-always", "yes", validation.EvidenceKnown, &no, false},
		{"numbering-staff-override", `<print><measure-numbering staff="2">measure</measure-numbering></print>`, "measure-number-staff", "2", validation.EvidenceKnown, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, tw := range []bool{false, true} {
				for _, model := range []bool{false, true} {
					source := localContextFixture(c.body, tw)
					if c.concert {
						source = strings.Replace(source, `<part-list>`, `<defaults><concert-score/></defaults><part-list>`, 1)
					}
					report := localContextAssess(t, engine, source, model)
					if !report.Satisfied() {
						t.Fatal(localFindings(report))
					}
					rows := layoutStateRows(report, c.property)
					if len(rows) != 1 {
						t.Fatalf("%s rows%d", c.property, len(rows))
					}
					row := rows[0]
					if row.Value.State != c.state {
						t.Fatalf("%s %+v", c.property, row)
					}
					if c.state == validation.EvidenceKnown {
						value := row.Value.Value.Text
						switch row.Value.Value.Kind {
						case validation.AtomicInteger:
							value = row.Value.Value.Integer.String()
						case validation.AtomicDecimal:
							value = row.Value.Value.Decimal.String()
						}
						if value != c.value {
							t.Fatalf("%s got%s want%s", c.property, value, c.value)
						}
					}
					if c.active != nil && (row.Layout.Applicability.State != validation.EvidenceKnown || row.Layout.Applicability.Value != *c.active) {
						t.Fatalf("%s %+v", c.property, row.Layout.Applicability)
					}
				}
			}
		})
	}
}
func TestLayoutStateUnplacedDivisionsExcursionPreservesGap(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	first := `<attributes><divisions>1</divisions><staves>2</staves></attributes><note><rest/><duration>1</duration></note>`
	second := `<note><rest/><duration>2</duration></note><attributes><divisions>2</divisions></attributes><backup><duration>8</duration></backup><attributes><part-symbol>bracket</part-symbol></attributes>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, layoutStateManyMeasures([]string{first, second}, tw), model)
			rows := layoutStateRows(report, "part-symbol")
			if len(rows) != 3 {
				t.Fatalf("rows%d", len(rows))
			}
			if rows[0].Value.State == validation.EvidenceKnown {
				t.Fatalf("unplaced future publisher lost placement: %+v", rows[0])
			}
			if rows[2].Value.State != validation.EvidenceKnown || rows[2].Value.Value.Text != "bracket" {
				t.Fatalf("explicit raw lost %+v", rows[2])
			}
		}
	}
}

func TestLayoutStatePercussionRecommendationAxes(t *testing.T) {
	t.Parallel()
	engine := layoutStateTestEngine(t)
	for _, c := range []struct {
		name, sign, note string
		state            validation.EvidenceState
		active           bool
	}{
		{"pitched-percussion", "percussion", `<pitch><step>C</step><octave>4</octave></pitch>`, validation.EvidenceKnown, true},
		{"unpitched-percussion", "percussion", `<unpitched/>`, validation.EvidenceKnown, true},
		{"rest-percussion", "percussion", `<rest/>`, validation.EvidenceKnown, false},
		{"pitched-G", "G", `<pitch><step>C</step><octave>4</octave></pitch>`, validation.EvidenceKnown, false},
		{"missing-clef", "", `<unpitched/>`, validation.EvidenceUnknown, false},
		{"invalid-clef", "bogus", `<unpitched/>`, validation.EvidenceInvalid, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, tw := range []bool{false, true} {
				for _, model := range []bool{false, true} {
					body := ""
					if c.sign != "" {
						body = `<attributes><clef><sign>` + c.sign + `</sign></clef></attributes>`
					}
					body += `<note>` + c.note + `<duration>1</duration></note>`
					report := localContextAssess(t, engine, localContextFixture(body, tw), model)
					rows := layoutStateRows(report, "advisory-percussion-unpitched")
					if len(rows) != 1 {
						t.Fatalf("rows%d", len(rows))
					}
					row := rows[0]
					if row.Value.State != validation.EvidenceKnown || row.Value.Value.Text != "use-unpitched-for-notes-in-percussion-clef" {
						t.Fatalf("guidance %+v", row)
					}
					app := row.Layout.Applicability
					if app.State != c.state || (c.state == validation.EvidenceKnown && app.Value != c.active) {
						t.Fatalf("app %+v", app)
					}
					if c.state == validation.EvidenceInvalid && app.Cause == nil {
						t.Fatal("missing cause")
					}
					if c.state != validation.EvidenceInvalid && !report.Satisfied() {
						t.Fatal(localFindings(report))
					}
				}
			}
		})
	}
}
