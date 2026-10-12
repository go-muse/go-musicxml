package validation

import "context"

// The pinned prose requires a number but does not settle a lexical grammar.
// XSD style-text validity is a prerequisite, not proof of numeric conformance.
// Nonempty ASCII digits are a sufficient number representation (the official
// figure-number example uses 6); other forms keep the open grammar gate.
// https://www.w3.org/2021/06/musicxml40/musicxml-reference/examples/figure-number-element/
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
	if value.Value.Kind == AtomicString && len(value.Value.Text) > 0 {
		if err := s.step(ctx, int64(len(value.Value.Text))); err != nil {
			return Decision{}, false, err
		}
		numeric := true
		for i := 0; i < len(value.Value.Text); i++ {
			if value.Value.Text[i] < '0' || value.Value.Text[i] > '9' {
				numeric = false
				break
			}
		}
		if numeric {
			return Decision{Outcome: OutcomePass}, false, nil
		}
	}
	return Decision{Outcome: OutcomeUnsupported, Reason: "figure-number-numeric-grammar-unreviewed"}, false, nil
}
