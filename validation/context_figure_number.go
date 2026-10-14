package validation

import "context"

// This legacy plan is retained for explicit caller compatibility only. MusicXML
// deliberately retained unrestricted figure-number text after considering an
// integer restriction; built-in conformance profiles do not select this plan.
// https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290221733
// Its historical ASCII-digit / Unsupported outcomes do not define MusicXML's
// lexical domain, and do not make other style-text nonconforming.
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
