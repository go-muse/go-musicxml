package musicxml

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type spanGapSink struct {
	*validation.Session
	headers map[validation.NodeID]validation.NodeHeader
	gapName string
}

func (s *spanGapSink) OpenNode(ctx context.Context, n validation.NodeHeader) error {
	s.headers[n.ID] = n
	return s.Session.OpenNode(ctx, n)
}
func (s *spanGapSink) CompleteAttributes(ctx context.Context, id validation.NodeID, v validation.Evidence[bool]) error {
	if "@"+s.headers[id].Name.Local == s.gapName {
		v = validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "test-span-attributes-gap"}
	}
	return s.Session.CompleteAttributes(ctx, id, v)
}
func (s *spanGapSink) CloseNode(ctx context.Context, n validation.NodeEnd) error {
	if s.headers[n.ID].Name.Local == s.gapName {
		n.ChildrenComplete = validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "test-span-enumeration-gap"}
	}
	if "@"+s.headers[n.ID].Name.Local == s.gapName {
		n.AttributesComplete = validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "test-span-attributes-gap"}
	}
	return s.Session.CloseNode(ctx, n)
}

type spanEngineFixture struct {
	engine   *validation.Engine
	profiles []validation.Profile
}

// The compiled engines are immutable; the two provider configurations have
// distinct caches. Never share accounts, sessions, models, sinks or reports.
var spanTestEngineWithoutTimeline = sync.OnceValues(func() (spanEngineFixture, error) {
	return buildSpanTestEngine(false)
})
var spanTestEngineWithTimeline = sync.OnceValues(func() (spanEngineFixture, error) {
	return buildSpanTestEngine(true)
})

func spanTestEngine(t *testing.T, withTimeline bool) (*validation.Engine, []validation.Profile) {
	t.Helper()
	get := spanTestEngineWithoutTimeline
	if withTimeline {
		get = spanTestEngineWithTimeline
	}
	fixture, err := get()
	if err != nil {
		t.Fatal(err)
	}
	// Preserve the fixture's original profile metadata while giving each caller
	// its own slices, including the nested required/support/root inventories.
	profiles := append([]validation.Profile(nil), fixture.profiles...)
	for i := range profiles {
		profiles[i].Required = append([]validation.RuleKey(nil), profiles[i].Required...)
		profiles[i].Support = append([]validation.RuleSupport(nil), profiles[i].Support...)
		profiles[i].SupportedRoots = append([]validation.QName(nil), profiles[i].SupportedRoots...)
	}
	return fixture.engine, profiles
}

func buildSpanTestEngine(withTimeline bool) (spanEngineFixture, error) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		return spanEngineFixture{}, err
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
				return spanEngineFixture{}, err
			}
			owners = append(owners, validation.ReferenceOwner{Declaration: part.Declaration, Attribute: a, Rule: validation.RuleKey{ID: validation.RuleID("MX40-LOCAL-part-id-target/use-" + strconv.Itoa(i+1)), Revision: 1}})
		}
		timeline = validation.RuleKey{ID: "span-test.timeline", Revision: 1}
		p, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: owners})
		if err != nil {
			return spanEngineFixture{}, err
		}
		add(timeline, p, nil)
	}
	p, err := validation.NewSpanContextPlan(validation.ContextPlanSpec{Schema: schema, TimelineProvider: timeline})
	if err != nil {
		return spanEngineFixture{}, err
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
		return spanEngineFixture{}, err
	}
	engine, err := validation.NewEngine(c)
	if err != nil {
		return spanEngineFixture{}, err
	}
	return spanEngineFixture{engine: engine, profiles: profiles}, nil
}
func spanBoth(t *testing.T, source string, withTimeline bool, check func(*testing.T, validation.Report)) {
	t.Helper()
	spanBothWithFailures(t, source, withTimeline, false, check)
}
func spanBothWithFailures(t *testing.T, source string, withTimeline, allowFailures bool, check func(*testing.T, validation.Report)) {
	t.Helper()
	spanBothWithGap(t, source, withTimeline, allowFailures, "", check)
}
func spanBothWithGap(t *testing.T, source string, withTimeline, allowFailures bool, gapName string, check func(*testing.T, validation.Report)) {
	t.Helper()
	engine, profiles := spanTestEngine(t, withTimeline)
	spanBothWithEngine(t, engine, profiles, source, allowFailures, gapName, check)
}

