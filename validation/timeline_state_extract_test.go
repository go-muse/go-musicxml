package validation

import (
	"context"
	"testing"
)

// Only immutable descriptor values are shared. Each fixture still owns its
// bindings and every mutable Roles slice, including callers' append capacity.
var stateBindingsFixtureUses = func() [timelineRoleLimit]SchemaUse {
	var uses [timelineRoleLimit]SchemaUse
	for role := timelineRoleInvalid + 1; role < timelineRoleLimit; role++ {
		uses[role] = SchemaUse{Declaration: DeclarationID{Occurrence: string(rune(1000 + role))}}
	}
	for _, role := range []timelineRole{timelineRoleKeyNumber, timelineRoleTimeNumber, timelineRoleClefNumber, timelineRoleTransposeNumber, timelineRoleForPartNumber, timelineRoleClefAdditional, timelineRoleClefSize, timelineRoleClefAfterBarline, timelineRoleDoubleAbove, timelineRoleKeyOctaveNumber, timelineRoleKeyOctaveCancel, timelineRoleCancelLocation, timelineRoleKeyAccidentalSMuFL, timelineRoleTimeSymbol, timelineRoleTimeSeparator, timelineRoleInterchangeableSymbol, timelineRoleInterchangeableSeparator, timelineRoleInstrumentID, timelineRoleInstrumentChangeID} {
		uses[role].Attribute = true
		uses[role].Name = QName{Local: string(rune(1000 + role))}
	}
	for _, shape := range timelineRoleShapes {
		if shape.attribute {
			uses[shape.role].Attribute = true
			uses[shape.role].Name = QName{Local: string(rune(1000 + shape.role))}
		}
	}
	return uses
}()

func stateBindings() *timelineBindings {
	fixture := &struct {
		bindings timelineBindings
		uses     [timelineRoleLimit]SchemaUse
	}{uses: stateBindingsFixtureUses}
	for role := timelineRoleInvalid + 1; role < timelineRoleLimit; role++ {
		fixture.bindings.Roles[role] = fixture.uses[role : role+1 : role+1]
	}
	return &fixture.bindings
}
func stateNode(b *timelineBindings, role timelineRole, id NodeID, scalar Evidence[Atomic], children ...*factNode) *factNode {
	n := &factNode{header: testRetainedHeader(NodeHeader{ID: id, Declaration: stateKnown(b.Roles[role][0].Declaration)}), closed: true, children: children, end: testRetainedEnd(NodeEnd{ID: id, Scalar: Scalar{Value: scalar}, ChildrenComplete: stateKnown(true), AttributesComplete: stateKnown(true), Order: stateKnown(OrderSource)})}
	for _, c := range children {
		c.header.Parent = id
	}
	return n
}
func stateAttribute(b *timelineBindings, n *factNode, role timelineRole, value Atomic) {
	u := b.Roles[role][0]
	n.attributes = append(n.attributes, AttributeFact{Owner: n.header.ID, Name: u.Name, Declaration: stateKnown(u.Declaration), Presence: stateKnown(true), Scalar: Scalar{Value: stateKnown(value)}})
}
func stateAtomicInteger(t *testing.T, n int64) Evidence[Atomic] {
	return stateKnown(Atomic{Kind: AtomicInteger, Integer: stateInt(t, n)})
}
func stateAtomicDecimal(t *testing.T, n, d int64) Evidence[Atomic] {
	return stateKnown(Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, n, d)})
}
func stateAtomicText(v string) Evidence[Atomic] {
	return stateKnown(Atomic{Kind: AtomicString, Text: v})
}

