package validation

import (
	"strconv"
	"strings"
)

const spanUnnamedLyricIdentity = "a:a:"

type spanLyricScopeKey struct {
	note     NodeRef
	identity string
}

// Absence is an unnamed lyric identity, not verse 1. The presence markers also
// distinguish an absent name from an explicitly empty xs:token name.
func spanLyricIdentity(r *contextReader, n *factNode, scratch *spanScratch) spanLyricIdentityValue {
	number, name := r.attribute(n, spanRole_lyric_number), r.attribute(n, spanRole_lyric_name)
	value := spanLyricIdentityValue{available: Evidence[bool]{State: EvidenceKnown, Value: true}}
	for _, v := range []Evidence[Atomic]{number, name} {
		// One independently known explicit string axis cannot equal absence,
		// even if the other axis is unavailable. Empty name is an explicit token;
		// a mistyped Known atomic value establishes no such identity proof.
		if v.State == EvidenceKnown && v.Value.Kind == AtomicString {
			value.excludesUnnamed = true
		}
		if v.State != EvidenceKnown && v.State != EvidenceAbsent {
			value.available = timelineMergeAvailability(value.available, stateUnavailable[Atomic, bool](v))
		} else if v.State == EvidenceKnown && v.Value.Kind != AtomicString {
			value.available = timelineMergeAvailability(value.available, stateUnknown[bool]("span-lyric-identity-unavailable"))
		}
	}
	if value.available.State != EvidenceKnown {
		return value
	}
	bytes := int64(len(number.Value.Text)) + int64(len(name.Value.Text)) + 128
	if !r.step(bytes) {
		value.available = stateUnknown[bool]("context-interrupted")
		return value
	}
	allocation, err := allocationSize(bytes - 64)
	if err != nil {
		r.err = err
		value.available = stateUnknown[bool]("context-interrupted")
		return value
	}
	if scratch == nil {
		r.err = r.s.retain(bytes)
	} else {
		r.err = scratch.reserve(bytes)
	}
	if r.err != nil {
		value.available = stateUnknown[bool]("context-interrupted")
		return value
	}
	var key strings.Builder
	key.Grow(allocation)
	for _, v := range []Evidence[Atomic]{number, name} {
		if v.State == EvidenceAbsent {
			key.WriteString("a:")
			continue
		}
		key.WriteByte('k')
		key.WriteString(strconv.Itoa(len(v.Value.Text)))
		key.WriteByte(':')
		key.WriteString(v.Value.Text)
	}
	value.key = key.String()
	return value
}

type spanLyricIdentityValue struct {
	key       string
	available Evidence[bool]
	retained  bool
	// excludesUnnamed is independent of full identity availability.
	excludesUnnamed bool
}

type spanLyricIndex struct {
	scratch    *spanScratch
	identities map[NodeID]spanLyricIdentityValue
	scopes     map[spanLyricScopeKey]Evidence[bool]
	domains    map[NodeID]Evidence[NodeRef]
	unnamed    map[NodeRef]Evidence[bool]
}

// Include every lyric, even one without an extend. Multiple unnamed lyrics on
// one note cannot silently turn into one established line on the next note.
// Number and name have no default (MusicXML 4.0 lyric type). For the wholly
// unnamed identity, no source-defined line-selection or reset rule distinguishes
// duplicate siblings, even before or after an extend. Conservatively withhold
// that identity across its part; this is not a rule that rejects the document.
// Explicit identities retain their separate, endpoint-local duplicate check.
func (index *spanLyricIndex) record(r *contextReader, n *factNode) {
	if r.err != nil {
		return
	}
	value := spanLyricIdentity(r, n, index.scratch)
	identity, available := value.key, value.available
	note := spanAncestor(r, n, spanRole_note)
	if r.err != nil || note == nil {
		return
	}
	if r.err = index.scratch.reserve(256); r.err != nil {
		return
	}
	if index.identities == nil {
		if r.err = index.scratch.reserve(512); r.err != nil {
			return
		}
		index.identities = make(map[NodeID]spanLyricIdentityValue)
		index.scopes = make(map[spanLyricScopeKey]Evidence[bool])
	}
	index.identities[n.header.ID] = value
	key := spanLyricScopeKey{note: NodeRef{r.s.documentID, note.header.ID}, identity: identity}
	if old, ok := index.scopes[key]; ok {
		if old.State == EvidenceKnown && available.State == EvidenceKnown {
			index.scopes[key] = stateUnknown[bool]("span-lyric-line-ambiguous")
		} else {
			index.scopes[key] = timelineMergeAvailability(old, available)
		}
	} else {
		if !r.step(1) {
			return
		}
		if r.err = index.scratch.reserve(256); r.err != nil {
			return
		}
		index.scopes[key] = available
	}
	// Unavailable identities share an endpoint-local wildcard bucket. That
	// bucket may include independently excluded explicit identities, so it
	// cannot supply evidence for the part-wide unnamed candidate. Only complete
	// unnamed identities use the sibling bucket to detect actual duplicates.
	gap := available
	if available.State == EvidenceKnown {
		gap = index.scopes[key]
	}
	if value.excludesUnnamed || (gap.State == EvidenceKnown && gap.Value) {
		return
	}
	// An unavailable identity can compete with the wholly unnamed identity only
	// when neither axis independently excludes absence. Preserve the cause of
	// that actual uncertainty rather than synthesizing duplicate ambiguity.
	domain := spanInheritedDomain(r, note, index.domains)
	if r.err != nil {
		return
	}
	var part NodeRef
	if domain.State == EvidenceKnown {
		part = domain.Value
	}
	if !r.step(1 + int64(len(part.DocumentID))) {
		return
	}
	if old, ok := index.unnamed[part]; ok {
		index.unnamed[part] = timelineMergeAvailability(old, gap)
		return
	}
	if r.err = index.scratch.reserve(256 + int64(len(part.DocumentID))); r.err != nil {
		return
	}
	if index.unnamed == nil {
		if r.err = index.scratch.reserve(512); r.err != nil {
			return
		}
		index.unnamed = make(map[NodeRef]Evidence[bool])
	}
	index.unnamed[part] = gap
}

func (index *spanLyricIndex) endpoint(r *contextReader, lyric *factNode) (string, Evidence[bool]) {
	if index == nil || lyric == nil {
		value := spanLyricIdentity(r, lyric, nil)
		return value.key, value.available
	}
	if !r.step(1) {
		return "", stateUnknown[bool]("context-interrupted")
	}
	value, ok := index.identities[lyric.header.ID]
	if !ok {
		return "", stateUnknown[bool]("span-lyric-identity-unavailable")
	}
	// Endpoints outlive the provider-local index. Retain their shared immutable
	// identity once, independently of scratch storage, before lending it out.
	if !value.retained {
		if r.err = r.s.retain(int64(len(value.key)) + 128); r.err != nil {
			return "", stateUnknown[bool]("context-interrupted")
		}
		value.retained = true
		index.identities[lyric.header.ID] = value
	}
	return value.key, value.available
}
