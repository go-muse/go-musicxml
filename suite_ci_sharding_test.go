package musicxml

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// suiteCIShards are the values CI gives TEST_SHARD.
var suiteCIShards = []string{"1", "2"}

// These tests run the real check script against a fake Go executable, as the
// race shard tests do: they check the complete argv of every command of the
// test stage without running the suite inside itself.
type suiteShardFixture struct {
	goPath, log, root, inventory string
}

func newSuiteShardFixture(t *testing.T) suiteShardFixture {
	t.Helper()
	dir := t.TempDir()
	f := suiteShardFixture{
		goPath: filepath.Join(dir, "fake go"), log: filepath.Join(dir, "calls"),
		root: filepath.Join(dir, "root"), inventory: filepath.Join(dir, "inventory"),
	}
	fake := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\0' "$@" >> "$FAKE_LOG"
printf '\0' >> "$FAKE_LOG"
case "$1" in
    list)
        if [[ "$#" != 2 ]]; then exit 90; fi
        case "$2" in
            .) cat "$FAKE_ROOT"; exit "$FAKE_ROOT_STATUS" ;;
            ./...) cat "$FAKE_INVENTORY"; exit "$FAKE_INVENTORY_STATUS" ;;
            *) exit 91 ;;
        esac ;;
    test)
        if [[ " $* " == *" $FAKE_FAILING_ARGUMENT "* ]]; then exit "$FAKE_TEST_STATUS"; fi ;;
    *) exit 92 ;;