func TestTimelineStateExtractTypedAttributesAtKnownPosition(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 1)
	b := stateBindings()
	empty := stateAbsent[Atomic]()
	key := stateNode(b, timelineRoleAttributesKey, 21, empty, stateNode(b, timelineRoleKeyStep, 22, stateAtomicText("F")), stateNode(b, timelineRoleKeyAlter, 23, stateAtomicDecimal(t, 1, 2)), stateNode(b, timelineRoleKeyAccidental, 24, stateAtomicText("quarter-sharp")))
	time := stateNode(b, timelineRoleAttributesTime, 31, empty, stateNode(b, timelineRoleTimeBeats, 32, stateAtomicText("2")), stateNode(b, timelineRoleTimeBeatType, 33, stateAtomicText("4")), stateNode(b, timelineRoleTimeBeats, 34, stateAtomicText("3+2")), stateNode(b, timelineRoleTimeBeatType, 35, stateAtomicText("8")), stateNode(b, timelineRoleTimeInterchangeable, 36, empty, stateNode(b, timelineRoleInterchangeableBeats, 37, stateAtomicText("6")), stateNode(b, timelineRoleInterchangeableBeatType, 38, stateAtomicText("8"))))
	clef := stateNode(b, timelineRoleAttributesClef, 41, empty, stateNode(b, timelineRoleClefSign, 42, stateAtomicText("G")), stateNode(b, timelineRoleClefLine, 43, stateAtomicInteger(t, 7)))
	extra := stateNode(b, timelineRoleAttributesClef, 44, empty, stateNode(b, timelineRoleClefSign, 45, stateAtomicText("F")), stateNode(b, timelineRoleClefLine, 46, stateAtomicInteger(t, 4)))
	stateAttribute(b, extra, timelineRoleClefAdditional, Atomic{Kind: AtomicString, Text: "yes"})
	trans := stateNode(b, timelineRoleAttributesTranspose, 51, empty, stateNode(b, timelineRoleTransposeChromatic, 52, stateAtomicDecimal(t, -2, 1)), stateNode(b, timelineRoleTransposeDiatonic, 53, stateAtomicInteger(t, -1)), stateNode(b, timelineRoleTransposeOctaveChange, 54, stateAtomicInteger(t, -1)), stateNode(b, timelineRoleTransposeDouble, 55, empty))
	created := stateNode(b, timelineRoleAttributesForPart, 61, empty, stateNode(b, timelineRoleForPartPartTranspose, 62, empty, stateNode(b, timelineRolePartTransposeChromatic, 63, stateAtomicDecimal(t, 2, 1))))
	attrs := stateNode(b, timelineRoleAttributes, 20, empty, stateNode(b, timelineRoleAttributesDivisions, 25, stateAtomicDecimal(t, 3, 1)), key, time, stateNode(b, timelineRoleAttributesStaves, 26, stateAtomicInteger(t, 2)), stateNode(b, timelineRoleAttributesInstruments, 27, stateAtomicInteger(t, 0)), clef, extra, trans, created)
	if e := h.readAttributes(context.Background(), s, b, attrs, statePosition(t, 0, 0, 1)); e != nil {
		t.Fatal(e)
	}
	at := statePosition(t, 1, 0, 1)
	staff := stateKnown(stateInt(t, 1))
	kr := stateQuery(t, s, h, &h.Keys, at, staff)
	if kr.Value.State != EvidenceKnown || kr.Value.Value.Traditional || len(kr.Value.Value.Alterations) != 1 || kr.Value.Value.Alterations[0].Alter.Value.String() != "0.5" {
		t.Fatalf("typed key %+v", kr)
	}
	tr := stateQuery(t, s, h, &h.Times, at, staff)
	if tr.Value.State != EvidenceKnown || len(tr.Value.Value.Pairs) != 2 || tr.Value.Value.Pairs[1].Beats.Value != "3+2" || tr.Value.Value.Interchangeable.Value.Pairs[0].Beats.Value != "6" {
		t.Fatalf("typed time %+v", tr)
	}
	cr := stateQuery(t, s, h, &h.Clefs, at, staff)
	if cr.Value.Value.Line.State != EvidenceKnown || cr.Value.Value.Line.Value.String() != "7" {
		t.Fatalf("outside staff line %+v", cr)
	}
	ar := stateQuery(t, s, h, &h.AdditionalClefs, at, staff)
	if ar.Value.Value.activeLine().State != EvidenceAbsent {
		t.Fatalf("additional active %+v", ar)
	}
	transpose := stateQuery(t, s, h, &h.Transpose, at, staff)
	if transpose.Value.Value.Chromatic.Value.String() != "-2" || transpose.Value.Value.Double.Value.Above.State != EvidenceAbsent {
		t.Fatalf("transpose %+v", transpose)
	}
	part := stateQuery(t, s, h, &h.CreatedPartTranspose, at, staff)
	if part.Value.Value.Chromatic.Value.String() != "2" {
		t.Fatalf("created part %+v", part)
	}
	count, e := h.effectiveInstruments(context.Background(), s, at)
	if e != nil || count.Value.Value.String() != "0" {
		t.Fatalf("zero instruments %+v,%v", count, e)
	}
}
func TestTimelineStateExtractNoNameFallbackAndInvalidComponent(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	b := stateBindings()
	empty := stateAbsent[Atomic]()
	cause := &DecisionRef{Rule: RuleKey{ID: "scalar", Revision: 1}, Subject: NodeRef{DocumentID: "d", Node: 42}}
	bad := stateNode(b, timelineRoleAttributesKey, 40, empty, stateNode(b, timelineRoleKeyFifths, 41, stateAtomicInteger(t, 0)), stateNode(b, timelineRoleKeyMode, 42, Evidence[Atomic]{State: EvidenceInvalid, Reason: "mode-invalid", Cause: cause}))
	fake := stateNode(b, timelineRoleNote, 50, empty)
	fake.header.Name = QName{Local: "key"}
	attrs := stateNode(b, timelineRoleAttributes, 20, empty, bad, fake)
	if e := h.readAttributes(context.Background(), s, b, attrs, statePosition(t, 0, 0, 1)); e != nil {
		t.Fatal(e)
	}
	if len(h.Keys.Publications) != 1 {
		t.Fatal("matched local name")
	}
	got := stateQuery(t, s, h, &h.Keys, statePosition(t, 0, 0, 1), stateKnown(stateInt(t, 1)))
	if got.Value.State != EvidenceKnown || got.Value.Value.Fifths.State != EvidenceKnown || got.Value.Value.Mode.State != EvidenceInvalid || *got.Value.Value.Mode.Cause != *cause {
		t.Fatalf("component evidence lost %+v", got)
	}
}
func TestTimelineStateInstrumentIdentitySetsAndContinuingChanges(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 2)
	part := statePosition(t, 0, 0, 1).Part
	i1 := NodeRef{DocumentID: "d", Node: 71}
	i2 := NodeRef{DocumentID: "d", Node: 72}
	for _, id := range []NodeRef{i1, i2} {
		if e := h.addInstrumentDefinition(context.Background(), s, instrumentDefinition{Part: part, Instrument: id, Value: stateKnown(instrumentState{Sound: stateKnown("initial")})}); e != nil {
			t.Fatal(e)
		}
	}
	refs := []Evidence[NodeRef]{stateKnown(i1), stateKnown(i2)}
	if e := h.addNoteInstruments(context.Background(), s, noteInstrumentSet{Part: part, Note: NodeRef{DocumentID: "d", Node: 80}, References: refs, Complete: stateKnown(true)}); e != nil {
		t.Fatal(e)
	}
	refs[0].Value = i2
	if e := h.addNoteInstruments(context.Background(), s, noteInstrumentSet{Part: part, Note: NodeRef{DocumentID: "d", Node: 81}, Complete: stateKnown(true)}); e != nil {
		t.Fatal(e)
	}
	if len(h.NoteInstruments[0].References) != 2 || h.NoteInstruments[0].References[0].Value != i1 || len(h.NoteInstruments[1].References) != 0 {
		t.Fatal("note selection inheritance or alias")
	}
	change := statePublication[instrumentState]{Position: statePosition(t, 0, 1, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(instrumentState{Sound: stateKnown("changed"), Virtual: stateAbsent[virtualInstrumentState]()})}
	if e := h.addInstrumentChange(context.Background(), s, i2, change); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		id   NodeRef
		want string
	}{{i1, "initial"}, {i2, "changed"}} {
		r, e := h.effectiveInstrument(context.Background(), s, statePosition(t, 2, 0, 1), tc.id)
		if e != nil || r.Value.State != EvidenceKnown || r.Value.Value.Sound.Value != tc.want {
			t.Fatalf("instrument bleed %+v,%v", r, e)
		}
	}
	// Omitted top-level fields make no new encoded statement. With no initial
	// descriptor, absence remains absence; no playback default is invented.
	r, e := h.effectiveInstrument(context.Background(), s, statePosition(t, 2, 0, 1), i2)
	if e != nil || r.Value.Value.Virtual.State != EvidenceAbsent {
		t.Fatalf("invented property reset %+v,%v", r, e)
	}
}

