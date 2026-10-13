package musicxml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// graphLeaseContext samples Account.Usage at existing cooperative checkpoints,
// before the getter's temporary-copy lease begins. It does not alter production
// code, cancellation behavior, or any accounting state.
type graphLeaseContext struct {
	context.Context
	account                                          *validation.Account
	contextBaseline, fragmentBaseline                int64
	associationScans, fragmentScans, fragmentOutputs []int64
}

func (c *graphLeaseContext) Err() error {
	if c.account == nil {
		return nil
	}
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		return nil
	}
	name := runtime.FuncForPC(pc).Name()
	if !strings.HasSuffix(name, ".(*ResourceAdoption).ContextAt") && !strings.HasSuffix(name, ".(*ResourceAdoption).IDAt") && !strings.HasSuffix(name, ".(*PackageSession).event") && !strings.HasSuffix(name, ".(*resourceGraphRun).enqueue") {
		return nil
	}
	var pcs [32]uintptr
	n := runtime.Callers(2, pcs[:])
	association, fragment, walk, adoption := false, false, false, false
	for _, p := range pcs[:n] {
		f := runtime.FuncForPC(p)
		if f == nil {
			continue
		}
		name := f.Name()
		association = association || strings.HasSuffix(name, ".(*resourceGraphRun).walkAssociation")
		fragment = fragment || strings.HasSuffix(name, ".(*resourceGraphRun).resolveFragment")
		walk = walk || strings.HasSuffix(name, ".(*resourceGraphRun).walkResources")
		adoption = adoption || strings.HasSuffix(name, ".(*PackageSession).Adoption")
	}
	used, _ := c.account.Usage(validation.BudgetBufferedBytes)
	switch {
	case strings.HasSuffix(name, ".(*ResourceAdoption).ContextAt") && walk && !association:
		c.contextBaseline = used
	case strings.HasSuffix(name, ".(*ResourceAdoption).IDAt") && association:
		c.associationScans = append(c.associationScans, used-c.contextBaseline)
	case strings.HasSuffix(name, ".(*PackageSession).event") && fragment && adoption:
		c.fragmentBaseline = used
	case strings.HasSuffix(name, ".(*ResourceAdoption).IDAt") && fragment:
		c.fragmentScans = append(c.fragmentScans, used-c.fragmentBaseline)
	case strings.HasSuffix(name, ".(*resourceGraphRun).enqueue") && c.fragmentBaseline != 0:
		c.fragmentOutputs = append(c.fragmentOutputs, used-c.fragmentBaseline)
		c.fragmentBaseline = 0
	}
	return nil
}
func graphLeaseRun(t *testing.T, source, target string, deps []ResourceDependency) *graphLeaseContext {
	t.Helper()
	ctx := &graphLeaseContext{Context: context.Background()}
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		if q.Document == nil {
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "metadata-unavailable"}, nil
		}
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "target", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(target)))}, RetrievalURI: "https://e.test/target.xml", Body: &resourceTestBody{Reader: strings.NewReader(target)}}, nil
	})
	g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.xml"}, Resolver: resolver, Dependencies: deps}, ProfileResourceGraph)
	if err != nil {
		t.Fatal(err)
	}
	ctx.account = g.options.account
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks)
	if err != nil {
		t.Fatal(err)
	}
	id := g.next("source")
	result := assessResourceSource(ctx, strings.NewReader(source), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks, Engine: engine}, ByteOwner: sourceBytesPrimary})
	if !result.DigestComplete {
		t.Fatal("fixture must be complete", result.Err)
	}
	_, err = g.finishPrimary(result, g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(result.Digest[:])), id)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, err := g.options.account.Usage(kind)
		if err != nil || used != 0 {
			t.Errorf("cleanup %s=%d err=%v", kind, used, err)
		}
	}
	return ctx
}
func TestResourceGraphAssociationWorkingLeases(t *testing.T) {
	source := `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="P1"><part-link xlink:href="part.musicxml"><instrument-link id="I1"/></part-link><part-name>P</part-name><score-instrument id="I1"><instrument-name>P</instrument-name></score-instrument></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	ctx := graphLeaseRun(t, source, resourceScore(""), []ResourceDependency{ResourceDocuments, ResourceAssociations})
	if len(ctx.associationScans) != 2 {
		t.Fatalf("want two ID copies, got %v", ctx.associationScans)
	}
	if ctx.associationScans[0] <= 0 {
		t.Errorf("association context/header copy has no root lease during first ID scan: %v", ctx.associationScans)
	}
	// The first ID is P1. Its selected text remains live when the I1 copy starts.
	if ctx.associationScans[1] <= ctx.associationScans[0] {
		t.Errorf("selected part ID has no additional root lease during next ID copy: %v", ctx.associationScans)
	}
}
func TestResourceGraphFragmentWorkingLeases(t *testing.T) {
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="target.xml#P1"/></opus>`
	ctx := graphLeaseRun(t, source, resourceScore(""), []ResourceDependency{ResourceDocuments})
	if len(ctx.fragmentScans) != 1 || len(ctx.fragmentOutputs) != 1 {
		t.Fatalf("missing fragment checkpoints: scans=%v outputs=%v", ctx.fragmentScans, ctx.fragmentOutputs)
	}
	if ctx.fragmentScans[0] <= 0 {
		t.Errorf("fragment header copy has no root lease during ID copy: %v", ctx.fragmentScans)
	}
	if ctx.fragmentOutputs[0] <= 0 {
		t.Errorf("returned fragment target has no root lease during enqueue: %v", ctx.fragmentOutputs)
	}
}
