package musicxml

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func spanTestEngine(t *testing.T, withTimeline bool) (*validation.Engine, []validation.Profile) {
	t.Helper()
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	schema := d.schemas[validation.AssemblyMusicXML]
	assembly := validation.AssemblyMusicXML
	var extra []validation.RuleKey
	add := func(key validation.RuleKey, p validation.Plan, deps []validation.Dependency) {
		d.rules = append(d.rules, validation.RuleDefinition{Key: key, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeDocument, Role: validation.RoleContextProvider, Class: validation.ClassMandatory, Category: validation.CategoryNormative, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Requires: deps, Text: validation.RuleText{Title: "Span test", Explanation: "Exact span context test.", Message: "Context unavailable."}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-number-level-overlap"}, URL: "https://www.w3.org/2021/06/musicxml40/musicxml-reference/data-types/number-level/", Locator: "span"}}})
		d.plans = append(d.plans, validation.PlanBinding{ID: string(key.ID), Plan: p})
		extra = append(extra, key)
	}
	timeline := validation.RuleKey{}
	if withTimeline {
		b := newLocalCatalogBuilder(schema)
		var owners []validation.ReferenceOwner
		for i, part := range b.namedElementUses("part") {
			a, err := b.typeAttribute(part.DeclaredType, "id")
			if err != nil {
				t.Fatal(err)
			}
			owners = append(owners, validation.ReferenceOwner{Declaration: part.Declaration, Attribute: a, Rule: validation.RuleKey{ID: validation.RuleID("MX40-LOCAL-part-id-target/use-" + strconv.Itoa(i+1)), Revision: 1}})
		}
		timeline = validation.RuleKey{ID: "span-test.timeline", Revision: 1}
		p, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: owners})
		if err != nil {
			t.Fatal(err)
		}
		add(timeline, p, nil)
	}
	p, err := validation.NewSpanContextPlan(validation.ContextPlanSpec{Schema: schema, TimelineProvider: timeline})
	if err != nil {
		t.Fatal(err)
	}
	var deps []validation.Dependency
	if withTimeline {
		deps = []validation.Dependency{{Kind: validation.DependencyRule, Rule: timeline, Relation: validation.RelationDocument}}
	}
	add(validation.RuleKey{ID: "span-test.spans", Revision: 1}, p, deps)
	var profiles []validation.Profile
	for _, base := range d.profiles {
		if base.ID != ProfileSourceLocal && base.ID != ProfileModelLocal {
			continue
		}
		base.ID = validation.ProfileID("span-test/" + string(base.Target))
		base.Required = append(append([]validation.RuleKey{}, base.Required...), extra...)
		base.Support = append([]validation.RuleSupport{}, base.Support...)
		for _, key := range extra {
			base.Support = append(base.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
		}
		profiles = append(profiles, base)
	}
	c, err := validation.NewCatalog(d.rules, profiles, d.plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := validation.NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	return engine, profiles
}
func spanBoth(t *testing.T, source string, withTimeline bool, check func(*testing.T, validation.Report)) {
	t.Helper()
	engine, profiles := spanTestEngine(t, withTimeline)
	for _, profile := range profiles {
		t.Run(string(profile.Target), func(t *testing.T) {
			ctx := context.Background()
			account, err := validation.NewBudgetAccount(validation.Budgets{})
			if err != nil {
				t.Fatal(err)
			}
			documentID := sourceDocumentID
			if profile.Target == validation.TargetModel {
				documentID = modelDocumentID
			}
			session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: documentID, Profile: profile.ID, Account: account})
			if err != nil {
				t.Fatal(err)
			}
			var report validation.Report
			if profile.Target == validation.TargetModel {
				doc, err := Decode(strings.NewReader(source))
				if err != nil {
					t.Fatal(err)
				}
				_, walkErr := walkModel(ctx, doc, session, account, documentID, modelWalkOptions{})
				report, err = session.Finish(ctx, validation.Completion{InputComplete: walkErr == nil, Syntax: validation.SyntaxNotAssessed, Cause: walkErr})
				if walkErr != nil {
					t.Fatal(walkErr)
				}
			} else {
				lexical, e := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: ctx, Account: account})
				if e != nil {
					t.Fatal(e)
				}
				observer, e := newSourceFactObserver(ctx, session, account)
				if e != nil {
					t.Fatal(e)
				}
				run := &sourceAssessmentRun{ctx: ctx, session: session, account: account, source: &observingXMLSource{ctx: ctx, lexical: lexical, decoder: xml.NewTokenDecoder(lexical), observer: observer}}
				report, err = run.finish(run.drain())
			}
			if err != nil {
				t.Fatalf("%v %s", err, localFindings(report))
			}
			if !report.InputComplete {
				t.Fatalf("incomplete input: %s", localFindings(report))
			}
			if report.Counts.Fail != 0 {
				t.Fatalf("fixture unexpectedly invalid: %s", localFindings(report))
			}
			check(t, report)
		})
	}
}
func spanResults(r validation.Report, f validation.SpanFamily) []validation.SpanContextResult {
	var out []validation.SpanContextResult
	for _, x := range r.Context {
		if x.Span != nil && x.Span.Family == f {
			out = append(out, *x.Span)
		}
	}
	return out
}
func spanProperty(r validation.Report, p validation.ContextProperty) []validation.Evidence[validation.Atomic] {
	var out []validation.Evidence[validation.Atomic]
	for _, x := range r.Context {
		if x.Property == p {
			out = append(out, x.Value)
		}
	}
	return out
}
func TestContextSpanExactBindingsAndOrdinaryPairs(t *testing.T) {
	source := (localFixture{body: `<attributes><divisions>1</divisions><staves>2</staves></attributes>` + timelineNote("1", "", `<staff>1</staff><notations><slur type="start"/></notations>`) + timelineNote("1", "", `<staff>2</staff><notations><slur type="stop" number="1"/></notations>`)}).xml()
	spanBoth(t, source, true, func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanSlur)
		if len(es) != 2 {
			t.Fatalf("got %d spans; %s", len(es), localFindings(r))
		}
		for _, e := range es {
			if e.Association.State != validation.EvidenceKnown || len(e.Members) != 2 {
				t.Fatalf("missing association %+v", e)
			}
		}
		if es[0].NumberExplicit.State != validation.EvidenceAbsent && es[0].NumberExplicit.Value {
			t.Fatalf("fabricated explicit number %+v", es[0])
		}
		if !es[1].NumberExplicit.Value {
			t.Fatal("lost explicit number")
		}
	})
}
func TestContextSpanLocalFactsWithoutTimeline(t *testing.T) {
	source := (localFixture{body: timelineNote("1", "", `<type>eighth</type><time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification><notations><tied type="let-ring"/><tied type="start"/><tied type="stop"/><tuplet type="start"/></notations><lyric><text>a</text><text>b</text><elision> </elision><text>c</text></lyric>`)}).xml()
	spanBoth(t, source, false, func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanTied)
		if len(es) != 3 {
			t.Fatalf("ties %d", len(es))
		}
		for _, e := range es {
			if e.Association.State != validation.EvidenceKnown {
				t.Fatalf("local tie depends on timeline %+v", e)
			}
			if e.Identity != validation.SpanIdentityImpliedTie {
				t.Fatalf("implied number lost %+v", e)
			}
		}
		ss := spanResults(r, validation.SpanLyricSyllable)
		if len(ss) != 2 || len(ss[0].Members) != 2 || len(ss[1].Members) != 1 {
			t.Fatalf("syllable groups %+v", ss)
		}
		actual := spanProperty(r, validation.ContextPropertySpanTupletActualNumber)
		if len(actual) != 1 || actual[0].Value.Integer.String() != "3" {
			t.Fatalf("actual fallback %+v", actual)
		}
		bracket := spanProperty(r, validation.ContextPropertySpanTupletBracket)
		if len(bracket) != 1 || bracket[0].State != validation.EvidenceUnknown {
			t.Fatalf("invented bracket %+v", bracket)
		}
	})
}
func TestContextPartGroupOverlapsWithoutTimeline(t *testing.T) {
	source := `<score-partwise version="4.0"><part-list><part-group type="start" number="A"/><score-part id="P1"><part-name>1</part-name></score-part><part-group type="start" number="B"/><score-part id="P2"><part-name>2</part-name></score-part><part-group type="stop" number="A"/><score-part id="P3"><part-name>3</part-name></score-part><part-group type="stop" number="B"/></part-list><part id="P1"><measure number="a"/></part><part id="P2"><measure number="a"/></part><part id="P3"><measure number="a"/></part></score-partwise>`
	spanBoth(t, source, false, func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanPartGroup)
		if len(es) != 4 {
			t.Fatalf("groups %+v", es)
		}
		for _, e := range es {
			if e.Association.State != validation.EvidenceKnown || len(e.Included) != 2 {
				t.Fatalf("LIFO or lost membership %+v", e)
			}
		}
	})
}

