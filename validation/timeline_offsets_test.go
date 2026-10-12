package validation

import (
	"context"
	"errors"
	"testing"
)

func TestTimelineOffsetPositionExactAndDependencies(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "offset-scalar", Revision: 1}, Subject: NodeRef{"d", 9}}
	knownBase := stateKnown(statePosition(t, 0, 2, 3))
	badBase := Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "invalid-base", Cause: cause}
	badOffset := Evidence[Decimal]{State: EvidenceInvalid, Reason: "invalid-offset", Cause: cause}
	badUnits := Evidence[Decimal]{State: EvidenceInvalid, Reason: "invalid-units", Cause: cause}
	knownUnits := stateKnown(stateDecimal(t, 3, 1))
	for _, tc := range []struct {
		name          string
		base          Evidence[musicalPosition]
		offset, units Evidence[Decimal]
		state         EvidenceState
		want          string
		reason        Code
		cause         *DecisionRef
	}{
		{"absent offset does not need units", knownBase, stateAbsent[Decimal](), badUnits, EvidenceKnown, "2/3", "", nil},
		{"exact third", knownBase, stateKnown(stateDecimal(t, 1, 1)), knownUnits, EvidenceKnown, "1", "", nil},
		{"exact fractional divisions", knownBase, stateKnown(stateDecimal(t, 1, 2)), stateKnown(stateDecimal(t, 3, 2)), EvidenceKnown, "1", "", nil},
		{"signed displacement in established units", knownBase, stateKnown(stateDecimal(t, -1, 1)), knownUnits, EvidenceKnown, "1/3", "", nil},
		{"base invalid outranks offset unknown", badBase, stateUnknown[Decimal]("unknown-offset"), knownUnits, EvidenceInvalid, "", "invalid-base", cause},
		{"offset invalid outranks base unsupported", Evidence[musicalPosition]{State: EvidenceUnsupported, Reason: "unsupported-base"}, badOffset, knownUnits, EvidenceInvalid, "", "invalid-offset", cause},
		{"units invalid outrank offset unknown", knownBase, stateUnknown[Decimal]("unknown-offset"), badUnits, EvidenceInvalid, "", "invalid-units", cause},
		{"unsupported units outrank unknown base", stateUnknown[musicalPosition]("unknown-base"), stateKnown(stateDecimal(t, 1, 1)), Evidence[Decimal]{State: EvidenceUnsupported, Reason: "unsupported-units"}, EvidenceUnsupported, "", "unsupported-units", nil},
		{"absent units do not invent one", knownBase, stateKnown(stateDecimal(t, 1, 1)), stateAbsent[Decimal](), EvidenceUnknown, "", "initial-divisions-unavailable", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			got, err := s.offsetPosition(context.Background(), tc.base, tc.offset, tc.units)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.state || got.Reason != tc.reason || (tc.cause != nil && (got.Cause == nil || *got.Cause != *tc.cause)) {
				t.Errorf("position=%+v, want state=%s reason=%s cause=%+v", got, tc.state, tc.reason, tc.cause)
			}
			if got.State == EvidenceKnown && (got.Value.Offset.String() != tc.want || got.Value.Part != knownBase.Value.Part || got.Value.Measure != knownBase.Value.Measure || got.Value.MeasureOrdinal != knownBase.Value.MeasureOrdinal) {
				t.Errorf("position=%+v want offset=%s with original frame", got.Value, tc.want)
			}
			if knownBase.Value.Offset.String() != "2/3" {
				t.Fatal("mutated original cursor position")
			}
		})
	}
}

func offsetSoundNode(t *testing.T, b *timelineBindings, role timelineRole, own Evidence[Decimal]) *factNode {
	t.Helper()
	children := []*factNode{stateNode(b, timelineRoleSoundPlay, 51, stateAbsent[Atomic](), stateNode(b, timelineRolePlayIpa, 52, stateAtomicText("a")))}
	if own.State != EvidenceAbsent {
		scalar := Evidence[Atomic]{State: own.State, Reason: own.Reason, Cause: own.Cause}
		if own.State == EvidenceKnown {
			scalar.Value = Atomic{Kind: AtomicDecimal, Decimal: own.Value}
		}
		children = append(children, stateNode(b, timelineRoleSoundOffset, 53, scalar))
	}
	return stateNode(b, role, 50, stateAbsent[Atomic](), children...)
}

