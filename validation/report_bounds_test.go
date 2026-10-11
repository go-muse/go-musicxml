package validation

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestReportFormatterLocalizedTextCannotReplaceMetadata(t *testing.T) {
	rule := reportTestRule()
	r := Report{Rules: []RuleDefinition{rule}, Findings: []Finding{{Decision: reportTestDecision(OutcomeFail), Category: CategoryNormative, Location: Location{Target: TargetModel, DocumentID: "doc", Path: "/root"}}}}
	var out bytes.Buffer
	if err := r.FormatLocalized(&out, map[RuleKey]RuleText{rule.Key: {Title: "Localized title", Explanation: "Localized explanation", Message: "Localized message"}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Localized title", "Localized explanation", "Localized message", "report.rule", "https://example.org/spec", "normative", "/root"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %q", want, out.String())
		}
	}
	for _, translation := range []RuleText{{}, {Title: "partial"}, {Title: "bad\x1b", Explanation: "bad", Message: "bad"}, {Title: strings.Repeat("x", 100000), Explanation: "bad", Message: "bad"}} {
		out.Reset()
		if err := r.FormatLocalized(&out, map[RuleKey]RuleText{rule.Key: translation}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), rule.Text.Explanation) || strings.Contains(out.String(), "\x1b") {
			t.Fatal("invalid translation did not safely fall back to English")
		}
	}
}

func TestReportFormatterLabelsActualSourceCoordinateStream(t *testing.T) {
	for _, coordinates := range []string{"original-bytes", "utf8-transcoded"} {
		r := Report{Findings: []Finding{{Decision: reportTestDecision(OutcomeFail), Location: Location{Target: TargetSource, Position: &SourcePosition{ByteOffset: 90, Line: 4, Column: 9, CoordinateSpace: coordinates}}}}}
		var out bytes.Buffer
		if err := r.Format(&out); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{coordinates, "line=4", "column=9", "byte=90"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("missing source coordinate %q", want)
			}
		}
	}
}

func FuzzReportFieldIsBoundedEscapedUTF8(f *testing.F) {
	for _, seed := range []string{"normal", "\x00\x1b\n\r\t\u202e\u2067", "\xff\xfe\"\\", strings.Repeat("é", 10000), strings.Repeat("\x00", 10000)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		encoded := reportField(input)
		if len(encoded) > reportFormatFieldBytes || !utf8.ValidString(encoded) {
			t.Fatal("field violated output bounds/encoding")
		}
		if _, err := strconv.Unquote(encoded); err != nil {
			t.Fatalf("malformed escaped field: %v", err)
		}
		for _, ch := range encoded {
			if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) || ch == '\u2028' || ch == '\u2029' {
				t.Fatal("raw hostile character escaped formatter")
			}
		}
	})
}

func TestReportFormatterBoundsFabricatedInputAndEscapesLocations(t *testing.T) {
	hostile := "\nforged\r\t\x00\x1b[31m\u202e\u2066"
	rule := reportTestRule()
	rule.Text.Explanation = strings.Repeat("x", 2<<20)
	rule.Sources = []SourceRef{{URL: "https://example.org/" + hostile}}
	d := reportTestDecision(OutcomeFail)
	d.Actual = hostile
	d.Expected = strings.Repeat("y", 2<<20)
	f := Finding{Decision: d, Category: CategoryNormative, Location: Location{Target: TargetModel, DocumentID: hostile, Entry: hostile, Path: "/root" + hostile, Position: &SourcePosition{Line: 400, Column: 3, ByteOffset: 10}}}
	r := Report{Rules: []RuleDefinition{rule}, Findings: []Finding{f}}
	var out bytes.Buffer
	if err := r.Format(&out); err != nil {
		t.Fatal(err)
	}
	if out.Len() > 65536 {
		t.Fatalf("one finding rendered unbounded fields: %d bytes", out.Len())
	}
	for _, unsafe := range []string{"\x00", "\x1b", "\r", "\t", "\u202e", "\u2066", "\nforged"} {
		if strings.Contains(out.String(), unsafe) {
			t.Errorf("raw hostile sequence %q", unsafe)
		}
	}
	if strings.Contains(out.String(), "line=400") || strings.Contains(out.String(), "byte=10") {
		t.Fatal("model diagnostic invented source coordinates")
	}
	r.Findings = make([]Finding, 100000)
	for i := range r.Findings {
		r.Findings[i] = f
	}
	out.Reset()
	if err := r.Format(&out); err != nil {
		t.Fatal(err)
	}
	if out.Len() > 1<<20 || !strings.Contains(out.String(), "truncated") {
		t.Fatalf("aggregate render was not bounded: %d", out.Len())
	}
}

func TestReportAcceptanceRejectsFabricatedContradictions(t *testing.T) {
	base := Report{Requested: true, Profile: reportTestProfile(), Target: TargetModel, Syntax: SyntaxNotAssessed, Conformance: ConformanceConformant, AssessmentComplete: true, InputComplete: true}
	for _, change := range []func(*Report){
		func(r *Report) { r.Target = "unknown"; r.Profile.Target = "unknown" },
		func(r *Report) { r.Profile.ID = "" },
		func(r *Report) { r.Syntax = SyntaxWellFormed },
		func(r *Report) { r.Counts.Fail = 1 },
		func(r *Report) { r.Counts.Unknown = 1 },
		func(r *Report) { r.Counts.Unsupported = 1 },
		func(r *Report) { r.Counts.Blocked = 1 },
		func(r *Report) { r.Terminal = &TerminalSummary{Category: CategoryResource, Reason: "stopped"} },
	} {
		r := base
		change(&r)
		if r.Satisfied() {
			t.Fatalf("contradictory report accepted: %+v", r)
		}
	}
	for _, target := range []Target{TargetSource, TargetPackage} {
		r := base
		r.Target, r.Profile.Target = target, target
		if target == TargetSource {
			r.Syntax = SyntaxWellFormed
		}
		if !r.Satisfied() {
			t.Fatalf("complete %s report rejected", target)
		}
	}
}
