package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func assessedScore(staves, extra string) string {
	return `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><part id="P1"><measure number="1"><attributes><staves>` + staves + `</staves>` + extra + `</attributes></measure></part></score-partwise>`
}
func strictSourceOptions() DecodeAssessmentOptions {
	return DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceLine}}
}

type onceEOFReader struct {
	input                   string
	offset, reads, afterEOF int
	eof                     bool
}

func (r *onceEOFReader) Read(p []byte) (int, error) {
	r.reads++
	if r.eof {
		r.afterEOF++
		return 0, errors.New("read again after EOF")
	}
	if r.offset == len(r.input) {
		r.eof = true
		return 0, io.EOF
	}
	p[0] = r.input[r.offset]
	r.offset++
	return 1, nil
}

func TestDecodeAssessedSourceAndConversionDomains(t *testing.T) {
	for _, tt := range []struct {
		value      string
		source     validation.ConformanceState
		conversion validation.ConversionState
		invalid    bool
	}{
		{"18446744073709551615", validation.ConformanceConformant, validation.ConversionConverted, false},
		// Conformant, but the source line rejects a value the model cannot hold.
		{"18446744073709551616", validation.ConformanceConformant, validation.ConversionUnrepresentable, true},
		{"-1", validation.ConformanceViolated, validation.ConversionUnrepresentable, true},
		{"1.5", validation.ConformanceViolated, validation.ConversionFailed, true},
	} {
		t.Run(tt.value, func(t *testing.T) {
			input := assessedScore(tt.value, `<instruments>1</instruments>`)
			inspection, inspectErr := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
			if inspectErr != nil || !inspection.InputComplete || !inspection.AssessmentComplete || inspection.Conformance != tt.source {
				t.Fatalf("inspection=%+v err=%v", inspection, inspectErr)
			}
			reader := &onceEOFReader{input: input}
			result, err := DecodeAssessed(context.Background(), reader, strictSourceOptions())
			if result.SourceLine.Conformance != tt.source || !result.SourceLine.InputComplete || !result.SourceLine.AssessmentComplete || result.Conversion.State != tt.conversion {
				t.Errorf("result=%+v err=%v", result, err)
			}
			if reader.offset != len(input) || reader.afterEOF != 0 {
				t.Errorf("not exactly once through EOF: %+v", reader)
			}
			if errors.Is(err, ErrInvalidDocument) != tt.invalid {
				t.Errorf("invalid classification=%v", err)
			}
			if tt.conversion == validation.ConversionConverted {
				if err != nil || result.Document == nil || !result.SourceLine.Satisfied() || !result.ModelLine.Satisfied() {
					t.Errorf("conversion result=%+v err=%v", result, err)
				}
			} else {
				if err == nil || result.Document != nil || result.ModelLine.Requested {
					t.Errorf("partial model escaped=%+v error=%v", result, err)
				}
			}
			if tt.conversion == validation.ConversionUnrepresentable {
				var converted *ConversionError
				var numeric *strconv.NumError
				if !errors.As(err, &converted) || !errors.As(err, &numeric) || !errors.Is(err, strconv.ErrRange) {
					t.Errorf("numeric cause lost=%v", err)
				}
			}
		})
	}
}

func TestDecodeAssessedDrainsAfterOverflowAndPreservesTailCauses(t *testing.T) {
	for _, tt := range []struct {
		name, extra, tail string
		invalid           bool
	}{
		// The overflowing staves value is itself a source-line failure.
		{"valid-tail", `<instruments>1</instruments>`, "", true},
		{"later-attribute-violation", `<instruments rubbish="bad">1</instruments>`, "", true},
		{"malformed-tail", `<instruments>1</instruments>`, `<`, true},
		{"violation-then-malformed-tail", `<instruments rubbish="bad">1</instruments>`, `<`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeAssessed(context.Background(), strings.NewReader(assessedScore("18446744073709551616", tt.extra)+tt.tail), strictSourceOptions())
			if result.Document != nil || result.Conversion.State != validation.ConversionUnrepresentable || !errors.Is(err, strconv.ErrRange) {
				t.Fatalf("conversion lost: %+v %v", result, err)
			}
			if errors.Is(err, ErrInvalidDocument) != tt.invalid {
				t.Errorf("independent invalidity mismatch: %+v %v", result.SourceLine, err)
			}
			if tt.tail != "" {
				var syntax *xml.SyntaxError
				if !errors.As(err, &syntax) || result.SourceLine.Syntax != validation.SyntaxMalformed || result.SourceLine.InputComplete || !errors.Is(err, ErrIncompleteAssessment) {
					t.Errorf("tail lost: %+v %v", result.SourceLine, err)
				}
			} else if !result.SourceLine.InputComplete {
				t.Errorf("healthy parser not drained: %+v %v", result.SourceLine, err)
			}
		})
	}
}

