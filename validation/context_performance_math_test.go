package validation

import (
	"context"
	"math/big"
	"testing"
)

func TestPerformanceReferenceVelocityExact(t *testing.T) {
	for _, tc := range []struct {
		input int64
		want  string
	}{{0, "0"}, {100, "90"}, {125, "225/2"}} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		d, _ := NewDecimal(big.NewRat(tc.input, 1))
		got, err := performanceVelocity(context.Background(), s, Evidence[Decimal]{State: EvidenceKnown, Value: d})
		if err != nil || got.State != EvidenceKnown || got.Value.String() != tc.want {
			t.Fatalf("%d: %v %v", tc.input, got, err)
		}
	}
}
func TestPerformanceSwingNominalEligibility(t *testing.T) {
	for _, tc := range []struct {
		typ, active string
		n, d        int64
		grace       bool
		want        bool
	}{{"eighth", "eighth", 1, 2, false, true}, {"eighth", "eighth", 3, 4, false, false}, {"16th", "eighth", 1, 4, false, false}, {"16th", "16th", 1, 4, false, true}, {"eighth", "eighth", 1, 2, true, false}, {"", "eighth", 1, 2, false, false}} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		duration, _ := NewRational(big.NewRat(tc.n, tc.d))
		typ := Evidence[string]{State: EvidenceKnown, Value: tc.typ}
		if tc.typ == "" {
			typ.State = EvidenceAbsent
		}
		got, err := performanceSwingEligible(context.Background(), s, typ, Evidence[string]{State: EvidenceKnown, Value: tc.active}, Evidence[Rational]{State: EvidenceKnown, Value: duration}, Evidence[bool]{State: EvidenceKnown, Value: tc.grace})
		if err != nil || got.State != EvidenceKnown || got.Value != tc.want {
			t.Fatalf("%+v: %v %v", tc, got, err)
		}
	}
}
func TestPerformanceUnknownRemainsCausal(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"xsd", 1}, Subject: NodeRef{"d", 1}}
	got, err := performanceVelocity(context.Background(), s, Evidence[Decimal]{State: EvidenceInvalid, Cause: cause})
	if err != nil || got.State != EvidenceInvalid || got.Cause != cause {
		t.Fatal(got, err)
	}
}

