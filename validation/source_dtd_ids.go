package validation

import (
	"context"
	"reflect"
	"strings"
)

// SourceDTDIDDeclaration describes an actually read, effective first ATTLIST
// ID binding. Its lexical names are resolved before XML namespace expansion.
// Required/implied are the legal ID default forms; other forms retain typing
// evidence but never establish DTD validity or complete selection authority.
type SourceDTDIDDeclaration struct {
	Sequence           uint64
	Element, Attribute string
	DefaultKind        string
	Origin             Evidence[Location]
}

// SourceDTDIDAttribute is one exact source attribute occurrence. IsID is known
// false only when a complete declaration domain excludes ID typing. Unknown
// classification must have a reason. Value is the actual XML/DTD-normalized
// input, before any XSD whitespace transformation. Declaration is present only
// for known ID typing. Namespace declaration attributes are excluded.
type SourceDTDIDAttribute struct {
	Subject     NodeRef
	Attribute   QName
	Value       string
	IsID        Evidence[bool]
	Declaration *SourceDTDIDDeclaration
	Specified   Evidence[bool]
	Use         Evidence[Location]
}

// SourceDTDIDDomain seals the actually read attribute-declaration domain,
// independently of occurrence enumeration and XSD identity validity.
type SourceDTDIDDomain struct {
	DocumentID string
	Complete   Evidence[bool]
}

type sourceDTDIDs struct {
	Uncertain    bool
	Domain       Evidence[bool]
	Complete     Evidence[bool]
	Seen         map[sourceOriginSubject]bool
	Declarations map[uint64]*SourceDTDIDDeclaration
	Bindings     map[string]uint64
	IDs          []SourceDTDIDAttribute
}

// sourceDTDIDsSelected reports whether this session's admitted root assembly
// elects effective DTD ID occurrence input. It is false before root admission,
// for current-model sessions, and for an unselected active assembly. This is a
// read-only capability query; it neither admits input nor asserts completion.
func (s *Session) sourceDTDIDsSelected() bool {
	return s != nil && s.catalog != nil && s.root != nil && s.profile.Target == TargetSource && s.fragmentMode() != fragmentLegacy && s.schema != nil && s.schema.active != nil && s.catalog.fragmentDTDs[s.profile.ID][s.schema.active.assembly] && s.catalog.sourceDeclarations[s.profile.ID][s.schema.active.assembly] == xsdSourceDeclarationsV1
}

// sourceDTDIDsSelected reports whether the immutable selected source profile
// elects effective DTD ID occurrence input. It does not claim completed input.
func (e *Engine) sourceDTDIDsSelected(profile ProfileID) bool {
	if e == nil || e.catalog == nil || e.catalog.profiles[profile].Target != TargetSource {
		return false
	}
	for _, selected := range e.catalog.fragmentDTDs[profile] {
		if selected {
			return true
		}
	}
	return false
}