func TestSourceAssessmentReaderOwnershipAndUnsupportedRoot(t *testing.T) {
	for _, input := range []string{`<unknown/>`, `<score-partwise xmlns="urn:foreign"/>`, `<score-partwise/>`} {
		reader := &assessmentCountingReader{source: strings.NewReader(input)}
		report, err := InspectXML(context.Background(), reader, AssessmentOptions{Profile: ProfileSourceXSDPreview})
		if reader.closed {
			t.Error("closed caller-owned reader")
		}
		if input != `<score-partwise/>` {
			var unsupported *validation.ExcludedRootError
			if !errors.As(err, &unsupported) || !errors.Is(err, ErrUnsupportedRoot) || errors.Is(err, ErrInvalidDocument) || report.Syntax == validation.SyntaxMalformed {
				t.Errorf("unsupported classification: %+v %v", report, err)
			}
		}
	}
}

func TestDecodeAssessedObservesFilteredSource(t *testing.T) {
	input := assessedScore("1", `<foreign:ignored xmlns:foreign="urn:foreign"><staves rubbish="yes">2</staves></foreign:ignored>`)
	permissive, err := Decode(strings.NewReader(input))
	if err != nil || permissive == nil {
		t.Fatalf("ordinary foreign extension: %T %v", permissive, err)
	}
	strict, err := DecodeAssessed(context.Background(), strings.NewReader(input), strictSourceOptions())
	if !errors.Is(err, ErrInvalidDocument) || strict.Document != nil || strict.SourceLine.Conformance != validation.ConformanceViolated {
		t.Fatalf("filtered source disappeared: %+v %v", strict, err)
	}
}

func TestSourceAssessmentBudgets(t *testing.T) {
	input := assessedScore("1", "")
	for _, tt := range []struct {
		name    string
		budgets validation.Budgets
		kind    validation.BudgetKind
	}{
		{"input", validation.Budgets{MaxInputBytes: int64(len(input) - 1)}, validation.BudgetInputBytes},
		{"nodes", validation.Budgets{MaxNodes: 2}, validation.BudgetNodes},
		{"attributes", validation.Budgets{MaxAttributes: 1}, validation.BudgetAttributes},
		{"depth", validation.Budgets{MaxDepth: 2}, validation.BudgetDepth},
		{"events", validation.Budgets{MaxEvents: 2}, validation.BudgetEvents},
		{"scalar", validation.Budgets{MaxScalarBytes: 3}, validation.BudgetScalarBytes},
		{"buffer", validation.Budgets{MaxBufferedBytes: 1}, validation.BudgetBufferedBytes},
		{"rule-steps", validation.Budgets{MaxRuleSteps: 1}, validation.BudgetRuleSteps},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: tt.budgets})
			var resource *validation.ResourceError
			if !errors.As(err, &resource) || resource.Kind != tt.kind || !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, ErrInvalidDocument) || report.Satisfied() {
				t.Fatalf("budget report=%+v error=%v kind=%+v", report, err, resource)
			}
		})
	}
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: validation.Budgets{MaxInputBytes: int64(len(input))}})
	if err != nil || !report.Satisfied() {
		t.Fatalf("exact byte limit should allow EOF: %+v %v", report, err)
	}
}

func TestDecodeAssessedDrainsAttributeAndChoiceConversionFailures(t *testing.T) {
	huge := strings.Repeat("9", 400)
	base := assessedScore("1", `<instruments>1</instruments>`)
	cases := map[string]string{
		"attribute":     strings.Replace(base, `<measure number="1">`, `<measure number="1" width="`+huge+`">`, 1),
		"nested-choice": strings.Replace(base, `</measure>`, `<note><rest/><duration>`+huge+`</duration></note></measure>`, 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			// The source line reports the value the model cannot hold; the
			// conversion still fails on it, and the source stays conformant.
			result, err := DecodeAssessed(context.Background(), &onceEOFReader{input: input}, strictSourceOptions())
			if !errors.Is(err, strconv.ErrRange) || result.Document != nil || result.Conversion.State != validation.ConversionUnrepresentable || result.SourceLine.Satisfied() || result.SourceLine.Conformance != validation.ConformanceConformant || !hasRepresentabilityFailure(result.SourceLine) {
				t.Fatalf("source/conversion=%+v error=%v", result, err)
			}
		})
	}
}

