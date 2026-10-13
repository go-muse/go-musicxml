package musicxml

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestQAPostExportRetentionKeepsFullReportWhenAffordable(t *testing.T) {
	ctx := &qaPostExportBufferContext{Context: context.Background(), stage: "OpenPackage"}
	g, intrinsic, pin, id := qaInvalidIntrinsicGraph(t, ctx)
	expected := intrinsic.Report
	ctx.account = g.options.account
	b, err := g.finishPrimary(intrinsic, pin, id)
	defer func() { _ = ctx.account.Release(validation.BudgetBufferedBytes, ctx.held) }()

	if !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal(err)
	}
	r, ok := b.FailureReport()
	if !ok || r.Counts != expected.Counts || r.Profile.ID != expected.Profile.ID || r.Target != expected.Target || r.Conformance != expected.Conformance || len(r.Findings) != len(expected.Findings) || r.DiagnosticsTruncated != expected.DiagnosticsTruncated {
		t.Fatalf("affordable full report lost or rewritten: ok=%t got=%+v want=%+v", ok, r.Counts, expected.Counts)
	}
	if b.ResourceCount() != 0 {
		t.Fatalf("untransferred report promoted canonical resource: %d", b.ResourceCount())
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
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus wrong="yes"/>`), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxRuleSteps: steps}}})
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