// Only immutable compiled catalog/engine setup is shared. Every leaf creates
// its own account, session, sink, input reader/model, completion and report.
func spanBothWithEngine(t *testing.T, engine *validation.Engine, profiles []validation.Profile, source string, allowFailures bool, gapName string, check func(*testing.T, validation.Report)) {
	t.Helper()
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
			sink := &spanGapSink{Session: session, headers: map[validation.NodeID]validation.NodeHeader{}, gapName: gapName}
			var report validation.Report
			if profile.Target == validation.TargetModel {
				doc, err := Decode(strings.NewReader(source))
				if err != nil {
					t.Fatal(err)
				}
				_, walkErr := walkModel(ctx, doc, sink, account, documentID, modelWalkOptions{})
				report, err = session.Finish(ctx, validation.Completion{InputComplete: walkErr == nil, Syntax: validation.SyntaxNotAssessed, Cause: walkErr})
				if walkErr != nil {
					t.Fatal(walkErr)
				}
			} else {
				lexical, e := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: ctx, Account: account})
				if e != nil {
					t.Fatal(e)
				}
				observer, e := newSourceFactObserver(ctx, sink, account)
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
			if !allowFailures && report.Counts.Fail != 0 {
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

func TestContextSpanTiesPitchPassAndLocalOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		family   validation.SpanFamily
		paired   bool
		property validation.ContextProperty
		want     string
	}{
		{"enharmonic", `<note><pitch><step>C</step><alter>1</alter><octave>4</octave></pitch><duration>1</duration><notations><tied type="start" number="1"/></notations></note><note><pitch><step>D</step><alter>-1</alter><octave>4</octave></pitch><duration>1</duration><notations><tied type="stop" number="1"/></notations></note>`, validation.SpanTied, true, validation.ContextPropertySpanPitchEquivalentRecommended, "true"},
		{"inequivalent-advisory", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><tied type="start" number="1"/></notations></note><note><pitch><step>D</step><octave>4</octave></pitch><duration>1</duration><notations><tied type="stop" number="1"/></notations></note>`, validation.SpanTied, true, validation.ContextPropertySpanPitchEquivalentRecommended, "false"},
		{"all-pass-absent-voice-instrument", timelineNote("1", "", `<tie type="start"/>`) + timelineNote("1", "", `<tie type="stop"/>`), validation.SpanTie, true, "", ""},
		{"equivalent-pass-spellings", timelineNote("1", "", `<tie type="start" time-only="1, 2"/>`) + timelineNote("1", "", `<tie type="stop" time-only="1,2"/>`), validation.SpanTie, true, "", ""},
		{"different-pass-guards", timelineNote("1", "", `<tie type="start" time-only="1"/>`) + timelineNote("1", "", `<tie type="stop" time-only="2"/>`), validation.SpanTie, false, "", ""},
		{"attack-discouraged", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><tie type="start"/></note><note attack="0"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><tie type="stop"/></note>`, validation.SpanTie, true, validation.ContextPropertySpanTieAttackRecommended, "false"},
		{"sound-start-stop-advisory", timelineNote("1", "", `<tie type="start"/><tie type="stop"/>`), validation.SpanTie, true, validation.ContextPropertySpanSameNoteOrderRecommended, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spanBoth(t, (localFixture{body: `<attributes><divisions>1</divisions></attributes>` + tc.body}).xml(), true, func(t *testing.T, r validation.Report) {
				es := spanResults(r, tc.family)
				if len(es) != 2 {
					t.Fatalf("ties %+v", es)
				}
				for _, e := range es {
					if (e.Association.State == validation.EvidenceKnown) != tc.paired {
						t.Fatalf("pairing %+v", e)
					}
				}
				if tc.property != "" {
					vs := spanProperty(r, tc.property)
					found := false
					for _, v := range vs {
						if v.State == validation.EvidenceKnown && v.Value.Text == tc.want {
							found = true
						}
					}
					if !found {
						t.Fatalf("property %s %+v want %s", tc.property, vs, tc.want)
					}
				}
			})
		})
	}
}
func TestContextSpanNestedTupletOverrideAndNonLIFOClose(t *testing.T) {
	body := `<attributes><divisions>3</divisions></attributes>` + timelineNote("2", "", `<type>quarter</type><time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification><notations><tuplet type="start" number="1"/></notations>`) + timelineNote("1", "", `<type>eighth</type><time-modification><actual-notes>9</actual-notes><normal-notes>4</normal-notes><normal-type>quarter</normal-type></time-modification><notations><tuplet type="start" number="2"><tuplet-actual><tuplet-number>3</tuplet-number><tuplet-type>eighth</tuplet-type></tuplet-actual><tuplet-normal><tuplet-number>1</tuplet-number><tuplet-type>quarter</tuplet-type></tuplet-normal></tuplet></notations>`) + timelineNote("1", "", `<type>eighth</type><time-modification><actual-notes>9</actual-notes><normal-notes>4</normal-notes><normal-type>quarter</normal-type></time-modification><notations><tuplet type="stop" number="1"/><tuplet type="stop" number="2"/></notations>`)
	spanBoth(t, (localFixture{body: body}).xml(), true, func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanTuplet)
		if len(es) != 4 {
			t.Fatal(es)
		}
		for _, e := range es {
			if e.Association.State != validation.EvidenceKnown {
				t.Fatal(e)
			}
		}
		actual := spanProperty(r, validation.ContextPropertySpanTupletActualNumber)
		normal := spanProperty(r, validation.ContextPropertySpanTupletNormalNumber)
		if actual[1].Value.Integer.String() != "3" || normal[1].Value.Integer.String() != "1" {
			t.Fatalf("cumulative ratio overwrote display %+v %+v", actual, normal)
		}
	})
}
func TestContextSpanLyricExtensionHistoricalAndSelectorScope(t *testing.T) {
	for _, version := range []string{"2.0", "4.0"} {
		source := `<score-partwise version="` + version + `"><part-list><score-part id="P1"><part-name>Singer</part-name></score-part></part-list><part id="P1"><measure number="1">` + timelineNote("1", "", `<lyric number="verse-A"><text>word</text><extend/></lyric>`) + `</measure></part></score-partwise>`
		spanBoth(t, source, false, func(t *testing.T, r validation.Report) {
			es := spanResults(r, validation.SpanLyricExtend)
			if len(es) != 1 {
				t.Fatal(es)
			}
			if version == "2.0" {
				if es[0].Role.State != validation.EvidenceKnown || es[0].Role.Value != "start" {
					t.Fatal(es)
				}
			} else if es[0].Role.State != validation.EvidenceUnknown {
				t.Fatal("invented 4.0 start")
			}
		})
	}
	body := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<lyric number="A"><text>a</text><extend type="start"/></lyric><lyric number="B"><text>b</text><extend type="start"/></lyric>`) + timelineNote("1", "", `<lyric number="A"><extend type="stop"/></lyric><lyric number="B"><extend type="stop"/></lyric>`)
	spanBoth(t, (localFixture{body: body}).xml(), true, func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanLyricExtend)
		if len(es) != 4 {
			t.Fatal(es)
		}
		for _, e := range es {
			if e.Association.State != validation.EvidenceKnown {
				t.Fatal(e)
			}
		}
		if es[0].Members[0] == es[1].Members[0] {
			t.Fatal("lyric lines merged")
		}
	})
}
func TestContextSpanTremoloAndBarlineRecommendations(t *testing.T) {
	body := `<attributes><divisions>2</divisions></attributes>` + timelineNote("3", "", `<type>half</type><dot/><time-modification><actual-notes>2</actual-notes><normal-notes>1</normal-notes></time-modification><notations><ornaments><tremolo type="start">2</tremolo></ornaments></notations>`) + timelineNote("3", "", `<type>half</type><dot/><time-modification><actual-notes>2</actual-notes><normal-notes>1</normal-notes></time-modification><notations><ornaments><tremolo type="stop">2</tremolo></ornaments></notations>`) + timelineNote("1", "", `<notations><ornaments><tremolo type="unmeasured">2</tremolo></ornaments></notations>`) + `<barline><wavy-line type="start"/></barline>`
	spanBoth(t, (localFixture{body: body}).xml(), true, func(t *testing.T, r validation.Report) {
		for _, p := range []validation.ContextProperty{validation.ContextPropertySpanTremoloRatioRecommended, validation.ContextPropertySpanTremoloDurationRecommended} {
			vs := spanProperty(r, p)
			if len(vs) != 2 {
				t.Fatal(vs)
			}
			for _, v := range vs {
				if v.State != validation.EvidenceKnown || v.Value.Text != "true" {
					t.Fatalf("%s %+v", p, v)
				}
			}
		}
		for _, p := range []validation.ContextProperty{validation.ContextPropertySpanTremoloMarksRecommended, validation.ContextPropertySpanBarlineContinueRecommended} {
			vs := spanProperty(r, p)
			if len(vs) != 1 || vs[0].Value.Text != "false" {
				t.Fatalf("advisory %s %+v", p, vs)
			}
		}
	})
}

func TestContextSpanVerticalChordMembership(t *testing.T) {
	for _, family := range []validation.SpanFamily{validation.SpanArpeggiate, validation.SpanNonArpeggiate} {
		t.Run(string(family), func(t *testing.T) {
			mark := func(typ, number string) string {
				if family == validation.SpanArpeggiate {
					return `<arpeggiate number="` + number + `"/>`
				}
				return `<non-arpeggiate type="` + typ + `" number="` + number + `"/>`
			}
			note := func(chord, typ, number, staff string) string {
				return `<note>` + chord + `<pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><staff>` + staff + `</staff><notations>` + mark(typ, number) + `</notations></note>`
			}
			body := `<attributes><divisions>1</divisions><staves>2</staves></attributes>` + note("", "bottom", "1", "1") + note("<chord/>", "top", "1", "2") + note("", "bottom", "2", "1") + note("<chord/>", "top", "2", "1")
			spanBoth(t, (localFixture{body: body}).xml(), true, func(t *testing.T, r validation.Report) {
				es := spanResults(r, family)
				if len(es) != 4 {
					t.Fatal(es)
				}
				for i, e := range es {
					if e.Association.State != validation.EvidenceKnown || len(e.Members) != 2 {
						t.Fatalf("%d %+v", i, e)
					}
				}
				if es[0].Members[0] == es[2].Members[0] {
					t.Fatal("different onsets merged")
				}
			})
		})
	}
}
func TestContextSpanMetronomeRelationSeparatesSides(t *testing.T) {
	body := `<direction><direction-type><metronome><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="start"/></metronome-note><metronome-relation>equals</metronome-relation><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="stop"/></metronome-note></metronome></direction-type></direction>`
	spanBoth(t, (localFixture{body: body}).xml(), false, func(t *testing.T, r validation.Report) {
		for _, e := range spanResults(r, validation.SpanMetronomeTied) {
			if e.Association.State == validation.EvidenceKnown {
				t.Fatalf("cross-side association %+v", e)
			}
		}
	})
}

func TestContextSpanTremoloSMuFLLocalConditions(t *testing.T) {
	for _, tc := range []struct {
		name, mark, want string
		state            validation.EvidenceState
	}{
		{"unmeasured-default", `<tremolo type="unmeasured">0</tremolo>`, "buzzRoll", validation.EvidenceKnown},
		{"unmeasured-explicit", `<tremolo type="unmeasured" smufl="unvalidatedGlyph">0</tremolo>`, "unvalidatedGlyph", validation.EvidenceKnown},
		{"single-ignored", `<tremolo type="single" smufl="unvalidatedGlyph">1</tremolo>`, "", validation.EvidenceAbsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spanBoth(t, (localFixture{body: timelineNote("1", "", `<notations><ornaments>`+tc.mark+`</ornaments></notations>`)}).xml(), false, func(t *testing.T, r validation.Report) {
				v := spanProperty(r, validation.ContextPropertySpanTremoloSMuFL)[0]
				if v.State != tc.state || v.Value.Text != tc.want {
					t.Fatal(v)
				}
			})
		})
	}
}

func TestContextSpanInvalidScalarCauseBothAdapters(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<notations><slur type="start"/></notations>`) + timelineNote("1", "", `<notations><slur type="start" number="99"/></notations>`) + timelineNote("1", "", `<notations><slur type="stop"/></notations>`)
	spanBothWithFailures(t, (localFixture{body: body}).xml(), true, true, func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanSlur)
		if len(es) != 3 {
			t.Fatal(es)
		}
		if r.Counts.Fail == 0 {
			t.Fatal("missing XSD scalar owner failure")
		}
		for _, e := range es {
			if e.Association.State != validation.EvidenceInvalid || e.Association.Cause == nil {
				t.Fatal(e)
			}
			if e.Role.State != validation.EvidenceKnown {
				t.Fatal("lost independent role")
			}
		}
	})
}
func TestContextSpanPartGroupEmptyReuseAndMissingBoundary(t *testing.T) {
	source := `<score-partwise version="4.0"><part-list><part-group type="start" number="A"/><part-group type="stop" number="A"/><part-group type="start" number="A"/><score-part id="P1"><part-name>Piano</part-name></score-part><part-group type="stop" number="A"><group-name>ignored</group-name><group-symbol>brace</group-symbol></part-group><part-group type="start" number="B"/></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	spanBoth(t, source, false, func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanPartGroup)
		if len(es) != 5 {
			t.Fatal(es)
		}
		if es[0].Association.State != validation.EvidenceKnown || len(es[0].Included) != 0 || len(es[2].Included) != 1 || es[4].Association.State != validation.EvidenceUnknown {
			t.Fatal(es)
		}
		active := spanProperty(r, validation.ContextPropertySpanGroupStopContentActive)
		if active[3].Value.Text != "false" {
			t.Fatal(active)
		}
	})
}

func TestContextSpanMissingEnumerationBothAdapters(t *testing.T) {
	for _, tc := range []struct {
		name, gap, body string
		family          validation.SpanFamily
	}{
		{"slur-domain", "measure", timelineNote("1", "", `<notations><slur type="start"/></notations>`) + timelineNote("1", "", `<notations><slur type="stop"/></notations>`), validation.SpanSlur},
		{"note-vertical", "note", timelineNote("1", "", `<notations><arpeggiate/></notations>`), validation.SpanArpeggiate},
		{"local-let-ring-survives", "measure", timelineNote("1", "", `<notations><tied type="let-ring"/></notations>`), validation.SpanTied},
		{"local-tie-stub-needs-domain-enumeration", "measure", timelineNote("1", "", `<notations><tied type="start"/><tied type="stop"/></notations>`), validation.SpanTied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spanBothWithGap(t, (localFixture{body: `<attributes><divisions>1</divisions></attributes>` + tc.body}).xml(), true, false, tc.gap, func(t *testing.T, r validation.Report) {
				es := spanResults(r, tc.family)
				if len(es) == 0 {
					t.Fatal("no projections")
				}
				for _, e := range es {
					if tc.name == "local-tie-stub-needs-domain-enumeration" && (e.Association.State != validation.EvidenceUnsupported || e.Association.Reason != "identity-scope-incomplete" || len(e.Members) != 0) {
						t.Fatal("stub bypassed adapter identity gate", e)
					}
					if strings.Contains(tc.name, "survives") {
						if e.Association.State != validation.EvidenceKnown {
							t.Fatal(e)
						}
					} else if e.Association.State == validation.EvidenceKnown {
						t.Fatal("fabricated closed enumeration", e)
					}
					if e.Role.State != validation.EvidenceKnown && tc.family != validation.SpanArpeggiate {
						t.Fatal("lost local role")
					}
				}
			})
		})
	}
	source := `<score-partwise version="4.0"><part-list><part-group type="start"/><score-part id="P1"><part-name>Piano</part-name></score-part><part-group type="stop"/></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	spanBothWithGap(t, source, false, false, "part-list", func(t *testing.T, r validation.Report) {
		for _, e := range spanResults(r, validation.SpanPartGroup) {
			if e.Association.State == validation.EvidenceKnown || len(e.Included) > 0 {
				t.Fatal("incomplete group interval", e)
			}
		}
	})
}

