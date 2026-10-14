package musicxml

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// These tests execute the real check script against a fake Go executable. They
// check the complete argv, discovery failures, and partition, without rerunning
// the expensive race suite inside an ordinary unit-test invocation.
type raceShardFixture struct {
	goPath, log, root, inventory string
}

func newRaceShardFixture(t *testing.T) raceShardFixture {
	t.Helper()
	dir := t.TempDir()
	f := raceShardFixture{
		goPath: filepath.Join(dir, "fake go"), log: filepath.Join(dir, "calls"),
		root: filepath.Join(dir, "root"), inventory: filepath.Join(dir, "inventory"),
	}
	fake := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\0' "$@" >> "$FAKE_LOG"
printf '\0' >> "$FAKE_LOG"
case "$1" in
    list)
        if [[ "$#" != 3 || "$2" != -race ]]; then exit 90; fi
        case "$3" in
            .) cat "$FAKE_ROOT"; exit "$FAKE_ROOT_STATUS" ;;
            ./...) cat "$FAKE_INVENTORY"; exit "$FAKE_INVENTORY_STATUS" ;;
            *) exit 91 ;;
        esac ;;
    test) exit "$FAKE_TEST_STATUS" ;;
    *) exit 92 ;;
esac
`
	if err := os.WriteFile(f.goPath, []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f raceShardFixture) run(t *testing.T, shard *string, root, inventory string, rootStatus, inventoryStatus, testStatus int) (int, [][]string, string) {
	t.Helper()
	for path, data := range map[string]string{f.root: root, f.inventory: inventory, f.log: ""} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", "scripts/check-all.sh", "race")
	cmd.Env = raceTestEnv(map[string]string{
		"GO": filepath.ToSlash(f.goPath), "FAKE_LOG": filepath.ToSlash(f.log),
		"FAKE_ROOT": filepath.ToSlash(f.root), "FAKE_INVENTORY": filepath.ToSlash(f.inventory),
		"FAKE_ROOT_STATUS": fmt.Sprint(rootStatus), "FAKE_INVENTORY_STATUS": fmt.Sprint(inventoryStatus),
		"FAKE_TEST_STATUS": fmt.Sprint(testStatus),
	}, "RACE_SHARD")
	if shard != nil {
		cmd.Env = append(cmd.Env, "RACE_SHARD="+*shard)
	}
	output, err := cmd.CombinedOutput()
	status := raceTestExit(t, err)
	raw, err := os.ReadFile(f.log)
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	for _, call := range bytes.Split(raw, []byte{0, 0}) {
		if len(call) != 0 {
			calls = append(calls, strings.Split(string(call), "\x00"))
		}
	}
	return status, calls, string(output)
}

func raceTestEnv(values map[string]string, unset ...string) []string {
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := values[key]; replace {
			continue
		}
		remove := false
		for _, keyToRemove := range unset {
			remove = remove || key == keyToRemove
		}
		if !remove {
			env = append(env, entry)
		}
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

func raceTestExit(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	t.Fatal(err)
	return -1
}

func TestRaceShardDefaultCommandIsUnchanged(t *testing.T) {
	f := newRaceShardFixture(t)
	for _, testStatus := range []int{0, 37} {
		status, calls, output := f.run(t, nil, "", "", 99, 99, testStatus)
		want := [][]string{{"test", "-race", "-count=1", "-timeout=15m", "./..."}}
		if status != testStatus || !reflect.DeepEqual(calls, want) {
			t.Fatalf("status=%d calls=%q output=%s; want status=%d calls=%q", status, calls, output, testStatus, want)
		}
	}
}

func TestRaceShardsAreAnExhaustiveDisjointPartition(t *testing.T) {
	const root = "example.org/musicxml"
	// Inventories can grow command, internal, nested, test-only, and race-only
	// packages. No package-name allowlist may silently exclude future entries.
	packages := []string{root + "/cmd/new-command", root + "/internal/nested/new", root,
		root + "/testonly", root + "/raceonly", root + "/música"}
	f := newRaceShardFixture(t)
	selected := make(map[string]string)
	for _, newline := range []string{"\n", ""} {
		for _, shard := range []string{"root", "packages"} {
			status, calls, output := f.run(t, &shard, root+newline, strings.Join(packages, "\n")+newline, 0, 0, 0)
			wantPackages := []string{root}
			if shard == "packages" {
				wantPackages = append(append([]string{}, packages[:2]...), packages[3:]...)
			}
			want := [][]string{{"list", "-race", "."}, {"list", "-race", "./..."},
				append([]string{"test", "-race", "-count=1", "-timeout=15m"}, wantPackages...)}
			if status != 0 || !reflect.DeepEqual(calls, want) {
				t.Fatalf("%s: status=%d calls=%q output=%s; want %q", shard, status, calls, output, want)
			}
			if newline == "\n" {
				for _, pkg := range calls[2][4:] {
					if previous := selected[pkg]; previous != "" {
						t.Fatalf("package %q occurs in both %s and %s", pkg, previous, shard)
					}
					selected[pkg] = shard
				}
			}
		}
	}
	if len(selected) != len(packages) {
		t.Fatalf("selected %q, want complete inventory %q", selected, packages)
	}
}

func TestRaceShardRejectsInvalidSelectionBeforeDiscovery(t *testing.T) {
	f := newRaceShardFixture(t)
	for _, shard := range []string{"", "all", "ROOT", "packages root", "root\n", "*"} {
		status, calls, output := f.run(t, &shard, "example/root\n", "example/root\nexample/pkg\n", 0, 0, 0)
		if status != 2 || len(calls) != 0 {
			t.Errorf("shard %q: status=%d calls=%q output=%s", shard, status, calls, output)
		}
	}
}

func TestRaceShardRejectsMalformedInventories(t *testing.T) {
	f := newRaceShardFixture(t)
	for _, tc := range []struct{ name, root, inventory string }{
		{"empty root", "", "example/root\nexample/pkg\n"},
		{"multiple roots", "example/root\nexample/other\n", "example/root\nexample/pkg\n"},
		{"blank root suffix", "example/root\n\n", "example/root\nexample/pkg\n"},
		{"root whitespace", "example/root bad\n", "example/root bad\nexample/pkg\n"},
		{"root option", "-run=none\n", "-run=none\nexample/pkg\n"},
		{"empty inventory", "example/root\n", ""},
		{"missing root", "example/root\n", "example/root/child\nexample/pkg\n"},
		{"duplicate root", "example/root\n", "example/root\nexample/pkg\nexample/root\n"},
		{"duplicate package", "example/root\n", "example/root\nexample/pkg\nexample/pkg\n"},
		{"blank line", "example/root\n", "example/root\n\nexample/pkg\n"},
		{"blank suffix", "example/root\n", "example/root\nexample/pkg\n\n"},
		{"space", "example/root\n", "example/root\nexample/pkg bad\n"},
		{"tab", "example/root\n", "example/root\nexample/pkg\tbad\n"},
		{"carriage return", "example/root\n", "example/root\nexample/pkg\r\n"},
		{"option", "example/root\n", "example/root\n-run=none\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, shard := range []string{"root", "packages"} {
				status, calls, output := f.run(t, &shard, tc.root, tc.inventory, 0, 0, 0)
				if status != 2 {
					t.Fatalf("%s: malformed inventory did not fail with status 2: calls=%q output=%s", shard, calls, output)
				}
				for _, call := range calls {
					if call[0] == "test" {
						t.Fatalf("%s: malformed inventory started tests: %q", shard, calls)
					}
				}
			}
		})
	}
	shard := "packages"
	status, calls, output := f.run(t, &shard, "example/root\n", "example/root\n", 0, 0, 0)
	if status != 2 || len(calls) != 2 {
		t.Fatalf("empty shard: status=%d calls=%q output=%s", status, calls, output)
	}
}

func TestRaceShardPreservesFailureStatus(t *testing.T) {
	f := newRaceShardFixture(t)
	for _, shard := range []string{"root", "packages"} {
		for _, tc := range []struct{ root, inventory, test, status, calls int }{
			{23, 0, 0, 23, 1}, {0, 29, 0, 29, 2}, {0, 0, 37, 37, 3},
		} {
			status, calls, output := f.run(t, &shard, "example/root\n", "example/root\nexample/pkg\n", tc.root, tc.inventory, tc.test)
			if status != tc.status || len(calls) != tc.calls {
				t.Fatalf("%s: status=%d calls=%q output=%s; want status=%d calls=%d", shard, status, calls, output, tc.status, tc.calls)
			}
		}
	}
}

func TestRaceShardNeverEvaluatesDiscoveredPaths(t *testing.T) {
	f := newRaceShardFixture(t)
	// The shell expansions would create a marker in the working directory if
	// discovery output were ever evaluated instead of passed as quoted argv.
	marker := filepath.ToSlash(filepath.Join(t.TempDir(), "executed"))
	payload := "example/$(touch${IFS}" + strings.ReplaceAll(marker, " ", "${IFS}") + ")"
	shard := "packages"
	status, calls, output := f.run(t, &shard, "example/root\n", "example/root\n"+payload+"\nexample/*;echo\n", 0, 0, 0)
	want := []string{"test", "-race", "-count=1", "-timeout=15m", payload, "example/*;echo"}
	if status != 0 || len(calls) != 3 || !reflect.DeepEqual(calls[2], want) {
		t.Fatalf("status=%d calls=%q output=%s; want final argv=%q", status, calls, output, want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("discovered path was evaluated: marker stat = %v", err)
	}
}

// The workflow intentionally uses explicit jobs. Exact block guards catch
// added conditions, matrices, error suppression, extra actions or cache keys
// without adding a YAML parser dependency to this module.
func raceCIJobs(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	_, text, ok := strings.Cut(text, "\njobs:\n")
	if !ok {
		t.Fatal("missing workflow jobs mapping")
	}
	matches := regexp.MustCompile(`(?m)^  ([a-z-]+):\n`).FindAllStringSubmatchIndex(text, -1)
	jobs := make(map[string]string)
	for i, match := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		jobs[text[match[2]:match[3]]] = strings.TrimRight(text[match[1]:end], "\n") + "\n"
	}
	return jobs
}

func raceCIShardBlock(shard string) string {
	return `    name: Race detector (` + shard + `)
    runs-on: ubuntu-latest
    timeout-minutes: 20

    steps:
      - name: Check out repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1

      - name: Set up Go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: true
          cache-dependency-path: go.sum

      - name: Run tests with race detector
        shell: bash
        env:
          RACE_SHARD: ` + shard + `
        run: |
          printf 'Available CPUs: '
          nproc
          awk '/^MemTotal:/ { print }' /proc/meminfo
          /usr/bin/time -v bash scripts/check-all.sh race
