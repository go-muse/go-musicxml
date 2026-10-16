package validation

import (
	"context"
	"unicode/utf8"
)

type fragmentMode uint8

const (
	fragmentLegacy fragmentMode = iota
	fragmentJoinedShorthand
	fragmentFramework
)

// The selected fragment payload is separate from the frozen schema-ID index.
// DTD and XSD validity remain independent of first-document-order selection.
// All references below remain in the intrinsic document until adoption.
type resourceFragmentSnapshot struct {
	Mode                                                 fragmentMode
	DTDRequired                                          bool
	DTDDeclarations, DTDOccurrences, DTDIdentityComplete Evidence[bool]
	DTDIDs                                               []SourceDTDIDAttribute
	Elements                                             []resourceFragmentElement
	Complete                                             Evidence[bool]
}
type resourceFragmentElement struct {
	Subject, Parent NodeRef
	Order           uint64
}

func fragmentKnown[T comparable](v T) Evidence[T] { return Evidence[T]{State: EvidenceKnown, Value: v} }
func (s *Session) fragmentMode() fragmentMode {
	if s == nil || s.schema == nil || s.schema.active == nil {
		return fragmentLegacy
	}
	return s.catalog.fragmentModes[s.profile.ID][s.schema.active.assembly]
}
func validateFragmentProfile(c *CompiledCatalog, profile Profile, closure map[RuleKey]bool, supports map[RuleKey]RuleSupport) error {
	for key := range closure {
		p := c.plans[c.rules[key].Predicate]
		if p.context == nil || !resourceLinkPlan(p.kind) || p.context.fragments == fragmentLegacy || supports[key].State != ImplementationAvailable {
			continue
		}
		if c.fragmentModes == nil {
			c.fragmentModes = make(map[ProfileID]map[Assembly]fragmentMode)
			c.fragmentDTDs = make(map[ProfileID]map[Assembly]bool)
		}
		if c.fragmentModes[profile.ID] == nil {
			c.fragmentModes[profile.ID] = make(map[Assembly]fragmentMode)
			c.fragmentDTDs[profile.ID] = make(map[Assembly]bool)
		}
		a := p.context.Schema.assembly
		if old := c.fragmentModes[profile.ID][a]; old != fragmentLegacy && old != p.context.fragments {
			return contract("XML fragment profile", "conflicting selected fragment interpretations")
		}
		if old := c.fragmentModes[profile.ID][a]; old != fragmentLegacy && c.fragmentDTDs[profile.ID][a] != p.context.sourceInfoset {
			return contract("XML fragment profile", "conflicting identity authorities")
		}
		c.fragmentModes[profile.ID][a] = p.context.fragments
		c.fragmentDTDs[profile.ID][a] = p.context.sourceInfoset
	}
	return nil
}
func (s *Session) resourceFragments(ctx context.Context) (*resourceFragmentSnapshot, error) {
	mode := s.fragmentMode()
	if mode == fragmentLegacy {
		return nil, nil
	}
	size, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(s.sequence)), 256)
	if err != nil {
		return nil, err
	}
	size, err = rationalCheckedAdd(BudgetBufferedBytes, size, 1024)
	if err != nil {
		return nil, err
	}
	if _, err = AllocationSize(size); err != nil {
		return nil, err
	}
	if err = s.retain(size); err != nil {
		return nil, err
	}
	out := &resourceFragmentSnapshot{Mode: mode, DTDRequired: s.sourceDTDIDsSelected(), Complete: stateUnknown[bool]("fragment-element-order-incomplete"), Elements: make([]resourceFragmentElement, len(s.sequence))}
	if s.inputComplete && !s.enumerationIncomplete && s.terminalCause == nil {
		out.Complete = fragmentKnown(true)
	}
	for i, id := range s.sequence {
		if err = s.step(ctx, 1); err != nil {
			return nil, err
		}
		n := s.nodes[id]
		out.Elements[i] = resourceFragmentElement{Subject: NodeRef{s.documentID, id}, Parent: NodeRef{s.documentID, n.header.Parent}, Order: uint64(i) + 1}
		if !n.closed || n.end.Order.State != EvidenceKnown || !knownComplete(n.end.ChildrenComplete) {
			out.Complete = stateUnknown[bool]("fragment-element-order-incomplete")
		}
	}
	if out.DTDRequired {
		out.DTDDeclarations = stateUnknown[bool]("source-dtd-id-declarations-unavailable")
		out.DTDOccurrences = stateUnknown[bool]("source-dtd-id-occurrences-unsealed")
		out.DTDIdentityComplete = stateUnknown[bool]("source-dtd-id-authority-incomplete")
		if f := s.dtdIDs; f != nil {
			out.DTDDeclarations, out.DTDOccurrences = f.Domain, f.Complete
			out.DTDIDs = f.IDs
			if knownComplete(f.Domain) && knownComplete(f.Complete) && !f.Uncertain {
				out.DTDIdentityComplete = fragmentKnown(true)
			}
		}
	}
	return out, nil
}