type interruptedSourceReader struct {
	input        string
	offset, stop int
	cause        error
	cancel       context.CancelFunc
}

func (r *interruptedSourceReader) Read(p []byte) (int, error) {
	if r.offset == r.stop {
		if r.cancel != nil {
			r.cancel()
		}
		return 0, r.cause
	}
	if r.offset == len(r.input) {
		return 0, io.EOF
	}
	p[0] = r.input[r.offset]
	r.offset++
	return 1, nil
}
func TestDecodeAssessedRetainsOverflowDuringInterruptedDrain(t *testing.T) {
	input := assessedScore("18446744073709551616", `<instruments>1</instruments>`)
	stop := strings.Index(input, `<instruments>`)
	marker := errors.New("source tail unavailable")
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		reader := &interruptedSourceReader{input: input, stop: stop, cause: marker}
		want := error(marker)
		if canceled {
			reader.cancel = cancel
			reader.cause = context.Canceled
			want = context.Canceled
		}
		result, err := DecodeAssessed(ctx, reader, strictSourceOptions())
		cancel()
		// The only invalidity is the staves value's source-line failure, when the
		// interrupted source proved it before stopping.
		if !errors.Is(err, strconv.ErrRange) || !errors.Is(err, want) || !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, ErrInvalidDocument) != hasRepresentabilityFailure(result.SourceLine) || result.Document != nil || result.SourceLine.InputComplete || result.SourceLine.Syntax != validation.SyntaxIncomplete {
			t.Errorf("interrupted drain: %+v error=%v", result, err)
		}
	}
}

func TestDecodeAssessedOriginalForeignRootName(t *testing.T) {
	for _, uri := range []string{"xml", "xmlns", "xlink", "urn:foreign"} {
		result, err := DecodeAssessed(context.Background(), strings.NewReader(`<score-partwise xmlns="`+uri+`"/>`), DecodeAssessmentOptions{})
		var unsupported *UnsupportedRootError
		if !errors.As(err, &unsupported) || unsupported.Name.Space != uri || result.Document != nil {
			t.Errorf("original foreign root %q lost: result=%+v error=%v unsupported=%+v", uri, result, err, unsupported)
		}
	}
}

func TestSourceAssessmentCumulativeBudgetBoundaries(t *testing.T) {
	input := strings.Replace(assessedScore("1", `<instruments>1</instruments>`), `</part-list>`, `<score-part id="P2"><part-name>Other</part-name></score-part></part-list>`, 1)
	run, err := newSourceAssessmentRun(context.Background(), strings.NewReader(input), DecodeOptions{}, &AssessmentOptions{Profile: ProfileSourceXSDPreview}, false)
	if err != nil {
		t.Fatal(err)
	}
	sourceErr := run.drain()
	report, err := run.finish(sourceErr)
	if err != nil || !report.Satisfied() {
		t.Fatalf("baseline %+v %v", report, err)
	}
	cases := []struct {
		kind validation.BudgetKind
		set  func(*validation.Budgets, int64)
	}{
		{validation.BudgetInputBytes, func(b *validation.Budgets, n int64) { b.MaxInputBytes = n }},
		{validation.BudgetNodes, func(b *validation.Budgets, n int64) { b.MaxNodes = n }},
		{validation.BudgetAttributes, func(b *validation.Budgets, n int64) { b.MaxAttributes = n }},
		{validation.BudgetEvents, func(b *validation.Budgets, n int64) { b.MaxEvents = n }},
		{validation.BudgetRuleSteps, func(b *validation.Budgets, n int64) { b.MaxRuleSteps = n }},
		{validation.BudgetIDs, func(b *validation.Budgets, n int64) { b.MaxIDs = n }},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			used, e := run.account.Usage(tc.kind)
			if e != nil || used < 2 {
				t.Fatalf("active resource baseline %s=%d error=%v", tc.kind, used, e)
			}
			for _, delta := range []int64{-1, 0, 1} {
				budgets := validation.Budgets{}
				tc.set(&budgets, used+delta)
				got, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: budgets})
				if delta < 0 {
					var limit *validation.ResourceError
					if !errors.As(err, &limit) || limit.Kind != tc.kind || got.Satisfied() {
						t.Errorf("below %d: %+v %v", used, got, err)
					}
				} else if err != nil || !got.Satisfied() {
					t.Errorf("at/above %d+%d: %+v %v", used, delta, got, err)
				}
			}
		})
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, _ := run.account.Usage(kind)
		if used != 0 {
			t.Errorf("run leaked %s=%d", kind, used)
		}
	}
}

func TestSourceAssessmentDepthAndScalarBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value string
		limit int64
		set   func(*validation.Budgets, int64)
	}{
		{"depth", "1", 5, func(b *validation.Budgets, n int64) { b.MaxDepth = n }},
		{"scalar", "1", 50, func(b *validation.Budgets, n int64) { b.MaxScalarBytes = n }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, delta := range []int64{-1, 0, 1} {
				budgets := validation.Budgets{}
				tt.set(&budgets, tt.limit+delta)
				input := assessedScore(tt.value, "")
				if tt.name == "scalar" {
					input = strings.Replace(input, "Music", strings.Repeat("a", 50), 1)
				}
				report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: budgets})
				if delta < 0 {
					if !errors.Is(err, validation.ErrResourceLimit) || report.Satisfied() {
						t.Errorf("below boundary: %+v %v", report, err)
					}
				} else if err != nil || !report.Satisfied() {
					t.Errorf("exact/above boundary: %+v %v", report, err)
				}
			}
		})
	}
}

func TestSourceAssessmentEmptyDocumentSyntax(t *testing.T) {
	for _, input := range []string{"", " \r\n\t", "<!-- no document element -->"} {
		report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
		if !errors.Is(err, ErrEmptyDocument) || !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, ErrInvalidDocument) || report.Syntax != validation.SyntaxMalformed || report.InputComplete || report.Terminal == nil || report.Terminal.Category != validation.CategorySyntax {
			t.Errorf("empty source: %+v %v", report, err)
		}
	}
}

func TestSourceAssessmentBufferedBoundaryAndCleanup(t *testing.T) {
	input := assessedScore("1", "")
	inspect := func(limit int64) (validation.Report, error) {
		return InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: validation.Budgets{MaxBufferedBytes: limit}})
	}
	low, high := int64(1), int64(2<<20)
	if r, err := inspect(high); err != nil || !r.Satisfied() {
		t.Fatalf("upper bound not sufficient: %+v %v", r, err)
	}
	for low < high {
		middle := low + (high-low)/2
		r, err := inspect(middle)
		if err == nil && r.Satisfied() {
			high = middle
		} else {
			var resource *validation.ResourceError
			if !errors.As(err, &resource) || resource.Kind != validation.BudgetBufferedBytes {
				t.Fatalf("unexpected boundary error at%d: %+v %v", middle, r, err)
			}
			low = middle + 1
		}
	}
	for _, delta := range []int64{-1, 0, 1} {
		r, err := inspect(low + delta)
		if delta < 0 {
			if !errors.Is(err, validation.ErrResourceLimit) || r.Satisfied() {
				t.Errorf("below live-buffer boundary: %+v %v", r, err)
			}
		} else if err != nil || !r.Satisfied() {
			t.Errorf("exact/above live-buffer boundary: %+v %v", r, err)
		}
	}
	// Fail after Begin and at several points in the parser/observer/session tree.
	// Every started run must release only its live buffers and depth on Finish.
	for _, limit := range []int64{low / 2, low - 1, low} {
		run, err := newSourceAssessmentRun(context.Background(), strings.NewReader(input), DecodeOptions{}, &AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: validation.Budgets{MaxBufferedBytes: limit}}, false)
		if run == nil {
			continue
		}
		if err == nil {
			cause := run.drain()
			_, _ = run.finish(cause)
		}
		for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
			used, _ := run.account.Usage(kind)
			if used != 0 {
				t.Errorf("limit%d leaked %s=%d", limit, kind, used)
			}
		}
	}
}

