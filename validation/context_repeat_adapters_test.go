package validation_test

import (
	"context"
	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func repeatAssessBoth(t *testing.T, body string, check func(*testing.T, validation.Report)) {
	t.Helper()
	for _, timewise := range []bool{false, true} {
		header := `<part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`
		source := `<score-partwise version="4.0">` + header + `<part id="P1"><measure number="A">` + body + `</measure></part></score-partwise>`
		if timewise {
			source = `<score-timewise version="4.0">` + header + `<measure number="A"><part id="P1">` + body + `</part></measure></score-timewise>`
		}
		for _, model := range []bool{false, true} {
			var report validation.Report
			var err error
			if model {
				doc, e := musicxml.Decode(strings.NewReader(source))
				if e != nil {
					t.Fatal(e)
				}
				report, err = musicxml.AssessModel(context.Background(), doc, musicxml.AssessmentOptions{Profile: musicxml.ProfileModelContext})
			} else {
				report, err = musicxml.InspectXML(context.Background(), strings.NewReader(source), musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceContext})
			}
			if err != nil {
				t.Fatalf("timewise=%v model=%v: %v", timewise, model, err)
			}
			check(t, report)
		}
	}
}

func TestRepeatAdaptersDefaultsWithoutNavigation(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`, func(t *testing.T, r validation.Report) {
		found := false
		for _, c := range r.Context {
			if c.Repeat != nil && c.Repeat.Kind == validation.RepeatStyleBeat {
				found = true
				v := c.Repeat
				if v.Slashes.State != validation.EvidenceKnown || v.Slashes.Value.String() != "1" || v.UseDots.State != validation.EvidenceKnown || v.UseDots.Value || v.AllStaves.State != validation.EvidenceKnown || !v.AllStaves.Value || v.Beat.State != validation.EvidenceKnown || v.Beat.Value.String() != "1" {
					t.Fatalf("defaults %+v", v)
				}
			}
		}
		if !found {
			t.Fatalf("missing repeat facts; findings %+v", r.Findings)
		}
	})
}

func TestRepeatAdaptersExplicitTraversal(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note><barline location="right"><repeat direction="backward" times="3"/></barline>`, func(t *testing.T, r validation.Report) {
		found := false
		for _, c := range r.Context {
			if c.Navigation != nil && c.Navigation.Kind == validation.NavigationTraversal {
				found = true
				v := c.Navigation
				if v.Complete.State != validation.EvidenceKnown || !v.Complete.Value {
					t.Fatalf("traversal incomplete %+v", v)
				}
				count := 0
				for _, o := range v.Traversal {
					if len(o.Frames) == 1 && o.Played.State == validation.EvidenceKnown {
						count++
					}
				}
				if count != 9 {
					t.Fatalf("occurrences=%d: %+v", count, v.Traversal)
				}
			}
		}
		if !found {
			t.Fatalf("missing traversal: %+v", r.Findings)
		}
	})
}

func TestRepeatAdaptersUnknownCountAndForwardApplicability(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward" times="3" after-jump="yes"/></barline><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note><barline location="right"><repeat direction="backward"/></barline>`, func(t *testing.T, r validation.Report) {
		forward, prefix := false, false
		for _, c := range r.Context {
			if c.Navigation == nil {
				continue
			}
			v := c.Navigation
			if v.Kind == validation.NavigationForward {
				forward = true
				if v.AttributesActive.State != validation.EvidenceKnown || v.AttributesActive.Value || v.Times.Value.String() != "3" {
					t.Fatalf("forward raw/applicable %+v", v)
				}
			}
			if v.Kind == validation.NavigationTraversal {
				prefix = true
				if v.Complete.State != validation.EvidenceUnknown || len(v.Traversal) != 3 {
					t.Fatalf("prefix %+v", v)
				}
			}
		}
		if !forward || !prefix {
			t.Fatalf("missing facts %+v", r.Context)
		}
	})
}

