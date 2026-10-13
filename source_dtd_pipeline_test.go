package musicxml

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestSourceInternalDTDRequiredCIGates(t *testing.T) {
	workflow, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("scripts/check-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	lf := strings.ReplaceAll(string(workflow), "\r\n", "\n")
	for _, input := range []string{lf, strings.ReplaceAll(lf, "\n", "\r\n")} {
		ci := strings.ReplaceAll(input, "\r\n", "\n")
		_, oracle, found := strings.Cut(ci, "\n  xsd-conformance:\n")
		if !found {
			t.Fatal("required external oracle job missing")
		}
		oracle, _, found = strings.Cut(oracle, "\n  fuzz:\n")
		if !found || !strings.Contains(oracle, `MUSICXML_REQUIRE_XMLLINT: "1"`) || !strings.Contains(oracle, "libxml2-utils") {
			t.Fatal("required real xmllint installation/selection missing")
		}
		command := regexp.MustCompile(`(?m)^\s*run: go test .* -run='([^']+)' \.$`).FindStringSubmatch(oracle)
		if len(command) != 2 {
			t.Fatal("external oracle test selection missing")
		}
		selected, err := regexp.Compile(command[1])
		if err != nil {
			t.Fatal(err)
		}
		if !selected.MatchString("TestSourceInternalDTDAgainstXMLLint") {
			t.Error("required external oracle job omits the internal-DTD differential")
		}
		_, fuzzJob, found := strings.Cut(ci, "\n  fuzz:\n")
		if !found || !strings.Contains(fuzzJob, "run: bash scripts/check-all.sh fuzz") {
			t.Fatal("remote fuzz job does not execute the shared counted stage")
		}
	}
	_, stage, found := strings.Cut(strings.ReplaceAll(string(script), "\r\n", "\n"), "stage_fuzz() {\n")
	if !found {
		t.Fatal("counted fuzz stage missing")
	}
	stage, _, found = strings.Cut(stage, "\n}")
	if !found || !strings.Contains(stage, "-fuzz='^FuzzSourceInternalDTDFrames$' \\\n        -fuzztime=\"$check_validation_fuzz_time\" .") {
		t.Fatal("internal-DTD frame target must run with the shared finite count")
	}
}
