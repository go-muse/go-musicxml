package validation

import (
	"context"
	"errors"
	"maps"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"
)

func stafflessPresenceKey(t *testing.T, fifths int64) keyState {
	t.Helper()
	return keyState{Traditional: true, Fifths: stateKnown(stateInt(t, fifths)), Mode: stateAbsent[string](), Cancel: stateAbsent[Integer](), CancelLocation: stateAbsent[string]()}
}

func TestTimelineStafflessKeyPresencePartitions(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"all-staves", "absent", "named-only", "all-named-without-baseline", "different-content", "future-named", "other-part", "invalid-named", "unknown-global-scope", "scoped-gap", "repaired-scoped-gap", "displaced-scoped-gap", "certified-scoped-gap", "certified-without-baseline", "floating-valid", "floating-invalid"} {
		t.Run(mode, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			stateClose(t, s, h, 2)
			all := stateScope{AllStaves: true}
			one, two := stateScope{Staff: stateInt(t, 1)}, stateScope{Staff: stateInt(t, 2)}
			start, at := statePosition(t, 0, 0, 1), statePosition(t, 0, 1, 1)
			statePublish(t, s, &h.Staves, start, all, stateKnown(stateInt(t, 2)), 90)
			want := EvidenceKnown
			cause := &DecisionRef{Rule: RuleKey{"staffless-scope", 1}, Subject: NodeRef{"d", 30}}
			if mode != "absent" && mode != "named-only" && mode != "all-named-without-baseline" && mode != "certified-without-baseline" {
				statePublish(t, s, &h.Keys, start, all, stateKnown(stafflessPresenceKey(t, 0)), 10)
			}
			switch mode {
			case "absent":
				want = EvidenceAbsent
			case "named-only", "all-named-without-baseline":
				want = EvidenceUnknown
				statePublish(t, s, &h.Keys, start, one, stateKnown(stafflessPresenceKey(t, 0)), 20)
				if mode == "all-named-without-baseline" {
					statePublish(t, s, &h.Keys, start, two, stateKnown(stafflessPresenceKey(t, 3)), 21)
				}
			case "different-content":
				statePublish(t, s, &h.Keys, at, two, stateKnown(stafflessPresenceKey(t, 3)), 20)
			case "future-named":
				statePublish(t, s, &h.Keys, statePosition(t, 0, 2, 1), two, Evidence[keyState]{State: EvidenceInvalid, Reason: "bad-key", Cause: cause}, 20)
			case "other-part":
				other := start
				other.Part.Node = 2
				statePublish(t, s, &h.Keys, other, two, Evidence[keyState]{State: EvidenceInvalid, Reason: "bad-key", Cause: cause}, 20)
			case "invalid-named":
				want = EvidenceInvalid
				statePublish(t, s, &h.Keys, at, two, Evidence[keyState]{State: EvidenceInvalid, Reason: "bad-key", Cause: cause}, 20)
			case "unknown-global-scope", "scoped-gap", "repaired-scoped-gap", "displaced-scoped-gap", "certified-scoped-gap", "certified-without-baseline":
				want = EvidenceUnsupported
				scope := two
				if mode == "unknown-global-scope" {
					scope = all
				}
				gap := stateGap{Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: scope, Availability: Evidence[bool]{State: EvidenceUnsupported, Reason: "key-unavailable", Cause: cause}}
				if mode == "repaired-scoped-gap" {
					gap.Position = &start
				}
				if mode == "displaced-scoped-gap" {
					gap.FirstMeasure = 1
					gap.LastMeasure = 1
					gap.Placement.Displaced = true
				}
				if err := addStateGap(ctx, s, &h.Keys, gap); err != nil {
					t.Fatal(err)
				}
				if mode == "repaired-scoped-gap" {
					statePublish(t, s, &h.Keys, at, two, stateKnown(stafflessPresenceKey(t, 3)), 20)
					want = EvidenceKnown
				}
				if mode == "certified-scoped-gap" || mode == "certified-without-baseline" {
					if err := h.certifyKeyPresenceGap(ctx, s, 0, NodeRef{"d", 30}); err != nil {
						t.Fatal(err)
					}
					if mode == "certified-scoped-gap" {
						want = EvidenceKnown
					}
				}
			case "floating-valid", "floating-invalid":
				floating := statePosition(t, 1, -1, 1)
				floating.UnprovedFrame = true
				value := stateKnown(stafflessPresenceKey(t, 3))
				if mode == "floating-invalid" {
					value = Evidence[keyState]{State: EvidenceInvalid, Reason: "bad-key", Cause: cause}
					want = EvidenceInvalid
				}
				statePublish(t, s, &h.Keys, floating, two, value, 20)
			}
			got, err := h.effectiveKeyPresence(ctx, s, at, stateAbsent[Integer]())
			if err != nil || got.Value.State != want || want == EvidenceKnown && !got.Value.Value {
				t.Fatalf("presence=%+v err=%v; want %s", got, err, want)
			}
			if want == EvidenceInvalid || want == EvidenceUnsupported {
				if got.Value.Cause == nil || !reflect.DeepEqual(got.Value.Cause, cause) || got.Value.Cause == cause {
					t.Fatalf("causal ownership lost: %+v", got)
				}
			}
			if mode == "different-content" || mode == "certified-scoped-gap" || mode == "floating-valid" {
				if len(got.Provenance) != 2 || got.Inherited || !got.InheritanceUnproved {
					t.Fatalf("consensus witnesses/inheritance lost: %+v", got)
				}
				got.Provenance[0].Node.Node = 999
				again, err := h.effectiveKeyPresence(ctx, s, at, stateAbsent[Integer]())
				if err != nil || again.Provenance[0].Node.Node == 999 {
					t.Fatalf("result aliases history: %+v %v", again, err)
				}
			}
			whole, err := h.effectiveKey(ctx, s, at, stateAbsent[Integer]())
			if err != nil || whole.Value.State != EvidenceUnknown || whole.Value.Reason != "timeline-staff-unresolved" {
				t.Fatalf("concrete key guard weakened: %+v %v", whole, err)
			}
		})
	}
}

