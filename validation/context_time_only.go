package validation

import "context"

// contextTimeOnlyOrder accepts an already XSD-assessed, whitespace-normalized
// time-only scalar. It adds only numerical order: grammar remains the shared
// XSD prerequisite, and equal passes retain the separately reviewed policy gate.
// Canonical positive decimal digit strings compare exactly by length then digits;
// this has no machine-integer bound and allocates no pass vector or big integers.
// contextScanPassSet validates the full normalized positive-decimal list.
// Work is prepaid and every scan/comparison polls at the same 256-byte cadence.
// Callers may collect a result, but must not publish it until valid is true.
func contextScanPassSet(ctx context.Context, s *Session, text string, visit func(string) error) (bool, error) {
	work, err := rationalCheckedMul(BudgetRuleSteps, int64(len(text)), 3)
	if err != nil {
		return false, err
	}
	if err = s.step(ctx, work); err != nil {
		return false, err
	}
	if len(text) == 0 {
		return false, nil
	}
	for start := 0; start < len(text); {
		if err = s.step(ctx, 0); err != nil {
			return false, err
		}
		if text[start] < '1' || text[start] > '9' {
			return false, nil
		}
		end := start
		for end < len(text) && text[end] >= '0' && text[end] <= '9' {
			if end&255 == 0 {
				if err = s.step(ctx, 0); err != nil {
					return false, err
				}
			}
			end++
		}
		if err = visit(text[start:end]); err != nil {
			return false, err
		}
		if end == len(text) {
			return true, nil
		}
		if text[end] != ',' {
			return false, nil
		}
		start = end + 1
		if start < len(text) && text[start] == ' ' {
			start++
		}
		if start == len(text) {
			return false, nil
		}
	}
	return false, nil
}
func contextTimeOnlyOrder(ctx context.Context, s *Session, text string) (Decision, error) {
	previous := ""
	equal, descending := false, false
	valid, err := contextScanPassSet(ctx, s, text, func(item string) error {
		if previous != "" {
			comparison := 0
			if len(previous) > len(item) {
				comparison = 1
			} else if len(previous) < len(item) {
				comparison = -1
			} else {
				for j := 0; j < len(item); j++ {
					if j&255 == 0 {
						if err := s.step(ctx, 0); err != nil {
							return err
						}
					}
					if previous[j] > item[j] {
						comparison = 1
						break
					}
					if previous[j] < item[j] {
						comparison = -1
						break
					}
				}
			}
			descending = descending || comparison > 0
			equal = equal || comparison == 0
		}
		previous = item
		return nil
	})
	if err != nil {
		return Decision{}, err
	}
	if !valid {
		return Decision{Outcome: OutcomeUnsupported, Reason: "time-only-normalized-domain-unavailable"}, nil
	}
	if descending {
		return Decision{Outcome: OutcomeFail, Reason: "time-only-descending-passes", Expected: "Numerically ascending pass numbers", Actual: "A pass number exceeds the following pass number"}, nil
	}
	if equal {
		return Decision{Outcome: OutcomeUnknown, Reason: "time-only-duplicate-policy-unresolved"}, nil
	}
	return Decision{Outcome: OutcomePass}, nil
}

// evaluateTimeOnlyOrder is a node-local predicate. It deliberately has no
// timeline or repeat service dependency; XSD supplies the exact attribute slot
// and normalized token, while this plan supplies only the extra numeric order.
func evaluateTimeOnlyOrder(ctx context.Context, s *Session, n *factNode, b *contextBindings) (Decision, bool, error) {
	if !n.closed {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: b}
	value := r.attribute(n, contextRoleTimeOnly)
	if r.err != nil {
		return Decision{}, false, r.err
	}
	if value.State == EvidenceAbsent {
		return Decision{Outcome: OutcomeNotApplicable, Reason: "time-only-absent"}, false, nil
	}
	if value.State != EvidenceKnown {
		return localGap(value), false, nil
	}
	if value.Value.Kind != AtomicString {
		return Decision{Outcome: OutcomeUnsupported, Reason: "time-only-string-domain-unavailable"}, false, nil
	}
	decision, err := contextTimeOnlyOrder(ctx, s, value.Value.Text)
	return decision, false, err
}
