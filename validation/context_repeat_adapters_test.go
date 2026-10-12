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
	repeatAssessHeader(t, body, `<part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`, check)
}

func repeatAssessHeader(t *testing.T, body, header string, check func(*testing.T, validation.Report)) {
	t.Helper()
	for _, timewise := range []bool{false, true} {
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

func TestRepeatAdaptersGraphicPlaybackAdvice(t *testing.T) {
	for _, symbol := range []string{"segno", "coda"} {
		for _, sound := range []bool{false, true} {
			suffix := ""
			if sound {
				suffix = `<sound ` + symbol + `="A"/>`
			}
			repeatAssessBoth(t, `<direction><direction-type><`+symbol+`/></direction-type>`+suffix+`</direction>`, func(t *testing.T, r validation.Report) {
				found := false
				for _, c := range r.Context {
					if c.Navigation == nil {
						continue
					}
					v := c.Navigation
					if v.Kind != validation.NavigationGraphicSegno && v.Kind != validation.NavigationGraphicCoda {
						continue
					}
					found = true
					if v.PlaybackSoundNeeded.State != validation.EvidenceKnown || !v.PlaybackSoundNeeded.Value {
						t.Fatalf("missing playback advice: %+v", v)
					}
					want := validation.EvidenceAbsent
					if sound {
						want = validation.EvidenceKnown
					}
					if v.LocalPlaybackSound.State != want {
						t.Fatalf("sound=%v association %+v", sound, v)
					}
				}
				if !found {
					t.Fatal("missing graphic advisory")
				}
			})
		}
	}
}

func TestRepeatAdaptersEmptyEndingAndFine(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes><barline location="left"><ending number="" type="start">ad lib.</ending></barline><note><rest/><duration>1</duration></note><sound fine="yes"/>`, func(t *testing.T, r validation.Report) {
		ending, fine := false, false
		for _, c := range r.Context {
			if c.Navigation == nil {
				continue
			}
			v := c.Navigation
			if v.Kind == validation.NavigationEnding {
				ending = true
				if v.EndingNumber.State != validation.EvidenceKnown || v.EndingNumber.Value != "" || v.Guard.State != validation.EvidenceUnknown {
					t.Fatalf("empty ending %+v", v)
				}
			}
			if v.Kind == validation.NavigationFine {
				fine = true
				if v.DurationUnchanged.State != validation.EvidenceKnown || !v.DurationUnchanged.Value {
					t.Fatalf("fine yes %+v", v)
				}
			}
		}
		if !ending || !fine {
			t.Fatalf("ending=%v fine=%v", ending, fine)
		}
	})
}

func TestRepeatAdaptersListeningDistinctAndAssessment(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		second := ` time-only="2"`
		if duplicate {
			second = ""
		}
		repeatAssessBoth(t, `<listening><sync type="none"/><sync type="bar"`+second+`/></listening><note><rest/><duration>1</duration><listen><assess type="no"/><assess type="yes"`+second+`/></listen></note>`, func(t *testing.T, r validation.Report) {
			listening, note := false, false
			for _, c := range r.Context {
				if c.Navigation == nil {
					continue
				}
				v := c.Navigation
				if v.Kind == validation.NavigationListening || v.Kind == validation.NavigationNote {
					if v.TargetsDistinct.State != validation.EvidenceKnown || v.TargetsDistinct.Value == duplicate {
						t.Fatalf("duplicate=%v advice %+v", duplicate, v)
					}
					if v.Kind == validation.NavigationListening {
						listening = true
					} else {
						note = true
						if v.RecommendedAssess.State != validation.EvidenceKnown || !v.RecommendedAssess.Value || len(v.NoteListening) != 2 || v.NoteListening[0].Override.State != validation.EvidenceKnown || v.NoteListening[0].Override.Value {
							t.Fatalf("assessment %+v", v)
						}
					}
				}
			}
			if !listening || !note {
				t.Fatalf("missing advice listening=%v note=%v", listening, note)
			}
		})
	}
	repeatAssessBoth(t, `<note><cue/><rest/><duration>1</duration></note>`, func(t *testing.T, r validation.Report) {
		found := false
		for _, c := range r.Context {
			if c.Navigation != nil && c.Navigation.Kind == validation.NavigationNote {
				found = true
				if c.Navigation.RecommendedAssess.State != validation.EvidenceKnown || c.Navigation.RecommendedAssess.Value {
					t.Fatalf("cue advice %+v", c.Navigation)
				}
			}
		}
		if !found {
			t.Fatal("missing cue")
		}
	})
}

func TestRepeatAdaptersPlayerReferenceScope(t *testing.T) {
	for _, defined := range []bool{false, true} {
		player := ""
		if defined {
			player = `<player id="R"><player-name>Reader</player-name></player>`
		}
		header := `<part-list><score-part id="P1"><part-name>Piano</part-name>` + player + `</score-part></part-list>`
		repeatAssessHeader(t, `<listening><sync type="none" player="R"/></listening><note><rest/><duration>1</duration><listen><assess type="no" player="R"/></listen></note>`, header, func(t *testing.T, r validation.Report) {
			found := 0
			for _, c := range r.Context {
				if c.Navigation == nil {
					continue
				}
				v := c.Navigation
				if v.Kind == validation.NavigationListening {
					for _, x := range v.Listening {
						found++
						if (x.PlayerTarget.State == validation.EvidenceKnown) != defined {
							t.Fatalf("defined=%v player %+v", defined, x)
						}
					}
				}
				if v.Kind == validation.NavigationNote {
					for _, x := range v.NoteListening {
						found++
						if (x.PlayerTarget.State == validation.EvidenceKnown) != defined || x.AllPlayers.State != validation.EvidenceKnown || x.AllPlayers.Value || x.AllPasses.State != validation.EvidenceKnown || !x.AllPasses.Value {
							t.Fatalf("defined=%v scope %+v", defined, x)
						}
					}
				}
			}
			if found != 2 {
				t.Fatalf("found=%d", found)
			}
		})
	}
}

func TestRepeatAdaptersEmptyTargetRequiresSoundCoverage(t *testing.T) {
	for _, proved := range []bool{false, true} {
		boundary := ""
		if proved {
			boundary = `<direction><direction-type><words>endpoint witness</words></direction-type></direction>`
		}
		for _, timewise := range []bool{false, true} {
			measures := []string{`<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>` + boundary, `<attributes><measure-style><measure-repeat type="start">1</measure-repeat></measure-style></attributes>`, `<attributes><measure-style><measure-repeat type="stop">1</measure-repeat></measure-style></attributes>`}
			source := `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><part id="P1">`
			if timewise {
				source = `<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`
			}
			for _, body := range measures {
				if timewise {
					source += `<measure number="same"><part id="P1">` + body + `</part></measure>`
				} else {
					source += `<measure number="same">` + body + `</measure>`
				}
			}
			if timewise {
				source += `</score-timewise>`
			} else {
				source += `</part></score-partwise>`
			}
			for _, model := range []bool{false, true} {
				var r validation.Report
				var err error
				if model {
					doc, e := musicxml.Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					r, err = musicxml.AssessModel(context.Background(), doc, musicxml.AssessmentOptions{Profile: musicxml.ProfileModelContext})
				} else {
					r, err = musicxml.InspectXML(context.Background(), strings.NewReader(source), musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, c := range r.Context {
					if c.Repeat == nil || c.Repeat.Kind != validation.RepeatStyleMeasure || c.Repeat.Type.Value != "start" {
						continue
					}
					found = true
					v := c.Repeat.ContentEquivalent
					if proved && (v.State != validation.EvidenceKnown || v.Value) {
						t.Fatalf("proved=%v timewise=%v model=%v: %+v", proved, timewise, model, v)
					}
					if !proved && v.State == validation.EvidenceKnown {
						t.Fatalf("terminal end alone proved empty content: %+v", v)
					}
				}
				if !found {
					t.Fatal("missing measure repeat")
				}
			}
		}
	}
}

func TestRepeatAdaptersSlashVoiceAndStopScope(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions><measure-style><slash type="start"><slash-type>quarter</slash-type><except-voice>1</except-voice></slash></measure-style></attributes><note><rest/><duration>1</duration><voice>1</voice></note><note><rest/><duration>1</duration><voice>2</voice></note><attributes><measure-style><slash type="stop"/></measure-style></attributes><note><rest/><duration>1</duration><voice>2</voice></note>`, func(t *testing.T, r validation.Report) {
		found := false
		for _, c := range r.Context {
			if c.Repeat == nil || c.Repeat.Kind != validation.RepeatStyleSlash || c.Repeat.Type.Value != "start" {
				continue
			}
			found = true
			rows := c.Repeat.Notes
			if len(rows) != 3 {
				t.Fatalf("scope rows %+v", rows)
			}
			for i, want := range []bool{false, true, false} {
				if rows[i].Applies.State != validation.EvidenceKnown || rows[i].Applies.Value != want {
					t.Fatalf("note %d scope %+v", i, rows[i])
				}
			}
		}
		if !found {
			t.Fatal("missing slash range")
		}
	})
}
