// Command validationplan checks documentation planning data. It does not validate
// MusicXML input, implement a runtime rule engine, or expose a public API.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type object map[string]any

type plan struct {
	planDir                                                                                                   string
	docs                                                                                                      map[string]object
	checks, instances, tests, requirements, stages, research, dispositions, questions, deferred, capabilities map[string]object
}

func main() {
	dir := flag.String("plan", "docs/validation/plan", "planning artifact directory")
	registry := flag.String("registry", "", "optional local pinned registry root (never fetched)")
	flag.Parse()
	p, err := load(*dir)
	if err == nil {
		err = p.validate()
	}
	if err == nil && *registry != "" {
		err = p.verifyRegistry(*registry)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "validation plan:", err)
		os.Exit(1)
	}
	fmt.Printf("planning integrity passed: %d research dispositions, %d XSD instances, %d check contracts, %d test contracts, %d deferred items; no product-conformance claim\n", len(p.research), len(p.instances), len(p.checks), len(p.tests), len(p.deferred))
	if *registry == "" {
		fmt.Println("external snapshot bytes not checked (use -registry for the pinned local checkout)")
	}
}

func load(dir string) (*plan, error) {
	p := &plan{planDir: dir, docs: map[string]object{}, checks: map[string]object{}, instances: map[string]object{}, tests: map[string]object{}, requirements: map[string]object{}, stages: map[string]object{}, research: map[string]object{}, dispositions: map[string]object{}, questions: map[string]object{}, deferred: map[string]object{}, capabilities: map[string]object{}}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no planning files at %s", dir)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if err = uniqueJSON(b); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		var d object
		if err = json.Unmarshal(b, &d); err != nil {
			return nil, err
		}
		if str(d, "format") != "musicxml-validation-plan-1" {
			return nil, fmt.Errorf("%s: unknown format", file)
		}
		p.docs[filepath.Base(file)] = d
		for _, set := range []struct {
			field, key string
			target     map[string]object
		}{
			{"checks", "id", p.checks}, {"instances", "id", p.instances}, {"test_contracts", "id", p.tests}, {"requirements", "id", p.requirements}, {"stages", "id", p.stages}, {"research_records", "research_id", p.research}, {"dispositions", "research_id", p.dispositions}, {"open_questions", "id", p.questions}, {"deferred_items", "id", p.deferred}, {"capability_gates", "id", p.capabilities},
		} {
			v, ok := d[set.field]
			if !ok {
				continue
			}
			rows, ok := v.([]any)
			if !ok {
				return nil, fmt.Errorf("%s must be an array", set.field)
			}
			for _, row := range rows {
				m, ok := row.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("%s row is not an object", set.field)
				}
				id := str(m, set.key)
				if id == "" {
					return nil, fmt.Errorf("%s has missing %s", set.field, set.key)
				}
				if _, ok = set.target[id]; ok {
					return nil, fmt.Errorf("duplicate %s %s", set.field, id)
				}
				set.target[id] = m
			}
		}
	}
	return p, nil
}

