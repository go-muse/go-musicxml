package validation

import (
	"context"
	"errors"
	"testing"
)

// A window bound before its ancestors that no ancestor witness reaches when
// the root closes cannot be undone: binding refuses with a configuration
// error instead of reporting what the buffered engine would not.
func TestBindingRefusesAWindowNoWitnessReaches(t *testing.T) {
	s := schemaScratchFixture(t, 2, "integer", Budgets{})
	s.schema.windows = map[NodeID]boundWindow{99: {size: 1}}
	err := s.bindSourceSchema(context.Background())
	var configuration *ConfigurationError
	if !errors.As(err, &configuration) || configuration.Field != "schema window" {
		t.Fatalf("binding with an unreached window: %v", err)
	}
	s2 := schemaScratchFixture(t, 2, "integer", Budgets{})
	if err := s2.bindSourceSchema(context.Background()); err != nil {
		t.Fatalf("binding without windows: %v", err)
	}
}
