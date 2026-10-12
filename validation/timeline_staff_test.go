package validation

import (
	"context"
	"testing"
)

func TestTimelineStateStaffDetailsTypedSparseAndInherited(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 2)
	b := stateBindings()
	empty := stateAbsent[Atomic]()
	for _, role := range []timelineRole{timelineRoleStaffDetailsNumber, timelineRoleStaffTuningLine, timelineRoleLineDetailLine, timelineRoleStaffSizeScaling} {
		b.Roles[role][0].Attribute = true
		b.Roles[role][0].Name = QName{Local: string(rune(1000 + role))}
	}
	tuning := stateNode(b, timelineRoleStaffDetailsStaffTuning, 34, empty, stateNode(b, timelineRoleStaffTuningTuningStep, 35, stateAtomicText("E")), stateNode(b, timelineRoleStaffTuningTuningAlter, 36, stateAtomicDecimal(t, -1, 2)), stateNode(b, timelineRoleStaffTuningTuningOctave, 37, stateAtomicInteger(t, 4)))
	stateAttribute(b, tuning, timelineRoleStaffTuningLine, Atomic{Kind: AtomicInteger, Integer: stateInt(t, 2)})
	line := stateNode(b, timelineRoleStaffDetailsLineDetail, 38, empty)
	stateAttribute(b, line, timelineRoleLineDetailLine, Atomic{Kind: AtomicInteger, Integer: stateInt(t, 2)})
	stateAttribute(b, line, timelineRoleLineDetailWidth, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 1, 2)})
	stateAttribute(b, line, timelineRoleLineDetailColor, Atomic{Kind: AtomicString, Text: "#123456"})
	stateAttribute(b, line, timelineRoleLineDetailLineType, Atomic{Kind: AtomicString, Text: "dashed"})
	stateAttribute(b, line, timelineRoleLineDetailPrintObject, Atomic{Kind: AtomicString, Text: "no"})
	size := stateNode(b, timelineRoleStaffDetailsStaffSize, 39, stateAtomicDecimal(t, 80, 1))
	stateAttribute(b, size, timelineRoleStaffSizeScaling, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60, 1)})
	details := stateNode(b, timelineRoleAttributesStaffDetails, 30, empty, stateNode(b, timelineRoleStaffDetailsStaffType, 31, stateAtomicText("ossia")), stateNode(b, timelineRoleStaffDetailsStaffLines, 32, stateAtomicInteger(t, 0)), line, tuning, stateNode(b, timelineRoleStaffDetailsCapo, 33, stateAtomicInteger(t, 2)), size)
	attrs := stateNode(b, timelineRoleAttributes, 20, empty, details)
	if e := h.readAttributes(context.Background(), s, b, attrs, statePosition(t, 0, 0, 1)); e != nil {
		t.Fatal(e)
	}
	r, e := h.effectiveStaffDetails(context.Background(), s, statePosition(t, 1, 0, 1), stateKnown(stateInt(t, 1)))
	if e != nil || r.Value.State != EvidenceKnown || !r.Inherited {
		t.Fatalf("staff details %+v,%v", r, e)
	}
	v := r.Value.Value
	if v.Type.Value != "ossia" || v.Lines.Value.String() != "0" || v.Tunings[0].Alter.Value.String() != "-0.5" || v.Tunings[0].Line.Value.String() != "2" || v.Capo.Value.String() != "2" || v.Size.Value.Scaling.Value.String() != "60" || v.LineDetails[0].Width.Value.String() != "0.5" || v.LineDetails[0].Color.Value != "#123456" || v.LineDetails[0].LineType.Value != "dashed" || v.LineDetails[0].PrintObject.Value {
		t.Fatalf("lost components %+v", v)
	}
	other, e := h.effectiveStaffDetails(context.Background(), s, statePosition(t, 1, 0, 1), stateKnown(stateInt(t, 2)))
	if e != nil || other.Value.State != EvidenceAbsent {
		t.Fatalf("staff details did not default to staff1 %+v,%v", other, e)
	}
	if e := h.gapAttributes(context.Background(), s, b, attrs, statePosition(t, 0, 0, 1).Part, 1, stateUnknown[musicalPosition]("unplaced-staff-details")); e != nil {
		t.Fatal(e)
	}
	r, e = h.effectiveStaffDetails(context.Background(), s, statePosition(t, 2, 0, 1), stateKnown(stateInt(t, 1)))
	if e != nil || r.Value.State != EvidenceUnknown || r.Value.Reason != "unplaced-staff-details" {
		t.Fatalf("staff details gap %+v,%v", r, e)
	}
}
func TestTimelineStateDocumentedDefaultsRequireKnownAbsence(t *testing.T) {
	v := timeState{Symbol: stateAbsent[string](), Separator: stateAbsent[string](), SenzaMisura: stateAbsent[string]()}
	symbol, sep := v.displaySymbol(), v.displaySeparator()
	if symbol.Raw.State != EvidenceAbsent || symbol.Effective.State != EvidenceKnown || symbol.Effective.Value != "normal" || symbol.Provenance != stateDocumentedDefault || symbol.Clause == "" || sep.Effective.Value != "none" {
		t.Fatalf("defaults %+v,%+v", symbol, sep)
	}
	v.Symbol = stateUnknown[string]("symbol-unavailable")
	if got := v.displaySymbol(); got.Effective.State != EvidenceUnknown || got.Provenance == stateDocumentedDefault {
		t.Fatalf("unknown defaulted %+v", got)
	}
	v.Symbol = stateKnown("cut")
	v.SenzaMisura = stateKnown("X")
	if got := v.displaySymbol(); got.Effective.State != EvidenceAbsent || got.Raw.Value != "cut" {
		t.Fatalf("senza symbol active %+v", got)
	}
	interchangeable := interchangeableState{Symbol: stateAbsent[string](), Separator: stateAbsent[string]()}
	if interchangeable.displaySymbol().Effective.Value != "normal" || interchangeable.displaySeparator().Effective.Value != "none" {
		t.Fatal("dual time defaults lost")
	}
	doubled := doublingState{Above: stateAbsent[bool]()}
	got := doubled.effectiveAbove()
	if got.Raw.State != EvidenceAbsent || got.Effective.State != EvidenceKnown || got.Effective.Value || got.Provenance != stateDocumentedDefault {
		t.Fatalf("omitted doubling direction %+v", got)
	}
	raw := staffDetailsState{Type: stateAbsent[string](), Lines: stateAbsent[Integer]()}
	if raw.effectiveType().Effective.Value != "regular" || raw.Lines.State != EvidenceAbsent {
		t.Fatal("invented staff lines or lost regular type default")
	}
}
func TestTimelineStatePriorKeyIsStrictlyEarlierWithConflict(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 1)
	all := stateScope{AllStaves: true}
	staff := stateKnown(stateInt(t, 1))
	key := func(n int64) Evidence[keyState] {
		return stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, n))})
	}
	statePublish(t, s, &h.Keys, statePosition(t, 0, 0, 1), all, key(0), 1)
	statePublish(t, s, &h.Keys, statePosition(t, 0, 1, 3), all, key(2), 2)
	prior, e := h.priorKey(context.Background(), s, statePosition(t, 0, 2, 6), staff)
	if e != nil || prior.Value.State != EvidenceKnown || prior.Value.Value.Fifths.Value.String() != "0" {
		t.Fatalf("same-position key included %+v,%v", prior, e)
	}
	statePublish(t, s, &h.Keys, statePosition(t, 0, 0, 1), all, key(3), 3)
	prior, e = h.priorKey(context.Background(), s, statePosition(t, 0, 1, 3), staff)
	if e != nil || prior.Value.State != EvidenceUnknown || prior.Value.Reason != "timeline-state-conflict" {
		t.Fatalf("prior conflict flattened %+v,%v", prior, e)
	}
	initial, e := h.priorKey(context.Background(), s, statePosition(t, 0, 0, 1), staff)
	if e != nil || initial.Value.State != EvidenceAbsent {
		t.Fatalf("prior initial invented %+v,%v", initial, e)
	}
}
