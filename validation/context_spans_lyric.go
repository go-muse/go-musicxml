package validation

import (
	"strconv"
	"strings"
)

type spanLyricScopeKey struct {
	note     NodeRef
	identity string
}

// Absence is an unnamed lyric identity, not verse 1. The presence markers also
// distinguish an absent name from an explicitly empty xs:token name.
func spanLyricIdentity(r *contextReader, n *factNode, scratch *spanScratch) (string, Evidence[bool]) {
	number, name := r.attribute(n, spanRole_lyric_number), r.attribute(n, spanRole_lyric_name)
	available := Evidence[bool]{State: EvidenceKnown, Value: true}
	for _, v := range []Evidence[Atomic]{number, name} {
		if v.State != EvidenceKnown && v.State != EvidenceAbsent {
			available = timelineMergeAvailability(available, stateUnavailable[Atomic, bool](v))
		} else if v.State == EvidenceKnown && v.Value.Kind != AtomicString {
			available = timelineMergeAvailability(available, stateUnknown[bool]("span-lyric-identity-unavailable"))
		}
	}
	if available.State != EvidenceKnown {
		return "", available
	}
	bytes := int64(len(number.Value.Text)) + int64(len(name.Value.Text)) + 128
	if !r.step(bytes) {
		return "", stateUnknown[bool]("context-interrupted")
	}
	allocation, err := AllocationSize(bytes - 64)
	if err != nil {
		r.err = err
		return "", stateUnknown[bool]("context-interrupted")
	}
	if scratch == nil {
		r.err = r.s.retain(bytes)
	} else {
		r.err = scratch.reserve(bytes)
	}
	if r.err != nil {
		return "", stateUnknown[bool]("context-interrupted")
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
	return key.String(), available
}

type spanLyricIdentityValue struct {
	key       string
	available Evidence[bool]
	retained  bool
}

type spanLyricIndex struct {
	scratch    *spanScratch
	identities map[NodeID]spanLyricIdentityValue
	scopes     map[spanLyricScopeKey]Evidence[bool]
}

// Include every lyric, even one without an extend. Multiple unnamed lyrics on
// one note cannot silently turn into one established line on the next note.
func (index *spanLyricIndex) record(r *contextReader, n *factNode) {
	if r.err != nil {
		return
	}
	identity, available := spanLyricIdentity(r, n, index.scratch)
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
	index.identities[n.header.ID] = spanLyricIdentityValue{key: identity, available: available}
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
}

func (index *spanLyricIndex) endpoint(r *contextReader, lyric *factNode) (string, Evidence[bool]) {
	if index == nil || lyric == nil {
		return spanLyricIdentity(r, lyric, nil)
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
