package validation

import (
	"context"
	"testing"
)

func performanceRound1SwingFixture(t *testing.T) (*contextReader, *timelineService, *factNode) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	t.Cleanup(s.cleanup)
	s.documentID = "d"
	s.nodes = map[NodeID]*factNode{}
	b := &contextBindings{Roles: map[contextRole][]SchemaUse{}}
	for _, name := range []string{"sound", "swing", "straight", "type", "swing-type", "swing-first", "swing-second"} {
		b.Roles[contextRole("perf-"+name)] = []SchemaUse{{Declaration: localDecl(name)}}
	}
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	node := func(id, parent NodeID, role string) *factNode {
		n := localNode(id, parent, role)
		s.nodes[id] = n
		s.sequence = append(s.sequence, id)
		if p := s.nodes[parent]; p != nil {
			p.children = append(p.children, n)
		}
		return n
	}
	node(10, 0, "sound")
	node(11, 10, "swing")
	node(12, 11, "straight")
	node(20, 0, "sound")
	node(21, 20, "swing")
	node(22, 21, "straight")
	note := node(30, 0, "note")
	typ := node(31, 30, "type")
	typ.end.Scalar.Value = stateKnown(localString("eighth"))
	service := &timelineService{Final: true}
	service.Playback.Sounds = map[NodeID]soundPlaybackContext{
		10: {Origin: playbackOrigin{Part: stateKnown(NodeRef{"d", 1}), MeasureOrdinal: 3, Position: stateUnknown[musicalPosition]("unplaced-sound")}, TimeOnly: stateAbsent[string]()},
		20: {Origin: playbackOrigin{Part: stateKnown(NodeRef{"d", 1}), MeasureOrdinal: 10, Position: stateKnown(statePosition(t, 10, 0, 1))}, TimeOnly: stateAbsent[string]()},
	}
	stateClose(t, s, &service.State, 20)
	return r, service, note
}
func TestPerformanceRound1SwingPointGapRepairAndUnconditionalGuard(t *testing.T) {
	r, service, note := performanceRound1SwingFixture(t)
	h, err := performanceBuildSwing(r, service)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.history.Gaps) != 1 || h.history.Gaps[0].FirstMeasure != 3 || h.history.Gaps[0].LastMeasure != 3 {
		t.Errorf("not a point gap: %+v", h.history.Gaps)
	}
	if len(h.history.Publications) != 1 || h.history.Publications[0].TimeOnly != nil {
		t.Errorf("unconditional publication carries guard: %+v", h.history.Publications)
	}
	for _, tc := range []struct {
		ordinal uint64
		state   EvidenceState
	}{{4, EvidenceUnknown}, {11, EvidenceKnown}} {
		out := newTimingContext("note")
		out.Origin = performancePosition(stateKnown(statePosition(t, tc.ordinal, 0, 1)))
		out.Duration = stateKnown(performanceRatio(1, 2))
		out.Grace = stateKnown(false)
		performanceNoteSwing(r, service, &h, note, &out)
		if r.err != nil || out.SwingEligible.State != tc.state || out.SwingEligible.Value {
			t.Errorf("measure=%d eligible=%+v err=%v", tc.ordinal, out.SwingEligible, r.err)
		}
	}
}
func TestPerformanceRound1SwingConditionalGuardPreserved(t *testing.T) {
	r, service, _ := performanceRound1SwingFixture(t)
	sound := service.Playback.Sounds[20]
	sound.TimeOnly = stateKnown("2")
	service.Playback.Sounds[20] = sound
	h, err := performanceBuildSwing(r, service)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.history.Publications) != 1 || h.history.Publications[0].TimeOnly == nil || h.history.Publications[0].TimeOnly.Value != "2" {
		t.Fatal("conditional guard lost")
	}
}
