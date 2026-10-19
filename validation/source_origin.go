package validation

import "context"

// SourceOriginKind distinguishes physical occurrences from DTD-provided values.
type SourceOriginKind string

// Source origin kinds distinguish the encoded occurrence from its effective value.
const (
	SourceOriginPhysical    SourceOriginKind = "physical"
	SourceOriginDefault     SourceOriginKind = "default"
	SourceOriginEntity      SourceOriginKind = "entity"
	SourceOriginUnavailable SourceOriginKind = "unavailable"
	// SourceOriginAncestryLimit bounds the selected source-origin transport profile.
	// An adapter must report incomplete ancestry explicitly, never silently truncate.
	SourceOriginAncestryLimit = 64
	// ContextPropertySourceOrigin identifies one exact source occurrence's origin.
	ContextPropertySourceOrigin ContextProperty = "source.origin"
)

// SourceOriginStep is one outermost-first entity expansion in the origin chain.
// Declaration and Use are separate physical sites and may be independently unknown.
type SourceOriginStep struct {
	Entity      string
	Declaration Evidence[Location]
	Use         Evidence[Location]
}

// SourceOriginContextResult retains the origin of one admitted source occurrence.
// Attribute nil denotes the element; otherwise it names the exact expanded slot.
// Physical describes the occurrence itself, never a replacement declaration or
// reference site. Synthetic occurrences have no physical Position in their fact
// Location; Declaration and Use identify their real sites separately. Specified
// means an original physical occurrence: false for both DTD defaults and entity-
// generated markup, true for physical attributes whose values expand entities.
// Unlike Infoset [specified], this is false for attributes written in entity
// replacement markup. Entity-kind physical origins are permitted only for
// attributes whose values expand entities; element entity origins are synthetic.
// Default origins carry no occurrence-expansion ancestry. This is distinct from
// effective AttributeFact.Presence. Each component's availability is independent;
// unknown is never inferred as absence or a default.
type SourceOriginContextResult struct {
	Subject          NodeRef
	Attribute        *QName `json:",omitempty"`
	Kind             SourceOriginKind
	Specified        Evidence[bool]
	Physical         Evidence[Location]
	Declaration      Evidence[Location]
	Use              Evidence[Location]
	Ancestry         []SourceOriginStep
	AncestryComplete Evidence[bool]
}

// The index borrows only owned projections. Entry admission includes overlapping
// map growth; payload storage is separately charged by context publication.
const sourceOriginIndexEntryBytes int64 = 128
const sourceOriginIndexHeaderBytes int64 = 512

type sourceOriginSubject struct {
	Node      NodeID
	Attribute QName
}

