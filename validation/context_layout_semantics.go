package validation

// layoutSelectorMatch preserves exact token strings; it neither parses lyric
// numbers as integers nor assigns specificity to competing partial selectors.
func layoutSelectorMatch(name, number, lyricName, lyricNumber Evidence[Atomic]) Evidence[bool] {
	result := Evidence[bool]{State: EvidenceKnown, Value: true}
	for _, pair := range [][2]Evidence[Atomic]{{name, lyricName}, {number, lyricNumber}} {
		a, b := pair[0], pair[1]
		if a.State == EvidenceAbsent {
			continue
		}
		if a.State != EvidenceKnown {
			result = stateUnavailable[Atomic, bool](a)
			continue
		}
		if b.State == EvidenceAbsent {
			return Evidence[bool]{State: EvidenceKnown, Value: false}
		}
		if b.State != EvidenceKnown {
			result = stateUnavailable[Atomic, bool](b)
			continue
		}
		if a.Value.Text != b.Value.Text {
			return Evidence[bool]{State: EvidenceKnown, Value: false}
		}
	}
	return result
}
func layoutConsensus(values []Evidence[Atomic], closed bool) Evidence[Atomic] {
	if !closed {
		return stateUnknown[Atomic]("layout-publisher-enumeration-unavailable")
	}
	result := Evidence[Atomic]{State: EvidenceAbsent}
	for _, value := range values {
		result = layoutConsensusPair(result, value)
	}
	return result
}
func layoutConsensusPair(a, b Evidence[Atomic]) Evidence[Atomic] {
	if b.State == EvidenceAbsent {
		return a
	}
	if a.State == EvidenceAbsent {
		return b
	}
	if a.State != EvidenceKnown {
		return a
	}
	if b.State != EvidenceKnown {
		return b
	}
	// Font/language publishers have string values except numeric font-size.
	if a.Value.Kind != b.Value.Kind || a.Value.Text != b.Value.Text || a.Value.Integer.String() != b.Value.Integer.String() || a.Value.Decimal.String() != b.Value.Decimal.String() {
		return stateUnknown[Atomic]("layout-competing-publishers")
	}
	return a
}
func layoutDefault(raw Evidence[Atomic], value string) Evidence[Atomic] {
	if raw.State == EvidenceAbsent {
		return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: value}}
	}
	return raw
}
func layoutUnknownDefault(raw Evidence[Atomic], reason Code) Evidence[Atomic] {
	if raw.State == EvidenceAbsent {
		return stateUnknown[Atomic](reason)
	}
	return raw
}
