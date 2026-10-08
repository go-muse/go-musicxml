package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPlan(t *testing.T) *plan {
	t.Helper()
	p, err := load(filepath.Join("..", "..", "docs", "validation", "plan"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlanningIntegrity(t *testing.T) {
	if err := testPlan(t).validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedRegistryIfProvided(t *testing.T) {
	root := os.Getenv("MUSICXML_PLAN_REGISTRY")
	if root == "" {
		t.Skip("external snapshot not materialized; offline planning integrity runs separately")
	}
	var err error
	root, err = registryTestRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := testPlan(t).verifyRegistry(root); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptionsRejected(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*plan)
	}{
		{"missing_research_record", func(p *plan) { delete(p.research, "MX40-PROSE-bend-release-negative") }},
		{"dangling_research_check", func(p *plan) { p.research["MX40-PROSE-accordion-at-least-one"]["check_ids"] = []any{"missing"} }},
		{"same_count_research_substitution", func(p *plan) {
			v := p.research["MX40-PROSE-bend-release-negative"]
			delete(p.research, "MX40-PROSE-bend-release-negative")
			p.research["invented"] = v
		}},
		{"wrong_registry_pin", func(p *plan) { p.docs["external-snapshot.json"]["registry_commit"] = "main" }},
		{"swapped_external_pointer", func(p *plan) {
			p.research["MX40-PROSE-accordion-at-least-one"]["registry_pointer"] = "/records/0/record"
		}},
		{"source_role_corruption", func(p *plan) { p.research["MX40-PROSE-bend-release-negative"]["role"] = "external" }},
		{"instance_revision_zero", func(p *plan) { p.instances["INSTANCE-MX40-REQ-4aefb63559e24c13"]["revision"] = float64(0) }},
		{"instance_stage_missing", func(p *plan) { p.instances["INSTANCE-MX40-REQ-4aefb63559e24c13"]["stage"] = "missing" }},
		{"instance_template_missing", func(p *plan) { p.instances["INSTANCE-MX40-REQ-4aefb63559e24c13"]["template_id"] = "missing" }},
		{"instance_pointer_mismatch", func(p *plan) {
			p.instances["INSTANCE-MX40-REQ-4aefb63559e24c13"]["registry_pointer"] = "/records/1/record"
		}},
		{"missing_simple_aggregate_reuse", func(p *plan) { p.instances["INSTANCE-MX40-REQ-c55288f5616199a3"]["reuse_instances"] = []any{} }},
		{"lost_complex_residual", func(p *plan) { p.instances["INSTANCE-MX40-REQ-4ae779abef572996"]["diagnostic_policy"] = "discard" }},
		{"missing_typed_dependency", func(p *plan) {
			g := p.instances["INSTANCE-MX40-REQ-4aefb63559e24c13"]["plan_dependencies"].(map[string]any)
			g["language_semantics"] = []any{}
		}},
		{"dangling_schema_edge", func(p *plan) {
			g := p.instances["INSTANCE-MX40-REQ-4aefb63559e24c13"]["plan_dependencies"].(map[string]any)
			g["schema_component"] = []any{"missing"}
		}},
		{"unknown_target_policy", func(p *plan) {
			p.instances["INSTANCE-MX40-REQ-4aefb63559e24c13"]["target_policy"] = "source_equals_model"
		}},
		{"missing_boundary_axis", func(p *plan) { delete(p.tests["TEST-accordion-at-least-one"]["cases"].(map[string]any), "boundary") }},
		{"invalid_case_type", func(p *plan) {
			p.tests["TEST-accordion-at-least-one"]["cases"].(map[string]any)["positive"] = []any{float64(1)}
		}},
		{"blank_case", func(p *plan) {
			p.tests["TEST-accordion-at-least-one"]["cases"].(map[string]any)["positive"] = []any{" "}
		}},
		{"fake_passed_test", func(p *plan) { p.tests["TEST-accordion-at-least-one"]["status"] = "passed" }},
		{"fake_implemented_check", func(p *plan) { p.checks["CHECK-accordion-at-least-one"]["status"] = "implemented" }},
		{"asymmetric_test_link", func(p *plan) {
			p.tests["TEST-accordion-at-least-one"]["check_ids"] = append(p.tests["TEST-accordion-at-least-one"]["check_ids"].([]any), "CHECK-beam-number-distinct")
		}},
		{"orphan_contract", func(p *plan) {
			p.checks["orphan"] = object{"id": "orphan", "revision": float64(1), "role": "predicate"}
		}},
		{"stage_cycle", func(p *plan) { p.stages["STAGE-DEF-01"]["depends_on"] = []any{"STAGE-DEF-05"} }},
		{"operator_cycle", func(p *plan) {
			p.checks["CHECK-accordion-at-least-one"]["dependencies"] = []any{"CHECK-accordion-at-least-one"}
		}},
		{"missing_deferred_section", func(p *plan) { delete(p.deferred, "D15") }},
		{"missing_deferred_bullet", func(p *plan) {
			for _, r := range p.requirements {
				r["source_bullets"] = []any{}
			}
		}},
		{"missing_issue", func(p *plan) { delete(p.questions, "MX40-ISSUE-left-barline-attributes") }},
		{"issue_hard_error_authority", func(p *plan) { p.questions["MX40-ISSUE-left-barline-attributes"]["hard_error_basis"] = true }},
		{"missing_translation", func(p *plan) { delete(p.dispositions, "MX40-PROSE-fractional-divisions") }},
		{"false_translation_claim", func(p *plan) { p.dispositions["MX40-PROSE-fractional-divisions"]["translation"] = "complete" }},
		{"unbound_capability_claimed_implemented", func(p *plan) {
			for _, c := range p.capabilities {
				c["provider_status"] = "implemented"
				break
			}
		}},
		{"capability_link_missing", func(p *plan) { p.checks["CHECK-accordion-at-least-one"]["capability_ids"] = []any{"missing"} }},
		{"non_string_dependency", func(p *plan) { p.checks["CHECK-accordion-at-least-one"]["dependencies"] = []any{true} }},
		{"product_coverage_claim", func(p *plan) { p.docs["external-snapshot.json"]["executable_product_predicates"] = float64(2560) }},
		{"full_standard_claim", func(p *plan) { p.docs["external-snapshot.json"]["full_normative_closure"] = true }},
		{"mixed_type_case", func(p *plan) {
			p.tests["TEST-accordion-at-least-one"]["cases"].(map[string]any)["negative"] = []any{"valid", float64(123)}
		}},
		{"invalid_test_target", func(p *plan) { p.tests["TEST-accordion-at-least-one"]["targets"] = []any{"imaginary-target"} }},
		{"invalid_requirement_target", func(p *plan) { p.requirements["REQ-DEF-XML-ATTR"]["targets"] = []any{"imaginary-target"} }},
		{"same_count_deferred_substitution", func(p *plan) { v := p.deferred["D01"]; delete(p.deferred, "D01"); p.deferred["D99"] = v }},
		{"invented_source_bullet", func(p *plan) {
			p.requirements["REQ-DEF-XML-ATTR"]["source_bullets"] = append(p.requirements["REQ-DEF-XML-ATTR"]["source_bullets"].([]any), "DEF99-99")
		}},
		{"missing_decision_closure", func(p *plan) { delete(p.checks["CHECK-bend-release-pair"], "closure") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPlan(t)
			tt.mutate(p)
			if err := p.validate(); err == nil {
				t.Fatal("corruption was accepted")
			}
		})
	}
}

func TestJSONIntegrity(t *testing.T) {
	for _, input := range []string{`{"id":1,"id":2}`, `{"cases":{"a":[],"a":[]}}`, `{} {}`, `{"a":`, ``} {
		if err := uniqueJSON([]byte(input)); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	if err := uniqueJSON([]byte(`{"a":[1,{"b":true}],"c":null}`)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsDuplicateIdentity(t *testing.T) {
	dir := t.TempDir()
	b := `{"format":"musicxml-validation-plan-1","checks":[{"id":"a"},{"id":"a"}]}`
	if err := os.WriteFile(filepath.Join(dir, "duplicate.json"), []byte(b), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := load(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRecordPointers(t *testing.T) {
	for _, p := range []string{"/records/0/record", "/records/2559/record"} {
		if _, err := recordIndex(p); err != nil {
			t.Error(err)
		}
	}
	for _, p := range []string{"records/0/record", "/records/zero/record", "/issues/0", "/records/0/record/parameters"} {
		if _, err := recordIndex(p); err == nil {
			t.Errorf("accepted %s", p)
		}
	}
}

func registryTestRoot(value string) (string, error) {
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("MUSICXML_PLAN_REGISTRY must be an absolute path; go test runs in internal/validationplan")
	}
	return value, nil
}
func TestRegistryTestRoot(t *testing.T) {
	if _, err := registryTestRoot("../registry"); err == nil {
		t.Fatal("relative registry path was accepted")
	}
	if _, err := registryTestRoot(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
