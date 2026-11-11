package validation

import (
	"math"
	"strings"
)

// own returns a session-owned copy of v that is shared by every equal value
// published by this session. The first occurrence is copied, so the result
// never aliases caller storage; later occurrences reuse that copy. Engine
// constants (evidence states, targets, coordinate spaces) and the session's
// own document ID are returned without a table entry.
func (s *Session) own(v string) string {
	if v == "" {
		return ""
	}
	if s == nil {
		return strings.Clone(v)
	}
	if v == s.documentID {
		return s.documentID
	}
	switch v {
	case string(TargetSource):
		return string(TargetSource)
	case string(TargetModel):
		return string(TargetModel)
	case string(TargetPackage):
		return string(TargetPackage)
	case CoordinateSpaceOriginalBytes:
		return CoordinateSpaceOriginalBytes
	case CoordinateSpaceExternalDTD:
		return CoordinateSpaceExternalDTD
	case string(AtomicString):
		return string(AtomicString)
	case string(AtomicBoolean):
		return string(AtomicBoolean)
	case string(AtomicInteger):
		return string(AtomicInteger)
	case string(AtomicDecimal):
		return string(AtomicDecimal)
	case string(AtomicQName):
		return string(AtomicQName)
	}
	if owned, ok := s.strings[v]; ok {
		return owned
	}
	// The table owns its copy beyond any one publication, so a new entry is
	// charged its slot and string as session retention that rollbacks keep
	// (internRetained): a publication or span evaluation that rolls back
	// leaves the entry and its charge until cleanup. When the account cannot
	// admit the entry, the value is copied without one, as before interning.
	owned := strings.Clone(v)
	charge := internSlotBytes + ownedStringBytes(owned)
	if s.strings == nil {
		charge += internTableBytes
	}
	if s.account == nil || s.retain(charge) != nil {
		return owned
	}
	s.internRetained += charge
	if s.strings == nil {
		s.strings = make(map[string]string)
	}
	s.strings[owned] = owned
	return owned
}

// internSlotBytes bounds one map entry of the intern table (two string
// headers, hash metadata and growth); internTableBytes its header.
const (
	internSlotBytes  int64 = 96
	internTableBytes int64 = 256
)

// retainedMark is the rollback point of a transactional operation.
type retainedMark struct{ retained, intern int64 }

func (s *Session) markRetained() retainedMark {
	return retainedMark{s.retained, s.internRetained}
}

// rollbackRetained releases what the session retained since m, except the
// intern table's new entries, which stay owned and charged until cleanup.
func (s *Session) rollbackRetained(m retainedMark) {
	kept := s.internRetained - m.intern
	if delta := s.retained - m.retained - kept; delta > 0 {
		_ = s.account.Release(BudgetBufferedBytes, delta)
		s.retained -= delta
	}
}

// ownKey returns the catalog's own copy of a catalog rule key.
func (s *Session) ownKey(k RuleKey) RuleKey {
	if s != nil && s.catalog != nil {
		if r, ok := s.catalog.rules[k]; ok && r.Key == k {
			return r.Key
		}
	}
	return RuleKey{ID: RuleID(s.own(string(k.ID))), Revision: k.Revision}
}

func (s *Session) ownLocation(v Location) Location {
	v.Target = Target(s.own(string(v.Target)))
	v.DocumentID = s.own(v.DocumentID)
	v.Entry = s.own(v.Entry)
	v.Path = s.own(v.Path)
	if v.Position != nil {
		p := *v.Position
		p.CoordinateSpace = s.own(p.CoordinateSpace)
		v.Position = &p
	}
	return v
}

func ownEvidence[T any](s *Session, v Evidence[T]) Evidence[T] {
	v = cloneEvidenceState(v)
	if v.Cause != nil {
		c := *v.Cause
		c.Rule = s.ownKey(c.Rule)
		c.Subject.DocumentID = s.own(c.Subject.DocumentID)
		v.Cause = &c
	}
	v.Reason = Code(s.own(string(v.Reason)))
	return v
}

func (s *Session) ownAtomic(v Atomic) Atomic {
	v.Kind = AtomicKind(s.own(string(v.Kind)))
	v.Text = s.own(v.Text)
	v.Name.Space = s.own(v.Name.Space)
	v.Name.Local = s.own(v.Name.Local)
	return v
}

