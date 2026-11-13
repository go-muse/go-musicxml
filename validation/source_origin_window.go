package validation

// Under the windowed strategy the source origin index keeps a full origin
// only while its node is open: the readers that need one, the DTD ID and
// attribute lexical observations, accept only an open subject. Once the node
// closes the session keeps the count of origins and, for each attribute, its
// specifiedness, which is all attribute evidence reads later. The buffered
// strategy keeps every full origin until the session ends.

// sourceOriginOpenNode holds the full origins of one open node, and what
// their owned copies retained.
type sourceOriginOpenNode struct {
	element  *SourceOriginContextResult
	attrs    map[QName]*SourceOriginContextResult
	retained int64
}

// Compact specifiedness of a closed attribute's origin; any other evidence
// is kept whole in an overflow map.
const (
	sourceOriginSpecifiedTrue uint8 = iota + 1
	sourceOriginSpecifiedFalse
	sourceOriginSpecifiedOther
)

// sourceOriginOpenBytes charges an open node's record and its attribute map.
const sourceOriginOpenBytes int64 = 512

func (s *Session) sourceOriginWindowed() bool {
	return s.strategy == strategyWindowed && !sourceOriginWhole
}

// sourceOriginWhole is a test switch only: it keeps every origin whole under
// the windowed strategy too, so that a test can tell apart what else the
// strategy keeps.
var sourceOriginWhole bool

func setSourceOriginWhole(whole bool) bool {
	previous := sourceOriginWhole
	sourceOriginWhole = whole
	return previous
}

// sourceOriginExists reports whether slot already has an origin.
func (s *Session) sourceOriginExists(slot sourceOriginSubject) bool {
	if !s.sourceOriginWindowed() {
		_, exists := s.sourceOriginSeen[slot]
		return exists
	}
	if slot.Attribute == (QName{}) {
		open := s.sourceOriginOpen[slot.Node]
		return open != nil && open.element != nil
	}
	_, exists := s.sourceOriginSpecified[slot]
	return exists
}

// sourceOriginFull is the full origin of slot, which an open node keeps.
func (s *Session) sourceOriginFull(slot sourceOriginSubject) *SourceOriginContextResult {
	if !s.sourceOriginWindowed() {
		return s.sourceOriginSeen[slot]
	}
	open := s.sourceOriginOpen[slot.Node]
	if open == nil {
		return nil
	}
	if slot.Attribute == (QName{}) {
		return open.element
	}
	return open.attrs[slot.Attribute]
}

// sourceOriginSpecifiedOf is the specifiedness of slot's origin, and whether
// it has one.
func (s *Session) sourceOriginSpecifiedOf(slot sourceOriginSubject) (Evidence[bool], bool) {
	if full := s.sourceOriginFull(slot); full != nil {
		return full.Specified, true
	}
	if !s.sourceOriginWindowed() {
		return Evidence[bool]{}, false
	}
	switch s.sourceOriginSpecified[slot] {
	case sourceOriginSpecifiedTrue:
		return Evidence[bool]{State: EvidenceKnown, Value: true}, true
	case sourceOriginSpecifiedFalse:
		return Evidence[bool]{State: EvidenceKnown, Value: false}, true
	case sourceOriginSpecifiedOther:
		return s.sourceOriginSpecifiedOverflow[slot], true
	}
	return Evidence[bool]{}, false
}

// keepSourceOrigin indexes owned as slot's origin, which retained bytes for
// its own copy.
func (s *Session) keepSourceOrigin(slot sourceOriginSubject, owned *SourceOriginContextResult, retained int64) error {
	if !s.sourceOriginWindowed() {
		if s.sourceOriginSeen == nil {
			s.sourceOriginSeen = make(map[sourceOriginSubject]*SourceOriginContextResult)
		}
		s.sourceOriginSeen[slot] = owned
		s.sourceOriginCount++
		return nil
	}
	// Every charge comes first: a refused one leaves no entry behind. An
	// attribute's open entry is a second map entry besides its compact
	// specifiedness, released with the open record.
	var attrEntry int64
	if slot.Attribute != (QName{}) {
		attrEntry = sourceOriginIndexEntryBytes
		if err := s.retain(attrEntry); err != nil {
			return err
		}
		if !(owned.Specified.State == EvidenceKnown && owned.Specified.Reason == "" && owned.Specified.Cause == nil) {
			if err := s.retain(sourceOriginOverflowBytes + evidenceLogicalBytes(owned.Specified)); err != nil {
				return err
			}
		}
	}
	open := s.sourceOriginOpen[slot.Node]
	if open == nil {
		if err := s.retain(sourceOriginOpenBytes); err != nil {
			return err
		}
		if s.sourceOriginOpen == nil {
			s.sourceOriginOpen = make(map[NodeID]*sourceOriginOpenNode)
		}
		open = &sourceOriginOpenNode{}
		s.sourceOriginOpen[slot.Node] = open
	}
	s.sourceOriginCount++
	open.retained += retained + attrEntry
	if slot.Attribute == (QName{}) {
		open.element = owned
		return nil
	}
	if open.attrs == nil {
		open.attrs = make(map[QName]*SourceOriginContextResult)
	}
	open.attrs[slot.Attribute] = owned
	code := sourceOriginSpecifiedOther
	if e := owned.Specified; e.State == EvidenceKnown && e.Reason == "" && e.Cause == nil {
		code = sourceOriginSpecifiedFalse
		if e.Value {
			code = sourceOriginSpecifiedTrue
		}
	}
	if s.sourceOriginSpecified == nil {
		s.sourceOriginSpecified = make(map[sourceOriginSubject]uint8)
	}
	s.sourceOriginSpecified[slot] = code
	if code == sourceOriginSpecifiedOther {
		if s.sourceOriginSpecifiedOverflow == nil {
			s.sourceOriginSpecifiedOverflow = make(map[sourceOriginSubject]Evidence[bool])
		}
		s.sourceOriginSpecifiedOverflow[slot] = cloneEvidence(owned.Specified)
	}
	return nil
}

// sourceOriginOverflowBytes charges one overflow entry and its cause.
const sourceOriginOverflowBytes int64 = 256

// closeSourceOrigins drops the full origins of n when it closes, and
// releases what their owned copies retained.
func (s *Session) closeSourceOrigins(n *factNode) {
	if !s.sourceOriginWindowed() || s.sourceOriginOpen == nil {
		return
	}
	open := s.sourceOriginOpen[n.header.ID]
	if open == nil {
		return
	}
	delete(s.sourceOriginOpen, n.header.ID)
	s.releaseFactBytes(open.retained + sourceOriginOpenBytes)
}