func TestContextSpanInvalidPitchRetainsCausalAdvisory(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<notations><tied type="start" number="1"/></notations>`) + `<note><pitch><step>C</step><octave>99</octave></pitch><duration>1</duration><notations><tied type="stop" number="1"/></notations></note>`
	spanBothWithFailures(t, (localFixture{body: body}).xml(), true, true, func(t *testing.T, r validation.Report) {
		advice := spanProperty(r, validation.ContextPropertySpanPitchEquivalentRecommended)
		if len(advice) != 1 || advice[0].State != validation.EvidenceInvalid || advice[0].Cause == nil {
			t.Fatal(advice)
		}
		for _, e := range spanResults(r, validation.SpanTied) {
			if e.Association.State != validation.EvidenceKnown {
				t.Fatal("explicit graphical identity unnecessarily erased", e)
			}
		}
	})
}

func TestContextSpanLyricGrammarOwnerAndMissingFacts(t *testing.T) {
	body := timelineNote("1", "", `<lyric><syllabic>begin</syllabic><text>one</text><syllabic>end</syllabic><text>two</text></lyric>`)
	spanBothWithFailures(t, (localFixture{body: body}).xml(), false, true, func(t *testing.T, r validation.Report) {
		if r.Counts.Fail == 0 {
			t.Fatal("missing shared grammar owner")
		}
		for _, v := range r.Context {
			if v.Property == validation.ContextPropertySpanLyricSyllable && v.Span != nil && v.Span.Association.State == validation.EvidenceKnown {
				t.Fatal("grouped malformed lyric", v)
			}
		}
	})
	body = timelineNote("1", "", `<lyric><text>one</text><elision> </elision><text>two</text></lyric>`)
	spanBothWithGap(t, (localFixture{body: body}).xml(), false, false, "lyric", func(t *testing.T, r validation.Report) {
		v := spanResults(r, validation.SpanLyricSyllable)
		if len(v) != 1 || v[0].Association.State != validation.EvidenceUnknown {
			t.Fatal(v)
		}
	})
}

func TestContextSpanLocalDefaultAxesBothAdapters(t *testing.T) {
	engine, profiles := spanTestEngine(t, false)
	for _, c := range []struct {
		name, element, attribute, def, explicit string
		property                                validation.ContextProperty
		direction                               bool
	}{
		{"octave-size", "octave-shift", "size", "8", "15", validation.ContextPropertySpanSize, true},
		{"bracket-line", "bracket", "line-type", "solid", "dashed", validation.ContextPropertySpanLineType, true},
		{"pedal-line", "pedal", "line", "no", "yes", validation.ContextPropertySpanPedalLine, true},
		{"wedge-niente", "wedge", "niente", "no", "yes", validation.ContextPropertySpanWedgeNiente, true},
		{"glissando", "glissando", "line-type", "wavy", "solid", validation.ContextPropertySpanLineType, false},
		{"slide", "slide", "line-type", "solid", "wavy", validation.ContextPropertySpanLineType, false},
		{"tuplet-number-display", "tuplet", "show-number", "actual", "both", validation.ContextPropertySpanTupletShowNumber, false},
		{"tuplet-type-display", "tuplet", "show-type", "none", "actual", validation.ContextPropertySpanTupletShowType, false},
		{"tuplet-shape", "tuplet", "line-shape", "straight", "curved", validation.ContextPropertySpanLineShape, false},
		{"arpeggio-direction", "arpeggiate", "direction", "up", "down", validation.ContextPropertySpanArpeggioDirection, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, axis := range []string{"positive", "negative", "boundary", "missing-fact", "interaction"} {
				t.Run(axis, func(t *testing.T) {
					role := ` type="start"`
					if c.element == "octave-shift" {
						role = ` type="down"`
					}
					if c.element == "wedge" {
						role = ` type="crescendo"`
					}
					if c.element == "arpeggiate" {
						role = ""
					}
					if c.element == "bracket" {
						role += ` line-end="up"`
					}
					attr := ""
					want := c.def
					if axis == "boundary" {
						attr = ` ` + c.attribute + `="` + c.explicit + `"`
						want = c.explicit
					}
					if axis == "negative" {
						bad := "invalid-value"
						if c.attribute == "size" {
							bad = "0"
						}
						attr = ` ` + c.attribute + `="` + bad + `"`
					}
					mark := `<` + c.element + role + attr + `/>`

					body := ""
					if c.direction {
						body = `<direction><direction-type>` + mark + `</direction-type></direction>`
					} else {
						body = timelineNote("1", "", `<notations>`+mark+`</notations>`)
					}
					if axis == "interaction" {
						other := `<` + c.element + role + ` ` + c.attribute + `="` + c.explicit + `"/>`
						if c.direction {
							body += `<direction><direction-type>` + other + `</direction-type></direction>`
						} else {
							body += timelineNote("1", "", `<notations>`+other+`</notations>`)
						}
					}
					gap := ""
					if axis == "missing-fact" {
						gap = "@" + c.element
					}
					spanBothWithEngine(t, engine, profiles, (localFixture{body: body}).xml(), axis == "negative", gap, func(t *testing.T, r validation.Report) {
						vs := spanProperty(r, c.property)
						if len(vs) == 0 {
							t.Fatal("missing property")
						}
						v := vs[0]
						if axis == "negative" {
							if v.State != validation.EvidenceInvalid || v.Cause == nil {
								t.Fatal(v)
							}
							return
						}
						if axis == "missing-fact" {
							if v.State != validation.EvidenceUnknown && v.State != validation.EvidenceUnsupported {
								t.Fatal(v)
							}
							return
						}
						got := v.Value.Text
						if v.Value.Kind == validation.AtomicInteger {
							got = v.Value.Integer.String()
						}
						if v.State != validation.EvidenceKnown || got != want {
							t.Fatalf("%+v want %s", v, want)
						}
						if axis == "interaction" {
							if len(vs) != 2 {
								t.Fatal(vs)
							}
							got = vs[1].Value.Text
							if vs[1].Value.Kind == validation.AtomicInteger {
								got = vs[1].Value.Integer.String()
							}
							if got != c.explicit {
								t.Fatal(vs)
							}
						}
					})
				})
			}
		})
	}
}

func TestContextSpanContinuationAssociationAxesBothAdapters(t *testing.T) {
	for _, axis := range []string{"positive", "negative", "boundary", "missing-fact", "interaction"} {
		t.Run(axis, func(t *testing.T) {
			number := "1"
			if axis == "negative" {
				number = "99"
			}
			start := `<slur type="start"/>`
			middle := `<slur type="continue" number="` + number + `"/>`
			stop := `<slur type="stop"/>`
			if axis == "boundary" {
				start, stop = stop, start
			}
			if axis == "interaction" {
				start += `<slur type="start" number="2"/>`
				middle += `<slur type="continue" number="2"/>`
				stop += `<slur type="stop" number="2"/>`
			}
			body := `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", `<notations>`+start+`</notations>`) + timelineNote("1", "", `<notations>`+middle+`</notations>`) + timelineNote("1", "", `<notations>`+stop+`</notations>`)
			// Reverse XML roles deliberately use no timeline: local endpoint roles remain
			// preserved, while chronology and part identity remain unavailable.
			timeline := axis != "boundary"
			gap := ""
			if axis == "missing-fact" {
				gap = "notations"
			}
			spanBothWithGap(t, (localFixture{body: body}).xml(), timeline, axis == "negative", gap, func(t *testing.T, r validation.Report) {
				es := spanResults(r, validation.SpanSlur)
				if len(es) < 3 {
					t.Fatal(es)
				}
				for _, e := range es {
					if axis == "negative" {
						if e.Association.State != validation.EvidenceInvalid || e.Association.Cause == nil {
							t.Fatal(e)
						}
						continue
					}
					if axis == "missing-fact" || axis == "boundary" {
						if e.Association.State == validation.EvidenceKnown {
							t.Fatal("unknown domain or enumeration", e)
						}
						continue
					}
					if e.Association.State != validation.EvidenceKnown || len(e.Members) != 2 {
						t.Fatal(e)
					}
				}
				if axis == "positive" && es[1].Members[0] != es[0].Members[0] {
					t.Fatal("continuation identity lost")
				}
				if axis == "interaction" && es[0].Members[0] == es[1].Members[0] {
					t.Fatal("number overlap merged")
				}
			})
		})
	}
}

