package validation

import (
	"context"
	"testing"
)

func TestTimelineDisplayStateDocumentedDefaultsAndMidmeasureApplicability(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	ctx := context.Background()
	key := keyState{Cancel: stateKnown(stateInt(t, -2)), CancelLocation: stateAbsent[string]()}
	for _, offset := range []int64{0, 1} {
		got, err := key.cancelLocation(ctx, s, stateKnown(statePosition(t, 0, offset, 1)))
		if err != nil || got.State != EvidenceKnown || got.Value != "left" {
			t.Fatalf("default=%+v %v", got, err)
		}
	}
	key.CancelLocation = stateKnown("before-barline")
	for _, tc := range []struct {
		offset int64
		want   string
	}{{0, "before-barline"}, {1, "left"}} {
		got, err := key.cancelLocation(ctx, s, stateKnown(statePosition(t, 0, tc.offset, 1)))
		if err != nil || got.State != EvidenceKnown || got.Value != tc.want {
			t.Fatalf("midmeasure=%+v %v", got, err)
		}
	}
	clef := clefState{AfterBarline: stateKnown(true)}
	first, err := clef.afterBarlineAt(ctx, s, stateKnown(statePosition(t, 0, 0, 1)))
	if err != nil || first.State != EvidenceKnown || !first.Value {
		t.Fatalf("start=%+v %v", first, err)
	}
	middle, err := clef.afterBarlineAt(ctx, s, stateKnown(statePosition(t, 0, 1, 1)))
	if err != nil || middle.State != EvidenceAbsent {
		t.Fatalf("middle=%+v %v", middle, err)
	}
	cause := &DecisionRef{Rule: RuleKey{"invalid", 1}, Subject: NodeRef{"d", 9}}
	clef.AfterBarline = Evidence[bool]{State: EvidenceInvalid, Cause: cause}
	middle, err = clef.afterBarlineAt(ctx, s, stateKnown(statePosition(t, 0, 1, 1)))
	if err != nil || middle.State != EvidenceAbsent {
		t.Fatalf("irrelevant=%+v %v", middle, err)
	}
	first, err = clef.afterBarlineAt(ctx, s, stateKnown(statePosition(t, 0, 0, 1)))
	if err != nil || first.State != EvidenceInvalid || first.Cause == cause || first.Cause == nil || *first.Cause != *cause {
		t.Fatalf("cause=%+v %v", first, err)
	}
	key.CancelLocation = stateKnown("before-barline")
	got, err := key.cancelLocation(ctx, s, stateUnknown[musicalPosition]("position-unknown"))
	if err != nil || got.State != EvidenceUnknown {
		t.Fatalf("missingposition=%+v %v", got, err)
	}
	key.CancelLocation = Evidence[string]{State: EvidenceInvalid, Cause: cause}
	got, err = key.cancelLocation(ctx, s, stateUnknown[musicalPosition]("position-unknown"))
	if err != nil || got.State != EvidenceInvalid || got.Cause == nil || *got.Cause != *cause {
		t.Fatalf("invalid=%+v %v", got, err)
	}
	key.Cancel = stateAbsent[Integer]()
	key.CancelLocation = stateAbsent[string]()
	got, err = key.cancelLocation(ctx, s, stateKnown(statePosition(t, 0, 0, 1)))
	if err != nil || got.State != EvidenceAbsent {
		t.Fatalf("absentcancel=%+v %v", got, err)
	}
}
