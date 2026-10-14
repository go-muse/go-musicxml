package packageinventory

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopLevelHelpListsCommandsAndOptionsWithoutInputs(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"-h", "--help", "help"} {
		t.Run(arg, func(t *testing.T) {
			root := t.TempDir()
			output := filepath.Join(root, "must-not-exist.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{arg, "--root", filepath.Join(root, "missing"), "--output", output, "--write"}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
			}
			if stdout.Len() != 0 {
				t.Fatalf("help wrote a JSON result: %s", &stdout)
			}
			for _, want := range []string{"usage: package-inventory {verify|refresh|test} [options]", "package-inventory verify [options]", "package-inventory refresh [options]", "package-inventory test [options]", "-strict-evidence", "-archive-dir", "-write", "-root", "-manifest", "-evidence", "-output"} {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("help missing %q: %s", want, &stderr)
				}
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("help touched requested result output: %v", err)
			}
		})
	}
}

func TestSubcommandHelpKeepsCommandSpecificFlags(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"verify", "refresh", "test"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run([]string{command, "--help"}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, &stderr)
			}
			for flag, want := range map[string]bool{"-strict-evidence": command == "verify", "-archive-dir": command != "test", "-write": command == "refresh"} {
				if got := strings.Contains(stderr.String(), flag); got != want {
					t.Errorf("command %s flag %s listed=%t, want %t", command, flag, got, want)
				}
			}
		})
	}
}

func TestInvalidCommandStillExitsTwo(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"unknown"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("args=%v exit=%d stdout=%q stderr=%q", args, code, &stdout, &stderr)
		}
	}
}
