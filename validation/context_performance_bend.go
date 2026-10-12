package validation

import "context"

type performanceBendProof struct {
	Applicable, Satisfied Evidence[bool]
	Prior                 Evidence[NodeRef]
}
type contextPerformanceService struct {
	Bends map[NodeID]performanceBendProof
}

func performanceBendRelation(r *contextReader, n *factNode) performanceBendProof {
	out := performanceBendProof{Applicable: stateUnknown[bool]("bend-gesture-unestablished"), Satisfied: stateUnknown[bool]("bend-gesture-unestablished"), Prior: stateUnknown[NodeRef]("bend-gesture-unestablished")}
	release, c := r.child(n, "perf-release")
	present := performancePresence(release, c)
	if present.State != EvidenceKnown {
		out.Applicable = present
		out.Satisfied = present
		return out
	}
	if !present.Value {
		out.Applicable = Evidence[bool]{State: EvidenceKnown, Value: false}
		out.Satisfied = Evidence[bool]{State: EvidenceAbsent}
		return out
	}
	parent := r.s.nodes[n.header.Parent]
	if parent == nil {
		return out
	}
	complete := r.enumerated(parent)
	if complete.State != EvidenceKnown || !complete.Value {
		out.Applicable = complete
		return out
	}
	var first, second *factNode
	count := 0
	for _, ch := range parent.children {
		if !r.step(1) {
			return out
		}
		if ch.header.Declaration.State != EvidenceKnown {
			out.Applicable = stateUnavailable[DeclarationID, bool](ch.header.Declaration)
			return out
		}
		if r.matches(ch, "perf-bend") {
			count++
			if count == 1 {
				first = ch
			}
			if count == 2 {
				second = ch
			}
		}
	}
	if count != 2 || second != n {
		return out
	}
	_, a := r.child(first, "perf-release")
	_, b := r.child(first, "perf-pre-bend")
	if a.State != EvidenceAbsent || b.State != EvidenceAbsent {
		return out
	}
	alter := stateDecimalValue(r.childScalar(first, "perf-bend-alter"))
	if alter.State != EvidenceKnown {
		out.Applicable = stateUnavailable[Decimal, bool](alter)
		return out
	}
	if alter.Value.value.Sign() <= 0 {
		return out
	}
	out.Applicable = Evidence[bool]{State: EvidenceKnown, Value: true}
	out.Prior = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, first.header.ID}}
	value := stateDecimalValue(r.childScalar(n, "perf-bend-alter"))
	out.Satisfied = stateUnavailable[Decimal, bool](value)
	if value.State == EvidenceKnown {
		if !r.step(value.Value.ByteSize()) {
			return out
		}
		out.Satisfied = Evidence[bool]{State: EvidenceKnown, Value: value.Value.value.Sign() < 0}
	}
	return out
}
func storePerformanceBend(r *contextReader, key RuleKey, n *factNode, p performanceBendProof) {
	if r.err != nil {
		return
	}
	service := r.s.contextServices[key]
	if service == nil {
		r.err = contract("bend context", "provider service required")
		return
	}
	if !r.step(1) {
		return
	}
	if r.err = r.s.retain(1024); r.err != nil {
		return
	}
	if r.err = stateRetainCopy(r.ctx, r.s, p); r.err != nil {
		return
	}
	if service.Performance == nil {
		service.Performance = &contextPerformanceService{Bends: map[NodeID]performanceBendProof{}}
	}
	p.Applicable = cloneEvidence(p.Applicable)
	p.Satisfied = cloneEvidence(p.Satisfied)
	p.Prior = cloneEvidence(p.Prior)
	service.Performance.Bends[n.header.ID] = p
}
func evaluateBendReleaseSign(ctx context.Context, s *Session, n *factNode, p *contextPlan, final bool) (Decision, bool, error) {
	if err := s.step(ctx, 1); err != nil {
		return Decision{}, false, err
	}
	service := s.contextServices[p.Provider]
	if service == nil || !service.Final {
		if !final {
			return Decision{}, true, nil
		}
		return Decision{Outcome: OutcomeUnsupported, Reason: "performance-service-unavailable"}, false, nil
	}
	if service.Performance == nil {
		return Decision{Outcome: OutcomeUnknown, Reason: "bend-gesture-unestablished"}, false, nil
	}
	proof, ok := service.Performance.Bends[n.header.ID]
	if !ok {
		return Decision{Outcome: OutcomeNotApplicable, Reason: "bend-role-mismatch"}, false, nil
	}
	if proof.Applicable.State != EvidenceKnown {
		return localGap(proof.Applicable), false, nil
	}
	if !proof.Applicable.Value {
		return Decision{Outcome: OutcomeNotApplicable, Reason: "bend-release-condition-false"}, false, nil
	}
	if proof.Satisfied.State != EvidenceKnown {
		return localGap(proof.Satisfied), false, nil
	}
	if proof.Satisfied.Value {
		return Decision{Outcome: OutcomePass}, false, nil
	}
	return Decision{Outcome: OutcomeFail, Reason: "bend-release-must-be-negative", Expected: "negative semitone displacement for an established release after a bend", Actual: "nonnegative bend-alter"}, false, nil
}
