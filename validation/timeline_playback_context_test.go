package validation

import (
	"context"
	"errors"
	"testing"
)

func playbackFixture(t *testing.T) (*Session, *timelinePlaybackContext, *timelineBindings, timelineMeasureBlock, timelineEventFact) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	t.Cleanup(s.cleanup)
	b := stateBindings()
	block := runtimeBlock(b, 0)
	block.measure.header.ID = 10
	fact := timelineEventFact{Part: block.part, Position: timelineAt(block, stateKnown(mustRational(t, 3, 2))), Staff: stateAbsent[Integer](), Voice: stateAbsent[string](), Principal: stateAbsent[NodeRef]()}
	return s, &timelinePlaybackContext{}, b, block, fact
}
func TestPlaybackBarlinePreservesObservedPositionAndLocation(t *testing.T) {
	for _, location := range []string{"", "left", "middle", "right"} {
		t.Run(location, func(t *testing.T) {
			s, p, b, block, fact := playbackFixture(t)
			bar := stateNode(b, timelineRoleBarline, 50, stateAbsent[Atomic]())
			if location != "" {
				stateAttribute(b, bar, timelineRoleBarlineLocation, Atomic{Kind: AtomicString, Text: location})
			}
			if err := p.readBarline(context.Background(), s, b, bar, block, fact); err != nil {
				t.Fatal(err)
			}
			got := p.Barlines[50]
			want := location
			if want == "" {
				want = "right"
			}
			if got.Location.State != EvidenceKnown || got.Location.Value != want || got.LocationDefaulted != (location == "") {
				t.Fatalf("location %+v", got)
			}
			if got.Origin.Position.Value.Offset.String() != "3/2" || got.Origin.Measure != fact.Position.Value.Measure {
				t.Fatalf("invented barline position %+v", got)
			}
			if got.RawLocation.State == EvidenceAbsent && location != "" {
				t.Fatal("lost explicit location")
			}
		})
	}
}
func TestPlaybackNavigationDivisionsAreMarkerScopedAndRaw(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	for i, marker := range []bool{false, true} {
		bar := stateNode(b, timelineRoleBarline, NodeID(50+i), stateAbsent[Atomic]())
		stateAttribute(b, bar, timelineRoleBarlineDivisions, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, -3, 2)})
		stateAttribute(b, bar, timelineRoleBarlineSegno, Atomic{Kind: AtomicString, Text: "S"})
		if marker {
			bar.children = []*factNode{stateNode(b, timelineRoleBarlineSegnoElement, 60, stateAbsent[Atomic]())}
		}
		if err := p.readBarline(context.Background(), s, b, bar, block, fact); err != nil {
			t.Fatal(err)
		}
		got := p.Barlines[bar.header.ID].Navigation
		if got.Divisions.Value.String() != "-1.5" || got.DivisionsApplicable.State != EvidenceKnown || got.DivisionsApplicable.Value != marker {
			t.Fatalf("navigation scope %+v", got)
		}
	}
	sound := stateNode(b, timelineRoleSound, 70, stateAbsent[Atomic]())
	stateAttribute(b, sound, timelineRoleSoundDivisions, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 0, 1)})
	stateAttribute(b, sound, timelineRoleSoundCoda, Atomic{Kind: AtomicString, Text: "C"})
	if err := p.readSound(context.Background(), s, b, sound, block, fact); err != nil {
		t.Fatal(err)
	}
	got := p.Sounds[70]
	if got.Navigation.Divisions.Value.String() != "0" || !got.Navigation.DivisionsApplicable.Value || got.Origin.Position.Value.Offset.String() != "3/2" {
		t.Fatalf("sound navigation %+v", got)
	}
}
func TestPlaybackNotePlayIsCurrentChordToneOnly(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		t.Run(string(order), func(t *testing.T) {
			s, p, b, block, fact := playbackFixture(t)
			n := stateNode(b, timelineRoleNote, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleNoteChord, 21, stateAbsent[Atomic]()), stateNode(b, timelineRoleNotePlay, 22, stateAbsent[Atomic](), stateNode(b, timelineRolePlayMute, 23, stateAtomicText("on"))))
			n.end.Order = testEvidence(stateKnown(order))
			fact.Principal = stateKnown(NodeRef{DocumentID: "d", Node: 10})
			stateAttribute(b, n, timelineRoleNotePizzicato, Atomic{Kind: AtomicString, Text: "no"})
			if err := p.readNote(context.Background(), s, b, n, block, fact); err != nil {
				t.Fatal(err)
			}
			got := p.Notes[20]
			if len(got.Play) != 1 || got.Play[0].Scope != playbackCurrentNote || got.Origin.Source.Node != 20 || got.Principal.Value.Node != 10 || !got.Chord.Value {
				t.Fatalf("note scope %+v", got)
			}
			if got.Origin.Position.Value.Offset.String() != "3/2" || got.Pizzicato.State != EvidenceKnown || got.Pizzicato.Value {
				t.Fatalf("note origin %+v", got)
			}
			other := stateNode(b, timelineRoleNote, 30, stateAbsent[Atomic]())
			if err := p.readNote(context.Background(), s, b, other, block, fact); err != nil {
				t.Fatal(err)
			}
			if len(p.Notes[30].Play) != 0 || p.Notes[30].Pizzicato.State != EvidenceAbsent || len(p.Pizzicato.Publications) != 0 {
				t.Fatal("note playback leaked to another note or continuing state")
			}
		})
	}
}
func TestPlaybackSoundStateUsesScoreOrderAndSeparateFamilies(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	h := &timelineState{}
	stateClose(t, s, h, 2)
	for i, at := range []int64{2, 0, 1} {
		n := stateNode(b, timelineRoleSound, NodeID(50+i), stateAbsent[Atomic]())
		stateAttribute(b, n, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60+at, 1)})
		fact.Position = timelineAt(block, stateKnown(mustRational(t, at, 1)))
		if at == 0 {
			stateAttribute(b, n, timelineRoleSoundPizzicato, Atomic{Kind: AtomicString, Text: "yes"})
		}
		if err := p.readSound(context.Background(), s, b, n, block, fact); err != nil {
			t.Fatal(err)
		}
	}
	at := statePosition(t, 0, 3, 1)
	tempo, err := p.effectiveTempo(context.Background(), s, h, at)
	if err != nil || tempo.Value.State != EvidenceKnown || tempo.Value.Value.String() != "62" || tempo.Provenance[0].Node.Node != 50 {
		t.Fatalf("tempo %+v,%v", tempo, err)
	}
	pizz, err := p.effectivePizzicato(context.Background(), s, h, at)
	if err != nil || pizz.Value.State != EvidenceKnown || !pizz.Value.Value {
		t.Fatalf("pizzicato %+v,%v", pizz, err)
	}
	dynamics, err := p.effectiveDynamics(context.Background(), s, h, at)
	if err != nil || dynamics.Value.State != EvidenceKnown || dynamics.Value.Value.String() != "100" || dynamics.Provenance[0].Kind != stateInitialDefault {
		t.Fatalf("documented initial forte %+v,%v", dynamics, err)
	}
}
func TestPlaybackNoTempoDefaultConditionalStateAndZero(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	h := &timelineState{}
	stateClose(t, s, h, 0)
	before, err := p.effectiveTempo(context.Background(), s, h, statePosition(t, 0, 0, 1))
	if err != nil || before.Value.State != EvidenceAbsent {
		t.Fatalf("invented tempo %+v,%v", before, err)
	}
	n := stateNode(b, timelineRoleSound, 50, stateAbsent[Atomic]())
	stateAttribute(b, n, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 0, 1)})
	stateAttribute(b, n, timelineRoleSoundDynamics, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 0, 1)})
	stateAttribute(b, n, timelineRoleSoundTimeOnly, Atomic{Kind: AtomicString, Text: "2,3"})
	if err := p.readSound(context.Background(), s, b, n, block, fact); err != nil {
		t.Fatal(err)
	}
	if p.Sounds[50].Tempo.Value.String() != "0" || p.Sounds[50].TimeOnly.Value != "2,3" {
		t.Fatal("lost raw conditional zero")
	}
	after, err := p.effectiveTempo(context.Background(), s, h, statePosition(t, 0, 2, 1))
	if err != nil || after.Value.State != EvidenceUnknown || after.Value.Reason != "playback-repeat-pass-unavailable" {
		t.Fatalf("unconditional repeat state %+v,%v", after, err)
	}
}
func TestPlaybackInvalidCauseOwnedAndIndependentFromRaw(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	h := &timelineState{}
	stateClose(t, s, h, 0)
	n := stateNode(b, timelineRoleSound, 50, stateAbsent[Atomic]())
	stateAttribute(b, n, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 88, 1)})
	cause := &DecisionRef{Rule: RuleKey{ID: "scalar", Revision: 1}, Subject: NodeRef{DocumentID: "d", Node: 50}}
	n.attributes[0].Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "tempo-invalid", Cause: cause}
	if err := p.readSound(context.Background(), s, b, n, block, fact); err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "mutated"
	got := p.Sounds[50]
	if got.Tempo.Cause == nil || got.Tempo.Cause.Rule.ID != "scalar" {
		t.Fatalf("borrowed input cause %+v", got)
	}
	got.Tempo.Cause.Rule.ID = "raw-mutated"
	state, err := p.effectiveTempo(context.Background(), s, h, statePosition(t, 0, 2, 1))
	if err != nil || state.Value.State != EvidenceInvalid || state.Value.Cause.Rule.ID != "scalar" {
		t.Fatalf("history shared raw cause %+v,%v", state, err)
	}
	dynamics, err := p.effectiveDynamics(context.Background(), s, h, statePosition(t, 0, 2, 1))
	if err != nil || dynamics.Value.State != EvidenceKnown {
		t.Fatalf("unrelated family masked %+v,%v", dynamics, err)
	}
}
func TestPlaybackQuotaCancellationAndNoPublication(t *testing.T) {
	for _, kind := range []string{"context", "work", "retention", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			s, p, b, block, fact := playbackFixture(t)
			n := stateNode(b, timelineRoleBarline, 50, stateAbsent[Atomic]())
			ctx := context.Background()
			switch kind {
			case "context":
				s.account, _ = NewBudgetAccount(Budgets{MaxContextFacts: 1})
				if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
					t.Fatal(err)
				}
			case "work":
				s.account, _ = NewBudgetAccount(Budgets{MaxRuleSteps: 1})
			case "retention":
				s.account, _ = NewBudgetAccount(Budgets{MaxBufferedBytes: 1})
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err := p.readBarline(ctx, s, b, n, block, fact)
			if err == nil || len(p.Barlines) != 0 {
				t.Fatalf("quota/cancellation publication %v,%d", err, len(p.Barlines))
			}
			if kind == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestPlaybackMissingFactsStayScopedAndDoNotInventDefaults(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	h := &timelineState{}
	stateClose(t, s, h, 2)
	n := stateNode(b, timelineRoleSound, 50, stateAbsent[Atomic]())
	stateAttribute(b, n, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 77, 1)})
	cause := &DecisionRef{Rule: RuleKey{ID: "duration", Revision: 2}, Subject: NodeRef{"d", 40}}
	fact.Position = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "duration-invalid", Cause: cause}
	if err := p.readSound(context.Background(), s, b, n, block, fact); err != nil {
		t.Fatal(err)
	}
	cause.Subject.Node = 999
	got, err := p.effectiveTempo(context.Background(), s, h, statePosition(t, 0, 2, 1))
	if err != nil || got.Value.State != EvidenceInvalid || got.Value.Cause.Subject.Node != 40 {
		t.Fatalf("missing position %+v,%v", got, err)
	}
	dynamics, err := p.effectiveDynamics(context.Background(), s, h, statePosition(t, 0, 2, 1))
	if err != nil || dynamics.Value.State != EvidenceKnown {
		t.Fatalf("unrelated absent family %+v,%v", dynamics, err)
	}
	// A confirmed publisher in a later closed measure establishes a new suffix.
	block.ordinal = 1
	block.measure.header.ID = 11
	fact.Position = stateKnown(statePosition(t, 1, 0, 1))
	next := stateNode(b, timelineRoleSound, 60, stateAbsent[Atomic]())
	stateAttribute(b, next, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 80, 1)})
	if err := p.readSound(context.Background(), s, b, next, block, fact); err != nil {
		t.Fatal(err)
	}
	got, err = p.effectiveTempo(context.Background(), s, h, statePosition(t, 1, 1, 1))
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.String() != "80" {
		t.Fatalf("suffix did not recover %+v,%v", got, err)
	}
	other := statePosition(t, 1, 1, 1)
	other.Part.Node = 2
	got, err = p.effectiveTempo(context.Background(), s, h, other)
	if err != nil || got.Value.State != EvidenceUnknown {
		t.Fatalf("part scope leaked %+v,%v", got, err)
	}
}
func TestPlaybackHierarchyGapPreventsFalseDefaultsAndPreservesPrefix(t *testing.T) {
	s, p, _, _, _ := playbackFixture(t)
	h := &timelineState{}
	stateClose(t, s, h, 2)
	cause := &DecisionRef{Rule: RuleKey{ID: "owner", Revision: 1}, Subject: NodeRef{"d", 90}}
	bad := Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "owner-unresolved", Cause: cause}
	if err := p.gapAll(context.Background(), s, NodeRef{"d", 1}, 1, bad); err != nil {
		t.Fatal(err)
	}
	cause.Subject.Node = 91
	before, err := p.effectiveDynamics(context.Background(), s, h, statePosition(t, 0, 0, 1))
	if err != nil || before.Value.State != EvidenceKnown {
		t.Fatalf("proved prefix lost %+v,%v", before, err)
	}
	after, err := p.effectiveDynamics(context.Background(), s, h, statePosition(t, 1, 0, 1))
	if err != nil || after.Value.State != EvidenceInvalid || after.Value.Cause.Subject.Node != 90 {
		t.Fatalf("false default after gap %+v,%v", after, err)
	}
	after.Value.Cause.Subject.Node = 92
	again, err := p.effectivePizzicato(context.Background(), s, h, statePosition(t, 1, 0, 1))
	if err != nil || again.Value.Cause.Subject.Node != 90 {
		t.Fatalf("cross-family borrowed gap %+v,%v", again, err)
	}
}
func TestPlaybackSamePositionConflictAndExplicitZero(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	h := &timelineState{}
	stateClose(t, s, h, 0)
	for i, value := range []int64{0, 90} {
		n := stateNode(b, timelineRoleSound, NodeID(50+i), stateAbsent[Atomic]())
		stateAttribute(b, n, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, value, 1)})
		if err := p.readSound(context.Background(), s, b, n, block, fact); err != nil {
			t.Fatal(err)
		}
	}
	got, err := p.effectiveTempo(context.Background(), s, h, statePosition(t, 0, 2, 1))
	if err != nil || got.Value.State != EvidenceUnknown || got.Value.Reason != "timeline-state-conflict" {
		t.Fatalf("invented tie winner %+v,%v", got, err)
	}
	if p.Sounds[50].Tempo.Value.String() != "0" {
		t.Fatal("zero became tempo default")
	}
}
func TestPlaybackUnknownLocationAndExactBinding(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	bar := stateNode(b, timelineRoleBarline, 50, stateAbsent[Atomic]())
	bar.end.AttributesComplete = testEvidence(stateUnknown[bool]("attributes-not-observed"))
	if err := p.readBarline(context.Background(), s, b, bar, block, fact); err != nil {
		t.Fatal(err)
	}
	if p.Barlines[50].Location.State != EvidenceUnknown || p.Barlines[50].LocationDefaulted {
		t.Fatal("unknown location became right default")
	}
	fake := stateNode(b, timelineRoleSound, 60, stateAbsent[Atomic]())
	fake.header.Name = QName{Local: "barline"}
	if err := p.readBarline(context.Background(), s, b, fake, block, fact); !errors.Is(err, ErrInvalidConfiguration) || len(p.Barlines) != 1 {
		t.Fatalf("matched runtime local name %v", err)
	}
}

