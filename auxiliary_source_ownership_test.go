package musicxml

import (
	"encoding/json"
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
			ID         string              `json:"research_id"`
			Instance   string              `json:"instance_id"`
			Assembly   string              `json:"schema_assembly"`
			Locator    string              `json:"source_xpath"`
			Occurrence string              `json:"scope_occurrence_id"`
			Source     string              `json:"source_url"`
			Axes       map[string][]string `json:"fixture_axes"`
			Parameters json.RawMessage     `json:"parameters"`
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
		for _, axis := range []string{"positive", "negative", "boundary", "missing_fact", "interaction"} {
			if len(row.Axes[axis]) == 0 {
				t.Errorf("%s: missing %s axis", row.ID, axis)
			}
			for _, name := range row.Axes[axis] {
				if axis == "missing_fact" {
					if name != "TestAuxiliarySourceUnavailableEvidence" {
						t.Errorf("unbound missing-fact test %s", name)
					}
				} else if !cases[name] {
					t.Errorf("unbound source case %s for %s", name, row.ID)
				}
			}
		}
	}
	if counts["container.xsd"] != 11 || counts["sounds.xsd"] != 25 {
		t.Fatalf("wrong assembly inventory: %v", counts)
	}
}
