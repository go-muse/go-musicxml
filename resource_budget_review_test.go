package musicxml

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func reviewBudgetScore(measures int) string {
	var b strings.Builder
	b.WriteString(`<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Part</part-name></score-part></part-list><part id="P1">`)
	for i := 1; i <= measures; i++ {
		fmt.Fprintf(&b, `<measure number="%d"><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type></note></measure>`, i)
	}
	b.WriteString(`</part></score-partwise>`)
	return b.String()
}
func TestReviewDefaultBudgetOriginGraph(t *testing.T) {
	input := reviewBudgetScore(500)
	b, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraphInternalDTDDictionaries}}.internal()})
	if err != nil {
		t.Fatalf("%d source bytes: %v", len(input), err)
	}
	if !b.EnumerationComplete() || !b.Satisfied() {
		t.Fatalf("incomplete or unsatisfied bundle: %+v", b.PackageReport())
	}
	_, r, ok := b.ResourceReport(0)
	if !ok {
		t.Fatal("missing primary report")
	}
	origins := 0
	for _, row := range r.Context {
		if row.SourceOrigin != nil {
			origins++
		}
	}
	if origins != 4008 {
		t.Fatalf("source-origin rows=%d, want every one of 4008 occurrences", origins)
	}
	t.Logf("source bytes=%d source-origin rows=%d", len(input), origins)
}
func TestReviewDefaultBudgetOfficialSourceComposition(t *testing.T) {
	original, err := os.ReadFile("testdata/official/MozaChloSample.musicxml")
	if err != nil {
		t.Fatal(err)
	}
	raw := reviewBudgetRemoveDOCTYPE(t, original)
	if len(original) != 223068 || len(raw) != 222824 || !bytes.HasPrefix(raw, []byte{0xfe, 0xff}) || bytes.Contains(raw, encodeUTF16("<!DOCTYPE", binary.BigEndian)[2:]) {
		t.Fatalf("fixture transformation changed: original=%d stripped=%d", len(original), len(raw))
	}
	for _, profile := range []validation.ProfileID{ProfileSourceInternalDTDDictionaries.ID(), ProfileSourceSemantics.ID()} {
		t.Run(string(profile), func(t *testing.T) {
			r, err := InspectXML(context.Background(), bytes.NewReader(raw), SourceOptions{Context: validation.ContextFamilyAll, Profile: SourceProfile(profile)})
			if profile == ProfileSourceSemantics.ID() && err != nil {
				// Historically this input reached the fixed 20M work limit; the
				// default rule-step limit now grows with the document, so with the
				// complete projection only the default buffer can refuse, at the
				// end of the complete input; at its edge, whether it does depends
				// on the platform. The refusal is not a completed or satisfied
				// assessment.
				var limit *validation.ResourceError
				if !errors.As(err, &limit) || limit.Kind != validation.BudgetBufferedBytes || !r.InputComplete || r.AssessmentComplete || r.Satisfied() {
					t.Fatalf("unexpected default-budget failure: %v", err)
				}
				if limit.Limit != validation.DefaultBudgets().MaxBufferedBytes {
					t.Fatalf("resource refusal did not use the default limit: %v", err)
				}
				t.Logf("stripped UTF-16 sample preserves complete-input/incomplete-assessment resource refusal: %v", err)
				return
			}
			if err != nil || !r.AssessmentComplete || !r.InputComplete {
				t.Fatalf("profile=%s err=%v complete=%v", profile, err, r.AssessmentComplete)
			}
		})
	}
}

func TestReviewOfficialOriginalDOCTYPEBufferAndEvidence(t *testing.T) {
	raw, err := os.ReadFile("testdata/official/MozaChloSample.musicxml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = InspectXML(context.Background(), bytes.NewReader(raw), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceSemantics})
	if err != nil {
		// With the complete projection only the default buffer can refuse; the
		// default rule-step limit grows with the document.
		var limit *validation.ResourceError
		if !errors.As(err, &limit) || limit.Kind != validation.BudgetBufferedBytes {
			t.Fatalf("unexpected original-byte limit: %v", err)
		}
		t.Logf("original UTF-16 sample default limit: %v", err)
	}
	// With configured capacity the original bytes complete: the external
	// subset comes from the bundled official MusicXML DTD.
	r, err := InspectXML(context.Background(), bytes.NewReader(raw), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceSemantics, Budgets: officialDTDCapacity})
	if err != nil || !r.InputComplete || !r.AssessmentComplete || !r.Satisfied() {
		t.Fatalf("original external-DOCTYPE sample with configured capacity: complete=%v satisfied=%v err=%v", r.AssessmentComplete, r.Satisfied(), err)
	}
	for _, finding := range r.Findings {
		if finding.Reason == "dtd-infoset-unavailable" {
			t.Fatalf("bundled external DTD still reported unavailable: %+v", finding)
		}
	}
}

