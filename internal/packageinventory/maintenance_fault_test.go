package packageinventory

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain removes the one compiled executable shared by the subprocess cases.
func TestMain(m *testing.M) {
	code := m.Run()
	if maintenanceBinaryPath != "" {
		_ = os.RemoveAll(filepath.Dir(maintenanceBinaryPath))
	}
	os.Exit(code)
}

func TestMaintenanceDanglingProtectedAliasAndCyclicInputAreRejected(t *testing.T) {
	f := newMaintenanceFixture(t)
	output, dangling, cycle := f.path("new-result.json"), f.path("dangling.json"), f.path("cyclic-input.json")
	maintenanceLink(t, filepath.Base(output), dangling)
	maintenanceLink(t, filepath.Base(cycle), cycle)
	for _, tc := range []struct{ path, message, target string }{
		{dangling, "output aliases protected input", filepath.Base(output)},
		{cycle, "cannot validate output path", filepath.Base(cycle)},
	} {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			err := rejectAlias(output, []string{tc.path}, defaultFileOps())
			if err == nil {
				t.Fatal("unsafe output unexpectedly accepted")
			}
			maintenanceContains(t, err.Error(), tc.message)
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("output was created: %v", err)
			}
			maintenanceReadlink(t, tc.path, tc.target)
		})
	}
}

func TestMaintenancePathProbeErrorsFailClosedBeforeAnyOutputWrite(t *testing.T) {
	for _, injected := range []error{os.ErrPermission, errors.New("path probe failed")} {
		t.Run(injected.Error(), func(t *testing.T) {
			f := newMaintenanceFixture(t)
			ops := defaultFileOps()
			ops.stat = func(string) (os.FileInfo, error) { return nil, injected }
			created := false
			ops.createTemp = func(string, string) (atomicFile, error) {
				created = true
				return nil, errors.New("temporary file must not be created")
			}
			if _, err := resolveRoot(f.root, ops); err == nil {
				t.Fatal("root probe failure was ignored")
			} else {
				maintenanceContains(t, err.Error(), "cannot resolve inventory root")
			}
			var stdout, stderr bytes.Buffer
			r := runner{fs: ops}
			code := r.run([]string{"verify", "--root", f.root, "--output", f.path("new-result.json")}, &stdout, &stderr)
			result := maintenanceResult(t, stdout.Bytes(), stderr.Bytes(), code, 1)
			maintenanceContains(t, maintenanceFailures(result), "cannot validate output path")
			if created {
				t.Fatal("path probe failure reached temporary-file creation")
			}
		})
	}
}

func TestMaintenanceRefreshWriteFailureDoesNotClaimSuccess(t *testing.T) {
	f := newMaintenanceFixture(t)
	f.mutateTest()
	before := f.read(maintenanceEvidence)
	ops := defaultFileOps()
	ops.createTemp = func(string, string) (atomicFile, error) { return nil, errors.New("injected write failure") }
	r := runner{fs: ops}
	var stdout, stderr bytes.Buffer
	code := r.run([]string{"refresh", "--root", f.root, "--write"}, &stdout, &stderr)
	result := maintenanceResult(t, stdout.Bytes(), stderr.Bytes(), code, 1)
	maintenanceEqual(t, result["written"], false)
	maintenanceContains(t, maintenanceFailures(result), "injected write failure")
	maintenanceEqual(t, f.read(maintenanceEvidence), before)
}

type maintenanceFaultyFile struct {
	atomicFile
	operation string
}

func (f *maintenanceFaultyFile) Write(p []byte) (int, error) {
	if f.operation == "write" {
		n, err := f.atomicFile.Write(p[:3])
		if err != nil {
			return n, err
		}
		return n, errors.New("injected write failure")
	}
	return f.atomicFile.Write(p)
}
func (f *maintenanceFaultyFile) Flush() error {
	if f.operation == "flush" {
		return errors.New("injected flush failure")
	}
	return f.atomicFile.Flush()
}
func (f *maintenanceFaultyFile) Sync() error {
	if f.operation == "fsync" {
		return errors.New("fsync failed")
	}
	return f.atomicFile.Sync()
}
func (f *maintenanceFaultyFile) Close() error {
	err := f.atomicFile.Close()
	if f.operation == "close" {
		return errors.New("injected close failure")
	}
	return err
}

