package musicxml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-muse/go-musicxml/validation"
)

func TestIndependentContextDefaultResourceMatrix(t *testing.T) {
	t.Parallel()
	fixtures := map[string]string{}
	for _, count := range []int{100, 1000} {
		fixtures[fmt.Sprintf("notes%d", count)] = independentTimelineScore(count, false, "1")
	}
	for _, name := range []string{"23a-Tuplets.xml", "33c-Spanners-Slurs.xml", "45a-SimpleRepeat.xml", "45d-Repeats-Nested-Alternatives.xml", "52a-PageLayout.xml"} {
		source, err := os.ReadFile(filepath.Join(musicXMLTestSuiteDirectory, name))
		if err != nil {
			t.Fatal(err)
		}
		fixtures[name] = string(source)
	}
	for name, source := range fixtures {
		t.Run(name, func(t *testing.T) {
			doc, err := Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, model := range []bool{false, true} {
				begin := time.Now()
				var report validation.Report
				if model {
					report, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
				} else {
					report, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceContext})
				}
				t.Logf("model=%t input_bytes=%d elapsed=%s complete=%t satisfied=%t conformance=%s records=%d findings=%d error=%v", model, len(source), time.Since(begin), report.AssessmentComplete, report.Satisfied(), report.Conformance, len(report.Context), len(report.Findings), err)
				if errors.Is(err, validation.ErrResourceLimit) {
					if report.Satisfied() || report.AssessmentComplete {
						t.Fatal("resource exhaustion reported complete")
					}
				} else if err != nil {
					t.Fatalf("unexpected assessment infrastructure error: %v", err)
				}
			}
		})
	}
}

func independentContextMutate(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				independentContextMutate(v.Field(i))
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			independentContextMutate(v.Elem())
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			independentContextMutate(v.Index(i))
		}
	case reflect.String:
		v.SetString("caller mutation")
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(987)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(87)
	}
}
func TestIndependentContextConcurrentReportsAndProfiles(t *testing.T) {
	source := contextFixtureXML(`<attributes><divisions>1</divisions><measure-style><slash type="start"/></measure-style></attributes><barline location="left"><repeat direction="forward"/></barline><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><notations><slur type="start"/></notations></note><note><pitch><step>D</step><octave>4</octave></pitch><duration>1</duration><notations><slur type="stop"/></notations></note><barline location="right"><repeat direction="backward" times="2"/></barline>`, false)
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	run := func(model, extended bool) (validation.Report, error) {
		p := ProfileSourceTimeline.ID()
		if model {
			p = ProfileModelTimeline.ID()
		}
		if extended {
			p = ProfileSourceContext.ID()
			if model {
				p = ProfileModelContext.ID()
			}
		}
		if model {
			return AssessModel(context.Background(), doc, ModelOptions{Profile: ModelProfile(p)})
		}
		return InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: SourceProfile(p)})
	}
	baselines := map[[2]bool][]byte{}
	for _, model := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			r, e := run(model, extended)
			if e != nil {
				t.Fatal(e)
			}
			if extended && len(r.Context) == 0 || !extended && len(r.Context) != 0 {
				t.Fatal("profile leakage")
			}
			data, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			baselines[[2]bool{model, extended}] = data
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			model, extended := i%2 == 0, i%4 < 2
			for j := 0; j < 3; j++ {
				r, e := run(model, extended)
				if e != nil {
					t.Error(e)
					return
				}
				data, e := json.Marshal(r)
				if e != nil {
					t.Error(e)
					return
				}
				if string(data) != string(baselines[[2]bool{model, extended}]) {
					t.Error("concurrent assessment or previous report mutation changed a fresh report")
					return
				}
				independentContextMutate(reflect.ValueOf(&r).Elem())
			}
		}(i)
	}
	wg.Wait()
	after, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("model ingress mutated by assessment")
	}
}

