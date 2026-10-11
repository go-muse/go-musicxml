package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The inventory is a compiled traceability artifact, not an executable profile.
func TestMusicXML40InventoryPinnedCountsAndRelationships(t *testing.T) {
	inv := MusicXML40Inventory()
	if inv.Version != "musicxml-validation-inventory-1" || inv.RegistryCommit != "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c" {
		t.Fatalf("unexpected inventory identity: %s / %s", inv.Version, inv.RegistryCommit)
	}
	if len(inv.Records) != 2560 || len(inv.Instances) != 2218 || len(inv.Checks) != 418 || len(inv.Capabilities) != 313 || len(inv.Requirements) != 95 || len(inv.Tests) != 513 || len(inv.Contexts) != 291 {
		t.Fatalf("inventory cardinalities changed: records=%d instances=%d checks=%d capabilities=%d requirements=%d tests=%d contexts=%d", len(inv.Records), len(inv.Instances), len(inv.Checks), len(inv.Capabilities), len(inv.Requirements), len(inv.Tests), len(inv.Contexts))
	}
	recordIDs, instanceIDs, checkIDs, capIDs, reqIDs, testIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	unique := func(ids map[string]bool, id string) {
		t.Helper()
		if id == "" || ids[id] {
			t.Errorf("empty or duplicate identity %q", id)
		}
		ids[id] = true
	}
	for _, r := range inv.Records {
		unique(recordIDs, r.ResearchID)
	}
	for _, r := range inv.Instances {
		unique(instanceIDs, r.ID)
		if r.RuntimeSupport != "unsupported" {
			t.Errorf("instance %s claims runtime support %q", r.ID, r.RuntimeSupport)
		}
	}
	statuses := map[string]int{}
	for _, r := range inv.Checks {
		unique(checkIDs, r.ID)
		statuses[r.Status]++
		if r.RuntimeSupport != "unsupported" {
			t.Errorf("check %s claims runtime support %q", r.ID, r.RuntimeSupport)
		}
	}
	for _, r := range inv.Capabilities {
		unique(capIDs, r.ID)
		if r.ProviderStatus != "unbound" {
			t.Errorf("capability %s was incorrectly bound", r.ID)
		}
	}
	for _, r := range inv.Requirements {
		unique(reqIDs, r.ID)
	}
	testStatuses := map[string]int{}
	for _, r := range inv.Tests {
		unique(testIDs, r.ID)
		testStatuses[r.Status]++
		if r.Status != "planned" && r.Status != "decision_required" {
			t.Errorf("test %s claims execution", r.ID)
		}
	}
	if !reflect.DeepEqual(testStatuses, map[string]int{"planned": 499, "decision_required": 14}) {
		t.Errorf("test contract statuses: %v", testStatuses)
	}
	if !reflect.DeepEqual(statuses, map[string]int{"planned": 281, "decision_required": 76, "retained_advisory": 59, "excluded": 2}) {
		t.Errorf("check statuses: %v", statuses)
	}
	links := func(owner string, ids []string, sets ...map[string]bool) {
		t.Helper()
		for _, id := range ids {
			found := false
			for _, set := range sets {
				found = found || set[id]
			}
			if !found {
				t.Errorf("%s: unresolved link %s", owner, id)
			}
		}
	}
	registryBase := inv.RegistryRepository + "/blob/" + inv.RegistryCommit + "/docs/validation/registry/catalog.json#"
	for _, r := range inv.Records {
		links(r.ResearchID, r.CheckIDs, checkIDs, instanceIDs, reqIDs)
		if r.RegistryURL != registryBase+r.RegistryPointer || !strings.HasPrefix(r.RegistryPointer, "/records/") {
			t.Errorf("%s: lost exact pinned provenance", r.ResearchID)
		}
	}
	for _, r := range inv.Instances {
		links(r.ID, []string{r.ResearchID}, recordIDs)
		links(r.ID, []string{r.TemplateID}, checkIDs)
		links(r.ID, r.ReuseInstances, instanceIDs)
		links(r.ID, r.PlanDependencies.SchemaComponent, instanceIDs)
		links(r.ID, r.PlanDependencies.LanguageSemantics, instanceIDs)
		if _, ok := inv.TargetPolicies[r.TargetPolicy]; !ok {
			t.Errorf("%s: unresolved target policy %s", r.ID, r.TargetPolicy)
		}
	}
	for _, r := range inv.Checks {
		links(r.ID, r.ResearchIDs, recordIDs)
		links(r.ID, r.CapabilityIDs, capIDs)
		links(r.ID, r.Dependencies, checkIDs)
		links(r.ID, r.TestContracts, testIDs)
		for key := range r.DependencyModes {
			links(r.ID, []string{key}, checkIDs)
		}
		if len(r.NormativeProvenance) == 0 {
			t.Errorf("%s has no normative sources", r.ID)
		}
	}
	for _, r := range inv.Capabilities {
		links(r.ID, r.CheckIDs, checkIDs)
		links(r.ID, r.ProviderRequirementIDs, reqIDs)
	}
	for _, r := range inv.Requirements {
		links(r.ID, r.Dependencies, reqIDs)
		links(r.ID, r.TestContracts, testIDs)
		links(r.ID, r.ResearchIDs, recordIDs)
	}
	for _, r := range inv.Tests {
		links(r.ID, r.CheckIDs, checkIDs)
		links(r.ID, r.RequirementIDs, reqIDs)
	}
	for _, r := range inv.Contexts {
		links(r.ResearchID, []string{r.ResearchID}, recordIDs)
	}
}

