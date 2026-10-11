package validation

import (
	"context"
	"slices"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

const planTypedReference planKind = "typed-reference"

// ReferenceOwner names a containing reference whose selected leaf typed rule
// establishes the canonical owner. Rule is a complete exact revision identity.
type ReferenceOwner struct {
	Declaration DeclarationID
	Attribute   AttributeSelector
	Rule        RuleKey
}

// ReferencePlanSpec describes one local typed reference consequence. Targets and
// optional TargetAncestors restrict the authoritative unique ID owner. Owners
// establishes matching scope, either by direct ancestry or OwnerReferences.
// TriggerChildren optionally restricts applicability to selected direct children.
// No field authorizes loading or inspecting another document.
type ReferencePlanSpec struct {
	Attribute       AttributeSelector
	Targets         []DeclarationID
	TargetAncestors []DeclarationID
	Owners          []DeclarationID
	OwnerReferences []ReferenceOwner
	TriggerChildren []DeclarationID
}

type referencePlan struct{ ReferencePlanSpec }

// NewTypedReferencePlan creates a sealed typed/scoped consequence over the shared
// XSD identity run. Generic missing, ambiguous and invalid references block it;
// they never produce another missing-target violation or a second ID index.
func NewTypedReferencePlan(spec ReferencePlanSpec) (Plan, error) {
	p := &referencePlan{spec}
	return checkedPlan(Plan{kind: planTypedReference, reference: p})
}
func (p *referencePlan) bounded() bool {
	return p != nil && len(p.Targets) <= 4096 && len(p.TargetAncestors) <= 4096 && len(p.Owners) <= 4096 && len(p.OwnerReferences) <= 4096 && len(p.TriggerChildren) <= 4096
}
func (p *referencePlan) valid() error {
	if !p.bounded() || !validAttributeSelector(p.Attribute) || !validDeclarationSet(p.Targets) {
		return contract("typed reference", "exact attribute and bounded target declarations required")
	}
	for _, ds := range [][]DeclarationID{p.TargetAncestors, p.Owners, p.TriggerChildren} {
		if len(ds) > 0 && !validDeclarationSet(ds) {
			return contract("typed reference", "invalid declaration set")
		}
	}
	if len(p.OwnerReferences) > 0 && len(p.Owners) == 0 {
		return contract("typed reference", "reference owners require canonical owner declarations")
	}
	seen := map[DeclarationID]bool{}
	for _, r := range p.OwnerReferences {
		if !validDeclaration(r.Declaration) || !validAttributeSelector(r.Attribute) || !validID(string(r.Rule.ID)) || r.Rule.Revision == 0 || seen[r.Declaration] {
			return contract("typed reference", "exact nonduplicate containing reference and rule required")
		}
		seen[r.Declaration] = true
	}
	return nil
}
func (p *referencePlan) size() int64 {
	if p == nil {
		return 0
	}
	n := int64(256) + localAttributeSize(p.Attribute)
	for _, ds := range [][]DeclarationID{p.Targets, p.TargetAncestors, p.Owners, p.TriggerChildren} {
		for _, d := range ds {
			n += int64(64 + len(d.OwnerOccurrence) + len(d.Occurrence))
		}
	}
	for _, r := range p.OwnerReferences {
		n += int64(192+len(r.Declaration.OwnerOccurrence)+len(r.Declaration.Occurrence)+len(r.Rule.ID)) + localAttributeSize(r.Attribute)
	}
	return n
}
func (p *referencePlan) clone() *referencePlan {
	if p == nil {
		return nil
	}
	q := *p
	q.Attribute = cloneLocalAttribute(p.Attribute)
	q.Targets = cloneDeclarationSet(p.Targets)
	q.TargetAncestors = cloneDeclarationSet(p.TargetAncestors)
	q.Owners = cloneDeclarationSet(p.Owners)
	q.TriggerChildren = cloneDeclarationSet(p.TriggerChildren)
	q.OwnerReferences = make([]ReferenceOwner, len(p.OwnerReferences))
	for i, r := range p.OwnerReferences {
		q.OwnerReferences[i] = ReferenceOwner{Declaration: cloneDeclaration(r.Declaration), Attribute: cloneLocalAttribute(r.Attribute), Rule: RuleKey{ID: RuleID(strings.Clone(string(r.Rule.ID))), Revision: r.Rule.Revision}}
	}
	slices.SortFunc(q.OwnerReferences, func(a, b ReferenceOwner) int {
		return compareLocalDeclaration(a.Declaration, b.Declaration)
	})
	return &q
}
func (p *referencePlan) digest() any {
	if p == nil {
		return nil
	}
	return struct {
		Kind planKind
		Spec ReferencePlanSpec
	}{planTypedReference, p.ReferencePlanSpec}
}

// Validate selected owner providers eagerly so final evaluation may call only a
// leaf predicate and can never recurse through user-supplied provider cycles.
func (p Plan) validateReferenceRules(c *CompiledCatalog, closure map[RuleKey]bool) error {
	if p.kind != planTypedReference {
		return nil
	}
	for _, owner := range p.reference.OwnerReferences {
		if !closure[owner.Rule] {
			continue
		}
		rule, ok := c.rules[owner.Rule]
		if !ok {
			return contract("typed reference owner", "unknown provider rule")
		}
		provider, ok := c.plans[rule.Predicate]
		if !ok || provider.kind != planTypedReference || provider.reference == nil {
			return contract("typed reference owner", "provider must be a typed-reference leaf")
		}
		ref := provider.reference
		if len(ref.OwnerReferences) != 0 || len(ref.Owners) != 0 || len(ref.TriggerChildren) != 0 || rule.Selector.Declaration == nil || *rule.Selector.Declaration != owner.Declaration || ref.Attribute != owner.Attribute || !slices.Equal(ref.Targets, p.reference.Owners) {
			return contract("typed reference owner", "provider must match exact declaration, slot and canonical owner targets")
		}
	}
	return nil
}
func (s *Session) referenceAncestor(ctx context.Context, n *factNode, declarations []DeclarationID) (*factNode, Decision, error) {
	for n != nil {
		if err := s.step(ctx, 1); err != nil {
			return nil, Decision{}, err
		}
		match, d, e := localMatches(ctx, s, n, declarations)
		if e != nil {
			return nil, d, e
		}
		if d.Outcome != OutcomePass {
			return nil, d, nil
		}
		if match {
			return n, Decision{Outcome: OutcomePass}, nil
		}
		n = s.nodes[n.header.Parent]
	}
	return nil, Decision{Outcome: OutcomeFail, Reason: "typed-target-ancestor-mismatch"}, nil
}
func (s *Session) referenceResolution(ctx context.Context, n *factNode, a AttributeSelector) (*factNode, Decision, error) {
	v, e := s.localScalar(ctx, n, &a)
	if e != nil {
		return nil, Decision{}, e
	}
	if v.State != EvidenceKnown {
		return nil, localGap(v), nil
	}
	if s.schema == nil || s.schema.run == nil {
		return nil, Decision{Outcome: OutcomeUnsupported, Reason: "authoritative-identity-unavailable"}, nil
	}
	resolution, cause, e := s.schemaResolveIdentity(ctx, n.header.ID, a.Name)
	if e != nil {
		return nil, Decision{}, e
	}
	if cause != nil {
		reason := Code(resolution.Reason)
		if reason == "" {
			reason = "identity-" + Code(resolution.Status)
		}
		return nil, Decision{Outcome: OutcomeBlocked, Reason: reason, Cause: cause}, nil
	}
	if resolution.Status != xsd.IdentityUnique {
		reason := Code(resolution.Reason)
		if reason == "" {
			reason = "identity-" + Code(resolution.Status)
		}
		// Missing/ambiguity without the selected authoritative cause cannot become
		// either a second error or a guessed successful resolution.
		return nil, Decision{Outcome: OutcomeUnsupported, Reason: reason}, nil
	}
	target := s.nodes[NodeID(resolution.Owner)]
	if target == nil {
		return nil, Decision{Outcome: OutcomeUnsupported, Reason: "identity-owner-unavailable"}, nil
	}
	return target, Decision{Outcome: OutcomePass}, nil
}
func (s *Session) referenceOwner(ctx context.Context, n *factNode, p *referencePlan) (*factNode, Decision, error) {
	for n != nil {
		if e := s.step(ctx, 1); e != nil {
			return nil, Decision{}, e
		}
		match, d, e := localMatches(ctx, s, n, p.Owners)
		if e != nil {
			return nil, d, e
		}
		if d.Outcome != OutcomePass {
			return nil, d, nil
		}
		if match {
			return n, d, nil
		}
		for _, owner := range p.OwnerReferences {
			if e := s.step(ctx, 1); e != nil {
				return nil, Decision{}, e
			}
			if n.header.Declaration.Value != owner.Declaration {
				continue
			}
			rule, selected := s.catalog.rules[owner.Rule]
			support, enabled := s.catalog.supports[s.profile.ID][owner.Rule]
			if !selected || !enabled || support.State != ImplementationAvailable {
				return nil, Decision{Outcome: OutcomeUnsupported, Reason: "typed-owner-provider-unavailable"}, nil
			}
			provider := s.catalog.plans[rule.Predicate]
			// Same invariant as eager catalog validation, repeated defensively before a
			// nested call; only a leaf can execute, never this referencing plan.
			if provider.kind != planTypedReference || provider.reference == nil || len(provider.reference.OwnerReferences) > 0 || len(provider.reference.Owners) > 0 {
				return nil, Decision{Outcome: OutcomeUnsupported, Reason: "typed-owner-provider-invalid"}, nil
			}
			if e := s.evaluate(ctx, n, owner.Rule, true); e != nil {
				return nil, Decision{}, e
			}
			decision, known := s.lookupDecision(decisionID{owner.Rule, n.header.ID})
			if !known {
				return nil, Decision{Outcome: OutcomeUnknown, Reason: "typed-owner-result-unavailable"}, nil
			}
			if decision.Outcome != OutcomePass {
				if decision.Outcome == OutcomeFail {
					return nil, Decision{Outcome: OutcomeBlocked, Reason: "typed-owner-invalid", Cause: &DecisionRef{Rule: owner.Rule, Subject: NodeRef{s.documentID, n.header.ID}}}, nil
				}
				if decision.Outcome == OutcomeNotApplicable {
					return nil, Decision{Outcome: OutcomeUnknown, Reason: "typed-owner-not-applicable"}, nil
				}
				return nil, decision, nil
			}
			return s.referenceResolution(ctx, n, owner.Attribute)
		}
		n = s.nodes[n.header.Parent]
	}
	return nil, Decision{Outcome: OutcomeUnknown, Reason: "typed-owner-scope-unavailable"}, nil
}
func (s *Session) evaluateReferencePlan(ctx context.Context, n *factNode, p *referencePlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	if len(p.TriggerChildren) > 0 {
		presence, _, e := s.localPresence(ctx, n, p.TriggerChildren, false)
		if e != nil {
			return presence, false, e
		}
		if presence.Outcome == OutcomeFail {
			return Decision{Outcome: OutcomeNotApplicable, Reason: "reference-trigger-absent"}, false, nil
		}
		if presence.Outcome != OutcomePass {
			return presence, false, nil
		}
	}
	presence, _, e := s.attributeEvidence(ctx, n, p.Attribute)
	if e != nil {
		return Decision{}, false, e
	}
	if presence.State == EvidenceAbsent || (presence.State == EvidenceKnown && !presence.Value) {
		return Decision{Outcome: OutcomeNotApplicable, Reason: "reference-attribute-absent"}, false, nil
	}
	if presence.State != EvidenceKnown {
		return localGap(presence), false, nil
	}
	target, d, e := s.referenceResolution(ctx, n, p.Attribute)
	if e != nil || d.Outcome != OutcomePass {
		return d, false, e
	}
	matches, d, e := localMatches(ctx, s, target, p.Targets)
	if e != nil || d.Outcome != OutcomePass {
		return d, false, e
	}
	if !matches {
		return Decision{Outcome: OutcomeFail, Reason: "typed-reference-target-kind", Related: []Location{target.header.Location}}, false, nil
	}
	if len(p.TargetAncestors) > 0 {
		_, d, e = s.referenceAncestor(ctx, s.nodes[target.header.Parent], p.TargetAncestors)
		if e != nil || d.Outcome != OutcomePass {
			if d.Outcome == OutcomeFail {
				d.Related = []Location{target.header.Location}
			}
			return d, false, e
		}
	}
	if len(p.Owners) > 0 {
		actual, ownerDecision, e := s.referenceAncestor(ctx, s.nodes[target.header.Parent], p.Owners)
		if e != nil || ownerDecision.Outcome != OutcomePass {
			return ownerDecision, false, e
		}
		expected, scopeDecision, e := s.referenceOwner(ctx, n, p)
		if e != nil || scopeDecision.Outcome != OutcomePass {
			return scopeDecision, false, e
		}
		if actual.header.ID != expected.header.ID {
			return Decision{Outcome: OutcomeFail, Reason: "typed-reference-owner-mismatch", Related: []Location{target.header.Location, actual.header.Location, expected.header.Location}}, false, nil
		}
	}
	return Decision{Outcome: OutcomePass}, false, nil
}