func TestContextSpanFamiliesExecuteCorrespondence(t *testing.T) {
	for _, tc := range []struct {
		name        string
		family      validation.SpanFamily
		first, last string
		direction   bool
	}{
		{"slur", validation.SpanSlur, `<slur type="start"/>`, `<slur type="stop"/>`, false},
		{"glissando", validation.SpanGlissando, `<glissando type="start"/>`, `<glissando type="stop"/>`, false},
		{"slide", validation.SpanSlide, `<slide type="start"/>`, `<slide type="stop"/>`, false},
		{"hammer", validation.SpanHammerOn, `<technical><hammer-on type="start"/></technical>`, `<technical><hammer-on type="stop"/></technical>`, false},
		{"pull", validation.SpanPullOff, `<technical><pull-off type="start"/></technical>`, `<technical><pull-off type="stop"/></technical>`, false},
		{"wavy", validation.SpanWavyLine, `<ornaments><wavy-line type="start"/></ornaments>`, `<ornaments><wavy-line type="stop"/></ornaments>`, false},
		{"tuplet", validation.SpanTuplet, `<tuplet type="start"/>`, `<tuplet type="stop"/>`, false},
		{"tied", validation.SpanTied, `<tied type="start"/>`, `<tied type="stop"/>`, false},
		{"octave", validation.SpanOctaveShift, `<octave-shift type="down"/>`, `<octave-shift type="stop"/>`, true},
		{"bracket", validation.SpanBracket, `<bracket type="start" line-end="up"/>`, `<bracket type="stop" line-end="none"/>`, true},
		{"dashes", validation.SpanDashes, `<dashes type="start"/>`, `<dashes type="stop"/>`, true},
		{"pedal", validation.SpanPedal, `<pedal type="resume" line="yes"/>`, `<pedal type="discontinue" line="yes"/>`, true},
		{"wedge", validation.SpanWedge, `<wedge type="diminuendo"/>`, `<wedge type="stop"/>`, true},
		{"principal", validation.SpanPrincipalVoice, `<principal-voice type="start" symbol="none"/>`, `<principal-voice type="stop" symbol="none"/>`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body string
			if tc.direction {
				body = `<direction><direction-type>` + tc.first + `</direction-type></direction>` + timelineNote("1", "", "") + `<direction><direction-type>` + tc.last + `</direction-type></direction>`
			} else {
				body = timelineNote("1", "", `<notations>`+tc.first+`</notations>`) + timelineNote("1", "", `<notations>`+tc.last+`</notations>`)
			}
			spanBoth(t, (localFixture{body: `<attributes><divisions>1</divisions></attributes>` + body}).xml(), true, func(t *testing.T, r validation.Report) {
				es := spanResults(r, tc.family)
				if len(es) != 2 {
					t.Fatalf("family %s: %+v", tc.family, es)
				}
				for _, e := range es {
					if e.Association.State != validation.EvidenceKnown || len(e.Members) != 2 {
						t.Fatalf("no indexed correspondence %+v", e)
					}
				}
			})
		})
	}
}
func TestContextSpanDocumentOrderCounterexampleBothHierarchies(t *testing.T) {
	body := `<attributes><divisions>1</divisions><staves>2</staves></attributes>` +
		timelineNote("1", "", `<staff>1</staff>`) + timelineNote("1", "", `<staff>1</staff><notations><slur type="start"/></notations>`) + timelineNote("1", "", `<staff>1</staff><notations><slur type="stop"/></notations>`) + timelineNote("1", "", `<staff>1</staff><notations><slur type="stop"/></notations>`) + `<backup><duration>4</duration></backup>` +
		timelineNote("1", "", `<staff>2</staff><notations><slur type="start"/></notations>`) + timelineNote("1", "", `<staff>2</staff><notations><slur type="start"/></notations>`) + timelineNote("1", "", `<staff>2</staff><notations><slur type="stop"/></notations>`)
	for _, source := range []string{(localFixture{body: body}).xml(), `<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><measure number="same"><part id="P1">` + body + `</part></measure></score-timewise>`} {
		spanBoth(t, source, true, func(t *testing.T, r validation.Report) {
			es := spanResults(r, validation.SpanSlur)
			if len(es) != 6 {
				t.Fatalf("got %d", len(es))
			}
			for _, e := range es {
				if e.Association.State != validation.EvidenceKnown {
					t.Fatalf("musical sort or staff split %+v", e)
				}
			}
			if es[2].Members[0] != es[3].Members[0] || es[2].Role.Value != "stop" {
				t.Fatal("reverse document object lost")
			}
		})
	}
}
func TestContextSpanBeamFanAndWedgeApplicability(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes>` +
		timelineNote("1", "", `<voice>x</voice><beam fan="accel">begin</beam>`) + timelineNote("1", "", `<voice>x</voice><beam fan="rit">continue</beam>`) + timelineNote("1", "", `<voice>x</voice><beam>end</beam>`) +
		`<direction><direction-type><wedge type="diminuendo"/></direction-type></direction>` + timelineNote("1", "", "") + `<direction><direction-type><wedge type="stop"/></direction-type></direction>`
	spanBoth(t, (localFixture{body: body}).xml(), true, func(t *testing.T, r validation.Report) {
		fan := spanProperty(r, validation.ContextPropertySpanFanApplicable)
		if len(fan) != 3 || fan[2].Value.Text != "true" {
			t.Fatalf("fan %+v", fan)
		}
		wedge := spanProperty(r, validation.ContextPropertySpanWedgeNienteActive)
		if len(wedge) != 2 || wedge[0].Value.Text != "false" || wedge[1].Value.Text != "true" {
			t.Fatalf("wedge %+v", wedge)
		}
	})
}
func TestContextSpanFrameMetronomeAndSingleDomains(t *testing.T) {
	frame := `<harmony><root><root-step>C</root-step></root><kind>major</kind><frame><frame-strings>6</frame-strings><frame-frets>4</frame-frets><frame-note><string>6</string><fret>1</fret><barre type="start"/></frame-note><frame-note><string>1</string><fret>1</fret><barre type="stop"/></frame-note></frame></harmony>`
	metronome := `<direction><direction-type><metronome><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="start"/></metronome-note><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="stop"/></metronome-note></metronome></direction-type></direction>`
	spanBoth(t, (localFixture{body: frame + metronome + `<grouping type="single"/>` + timelineNote("1", "", `<notations><other-notation type="single">X</other-notation><ornaments><tremolo>2</tremolo></ornaments></notations>`)}).xml(), false, func(t *testing.T, r validation.Report) {
		for _, f := range []validation.SpanFamily{validation.SpanBarre, validation.SpanMetronomeTied, validation.SpanGrouping, validation.SpanOtherNotation, validation.SpanTremolo} {
			es := spanResults(r, f)
			if len(es) == 0 {
				t.Fatal(f)
			}
			for _, e := range es {
				if e.Association.State != validation.EvidenceKnown {
					t.Fatalf("domain %s %+v", f, e)
				}
			}
		}
	})
}
