package musicxml

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func qaInvalidIntrinsicGraph(t *testing.T, ctx context.Context) (*resourceGraphRun, resourceIntrinsicResult, validation.ResourcePin, string) {
	t.Helper()
	g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}}.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
	if err != nil {
		t.Fatal(err)
	}
	id := g.next("source")
	result := assessResourceSource(ctx, strings.NewReader(`<opus wrong="yes"/>`), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
	if result.Err != nil || result.Snapshot == nil || result.Report.Counts.Fail == 0 || result.Report.Conformance != validation.ConformanceViolated {
		t.Fatalf("invalid primary did not export real best report: %v %+v", result.Err, result.Report)
	}
	return g, result, g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(result.Digest[:])), id
}
func qaBundleHasKnownChildFailure(b validation.AssessmentBundle) bool {
	if f, ok := b.Failure(); ok && f.Counts.Fail > 0 && f.Conformance == validation.ConformanceViolated {
		return true
	}
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		if r.Counts.Fail > 0 && r.Conformance == validation.ConformanceViolated {
			return true
		}
	}
	return false
}
func TestQAPrimaryPostExportStepRefusalPreservesKnownChildReport(t *testing.T) {
	g, result, pin, id := qaInvalidIntrinsicGraph(t, context.Background())
	steps, _ := g.options.account.Usage(validation.BudgetRuleSteps)
	baseline, err := g.finishPrimary(result, pin, id)
	if err != nil || !qaBundleHasKnownChildFailure(baseline) {
		t.Fatal("baseline", err)
	}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus wrong="yes"/>`), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxRuleSteps: steps}}})
	if !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("expected post-export step refusal at %d: %v", steps, err)
	}
	if !qaBundleHasKnownChildFailure(b) {
		t.Fatalf("known invalid primary report lost at post-export step limit=%d: resources=%d package=%+v", steps, b.ResourceCount(), b.PackageReport())
	}
}

type qaPostExportBufferContext struct {
	context.Context
	account  *validation.Account
	stage    string
	ancestor string
	held     int64
	fired    bool
}

func (c *qaPostExportBufferContext) Err() error {
	if c.account != nil && !c.fired {
		pc, _, _, ok := runtime.Caller(2)
		if ok {
			f := runtime.FuncForPC(pc)
			if f != nil && strings.HasSuffix(f.Name(), ".(*PackageSession)."+strings.ToLower(c.stage[:1])+c.stage[1:]) {
				matched := c.ancestor == ""
				if !matched {
					var callers [24]uintptr
					n := runtime.Callers(3, callers[:])
					frames := runtime.CallersFrames(callers[:n])
					for {
						frame, more := frames.Next()
						if strings.HasSuffix(frame.Function, c.ancestor) {
							matched = true
							break
						}
						if !more {
							break
						}
					}
				}
				if !matched {
					return c.Context.Err()
				}
				used, _ := c.account.Usage(validation.BudgetBufferedBytes)
				c.held = c.account.Limits().MaxBufferedBytes - used - 1
				if c.held > 0 && c.account.Charge(validation.BudgetBufferedBytes, c.held) == nil {
					c.fired = true
				}
			}
		}
	}
	return c.Context.Err()
}
func TestQAPrimaryPostExportBufferRefusalPreservesKnownChildReport(t *testing.T) {
	for _, stage := range []string{"OpenPackage", "ObserveFileMetadata", "AddResource", "AdoptResource"} {
		t.Run(stage, func(t *testing.T) {
			ctx := &qaPostExportBufferContext{Context: context.Background(), stage: stage}
			g, result, pin, id := qaInvalidIntrinsicGraph(t, ctx)
			ctx.account = g.options.account
			if stage == "ObserveFileMetadata" {
				g.options.identity.RetrievalURI = "https://example.test/primary.musicxml"
			}
			b, err := g.finishPrimary(result, pin, id)
			if !ctx.fired || !errors.Is(err, validation.ErrResourceLimit) {
				t.Fatalf("stage not reached under injected live hold: %t %v", ctx.fired, err)
			}
			if !qaBundleHasKnownChildFailure(b) {
				t.Errorf("known child failure lost after export at %s: %+v", stage, b.PackageReport())
			}
			if used, _ := ctx.account.Usage(validation.BudgetBufferedBytes); used != ctx.held {
				t.Errorf("own cleanup altered foreign hold: %d want%d", used, ctx.held)
			}
			_ = ctx.account.Release(validation.BudgetBufferedBytes, ctx.held)
		})
	}
}
func TestQANestedSourcePostExportAdmissionPreservesKnownChildReport(t *testing.T) {
	ctx := &qaPostExportBufferContext{Context: context.Background(), stage: "AddResource"}
	g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}}.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.newPackage(g.primaryPin(validation.ResourceRawMXL, strings.Repeat("b", 64)), "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx.account = g.options.account
	data := []byte(`<opus wrong="yes"/>`)
	if err = g.options.account.Charge(validation.BudgetExternalBytes, int64(len(data))); err != nil {
		t.Fatal(err)
	}
	resource, _, err := g.assessPackageSource(p, "bad.opus", bytes.Clone(data), ProfileSourceLinks.ID())
	if resource != nil || !ctx.fired || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("expected actual nested transfer failure: %v %t %v", resource, ctx.fired, err)
	}
	b, err := g.finish(err)
	if !qaBundleHasKnownChildFailure(b) {
		t.Errorf("known invalid nested child report lost on transfer: %v %+v", err, b.PackageReport())
	}
	if used, _ := ctx.account.Usage(validation.BudgetBufferedBytes); used != ctx.held {
		t.Errorf("own cleanup altered foreign hold: %d want%d", used, ctx.held)
	}
	_ = ctx.account.Release(validation.BudgetBufferedBytes, ctx.held)
}

func TestQACurrentPackagePostExportAdmissionPreservesKnownChildReport(t *testing.T) {
	ctx := &qaPostExportBufferContext{Context: context.Background(), stage: "AddResource"}
	g, err := startResourceGraph(ctx, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}}.internal().linked(), ProfileMXLModel.ID())
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.newPackage(g.primaryPin(validation.ResourceCurrentMXL, ""), "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Decode(strings.NewReader(`<score-partwise version="4.0"><part-list/></score-partwise>`))
	if err != nil {
		t.Fatal(err)
	}
	control, err := AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelLinks})
	if err != nil || control.Counts.Fail == 0 {
		t.Fatalf("model fixture not independently invalid: %v %+v", err, control.Counts)
	}
	ctx.account = g.options.account
	err = g.importCurrentPackage(p, &MXLPackage{Document: doc}, "")
	if !ctx.fired || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("current transfer seam not reached: %t %v", ctx.fired, err)
	}
	b, err := g.finish(err)
	if !qaBundleHasKnownChildFailure(b) {
		t.Errorf("known invalid current child report lost on transfer: %v", err)
	}
	if used, _ := ctx.account.Usage(validation.BudgetBufferedBytes); used != ctx.held {
		t.Errorf("own cleanup altered foreign hold: %d want%d", used, ctx.held)
	}
	_ = ctx.account.Release(validation.BudgetBufferedBytes, ctx.held)
}

func TestQAPrimaryPostExportCancellationPreservesKnownChildReport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g, result, pin, id := qaInvalidIntrinsicGraph(t, ctx)
	cancel()
	b, err := g.finishPrimary(result, pin, id)
	if !errors.Is(err, context.Canceled) || !qaBundleHasKnownChildFailure(b) {
		t.Fatalf("post-export cancellation lost known child failure: %v %+v", err, b.PackageReport())
	}
	if used, _ := g.options.account.Usage(validation.BudgetBufferedBytes); used != 0 {
		t.Fatalf("retained after canceled transfer: %d", used)
	}
}

func TestQAExternalChildPostExportAdmissionPreservesKnownChildReport(t *testing.T) {
	ctx := &qaPostExportBufferContext{Context: context.Background(), stage: "AddResource", ancestor: ".(*resourceGraphRun).loadExternalDocument"}
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="target.opus"/></opus>`
	resolver := &graphFixtureResolver{resources: map[string]string{"https://e.test/target.opus": `<opus wrong="yes"/>`}, pins: map[string]string{"https://e.test/target.opus": "target"}}
	g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}, Identity: ResourceIdentity{CanonicalID: "root", RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	ctx.account = g.options.account
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
	if err != nil {
		t.Fatal(err)
	}
	id := g.next("source")
	result := assessResourceSource(ctx, strings.NewReader(root), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
	if result.Err != nil || result.Snapshot == nil {
		t.Fatalf("source root: %v", result.Err)
	}
	b, err := g.finishPrimary(result, g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(result.Digest[:])), id)
	if !ctx.fired || resolver.calls != 1 || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("external transfer seam not reached: %t calls=%d err=%v", ctx.fired, resolver.calls, err)
	}
	if !qaBundleHasKnownChildFailure(b) {
		t.Errorf("known invalid external child report lost on transfer: %v", err)
	}
	if used, _ := ctx.account.Usage(validation.BudgetBufferedBytes); used != ctx.held {
		t.Errorf("own cleanup altered foreign hold: %d want%d", used, ctx.held)
	}
	_ = ctx.account.Release(validation.BudgetBufferedBytes, ctx.held)
}
