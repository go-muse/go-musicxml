package validation

import (
	"context"
	"testing"
	"unsafe"
)

func TestPlaybackOwnedRecordAllowanceCoversWidthsAndGrowth(t *testing.T) {
	// Include two simultaneous fixed values (the newly assembled record and map's
	// indirect value) plus conservative per-entry map/table growth. Variable Play
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
			if tc.reserve < int64(2*tc.width)+512 {
				t.Fatalf("record/growth allowance too small: width=%d reserve=%d", tc.width, tc.reserve)
			}
		})
	}
	if playbackMapReserveBytes < 512 {
		t.Fatal("initial map header/group admission too small")
	}
	if unsafe.Sizeof(playbackInstruction{}) > 512 {
		t.Fatal("playback instruction exceeds separately admitted slot")
	}
	if unsafe.Sizeof(playTechnique{}) > 512 {
		t.Fatal("play technique exceeds separately admitted slot")
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