esac
`
	if err := os.WriteFile(f.goPath, []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	return f
}

// suiteShardRun describes one run of the test stage: the shard (nil leaves
// TEST_SHARD unset), what discovery prints and returns, and the argument
// whose test command fails with testStatus.
type suiteShardRun struct {
	shard                       *string
	root, inventory             string
	rootStatus, inventoryStatus int
	failing                     string
	testStatus                  int
}

func (f suiteShardFixture) run(t *testing.T, r suiteShardRun) (int, [][]string, string) {
	t.Helper()
	for path, data := range map[string]string{f.root: r.root, f.inventory: r.inventory, f.log: ""} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", "scripts/check-all.sh", "test")
	cmd.Env = raceTestEnv(map[string]string{
		"GO": filepath.ToSlash(f.goPath), "FAKE_LOG": filepath.ToSlash(f.log),
		"FAKE_ROOT": filepath.ToSlash(f.root), "FAKE_INVENTORY": filepath.ToSlash(f.inventory),
		"FAKE_ROOT_STATUS": fmt.Sprint(r.rootStatus), "FAKE_INVENTORY_STATUS": fmt.Sprint(r.inventoryStatus),
		"FAKE_FAILING_ARGUMENT": r.failing, "FAKE_TEST_STATUS": fmt.Sprint(r.testStatus),
	}, "TEST_SHARD")
	if r.shard != nil {
		cmd.Env = append(cmd.Env, "TEST_SHARD="+*r.shard)
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

const (
	suiteShardRoot      = "example.org/musicxml"
	suiteShardInventory = suiteShardRoot + "/cmd/tool\n" + suiteShardRoot + "\n" + suiteShardRoot + "/validation\n"
)

func TestSuiteShardDefaultCommandIsUnchanged(t *testing.T) {
	f := newSuiteShardFixture(t)
	for _, testStatus := range []int{0, 37} {
		status, calls, output := f.run(t, suiteShardRun{rootStatus: 99, inventoryStatus: 99, failing: "./...", testStatus: testStatus})
		want := [][]string{{"test", "-count=1", "./..."}}
		if status != testStatus || !reflect.DeepEqual(calls, want) {
			t.Fatalf("status=%d calls=%q output=%s; want status=%d calls=%q", status, calls, output, testStatus, want)
		}
	}
}

// Shard 1 selects root tests with -run and shard 2 leaves the same tests out
// with -skip, so the two shards differ only in the flag: a root test, example
// or fuzz seed is in exactly one of them. Every other package runs in shard 2.
func TestSuiteShardsPartitionTheSuite(t *testing.T) {
	f := newSuiteShardFixture(t)
	one, two := "1", "2"
	status, first, output := f.run(t, suiteShardRun{shard: &one, root: suiteShardRoot + "\n", inventory: suiteShardInventory, rootStatus: 99, inventoryStatus: 99})
	if status != 0 || len(first) != 1 {
		t.Fatalf("shard 1: status=%d calls=%q output=%s", status, first, output)
	}
	status, second, output := f.run(t, suiteShardRun{shard: &two, root: suiteShardRoot + "\n", inventory: suiteShardInventory})
	if status != 0 || len(second) != 4 {
		t.Fatalf("shard 2: status=%d calls=%q output=%s", status, second, output)
	}
	selected, found := strings.CutPrefix(first[0][2], "-run=")
	skipped, cut := strings.CutPrefix(second[2][2], "-skip=")
	if !found || !cut || selected != skipped || selected == "" {
		t.Fatalf("shard 1 selects %q and shard 2 skips %q: they must name one pattern", first[0], second[2])
	}
	if want := [][]string{{"test", "-count=1", "-run=" + selected, "."}}; !reflect.DeepEqual(first, want) {
		t.Fatalf("shard 1 calls = %q, want %q", first, want)
	}
	want := [][]string{{"list", "."}, {"list", "./..."},
		{"test", "-count=1", "-skip=" + selected, "."},
		{"test", "-count=1", suiteShardRoot + "/cmd/tool", suiteShardRoot + "/validation"}}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("shard 2 calls = %q, want %q", second, want)
	}
	// A pattern with a slash would also select or skip subtests by name.
	pattern, err := regexp.Compile(selected)
	if err != nil || strings.Contains(selected, "/") {
		t.Fatalf("shard pattern %q: %v", selected, err)
	}
	// Neither shard of the real root package is empty.
	names, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	declaration := regexp.MustCompile(`(?m)^func ((?:Test|Example|Fuzz)\w*)\(`)
	matching, other := 0, 0
	for _, name := range names {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range declaration.FindAllSubmatch(source, -1) {
			if pattern.Match(match[1]) {
				matching++
			} else {
				other++
			}
		}
	}
	if matching < 100 || other < 100 {
		t.Fatalf("pattern %q puts %d root tests in shard 1 and %d in shard 2", selected, matching, other)
	}
}

func TestSuiteShardRejectsInvalidSelectionBeforeAnyCommand(t *testing.T) {
	f := newSuiteShardFixture(t)
	for _, shard := range []string{"", "0", "3", "12", "1 2", "all", "root", "1\n", "*"} {
		status, calls, output := f.run(t, suiteShardRun{shard: &shard, root: suiteShardRoot + "\n", inventory: suiteShardInventory})
		if status != 2 || len(calls) != 0 {
			t.Errorf("shard %q: status=%d calls=%q output=%s", shard, status, calls, output)
		}
	}
}

// A failed discovery or a failed command ends the stage with its status, and
// nothing runs after it.
func TestSuiteShardPreservesFailureStatus(t *testing.T) {
	f := newSuiteShardFixture(t)
	one, two := "1", "2"
	for _, tc := range []struct {
		name          string
		run           suiteShardRun
		status, calls int
	}{
		{"root tests of shard 1", suiteShardRun{shard: &one, failing: ".", testStatus: 37}, 37, 1},
		{"root discovery", suiteShardRun{shard: &two, rootStatus: 23}, 23, 1},
		{"inventory discovery", suiteShardRun{shard: &two, inventoryStatus: 29}, 29, 2},
		{"root tests of shard 2", suiteShardRun{shard: &two, failing: ".", testStatus: 43}, 43, 3},
		{"other packages", suiteShardRun{shard: &two, failing: suiteShardRoot + "/validation", testStatus: 41}, 41, 4},
	} {
		tc.run.root, tc.run.inventory = suiteShardRoot+"\n", suiteShardInventory
		status, calls, output := f.run(t, tc.run)
		if status != tc.status || len(calls) != tc.calls {
			t.Errorf("%s: status=%d calls=%q output=%s; want status=%d and %d calls", tc.name, status, calls, output, tc.status, tc.calls)
		}
	}
}

// The inventory checks are those of the race shards; here only that the test
// stage runs them before any test starts.
func TestSuiteShardRejectsMalformedInventories(t *testing.T) {
	f := newSuiteShardFixture(t)
	two := "2"
	for _, tc := range []struct{ name, root, inventory string }{
		{"empty root", "", suiteShardInventory},
		{"missing root", suiteShardRoot + "\n", suiteShardRoot + "/validation\n"},
		{"duplicate package", suiteShardRoot + "\n", suiteShardInventory + suiteShardRoot + "/validation\n"},
		{"blank line", suiteShardRoot + "\n", suiteShardRoot + "\n\n" + suiteShardRoot + "/validation\n"},
		{"option", suiteShardRoot + "\n", suiteShardRoot + "\n-run=none\n"},
		{"no other package", suiteShardRoot + "\n", suiteShardRoot + "\n"},
	} {
		status, calls, output := f.run(t, suiteShardRun{shard: &two, root: tc.root, inventory: tc.inventory})
		if status != 2 {
			t.Errorf("%s: status=%d calls=%q output=%s", tc.name, status, calls, output)
		}
		for _, call := range calls {
			if call[0] == "test" {
				t.Errorf("%s: a malformed inventory started tests: %q", tc.name, calls)
			}
		}
	}
}

const suiteCIShardBlock = `    name: Test shard ${{ matrix.shard }} (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    timeout-minutes: 15
    strategy:
      fail-fast: false
      matrix:
        os:
          - ubuntu-latest
          # Off for now (owner, 2026-11-08); see CONTRIBUTING.md to restore.
          # - macos-latest
          # - windows-latest
        shard: [1, 2]

    steps:
      - name: Check out repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1

      - name: Set up Go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: true
          cache-dependency-path: go.sum

      - name: Run tests
        shell: bash
        env:
          TEST_SHARD: ${{ matrix.shard }}
        run: bash scripts/check-all.sh test
`

// The shard job keeps the runner, the pinned actions, the 15-minute cap and
// the command of the job it replaces; only the shard is new.
func TestSuiteCIShardsPreserveRunnerActionsAndBudget(t *testing.T) {
	job, ok := raceCIJobs(t)["test-shard"]
	if !ok {
		t.Fatal("missing test shard job")
	}
	if job != suiteCIShardBlock {
		t.Fatalf("test-shard changed:\n%s", job)
	}
	if !strings.Contains(suiteCIShardBlock, "        shard: ["+strings.Join(suiteCIShards, ", ")+"]\n") {
		t.Fatalf("the shard job does not run exactly shards %q", suiteCIShards)
	}
	for name, want := range map[string]int{".github/workflows/ci.yml": 1, ".github/workflows/release.yml": 0} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), "TEST_SHARD"); got != want {
			t.Fatalf("%s mentions TEST_SHARD %d times, want %d", name, got, want)
		}
	}
}

// `Test (ubuntu-latest)` is the check the ruleset requires. It succeeds only
// when every shard succeeded: a failed, cancelled, skipped or missing result
// cannot pass it.
func TestSuiteCIGateFailsClosedForAllShardResults(t *testing.T) {
	job, ok := raceCIJobs(t)["test"]
	if !ok {
		t.Fatal("missing the stable test gate")
	}
	const prefix = `    name: Test (${{ matrix.os }})
    needs: [test-shard]
    if: ${{ always() }}
    runs-on: ubuntu-latest
    timeout-minutes: 5
    strategy:
      fail-fast: false
      matrix:
        os:
          - ubuntu-latest
          # Off for now (owner, 2026-11-08); see CONTRIBUTING.md to restore.
          # - macos-latest
          # - windows-latest

    steps:
      - name: Require every test shard
        shell: bash
        env:
          SHARDS_RESULT: ${{ needs.test-shard.result }}
        run: |
`
	if !strings.HasPrefix(job, prefix) {
		t.Fatalf("the gate's identity, dependency, always guard, cap or result binding changed:\n%s", job)
	}
	var gate strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(job, prefix), "\n"), "\n") {
		if !strings.HasPrefix(line, "          ") {
			t.Fatalf("unexpected configuration outside the gate: %q", line)
		}
		gate.WriteString(strings.TrimPrefix(line, "          "))
		gate.WriteByte('\n')
	}
	for _, result := range []string{"success", "failure", "cancelled", "skipped", "", "unknown", "SUCCESS", "success\nfailure"} {
		cmd := raceHostedBashCommand(t, gate.String())
		cmd.Env = raceTestEnv(map[string]string{"SHARDS_RESULT": result})
		output, err := cmd.CombinedOutput()
		if status := raceTestExit(t, err); (status == 0) != (result == "success") {
			t.Fatalf("result=%q: status=%d output=%s", result, status, output)
		}
	}
	cmd := raceHostedBashCommand(t, gate.String())
	cmd.Env = raceTestEnv(nil, "SHARDS_RESULT")
	output, err := cmd.CombinedOutput()
	if status := raceTestExit(t, err); status != 1 || string(output) != "Test shards did not all succeed (result=).\n" {
		t.Fatalf("a missing result must fail in the gate: status=%d output=%q", status, output)
	}
}

// The short tier is named, not part of the default sequence, and the only
// command of the script that passes -short.
func TestShortTierIsNotPartOfTheFullSequence(t *testing.T) {
	raw, err := os.ReadFile("scripts/check-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	var short []string
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, `"$check_go"`) && strings.Contains(line, "-short") {
			short = append(short, strings.TrimSpace(line))
		}
	}
	if want := []string{`"$check_go" test -short -count=1 ./...`}; !reflect.DeepEqual(short, want) || !strings.Contains(script, "named_stages=(short)\n") {
		t.Fatalf("commands with -short = %q, want only %q, in a stage that runs only when named", short, want)
	}
	for _, name := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		workflow, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(workflow), "short") {
			t.Fatalf("%s must run the full tier", name)
		}
	}
}