func TestMaintenanceAtomicWriteFailuresPreserveOriginalAndReportCleanup(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "result.json")
	maintenanceWrite(t, dest, []byte("unchanged"))
	for _, operation := range []string{"replace", "fsync"} {
		t.Run(operation, func(t *testing.T) {
			ops := defaultFileOps()
			if operation == "replace" {
				ops.rename = func(string, string) error { return errors.New("replace failed") }
			} else {
				create := ops.createTemp
				ops.createTemp = func(dir, pattern string) (atomicFile, error) {
					f, err := create(dir, pattern)
					if err != nil {
						return nil, err
					}
					return &maintenanceFaultyFile{f, operation}, nil
				}
			}
			err := atomicWrite(dest, []byte("{\"new\":true}\n"), ops)
			if err == nil {
				t.Fatal("injected failure was ignored")
			}
			maintenanceContains(t, err.Error(), operation+" failed")
			maintenanceEqual(t, string(maintenanceRead(t, dest)), "unchanged")
			maintenanceNoTemps(t, root, ".result.json.*")
		})
	}
	t.Run("replace and cleanup", func(t *testing.T) {
		ops := defaultFileOps()
		ops.rename = func(string, string) error { return errors.New("replace failed") }
		ops.remove = func(string) error { return errors.New("cleanup failed") }
		err := atomicWrite(dest, []byte("{\"new\":true}\n"), ops)
		if err == nil {
			t.Fatal("injected failures were ignored")
		}
		maintenanceContains(t, err.Error(), "cleanup failed")
		maintenanceContains(t, err.Error(), "replace failed")
		maintenanceEqual(t, string(maintenanceRead(t, dest)), "unchanged")
	})
}

func TestMaintenanceAtomicPartialWriteFlushAndCloseFailuresCleanUp(t *testing.T) {
	for _, operation := range []string{"write", "flush", "close"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			dest := filepath.Join(root, "result.json")
			maintenanceWrite(t, dest, []byte("unchanged"))
			ops := defaultFileOps()
			create := ops.createTemp
			ops.createTemp = func(dir, pattern string) (atomicFile, error) {
				f, err := create(dir, pattern)
				if err != nil {
					return nil, err
				}
				return &maintenanceFaultyFile{f, operation}, nil
			}
			err := atomicWrite(dest, []byte("{\"new\":true}\n"), ops)
			if err == nil {
				t.Fatal("injected failure was ignored")
			}
			maintenanceContains(t, err.Error(), "injected "+operation+" failure")
			maintenanceEqual(t, string(maintenanceRead(t, dest)), "unchanged")
			maintenanceNoTemps(t, root, ".result.json.*")
		})
	}
}

func TestMaintenanceHarnessParseFailureIncludesExitAndStderr(t *testing.T) {
	// Preserve the diagnostic boundary even when controls run the Go verifier
	// in-process: failed and malformed verifier results retain the case and exit.
	for _, tc := range []struct {
		name, stdout     string
		actual, expected int
	}{
		{"invalid JSON", "", 23, 0},
		{"invalid result shape", "[]", 23, 0},
		{"missing failures", "{}", 23, 0},
		{"wrong exit", "{\"failures\":[]}", 23, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseVerifierResult([]byte(tc.stdout), []byte("specific tool failure"), tc.actual, "probe", tc.expected)
			if err == nil {
				t.Fatal("invalid verifier output accepted")
			}
			maintenanceContains(t, err.Error(), "specific tool failure")
			maintenanceContains(t, err.Error(), fmt.Sprint(tc.actual))
			maintenanceContains(t, err.Error(), "probe")
		})
	}
}
