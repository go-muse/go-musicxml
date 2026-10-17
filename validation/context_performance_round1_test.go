package validation

import (
	"context"
	"errors"
	"testing"
)

func TestPerformanceRound1RatioRejectsNonpositiveKnownDenominator(t *testing.T) {
	for _, value := range []int64{0, -1} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		ratio, reduced, err := performanceSwingRatio(context.Background(), s, stateKnown(stateInt(t, 3)), stateKnown(stateInt(t, value)))
		if err != nil || ratio.State != EvidenceUnknown || reduced.State != EvidenceUnknown || ratio.Reason != "swing-ratio-nonpositive" {
			t.Errorf("second=%d: ratio=%+v reduced=%+v err=%v", value, ratio, reduced, err)
		}
	}
}
func TestPerformanceRound1MissingEventAndDeclarationStayCausal(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	key := RuleKey{"performance", 1}
	s.contextServices = map[RuleKey]*contextService{key: {Final: true, Timeline: stateKnown(true)}}
	out := newTimingContext("note")
	performanceUnavailableEvent(s, key, &out)
	if out.Position.State != EvidenceUnknown || out.Position.Reason != "event-position-unavailable" {
		t.Errorf("missing event=%+v", out.Position)
	}
	cause := &DecisionRef{Rule: RuleKey{"declaration", 1}, Subject: NodeRef{"d", 2}}
	n := localNode(2, 0, "bend")
	n.header.Declaration = testEvidence(Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "invalid-declaration", Cause: cause})
	for _, populated := range []bool{false, true} {
		if populated {
			s.contextServices[key].Performance = &contextPerformanceService{Bends: map[NodeID]performanceBendProof{99: {}}}
		}
		d, _, err := evaluateBendReleaseSign(context.Background(), s, n, &contextPlan{Provider: key}, true)
		if err != nil || d.Outcome != OutcomeBlocked || d.Reason != "invalid-declaration" || d.Cause == nil || *d.Cause != *cause {
			t.Errorf("populated=%v: %+v %v", populated, d, err)
		}
	}
}
func TestPerformanceRound1LocalStatePreservesFirstError(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	sentinel := errors.New("first reader interruption")
	r := &contextReader{ctx: context.Background(), s: s, err: sentinel}
	sr := &stateReader{ctx: context.Background(), s: s, b: &timelineBindings{}}
	out := newTimingContext("note")
	count := int64(0)
	performanceLocalState(r, sr, localNode(1, 0, "other"), &out, &count)
	if r.err != sentinel {
		t.Errorf("first error lost: %v", r.err)
	}
}

