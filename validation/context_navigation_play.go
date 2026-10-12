package validation

import "context"

// NavigationPlayInstruction retains pass-qualified applicability independently
// of the raw technique list. Multiple applicable techniques are not flattened
// into an application-specific hardware configuration.
type NavigationPlayInstruction struct {
	Instruction TimingPlayInstruction
	Active      Evidence[bool]
}

func navigationApplyPlay(ctx context.Context, s *Session, current []NavigationPlayInstruction, updates []TimingPlayInstruction, guard Evidence[bool]) ([]NavigationPlayInstruction, []NavigationPlayInstruction, error) {
	var local []NavigationPlayInstruction
	for _, p := range updates {
		if err := s.step(ctx, 1); err != nil {
			return nil, nil, err
		}
		active := guard
		if active.State == EvidenceKnown && !active.Value {
			continue
		}
		item := NavigationPlayInstruction{Instruction: p, Active: active}
		item.Instruction.Value = navigationConditionalValue(item.Instruction.Value, active)
		if _, err := stateReserve(ctx, s, item); err != nil {
			return nil, nil, err
		}
		if p.Scope == TimingPlayCurrentNote {
			local = append(local, stateClone(item))
			continue
		}
		found := false
		for i := range current {
			if err := s.step(ctx, 1); err != nil {
				return nil, nil, err
			}
			if current[i].Instruction.Source == p.Source {
				current[i] = stateClone(item)
				found = true
				break
			}
		}
		if !found {
			current = append(current, stateClone(item))
		}
	}
	return current, local, nil
}
