package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

const prefixBindingScore = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0">
  <part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>
  <part id="P1">
    <measure number="1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>4</duration></note></measure>
    MIDDLE
    <measure number="2"><note><rest/><duration>4</duration></note></measure>
    TRAILING
  </part>
</score-partwise>`

// prefixBindingBlocked returns the source lines of the measure elements that
// the source line reports blocked because no declaration reaches them
// (measure 1 is on line 5, measure 2 on line 7), and whether the part's
// content-model failure is reported.
func prefixBindingBlocked(t *testing.T, middle, trailing string) (map[int]bool, bool) {
	t.Helper()
	text := strings.NewReplacer("MIDDLE", middle, "TRAILING", trailing).Replace(prefixBindingScore)
	report, err := InspectXML(context.Background(), strings.NewReader(text), SourceOptions{Profile: ProfileSourceLine})
	if err != nil {
		t.Fatal(err)
	}
	blocked := map[int]bool{}
	failed := false
	for _, f := range report.Findings {
		if f.Outcome == validation.OutcomeFail && f.Reason == "content-model" && strings.HasPrefix(f.Location.Path, "/score-partwise/part") {
			failed = true
		}
		if f.Outcome == validation.OutcomeBlocked && f.Reason == "exact-declaration-use-unavailable" && strings.HasPrefix(f.Location.Path, "/score-partwise/part/measure") {
			if f.Location.Position == nil {
				t.Fatalf("blocked finding without a position: %+v", f)
			}
			blocked[f.Location.Position.Line] = true
		}
	}
	if report.Conformance != validation.ConformanceViolated {
		t.Fatalf("conformance %s", report.Conformance)
	}
	return blocked, failed
}

// The source line binds every child that precedes the first child its
// parent's content model rejects, whatever follows: a bogus element after the
// last measure leaves every measure assessed, a bogus element between two
// measures leaves the first assessed and the second without a declaration,
// and the part's content-model failure is reported in both cases.
func TestSourceLineBindsChildrenBeforeAContentFailure(t *testing.T) {
	blocked, failed := prefixBindingBlocked(t, "", "<bogus/>")
	if !failed || len(blocked) != 0 {
		t.Fatalf("trailing bogus element: content failure %v, blocked %v", failed, blocked)
	}
	blocked, failed = prefixBindingBlocked(t, "<bogus/>", "")
	if !failed || len(blocked) != 1 || !blocked[7] {
		t.Fatalf("bogus element between measures: content failure %v, blocked %v", failed, blocked)
	}
}

// Children bound before a content failure take part in identity: two
// measures with the same ID before a stray element are reported as a
// duplicate ID.
func TestSourceLineFindsDuplicateIDsBeforeAContentFailure(t *testing.T) {
	text := strings.NewReplacer("MIDDLE", "", "TRAILING", "<bogus/>", `<measure number="1">`, `<measure number="1" id="m">`, `<measure number="2">`, `<measure number="2" id="m">`).Replace(prefixBindingScore)
	report, err := InspectXML(context.Background(), strings.NewReader(text), SourceOptions{Profile: ProfileSourceLine})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Findings {
		if f.Outcome == validation.OutcomeFail && f.Reason == "ID" && strings.Contains(f.Actual, `duplicate ID "m"`) {
			return
		}
	}
	t.Fatalf("no duplicate-ID finding among %d findings", len(report.Findings))
}
