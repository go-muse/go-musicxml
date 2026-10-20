package musicxml

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
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
				// Historically this input reached the 20M work limit before layout
				// publication; reduced repeated work then moved the refusal to the
				// buffer. With real-layout projection accounting the buffer and the
				// work limit are both close to exhaustion at the end of the same
				// complete input, so either may refuse first. Neither
				// default-budget refusal is a completed or satisfied assessment.
				var limit *validation.ResourceError
				if !errors.As(err, &limit) || limit.Kind != validation.BudgetBufferedBytes && limit.Kind != validation.BudgetRuleSteps || !r.InputComplete || r.AssessmentComplete || r.Satisfied() {
					t.Fatalf("unexpected default-budget failure: %v", err)
				}
				wantLimit := validation.DefaultBudgets().MaxBufferedBytes
				if limit.Kind == validation.BudgetRuleSteps {
					wantLimit = validation.DefaultBudgets().MaxRuleSteps
				}
				if limit.Limit != wantLimit {
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
		// The default buffer or work limit refuses, whichever comes first.
		var limit *validation.ResourceError
		if !errors.As(err, &limit) || limit.Kind != validation.BudgetBufferedBytes && limit.Kind != validation.BudgetRuleSteps {
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

func reviewBudgetFragmentOptions(buffer int64) linkedDictionaryAssessmentOptions {
	return linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{
		Graph:        GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraphInternalDTDDictionaries, Budgets: validation.Budgets{MaxBufferedBytes: buffer}},
		Identity:     ResourceIdentity{RetrievalURI: "https://budget.test/score.musicxml"},
		Dependencies: []ResourceDependency{ResourceDocuments},
	}.internal()}
}

func TestReviewFragmentGraphMediumScoreDefaultRuleBudget(t *testing.T) {
	source := reviewBudgetFragmentScore()
	// This deliberately isolates work; it is not an all-default capacity claim.
	options := reviewBudgetFragmentOptions(512 << 20)
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

// These are fixture-specific amd64 admission witnesses, not portable source-byte
// guarantees. Concrete owned-record sizes can change with the host architecture.
func TestReviewOriginGraphDefaultBufferBoundary(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("documented amd64 capacity witness")
	}
	for _, tc := range []struct {
		measures, bytes int
		refuses         bool
		used, requested int64
	}{
		// Historical d336f089 boundary: 531 completed; 532 refused at
		// used44,066,863/requested23,115,219. Both now complete after
		// compact facts and session-wide context-role reuse.
		{531, 71197, false, 0, 0},
		{532, 71331, false, 0, 0},
		// The first compact-fact checkpoint completed 568 and refused 569 at
		// used42,495,274/requested24,717,911. Declaration-canonical role reuse
		// now admits both and moves the adjacent default boundary to 576/577.
		// The current-state schema-hint wording adds 84 live bytes. The source
		// representability rule's retained definitions and per-node results
		// moved the boundary to 571/572; since that rule belongs to the
		// strict-loading source line only, this internal-DTD graph profile no
		// longer carries it, and the boundary is 576/577 again. The structural
		// score/part role wording adds 59 live bytes. Accounting the context
		// projection by its real layout moves the boundary to 613/614.
		// Releasing the work and subject charges of completed XSD subjects
		// and interning the engine's own strings move it to 794/795.
		{568, 76155, false, 0, 0},
		{569, 76289, false, 0, 0},
		{571, 76557, false, 0, 0},
		{572, 76691, false, 0, 0},
		{576, 77227, false, 0, 0},
		{577, 77361, false, 0, 0},
		{613, 82185, false, 0, 0},
		{614, 82319, false, 0, 0},
		{794, 106439, false, 0, 0},
		{795, 106573, true, 32640336, 34507463},
	} {
		t.Run(fmt.Sprint(tc.measures), func(t *testing.T) {
			source := reviewBudgetScore(tc.measures)
			if len(source) != tc.bytes {
				t.Fatalf("fixture bytes=%d, want %d", len(source), tc.bytes)
			}
			bundle, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraphInternalDTDDictionaries}}.internal()})
			if tc.refuses {
				reviewBudgetAssertGraphBufferRefusal(t, bundle, err, tc.used, tc.requested)
			} else {
				reviewBudgetAssertCompleteOrigins(t, bundle, err, 8*tc.measures+8)
			}
			t.Logf("source bytes=%d measures=%d err=%v", len(source), tc.measures, err)
		})
	}
}

func TestReviewFragmentGraphMediumScoreDefaultBufferRefusal(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("documented amd64 capacity witness")
	}
	source := reviewBudgetFragmentScore()
	if len(source) != 86052 {
		t.Fatalf("fixture bytes=%d, want 86052", len(source))
	}
	bundle, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source), reviewBudgetFragmentOptions(0))
	// Historical live-byte witnesses were 57,607,574 and then 54,998,870;
	// declaration-canonical role reuse lowers only the live side again.
	// The external-DTD closure wording adds 202 live and 104 requested bytes:
	// the graph and intrinsic source profile texts are charged as retained
	// strings. The current-state schema-hint wording adds 84 live bytes. The
	// source representability rule added its definitions and node results
	// until it moved to the strict-loading source line. The structural
	// score/part role wording adds 59 live bytes. Real-layout projection
	// accounting lowers the live side to 48,656,055, and releasing the work
	// and subject charges of completed XSD subjects to 41,640,021.
	reviewBudgetAssertGraphBufferRefusal(t, bundle, err, 41640021, 34741296)
	t.Logf("source bytes=%d err=%v", len(source), err)
}

func TestReviewOriginGraphExactSnapshotBufferAdmission(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("documented amd64 capacity witness")
	}
	// Retain the historical 569-measure fixture and the current 795-measure
	// default refusal. Their measured final-snapshot overlaps establish exact
	// and one-short admission controls, without changing production defaults.
	for _, tc := range []struct {
		measures, bytes int
		used, requested int64
	}{
		{569, 76289, 24284024, 24718047},
		{795, 106573, 32640336, 34507463},
	} {
		for _, short := range []int64{0, 1} {
			t.Run(fmt.Sprintf("%d/short-%d", tc.measures, short), func(t *testing.T) {
				source := reviewBudgetScore(tc.measures)
				if len(source) != tc.bytes {
					t.Fatalf("fixture bytes=%d, want %d", len(source), tc.bytes)
				}
				limit := tc.used + tc.requested - short
				options := linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraphInternalDTDDictionaries, Budgets: validation.Budgets{MaxBufferedBytes: limit}}}.internal()}
				bundle, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source), options)
				if short == 0 {
					reviewBudgetAssertCompleteOrigins(t, bundle, err, 8*tc.measures+8)
				} else {
					reviewBudgetAssertGraphBufferRefusalAtLimit(t, bundle, err, limit, tc.used, tc.requested)
				}
				t.Logf("source bytes=%d measures=%d limit=%d err=%v", len(source), tc.measures, limit, err)
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

func reviewBudgetAssertGraphBufferRefusalAtLimit(t *testing.T, bundle validation.AssessmentBundle, err error, wantLimit, wantUsed, wantRequested int64) {
	t.Helper()
	var limit *validation.ResourceError
	if !errors.As(err, &limit) || limit.Kind != validation.BudgetBufferedBytes || limit.Limit != wantLimit {
		t.Fatalf("want specified buffered-byte refusal, got %v", err)
	}
	if limit.Used != wantUsed || limit.Requested != wantRequested {
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
