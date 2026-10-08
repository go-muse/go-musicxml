package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func overlap(t *testing.T, p *plan, id string) object {
	t.Helper()
	for _, d := range p.docs {
		for _, r := range rows(d, "overlap_reviews") {
			if str(r, "research_id") == "MX40-PROSE-"+id {
				return r
			}
		}
	}
	t.Fatalf("missing overlap %s", id)
	return nil
}
func TestCopiedOverlapClausesRejected(t *testing.T) {
	pairs := map[string]string{
		"fermata-upright": "one-note-visual-tie", "empty-fermata-normal": "horizontal-turn-slash",
		"transpose-additive": "senza-misura", "bend-release-pair": "beam-fan-conditions",
		"figure-number-numeric": "figured-bass-prefix-open", "lyric-syllable-structure": "listen-distinct-targets",
		"normal-type-when-different": "time-modification-cumulative", "part-group-boundaries": "group-symbol-default",
		"diatonic-plus-octave": "except-voice", "opus-link-target": "opus-version-default",
		"opus-score-target": "opus-version-default", "tablature-rhythm-required": "no-default-tempo",
	}
	for own, other := range pairs {
		t.Run(own, func(t *testing.T) {
			p := testPlan(t)
			overlap(t, p, own)["additional_condition"] = overlap(t, p, other)["additional_condition"]
			if err := p.validate(); err == nil {
				t.Fatal("accepted another record's clause")
			}
		})
	}
}
func TestReviewCorruptionsRejected(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*plan)
	}{
		{"legacy_not_applicable_outcome", func(p *plan) {
			p.checks["CHECK-XSD-schema_namespace_contract"]["expected_outcomes"].(map[string]any)["not_applicable"] = "legacy term"
		}},
		{"legacy_violation_outcome", func(p *plan) {
			p.checks["CHECK-XSD-schema_namespace_contract"]["expected_outcomes"].(map[string]any)["violation"] = "legacy term"
		}},

		{"duplicate_overlap", func(p *plan) {
			d := p.docs["context-dispositions.json"]
			d["overlap_reviews"] = append(d["overlap_reviews"].([]any), d["overlap_reviews"].([]any)[0])
		}},
		{"same_count_duplicate_overlap", func(p *plan) {
			d := p.docs["context-dispositions.json"]
			xs := d["overlap_reviews"].([]any)
			xs[1] = xs[0]
		}},
		{"missing_overlap", func(p *plan) {
			d := p.docs["context-dispositions.json"]
			d["overlap_reviews"] = d["overlap_reviews"].([]any)[1:]
		}},
		{"weakened_clause_binding", func(p *plan) { overlap(t, p, "fermata-upright")["additional_condition_source"] = "reviewed_overlap" }},
		{"wrong_own_check_binding", func(p *plan) {
			overlap(t, p, "fermata-upright")["additional_condition_check_ids"] = []any{"CHECK-context-transposition-additive"}
		}},
		{"capability_label_drift", func(p *plan) { p.checks["CHECK-accordion-at-least-one"]["capabilities"] = []any{"made-up-label"} }},
		{"test_target_exceeds_check", func(p *plan) { p.tests["TEST-accordion-at-least-one"]["targets"] = []any{"package"} }},
		{"test_target_exceeds_requirement", func(p *plan) { p.tests["TEST-DEF-XML-ATTR"]["targets"] = []any{"model"} }},
		{"stale_research_count", func(p *plan) { p.docs["external-snapshot.json"]["research_count"] = float64(2559) }},
		{"stale_count_dimension", func(p *plan) {
			p.docs["external-snapshot.json"]["inventory_counts"].(map[string]any)["overlap_reviews"] = float64(343)
		}},
		{"missing_axis_specificity", func(p *plan) { delete(p.tests["TEST-context-fractional-divisions"], "axis_status") }},
		{"false_axis_coverage", func(p *plan) {
			p.tests["TEST-context-fractional-divisions"]["axis_status"].(map[string]any)["interaction"] = "covered"
		}},
		{"missing_generic_instantiation_gate", func(p *plan) { delete(p.tests["TEST-context-fractional-divisions"], "case_instantiation_gate") }},
		{"unpublished_source_link", func(p *plan) { p.requirements["REQ-DEF-XML-ATTR"]["source_inventory"] = "unpublished.md" }},
		{"wrong_source_section", func(p *plan) { p.deferred["D01"]["source_section"] = "../deferred-inventory.md#missing" }},
		{"wrong_published_source_hash", func(p *plan) {
			p.docs["external-snapshot.json"]["deferred_inventory"].(map[string]any)["sha256"] = "wrong"
		}},
		{"stale_shared_clause_allowlist", func(p *plan) {
			a := rows(p.docs["overlap-policy.json"], "shared_additional_conditions")[0]
			a["research_ids"] = []any{"MX40-PROSE-fermata-upright", "MX40-PROSE-empty-fermata-normal"}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := testPlan(t)
			c.mutate(p)
			if err := p.validate(); err == nil {
				t.Fatal("review corruption accepted")
			}
		})
	}
}
func TestPublishedSourceHeadingCoverage(t *testing.T) {
	p := testPlan(t)
	b, err := os.ReadFile(filepath.Join(p.planDir, "..", "deferred-inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.Replace(string(b), "### DEF01-01", "### RENAMED", 1))
	dir := t.TempDir()
	p.planDir = filepath.Join(dir, "plan")
	if err := os.Mkdir(p.planDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deferred-inventory.md"), b, 0600); err != nil {
		t.Fatal(err)
	}
	p.docs["external-snapshot.json"]["deferred_inventory"].(map[string]any)["sha256"] = digest(b)
	if err := p.validate(); err == nil {
		t.Fatal("accepted a missing published source key after hash update")
	}
}
func TestExternalRecordShape(t *testing.T) {
	for _, v := range []any{nil, "row", []any{}, map[string]any{"id": "x", "registry_origin": "XSD"}, map[string]any{"id": "x", "registry_origin": "XSD", "record": "not an object"}} {
		if _, err := externalRecord(v); err == nil {
			t.Errorf("accepted malformed row %#v", v)
		}
	}
	if _, err := externalRecord(map[string]any{"id": "x", "registry_origin": "XSD", "record": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
}
