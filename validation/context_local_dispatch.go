package validation

// This fixed dispatch vocabulary is derived from the same exact binding specs
// as the provider. Input names never select a semantic branch.
type localFamilyMask uint8

const (
	localPrintFamily localFamilyMask = 1 << iota
	localStructuralFamily
	localGlyphFamily
	localAdvisoryFamily
)

var localDispatchFamilies = func() map[contextRole]localFamilyMask {
	out := make(map[contextRole]localFamilyMask)
	add := func(specs []contextBindingSpec, mask localFamilyMask) {
		for _, s := range specs {
			out[s.Role] |= mask
		}
	}
	add(localStructuralBindingSpecs(), localStructuralFamily)
	add(contextGlyphBindingSpecs(), localGlyphFamily)
	add(localAdvisoryBindingSpecs(), localAdvisoryFamily)
	for _, r := range []contextRole{localKind, localClefSign, localCreator, localEncoder, localText} {
		out[r] |= localAdvisoryFamily
	}
	for _, r := range []contextRole{localNote, localFiguredBass, localLyric, localNoteType, localNotehead, localPrintObject} {
		out[r] |= localPrintFamily
	}
	out[localNotehead] |= localGlyphFamily
	return out
}()
var localDispatchConstants = func() map[contextRole]int {
	out := make(map[contextRole]int)
	for i, s := range localConstantRules {
		if s.Layout {
			continue
		}
		role := localRuleBindings[i].attribute
		if s.Element != "" {
			role = localRuleBindings[i].element
		}
		out[role] = i
	}
	return out
}()
