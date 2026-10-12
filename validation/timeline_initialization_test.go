package validation

import (
	"errors"
	"sync"
	"testing"
)

func TestTimelineInitializationFailureIsCachedAndCallerOwned(t *testing.T) {
	schema := miniatureSourceSchema(t)
	const count = 24
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() {
			copy := *schema
			_, errs[i] = NewChordDurationPlan(TimelineConsumerSpec{Schema: &copy, Provider: RuleKey{ID: "provider", Revision: 1}})
		})
	}
	wg.Wait()
	if schema.timeline.err == nil || schema.timeline.bindings != nil {
		t.Fatal("incompatible schema did not retain its binding failure")
	}
	want := schema.timeline.err.Error()
	for i, err := range errs {
		var config *ConfigurationError
		if !errors.As(err, &config) || err.Error() != want || err == schema.timeline.err {
			t.Fatalf("constructor %d returned a shared or inconsistent error: %v", i, err)
		}
		config.Reason = "caller mutation"
	}
	if schema.timeline.err.Error() != want {
		t.Fatal("caller changed memoized failure")
	}
}

func TestTimelineInitializationSnapshotDetachesCache(t *testing.T) {
	schema := miniatureSourceSchema(t)
	engine := sourceSchemaEngineFor(t, schema, XSDCheckScalar)
	for _, plan := range engine.catalog.plans {
		if plan.schema == schema || plan.schema.timeline != nil || plan.schema.kernel != schema.kernel {
			t.Fatal("XSD plan did not detach construction cache while preserving schema graph")
		}
	}
	_, _ = schema.sharedTimelineBindings()
	for _, plan := range engine.catalog.plans {
		if plan.schema.timeline != nil {
			t.Fatal("warming caller schema grew an admitted XSD catalog")
		}
	}
}