func TestTimelineStateUnplacedAttributesGapOnlyAffectedFamilies(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 2)
	b := stateBindings()
	empty := stateAbsent[Atomic]()
	all := stateScope{AllStaves: true}
	part := statePosition(t, 0, 0, 1).Part
	staff := stateKnown(stateInt(t, 1))
	statePublish(t, s, &h.Divisions, statePosition(t, 0, 0, 1), all, stateKnown(stateDecimal(t, 4, 1)), 1)
	statePublish(t, s, &h.Keys, statePosition(t, 0, 0, 1), all, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 0))}), 2)
	attrs := stateNode(b, timelineRoleAttributes, 10, empty, stateNode(b, timelineRoleAttributesKey, 11, empty, stateNode(b, timelineRoleKeyFifths, 12, stateAtomicInteger(t, 2))))
	cause := &DecisionRef{Rule: RuleKey{ID: "duration", Revision: 1}, Subject: NodeRef{DocumentID: "d", Node: 9}}
	unavailable := Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "duration-invalid", Cause: cause}
	if e := h.gapAttributes(context.Background(), s, b, attrs, part, 1, unavailable); e != nil {
		t.Fatal(e)
	}
	key := stateQuery(t, s, h, &h.Keys, statePosition(t, 2, 0, 1), staff)
	if key.Value.State != EvidenceInvalid || *key.Value.Cause != *cause {
		t.Fatalf("missing causal key gap %+v", key)
	}
	divisions := stateQuery(t, s, h, &h.Divisions, statePosition(t, 2, 0, 1), staff)
	if divisions.Value.State != EvidenceKnown || divisions.Value.Value.String() != "4" || len(h.Divisions.Gaps) != 0 {
		t.Fatalf("unrelated divisions gap %+v", divisions)
	}
}
func TestTimelineStateUnresolvedInstrumentChangeMasksOlderChange(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 3)
	id := NodeRef{DocumentID: "d", Node: 20}
	p := statePublication[instrumentState]{Position: statePosition(t, 0, 0, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(instrumentState{Sound: stateKnown("old")})}
	if e := h.addInstrumentChange(context.Background(), s, id, p); e != nil {
		t.Fatal(e)
	}
	cause := &DecisionRef{Rule: RuleKey{ID: "identity", Revision: 1}, Subject: NodeRef{DocumentID: "d", Node: 21}}
	h.InstrumentGaps = []stateGap{{Part: p.Position.Part, FirstMeasure: 1, LastMeasure: 1, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceInvalid, Reason: "unknown-instrument-id", Cause: cause}}}
	r, e := h.effectiveInstrument(context.Background(), s, statePosition(t, 2, 0, 1), id)
	if e != nil || r.Value.State != EvidenceInvalid || r.Value.Cause == nil || *r.Value.Cause != *cause {
		t.Fatalf("instrument gap bypass %+v,%v", r, e)
	}
	p.Position = statePosition(t, 2, 0, 1)
	p.Value.Value.Sound = stateKnown("replacement")
	if e := h.addInstrumentChange(context.Background(), s, id, p); e != nil {
		t.Fatal(e)
	}
	r, e = h.effectiveInstrument(context.Background(), s, statePosition(t, 3, 0, 1), id)
	if e != nil || r.Value.State != EvidenceKnown || r.Value.Value.Sound.Value != "replacement" {
		t.Fatalf("instrument recovery %+v,%v", r, e)
	}
}

