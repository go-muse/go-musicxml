package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

// cloneCorruptionPlan is only for tests that mutate an already-loaded plan and
// exercise validate. Their immutable fixture is loaded once per test group;
// every case still owns fresh maps/slices and runs the real validator.
//
// source must be the pristine result of load, not an already-corrupted plan.
// JSON document trees are acyclic. load adds exactly the document-row aliases in
// the ten indexes below, so copy each tree once and rebuild those aliases from
// its copied rows. Parser/loader tests continue to call load directly.
func cloneCorruptionPlan(source *plan) *plan {
	p := &plan{
		planDir:      source.planDir,
		docs:         make(map[string]object, len(source.docs)),
		checks:       make(map[string]object, len(source.checks)),
		instances:    make(map[string]object, len(source.instances)),
		tests:        make(map[string]object, len(source.tests)),
		requirements: make(map[string]object, len(source.requirements)),
		stages:       make(map[string]object, len(source.stages)),
		research:     make(map[string]object, len(source.research)),
		dispositions: make(map[string]object, len(source.dispositions)),
		questions:    make(map[string]object, len(source.questions)),
		deferred:     make(map[string]object, len(source.deferred)),
		capabilities: make(map[string]object, len(source.capabilities)),
	}
	for name, d := range source.docs {
		p.docs[name] = clonePlanJSON(d).(object)
	}
	for _, index := range []struct {
		field, key string
		rows       map[string]object
	}{
		{"checks", "id", p.checks}, {"instances", "id", p.instances},
		{"test_contracts", "id", p.tests}, {"requirements", "id", p.requirements},
		{"stages", "id", p.stages}, {"research_records", "research_id", p.research},
		{"dispositions", "research_id", p.dispositions}, {"open_questions", "id", p.questions},
		{"deferred_items", "id", p.deferred}, {"capability_gates", "id", p.capabilities},
	} {
		for _, d := range p.docs {
			for _, row := range rows(d, index.field) {
				index.rows[str(row, index.key)] = row
			}
		}
	}
	return p
}

func clonePlanJSON(value any) any {
	switch value := value.(type) {
	case object:
		return object(clonePlanJSON(map[string]any(value)).(map[string]any))
	case map[string]any:
		if value == nil {
			return map[string]any(nil)
		}
		cloned := make(map[string]any, len(value))
		for key, child := range value {
			cloned[key] = clonePlanJSON(child)
		}
		return cloned
	case []any:
		if value == nil {
			return []any(nil)
		}
		cloned := make([]any, len(value))
		for i, child := range value {
			cloned[i] = clonePlanJSON(child)
		}
		return cloned
	case nil, bool, string, float64, json.Number:
		return value
	default:
		panic(fmt.Sprintf("unexpected plan JSON type %T", value))
	}
}

func TestCorruptionPlanClone(t *testing.T) {
	source := testPlan(t)
	pristine := testPlan(t)
	p, sibling := cloneCorruptionPlan(source), cloneCorruptionPlan(source)
	if !reflect.DeepEqual(p, pristine) {
		t.Fatal("cloned plan differs from a freshly loaded plan")
	}
	if err := p.validate(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, pristine) {
		t.Fatal("validation mutated the cloned plan")
	}
	for name, d := range source.docs {
		assertPlanJSONIsolation(t, d, p.docs[name])
	}

	// Check every indexed row against the actual copied document row. Equal
	// values are not enough: validation must observe mutations through either
	// the document tree or the index, just as it does after load.
	for _, index := range []struct {
		field, key string
		rows       map[string]object
	}{
		{"checks", "id", p.checks},
		{"instances", "id", p.instances},
		{"test_contracts", "id", p.tests},
		{"requirements", "id", p.requirements},
		{"stages", "id", p.stages},
		{"research_records", "research_id", p.research},
		{"dispositions", "research_id", p.dispositions},
		{"open_questions", "id", p.questions},
		{"deferred_items", "id", p.deferred},
		{"capability_gates", "id", p.capabilities},
	} {
		count := 0
		for _, d := range p.docs {
			values, ok := d[index.field].([]any)
			if !ok {
				continue
			}
			for _, value := range values {
				row := value.(map[string]any)
				id := row[index.key].(string)
				indexed, ok := index.rows[id]
				if !ok {
					t.Fatalf("missing %s index row %s", index.field, id)
				}
				indexed["__clone_index_probe"] = true
				if row["__clone_index_probe"] != true {
					t.Fatalf("%s index row %s is detached from its document", index.field, id)
				}
				delete(row, "__clone_index_probe")
				if _, ok := indexed["__clone_index_probe"]; ok {
					t.Fatalf("%s document row %s is detached from its index", index.field, id)
				}
				count++
			}
		}
		if count != len(index.rows) {
			t.Fatalf("%s: %d document rows, %d index rows", index.field, count, len(index.rows))
		}
		for id := range index.rows {
			delete(index.rows, id)
			break
		}
	}
	p.docs["external-snapshot.json"]["registry_commit"] = "changed"
	delete(p.docs, "overlap-policy.json")
	p.planDir = "changed"
	if !reflect.DeepEqual(source, pristine) || !reflect.DeepEqual(sibling, pristine) {
		t.Fatal("mutating a clone changed its source or another clone")
	}
}

func TestClonePlanJSON(t *testing.T) {
	backing := []any{"first", "reserved", "tail"}
	source := object{
		"null": nil, "bool": true, "text": "value", "float": float64(1.25),
		"number":     json.Number("9007199254740993"),
		"nil_object": object(nil), "empty_object": object{},
		"nil_map": map[string]any(nil), "empty_map": map[string]any{},
		"nil_slice": []any(nil), "empty_slice": []any{},
		"nested":         map[string]any{"rows": []any{map[string]any{"id": "row"}}},
		"spare_capacity": backing[:1],
	}
	cloned := clonePlanJSON(source).(object)
	if !reflect.DeepEqual(cloned, source) {
		t.Fatal("clone changed JSON values, numeric types, or nil/empty containers")
	}
	assertPlanJSONIsolation(t, source, cloned)
	cloned["spare_capacity"] = append(cloned["spare_capacity"].([]any), "appended")
	if backing[1] != "reserved" {
		t.Fatal("appending to a cloned slice reused the source backing array")
	}
}

func assertPlanJSONIsolation(t *testing.T, source, cloned any) {
	t.Helper()
	switch source := source.(type) {
	case object:
		assertPlanJSONIsolation(t, map[string]any(source), map[string]any(cloned.(object)))
	case map[string]any:
		if source == nil {
			return
		}
		copied := cloned.(map[string]any)
		copied["__clone_map_probe"] = true
		if _, ok := source["__clone_map_probe"]; ok {
			t.Fatal("cloned JSON map aliases its source")
		}
		delete(copied, "__clone_map_probe")
		for key, value := range source {
			assertPlanJSONIsolation(t, value, copied[key])
		}
	case []any:
		copied := cloned.([]any)
		for i, value := range source {
			child := copied[i]
			copied[i] = struct{}{}
			if _, ok := source[i].(struct{}); ok {
				t.Fatal("cloned JSON slice aliases its source")
			}
			copied[i] = child
			assertPlanJSONIsolation(t, value, child)
		}
	}
}

func BenchmarkCorruptionPlanSetup(b *testing.B) {
	dir := filepath.Join("..", "..", "docs", "validation", "plan")
	source, err := load(dir)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("load", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := load(dir); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("clone", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			cloneCorruptionPlan(source)
		}
	})
}
