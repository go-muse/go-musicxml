package musicxml

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The external oracle job selects tests explicitly: adding a test to the Go
// suite alone must not leave its required-xmllint branch unexecuted in CI.
func TestPackageVerificationPipeline(t *testing.T) {
	raw, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := strings.ReplaceAll(string(raw), "\r\n", "\n")
	command := regexp.MustCompile(`(?m)^\s*run: go test .* -run='([^']+)' \.$`).FindStringSubmatch(ci)
	if len(command) != 2 {
		t.Fatal("required external oracle selection not found")
	}
	selected, err := regexp.Compile(command[1])
	if err != nil {
		t.Fatal(err)
	}
	if !selected.MatchString("TestAuxiliarySourceAgainstOriginalSchema") {
		t.Error("required external oracle job omits original-byte auxiliary schema differential test")
	}
	for _, path := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range []string{"python3 scripts/verify-package-inventory.py --strict-evidence", "python3 scripts/test-package-inventory.py"} {
			if !strings.Contains(string(raw), command) {
				t.Errorf("%s omits %s", path, command)
			}
		}
	}
}
