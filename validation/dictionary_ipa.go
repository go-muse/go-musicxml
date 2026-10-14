package validation

import (
	"unicode/utf8"
)

func dictionaryIPANode(r *contextReader, n *factNode, key RuleKey) (Decision, error) {
	if !r.matches(n, "dictionary.ipa.play") {
		return dictionarySubjectSelection(n), r.err
	}
	input := r.scalar(n)
	if r.err != nil {
		return Decision{}, r.err
	}
	decision := Decision{Outcome: OutcomeUnsupported, Reason: "ipa-complete-repertoire-sequence-unavailable"}
	if input.State != EvidenceKnown {
		decision = evidenceDecision(input.State, input.Reason, input.Cause, false)
	}
	for _, relation := range []DictionaryRelation{DictionaryRelationIPASymbol, DictionaryRelationIPAAttestedExample, DictionaryRelationIPACanonicalEquivalent} {
		out := dictionaryPayload(input, nil, DictionaryManifestIPAPositive2015Unicode13, relation)
		out.WholeSequence = Evidence[bool]{State: EvidenceUnsupported, Reason: "ipa-complete-repertoire-sequence-unavailable"}
		if input.State != EvidenceKnown {
			out.Result = stateUnavailable[Atomic, bool](input)
			out.WholeSequence = out.Result
		} else if input.Value.Kind != AtomicString {
			out.Result = stateUnknown[bool]("dictionary-string-value-unavailable")
		} else {
			entry, availability, err := r.s.dictionaryCapability(r.ctx, out.Manifest, relation)
			if err != nil {
				return decision, err
			}
			out.Result = availability
			if availability.State == EvidenceKnown {
				dictionaryAttachEntry(out, entry)
				if err := dictionaryIPARelation(r, out, entry); err != nil {
					return decision, err
				}
			} else {
				// Retain exact unavailable input, including explicitly empty input.
				span := DictionarySpan{Start: 0, End: uint64(len(input.Value.Text)), Text: input.Value.Text, Result: availability}
				if err := contextRetainProjection(r.ctx, r.s, span); err != nil {
					return decision, err
				}
				out.Spans = []DictionarySpan{span}
			}
		}
		if err := dictionaryPublish(r, n, key, ContextDictionaryIPA, out); err != nil {
			return decision, err
		}
	}
	return dictionaryIPARepresentationNode(r, n, key, input, decision)
}

// The separately selected representation relation is the only whole-input IPA
// gate. The three legacy projections above keep their exact partial semantics.
func dictionaryIPARepresentationNode(r *contextReader, n *factNode, key RuleKey, input Evidence[Atomic], legacy Decision) (Decision, error) {
	entry, availability, err := r.s.dictionaryCapability(r.ctx, DictionaryManifestIPARepresentation2015Unicode13, DictionaryRelationIPARepresentation)
	if err != nil {
		return legacy, err
	}
	if entry == nil {
		return legacy, r.err
	} // No implicit activation/projection.
	out := dictionaryPayload(input, nil, DictionaryManifestIPARepresentation2015Unicode13, DictionaryRelationIPARepresentation)
	out.Result = availability
	if availability.State == EvidenceKnown {
		dictionaryAttachEntry(out, entry)
	}
	if input.State != EvidenceKnown {
		out.Result = stateUnavailable[Atomic, bool](input)
	} else if input.Value.Kind != AtomicString {
		out.Result = stateUnknown[bool]("dictionary-string-value-unavailable")
	} else if availability.State == EvidenceKnown {
		ok, err := ipaRepresentationWitness(r.ctx, r.s.account, entry.representation, input.Value.Text)
		if err != nil {
			return legacy, err
		}
		out.Result = stateUnknown[bool]("ipa-representation-witness-unavailable")
		if ok {
			out.Result = dictionaryKnown(true)
		}
	}
	out.WholeSequence = out.Result
	// Normalized leaves may be discontiguous and share one raw scalar interval.
	// Publish the one exact raw interval, never fabricated contiguous atom spans.
	// Missing or non-string scalar evidence has no raw string interval to project.
	// A known string with missing capability still retains its genuine interval.
	if input.State == EvidenceKnown && input.Value.Kind == AtomicString {
		span := DictionarySpan{Start: 0, End: uint64(len(input.Value.Text)), Text: input.Value.Text, Result: out.Result, Category: "whole-representation"}
		if err := contextRetainProjection(r.ctx, r.s, span); err != nil {
			return legacy, err
		}
		out.Spans = []DictionarySpan{span}
	}
	decision := evidenceDecision(out.WholeSequence.State, out.WholeSequence.Reason, out.WholeSequence.Cause, false)
	return decision, dictionaryPublish(r, n, key, ContextDictionaryIPARepresentation, out)
}

