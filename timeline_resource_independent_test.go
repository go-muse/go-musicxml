package musicxml

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func independentTimelineScore(notes int, manyAttributes bool, duration string) string {
	var body strings.Builder
	body.WriteString(`<attributes><divisions>3</divisions><key><fifths>0</fifths></key></attributes>`)
	for i := 0; i < notes; i++ {
		if manyAttributes {
			body.WriteString(`<attributes><key><fifths>0</fifths></key><time><beats>4</beats><beat-type>4</beat-type></time><clef><sign>G</sign><line>2</line></clef></attributes>`)
		}
		body.WriteString(timelineNote(duration, "", ""))
	}
	return (localFixture{body: body.String()}).xml()
}

func TestIndependentTimelinePracticalDefaults(t *testing.T) {
	t.Parallel()
	for _, count := range []int{100, 1000} {
		for _, attr := range []bool{false, true} {
			if count == 1000 && attr {
				continue
			}
			for _, duration := range []string{"1", "0.1"} {
				t.Run(fmt.Sprintf("notes%d/attributes%t/duration%s", count, attr, duration), func(t *testing.T) {
					source := independentTimelineScore(count, attr, duration)
					document, err := Decode(strings.NewReader(source))
					if err != nil {
						t.Fatal(err)
					}
					for _, model := range []bool{false, true} {
						var report validation.Report
						if model {
							report, err = AssessModel(context.Background(), document, AssessmentOptions{Profile: ProfileModelTimeline})
						} else {
							report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceTimeline})
						}
						t.Logf("model=%t bytes=%d complete=%t satisfied=%t err=%v", model, len(source), report.AssessmentComplete, report.Satisfied(), err)
						if err != nil || !report.Satisfied() {
							t.Errorf("ordinary default fixture model=%t: %v; %s", model, err, localFindings(report))
						}
					}
				})
			}
		}
	}
}

func TestIndependentTimelineParallelProfilesAndOwnedReports(t *testing.T) {
	source := (localFixture{body: timelineNumeral("", "")}).xml()
	document, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	// Warm the lazy immutable engines before the race stress.
	for _, p := range []validation.ProfileID{ProfileSourceLocal, ProfileSourceTimeline} {
		if _, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 4; j++ {
				var got validation.Report
				var err error
				timeline := i%2 == 0
				if i%3 == 0 {
					p := ProfileModelLocal
					if timeline {
						p = ProfileModelTimeline
					}
					got, err = AssessModel(context.Background(), document, AssessmentOptions{Profile: p})
				} else {
					p := ProfileSourceLocal
					if timeline {
						p = ProfileSourceTimeline
					}
					got, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p})
				}
				if err != nil || got.Satisfied() == timeline {
					t.Errorf("profile leakage timeline=%t satisfied=%t err=%v", timeline, got.Satisfied(), err)
					return
				}
				for k := range got.Findings {
					got.Findings[k].Reason = "caller-owned-mutation"
					if got.Findings[k].Cause != nil {
						got.Findings[k].Cause.Rule.ID = "caller-owned-mutation"
					}
				}
			}
		}(i)
	}
	wg.Wait()
	again, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceTimeline})
	if err != nil || again.Satisfied() {
		t.Fatalf("fresh timeline report=%+v err=%v", again, err)
	}
	for _, f := range again.Findings {
		if f.Reason == "caller-owned-mutation" || f.Cause != nil && f.Cause.Rule.ID == "caller-owned-mutation" {
			t.Fatal("report mutation entered immutable profiles")
		}
	}
}

func TestIndependentTimelineDiagnosticCapKeepsProvedFailure(t *testing.T) {
	source := (localFixture{body: timelineNumeral("", "")}).xml()
	for _, budgets := range []validation.Budgets{{MaxDiagnostics: 1}, {MaxDiagnosticBytes: 1}} {
		r, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceTimeline, Budgets: budgets})
		if r.Conformance != validation.ConformanceViolated || r.Satisfied() {
			t.Fatalf("proved failure disappeared: conformance=%s err=%v findings=%s", r.Conformance, err, localFindings(r))
		}
		if err != nil && !errors.Is(err, validation.ErrIncompleteAssessment) {
			t.Fatalf("diagnostic stop misclassified: %v", err)
		}
	}
}

