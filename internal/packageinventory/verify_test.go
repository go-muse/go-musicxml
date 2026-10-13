package packageinventory

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestVerifyCommittedInventory(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := Run([]string{"verify", "--root", root, "--strict-evidence"}, &out, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &stderr)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result["failures"].([]any)) != 0 || result["test_functions"] != float64(233) {
		t.Fatalf("%#v", result)
	}
	retained := result["retained_counts"].(map[string]any)
	if retained["requirements"] != float64(95) || retained["context_checks"] != float64(307) {
		t.Fatal(retained)
	}
}

func TestSuppliedArchiveUsesExactBytesAndProtectedPaths(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"unchanged", "CRLF", "size", "missing", "missing-function", "invalid-UTF8", "output-alias"} {
		t.Run(mutation, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			historical := mo(f.e["historical_tests"])
			var data bytes.Buffer
			data.WriteString("package musicxml\n")
			for _, ref := range sortedKeys(historical) {
				data.WriteString("func " + ms(mo(historical[ref])["function"]) + "() {}\n")
			}
			raw := data.Bytes()
			name := "archive-tests.go"
			sum := maintenanceDigest(raw)
			f.m["external_archive"] = []any{map[string]any{"artifact": name, "sha256": sum, "bytes": len(raw)}}
			for _, v := range historical {
				record := mo(v)
				record["artifact"] = name
				record["file_sha256"] = sum
			}
			f.write(name, raw)
			switch mutation {
			case "CRLF":
				f.write(name, bytes.ReplaceAll(raw, []byte("\n"), []byte("\r\n")))
			case "size":
				mo(ma(f.m["external_archive"])[0])["bytes"] = len(raw) + 1
			case "missing":
				if err := os.Remove(f.path(name)); err != nil {
					t.Fatal(err)
				}
			case "missing-function":
				raw = []byte("package musicxml\n")
				f.write(name, raw)
				sum = maintenanceDigest(raw)
				mo(ma(f.m["external_archive"])[0])["sha256"] = sum
				mo(ma(f.m["external_archive"])[0])["bytes"] = len(raw)
				for _, v := range historical {
					mo(v)["file_sha256"] = sum
				}
			case "invalid-UTF8":
				f.write(name, []byte{0xff})
			}
			f.save()
			want := 1
			if mutation == "unchanged" {
				want = 0
			}
			args := []string{"--archive-dir", f.root}
			if mutation == "output-alias" {
				args = append(args, "--output", f.path(name))
			}
			result := f.run(t, "verify", want, args...)
			if mutation == "output-alias" {
				maintenanceContains(t, maintenanceFailures(result), "protected input")
				maintenanceEqual(t, f.read(name), raw)
			}
			if mutation == "unchanged" {
				maintenanceEqual(t, result["external_archive"], "all supplied external artifacts checked against preserved hashes")
			}
		})
	}
}

func TestToolInputsProtectedOutsideSelectedRoot(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	f.mutateTest()
	beforeEvidence := f.read(maintenanceEvidence)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{filepath.Join(maintenanceRepo(t), "internal/packageinventory/common.go"), filepath.Join(maintenanceRepo(t), "cmd/package-inventory/main.go"), exe} {
		before := maintenanceRead(t, target)
		for _, command := range []string{"verify", "refresh", "test"} {
			args := []string{"--output", target}
			if command == "refresh" {
				args = append(args, "--write")
			}
			result := f.run(t, command, 1, args...)
			maintenanceContains(t, maintenanceFailures(result), "protected input")
			maintenanceEqual(t, maintenanceRead(t, target), before)
			maintenanceEqual(t, f.read(maintenanceEvidence), beforeEvidence)
		}
	}
}

