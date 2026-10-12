package validation

// localConstantRule is a private configuration-time specification. Element and
// attribute strings are resolved into exact declaration identities before use;
// no input document is selected by a runtime local-name comparison.
type localConstantRule struct {
	Element, Type, Attribute, AttributeType, Child, ChildType string
	Property, Default, Unknown                                string
	Kind                                                      AtomicKind
	Layout, Empty                                             bool
}

// The table deliberately omits conditional defaults handled by typed local
// projections (note printing, lyric visibility, halign and notation size).
var localConstantRules = []localConstantRule{
	{Attribute: "version", AttributeType: "xs:token", Property: "score-version", Default: "1.0", Kind: AtomicString},
	{Attribute: "winged", AttributeType: "winged", Property: "winged", Default: "none", Kind: AtomicString},
	{Attribute: "show-frets", AttributeType: "show-frets", Property: "show-frets", Default: "numbers", Kind: AtomicString},
	{Attribute: "non-controlling", AttributeType: "yes-no", Property: "non-controlling", Default: "no", Kind: AtomicString},
	{Type: "horizontal-turn", Attribute: "slash", AttributeType: "yes-no", Property: "horizontal-turn-slash", Default: "no", Kind: AtomicString},
	{Element: "multiple-rest", Type: "multiple-rest", Attribute: "use-symbols", AttributeType: "yes-no", Property: "multiple-rest-use-symbols", Default: "no", Kind: AtomicString},
	{Element: "bass-alter", Type: "harmony-alter", Attribute: "location", AttributeType: "left-right", Property: "bass-alter-location", Default: "right", Kind: AtomicString},
	{Element: "numeral-alter", Type: "harmony-alter", Attribute: "location", AttributeType: "left-right", Property: "numeral-alter-location", Default: "left", Kind: AtomicString},
	{Element: "degree-alter", Type: "degree-alter", Attribute: "plus-minus", AttributeType: "yes-no", Property: "degree-plus-minus", Default: "no", Kind: AtomicString},
	{Element: "frame", Type: "frame", Child: "first-fret", ChildType: "first-fret", Property: "first-fret", Default: "1", Kind: AtomicInteger},
	{Element: "metronome", Type: "metronome", Attribute: "parentheses", AttributeType: "yes-no", Property: "metronome-parentheses", Default: "no", Kind: AtomicString},
	{Element: "accidental", Type: "accidental", Attribute: "editorial", AttributeType: "yes-no", Property: "accidental-editorial", Default: "no", Kind: AtomicString},
	{Element: "accidental", Type: "accidental", Attribute: "cautionary", AttributeType: "yes-no", Property: "accidental-cautionary", Default: "no", Kind: AtomicString},
	{Element: "heel", Type: "heel-toe", Attribute: "substitution", AttributeType: "yes-no", Property: "heel-substitution", Default: "no", Kind: AtomicString},
	{Element: "toe", Type: "heel-toe", Attribute: "substitution", AttributeType: "yes-no", Property: "toe-substitution", Default: "no", Kind: AtomicString},
	{Element: "hole", Type: "hole", Child: "hole-shape", ChildType: "xs:string", Property: "hole-shape", Default: "circle", Kind: AtomicString},
	{Element: "notehead", Type: "notehead", Attribute: "parentheses", AttributeType: "yes-no", Property: "notehead-parentheses", Default: "no", Kind: AtomicString},
	{Element: "strong-accent", Type: "strong-accent", Attribute: "type", AttributeType: "up-down", Property: "strong-accent-type", Default: "up", Kind: AtomicString},
	{Element: "caesura", Type: "caesura", Property: "empty-caesura", Default: "normal", Kind: AtomicString, Empty: true},
	{Element: "fermata", Type: "fermata", Property: "fermata-shape", Default: "normal", Kind: AtomicString, Empty: true},
	{Element: "fermata", Type: "fermata", Attribute: "type", AttributeType: "upright-inverted", Property: "fermata-orientation", Default: "upright", Kind: AtomicString},
	{Element: "stopped", Type: "empty-placement-smufl", Attribute: "smufl", AttributeType: "smufl-glyph-name", Property: "stopped-smufl", Default: "brassMuteClosed", Kind: AtomicString},
	{Element: "open", Type: "empty-placement-smufl", Attribute: "smufl", AttributeType: "smufl-glyph-name", Property: "open-smufl", Default: "brassMuteOpen", Kind: AtomicString},
	{Element: "half-muted", Type: "empty-placement-smufl", Attribute: "smufl", AttributeType: "smufl-glyph-name", Property: "half-muted-smufl", Default: "brassMuteHalfClosed", Kind: AtomicString},
}