func str(o object, key string) string { s, _ := o[key].(string); return s }
func stringsAt(o object, key string) []string {
	xs, _ := o[key].([]any)
	out := []string{}
	for _, v := range xs {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
func has(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func nonempty(o object, keys ...string) error {
	for _, k := range keys {
		v, ok := o[k]
		if !ok || v == nil {
			return fmt.Errorf("missing %s", k)
		}
		switch x := v.(type) {
		case string:
			if strings.TrimSpace(x) == "" {
				return fmt.Errorf("empty %s", k)
			}
		case []any:
			if len(x) == 0 {
				return fmt.Errorf("empty %s", k)
			}
		}
	}
	return nil
}
func links(o object, key string, to map[string]object, required bool) error {
	xs := stringsAt(o, key)
	if required && len(xs) == 0 {
		return fmt.Errorf("missing %s links", key)
	}
	seen := map[string]bool{}
	for _, id := range xs {
		if _, ok := to[id]; !ok {
			return fmt.Errorf("unresolved %s: %s", key, id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate %s link %s", key, id)
		}
		seen[id] = true
	}
	return nil
}
func allMaps(ms ...map[string]object) map[string]object {
	out := map[string]object{}
	for _, m := range ms {
		for id, v := range m {
			out[id] = v
		}
	}
	return out
}
func withID(id string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", id, err)
}

func (p *plan) validate() error {
	for _, d := range p.docs {
		if err := stringArrays(d); err != nil {
			return err
		}
	}
	snapshot, ok := p.docs["external-snapshot.json"]
	if !ok {
		return fmt.Errorf("missing external-snapshot.json")
	}
	if str(snapshot, "registry_commit") != "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c" {
		return fmt.Errorf("unexpected registry pin; review snapshot migration explicitly")
	}
	if err := p.validateCounts(snapshot); err != nil {
		return err
	}

	ids := make([]string, 0, len(p.research))
	for id := range p.research {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if digest([]byte(strings.Join(ids, "\n")+"\n")) != str(snapshot, "research_ids_sha256") {
		return fmt.Errorf("research identity set does not match pinned inventory")
	}
	if len(p.checks) == 0 || len(p.tests) == 0 || len(p.stages) == 0 || len(p.requirements) == 0 {
		return fmt.Errorf("empty contract/stage/requirement set")
	}
	globalIDs := map[string]bool{}
	for _, m := range []map[string]object{p.checks, p.instances, p.tests, p.requirements, p.stages, p.questions, p.deferred, p.capabilities} {
		for id := range m {
			if globalIDs[id] {
				return fmt.Errorf("cross-kind duplicate ID %s", id)
			}
			globalIDs[id] = true
		}
	}
	all := allMaps(p.checks, p.instances, p.requirements)
	if len(all) != len(p.checks)+len(p.instances)+len(p.requirements) {
		return fmt.Errorf("cross-kind duplicate contract ID")
	}
	roles := []string{"predicate", "context_fact", "advisory", "out_of_scope"}
	targets := []string{"source", "model", "package", "catalog"}
	checkUsed := map[string]bool{}
	testUsed := map[string]bool{}
	requirementUsed := map[string]bool{}
	for id, r := range p.research {
		if !has(roles, str(r, "role")) {
			return fmt.Errorf("%s invalid role", id)
		}
		if err := nonempty(r, "registry_pointer", "origin", "disposition", "check_ids"); err != nil {
			return withID(id, err)
		}
		if err := links(r, "check_ids", all, true); err != nil {
			return withID(id, err)
		}
		for _, ref := range stringsAt(r, "check_ids") {
			checkUsed[ref] = true
			if c, ok := p.checks[ref]; ok && !has(stringsAt(c, "research_ids"), id) {
				return fmt.Errorf("%s asymmetric check research link %s", id, ref)
			}
		}
	}
	for id, c := range p.checks {
		if _, ok := c["research_ids"]; ok {
			if err := links(c, "research_ids", p.research, true); err != nil {
				return withID(id, err)
			}
			for _, rid := range stringsAt(c, "research_ids") {
				if !has(stringsAt(p.research[rid], "check_ids"), id) {
					return fmt.Errorf("%s asymmetric research link %s", id, rid)
				}
			}
		}
		if err := links(c, "capability_ids", p.capabilities, true); err != nil {
			return withID(id, err)
		}
		for _, cap := range stringsAt(c, "capability_ids") {
			if !has(stringsAt(p.capabilities[cap], "check_ids"), id) {
				return fmt.Errorf("%s asymmetric capability %s", id, cap)
			}
		}
		labels := []string{}
		for _, capID := range stringsAt(c, "capability_ids") {
			labels = append(labels, str(p.capabilities[capID], "label"))
		}
		if !sameStrings(labels, stringsAt(c, "capabilities")) {
			return fmt.Errorf("%s capability labels disagree with IDs", id)
		}

		if !has([]string{"planned", "decision_required", "optional", "retained_advisory", "excluded"}, str(c, "status")) {
			return fmt.Errorf("%s invalid check status", id)
		}
		if str(c, "status") == "decision_required" {
			if err := nonempty(c, "closure"); err != nil {
				return withID(id, err)
			}
		}
		if !has(roles, str(c, "role")) {
			return fmt.Errorf("%s invalid role", id)
		}
		if outcomes, ok := c["expected_outcomes"].(map[string]any); ok {
			if len(outcomes) != 6 {
				return fmt.Errorf("%s invalid outcome vocabulary", id)
			}
			for _, outcome := range []string{"pass", "fail", "not-applicable", "unknown", "unsupported", "blocked"} {
				if str(outcomes, outcome) == "" {
					return fmt.Errorf("%s missing outcome %s", id, outcome)
				}
			}
		}
		if c["revision"] != float64(1) {
			return fmt.Errorf("%s invalid revision", id)
		}
		if err := nonempty(c, "inputs", "selector", "applicability", "semantics", "targets", "normative_provenance", "test_contracts", "status", "stage"); err != nil {
			return withID(id, err)
		}
		for _, t := range stringsAt(c, "targets") {
			if !has(targets, t) {
				return fmt.Errorf("%s invalid target %s", id, t)
			}
		}
		if _, ok := p.stages[str(c, "stage")]; !ok {
			return fmt.Errorf("%s unresolved stage", id)
		}
		if has(stringsAt(c, "dependencies"), "CHECK-XSD-LANG-LEXICAL") {
			modes, ok := c["dependency_modes"].(map[string]any)
			if !ok || modes["CHECK-XSD-LANG-LEXICAL"] == nil {
				return fmt.Errorf("%s lacks target-specific lexical prerequisite mode", id)
			}
		}
		if err := links(c, "dependencies", all, false); err != nil {
			return withID(id, err)
		}
		if err := links(c, "test_contracts", p.tests, true); err != nil {
			return withID(id, err)
		}
		for _, t := range stringsAt(c, "test_contracts") {
			testUsed[t] = true
			if !has(stringsAt(p.tests[t], "check_ids"), id) {
				return fmt.Errorf("%s test %s missing reverse check link", id, t)
			}
		}
	}
	policies, _ := p.docs["xsd-contracts.json"]["target_policies"].(map[string]any)
	for id, i := range p.instances {
		if i["revision"] != float64(1) || str(i, "status") != "planned" {
			return fmt.Errorf("%s invalid instance revision/status", id)
		}
		if _, ok := p.stages[str(i, "stage")]; !ok {
			return fmt.Errorf("%s invalid instance stage", id)
		}
		if str(i, "registry_pointer") != str(p.research[str(i, "research_id")], "registry_pointer") {
			return fmt.Errorf("%s pointer mismatch", id)
		}
		graph, ok := i["plan_dependencies"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s missing dependency graph", id)
		}
		for _, kind := range []string{"schema_component", "language_semantics"} {
			if _, ok := graph[kind]; !ok {
				return fmt.Errorf("%s missing %s graph", id, kind)
			}
			if err := links(graph, kind, p.instances, false); err != nil {
				return withID(id, err)
			}
		}
		if _, ok := policies[str(i, "target_policy")]; !ok {
			return fmt.Errorf("%s missing target policy", id)
		}

		t := str(i, "template_id")
		if _, ok := p.checks[t]; !ok {
			return fmt.Errorf("%s unresolved template", id)
		}
		checkUsed[t] = true
		rid := str(i, "research_id")
		r, ok := p.research[rid]
		if !ok || !has(stringsAt(r, "check_ids"), id) {
			return fmt.Errorf("%s orphan instance/research link", id)
		}
		if !has(roles, str(i, "role")) || str(i, "role") != str(p.checks[t], "role") {
			return fmt.Errorf("%s inconsistent role", id)
		}
		if err := links(i, "reuse_instances", p.instances, false); err != nil {
			return withID(id, err)
		}
		if str(i, "composition") == "aggregate_reuse_view" && str(i, "diagnostic_policy") == "" {
			return fmt.Errorf("%s missing aggregate dedup policy", id)
		}
		if str(i, "template_id") == "CHECK-XSD-simple_value_domain" && (len(stringsAt(i, "reuse_instances")) != 1 || str(i, "diagnostic_policy") != "reuse_origin_only") {
			return fmt.Errorf("%s missing exact origin reuse", id)
		}
		if str(i, "template_id") == "CHECK-XSD-complex_content_contract" && str(i, "diagnostic_policy") != "reuse_leaves_keep_closedness_and_text" {
			return fmt.Errorf("%s discarded aggregate residual", id)
		}
	}
	for id, r := range p.requirements {
		for _, target := range stringsAt(r, "targets") {
			if !has(targets, target) {
				return fmt.Errorf("%s invalid requirement target %s", id, target)
			}
		}
		if !has([]string{"planned", "decision_required", "optional"}, str(r, "status")) {
			return fmt.Errorf("%s invalid requirement status", id)
		}
		if err := nonempty(r, "title", "inputs", "applicability", "targets", "normative_provenance", "test_contracts", "stage", "status", "acceptance"); err != nil {
			return withID(id, err)
		}
		if !has(roles, str(r, "role")) {
			return fmt.Errorf("%s invalid role", id)
		}
		if _, ok := p.stages[str(r, "stage")]; !ok {
			return fmt.Errorf("%s unresolved stage", id)
		}
		if err := links(r, "dependencies", all, false); err != nil {
			return withID(id, err)
		}
		if err := links(r, "test_contracts", p.tests, true); err != nil {
			return withID(id, err)
		}
		for _, t := range stringsAt(r, "test_contracts") {
			testUsed[t] = true
			if !has(stringsAt(p.tests[t], "requirement_ids"), id) {
				return fmt.Errorf("%s test %s missing reverse requirement link", id, t)
			}
		}
	}
	for id, t := range p.tests {
		for _, target := range stringsAt(t, "targets") {
			if !has(targets, target) {
				return fmt.Errorf("%s invalid test target %s", id, target)
			}
		}
		for _, contractID := range append(stringsAt(t, "check_ids"), stringsAt(t, "requirement_ids")...) {
			for _, target := range stringsAt(t, "targets") {
				if !has(stringsAt(all[contractID], "targets"), target) {
					return fmt.Errorf("%s target %s exceeds contract %s", id, target, contractID)
				}
			}
		}
		axisStatus, ok := t["axis_status"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s missing axis status", id)
		}
		for _, axis := range []string{"positive", "negative", "boundary", "missing_fact", "interaction"} {
			status := str(axisStatus, axis)
			if !has([]string{"generic", "parameterized", "record_specific"}, status) {
				return fmt.Errorf("%s invalid %s axis status", id, axis)
			}
			if status != "record_specific" && str(t, "case_instantiation_gate") == "" {
				return fmt.Errorf("%s missing case instantiation gate", id)
			}
		}

		if !has([]string{"planned", "decision_required"}, str(t, "status")) {
			return fmt.Errorf("%s test contract is not an executed test", id)
		}
		for _, c := range stringsAt(t, "check_ids") {
			if !has(stringsAt(p.checks[c], "test_contracts"), id) {
				return fmt.Errorf("%s asymmetric check link %s", id, c)
			}
		}
		for _, r := range stringsAt(t, "requirement_ids") {
			if !has(stringsAt(p.requirements[r], "test_contracts"), id) {
				return fmt.Errorf("%s asymmetric requirement link %s", id, r)
			}
		}
		if err := nonempty(t, "targets", "oracle", "status", "cases"); err != nil {
			return withID(id, err)
		}
		cases, ok := t["cases"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s cases is not an object", id)
		}
		for _, axis := range []string{"positive", "negative", "boundary", "missing_fact", "interaction"} {
			if len(stringsAt(cases, axis)) == 0 {
				return fmt.Errorf("%s missing %s case axis", id, axis)
			}
		}
		if len(stringsAt(t, "check_ids"))+len(stringsAt(t, "requirement_ids")) == 0 {
			return fmt.Errorf("%s no contract link", id)
		}
		if err := links(t, "check_ids", p.checks, false); err != nil {
			return withID(id, err)
		}
		if err := links(t, "requirement_ids", p.requirements, false); err != nil {
			return withID(id, err)
		}
		if !testUsed[id] {
			return fmt.Errorf("orphan test contract %s", id)
		}
	}
	for id, d := range p.deferred {
		valid := false
		for n := 1; n <= p.inventoryCount("deferred_items"); n++ {
			if id == fmt.Sprintf("D%02d", n) {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("unknown deferred item %s", id)
		}
		if err := nonempty(d, "title", "source_section", "closure"); err != nil {
			return withID(id, err)
		}
		if err := links(d, "requirements", p.requirements, true); err != nil {
			return withID(id, err)
		}
		for _, r := range stringsAt(d, "requirements") {
			requirementUsed[r] = true
		}
	}
	for id := range p.requirements {
		if !requirementUsed[id] {
			return fmt.Errorf("orphan deferred requirement %s", id)
		}
	}
	for id := range p.checks {
		if !checkUsed[id] {
			return fmt.Errorf("orphan check contract %s", id)
		}
	}
	for id, s := range p.stages {
		if err := nonempty(s, "title", "acceptance"); err != nil {
			return withID(id, err)
		}
		if err := links(s, "depends_on", p.stages, false); err != nil {
			return withID(id, err)
		}
	}
	if err := acyclic(p.stages, "depends_on"); err != nil {
		return err
	}
	if err := acyclic(all, "dependencies"); err != nil {
		return err
	}
	for id, d := range p.dispositions {
		if _, ok := p.research[id]; !ok {
			return fmt.Errorf("orphan prose disposition %s", id)
		}
		if str(d, "role") != str(p.research[id], "role") {
			return fmt.Errorf("%s role mismatch", id)
		}
		if err := nonempty(d, "rationale", "closure", "translation", "disposition"); err != nil {
			return withID(id, err)
		}
		if str(d, "translation") != "pending" {
			return fmt.Errorf("%s translation status changed without pin review", id)
		}
	}
	for id, q := range p.questions {
		if q["hard_error_basis"] != false || str(q, "disposition") != "decision_required" {
			return fmt.Errorf("%s issue converted into unreviewed authority", id)
		}
		if err := nonempty(q, "registry_pointer", "disposition", "closure", "requirements"); err != nil {
			return withID(id, err)
		}
		if err := links(q, "requirements", p.requirements, true); err != nil {
			return withID(id, err)
		}
		if err := links(q, "contextual_research_ids", p.research, false); err != nil {
			return withID(id, err)
		}
	}

	for id, c := range p.capabilities {
		if err := nonempty(c, "label", "kind", "provider_status", "closure", "missing_outcome", "targets"); err != nil {
			return withID(id, err)
		}
		if str(c, "provider_status") != "unbound" {
			return fmt.Errorf("%s makes an unverified provider claim", id)
		}
		if err := links(c, "provider_requirement_ids", p.requirements, true); err != nil {
			return withID(id, err)
		}
		if err := links(c, "check_ids", p.checks, true); err != nil {
			return withID(id, err)
		}
		for _, check := range stringsAt(c, "check_ids") {
			if !has(stringsAt(p.checks[check], "capability_ids"), id) {
				return fmt.Errorf("%s asymmetric check %s", id, check)
			}
		}
	}
	advisories := rows(p.docs["advisory-clause-review.json"], "advisory_clause_reviews")
	if len(advisories) != p.inventoryCount("advisory_reviews") {
		return fmt.Errorf("incomplete compound-advisory review")
	}
	seenAdvisory := map[string]bool{}
	for _, a := range advisories {
		id := str(a, "research_id")
		if seenAdvisory[id] || str(p.research[id], "role") != "advisory" {
			return fmt.Errorf("invalid advisory review %s", id)
		}
		seenAdvisory[id] = true
		if err := links(a, "primary_check_ids", all, true); err != nil {
			return withID(id, err)
		}
		if err := links(a, "reuse_check_ids", all, false); err != nil {
			return withID(id, err)
		}
		if err := links(a, "related_research_ids", p.research, false); err != nil {
			return withID(id, err)
		}
		if err := links(a, "related_xsd_research_ids", p.research, false); err != nil {
			return withID(id, err)
		}
	}
	if err := p.validateSharedRecipes(); err != nil {
		return err
	}
	return p.validateTraceability(snapshot)
}

func (p *plan) validateTraceability(snapshot object) error {
	// Independently pinned identity sets prevent same-count substitutions and silent loss.
	if snapshot["executable_product_predicates"] != float64(0) || snapshot["product_tests_added"] != float64(0) || snapshot["full_normative_closure"] != false {
		return fmt.Errorf("planning metadata claims unimplemented product coverage")
	}
	order := []string{}
	for id, r := range p.research {
		order = append(order, str(r, "registry_pointer")+"|"+id+"|"+str(r, "origin"))
	}
	sort.Strings(order)
	if digest([]byte(strings.Join(order, "\n")+"\n")) != str(snapshot, "research_mapping_sha256") {
		return fmt.Errorf("pinned research mapping mismatch")
	}
	edges := []string{}
	for id, i := range p.instances {
		graph, _ := i["plan_dependencies"].(map[string]any)
		for _, kind := range []string{"schema_component", "language_semantics"} {
			for _, target := range stringsAt(graph, kind) {
				edges = append(edges, id+"|"+kind+"|"+target)
			}
		}
	}
	sort.Strings(edges)
	if digest([]byte(strings.Join(edges, "\n")+"\n")) != str(snapshot, "xsd_dependency_edges_sha256") {
		return fmt.Errorf("pinned XSD dependency graph mismatch")
	}

	for _, s := range []struct {
		key string
		ids []string
	}{{"issue_ids_sha256", keys(p.questions)}, {"translation_ids_sha256", keys(p.dispositions)}} {
		sort.Strings(s.ids)
		if digest([]byte(strings.Join(s.ids, "\n")+"\n")) != str(snapshot, s.key) {
			return fmt.Errorf("%s mismatch", s.key)
		}
	}
	covered := map[string]bool{}
	for _, r := range p.requirements {
		for _, s := range stringsAt(r, "source_bullets") {
			if !has(stringsAt(snapshot, "deferred_source_bullets"), s) {
				return fmt.Errorf("unexpected deferred source key %s", s)
			}
			covered[s] = true
		}
	}
	for _, s := range stringsAt(snapshot, "deferred_source_bullets") {
		if !covered[s] {
			return fmt.Errorf("unmapped deferred source bullet %s", s)
		}
	}
	for _, id := range stringsAt(snapshot, "candidate_prose_ids") {
		if _, ok := p.research[id]; !ok {
			return fmt.Errorf("missing candidate %s", id)
		}
		refs := stringsAt(p.research[id], "check_ids")
		if len(refs) == 0 {
			return fmt.Errorf("unmapped candidate %s", id)
		}
	}
	if err := p.validateOverlaps(); err != nil {
		return err
	}
	if err := p.validateDeferredInventory(snapshot); err != nil {
		return err
	}

	return nil
}
func keys(m map[string]object) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
func rows(o object, key string) []object {
	out := []object{}
	xs, _ := o[key].([]any)
	for _, v := range xs {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func acyclic(items map[string]object, key string) error {
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("dependency cycle at %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, next := range stringsAt(items[id], key) {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range items {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func (p *plan) verifyRegistry(root string) error {
	snapshot := p.docs["external-snapshot.json"]
	documents := map[string]object{}
	for _, f := range rows(snapshot, "files") {
		path := str(f, "path")
		if filepath.IsAbs(path) || strings.Contains(path, "..") {
			return fmt.Errorf("invalid snapshot path %q", path)
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if digest(b) != str(f, "sha256") {
			return fmt.Errorf("external snapshot mismatch: %s", path)
		}
		var d object
		if err = json.Unmarshal(b, &d); err != nil {
			return err
		}
		documents[filepath.Base(path)] = d
	}
	cat := documents["catalog.json"]
	raw, _ := cat["records"].([]any)
	if len(raw) != len(p.research) {
		return fmt.Errorf("external research count mismatch")
	}
	for id, r := range p.research {
		index, err := recordIndex(str(r, "registry_pointer"))
		if err != nil || index < 0 || index >= len(raw) {
			return fmt.Errorf("%s invalid external pointer", id)
		}
		ext, ok := raw[index].(map[string]any)
		if !ok || str(ext, "id") != id || str(ext, "registry_origin") != str(r, "origin") {
			return fmt.Errorf("%s external pointer resolves to wrong record", id)
		}
		for _, ref := range stringsAt(r, "check_ids") {
			if inst, ok := p.instances[ref]; ok && str(inst, "registry_pointer") != str(r, "registry_pointer") {
				return fmt.Errorf("%s inconsistent instance pointer", id)
			}
		}
	}
	// Independently derive all structured parameter and semantic edges from verified source bytes.
	occurrenceMap := map[string][]string{}
	for _, m := range rows(documents["occurrence-requirement-map.json"], "mapping") {
		occurrenceMap[str(m, "occurrence_id")] = stringsAt(m, "requirement_ids")
	}
	if len(occurrenceMap) != p.inventoryCount("occurrence_mappings") {
		return fmt.Errorf("incomplete external occurrence map")
	}
	for _, item := range raw {
		ext, err := externalRecord(item)
		if err != nil {
			return err
		}
		if str(ext, "registry_origin") != "XSD" {
			continue
		}
		r, ok := ext["record"].(map[string]any)
		if !ok {
			return fmt.Errorf("external XSD record %s lacks record object", str(ext, "id"))
		}
		rid := str(ext, "id")
		instance := p.instances["INSTANCE-"+rid]
		expected := map[string]map[string]bool{"schema_component": {}, "language_semantics": {}}
		for _, ref := range structuredReferences(r["parameters"]) {
			targets := occurrenceMap[ref]
			if strings.HasPrefix(ref, "XSD10-TYPE-") {
				targets = []string{"MX40-REQ-LANG-TYPE-" + strings.TrimPrefix(ref, "XSD10-TYPE-")}
			}
			if len(targets) == 0 {
				return fmt.Errorf("unresolved external parameter reference %s", ref)
			}
			for _, target := range targets {
				if target != rid {
					expected["schema_component"]["INSTANCE-"+target] = true
				}
			}
		}
		for _, sem := range stringsAt(r, "semantic_rule_ids") {
			target := "MX40-REQ-LANG-" + strings.TrimPrefix(sem, "XSD10-SEM-")
			if target != rid {
				expected["language_semantics"]["INSTANCE-"+target] = true
			}
		}
		graph, _ := instance["plan_dependencies"].(map[string]any)
		for kind, want := range expected {
			got := stringsAt(graph, kind)
			if len(got) != len(want) {
				return fmt.Errorf("%s external %s edge mismatch", rid, kind)
			}
			for _, g := range got {
				if !want[g] {
					return fmt.Errorf("%s extra external %s edge %s", rid, kind, g)
				}
			}
		}
	}
	// External issue and translation sets are also compared by identity, not counts alone.
	extIssues := rows(documents["issues.json"], "issues")
	if len(extIssues) != len(p.questions) {
		return fmt.Errorf("external issue count mismatch")
	}
	for idx, q := range extIssues {
		id := str(q, "id")
		local, ok := p.questions[id]
		if !ok || str(local, "registry_pointer") != "/issues/"+strconv.Itoa(idx) {
			return fmt.Errorf("unresolved issue pointer %s", id)
		}
	}
	overlaps := map[string]object{}
	for _, d := range p.docs {
		for _, r := range rows(d, "overlap_reviews") {
			id := str(r, "research_id")
			if _, ok := overlaps[id]; ok {
				return fmt.Errorf("duplicate overlap %s", id)
			}
			overlaps[id] = r
		}
	}
	if len(overlaps) != p.inventoryCount("overlap_reviews") {
		return fmt.Errorf("incomplete prose overlap inventory")
	}
	for _, rel := range rows(documents["reconciliation.json"], "relations") {
		id := str(rel, "prose_id")
		local, ok := overlaps[id]
		if !ok || str(local, "kind") != str(rel, "overlap") {
			return fmt.Errorf("%s overlap kind mismatch", id)
		}
		a, b := stringsAt(local, "xsd_research_ids"), stringsAt(rel, "related_xsd_contract_ids")
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, "\n") != strings.Join(b, "\n") {
			return fmt.Errorf("%s overlap link mismatch", id)
		}
		if _, ok := p.research[id]; !ok {
			return fmt.Errorf("external overlap orphan %s", id)
		}
		for _, x := range stringsAt(rel, "related_xsd_contract_ids") {
			if _, ok := p.research[x]; !ok {
				return fmt.Errorf("external related contract orphan %s", x)
			}
		}
	}
	return nil
}
func recordIndex(pointer string) (int, error) {
	if !strings.HasPrefix(pointer, "/records/") || !strings.HasSuffix(pointer, "/record") {
		return -1, fmt.Errorf("invalid pointer")
	}
	s := strings.TrimSuffix(strings.TrimPrefix(pointer, "/records/"), "/record")
	return strconv.Atoi(s)
}

// encoding/json otherwise silently accepts duplicate object keys. Planning data
// must reject them before decoding, including nested cases and metadata.
func uniqueJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	var value func() error
	value = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				s, ok := k.(string)
				if !ok {
					return fmt.Errorf("non-string key")
				}
				if seen[s] {
					return fmt.Errorf("duplicate JSON key %s", s)
				}
				seen[s] = true
				if err = value(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case json.Delim('['):
			for d.More() {
				if err = value(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

// Check array shape at ingress; links must not silently ignore corrupt members.
func stringArrays(value any) error {
	switch v := value.(type) {
	case object:
		return stringArrays(map[string]any(v))
	case map[string]any:
		known := map[string]bool{}
		for _, k := range []string{"inputs", "targets", "capabilities", "capability_ids", "provider_requirement_ids", "dependencies", "normative_provenance", "test_contracts", "check_ids", "requirement_ids", "research_ids", "requirements", "source_bullets", "depends_on", "acceptance", "reuse_instances", "contextual_research_ids", "xsd_research_ids", "schema_component", "language_semantics", "candidate_prose_ids", "deferred_source_bullets", "primary_check_ids", "reuse_check_ids", "related_research_ids", "related_xsd_research_ids", "additional_condition_check_ids", "positive", "negative", "boundary", "missing_fact", "interaction"} {
			known[k] = true
		}
		for key, item := range v {
			_, document := v["format"]
			collection := document && (key == "test_contracts" || key == "requirements")
			_, identified := v["id"]
			_, research := v["research_id"]
			_, cases := v["positive"].([]any)
			_, graph := v["schema_component"]
			structured := document || identified || research || cases || graph
			if known[key] && !collection && structured {
				xs, ok := item.([]any)
				if !ok {
					return fmt.Errorf("%s must be a string array", key)
				}
				for _, x := range xs {
					s, ok := x.(string)
					if !ok || strings.TrimSpace(s) == "" {
						return fmt.Errorf("%s contains a blank/non-string member", key)
					}
				}
			}
			if err := stringArrays(item); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := stringArrays(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func structuredReferences(v any) []string {
	out := []string{}
	switch x := v.(type) {
	case map[string]any:
		for _, child := range x {
			out = append(out, structuredReferences(child)...)
		}
	case []any:
		for _, child := range x {
			out = append(out, structuredReferences(child)...)
		}
	case string:
		if strings.HasPrefix(x, "MX40-XSD-") || strings.HasPrefix(x, "XSD10-TYPE-") {
			out = append(out, x)
		}
	}
	return out
}
