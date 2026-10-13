package packageinventory

// These 43 TestMaintenance functions map one-for-one, in the same order, to
// the regression methods in the former test-package-inventory-maintenance.py.
// Table-driven subtests retain every mutation and every command/path combination.
// Top-level tests own independent fixtures and use testing's default parallel
// limit; subtests that share a fixture intentionally remain sequential.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

const maintenanceManifest = "docs/validation/package-assessment-ownership.json"
const maintenanceEvidence = "docs/validation/evidence/package-row-evidence.json"

var maintenanceCommands = []string{"verify", "refresh", "test"}

type maintenanceFixture struct {
	t         testing.TB
	root      string
	m, e      map[string]any
	originals map[string][]byte
}

func maintenanceRepo(t testing.TB) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func maintenanceRead(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func maintenanceWrite(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func maintenanceJSON(t testing.TB, data []byte) map[string]any {
	t.Helper()
	value, err := maintenanceDecodeJSON(data)
	if err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	return value
}

func maintenanceDecodeJSON(data []byte) (map[string]any, error) {
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New("expected JSON object")
	}
	return value, nil
}

func maintenanceEncode(t testing.TB, value any) []byte {
	t.Helper()
	data, err := maintenanceEncodeJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func maintenanceEncodeJSON(value any) ([]byte, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Only the original test inputs are shared. Every fixture gets recursively
// copied values and source bytes, plus fresh files; command results and reads
// of those files are never cached.
type maintenanceFixtureBaseline struct {
	m, e               map[string]any
	originals          map[string][]byte
	manifest, evidence []byte
}

type maintenanceFixtureCache struct {
	once     sync.Once
	baseline *maintenanceFixtureBaseline
	err      error
}

var maintenanceBaseline maintenanceFixtureCache

func (c *maintenanceFixtureCache) load(repo string) (*maintenanceFixtureBaseline, error) {
	c.once.Do(func() {
		// Do not call testing.Fatal inside Once: a failed load must leave an
		// error for every caller, not a nil baseline after Goexit completes it.
		c.baseline, c.err = maintenanceLoadBaseline(repo)
	})
	return c.baseline, c.err
}

func maintenanceLoadBaseline(repo string) (*maintenanceFixtureBaseline, error) {
	readJSON := func(path string) (map[string]any, error) {
		data, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil {
			return nil, err
		}
		value, err := maintenanceDecodeJSON(data)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		return value, nil
	}
	baseline := &maintenanceFixtureBaseline{originals: map[string][]byte{}}
	var err error
	if baseline.m, err = readJSON(maintenanceManifest); err != nil {
		return nil, err
	}
	if baseline.e, err = readJSON(maintenanceEvidence); err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	sources, ok := baseline.m["source_files"].(map[string]any)
	if !ok {
		return nil, errors.New("fixture manifest source_files must be an object")
	}
	for path := range sources {
		paths[path] = true
	}
	corrections, ok := baseline.m["reviewed_source_corrections"].([]any)
	if !ok {
		return nil, errors.New("fixture manifest reviewed_source_corrections must be an array")
	}
	addPath := func(value any, key string) error {
		record, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("fixture %s record must be an object", key)
		}
		path, ok := record[key].(string)
		if !ok || path == "" {
			return fmt.Errorf("fixture %s path must be a nonempty string", key)
		}
		paths[path] = true
		return nil
	}
	for _, value := range corrections {
		if err := addPath(value, "reconciliation"); err != nil {
			return nil, err
		}
	}
	tests, ok := baseline.e["tests"].(map[string]any)
	if !ok {
		return nil, errors.New("fixture evidence tests must be an object")
	}
	for _, value := range tests {
		if err := addPath(value, "file"); err != nil {
			return nil, err
		}
	}
	for path := range paths {
		if baseline.originals[path], err = os.ReadFile(filepath.Join(repo, path)); err != nil {
			return nil, err
		}
	}
	if baseline.manifest, err = maintenanceEncodeJSON(baseline.m); err != nil {
		return nil, err
	}
	if baseline.evidence, err = maintenanceEncodeJSON(baseline.e); err != nil {
		return nil, err
	}
	return baseline, nil
}

func newMaintenanceFixture(t testing.TB) *maintenanceFixture {
	t.Helper()
	baseline, err := maintenanceBaseline.load(maintenanceRepo(t))
	if err != nil {
		t.Fatalf("load maintenance fixture baseline: %v", err)
	}
	f := &maintenanceFixture{
		t: t, root: t.TempDir(),
		m: maintenanceClone(t, baseline.m), e: maintenanceClone(t, baseline.e),
		originals: make(map[string][]byte, len(baseline.originals)),
	}
	for path, data := range baseline.originals {
		f.originals[path] = bytes.Clone(data)
		maintenanceWrite(t, f.path(path), f.originals[path])
	}
	// Use the same encoding as save, computed once from immutable originals.
	f.write(maintenanceManifest, baseline.manifest)
	f.write(maintenanceEvidence, baseline.evidence)
	return f
}

func mo(v any) map[string]any { return v.(map[string]any) }
func ma(v any) []any          { return v.([]any) }
func ms(v any) string         { return v.(string) }
func mf(v any, keys ...string) map[string]any {
	for _, key := range keys {
		v = mo(v)[key]
	}
	return mo(v)
}
func maintenanceFirst(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys[0]
}
func maintenanceClone(t testing.TB, m map[string]any) map[string]any {
	t.Helper()
	return maintenanceCloneValue(m).(map[string]any)
}

func maintenanceCloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		if value == nil {
			return map[string]any(nil)
		}
		clone := make(map[string]any, len(value))
		for key, item := range value {
			clone[key] = maintenanceCloneValue(item)
		}
		return clone
	case []any:
		if value == nil {
			return []any(nil)
		}
		clone := make([]any, len(value))
		for i, item := range value {
			clone[i] = maintenanceCloneValue(item)
		}
		return clone
	case []byte:
		return bytes.Clone(value)
	default:
		// JSON scalars, including json.Number's original token, are immutable.
		return value
	}
}
func (f *maintenanceFixture) path(p string) string {
	return filepath.Join(f.root, filepath.FromSlash(p))
}
func (f *maintenanceFixture) read(p string) []byte {
	f.t.Helper()
	return maintenanceRead(f.t, f.path(p))
}
func (f *maintenanceFixture) write(p string, b []byte) {
	f.t.Helper()
	maintenanceWrite(f.t, f.path(p), b)
}
func (f *maintenanceFixture) save() {
	f.t.Helper()
	f.write(maintenanceManifest, maintenanceEncode(f.t, f.m))
	f.write(maintenanceEvidence, maintenanceEncode(f.t, f.e))
}
func (f *maintenanceFixture) mutateTest() {
	f.t.Helper()
	f.write("link_intent_test.go", append(f.read("link_intent_test.go"), []byte("// Maintenance regression.\n")...))
}
func (f *maintenanceFixture) run(t *testing.T, command string, expected int, args ...string) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{command, "--root", f.root}, args...), &stdout, &stderr)
	return maintenanceResult(t, stdout.Bytes(), stderr.Bytes(), code, expected)
}
func maintenanceResult(t *testing.T, stdout, stderr []byte, code, expected int) map[string]any {
	t.Helper()
	result := maintenanceJSON(t, stdout)
	if code != expected {
		t.Fatalf("exit = %d, want %d; result=%s; stderr=%s", code, expected, stdout, stderr)
	}
	if bytes.Contains(stderr, []byte("Traceback")) || bytes.Contains(stderr, []byte("panic:")) {
		t.Fatalf("unhandled failure: %s", stderr)
	}
	return result
}
func maintenanceFailures(result map[string]any) string {
	var a []string
	for _, v := range ma(result["failures"]) {
		a = append(a, ms(v))
	}
	return strings.Join(a, "\n")
}
func maintenanceContains(t *testing.T, value, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("%q does not contain %q", value, want)
	}
}
func maintenanceEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v; want %#v", got, want)
	}
}
func maintenanceNoTemps(t *testing.T, root, pattern string) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) > 0 {
		t.Fatalf("temporary files remain: %v", names)
	}
}
func maintenanceLink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks unavailable: %v", err)
		}
		t.Fatal(err)
	}
}
func maintenanceReadlink(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	maintenanceEqual(t, got, want)
}

