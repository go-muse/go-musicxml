package musicxml

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func dtdIDGraph(t *testing.T, source string) validation.AssessmentBundle {
	t.Helper()
	b, e := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(source), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphInternalDTDDictionaries}}})
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestDTDIDJoinedShorthandSourceCases(t *testing.T) {
	for _, tc := range []struct {
		name, decl, body string
		want             validation.Outcome
	}{
		{"implied", `<!ATTLIST opus version ID #IMPLIED>`, `<opus version="O1" xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#O1"/></opus>`, validation.OutcomePass},
		{"required", `<!ATTLIST opus version ID #REQUIRED>`, `<opus version="O1" xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#O1"/></opus>`, validation.OutcomePass},
		{"normalized", `<!ATTLIST opus version ID #IMPLIED>`, `<opus version="  O1  " xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#O1"/></opus>`, validation.OutcomePass},
		{"id-first", `<!ATTLIST opus version ID #IMPLIED><!ATTLIST opus version CDATA #IMPLIED>`, `<opus version="O1" xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#O1"/></opus>`, validation.OutcomePass},
		{"cdata-first", `<!ATTLIST opus version CDATA #IMPLIED><!ATTLIST opus version ID #IMPLIED>`, `<opus version="O1" xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#O1"/></opus>`, validation.OutcomeFail},
		{"complete-miss", `<!ATTLIST opus version ID #IMPLIED>`, `<opus version="O1" xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#absent"/></opus>`, validation.OutcomeFail},
		{"no-dtd", "", `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#absent"/></opus>`, validation.OutcomeFail},
		{"invalid-id-default", `<!ATTLIST opus version ID "O1">`, `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#O1"/></opus>`, validation.OutcomeUnknown},
		{"entity-content", `<!ATTLIST opus version ID #IMPLIED><!ENTITY e '<opus version="O1"><opus-link xlink:href="#O1"/></opus>'>`, `<opus xmlns:xlink="http://www.w3.org/1999/xlink">&e;</opus>`, validation.OutcomePass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.body
			if tc.decl != "" {
				source = "<!DOCTYPE opus [" + tc.decl + "]>" + source
			}
			b := dtdIDGraph(t, source)
			r := b.PackageReport()
			found := validation.OutcomePass
			for _, f := range r.Findings {
				if f.Decision.Rule.ID == "MX40-PACKAGE-link-target" {
					found = f.Decision.Outcome
				}
			}
			if found != tc.want {
				t.Fatalf("got %s want %s findings=%+v", found, tc.want, r.Findings)
			}
			if tc.want == validation.OutcomePass && !b.Satisfied() {
				t.Fatalf("not satisfied: %+v", r)
			}
		})
	}
}
func TestDTDIDMusicXMLMeasureAndIndependentXSDDuplicate(t *testing.T) {
	source := `<!DOCTYPE score-partwise [<!ATTLIST measure number ID #REQUIRED>]><score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Test</part-name></score-part></part-list><part id="P1"><measure number="M1"><link xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="#M1"/></measure></part></score-partwise>`
	if b := dtdIDGraph(t, source); !b.Satisfied() {
		t.Fatalf("measure ID lost: %+v", b.PackageReport())
	}
	// DTD duplicate IDs do not cause an invented validation failure. XSD ID
	// duplicates remain separately reported while Framework finds the first owner.
	source = `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Test</part-name></score-part></part-list><part id="P1"><measure number="1"><bookmark id="B"/><bookmark id="B"/><link xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="#B"/></measure></part></score-partwise>`
	b := dtdIDGraph(t, source)
	_, intrinsic, _ := b.ResourceReport(0)
	if intrinsic.Conformance != validation.ConformanceViolated {
		t.Fatal("XSD duplicate validity lost")
	}
	for _, f := range b.PackageReport().Findings {
		if f.Decision.Rule.ID == "MX40-PACKAGE-link-target" {
			t.Fatalf("duplicate typing lost: %+v", f)
		}
	}
}

func TestDTDIDRepeatedAdoptionAndModelSeparation(t *testing.T) {
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.musicxml#O1"/><opus-link xlink:href="alias.musicxml#O1"/></opus>`
	child := `<!DOCTYPE opus [<!ATTLIST opus version ID #IMPLIED>]><opus version="O1"/>`
	calls := 0
	var bodies []*resourceTestBody
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		body := &resourceTestBody{Reader: strings.NewReader(child)}
		bodies = append(bodies, body)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(child)))}, RetrievalURI: "https://e.test/child.musicxml", Body: body}, nil
	})
	b, e := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(root), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphInternalDTDDictionaries}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.musicxml"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}})
	if e != nil || !b.Satisfied() || b.ResourceCount() != 2 || b.AdoptionCount() != 3 || calls != 2 {
		t.Fatalf("adoption: err=%v resources=%d adoptions=%d calls=%d findings=%+v", e, b.ResourceCount(), b.AdoptionCount(), calls, b.PackageReport().Findings)
	}
	for _, body := range bodies {
		if body.closed != 1 {
			t.Fatalf("body closed %d times", body.closed)
		}
	}
	source := `<!DOCTYPE opus [<!ATTLIST opus version ID #IMPLIED>]><opus version="O1" xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="#O1"/></opus>`
	model, e := Decode(strings.NewReader(source))
	if e != nil {
		t.Fatal(e)
	}
	b, e = assessModelWithResourcesAndDictionaries(context.Background(), model, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphInternalDTDDictionaries}}})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range b.PackageReport().Findings {
		if f.Decision.Reason == "fragment-id-absent" {
			found = true
		}
	}
	if !found {
		t.Fatal("current model invented historical DTD identity")
	}
}

func TestDTDIDFirstOwnerBeforeDistinctSchemaOwner(t *testing.T) {
	source := `<!DOCTYPE score-partwise [<!ATTLIST measure number ID #IMPLIED>]><score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Test</part-name></score-part></part-list><part id="P1"><measure number="B"><bookmark id="B"/><link xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="#B"/></measure></part></score-partwise>`
	b := dtdIDGraph(t, source)
	// The independent audit did not certify this cross-domain collision as
	// schema-valid; assert only the fragment consumer's first-owner selection.
	for _, f := range b.PackageReport().Findings {
		if f.Decision.Rule.ID == "MX40-PACKAGE-link-target" {
			t.Fatalf("schema-only adapter overrode earlier DTD owner: %+v", f)
		}
	}
}