func TestSourceAssessmentDiagnosticBoundaries(t *testing.T) {
	input := `<score-partwise bad="x" also="y"/>`
	run, err := newSourceAssessmentRun(context.Background(), strings.NewReader(input), DecodeOptions{}, &AssessmentOptions{Profile: ProfileSourceXSDPreview}, false)
	if err != nil {
		t.Fatal(err)
	}
	cause := run.drain()
	baseline, err := run.finish(cause)
	if err != nil || baseline.Counts.Fail < 2 || len(baseline.Findings) < 2 {
		t.Fatalf("diagnostic baseline: %+v %v", baseline, err)
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetDiagnostics, validation.BudgetDiagnosticBytes} {
		used, _ := run.account.Usage(kind)
		for _, delta := range []int64{-1, 0, 1} {
			budgets := validation.Budgets{}
			if kind == validation.BudgetDiagnostics {
				budgets.MaxDiagnostics = used + delta
			} else {
				budgets.MaxDiagnosticBytes = used + delta
			}
			report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: budgets})
			if err != nil || report.Counts != baseline.Counts || report.Conformance != validation.ConformanceViolated || !report.AssessmentComplete || report.DiagnosticsTruncated != (delta < 0) {
				t.Errorf("%s limit%d: %+v %v", kind, used+delta, report, err)
			}
		}
	}
}

func TestSourceAssessmentUnusedBudgetsDoNotInventWork(t *testing.T) {
	run, err := newSourceAssessmentRun(context.Background(), strings.NewReader(assessedScore("1", "")), DecodeOptions{}, &AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: validation.Budgets{MaxExternalBytes: 1, MaxContextFacts: 1, MaxLinkedDocuments: 1}}, false)
	if err != nil {
		t.Fatal(err)
	}
	cause := run.drain()
	report, err := run.finish(cause)
	if err != nil || !report.Satisfied() {
		t.Fatalf("unimplemented service budgets unexpectedly used: %+v %v", report, err)
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetExternalBytes, validation.BudgetContextFacts, validation.BudgetLinkedDocuments} {
		used, _ := run.account.Usage(kind)
		if used != 0 {
			t.Errorf("unimplemented service charged %s=%d", kind, used)
		}
	}
}

func TestDecodeAssessedUsesTighterDepthLimit(t *testing.T) {
	input := `<score-partwise><extension><nested/></extension></score-partwise>`
	for _, tt := range []struct {
		name       string
		decode     int
		assessment int64
		legacy     bool
	}{
		{"decode-is-tighter", 2, 3, true}, {"assessment-is-tighter", 3, 2, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Decode: DecodeOptions{MaxXMLDepth: tt.decode}, Assessment: &AssessmentOptions{Profile: ProfileSourceLine, Budgets: validation.Budgets{MaxDepth: tt.assessment}}})
			var resource *validation.ResourceError
			if !errors.As(err, &resource) || resource.Kind != validation.BudgetDepth || resource.Limit != 2 || errors.Is(err, ErrXMLTooDeep) != tt.legacy || errors.Is(err, ErrInvalidDocument) || result.SourceLine.Terminal == nil || result.SourceLine.Terminal.Category != validation.CategoryResource || result.Document != nil {
				t.Errorf("tighter depth result=%+v resource=%+v error=%v", result, resource, err)
			}
		})
	}
	ordinary, err := DecodeWithOptions(strings.NewReader(input), DecodeOptions{MaxXMLDepth: 2})
	if !errors.Is(err, ErrXMLTooDeep) || errors.Is(err, validation.ErrResourceLimit) || ordinary != nil {
		t.Errorf("ordinary decode changed legacy depth error: %T %v", ordinary, err)
	}
	result, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Decode: DecodeOptions{MaxXMLDepth: 2}})
	if !errors.Is(err, ErrXMLTooDeep) || !errors.Is(err, validation.ErrResourceLimit) || !result.SourceLine.Requested || result.Document != nil {
		t.Errorf("default assessment did not enforce mandatory depth accounting: %+v %v", result, err)
	}
}

func TestDecodeAssessedConversionDetailCannotStarveSourceFailure(t *testing.T) {
	options := strictSourceOptions()
	options.Assessment.Budgets.MaxDiagnostics = 1
	result, err := DecodeAssessed(context.Background(), strings.NewReader(assessedScore("18446744073709551616", `<instruments rubbish="bad">1</instruments>`)), options)
	// The unrepresentable staves value and the undeclared attribute are both
	// source failures; the one stored finding belongs to the source.
	if !errors.Is(err, strconv.ErrRange) || !errors.Is(err, ErrInvalidDocument) || result.Conversion.State != validation.ConversionUnrepresentable || len(result.Conversion.Findings) != 0 || len(result.SourceLine.Findings) != 1 || result.SourceLine.Counts.Fail == 0 {
		t.Fatalf("conversion detail stole source quota: %+v %v", result, err)
	}
}

