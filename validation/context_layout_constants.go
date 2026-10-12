package validation

var layoutConstantRules = []localConstantRule{
	{Attribute: "implicit", AttributeType: "yes-no", Property: "implicit-measure", Default: "no", Kind: AtomicString},
	{Attribute: "valign", AttributeType: "valign-image", Property: "image-valign", Unknown: "application-image-alignment"},
	{Element: "level", Type: "level", Attribute: "parentheses", AttributeType: "yes-no", Property: "editorial-parentheses", Unknown: "application-editorial-parentheses"},
	{Element: "level", Type: "level", Attribute: "bracket", AttributeType: "yes-no", Property: "editorial-bracket", Unknown: "application-editorial-bracket"},
	{Element: "level", Type: "level", Attribute: "size", AttributeType: "symbol-size", Property: "editorial-size", Unknown: "application-editorial-size"},
	{Element: "accidental", Type: "accidental", Attribute: "parentheses", AttributeType: "yes-no", Property: "editorial-parentheses", Unknown: "application-editorial-parentheses"},
	{Element: "accidental", Type: "accidental", Attribute: "bracket", AttributeType: "yes-no", Property: "editorial-bracket", Unknown: "application-editorial-bracket"},
	{Element: "accidental", Type: "accidental", Attribute: "size", AttributeType: "symbol-size", Property: "editorial-size", Unknown: "application-editorial-size"},

	{Element: "frame", Type: "frame", Attribute: "unplayed", AttributeType: "xs:token", Property: "unplayed-string", Unknown: "application-unplayed-string"},
	{Element: "kind", Type: "kind", Attribute: "stack-degrees", AttributeType: "yes-no", Property: "kind-stack-degrees", Unknown: "application-kind-degrees"},
	{Element: "kind", Type: "kind", Attribute: "parentheses-degrees", AttributeType: "yes-no", Property: "kind-parentheses-degrees", Unknown: "application-kind-degrees"},
	{Element: "kind", Type: "kind", Attribute: "bracket-degrees", AttributeType: "yes-no", Property: "kind-bracket-degrees", Unknown: "application-kind-degrees"},
	{Element: "kind", Type: "kind", Attribute: "text", AttributeType: "xs:token", Property: "kind-text", Unknown: "application-kind-text"},
	{Element: "kind", Type: "kind", Attribute: "use-symbols", AttributeType: "yes-no", Property: "kind-use-symbols", Unknown: "application-kind-symbols"},
	{Element: "numeral-root", Type: "numeral-root", Attribute: "text", AttributeType: "xs:token", Property: "numeral-root-text", Unknown: "application-numeral-root"},
	{Element: "breath-mark", Type: "breath-mark", Property: "breath-mark-symbol", Unknown: "application-breath-mark", Empty: true},
	{Element: "credit", Type: "credit", Attribute: "page", AttributeType: "xs:positiveInteger", Property: "credit-page", Default: "1", Kind: AtomicInteger},
}