func TestMusicXML40InventoryExhaustivePlanningProjection(t *testing.T) {
	inv := MusicXML40Inventory()
	bytes, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(bytes, &got); err != nil {
		t.Fatal(err)
	}
	// Every retained field is compared for every row against the independent source.
	groups := []struct {
		file, source, target, id string
		fields                   []string
	}{
		{"research-map.json", "research_records", "records", "research_id", strings.Fields("research_id origin role disposition check_ids registry_pointer")},
		{"xsd-contracts.json", "instances", "instances", "id", strings.Fields("id revision research_id template_id role registry_pointer composition status stage reuse_instances diagnostic_policy plan_dependencies target_policy")},
		{"xsd-contracts.json", "checks", "checks", "id", inventoryCheckFields()},
		{"prose-contracts.json", "checks", "checks", "id", inventoryCheckFields()},
		{"context-dispositions.json", "checks", "checks", "id", inventoryCheckFields()},
		{"capabilities.json", "capability_gates", "capabilities", "id", strings.Fields("id label kind provider_status provider_requirement_ids check_ids targets closure missing_outcome")},
		{"implementation-plan.json", "requirements", "requirements", "id", strings.Fields("id title role inputs applicability targets dependencies normative_provenance test_contracts stage status acceptance source_bullets source_inventory research_ids")},
		{"context-dispositions.json", "dispositions", "contexts", "research_id", strings.Fields("research_id role disposition rationale produces dependencies closure translation")},
	}
	for _, f := range []string{"xsd-test-contracts.json", "prose-contracts.json", "context-dispositions.json", "implementation-plan.json"} {
		groups = append(groups, struct {
			file, source, target, id string
			fields                   []string
		}{f, "test_contracts", "tests", "id", strings.Fields("id check_ids requirement_ids targets status axis_status case_instantiation_gate")})
	}
	for _, g := range groups {
		src := inventoryPlan(t, g.file)
		indexed := map[string]map[string]any{}
		for _, r := range got[g.target].([]any) {
			m := r.(map[string]any)
			indexed[m[g.id].(string)] = m
		}
		for _, r := range src[g.source].([]any) {
			want := r.(map[string]any)
			id := want[g.id].(string)
			actual, ok := indexed[id]
			if !ok {
				t.Errorf("missing %s %s", g.target, id)
				continue
			}
			for _, f := range g.fields {
				w, wok := want[f]
				a, aok := actual[f]
				if !wok && !aok {
					continue
				}
				if !reflect.DeepEqual(a, w) {
					t.Errorf("%s.%s: got %#v, want %#v", id, f, a, w)
				}
			}
		}
	}
	xsd := inventoryPlan(t, "xsd-contracts.json")
	for _, key := range []string{"target_policies", "dependency_semantics"} {
		if !reflect.DeepEqual(got[key], xsd[key]) {
			t.Errorf("lost exact %s", key)
		}
	}
	snapshot := inventoryPlan(t, "external-snapshot.json")
	if !reflect.DeepEqual(got["external_files"], snapshot["files"]) {
		t.Error("external file provenance differs")
	}
}

func inventoryCheckFields() []string {
	return strings.Fields("id research_ids revision role inputs selector targets capabilities dependencies dependency_modes normative_provenance test_contracts stage status closure readiness_gate capability_ids semantic_status implementation_binding model_limit model_evidence expected_outcomes research_clauses")
}

