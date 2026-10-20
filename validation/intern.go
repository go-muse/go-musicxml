package validation

import "strings"

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
func (s *Session) ownAttribute(v AttributeFact) AttributeFact {
	v.Location.Target = Target(s.own(string(v.Location.Target)))
	v.Location.DocumentID = s.own(v.Location.DocumentID)
	v.Location.Entry = s.own(v.Location.Entry)
	v.Location.Path = strings.Clone(v.Location.Path)
	if v.Location.Position != nil {
		p := *v.Location.Position
		p.CoordinateSpace = s.own(p.CoordinateSpace)
		v.Location.Position = &p
	}
	v.Declaration = ownEvidence(s, v.Declaration)
	v.Declaration.Value = DeclarationID{Assembly: Assembly(s.own(string(v.Declaration.Value.Assembly))), OwnerOccurrence: s.own(v.Declaration.Value.OwnerOccurrence), Occurrence: s.own(v.Declaration.Value.Occurrence)}
	v.Presence = ownEvidence(s, v.Presence)
	v.Scalar.Lexeme = ownEvidence(s, v.Scalar.Lexeme)
	v.Scalar.Lexeme.Value = strings.Clone(v.Scalar.Lexeme.Value)
	v.Scalar.Value = ownEvidence(s, v.Scalar.Value)
	v.Scalar.Value.Value = s.ownAtomic(v.Scalar.Value.Value)
	v.Name = QName{s.own(v.Name.Space), s.own(v.Name.Local)}
	return v
}
