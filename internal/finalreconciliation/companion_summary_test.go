package finalreconciliation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const companionTitle = "# Final requirement reconciliation: baseline, PR 42 and PR 46 evidence"
const companionSummaryStart = "<!-- evidence-summary:start -->"
const companionSummaryEnd = "<!-- evidence-summary:end -->"

// Keep the review-facing inventory readable while deriving its counts and
// nonbaseline route list from the same rows as the machine-readable summary.
// This checks a bounded summary, not arbitrary prose or runtime conformance.
func companionSummary(ledger map[string]any) string {
	s := ledger["evidence_summary"].(map[string]any)
	layers := s["routes_by_evidence_layer"].(map[string]any)
	axes := s["axis_dispositions"].(map[string]any)
	classes := s["requirement_classifications"].(map[string]any)
	var out strings.Builder
	fmt.Fprintln(&out, companionSummaryStart)
	fmt.Fprintln(&out, "## Evidence summary")
	fmt.Fprintf(&out, "\nAnchored routes: %.0f total; %.0f `baseline`, %.0f `pr42-added`, and\n%.0f `pr46-assessed-loading`.\n", s["anchored_routes"], layers["baseline"], layers["pr42-added"], layers["pr46-assessed-loading"])
	fmt.Fprintf(&out, "\nRequirement classifications: %.0f implemented-awaiting-current-verification,\n%.0f explicit-bounded-limitation, %.0f unresolved-decision, and\n%.0f optional-not-elected.\n", classes["implemented-awaiting-current-verification"], classes["explicit-bounded-limitation"], classes["unresolved-decision"], classes["optional-not-elected"])
	fmt.Fprintf(&out, "\nAxis dispositions: %.0f mapped, %.0f partial, %.0f decision-only,\n%.0f not-elected, and %.0f missing-evidence.\n", axes["mapped"], axes["partial"], axes["decision-only"], axes["not-elected"], axes["missing-evidence"])
	for _, layer := range []string{"pr42-added", "pr46-assessed-loading"} {
		fmt.Fprintf(&out, "\n### %s routes\n\n", layer)
		var routes []string
		for _, raw := range ledger["cases"].([]any) {
			c := raw.(map[string]any)
			if c["evidence_layer"] != layer {
				continue
			}
			line := fmt.Sprintf("- `%s`", c["id"])
			if prior := stringField(c, "prior_evidence_layer"); prior != "" {
				line += fmt.Sprintf(" (prior layer: `%s`)", prior)
			}
			routes = append(routes, line)
		}
		sort.Strings(routes)
		for _, line := range routes {
			fmt.Fprintln(&out, line)
		}
	}
	fmt.Fprintln(&out, "\n### Superseded opt-in axes")
	for _, raw := range ledger["test_contracts"].([]any) {
		c := raw.(map[string]any)
		if c["id"] != "TEST-DEF-ADAPT-OPTIN" {
			continue
		}
		for _, name := range []string{"boundary", "missing_fact"} {
			a := c["axes"].(map[string]any)[name].(map[string]any)
			fmt.Fprintf(&out, "\n- `TEST-DEF-ADAPT-OPTIN/%s`: `%s`,\n  with `%s` gap.\n", name, a["assessment"], a["gap_kind"])
		}
	}
	fmt.Fprintln(&out, companionSummaryEnd)
	return out.String()
}

func validateCompanionSummary(ledger map[string]any, text string) error {
	if strings.Count(text, companionTitle+"\n") != 1 {
		return fmt.Errorf("companion title must name the baseline, PR 42 and PR 46 evidence layers")
	}
	if strings.Count(text, companionSummaryStart) != 1 || strings.Count(text, companionSummaryEnd) != 1 || !strings.Contains(text, companionSummary(ledger)) {
		return fmt.Errorf("companion evidence summary must match ledger counts, route layers and superseded axes")
	}
	return nil
}

func TestFinalReconciliationCompanionSummary(t *testing.T) {
	root, ledger := loadLedger(t)
	data, err := os.ReadFile(filepath.Join(root, "docs/validation/final-requirement-reconciliation.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCompanionSummary(ledger, string(data)); err != nil {
		t.Fatal(err)
	}
}

func TestFinalReconciliationRejectsStaleCompanionSummary(t *testing.T) {
	_, ledger := loadLedger(t)
	good := companionTitle + "\n\n" + companionSummary(ledger)
	if err := validateCompanionSummary(ledger, good); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, from, to string }{
		{"title", "PR 42 and PR 46 evidence", "PR 42 evidence"},
		{"route count", "127 total", "126 total"},
		{"layer", "`pr46-assessed-loading`", "`baseline`"},
		{"axis count", "89 mapped", "91 mapped"},
		{"route list", "- `TestSourceTruncatedMarkupPublic`", "- `TestOmittedMigratedRoute`"},
		{"superseded status", "`decision-only`,", "`mapped`,"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.Replace(good, tc.from, tc.to, 1)
			if bad == good {
				t.Fatal("mutation did not change the summary")
			}
			if err := validateCompanionSummary(ledger, bad); err == nil {
				t.Fatal("stale companion summary accepted")
			}
		})
	}
}