func TestContextSpanPitchUnavailableDoesNotInventEquality(t *testing.T) {
	note := func(role string) string {
		return `<note><unpitched><display-step>C</display-step><display-octave>4</display-octave></unpitched><duration>1</duration><notations><tied type="` + role + `" number="1"/></notations></note>`
	}
	spanBoth(t, (localFixture{body: `<attributes><divisions>1</divisions></attributes>` + note("start") + note("stop")}).xml(), true, func(t *testing.T, r validation.Report) {
		v := spanProperty(r, validation.ContextPropertySpanPitchEquivalentRecommended)
		if len(v) != 1 || v[0].State != validation.EvidenceUnknown {
			t.Fatal(v)
		}
	})
}

func TestContextSpanBeamLevelRepeaterAndMissingVoice(t *testing.T) {
	for _, number := range []string{"8", "9"} {
		t.Run(number, func(t *testing.T) {
			body := `<attributes><divisions>1</divisions><staves>2</staves></attributes>` + timelineNote("1", "", `<staff>1</staff><beam number="`+number+`" repeater="yes">begin</beam>`) + timelineNote("1", "", `<staff>2</staff><beam number="`+number+`">continue</beam>`) + timelineNote("1", "", `<staff>2</staff><beam number="`+number+`">end</beam>`)
			spanBothWithFailures(t, (localFixture{body: body}).xml(), true, number == "9", func(t *testing.T, r validation.Report) {
				es := spanResults(r, validation.SpanBeam)
				if len(es) != 3 {
					t.Fatal(es)
				}
				for _, e := range es {
					if number == "9" {
						if e.Number.State != validation.EvidenceInvalid || e.Association.State != validation.EvidenceInvalid {
							t.Fatal(e)
						}
					} else if e.Association.State != validation.EvidenceKnown || len(e.Members) != 2 {
						t.Fatal(e)
					}
				}
				repeat := spanProperty(r, validation.ContextPropertySpanRepeater)
				if repeat[0].Value.Text != "yes" || repeat[1].State != validation.EvidenceAbsent || repeat[2].State != validation.EvidenceAbsent {
					t.Fatal("inherited repeater", repeat)
				}
			})
		})
	}
}

