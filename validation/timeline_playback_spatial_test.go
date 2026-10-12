package validation

import (
	"context"
	"errors"
	"testing"
)

func playbackSpatialFixture(t *testing.T) (*Session, soundPlaybackContext, Evidence[NodeRef], Evidence[instrumentRecord[midiState]]) {
	t.Helper()
	s, p, b, block, fact := playbackFixture(t)
	sound := stateNode(b, timelineRoleSound, 50, stateAbsent[Atomic]())
	stateAttribute(b, sound, timelineRoleSoundPan, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, -20, 1)})
	stateAttribute(b, sound, timelineRoleSoundElevation, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 10, 1)})
	if err := p.readSound(context.Background(), s, b, sound, block, fact); err != nil {
		t.Fatal(err)
	}
	child := stateNode(b, timelineRoleSoundMidiInstrument, 51, stateAbsent[Atomic]())
	child.header.Parent = 50
	s.nodes = map[NodeID]*factNode{50: sound, 51: child}
	instrument := stateKnown(NodeRef{"d", 80})
	midi := stateKnown(instrumentRecord[midiState]{Source: NodeRef{"d", 51}, Part: fact.Part.Value, Position: fact.Position, Instrument: instrument, Value: stateKnown(midiState{Pan: stateKnown(stateDecimal(t, 30, 1)), Elevation: stateAbsent[Decimal]()})})
	return s, p.Sounds[50], instrument, midi
}
func TestPlaybackSpatialMIDIFieldPrecedenceAndLegacyFallback(t *testing.T) {
	s, sound, instrument, midi := playbackSpatialFixture(t)
	if sound.Pan.Value.String() != "-20" || sound.Elevation.Value.String() != "10" {
		t.Fatalf("legacy raw fields absent: %+v", sound)
	}
	got, err := sound.spatial(context.Background(), s, instrument, midi)
	if err != nil || got.Pan.Value.Value.String() != "30" || got.Pan.Provenance[0].Node.Node != 51 || got.Elevation.Value.Value.String() != "10" || got.Elevation.Provenance[0].Node.Node != 50 {
		t.Fatalf("per-field precedence %+v,%v", got, err)
	}
	absent, err := sound.spatial(context.Background(), s, stateAbsent[NodeRef](), stateAbsent[instrumentRecord[midiState]]())
	if err != nil || absent.Pan.Value.Value.String() != "-20" {
		t.Fatalf("known absent MIDI fallback %+v,%v", absent, err)
	}
}
func TestPlaybackSpatialUnknownAndInvalidDoNotFallBack(t *testing.T) {
	s, sound, instrument, midi := playbackSpatialFixture(t)
	cause := &DecisionRef{Rule: RuleKey{ID: "pan", Revision: 1}, Subject: NodeRef{"d", 51}}
	midi.Value.Value.Value.Pan = Evidence[Decimal]{State: EvidenceInvalid, Reason: "pan-invalid", Cause: cause}
	got, err := sound.spatial(context.Background(), s, instrument, midi)
	if err != nil || got.Pan.Value.State != EvidenceInvalid || got.Pan.Value.Cause.Subject.Node != 51 || got.Elevation.Value.Value.String() != "10" {
		t.Fatalf("invalid field fallback %+v,%v", got, err)
	}
	cause.Subject.Node = 999
	if got.Pan.Value.Cause.Subject.Node != 51 {
		t.Fatal("borrowed result cause")
	}
	unknown, err := sound.spatial(context.Background(), s, instrument, stateUnknown[instrumentRecord[midiState]]("midi-enumeration-unknown"))
	if err != nil || unknown.Pan.Value.State != EvidenceUnknown || unknown.Elevation.Value.State != EvidenceUnknown {
		t.Fatalf("unknown MIDI fallback %+v,%v", unknown, err)
	}
}
func TestPlaybackSpatialRequiresExactInstrumentPartAndParent(t *testing.T) {
	for _, kind := range []string{"instrument", "part", "parent", "missing-parent"} {
		t.Run(kind, func(t *testing.T) {
			s, sound, instrument, midi := playbackSpatialFixture(t)
			switch kind {
			case "instrument":
				midi.Value.Instrument.Value.Node = 81
			case "part":
				midi.Value.Part.Node = 2
			case "parent":
				s.nodes[51].header.Parent = 90
			case "missing-parent":
				delete(s.nodes, 51)
			}
			got, err := sound.spatial(context.Background(), s, instrument, midi)
			if kind == "missing-parent" {
				if err != nil || got.Pan.Value.State != EvidenceUnknown {
					t.Fatalf("missing parent proof %+v,%v", got, err)
				}
			} else if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("accepted mismatched %s: %+v,%v", kind, got, err)
			}
		})
	}
}
func TestPlaybackSpatialCancellationAndRetentionAdmission(t *testing.T) {
	for _, kind := range []string{"cancel", "retention"} {
		t.Run(kind, func(t *testing.T) {
			s, sound, instrument, midi := playbackSpatialFixture(t)
			ctx := context.Background()
			if kind == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			} else {
				used := rationalUsage(t, s, BudgetBufferedBytes)
				if err := s.account.Charge(BudgetBufferedBytes, s.account.Limits().MaxBufferedBytes-used); err != nil {
					t.Fatal(err)
				}
			}
			_, err := sound.spatial(ctx, s, instrument, midi)
			if kind == "cancel" && !errors.Is(err, context.Canceled) || kind == "retention" && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("admission %v", err)
			}
			// Directly acquired test bytes are released outside Session cleanup.
			if kind == "retention" {
				used := rationalUsage(t, s, BudgetBufferedBytes)
				if err := s.account.Release(BudgetBufferedBytes, used-s.retained); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
