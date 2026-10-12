package validation

import (
	"context"
	"testing"
)

// The pinned MusicXML 4.0 sound annotation applies time-only to the sound
// element, including its instrument-change/MIDI/device children. A statement
// query without a repeat-pass argument cannot establish an unconditional value.
func TestTimelineFinalReviewConditionalMIDIDevice(t *testing.T) {
	for _, conditional := range []bool{false, true} {
		t.Run(map[bool]string{false: "unconditional", true: "time-only"}[conditional], func(t *testing.T) {
			s, svc, b := runtimeFixture(t)
			device := stateNode(b, timelineRoleSoundMidiDevice, 31, stateAtomicText("second-pass-device"))
			sound := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic](), device)
			if conditional {
				stateAttribute(b, sound, timelineRoleSoundTimeOnly, Atomic{Kind: AtomicString, Text: "2"})
			}
			attr := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
			if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, attr, sound)); err != nil {
				t.Fatal(err)
			}
			if len(svc.State.MIDIDevices) != 1 {
				t.Fatalf("missing raw record: %+v", svc.State.MIDIDevices)
			}
			raw := svc.State.MIDIDevices[0]
			if raw.Value.State != EvidenceKnown || raw.Value.Value.Name.Value != "second-pass-device" {
				t.Fatalf("lost raw value: %+v", raw)
			}
			got, err := svc.State.effectiveMIDIDeviceStatements(context.Background(), s, svc.Facts[30].Position.Value, NodeRef{"d", 71})
			if err != nil {
				t.Fatal(err)
			}
			if conditional && got.Name.Value.State == EvidenceKnown {
				t.Fatalf("conditional sound became unconditional device: %+v", got)
			}
			if !conditional && (got.Name.Value.State != EvidenceKnown || got.Name.Value.Value != "second-pass-device") {
				t.Fatalf("unconditional device unavailable: %+v", got)
			}
		})
	}
}
