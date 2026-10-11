package validation

import "errors"

// provedPlanInterruption is produced only after a sealed local/reference
// operator has established its own false consequence. It does not infer proof
// from an arbitrary nonzero Decision returned alongside an error.
type provedPlanInterruption struct {
	node      NodeID
	local     *localPlan
	reference *referencePlan
	decision  Decision
	cause     error
}

func (e *provedPlanInterruption) Error() string { return e.cause.Error() }
func (e *provedPlanInterruption) Unwrap() error { return e.cause }
func planProofStop(n *factNode, local *localPlan, reference *referencePlan, d Decision, cause error) error {
	if cause == nil {
		return nil
	}
	if d.Outcome != OutcomeFail {
		return cause
	}
	return &provedPlanInterruption{node: n.header.ID, local: local, reference: reference, decision: d, cause: cause}
}
func (s *Session) recordProvedPlanInterruption(r RuleDefinition, n *factNode, p Plan, err error) error {
	// Deliberately do not use errors.As: a nested provider's interruption belongs
	// to its own rule and occurrence, never to the consuming reference predicate.
	proof, ok := err.(*provedPlanInterruption)
	if !ok || proof.node != n.header.ID || proof.decision.Outcome != OutcomeFail {
		return err
	}
	if !((p.kind == planLocal && proof.local == p.local && proof.reference == nil) || (p.kind == planTypedReference && proof.reference == p.reference && proof.local == nil)) {
		return err
	}
	// record latches the outcome before diagnostics or dependency-cache storage.
	// If the same exhausted allowance rejects the cache, the count still survives;
	// public session entry points make the interruption terminal before any retry.
	return errors.Join(err, s.record(r, n, proof.decision))
}
func (s *Session) withProvedPlanRelated(n *factNode, local *localPlan, reference *referencePlan, d Decision, locations ...Location) (Decision, error) {
	if err := s.retain(int64(len(locations)) * 128); err != nil {
		return d, planProofStop(n, local, reference, d, err)
	}
	d.Related = make([]Location, len(locations))
	copy(d.Related, locations)
	return d, nil
}
