package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestLayoutRound3TransientStaffSelectionReleased(t *testing.T) {
	measure := localNode(1, 0, "measure")
	r := layoutRound1Reader(t, measure)
	l := layoutScopes{r: r, key: RuleKey{"layout", 1}, staffOverrides: map[NodeID]map[string]layoutStaffBucket{2: {}}, staffGaps: map[NodeID]Evidence[bool]{}}
	b := timelineMeasureBlock{measure: measure, content: measure, part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 2}}}
	before := r.s.retained
	for i := 0; i < 2000*30; i++ {
		if err := l.publishStaffLayouts(b, Evidence[layoutStaffRef]{State: EvidenceAbsent}, layoutFrame{}); err != nil {
			t.Fatal(err)
		}
	}
	if r.s.retained != before {
		t.Errorf("scratch retained: %d -> %d", before, r.s.retained)
	}
}

func TestLayoutRound3StaffApplicabilityUnavailablePrerequisites(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, field := range []string{"first", "number", "system", "part"} {
			t.Run(string(state)+"/"+field, func(t *testing.T) {
				measure := localNode(1, 0, "measure")
				r := layoutRound1Reader(t, measure)
				cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 9}}
				part := Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 2}}
				number := Evidence[Atomic]{State: EvidenceKnown, Value: localInteger(t, 1)}
				first := Evidence[layoutStaffRef]{State: EvidenceKnown, Value: layoutStaffRef{Part: part.Value, Staff: localOne}}
				frame := layoutFrame{system: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 1}}}
				switch field {
				case "first":
					first = Evidence[layoutStaffRef]{State: state, Reason: "fixture-gap", Cause: cause}
				case "number":
					number = Evidence[Atomic]{State: state, Reason: "fixture-gap", Cause: cause}
				case "system":
					frame.system = Evidence[NodeRef]{State: state, Reason: "fixture-gap", Cause: cause}
				case "part":
					part = Evidence[NodeRef]{State: state, Reason: "fixture-gap", Cause: cause}
				}
				l := layoutScopes{r: r, key: RuleKey{"layout", 1}, staffDefaults: map[string]layoutStaffBucket{"1": {Number: number}}, staffOverrides: map[NodeID]map[string]layoutStaffBucket{2: {}}, staffGaps: map[NodeID]Evidence[bool]{}}
				if err := l.publishStaffLayouts(timelineMeasureBlock{measure: measure, content: measure, part: part}, first, frame); err != nil {
					t.Fatal(err)
				}
				if len(r.s.contextResults) != 1 {
					t.Fatalf("rows: %+v", r.s.contextResults)
				}
				got := r.s.contextResults[0].Layout.Applicability
				if got.State != state || got.Cause == nil || *got.Cause != *cause || got.Reason != "fixture-gap" {
					t.Errorf("prerequisite lost: %+v", got)
				}
			})
		}
	}
}

func TestLayoutRound3StaffScratchExactAdmissionAndCleanup(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		for _, limit := range []int64{511, 512, 513, 1023} {
			t.Run(fmt.Sprintf("unknown=%t/limit=%d", unknown, limit), func(t *testing.T) {
				measure := localNode(1, 0, "measure")
				r := layoutRound1Reader(t, measure)
				account, err := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
				if err != nil {
					t.Fatal(err)
				}
				r.s.account = account
				l := layoutScopes{r: r, key: RuleKey{"layout", 1}, staffOverrides: map[NodeID]map[string]layoutStaffBucket{2: {}}, staffGaps: map[NodeID]Evidence[bool]{}}
				part := Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 2}}
				if unknown {
					part = stateUnknown[NodeRef]("part-unavailable")
				}
				err = l.publishStaffLayouts(timelineMeasureBlock{measure: measure, content: measure, part: part}, Evidence[layoutStaffRef]{State: EvidenceAbsent}, layoutFrame{})
				denied := unknown || limit < 512
				if denied && !errors.Is(err, ErrResourceLimit) || !denied && err != nil {
					t.Errorf("admission err=%v denied=%t", err, denied)
				}
				if r.s.retained != 0 || rationalUsage(t, r.s, BudgetBufferedBytes) != 0 {
					t.Errorf("scratch survived return: retained=%d gauge=%d", r.s.retained, rationalUsage(t, r.s, BudgetBufferedBytes))
				}
				r.s.cleanup()
				if rationalUsage(t, r.s, BudgetBufferedBytes) != 0 {
					t.Error("cleanup left scratch")
				}
			})
		}
	}
	for _, unknown := range []bool{false, true} {
		measure := localNode(1, 0, "measure")
		r := layoutRound1Reader(t, measure)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r.ctx = ctx
		l := layoutScopes{r: r, key: RuleKey{"layout", 1}, staffOverrides: map[NodeID]map[string]layoutStaffBucket{2: {}}, staffGaps: map[NodeID]Evidence[bool]{}}
		part := Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 2}}
		if unknown {
			part = stateUnknown[NodeRef]("part-unavailable")
		}
		err := l.publishStaffLayouts(timelineMeasureBlock{measure: measure, content: measure, part: part}, Evidence[layoutStaffRef]{State: EvidenceAbsent}, layoutFrame{})
		if !errors.Is(err, context.Canceled) || r.s.retained != 0 || rationalUsage(t, r.s, BudgetBufferedBytes) != 0 {
			t.Errorf("canceled scratch: err=%v retained=%d gauge=%d", err, r.s.retained, rationalUsage(t, r.s, BudgetBufferedBytes))
		}
	}
}