func TestRefreshChangesOnlyHashTokenBytes(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	tests := mo(f.e["tests"])
	old := ""
	for _, v := range tests {
		record := mo(v)
		if ms(record["file"]) == "link_intent_test.go" {
			old = ms(record["file_sha256"])
			break
		}
	}
	if old == "" {
		t.Fatal("missing routed file")
	}
	f.e["unknown_metadata"] = map[string]any{"same_hash_outside_tests": old, "number": json.Number("1.000000e+09")}
	f.save()
	before := append([]byte(" \n\t"), f.read(maintenanceEvidence)...)
	before = append(before, []byte("\n \t")...)
	f.write(maintenanceEvidence, before)
	f.mutateTest()
	current := maintenanceDigest(bytes.ReplaceAll(f.read("link_intent_test.go"), []byte("\r\n"), []byte("\n")))
	f.run(t, "refresh", 0, "--write")
	after := f.read(maintenanceEvidence)
	maintenanceEqual(t, bytes.ReplaceAll(after, []byte(current), []byte(old)), before)
	maintenanceEqual(t, mo(maintenanceJSON(t, after)["unknown_metadata"])["same_hash_outside_tests"], old)
	f.run(t, "refresh", 0, "--write")
	maintenanceEqual(t, f.read(maintenanceEvidence), after)
}

func TestRelativeInputPathsResolveSymlinkBeforeParent(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"manifest", "evidence"} {
		t.Run(kind, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			f.write("nested/child/placeholder", nil)
			if err := os.Symlink(f.path("nested/child"), f.path("input-link")); err != nil {
				t.Skip(err)
			}
			name := "target.json"
			source := maintenanceManifest
			if kind == "evidence" {
				source = maintenanceEvidence
			}
			valid := f.read(source)
			f.write("nested/"+name, valid)
			f.write(name, []byte(`{"wrong":"lexical target"}`))
			input := "input-link" + string(filepath.Separator) + ".." + string(filepath.Separator) + name
			f.run(t, "verify", 0, "--"+kind, input)
			if kind == "evidence" {
				f.mutateTest()
				f.run(t, "refresh", 0, "--evidence", input, "--write")
				maintenanceEqual(t, string(f.read(name)), `{"wrong":"lexical target"}`)
				f.run(t, "verify", 0, "--evidence", input, "--strict-evidence")
			}
		})
	}
}

func TestRepositoryPathPreservesArchiveBaseTraversal(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	f.write("nested/child/placeholder", nil)
	f.write("nested/artifact", []byte("correct"))
	f.write("artifact", []byte("wrong"))
	if err := os.Symlink(f.path("nested/child"), f.path("archive-link")); err != nil {
		t.Skip(err)
	}
	base := f.path("archive-link") + string(filepath.Separator) + ".."
	maintenanceEqual(t, string(maintenanceRead(t, repositoryPath(base, "artifact"))), "correct")
	if err := rejectAlias(f.path("nested/artifact"), []string{repositoryPath(base, "artifact")}, defaultFileOps()); err == nil {
		t.Fatal("archive alias not protected")
	}
}

func TestRepositorySourceFailuresUseInventoryLabels(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"missing", "malformed"} {
		t.Run(mutation, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			source := maintenanceFirst(mo(f.m["source_files"]))
			want := "cannot read input: " + source + ":"
			if mutation == "missing" {
				if err := os.Remove(f.path(source)); err != nil {
					t.Fatal(err)
				}
			} else {
				f.write(source, []byte("not JSON"))
				want = "cannot parse JSON: " + source + ":"
			}
			before := f.read(maintenanceEvidence)
			for _, command := range []string{"verify", "refresh"} {
				args := []string{}
				if command == "refresh" {
					args = append(args, "--write")
				}
				failures := maintenanceFailures(f.run(t, command, 1, args...))
				if !strings.HasPrefix(failures, want) {
					t.Errorf("%s diagnostic=%q, want repository label prefix %q", command, failures, want)
				}
				maintenanceEqual(t, f.read(maintenanceEvidence), before)
			}
		})
	}
}

