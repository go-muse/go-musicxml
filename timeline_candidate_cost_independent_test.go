package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestIndependentTimelinePracticalDirectionCandidates(t *testing.T) {
	var body strings.Builder
	body.WriteString(`<attributes><divisions>1</divisions></attributes>`)
	for i := 0; i < 1000; i++ {
		body.WriteString(`<direction><direction-type><words>dolce</words></direction-type></direction>`)
		body.WriteString(timelineNote("1", "", ""))
	}
	source := (localFixture{body: body.String()}).xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReadOnlyModelFixture(t, source, doc)
	for _, liftedBuffer := range []bool{false, true} {
		for _, p := range []validation.ProfileID{ProfileSourceLocal, ProfileSourceTimeline, ProfileModelLocal, ProfileModelTimeline} {
			label := string(p)
			budget := validation.Budgets{}
			if liftedBuffer {
				label += "/diagnostic-buffer-only"
				budget.MaxBufferedBytes = 512 << 20
			}
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				var report validation.Report
				var err error
				if p == ProfileSourceLocal || p == ProfileSourceTimeline {
					report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p, Budgets: budget})
				} else {
					report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: p, Budgets: budget})
				}
				t.Logf("1000 notes+1000 directions bytes=%d complete=%t satisfied=%t error=%v", len(source), report.AssessmentComplete, report.Satisfied(), err)
				if err != nil || !report.Satisfied() {
					t.Fatalf("ordinary direction candidates exhausted default assessment: %v; %s", err, localFindings(report))
				}
			})
		}
	}

}

func TestIndependentTimelinePracticalFiguredBassCandidates(t *testing.T) {
	var body strings.Builder
	body.WriteString(`<attributes><divisions>1</divisions></attributes>`)
	for i := 0; i < 1000; i++ {
		body.WriteString(`<figured-bass><figure><figure-number>6</figure-number></figure></figured-bass>`)
		body.WriteString(timelineNote("1", "", ""))
	}
	source := (localFixture{body: body.String()}).xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	assertReadOnlyModelFixture(t, source, doc)
	for _, liftedBuffer := range []bool{false, true} {
		for _, p := range []validation.ProfileID{ProfileSourceLocal, ProfileSourceTimeline, ProfileModelLocal, ProfileModelTimeline} {
			label := string(p)
			budget := validation.Budgets{}
			if liftedBuffer {
				label += "/diagnostic-buffer-only"
				budget.MaxBufferedBytes = 512 << 20
			}
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				var report validation.Report
				var err error
				if p == ProfileSourceLocal || p == ProfileSourceTimeline {
					report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p, Budgets: budget})
				} else {
					report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: p, Budgets: budget})
				}
				t.Logf("1000 notes+1000 figured-bass records bytes=%d complete=%t satisfied=%t error=%v", len(source), report.AssessmentComplete, report.Satisfied(), err)
				if err != nil || !report.Satisfied() {
					t.Fatalf("ordinary figured-bass candidates exhausted default assessment: %v; %s", err, localFindings(report))
				}
			})
		}
	}

}

func TestIndependentTimelinePracticalGraceCandidates(t *testing.T) {
	grace := `<note><grace steal-time-following="10"/><pitch><step>C</step><octave>4</octave></pitch><type>eighth</type></note>`
	for _, grouped := range []bool{false, true} {
		name := "1000-notes-100-spaced-grace"
		var body strings.Builder
		body.WriteString(`<attributes><divisions>1</divisions></attributes>`)
		if grouped {
			name = "1000-grace-group-two-donors"
			body.WriteString(timelineNote("1", "", ""))
			for i := 0; i < 1000; i++ {
				body.WriteString(grace)
			}
			body.WriteString(timelineNote("1", "", ""))
		} else {
			for i := 0; i < 1000; i++ {
				if i%10 == 5 {
					body.WriteString(grace)
				} else {
					body.WriteString(timelineNote("1", "", ""))
				}
			}
		}
		source := (localFixture{body: body.String()}).xml()
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			t.Fatal(err)
		}
		assertReadOnlyModelFixture(t, source, doc)
		for _, liftedBuffer := range []bool{false, true} {
			for _, p := range []validation.ProfileID{ProfileSourceLocal, ProfileSourceTimeline, ProfileModelLocal, ProfileModelTimeline} {
				label := name + "/" + string(p)
				budget := validation.Budgets{}
				if liftedBuffer {
					label += "/diagnostic-buffer-only"
					budget.MaxBufferedBytes = 512 << 20
				}
				t.Run(label, func(t *testing.T) {
					t.Parallel()
					var report validation.Report
					var err error
					if p == ProfileSourceLocal || p == ProfileSourceTimeline {
						report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p, Budgets: budget})
					} else {
						report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: p, Budgets: budget})
					}
					t.Logf("grace trajectory bytes=%d complete=%t satisfied=%t error=%v", len(source), report.AssessmentComplete, report.Satisfied(), err)
					if err != nil || !report.Satisfied() {
						t.Fatalf("grace trajectory failed: %v; %s", err, localFindings(report))
					}
				})
			}
		}
	}
}