`
}

func TestRaceCIShardsPreserveRunnerActionsAndBudgets(t *testing.T) {
	jobs := raceCIJobs(t)
	for _, shard := range []string{"root", "packages"} {
		id := "race-" + shard
		job, ok := jobs[id]
		if !ok {
			t.Fatalf("missing explicit shard job %s", id)
		}
		if job != raceCIShardBlock(shard) {
			t.Fatalf("%s must retain independent standard runner, pinned actions/cache, execution-scoped shard, measurement and finite budgets; got:\n%s", id, job)
		}
	}
	raw, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "RACE_SHARD") != 2 {
		t.Fatal("RACE_SHARD must only occur in the two race execution steps")
	}
}

func TestRaceCIHasElevenExplicitExpandedChecks(t *testing.T) {
	jobs := raceCIJobs(t)
	var ids, names []string
	for id, job := range jobs {
		ids = append(ids, id)
		match := regexp.MustCompile(`(?m)^    name: (.+)$`).FindStringSubmatch(job)
		if match == nil {
			t.Fatalf("job %s has no explicit check name", id)
		}
		name := match[1]
		if id == "quality" || id == "test" {
			header, _, ok := strings.Cut(job, "\n    steps:\n")
			wantHeader := `    name: ` + name + `
    runs-on: ${{ matrix.os }}
    timeout-minutes: 15
    strategy:
      fail-fast: false
      matrix:
        os:
          - ubuntu-latest
          - macos-latest
          - windows-latest
