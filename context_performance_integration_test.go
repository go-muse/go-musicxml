package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func performanceReports(t *testing.T, body string, check func(*testing.T, validation.Report)) {
	t.Helper()
	for _, timewise := range []bool{false, true} {
		src := contextFixtureXML(body, timewise)
		doc, err := Decode(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			name := "source"
			if model {
				name = "model"
			}
			if timewise {
				name += "-timewise"
			}
			t.Run(name, func(t *testing.T) {
				var r validation.Report
				var err error
				if model {
					r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
				} else {
					r, err = InspectXML(context.Background(), strings.NewReader(src), AssessmentOptions{Profile: ProfileSourceContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				check(t, r)
			})
		}
	}
}
func performanceTiming(r validation.Report, kind validation.Code) []*validation.TimingContextResult {
	var out []*validation.TimingContextResult
	for _, c := range r.Context {
		if c.Timing != nil && c.Timing.Kind == kind {
			out = append(out, c.Timing)
		}
	}
	return out
}
func TestPerformanceSourceModelDefaultsTiming(t *testing.T) {
	body := `<attributes><divisions>4</divisions><transpose><diatonic>1</diatonic><chromatic>2</chromatic><octave-change>1</octave-change></transpose></attributes><direction><direction-type><words>test</words></direction-type><sound tempo="120" dynamics="125"/></direction><note attack="1" release="-1"><pitch><step>C</step><alter>0.5</alter><octave>4</octave></pitch><duration>4</duration><type>quarter</type><notations><ornaments><mordent/></ornaments><technical><fingering>2</fingering><bend><bend-alter>2</bend-alter></bend><bend><bend-alter>-2</bend-alter><release/></bend></technical></notations></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		sounds := performanceTiming(r, "sound")
		if len(sounds) != 1 || sounds[0].ReferenceVelocity.State != validation.EvidenceKnown || sounds[0].ReferenceVelocity.Value.String() != "225/2" {
			t.Fatalf("sound reference: %+v; findings %s", sounds, localFindings(r))
		}
		notes := performanceTiming(r, "note")
		if len(notes) != 1 {
			t.Fatalf("notes=%d", len(notes))
		}
		n := notes[0]
		if n.Duration.Value.String() != "1" || n.Attack.Delta.Value.String() != "1/4" || n.Release.Delta.Value.String() != "-1/4" || n.ChromaticDisplacement.Value.String() != "14" || n.WrittenPitch.Value.String() != "97/2" || n.SoundingPitch.Value.String() != "125/2" {
			t.Errorf("timing %+v", n)
		}
		expected := map[validation.ContextProperty]string{"perf-mordent-start-note": "main", "perf-mordent-beats": "3", "perf-mordent-second-beat": "12", "perf-mordent-last-beat": "24", "perf-fingering-substitution": "no", "perf-bend-first-beat": "25"}
		for _, c := range r.Context {
			if want, ok := expected[c.Property]; ok {
				got := c.Value.Value.Text
				if c.Value.Value.Kind == validation.AtomicDecimal {
					got = c.Value.Value.Decimal.String()
				}
				if c.Value.State != validation.EvidenceKnown || got != want {
					t.Errorf("%s=%+v want%s", c.Property, c.Value, want)
				}
				delete(expected, c.Property)
			}
		}
		if len(expected) != 0 {
			t.Errorf("missing %v", expected)
		}
		bends := performanceTiming(r, "bend")
		if len(bends) != 2 || bends[1].BendNegativeApplicable.State != validation.EvidenceKnown || !bends[1].BendNegativeSatisfied.Value {
			t.Errorf("bend proofs %+v", bends)
		}
	})
}
func TestPerformanceSwingAndCueAcrossTargets(t *testing.T) {
	body := `<attributes><divisions>4</divisions></attributes><direction><direction-type><words>Swing</words></direction-type><sound><swing><first>3</first><second>2</second></swing></sound></direction><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><type size="cue">eighth</type></note><note><cue/><pitch><step>D</step><octave>4</octave></pitch><duration>3</duration><type>eighth</type><dot/></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		notes := performanceTiming(r, "note")
		if len(notes) != 2 {
			t.Fatal(len(notes), localFindings(r))
		}
		if notes[0].Cue.State != validation.EvidenceKnown || notes[0].Cue.Value || notes[0].SwingEligible.State != validation.EvidenceKnown || !notes[0].SwingEligible.Value {
			t.Errorf("ordinary cue-size eligibility %+v", notes[0])
		}
		if !notes[1].Cue.Value || notes[1].SwingEligible.State != validation.EvidenceKnown || notes[1].SwingEligible.Value {
			t.Errorf("dotted cue %+v", notes[1])
		}
	})
}

func TestPerformanceCapoTuningEvidence(t *testing.T) {
	for _, tc := range []struct {
		capo, want string
		known      bool
	}{{"<capo>0</capo>", "105/2", true}, {"<capo>2</capo>", "109/2", true}, {"", "", false}} {
		body := `<attributes><divisions>1</divisions><staff-details><staff-lines>6</staff-lines><staff-tuning line="1"><tuning-step>E</tuning-step><tuning-alter>0.5</tuning-alter><tuning-octave>4</tuning-octave></staff-tuning>` + tc.capo + `</staff-details></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`
		performanceReports(t, body, func(t *testing.T, r validation.Report) {
			notes := performanceTiming(r, "note")
			if len(notes) != 1 || len(notes[0].Tunings) != 1 {
				t.Fatalf("missing tuning %s", localFindings(r))
			}
			g := notes[0].Tunings[0]
			if tc.known {
				if g.AdjustedPitch.State != validation.EvidenceKnown || g.AdjustedPitch.Value.String() != tc.want {
					t.Errorf("capo %+v", g)
				}
			} else if g.Capo.State != validation.EvidenceAbsent || g.AdjustedPitch.State != validation.EvidenceUnknown {
				t.Errorf("absent capo invented %+v", g)
			}
		})
	}
}
func TestPerformanceAllStaticFamiliesAndOrigins(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes><direction><direction-type><percussion><stick><stick-type>snare</stick-type><stick-material>wood</stick-material></stick></percussion></direction-type></direction><harmony><root><root-step>C</root-step><root-alter print-object="no">1</root-alter></root><kind>major</kind></harmony><figured-bass><figure><prefix>custom-prefix</prefix><figure-number>6</figure-number><suffix>custom-suffix</suffix></figure></figured-bass><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><level reference="yes">original</level><notations><slide type="start"/><ornaments><trill-mark beats="3.5"/><accidental-mark>sharp</accidental-mark><turn/><shake/><wavy-line type="start"/><inverted-mordent/></ornaments><technical><fret>0</fret><fingering alternate="yes">2</fingering></technical></notations></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		expected := map[validation.ContextProperty]string{"perf-slide-beats": "4", "perf-empty-trill-sound-beats": "3.5", "perf-horizontal-turn-start-note": "upper", "perf-wavy-line-last-beat": "75", "perf-mordent-start-note": "main", "perf-level-reference": "yes", "perf-level-type": "single", "perf-stick-parentheses": "no", "perf-stick-dashed-circle": "no", "perf-root-alter-location": "right", "performance-figure-prefix": "custom-prefix", "performance-figure-suffix": "custom-suffix"}
		for _, c := range r.Context {
			want, ok := expected[c.Property]
			if !ok {
				continue
			}
			got := c.Value.Value.Text
			if c.Value.Value.Kind == validation.AtomicDecimal {
				got = c.Value.Value.Decimal.String()
			}
			if got == want && c.Value.State == validation.EvidenceKnown {
				delete(expected, c.Property)
			}
		}
		if len(expected) != 0 {
			t.Errorf("missing static facts %v; findings %s", expected, localFindings(r))
		}
		fret := performanceTiming(r, "fret")
		if len(fret) != 1 || !fret[0].FretOpenString.Value {
			t.Error("fret zero not open")
		}
		level := performanceTiming(r, "editorial-level")
		if len(level) != 1 || !level[0].EditorialPlaybackSuppressed.Value {
			t.Error("reference editorial effect")
		}
		acc := performanceTiming(r, "ornament-accidental")
		if len(acc) != 1 || acc[0].LayoutOrigin.State != validation.EvidenceKnown {
			t.Error("ornament origin missing")
		}
		figs := performanceTiming(r, "figured-bass")
		if len(figs) != 1 || figs[0].Anchor.State != validation.EvidenceKnown || len(figs[0].Figures) != 1 {
			t.Error("figure anchor missing")
		}
	})
}
func TestPerformanceMissingUnitsPreserveLocalDefaults(t *testing.T) {
	performanceReports(t, `<note><cue/><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><bend><bend-alter>2</bend-alter></bend></technical></notations></note>`, func(t *testing.T, r validation.Report) {
		notes := performanceTiming(r, "note")
		if len(notes) != 1 || notes[0].Cue.State != validation.EvidenceKnown || !notes[0].Cue.Value || notes[0].Duration.State == validation.EvidenceKnown {
			t.Error("local cue lost or units invented")
		}
		found := false
		for _, c := range r.Context {
			if c.Property == "perf-bend-beats" {
				found = c.Value.State == validation.EvidenceKnown && c.Value.Value.Decimal.String() == "4"
			}
		}
		if !found {
			t.Error("independent default lost")
		}
	})
}
func TestPerformanceInvalidBlocksOnlyField(t *testing.T) {
	performanceReports(t, `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><bend first-beat="-1"><bend-alter>2</bend-alter></bend></technical></notations></note>`, func(t *testing.T, r validation.Report) {
		invalid, known := false, false
		for _, c := range r.Context {
			if c.Property == "perf-bend-first-beat" {
				invalid = c.Value.State == validation.EvidenceInvalid && c.Value.Cause != nil
			}
			if c.Property == "perf-bend-last-beat" {
				known = c.Value.State == validation.EvidenceKnown && c.Value.Value.Decimal.String() == "75"
			}
		}
		if !invalid || !known {
			t.Errorf("field isolation invalid=%t default=%t: %s", invalid, known, localFindings(r))
		}
	})
}
func TestPerformanceBendConditionalNegativePredicate(t *testing.T) {
	for _, tc := range []struct {
		sequence string
		fail     bool
	}{{`<bend><bend-alter>2</bend-alter></bend><bend><bend-alter>0</bend-alter><release/></bend>`, true}, {`<bend><bend-alter>0</bend-alter><release/></bend>`, false}, {`<bend><bend-alter>0</bend-alter><pre-bend/></bend>`, false}} {
		performanceReports(t, `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical>`+tc.sequence+`</technical></notations></note>`, func(t *testing.T, r validation.Report) {
			fail := false
			for _, f := range r.Findings {
				if f.Rule.ID == "MX40-CONTEXT-bend-release-negative" && f.Outcome == validation.OutcomeFail {
					fail = true
				}
			}
			if fail != tc.fail {
				t.Errorf("conditional sign fail=%t want%t %s", fail, tc.fail, localFindings(r))
			}
		})
	}
}
func TestPerformanceExactSourceVersusCurrentModel(t *testing.T) {
	src := contextFixtureXML(`<direction><direction-type><words>Dynamics</words></direction-type><sound dynamics="100.00000000000000001"/></direction>`, false)
	source, err := InspectXML(context.Background(), strings.NewReader(src), AssessmentOptions{Profile: ProfileSourceContext})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	model, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
	if err != nil {
		t.Fatal(err)
	}
	a, b := performanceTiming(source, "sound"), performanceTiming(model, "sound")
	if len(a) != 1 || len(b) != 1 {
		t.Fatal("missing source/model sound")
	}
	if a[0].ReferenceVelocity.Value.String() == b[0].ReferenceVelocity.Value.String() || b[0].ReferenceVelocity.Value.String() != "90" {
		t.Errorf("source=%v model=%v", a[0].ReferenceVelocity, b[0].ReferenceVelocity)
	}
}
func TestPerformanceMIDIEdgesAndAngles(t *testing.T) {
	for _, tc := range []struct{ midi, want string }{{"1", "0"}, {"128", "127"}} {
		body := `<sound><midi-instrument id="I1"><midi-unpitched>` + tc.midi + `</midi-unpitched><pan>-90</pan><elevation>90</elevation></midi-instrument></sound>`
		performanceReports(t, body, func(t *testing.T, r validation.Report) {
			m := performanceTiming(r, "midi-unpitched")
			if len(m) != 1 || m[0].MIDINote.State != validation.EvidenceKnown || m[0].MIDINote.Value.String() != tc.want {
				t.Error("MIDI one based", m)
			}
			pan, elevation := performanceTiming(r, "pan"), performanceTiming(r, "elevation")
			if len(pan) != 1 || len(elevation) != 1 || pan[0].Pan.Value.String() != "-90" || elevation[0].Elevation.Value.String() != "90" {
				t.Error("angle projection")
			}
		})
	}
}
func TestPerformanceGraceDonorAndFigures(t *testing.T) {
	body := `<attributes><divisions>4</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><voice>alto</voice></note><note><grace steal-time-previous="25" steal-time-following="50"/><pitch><step>D</step><octave>4</octave></pitch><voice>alto</voice><type>eighth</type></note><figured-bass><figure><figure-number>6</figure-number></figure><duration>4</duration></figured-bass><note><pitch><step>E</step><octave>4</octave></pitch><duration>4</duration><voice>alto</voice></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		notes := performanceTiming(r, "note")
		if len(notes) != 3 {
			t.Fatal(len(notes))
		}
		g := notes[1]
		if g.StealPrevious.Value.String() != "25" || g.StealFollowing.Value.String() != "50" || g.Previous.Note.State != validation.EvidenceKnown || g.Following.Note.State != validation.EvidenceKnown || g.Previous.StolenDuration.Value.String() != "1/4" || g.Following.StolenDuration.Value.String() != "1/2" {
			t.Errorf("grace donors previous=%+v following=%+v %s", g.Previous, g.Following, localFindings(r))
		}
		fig := performanceTiming(r, "figured-bass")
		if len(fig) != 1 || fig[0].Anchor.State != validation.EvidenceKnown {
			t.Error("anchor unavailable")
		}
	})
}

func TestPerformanceLocalStateDefaultsWithoutTimelineUnits(t *testing.T) {
	body := `<attributes><key><fifths>0</fifths></key><time><beats>3</beats><beat-type>4</beat-type></time><staff-details><staff-lines>5</staff-lines></staff-details><transpose><chromatic>2</chromatic><octave-change>1</octave-change></transpose></attributes>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		tim := performanceTiming(r, "time")
		if len(tim) != 1 || tim[0].TimeSymbol.State != validation.EvidenceKnown || tim[0].TimeSymbol.Value != "normal" || tim[0].TimeSeparator.Value != "none" || !tim[0].TimePrintObject.Value {
			t.Error("local time defaults missing")
		}
		key := performanceTiming(r, "key")
		if len(key) != 1 || key[0].KeyFifths.Value.String() != "0" || key[0].KeyCancel.State != validation.EvidenceAbsent || key[0].CancelLocation.State != validation.EvidenceAbsent {
			t.Error("key/cancel absence defaults wrong")
		}
		staff := performanceTiming(r, "staff-details")
		if len(staff) != 1 || staff[0].StaffType.Value != "regular" {
			t.Error("local staff type default")
		}
		trans := performanceTiming(r, "transpose")
		if len(trans) != 1 || trans[0].ChromaticDisplacement.Value.String() != "14" || trans[0].DiatonicDisplacement.State != validation.EvidenceAbsent {
			t.Error("local transpose contributions")
		}
	})
}
func TestPerformanceBendInvalidChoiceIsCausal(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><bend><bend-alter>2</bend-alter></bend><bend><bend-alter>0</bend-alter><pre-bend/><release/></bend></technical></notations></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		b := performanceTiming(r, "bend")
		if len(b) != 2 || b[1].BendNegativeApplicable.State != validation.EvidenceInvalid || b[1].BendNegativeApplicable.Cause == nil {
			t.Errorf("invalid bend choice did not block causally: %s", localFindings(r))
		}
		blocked := false
		for _, f := range r.Findings {
			if f.Rule.ID == "MX40-CONTEXT-bend-release-negative" {
				if f.Outcome == validation.OutcomeFail {
					t.Error("duplicate conditional sign failure")
				}
				if f.Outcome == validation.OutcomeBlocked && f.Cause != nil {
					blocked = true
				}
			}
		}
		if !blocked {
			t.Error("missing causal predicate block")
		}
	})
}

