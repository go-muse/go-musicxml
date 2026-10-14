package validation

import (
	"context"
	"strings"
)

func (s *Session) evaluateDictionary(ctx context.Context, n *factNode, rule RuleDefinition, p *dictionaryPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	var d Decision
	var err error
	switch p.Operation {
	case DictionaryCanonical:
		d, err = dictionaryCanonicalNode(r, n, rule.Key, p)
	case DictionaryStandardSounds:
		d, err = dictionarySoundsNode(r, n, rule.Key)
	case DictionaryIPA:
		d, err = dictionaryIPANode(r, n, rule.Key)
	case DictionaryRendering:
		d, err = dictionaryRenderingNode(r, n, rule.Key)
	}
	if err == nil {
		err = r.err
	}
	if err != nil && d.Outcome == OutcomeFail {
		err = &provedPlanInterruption{node: n.header.ID, dictionary: p, kind: planDictionary, decision: d, cause: err}
	}
	return d, false, err
}
func dictionaryMergeDecision(a, b Decision) Decision {
	rank := func(o Outcome) int {
		switch o {
		case OutcomeFail:
			return 6
		case OutcomeBlocked:
			return 5
		case OutcomeUnsupported:
			return 4
		case OutcomeUnknown:
			return 3
		case OutcomePass:
			return 2
		}
		return 1
	}
	if rank(b.Outcome) > rank(a.Outcome) {
		return b
	}
	return a
}
func dictionarySubjectSelection(n *factNode) Decision {
	if n.header.Declaration.State != EvidenceKnown {
		return evidenceDecision(n.header.Declaration.State, n.header.Declaration.Reason, n.header.Declaration.Cause, false)
	}
	if n.header.Type.State != EvidenceKnown {
		return evidenceDecision(n.header.Type.State, n.header.Type.Reason, n.header.Type.Cause, false)
	}
	return Decision{Outcome: OutcomeNotApplicable, Reason: "dictionary-subject-not-selected"}
}
func dictionaryPayload(input Evidence[Atomic], attribute *AttributeSelector, manifest DictionaryManifestID, relation DictionaryRelation) *DictionaryContextResult {
	return &DictionaryContextResult{Manifest: manifest, Relation: relation, Attribute: attribute, Input: input, Result: Evidence[bool]{State: EvidenceAbsent}, Codepoint: Evidence[uint32]{State: EvidenceAbsent}, WholeSequence: Evidence[bool]{State: EvidenceAbsent}, Effective: Evidence[Atomic]{State: EvidenceAbsent}, Applicability: dictionaryKnown(true)}
}
func dictionaryAttachEntry(out *DictionaryContextResult, entry *dictionaryEntry) {
	out.ComponentVerified = entry.certificate
	out.SourceKind = entry.manifest.Component.SourceKind
	out.DataSHA256 = entry.manifest.DataSHA256
	out.Policy = entry.manifest.PolicyRevision
}
func dictionaryPublish(r *contextReader, n *factNode, key RuleKey, property ContextProperty, p *DictionaryContextResult) error {
	if r.err != nil {
		return r.err
	}
	return r.s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: NodeRef{r.s.documentID, n.header.ID}, Property: property, Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: []Location{n.header.Location}, Dictionary: p})
}
func dictionaryCanonicalLookup(r *contextReader, input Evidence[Atomic], attribute *AttributeSelector) (out *DictionaryContextResult, decision Decision, err error) {
	out = dictionaryPayload(input, attribute, DictionaryManifestSMuFL14Canonical, DictionaryRelationCanonicalName)
	// Primary-codepoint selection is independent. Even a complete codepoint
	// observation cannot replace the mandatory canonical-name relation.
	defer func() {
		if err == nil && input.State == EvidenceKnown && input.Value.Kind == AtomicString {
			err = dictionaryPrimaryCodepoint(r, out)
		}
	}()
	if input.State != EvidenceKnown {
		out.Result = stateUnavailable[Atomic, bool](input)
		return out, evidenceDecision(input.State, input.Reason, input.Cause, true), nil
	}
	if input.Value.Kind != AtomicString {
		out.Result = stateUnknown[bool]("dictionary-string-value-unavailable")
		return out, evidenceDecision(out.Result.State, out.Result.Reason, nil, false), nil
	}
	entry, availability, err := r.s.dictionaryCapability(r.ctx, out.Manifest, out.Relation)
	if err != nil {
		return out, Decision{}, err
	}
	if availability.State != EvidenceKnown {
		out.Result = availability
		return out, evidenceDecision(availability.State, availability.Reason, availability.Cause, false), nil
	}
	dictionaryAttachEntry(out, entry)
	_, found, err := dictionarySearch(r.ctx, r.s.account, len(entry.smufl.Glyphs), input.Value.Text, func(i int) string { return entry.smufl.Glyphs[i].Name })
	if err != nil {
		return out, Decision{}, err
	}
	out.Result = dictionaryKnown(found)
	if !found {
		return out, Decision{Outcome: OutcomeFail, Reason: "smufl-canonical-name-not-listed", Expected: "An exact canonical SMuFL 1.4 glyph name", Actual: input.Value.Text}, nil
	}
	return out, Decision{Outcome: OutcomePass}, nil
}
func dictionaryCanonicalNode(r *contextReader, n *factNode, key RuleKey, p *dictionaryPlan) (Decision, error) {
	d := dictionarySubjectSelection(n)
	publish := func(input Evidence[Atomic], attribute *AttributeSelector) error {
		if input.State == EvidenceAbsent {
			return nil
		}
		result, next, err := dictionaryCanonicalLookup(r, input, attribute)
		// A later independent lookup or projection budget refusal must not erase
		// an already proved canonical nonmembership.
		d = dictionaryMergeDecision(d, next)
		if err != nil {
			return err
		}
		return dictionaryPublish(r, n, key, ContextDictionaryCanonical, result)
	}
	if n.header.Declaration.State == EvidenceKnown && n.header.Type.State == EvidenceKnown {
		if !r.step(contextComponentBytes(n.header.Type.Value)*int64(len(p.Types)) + 1) {
			return d, r.err
		}
		if p.actualSMuFL(n.header.Type.Value) {
			if err := publish(r.scalar(n), nil); err != nil {
				return d, err
			}
		}
	}
	for _, role := range r.attributeRoles(n) {
		if !strings.HasPrefix(string(role), "dictionary.smufl.") {
			continue
		}
		selector, ok := r.attributeSelector(n, role)
		if !ok {
			continue
		}
		if err := publish(r.attribute(n, role), &selector); err != nil {
			return d, err
		}
	}
	return d, r.err
}
func dictionarySoundsNode(r *contextReader, n *factNode, key RuleKey) (Decision, error) {
	selected := false
	var input Evidence[Atomic]
	var attribute *AttributeSelector
	if r.b.assembly == AssemblySounds {
		if r.matches(n, "dictionary.sound") {
			selected = true
			input = r.attribute(n, "dictionary.sound-id")
			if a, ok := r.attributeSelector(n, "dictionary.sound-id"); ok {
				attribute = &a
			}
		}
	} else if r.matches(n, "dictionary.sounds.score-instrument") || r.matches(n, "dictionary.sounds.instrument-change") {
		selected = true
		input = r.scalar(n)
	}
	if r.err != nil {
		return Decision{}, r.err
	}
	if !selected {
		return dictionarySubjectSelection(n), nil
	}
	out := dictionaryPayload(input, attribute, DictionaryManifestStandardSounds40, DictionaryRelationStandardSound)
	if input.State != EvidenceKnown {
		out.Result = stateUnavailable[Atomic, bool](input)
	} else if input.Value.Kind != AtomicString {
		out.Result = stateUnknown[bool]("dictionary-string-value-unavailable")
	} else {
		entry, availability, err := r.s.dictionaryCapability(r.ctx, out.Manifest, out.Relation)
		if err != nil {
			return Decision{}, err
		}
		out.Result = availability
		if availability.State == EvidenceKnown {
			dictionaryAttachEntry(out, entry)
			_, found, err := dictionarySearch(r.ctx, r.s.account, len(entry.sounds.IDs), input.Value.Text, func(i int) string { return entry.sounds.IDs[i] })
			if err != nil {
				return Decision{}, err
			}
			out.Result = dictionaryKnown(found)
		}
	}
	// Both a recognized and an unrecognized sound are permitted open strings.
	// Provider completion records observation, never a membership predicate.
	return Decision{Outcome: OutcomePass}, dictionaryPublish(r, n, key, ContextDictionarySounds, out)
}

func dictionaryKnown[T any](v T) Evidence[T] { return Evidence[T]{State: EvidenceKnown, Value: v} }

func dictionaryPrimaryCodepoint(r *contextReader, out *DictionaryContextResult) error {
	entry, availability, err := r.s.dictionaryCapability(r.ctx, DictionaryManifestSMuFL14Canonical, DictionaryRelationPrimaryCodepoint)
	if err != nil {
		return err
	}
	if availability.State != EvidenceKnown {
		out.Codepoint = stateUnavailable[bool, uint32](availability)
		return nil
	}
	dictionaryAttachEntry(out, entry)
	index, found, err := dictionarySearch(r.ctx, r.s.account, len(entry.smufl.Glyphs), out.Input.Value.Text, func(i int) string { return entry.smufl.Glyphs[i].Name })
	if err != nil {
		return err
	}
	out.Codepoint = Evidence[uint32]{State: EvidenceUnknown, Reason: "smufl-primary-codepoint-unlisted"}
	if found {
		out.Codepoint = dictionaryKnown(entry.smufl.Glyphs[index].Codepoint)
	}
	return nil
}