func TestTimelineStateModelFieldsAreStructuralNotChronology(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	b := stateBindings()
	key := stateNode(b, timelineRoleAttributesKey, 21, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, 22, stateAtomicInteger(t, 0)))
	key.end.Order = testEvidence(stateKnown(OrderModelFields))
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), key)
	attrs.end.Order = testEvidence(stateKnown(OrderModelFields))
	if e := h.readAttributes(context.Background(), s, b, attrs, statePosition(t, 0, 0, 1)); e != nil {
		t.Fatal(e)
	}
	r := stateQuery(t, s, h, &h.Keys, statePosition(t, 0, 0, 1), stateKnown(stateInt(t, 1)))
	if r.Value.State != EvidenceKnown || r.Value.Value.Fifths.Value.String() != "0" {
		t.Fatalf("model structural fields rejected %+v", r)
	}
	reader := &stateReader{ctx: context.Background(), s: s, b: b}
	before := rationalUsage(t, s, BudgetBufferedBytes)
	for range 100 {
		reader.child(key, timelineRoleKeyMode)
	}
	if reader.err != nil {
		t.Fatal(reader.err)
	}
	if after := rationalUsage(t, s, BudgetBufferedBytes); after != before {
		t.Fatalf("singleton field scans retained %d bytes", after-before)
	}
}

