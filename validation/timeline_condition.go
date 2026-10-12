package validation

// stateConditionalValue projects an encoded statement without selecting a repeat
// traversal. Absence is no statement even when the enclosing sound is conditional.
// Stronger actual scalar/condition failures take precedence over repeat unknown.
// The returned evidence borrows causes; the publication/query owner must copy it.
func stateConditionalValue[T any](value Evidence[T], condition *Evidence[string]) Evidence[T] {
	if value.State == EvidenceAbsent || value.State == "" || condition == nil || condition.State == EvidenceAbsent || condition.State == "" {
		return value
	}
	unavailable := stateUnavailable[string, bool](*condition)
	if condition.State == EvidenceKnown {
		unavailable = stateUnknown[bool]("playback-repeat-pass-unavailable")
	}
	return stateUnavailable[bool, T](stateMergeGap(stateUnavailable[T, bool](value), unavailable))
}

// Only sound statements use this repeat metadata. Initial score-part records
// remain unconditional, and note-local play remains its separately scoped record.
func (r *stateReader) soundCondition(n *factNode) *Evidence[string] {
	if !r.matches(n, timelineRoleSound) && !r.matches(n, timelineRoleDirectionSound) {
		return nil
	}
	condition := stateText(r.attribute(n, timelineRoleSoundTimeOnly))
	if condition.State == EvidenceAbsent {
		return nil
	}
	return &condition
}