func TestPerformanceRound1MovementLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
		b := &contextBindings{Roles: map[contextRole][]SchemaUse{"perf-backup": {{Declaration: localDecl("backup")}}}}
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		for i := 0; i < count; i++ {
			pos := statePosition(t, uint64(i), 1, 1)
			service.State.Divisions.Publications = append(service.State.Divisions.Publications, statePublication[Decimal]{Position: pos, Value: stateKnown(stateDecimal(t, 1, 1))})
			service.Facts[NodeID(i+100)] = timelineEventFact{Origin: stateKnown(pos)}
		}
		index, err := buildPerformanceAdviceIndex(context.Background(), s, service)
		if err != nil {
			t.Fatal(err)
		}
		defer index.release(s)
		for i := 0; i < count; i++ {
			out := newTimingContext("backup")
			out.Origin = performancePosition(stateKnown(statePosition(t, uint64(i), 1, 1)))
			out.Duration = stateKnown(performanceRatio(1, 1))
			out.Units = stateKnown(stateDecimal(t, 1, 1))
			if !performanceMovement(r, service, index, localNode(NodeID(i+100), 0, "backup"), &out) || r.err != nil {
				t.Fatal(r.err)
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("movement100=%d movement1000=%d", small, large)
	if large > 15*small {
		t.Errorf("quadratic movement lookup %d > 15 * %d", large, small)
	}
}
func TestPerformanceRound1SoundSpatialLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		s.documentID = "d"
		s.nodes = map[NodeID]*factNode{}
		service := &timelineService{}
		service.Playback.Sounds = map[NodeID]soundPlaybackContext{}
		for i := 0; i < count; i++ {
			id := NodeID(i + 100)
			service.Playback.Sounds[id] = soundPlaybackContext{Origin: playbackOrigin{Source: NodeRef{"d", id}, Part: stateKnown(NodeRef{"d", 1})}, Pan: stateAbsent[Decimal](), Elevation: stateAbsent[Decimal]()}
			node := localNode(id+NodeID(count), id, "midi")
			s.nodes[node.header.ID] = node
			service.State.MIDI = append(service.State.MIDI, instrumentRecord[midiState]{Source: NodeRef{"d", node.header.ID}, Part: NodeRef{"d", 1}, Instrument: stateKnown(NodeRef{"d", 2}), Value: stateKnown(midiState{Pan: stateAbsent[Decimal](), Elevation: stateAbsent[Decimal]()})})
		}
		index, err := buildPerformanceAdviceIndex(context.Background(), s, service)
		if err != nil {
			t.Fatal(err)
		}
		defer index.release(s)
		for i := 0; i < count; i++ {
			out := newTimingContext("sound")
			if err := performanceSoundSpatial(context.Background(), s, service, index, localNode(NodeID(i+100), 0, "sound"), &out); err != nil {
				t.Fatal(err)
			}
			if len(out.Spatial) != 1 {
				t.Fatal(len(out.Spatial))
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("spatial100=%d spatial1000=%d", small, large)
	if large > 15*small {
		t.Errorf("quadratic parent lookup %d > 15 * %d", large, small)
	}
}

func TestPerformanceRound1MissingBendEvidenceAxes(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	key := RuleKey{"performance", 1}
	s.contextServices = map[RuleKey]*contextService{key: {Final: true, Performance: &contextPerformanceService{Bends: map[NodeID]performanceBendProof{99: {}}}}}
	for _, tc := range []struct {
		state EvidenceState
		want  Outcome
	}{{EvidenceKnown, OutcomeUnknown}, {EvidenceAbsent, OutcomeUnknown}, {EvidenceUnknown, OutcomeUnknown}, {EvidenceUnsupported, OutcomeUnsupported}, {EvidenceInvalid, OutcomeBlocked}} {
		n := localNode(2, 0, "bend")
		cause := &DecisionRef{Rule: RuleKey{"declaration", 1}, Subject: NodeRef{"d", 2}}
		n.header.Declaration.State = tc.state
		n.header.Declaration.Reason = "declaration-unavailable"
		if tc.state == EvidenceUnknown || tc.state == EvidenceUnsupported || tc.state == EvidenceInvalid {
			n.header.Declaration.Cause = cause
		}
		d, _, err := evaluateBendReleaseSign(context.Background(), s, n, &contextPlan{Provider: key}, true)
		if err != nil || d.Outcome != tc.want {
			t.Errorf("state=%s outcome=%+v err=%v", tc.state, d, err)
		}
		if n.header.Declaration.Cause != nil && (d.Cause == nil || *d.Cause != *cause) {
			t.Errorf("state=%s lost cause: %+v", tc.state, d)
		}
	}
}

func TestPerformanceRound1SwingRatioPreservesUnavailableAndInterruptions(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"ratio-input", 1}, Subject: NodeRef{"d", 4}}
	for _, st := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		s := rationalSession(t, Budgets{})
		raw := Evidence[Integer]{State: st, Reason: "ratio-input-unavailable", Cause: cause}
		got, advice, err := performanceSwingRatio(context.Background(), s, stateKnown(stateInt(t, 2)), raw)
		if err != nil || got.State != st || advice.State != st || got.Reason != raw.Reason || got.Cause != cause || advice.Cause != cause {
			t.Errorf("state=%s got=%+v advice=%+v err=%v", st, got, advice, err)
		}
		s.cleanup()
	}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := performanceSwingRatio(ctx, s, stateKnown(stateInt(t, 2)), stateKnown(stateInt(t, 0))); !errors.Is(err, context.Canceled) {
		t.Errorf("zero denominator hid cancellation: %v", err)
	}
	tight := rationalSession(t, Budgets{MaxRuleSteps: 1})
	defer tight.cleanup()
	if _, _, err := performanceSwingRatio(context.Background(), tight, stateKnown(stateInt(t, 2)), stateKnown(stateInt(t, 0))); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("zero denominator hid quota refusal: %v", err)
	}
}

func TestPerformanceRound1ProjectionHelpersPreserveUnavailableCauses(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"local-state", 1}, Subject: NodeRef{"d", 2}}
	for _, st := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		out := newTimingContext("projection")
		key := Evidence[keyState]{State: st, Reason: "key-gap", Cause: cause}
		tim := Evidence[timeState]{State: st, Reason: "time-gap", Cause: cause}
		trans := Evidence[transposeState]{State: st, Reason: "transpose-gap", Cause: cause}
		details := Evidence[staffDetailsState]{State: st, Reason: "details-gap", Cause: cause}
		for _, err := range []error{performanceProjectKey(context.Background(), s, &out, key), performanceProjectTime(context.Background(), s, &out, tim), performanceProjectTranspose(context.Background(), s, &out, trans), performanceProjectStaffDetails(context.Background(), s, &out, details)} {
			if err != nil {
				t.Fatal(err)
			}
		}
		if out.KeyFifths.State != st || out.KeyFifths.Cause != cause || out.TimeSymbol.State != st || out.TimeSymbol.Cause != cause || out.ChromaticDisplacement.State != st || out.ChromaticDisplacement.Cause != cause || out.Capo.State != st || out.Capo.Cause != cause {
			t.Errorf("projection replaced %s evidence", st)
		}
	}
}