func TestTimelineStateUnknownPublisherGapsAllButKnownTypeNarrows(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 2)
	b := stateBindings()
	part := statePosition(t, 0, 0, 1).Part
	availability := stateUnknown[musicalPosition]("unobserved-event")
	if e := h.gapAll(context.Background(), s, part, 1, availability); e != nil {
		t.Fatal(e)
	}
	if len(h.Keys.Gaps) != 1 || len(h.Divisions.Gaps) != 1 || len(h.InstrumentGaps) != 1 {
		t.Fatal("unknown event omitted possible state families")
	}
	isolated := &timelineState{}
	stateClose(t, s, isolated, 2)
	keyType := ComponentID{Kind: ComponentType, Name: QName{Local: "exact-key-type"}}
	b.Roles[timelineRoleAttributesKey][0].DeclaredTypeKnown = true
	b.Roles[timelineRoleAttributesKey][0].DeclaredType = keyType
	for _, role := range []timelineRole{timelineRoleAttributesDivisions, timelineRoleAttributesTime, timelineRoleAttributesStaves, timelineRoleAttributesInstruments, timelineRoleAttributesClef, timelineRoleAttributesTranspose, timelineRoleAttributesForPart} {
		b.Roles[role][0].DeclaredTypeKnown = true
		b.Roles[role][0].DeclaredType = ComponentID{Kind: ComponentType, Name: QName{Local: "other-type"}}
	}
	child := stateNode(b, timelineRoleAttributesKey, 31, stateAbsent[Atomic]())
	child.header.Declaration = testEvidence(Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "unbound-key-use"})
	child.header.Type = testEvidence(stateKnown(keyType))
	attrs := stateNode(b, timelineRoleAttributes, 30, stateAbsent[Atomic](), child)
	if e := isolated.readAttributes(context.Background(), s, b, attrs, statePosition(t, 0, 0, 1)); e != nil {
		t.Fatal(e)
	}
	if len(isolated.Keys.Publications) != 1 || len(isolated.Divisions.Publications) != 0 || len(isolated.Times.Publications) != 0 {
		t.Fatal("known exact key type poisoned other families")
	}
}