func TestSourceAssessmentWorkIsRepeatable(t *testing.T) {
	input := strings.Replace(assessedScore("1", `<instruments>1</instruments>`), `</part-list>`, `<score-part id="P2"><part-name>Other</part-name></score-part></part-list>`, 1)
	var expected int64
	for repeat := 0; repeat < 20; repeat++ {
		run, err := newSourceAssessmentRun(context.Background(), strings.NewReader(input), DecodeOptions{}, &AssessmentOptions{Profile: ProfileSourceXSDPreview}, false)
		if err != nil {
			t.Fatal(err)
		}
		cause := run.drain()
		report, err := run.finish(cause)
		if err != nil || !report.Satisfied() {
			t.Fatalf("repeated source assessment: %+v %v", report, err)
		}
		used, _ := run.account.Usage(validation.BudgetRuleSteps)
		if repeat == 0 {
			expected = used
		}
		t.Logf("run%d rule-steps=%d", repeat, used)
		if used != expected {
			t.Errorf("same source has map-order-dependent budget admission: got%d want%d", used, expected)
		}
	}
}

func TestDecodeAssessedConversionLocationKeepsMappedOwner(t *testing.T) {
	result, err := DecodeAssessed(context.Background(), strings.NewReader(assessedScore(`1<unknown/>.5`, "")), strictSourceOptions())
	if err == nil || result.Conversion.State != validation.ConversionFailed || len(result.Conversion.Findings) != 1 {
		t.Fatalf("fixture did not fail mapping: %+v %v", result, err)
	}
	finding := result.Conversion.Findings[0]
	if !strings.HasSuffix(finding.Location.Path, "/staves") {
		t.Errorf("conversion blamed skipped child instead of mapped scalar owner: %+v", finding)
	}
}

func TestAssessedModelRetentionSurvivesSourceFinalization(t *testing.T) {
	run, err := newSourceAssessmentRun(context.Background(), strings.NewReader(assessedScore("1", "")), DecodeOptions{}, &AssessmentOptions{Profile: ProfileSourceXSDPreview}, true)
	if err != nil {
		t.Fatal(err)
	}
	projected := &namespaceXMLTokenReader{source: run.source, allowQualifiedAttribute: musicXMLQualifiedAttribute}
	model := &assessedModelTokenReader{source: projected, account: run.account, ctx: context.Background(), observer: run.source.observer}
	decoder := xml.NewTokenDecoder(model)
	start, err := readRoot(decoder)
	if err != nil {
		t.Fatal(err)
	}
	document := &ScorePartwise{}
	if err := decoder.DecodeElement(document, &start); err != nil {
		t.Fatal(err)
	}
	retained := model.retained
	if retained == 0 {
		t.Fatal("model candidate was not admitted")
	}
	cause := run.drain()
	report, err := run.finish(cause)
	if err != nil || !report.Satisfied() {
		t.Fatalf("finalization: %+v %v", report, err)
	}
	used, _ := run.account.Usage(validation.BudgetBufferedBytes)
	if used != retained {
		t.Fatalf("source finalization stole live candidate storage: got %d want %d", used, retained)
	}
	if len(document.Part) == 0 {
		t.Fatal("fixture did not retain a decoded model")
	}
	if err := model.close(); err != nil {
		t.Fatal(err)
	}
	used, _ = run.account.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("candidate leaked %d bytes", used)
	}
}

func TestSourceAssessmentKeepsClosedFailureWithUnclosedAncestors(t *testing.T) {
	input := strings.TrimSuffix(assessedScore("bad", ""), "</attributes></measure></part></score-partwise>")
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) || report.Conformance != validation.ConformanceViolated || report.AssessmentComplete || report.InputComplete {
		t.Fatalf("closed scalar proof lost under unclosed ancestors: %+v / %v", report, err)
	}
}

// hasRepresentabilityFailure reports a failed source representability finding.
func hasRepresentabilityFailure(r validation.Report) bool {
	for _, f := range r.Findings {
		if strings.HasPrefix(string(f.Rule.ID), "MX40-SOURCE-REPRESENTABLE-") && f.Outcome == validation.OutcomeFail && f.Category == validation.CategoryModel {
			return true
		}
	}
	return false
}
