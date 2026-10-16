package musicxml

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func sourceDTDIDOneAssemblyEngine(t *testing.T) *validation.Engine {
	t.Helper()
	d, err := buildSourceCompositionDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	// Keep both independently assessed assemblies, but elect DTD fragment
	// identity only for MusicXML. No cached production catalog is mutated.
	var predicate string
	for _, rule := range d.rules {
		if rule.Key.ID == "MX40-RESOURCE-local-links/opus" {
			predicate = rule.Predicate
		}
	}
	plan, err := validation.NewLocalLinkContextPlan(d.schemas[validation.AssemblyOpus])
	if err != nil || predicate == "" {
		t.Fatalf("Opus local plan: %v", err)
	}
	for i := range d.plans {
		if d.plans[i].ID == predicate {
			d.plans[i].Plan = plan
		}
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := validation.NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestSourceDTDIDObserverUsesActiveAssemblySelection(t *testing.T) {
	engine := sourceDTDIDOneAssemblyEngine(t)
	if !engine.SourceDTDIDsSelected(ProfileSourceInternalDTDLinks) {
		t.Fatal("pre-root capture must preserve declarations for the selected assembly")
	}
	t.Run("unselected-opus", func(t *testing.T) {
		account := fragmentCompositionAccount(t)
		for _, tc := range []struct {
			input string
			valid bool
		}{
			{`<opus version="4.0"/>`, true},
			{`<!DOCTYPE opus [<!ATTLIST opus version ID #IMPLIED>]><opus version="O1"/>`, true},
			{`<opus invalid="x"/>`, false},
		} {
			resource := fragmentCompositionSource(t, engine, ProfileSourceInternalDTDLinks, account, "unselected-opus", tc.input, true)
			if resource.report.Satisfied() != tc.valid || resource.report.Terminal != nil || !tc.valid && resource.report.Conformance != validation.ConformanceViolated {
				t.Fatalf("unselected DTD input disrupted independent Opus XSD assessment: %+v", resource.report)
			}
		}
	})
	t.Run("selected-musicxml", func(t *testing.T) {
		account := fragmentCompositionAccount(t)
		input := `<!DOCTYPE score-partwise [<!ATTLIST measure number ID #IMPLIED>]><score-partwise><part-list><score-part id="P1"><part-name>Part</part-name></score-part></part-list><part id="P1"><measure number="anchor"/></part></score-partwise>`
		resource := fragmentCompositionSource(t, engine, ProfileSourceInternalDTDLinks, account, "selected-musicxml", input, true)
		if !resource.report.Satisfied() {
			t.Fatalf("selected MusicXML assessment: %+v", resource.report)
		}
		p := newFragmentCompositionPackage(t, account, resource)
		_, view := p.adopt(t, p.add(t, resource), resource.pin.CanonicalID, true)
		fragmentCompositionResolve(t, view, "anchor", validation.EvidenceKnown, 6)
		_ = p.finish(t)
	})
}

func TestSourceDTDIDCustomAssemblyCatalogDoesNotBypassPreflight(t *testing.T) {
	engine := sourceDTDIDOneAssemblyEngine(t)
	account := fragmentCompositionAccount(t)
	reader := &assessmentCountingReader{source: strings.NewReader(`<opus/>`)}
	_, err := newSourceAssessmentRunWithConfig(context.Background(), reader, DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: account, DocumentID: "source", Profile: ProfileSourceInternalDTDLinks, Engine: engine}, ByteOwner: sourceBytesPrimary}, false)
	if !errors.Is(err, validation.ErrInvalidConfiguration) || reader.reads != 0 {
		t.Fatalf("custom catalog bypassed pre-I/O production identity check: err=%v reads=%d", err, reader.reads)
	}
}