func TestTimelineReadSoundOffsetPrecedence(t *testing.T) {
	absent := stateAbsent[Decimal]()
	units := stateKnown(stateDecimal(t, 3, 1))
	dirOffset := stateKnown(stateDecimal(t, 6, 1))
	ownOffset := stateKnown(stateDecimal(t, 1, 1))
	cause := &DecisionRef{Rule: RuleKey{ID: "sound-offset", Revision: 1}, Subject: NodeRef{"d", 53}}
	invalid := Evidence[Decimal]{State: EvidenceInvalid, Reason: "invalid-sound-offset", Cause: cause}
	for _, tc := range []struct {
		name                  string
		role                  timelineRole
		own, direction, units Evidence[Decimal]
		playback              Evidence[bool]
		want                  string
		state                 EvidenceState
		reason                Code
	}{
		{"direct offset", timelineRoleSound, ownOffset, absent, units, stateAbsent[bool](), "7/3", EvidenceKnown, ""},
		{"direct no offset needs no units", timelineRoleSound, absent, absent, stateAbsent[Decimal](), stateAbsent[bool](), "2", EvidenceKnown, ""},
		{"direction sound absent defaults no", timelineRoleDirectionSound, absent, dirOffset, units, stateAbsent[bool](), "2", EvidenceKnown, ""},
		{"direction sound no", timelineRoleDirectionSound, absent, dirOffset, units, stateKnown(false), "2", EvidenceKnown, ""},
		{"direction sound yes", timelineRoleDirectionSound, absent, dirOffset, units, stateKnown(true), "4", EvidenceKnown, ""},
		{"own overrides parent without adding", timelineRoleDirectionSound, ownOffset, dirOffset, units, stateKnown(true), "7/3", EvidenceKnown, ""},
		{"own zero overrides parent", timelineRoleDirectionSound, stateKnown(stateDecimal(t, 0, 1)), dirOffset, units, stateKnown(true), "2", EvidenceKnown, ""},
		{"own override ignores invalid direction", timelineRoleDirectionSound, ownOffset, invalid, units, Evidence[bool]{State: EvidenceInvalid, Reason: "invalid-direction-flag", Cause: cause}, "7/3", EvidenceKnown, ""},
		{"no playback ignores invalid direction offset and units", timelineRoleDirectionSound, absent, invalid, invalid, stateKnown(false), "2", EvidenceKnown, ""},
		{"invalid own does not fall back", timelineRoleDirectionSound, invalid, dirOffset, units, stateKnown(true), "", EvidenceInvalid, "invalid-sound-offset"},
		{"unknown playback flag is not default no", timelineRoleDirectionSound, absent, dirOffset, units, stateUnknown[bool]("unknown-sound-flag"), "", EvidenceUnknown, "unknown-sound-flag"},
		{"no direction offset needs no playback flag", timelineRoleDirectionSound, absent, absent, units, stateUnknown[bool]("unknown-sound-flag"), "2", EvidenceKnown, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, service, b := runtimeFixture(t)
			sound := offsetSoundNode(t, b, tc.role, tc.own)
			block := runtimeBlock(b, 0)
			base := timelineAt(block, stateKnown(mustRational(t, 2, 1)))
			if err := service.readSoundAt(context.Background(), s, b, sound, block, base, tc.units, tc.direction, tc.playback); err != nil {
				t.Fatal(err)
			}
			fact, ok := service.Facts[sound.header.ID]
			if !ok || fact.Position.State != tc.state || fact.Position.Reason != tc.reason || fact.Part != block.part {
				t.Errorf("own sound fact=%+v exists=%v", fact, ok)
			}
			if len(service.State.Play) != 1 {
				t.Fatalf("raw sound play records=%d", len(service.State.Play))
			}
			got := service.State.Play[0]
			if got.Position.State != tc.state || got.Position.Reason != tc.reason {
				t.Errorf("raw play position=%+v", got.Position)
			}
			if tc.state == EvidenceKnown && (got.Position.Value.Offset.String() != tc.want || fact.Position.Value.Offset.String() != tc.want) {
				t.Errorf("sound position=%+v want%s", got.Position, tc.want)
			}
			if tc.state == EvidenceInvalid && (got.Position.Cause == nil || *got.Position.Cause != *cause) {
				t.Errorf("lost invalid offset cause: %+v", got.Position)
			}
			if got.Value.State != EvidenceKnown || len(got.Value.Value.Techniques) != 1 || got.Value.Value.Techniques[0].Value.Value != "a" {
				t.Errorf("raw play value lost: %+v", got.Value)
			}
			if len(service.State.InstrumentGaps) != 0 {
				t.Error("sound without instrument changes fabricated virtual instrument gap")
			}
		})
	}
}

