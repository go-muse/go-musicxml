package musicxml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

const aggregateBudgetOpus = `<opus version="4.0" xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="first.musicxml"/><score xlink:href="second.musicxml"/></opus>`

// Both targets are intrinsically valid, have distinct canonical identities, and
// consume IDs and attributes. A per-document reset would incorrectly admit the
// one-under case because each individual target fits that aggregate limit.
func aggregateBudgetScore(part string) string {
	return `<score-partwise version="4.0"><part-list><score-part id="` + part + `"><part-name>P</part-name><score-instrument id="I` + part + `"><instrument-name>P</instrument-name></score-instrument></score-part></part-list><part id="` + part + `"><measure number="1"/></part></score-partwise>`
}

type aggregateBudgetObservation struct {
	bundle      validation.AssessmentBundle
	err         error
	account     *validation.Account
	beforeChild []int64
	bodies      []*resourceTestBody
}

// The measured path is the public entry point's existing orchestration with
// its account retained for final gauge checks; the public path is also run at
// the measured exact and one-under limits below.
func runAggregateBudgetGraph(t *testing.T, model, measured bool, kind validation.BudgetKind, limit int64) aggregateBudgetObservation {
	t.Helper()
	var out aggregateBudgetObservation
	budgets := validation.Budgets{}
	if limit > 0 {
		switch kind {
		case validation.BudgetIDs:
			budgets.MaxIDs = limit
		case validation.BudgetAttributes:
			budgets.MaxAttributes = limit
		default:
			t.Fatalf("unsupported test budget %s", kind)
		}
	}
	opts := LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: budgets}, Identity: ResourceIdentity{RetrievalURI: "https://example.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}}
	opts.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		if q.Document == nil || len(out.bodies) >= 2 {
			t.Fatalf("unexpected resolver request: %+v", q)
		}
		if out.account != nil {
			used, err := out.account.Usage(kind)
			if err != nil {
				t.Fatal(err)
			}
			out.beforeChild = append(out.beforeChild, used)
		}
		part := fmt.Sprintf("P%d", len(out.bodies)+1)
		data := aggregateBudgetScore(part)
		body := &resourceTestBody{Reader: strings.NewReader(data)}
		out.bodies = append(out.bodies, body)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: part, Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}, RetrievalURI: "https://example.test/" + part + ".musicxml", Body: body}, nil
	})
	ctx := context.Background()
	var doc Document
	if model {
		var err error
		doc, err = Decode(strings.NewReader(aggregateBudgetOpus))
		if err != nil {
			t.Fatal(err)
		}
	}
	if !measured {
		if model {
			out.bundle, out.err = AssessModelWithResources(ctx, doc, opts)
		} else {
			out.bundle, out.err = InspectXMLWithResources(ctx, strings.NewReader(aggregateBudgetOpus), opts)
		}
		return out
	}
	g, err := startResourceGraph(ctx, opts.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	out.account = g.options.account
	if model {
		engine, err := modelAssessmentEngineForProfile(ProfileModelLinks.ID())
		if err != nil {
			t.Fatal(err)
		}
		id := g.next("model")
		pin := g.primaryPin(validation.ResourceCurrentModel, "")
		result := assessResourceModel(ctx, doc, assessmentRunConfig{Context: validation.ContextFamilyAll, Account: g.options.account, DocumentID: id, Profile: ProfileModelLinks.ID(), Engine: engine})
		out.bundle, out.err = g.finishPrimary(result, pin, id)
	} else {
		engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
		if err != nil {
			t.Fatal(err)
		}
		id := g.next("source")
		result := assessResourceSource(ctx, strings.NewReader(aggregateBudgetOpus), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
		digest := ""
		if result.DigestComplete {
			digest = hex.EncodeToString(result.Digest[:])
		}
		out.bundle, out.err = g.finishPrimary(result, g.primaryPin(validation.ResourceSourceXML, digest), id)
	}
	return out
}

func assertAggregateBudgetCleanup(t *testing.T, out aggregateBudgetObservation, model bool) {
	t.Helper()
	if len(out.bodies) != 2 {
		t.Fatalf("resolver bodies=%d, want two distinct linked resources", len(out.bodies))
	}
	for i, b := range out.bodies {
		if b.closed != 1 {
			t.Errorf("body %d closed %d times", i, b.closed)
		}
	}
	if out.account == nil {
		return
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, err := out.account.Usage(kind)
		if err != nil || used != 0 {
			t.Errorf("final %s gauge=%d err=%v", kind, used, err)
		}
	}
	if model {
		used, err := out.account.Usage(validation.BudgetInputBytes)
		if err != nil || used != 0 {
			t.Errorf("direct model charged original source input bytes=%d err=%v", used, err)
		}
	}
}

func testAggregateBudgetBoundary(t *testing.T, model bool, kind validation.BudgetKind) {
	t.Helper()
	baseline := runAggregateBudgetGraph(t, model, true, kind, 0)
	assertAggregateBudgetCleanup(t, baseline, model)
	if baseline.err != nil || !baseline.bundle.EnumerationComplete() || baseline.bundle.ResourceCount() != 3 || baseline.bundle.PackageReport().Counts.Fail != 0 {
		t.Fatalf("invalid baseline: error=%v resources=%d package=%+v", baseline.err, baseline.bundle.ResourceCount(), baseline.bundle.PackageReport())
	}
	total, err := baseline.account.Usage(kind)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.beforeChild) != 2 || baseline.beforeChild[1] <= baseline.beforeChild[0] || total <= baseline.beforeChild[1] || baseline.beforeChild[1] > total-1 {
		t.Fatalf("counter is not cumulative across both children: before=%v total=%d", baseline.beforeChild, total)
	}
	if model && baseline.beforeChild[0] == 0 {
		t.Fatal("direct model root attributes were not charged before expansion")
	}
	first := baseline.beforeChild[1] - baseline.beforeChild[0]
	second := total - baseline.beforeChild[1]
	if first >= total-1 || second >= total-1 {
		t.Fatalf("fixture does not distinguish aggregate from per-child limit: first=%d second=%d total=%d", first, second, total)
	}
	t.Logf("%s: before-child=%v total=%d; each linked target individually fits one-under=%d", kind, baseline.beforeChild, total, total-1)
	for _, measured := range []bool{true, false} {
		for _, under := range []bool{false, true} {
			name := fmt.Sprintf("measured=%t/one-under=%t", measured, under)
			t.Run(name, func(t *testing.T) {
				limit := total
				if under {
					limit--
				}
				out := runAggregateBudgetGraph(t, model, measured, kind, limit)
				assertAggregateBudgetCleanup(t, out, model)
				if under {
					var resource *validation.ResourceError
					if !errors.Is(out.err, validation.ErrResourceLimit) || !errors.As(out.err, &resource) || resource.Kind != kind || resource.Limit != limit || resource.Used+resource.Requested <= limit || out.bundle.Satisfied() || out.bundle.EnumerationComplete() {
						t.Fatalf("one-under did not fail with exact aggregate budget: limit=%d err=%v complete=%t satisfied=%t", limit, out.err, out.bundle.EnumerationComplete(), out.bundle.Satisfied())
					}
				} else if out.err != nil || !out.bundle.EnumerationComplete() || out.bundle.ResourceCount() != 3 || out.bundle.PackageReport().Counts.Fail != 0 {
					t.Fatalf("exact aggregate cap failed: limit=%d err=%v resources=%d report=%+v", limit, out.err, out.bundle.ResourceCount(), out.bundle.PackageReport())
				}
				if measured {
					used, err := out.account.Usage(kind)
					if err != nil || used > limit || !under && used != total {
						t.Fatalf("usage=%d limit=%d total=%d err=%v", used, limit, total, err)
					}
					if len(out.beforeChild) != 2 || out.beforeChild[0] != baseline.beforeChild[0] || out.beforeChild[1] != baseline.beforeChild[1] {
						t.Fatalf("prior resources were reset or changed: before=%v want=%v", out.beforeChild, baseline.beforeChild)
					}
				}
			})
		}
	}
}

func TestResourceGraphAggregateSourceIDBoundary(t *testing.T) {
	testAggregateBudgetBoundary(t, false, validation.BudgetIDs)
}
func TestResourceGraphAggregateModelAttributeBoundary(t *testing.T) {
	testAggregateBudgetBoundary(t, true, validation.BudgetAttributes)
}