var maintenanceBuildOnce sync.Once
var maintenanceBinaryPath string
var maintenanceBuildError error

func maintenanceBinary(t *testing.T) string {
	t.Helper()
	repo := maintenanceRepo(t)
	maintenanceBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "package-inventory-binary-")
		if err != nil {
			maintenanceBuildError = err
			return
		}
		maintenanceBinaryPath = filepath.Join(dir, "package-inventory")
		if runtime.GOOS == "windows" {
			maintenanceBinaryPath += ".exe"
		}
		command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", maintenanceBinaryPath, "./cmd/package-inventory")
		command.Dir = repo
		out, err := command.CombinedOutput()
		if err != nil {
			maintenanceBuildError = fmt.Errorf("build inventory executable: %w: %s", err, out)
		}
	})
	if maintenanceBuildError != nil {
		t.Fatal(maintenanceBuildError)
	}
	return maintenanceBinaryPath
}
func (f *maintenanceFixture) binary(t *testing.T, command string, expected int, emptyPath bool, args ...string) map[string]any {
	t.Helper()
	cmd := exec.Command(maintenanceBinary(t), append([]string{command, "--root", f.root}, args...)...)
	cmd.Dir = f.root
	cmd.Env = os.Environ()
	if emptyPath {
		for i := len(cmd.Env) - 1; i >= 0; i-- {
			if strings.HasPrefix(strings.ToUpper(cmd.Env[i]), "PATH=") {
				cmd.Env = append(cmd.Env[:i], cmd.Env[i+1:]...)
			}
		}
		cmd.Env = append(cmd.Env, "PATH=")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return maintenanceResult(t, stdout.Bytes(), stderr.Bytes(), code, expected)
}

func TestMaintenanceDryRunThenWriteOnlyCurrentHashesAndIdempotence(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	f.mutateTest()
	oldM, oldE := f.read(maintenanceManifest), f.read(maintenanceEvidence)
	dry := f.run(t, "refresh", 0)
	maintenanceEqual(t, dry["mode"], "dry-run")
	maintenanceEqual(t, dry["changed_test_files"], []any{"link_intent_test.go"})
	maintenanceEqual(t, f.read(maintenanceEvidence), oldE)
	maintenanceEqual(t, f.read(maintenanceManifest), oldM)
	f.run(t, "refresh", 0, "--write")
	actual := maintenanceJSON(t, f.read(maintenanceEvidence))
	before := maintenanceClone(t, f.e)
	for ref, value := range mo(before["tests"]) {
		mo(value)["file_sha256"] = mo(mo(actual["tests"])[ref])["file_sha256"]
	}
	maintenanceEqual(t, actual, before)
	// Hash-only maintenance preserves formatting and every non-hash byte too.
	hashTokens := regexp.MustCompile(`("file_sha256"\s*:\s*")[a-f0-9]{64}(")`)
	maintenanceEqual(t, hashTokens.ReplaceAll(f.read(maintenanceEvidence), []byte(`${1}<hash>${2}`)), hashTokens.ReplaceAll(oldE, []byte(`${1}<hash>${2}`)))
	maintenanceEqual(t, f.read(maintenanceManifest), oldM)
	f.run(t, "verify", 0, "--strict-evidence")
	written := f.read(maintenanceEvidence)
	maintenanceEqual(t, f.run(t, "refresh", 0, "--write")["changed_test_files"], []any{})
	maintenanceEqual(t, f.read(maintenanceEvidence), written)
}

func TestMaintenanceCRLFAndMixedAreNoopButLoneCRIsNot(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	for name, data := range f.originals {
		f.write(name, bytes.ReplaceAll(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), []byte("\n"), []byte("\r\n")))
	}
	maintenanceEqual(t, f.run(t, "refresh", 0)["changed_test_files"], []any{})
	f.write("link_intent_test.go", bytes.Replace(f.read("link_intent_test.go"), []byte("\r\n"), []byte("\n"), 1))
	maintenanceEqual(t, f.run(t, "refresh", 0)["changed_test_files"], []any{})
	f.write("link_intent_test.go", append(f.read("link_intent_test.go"), '\r'))
	maintenanceEqual(t, f.run(t, "refresh", 0)["changed_test_files"], []any{"link_intent_test.go"})
}