func stafflessResourceFixture(t *testing.T) (*Session, *timelineState, musicalPosition) {
	t.Helper()
	s, h, at := independentKeyPresenceFixture(t, 1)
	two := stateScope{Staff: stateInt(t, 2)}
	statePublish(t, s, &h.Keys, at, two, stateKnown(stafflessPresenceKey(t, 3)), 20)
	gap := stateGap{Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: two, Availability: Evidence[bool]{State: EvidenceUnknown, Reason: "scoped-position-unavailable"}}
	if err := addStateGap(context.Background(), s, &h.Keys, gap); err != nil {
		t.Fatal(err)
	}
	if err := h.certifyKeyPresenceGap(context.Background(), s, 1, NodeRef{"d", 30}); err != nil {
		t.Fatal(err)
	}
	if _, err := stateIndex(context.Background(), s, &h.Keys); err != nil {
		t.Fatal(err)
	}
	return s, h, at
}

func TestTimelineStafflessKeyPresenceCancellationAndOwnership(t *testing.T) {
	s, h, at := stafflessResourceFixture(t)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	got, err := h.queryKeyPresence(probe, s, at, stateUnknown[Integer]("timeline-staff-unresolved"))
	if err != nil || got.Value.State != EvidenceKnown || !got.Value.Value || len(got.Provenance) != 4 {
		t.Fatalf("presence=%+v %v", got, err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("successful query leaked scratch")
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s, h, at := stafflessResourceFixture(t)
		index := h.Keys.index
		publications, gaps, certificates := slices.Clone(h.Keys.Publications), stateClone(h.Keys.Gaps), maps.Clone(h.keyPresenceGaps)
		for i := range publications {
			publications[i].Value = stateClone(publications[i].Value)
			publications[i].Provenance = stateClone(publications[i].Provenance)
		}
		beforeFacts := rationalUsage(t, s, BudgetContextFacts)
		_, err := h.queryKeyPresence(&rationalCancelContext{Context: context.Background(), at: poll}, s, at, stateUnknown[Integer]("timeline-staff-unresolved"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d did not cancel: %v", poll, err)
		}
		if h.Keys.index != index || !reflect.DeepEqual(publications, h.Keys.Publications) || !reflect.DeepEqual(gaps, h.Keys.Gaps) || !reflect.DeepEqual(certificates, h.keyPresenceGaps) || rationalUsage(t, s, BudgetContextFacts) != beforeFacts || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("poll %d mutated history or leaked scratch", poll)
		}
		got, err := h.queryKeyPresence(context.Background(), s, at, stateUnknown[Integer]("timeline-staff-unresolved"))
		if err != nil || got.Value.State != EvidenceKnown || !got.Value.Value {
			t.Fatalf("poll %d retry failed: %+v %v", poll, got, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll %d cleanup leaked", poll)
		}
	}
	t.Logf("all %d staffless consensus cancellation polls", probe.calls)
}

func TestTimelineStafflessKeyPresenceResourceAdmission(t *testing.T) {
	if need := 4*int64(unsafe.Sizeof("")+unsafe.Sizeof(struct{}{})) + int64(unsafe.Sizeof(Integer{})); need > timelineKeyPresenceScopeScratchBytes {
		t.Fatalf("scope map and slice bound too small: %d", need)
	}
	s, h, at := stafflessResourceFixture(t)
	defer s.cleanup()
	scratch := int64(len(h.Keys.Publications)+len(h.Keys.Gaps))*timelineKeyPresenceScopeScratchBytes + timelineKeyPresenceScratchFixedBytes
	held := s.account.Limits().MaxBufferedBytes - rationalUsage(t, s, BudgetBufferedBytes) - scratch + 1
	if err := s.account.Charge(BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
	_, err := h.queryKeyPresence(context.Background(), s, at, stateUnknown[Integer]("timeline-staff-unresolved"))
	var resource *ResourceError
	if !errors.As(err, &resource) || resource.Kind != BudgetBufferedBytes || resource.Requested != scratch {
		t.Fatalf("scope scratch was not admitted before allocation: %v; want %d", err, scratch)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained+held {
		t.Fatal("failed query leaked scratch")
	}
	if err := s.account.Release(BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
	// A caller-sized document identifier is charged before scope comparison,
	// including a comparison that would immediately reject the different part.
	at.Part.DocumentID = strings.Repeat("document", 4096)
	heldSteps := s.account.Limits().MaxRuleSteps - rationalUsage(t, s, BudgetRuleSteps) - 100
	if err := s.account.Charge(BudgetRuleSteps, heldSteps); err != nil {
		t.Fatal(err)
	}
	_, err = h.queryKeyPresence(context.Background(), s, at, stateUnknown[Integer]("timeline-staff-unresolved"))
	if !errors.As(err, &resource) || resource.Kind != BudgetRuleSteps || resource.Requested < int64(len(at.Part.DocumentID)) {
		t.Fatalf("unadmitted document comparison: %v", err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("work-limited query leaked scratch")
	}
}

func TestTimelineStafflessKeyPresenceCountIndependentWork(t *testing.T) {
	var costs []int64
	for _, count := range []string{"2", strings.Repeat("9", 4096)} {
		s, h, at := stafflessResourceFixture(t)
		number, ok := new(big.Int).SetString(count, 10)
		if !ok {
			t.Fatal("invalid test count")
		}
		value, err := NewInteger(number)
		if err != nil {
			t.Fatal(err)
		}
		statePublish(t, s, &h.Staves, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(value), 90)
		// The count-resolution cost depends on encoded bytes. The presence query
		// itself must not consult, convert, or enumerate the numerical count.
		before := rationalUsage(t, s, BudgetRuleSteps)
		got, err := h.queryKeyPresence(context.Background(), s, at, stateUnknown[Integer]("timeline-staff-unresolved"))
		if err != nil || got.Value.State != EvidenceKnown || !got.Value.Value {
			t.Fatalf("count %d bytes: %+v %v", len(count), got, err)
		}
		costs = append(costs, rationalUsage(t, s, BudgetRuleSteps)-before)
		s.cleanup()
	}
	if costs[0] != costs[1] {
		t.Fatalf("key presence consulted encoded count: %v", costs)
	}
	t.Logf("2 and 4096-digit staves counts both use %d presence-query steps", costs[0])
}

func TestTimelineStafflessKeyPresenceKeepsUnknownScopeCause(t *testing.T) {
	for _, status := range []EvidenceState{EvidenceUnknown, EvidenceInvalid} {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			s, svc, b := runtimeFixture(t)
			initialKey := stateNode(b, timelineRoleAttributesKey, 22, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, 23, stateAtomicInteger(t, 0)))
			attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)), initialKey, stateNode(b, timelineRoleAttributesStaves, 24, stateAtomicInteger(t, 2)))
			cause := &DecisionRef{Rule: RuleKey{"staff-number", 1}, Subject: NodeRef{"d", 42}}
			key := stateNode(b, timelineRoleAttributesKey, 42, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, 43, stateAtomicInteger(t, 3)))
			stateAttribute(b, key, timelineRoleKeyNumber, Atomic{Kind: AtomicInteger, Integer: stateInt(t, 2)})
			key.attributes[0].Scalar.Value = Evidence[Atomic]{State: status, Reason: "number-unavailable", Cause: cause}
			changed := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), key)
			blocks := []timelineMeasureBlock{runtimeBlock(b, 0, attrs, runtimeNote(t, b, 30, 1, false), changed, runtimeNote(t, b, 50, 1, false))}
			if err := svc.readMeasure(ctx, s, b, blocks[0]); err != nil {
				t.Fatal(err)
			}
			if err := svc.finalizeTimingOrigins(ctx, s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got, err := svc.State.effectiveKeyPresence(ctx, s, svc.Facts[50].Position.Value, stateAbsent[Integer]())
			if err != nil || got.Value.State != status || got.Value.Cause == nil || *got.Value.Cause != *cause {
				t.Fatalf("uncertain key number lost its global cause: %+v %v", got, err)
			}
		})
	}
}