// sourceDTDIDDomain admits exactly one owned declaration-domain seal immediately
// after root Open and before any root attribute. Missing input is unavailable.
func (s *Session) sourceDTDIDDomain(ctx context.Context, f SourceDTDIDDomain) (err error) {
	if err = s.event(ctx, "SourceDTDIDDomain"); err != nil {
		return err
	}
	if !s.sourceDTDIDsSelected() {
		return s.fail(contract("source DTD IDs", "selected source fragment authority required"))
	}
	if s.root == nil || len(s.stack) != 1 || s.stack[0] != s.root || len(s.sequence) != 1 || len(s.root.attributes) != 0 || s.root.attributeSeal != nil || s.dtdIDs != nil {
		return s.fail(&SessionError{Operation: "SourceDTDIDDomain", Reason: "one domain before root attributes required"})
	}
	if f.DocumentID != s.documentID {
		return s.fail(contract("source DTD IDs", "document mismatch"))
	}
	if err = sourceDTDIDSeal(f.Complete); err != nil {
		return s.fail(err)
	}
	if err = s.step(ctx, int64(len(f.Complete.Reason))+1); err != nil {
		return s.fail(err)
	}
	if err = s.retain(1536 + int64(len(f.Complete.Reason))); err != nil {
		return s.fail(err)
	}
	s.dtdIDs = &sourceDTDIDs{Domain: cloneEvidence(f.Complete), Complete: stateUnknown[bool]("source-dtd-id-occurrences-unsealed"), Seen: make(map[sourceOriginSubject]bool), Declarations: make(map[uint64]*SourceDTDIDDeclaration), Bindings: make(map[string]uint64)}
	return nil
}
func sourceDTDIDSeal(e Evidence[bool]) error {
	if len(e.Reason) > maxIdentityBytes || e.Cause != nil {
		return contract("source DTD ID seal", "bounded cause-free evidence required")
	}
	if err := validateEvidence(e, "source DTD ID seal"); err != nil {
		return err
	}
	if e.State != EvidenceKnown && e.State != EvidenceUnknown && e.State != EvidenceUnsupported || !knownComplete(e) && !validID(string(e.Reason)) {
		return contract("source DTD ID seal", "explicit completeness and reason required")
	}
	return nil
}