func cloneEvidenceState[T any](v Evidence[T]) Evidence[T] {
	switch v.State {
	case EvidenceKnown:
		v.State = EvidenceKnown
	case EvidenceAbsent:
		v.State = EvidenceAbsent
	case EvidenceUnknown:
		v.State = EvidenceUnknown
	case EvidenceUnsupported:
		v.State = EvidenceUnsupported
	case EvidenceInvalid:
		v.State = EvidenceInvalid
	default:
		v.State = EvidenceState(strings.Clone(string(v.State)))
	}
	return v
}

// ownAttribute is cloneAttribute with shared closed-vocabulary strings. The
// attribute path and lexeme stay private copies.
func (s *Session) ownAttribute(n *factNode, v AttributeFact) retainedAttribute {
	target := Target(s.own(string(v.Location.Target)))
	docID := s.own(v.Location.DocumentID)
	var anchor string
	if n != nil {
		anchor = n.header.loc.pathPrefix
	}
	loc := s.buildRetainedLocation(target, docID, v.Location, nil, anchor)
	decl := ownEvidence(s, v.Declaration)
	decl.Value = DeclarationID{Assembly: Assembly(s.own(string(v.Declaration.Value.Assembly))), OwnerOccurrence: s.own(v.Declaration.Value.OwnerOccurrence), Occurrence: s.own(v.Declaration.Value.Occurrence)}
	var declPtr *Evidence[DeclarationID]
	if s.facts != nil && decl == s.facts.pendingDeclaration {
		declPtr = &s.facts.pendingDeclaration
	} else if s.facts != nil && decl == s.facts.modelDeclaration {
		declPtr = &s.facts.modelDeclaration
	} else {
		declPtr = &decl
	}
	pres := ownEvidence(s, v.Presence)
	var presPtr *Evidence[bool]
	if s.facts != nil && pres == s.facts.complete {
		presPtr = &s.facts.complete
	} else if s.facts != nil && pres == s.facts.incomplete {
		presPtr = &s.facts.incomplete
	} else if s.facts != nil && pres == s.facts.absentBool {
		presPtr = &s.facts.absentBool
	} else {
		presPtr = &pres
	}
	lex := ownEvidence(s, v.Scalar.Lexeme)
	var lexPtr *Evidence[string]
	var lexBytes int32
	if s.facts != nil && lex == s.facts.modelLexeme {
		lexPtr = &s.facts.modelLexeme
	} else if s.facts != nil && lex == s.facts.absentLexeme {
		lexPtr = &s.facts.absentLexeme
	} else if s.facts != nil && s.facts.emptyKnownLexeme != nil && lex == *s.facts.emptyKnownLexeme {
		lexPtr = s.facts.emptyKnownLexeme
	} else {
		lex.Value = strings.Clone(v.Scalar.Lexeme.Value)
		lexPtr = &lex
		if b := int64(64 + len(lex.Value)); b <= math.MaxInt32 {
			lexBytes = int32(b)
		}
	}
	val := ownEvidence(s, v.Scalar.Value)
	val.Value = s.ownAtomic(v.Scalar.Value.Value)
	var valPtr *Evidence[Atomic]
	var atomicBytes int32
	if s.facts != nil && atomicEvidenceIdentical(val, s.facts.pendingAtomic) {
		valPtr = &s.facts.pendingAtomic
	} else if s.facts != nil && atomicEvidenceIdentical(val, s.facts.absentAtomic) {
		valPtr = &s.facts.absentAtomic
	} else {
		valPtr = &val
		if b := 128 + atomicLogicalBytes(val.Value); b <= math.MaxInt32 {
			atomicBytes = int32(b)
		}
	}
	return retainedAttribute{
		Name:                QName{s.own(v.Name.Space), s.own(v.Name.Local)},
		Declaration:         declPtr,
		Presence:            presPtr,
		Scalar:              retainedScalar{Lexeme: lexPtr, Value: valPtr},
		loc:                 loc,
		lexemeRetainedBytes: lexBytes,
		atomicRetainedBytes: atomicBytes,
	}
}