func TestPerformancePayloadOwnershipAndBudget(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"xsd-original", 1}, Subject: NodeRef{"d", 3}}
	p := newTimingContext("note")
	p.Duration = Evidence[Rational]{State: EvidenceInvalid, Cause: cause}
	p.Attack.Raw = Evidence[Decimal]{State: EvidenceInvalid, Cause: cause}
	p.Bends = []TimingBend{{Release: TimingAdjustment{Raw: Evidence[Decimal]{State: EvidenceInvalid, Cause: cause}}}}
	p.Figures = []NodeRef{{"d", 8}}
	p.Tunings = []TimingTuning{{Step: Evidence[string]{State: EvidenceInvalid, Cause: cause}}}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	got, err := cloneTimingContext(context.Background(), s, &p)
	if err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "changed"
	p.Figures[0].Node = 9
	p.Bends[0].Release.Raw.Cause.Rule.ID = "changed-again"
	if got.Duration.Cause.Rule.ID != "xsd-original" || got.Attack.Raw.Cause.Rule.ID != "xsd-original" || got.Bends[0].Release.Raw.Cause.Rule.ID != "xsd-original" || got.Tunings[0].Step.Cause.Rule.ID != "xsd-original" || got.Figures[0].Node != 8 {
		t.Fatal("snapshot aliases provider")
	}
	got.Duration.Cause.Rule.ID = "caller-change"
	if got.Attack.Raw.Cause.Rule.ID != "xsd-original" {
		t.Fatal("fields share mutable causes")
	}
	tight := rationalSession(t, Budgets{MaxBufferedBytes: 1})
	defer tight.cleanup()
	if _, err := cloneTimingContext(context.Background(), tight, &p); err == nil {
		t.Fatal("missing preallocation budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cloneTimingContext(ctx, s, &p); err == nil {
		t.Fatal("missing cancellation")
	}
}
func TestPerformanceMissingCapoAndUnknownUnits(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	d, _ := NewDecimal(big.NewRat(1, 2))
	oct, _ := NewInteger(big.NewInt(4))
	line, _ := NewInteger(big.NewInt(1))
	tuning := staffTuningState{Line: Evidence[Integer]{State: EvidenceKnown, Value: line}, Step: Evidence[string]{State: EvidenceKnown, Value: "E"}, Alter: Evidence[Decimal]{State: EvidenceKnown, Value: d}, Octave: Evidence[Integer]{State: EvidenceKnown, Value: oct}}
	got, err := performanceTuning(context.Background(), s, tuning, Evidence[Integer]{State: EvidenceAbsent})
	if err != nil || got.OpenPitch.Value.String() != "105/2" || got.AdjustedPitch.State != EvidenceUnknown {
		t.Fatal(got, err)
	}
	unknown := Evidence[Rational]{State: EvidenceUnknown, Reason: "divisions-unavailable"}
	eligible, err := performanceSwingEligible(context.Background(), s, Evidence[string]{State: EvidenceKnown, Value: "eighth"}, Evidence[string]{State: EvidenceKnown, Value: "eighth"}, unknown, Evidence[bool]{State: EvidenceKnown})
	if err != nil || eligible.State != EvidenceUnknown || eligible.Reason != "divisions-unavailable" {
		t.Fatal(eligible, err)
	}
}
func TestPerformanceSwingFalseAntecedentDominatesUnknown(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	d, _ := NewRational(big.NewRat(3, 4))
	got, err := performanceSwingEligible(context.Background(), s, Evidence[string]{State: EvidenceKnown, Value: "eighth"}, Evidence[string]{State: EvidenceUnknown, Reason: "active-unknown"}, Evidence[Rational]{State: EvidenceKnown, Value: d}, Evidence[bool]{State: EvidenceUnknown, Reason: "grace-unknown"})
	if err != nil || got.State != EvidenceKnown || got.Value {
		t.Errorf("known nominal mismatch must be ineligible: %+v %v", got, err)
	}
}