// sourceDTDIDAttribute admits the exact most recently supplied attribute while
// its owner is open. Every non-namespace source occurrence must be marked before
// CompleteSourceDTDIDs can assert full enumeration. All retained data is copied.
func (s *Session) sourceDTDIDAttribute(ctx context.Context, f SourceDTDIDAttribute) (err error) {
	if err = s.event(ctx, "SourceDTDIDAttribute"); err != nil {
		return err
	}
	if !s.sourceDTDIDsSelected() || s.dtdIDs == nil {
		return s.fail(contract("source DTD IDs", "selected and admitted domain required"))
	}
	if len(s.stack) == 0 || knownComplete(s.dtdIDs.Complete) {
		return s.fail(&SessionError{Operation: "SourceDTDIDAttribute", Reason: "open attribute occurrence required"})
	}
	n := s.stack[len(s.stack)-1]
	if f.Subject != (NodeRef{s.documentID, n.header.ID}) || n.attributeSeal != nil || len(n.attributes) == 0 || !validQName(f.Attribute) || f.Attribute.Space == "http://www.w3.org/2000/xmlns/" {
		return s.fail(contract("source DTD ID occurrence", "exact non-namespace open owner required"))
	}
	if err = s.account.Check(BudgetScalarBytes, int64(len(f.Value))); err != nil {
		return s.fail(err)
	}
	if err = s.step(ctx, int64(len(f.Value))+1); err != nil {
		return s.fail(err)
	}
	a := n.attributes[len(n.attributes)-1]
	if a.Name != f.Attribute || a.Presence.State != EvidenceKnown || !a.Presence.Value || a.Scalar.Lexeme.Value != f.Value {
		return s.fail(contract("source DTD ID occurrence", "exact admitted attribute input required"))
	}
	if len(f.IsID.Reason) > maxIdentityBytes || f.IsID.Cause != nil || f.IsID.State != EvidenceKnown && f.IsID.State != EvidenceUnknown && f.IsID.State != EvidenceUnsupported {
		return s.fail(contract("source DTD ID classification", "bounded cause-free typing evidence required"))
	}
	if err = validateEvidence(f.IsID, "source DTD ID classification"); err != nil {
		return s.fail(err)
	}
	if f.IsID.State == EvidenceKnown && !f.IsID.Value && !knownComplete(s.dtdIDs.Domain) {
		return s.fail(contract("source DTD ID occurrence", "negative classification requires complete declarations"))
	}
	slot := sourceOriginSubject{Node: f.Subject.Node, Attribute: f.Attribute}
	if s.dtdIDs.Seen[slot] {
		return s.fail(contract("source DTD ID occurrence", "duplicate attribute classification"))
	}
	if err = sourceDTDIDLocation(f.Use, s.documentID); err != nil {
		return s.fail(err)
	}
	if err = validateEvidence(f.Specified, "source DTD ID specifiedness"); err != nil {
		return s.fail(err)
	}
	if f.Specified.State != EvidenceKnown || f.Specified.Cause != nil || len(f.Specified.Reason) > maxIdentityBytes {
		return s.fail(contract("source DTD ID specifiedness", "known source occurrence required"))
	}
	origin := s.sourceOriginSeen[slot]
	if origin == nil || origin.Specified.State != EvidenceKnown || origin.Specified.Value != f.Specified.Value {
		return s.fail(contract("source DTD ID specifiedness", "matching admitted source origin required"))
	}
	expectedUse := origin.Use
	if origin.Physical.State == EvidenceKnown {
		expectedUse = origin.Physical
	}
	if !sourceDTDIDSameSite(f.Use, expectedUse) {
		return s.fail(contract("source DTD ID use", "must match the admitted physical or synthetic use site"))
	}
	positive := f.IsID.State == EvidenceKnown && f.IsID.Value
	if positive != (f.Declaration != nil) {
		return s.fail(contract("source DTD ID declaration", "declaration required exactly for positive typing"))
	}
	key := ""
	if d := f.Declaration; d != nil {
		if d.Sequence == 0 || len(d.Element) > 2*maxIdentityBytes || len(d.Attribute) > 2*maxIdentityBytes {
			return s.fail(contract("source DTD ID declaration", "bounded positive first-binding identity required"))
		}
		if err = s.sourceDTDIDLexical(ctx, d.Element, n.header.Name, n.header.Bindings, true); err != nil {
			return s.fail(err)
		}
		if err = s.sourceDTDIDLexical(ctx, d.Attribute, f.Attribute, n.header.Bindings, false); err != nil {
			return s.fail(err)
		}
		switch d.DefaultKind {
		case "required", "implied", "value", "fixed":
		default:
			return s.fail(contract("source DTD ID declaration", "exact default kind required"))
		}
		if err = sourceDTDIDLocation(d.Origin, s.documentID); err != nil {
			return s.fail(err)
		}
		if !sourceDTDIDSameSite(d.Origin, origin.Declaration) {
			return s.fail(contract("source DTD ID declaration origin", "must match admitted declaration site"))
		}
		if err = s.step(ctx, int64(len(d.Element))+int64(len(d.Attribute))+1); err != nil {
			return s.fail(err)
		}
		// The separator is not an XML Name character; no lexical key ambiguity.
		if err = s.retain(int64(len(d.Element)) + int64(len(d.Attribute)) + 1); err != nil {
			return s.fail(err)
		}
		key = d.Element + "\x00" + d.Attribute
		if old := s.dtdIDs.Declarations[d.Sequence]; old != nil && (old.Element != d.Element || old.Attribute != d.Attribute || old.DefaultKind != d.DefaultKind) {
			return s.fail(contract("source DTD ID declaration", "sequence reused for another first binding"))
		}
		if old := s.dtdIDs.Bindings[key]; old != 0 && old != d.Sequence {
			return s.fail(contract("source DTD ID declaration", "lexical binding reclassified"))
		}
	}
	size, err := resourceMeasure(ctx, s.account, reflect.ValueOf(f))
	if err != nil {
		return s.fail(err)
	}
	size, err = rationalCheckedAdd(BudgetBufferedBytes, size, 1024)
	if err != nil {
		return s.fail(err)
	}
	if err = s.retain(size); err != nil {
		return s.fail(err)
	}
	var owned SourceDTDIDAttribute
	if err = resourceCopyInto(ctx, reflect.ValueOf(&owned).Elem(), reflect.ValueOf(f), "", ""); err != nil {
		return s.fail(err)
	}
	// Conservative per-event allowance pays for map growth and candidate slice
	// capacity before either allocation, including repeated declaration metadata.
	s.dtdIDs.Seen[sourceOriginSubject{Node: slot.Node, Attribute: owned.Attribute}] = true
	if owned.IsID.State != EvidenceKnown || owned.Declaration != nil && owned.Declaration.DefaultKind != "required" && owned.Declaration.DefaultKind != "implied" {
		s.dtdIDs.Uncertain = true
	}
	if positive {
		s.dtdIDs.Declarations[owned.Declaration.Sequence] = owned.Declaration
		s.dtdIDs.Bindings[key] = owned.Declaration.Sequence
		s.dtdIDs.IDs = append(s.dtdIDs.IDs, owned)
	}
	return nil
}
func sourceDTDIDLocation(e Evidence[Location], doc string) error {
	if _, err := sourceEntityOriginBytes(e); err != nil {
		return err
	}
	if err := validateEvidence(e, "source DTD ID location"); err != nil {
		return err
	}
	if e.State == EvidenceKnown {
		if e.Value.Position == nil {
			return contract("source DTD ID location", "known original position required")
		}
		return validateLocation(e.Value, TargetSource, doc)
	}
	if e.State != EvidenceUnknown && e.State != EvidenceAbsent {
		return contract("source DTD ID location", "known, absent or unavailable required")
	}
	return nil
}
func (s *Session) sourceDTDIDLexical(ctx context.Context, lexical string, want QName, bindings []Binding, element bool) error {
	field := "source DTD ID declaration attribute"
	if element {
		field = "source DTD ID declaration element"
	}
	prefix, local, qualified := strings.Cut(lexical, ":")
	if !qualified {
		local, prefix = prefix, ""
	}
	if err := s.validateSourceNCName(ctx, local, field); err != nil {
		return err
	}
	if qualified {
		if err := s.validateSourceNCName(ctx, prefix, field); err != nil {
			return err
		}
	}
	space := ""
	if element || qualified {
		for _, b := range bindings {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			if b.Prefix == prefix {
				space = b.URI
				break
			}
		}
	}
	if qualified && space == "" || want != (QName{Space: space, Local: local}) {
		return contract("source DTD ID lexical binding", "lexical declaration does not name admitted occurrence")
	}
	return nil
}