func dictionaryIPARelation(r *contextReader, out *DictionaryContextResult, entry *dictionaryEntry) error {
	if !r.step(1) {
		return r.err
	}
	text := out.Input.Value.Text
	out.Result = stateUnknown[bool]("ipa-positive-table-is-not-complete-repertoire")
	if out.Relation == DictionaryRelationIPAAttestedExample {
		i, found, err := dictionarySearch(r.ctx, r.s.account, len(entry.ipa.Examples), text, func(i int) string { return entry.ipa.Examples[i].Encoding })
		if err != nil {
			return err
		}
		span := DictionarySpan{Start: 0, End: uint64(len(text)), Text: text, Result: stateUnknown[bool]("ipa-example-not-attested")}
		if found {
			row := entry.ipa.Examples[i]
			span.Result = dictionaryKnown(true)
			span.RowID = row.RowID
			span.Category = row.Category
			out.Result = dictionaryKnown(true)
		}
		if err := contextRetainProjection(r.ctx, r.s, span); err != nil {
			return err
		}
		out.Spans = []DictionarySpan{span}
		return nil
	}
	maxBytes := entry.ipaSymbolMaxBytes
	count := len(entry.ipa.Symbols)
	keyAt := func(i int) string { return entry.ipa.Symbols[i].Encoding }
	if out.Relation == DictionaryRelationIPACanonicalEquivalent {
		maxBytes = entry.ipaEquivalentMaxBytes
		count = len(entry.ipa.Equivalences)
		keyAt = func(i int) string { return entry.ipa.Equivalences[i].Encoding }
	}
	// The immutable constructor computes each table's maximum while copying
	// its already bounded (at most 16-byte) encodings. This finite probe bound
	// is not a restriction on user text, combining marks, or IPA sequences.
	allKnown := len(text) > 0
	appendSpan := func(span DictionarySpan) error {
		// Geometric backing and the temporary input graph coexist with publication.
		if err := r.s.retain(1024 + int64(len(span.Text)+len(span.RowID)+len(span.Category)+len(span.Equivalent))); err != nil {
			return err
		}
		out.Spans = append(out.Spans, span)
		return nil
	}
	if text == "" {
		return appendSpan(DictionarySpan{Text: "", Result: stateUnknown[bool]("ipa-empty-sequence-unreviewed")})
	}
	for start := 0; start < len(text); {
		if !r.step(1) {
			return r.err
		}
		_, width := utf8.DecodeRuneInString(text[start:])
		end := start + width
		matched, matchEnd := -1, end
		for candidate := end; candidate <= len(text) && candidate-start <= maxBytes; {
			index, found, err := dictionarySearch(r.ctx, r.s.account, count, text[start:candidate], keyAt)
			if err != nil {
				return err
			}
			if found {
				matched, matchEnd = index, candidate
			}
			if candidate == len(text) {
				break
			}
			_, w := utf8.DecodeRuneInString(text[candidate:])
			candidate += w
		}
		span := DictionarySpan{Start: uint64(start), End: uint64(end), Text: text[start:end], Result: stateUnknown[bool]("ipa-encoding-not-in-positive-table")}
		if matched >= 0 {
			end = matchEnd
			span.End = uint64(end)
			span.Text = text[start:end]
			span.Result = dictionaryKnown(true)
			if out.Relation == DictionaryRelationIPASymbol {
				row := entry.ipa.Symbols[matched]
				span.RowID = row.RowID
				span.Category = row.Category
			} else {
				row := entry.ipa.Equivalences[matched]
				span.RowID = row.RowID
				span.Equivalent = row.EquivalentToEncoding
			}
		} else {
			allKnown = false
		}
		if err := appendSpan(span); err != nil {
			return err
		}
		start = end
	}
	if allKnown {
		out.Result = dictionaryKnown(true)
	}
	return nil
}
