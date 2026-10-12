package musicxml

import (
	"context"
	"encoding/xml"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

// This independent harness selects the real local context provider over the
// unchanged source/model prerequisites. It never selects unfinished siblings.
func localContextTestEngine(t *testing.T) *validation.Engine {
	t.Helper()
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	schema := d.schemas[validation.AssemblyMusicXML]
	plan, err := validation.NewLocalContextPlan(schema)
	if err != nil {
		t.Fatal(err)
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
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := validation.NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine
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
