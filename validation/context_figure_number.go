package validation

import "context"

// The pinned prose requires a number but does not settle a lexical grammar.
// XSD style-text validity is a prerequisite, not proof of numeric conformance.
func evaluateFigureNumberNumeric(ctx context.Context, s *Session, n *factNode) (Decision, bool, error) {
	if !n.closed {
		return Decision{}, true, nil
	}
	value, err := s.localScalar(ctx, n, nil)
	if err != nil {
		return Decision{}, false, err
	}
	if value.State != EvidenceKnown {
		return localGap(value), false, nil
	}
	return Decision{Outcome: OutcomeUnsupported, Reason: "figure-number-numeric-grammar-unreviewed"}, false, nil
}
