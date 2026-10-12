package validation

import "context"

// contextTimeOnlyOrder accepts an already XSD-assessed, whitespace-normalized
// time-only scalar. It adds only numerical order: grammar remains the shared
// XSD prerequisite, and equal passes retain the separately reviewed policy gate.
// Canonical positive decimal digit strings compare exactly by length then digits;
// this has no machine-integer bound and allocates no pass vector or big integers.
func contextTimeOnlyOrder(ctx context.Context, s *Session, text string) (Decision, error) {
	work, err := rationalCheckedMul(BudgetRuleSteps, int64(len(text)), 3)
	if err != nil {
		return Decision{}, err
	}
	if err = s.step(ctx, work); err != nil {
		return Decision{}, err
	}
	unsupported := Decision{Outcome: OutcomeUnsupported, Reason: "time-only-normalized-domain-unavailable"}
	if len(text) == 0 {
		return unsupported, nil
	}
	// Defensively verify the full admitted domain before retaining any order proof.
	// In normal assessment malformed values are blocked by the authoritative XSD
	// result, not by this guard and never by a duplicate lexical failure here.
	for i := 0; i < len(text); {
		if text[i] < '1' || text[i] > '9' {
			return unsupported, nil
		}
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			if i&255 == 0 {
				if err := ctx.Err(); err != nil {
					return Decision{}, err
				}
			}
			i++
		}
		if i == len(text) {
			break
		}
		if text[i] != ',' {
			return unsupported, nil
		}
		i++
		if i < len(text) && text[i] == ' ' {
			i++
		}
		if i == len(text) {
			return unsupported, nil
		}
	}
	previousStart, previousEnd := -1, -1
	equal := false
	for i := 0; i < len(text); {
		if err := ctx.Err(); err != nil {
			return Decision{}, err
		}
		start := i
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			if i&255 == 0 {
				if err := ctx.Err(); err != nil {
					return Decision{}, err
				}
			}
			i++
		}
		if previousStart >= 0 {
			previousLength, currentLength := previousEnd-previousStart, i-start
			comparison := 0
			if previousLength > currentLength {
				comparison = 1
			} else if previousLength < currentLength {
				comparison = -1
			} else {
				for j := 0; j < currentLength; j++ {
					if j&255 == 0 {
						if err := ctx.Err(); err != nil {
							return Decision{}, err
						}
					}
					a, b := text[previousStart+j], text[start+j]
					if a > b {
						comparison = 1
						break
					}
					if a < b {
						comparison = -1
						break
					}
				}
			}
			if comparison > 0 {
				return Decision{Outcome: OutcomeFail, Reason: "time-only-descending-passes", Expected: "Numerically ascending pass numbers", Actual: "A pass number exceeds the following pass number"}, nil
			}
			equal = equal || comparison == 0
		}
		previousStart, previousEnd = start, i
		if i < len(text) {
			i++
			if i < len(text) && text[i] == ' ' {
				i++
			}
		}
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
