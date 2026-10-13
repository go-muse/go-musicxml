package validation

import "testing"

func TestEngineProfileIsDefensiveMetadata(t *testing.T) {
	e := sourceSchemaEngine(t, XSDCheckScalar)
	p, ok := e.Profile("source-schema-test")
	if !ok {
		t.Fatal("missing profile")
	}
	p.Required[0].ID = "edited"
	q, _ := e.Profile("source-schema-test")
	if q.Required[0].ID == "edited" {
		t.Fatal("profile getter aliases catalog")
	}
	var nilEngine *Engine
	if _, ok := nilEngine.Profile("x"); ok {
		t.Fatal("nil engine has profile")
	}
}
