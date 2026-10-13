package musicxml

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"
)

// This verifies provenance/fixture integrity, not a percentage or proof that
// every transitive external language contract has been independently closed.
func TestAuxiliarySourceOwnershipMap(t *testing.T) {
	var mapData struct {
		Baseline string `json:"baseline_commit"`
		Rows     []struct {
			ID              string              `json:"research_id"`
			Instance        string              `json:"instance_id"`
			Assembly        string              `json:"schema_assembly"`
			Locator         string              `json:"source_xpath"`
			Occurrence      string              `json:"scope_occurrence_id"`
			Source          string              `json:"source_url"`
			Axes            map[string][]string `json:"fixture_axes"`
			Parameters      json.RawMessage     `json:"parameters"`
			MissingFactTest string              `json:"missing_fact_test"`
		} `json:"xsd_instances"`
		Context []struct {
			ID string `json:"research_id"`
		} `json:"sounds_context"`
	}
	data, err := os.ReadFile("docs/validation/auxiliary-source-ownership.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &mapData); err != nil {
		t.Fatal(err)
	}
	if mapData.Baseline != "3d3d936fa4ba250083da03651cbfb4bbbe936914" || len(mapData.Rows) != 36 || len(mapData.Context) != 4 {
		t.Fatal("scope inventory changed")
	}
	var plan struct {
		Instances []struct {
			ID       string `json:"id"`
			Research string `json:"research_id"`
		} `json:"instances"`
	}
	data, err = os.ReadFile("docs/validation/plan/xsd-contracts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	original := map[string]string{}
	for _, v := range plan.Instances {
		original[v.ID] = v.Research
	}
	cases := map[string]bool{}
	for _, tc := range auxiliarySourceCases() {
		if cases[tc.name] {
			t.Fatalf("duplicate fixture: %s", tc.name)
		}
		cases[tc.name] = true
	}
	seen, counts := map[string]bool{}, map[string]int{}
	for _, row := range mapData.Rows {
		if seen[row.ID] || original[row.Instance] != row.ID {
			t.Fatalf("invalid instance identity: %+v", row)
		}
		seen[row.ID] = true
		counts[row.Assembly]++
		if row.Locator == "" || row.Occurrence == "" || len(row.Parameters) == 0 || !strings.Contains(row.Source, "799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/"+row.Assembly) {
			t.Fatalf("source binding incomplete: %s", row.ID)
		}
		if err := validateAuxiliaryFixtureReferences(row.Assembly, row.Axes, row.MissingFactTest); err != nil {
			t.Errorf("%s: %v", row.ID, err)
		}

	}
	if counts["container.xsd"] != 11 || counts["sounds.xsd"] != 25 {
		t.Fatalf("wrong assembly inventory: %v", counts)
	}
}

// Fixture labels and test-function references occupy different schema fields.
func validateAuxiliaryFixtureReferences(assembly string, axes map[string][]string, missingFactTest string) error {
	if len(axes) != 4 {
		return fmt.Errorf("want four fixture axes, got %d", len(axes))
	}
	// This compile-time reference also prevents a renamed test becoming a dead route.
	_ = TestAuxiliarySourceUnavailableEvidence
	if missingFactTest != "TestAuxiliarySourceUnavailableEvidence" {
		return fmt.Errorf("unbound missing-fact test %q", missingFactTest)
	}
	cases := map[string]string{}
	for _, tc := range auxiliarySourceCases() {
		cases[tc.name] = tc.assembly + ".xsd"
	}
	for _, axis := range []string{"positive", "negative", "boundary", "interaction"} {
		if len(axes[axis]) == 0 {
			return fmt.Errorf("missing %s fixture axis", axis)
		}
		for _, name := range axes[axis] {
			if cases[name] != assembly {
				return fmt.Errorf("unbound or wrong-assembly source case %q for %s", name, assembly)
			}
		}
	}
	return nil
}

func TestAuxiliaryFixtureReferenceTypes(t *testing.T) {
	base := map[string][]string{
		"positive": {"container minimal"}, "negative": {"container missing rootfiles"},
		"boundary": {"container multiple token values"}, "interaction": {"container wrong root"},
	}
	const missing = "TestAuxiliarySourceUnavailableEvidence"
	if err := validateAuxiliaryFixtureReferences("container.xsd", base, missing); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"unknown fixture", "missing axis", "wrong assembly", "function as fixture", "unknown function", "missing function", "extra axis"} {
		t.Run(mutation, func(t *testing.T) {
			axes, function := maps.Clone(base), missing
			switch mutation {
			case "unknown fixture":
				axes["positive"] = []string{"unknown fixture"}
			case "missing axis":
				delete(axes, "positive")
			case "wrong assembly":
				axes["positive"] = []string{"sounds empty mapping"}
			case "function as fixture":
				axes["positive"] = []string{missing}
			case "unknown function":
				function = "TestUnknown"
			case "missing function":
				function = ""
			case "extra axis":
				axes["missing_fact"] = []string{missing}
			}
			if err := validateAuxiliaryFixtureReferences("container.xsd", axes, function); err == nil {
				t.Fatal("invalid typed fixture route accepted")
			}
		})
	}
}