func TestPerformanceEveryDefaultEvidenceAxes(t *testing.T) {
	for _, d := range performanceDefaults {
		for _, state := range []EvidenceState{EvidenceAbsent, EvidenceKnown, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
			t.Run(string(d.role)+"/"+string(state), func(t *testing.T) {
				s := rationalSession(t, Budgets{})
				defer s.cleanup()
				s.documentID = "d"
				owner := contextNamedType(d.owner)
				decl := localDecl("perf-attribute")
				attr := AttributeSelector{Name: QName{Local: d.name}, Declaration: decl}
				element := localDecl("perf-element")
				b := &contextBindings{Roles: map[contextRole][]SchemaUse{"perf-root-alter": {{Declaration: element}}}, Attributes: map[contextRole]map[ComponentID]AttributeSelector{d.role: {owner: attr}}}
				n := &factNode{header: NodeHeader{ID: 1, Type: stateKnown(owner), Declaration: stateKnown(element)}, closed: true, end: NodeEnd{AttributesComplete: stateKnown(true), ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)}}
				cause := &DecisionRef{Rule: RuleKey{"test-cause", 1}, Subject: NodeRef{"d", 1}}
				if state != EvidenceAbsent {
					value := Evidence[Atomic]{State: state, Value: d.fallback}
					if state != EvidenceKnown {
						value.Value = Atomic{}
						value.Cause = cause
						value.Reason = "test-field-unavailable"
					}
					n.attributes = []AttributeFact{{Owner: 1, Name: attr.Name, Declaration: stateKnown(decl), Presence: stateKnown(true), Scalar: Scalar{Value: value}}}
				}
				r := &contextReader{ctx: context.Background(), s: s, b: b}
				performanceStatic(r, RuleKey{"test-performance", 1}, n)
				if r.err != nil {
					t.Fatal(r.err)
				}
				if len(s.contextResults) != 1 {
					t.Fatalf("got %d fields", len(s.contextResults))
				}
				v := s.contextResults[0].Value
				want := state
				if state == EvidenceAbsent {
					want = EvidenceKnown
				}
				if v.State != want {
					t.Fatalf("got%s want%s", v.State, want)
				}
				if state != EvidenceAbsent && state != EvidenceKnown && (v.Cause == nil || v.Cause.Rule.ID != "test-cause") {
					t.Fatal("lost unavailable cause")
				}
				if state == EvidenceAbsent && v.Reason != "documented-default" {
					t.Fatal("default provenance missing")
				}
			})
		}
	}
}
func TestPerformanceTimelineProjectionEvidenceAxes(t *testing.T) {
	for _, st := range []EvidenceState{EvidenceKnown, EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		s := rationalSession(t, Budgets{})
		s.documentID = "d"
		cause := &DecisionRef{Rule: RuleKey{"timeline-cause", 1}, Subject: NodeRef{"d", 1}}
		c := cause
		if st == EvidenceKnown || st == EvidenceAbsent {
			c = nil
		}
		pos := Evidence[musicalPosition]{State: st, Cause: c, Value: statePosition(t, 0, 0, 1)}
		rational := Evidence[Rational]{State: st, Cause: c, Value: performanceRatio(1, 1)}
		integer := Evidence[Integer]{State: st, Cause: c, Value: stateOne()}
		decimal := Evidence[Decimal]{State: st, Cause: c, Value: noteTimingBeats}
		ref := Evidence[NodeRef]{State: st, Cause: c, Value: NodeRef{"d", 2}}
		voice := Evidence[string]{State: st, Cause: c, Value: "V"}
		service := &timelineService{Facts: map[NodeID]timelineEventFact{1: {Origin: pos, Position: pos, End: pos, Duration: rational, PrincipalDuration: rational, Principal: ref, Units: decimal, Staff: integer, Voice: voice, Part: ref}}}
		n := &factNode{header: NodeHeader{ID: 1}}
		out, ok, err := performanceTimelineFact(context.Background(), s, service, &performanceAdviceIndex{}, n)
		if err != nil || !ok {
			t.Fatal(ok, err)
		}
		if out.Origin.State != st || out.Duration.State != st || out.Staff.State != st || out.Voice.State != st || out.Units.State != st || out.Principal.State != st {
			t.Errorf("timeline projection changed state%s", st)
		}
		if st == EvidenceUnknown || st == EvidenceUnsupported || st == EvidenceInvalid {
			if err := performanceEffectiveState(context.Background(), s, service, &out); err != nil {
				t.Fatal(err)
			}
			if out.KeyFifths.State != st || out.TimeSymbol.State != st || out.Staves.State != st || out.TranspositionChromatic.State != st || out.EffectiveTempo.State != st {
				t.Errorf("unavailable chronology became omission: state%s key%s time%s staves%s transpose%s tempo%s", st, out.KeyFifths.State, out.TimeSymbol.State, out.Staves.State, out.TranspositionChromatic.State, out.EffectiveTempo.State)
			}
		}
		s.cleanup()
	}
}
func TestPerformanceUnavailableStaffCountRemainsCausal(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	s.documentID = "d"
	service := &timelineService{}
	stateClose(t, s, &service.State, 1)
	pos := statePosition(t, 0, 0, 1)
	cause := &DecisionRef{Rule: RuleKey{"invalid-count", 1}, Subject: NodeRef{"d", 1}}
	if err := publishState(context.Background(), s, &service.State.Staves, statePublication[Integer]{Position: pos, Scope: stateScope{AllStaves: true}, Value: Evidence[Integer]{State: EvidenceInvalid, Cause: cause}}); err != nil {
		t.Fatal(err)
	}
	out := newTimingContext("note")
	out.Origin = performancePosition(stateKnown(pos))
	out.Staff = stateKnown(stateOne())
	if err := performanceEffectiveState(context.Background(), s, service, &out); err != nil {
		t.Fatal(err)
	}
	if out.Staves.State != EvidenceInvalid || out.StaffExists.State != EvidenceInvalid || out.StaffExists.Cause == nil {
		t.Errorf("invalid count became known staff nonexistence: %+v", out.StaffExists)
	}
}
