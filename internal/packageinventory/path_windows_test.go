package packageinventory

import (
	"os"
	"path/filepath"
	"strings"
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
