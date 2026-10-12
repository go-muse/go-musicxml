package validation

// These projections are component-local: a known false antecedent is decisive
// even when an unrelated input is unavailable.
func layoutStateCutaway(object, spacing Evidence[bool]) Evidence[bool] {
	if object.State == EvidenceKnown && object.Value {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	if spacing.State == EvidenceKnown && !spacing.Value {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	if object.State != EvidenceKnown {
		return object
	}
	return spacing
}
func layoutStateNoteCategory(size Evidence[string], grace Evidence[bool]) Evidence[string] {
	if size.State != EvidenceKnown {
		return size
	}
	switch size.Value {
	case "full":
		return Evidence[string]{State: EvidenceAbsent}
	case "large", "grace-cue":
		return size
	case "cue":
		if grace.State != EvidenceKnown {
			return stateUnavailable[bool, string](grace)
		}
		if grace.Value {
			return Evidence[string]{State: EvidenceKnown, Value: "grace"}
		}
		return size
	}
	return stateUnknown[string]("note-size-category-unavailable")
}
func layoutStateNot(v Evidence[bool]) Evidence[bool] {
	if v.State == EvidenceKnown {
		v.Value = !v.Value
	}
	return v
}
func layoutStateAnd(a, b Evidence[bool]) Evidence[bool] {
	if a.State == EvidenceKnown && !a.Value {
		return a
	}
	if b.State == EvidenceKnown && !b.Value {
		return b
	}
	if a.State != EvidenceKnown {
		return a
	}
	return b
}
