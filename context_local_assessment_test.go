package musicxml

import (
	"context"
	"encoding/xml"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"sync"
	"testing"
)

// This independent harness selects the real local context provider over the
// unchanged source/model prerequisites. It never selects unfinished siblings.
var cachedLocalContextTestEngine = sync.OnceValues(buildLocalContextTestEngine)

func localContextTestEngine(t *testing.T) *validation.Engine {
	t.Helper()
	engine, err := cachedLocalContextTestEngine()
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func buildLocalContextTestEngine() (*validation.Engine, error) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	schema := d.schemas[validation.AssemblyMusicXML]
	plan, err := validationNewLocalContextPlan(schema)
	if err != nil {
		return nil, err
	}
	key := validation.RuleKey{ID: "test-local-context", Revision: 1}
	assembly := validation.AssemblyMusicXML
	rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Local context", Explanation: "Independent local context fixture", Message: "Local context incomplete"}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-lyric-defaults"}, URL: localSchemaSource(5055), Locator: "Lyric printing"}}}
	d.rules = append(d.rules, rule)
	d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
	for i := range d.profiles {
		if d.profiles[i].ID == ProfileSourceLocal || d.profiles[i].ID == ProfileModelLocal {
			d.profiles[i].Required = append(d.profiles[i].Required, key)
			d.profiles[i].Support = append(d.profiles[i].Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
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
func localContextAssess(t *testing.T, engine *validation.Engine, source string, model bool) validation.Report {
	t.Helper()
	ctx := context.Background()
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	profile, documentID := ProfileSourceLocal, sourceDocumentID
	if model {
		profile, documentID = ProfileModelLocal, modelDocumentID
	}
	session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: documentID, Profile: profile, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	if model {
		doc, e := Decode(strings.NewReader(source))
		if e != nil {
			t.Fatal(e)
		}
		_, walkErr := walkModel(ctx, doc, session, account, documentID, modelWalkOptions{})
		report, e := session.Finish(ctx, validation.Completion{InputComplete: walkErr == nil, Syntax: validation.SyntaxNotAssessed, Cause: walkErr})
		if e != nil {
			t.Fatal(e)
		}
		return report
	}
	lexical, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: ctx, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	observer, err := newSourceFactObserver(ctx, session, account)
	if err != nil {
		t.Fatal(err)
	}
	run := sourceAssessmentRun{ctx: ctx, account: account, session: session, source: &observingXMLSource{ctx: ctx, lexical: lexical, decoder: xml.NewTokenDecoder(lexical), observer: observer}}
	e := run.drain()
	report, err := run.finish(e)
	if err != nil {
		t.Fatal(err)
	}
	return report
}
func localContextFixture(body string, timewise bool) string {
	head := `<part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`
	if timewise {
		return `<score-timewise version="4.0">` + head + `<measure number="label"><part id="P1">` + body + `</part></measure></score-timewise>`
	}
	return `<score-partwise version="4.0">` + head + `<part id="P1"><measure number="label">` + body + `</measure></part></score-partwise>`
}
func TestLocalContextPrintingAcrossTargetsAndHierarchies(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<note print-object="no"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type><notehead>diamond</notehead><lyric print-object="yes"><text>la</text></lyric></note>`
	for _, timewise := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, localContextFixture(body, timewise), model)
			if !report.Satisfied() {
				t.Fatalf("invalid fixture: %s", localFindings(report))
			}
			counts := map[validation.ContextProperty]int{}
			for _, f := range report.Context {
				counts[f.Property]++
				if f.Property == validation.ContextPrintDot || f.Property == validation.ContextPrintLyric || f.Property == validation.ContextPrintLeger {
					if f.Value.State != validation.EvidenceKnown || f.Value.Value.Text != "no" {
						t.Fatalf("conditional print default %s: %+v", f.Property, f.Value)
					}
				}
				if f.Property == validation.ContextPrintSpacing {
					if f.Layout == nil || f.Layout.Applicability.State != validation.EvidenceKnown || f.Layout.Applicability.Value {
						t.Fatalf("visible lyric cannot permit spacing-only: %+v", f)
					}
				}
				if f.Property == validation.ContextNoteheadFilled {
					if f.Value.State != validation.EvidenceKnown || f.Value.Value.Text != "yes" {
						t.Fatalf("quarter fill: %+v", f)
					}
				}
			}
			for _, p := range []validation.ContextProperty{validation.ContextPrintLeger, validation.ContextPrintLyric, validation.ContextPrintDot, validation.ContextNoteheadFilled, validation.ContextNoteTypeSize} {
				if counts[p] != 1 {
					t.Fatalf("%s count=%d", p, counts[p])
				}
			}
		}
	}
}
func TestLocalContextUnknownDoesNotBecomeDefault(t *testing.T) {
	engine := localContextTestEngine(t)
	source := localContextFixture(`<note print-object="maybe"><rest/><duration>1</duration></note>`, false)
	for _, model := range []bool{false, true} {
		report := localContextAssess(t, engine, source, model)
		found := false
		for _, f := range report.Context {
			if f.Property == validation.ContextPrintLyric {
				found = true
				if f.Value.State != validation.EvidenceInvalid || f.Value.Cause == nil {
					t.Fatalf("invalid prerequisite defaulted: %+v", f)
				}
			}
		}
		if !found {
			t.Fatal("missing dependent evidence")
		}
	}

}

func localAllConstantsBody() string {
	return `<attributes><staff-details/><measure-style><multiple-rest>1</multiple-rest></measure-style></attributes><harmony><root><root-step>C</root-step></root><kind>major</kind><bass><bass-step>G</bass-step><bass-alter>1</bass-alter></bass><degree><degree-value>9</degree-value><degree-alter>0</degree-alter><degree-type>add</degree-type></degree></harmony><harmony><numeral><numeral-root>2</numeral-root><numeral-alter>1</numeral-alter></numeral><kind>major</kind><frame><frame-strings>6</frame-strings><frame-frets>4</frame-frets><frame-note><string>1</string><fret>0</fret></frame-note></frame></harmony><direction><direction-type><metronome><beat-unit>quarter</beat-unit><per-minute>120</per-minute></metronome></direction-type></direction><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type><accidental>natural</accidental><notehead>diamond</notehead><notations><fermata/><articulations><strong-accent/><caesura/></articulations><technical><heel/><toe/><hole><hole-closed>yes</hole-closed></hole></technical><ornaments><turn/></ornaments></notations><lyric><text>la</text></lyric></note><barline><repeat direction="backward"/></barline>`
}
func TestLocalContextEveryConstantDefaultAndOverrideBothTargets(t *testing.T) {
	engine := localContextTestEngine(t)
	defaults := map[string]string{"score-version": "1.0", "winged": "none", "show-frets": "numbers", "non-controlling": "no", "horizontal-turn-slash": "no", "multiple-rest-use-symbols": "no", "bass-alter-location": "right", "numeral-alter-location": "left", "degree-plus-minus": "no", "first-fret": "1", "metronome-parentheses": "no", "accidental-editorial": "no", "accidental-cautionary": "no", "heel-substitution": "no", "toe-substitution": "no", "hole-shape": "circle", "notehead-parentheses": "no", "strong-accent-type": "up", "empty-caesura": "normal", "fermata-shape": "normal", "fermata-orientation": "upright"}
	explicit := map[string]string{"score-version": "4.0", "winged": "straight", "show-frets": "letters", "non-controlling": "yes", "horizontal-turn-slash": "yes", "multiple-rest-use-symbols": "yes", "bass-alter-location": "left", "numeral-alter-location": "right", "degree-plus-minus": "yes", "first-fret": "3", "metronome-parentheses": "yes", "accidental-editorial": "yes", "accidental-cautionary": "yes", "heel-substitution": "yes", "toe-substitution": "yes", "hole-shape": "square", "notehead-parentheses": "yes", "strong-accent-type": "down", "empty-caesura": "short", "fermata-shape": "square", "fermata-orientation": "inverted"}
	for _, timewise := range []bool{false, true} {
		for _, override := range []bool{false, true} {
			source := localContextFixture(localAllConstantsBody(), timewise)
			want := defaults
			if !override {
				source = strings.Replace(source, ` version="4.0"`, "", 1)
			} else {
				want = explicit
				source = strings.NewReplacer(`<staff-details/>`, `<staff-details show-frets="letters"/>`, `<measure number=`, `<measure non-controlling="yes" number=`, `<multiple-rest>`, `<multiple-rest use-symbols="yes">`, `<bass-alter>`, `<bass-alter location="left">`, `<numeral-alter>`, `<numeral-alter location="right">`, `<degree-alter>`, `<degree-alter plus-minus="yes">`, `<frame-note>`, `<first-fret>3</first-fret><frame-note>`, `<metronome>`, `<metronome parentheses="yes">`, `<accidental>`, `<accidental editorial="yes" cautionary="yes">`, `<notehead>`, `<notehead parentheses="yes">`, `<fermata/>`, `<fermata type="inverted">square</fermata>`, `<strong-accent/>`, `<strong-accent type="down"/>`, `<caesura/>`, `<caesura>short</caesura>`, `<heel/>`, `<heel substitution="yes"/>`, `<toe/>`, `<toe substitution="yes"/>`, `</hole>`, `<hole-shape>square</hole-shape></hole>`, `<turn/>`, `<turn slash="yes"/>`, `<text>`, `<text dir="rtl" letter-spacing="0">`, `direction="backward"`, `direction="backward" winged="straight"`).Replace(source)
			}
			for _, model := range []bool{false, true} {
				report := localContextAssess(t, engine, source, model)
				if !report.Satisfied() {
					t.Fatalf("timewise=%v model=%v override=%v unintended defect: %s", timewise, model, override, localFindings(report))
				}
				seen := map[string]bool{}
				for _, f := range report.Context {
					p := strings.TrimPrefix(string(f.Property), "local.")
					if expected, ok := want[p]; ok {
						actual := f.Value.Value.Text
						if f.Value.Value.Kind == validation.AtomicInteger {
							actual = f.Value.Value.Integer.String()
						}
						if f.Value.Value.Kind == validation.AtomicDecimal {
							actual = f.Value.Value.Decimal.String()
						}
						if f.Value.State != validation.EvidenceKnown || actual != expected {
							t.Errorf("timewise=%v model=%v override=%v %s=%+v want %s", timewise, model, override, p, f.Value, expected)
						}
						seen[p] = true
					}
				}
				for p := range want {
					if !seen[p] {
						t.Errorf("missing %s timewise=%v model=%v override=%v", p, timewise, model, override)
					}
				}
			}
		}
	}
}

func TestLocalContextSemanticRelationsAndOpenDomainsBothTargets(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<attributes><key><fifths>0</fifths><mode>minor</mode></key><clef><sign>G</sign><line>999</line></clef></attributes><harmony><root><root-step>C</root-step></root><kind>none</kind><degree><degree-value symbol="major" text="extension">7</degree-value><degree-alter>0</degree-alter><degree-type>subtract</degree-type></degree></harmony><direction><direction-type><words font-family="First, Second" color="#00FF0000">word</words></direction-type></direction><note><pitch><step>C</step><alter>0.5</alter><octave>4</octave></pitch><duration>1</duration><notations><slur type="start" line-type="solid" dash-length="5" space-length="2"/></notations><lyric><text>well-known</text></lyric></note>`
	for _, model := range []bool{false, true} {
		source := localContextFixture(body, false)
		source = strings.Replace(source, `<part-list>`, `<identification><creator type="custom-role">Person</creator><encoding><encoder type="custom-encoder">Program</encoder><supports type="yes" element="print" value="yes"/></encoding></identification><defaults><appearance><line-width type="custom-line">0.5</line-width><distance type="custom-distance">4</distance></appearance></defaults><part-list>`, 1)
		source = strings.Replace(source, `<part-name>Piano</part-name>`, `<part-name>Piano</part-name><part-name-display/><score-instrument id="I1"><instrument-name>People</instrument-name><ensemble/></score-instrument>`, 1)
		report := localContextAssess(t, engine, source, model)
		if !report.Satisfied() {
			t.Fatalf("unintended local defect model=%v: %s", model, localFindings(report))
		}
		want := map[validation.ContextProperty]string{validation.ContextMinorMode: "natural minor", validation.ContextHarmonyActive: "no", validation.ContextDegreeReference: "harmony-kind-degree", validation.ContextDegreeTextComposition: "text-follows-symbol", validation.ContextColorRGB: "#FF0000", validation.ContextColorAlpha: "0", validation.ContextFontFamilyOrder: "comma-separated-preference-order", validation.ContextLineWidthType: "custom-line", validation.ContextDistanceType: "custom-distance", validation.ContextLyricLiteralText: "well-known", validation.ContextPartNameDisplay: "yes", validation.ContextStaffLineBasis: "one-is-bottom-line-unbounded-by-line-count"}
		seen := map[validation.ContextProperty]bool{}
		for _, f := range report.Context {
			if expected, ok := want[f.Property]; ok {
				actual := f.Value.Value.Text
				if f.Value.Value.Kind == validation.AtomicInteger {
					actual = f.Value.Value.Integer.String()
				}
				if f.Value.State != validation.EvidenceKnown || actual != expected {
					t.Errorf("%s got %+v want %s", f.Property, f.Value, expected)
				}
				seen[f.Property] = true
			}
			switch f.Property {
			case validation.ContextSupportsValue:
				if f.Layout.Applicability.State != validation.EvidenceKnown || f.Layout.Applicability.Value {
					t.Errorf("supports value active without attribute: %+v", f)
				}
			case validation.ContextDashLength, validation.ContextSpaceLength:
				if f.Layout.Applicability.State != validation.EvidenceKnown || f.Layout.Applicability.Value {
					t.Errorf("solid dash geometry active: %+v", f)
				}
			case validation.ContextEnsembleSize:
				if f.Value.State != validation.EvidenceUnknown {
					t.Errorf("empty ensemble assigned size: %+v", f)
				}
			}
		}
		for p := range want {
			if !seen[p] {
				t.Errorf("missing semantic fact %s model=%v", p, model)
			}
		}
	}
}

func TestLocalContextSparseFrameAndMinorModes(t *testing.T) {
	engine := localContextTestEngine(t)
	frame := `<harmony><root><root-step>C</root-step></root><kind>major</kind><frame><frame-strings>184467440737095516160000</frame-strings><frame-frets>4</frame-frets><frame-note><string>1</string><fret>0</fret></frame-note></frame></harmony>`
	for _, mode := range []string{"minor", "natural minor", "harmonic minor", "melodic minor"} {
		body := frame + `<harmony><numeral><numeral-root>6</numeral-root><numeral-key><numeral-fifths>0</numeral-fifths><numeral-mode>` + mode + `</numeral-mode></numeral-key></numeral><kind>minor</kind></harmony>`
		// A huge exact frame count is independently source-representable. The
		// generated model's machine integer limit is tested with a finite count.
		for _, model := range []bool{false, true} {
			source := localContextFixture(body, false)
			if model {
				source = strings.Replace(source, "184467440737095516160000", "1000000000", 1)
			}
			report := localContextAssess(t, engine, source, model)
			if !report.Satisfied() {
				t.Fatalf("mode %s model %t: %s", mode, model, localFindings(report))
			}
			count := 0
			seen := map[validation.ContextProperty]bool{}
			for _, f := range report.Context {
				switch f.Property {
				case validation.ContextFrameString:
					count++
					if f.Value.Value.Integer.String() != "1" {
						t.Fatalf("frame member: %+v", f)
					}
				case validation.ContextFrameMissingString:
					seen[f.Property] = true
					if f.Value.State != validation.EvidenceKnown || f.Value.Value.Text != "unlisted-strings-muted" {
						t.Fatalf("frame complement: %+v", f)
					}
				case validation.ContextOpenString:
					seen[f.Property] = true
					if f.Value.Value.Text != "yes" {
						t.Fatalf("fret zero: %+v", f)
					}
				case validation.ContextMinorRaisedSixth, validation.ContextMinorRaisedSeventh:
					seen[f.Property] = true
					want := "no"
					if mode == "melodic minor" || (mode == "harmonic minor" && f.Property == validation.ContextMinorRaisedSeventh) {
						want = "yes"
					}
					if f.Value.State != validation.EvidenceKnown || f.Value.Value.Text != want {
						t.Fatalf("mode %s %s: %+v want%s", mode, f.Property, f.Value, want)
					}
				}
			}
			if count != 1 || len(report.Context) > 200 {
				t.Fatalf("frame count was expanded: memberships=%d facts=%d", count, len(report.Context))
			}
			for _, p := range []validation.ContextProperty{validation.ContextFrameMissingString, validation.ContextOpenString, validation.ContextMinorRaisedSixth, validation.ContextMinorRaisedSeventh} {
				if !seen[p] {
					t.Fatalf("missing %s", p)
				}
			}
		}
	}
}

func TestLocalContextPermittedDeprecatedAndAdvisoryForms(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<attributes><divisions>0.5</divisions><clef><sign>none</sign></clef></attributes><harmony><function>I</function><kind>none</kind></harmony><direction><direction-type><harp-pedals><pedal-tuning><pedal-step>A</pedal-step><pedal-alter>0</pedal-alter></pedal-tuning><pedal-tuning><pedal-step>D</pedal-step><pedal-alter>0</pedal-alter></pedal-tuning></harp-pedals></direction-type></direction><direction><direction-type><principal-voice type="stop" symbol="Hauptstimme"/></direction-type></direction><note><pitch><step>C</step><octave>4</octave></pitch><duration>0.5</duration><lyric><text>well-known</text></lyric></note>`
	for _, model := range []bool{false, true} {
		source := localContextFixture(body, false)
		source = strings.Replace(source, "<part-list>", `<identification><creator>Person</creator><encoding><encoder>Program</encoder></encoding></identification><part-list>`, 1)
		report := localContextAssess(t, engine, source, model)
		if !report.Satisfied() {
			t.Fatalf("recommendation became hard error model=%t: %s", model, localFindings(report))
		}
		deprecated := 0
		for _, f := range report.Context {
			if f.Property == validation.ContextDeprecatedNotation && f.Value.State == validation.EvidenceKnown && f.Value.Value.Text == "yes" {
				deprecated++
			}
			if f.Property == validation.ContextMetadataRole && f.Value.State != validation.EvidenceAbsent {
				t.Fatalf("missing role invented: %+v", f)
			}
		}
		if deprecated != 2 {
			t.Fatalf("deprecation facts=%d", deprecated)
		}
	}
}

func TestLocalHorizontalTurnSlashDoesNotSelectGrace(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<note><grace slash="yes"/><pitch><step>C</step><octave>4</octave></pitch><type>eighth</type><notations><ornaments><turn/><delayed-turn/><inverted-turn/><delayed-inverted-turn/></ornaments></notations></note>`
	for _, model := range []bool{false, true} {
		report := localContextAssess(t, engine, localContextFixture(body, false), model)
		if !report.Satisfied() {
			t.Fatalf("bad fixture: %s", localFindings(report))
		}
		count := 0
		for _, f := range report.Context {
			if f.Property == validation.ContextLocalHorizontalTurnSlash {
				count++
				if f.Value.State != validation.EvidenceKnown || f.Value.Value.Text != "no" {
					t.Errorf("grace slash leaked into horizontal-turn property: %+v", f)
				}
			}
		}
		if count != 4 {
			t.Errorf("horizontal turn selected %d subjects, want4", count)
		}
	}
}

func TestLocalTechnicalSMuFLDefaultsAndExplicitNames(t *testing.T) {
	engine := localContextTestEngine(t)
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, explicit := range []bool{false, true} {
				attr := ""
				if explicit {
					attr = ` smufl="customUnregisteredName"`
				}
				body := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><stopped` + attr + `/><open` + attr + `/><half-muted` + attr + `/><up-bow/></technical></notations></note>`
				report := localContextAssess(t, engine, localContextFixture(body, tw), model)
				if !report.Satisfied() {
					t.Fatal(localFindings(report))
				}
				expected := map[string]string{"stopped-smufl": "brassMuteClosed", "open-smufl": "brassMuteOpen", "half-muted-smufl": "brassMuteHalfClosed"}
				for property, want := range expected {
					count := 0
					for _, f := range report.Context {
						if f.Property != validation.ContextProperty("local."+property) {
							continue
						}
						count++
						if explicit {
							want = "customUnregisteredName"
						}
						if f.Value.State != validation.EvidenceKnown || f.Value.Value.Text != want {
							t.Fatalf("%s %+v", property, f)
						}
						if !explicit && f.Layout.Raw.State != validation.EvidenceAbsent {
							t.Fatalf("default raw: %+v", f)
						}
					}
					if count != 1 {
						t.Fatalf("%s count %d", property, count)
					}
				}
			}
		}
	}
}

func TestLocalTechnicalSMuFLInvalidDoesNotDefault(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><stopped smufl="not valid"/><open smufl="not valid"/><half-muted smufl="not valid"/></technical></notations></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, localContextFixture(body, tw), model)
			for _, property := range []validation.ContextProperty{validation.ContextLocalStoppedSMuFL, validation.ContextLocalOpenSMuFL, validation.ContextLocalHalfMutedSMuFL} {
				count := 0
				for _, f := range report.Context {
					if f.Property == property {
						count++
						if f.Value.State != validation.EvidenceInvalid || f.Value.Cause == nil {
							t.Fatalf("%s invalid prerequisite replaced: %+v", property, f)
						}
					}
				}
				if count != 1 {
					t.Fatalf("%s missing", property)
				}
			}
		}
	}
}

func TestLocalContextRecordSpecificStructuralMatrix(t *testing.T) {
	engine := localContextTestEngine(t)
	type assertion struct {
		property validation.ContextProperty
		value    string
		state    validation.EvidenceState
		active   *bool
	}
	yes, no := true, false
	cases := []struct {
		name, body, header, part string
		want                     []assertion
	}{
		{name: "octave-four-and-quarter-tone", body: `<note><pitch><step>C</step><alter>-0.5</alter><octave>4</octave></pitch><duration>1</duration></note>`, want: []assertion{{validation.ContextOctaveBasis, "octave-four-begins-at-middle-c", validation.EvidenceKnown, nil}, {validation.ContextAlterSemitones, "-0.5", validation.EvidenceKnown, nil}}},
		{name: "other-kind-and-add-reference", body: `<harmony><root><root-step>C</root-step></root><kind>other</kind><degree><degree-value>7</degree-value><degree-alter>-1</degree-alter><degree-type>add</degree-type></degree></harmony>`, want: []assertion{{validation.ContextHarmonyDescription, "added-degrees", validation.EvidenceKnown, nil}, {validation.ContextHarmonyActive, "yes", validation.EvidenceKnown, nil}, {validation.ContextDegreeReference, "dominant-with-minor-seventh", validation.EvidenceKnown, nil}, {validation.ContextDegreeTextComposition, "", validation.EvidenceAbsent, &no}}},
		{name: "alter-reference", body: `<harmony><root><root-step>D</root-step></root><kind>major</kind><degree><degree-value text="degree">3</degree-value><degree-alter>1</degree-alter><degree-type>alter</degree-type></degree></harmony>`, want: []assertion{{validation.ContextDegreeReference, "harmony-kind-degree", validation.EvidenceKnown, nil}, {validation.ContextDegreeTextComposition, "degree", validation.EvidenceKnown, &no}}},
		{name: "generic-font-families", body: `<direction><direction-type><words font-family="music, engraved, handwritten, text, serif, custom">word</words></direction-type></direction>`, want: []assertion{{validation.ContextFontFamilyCategories, "music,music,music,text,text,specific", validation.EvidenceKnown, nil}}},
		{name: "opaque-srgb", body: `<direction><direction-type><words color="#12ABCD" font-family="First, Second">text</words></direction-type></direction>`, want: []assertion{{validation.ContextColorRGB, "#12ABCD", validation.EvidenceKnown, nil}, {validation.ContextColorAlpha, "255", validation.EvidenceKnown, nil}, {validation.ContextFontFamilyOrder, "comma-separated-preference-order", validation.EvidenceKnown, nil}, {validation.ContextFontFamilyCategories, "specific,specific", validation.EvidenceKnown, nil}}},
		{name: "custom-open-domains", header: `<identification><creator type="custom creator">A</creator><encoding><encoder type="custom encoder">B</encoder></encoding></identification><defaults><appearance><line-width type="new-width">0</line-width><distance type="new-distance">-1</distance></appearance></defaults>`, want: []assertion{{validation.ContextMetadataRole, "custom creator|custom encoder", validation.EvidenceKnown, nil}, {validation.ContextLineWidthType, "new-width", validation.EvidenceKnown, nil}, {validation.ContextDistanceType, "new-distance", validation.EvidenceKnown, nil}}},
		{name: "empty-name-overrides", part: `<part-name-display/><part-abbreviation>Ordinary</part-abbreviation><part-abbreviation-display/>`, want: []assertion{{validation.ContextPartNameDisplay, "yes", validation.EvidenceKnown, nil}, {validation.ContextPartAbbreviationDisplay, "yes", validation.EvidenceKnown, nil}}},
		{name: "absent-name-overrides", want: []assertion{{validation.ContextPartNameDisplay, "no", validation.EvidenceKnown, nil}, {validation.ContextPartAbbreviationDisplay, "no", validation.EvidenceKnown, nil}}},
		{name: "supports-qualified-claim", header: `<identification><encoding><supports type="yes" element="print" attribute="new-system" value="yes"/></encoding></identification>`, want: []assertion{{validation.ContextSupportsElement, "print", validation.EvidenceKnown, nil}, {validation.ContextSupportsAttribute, "new-system", validation.EvidenceKnown, nil}, {validation.ContextSupportsType, "yes", validation.EvidenceKnown, nil}, {validation.ContextSupportsValue, "yes", validation.EvidenceKnown, &yes}, {validation.ContextSupportUnreported, "", validation.EvidenceUnknown, nil}}},
		{name: "supports-omission-unspecified", want: []assertion{{validation.ContextSupportUnreported, "", validation.EvidenceUnknown, nil}}},
		{name: "ensemble-one", part: `<score-instrument id="I1"><instrument-name>Voice</instrument-name><ensemble>1</ensemble></score-instrument>`, want: []assertion{{validation.ContextEnsembleSize, "1", validation.EvidenceKnown, nil}}},
		{name: "dashed-active", body: `<note><rest/><duration>1</duration><notations><slur type="start" line-type="dashed" dash-length="0" space-length="2"/></notations></note>`, want: []assertion{{validation.ContextDashLength, "0", validation.EvidenceKnown, &yes}, {validation.ContextSpaceLength, "2", validation.EvidenceKnown, &yes}}},
		{name: "slur-omitted-line-type-unavailable", body: `<note><rest/><duration>1</duration><notations><slur type="start" dash-length="3"/></notations></note>`, want: []assertion{{validation.ContextDashLength, "3", validation.EvidenceKnown, nil}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, tw := range []bool{false, true} {
				for _, model := range []bool{false, true} {
					source := localContextFixture(c.body, tw)
					if c.header != "" {
						source = strings.Replace(source, `<part-list>`, c.header+`<part-list>`, 1)
					}
					if c.part != "" {
						source = strings.Replace(source, `</part-name>`, `</part-name>`+c.part, 1)
					}
					report := localContextAssess(t, engine, source, model)
					if !report.Satisfied() {
						t.Fatalf("%s", localFindings(report))
					}
					for _, want := range c.want {
						var rows []validation.ContextResult
						for _, row := range report.Context {
							if row.Property == want.property {
								rows = append(rows, row)
							}
						}
						values := strings.Split(want.value, "|")
						if len(rows) != len(values) {
							t.Fatalf("%s rows=%d want%d", want.property, len(rows), len(values))
						}
						for i, row := range rows {
							if row.Value.State != want.state {
								t.Fatalf("%s %+v", want.property, row)
							}
							if want.state == validation.EvidenceKnown {
								got := row.Value.Value.Text
								switch row.Value.Value.Kind {
								case validation.AtomicInteger:
									got = row.Value.Value.Integer.String()
								case validation.AtomicDecimal:
									got = row.Value.Value.Decimal.String()
								}
								if got != values[i] {
									t.Fatalf("%s got%s want%s", want.property, got, values[i])
								}
							}
							if want.active != nil && (row.Layout.Applicability.State != validation.EvidenceKnown || row.Layout.Applicability.Value != *want.active) {
								t.Fatalf("%s applicability %+v", want.property, row.Layout.Applicability)
							}
							if c.name == "slur-omitted-line-type-unavailable" && row.Layout.Applicability.State != validation.EvidenceUnknown {
								t.Fatalf("omitted slur style guessed: %+v", row)
							}
						}
					}
				}
			}
		})
	}
}

func TestLocalInheritedLyricAndExplicitNameOrigins(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<note print-lyric="no"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><lyric><text>word</text></lyric></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			source := strings.Replace(localContextFixture(body, tw), `</part-name>`, `</part-name><part-name-display/>`, 1)
			report := localContextAssess(t, engine, source, model)
			seen := false
			for _, row := range report.Context {
				if row.Property == validation.ContextPrintObject && row.Value.State == validation.EvidenceKnown && row.Value.Value.Text == "no" {
					seen = true
					if row.Layout.Provenance != validation.ContextInherited || row.Layout.Origin == row.Subject || row.Value.Value.Text != "no" {
						t.Fatalf("lyric inherited origin %+v", row)
					}
				}
				if row.Property == validation.ContextPartNameDisplay {
					if row.Layout.Provenance != validation.ContextExplicit || row.Layout.Origin == row.Subject {
						t.Fatalf("explicit child origin %+v", row)
					}
				}
			}
			if !seen {
				t.Fatal("lyric output missing")
			}
		}
	}
}

func TestLocalStructuralInvalidPrerequisitesRemainCausal(t *testing.T) {
	engine := localContextTestEngine(t)
	cases := []struct {
		body       string
		properties []validation.ContextProperty
	}{
		{`<note><pitch><step>C</step><alter>oops</alter><octave>12</octave></pitch><duration>1</duration></note>`, []validation.ContextProperty{validation.ContextAlterSemitones, validation.ContextOctaveBasis}},
		{`<attributes><clef><sign>G</sign><line>oops</line></clef></attributes>`, []validation.ContextProperty{validation.ContextStaffLineBasis}},
		{`<direction><direction-type><words color="#GG0000">text</words></direction-type></direction>`, []validation.ContextProperty{validation.ContextColorRGB, validation.ContextColorAlpha}},
		{`<harmony><root><root-step>C</root-step></root><kind>major</kind><degree><degree-value>7</degree-value><degree-alter>0</degree-alter><degree-type>wrong</degree-type></degree></harmony>`, []validation.ContextProperty{validation.ContextDegreeReference}},
	}
	for _, c := range cases {
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				source := localContextFixture(c.body, tw)
				if model && strings.Contains(source, "<line>oops</line>") {
					continue
				}
				if model && strings.Contains(source, "oops") {
					source = strings.Replace(source, "<alter>oops</alter>", "", 1)
				}
				report := localContextAssess(t, engine, source, model)
				for _, property := range c.properties {
					if model && property == validation.ContextAlterSemitones {
						continue
					}
					count := 0
					for _, row := range report.Context {
						if row.Property == property {
							count++
							if row.Value.State != validation.EvidenceInvalid || row.Value.Cause == nil {
								t.Fatalf("%s %+v", property, row)
							}
						}
					}
					if count != 1 {
						t.Fatalf("%s rows%d", property, count)
					}
				}
			}
		}
	}
}

func TestLocalAdvisoryRecommendationsAreObservableAndNonrejecting(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<attributes><divisions>0.5</divisions><key><cancel location="before-barline">1</cancel><fifths>0</fifths></key><clef><sign>none</sign></clef></attributes><harmony><root><root-step>C</root-step></root><kind>none</kind></harmony><direction><direction-type><harp-pedals><pedal-tuning><pedal-step>A</pedal-step><pedal-alter>0</pedal-alter></pedal-tuning><pedal-tuning><pedal-step>D</pedal-step><pedal-alter>0</pedal-alter></pedal-tuning></harp-pedals></direction-type></direction><direction><direction-type><principal-voice type="stop" symbol="Hauptstimme">analysis</principal-voice></direction-type></direction><note><rest/><duration>1</duration><lyric><text>well-known</text></lyric></note><barline location="left"/>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			source := localContextFixture(body, tw)
			source = strings.Replace(source, `<part-list>`, `<identification><creator>A</creator><encoding><encoder>X</encoder><encoder>Y</encoder></encoding></identification><part-list>`, 1)
			report := localContextAssess(t, engine, source, model)
			if !report.Satisfied() {
				t.Fatalf("advisory forms rejected: %s", localFindings(report))
			}
			expected := map[string]string{
				"integral-divisions": "prefer-integral-divisions-for-MIDI-interoperability-and-roundoff", "cancel-midmeasure": "treat-before-barline-as-left-when-midmeasure", "deprecated-clef": "prefer-clef-print-object-over-deprecated-none-sign", "harmony-none-root": "set-root-step-text-to-empty-to-suppress-root", "measure-text-hide": "use-implicit-yes-to-hide-measure-number-rather-than-empty-text", "measure-label-alignment": "aligned-partwise-measures-prefer-equal-labels-nonnumeric-and-repeated-labels-remain-permitted", "barline-order": "prefer-first-musical-element-except-print-bookmark-link", "harp-pedal-order": "prefer-D-C-B-E-F-G-A-for-easiest-reading", "principal-analysis": "analysis-only-principal-voice-prefers-symbol-none", "principal-stop": "visible-principal-voice-stop-prefers-symbol-plain", "creator-role": "prefer-creator-type-even-for-one-creator-open-role-vocabulary", "encoder-role": "multiple-encoders-can-use-open-contribution-role-types", "supports-reporting": "report-supported-accidental-beam-stem-encoding-to-interpret-omissions", "lyric-literal-hyphen": "reserve-literal-hyphens-for-actually-hyphenated-words-use-syllabic-for-syllable-separation"}
			for name, want := range expected {
				var rows []validation.ContextResult
				for _, row := range report.Context {
					if row.Property == validation.ContextProperty("local.advisory-"+name) {
						rows = append(rows, row)
					}
				}
				count := 1
				if name == "integral-divisions" || name == "encoder-role" {
					count = 2
				}
				if len(rows) != count {
					t.Fatalf("%s rows%d want%d", name, len(rows), count)
				}
				for i, row := range rows {
					if row.Value.State != validation.EvidenceKnown || row.Value.Value.Text != want {
						t.Fatalf("%s %+v", name, row)
					}
					app := row.Layout.Applicability
					if name == "cancel-midmeasure" || name == "principal-analysis" || name == "measure-label-alignment" && !tw {
						if app.State != validation.EvidenceUnknown {
							t.Fatalf("%s unresolved antecedent %+v", name, app)
						}
					} else {
						wantActive := true
						if name == "measure-label-alignment" && tw || name == "integral-divisions" && i == 1 {
							wantActive = false
						}
						if app.State != validation.EvidenceKnown || app.Value != wantActive {
							t.Fatalf("%s applicability %+v", name, app)
						}
					}
				}
			}
		}
	}
}

func TestLocalAdvisoryFalseConditionsAndNumericDomains(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<attributes><divisions>1</divisions><key><cancel>1</cancel><fifths>0</fifths></key><clef><sign>G</sign><line>0</line></clef></attributes><harmony><root><root-step>C</root-step></root><kind>major</kind></harmony><direction><direction-type><principal-voice type="stop" symbol="none">analysis</principal-voice></direction-type></direction><note><pitch><step>C</step><alter>0.5</alter><octave>0</octave></pitch><duration>1</duration><lyric><text>hello</text></lyric></note><note><pitch><step>B</step><octave>9</octave></pitch><duration>1</duration></note><barline location="middle"/>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			source := strings.Replace(localContextFixture(body, tw), `<part-list>`, `<identification><encoding><encoder>One</encoder></encoding></identification><part-list>`, 1)
			report := localContextAssess(t, engine, source, model)
			if !report.Satisfied() {
				t.Fatal(localFindings(report))
			}
			seen := map[string]bool{}
			for _, row := range report.Context {
				name := strings.TrimPrefix(string(row.Property), "local.advisory-")
				switch name {
				case "integral-divisions", "cancel-midmeasure", "deprecated-clef", "harmony-none-root", "barline-order", "principal-stop", "encoder-role", "lyric-literal-hyphen":
					seen[name] = true
					if row.Layout.Applicability.State != validation.EvidenceKnown || row.Layout.Applicability.Value {
						t.Fatalf("false condition %s %+v", name, row)
					}
				}
			}
			if len(seen) != 8 {
				t.Fatalf("missing controls %+v", seen)
			}
			count := 0
			for _, row := range report.Context {
				if row.Property == validation.ContextOctaveBasis {
					count++
					if row.Value.State != validation.EvidenceKnown {
						t.Fatal(row)
					}
				}
				if row.Property == validation.ContextStaffLineBasis && row.Value.State != validation.EvidenceKnown {
					t.Fatalf("line zero is permitted: %+v", row)
				}
			}
			if count != 2 {
				t.Fatalf("octave boundaries%d", count)
			}
		}
	}
}
func TestLocalAdvisoryInvalidInputsRemainCausal(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<attributes><divisions>0</divisions><clef><sign>wrong</sign></clef></attributes><direction><direction-type><principal-voice type="stop" symbol="wrong"/></direction-type></direction><note><rest/><duration>1</duration></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, localContextFixture(body, tw), model)
			seen := map[string]bool{}
			for _, row := range report.Context {
				name := strings.TrimPrefix(string(row.Property), "local.advisory-")
				switch name {
				case "integral-divisions":
					if row.Layout.Raw.State != validation.EvidenceInvalid {
						continue
					}
					fallthrough
				case "deprecated-clef", "principal-analysis", "principal-stop":
					seen[name] = true
					if row.Layout.Applicability.State != validation.EvidenceInvalid || row.Layout.Applicability.Cause == nil {
						t.Fatalf("causal advisory %s %+v", name, row)
					}
				}
			}
			if len(seen) != 4 {
				t.Fatalf("missing causal advice %+v", seen)
			}
		}
	}
}
