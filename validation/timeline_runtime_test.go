package validation

import (
	"context"
	"testing"
)

func runtimeFixture(t *testing.T) (*Session, *timelineService, *timelineBindings) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	t.Cleanup(s.cleanup)
	service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	stateClose(t, s, &service.State, 5)
	return s, service, stateBindings()
}
func runtimeBlock(b *timelineBindings, ordinal uint64, children ...*factNode) timelineMeasureBlock {
	n := stateNode(b, timelineRoleMeasure, NodeID(500+ordinal), stateAbsent[Atomic](), children...)
	return timelineMeasureBlock{content: n, measure: n, part: stateKnown(NodeRef{"d", 1}), ordinal: ordinal, complete: stateKnown(true)}
}
func runtimeNote(t *testing.T, b *timelineBindings, id NodeID, duration int64, chord bool) *factNode {
	children := []*factNode{stateNode(b, timelineRoleNoteDuration, id+1, stateAtomicDecimal(t, duration, 1))}
	if chord {
		children = append(children, stateNode(b, timelineRoleNoteChord, id+2, stateAbsent[Atomic]()))
	}
	return stateNode(b, timelineRoleNote, id, stateAbsent[Atomic](), children...)
}
func TestTimelineRuntimeExactMeasureAndInheritance(t *testing.T) {
	s, service, b := runtimeFixture(t)
	attr := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 3, 1)))
	backup := stateNode(b, timelineRoleBackup, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 61, stateAtomicDecimal(t, 2, 1)))
	key := stateNode(b, timelineRoleAttributes, 70, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesKey, 71, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, 72, stateAtomicInteger(t, 2))))
	block := runtimeBlock(b, 0, attr, runtimeNote(t, b, 30, 1, false), runtimeNote(t, b, 40, 1, true), runtimeNote(t, b, 50, 1, false), backup, key, runtimeNote(t, b, 80, 1, false))
	if err := service.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[NodeID]string{20: "0", 30: "0", 40: "0", 50: "1/3", 60: "2/3", 70: "0", 80: "0"} {
		fact := service.Facts[id]
		if fact.Position.State != EvidenceKnown || fact.Position.Value.Offset.String() != want {
			t.Errorf("node%d=%+v want%s", id, fact.Position, want)
		}
	}
	next := runtimeBlock(b, 1, runtimeNote(t, b, 90, 2, false), runtimeNote(t, b, 100, 1, false))
	if err := service.readMeasure(context.Background(), s, b, next); err != nil {
		t.Fatal(err)
	}
	if got := service.Facts[100].Position.Value.Offset.String(); got != "2/3" {
		t.Fatalf("inherited exactunits=%s", got)
	}
	keyResult, err := service.State.effectiveKey(context.Background(), s, service.Facts[50].Position.Value, stateKnown(stateInt(t, 1)))
	if err != nil || keyResult.Value.State != EvidenceKnown || keyResult.Value.Value.Fifths.Value.String() != "2" {
		t.Fatalf("score-order key=%+v,%v", keyResult, err)
	}
}
func TestTimelineRuntimeUnknownPositionOnlyGapsPublishedFamily(t *testing.T) {
	s, service, b := runtimeFixture(t)
	attr := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesKey, 21, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, 22, stateAtomicInteger(t, 0))))
	later := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesClef, 41, stateAbsent[Atomic](), stateNode(b, timelineRoleClefSign, 42, stateAtomicText("G"))))
	block := runtimeBlock(b, 0, attr, runtimeNote(t, b, 30, 1, false), later)
	if err := service.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	at := statePosition(t, 1, 0, 1)
	key, err := service.State.effectiveKey(context.Background(), s, at, stateKnown(stateInt(t, 1)))
	if err != nil || key.Value.State != EvidenceKnown {
		t.Fatalf("unrelated key lost=%+v,%v", key, err)
	}
	clef := stateQuery(t, s, &service.State, &service.State.Clefs, at, stateKnown(stateInt(t, 1)))
	if clef.Value.State != EvidenceUnknown || clef.Value.Reason != "initial-divisions-unavailable" {
		t.Fatalf("unknown placed clef fabricated absence=%+v", clef)
	}
}

func TestTimelineRuntimeDoesNotChooseDisputedUnits(t *testing.T) {
	s, service, b := runtimeFixture(t)
	d1 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	d2 := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 41, stateAtomicDecimal(t, 2, 1)))
	backup := stateNode(b, timelineRoleBackup, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 51, stateAtomicDecimal(t, 2, 1)))
	if err := service.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, d1, runtimeNote(t, b, 30, 1, false), d2, backup, runtimeNote(t, b, 60, 2, false))); err != nil {
		t.Fatal(err)
	}
	if got := service.Facts[60].Position; got.State == EvidenceKnown {
		t.Fatalf("unapproved lexical/onset/departure convention chose %s", got.Value.Offset)
	}
}
func TestTimelineRuntimeChordDoesNotBorrowLaterDivisions(t *testing.T) {
	s, service, b := runtimeFixture(t)
	d1 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	d2 := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 41, stateAtomicDecimal(t, 2, 1)))
	if err := service.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, d1, runtimeNote(t, b, 30, 1, false), d2, runtimeNote(t, b, 50, 2, true))); err != nil {
		t.Fatal(err)
	}
	if got := service.Facts[50].Duration; got.State == EvidenceKnown {
		t.Fatalf("chord chose unapproved normalization %s", got.Value)
	}
}