func TestMaintenanceSourceWhitespaceStatusAndOwnerChangesAreBlocked(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"whitespace", "status", "owner"} {
		t.Run(mutation, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			path := "docs/validation/plan/context-dispositions.json"
			data := append([]byte{}, f.originals[path]...)
			if mutation == "whitespace" {
				data = append(data, ' ')
			} else {
				source := maintenanceJSON(t, data)
				mo(ma(source["checks"])[0])[mutation] = "unreviewed change"
				data = maintenanceEncode(t, source)
			}
			f.write(path, data)
			before := f.read(maintenanceEvidence)
			maintenanceContains(t, maintenanceFailures(f.run(t, "refresh", 1, "--write")), "source")
			maintenanceEqual(t, f.read(maintenanceEvidence), before)
		})
	}
}

func TestMaintenanceBlanketSourceRefreshIsNotSupported(t *testing.T) {
	t.Parallel()
	var out, err bytes.Buffer
	if code := Run([]string{"refresh", "--refresh-sources"}, &out, &err); code == 0 {
		t.Fatal("blanket source refresh unexpectedly supported")
	}
}

func TestMaintenanceDuplicateIssueRoutesAndDeclaredSelectedCountsAreRejected(t *testing.T) {
	t.Parallel()
	for _, which := range []string{"duplicate issue route", "selected count"} {
		t.Run(which, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			if which == "duplicate issue route" {
				gates := ma(mo(f.m["selected"])["issue_gates"])
				gates[1] = gates[0]
			} else {
				mo(f.m["counts"])["selected_engineering"] = json.Number("45")
			}
			f.save()
			f.run(t, "refresh", 1, "--write")
		})
	}
}

func TestMaintenanceMalformedNestedTypesAndMissingMetadataAreJSONFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(*maintenanceFixture)
	}{
		{"numeric pointer", func(f *maintenanceFixture) { mf(f.m, "preserved_collections", "requirements")["pointer"] = 12 }},
		{"array status", func(f *maintenanceFixture) {
			mo(ma(mf(f.m, "preserved_collections", "requirements")["records"])[0])["original_status_fields"] = []any{}
		}},
		{"array owner", func(f *maintenanceFixture) { mo(ma(mo(f.m["selected"])["requirements"])[0])["owner"] = []any{} }},
		{"array rows", func(f *maintenanceFixture) { f.e["rows"] = []any{} }},
		{"numeric hash convention", func(f *maintenanceFixture) { mo(f.e["hashing"])["test_file_sha256"] = 12 }},
		{"missing test hash", func(f *maintenanceFixture) {
			tests := mo(f.e["tests"])
			delete(mo(tests[maintenanceFirst(tests)]), "file_sha256")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			tc.change(f)
			f.save()
			f.run(t, "refresh", 1, "--write")
		})
	}
}

func TestMaintenanceSourceRecordHashIsNotRefreshed(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	mo(ma(mf(f.m, "preserved_collections", "requirements")["records"])[0])["sha256"] = strings.Repeat("0", 64)
	f.save()
	if maintenanceFailures(f.run(t, "refresh", 1, "--write")) == "" {
		t.Fatal("missing failure")
	}
}