func TestTimelineReadSoundUnplacedPreservesRawRecords(t *testing.T) {
	s, service, b := runtimeFixture(t)
	for _, role := range []timelineRole{timelineRoleMidiDevicePort, timelineRoleMidiDeviceID, timelineRoleMidiInstrumentID, timelineRolePlayID} {
		b.Roles[role][0].Attribute = true
		b.Roles[role][0].Name = QName{Local: string(rune(1000 + role))}
	}
	cause := &DecisionRef{Rule: RuleKey{ID: "duration", Revision: 1}, Subject: NodeRef{"d", 31}}
	unavailable := Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "invalid-duration", Cause: cause}
	sound := offsetSoundNode(t, b, timelineRoleSound, stateAbsent[Decimal]())
	midi := stateNode(b, timelineRoleSoundMidiInstrument, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleMidiInstrumentMidiProgram, 61, stateAtomicInteger(t, 71)))
	device := stateNode(b, timelineRoleSoundMidiDevice, 62, stateAtomicText("synth"))
	stateAttribute(b, device, timelineRoleMidiDevicePort, Atomic{Kind: AtomicInteger, Integer: stateInt(t, 2)})
	sound.children = append(sound.children, midi, device)
	if err := service.readSoundAt(context.Background(), s, b, sound, runtimeBlock(b, 1), unavailable, stateAbsent[Decimal](), stateAbsent[Decimal](), stateAbsent[bool]()); err != nil {
		t.Fatal(err)
	}
	if len(service.State.MIDI) != 1 || len(service.State.MIDIDevices) != 1 || len(service.State.Play) != 1 {
		t.Fatalf("unplaced records lost: MIDI=%d devices=%d play=%d", len(service.State.MIDI), len(service.State.MIDIDevices), len(service.State.Play))
	}
	for _, got := range []Evidence[musicalPosition]{service.Facts[50].Position, service.State.MIDI[0].Position, service.State.MIDIDevices[0].Position, service.State.Play[0].Position} {
		if got.State != EvidenceInvalid || got.Reason != unavailable.Reason || got.Cause == nil || *got.Cause != *cause {
			t.Errorf("unplaced causal evidence=%+v", got)
		}
	}
	if service.State.MIDI[0].Value.Value.Program.Value.String() != "71" || service.State.MIDIDevices[0].Value.Value.Name.Value != "synth" || service.State.MIDIDevices[0].Value.Value.Port.Value.String() != "2" {
		t.Fatal("unknown placement erased typed MIDI payload")
	}
}

func TestTimelineOffsetPositionCancellationAndBudgets(t *testing.T) {
	base := stateKnown(statePosition(t, 0, 1, 3))
	offset, units := stateKnown(stateDecimal(t, 1, 1)), stateKnown(stateDecimal(t, 3, 1))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	if _, err := s.offsetPosition(ctx, base, stateAbsent[Decimal](), units); !errors.Is(err, context.Canceled) {
		t.Errorf("absent-offset cancellation=%v", err)
	}
	for _, budgets := range []Budgets{{MaxRuleSteps: 1}, {MaxBufferedBytes: 1}, {MaxScalarBytes: 1}} {
		t.Run("bounded", func(t *testing.T) {
			s := rationalSession(t, budgets)
			defer s.cleanup()
			if _, err := s.offsetPosition(context.Background(), base, offset, units); !errors.Is(err, ErrResourceLimit) {
				t.Errorf("budgets=%+v error=%v", budgets, err)
			}
			if base.Value.Offset.String() != "1/3" {
				t.Fatal("failed arithmetic mutated base")
			}
		})
	}
}

