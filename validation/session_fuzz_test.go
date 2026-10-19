package validation

import (
	"context"
	"errors"
	"testing"
)

// Arbitrary event streams must terminate conservatively and release live gauges;
// malformed adapter input is a contract error, never a panic or successful report.
func FuzzSessionLifecycle(f *testing.F) {
	for _, seed := range [][]byte{{}, {0, 1}, {0, 0, 1, 1}, {0, 2, 1}, {0, 3}, {0, 5}, {1}, {0, 4}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64 {
			data = data[:64]
		}
		_, s := sessionFixture(t, Budgets{MaxNodes: 16, MaxDepth: 8, MaxAttributes: 16, MaxEvents: 64, MaxBufferedBytes: 65536, MaxRuleSteps: 10000})
		account := s.account
		ctx := context.Background()
		var eventErr error
		next := NodeID(1)
		for _, op := range data {
			var top NodeID
			if len(s.stack) != 0 {
				top = s.stack[len(s.stack)-1].header.ID
			}
			switch op % 7 {
			case 0:
				occurrence, name := "test.child", "child"
				if top == 0 {
					occurrence, name = "test.root", "root"
				}
				eventErr = s.OpenNode(ctx, factHeader(next, top, occurrence, name))
				next++
			case 1, 3:
				end := factEnd(top)
				if op%7 == 3 {
					end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "fuzz-incomplete"}
				}
				eventErr = s.CloseNode(ctx, end)
			case 2:
				eventErr = s.Attribute(ctx, AttributeFact{Owner: top, Name: QName{Local: "id"}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "unbound"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: "value"}, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "value"}}}, Location: Location{Target: TargetSource, DocumentID: "doc", Path: "/@id"}})
			case 4:
				eventErr = s.OpenNode(ctx, factHeader(next, top+100, "test.child", "child"))
				next++
			case 5:
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				eventErr = s.CloseNode(canceled, factEnd(top))
			case 6:
				data = nil
			}
			if eventErr != nil || data == nil {
				break
			}
		}
		complete := s.root != nil && len(s.stack) == 0 && eventErr == nil
		syntax := SyntaxIncomplete
		if complete {
			syntax = SyntaxWellFormed
		}
		report, err := s.Finish(ctx, Completion{InputComplete: complete, Syntax: syntax})
		if eventErr != nil && (!errors.Is(err, eventErr) || !errors.Is(err, ErrIncompleteAssessment)) {
			t.Fatalf("event cause lost: event=%v final=%v", eventErr, err)
		}
		if (err != nil || !complete) && report.Satisfied() {
			t.Fatal("interrupted assessment satisfied")
		}
		if report.Satisfied() && (!report.AssessmentComplete || report.Conformance != ConformanceConformant) {
			t.Fatal("contradictory satisfied report")
		}
		for _, kind := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
			if used, err := account.Usage(kind); err != nil || used != 0 {
				t.Fatalf("gauge %s after Finish=%d, %v", kind, used, err)
			}
		}
		if err := s.OpenNode(ctx, factHeader(next, 0, "test.root", "root")); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("post-Finish Open=%v", err)
		}
		if _, err := s.Finish(ctx, Completion{}); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("second Finish=%v", err)
		}
	})
}
