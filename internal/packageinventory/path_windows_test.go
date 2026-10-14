package packageinventory

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// These spellings use Windows drive-current-directory rules even though
// filepath.IsAbs returns false. Native Windows CI executes these regressions.
func TestWindowsRelativePathAnchors(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	volume := filepath.VolumeName(cwd)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skip("requires a drive-letter working directory")
	}
	for _, input := range []string{volume + `inventory-missing\report.json`, `\inventory-missing\report.json`, `/inventory-missing/report.json`} {
		want, err := filepath.Abs(input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := checkedResolve(input, defaultFileOps())
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("resolve(%q)=%q, want %q", input, got, want)
		}
		if err := rejectAlias(input, []string{want}, defaultFileOps()); err == nil {
			t.Errorf("missing-path alias accepted: %q / %q", input, want)
		}
	}
}

func TestWindowsExplicitInputPathVolumes(t *testing.T) {
	f := newMaintenanceFixture(t)
	volume := filepath.VolumeName(f.root)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skip("requires a drive-letter temporary directory")
	}
	for _, kind := range []string{"manifest", "evidence"} {
		source := maintenanceManifest
		if kind == "evidence" {
			source = maintenanceEvidence
		}
		for _, input := range []string{volume + filepath.FromSlash(source), f.path(source)[len(volume):]} {
			f.run(t, "verify", 0, "--"+kind, input)
		}
	}
	for _, c := range []struct{ base, value, want string }{{`C:\root`, `C:child`, `C:\root\child`}, {`C:\root`, `D:child`, `D:child`}, {`C:\root`, `\child`, `C:\child`}, {`C:\root`, `C:\child`, `C:\child`}} {
		if got := joinUnclean(c.base, c.value); got != c.want {
			t.Errorf("join(%q,%q)=%q want %q", c.base, c.value, got, c.want)
		}
	}
}

func TestWindowsMissingPathAliasesIgnoreCase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Missing-Result.json")
	if err := rejectAlias(path, []string{strings.ToUpper(path)}, defaultFileOps()); err == nil {
		t.Fatal("case-insensitive missing-path alias accepted")
	}
}

func windowsShortPath(t *testing.T, path string) string {
	t.Helper()
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 256)
	for {
		count, err := syscall.GetShortPathName(pointer, &buffer[0], uint32(len(buffer)))
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatal("GetShortPathName returned an empty path")
		}
		if int(count) < len(buffer) {
			return syscall.UTF16ToString(buffer[:count])
		}
		buffer = make([]uint16, count+1)
	}
}

func TestWindowsMissingOutputShortNameAliases(t *testing.T) {
	t.Parallel()
	root, nested, _ := physicalTraversalFixture(t)
	short := windowsShortPath(t, root)
	long, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("native short path=%q, long path=%q", short, long)
	// Hosts that disable new 8.3 names still run every check; on the GitHub runner
	// the existing RUNNER~1 user-directory alias exercises real short expansion.
	for _, spelling := range []string{short, strings.ToUpper(long)} {
		resolved, err := checkedResolve(spelling, defaultFileOps())
		if err != nil || resolved != long {
			t.Fatalf("resolved=%q, want long form %q: %v", resolved, long, err)
		}
		for _, suffix := range []string{"report.json", filepath.Join("missing", "nested", "report.json")} {
			output, protected := filepath.Join(spelling, suffix), filepath.Join(long, suffix)
			if err := rejectAlias(output, []string{protected}, defaultFileOps()); err == nil || !strings.Contains(err.Error(), "protected input") {
				t.Fatalf("missing short/long alias accepted: %q / %q: %v", output, protected, err)
			}
			if err := rejectAlias(output, []string{filepath.Join(t.TempDir(), suffix)}, defaultFileOps()); err != nil {
				t.Fatalf("wrong-directory negative control: %v", err)
			}
			if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("output changed: %v", err)
			}
		}
		// Native normalization must happen after, not before, following alias/..
		// into nested. filepath.Join here would deliberately destroy the fixture.
		output := spelling + `\alias\..\missing\report.json`
		longNested, err := filepath.EvalSymlinks(nested)
		if err != nil {
			t.Fatal(err)
		}
		if err := rejectAlias(output, []string{filepath.Join(longNested, "missing", "report.json")}, defaultFileOps()); err == nil {
			t.Fatal("physical-parent short-name alias accepted")
		}
		if err := rejectAlias(output, []string{filepath.Join(long, "missing", "report.json")}, defaultFileOps()); err != nil {
			t.Fatalf("lexical-parent negative control: %v", err)
		}
	}
}
