package validation

import (
	"context"
	"errors"
	"math"
	"testing"
	"unsafe"
)

// Independent admitted facts isolate the new theorem from the positive
// representation, parser, and timeline inference. No positive.available is set.
func singletonRepeatFixture(t *testing.T, limits Budgets) (*contextReader, *timelineService, []contextRepeatRecord, *contextRepeatProofIndex, *timelinePlan) {
	t.Helper()
	s := rationalSession(t, limits)
	s.documentID = "d"
	s.nodes = map[NodeID]*factNode{}
	b := &contextBindings{Roles: map[contextRole][]SchemaUse{}, Attributes: map[contextRole]map[ComponentID]AttributeSelector{}, AttributeUses: map[contextRole]map[DeclarationID]AttributeSelector{}}
	for _, spec := range contextRepeatBindingSpecs() {
		u := SchemaUse{Name: QName{Local: spec.Name}, Declaration: localDecl(string(spec.Role)), OwnerType: contextNamedType(spec.Owner), OwnerTypeKnown: true, DeclaredType: contextNamedType(spec.Type), DeclaredTypeKnown: true, Attribute: spec.Attribute}
		b.Roles[spec.Role] = []SchemaUse{u}
		if spec.Attribute {
			a := AttributeSelector{Name: u.Name, Declaration: u.Declaration}
			b.Attributes[spec.Role] = map[ComponentID]AttributeSelector{u.OwnerType: a}
			b.AttributeUses[spec.Role] = map[DeclarationID]AttributeSelector{u.Declaration: a}
		}
	}
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	part := NodeRef{"d", 100}
	instrument := NodeRef{"d", 101}
	timeline := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	var ids []NodeID
	for i, step := range []string{"C", "D"} {
		n := spanUnitNode(r, contextRepeatAnyNote, nil)
		ids = append(ids, n.header.ID)
		pitch := spanUnitNode(r, contextRepeatNotePitch, n)
		for _, v := range []struct {
			role  contextRole
			value Atomic
		}{{contextRepeatPitchStep, localString(step)}, {contextRepeatPitchAlter, Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "0")}}, {contextRepeatPitchOctave, localInteger(t, 4)}} {
			scalar := spanUnitNode(r, v.role, pitch)
			scalar.end.Scalar.Value = testEvidence(stateKnown(v.value))
		}
		duration := spanUnitNode(r, contextRepeatNoteDuration, n)
		duration.end.Scalar.Value = testEvidence(stateAtomicDecimal(t, 1, 1))
		in := spanUnitNode(r, contextRepeatNoteInstrument, n)
		spanUnitAttribute(r, in, contextRepeatInstrumentID, stateAtomicText("I1"))
		voice := spanUnitNode(r, contextRepeatNoteVoice, n)
		voice.end.Scalar.Value = testEvidence(stateAtomicText("1"))
		staff := spanUnitNode(r, contextRepeatNoteStaff, n)
		staff.end.Scalar.Value = testEvidence(stateAtomicInteger(t, 1))
		pos, end := statePosition(t, 0, int64(i), 1), statePosition(t, 0, int64(i+1), 1)
		pos.Part = part
		end.Part = part
		timeline.Facts[n.header.ID] = timelineEventFact{Part: stateKnown(part), Position: stateKnown(pos), Origin: stateKnown(pos), End: stateKnown(end), Duration: stateKnown(mustRational(t, 1, 1)), Staff: stateKnown(stateOne()), Voice: stateKnown("1")}
		timeline.State.NoteInstruments = append(timeline.State.NoteInstruments, noteInstrumentSet{Note: NodeRef{"d", n.header.ID}, Part: part, Complete: stateKnown(true), References: []Evidence[NodeRef]{stateKnown(instrument)}})
	}
	origin := statePosition(t, 0, 0, 1)
	origin.Part = part
	timeline.State.Transpose.Publications = []statePublication[transposeState]{{Position: origin, Scope: stateScope{Staff: stateOne()}, Value: stateKnown(transposeState{Chromatic: stateKnown(localDecimal(t, "0")), OctaveChange: stateAbsent[Integer](), Double: stateAbsent[doublingState]()})}}
	start, end := statePosition(t, 0, 1, 1), statePosition(t, 0, 2, 1)
	start.Part = part
	end.Part = part
	rows := []contextRepeatRecord{{source: NodeRef{"d", 1000}, position: stateKnown(start), value: RepeatContextResult{Kind: RepeatStyleBeat, Type: stateKnown("start"), Part: stateKnown(part), AllStaves: stateKnown(false), Staff: stateKnown(stateOne()), Beat: stateKnown(mustRational(t, 1, 1)), NoteScopesComplete: stateKnown(true), Complete: stateKnown(true), Stop: stateKnown(NodeRef{"d", 1001}), ContentEquivalent: stateUnknown[bool]("positive-proof-unavailable")}}, {source: NodeRef{"d", 1001}, position: stateKnown(end)}}
	index := &contextRepeatProofIndex{events: map[NodeRef][]NodeID{part: ids}, rows: map[NodeRef]int{rows[1].source: 1}}
	return r, timeline, rows, index, &timelinePlan{Bindings: stateBindings()}
}
func TestIndependentSingletonRepeatResourceBoundary(t *testing.T) {
	run := func(ctx context.Context, limits Budgets) (int64, Evidence[bool], error) {
		r, timeline, rows, index, plan := singletonRepeatFixture(t, limits)
		r.ctx = ctx
		err := contextRepeatProveSingletons(r, timeline, rows, index, plan)
		steps := rationalUsage(t, r.s, BudgetRuleSteps)
		result := rows[0].value.ContentEquivalent
		if rationalUsage(t, r.s, BudgetBufferedBytes) != r.s.retained {
			t.Fatal("singleton scratch leaked")
		}
		r.s.cleanup()
		if rationalUsage(t, r.s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
		return steps, result, err
	}
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	steps, result, err := run(counter, Budgets{})
	if err != nil || result.State != EvidenceKnown || result.Value {
		t.Fatalf("proof %+v %v", result, err)
	}
	for _, delta := range []int64{0, -1} {
		_, result, err = run(context.Background(), Budgets{MaxRuleSteps: steps + delta})
		if delta == 0 && (err != nil || result.State != EvidenceKnown) || delta < 0 && (!errors.Is(err, ErrResourceLimit) || result.State == EvidenceKnown) {
			t.Fatalf("work delta=%d result=%+v err=%v", delta, result, err)
		}
	}
	// Find the exact live-storage boundary independently of the implementation's
	// reserve formula, then exercise the boundary and its immediate predecessor.
	low, high := int64(1), int64(1<<20)
	for low < high {
		mid := low + (high-low)/2
		_, _, err := run(context.Background(), Budgets{MaxBufferedBytes: mid})
		if err == nil {
			high = mid
		} else if errors.Is(err, ErrResourceLimit) {
			low = mid + 1
		} else {
			t.Fatal(err)
		}
	}
	for _, delta := range []int64{0, -1} {
		_, result, err = run(context.Background(), Budgets{MaxBufferedBytes: low + delta})
		if delta == 0 && (err != nil || result.State != EvidenceKnown) || delta < 0 && (!errors.Is(err, ErrResourceLimit) || result.State == EvidenceKnown) {
			t.Fatalf("storage delta=%d result=%+v err=%v", delta, result, err)
		}
	}
	for at := 1; at <= counter.calls; at++ {
		_, result, err := run(&rationalCancelContext{Context: context.Background(), at: at}, Budgets{})
		if !errors.Is(err, context.Canceled) || result.State == EvidenceKnown {
			t.Fatalf("poll=%d result=%+v err=%v", at, result, err)
		}
	}
	t.Logf("exact work=%d, exact live storage=%d; interrupted all %d cancellation polls", steps, low, counter.calls)
}
func TestIndependentSingletonRepeatOwnedPrerequisiteCauses(t *testing.T) {
	for _, kind := range []string{"attributes", "end", "instrument"} {
		t.Run(kind, func(t *testing.T) {
			r, timeline, rows, index, plan := singletonRepeatFixture(t, Budgets{})
			defer r.s.cleanup()
			id := index.events[rows[0].value.Part.Value][0]
			cause := &DecisionRef{Rule: RuleKey{"independent-prerequisite", 1}, Subject: NodeRef{"d", id}}
			invalid := Evidence[bool]{State: EvidenceInvalid, Reason: "independent-unavailable", Cause: cause}
			switch kind {
			case "attributes":
				r.s.nodes[id].end.AttributesComplete = testEvidence(invalid)
			case "end":
				f := timeline.Facts[id]
				f.End = stateUnavailable[bool, musicalPosition](invalid)
				timeline.Facts[id] = f
			case "instrument":
				timeline.State.NoteInstruments[0].Complete = invalid
			}
			if err := contextRepeatProveSingletons(r, timeline, rows, index, plan); err != nil {
				t.Fatal(err)
			}
			got := rows[0].value.ContentEquivalent
			if got.State != EvidenceInvalid || got.Cause == nil || *got.Cause != *cause {
				t.Fatalf("cause lost %+v", got)
			}
			clone, err := cloneRepeatContext(context.Background(), r.s, &rows[0].value)
			if err != nil {
				t.Fatal(err)
			}
			before := *cause
			cause.Rule.ID = "mutated-source"
			if clone.ContentEquivalent.Cause == cause || *clone.ContentEquivalent.Cause != before {
				t.Fatal("public projection aliases prerequisite cause")
			}
		})
	}
}
func TestIndependentSingletonRepeatStorageWidths(t *testing.T) {
	const allowance = uintptr(1024)
	note := unsafe.Sizeof(contextRepeatSingletonNote{})
	// An event-capacity note slot plus ample map/control overhead must fit even
	// an empty attributes event, which provides only its own sequence node.
	if note+128 > allowance {
		t.Fatal("event-capacity slice exceeds per-node envelope")
	}
	// One full note slot plus three per-measure slots cover old+new append
	// backing. The smallest admitted rest has six independently enumerated nodes.
	measureEntry := unsafe.Sizeof(uint64(0)) + unsafe.Sizeof([]contextRepeatSingletonNote(nil))
	if 4*note+3*measureEntry+512 > 6*allowance {
		t.Fatal("note/measure append growth exceeds six-node envelope")
	}
	// Map expansion includes old/new key-value storage. References remain
	// borrowed; this map never clones their backing slice.
	instrumentEntry := unsafe.Sizeof(NodeID(0)) + unsafe.Sizeof(noteInstrumentSet{})
	if 3*instrumentEntry+256 > allowance {
		t.Fatal("instrument-map growth exceeds per-record envelope")
	}
	partEntry := unsafe.Sizeof(NodeRef{}) + unsafe.Sizeof((*contextRepeatSingletonPart)(nil))
	rowEnvelope := 3*partEntry + 2*unsafe.Sizeof(contextRepeatSingletonPart{}) + unsafe.Sizeof(Evidence[bool]{}) + 256
	if rowEnvelope > allowance {
		t.Fatal("part-map and staged-result growth exceeds per-row envelope")
	}
	t.Logf("note=%d; six-node envelope=%d; instrument envelope=%d; row envelope=%d", note, 4*note+3*measureEntry+512, 3*instrumentEntry+256, rowEnvelope)
}

func TestIndependentSingletonRepeatDeclinesIncompleteDomain(t *testing.T) {
	for _, kind := range []string{"scope-incomplete", "range-incomplete", "missing-stop", "unplaced-note", "uncontained-end", "uncontained-source", "uncontained-stop", "missing-transpose", "ambiguous-transpose", "transpose-gap", "unavailable-transpose", "unknown-event", "unknown-attributes", "wrong-instrument-part", "wrong-instrument-document", "different-voice", "shifted-onset"} {
		t.Run(kind, func(t *testing.T) {
			r, timeline, rows, index, plan := singletonRepeatFixture(t, Budgets{})
			defer r.s.cleanup()
			part := rows[0].value.Part.Value
			ids := index.events[part]
			switch kind {
			case "scope-incomplete":
				rows[0].value.NoteScopesComplete = stateUnknown[bool]("incomplete")
			case "range-incomplete":
				rows[0].value.Complete = stateUnknown[bool]("incomplete")
			case "missing-stop":
				rows[0].value.Stop = stateAbsent[NodeRef]()
			case "unplaced-note":
				f := timeline.Facts[ids[0]]
				f.Position = stateUnknown[musicalPosition]("unplaced")
				timeline.Facts[ids[0]] = f
			case "uncontained-end":
				f := timeline.Facts[ids[0]]
				f.End.Value.UnprovedFrame = true
				timeline.Facts[ids[0]] = f
			case "uncontained-source":
				f := timeline.Facts[ids[0]]
				f.Position.Value.UnprovedFrame = true
				f.Position.Value.Offset = mustRational(t, 1, 2)
				timeline.Facts[ids[0]] = f
			case "uncontained-stop":
				rows[1].position.Value.UnprovedFrame = true
			case "missing-transpose":
				timeline.State.Transpose.Publications = nil
			case "ambiguous-transpose":
				timeline.State.Transpose.Publications = append(timeline.State.Transpose.Publications, timeline.State.Transpose.Publications[0])
			case "transpose-gap":
				timeline.State.Transpose.Gaps = []stateGap{{Part: part, Availability: stateUnknown[bool]("unresolved-transpose")}}
			case "unavailable-transpose":
				timeline.State.Transpose.Publications[0].Value = stateUnknown[transposeState]("unknown-transpose")
			case "unknown-event":
				id := NodeID(10000)
				r.s.nodes[id] = localNode(id, 0, "unknown")
				r.s.sequence = append(r.s.sequence, id)
				index.events[part] = append(index.events[part], id)
			case "unknown-attributes":
				r.s.nodes[ids[0]].end.AttributesComplete = testEvidence(stateUnknown[bool]("unknown-attributes"))
			case "wrong-instrument-part":
				timeline.State.NoteInstruments[0].Part = NodeRef{"d", 200}
			case "wrong-instrument-document":
				timeline.State.NoteInstruments[1].References[0].Value.DocumentID = "other"
			case "different-voice":
				f := timeline.Facts[ids[1]]
				f.Voice = stateKnown("2")
				timeline.Facts[ids[1]] = f
			case "shifted-onset":
				f := timeline.Facts[ids[1]]
				f.Position.Value.Offset = mustRational(t, 3, 2)
				f.End.Value.Offset = mustRational(t, 5, 2)
				timeline.Facts[ids[1]] = f
			}
			if err := contextRepeatProveSingletons(r, timeline, rows, index, plan); err != nil {
				t.Fatal(err)
			}
			if got := rows[0].value.ContentEquivalent; got.State != EvidenceUnknown {
				t.Fatalf("incomplete domain became decisive %+v", got)
			}
		})
	}
}

func TestIndependentSingletonRepeatEventRosterAdmission(t *testing.T) {
	run := func(ctx context.Context, limits Budgets) (int64, error) {
		s, r, timeline, plan, _ := repeatR3ContainerFixture(t, 1, false, limits)
		r.ctx = ctx
		s.nodes[3].end.ChildrenComplete = testEvidence(stateKnown(true))
		// Seed an already established canonical identity at the enclosing part;
		// this isolates roster allocation from the identity resolver's own budgets.
		timeline.Facts = map[NodeID]timelineEventFact{2: {Part: stateKnown(NodeRef{"d", 100})}}
		event := localNode(4, 3, "unrepresented-event")
		s.nodes[4] = event
		s.sequence = append(s.sequence, 4)
		index, free, err := contextRepeatBuildProofIndex(r, timeline, nil, plan)
		steps := rationalUsage(t, s, BudgetRuleSteps)
		if err == nil {
			if len(index.events[NodeRef{"d", 100}]) != 1 {
				t.Fatal("unrepresented direct event omitted")
			}
			free()
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("event-roster admission leaked")
		}
		s.cleanup()
		return steps, err
	}
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	steps, err := run(counter, Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	for _, limits := range []Budgets{{MaxBufferedBytes: 4 * (640 + 128)}, {MaxBufferedBytes: 4*(640+128) - 1}, {MaxRuleSteps: steps}, {MaxRuleSteps: steps - 1}} {
		_, err := run(context.Background(), limits)
		exact := limits.MaxBufferedBytes == 4*(640+128) || limits.MaxRuleSteps == steps
		if exact && err != nil || !exact && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("limits=%+v err=%v", limits, err)
		}
	}
	for at := 1; at <= counter.calls; at++ {
		if _, err := run(&rationalCancelContext{Context: context.Background(), at: at}, Budgets{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d: %v", at, err)
		}
	}
}

func TestIndependentSingletonRepeatPreservesPriorProofsOnInterruption(t *testing.T) {
	r, timeline, rows, index, plan := singletonRepeatFixture(t, Budgets{MaxRuleSteps: 1})
	defer r.s.cleanup()
	prior := contextRepeatRecord{source: NodeRef{"d", 2000}, value: RepeatContextResult{ContentEquivalent: stateKnown(false)}}
	rows = append(rows, prior)
	err := contextRepeatProveSingletons(r, timeline, rows, index, plan)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatal(err)
	}
	if got := rows[len(rows)-1].value.ContentEquivalent; got.State != EvidenceKnown || got.Value {
		t.Fatalf("prior independent failure erased %+v", got)
	}
}

func TestIndependentSingletonRepeatCheckedWorkAmounts(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 1})
	defer s.cleanup()
	if err := contextRepeatSingletonWork(context.Background(), s, 1); err != nil {
		t.Fatal(err)
	}
	if err := contextRepeatSingletonWork(context.Background(), s, math.MaxInt64, 1); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("work overflow not refused: %v", err)
	}
	if got := rationalUsage(t, s, BudgetRuleSteps); got != 1 {
		t.Fatalf("failed overflow charged %d", got)
	}
}