func TestGitFailureWarningIncludesCommandAndStderr(t *testing.T) {
	t.Parallel()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed; empty-PATH behavior is tested separately")
	}
	for _, phase := range []string{"rev-parse", "status"} {
		t.Run(phase, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			run := func(args ...string) {
				t.Helper()
				cmd := exec.Command(git, append([]string{"-C", f.root}, args...)...)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
			}
			run("init", "--quiet")
			args := []string{"rev-parse", "HEAD"}
			if phase == "status" {
				run("-c", "user.name=Inventory Test", "-c", "user.email=inventory@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "published head")
				f.write(".git/index", []byte("invalid index"))
				args = []string{"status", "--porcelain"}
			}
			_, err := exec.Command(git, append([]string{"-C", f.root}, args...)...).Output()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || len(bytes.TrimSpace(exit.Stderr)) == 0 {
				t.Fatalf("expected git failure with stderr, got %v", err)
			}
			resolvedRoot, err := filepath.EvalSymlinks(f.root)
			if err != nil {
				t.Fatal(err)
			}
			result := f.run(t, "verify", 0)
			maintenanceEqual(t, result["observed_current_head"], "unavailable")
			maintenanceEqual(t, result["observed_worktree_status"], "unavailable")
			warning := ""
			for _, value := range ma(result["warnings"]) {
				if strings.HasPrefix(ms(value), "git observation unavailable:") {
					warning = ms(value)
				}
			}
			for _, want := range []string{"git -C ", strconv.Quote(resolvedRoot), strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr))} {
				if !strings.Contains(warning, want) {
					t.Errorf("warning=%q missing diagnostic context %q", warning, want)
				}
			}
			if strings.TrimSpace(warning) != warning {
				t.Errorf("git warning contains surrounding whitespace: %q", warning)
			}
		})
	}
}

func TestOversizedInventoryIntegersRemainIntegrityFailures(t *testing.T) {
	t.Parallel()
	huge := json.Number("99999999999999999999")
	for _, tc := range []struct {
		name   string
		mutate func(*maintenanceFixture) []string
	}{
		{"source-size", func(f *maintenanceFixture) []string {
			source := maintenanceFirst(mo(f.m["source_files"]))
			mo(mo(f.m["source_files"])[source])["bytes"] = huge
			return []string{"changed original source size: " + source}
		}},
		{"collection-count", func(f *maintenanceFixture) []string {
			mf(f.m, "preserved_collections", "requirements")["count"] = huge
			return []string{"record count: requirements"}
		}},
		{"declared-counts", func(f *maintenanceFixture) []string {
			mo(f.m["counts"])["requirements"] = huge
			mo(f.m["counts"])["selected_engineering"] = huge
			return []string{"declared original denominator drift", "declared selected denominator drift"}
		}},
		{"positive-index", func(f *maintenanceFixture) []string {
			mo(ma(mf(f.m, "preserved_collections", "requirements")["records"])[0])["index"] = huge
			return []string{"lost index: requirements"}
		}},
		{"negative-index", func(f *maintenanceFixture) []string {
			record := mo(ma(mf(f.m, "preserved_collections", "requirements")["records"])[0])
			record["index"] = json.Number("-99999999999999999999")
			return []string{"negative record index: " + ms(record["id"]), "lost index: requirements"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			want := tc.mutate(f)
			f.save()
			before := f.read(maintenanceEvidence)
			for _, command := range []string{"verify", "refresh"} {
				args := []string{}
				if command == "refresh" {
					args = append(args, "--write")
				}
				result := f.run(t, command, 1, args...)
				failures := maintenanceFailures(result)
				for _, text := range want {
					maintenanceContains(t, failures, text)
				}
				if strings.Contains(failures, "invalid inventory integer") || result["test_functions"] == nil {
					t.Fatalf("integer overflow aborted verification instead of reporting integrity failures: %#v", result)
				}
				maintenanceEqual(t, f.read(maintenanceEvidence), before)
			}
		})
	}
}

func TestOversizedArchiveSizeRemainsIntegrityFailure(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	data := []byte("raw archive bytes\r\n")
	name := "oversized-record.bin"
	f.write(name, data)
	f.m["external_archive"] = []any{map[string]any{"artifact": name, "sha256": maintenanceDigest(data), "bytes": json.Number("99999999999999999999")}}
	f.save()
	before := f.read(maintenanceEvidence)
	result := f.run(t, "refresh", 1, "--archive-dir", f.root, "--write")
	maintenanceContains(t, maintenanceFailures(result), "changed archive: "+name)
	if result["test_functions"] == nil {
		t.Fatalf("integer overflow aborted verification instead of reporting integrity failures: %#v", result)
	}
	maintenanceEqual(t, f.read(maintenanceEvidence), before)
	maintenanceEqual(t, f.read(name), data)
}