func inventoryPlan(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "docs", "validation", "plan", name))
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err = json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestMusicXML40InventoryDigestsAndReproducibleOrdering(t *testing.T) {
	inv := MusicXML40Inventory()
	b, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err = json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	delete(d, "sha256")
	b, err = json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if inv.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("payload digest mismatch: %s", inv.SHA256)
	}
	paths, _ := filepath.Glob(filepath.Join("..", "docs", "validation", "plan", "*.json"))
	var manifest strings.Builder
	if len(inv.SourceFiles) != len(paths) {
		t.Fatalf("source file count %d, want %d", len(inv.SourceFiles), len(paths))
	}
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		hexsum := hex.EncodeToString(hash[:])
		name := filepath.Base(path)
		if inv.SourceFiles[i].Path != name || inv.SourceFiles[i].SHA256 != hexsum || inv.SourceFiles[i].Bytes != int64(len(data)) {
			t.Errorf("source manifest mismatch %s", name)
		}
		manifest.WriteString(name + "\x00" + hexsum + "\n")
	}
	sum = sha256.Sum256([]byte(manifest.String()))
	if inv.SourceSHA256 != hex.EncodeToString(sum[:]) {
		t.Error("source set digest mismatch")
	}
	for _, key := range []string{"records", "instances", "checks", "capabilities", "requirements", "tests", "contexts", "overlaps", "advisory_reviews", "stages", "questions"} {
		idkey := "id"
		if key == "records" || key == "contexts" || key == "advisory_reviews" {
			idkey = "research_id"
		}
		var ids []string
		for _, r := range d[key].([]any) {
			ids = append(ids, r.(map[string]any)[idkey].(string))
		}
		if !sort.StringsAreSorted(ids) {
			t.Errorf("%s has nondeterministic order", key)
		}
	}
}

func TestMusicXML40InventoryOwnsAllNestedData(t *testing.T) {
	pristine := MusicXML40Inventory()
	victim := MusicXML40Inventory()
	mutateInventory(reflect.ValueOf(&victim).Elem())
	if reflect.DeepEqual(victim, pristine) {
		t.Fatal("mutation did not exercise inventory")
	}
	if fresh := MusicXML40Inventory(); !reflect.DeepEqual(fresh, pristine) {
		t.Fatal("caller mutation corrupted compiled inventory")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			v := MusicXML40Inventory()
			mutateInventory(reflect.ValueOf(&v).Elem())
			if !reflect.DeepEqual(MusicXML40Inventory(), pristine) {
				t.Error("concurrent caller mutation leaked")
			}
		})
	}
	wg.Wait()
}

func mutateInventory(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			mutateInventory(v.Field(i))
		}
	case reflect.Pointer:
		if !v.IsNil() {
			mutateInventory(v.Elem())
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			mutateInventory(v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			v.SetMapIndex(key, reflect.Zero(v.Type().Elem()))
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString("mutated")
		}
	case reflect.Int, reflect.Int64:
		if v.CanSet() {
			v.SetInt(-1)
		}
	case reflect.Bool:
		if v.CanSet() {
			v.SetBool(!v.Bool())
		}
	}
}

func TestMusicXML40InventoryRemainingPlanningRelations(t *testing.T) {
	inv := MusicXML40Inventory()
	if len(inv.Overlaps) != 342 || len(inv.AdvisoryReviews) != 59 || len(inv.Stages) != 20 || len(inv.Questions) != 28 {
		t.Fatalf("planning relation counts: %d/%d/%d/%d", len(inv.Overlaps), len(inv.AdvisoryReviews), len(inv.Stages), len(inv.Questions))
	}
	b, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	groups := []struct {
		file, source, target, id string
		fields                   []string
	}{
		{"prose-contracts.json", "overlap_reviews", "overlaps", "id", strings.Fields("id research_id kind shared_condition additional_condition xsd_research_ids disposition justification additional_condition_check_ids additional_condition_source")},
		{"context-dispositions.json", "overlap_reviews", "overlaps", "id", strings.Fields("id research_id kind shared_condition additional_condition xsd_research_ids disposition justification additional_condition_check_ids additional_condition_source")},
		{"advisory-clause-review.json", "advisory_clause_reviews", "advisory_reviews", "research_id", strings.Fields("research_id decision related_research_ids related_xsd_research_ids rationale normative_provenance reuse_check_ids primary_check_ids reuse_scope")},
		{"implementation-plan.json", "stages", "stages", "id", strings.Fields("id title depends_on acceptance dependency_interpretation scope_note")},
		{"open-questions.json", "open_questions", "questions", "id", strings.Fields("id registry_pointer description disposition proposed_interpretation hard_error_basis requirements contextual_research_ids closure")},
	}
	for _, g := range groups {
		src := inventoryPlan(t, g.file)
		indexed := map[string]map[string]any{}
		for _, r := range got[g.target].([]any) {
			m := r.(map[string]any)
			indexed[m[g.id].(string)] = m
		}
		for _, r := range src[g.source].([]any) {
			want := r.(map[string]any)
			id := want[g.id].(string)
			actual, ok := indexed[id]
			if !ok {
				t.Errorf("missing %s %s", g.target, id)
				continue
			}
			for _, f := range g.fields {
				if !reflect.DeepEqual(actual[f], want[f]) {
					t.Errorf("%s.%s: got %#v, want %#v", id, f, actual[f], want[f])
				}
			}
		}
	}
}

