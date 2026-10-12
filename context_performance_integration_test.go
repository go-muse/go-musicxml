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