func sourceOriginSite(ctx context.Context, s *Session, v Evidence[Location], doc string) error {
	if err := s.step(ctx, 1+locationLogicalBytes(v.Value)+evidenceLogicalBytes(v)); err != nil {
		return err
	}
	if err := validateEvidence(v, "source origin site"); err != nil {
		return err
	}
	if v.State == EvidenceKnown {
		if err := validateLocation(v.Value, TargetSource, doc); err != nil {
			return err
		}
		if v.Value.Position == nil {
			return contract("source origin site", "known physical site requires real coordinates")
		}
	}
	return nil
}
func validateSourceOrigin(ctx context.Context, s *Session, in *SourceOriginContextResult, subject NodeRef) error {
	if in == nil || in.Subject != subject || !validID(in.Subject.DocumentID) || in.Subject.Node == 0 {
		return contract("source origin", "exact result subject required")
	}
	if len(in.Ancestry) > SourceOriginAncestryLimit {
		return contract("source origin ancestry", "profile capacity exceeded; explicit unavailable ancestry required")
	}
	if err := s.step(ctx, 1+int64(len(subject.DocumentID))+evidenceLogicalBytes(in.Specified)+evidenceLogicalBytes(in.AncestryComplete)); err != nil {
		return err
	}
	if in.Attribute != nil {
		if err := s.step(ctx, 1+int64(len(in.Attribute.Space))+int64(len(in.Attribute.Local))); err != nil {
			return err
		}
	}
	if in.Attribute != nil && !validQName(*in.Attribute) {
		return contract("source origin attribute", "exact expanded name required")
	}
	if err := validateEvidence(in.Specified, "source origin specified"); err != nil {
		return err
	}
	if err := validateEvidence(in.AncestryComplete, "source origin ancestry completeness"); err != nil {
		return err
	}
	if in.Specified.State == EvidenceAbsent || in.AncestryComplete.State == EvidenceAbsent {
		return contract("source origin", "specifiedness and completeness cannot be absent")
	}
	for _, v := range []Evidence[Location]{in.Physical, in.Declaration, in.Use} {
		if err := sourceOriginSite(ctx, s, v, subject.DocumentID); err != nil {
			return err
		}
	}
	if in.Specified.State == EvidenceKnown && ((in.Physical.State == EvidenceKnown && !in.Specified.Value) || (in.Physical.State == EvidenceAbsent && in.Specified.Value)) {
		return contract("source origin specified", "physical occurrence and specifiedness contradict")
	}
	if in.AncestryComplete.State == EvidenceKnown && !in.AncestryComplete.Value && !validID(string(in.AncestryComplete.Reason)) {
		return contract("source origin ancestry", "incomplete ancestry requires an explicit reason")
	}
	switch in.Kind {
	case SourceOriginPhysical:
		if in.Specified.State != EvidenceKnown || !in.Specified.Value || in.Physical.State != EvidenceKnown || len(in.Ancestry) != 0 || in.AncestryComplete.State != EvidenceKnown || !in.AncestryComplete.Value {
			return contract("source origin physical", "physical origin contradicts specifiedness, sites, or ancestry")
		}
	case SourceOriginDefault:
		if in.Attribute == nil || in.Specified.State != EvidenceKnown || in.Specified.Value || in.Physical.State == EvidenceKnown || in.Declaration.State == EvidenceAbsent || in.Use.State == EvidenceAbsent || len(in.Ancestry) != 0 {
			return contract("source origin default", "attribute default requires unspecified, nonphysical occurrence, declaration evidence and no occurrence ancestry")
		}
	case SourceOriginEntity:
		if in.Specified.State != EvidenceKnown || in.Specified.Value && (in.Attribute == nil || in.Physical.State != EvidenceKnown) || !in.Specified.Value && in.Physical.State == EvidenceKnown {
			return contract("source origin entity", "entity markup must be unspecified and nonphysical; only a physical attribute value may be specified")
		}
		if in.AncestryComplete.State == EvidenceKnown && in.AncestryComplete.Value && len(in.Ancestry) == 0 {
			return contract("source origin entity", "complete entity ancestry must include its expansion")
		}
		if in.Declaration.State == EvidenceAbsent || in.Use.State == EvidenceAbsent {
			return contract("source origin entity", "declaration and use evidence required")
		}
	case SourceOriginUnavailable:
	default:
		return contract("source origin kind", "unknown kind")
	}
	for _, step := range in.Ancestry {
		if err := s.step(ctx, 1+int64(len(step.Entity))); err != nil {
			return err
		}
		if err := s.account.Check(BudgetScalarBytes, int64(len(step.Entity))); err != nil {
			return err
		}
		if !validID(step.Entity) {
			return contract("source origin ancestry", "exact entity name required")
		}
		if err := sourceOriginSite(ctx, s, step.Declaration, subject.DocumentID); err != nil {
			return err
		}
		if err := sourceOriginSite(ctx, s, step.Use, subject.DocumentID); err != nil {
			return err
		}
		if step.Declaration.State == EvidenceAbsent || step.Use.State == EvidenceAbsent {
			return contract("source origin ancestry", "declaration and use cannot be absent")
		}
	}
	return nil
}
func (s *Session) sourceOriginKey() RuleKey {
	if s == nil || s.catalog == nil || s.schema == nil || s.schema.active == nil {
		return RuleKey{}
	}
	return s.catalog.sourceOrigins[s.profile.ID][s.schema.active.assembly]
}
func sameSourceOriginLocation(a, b Location) bool {
	if a.Target != b.Target || a.DocumentID != b.DocumentID || a.Entry != b.Entry || a.Path != b.Path {
		return false
	}
	if a.Position == nil || b.Position == nil {
		return a.Position == nil && b.Position == nil
	}
	return *a.Position == *b.Position
}