func TestMaintenancePreservedStatusAndDenominatorsCannotBePromoted(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"status", "collection count", "selected row", "preserved row", "declared count"} {
		t.Run(name, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			collection := mf(f.m, "preserved_collections", "requirements")
			switch name {
			case "status":
				mo(mo(ma(collection["records"])[0])["original_status_fields"])["status"] = "implemented"
			case "collection count":
				collection["count"] = 96
			case "selected row":
				selected := mo(f.m["selected"])
				rows := ma(selected["requirements"])
				selected["requirements"] = rows[:len(rows)-1]
			case "preserved row":
				rows := ma(collection["records"])
				collection["records"] = rows[:len(rows)-1]
			case "declared count":
				mo(f.m["counts"])["requirements"] = 96
			}
			f.save()
			f.run(t, "refresh", 1, "--write")
		})
	}
}

func TestMaintenanceMissingSourceAndTestFilesAreJSONFailures(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	source := maintenanceFirst(mo(f.m["source_files"]))
	// Current test routes retain inventory spelling; source I/O errors name
	// the resolved native filesystem path.
	for _, tc := range []struct{ path, diagnosticPath string }{
		{"link_intent_test.go", "link_intent_test.go"},
		{source, filepath.FromSlash(source)},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if err := os.Remove(f.path(tc.path)); err != nil {
				t.Fatal(err)
			}
			maintenanceContains(t, maintenanceFailures(f.run(t, "refresh", 1, "--write")), tc.diagnosticPath)
			f.write(tc.path, f.originals[tc.path])
		})
	}
}

func TestMaintenanceMissingAndInventedTestRoutesAreBlocked(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	tests := mo(f.e["tests"])
	ref := maintenanceFirst(tests)
	record := tests[ref]
	delete(tests, ref)
	f.save()
	f.run(t, "refresh", 1, "--write")
	tests[ref] = record
	mo(record)["function"] = "TestInventedMissing"
	f.save()
	f.run(t, "refresh", 1, "--write")
}

func TestMaintenanceRouteKeyMustMatchFileAndFunction(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	tests := mo(f.e["tests"])
	key := maintenanceFirst(tests)
	tests["invented_test.go::TestInvented"] = tests[key]
	delete(tests, key)
	f.save()
	f.run(t, "verify", 1)
}

func TestMaintenanceAuxiliaryTestRoutesAndOriginalIDsAreValidated(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"test", "id"} {
		t.Run(name, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			aux := mo(ma(f.e["auxiliary_xsd_axes"])[0])
			if name == "test" {
				axis := mf(aux, "axes", "positive")
				axis["tests"] = append(ma(axis["tests"]), "unknown.go::TestUnknown")
			} else {
				aux["id"] = "unknown-instance"
			}
			f.save()
			f.run(t, "refresh", 1, "--write")
		})
	}
}

func TestMaintenanceUndeclaredSourceRouteIsRejected(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	delete(mo(f.m["source_files"]), "docs/validation/plan/context-dispositions.json")
	f.save()
	f.run(t, "refresh", 1, "--write")
}

func TestMaintenanceMalformedPointersProduceSpecificJSONErrors(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	for _, pointer := range []string{"#/absent", "#/requirements/nope", "#/requirements/999999", "#/requirements/-1", "#/requirements/00", "#/requirements/0/id/child", "#/bad~2escape", "requirements"} {
		t.Run(pointer, func(t *testing.T) {
			mf(f.m, "preserved_collections", "requirements")["pointer"] = pointer
			f.save()
			message := maintenanceFailures(f.run(t, "verify", 1))
			maintenanceContains(t, message, "unresolvable pointer:")
			maintenanceContains(t, message, "implementation-plan.json")
		})
	}
}

func TestMaintenanceMalformedJSONAndShapesHaveUsefulErrors(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	source := maintenanceFirst(mo(f.m["source_files"]))
	// Explicit input labels retain their supplied spelling. Only repository
	// source paths have been resolved to native separators before parsing.
	for _, tc := range []struct{ path, data, diagnosticPath string }{
		{maintenanceManifest, "{", maintenanceManifest},
		{maintenanceEvidence, "[]", maintenanceEvidence},
		{source, "not JSON", filepath.FromSlash(source)},
	} {
		t.Run(tc.path, func(t *testing.T) {
			old := f.read(tc.path)
			f.write(tc.path, []byte(tc.data))
			maintenanceContains(t, maintenanceFailures(f.run(t, "verify", 1)), tc.diagnosticPath)
			f.write(tc.path, old)
		})
	}
	for _, value := range []any{true, "0", -1, nil} {
		t.Run(fmt.Sprintf("index_%v", value), func(t *testing.T) {
			mo(ma(mf(f.m, "preserved_collections", "requirements")["records"])[0])["index"] = value
			f.save()
			f.run(t, "verify", 1)
		})
	}
}

func TestMaintenanceMissingGitIsAWarningNotTraceback(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	result := f.binary(t, "verify", 0, true)
	maintenanceEqual(t, result["observed_current_head"], "unavailable")
	found := false
	for _, w := range ma(result["warnings"]) {
		found = found || strings.Contains(strings.ToLower(ms(w)), "git")
	}
	if !found {
		t.Fatalf("missing git warning: %#v", result)
	}
}

