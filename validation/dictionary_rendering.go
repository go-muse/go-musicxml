package validation

func dictionaryDefault(raw Evidence[Atomic], value string) Evidence[Atomic] {
	if raw.State == EvidenceAbsent {
		return dictionaryKnown(Atomic{Kind: AtomicString, Text: value})
	}
	return raw
}
func dictionaryRenderingNode(r *contextReader, n *factNode, key RuleKey) (Decision, error) {
	matched := false
	publish := func(property ContextProperty, role string, raw, effective Evidence[Atomic], active Evidence[bool], result Evidence[bool]) error {
		matched = true
		out := dictionaryPayload(raw, nil, "", "")
		out.Role = role
		out.Effective = effective
		out.Applicability = active
		out.Result = result
		return dictionaryPublish(r, n, key, property, out)
	}
	yes := dictionaryKnown(true)
	absent := Evidence[bool]{State: EvidenceAbsent}
	for _, v := range []struct{ role, value string }{{"stopped", "brassMuteClosed"}, {"open", "brassMuteOpen"}, {"half-muted", "brassMuteHalfClosed"}} {
		if r.matches(n, contextRole("dictionary."+v.role)) {
			raw := localRawAttribute(r, n, "dictionary.smufl.empty-placement-smufl")
			if err := publish(ContextDictionaryRendering, v.role+"-smufl", raw, dictionaryDefault(raw, v.value), yes, absent); err != nil {
				return Decision{}, err
			}
		}
	}
	if r.matches(n, "dictionary.tremolo") {
		raw := localRawAttribute(r, n, "dictionary.smufl.tremolo")
		typ := r.attribute(n, "dictionary.tremolo-type")
		active := stateUnavailable[Atomic, bool](typ)
		effective := stateUnavailable[Atomic, Atomic](typ)
		if typ.State == EvidenceKnown {
			active = dictionaryKnown(typ.Value.Text == "unmeasured")
			effective = Evidence[Atomic]{State: EvidenceAbsent}
			if active.Value {
				effective = dictionaryDefault(raw, "buzzRoll")
			}
		}
		if err := publish(ContextDictionaryRendering, "tremolo-smufl", raw, effective, active, absent); err != nil {
			return Decision{}, err
		}
		if active.State == EvidenceKnown && active.Value {
			if err := dictionaryPublishRange(r, n, key, "tremolo-range", effective, active, []string{"tremolos"}); err != nil {
				return Decision{}, err
			}
		}
	}
	if r.matches(n, "dictionary.notehead") {
		raw := localRawAttribute(r, n, "dictionary.smufl.notehead")
		typ := r.scalar(n)
		active := yes
		if typ.State != EvidenceKnown {
			active = stateUnavailable[Atomic, bool](typ)
		} else if raw.State == EvidenceAbsent && typ.Value.Text != "other" {
			active = dictionaryKnown(false)
		}
		if err := dictionaryPublishRange(r, n, key, "notehead-text-recommended", raw, active, []string{"noteNameNoteheads", "noteNameNoteheadsSupplement"}); err != nil {
			return Decision{}, err
		}
		matched = true
	}
	if r.matches(n, "dictionary.smufl.direction-type") {
		for _, v := range []struct{ role, value string }{{"symbol-justify", "left"}, {"symbol-enclosure", "none"}} {
			raw := localRawAttribute(r, n, contextRole("dictionary."+v.role))
			if err := publish(ContextDictionaryRendering, v.role, raw, dictionaryDefault(raw, v.value), yes, absent); err != nil {
				return Decision{}, err
			}
		}
		raw := r.scalar(n)
		value := stateUnknown[Atomic]("smufl-specialized-markup-dictionary-unavailable")
		if raw.State != EvidenceKnown {
			value = raw
		}
		if err := publish(ContextDictionaryAdvice, "symbol-specialized-markup", raw, value, yes, absent); err != nil {
			return Decision{}, err
		}
	}
	if r.matches(n, "dictionary.smufl.appearance") {
		typ, name := r.attribute(n, "dictionary.glyph-type"), r.scalar(n)
		if err := publish(ContextDictionaryAdvice, "glyph-semantic-target", typ, glyphSemanticTarget(typ), yes, absent); err != nil {
			return Decision{}, err
		}
		if err := publish(ContextDictionaryAdvice, "glyph-type-match", name, glyphTypeMatch(typ, name), yes, absent); err != nil {
			return Decision{}, err
		}
	}
	if r.matches(n, "dictionary.arrow") {
		for _, role := range []string{"arrow-direction", "arrow-style", "circular-arrow"} {
			raw := r.childScalar(n, contextRole("dictionary."+role))
			if err := publish(ContextDictionaryRendering, role, raw, raw, yes, absent); err != nil {
				return Decision{}, err
			}
		}
		head := localPresence(r, n, "dictionary.arrowhead")
		raw := localBoolAtomic(head)
		if err := publish(ContextDictionaryRendering, "arrow-head-only", raw, raw, yes, absent); err != nil {
			return Decision{}, err
		}
		name := localRawAttribute(r, n, "dictionary.smufl.arrow")
		value := stateUnknown[Atomic]("smufl-arrow-description-dictionary-unavailable")
		if name.State != EvidenceKnown {
			value = name
		}
		if err := publish(ContextDictionaryAdvice, "arrow-glyph-match", name, value, yes, absent); err != nil {
			return Decision{}, err
		}
	}
	if r.matches(n, "dictionary.mordent") || r.matches(n, "dictionary.inverted-mordent") {
		raw := localRawAttribute(r, n, "dictionary.mordent-long")
		if err := publish(ContextDictionaryRendering, "mordent-long", raw, dictionaryDefault(raw, "no"), yes, absent); err != nil {
			return Decision{}, err
		}
	}
	if r.err != nil {
		return Decision{}, r.err
	}
	if matched {
		return Decision{Outcome: OutcomePass}, nil
	}
	return dictionarySubjectSelection(n), nil
}
func dictionaryPublishRange(r *contextReader, n *factNode, key RuleKey, role string, input Evidence[Atomic], active Evidence[bool], ranges []string) error {
	out := dictionaryPayload(input, nil, DictionaryManifestSMuFL14Ranges, DictionaryRelationRangeMembership)
	out.Role = role
	attributeRole := contextRole("dictionary.smufl.notehead")
	if role == "tremolo-range" {
		attributeRole = "dictionary.smufl.tremolo"
	}
	if selector, ok := r.attributeSelector(n, attributeRole); ok {
		out.Attribute = &selector
	}
	out.Applicability = active
	if active.State != EvidenceKnown {
		out.Result = active
		return dictionaryPublish(r, n, key, ContextDictionaryAdvice, out)
	}
	if !active.Value {
		out.Result = Evidence[bool]{State: EvidenceAbsent}
		return dictionaryPublish(r, n, key, ContextDictionaryAdvice, out)
	}
	if input.State != EvidenceKnown {
		out.Result = stateUnavailable[Atomic, bool](input)
		if input.State == EvidenceAbsent {
			out.Result = stateUnknown[bool]("smufl-range-name-unavailable")
		}
		return dictionaryPublish(r, n, key, ContextDictionaryAdvice, out)
	}
	canonical, _, err := dictionaryCanonicalLookup(r, input, nil)
	if err != nil {
		return err
	}
	if canonical.Result.State != EvidenceKnown {
		out.Result = canonical.Result
		return dictionaryPublish(r, n, key, ContextDictionaryAdvice, out)
	}
	entry, availability, err := r.s.dictionaryCapability(r.ctx, out.Manifest, out.Relation)
	if err != nil {
		return err
	}
	out.Result = availability
	if availability.State == EvidenceKnown {
		dictionaryAttachEntry(out, entry)
		out.Result = dictionaryKnown(false)
		if canonical.Result.Value {
			for _, id := range ranges {
				index, found, err := dictionarySearch(r.ctx, r.s.account, len(entry.smufl.Ranges), id, func(i int) string { return entry.smufl.Ranges[i].ID })
				if err != nil {
					return err
				}
				if !found {
					return contract("dictionary range", "reviewed fixed range absent from verified data")
				}
				names := entry.smufl.Ranges[index].Names
				_, member, err := dictionarySearch(r.ctx, r.s.account, len(names), input.Value.Text, func(i int) string { return names[i] })
				if err != nil {
					return err
				}
				if member {
					out.Result = dictionaryKnown(true)
					break
				}
			}
		}
	}
	return dictionaryPublish(r, n, key, ContextDictionaryAdvice, out)
}