// sourceOrigin admits one owned origin while its actual subject is open. The
// provider must be the selected available sealed source-origin plan for the
// active root assembly. Attribute origins follow the corresponding present
// Attribute event; schema declaration validity does not establish ownership.
func (s *Session) sourceOrigin(ctx context.Context, provider RuleKey, in SourceOriginContextResult) (err error) {
	if err = s.event(ctx, "SourceOrigin"); err != nil {
		return err
	}
	if s.profile.Target != TargetSource || provider != (s.sourceOriginKey()) || provider == (RuleKey{}) {
		return s.fail(contract("source origin provider", "matching selected source-only provider required"))
	}
	if len(s.stack) == 0 || s.sourceOriginSeal != nil || in.Subject != (NodeRef{s.documentID, s.stack[len(s.stack)-1].header.ID}) {
		return s.fail(&SessionError{Operation: "SourceOrigin", Reason: "currently open exact subject required"})
	}
	n := s.stack[len(s.stack)-1]
	location := n.header.Location
	slot := sourceOriginSubject{Node: in.Subject.Node}
	if in.Attribute != nil {
		slot.Attribute = *in.Attribute
		found := false
		for i := len(n.attributes) - 1; i >= 0; i-- {
			if err = s.step(ctx, 1); err != nil {
				return s.fail(err)
			}
			a := n.attributes[i]
			if a.Name == *in.Attribute {
				if a.Presence.State == EvidenceKnown && a.Presence.Value {
					location = a.Location
					found = true
				}
				break
			}
		}
		if !found {
			return s.fail(&SessionError{Operation: "SourceOrigin", Reason: "admitted present attribute required"})
		}
	}
	if _, exists := s.sourceOriginSeen[slot]; exists {
		return s.fail(&SessionError{Operation: "SourceOrigin", Reason: "duplicate origin for subject"})
	}
	publication := ContextResult{Provider: provider, Subject: in.Subject, Property: ContextPropertySourceOrigin, Value: Evidence[Atomic]{State: EvidenceAbsent}, SourceOrigin: &in}
	if err = s.validateContextResult(ctx, publication); err != nil {
		return s.fail(err)
	}
	if in.Physical.State == EvidenceKnown {
		if !sameSourceOriginLocation(in.Physical.Value, location) {
			return s.fail(contract("source origin physical", "physical site must identify the admitted occurrence"))
		}
	} else if location.Position != nil && (in.Kind != SourceOriginUnavailable || in.Physical.State == EvidenceAbsent || in.Specified.State == EvidenceKnown && !in.Specified.Value) {
		return s.fail(contract("source origin physical", "synthetic occurrence must not carry source coordinates"))
	}
	before := s.retained
	defer func() {
		if err != nil {
			if delta := s.retained - before; delta > 0 {
				_ = s.account.Release(BudgetBufferedBytes, delta)
				s.retained = before
			}
			s.fail(err)
		}
	}()
	cost := sourceOriginIndexEntryBytes
	if s.sourceOriginSeen == nil {
		cost += sourceOriginIndexHeaderBytes
	}
	if err = s.retain(cost); err != nil {
		return err
	}
	if err = s.publishValidatedContextResult(ctx, publication); err != nil {
		return err
	}
	var owned *SourceOriginContextResult
	if s.omitContext {
		// Without a projection the index owns its own copy.
		if owned, err = cloneSourceOriginContext(ctx, s, &in); err != nil {
			return err
		}
	} else {
		// Borrow only from the newly owned projection, never from the adapter.
		owned = s.contextResults[len(s.contextResults)-1].SourceOrigin
	}
	if s.sourceOriginSeen == nil {
		s.sourceOriginSeen = make(map[sourceOriginSubject]*SourceOriginContextResult)
	}
	if owned.Attribute != nil {
		slot.Attribute = *owned.Attribute
	}
	s.sourceOriginSeen[slot] = owned
	if !sourceOriginComponentsComplete(owned) {
		s.sourceOriginUnavailable = true
	}
	return nil
}