// Observe the actual publication lifetime rather than only the final gauge.
type layoutRound3ScratchContext struct {
	context.Context
	s                   *Session
	peak, publication   int64
	cancelOnPublication bool
}

func (c *layoutRound3ScratchContext) Err() error {
	used, _ := c.s.account.Usage(BudgetBufferedBytes)
	scratch := used - c.s.retained
	if scratch > c.peak {
		c.peak = scratch
	}
	facts, _ := c.s.account.Usage(BudgetContextFacts)
	if facts > 0 {
		c.publication = scratch
		if c.cancelOnPublication {
			return context.Canceled
		}
	}
	return nil
}

func TestLayoutRound3UnknownPartBucketsLiveThroughPublication(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			measure := localNode(1, 0, "measure")
			print := localNode(2, 1, "print")
			staff := localNode(3, 2, "staff-layout")
			measure.children = []*factNode{print}
			print.children = []*factNode{staff}
			r := layoutRound1Reader(t, measure, print, staff)
			staff.header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextNamedType("staff-layout")})
			r.b.Attributes = map[contextRole]map[ComponentID]AttributeSelector{layoutRole("staff-layout.@number"): {staff.header.Type.Value: localAttr("number")}}
			probe := &layoutRound3ScratchContext{Context: context.Background(), s: r.s, cancelOnPublication: cancel}
			r.ctx = probe
			l := layoutScopes{r: r, key: RuleKey{"layout", 1}, staffOverrides: map[NodeID]map[string]layoutStaffBucket{}, staffGaps: map[NodeID]Evidence[bool]{}}
			part := stateUnknown[NodeRef]("part-unavailable")
			err := l.publishStaffLayouts(timelineMeasureBlock{measure: measure, content: measure, part: part}, Evidence[layoutStaffRef]{State: EvidenceAbsent}, layoutFrame{})
			if cancel && !errors.Is(err, context.Canceled) || !cancel && err != nil {
				t.Fatalf("publication err=%v", err)
			}
			// Empty local map + one sparse bucket (including its string) + keys/order.
			minimum := int64(512 + 768 + 512 + 128)
			if probe.publication < minimum || probe.peak < minimum {
				t.Errorf("scratch released before owned publication: peak=%d publication=%d", probe.peak, probe.publication)
			}
			if got := rationalUsage(t, r.s, BudgetBufferedBytes); got != r.s.retained {
				t.Errorf("leaked scratch: gauge=%d retained=%d", got, r.s.retained)
			}
			if len(l.staffOverrides) != 0 {
				t.Errorf("unknown part cached: %+v", l.staffOverrides)
			}
			if cancel && len(r.s.contextResults) != 0 {
				t.Error("canceled output published")
			}
			r.s.cleanup()
			if got := rationalUsage(t, r.s, BudgetBufferedBytes); got != 0 {
				t.Errorf("cleanup gauge=%d", got)
			}
		})
	}
}

func TestLayoutRound3BlankPageUnavailableConsensus(t *testing.T) {
	absent := Evidence[Atomic]{State: EvidenceAbsent}
	known := Evidence[Atomic]{State: EvidenceKnown, Value: localInteger(t, 1)}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 1}}
		gap := Evidence[Atomic]{State: state, Reason: "blank-unavailable", Cause: cause}
		for _, peer := range []Evidence[Atomic]{absent, known} {
			for _, pair := range [][2]Evidence[Atomic]{{gap, peer}, {peer, gap}} {
				got := layoutBlankPageConsensus(pair[0], pair[1])
				if got.State != state || got.Cause != cause || got.Reason != gap.Reason {
					t.Errorf("causal blank consensus: %+v", got)
				}
			}
		}
	}
	if got := layoutBlankPageConsensus(absent, absent); got.State != EvidenceAbsent {
		t.Errorf("absence fabricated: %+v", got)
	}
	for _, pair := range [][2]Evidence[Atomic]{{absent, known}, {known, absent}} {
		if got := layoutBlankPageConsensus(pair[0], pair[1]); got.State != EvidenceUnknown || got.Cause != nil {
			t.Errorf("absence converted to default/defect: %+v", got)
		}
	}
}

