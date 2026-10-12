package validation

import (
	"context"
	"testing"
	"unsafe"
)

func TestPlaybackOwnedRecordAllowanceCoversWidthsAndGrowth(t *testing.T) {
	// Large map values are indirect; growth retains pointers to the same fixed
	// value. Include conservative per-entry map/table growth. Variable Play
	// and Technique backing is admitted separately by its construction helpers.
	for _, tc := range []struct {
		name    string
		width   uintptr
		reserve int64
	}{
		{"note", unsafe.Sizeof(notePlaybackContext{}), playbackNoteReserveBytes},
		{"sound", unsafe.Sizeof(soundPlaybackContext{}), playbackSoundReserveBytes},
		{"barline", unsafe.Sizeof(barlinePlaybackContext{}), playbackBarlineReserveBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("width=%d reserve=%d", tc.width, tc.reserve)
			if tc.reserve < int64(tc.width)+512 {
				t.Fatalf("record/growth allowance too small: width=%d reserve=%d", tc.width, tc.reserve)
			}
		})
	}
	if playbackMapReserveBytes < 256 {
		t.Fatal("initial map header/group admission too small")
	}
	if unsafe.Sizeof(playbackInstruction{}) > 512 {
		t.Fatal("playback instruction exceeds separately admitted slot")
	}
	if 2*unsafe.Sizeof(playTechnique{})+64 > 512 {
		t.Fatal("play technique and overlapping append growth exceed admitted slot")
	}
	if unsafe.Sizeof(DecisionRef{}) > 128 {
		t.Fatal("causal record exceeds ownOrigin admission")
	}
}
func TestPlaybackOwnedRecordDeepCausesWithoutBorrowedVectors(t *testing.T) {
	s, p, b, block, fact := playbackFixture(t)
	inputCause := &DecisionRef{Rule: RuleKey{ID: "ipa", Revision: 1}, Subject: NodeRef{"d", 23}}
	bad := Evidence[Atomic]{State: EvidenceInvalid, Reason: "ipa-invalid", Cause: inputCause}
	child := stateNode(b, timelineRolePlayIpa, 23, bad)
	play := stateNode(b, timelineRoleNotePlay, 22, stateAbsent[Atomic](), child)
	n := stateNode(b, timelineRoleNote, 20, stateAbsent[Atomic](), play)
	if err := p.readNote(context.Background(), s, b, n, block, fact); err != nil {
		t.Fatal(err)
	}
	inputCause.Subject.Node = 999
	first := p.Notes[20]
	if first.Play[0].Value.Value.Techniques[0].Value.Cause.Subject.Node != 23 {
		t.Fatal("nested input cause borrowed")
	}
	// A second read of the same source child must construct independent vectors.
	inputCause.Subject.Node = 23
	n.header.ID = 30
	if err := p.readNote(context.Background(), s, b, n, block, fact); err != nil {
		t.Fatal(err)
	}
	first.Play[0].Value.Value.Techniques[0].Value.Cause.Subject.Node = 998
	first.Play[0].Value.Value.Techniques[0].Kind = playOther
	if p.Notes[30].Play[0].Value.Value.Techniques[0].Value.Cause.Subject.Node != 23 || p.Notes[30].Play[0].Value.Value.Techniques[0].Kind != playIPA {
		t.Fatal("records share mutable vector or nested cause")
	}
}

func TestPlaybackOwnedRecordConcurrentScratchAdmission(t *testing.T) {
	for _, delta := range []int64{-1, 0} {
		t.Run(map[int64]string{-1: "below-peak", 0: "at-peak"}[delta], func(t *testing.T) {
			s := rationalSession(t, Budgets{MaxBufferedBytes: 2*playbackNoteReserveBytes + playbackMapReserveBytes + delta})
			defer s.cleanup()
			var records map[NodeID]notePlaybackContext
			err := playbackStore(context.Background(), s, &records, 1, notePlaybackContext{})
			if delta < 0 {
				if err == nil || len(records) != 0 {
					t.Fatalf("concurrent temporary+map value was not admitted: err=%v records=%d", err, len(records))
				}
			} else {
				if err != nil || len(records) != 1 {
					t.Fatalf("exact peak failed: %v", err)
				}
				if got := rationalUsage(t, s, BudgetBufferedBytes); got != playbackNoteReserveBytes+playbackMapReserveBytes {
					t.Fatalf("scratch not released: %d", got)
				}
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("failed publication leaked scratch")
			}
		})
	}
}
