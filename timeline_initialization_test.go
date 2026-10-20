package musicxml

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Inspect only identity, without exposing or mutating the sealed role table.
// The real generated schema lives in this package, outside validation's tests.
func timelineBindingIdentity(plan reflect.Value) uintptr {
	for _, name := range []string{"timeline", "timelineConsumer"} {
		p := plan.FieldByName(name)
		if !p.IsNil() {
			return p.Elem().FieldByName("Bindings").Pointer()
		}
	}
	return 0
}

func TestTimelineConstructorsShareImmutableBindings(t *testing.T) {
	schema := timelineContractSchema(t)
	owners := timelineContractOwners(t, schema)
	const count = 24
	plans := make([]validation.Plan, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range plans {
		wg.Go(func() {
			// Value-copied immutable schema handles must share cold initialization.
			copy := *schema
			<-start
			switch i % 3 {
			case 0:
				plans[i], errs[i] = validationNewTimelinePlan(validation.TimelinePlanSpec{Schema: &copy, PartOwners: owners})
			case 1:
				plans[i], errs[i] = validationNewChordDurationPlan(validation.ConsumerSpec{Schema: &copy, Provider: validation.RuleKey{ID: "provider", Revision: 1}})
			case 2:
				plans[i], errs[i] = validationNewNumeralKeyPlan(validation.ConsumerSpec{Schema: &copy, Provider: validation.RuleKey{ID: "provider", Revision: 1}})
			}
		})
	}
	close(start)
	wg.Wait()
	var shared uintptr
	for i, plan := range plans {
		if errs[i] != nil {
			t.Fatalf("constructor %d: %v", i, errs[i])
		}
		got := timelineBindingIdentity(reflect.ValueOf(plan))
		if got == 0 {
			t.Fatalf("constructor %d did not bind the timeline roles", i)
		}
		if i == 0 {
			shared = got
		} else if got != shared {
			t.Errorf("constructor %d retained another role table; constructors over one schema must share immutable bindings", i)
		}
	}
}

func TestTimelineCatalogKeepsSharedImmutableBindings(t *testing.T) {
	engine, err := buildTimelineAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	plans := reflect.ValueOf(engine).Elem().FieldByName("catalog").Elem().FieldByName("plans")
	var shared uintptr
	count := 0
	for _, key := range plans.MapKeys() {
		got := timelineBindingIdentity(plans.MapIndex(key))
		if got == 0 {
			continue
		}
		if count == 0 {
			shared = got
		} else if got != shared {
			t.Errorf("catalog plan %s copied or recompiled the shared role table", key.String())
		}
		count++
	}
	if count != 3 {
		t.Fatalf("got %d timeline plans, want provider and both consumers", count)
	}
}

func TestTimelineBindingInitializationSeparatesSchemasAndErrors(t *testing.T) {
	first := timelineContractSchema(t)
	second := timelineContractSchema(t)
	spec := validation.ConsumerSpec{Provider: validation.RuleKey{ID: "provider", Revision: 1}}
	spec.Schema = first
	a, err := validationNewChordDurationPlan(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Schema = second
	b, err := validationNewChordDurationPlan(spec)
	if err != nil {
		t.Fatal(err)
	}
	if timelineBindingIdentity(reflect.ValueOf(a)) == timelineBindingIdentity(reflect.ValueOf(b)) {
		t.Fatal("independently compiled schemas unexpectedly share initialization state")
	}
	invalid, err := validation.CompileSchema(validation.AssemblyMusicXML, validation.SchemaData{})
	if err != nil {
		t.Fatal(err)
	}
	spec.Schema = invalid
	_, err = validationNewChordDurationPlan(spec)
	var config *validation.ConfigurationError
	if !errors.As(err, &config) {
		t.Fatalf("missing invalid-schema configuration error: %v", err)
	}
	want := *config
	config.Field, config.Reason = "caller field", "caller reason"
	_, err = validationNewNumeralKeyPlan(spec)
	var again *validation.ConfigurationError
	if !errors.As(err, &again) || *again != want {
		t.Fatalf("caller mutation poisoned a later constructor error: %v", err)
	}
	if _, err := validationNewNumeralKeyPlan(validation.ConsumerSpec{Schema: first, Provider: spec.Provider}); err != nil {
		t.Fatalf("invalid schema poisoned independent valid schema: %v", err)
	}
}

func TestTimelineInitializationCannotGrowAdmittedModelCatalog(t *testing.T) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.schemas) != 2 {
		t.Fatalf("got %d retained schema assemblies, want MusicXML and opus", len(d.schemas))
	}
	for _, binding := range d.plans {
		schema := reflect.ValueOf(binding.Plan).FieldByName("schema")
		if schema.IsNil() {
			continue
		}
		assembly := validation.Assembly(schema.Elem().FieldByName("assembly").String())
		original := d.schemas[assembly]
		if original == nil || schema.Elem().FieldByName("kernel").Pointer() != reflect.ValueOf(original).Elem().FieldByName("kernel").Pointer() {
			t.Fatalf("%s did not retain its already compiled schema", binding.ID)
		}
	}
	catalog, err := newCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := catalog.Profile(ProfileModelLocal.ID())
	assertDetached := func() {
		t.Helper()
		plans := reflect.ValueOf(catalog).Elem().FieldByName("plans")
		for _, key := range plans.MapKeys() {
			schema := plans.MapIndex(key).FieldByName("schema")
			if !schema.IsNil() && !schema.Elem().FieldByName("timeline").IsNil() {
				t.Errorf("admitted plan %s retains the caller's growable construction cache", key.String())
			}
		}
	}
	assertDetached()
	schema := d.schemas[validation.AssemblyMusicXML]
	provider, consumer := timelineContractPlans(t, schema, false)
	for _, plan := range []validation.Plan{provider, consumer} {
		p := reflect.ValueOf(plan)
		for _, name := range []string{"timeline", "timelineConsumer"} {
			v := p.FieldByName(name)
			if !v.IsNil() && !v.Elem().FieldByName("Schema").Elem().FieldByName("timeline").IsNil() {
				t.Error("timeline plan retains the caller's construction cache")
			}
		}
	}
	assertDetached()
	after, _ := catalog.Profile(ProfileModelLocal.ID())
	if !reflect.DeepEqual(before, after) {
		t.Fatal("warming the caller schema changed the admitted profile")
	}
	// The caller can replace its handle without retargeting any existing plan.
	*schema = validation.CompiledSchema{}
	rules, profiles, plans := timelineContractCatalog(t, timelineContractSchema(t), provider, consumer, false)
	if _, err := validation.NewCatalog(rules, profiles, plans); err != nil {
		t.Fatalf("caller overwriting schema handle retargeted timeline plan: %v", err)
	}
	if _, err := newCatalog(d.rules, d.profiles, d.plans); err != nil {
		t.Fatalf("caller overwriting schema handle retargeted XSD plan: %v", err)
	}
}

