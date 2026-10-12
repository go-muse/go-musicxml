package validation

// Raw absence is the only evidence state that permits a documented default.
// This helper does not own its result; session publication owns evidence.
func localDefault[T any](raw Evidence[T], fallback T) Evidence[T] {
	if raw.State == EvidenceAbsent {
		return Evidence[T]{State: EvidenceKnown, Value: fallback}
	}
	return raw
}
func localInheritedBool(raw, parent Evidence[bool]) Evidence[bool] {
	if raw.State == EvidenceAbsent {
		return parent
	}
	return raw
}

type localPrintValues struct{ Object, Dot, Lyric, Leger, LegerActive Evidence[bool] }

func localPrinting(object, dot, lyric, leger, rest Evidence[bool]) localPrintValues {
	object = localDefault(object, true)
	out := localPrintValues{Object: object, Dot: localInheritedBool(dot, object), Lyric: localInheritedBool(lyric, object), Leger: localInheritedBool(leger, object), LegerActive: rest}
	if rest.State == EvidenceKnown {
		out.LegerActive.Value = !rest.Value
	}
	return out
}
func localNoteSize(raw Evidence[string], grace, cue Evidence[bool]) Evidence[string] {
	if raw.State != EvidenceAbsent {
		return raw
	}
	if grace.State != EvidenceKnown {
		return stateUnavailable[bool, string](grace)
	}
	if cue.State != EvidenceKnown {
		return stateUnavailable[bool, string](cue)
	}
	value := "full"
	if grace.Value && cue.Value {
		value = "grace-cue"
	} else if grace.Value || cue.Value {
		value = "cue"
	}
	return Evidence[string]{State: EvidenceKnown, Value: value}
}
