package validation

import (
	"context"
	"testing"
)

func semanticQAKey(t *testing.T, b *timelineBindings, id NodeID, fifths int64, staff int64) *factNode {
	t.Helper()
	key := stateNode(b, timelineRoleAttributesKey, id, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, id+1, stateAtomicInteger(t, fifths)))
	if staff != 0 {
		stateAttribute(b, key, timelineRoleKeyNumber, Atomic{Kind: AtomicInteger, Integer: stateInt(t, staff)})
	}
	return stateNode(b, timelineRoleAttributes, id+2, stateAbsent[Atomic](), key)
}

func TestTimelineSemanticQARuntimeNewestAllStaffStateAndHighWaterCarry(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	initial := semanticQAKey(t, b, 20, 0, 0)
	initial.children = append(initial.children, stateNode(b, timelineRoleAttributesDivisions, 24, stateAtomicDecimal(t, 1, 1)), stateNode(b, timelineRoleAttributesStaves, 25, stateAtomicInteger(t, 2)))
	local := semanticQAKey(t, b, 40, 2, 2)
	all := semanticQAKey(t, b, 60, 3, 0)
	rewind := stateNode(b, timelineRoleBackup, 80, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 81, stateAtomicDecimal(t, 3, 1)))
	// XML order visits 0,1,2,3 and finally 0 again. Current lexical cursor 1
	// is not the carry position; the latest established state remains at 2.
	block := runtimeBlock(b, 0, initial, runtimeNote(t, b, 30, 1, false), local, runtimeNote(t, b, 50, 1, false), all, runtimeNote(t, b, 70, 1, false), rewind, runtimeNote(t, b, 90, 1, false))
	if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		event NodeID
		staff int64
		want  string
	}{{30, 1, "0"}, {30, 2, "0"}, {50, 1, "0"}, {50, 2, "2"}, {70, 1, "3"}, {70, 2, "3"}, {90, 2, "0"}} {
		at := svc.Facts[tc.event].Position
		if at.State != EvidenceKnown {
			t.Fatalf("position%d=%+v", tc.event, at)
		}
		key, err := svc.State.effectiveKey(context.Background(), s, at.Value, stateKnown(stateInt(t, tc.staff)))
		if err != nil || key.Value.State != EvidenceKnown || key.Value.Value.Fifths.Value.String() != tc.want {
			t.Errorf("node%d staff%d key=%+v err=%v want%s", tc.event, tc.staff, key, err, tc.want)
		}
	}
	next := runtimeBlock(b, 1, runtimeNote(t, b, 100, 1, false))
	if err := svc.readMeasure(context.Background(), s, b, next); err != nil {
		t.Fatal(err)
	}
	key, err := svc.State.effectiveKey(context.Background(), s, svc.Facts[100].Position.Value, stateKnown(stateInt(t, 2)))
	if err != nil || key.Value.State != EvidenceKnown || key.Value.Value.Fifths.Value.String() != "3" || !key.Inherited {
		t.Fatalf("last lexical cursor replaced musical carry: %+v %v", key, err)
	}
	if svc.Facts[30].Position.Value.Measure == svc.Facts[100].Position.Value.Measure {
		t.Fatal("different measure occurrences collapsed")
	}
}

func TestTimelineSemanticQARuntimeExactThirdsPrincipalAndAbsentVoice(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	divisions := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 3, 1)))
	events := []*factNode{divisions, runtimeNote(t, b, 30, 4, false), runtimeNote(t, b, 40, 1, true), runtimeNote(t, b, 50, 3, true), runtimeNote(t, b, 60, 1, false)}
	if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, events...)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id              NodeID
		onset, duration string
	}{{30, "0", "4/3"}, {40, "0", "1/3"}, {50, "0", "1"}, {60, "4/3", "1/3"}} {
		f := svc.Facts[tc.id]
		if f.Position.State != EvidenceKnown || f.Position.Value.Offset.String() != tc.onset || f.Duration.State != EvidenceKnown || f.Duration.Value.String() != tc.duration {
			t.Errorf("node%d=%+v want onset%s duration%s", tc.id, f, tc.onset, tc.duration)
		}
		if f.Voice.State != EvidenceAbsent {
			t.Errorf("invented default voice: %+v", f.Voice)
		}
		if tc.id == 40 || tc.id == 50 {
			if f.Principal.State != EvidenceKnown || f.Principal.Value.Node != 30 || f.PrincipalDuration.State != EvidenceKnown || f.PrincipalDuration.Value.String() != "4/3" {
				t.Errorf("adjacent chord replaced principal: %+v", f)
			}
		}
	}
}

func TestTimelineSemanticQARuntimeUnobservedNoteBreaksPrincipal(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	divisions := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	hidden := runtimeNote(t, b, 40, 1, false)
	hidden.header.Declaration = Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "qa-intervening-note-binding-unavailable"}
	if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, divisions, runtimeNote(t, b, 30, 4, false), hidden, runtimeNote(t, b, 50, 2, true))); err != nil {
		t.Fatal(err)
	}
	got := svc.Facts[50]
	if got.Principal.State == EvidenceKnown {
		t.Fatalf("unobserved intervening note borrowed older principal: %+v", got)
	}
	if got.Principal.Reason != "qa-intervening-note-binding-unavailable" {
		t.Fatalf("intervening cause was erased: %+v", got)
	}
}
