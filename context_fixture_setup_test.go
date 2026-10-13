package musicxml

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestContextFixtureSharedModelConcurrentReadOnly(t *testing.T) {
	source := (localFixture{body: `<attributes><divisions>1</divisions></attributes>` + timelineNote("1", "", "")}).xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []validation.ProfileID{ProfileModelLocal, ProfileModelTimeline} {
		t.Run(string(profile), func(t *testing.T) {
			baseline, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: profile})
			if err != nil || !baseline.Satisfied() {
				t.Fatalf("baseline: %v; %s", err, localFindings(baseline))
			}
			for worker := 0; worker < 4; worker++ {
				t.Run(fmt.Sprintf("worker%d", worker), func(t *testing.T) {
					t.Parallel()
					report, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: profile})
					if err != nil || !reflect.DeepEqual(report, baseline) {
						t.Fatalf("shared-model assessment changed: %v", err)
					}
					report.Profile.Required[0].ID = "caller-mutated"
				})
			}
		})
	}
	if !reflect.DeepEqual(doc, snapshot) {
		t.Fatal("concurrent assessments mutated their shared model fixture")
	}
}

// Large parallel assessment matrices reuse their decoded input only. Verify
// the actual fixture after every descendant has finished, including failures.
func assertReadOnlyModelFixture(t *testing.T, source string, document Document) {
	t.Helper()
	snapshot, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !reflect.DeepEqual(document, snapshot) {
			t.Error("assessments mutated their shared model fixture")
		}
	})
}

// These fixtures may share only the immutable compiled engine. Caller-owned
// metadata and every assessment's account, session, input and report stay local.
func TestContextFixtureEnginesReuseImmutableSetup(t *testing.T) {
	for _, timeline := range []bool{false, true} {
		t.Run(fmt.Sprintf("span/timeline=%t", timeline), func(t *testing.T) {
			first, profiles := spanTestEngine(t, timeline)
			second, otherProfiles := spanTestEngine(t, timeline)
			if first != second {
				t.Error("identical span fixture rebuilt its immutable engine")
			}
			if !reflect.DeepEqual(profiles, otherProfiles) {
				t.Fatal("repeated span fixture changed its profile metadata")
			}
			profiles[0].ID = "caller-mutated"
			profiles[0].Required[0].ID = "caller-mutated"
			profiles[0].Support[0].Reason = "caller-mutated"
			profiles[0].SupportedRoots[0].Local = "caller-mutated"
			_, freshProfiles := spanTestEngine(t, timeline)
			if !reflect.DeepEqual(freshProfiles, otherProfiles) {
				t.Error("caller mutation escaped into cached span profile metadata")
			}
		})
	}
	withoutTimeline, _ := spanTestEngine(t, false)
	withTimeline, _ := spanTestEngine(t, true)
	if withoutTimeline == withTimeline {
		t.Error("distinct span provider configurations share an engine")
	}
	if localContextTestEngine(t) != localContextTestEngine(t) {
		t.Error("identical local context fixture rebuilt its immutable engine")
	}
	if layoutStateTestEngine(t) != layoutStateTestEngine(t) {
		t.Error("identical layout state fixture rebuilt its immutable engine")
	}
}

func TestContextFixtureEnginesKeepConcurrentReportsIndependent(t *testing.T) {
	const body = `<attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration><notations><slur type="start"/></notations></note><note><rest/><duration>1</duration><notations><slur type="stop"/></notations></note>`
	for _, fixture := range []struct {
		name   string
		engine func(*testing.T) *validation.Engine
	}{
		{"local", localContextTestEngine},
		{"layout", layoutStateTestEngine},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			source := localContextFixture(body, false)
			engine := fixture.engine(t)
			var baseline [2]validation.Report
			for i, model := range []bool{false, true} {
				baseline[i] = localContextAssess(t, engine, source, model)
			}
			for worker := 0; worker < 4; worker++ {
				t.Run(fmt.Sprintf("worker%d", worker), func(t *testing.T) {
					t.Parallel()
					for i, model := range []bool{false, true} {
						report := localContextAssess(t, fixture.engine(t), source, model)
						if !reflect.DeepEqual(report, baseline[i]) {
							t.Fatal("fresh concurrent assessment differs from sequential baseline")
						}
						// Finished reports must not lend mutable slices to other runs.
						report.Profile.Required[0].ID = "caller-mutated"
						if len(report.Context) != 0 {
							report.Context[0].Property = "caller-mutated"
						}
					}
				})
			}
		})
	}
	for _, timeline := range []bool{false, true} {
		t.Run(fmt.Sprintf("span/timeline=%t", timeline), func(t *testing.T) {
			source := (localFixture{body: body}).xml()
			engine, profiles := spanTestEngine(t, timeline)
			baseline := make(map[validation.Target]validation.Report)
			spanBothWithEngine(t, engine, profiles, source, false, "", func(t *testing.T, report validation.Report) {
				baseline[report.Target] = report
			})
			for worker := 0; worker < 4; worker++ {
				t.Run(fmt.Sprintf("worker%d", worker), func(t *testing.T) {
					t.Parallel()
					engine, profiles := spanTestEngine(t, timeline)
					spanBothWithEngine(t, engine, profiles, source, false, "", func(t *testing.T, report validation.Report) {
						if !reflect.DeepEqual(report, baseline[report.Target]) {
							t.Fatal("fresh concurrent span assessment differs from sequential baseline")
						}
						if len(report.Context) != 0 {
							report.Context[0].Property = "caller-mutated"
						}
						report.Profile.Required[0].ID = "caller-mutated"
					})
				})
			}
		})
	}
}

func BenchmarkContextFixtureEngineSetup(b *testing.B) {
	for _, fixture := range []struct {
		name  string
		build func() (*validation.Engine, error)
		get   func() (*validation.Engine, error)
	}{
		{"local", buildLocalContextTestEngine, cachedLocalContextTestEngine},
		{"layout", buildLayoutStateTestEngine, cachedLayoutStateTestEngine},
		{"span", func() (*validation.Engine, error) {
			fixture, err := buildSpanTestEngine(true)
			return fixture.engine, err
		}, func() (*validation.Engine, error) {
			fixture, err := spanTestEngineWithTimeline()
			return fixture.engine, err
		}},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			for _, setup := range []struct {
				name string
				get  func() (*validation.Engine, error)
			}{{"cold", fixture.build}, {"cached", fixture.get}} {
				b.Run(setup.name, func(b *testing.B) {
					if _, err := fixture.get(); err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if engine, err := setup.get(); err != nil || engine == nil {
							b.Fatalf("fixture setup: %v", err)
						}
					}
				})
			}
		})
	}
}
