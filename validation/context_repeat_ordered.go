package validation

import "context"

// An ordered match is only a sufficient proof. A mismatch must fall back to the
// existing multiset comparison, which also preserves unavailable-evidence order.
// Beat windows already carry offsets relative to the start of their own window.
func contextRepeatOrderedEqual(ctx context.Context, s *Session, source, target []contextRepeatNote) (bool, error) {
	if err := s.step(ctx, 1); err != nil {
		return false, err
	}
	if len(source) == 0 || len(source) != len(target) {
		return false, nil
	}
	for i := range source {
		equal, err := contextRepeatNoteEqual(ctx, s, source[i], target[i])
		if err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

// The caller selects the two measures without changing either note order. The
// ordered proof avoids a used-mask scan for the common equally ordered case;
// the fallback retains the original multiset and unavailable-evidence semantics.
func contextRepeatSelectedMeasureEqual(ctx context.Context, s *Session, source, target []contextRepeatNote) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	if len(source) == 0 || len(source) != len(target) {
		return stateUnknown[bool]("context-repeat-content-equivalence-unresolved"), nil
	}
	ordered, err := contextRepeatOrderedEqual(ctx, s, source, target)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if ordered {
		return navigationKnownBool(true), nil
	}
	if err = s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(target)), 1)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if _, err = AllocationSize(bytes); err != nil {
		return Evidence[bool]{}, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return Evidence[bool]{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	used := make([]bool, len(target))
	for _, x := range source {
		if err = s.step(ctx, 1); err != nil {
			return Evidence[bool]{}, err
		}
		if x.available.State != EvidenceKnown {
			return x.available, nil
		}
		found := false
		for j, y := range target {
			if err = s.step(ctx, 1); err != nil {
				return Evidence[bool]{}, err
			}
			if used[j] {
				continue
			}
			if y.available.State != EvidenceKnown {
				return y.available, nil
			}
			equal, e := contextRepeatNoteEqual(ctx, s, x, y)
			if e != nil {
				return Evidence[bool]{}, e
			}
			if equal {
				used[j] = true
				found = true
				break
			}
		}
		if !found {
			return stateUnknown[bool]("context-repeat-content-equivalence-unresolved"), nil
		}
	}
	return navigationKnownBool(true), nil
}