func TestPlaybackRawAttributePresenceIsNotSharedXSDDefault(t *testing.T) {
	for _, target := range []Target{TargetSource, TargetModel} {
		t.Run(string(target), func(t *testing.T) {
			schema := localSchema(t)
			engine := localEngine(t, schema, target)
			// The shared schema's beam@number default supplies a real effective scalar,
			// without inventing an explicit input fact. The raw reader must preserve that
			// distinction for every defaulted declaration, including barline@location.
			s := localAdmit(t, engine, target, lt("root", lt("note", lt("beam"))), nil)
			defer s.cleanup()
			b := stateBindings()
			use := localUse(t, schema, "beam", "number", true)
			b.Roles[timelineRoleBarlineLocation] = []SchemaUse{use}
			r := &stateReader{ctx: context.Background(), s: s, b: b}
			effective := r.attribute(s.nodes[3], timelineRoleBarlineLocation)
			if r.err != nil || effective.State != EvidenceKnown || effective.Value.Integer.String() != "1" {
				t.Fatalf("fixture lacks XSD default: %+v,%v", effective, r.err)
			}
			raw := r.playbackRawAttribute(s.nodes[3], timelineRoleBarlineLocation)
			if r.err != nil || raw.State != EvidenceAbsent {
				t.Fatalf("default presented as explicit: %+v,%v", raw, r.err)
			}
		})
	}
}

func TestPlaybackOrdinaryNoteDefaultBudgetScaling(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	for i := 0; i < 1000; i++ {
		n := runtimeNote(t, b, NodeID(100+i*4), 3, false)
		if err := p.readNote(context.Background(), s, b, n, block, fact); err != nil {
			t.Fatalf("ordinary note %d: %v", i, err)
		}
	}
	if len(p.Notes) != 1000 {
		t.Fatal("missing ordinary note evidence")
	}
	t.Logf("1000 note playback contexts: work=%d bytes=%d context=%d", rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes), rationalUsage(t, s, BudgetContextFacts))
	s.cleanup()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("live bytes not released")
	}
	if rationalUsage(t, s, BudgetContextFacts) != 1000 {
		t.Fatal("cumulative work refunded")
	}
}
