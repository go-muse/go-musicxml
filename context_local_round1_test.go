package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestLocalRound1RootAlterAndFiguredBassAllAdapters(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<harmony><root><root-step>C</root-step><root-alter>1</root-alter></root><kind>major</kind></harmony><figured-bass print-object="no"><figure><figure-number>6</figure-number></figure></figured-bass>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, localContextFixture(body, tw), model)
			want := map[validation.ContextProperty]string{"local.root-alter-location": "right", validation.ContextPrintDot: "no", validation.ContextPrintLyric: "no", validation.ContextPrintSpacing: "yes"}
			for _, row := range report.Context {
				if expected, ok := want[row.Property]; ok {
					if row.Value.State != validation.EvidenceKnown || row.Value.Value.Text != expected {
						t.Errorf("%s %+v", row.Property, row.Value)
					}
					delete(want, row.Property)
				}
			}
			if len(want) > 0 {
				t.Errorf("timewise=%t model=%t missing %v", tw, model, want)
			}
		}
	}
}

func TestLocalRound1ProvenanceAndDashesAllAdapters(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<harmony><root><root-step>C</root-step></root><kind>major</kind><degree><degree-value symbol="major" text="">7</degree-value><degree-alter>0</degree-alter><degree-type>add</degree-type></degree></harmony><direction><direction-type><dashes type="start" dash-length="7.5" space-length="7.5"/></direction-type></direction><note><rest/><duration>1</duration><notations><technical><arrow><circular-arrow>clockwise</circular-arrow></arrow></technical></notations></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := localContextAssess(t, engine, localContextFixture(body, tw), model)
			seen := map[validation.ContextProperty]bool{}
			for _, row := range report.Context {
				switch row.Property {
				case validation.ContextDegreeTextComposition, validation.ContextArrowDirection, validation.ContextArrowStyle:
					seen[row.Property] = true
					if row.Layout.Provenance != validation.ContextExplicit {
						t.Errorf("%s provenance %+v", row.Property, row.Layout)
					}
				case validation.ContextDashLength, validation.ContextSpaceLength:
					seen[row.Property] = true
					if row.Layout.Applicability.State != validation.EvidenceKnown || !row.Layout.Applicability.Value || row.Layout.Provenance != validation.ContextExplicit {
						t.Errorf("dashes semantics %+v", row)
					}
				}
			}
			if len(seen) != 5 {
				t.Errorf("timewise=%t model=%t rows %v", tw, model, seen)
			}
		}
	}
}

func TestRound1LayoutProjectionUnusedAxesAreExplicit(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			source := independentContextScore("", [][]string{{`<attributes><divisions>1</divisions><clef><sign>G</sign><line>2</line></clef></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type><notations><slur type="start"/></notations></note>`}}, tw)
			report := independentContextReport(t, source, model)
			count := 0
			for _, row := range report.Context {
				if row.Layout == nil {
					continue
				}
				count++
				p := row.Layout
				if p.Part.State == "" || p.Page.State == "" || p.System.State == "" || p.Staff.State == "" || p.Conversion.State == "" {
					t.Errorf("timewise=%t model=%t %s has zero-state metadata: %+v", tw, model, row.Property, p)
				}
			}
			if count == 0 {
				t.Fatal("no layout projections")
			}
		}
	}
}

func TestRound1NonDurationTimingAndGraceAxesAreExplicit(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes><note><grace make-time="1"/><pitch><step>C</step><octave>4</octave></pitch><type>eighth</type></note><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type></note><direction><direction-type><words>Fine</words></direction-type><sound tempo="120"/></direction><harmony><root><root-step>C</root-step></root><kind>major</kind></harmony>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
			events, grace := 0, 0
			for _, row := range report.Context {
				if row.Timing == nil {
					continue
				}
				p := row.Timing
				if p.Kind == "event" || p.Kind == "direction" || p.Kind == "sound" {
					events++
					if p.End.State != validation.EvidenceAbsent {
						t.Errorf("non-duration %s End %+v", p.Kind, p.End)
					}
				}
				if p.Kind == "note" && p.MakeTime.Raw.State == validation.EvidenceKnown {
					grace++
					if p.MakeTime.Origin.State == "" || p.MakeTime.Position.State == "" {
						t.Errorf("uninitialized grace coordinate %+v", p.MakeTime)
					}
				}
			}
			if events == 0 || grace != 1 {
				t.Errorf("events%d grace%d", events, grace)
			}
		}
	}
}

func TestRound1ChordEndRemainsUnestablished(t *testing.T) {
	body := `<attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note><note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>1</duration></note>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
			count := 0
			for _, row := range report.Context {
				if row.Timing != nil && row.Timing.Kind == "note" && row.Timing.Chord.State == validation.EvidenceKnown && row.Timing.Chord.Value {
					count++
					if row.Timing.End.State != validation.EvidenceUnknown {
						t.Errorf("chord end %+v", row.Timing.End)
					}
				}
			}
			if count != 1 {
				t.Fatalf("chord rows %d", count)
			}
		}
	}
}
func TestRound1LayoutDefaultProvenanceAtSemanticSites(t *testing.T) {
	body := `<print><staff-layout><staff-distance>22</staff-distance></staff-layout></print><attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><lyric><text>la</text></lyric></note><harmony><root><root-step>C</root-step></root><kind>major</kind></harmony>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
			want := map[validation.ContextProperty]validation.ContextProvenance{"layout.staff-number": validation.ContextDocumentedDefault, "layout.lyric-placement": validation.ContextDocumentedDefault, "layout.arrangement": validation.ContextApplicationChoice}
			for _, row := range report.Context {
				if expected, ok := want[row.Property]; ok {
					if row.Layout == nil || row.Layout.Provenance != expected {
						t.Errorf("%s %+v", row.Property, row.Layout)
					}
					delete(want, row.Property)
				}
			}
			if len(want) != 0 {
				t.Errorf("missing %v", want)
			}
		}
	}
}