// completeSourceOrigins seals origin enumeration once after root closure. Known
// true requires every admitted node and present attribute to have an origin.
// Final assessment additionally requires complete input and fact enumeration.
func (s *Session) completeSourceOrigins(ctx context.Context, provider RuleKey, complete Evidence[bool]) error {
	if err := s.event(ctx, "CompleteSourceOrigins"); err != nil {
		return err
	}
	if provider == (RuleKey{}) || provider != s.sourceOriginKey() {
		return s.fail(contract("source origin provider", "matching selected provider required"))
	}
	if s.root == nil || !s.root.closed || len(s.stack) != 0 || s.sourceOriginSeal != nil {
		return s.fail(&SessionError{Operation: "CompleteSourceOrigins", Reason: "one seal after root closure required"})
	}
	if err := validateEvidence(complete, "source origin completeness"); err != nil {
		return s.fail(err)
	}
	if complete.State == EvidenceAbsent {
		return s.fail(contract("source origin completeness", "absence is not completeness"))
	}
	if complete.State == EvidenceKnown && complete.Value && uint64(len(s.sourceOriginSeen)) != s.sourceOriginExpected {
		return s.fail(contract("source origin completeness", "origins missing for admitted occurrences"))
	}
	if err := s.step(ctx, 1+evidenceLogicalBytes(complete)); err != nil {
		return s.fail(err)
	}
	if err := s.retain(128 + evidenceLogicalBytes(complete)); err != nil {
		return s.fail(err)
	}
	owned := cloneEvidence(complete)
	s.sourceOriginSeal = &owned
	return nil
}
func (s *Session) evaluateSourceOrigins(ctx context.Context) (Decision, bool, error) {
	if err := s.step(ctx, 1); err != nil {
		return Decision{}, false, err
	}
	if s.sourceOriginSeal == nil {
		return Decision{Outcome: OutcomeUnknown, Reason: "source-origin-enumeration-unavailable"}, false, nil
	}
	e := *s.sourceOriginSeal
	if e.State != EvidenceKnown {
		return evidenceDecision(e.State, e.Reason, e.Cause, false), false, nil
	}
	if s.sourceOriginUnavailable {
		return Decision{Outcome: OutcomeUnknown, Reason: "source-origin-components-unavailable"}, false, nil
	}
	if !e.Value || !s.inputComplete || s.enumerationIncomplete || s.attributeEnumerationIncomplete || s.sourceOriginUncertain {
		return Decision{Outcome: OutcomeUnknown, Reason: "source-origin-enumeration-incomplete"}, false, nil
	}
	return Decision{Outcome: OutcomePass}, false, nil
}

func sourceOriginComponentsComplete(in *SourceOriginContextResult) bool {
	if in.Kind == SourceOriginUnavailable || in.Specified.State != EvidenceKnown || in.AncestryComplete.State != EvidenceKnown || !in.AncestryComplete.Value {
		return false
	}
	if in.Physical.State != EvidenceKnown && in.Physical.State != EvidenceAbsent {
		return false
	}
	if in.Declaration.State != EvidenceKnown && in.Declaration.State != EvidenceAbsent || in.Use.State != EvidenceKnown && in.Use.State != EvidenceAbsent {
		return false
	}
	if in.Kind != SourceOriginPhysical && (in.Declaration.State != EvidenceKnown || in.Use.State != EvidenceKnown) {
		return false
	}
	for _, step := range in.Ancestry {
		if step.Declaration.State != EvidenceKnown || step.Use.State != EvidenceKnown {
			return false
		}
	}
	return true
}