func TestTimelineInitializationIndependentSchemaInternOrder(t *testing.T) {
	first := timelineContractSchema(t)
	second := timelineContractSchema(t)
	provider, _ := timelineContractPlans(t, first, false)
	_, consumer := timelineContractPlans(t, second, false)
	rules, profiles, plans := timelineContractCatalog(t, first, provider, consumer, false)
	forward, err := validation.NewCatalog(rules, profiles, plans)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(plans)
	reverse, err := validation.NewCatalog(rules, profiles, plans)
	if err != nil {
		t.Fatal(err)
	}
	if timelineContractDigest(forward) != timelineContractDigest(reverse) {
		t.Fatal("equal-digest schema interning order changed catalog digest")
	}
}

func TestTimelineSharedBindingsConcurrentSessions(t *testing.T) {
	sources := []string{
		(localFixture{body: `<attributes><divisions>1</divisions><key><fifths>0</fifths></key></attributes>` + timelineNumeral("", "")}).xml(),
		(localFixture{body: timelineNumeral("", "")}).xml(),
	}
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Go(func() {
			report, err := InspectXML(context.Background(), strings.NewReader(sources[i%2]), SourceOptions{Profile: ProfileSourceTimeline})
			if err != nil || !report.AssessmentComplete || report.Satisfied() != (i%2 == 0) {
				t.Errorf("concurrent session %d: satisfied=%t complete=%t error=%v", i, report.Satisfied(), report.AssessmentComplete, err)
			}
		})
	}
	wg.Wait()
}

func BenchmarkTimelineColdInitialization(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := buildTimelineAssessmentEngine(); err != nil {
			b.Fatal(err)
		}
	}
}