func TestTimelineStafflessKeyPresenceStaffIdentityAdmission(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	number, ok := new(big.Int).SetString(strings.Repeat("9", 4096), 10)
	if !ok {
		t.Fatal("invalid test number")
	}
	staff, err := NewInteger(number)
	if err != nil {
		t.Fatal(err)
	}
	statePublish(t, s, &h.Keys, at, stateScope{Staff: staff}, stateKnown(stafflessPresenceKey(t, 0)), 10)
	held := s.account.Limits().MaxRuleSteps - rationalUsage(t, s, BudgetRuleSteps) - 100
	if err := s.account.Charge(BudgetRuleSteps, held); err != nil {
		t.Fatal(err)
	}
	_, err = h.queryKeyPresence(context.Background(), s, at, stateUnknown[Integer]("timeline-staff-unresolved"))
	var resource *ResourceError
	if !errors.As(err, &resource) || resource.Kind != BudgetRuleSteps || resource.Requested != staff.ByteSize()+1 {
		t.Fatalf("sparse staff key hash lacked integer admission: %v", err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("integer-limited query leaked scratch")
	}
}

func TestTimelineStafflessKeyPresenceKeepsUnavailableStaffCause(t *testing.T) {
	s, h, at := stafflessResourceFixture(t)
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"harmony-staff", 1}, Subject: NodeRef{"d", 70}}
	for _, status := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceUnsupported} {
		staff := Evidence[Integer]{State: status, Reason: "staff-unavailable", Cause: cause}
		got, err := h.effectiveKeyPresence(context.Background(), s, at, staff)
		if err != nil || got.Value.State != status || got.Value.Cause == nil || *got.Value.Cause != *cause || got.Value.Cause == cause {
			t.Fatalf("explicit unavailable staff was erased: %+v %v", got, err)
		}
	}
}