func TestContextSpanHistoricalExtendRequiresVersionEvidence(t *testing.T) {
	body := timelineNote("1", "", `<lyric number="A"><text>la</text><extend/></lyric>`)
	spanBothWithGap(t, (localFixture{body: body}).xml(), false, false, "@score-partwise", func(t *testing.T, r validation.Report) {
		es := spanResults(r, validation.SpanLyricExtend)
		for _, e := range es {
			if e.Role.State == validation.EvidenceKnown {
				t.Fatal("fabricated historical type", es)
			}
		}
	})
}

func TestContextSpanTieAdjustmentPresenceAxesBothAdapters(t *testing.T) {
	for _, axis := range []string{"positive", "zero-present", "missing-fact", "interaction"} {
		t.Run(axis, func(t *testing.T) {
			attrs := ""
			if axis == "zero-present" || axis == "interaction" {
				attrs = ` attack="0" release="0"`
			}
			note := func(a string) string {
				return `<note` + a + `><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><tie type="stop"/><tie type="start"/></note>`
			}
			body := note(attrs)
			if axis == "interaction" {
				body += note("")
			}
			gap := ""
			if axis == "missing-fact" {
				gap = "@note"
			}
			spanBothWithGap(t, (localFixture{body: body}).xml(), false, false, gap, func(t *testing.T, r validation.Report) {
				for _, p := range []validation.ContextProperty{validation.ContextPropertySpanTieAttackRecommended, validation.ContextPropertySpanTieReleaseRecommended} {
					vs := spanProperty(r, p)
					if len(vs) == 0 {
						if axis == "missing-fact" {
							continue
						}
						t.Fatal("no advice")
					}
					v := vs[0]
					if axis == "missing-fact" {
						if v.State != validation.EvidenceUnknown && v.State != validation.EvidenceUnsupported {
							t.Fatal(v)
						}
						continue
					}
					want := "true"
					if attrs != "" {
						want = "false"
					}
					if v.State != validation.EvidenceKnown || v.Value.Text != want {
						t.Fatal(v)
					}
					if axis == "interaction" && (len(vs) != 2 || vs[1].Value.Text != "true") {
						t.Fatal("adjustment presence leaked", vs)
					}
				}
			})
		})
	}
}

func TestContextSpanMetronomeMissingEnumeration(t *testing.T) {
	body := `<direction><direction-type><metronome><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="start"/></metronome-note><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="stop"/></metronome-note></metronome></direction-type></direction>`
	spanBothWithGap(t, (localFixture{body: body}).xml(), false, false, "metronome", func(t *testing.T, r validation.Report) {
		for _, e := range spanResults(r, validation.SpanMetronomeTied) {
			if e.Association.State == validation.EvidenceKnown {
				t.Fatal("incomplete metronome association", e)
			}
		}
	})
}

func TestContextSpanPartGroupReverseIsUnresolved(t *testing.T) {
	source := `<score-partwise version="4.0"><part-list><part-group type="stop"/><score-part id="P1"><part-name>Piano</part-name></score-part><part-group type="start"/></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	spanBoth(t, source, false, func(t *testing.T, r validation.Report) {
		for _, e := range spanResults(r, validation.SpanPartGroup) {
			if e.Association.State != validation.EvidenceUnknown || len(e.Included) != 0 {
				t.Fatal("reversed group interval", e)
			}
		}
	})
}
