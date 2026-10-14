package musicxml

import (
	"os"
	"strings"
	"testing"
)

func TestRaceCIRecordsBoundedResourceMeasurements(t *testing.T) {
	jobs := raceCIJobs(t)
	for _, name := range []string{"race-root", "race-packages"} {
		race, ok := jobs[name]
		if !ok {
			t.Fatalf("missing %s job", name)
		}
		for _, required := range []string{
			"runs-on: ubuntu-latest", "timeout-minutes: 20",
			"nproc", "/proc/meminfo",
			"/usr/bin/time -v bash scripts/check-all.sh race",
		} {
			if !strings.Contains(race, required) {
				t.Errorf("%s job omits %q", name, required)
			}
		}
		if strings.Count(race, "bash scripts/check-all.sh race") != 1 ||
			strings.Contains(race, "|| true") || strings.Contains(race, "continue-on-error") {
			t.Fatalf("%s command must execute once and preserve failure status", name)
		}
	}
	script, err := os.ReadFile("scripts/check-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `"$check_go" test -race -count=1 -timeout=15m ./...`) {
		t.Fatal("resource measurement must retain the complete race command")
	}
}