func reviewBudgetFragmentScore() string {
	var source strings.Builder
	source.WriteString(`<score-partwise version="4.0" xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="P1"><part-name>Part</part-name></score-part></part-list><part id="P1">`)
	for i := 1; i <= 2000; i++ {
		// Keep m1 last in document order: the old nested scan returned early
		// when m1 was first and would not expose its element-by-ID cost.
		fmt.Fprintf(&source, `<measure number="%d" id="m%d">`, i, 2001-i)
		if i == 2000 {
			source.WriteString(`<link xlink:href="#m1"/><link xlink:href="#m1"/><link xlink:href="#m1"/>`)
		}
		source.WriteString(`</measure>`)
	}
	source.WriteString(`</part></score-partwise>`)
	return source.String()
}

func reviewBudgetFragmentOptions(buffer, steps int64) linkedDictionaryAssessmentOptions {
	return linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{
		Graph:        GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraphInternalDTDDictionaries, Budgets: validation.Budgets{MaxBufferedBytes: buffer, MaxRuleSteps: steps}},
		Identity:     ResourceIdentity{RetrievalURI: "https://budget.test/score.musicxml"},
		Dependencies: []ResourceDependency{ResourceDocuments},
	}.internal()}
}

func TestReviewFragmentGraphMediumScoreDefaultRuleBudget(t *testing.T) {
	source := reviewBudgetFragmentScore()
	// This deliberately isolates work against the base of the default rule-step
	// limit, without the allowance the 2,000 measures would add; it is not an
	// all-default capacity claim.
	options := reviewBudgetFragmentOptions(512<<20, validation.DefaultRuleStepBase)
	bundle, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source), options)
	if err != nil || !bundle.EnumerationComplete() || !bundle.Satisfied() {
		t.Fatalf("three shorthand links failed at default work budget: %v %+v", err, bundle.PackageReport())
	}
	_, report, ok := bundle.ResourceReport(0)
	if !ok {
		t.Fatal("missing primary report")
	}
	links := 0
	for _, row := range report.Context {
		if row.Resource != nil && row.Resource.Link != nil {
			links++
		}
	}
	// Pass decisions are authoritative counts, not retained diagnostics. Every
	// link must contribute a distinct adoption, and complete/satisfied package
	// assessment proves all three selected fragment-target decisions succeeded.
	if links != 3 || bundle.AdoptionCount() != 4 || bundle.PackageReport().Counts.Pass != 3 {
		t.Fatalf("links=%d adoptions=%d package counts=%+v", links, bundle.AdoptionCount(), bundle.PackageReport().Counts)
	}
	t.Logf("source bytes=%d links=%d adoptions=%d package counts=%+v", len(source), links, bundle.AdoptionCount(), bundle.PackageReport().Counts)
}

// The default buffer admits a moderate origin graph and refuses a large one:
// with the complete projection, 700 measures complete with every source-origin
// row and 1,200 measures are refused by the buffer with complete input and an
// incomplete assessment. The exact boundary between them follows the retained
// record sizes; TestReviewOriginGraphExactSnapshotBufferAdmission checks it
// exactly from the refusal itself.
func TestReviewOriginGraphDefaultBufferBoundary(t *testing.T) {
	options := linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraphInternalDTDDictionaries}}.internal()}
	for _, tc := range []struct {
		measures int
		refuses  bool
	}{{700, false}, {1200, true}} {
		t.Run(fmt.Sprint(tc.measures), func(t *testing.T) {
			bundle, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(reviewBudgetScore(tc.measures)), options)
			if tc.refuses {
				reviewBudgetAssertGraphBufferRefusalAtLimit(t, bundle, err, validation.DefaultBudgets().MaxBufferedBytes, -1, -1)
			} else {
				reviewBudgetAssertCompleteOrigins(t, bundle, err, 8*tc.measures+8)
			}
		})
	}
}

// The 86,052-byte fragment graph is refused by the default buffer at snapshot
// admission: a typed buffer refusal with zero adoptions, incomplete
// enumeration, an unsatisfied bundle and the failed child's report retained.
func TestReviewFragmentGraphMediumScoreDefaultBufferRefusal(t *testing.T) {
	source := reviewBudgetFragmentScore()
	if len(source) != 86052 {
		t.Fatalf("fixture bytes=%d, want 86052", len(source))
	}
	bundle, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source), reviewBudgetFragmentOptions(0, 0))
	reviewBudgetAssertGraphBufferRefusalAtLimit(t, bundle, err, validation.DefaultBudgets().MaxBufferedBytes, -1, -1)
}