// Matched warmed incremental profiles on the same head; this does not remeasure
// or close the older B5-PERF-01 shared-kernel regression.
func BenchmarkIndependentTimelineProfiles(b *testing.B) {
	for _, fixture := range []struct {
		name  string
		count int
		attrs bool
	}{{"notes100", 100, false}, {"notes1000", 1000, false}, {"attributes100", 100, true}} {
		source := independentTimelineScore(fixture.count, fixture.attrs, "1")
		document, err := Decode(strings.NewReader(source))
		if err != nil {
			b.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			for _, timeline := range []bool{false, true} {
				b.Run(fmt.Sprintf("%s/model%t/timeline%t", fixture.name, model, timeline), func(b *testing.B) {
					p := ProfileSourceLocal
					if model {
						p = ProfileModelLocal
					}
					if timeline {
						p = ProfileSourceTimeline
						if model {
							p = ProfileModelTimeline
						}
					}
					opts := AssessmentOptions{Profile: p, Budgets: validation.Budgets{MaxBufferedBytes: 512 << 20, MaxRuleSteps: 1 << 34}}
					run := func() {
						var r validation.Report
						var err error
						if model {
							r, err = AssessModel(context.Background(), document, opts)
						} else {
							r, err = InspectXML(context.Background(), strings.NewReader(source), opts)
						}
						if err != nil || !r.Satisfied() {
							b.Fatalf("benchmark fixture: %v; %s", err, localFindings(r))
						}
					}
					run()
					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						run()
					}
				})
			}
		}
	}
}

func TestIndependentTimelineRepeatedDivisionsCosts(t *testing.T) {
	t.Parallel()
	for _, count := range []int{100, 1000} {
		for _, timeline := range []bool{false, true} {
			t.Run(fmt.Sprintf("notes%d/timeline%t", count, timeline), func(t *testing.T) {
				body := strings.Repeat(`<attributes><divisions>3</divisions></attributes>`+timelineNote("1", "", ""), count)
				source := (localFixture{body: body}).xml()
				p := ProfileSourceLocal
				if timeline {
					p = ProfileSourceTimeline
				}
				r, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p})
				t.Logf("complete=%t conformance=%s bytes=%d err=%v", r.AssessmentComplete, r.Conformance, len(source), err)
				if err != nil || !r.Satisfied() {
					t.Errorf("repeated ordinary divisions failed: %v", err)
				}
			})
		}
	}
}

// The long composite-attribute case is an observation of the documented finite
// limits, not a throughput promise. Both the prior and timeline profile must
// preserve incomplete/undetermined classification if a safety quota stops work.
func TestIndependentTimelineManyAttributesDefaultClassification(t *testing.T) {
	t.Parallel()
	source := independentTimelineScore(1000, true, "1")
	document, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []validation.ProfileID{ProfileSourceLocal, ProfileSourceTimeline, ProfileModelLocal, ProfileModelTimeline} {
		var r validation.Report
		if p == ProfileModelLocal || p == ProfileModelTimeline {
			r, err = AssessModel(context.Background(), document, AssessmentOptions{Profile: p})
		} else {
			r, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p})
		}
		t.Logf("profile=%s complete=%t satisfied=%t err=%v", p, r.AssessmentComplete, r.Satisfied(), err)
		if err != nil {
			if !errors.Is(err, validation.ErrResourceLimit) || !errors.Is(err, validation.ErrIncompleteAssessment) || r.Conformance == validation.ConformanceViolated || r.AssessmentComplete {
				t.Fatalf("misclassified limit: %v; %+v", err, r)
			}
		} else if !r.Satisfied() {
			t.Fatalf("unexpected verdict: %s", localFindings(r))
		}
	}
}

func TestIndependentTimelineSourceHugeExactStaffIsSparse(t *testing.T) {
	// Larger than every host integer. Source validation must neither narrow it nor
	// allocate a staff-count-sized container; explicit exact identity is enough.
	huge := new(big.Int).Lsh(big.NewInt(1), 8192).String()
	body := `<attributes><divisions>3</divisions><key number="` + huge + `"><fifths>0</fifths></key><staves>` + huge + `</staves></attributes>` + timelineNumeral("", `<staff>`+huge+`</staff>`)
	source := (localFixture{body: body}).xml()
	r, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceTimeline})
	if err != nil || !r.Satisfied() {
		t.Fatalf("large exact staff was enumerated/narrowed: %v %s", err, localFindings(r))
	}
	body = `<attributes><divisions>3</divisions><key number="` + huge + `"><fifths>0</fifths></key><staves>` + huge + `</staves></attributes>` + timelineNumeral("", "")
	r, err = InspectXML(context.Background(), strings.NewReader((localFixture{body: body}).xml()), AssessmentOptions{Profile: ProfileSourceTimeline})
	if err != nil || r.Conformance == validation.ConformanceViolated || r.AssessmentComplete || r.Satisfied() {
		t.Fatalf("unresolved implicit staff became an invented result: %v %s", err, localFindings(r))
	}
}