`
			if !ok || header != wantHeader {
				t.Fatalf("%s must retain exactly the three existing OS checks", id)
			}
			for _, os := range []string{"ubuntu-latest", "macos-latest", "windows-latest"} {
				names = append(names, strings.ReplaceAll(name, "${{ matrix.os }}", os))
			}
		} else {
			if strings.Contains(job, "\n    strategy:") {
				t.Fatalf("unexpected matrix in %s", id)
			}
			names = append(names, name)
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"fuzz", "quality", "race", "race-packages", "race-root", "test", "xsd-conformance"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("job IDs = %q, want %q", ids, wantIDs)
	}
	sort.Strings(names)
	wantNames := []string{"External XSD conformance", "Fuzz smoke", "Quality (macos-latest)", "Quality (ubuntu-latest)", "Quality (windows-latest)", "Race detector", "Race detector (packages)", "Race detector (root)", "Test (macos-latest)", "Test (ubuntu-latest)", "Test (windows-latest)"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("expanded checks = %q, want all eleven %q", names, wantNames)
	}
}

func TestRaceCIAggregateFailsClosedForAllShardResults(t *testing.T) {
	job, ok := raceCIJobs(t)["race"]
	if !ok {
		t.Fatal("missing stable race aggregate job")
	}
	const prefix = `    name: Race detector
    needs: [race-root, race-packages]
    if: ${{ always() }}
    runs-on: ubuntu-latest
    timeout-minutes: 5

    steps:
      - name: Require both race shards
        if: ${{ always() }}
        shell: bash
        env:
          ROOT_RESULT: ${{ needs.race-root.result }}
          PACKAGES_RESULT: ${{ needs.race-packages.result }}
        run: |
