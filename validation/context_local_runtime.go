package validation

import "context"

// Local context properties are exact, bounded projections from the selected
// source/model facts. Their raw and applicability axes remain independent.
const (
	ContextPrintObject    ContextProperty = "local.print-object"
	ContextPrintDot       ContextProperty = "local.print-dot"
	ContextPrintLyric     ContextProperty = "local.print-lyric"
	ContextPrintLeger     ContextProperty = "local.print-leger"
	ContextPrintSpacing   ContextProperty = "local.print-spacing"
	ContextNoteTypeSize   ContextProperty = "local.note-type-size"
	ContextNoteheadFilled ContextProperty = "local.notehead-filled"
)

func contextLocalPropertyValid(p ContextProperty) bool {
	switch p {
	case ContextPrintObject, ContextPrintDot, ContextPrintLyric, ContextPrintLeger, ContextPrintSpacing, ContextNoteTypeSize, ContextNoteheadFilled:
		return true
	}
	for _, r := range localRuleBindings {
		if r.property == p {
			return true
		}
	}
	return localStructuralPropertyValid(p) || localAdvisoryPropertyValid(p) || layoutPropertyValid(p) || contextGlyphPropertyValid(p)
}
func evaluateContextLocal(ctx context.Context, s *Session, n *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	for _, id := range s.sequence {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		node := s.nodes[id]
		if node == nil {
			continue
		}
		for i, rule := range localConstantRules {
			if !rule.Layout {
				if err := localPublishConstant(ctx, s, key, node, r, i); err != nil {
					return Decision{}, false, err
				}
			}
		}
		if err := localPrintNode(ctx, s, key, node, r); err != nil {
			return Decision{}, false, err
		}
		if err := localGlyphFacts(r, key, node); err != nil {
			return Decision{}, false, err
		}
		if err := localStructuralNode(ctx, s, key, node, r); err != nil {
			return Decision{}, false, err
		}
		if err := localAdvisoryNode(ctx, s, key, node, r); err != nil {
			return Decision{}, false, err
		}
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}
func localBoolAtomic(e Evidence[bool]) Evidence[Atomic] {
	if e.State != EvidenceKnown {
		return stateUnavailable[bool, Atomic](e)
	}
	text := "no"
	if e.Value {
		text = "yes"
	}
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: text}}
}
func localStringAtomic(e Evidence[string]) Evidence[Atomic] {
	if e.State != EvidenceKnown {
		return stateUnavailable[string, Atomic](e)
	}
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: e.Value}}
}
func localPresence(r *contextReader, n *factNode, role contextRole) Evidence[bool] {
	_, e := r.child(n, role)
	if e.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	return e
}
func localPrintNode(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader) error {
	active := Evidence[bool]{State: EvidenceKnown, Value: true}
	if r.matches(n, localNote) {
		raw := []Evidence[Atomic]{localRawAttribute(r, n, localPrintObject), localRawAttribute(r, n, localPrintDot), localRawAttribute(r, n, localPrintLyric), localRawAttribute(r, n, localPrintLeger)}
		values := localPrinting(stateYesNo(raw[0]), stateYesNo(raw[1]), stateYesNo(raw[2]), stateYesNo(raw[3]), localPresence(r, n, localRest))
		fields := []Evidence[bool]{values.Object, values.Dot, values.Lyric, values.Leger}
		properties := []ContextProperty{ContextPrintObject, ContextPrintDot, ContextPrintLyric, ContextPrintLeger}
		for i, v := range fields {
			a := active
			if i == 3 {
				a = values.LegerActive
			}
			if err := localPublish(ctx, s, key, n, properties[i], raw[i], localBoolAtomic(v), a, NodeRef{}, ContextUnitNone); err != nil {
				return err
			}
		}
		spacingRaw := localRawAttribute(r, n, localPrintSpacing)
		spacing := localDefault(stateYesNo(spacingRaw), true)
		applicability := localSpacingActive(r, n, values)
		if err := localPublish(ctx, s, key, n, ContextPrintSpacing, spacingRaw, localBoolAtomic(spacing), applicability, NodeRef{}, ContextUnitNone); err != nil {
			return err
		}
	} else if r.attributeOwner(n, localPrintObject) {
		raw := localRawAttribute(r, n, localPrintObject)
		value := localDefault(stateYesNo(raw), true)
		if r.matches(n, localLyric) && raw.State == EvidenceAbsent {
			parent := s.nodes[n.header.Parent]
			if parent == nil || !r.matches(parent, localNote) {
				value = stateUnknown[bool]("lyric-note-parent-unavailable")
			} else {
				value = localInheritedBool(stateYesNo(localRawAttribute(r, parent, localPrintLyric)), localDefault(stateYesNo(localRawAttribute(r, parent, localPrintObject)), true))
			}
		}
		var err error
		parent := s.nodes[n.header.Parent]
		if r.matches(n, localLyric) && raw.State == EvidenceAbsent && parent != nil && r.matches(parent, localNote) {
			err = localPublishAtOrigin(ctx, s, key, n, parent, ContextPrintObject, raw, localBoolAtomic(value), active, ContextInherited)
		} else {
			err = localPublish(ctx, s, key, n, ContextPrintObject, raw, localBoolAtomic(value), active, NodeRef{}, ContextUnitNone)
		}
		if err != nil {
			return err
		}
	}
	if r.matches(n, localNoteType) {
		raw := localRawAttribute(r, n, localTypeSize)
		parent := s.nodes[n.header.Parent]
		grace, cue := stateUnknown[bool]("note-owner-unavailable"), stateUnknown[bool]("note-owner-unavailable")
		if parent != nil && r.matches(parent, localNote) {
			grace, cue = localPresence(r, parent, localGrace), localPresence(r, parent, localCue)
		}
		value := localNoteSize(stateText(raw), grace, cue)
		if err := localPublish(ctx, s, key, n, ContextNoteTypeSize, raw, localStringAtomic(value), active, NodeRef{}, ContextUnitNone); err != nil {
			return err
		}
	}
	if r.matches(n, localNotehead) {
		raw := localRawAttribute(r, n, localFilled)
		value := raw
		shape := stateText(r.scalar(n))
		applicable := active
		if raw.State == EvidenceAbsent {
			enclosed := localEnclosedNotehead(shape)
			applicable = enclosed
			if enclosed.State != EvidenceKnown {
				value = stateUnavailable[bool, Atomic](enclosed)
			} else if !enclosed.Value {
				value = Evidence[Atomic]{State: EvidenceAbsent}
			} else {
				parent := s.nodes[n.header.Parent]
				value = stateUnknown[Atomic]("notehead-note-type-unavailable")
				if parent != nil && r.matches(parent, localNote) {
					value = localNoteheadFill(r.childScalar(parent, localNoteType))
				}
			}
		}
		if err := localPublish(ctx, s, key, n, ContextNoteheadFilled, raw, value, applicable, NodeRef{}, ContextUnitNone); err != nil {
			return err
		}
	}
	return r.err
}
func localSpacingActive(r *contextReader, n *factNode, v localPrintValues) Evidence[bool] {
	if v.Object.State == EvidenceKnown && v.Object.Value {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return complete
	}
	unknown := v.Object
	for _, ch := range n.children {
		if !r.step(1) {
			return stateUnknown[bool]("context-interrupted")
		}
		if r.matches(ch, localDot) {
			if v.Dot.State == EvidenceKnown && v.Dot.Value {
				return Evidence[bool]{State: EvidenceKnown, Value: false}
			}
			if v.Dot.State != EvidenceKnown {
				unknown = v.Dot
			}
		}
		if r.matches(ch, localLyric) {
			visible := localInheritedBool(stateYesNo(localRawAttribute(r, ch, localPrintObject)), v.Lyric)
			if visible.State == EvidenceKnown && visible.Value {
				return Evidence[bool]{State: EvidenceKnown, Value: false}
			}
			if visible.State != EvidenceKnown {
				unknown = visible
			}
		}
	}
	if unknown.State != EvidenceKnown {
		return unknown
	}
	return Evidence[bool]{State: EvidenceKnown, Value: true}
}
func localEnclosedNotehead(shape Evidence[string]) Evidence[bool] {
	if shape.State != EvidenceKnown {
		return stateUnavailable[string, bool](shape)
	}
	switch shape.Value {
	case "normal", "diamond", "square", "rectangle", "triangle", "inverted triangle", "left triangle", "circle-x", "circled", "circle dot", "slashed", "back slashed":
		return Evidence[bool]{State: EvidenceKnown, Value: true}
	case "x", "cross", "none":
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	return stateUnknown[bool]("notehead-enclosed-shape-interpretation")
}
func localNoteheadFill(noteType Evidence[Atomic]) Evidence[Atomic] {
	if noteType.State == EvidenceAbsent {
		return stateUnknown[Atomic]("notehead-note-type-unavailable")
	}
	if noteType.State != EvidenceKnown {
		return noteType
	}
	switch noteType.Value.Text {
	case "maxima", "long", "breve", "whole", "half":
		return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "no"}}
	case "quarter", "eighth", "16th", "32nd", "64th", "128th", "256th", "512th", "1024th":
		return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "yes"}}
	}
	return stateUnknown[Atomic]("notehead-note-type-unavailable")
}
