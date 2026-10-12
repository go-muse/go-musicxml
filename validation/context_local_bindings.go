package validation

const (
	localNote         contextRole = "local.note"
	localLyric        contextRole = "local.lyric"
	localGrace        contextRole = "local.grace"
	localCue          contextRole = "local.cue"
	localRest         contextRole = "local.rest"
	localDot          contextRole = "local.dot"
	localNoteType     contextRole = "local.note-type"
	localNotehead     contextRole = "local.notehead"
	localPrintObject  contextRole = "local.print-object"
	localPrintDot     contextRole = "local.print-dot"
	localPrintLyric   contextRole = "local.print-lyric"
	localPrintLeger   contextRole = "local.print-leger"
	localPrintSpacing contextRole = "local.print-spacing"
	localTypeSize     contextRole = "local.type-size"
	localFilled       contextRole = "local.filled"
)

func contextLocalBindingSpecs() []contextBindingSpec {
	specs := []contextBindingSpec{
		{Role: localNote, Name: "note", Type: "note", Count: -1},
		{Role: localLyric, Name: "lyric", Type: "lyric", Count: -1},
		{Role: localGrace, Name: "grace", Type: "grace", Count: -1},
		{Role: localCue, Name: "cue", Type: "empty", Count: -1},
		{Role: localRest, Name: "rest", Type: "rest", Count: -1},
		{Role: localDot, Name: "dot", Type: "empty-placement", Count: -1},
		{Role: localNoteType, Name: "type", Type: "note-type", Count: -1},
		{Role: localNotehead, Name: "notehead", Type: "notehead", Count: -1},
		{Role: localPrintObject, Name: "print-object", Type: "yes-no", Attribute: true, Count: -1},
		{Role: localPrintDot, Name: "print-dot", Type: "yes-no", Attribute: true, Count: -1},
		{Role: localPrintLyric, Name: "print-lyric", Type: "yes-no", Attribute: true, Count: -1},
		{Role: localPrintLeger, Name: "print-leger", Type: "yes-no", Attribute: true, Count: -1},
		{Role: localPrintSpacing, Name: "print-spacing", Type: "yes-no", Attribute: true, Count: -1},
		{Role: localTypeSize, Owner: "note-type", Name: "size", Type: "symbol-size", Attribute: true, Count: 1},
		{Role: localFilled, Owner: "notehead", Name: "filled", Type: "yes-no", Attribute: true, Count: 1},
	}
	for i, r := range localConstantRules {
		role := localRuleBindings[i]
		if r.Element != "" {
			specs = append(specs, contextBindingSpec{Role: role.element, Name: r.Element, Type: r.Type, Count: -1})
		}
		if r.Attribute != "" {
			owner, count := "", -1
			if r.Element == "" && r.Type != "" {
				owner, count = r.Type, 1
			}
			specs = append(specs, contextBindingSpec{Role: role.attribute, Owner: owner, Name: r.Attribute, Type: r.AttributeType, Attribute: true, Count: count})
		}
		if r.Child != "" {
			specs = append(specs, contextBindingSpec{Role: role.child, Name: r.Child, Type: r.ChildType, Count: -1})
		}
	}
	specs = append(specs, localStructuralBindingSpecs()...)
	specs = append(specs, contextGlyphBindingSpecs()...)
	return append(specs, contextLayoutBindingSpecs()...)
}