`
	if !strings.HasPrefix(job, prefix) {
		t.Fatalf("aggregate identity, exact dependencies, always guards, finite cap or result bindings changed:\n%s", job)
	}
	var gate strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(job, prefix), "\n"), "\n") {
		if !strings.HasPrefix(line, "          ") {
			t.Fatalf("unexpected aggregate configuration outside gate: %q", line)
		}
		gate.WriteString(strings.TrimPrefix(line, "          "))
		gate.WriteByte('\n')
	}
	results := []string{"success", "failure", "cancelled", "skipped", "", "unknown", "SUCCESS", "success\nfailure"}
	for _, root := range results {
		for _, packages := range results {
			cmd := exec.Command("bash", "-euo", "pipefail", "-c", gate.String())
			cmd.Env = raceTestEnv(map[string]string{"ROOT_RESULT": root, "PACKAGES_RESULT": packages})
			output, err := cmd.CombinedOutput()
			status := raceTestExit(t, err)
			if (status == 0) != (root == "success" && packages == "success") {
				t.Fatalf("root=%q packages=%q: status=%d output=%s", root, packages, status, output)
			}
		}
	}
	for _, unset := range [][]string{{"ROOT_RESULT"}, {"PACKAGES_RESULT"}, {"ROOT_RESULT", "PACKAGES_RESULT"}} {
		cmd := exec.Command("bash", "-euo", "pipefail", "-c", gate.String())
		values := map[string]string{"ROOT_RESULT": "success", "PACKAGES_RESULT": "success"}
		for _, key := range unset {
			delete(values, key)
		}
		cmd.Env = raceTestEnv(values, unset...)
		if err := cmd.Run(); err == nil {
			t.Fatalf("aggregate accepted missing result(s) %q", unset)
		}
	}
}

func TestRaceShardingLeavesLocalStagesAndReleaseUnchanged(t *testing.T) {
	script, err := os.ReadFile("scripts/check-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"all_stages=(mod generate format test vet race fuzz)", `"$check_go" test -race -count=1 -timeout=15m ./...`} {
		if !strings.Contains(string(script), required) {
			t.Fatalf("local contract omits %q", required)
		}
	}
	release, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(release), "RACE_SHARD") || !strings.Contains(string(release), "          bash scripts/check-all.sh\n") {
		t.Fatal("release must retain the full unsharded check sequence")
	}
}