func TestPerformanceStaffScopesOrderAndOpenLabels(t *testing.T) {
	body := `<attributes><divisions>2</divisions><key number="1"><cancel>2</cancel><fifths>-3</fifths><mode>minor</mode></key><time number="1" symbol="cut" separator="diagonal" print-object="no"><senza-misura>free</senza-misura></time><staves>2</staves><instruments>2</instruments><clef number="1"><sign>C</sign><line>8</line></clef><transpose number="1"><chromatic>2</chromatic><octave-change>1</octave-change><double/></transpose></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><voice>upper line</voice><staff>1</staff></note><note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>1</duration><voice>upper line</voice><staff>1</staff></note><note><pitch><step>G</step><octave>4</octave></pitch><duration>2</duration><voice>lower line</voice><staff>2</staff></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		notes := performanceTiming(r, "note")
		if len(notes) != 3 {
			t.Fatal(len(notes))
		}
		a, b, c := notes[0], notes[1], notes[2]
		if a.Staves.Value.String() != "2" || a.Instruments.Value.String() != "2" || a.KeyFifths.Value.String() != "-3" || a.KeyCancel.Value.String() != "2" || a.CancelLocation.Value != "left" || a.ClefLine.Value.String() != "8" || a.DoublingAbove.State != validation.EvidenceKnown || a.DoublingAbove.Value {
			t.Errorf("scoped state a=%+v", a)
		}
		if a.TimeSymbol.State != validation.EvidenceAbsent || a.SenzaMisura.Value != "free" || a.TimeSeparator.Value != "diagonal" || a.TimePrintObject.Value {
			t.Error("senza misura display projection")
		}
		if !b.Chord.Value || b.Origin.Value.Offset.String() != "0" || b.Duration.Value.String() != "1/2" || b.PrincipalDuration.Value.String() != "1" || b.Principal.Value != a.Principal.Value {
			t.Error("chord cursor/duration projection")
		}
		if c.KeyFifths.State != validation.EvidenceAbsent || c.ClefSign.State != validation.EvidenceAbsent || c.TranspositionChromatic.State != validation.EvidenceAbsent || c.Origin.Value.Offset.String() != "1" || c.Voice.Value != "lower line" {
			t.Error("cross staff state leak")
		}
		keys := performanceTiming(r, "key")
		if len(keys) != 1 || keys[0].AllStaves.Value || keys[0].ScopeStaff.Value.String() != "1" {
			t.Error("key selector scope")
		}
		labels := performanceTiming(r, "measure-label")
		if len(labels) != 1 || labels[0].MeasureLabel.Value != "A" {
			t.Error("measure label changed into ordinal")
		}
		parts := performanceTiming(r, "part-order")
		if len(parts) != 1 || parts[0].PartOrder.Value.String() != "1" {
			t.Error("part list ordering")
		}
		if len(a.StateOrigins) == 0 {
			t.Error("state provenance missing")
		}
		found := false
		for _, v := range a.Advisories {
			if v.Clause == "multi-instrument-note-identifier" {
				found = v.Satisfied.State == validation.EvidenceKnown && !v.Satisfied.Value
			}
		}
		if !found {
			t.Error("missing bounded multi-instrument advice")
		}
	})
}
func TestPerformanceMovementsAndSwingRatioAdvice(t *testing.T) {
	body := `<attributes><divisions>2</divisions></attributes><sound><swing><first>6</first><second>4</second><swing-type>16th</swing-type></swing></sound><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration></note><note><pitch><step>D</step><octave>4</octave></pitch><duration>2</duration></note><backup><duration>2</duration></backup><forward><duration>1</duration></forward>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		swing := performanceTiming(r, "swing")
		if len(swing) != 1 || swing[0].SwingRatio.Value.String() != "3/2" || swing[0].SwingType.Value != "16th" {
			t.Error("reduced ratio identity")
		}
		if len(swing[0].Advisories) != 1 || swing[0].Advisories[0].Satisfied.State != validation.EvidenceKnown || swing[0].Advisories[0].Satisfied.Value {
			t.Error("unreduced ratio not separate guidance")
		}
		backup, forward := performanceTiming(r, "backup"), performanceTiming(r, "forward")
		if len(backup) != 1 || len(forward) != 1 || backup[0].CursorAfter.Value.Offset.String() != "1" || forward[0].CursorAfter.Value.Offset.String() != "3/2" {
			t.Error("signed cursor adjustments")
		}
		for _, f := range r.Findings {
			if f.Rule.ID == "MX40-CONTEXT-performance" && f.Outcome == validation.OutcomeFail {
				t.Error("advice became conformance error")
			}
		}
	})
}
func TestPerformanceOffsetsRemainIndependent(t *testing.T) {
	body := `<attributes><divisions>4</divisions></attributes><direction><direction-type><words>Offsets</words></direction-type><offset sound="yes">1</offset><voice>V</voice><sound tempo="120"><offset>0</offset></sound><listening><sync type="none"/><offset>2</offset></listening></direction><note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><voice>V</voice></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		direction := performanceTiming(r, "direction")
		sound := performanceTiming(r, "sound")
		if len(direction) != 1 || direction[0].Position.Value.Offset.String() != "1/4" || len(sound) != 1 || sound[0].Position.Value.Offset.String() != "0" {
			t.Errorf("direction vs sound precedence: %s", localFindings(r))
		}
		listening := false
		for _, c := range r.Context {
			if c.Timing != nil && c.Timing.Kind == "event" && c.Timing.Position.State == validation.EvidenceKnown && c.Timing.Position.Value.Offset.String() == "1/2" {
				listening = true
			}
		}
		if !listening {
			t.Error("own listening offset missing")
		}
	})
}

func TestPerformanceTwoPartIsolationAndSparseStaff(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		header := `<part-list><score-part id="P1"><part-name>One</part-name></score-part><score-part id="P2"><part-name>Two</part-name></score-part></part-list>`
		first := `<attributes><divisions>1</divisions><key><fifths>5</fifths></key><staves>18446744073709551615</staves></attributes><sound dynamics="125"/><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><voice>shared-name</voice><staff>18446744073709551615</staff></note>`
		second := `<attributes><divisions>2</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><voice>shared-name</voice></note>`
		src := `<score-partwise version="4.0">` + header + `<part id="P1"><measure number="pickup-X">` + first + `</measure></part><part id="P2"><measure number="pickup-X">` + second + `</measure></part></score-partwise>`
		if timewise {
			src = `<score-timewise version="4.0">` + header + `<measure number="pickup-X"><part id="P1">` + first + `</part><part id="P2">` + second + `</part></measure></score-timewise>`
		}
		doc, err := Decode(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			var r validation.Report
			if model {
				r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
			} else {
				r, err = InspectXML(context.Background(), strings.NewReader(src), AssessmentOptions{Profile: ProfileSourceContext})
			}
			if err != nil {
				t.Fatal(err)
			}
			notes := performanceTiming(r, "note")
			if len(notes) != 2 {
				t.Fatalf("notes %d", len(notes))
			}
			a, b := notes[0], notes[1]
			if a.Part.Value == b.Part.Value || a.KeyFifths.Value.String() != "5" || b.KeyFifths.State != validation.EvidenceAbsent || a.Duration.Value.String() != "1" || b.Duration.Value.String() != "1/2" || a.EffectiveVelocity.Value.String() != "225/2" || b.EffectiveVelocity.Value.String() != "90" {
				t.Errorf("part isolation timewise%t model%t %s", timewise, model, localFindings(r))
			}
			if a.Staff.Value.String() != "18446744073709551615" || !a.StaffExists.Value {
				t.Error("sparse staff magnitude changed")
			}
			parts := performanceTiming(r, "part-order")
			if len(parts) != 2 || parts[0].PartOrder.Value.String() != "1" || parts[1].PartOrder.Value.String() != "2" {
				t.Error("part-list vertical order")
			}
		}
	}
}

func TestPerformanceSameSoundSpatialPlayAndPizzicato(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes><sound pizzicato="yes" pan="10" elevation="20"><midi-instrument id="I1"><pan>-90</pan></midi-instrument><midi-instrument id="I2"><elevation>90</elevation></midi-instrument><midi-instrument id="I1"><volume>100</volume></midi-instrument><play id="I1"><mute>on</mute></play></sound><note pizzicato="no"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><instrument id="I1"/><play id="I1"><other-play type="test">current</other-play></play></note><note><pitch><step>D</step><octave>4</octave></pitch><duration>1</duration><instrument id="I1"/></note><sound pizzicato="no"/><note><pitch><step>E</step><octave>4</octave></pitch><duration>1</duration><instrument id="I1"/></note>`
	for _, tw := range []bool{false, true} {
		src := contextFixtureXML(body, tw)
		src = strings.Replace(src, `<part-name>Piano</part-name>`, `<part-name>Piano</part-name><score-instrument id="I1"><instrument-name>One</instrument-name></score-instrument><score-instrument id="I2"><instrument-name>Two</instrument-name></score-instrument>`, 1)
		doc, err := Decode(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			var r validation.Report
			if model {
				r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
			} else {
				r, err = InspectXML(context.Background(), strings.NewReader(src), AssessmentOptions{Profile: ProfileSourceContext})
			}
			if err != nil {
				t.Fatal(err)
			}
			sounds := performanceTiming(r, "sound")
			notes := performanceTiming(r, "note")
			if len(sounds) != 2 || len(notes) != 3 {
				t.Fatal("missing sound or note")
			}
			a := sounds[0]
			if len(a.Spatial) != 3 || a.Spatial[0].Pan.Value.String() != "-90" || a.Spatial[0].Elevation.Value.String() != "20" || a.Spatial[1].Pan.Value.String() != "10" || a.Spatial[1].Elevation.Value.String() != "90" {
				t.Error("per-instrument spatial precedence")
			}
			if len(a.Play) != 1 || a.Play[0].Scope != validation.TimingPlayFollowingNotes || len(notes[0].Play) != 1 || notes[0].Play[0].Scope != validation.TimingPlayCurrentNote {
				t.Error("play scope")
			}
			if notes[0].EffectivePizzicato.State != validation.EvidenceKnown || notes[0].EffectivePizzicato.Value || !notes[1].EffectivePizzicato.Value || notes[2].EffectivePizzicato.Value {
				t.Error("note-local versus subsequent pizzicato")
			}
			if notes[0].EffectiveTempo.State != validation.EvidenceAbsent {
				t.Error("invented tempo")
			}
			found := false
			for _, a := range a.Advisories {
				if a.Clause == "sound-instrument-grouping" {
					found = a.Satisfied.State == validation.EvidenceKnown && !a.Satisfied.Value
				}
			}
			if !found {
				t.Error("noncontiguous instrument grouping guidance missing")
			}
		}
	}
}

