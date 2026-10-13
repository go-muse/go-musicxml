package musicxml

import (
	"os"
	"strings"
	"testing"
)

func TestRaceCIRecordsBoundedResourceMeasurements(t *testing.T) {
	workflow, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(workflow), "\r\n", "\n")
	_, race, ok := strings.Cut(text, "\n  race:\n")
	if !ok {
		t.Fatal("missing race job")
	}
	race, _, ok = strings.Cut(race, "\n  xsd-conformance:\n")
	if !ok {
		t.Fatal("missing race job boundary")
	}
	for _, required := range []string{
		"runs-on: ubuntu-latest", "timeout-minutes: 20",
		"nproc", "/proc/meminfo",
		"/usr/bin/time -v bash scripts/check-all.sh race",
	} {
		if !strings.Contains(race, required) {
			t.Errorf("race job omits %q", required)
		}
	}
	if strings.Count(race, "bash scripts/check-all.sh race") != 1 ||
		strings.Contains(race, "|| true") || strings.Contains(race, "continue-on-error") {
		t.Fatal("race command must execute once and preserve failure status")
	}
	script, err := os.ReadFile("scripts/check-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `"$check_go" test -race -count=1 -timeout=15m ./...`) {
		t.Fatal("resource measurement must retain the complete race command")
	}
}
