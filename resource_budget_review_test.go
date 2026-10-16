package musicxml

import (
	"context"
	"fmt"
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
	b, err := InspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphInternalDTDDictionaries}}})
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

func TestReviewFragmentGraphMediumScoreDefaultRuleBudget(t *testing.T) {
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
	options := LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{
		Graph:        AssessmentOptions{Profile: ProfileResourceGraphInternalDTDDictionaries, Budgets: validation.Budgets{MaxBufferedBytes: 512 << 20}},
		Identity:     ResourceIdentity{RetrievalURI: "https://budget.test/score.musicxml"},
		Dependencies: []ResourceDependency{ResourceDocuments},
	}}
	bundle, err := InspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source.String()), options)
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
	t.Logf("source bytes=%d links=%d adoptions=%d package counts=%+v", source.Len(), links, bundle.AdoptionCount(), bundle.PackageReport().Counts)
}