func TestIndependentContextOpusDoesNotFetch(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); http.Error(w, "must not fetch", 500) }))
	defer server.Close()
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="` + server.URL + `/one" new-page="yes"/><opus><score xlink:href="` + server.URL + `/two"/></opus><score xlink:href="` + server.URL + `/three" new-page="no"/></opus>`
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []bool{false, true} {
		var r validation.Report
		if model {
			r, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
		} else {
			r, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceContext})
		}
		if err != nil || !r.Satisfied() {
			t.Fatalf("model=%t err=%v", model, err)
		}
		pagination := make([]validation.ContextResult, 0, 3)
		for _, v := range r.Context {
			if v.Property == "layout.opus-new-page" {
				pagination = append(pagination, v)
			}
		}
		r.Context = pagination
		if len(r.Context) != 3 {
			t.Fatalf("local scores=%d", len(r.Context))
		}
		if r.Context[0].Layout.Scope != r.Context[2].Layout.Scope || r.Context[0].Layout.Scope == r.Context[1].Layout.Scope {
			t.Fatal("nested opus scope collapsed")
		}
		if r.Context[0].Value.Value.Text != "yes" || r.Context[1].Value.State != validation.EvidenceUnknown || r.Context[2].Value.Value.Text != "no" {
			t.Fatal("explicit/application-dependent pagination confused")
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("linked scores fetched %d times", hits.Load())
	}
}

func TestIndependentContextHugeSparseSourceValues(t *testing.T) {
	const huge = "18446744073709551616"
	source := contextFixtureXML(`<attributes><divisions>10</divisions><staves>`+huge+`</staves></attributes><barline location="left"><repeat direction="forward" times="`+huge+`"/></barline><note time-only="18446744073709551616,18446744073709551617" dynamics="125"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><staff>`+huge+`</staff></note>`, false)
	r, err := InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceContext, Budgets: validation.Budgets{MaxContextFacts: 1000}})
	if err != nil || !r.Satisfied() {
		t.Fatalf("symbolic huge values required expansion or host int: %v %s", err, localFindings(r))
	}
	if len(r.Context) > 100 {
		t.Fatalf("huge scalar expanded projection: %d records", len(r.Context))
	}
	found := false
	for _, v := range r.Context {
		if v.Timing != nil && v.Timing.Kind == "note" {
			found = true
			p := v.Timing
			if p.Staves.State != validation.EvidenceKnown || p.Staves.Value.String() != huge || p.Staff.State != validation.EvidenceKnown || p.Staff.Value.String() != huge || p.StaffExists.State != validation.EvidenceKnown || !p.StaffExists.Value {
				t.Fatalf("huge sparse identity lost: %+v", p)
			}
			if p.Duration.Value.String() != "1/10" || p.ReferenceVelocity.Value.String() != "225/2" {
				t.Fatalf("exact arithmetic rounded: duration=%s velocity=%s", p.Duration.Value.String(), p.ReferenceVelocity.Value.String())
			}
		}
	}
	if !found {
		t.Fatal("missing exact timing payload")
	}
}

func TestIndependentContextWrongAssemblyConstructors(t *testing.T) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	opus := d.schemas[validation.AssemblyOpus]
	for name, fn := range map[string]func() error{
		"local": func() error { _, e := validationNewLocalContextPlan(opus); return e },
		"span": func() error {
			_, e := validationNewSpanContextPlan(validation.ContextPlanSpec{Schema: opus})
			return e
		},
		"repeat": func() error {
			_, e := validationNewRepeatContextPlan(validation.ContextPlanSpec{Schema: opus, TimelineProvider: validation.RuleKey{ID: "timeline", Revision: 1}})
			return e
		},
		"navigation": func() error {
			_, e := validationNewNavigationContextPlan(validation.ContextPlanSpec{Schema: opus, TimelineProvider: validation.RuleKey{ID: "timeline", Revision: 1}})
			return e
		},
		"layout": func() error {
			_, e := validationNewLayoutContextPlan(validation.ContextPlanSpec{Schema: opus})
			return e
		},
		"performance": func() error {
			_, e := validationNewPerformanceContextPlan(validation.ContextPlanSpec{Schema: opus})
			return e
		},
		"time-only":     func() error { _, e := validationNewTimeOnlyOrderPlan(opus); return e },
		"figure-number": func() error { _, e := validationNewFigureNumberNumericPlan(opus); return e },
		"bend-consumer": func() error {
			_, e := validationNewBendReleaseSignPlan(validation.ConsumerSpec{Schema: opus, Provider: validation.RuleKey{ID: "p", Revision: 1}})
			return e
		},
		"repeat-consumer": func() error {
			_, e := validationNewGraphicalRepeatContentPlan(validation.ConsumerSpec{Schema: opus, Provider: validation.RuleKey{ID: "p", Revision: 1}})
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			if e := fn(); !errors.Is(e, validation.ErrInvalidConfiguration) {
				t.Fatalf("wrong assembly accepted: %v", e)
			}
		})
	}
}
