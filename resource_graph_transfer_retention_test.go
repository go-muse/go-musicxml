package musicxml

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Exported intrinsic failure remains eligible for ordinary retention while the
// package session is healthy and the shared work/storage quotas admit the copy.
func TestQAPostExportRetentionKeepsFullReportWhenAffordable(t *testing.T) {
	g, intrinsic, pin, _ := qaInvalidIntrinsicGraph(t, context.Background())
	expected := intrinsic.Report
	if err := validationPackageSessionTerminalCause(g.session); err != nil {
		t.Fatal("fixture package already terminal", err)
	}
	cause := errors.New("fixture pre-admission failure")
	err := g.retainUntransferredIntrinsic(intrinsic, pin, cause)
	b, err := g.finish(err)
	if !errors.Is(err, cause) {
		t.Fatal("original cause lost", err)
	}
	r, ok := b.FailureReport()
	if !ok || !reflect.DeepEqual(r, expected) {
		t.Fatalf("affordable original report lost or rewritten: ok=%t got=%+v want=%+v", ok, r, expected)
	}
	f, ok := b.Failure()
	if !ok || f.Pin != pin || f.Counts != expected.Counts || f.DiagnosticsTruncated || f.MetadataTruncated {
		t.Fatalf("ordinary retained report mislabeled: %+v", f)
	}
	if b.ResourceCount() != 0 || b.AdoptionCount() != 0 || b.Satisfied() || b.EnumerationComplete() {
		t.Fatal("untransferred failure promoted into completed graph")
	}
	if used, _ := g.options.account.Usage(validation.BudgetBufferedBytes); used != 0 {
		t.Fatalf("retained storage not released: %d", used)
	}
}

// Once ordinary package admission terminates, freeing snapshot storage does not
// reopen ordinary report measurement/copy work. Only prepaid summary ingress runs.
func TestQAPostExportRetentionAfterTerminalKeepsOnlySummary(t *testing.T) {
	ctx := &qaPostExportBufferContext{Context: context.Background(), stage: "OpenPackage"}
	g, intrinsic, pin, id := qaInvalidIntrinsicGraph(t, ctx)
	expected := intrinsic.Report
	ctx.account = g.options.account
	b, err := g.finishPrimary(intrinsic, pin, id)
	defer func() { _ = ctx.account.Release(validation.BudgetBufferedBytes, ctx.held) }()
	if !ctx.fired || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal("terminal admission seam not reached", err)
	}
	if _, ok := b.FailureReport(); ok {
		t.Fatal("terminal session admitted a new full report copy")
	}
	f, ok := b.Failure()
	if !ok || f.Pin != pin || f.Target != expected.Target || f.Profile != expected.Profile.ID || f.CatalogDigest != expected.Profile.CatalogDigest || f.Counts != expected.Counts || f.Conformance != expected.Conformance || !f.DiagnosticsTruncated || !f.MetadataTruncated {
		t.Fatalf("prepaid original facts lost or mislabeled: %+v", f)
	}
	if b.ResourceCount() != 0 || b.AdoptionCount() != 0 || b.Satisfied() || b.EnumerationComplete() {
		t.Fatal("untransferred failure promoted into completed graph")
	}
	if used, _ := ctx.account.Usage(validation.BudgetBufferedBytes); used != ctx.held {
		t.Fatalf("own cleanup altered foreign hold: %d want %d", used, ctx.held)
	}
}

// Step refusal can make full-report traversal unaffordable even with spare buffer.
func TestQAPostExportRetentionKeepsBoundedSummaryWhenWorkRefused(t *testing.T) {
	g, intrinsic, pin, id := qaInvalidIntrinsicGraph(t, context.Background())
	steps, _ := g.options.account.Usage(validation.BudgetRuleSteps)
	expected := intrinsic.Report
	if _, err := g.finishPrimary(intrinsic, pin, id); err != nil {
		t.Fatal(err)
	}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus wrong="yes"/>`), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxRuleSteps: steps}}})
	if !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal(err)
	}
	if _, ok := b.FailureReport(); ok {
		t.Fatal("unadmitted report traversal unexpectedly retained full copy")
	}
	f, ok := b.Failure()
	if !ok || f.Counts != expected.Counts || f.Conformance != expected.Conformance || f.Profile != expected.Profile.ID || !f.MetadataTruncated || !f.DiagnosticsTruncated {
		t.Fatalf("fallback summary lost truth or truncation labels: %+v", f)
	}
	if b.ResourceCount() != 0 {
		t.Fatalf("untransferred report promoted canonical resource: %d", b.ResourceCount())
	}
}
