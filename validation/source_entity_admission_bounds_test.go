package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSourceEntityDeclarationNameScanPollsCancellation(t *testing.T) {
	e := entityDeclarationEngine(t, entityDeclarationSchema(t, entityBuiltin("string")), XSDCheckScalar)
	s := entityDeclarationSession(t, e)
	ctx := context.Background()
	if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	probe := &rationalCancelContext{Context: ctx, at: 8}
	facts := entitySnapshot(true, entityRow(strings.Repeat("a", 65536), true, 1))
	err := s.sourceEntityDeclarations(probe, facts)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("large name scan did not observe bounded cancellation: %v", err)
	}
	if s.sourceEntities != nil {
		t.Error("partially inspected declaration snapshot installed")
	}
	_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
	if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
		t.Fatalf("retained=%d", n)
	}
}
func TestSourceEntityDeclarationMalformedMetadataIsRejected(t *testing.T) {
	e := entityDeclarationEngine(t, entityDeclarationSchema(t, entityBuiltin("string")), XSDCheckScalar)
	for _, tc := range []struct {
		name   string
		change func(*SourceEntityDeclarations)
	}{
		{"coordinate-system", func(f *SourceEntityDeclarations) {
			f.Declarations[0].Origin = Evidence[Location]{State: EvidenceKnown, Value: Location{Target: TargetSource, DocumentID: "source", Position: &SourcePosition{CoordinateSpace: "replacement-text"}}}
		}},
		{"oversized-path", func(f *SourceEntityDeclarations) {
			f.Declarations[0].Origin = Evidence[Location]{State: EvidenceKnown, Value: Location{Target: TargetSource, DocumentID: "source", Path: strings.Repeat("x", 16385), Position: &SourcePosition{CoordinateSpace: "original-bytes"}}}
		}},
		{"oversized-reason", func(f *SourceEntityDeclarations) {
			f.Complete = Evidence[bool]{State: EvidenceUnknown, Reason: Code(strings.Repeat("x", 1025))}
		}},
		{"invalid-name-utf8", func(f *SourceEntityDeclarations) { f.Declarations[0].Name = "a\xff" }},
		{"invalid-completeness-cause", func(f *SourceEntityDeclarations) {
			f.Complete = Evidence[bool]{State: EvidenceInvalid, Reason: "invalid", Cause: &DecisionRef{Rule: RuleKey{ID: "invented", Revision: 1}, Subject: NodeRef{"source", 1}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := entityDeclarationSession(t, e)
			if err := s.OpenNode(context.Background(), schemaPendingHeader()); err != nil {
				t.Fatal(err)
			}
			facts := entitySnapshot(true, entityRow("asset", true, 1))
			tc.change(&facts)
			err := s.sourceEntityDeclarations(context.Background(), facts)
			if err == nil {
				t.Fatal("malformed facts accepted")
			}
			_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete, Cause: err})
		})
	}
}

func TestSourceEntityDeclarationMapSizeArithmetic(t *testing.T) {
	hostMax := int64(^uint(0) >> 1)
	for _, extra := range []int64{0, 1024} {
		count := (hostMax - 512 - extra) / 512
		if _, err := sourceEntityMapBytes(int(count), extra); err != nil {
			t.Fatalf("exact admitted bound: %v", err)
		}
		if _, err := sourceEntityMapBytes(int(count+1), extra); err == nil {
			t.Fatal("overflowing map count admitted")
		}
	}
	for _, tc := range []struct {
		count int
		extra int64
	}{{-1, 0}, {0, -1}, {0, hostMax}, {1, hostMax - 512}} {
		if _, err := sourceEntityMapBytes(tc.count, tc.extra); err == nil {
			t.Fatalf("bad dimensions %+v", tc)
		}
	}
}
