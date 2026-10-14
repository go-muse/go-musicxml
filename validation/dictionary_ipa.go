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
	return decision, r.err
}

func dictionaryIPARelation(r *contextReader, out *DictionaryContextResult, entry *dictionaryEntry) error {
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
	count := len(entry.ipa.Symbols)
	keyAt := func(i int) string { return entry.ipa.Symbols[i].Encoding }
	if out.Relation == DictionaryRelationIPACanonicalEquivalent {
		count = len(entry.ipa.Equivalences)
		keyAt = func(i int) string { return entry.ipa.Equivalences[i].Encoding }
	}
	maxBytes := 0
	for i := 0; i < count; i++ {
		encoding := keyAt(i)
		if !r.step(1 + int64(len(encoding))) {
			return r.err
		}
		maxBytes = max(maxBytes, len(encoding))
	}
	// This is a finite compiled-table bound, not a restriction on user text,
	// combining marks or IPA sequences. It keeps longest-match probing linear.
	if maxBytes > 64 {
		return contract("IPA positive table", "reviewed encoding span exceeds finite lookup bound")
	}
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