// completeSourceDTDIDs seals observed occurrences after root closure. It never
// fills absent classifications and is called by the source adapter only at EOF.
func (s *Session) completeSourceDTDIDs(ctx context.Context, doc string) error {
	if err := s.event(ctx, "CompleteSourceDTDIDs"); err != nil {
		return err
	}
	if !s.sourceDTDIDsSelected() || s.dtdIDs == nil || doc != s.documentID || s.root == nil || !s.root.closed || len(s.stack) != 0 || knownComplete(s.dtdIDs.Complete) {
		return s.fail(&SessionError{Operation: "CompleteSourceDTDIDs", Reason: "one selected closed-document seal required"})
	}
	for _, id := range s.sequence {
		n := s.nodes[id]
		for _, a := range n.attributes {
			if err := s.step(ctx, 1); err != nil {
				return s.fail(err)
			}
			if a.Name.Space == "http://www.w3.org/2000/xmlns/" {
				continue
			}
			if !s.dtdIDs.Seen[sourceOriginSubject{Node: id, Attribute: a.Name}] {
				return s.fail(contract("source DTD ID occurrences", "missing admitted attribute classification"))
			}
		}
	}
	s.dtdIDs.Complete = fragmentKnown(true)
	return nil
}

func sourceDTDIDSameSite(a, b Evidence[Location]) bool {
	if a.State != b.State {
		return false
	}
	if a.State != EvidenceKnown {
		return true
	}
	x, y := a.Value, b.Value
	if x.Target != y.Target || x.DocumentID != y.DocumentID || x.Entry != y.Entry || x.Path != y.Path || (x.Position == nil) != (y.Position == nil) {
		return false
	}
	return x.Position == nil || *x.Position == *y.Position
}