func TestMaintenanceOutputCannotAliasAnyInput(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	for _, path := range []string{maintenanceManifest, maintenanceEvidence, "link_intent_test.go", maintenanceFirst(mo(f.m["source_files"]))} {
		for _, command := range maintenanceCommands {
			t.Run(command+"/"+path, func(t *testing.T) {
				before := f.read(path)
				maintenanceContains(t, maintenanceFailures(f.run(t, command, 1, "--output", f.path(path))), "protected input")
				maintenanceEqual(t, f.read(path), before)
			})
		}
	}
}

func TestMaintenanceSymlinkAndHardlinkOutputAliasesAreRejected(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			alias, target := f.path(kind+".json"), f.path(maintenanceManifest)
			if kind == "symlink" {
				maintenanceLink(t, target, alias)
			} else if err := os.Link(target, alias); err != nil {
				t.Fatal(err)
			}
			before := f.read(maintenanceManifest)
			for _, command := range maintenanceCommands {
				t.Run(command, func(t *testing.T) {
					f.run(t, command, 1, "--output", alias)
					maintenanceEqual(t, f.read(maintenanceManifest), before)
				})
			}
		})
	}
}

func TestMaintenanceCyclicRootReturnsJSONFailureWithoutTouchingOutput(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	root := f.path("cyclic-root")
	maintenanceLink(t, root, root)
	output := f.path("existing-output.json")
	maintenanceWrite(t, output, []byte("preserve existing output"))
	for _, command := range maintenanceCommands {
		for _, report := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/report_%t", command, report), func(t *testing.T) {
				args := []string{"--root", root}
				if report {
					args = append(args, "--output", output)
				}
				message := maintenanceFailures(f.run(t, command, 1, args...))
				maintenanceContains(t, message, "cannot resolve inventory root")
				maintenanceContains(t, message, root)
				maintenanceEqual(t, string(maintenanceRead(t, output)), "preserve existing output")
			})
		}
	}
}