func TestTimelineReadSoundUnplacedChangeCannotLeaveInitialStateEffective(t *testing.T) {
	s, service, b := runtimeFixture(t)
	part, instrument := NodeRef{"d", 1}, NodeRef{"d", 70}
	if err := service.State.addInstrumentDefinition(context.Background(), s, instrumentDefinition{Part: part, Instrument: instrument, Value: stateKnown(instrumentState{Sound: stateKnown("initial")})}); err != nil {
		t.Fatal(err)
	}
	cause := &DecisionRef{Rule: RuleKey{ID: "instrument-reference", Revision: 1}, Subject: NodeRef{"d", 60}}
	change := stateNode(b, timelineRoleSoundInstrumentChange, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleInstrumentChangeInstrumentSound, 61, stateAtomicText("changed")))
	use := b.Roles[timelineRoleInstrumentChangeID][0]
	change.attributes = []AttributeFact{{Owner: 60, Name: use.Name, Declaration: stateKnown(use.Declaration), Presence: stateKnown(true), Scalar: Scalar{Value: Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-change-reference", Cause: cause}}}}
	sound := offsetSoundNode(t, b, timelineRoleSound, stateAbsent[Decimal]())
	sound.children = append(sound.children, change)
	if err := service.readSoundAt(context.Background(), s, b, sound, runtimeBlock(b, 1), stateUnknown[musicalPosition]("unknown-cursor"), stateAbsent[Decimal](), stateAbsent[Decimal](), stateAbsent[bool]()); err != nil {
		t.Fatal(err)
	}
	got, err := service.State.effectiveInstrument(context.Background(), s, statePosition(t, 2, 0, 1), instrument)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value.State != EvidenceInvalid || got.Value.Reason != "invalid-change-reference" || got.Value.Cause == nil || *got.Value.Cause != *cause {
		t.Errorf("unplaced change left initial state effective: %+v", got.Value)
	}
	if len(service.State.Play) != 1 || service.State.Play[0].Position.State != EvidenceUnknown {
		t.Error("instrument gap erased independent raw play record")
	}
}

func TestTimelineReadSoundUnavailablePlaybackKeepsStrongestCause(t *testing.T) {
	s, service, b := runtimeFixture(t)
	cause := &DecisionRef{Rule: RuleKey{ID: "divisions", Revision: 1}, Subject: NodeRef{"d", 21}}
	sound := offsetSoundNode(t, b, timelineRoleDirectionSound, stateAbsent[Decimal]())
	units := Evidence[Decimal]{State: EvidenceInvalid, Reason: "invalid-divisions", Cause: cause}
	if err := service.readSoundAt(context.Background(), s, b, sound, runtimeBlock(b, 0), stateUnknown[musicalPosition]("unknown-cursor"), units, stateKnown(stateDecimal(t, 1, 1)), Evidence[bool]{State: EvidenceUnsupported, Reason: "unsupported-playback-flag"}); err != nil {
		t.Fatal(err)
	}
	got := service.Facts[50].Position
	if got.State != EvidenceInvalid || got.Reason != "invalid-divisions" || got.Cause == nil || *got.Cause != *cause {
		t.Errorf("stronger causal evidence discarded: %+v", got)
	}
}

func TestTimelineReadSoundExactBindingAndInterruption(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		budgets Budgets
		role    timelineRole
		want    error
	}{
		{"exact binding", context.Background(), Budgets{}, timelineRoleNote, ErrInvalidConfiguration},
		{"work budget", context.Background(), Budgets{MaxRuleSteps: 1}, timelineRoleSound, ErrResourceLimit},
		{"retained budget", context.Background(), Budgets{MaxBufferedBytes: 1}, timelineRoleSound, ErrResourceLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, tc.budgets)
			s.documentID = "d"
			defer s.cleanup()
			service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
			b := stateBindings()
			sound := offsetSoundNode(t, b, tc.role, stateAbsent[Decimal]())
			sound.header.Name = QName{Local: "sound"}
			err := service.readSoundAt(tc.ctx, s, b, sound, runtimeBlock(b, 0), stateKnown(statePosition(t, 0, 0, 1)), stateAbsent[Decimal](), stateAbsent[Decimal](), stateAbsent[bool]())
			if !errors.Is(err, tc.want) {
				t.Errorf("error=%v want %v", err, tc.want)
			}
			if len(service.Facts) != 0 {
				t.Error("failed extraction published complete sound fact")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, service, b := runtimeFixture(t)
	if err := service.readSoundAt(ctx, s, b, offsetSoundNode(t, b, timelineRoleSound, stateAbsent[Decimal]()), runtimeBlock(b, 0), stateKnown(statePosition(t, 0, 0, 1)), stateAbsent[Decimal](), stateAbsent[Decimal](), stateAbsent[bool]()); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled sound read=%v", err)
	}
	if len(service.Facts) != 0 {
		t.Fatal("canceled extraction published sound fact")
	}
}

func TestTimelineReadListeningOffsetPrecedence(t *testing.T) {
	absent := stateAbsent[Decimal]()
	units := stateKnown(stateDecimal(t, 3, 1))
	ownOffset, directionOffset := stateKnown(stateDecimal(t, 1, 1)), stateKnown(stateDecimal(t, 6, 1))
	cause := &DecisionRef{Rule: RuleKey{ID: "listening-offset", Revision: 1}, Subject: NodeRef{"d", 81}}
	invalid := Evidence[Decimal]{State: EvidenceInvalid, Reason: "invalid-listening-offset", Cause: cause}
	for _, tc := range []struct {
		name                  string
		role                  timelineRole
		own, direction, units Evidence[Decimal]
		playback              Evidence[bool]
		want                  string
		state                 EvidenceState
		reason                Code
	}{
		{"direct offset", timelineRoleListening, ownOffset, absent, units, stateAbsent[bool](), "7/3", EvidenceKnown, ""},
		{"direct no offset needs no units", timelineRoleListening, absent, absent, stateAbsent[Decimal](), stateAbsent[bool](), "2", EvidenceKnown, ""},
		{"direction sound absent defaults no", timelineRoleDirectionListening, absent, directionOffset, units, stateAbsent[bool](), "2", EvidenceKnown, ""},
		{"direction sound no", timelineRoleDirectionListening, absent, directionOffset, units, stateKnown(false), "2", EvidenceKnown, ""},
		{"direction sound yes", timelineRoleDirectionListening, absent, directionOffset, units, stateKnown(true), "4", EvidenceKnown, ""},
		{"own replaces rather than adds", timelineRoleDirectionListening, ownOffset, directionOffset, units, stateKnown(true), "7/3", EvidenceKnown, ""},
		{"own zero replaces parent", timelineRoleDirectionListening, stateKnown(stateDecimal(t, 0, 1)), directionOffset, units, stateKnown(true), "2", EvidenceKnown, ""},
		{"own override ignores parent evidence", timelineRoleDirectionListening, ownOffset, invalid, units, stateUnknown[bool]("unknown-playback"), "7/3", EvidenceKnown, ""},
		{"no playback ignores offset and units", timelineRoleDirectionListening, absent, invalid, invalid, stateKnown(false), "2", EvidenceKnown, ""},
		{"invalid own never falls back", timelineRoleDirectionListening, invalid, directionOffset, units, stateKnown(true), "", EvidenceInvalid, "invalid-listening-offset"},
		{"unknown flag not default no", timelineRoleDirectionListening, absent, directionOffset, units, stateUnknown[bool]("unknown-playback"), "", EvidenceUnknown, "unknown-playback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, service, b := runtimeFixture(t)
			listening := stateNode(b, tc.role, 80, stateAbsent[Atomic]())
			if tc.own.State != EvidenceAbsent {
				scalar := Evidence[Atomic]{State: tc.own.State, Reason: tc.own.Reason, Cause: tc.own.Cause}
				if tc.own.State == EvidenceKnown {
					scalar.Value = Atomic{Kind: AtomicDecimal, Decimal: tc.own.Value}
				}
				listening.children = append(listening.children, stateNode(b, timelineRoleListeningOffset, 81, scalar))
			}
			block := runtimeBlock(b, 0)
			base := timelineAt(block, stateKnown(mustRational(t, 2, 1)))
			if err := service.readListeningAt(context.Background(), s, b, listening, block, base, tc.units, tc.direction, tc.playback); err != nil {
				t.Fatal(err)
			}
			got, ok := service.Facts[80]
			if !ok || got.Part != block.part || got.Position.State != tc.state || got.Position.Reason != tc.reason {
				t.Errorf("listening fact=%+v exists=%v", got, ok)
			}
			if tc.state == EvidenceKnown && got.Position.Value.Offset.String() != tc.want {
				t.Errorf("listening position=%s want%s", got.Position.Value.Offset, tc.want)
			}
			if tc.state == EvidenceInvalid && (got.Position.Cause == nil || *got.Position.Cause != *cause) {
				t.Errorf("lost cause=%+v", got.Position)
			}
			if base.Value.Offset.String() != "2" {
				t.Error("listening displacement changed containing cursor")
			}
			if len(service.State.InstrumentChanges) != 0 || len(service.State.InstrumentGaps) != 0 || len(service.State.Play) != 0 {
				t.Error("listening manufactured sound state")
			}
		})
	}
}