func TestLayoutRound3KnownPartBucketsRemainRetained(t *testing.T) {
	measure := localNode(1, 0, "measure")
	print := localNode(2, 1, "print")
	staff := localNode(3, 2, "staff-layout")
	measure.children = []*factNode{print}
	print.children = []*factNode{staff}
	r := layoutRound1Reader(t, measure, print, staff)
	staff.header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextNamedType("staff-layout")})
	r.b.Attributes = map[contextRole]map[ComponentID]AttributeSelector{layoutRole("staff-layout.@number"): {staff.header.Type.Value: localAttr("number")}}
	probe := &layoutRound3ScratchContext{Context: context.Background(), s: r.s}
	r.ctx = probe
	l := layoutScopes{r: r, key: RuleKey{"layout", 1}, staffOverrides: map[NodeID]map[string]layoutStaffBucket{}, staffGaps: map[NodeID]Evidence[bool]{}}
	part := Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 9}}
	err := l.publishStaffLayouts(timelineMeasureBlock{measure: measure, content: measure, part: part}, Evidence[layoutStaffRef]{State: EvidenceAbsent}, layoutFrame{})
	if err != nil {
		t.Fatal(err)
	}
	bucket := l.staffOverrides[9]["1"]
	if len(bucket.Nodes) != 1 || bucket.Nodes[0] != staff {
		t.Fatalf("persistent bucket lost: %+v", bucket)
	}
	if probe.peak != 640 || probe.publication != 640 {
		t.Errorf("persistent bucket charged as scratch: peak=%d publication=%d", probe.peak, probe.publication)
	}
	if got := rationalUsage(t, r.s, BudgetBufferedBytes); got != r.s.retained || got < 512+768 {
		t.Errorf("persistent charge lost: gauge=%d retained=%d", got, r.s.retained)
	}
}

func TestLayoutRound3BlankParityPreservesStrongerCause(t *testing.T) {
	for _, tc := range []struct{ parity, blank, want EvidenceState }{
		{EvidenceInvalid, EvidenceUnknown, EvidenceInvalid}, {EvidenceUnknown, EvidenceInvalid, EvidenceInvalid},
		{EvidenceUnsupported, EvidenceUnknown, EvidenceUnsupported}, {EvidenceUnknown, EvidenceUnsupported, EvidenceUnsupported},
	} {
		pc := &DecisionRef{Rule: RuleKey{"parity", 1}, Subject: NodeRef{"score", 1}}
		bc := &DecisionRef{Rule: RuleKey{"blank", 1}, Subject: NodeRef{"score", 2}}
		got := layoutBlankPageParity(Evidence[bool]{State: tc.parity, Reason: "parity-gap", Cause: pc}, Evidence[Atomic]{State: tc.blank, Reason: "blank-gap", Cause: bc})
		cause := bc
		if tc.want == tc.parity {
			cause = pc
		}
		if got.State != tc.want || got.Cause != cause {
			t.Errorf("%s/%s lost stronger cause: %+v", tc.parity, tc.blank, got)
		}
	}
}

func TestLayoutRound3UnavailableDividerSelectionHasNoDefaultProvenance(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		t.Run(string(state), func(t *testing.T) {
			measure := localNode(1, 0, "measure")
			system := localNode(2, 0, "system-layout")
			cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 2}}
			system.end.ChildrenComplete = testEvidence(Evidence[bool]{State: state, Reason: "divider-selection-unavailable", Cause: cause})
			r := layoutRound1Reader(t, measure, system)
			l := layoutScopes{r: r, key: RuleKey{"layout", 1}}
			frame := layoutFrame{systemLayout: system, systemCertain: true, first: Evidence[bool]{State: EvidenceKnown, Value: false}, page: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 1}}, system: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 1}}}
			if err := l.publishDividers(timelineMeasureBlock{measure: measure, part: Evidence[NodeRef]{State: EvidenceAbsent}}, frame, nil); err != nil {
				t.Fatal(err)
			}
			if len(r.s.contextResults) != 4 {
				t.Fatalf("divider rows: %+v", r.s.contextResults)
			}
			for _, row := range r.s.contextResults {
				got := row.Layout
				if got.Provenance != ContextExplicit || got.Raw.State != state || got.Raw.Cause == nil || *got.Raw.Cause != *cause || got.Conversion.State != state || got.Conversion.Cause == nil || *got.Conversion.Cause != *cause {
					t.Errorf("selection failure fabricated absence/default: %+v", got)
				}
			}
		})
	}
}