func TestRepeatAdaptersMeasureContentAndStopBoundary(t *testing.T) {
	note := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><voice>1</voice><type>half</type></note>`
	source := `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><part id="P1"><measure number="one"><attributes><divisions>1</divisions></attributes>` + note + `</measure><measure number="repeat"><attributes><measure-style><measure-repeat type="start">1</measure-repeat></measure-style></attributes>` + note + `</measure><measure number="ordinary"><attributes><measure-style><measure-repeat type="stop">99</measure-repeat></measure-style></attributes><note><rest/><duration>2</duration></note></measure></part></score-partwise>`
	for _, model := range []bool{false, true} {
		var report validation.Report
		var err error
		if model {
			doc, e := musicxml.Decode(strings.NewReader(source))
			if e != nil {
				t.Fatal(e)
			}
			report, err = musicxml.AssessModel(context.Background(), doc, musicxml.AssessmentOptions{Profile: musicxml.ProfileModelContext})
		} else {
			report, err = musicxml.InspectXML(context.Background(), strings.NewReader(source), musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceContext})
		}
		if err != nil {
			t.Fatal(err)
		}
		start, stop := false, false
		for _, c := range report.Context {
			if c.Repeat == nil || c.Repeat.Kind != validation.RepeatStyleMeasure {
				continue
			}
			v := c.Repeat
			if v.Type.Value == "start" {
				start = true
				if v.Complete.State != validation.EvidenceKnown || v.Stop.State != validation.EvidenceKnown || v.ContentEquivalent.State != validation.EvidenceKnown || !v.ContentEquivalent.Value {
					t.Fatalf("model=%v content=%+v", model, v)
				}
			} else {
				stop = true
				if v.PatternMeasures.State != validation.EvidenceAbsent || v.Active.Value {
					t.Fatalf("stop %+v", v)
				}
			}
		}
		if !start || !stop {
			t.Fatalf("missing repeat rows %+v", report.Context)
		}
	}
}

func TestRepeatAdaptersListeningOffsetAndPassState(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline><listening><sync type="none" time-only="2"/><offset>0</offset></listening><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note><barline location="right"><repeat direction="backward" times="2"/></barline>`, func(t *testing.T, r validation.Report) {
		found := false
		for _, c := range r.Context {
			if c.Navigation != nil && c.Navigation.Kind == validation.NavigationTraversal {
				found = true
				v := c.Navigation
				if v.Complete.State != validation.EvidenceKnown {
					t.Fatalf("listening traversal %+v", v)
				}
				seen := false
				for _, o := range v.Traversal {
					if o.Pass.State == validation.EvidenceKnown && o.Pass.Value.String() == "1" && len(o.Listening) > 0 {
						t.Fatalf("first pass inherited inactive listening %+v", o)
					}
					if o.Pass.State == validation.EvidenceKnown && o.Pass.Value.String() == "2" && len(o.Listening) > 0 {
						seen = true
					}
				}
				if !seen {
					t.Fatalf("missing active pass listening %+v", v)
				}
			}
		}
		if !found {
			t.Fatal("missing traversal")
		}
	})
}

func TestRepeatAdaptersBeatContentExactAndMismatch(t *testing.T) {
	for _, pitch := range []string{"C", "D"} {
		body := `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note><attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes><note><pitch><step>` + pitch + `</step><octave>4</octave></pitch><duration>1</duration></note><attributes><measure-style><beat-repeat type="stop"/></measure-style></attributes>`
		repeatAssessBoth(t, body, func(t *testing.T, r validation.Report) {
			found := false
			for _, c := range r.Context {
				if c.Repeat == nil || c.Repeat.Kind != validation.RepeatStyleBeat || c.Repeat.Type.Value != "start" {
					continue
				}
				found = true
				v := c.Repeat.ContentEquivalent
				if pitch == "C" && (v.State != validation.EvidenceKnown || !v.Value) {
					t.Fatalf("equal beat %+v", c.Repeat)
				}
				if pitch == "D" && v.State != validation.EvidenceUnknown {
					t.Fatalf("unreviewed mismatch rejection %+v", v)
				}
			}
			if !found {
				t.Fatal("missing beat proof")
			}
		})
	}
}

func TestRepeatAdaptersInstrumentPassUpdates(t *testing.T) {
	source := `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name><score-instrument id="I"><instrument-name>Piano</instrument-name><instrument-sound>keyboard.piano</instrument-sound></score-instrument><midi-instrument id="I"><midi-channel>1</midi-channel><midi-program>1</midi-program></midi-instrument></score-part></part-list><part id="P1"><measure number="1"><attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline><sound time-only="2"><instrument-change id="I"><instrument-sound>keyboard.organ</instrument-sound><virtual-instrument><virtual-library>new</virtual-library></virtual-instrument></instrument-change><midi-instrument id="I"><volume>40</volume></midi-instrument></sound><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><instrument id="I"/></note><barline location="right"><repeat direction="backward" times="2"/></barline></measure></part></score-partwise>`
	for _, model := range []bool{false, true} {
		var report validation.Report
		var err error
		if model {
			doc, e := musicxml.Decode(strings.NewReader(source))
			if e != nil {
				t.Fatal(e)
			}
			report, err = musicxml.AssessModel(context.Background(), doc, musicxml.AssessmentOptions{Profile: musicxml.ProfileModelContext})
		} else {
			report, err = musicxml.InspectXML(context.Background(), strings.NewReader(source), musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceContext})
		}
		if err != nil {
			t.Fatal(err)
		}
		initial, updated := false, false
		for _, c := range report.Context {
			if c.Navigation == nil || c.Navigation.Kind != validation.NavigationTraversal {
				continue
			}
			v := c.Navigation
			if v.Complete.State != validation.EvidenceKnown {
				t.Fatalf("incomplete %+v", v)
			}
			for _, o := range v.Traversal {
				for _, i := range o.Instruments {
					if i.Sound.Value.State == validation.EvidenceKnown && i.Sound.Value.Value == "keyboard.piano" {
						initial = true
					}
					if i.Sound.Value.State == validation.EvidenceKnown && i.Sound.Value.Value == "keyboard.organ" {
						updated = true
						if i.Program.Value.State != validation.EvidenceKnown || i.Program.Value.Value.String() != "1" || i.Volume.Value.State != validation.EvidenceKnown || i.Volume.Value.Value.String() != "40" || i.Virtual.Value.State != validation.EvidenceKnown || i.Virtual.Value.Value.Name.State != validation.EvidenceAbsent {
							t.Fatalf("sparse/atomic model=%v %+v", model, i)
						}
					}
				}
			}
		}
		if !initial || !updated {
			t.Fatalf("model=%v missing initial=%v update=%v findings=%+v", model, initial, updated, report.Findings)
		}
	}
}
