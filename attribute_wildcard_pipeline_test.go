package musicxml

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

var attributeWildcardOracleTests = []string{"TestAttributeWildcardUnionAgainstXMLLint", "TestAttributeWildcardUnexpressibleUnionXMLLintSchemaError"}

func attributeWildcardOracleGate(workflow string) error {
	ci := strings.ReplaceAll(workflow, "\r\n", "\n")
	_, job, ok := strings.Cut(ci, "\n  xsd-conformance:\n")
	if !ok {
		return fmt.Errorf("missing external oracle job")
	}
	job, _, ok = strings.Cut(job, "\n  fuzz:\n")
	if !ok || !strings.Contains(job, `MUSICXML_REQUIRE_XMLLINT: "1"`) || !strings.Contains(job, "libxml2-utils") {
		return fmt.Errorf("missing required real xmllint setup")
	}
	command := regexp.MustCompile(`(?m)^\s*run: go test .* -run='([^']+)' \.$`).FindStringSubmatch(job)
	if len(command) != 2 {
		return fmt.Errorf("missing external oracle selection")
	}
	selected, err := regexp.Compile(command[1])
	if err != nil {
		return err
	}
	for _, name := range attributeWildcardOracleTests {
		if !selected.MatchString(name) {
			return fmt.Errorf("required oracle omits %s", name)
		}
	}
	return nil
}
func TestAttributeWildcardRequiredCIGate(t *testing.T) {
	raw, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	lf := strings.ReplaceAll(string(raw), "\r\n", "\n")
	for _, newline := range []string{"\n", "\r\n"} {
		if err := attributeWildcardOracleGate(strings.ReplaceAll(lf, "\n", newline)); err != nil {
			t.Error(err)
		}
	}
}
func TestAttributeWildcardCIGateDetectsOmission(t *testing.T) {
	raw, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	lf := strings.ReplaceAll(string(raw), "\r\n", "\n")
	for _, changed := range []string{
		strings.Replace(lf, attributeWildcardOracleTests[0], "TestDifferentOracle", 1),
		strings.Replace(lf, attributeWildcardOracleTests[1], "TestDifferentOracle", 1),
		strings.Replace(lf, `MUSICXML_REQUIRE_XMLLINT: "1"`, `MUSICXML_REQUIRE_XMLLINT: "0"`, 1),
		strings.Replace(lf, "libxml2-utils", "different-package", 1),
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			if err := attributeWildcardOracleGate(strings.ReplaceAll(changed, "\n", newline)); err == nil {
				t.Fatal("accepted omitted required oracle/setup")
			}
		}
	}
}
func TestAttributeWildcardOracleRequiredToolGuard(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range attributeWildcardOracleTests {
		for _, required := range []bool{false, true} {
			command := exec.Command(executable, "-test.run=^"+name+"$", "-test.v")
			flag := "0"
			if required {
				flag = "1"
			}
			command.Env = append(os.Environ(), "PATH="+t.TempDir(), "MUSICXML_REQUIRE_XMLLINT="+flag)
			output, err := command.CombinedOutput()
			if required {
				if err == nil || !strings.Contains(string(output), "--- FAIL: "+name) || strings.Contains(string(output), "--- SKIP:") {
					t.Fatalf("missing xmllint must fail %s: %v\n%s", name, err, output)
				}
			} else if err != nil || !strings.Contains(string(output), "--- SKIP: "+name) {
				t.Fatalf("optional xmllint must skip %s: %v\n%s", name, err, output)
			}
		}
	}
}