func TestMusicXML40InventoryCompoundAndStageLinks(t *testing.T) {
	inv := MusicXML40Inventory()
	records, checks, requirements, stages := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	contracts := map[string]bool{}
	for _, r := range inv.Checks {
		contracts[r.ID] = true
	}
	for _, r := range inv.Instances {
		contracts[r.ID] = true
	}
	for _, r := range inv.Requirements {
		contracts[r.ID] = true
	}
	for _, r := range inv.Records {
		records[r.ResearchID] = true
	}
	for _, r := range inv.Checks {
		checks[r.ID] = true
	}
	for _, r := range inv.Requirements {
		requirements[r.ID] = true
	}
	for _, r := range inv.Stages {
		stages[r.ID] = true
	}
	links := func(owner string, ids []string, table map[string]bool) {
		t.Helper()
		for _, id := range ids {
			if !table[id] {
				t.Errorf("%s: unresolved relationship %s", owner, id)
			}
		}
	}
	for _, r := range inv.Overlaps {
		links(r.ID, []string{r.ResearchID}, records)
		links(r.ID, r.XSDResearchIDs, records)
		links(r.ID, r.AdditionalConditionCheckIDs, checks)
	}
	for _, r := range inv.AdvisoryReviews {
		links(r.ResearchID, []string{r.ResearchID}, records)
		links(r.ResearchID, r.RelatedResearchIDs, records)
		links(r.ResearchID, r.RelatedXSDResearchIDs, records)
		links(r.ResearchID, r.ReuseCheckIDs, contracts)
		links(r.ResearchID, r.PrimaryCheckIDs, contracts)
	}
	for _, r := range inv.Stages {
		links(r.ID, r.DependsOn, stages)
	}
	for _, r := range inv.Checks {
		links(r.ID, []string{r.Stage}, stages)
		for id := range r.ResearchClauses {
			links(r.ID, []string{id}, records)
		}
	}
	for _, r := range inv.Instances {
		links(r.ID, []string{r.Stage}, stages)
		if r.RegistryURL != inv.RegistryRepository+"/blob/"+inv.RegistryCommit+"/docs/validation/registry/catalog.json#"+r.RegistryPointer {
			t.Errorf("%s: lost registry provenance", r.ID)
		}
	}
	for _, r := range inv.Requirements {
		links(r.ID, []string{r.Stage}, stages)
	}
	for _, r := range inv.Questions {
		links(r.ID, r.Requirements, requirements)
		links(r.ID, r.ContextualResearchIDs, records)
		if r.RegistryURL != inv.RegistryRepository+"/blob/"+inv.RegistryCommit+"/docs/validation/registry/issues.json#"+r.RegistryPointer {
			t.Errorf("%s: lost issue provenance", r.ID)
		}
	}
}

func TestInventoryExpectedOutcomesJSONContract(t *testing.T) {
	for _, input := range []string{`"context stays separate"`, `{"unknown":"missing instance evidence","unsupported":"no provider"}`} {
		var p InventoryExpectedOutcomes
		if err := json.Unmarshal([]byte(input), &p); err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != input {
			t.Errorf("outcome representation: got %s, want %s", got, input)
		}
	}
	for _, input := range []string{`12`, `[]`, `{"unsupported":false}`} {
		var p InventoryExpectedOutcomes
		if err := json.Unmarshal([]byte(input), &p); err == nil {
			t.Errorf("accepted malformed outcome representation %s", input)
		}
	}
	if _, err := json.Marshal(InventoryExpectedOutcomes{Description: "text", Outcomes: map[string]string{"fail": "false"}}); err == nil {
		t.Error("ambiguous outcome representation accepted")
	}
}