// A buffer refusal names exactly what the run needed: for origin graphs of
// 795 and 1,200 measures, the default buffer refuses at snapshot admission; a
// buffer of the refusal's used plus requested bytes admits the graph
// completely, and one byte less refuses at the same used and requested bytes.
func TestReviewOriginGraphExactSnapshotBufferAdmission(t *testing.T) {
	for _, measures := range []int{795, 1200} {
		source := reviewBudgetScore(measures)
		graph := func(limit int64) (validation.AssessmentBundle, error) {
			return inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraphInternalDTDDictionaries, Budgets: validation.Budgets{MaxBufferedBytes: limit}}}.internal()})
		}
		_, err := graph(0)
		var refusal *validation.ResourceError
		if !errors.As(err, &refusal) || refusal.Kind != validation.BudgetBufferedBytes {
			t.Fatalf("%d measures: want a default buffer refusal, got %v", measures, err)
		}
		used, requested := refusal.Used, refusal.Requested
		for _, short := range []int64{0, 1} {
			t.Run(fmt.Sprintf("%d/short-%d", measures, short), func(t *testing.T) {
				limit := used + requested - short
				bundle, err := graph(limit)
				if short == 0 {
					reviewBudgetAssertCompleteOrigins(t, bundle, err, 8*measures+8)
				} else {
					reviewBudgetAssertGraphBufferRefusalAtLimit(t, bundle, err, limit, used, requested)
				}
			})
		}
	}
}

func reviewBudgetAssertCompleteOrigins(t *testing.T, bundle validation.AssessmentBundle, err error, wantOrigins int) {
	t.Helper()
	if err != nil || !bundle.EnumerationComplete() || !bundle.Satisfied() {
		t.Fatalf("documented capacity fixture failed: %v", err)
	}
	_, report, ok := bundle.ResourceReport(0)
	if !ok {
		t.Fatal("missing primary report")
	}
	origins := 0
	for _, row := range report.Context {
		if row.SourceOrigin != nil {
			origins++
		}
	}
	if origins != wantOrigins {
		t.Fatalf("source-origin rows=%d, want every one of %d occurrences", origins, wantOrigins)
	}
}

func reviewBudgetAssertGraphBufferRefusal(t *testing.T, bundle validation.AssessmentBundle, err error, wantUsed, wantRequested int64) {
	t.Helper()
	reviewBudgetAssertGraphBufferRefusalAtLimit(t, bundle, err, validation.DefaultBudgets().MaxBufferedBytes, wantUsed, wantRequested)
}

// reviewBudgetAssertGraphBufferRefusalAtLimit checks a buffer refusal at
// wantLimit; a negative wantUsed accepts any used and requested bytes.
func reviewBudgetAssertGraphBufferRefusalAtLimit(t *testing.T, bundle validation.AssessmentBundle, err error, wantLimit, wantUsed, wantRequested int64) {
	t.Helper()
	var limit *validation.ResourceError
	if !errors.As(err, &limit) || limit.Kind != validation.BudgetBufferedBytes || limit.Limit != wantLimit {
		t.Fatalf("want specified buffered-byte refusal, got %v", err)
	}
	if wantUsed >= 0 && (limit.Used != wantUsed || limit.Requested != wantRequested) {
		t.Fatalf("documented admission changed: used=%d requested=%d, want %d/%d", limit.Used, limit.Requested, wantUsed, wantRequested)
	}
	if bundle.EnumerationComplete() || bundle.Satisfied() || bundle.AdoptionCount() != 0 {
		t.Fatalf("snapshot refusal exposed complete/adopted graph: adoptions=%d report=%+v", bundle.AdoptionCount(), bundle.PackageReport())
	}
	report, ok := bundle.FailureReport()
	if !ok || !report.InputComplete || report.AssessmentComplete || report.Satisfied() {
		t.Fatalf("snapshot refusal lost complete-input/incomplete-assessment report: present=%v report=%+v", ok, report)
	}
}

// The official sample is UTF-16BE. Remove the encoded declaration without
// transcoding or silently leaving the external-DOCTYPE profile boundary active.
func reviewBudgetRemoveDOCTYPE(t *testing.T, raw []byte) []byte {
	t.Helper()
	open, close := []byte("<!DOCTYPE"), []byte(">")
	if bytes.HasPrefix(raw, []byte{0xfe, 0xff}) {
		open = encodeUTF16("<!DOCTYPE", binary.BigEndian)[2:]
		close = encodeUTF16(">", binary.BigEndian)[2:]
	}
	start := bytes.Index(raw, open)
	if start < 0 {
		t.Fatal("official fixture DOCTYPE not found")
	}
	end := bytes.Index(raw[start:], close)
	if end < 0 {
		t.Fatal("official fixture DOCTYPE not closed")
	}
	out := append([]byte(nil), raw[:start]...)
	return append(out, raw[start+end+len(close):]...)
}