func TestPerformanceInvalidLocalContextMatrix(t *testing.T) {
	cases := []struct {
		name, body, kind string
		field            func(*validation.TimingContextResult) validation.EvidenceState
	}{
		{"time", `<attributes><time separator="invalid"><beats>4</beats><beat-type>4</beat-type></time></attributes>`, "time", func(v *validation.TimingContextResult) validation.EvidenceState { return v.TimeSeparator.State }},
		{"staff", `<attributes><staff-details><staff-type>invalid</staff-type></staff-details></attributes>`, "staff-details", func(v *validation.TimingContextResult) validation.EvidenceState { return v.StaffType.State }},
		{"transpose", `<attributes><transpose><chromatic>NaN</chromatic></transpose></attributes>`, "transpose", func(v *validation.TimingContextResult) validation.EvidenceState { return v.ChromaticDisplacement.State }},
		{"key", `<attributes><key><fifths>NaN</fifths></key></attributes>`, "key", func(v *validation.TimingContextResult) validation.EvidenceState { return v.KeyFifths.State }},
		{"MIDI", `<sound><midi-instrument id="I1"><midi-unpitched>129</midi-unpitched></midi-instrument></sound>`, "midi-unpitched", func(v *validation.TimingContextResult) validation.EvidenceState { return v.MIDINote.State }},
		{"pan", `<sound><midi-instrument id="I1"><pan>181</pan></midi-instrument></sound>`, "pan", func(v *validation.TimingContextResult) validation.EvidenceState { return v.Pan.State }},
		{"elevation", `<sound><midi-instrument id="I1"><elevation>181</elevation></midi-instrument></sound>`, "elevation", func(v *validation.TimingContextResult) validation.EvidenceState { return v.Elevation.State }},
		{"fret", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><technical><fret>-1</fret></technical></notations></note>`, "fret", func(v *validation.TimingContextResult) validation.EvidenceState { return v.FretOpenString.State }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := contextFixtureXML(tc.body, false)
			r, err := InspectXML(context.Background(), strings.NewReader(src), AssessmentOptions{Profile: ProfileSourceContext})
			if err != nil {
				t.Fatal(err)
			}
			v := performanceTiming(r, validation.Code(tc.kind))
			if len(v) != 1 || tc.field(v[0]) == validation.EvidenceKnown || tc.field(v[0]) == validation.EvidenceAbsent {
				t.Errorf("invalid field became known/default/absent: %s", localFindings(r))
			}
			doc, e := Decode(strings.NewReader(src))
			if e != nil {
				t.Logf("Source-only invalid lexical case: decoder cannot construct this current-model numeric domain: %v", e)
				return
			}
			r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
			if err != nil {
				t.Fatal(err)
			}
			v = performanceTiming(r, validation.Code(tc.kind))
			if len(v) != 1 || tc.field(v[0]) == validation.EvidenceKnown || tc.field(v[0]) == validation.EvidenceAbsent {
				t.Errorf("invalid current model field became known/default/absent: %s", localFindings(r))
			}
		})
	}
}

func TestPerformanceMordentLongLocalDefault(t *testing.T) {
	for _, attr := range []string{"", ` long="yes"`, ` long="no"`, ` long="invalid"`} {
		performanceReports(t, `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><ornaments><mordent`+attr+`/></ornaments></notations></note>`, func(t *testing.T, r validation.Report) {
			found := false
			for _, c := range r.Context {
				if c.Property != "perf-mordent-long" {
					continue
				}
				found = true
				if attr == ` long="invalid"` {
					if c.Value.State != validation.EvidenceInvalid || c.Value.Cause == nil {
						t.Error("invalid long missing cause")
					}
				} else {
					want := "no"
					if attr == ` long="yes"` {
						want = "yes"
					}
					if c.Value.State != validation.EvidenceKnown || c.Value.Value.Text != want {
						t.Error("long default/explicit override")
					}
				}
			}
			if !found {
				t.Error("missing mordent long local fact")
			}
		})
	}
}
func TestPerformanceTablatureDoesNotWaiveOrdinaryDuration(t *testing.T) {
	for _, tc := range []struct {
		note    string
		missing bool
	}{{`<note><pitch><step>C</step><octave>4</octave></pitch></note>`, true}, {`<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`, false}, {`<note><grace/><pitch><step>C</step><octave>4</octave></pitch></note>`, false}} {
		performanceReports(t, `<attributes><divisions>1</divisions><clef><sign>TAB</sign></clef><staff-details><staff-lines>6</staff-lines></staff-details></attributes>`+tc.note, func(t *testing.T, r validation.Report) {
			failed := false
			for _, f := range r.Findings {
				if strings.HasPrefix(string(f.Rule.ID), "MX40-XSD-") && f.Outcome == validation.OutcomeFail {
					failed = true
				}
			}
			if failed != tc.missing {
				t.Errorf("tablature shared XSD duration: fail%t want%t %s", failed, tc.missing, localFindings(r))
			}
			for _, f := range r.Findings {
				if f.Rule.ID == "MX40-CONTEXT-performance" && f.Outcome == validation.OutcomeFail {
					t.Error("invented tablature notation/type obligation")
				}
			}
		})
	}
}

func TestPerformanceDefaultInvalidFieldMatrix(t *testing.T) {
	note := func(inside string) string {
		return `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations>` + inside + `</notations></note>`
	}
	cases := []struct {
		property validation.ContextProperty
		body     string
	}{
		{"perf-fingering-substitution", note(`<technical><fingering substitution="invalid">2</fingering></technical>`)},
		{"perf-empty-trill-sound-beats", note(`<ornaments><trill-mark beats="-1"/></ornaments>`)},
		{"perf-mordent-start-note", note(`<ornaments><mordent start-note="invalid"/></ornaments>`)},
		{"perf-level-reference", `<attributes><level reference="invalid">editorial</level><divisions>1</divisions></attributes>`},
		{"perf-stick-parentheses", `<direction><direction-type><percussion><stick parentheses="invalid"><stick-type>snare</stick-type><stick-material>wood</stick-material></stick></percussion></direction-type></direction>`},
		{"perf-root-alter-location", `<harmony><root><root-step>C</root-step><root-alter location="invalid">1</root-alter></root><kind>major</kind></harmony>`},
	}
	for _, tc := range cases {
		t.Run(string(tc.property), func(t *testing.T) {
			performanceReports(t, tc.body, func(t *testing.T, r validation.Report) {
				found := false
				for _, c := range r.Context {
					if c.Property == tc.property {
						found = true
						if c.Value.State != validation.EvidenceInvalid || c.Value.Cause == nil {
							t.Errorf("invalid prerequisite defaulted: %+v", c.Value)
						}
					}
				}
				if !found {
					t.Error("missing invalid field evidence")
				}
			})
		})
	}
}
func TestPerformanceSoundInvalidAndNoInitialState(t *testing.T) {
	performanceReports(t, `<attributes><divisions>1</divisions></attributes><sound tempo="-1" dynamics="-1"/><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><staff>0</staff></note>`, func(t *testing.T, r validation.Report) {
		sound := performanceTiming(r, "sound")
		if len(sound) != 1 || sound[0].Tempo.State != validation.EvidenceInvalid || sound[0].ReferenceVelocity.State != validation.EvidenceInvalid || sound[0].ReferenceVelocity.Cause == nil {
			t.Error("invalid sound facts")
		}
		note := performanceTiming(r, "note")
		if len(note) != 1 || note[0].Staff.State != validation.EvidenceInvalid || note[0].StaffExists.State == validation.EvidenceKnown {
			t.Error("invalid staff invented")
		}
	})
	performanceReports(t, `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`, func(t *testing.T, r validation.Report) {
		n := performanceTiming(r, "note")[0]
		if n.Staves.Value.String() != "1" || n.Instruments.Value.String() != "1" || n.KeyFifths.State != validation.EvidenceAbsent || n.TimeSymbol.State != validation.EvidenceAbsent || n.ClefSign.State != validation.EvidenceAbsent || n.Voice.State != validation.EvidenceAbsent || n.EffectiveTempo.State != validation.EvidenceAbsent {
			t.Error("invented unspecified initial state")
		}
	})
}

func TestPerformanceDirectionAssociationAndOriginalBendOrigin(t *testing.T) {
	body := `<attributes><divisions>4</divisions></attributes><direction><direction-type><words>Voice V</words></direction-type><voice>V</voice></direction><note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><voice>other</voice></note><note attack="2" release="-1"><pitch><step>D</step><octave>4</octave></pitch><duration>4</duration><voice>V</voice><notations><technical><bend><bend-alter>2</bend-alter></bend><bend first-beat="25" last-beat="75"><bend-alter>-2</bend-alter><release offset="2"/></bend></technical></notations></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		d := performanceTiming(r, "direction")
		n := performanceTiming(r, "note")
		if len(d) != 1 || len(n) != 2 {
			t.Fatal("missing direction/note")
		}
		if d[0].Anchor.State != validation.EvidenceKnown || d[0].Anchor.Value != n[1].Principal.Value || d[0].AnchorPosition.Value.Offset.String() != "1" {
			t.Error("voice-qualified first following note")
		}
		b := n[1].Bends
		if len(b) != 2 || b[1].OriginalNote.Value.Offset.String() != "1" || b[1].Release.Position.Value.Offset.String() != "3/2" || b[1].FirstBeat.Origin.Value.Offset.String() != "1" || b[1].FirstBeat.Position.Value.Offset.String() != "5/4" || b[1].LastBeat.Position.Value.Offset.String() != "7/4" {
			t.Error("bend percentages shifted with release origin")
		}
		if n[1].Attack.Position.Value.Offset.String() != "3/2" || n[1].Release.Position.Value.Offset.String() != "7/4" {
			t.Error("note attack/release independent origins")
		}
	})
}
func TestPerformanceConditionalFrettedRolesAndTrillCount(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><ornaments><trill-mark beats="3.5" two-note-turn="whole"/></ornaments><technical><fingering>2</fingering><pluck>p</pluck><fret>1</fret></technical></notations></note>`
	performanceReports(t, body, func(t *testing.T, r validation.Report) {
		roles := map[string]bool{}
		for _, c := range r.Context {
			if c.Property == "performance-fretted-instrument-role" && c.Value.State == validation.EvidenceKnown {
				roles[c.Value.Value.Text] = true
			}
		}
		if !roles["stopping-finger"] || !roles["plucking-finger"] {
			t.Error("conditional fretted roles")
		}
		trill := performanceTiming(r, "trill")
		if len(trill) != 1 || !trill[0].TrillIncludesInitialSound.Value || !trill[0].TrillExcludesClosingTurn.Value {
			t.Error("trill count excludes closing turn and includes first sound")
		}
		fret := performanceTiming(r, "fret")
		if len(fret) != 1 || fret[0].FretOpenString.State != validation.EvidenceKnown || fret[0].FretOpenString.Value {
			t.Error("first stopped fret is not open string")
		}
	})
}