func TestTimelineStateRawInstrumentFieldsRemainTyped(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	b := stateBindings()
	empty := stateAbsent[Atomic]()
	r := &stateReader{ctx: context.Background(), s: s, b: b}
	for _, role := range []timelineRole{timelineRoleOtherPlayType, timelineRoleMidiDevicePort, timelineRoleMidiDeviceID, timelineRoleMidiInstrumentID, timelineRolePlayID} {
		b.Roles[role][0].Attribute = true
		b.Roles[role][0].Name = QName{Local: string(rune(1000 + role))}
	}
	other := stateNode(b, timelineRolePlayOtherPlay, 81, stateAtomicText("bow-on-bridge"))
	stateAttribute(b, other, timelineRoleOtherPlayType, Atomic{Kind: AtomicString, Text: "bow-position"})
	play := stateNode(b, timelineRoleSoundPlay, 80, empty, stateNode(b, timelineRolePlayIpa, 82, stateAtomicText("a")), other)
	got := r.play(play)
	if r.err != nil || got.State != EvidenceKnown || len(got.Value.Techniques) != 2 || got.Value.Techniques[1].Kind != playOther || got.Value.Techniques[1].Type.Value != "bow-position" {
		t.Fatalf("raw play %+v %v", got, r.err)
	}
	midi := stateNode(b, timelineRoleSoundMidiInstrument, 90, empty, stateNode(b, timelineRoleMidiInstrumentMidiProgram, 91, stateAtomicInteger(t, 71)), stateNode(b, timelineRoleMidiInstrumentVolume, 92, stateAtomicDecimal(t, 125, 2)))
	m := r.midi(midi)
	if r.err != nil || m.Value.Program.Value.String() != "71" || m.Value.Volume.Value.String() != "62.5" || m.Value.Channel.State != EvidenceAbsent {
		t.Fatalf("raw midi %+v %v", m, r.err)
	}
	h := &timelineState{}
	score := stateNode(b, timelineRoleScorePart, 1, empty, stateNode(b, timelineRoleScoreInstrument, 20, empty, stateNode(b, timelineRoleScoreInstrumentInstrumentName, 21, stateAtomicText("Oboe")), stateNode(b, timelineRoleScoreInstrumentInstrumentAbbreviation, 22, stateAtomicText("Ob."))))
	if e := h.readInstrumentDefinitions(context.Background(), s, b, score, NodeRef{DocumentID: "d", Node: 1}); e != nil {
		t.Fatal(e)
	}
	if len(h.Definitions) != 1 || h.Definitions[0].Name.Value != "Oboe" || h.Definitions[0].Abbreviation.Value != "Ob." {
		t.Fatalf("declaration data lost %+v", h.Definitions)
	}
}
func TestTimelineStateUnplacedSoundWithoutChangesDoesNotMaskVirtualState(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	b := stateBindings()
	h := &timelineState{}
	sound := stateNode(b, timelineRoleSound, 20, stateAbsent[Atomic]())
	part := statePosition(t, 0, 0, 1).Part
	if e := h.gapInstrumentChanges(context.Background(), s, b, sound, part, 1, stateUnknown[musicalPosition]("sound-position-unavailable")); e != nil {
		t.Fatal(e)
	}
	if len(h.InstrumentGaps) != 0 || len(h.InstrumentChanges) != 0 {
		t.Fatal("sound without virtual publisher erased instruments")
	}
}

func TestTimelineStateInstrumentGapsPreserveStrongestCause(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	b := stateBindings()
	part := statePosition(t, 0, 0, 1).Part
	cause := &DecisionRef{Rule: RuleKey{ID: "duration", Revision: 1}, Subject: NodeRef{DocumentID: "d", Node: 4}}
	position := Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "duration-invalid", Cause: cause}
	for _, incomplete := range []bool{false, true} {
		h := &timelineState{}
		sound := stateNode(b, timelineRoleSound, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundInstrumentChange, 21, stateAbsent[Atomic](), stateNode(b, timelineRoleInstrumentChangeInstrumentSound, 22, stateAtomicText("stated"))))
		if incomplete {
			sound.end.ChildrenComplete = testEvidence(stateUnknown[bool]("children-unavailable"))
		}
		if e := h.gapInstrumentChanges(context.Background(), s, b, sound, part, 1, position); e != nil {
			t.Fatal(e)
		}
		gaps := h.InstrumentGaps
		if !incomplete {
			gaps = h.InstrumentPropertyGaps[0]
			if len(h.InstrumentGaps) != 0 || len(h.InstrumentPropertyGaps[1]) != 0 || len(h.InstrumentPropertyGaps[2]) != 0 {
				t.Fatal("proved absent properties were gapped")
			}
		}
		if len(gaps) != 1 || gaps[0].Availability.State != EvidenceInvalid || gaps[0].Availability.Cause == nil || *gaps[0].Availability.Cause != *cause {
			t.Fatalf("stronger position cause hidden %+v", gaps)
		}
	}
	h := &timelineState{}
	sound := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundInstrumentChange, 31, stateAbsent[Atomic]()))
	if e := h.readInstrumentChanges(context.Background(), s, b, sound, statePosition(t, 0, 0, 1)); e != nil {
		t.Fatal(e)
	}
	if len(h.UnresolvedInstrumentChanges) != 1 || h.UnresolvedInstrumentChanges[0].Instrument.State != EvidenceUnknown || h.UnresolvedInstrumentChanges[0].Instrument.Reason != "timeline-required-instrument-id-absent" {
		t.Fatalf("missing required ID became absence %+v", h.UnresolvedInstrumentChanges)
	}
	if len(h.InstrumentGaps) != 0 || len(h.InstrumentPropertyGaps[0]) != 0 || len(h.InstrumentPropertyGaps[1]) != 0 || len(h.InstrumentPropertyGaps[2]) != 0 {
		t.Fatal("empty bad-target record published a property update")
	}
}