// resolveFragmentID deliberately ignores ResourceIDFact.Unique. That fact is
// schema validity; Framework selects the first matching owner even when an
// independent intrinsic duplicate-ID failure is retained. A whole-domain seal
// is a conservative requirement before ruling out an earlier unknown match.
func (s *PackageSession) resolveFragmentID(ctx context.Context, snap *ResourceSnapshot, value string) (Evidence[NodeRef], error) {
	f := snap.fragments
	if f == nil {
		return stateUnknown[NodeRef]("fragment-authority-unselected"), nil
	}
	if !knownComplete(snap.header.IDScopeComplete) {
		return stateUnknown[NodeRef]("fragment-xsd-id-scope-incomplete"), nil
	}
	if f.DTDRequired && !knownComplete(f.DTDIdentityComplete) {
		return stateUnknown[NodeRef]("fragment-dtd-id-scope-incomplete"), nil
	}
	if !knownComplete(f.Complete) {
		return stateUnknown[NodeRef]("fragment-element-order-incomplete"), nil
	}
	// The snapshot index is built once from admitted document order. Scan each
	// authority once, then compare only matching owners' orders; neither NodeID
	// magnitude nor schema uniqueness determines which occurrence is first.
	if err := s.scan(ctx, 1); err != nil {
		return Evidence[NodeRef]{}, err
	}
	var first NodeRef
	var firstOrder uint64
	consider := func(subject NodeRef) error {
		if err := s.scan(ctx, 1+int64(len(subject.DocumentID))); err != nil {
			return err
		}
		if order := snap.fragmentOrder[subject]; order != 0 && (firstOrder == 0 || order < firstOrder) {
			first, firstOrder = subject, order
		}
		return nil
	}
	for _, id := range snap.ids {
		if err := s.scan(ctx, 1+int64(len(value))); err != nil {
			return Evidence[NodeRef]{}, err
		}
		if id.Value == value {
			if err := consider(id.Subject); err != nil {
				return Evidence[NodeRef]{}, err
			}
		}
	}
	for _, id := range f.DTDIDs {
		if err := s.scan(ctx, 1+int64(len(value))); err != nil {
			return Evidence[NodeRef]{}, err
		}
		if id.Value == value {
			if err := consider(id.Subject); err != nil {
				return Evidence[NodeRef]{}, err
			}
		}
	}
	if firstOrder != 0 {
		return fragmentKnown(first), nil
	}
	return Evidence[NodeRef]{State: EvidenceAbsent}, nil
}
func (s *PackageSession) fragmentSelectionDecision(ctx context.Context, d Decision, e PackageEdgeObservation, target *packageAdoption) Decision {
	result, err := s.resolveXMLFragment(ctx, target.resource.snapshot, e.Fragment.Value)
	if err != nil {
		return d
	}
	if result.State == EvidenceAbsent {
		d.Outcome, d.Reason = OutcomeFail, "fragment-id-absent"
		if result.Reason != "" {
			d.Reason = result.Reason
		}
		return d
	}
	if result.State != EvidenceKnown {
		return packageAdoptDecision(packageRequiredEvidenceDecision(d, result), target)
	}
	if e.FragmentTarget.State == EvidenceKnown && result.Value.Node != e.FragmentTarget.Value.Node {
		d.Outcome, d.Reason = OutcomeUnknown, "fragment-target-identity-mismatch"
		return d
	}
	d.Outcome, d.Reason = OutcomePass, "linked-target-kind-established"
	return d
}

// ResolveXMLFragment evaluates the explicitly selected XML fragment capability
// of this adopted resource. The fragment has already been URI-decoded exactly
// once. selected is false for legacy snapshots, whose existing schema-ID API is
// unchanged. The result is an owned copy with this adoption's document identity;
// neither retained DTD declarations nor tree internals are exposed.
func (a *ResourceAdoption) ResolveXMLFragment(ctx context.Context, fragment string) (result Evidence[NodeRef], selected bool, err error) {
	if err = a.live(); err != nil {
		return result, false, err
	}
	if ctx == nil {
		return result, false, contract("context", "must not be nil")
	}
	if err = ctx.Err(); err != nil {
		return result, false, err
	}
	snap := a.value.resource.snapshot
	if snap.fragments == nil {
		return result, false, nil
	}
	if len(fragment) > 4096 {
		return result, true, contract("XML fragment", "bounded once-decoded text required")
	}
	if err = a.owner.account.Check(BudgetScalarBytes, int64(len(fragment))); err != nil {
		return result, true, err
	}
	if err = a.owner.scan(ctx, 1+3*int64(len(fragment))); err != nil {
		return result, true, err
	}
	if !utf8.ValidString(fragment) || snap.fragments.Mode != fragmentFramework && !validText(fragment, 4096) {
		return result, true, contract("XML fragment", "bounded once-decoded text required")
	}
	if fragment == "" {
		if snap.header.Root.Node == 0 {
			result = stateUnknown[NodeRef]("fragment-root-occurrence-unavailable")
		} else {
			result = fragmentKnown(snap.header.Root)
		}
	} else {
		result, err = a.owner.resolveXMLFragment(ctx, snap, fragment)
	}
	if err != nil {
		return Evidence[NodeRef]{}, true, err
	}
	result, err = resourceLiveCopy(ctx, a.owner.account, result, snap.header.DocumentID, a.value.documentID)
	return result, true, err
}

// The immutable order index borrows document strings from the owned Elements.
// The allowance includes map control storage, spare capacity and overlapping
// growth; keys and values are fixed records. It is never exported or remapped.
const resourceFragmentOrderHeaderBytes int64 = 512
const resourceFragmentOrderEntryBytes int64 = 128

func (s *ResourceSnapshot) indexFragmentOrder(ctx context.Context) error {
	if ctx == nil {
		return contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.fragments == nil {
		return nil
	}
	s.fragmentOrder = make(map[NodeRef]uint64, len(s.fragments.Elements))
	for _, element := range s.fragments.Elements {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.account.Charge(BudgetRuleSteps, 1+int64(len(element.Subject.DocumentID))); err != nil {
			return err
		}
		s.fragmentOrder[element.Subject] = element.Order
	}
	return ctx.Err()
}