func TestTimelineRuntimeChangedUnitsWithinProvenRegion(t *testing.T) {
	s, service, b := runtimeFixture(t)
	d1 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	d2 := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 41, stateAtomicDecimal(t, 2, 1)))
	backup := stateNode(b, timelineRoleBackup, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 61, stateAtomicDecimal(t, 2, 1)))
	if err := service.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, d1, runtimeNote(t, b, 30, 4, false), d2, runtimeNote(t, b, 50, 4, false), backup, runtimeNote(t, b, 70, 2, false))); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[NodeID]string{30: "0", 40: "4", 50: "4", 60: "6", 70: "5"} {
		got := service.Facts[id].Position
		if got.State != EvidenceKnown || got.Value.Offset.String() != want {
			t.Errorf("node%d=%+v,want%s", id, got, want)
		}
	}
}
func TestTimelineRuntimeSamePositionDivisionsConflict(t *testing.T) {
	s, service, b := runtimeFixture(t)
	var events []*factNode
	for i, value := range []int64{1, 2, 1} {
		id := NodeID(20 + i*10)
		events = append(events, stateNode(b, timelineRoleAttributes, id, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, id+1, stateAtomicDecimal(t, value, 1))))
	}
	events = append(events, runtimeNote(t, b, 60, 2, false), runtimeNote(t, b, 70, 1, false))
	if err := service.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, events...)); err != nil {
		t.Fatal(err)
	}
	if got := service.Facts[60].Duration; got.State != EvidenceUnknown {
		t.Fatalf("sameposition conflict normalized=%+v", got)
	}
	if got := service.Facts[70].Position; got.State != EvidenceUnknown {
		t.Fatalf("sameposition conflict advanced=%+v", got)
	}
}

func TestTimelineRuntimeUnknownPublisherMasksInheritedState(t *testing.T) {
	s, service, b := runtimeFixture(t)
	attr := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)), stateNode(b, timelineRoleAttributesKey, 22, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, 23, stateAtomicInteger(t, 0))))
	unknown := stateNode(b, timelineRoleDirection, 30, stateAbsent[Atomic]())
	unknown.header.Declaration = Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "publisher-kind-unknown"}
	if err := service.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, attr, unknown)); err != nil {
		t.Fatal(err)
	}
	next := runtimeBlock(b, 1, runtimeNote(t, b, 40, 1, false))
	if err := service.readMeasure(context.Background(), s, b, next); err != nil {
		t.Fatal(err)
	}
	key, err := service.State.effectiveKey(context.Background(), s, statePosition(t, 1, 0, 1), stateKnown(stateInt(t, 1)))
	if err != nil || key.Value.State != EvidenceUnknown || key.Value.Reason != "publisher-kind-unknown" {
		t.Fatalf("hidden publisher inherited old key=%+v,%v", key, err)
	}
	if got := service.Facts[40].Duration; got.State != EvidenceUnknown {
		t.Fatalf("hidden publisher inherited old divisions=%+v", got)
	}
}
func TestTimelineRuntimeIncompleteContainerDoesNotInventStateAbsence(t *testing.T) {
	s, service, b := runtimeFixture(t)
	block := runtimeBlock(b, 0)
	block.complete = Evidence[bool]{State: EvidenceUnknown, Reason: "events-omitted"}
	if err := service.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	key, err := service.State.effectiveKey(context.Background(), s, statePosition(t, 1, 0, 1), stateKnown(stateInt(t, 1)))
	if err != nil || key.Value.State != EvidenceUnknown || key.Value.Reason != "events-omitted" {
		t.Fatalf("omitted state became absence=%+v,%v", key, err)
	}
}

func TestTimelineRuntimeBoundaryGapPreservesProvenPrefix(t *testing.T) {
	s, service, b := runtimeFixture(t)
	d1 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	d2 := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 41, stateAtomicDecimal(t, 2, 1)))
	backup := stateNode(b, timelineRoleBackup, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 51, stateAtomicDecimal(t, 2, 1)))
	if err := service.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, d1, runtimeNote(t, b, 30, 4, false), d2, backup, runtimeNote(t, b, 60, 1, false))); err != nil {
		t.Fatal(err)
	}
	if got := service.Facts[30].Duration; got.State != EvidenceKnown || got.Value.String() != "4" {
		t.Fatalf("proven prefix poisoned=%+v", got)
	}
	if got := service.Facts[40].Position; got.State != EvidenceKnown || got.Value.Offset.String() != "4" {
		t.Fatalf("change position poisoned=%+v", got)
	}
	if got := service.Facts[60].Position; got.State == EvidenceKnown {
		t.Fatalf("ambiguous suffix chosen=%+v", got)
	}
}

func TestTimelinePartClosureCarriesHierarchyAvailability(t *testing.T) {
	s, service, b := runtimeFixture(t)
	service.State.Closures = nil
	block := runtimeBlock(b, 0)
	block.coverage = Evidence[bool]{State: EvidenceUnknown, Reason: "measure-list-incomplete"}
	if _, err := service.prepareTimelineParts(context.Background(), s, []timelineMeasureBlock{block}); err != nil {
		t.Fatal(err)
	}
	key, err := service.State.effectiveKey(context.Background(), s, statePosition(t, 0, 0, 1), stateKnown(stateInt(t, 1)))
	if err != nil || key.Value.State != EvidenceUnknown || key.Value.Reason != "measure-list-incomplete" {
		t.Fatalf("incomplete hierarchy proved absence=%+v,%v", key, err)
	}
}