func TestMaintenanceCyclicOutputSymlinkReturnsJSONFailure(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	alias := f.path("cycle.json")
	for _, command := range maintenanceCommands {
		t.Run(command, func(t *testing.T) {
			maintenanceLink(t, alias, alias)
			maintenanceContains(t, maintenanceFailures(f.run(t, command, 1, "--output", alias)), "cannot validate output path")
			maintenanceReadlink(t, alias, alias)
			maintenanceNoTemps(t, f.root, ".cycle.json.*")
			if err := os.Remove(alias); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMaintenanceIndirectAndParentCyclesFailWithoutChangingLinksOrEvidence(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	first, second := f.path("cycle-first"), f.path("cycle-second")
	f.mutateTest()
	before := f.read(maintenanceEvidence)
	output := f.path("preserved.json")
	maintenanceWrite(t, output, []byte("preserve existing output"))
	for _, command := range maintenanceCommands {
		for _, path := range []string{first, filepath.Join(first, "child")} {
			for _, kind := range []string{"root", "output"} {
				t.Run(command+"/"+filepath.Base(path)+"/"+kind, func(t *testing.T) {
					_ = os.Remove(first)
					_ = os.Remove(second)
					maintenanceLink(t, filepath.Base(second), first)
					maintenanceLink(t, filepath.Base(first), second)
					f.write(maintenanceEvidence, before)
					args := []string{}
					if command == "refresh" {
						args = append(args, "--write")
					}
					message := "cannot validate output path"
					if kind == "root" {
						args = append(args, "--root", path, "--output", output)
						message = "cannot resolve inventory root"
					} else {
						args = append(args, "--output", path)
					}
					maintenanceContains(t, maintenanceFailures(f.run(t, command, 1, args...)), message)
					maintenanceReadlink(t, first, filepath.Base(second))
					maintenanceReadlink(t, second, filepath.Base(first))
					maintenanceEqual(t, f.read(maintenanceEvidence), before)
					maintenanceEqual(t, string(maintenanceRead(t, output)), "preserve existing output")
					maintenanceNoTemps(t, f.root, ".cycle-first.*")
				})
			}
		}
	}
}

func TestMaintenanceMissingOutputAndDanglingSymlinkRemainSupported(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	missing := f.path("missing-target.json")
	for _, command := range maintenanceCommands {
		for _, kind := range []string{"missing", "dangling"} {
			t.Run(command+"/"+kind, func(t *testing.T) {
				output := f.path("new-result.json")
				if kind == "dangling" {
					maintenanceLink(t, filepath.Base(missing), output)
				}
				result := f.run(t, command, 0, "--output", output)
				maintenanceEqual(t, maintenanceJSON(t, maintenanceRead(t, output)), result)
				info, err := os.Lstat(output)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode()&os.ModeSymlink != 0 {
					t.Fatal("output is still a symlink")
				}
				if _, err = os.Stat(missing); !os.IsNotExist(err) {
					t.Fatalf("missing target was created: %v", err)
				}
				if err = os.Remove(output); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Direct I/O boundary and fault tests corresponding to methods 23, 26, 30,
// 31, 32, and 36 live in maintenance_fault_test.go.

func TestMaintenanceSymlinkedParentOutputCannotReplaceProtectedInput(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	alias := f.path("root-alias")
	maintenanceLink(t, f.root, alias)
	before := f.read(maintenanceManifest)
	for _, command := range maintenanceCommands {
		t.Run(command, func(t *testing.T) {
			maintenanceContains(t, maintenanceFailures(f.run(t, command, 1, "--output", filepath.Join(alias, maintenanceManifest))), "output aliases protected input")
			maintenanceEqual(t, f.read(maintenanceManifest), before)
			maintenanceReadlink(t, alias, f.root)
		})
	}
}

func TestMaintenanceIsolatedAndSafePathInvocationsUseTrustedBinary(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	// An executable must not import code or discover a Python interpreter in the
	// caller's working directory. These files are deliberately hostile decoys.
	for _, name := range []string{"inventory_common.py", "sitecustomize.py", "verify-package-inventory.py", "refresh-package-inventory.py", "test-package-inventory.py"} {
		f.write(name, []byte("raise RuntimeError(\"untrusted cwd import\")\n"))
	}
	for _, command := range maintenanceCommands {
		t.Run(command+"/empty_PATH", func(t *testing.T) { f.binary(t, command, 0, true) })
	}
}

func TestMaintenanceEvidenceAliasOfSourceCannotBeWritten(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	target := f.path("link_intent_test.go")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(f.path(maintenanceEvidence), target); err != nil {
		t.Fatal(err)
	}
	before := maintenanceRead(t, target)
	f.run(t, "refresh", 1, "--write")
	maintenanceEqual(t, maintenanceRead(t, target), before)
}

func TestMaintenanceOutputReplaceFailureReturnsJSONAndLeavesNoTemp(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	dest := f.path("directory-output")
	if err := os.Mkdir(dest, 0755); err != nil {
		t.Fatal(err)
	}
	maintenanceContains(t, maintenanceFailures(f.run(t, "verify", 1, "--output", dest)), "write")
	maintenanceNoTemps(t, f.root, ".directory-output.*")
}

func TestMaintenanceMissingOutputDirectoryReturnsJSONWithoutClobberingInputs(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	before := f.read(maintenanceEvidence)
	for _, command := range maintenanceCommands {
		t.Run(command, func(t *testing.T) {
			result := f.run(t, command, 1, "--output", f.path("absent/report.json"))
			maintenanceContains(t, maintenanceFailures(result), "cannot write JSON atomically")
			if result["status"] == "PASS" {
				t.Fatal("failed report claims PASS")
			}
			maintenanceEqual(t, f.read(maintenanceEvidence), before)
		})
	}
}

func TestMaintenanceRefreshChecksOutputAliasBeforeEvidenceWrite(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	f.mutateTest()
	before := f.read(maintenanceEvidence)
	for _, path := range []string{maintenanceManifest, maintenanceEvidence, "link_intent_test.go"} {
		t.Run(path, func(t *testing.T) {
			f.run(t, "refresh", 1, "--write", "--output", f.path(path))
			maintenanceEqual(t, f.read(maintenanceEvidence), before)
		})
	}
}

func TestMaintenanceParseFailureDoesNotOverwriteUndiscoveredInput(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	f.write(maintenanceManifest, []byte("{"))
	for _, command := range maintenanceCommands {
		t.Run(command, func(t *testing.T) {
			before := f.read("link_intent_test.go")
			f.run(t, command, 1, "--output", f.path("link_intent_test.go"))
			maintenanceEqual(t, f.read("link_intent_test.go"), before)
		})
	}
}

func TestMaintenanceUnreferencedTestRecordIsRejected(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	mo(f.e["tests"])["unused.go::TestUnused"] = map[string]any{"file": "unused.go", "function": "TestUnused", "file_sha256": strings.Repeat("0", 64)}
	f.write("unused.go", []byte("package musicxml\nfunc TestUnused(t *testing.T) {}\n"))
	f.save()
	f.run(t, "refresh", 1, "--write")
}

func maintenanceWorkflowText(raw []byte) string {
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func TestMaintenanceWorkflowLineEndings(t *testing.T) {
	t.Parallel()
	const lf = "jobs:\n  quality:\n    steps:\n      - name: Inventory\n"
	for _, newline := range []string{"\n", "\r\n"} {
		raw := strings.ReplaceAll(lf, "\n", newline)
		if got := maintenanceWorkflowText([]byte(raw)); got != lf {
			t.Fatalf("normalized workflow = %q, want %q", got, lf)
		}
	}
	for _, changed := range []string{
		strings.Replace(lf, "quality", "unexpected-job", 1),
		strings.Replace(lf, "jobs:", "jobs :", 1),
		strings.Replace(lf, "jobs:\n", "jobs:\r", 1),
	} {
		if maintenanceWorkflowText([]byte(changed)) == lf {
			t.Fatalf("normalization concealed workflow mutation: %q", changed)
		}
	}
}

func TestMaintenanceExistingLinuxJobsRequireMaintenanceWithoutNewGoStage(t *testing.T) {
	t.Parallel()
	repo := maintenanceRepo(t)
	for _, tc := range []struct {
		name string
		jobs []string
	}{{"ci.yml", []string{"quality", "test", "race", "xsd-conformance", "fuzz"}}, {"release.yml", []string{"verify", "publish"}}} {
		t.Run(tc.name, func(t *testing.T) {
			text := maintenanceWorkflowText(maintenanceRead(t, filepath.Join(repo, ".github/workflows", tc.name)))
			parts := strings.SplitN(text, "jobs:\n", 2)
			if len(parts) != 2 {
				t.Fatal("missing jobs mapping")
			}
			body := parts[1]
			var jobs []string
			for _, match := range regexp.MustCompile(`(?m)^  ([a-z-]+):$`).FindAllStringSubmatch(body, -1) {
				jobs = append(jobs, match[1])
			}
			sort.Strings(jobs)
			sort.Strings(tc.jobs)
			maintenanceEqual(t, jobs, tc.jobs)
			steps := strings.Split(body, "      - name:")
			var matching []string
			for _, step := range steps {
				if strings.Contains(step, "go test") && strings.Contains(step, "./internal/packageinventory") {
					matching = append(matching, step)
				}
			}
			if len(matching) != 1 {
				t.Fatalf("want one explicit inventory maintenance step; got %d", len(matching))
			}
			step := matching[0]
			maintenanceContains(t, step, "if: runner.os == 'Linux'")
			maintenanceContains(t, step, "go run ./cmd/package-inventory verify --strict-evidence")
			maintenanceContains(t, step, "go run ./cmd/package-inventory test")
			if strings.Contains(step, "python") {
				t.Fatalf("inventory CI step still requires Python: %s", step)
			}
		})
	}
	maintenanceContains(t, string(maintenanceRead(t, filepath.Join(repo, "scripts/check-all.sh"))), "all_stages=(mod generate format test vet race fuzz)")
}

func TestMaintenanceReviewedReconciliationOutputIsProtected(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	path := ms(mo(ma(f.m["reviewed_source_corrections"])[0])["reconciliation"])
	before := f.read(path)
	for _, command := range maintenanceCommands {
		t.Run(command, func(t *testing.T) {
			f.write(path, before)
			maintenanceContains(t, maintenanceFailures(f.run(t, command, 1, "--output", f.path(path))), "protected input")
			maintenanceEqual(t, f.read(path), before)
		})
	}
}

func TestMaintenanceMalformedReviewedRoutesCannotWriteEvidenceOrOutput(t *testing.T) {
	t.Parallel()
	for _, routes := range []any{nil, map[string]any{}, []any{"not a record"}, []any{map[string]any{}}, []any{map[string]any{"reconciliation": 7}}} {
		t.Run(fmt.Sprintf("%v", routes), func(t *testing.T) {
			f := newMaintenanceFixture(t)
			output := f.path("result.json")
			maintenanceWrite(t, output, []byte("existing result"))
			f.m["reviewed_source_corrections"] = routes
			f.save()
			f.mutateTest()
			before := f.read(maintenanceEvidence)
			result := f.run(t, "refresh", 1, "--write", "--output", output)
			maintenanceEqual(t, result["written"], false)
			maintenanceEqual(t, f.read(maintenanceEvidence), before)
			maintenanceEqual(t, string(maintenanceRead(t, output)), "existing result")
		})
	}
}

func maintenanceDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestMaintenanceReviewedAuxiliarySchemaDiffIsNarrowAndReproducible(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	review := maintenanceJSON(t, maintenanceRead(t, filepath.Join(maintenanceRepo(t), "docs/validation/evidence/auxiliary-fixture-reference-review.json")))
	maintenanceEqual(t, review["review_comment_id"], json.Number("4255983792"))
	source := bytes.ReplaceAll(f.read(ms(review["source"])), []byte("\r\n"), []byte("\n"))
	maintenanceEqual(t, maintenanceDigest(source), review["after_sha256"])
	maintenanceEqual(t, json.Number(fmt.Sprint(len(source))), review["after_bytes"])
	original := maintenanceJSON(t, source)
	records := map[string]map[string]any{}
	for _, v := range ma(review["records"]) {
		r := mo(v)
		records[ms(r["instance_id"])] = r
	}
	maintenanceEqual(t, len(records), 36)
	rows := ma(original["xsd_instances"])
	maintenanceEqual(t, len(rows), len(records))
	for _, v := range rows {
		row := mo(v)
		record, ok := records[ms(row["instance_id"])]
		if !ok {
			t.Fatalf("missing reviewed instance %s", row["instance_id"])
		}
		maintenanceEqual(t, row["research_id"], record["research_id"])
		maintenanceEqual(t, maintenanceCanonicalDigest(t, row), record["after_record_sha256"])
		maintenanceEqual(t, record["changes"], []any{map[string]any{"field": "fixture_axes.missing_fact", "before": []any{"TestAuxiliarySourceUnavailableEvidence"}, "after_field_absent": true}, map[string]any{"field": "missing_fact_test", "before_field_absent": true, "after": "TestAuxiliarySourceUnavailableEvidence"}})
		maintenanceEqual(t, row["missing_fact_test"], "TestAuxiliarySourceUnavailableEvidence")
		delete(row, "missing_fact_test")
		axes := mo(row["fixture_axes"])
		if _, exists := axes["missing_fact"]; exists {
			t.Fatal("missing_fact remained inside fixture_axes")
		}
		axes["missing_fact"] = []any{"TestAuxiliarySourceUnavailableEvidence"}
		maintenanceEqual(t, maintenanceCanonicalDigest(t, row), record["before_record_sha256"])
	}
	// Retain the original insertion order while reversing exactly the reviewed
	// relocation; Go's ordinary map serializer sorts keys and cannot reproduce it.
	restored := maintenanceRestoreReviewedBytes(t, source)
	maintenanceEqual(t, maintenanceJSON(t, restored), original)
	maintenanceEqual(t, maintenanceDigest(restored), review["before_sha256"])
	maintenanceEqual(t, json.Number(fmt.Sprint(len(restored))), review["before_bytes"])
	maintenanceEqual(t, mo(f.m["source_files"])[ms(review["source"])], map[string]any{"sha256": review["after_sha256"], "bytes": review["after_bytes"]})
}

func maintenanceCanonicalDigest(t *testing.T, value any) string {
	t.Helper()
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		t.Fatal(err)
	}
	// encoding/json escapes these two Unicode line separators even with HTML
	// escaping disabled; Python's ensure_ascii=False canonical form retains them.
	data := bytes.TrimSuffix(out.Bytes(), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte(`\u2028`), []byte("\u2028"))
	data = bytes.ReplaceAll(data, []byte(`\u2029`), []byte("\u2029"))
	return maintenanceDigest(data)
}
func maintenanceRestoreReviewedBytes(t *testing.T, source []byte) []byte {
	t.Helper()
	text := string(source)

	// fixture_axes ends immediately before runtime_test. The previous final
	// array closes with eight spaces; insert its comma and the removed final axis.
	before := "        ]\n      },\n      \"runtime_test\":"
	after := "        ],\n        \"missing_fact\": [\n          \"TestAuxiliarySourceUnavailableEvidence\"\n        ]\n      },\n      \"runtime_test\":"
	if count := strings.Count(text, before); count != 36 {
		t.Fatalf("fixture_axes insertion points = %d, want 36", count)
	}
	text = strings.ReplaceAll(text, before, after)
	last := ",\n      \"missing_fact_test\": \"TestAuxiliarySourceUnavailableEvidence\""
	if count := strings.Count(text, last); count != 36 {
		t.Fatalf("missing_fact_test removal points = %d, want 36", count)
	}
	return []byte(strings.ReplaceAll(text, last, ""))
}

func TestMaintenanceHistoricalObservationDoesNotCompareCurrentHead(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed; empty-PATH behavior is tested separately")
	}
	for _, args := range [][]string{{"init", "--quiet"}, {"-c", "user.name=Inventory Test", "-c", "user.email=inventory@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "published head"}} {
		cmd := exec.Command(git, args...)
		cmd.Dir = f.root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	cmd := exec.Command(git, "rev-parse", "HEAD")
	cmd.Dir = f.root
	head, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	result := f.run(t, "verify", 0)
	maintenanceEqual(t, result["failures"], []any{})
	maintenanceEqual(t, result["observed_current_head"], strings.TrimSpace(string(head)))
	history := mo(result["historical_observation"])
	maintenanceEqual(t, history["head"], mo(f.m["snapshot"])["head_at_end"])
	maintenanceContains(t, ms(history["interpretation"]), "historical")
	for _, warning := range ma(result["warnings"]) {
		if strings.Contains(ms(warning), "Integration differs") {
			t.Fatalf("historical snapshot compared with current HEAD: %s", warning)
		}
	}
}

func TestMaintenanceAuxiliaryFixtureRoutesMatchTypedOriginal(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"unknown fixture", "missing fixture", "wrong axis", "function as fixture", "wrong function"} {
		t.Run(mutation, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			axes := mo(mo(ma(f.e["auxiliary_xsd_axes"])[0])["axes"])
			switch mutation {
			case "unknown fixture":
				mo(axes["positive"])["case_labels"] = []any{"unknown fixture"}
			case "missing fixture":
				mo(axes["positive"])["case_labels"] = []any{}
			case "wrong axis":
				mo(axes["positive"])["case_labels"] = mo(axes["negative"])["case_labels"]
			case "function as fixture":
				mo(axes["missing_fact"])["case_labels"] = []any{"TestAuxiliarySourceUnavailableEvidence"}
			case "wrong function":
				mo(axes["missing_fact"])["tests"] = mo(axes["positive"])["tests"]
			}
			f.save()
			maintenanceContains(t, maintenanceFailures(f.run(t, "verify", 1)), "auxiliary fixture route")
		})
	}
}

func TestMaintenanceHistoricalHashesAreNeverRefreshed(t *testing.T) {
	t.Parallel()
	f := newMaintenanceFixture(t)
	tests := mo(f.e["historical_tests"])
	mo(tests[maintenanceFirst(tests)])["file_sha256"] = strings.Repeat("f", 64)
	f.save()
	f.mutateTest()
	f.run(t, "refresh", 0, "--write")
	actual := maintenanceJSON(t, f.read(maintenanceEvidence))
	for _, field := range []string{"historical_tests", "execution_results", "snapshot", "final_acceptance"} {
		t.Run(field, func(t *testing.T) { maintenanceEqual(t, actual[field], f.e[field]) })
	}
}

// Assert the API does not depend on an os.File writer; callers can embed it.
var _ func([]string, io.Writer, io.Writer) int = Run
