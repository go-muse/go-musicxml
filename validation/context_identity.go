package validation

import (
	"context"
	"strings"
)

// These indexes concern explicitly selected declaration slots and exact supplied
// strings only. They deliberately contain no XML ID lexer, datatype inference,
// namespace fallback, target-kind rule, or musical scope policy.
type identityIndex struct {
	hasProvider bool
	incomplete  bool
	values      map[string]*identityEntry
}
type identityEntry struct {
	first       NodeRef
	original    Location
	owners      map[NodeRef]bool
	collision   *DecisionRef
	conflicting Location
}

func planDecision(s *Session, n *factNode, r RuleDefinition) Decision {
	return Decision{Rule: r.Key, Subject: NodeRef{DocumentID: s.documentID, Node: n.header.ID}}
}
func contextSubject(s *Session, n *factNode, relation RelationKind) (NodeRef, bool) {
	at := NodeRef{DocumentID: s.documentID, Node: n.header.ID}
	switch relation {
	case RelationSameNode:
	case RelationParent:
		if n.header.Parent == 0 {
			return NodeRef{}, false
		}
		at.Node = n.header.Parent
	case RelationDocument:
		at.Node = 0
	default:
		return NodeRef{}, false
	}
	return at, true
}

// Completeness belongs to the context's provider domain. An unrelated missing
// subtree cannot invalidate a closed same-node value, but an omitted child can
// hide a parent-targeted publisher and an omitted node can hide a document one.
func (s *Session) contextScopeComplete(ctx context.Context, n *factNode, relation RelationKind, key string) (bool, error) {
	if relation == RelationDocument {
		return s.inputComplete && !s.enumerationIncomplete && s.terminalCause == nil, nil
	}
	at := n
	if relation == RelationParent {
		at = s.nodes[n.header.Parent]
	}
	if at == nil || !at.closed {
		return false, nil
	}
	if at.end.ChildrenComplete.State == EvidenceKnown && at.end.ChildrenComplete.Value {
		return true, nil
	}
	for _, rule := range s.catalog.orders[s.profile.ID] {
		if err := s.step(ctx, 1); err != nil {
			return false, err
		}
		p := s.catalog.plans[s.catalog.rules[rule].Predicate]
		if (p.kind == planContextValue || p.kind == planContextDefault) && p.key == key && p.relation == RelationParent {
			return false, nil
		}
	}
	return true, nil
}
func contextLocationBytes(l Location) int64 {
	size := int64(160 + len(l.DocumentID) + len(l.Entry) + len(l.Path))
	if l.Position != nil {
		size += int64(64 + len(l.Position.CoordinateSpace))
	}
	return size
}
func contextAtomicBytes(a Atomic) int64 {
	return int64(96+len(a.Text)+len(a.Name.Space)+len(a.Name.Local)) + a.Integer.ByteSize() + a.Decimal.ByteSize()
}
func contextEvidenceDecision(d Decision, e Evidence[Atomic], absent Outcome) Decision {
	d.Reason = e.Reason
	switch e.State {
	case EvidenceAbsent:
		d.Outcome = absent
	case EvidenceUnknown:
		d.Outcome = OutcomeUnknown
	case EvidenceUnsupported:
		d.Outcome = OutcomeUnsupported
	case EvidenceInvalid:
		d.Outcome = OutcomeBlocked
		if e.Cause != nil {
			cause := *e.Cause
			d.Cause = &cause
		}
	}
	return d
}

// publishContext merges owned evidence monotonically. Unknown/conflicting or
// unavailable publications cannot be erased by subsequent known constants.
func (s *Session) publishContext(ctx context.Context, n *factNode, r RuleDefinition, p Plan, e Evidence[Atomic], origin Location) error {
	id := decisionID{rule: r.Key, node: n.header.ID}
	if s.registered[id] {
		return nil
	}
	at, ok := contextSubject(s, n, p.relation)
	if !ok {
		return nil
	}
	if err := s.account.Check(BudgetScalarBytes, atomicLogicalBytes(e.Value)); err != nil {
		return err
	}
	if err := s.step(ctx, 1+contextAtomicBytes(e.Value)); err != nil {
		return err
	}
	if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	if err := s.retain(256 + int64(len(p.key)+len(r.Key.ID)) + contextLocationBytes(origin) + contextAtomicBytes(e.Value) + evidenceLogicalBytes(e)); err != nil {
		return err
	}
	key := ContextKey{Name: p.key, At: at}
	fact, exists := s.contexts[key]
	if !exists {
		fact = ContextFact{Value: cloneEvidence(e)}
		fact.Value.Value = cloneAtomic(e.Value)
	} else {
		switch {
		case fact.Value.State == EvidenceKnown && e.State == EvidenceKnown:
			equal := false
			if fact.Value.Value.Kind == e.Value.Kind {
				if err := s.step(ctx, 1+contextAtomicBytes(fact.Value.Value)); err != nil {
					return err
				}
				var err error
				equal, err = compareAtomic(fact.Value.Value, e.Value, ComparisonEqual)
				if err != nil {
					return err
				}
			}
			if !equal {
				fact.Value = Evidence[Atomic]{State: EvidenceUnknown, Reason: "context-conflict"}
			}
		case fact.Value.State == EvidenceKnown:
			fact.Value = cloneEvidence(e)
			fact.Value.Value = cloneAtomic(e.Value)
		// A failure or unsupported prerequisite is stronger provenance than a
		// generic unknown, but neither justifies any normative value failure.
		case fact.Value.State == EvidenceUnknown && (e.State == EvidenceUnsupported || e.State == EvidenceInvalid):
			fact.Value = cloneEvidence(e)
		}
	}
	fact.Origins = append(fact.Origins, cloneLocation(origin))
	fact.Providers = append(fact.Providers, cloneKey(r.Key))
	s.contexts[key] = fact
	s.registered[id] = true
	return nil
}

// markContextIncomplete is called by scheduling when a possible provider could
// not execute. Proven selector mismatches must not call it.
func (s *Session) markContextIncomplete(ctx context.Context, n *factNode, r RuleDefinition, p Plan, d Decision) error {
	if p.kind != planContextValue && p.kind != planContextDefault {
		return nil
	}
	e := Evidence[Atomic]{State: EvidenceUnknown, Reason: d.Reason}
	if e.Reason == "" {
		e.Reason = "context-provider-unavailable"
	}
	if d.Outcome == OutcomeUnsupported {
		e.State = EvidenceUnsupported
	}
	if d.Outcome == OutcomeBlocked && d.Cause != nil {
		e.State = EvidenceInvalid
		e.Cause = d.Cause
	}
	return s.publishContext(ctx, n, r, p, e, n.header.Location)
}

// Context provenance may contain many publishers. Reserve the output copy and
// account/check cancellation for each origin before copying it for a decision.
func (s *Session) contextRelated(ctx context.Context, origins []Location) ([]Location, error) {
	size := int64(0)
	for _, origin := range origins {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		size += contextLocationBytes(origin)
	}
	if err := s.retain(size); err != nil {
		return nil, err
	}
	result := make([]Location, len(origins))
	for i, origin := range origins {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		result[i] = cloneLocation(origin)
	}
	return result, nil
}

func evaluateContextPlan(ctx context.Context, s *Session, n *factNode, r RuleDefinition, p Plan, final bool) (Decision, bool, error) {
	d := planDecision(s, n, r)
	if err := s.step(ctx, 1); err != nil {
		return d, false, err
	}
	if !validLocalRelation(p.relation) {
		d.Outcome = OutcomeUnsupported
		d.Reason = "context-relation-unsupported"
		return d, false, nil
	}
	at, ok := contextSubject(s, n, p.relation)
	if !ok {
		d.Outcome = OutcomeUnknown
		d.Reason = "context-scope-missing"
		return d, false, nil
	}
	switch p.kind {
	case planContextValue, planContextDefault:
		if !n.closed {
			if !final {
				return d, true, nil
			}
			d.Outcome = OutcomeUnknown
			d.Reason = "context-provider-incomplete"
			return d, false, nil
		}
		value := Evidence[Atomic]{State: EvidenceKnown, Value: p.expected}
		origin := n.header.Location
		if p.kind == planContextDefault {
			presence, attribute, err := s.attributeEvidence(ctx, n, p.attribute)
			if err != nil {
				return d, false, err
			}
			switch presence.State {
			case EvidenceAbsent:
				// Only proven absence selects the catalog's explicit fallback.
			case EvidenceKnown:
				if attribute == nil {
					value = Evidence[Atomic]{State: EvidenceUnknown, Reason: "attribute-value-unavailable"}
				} else {
					origin = attribute.Location
					value = attribute.Scalar.Value
					if value.State == EvidenceAbsent {
						value = Evidence[Atomic]{State: EvidenceUnknown, Reason: "required-value-absent"}
					}
				}
			default:
				value = Evidence[Atomic]{State: presence.State, Reason: presence.Reason, Cause: presence.Cause}
			}
		}
		if err := s.publishContext(ctx, n, r, p, value, origin); err != nil {
			return d, false, err
		}
		if value.State != EvidenceKnown {
			return contextEvidenceDecision(d, value, OutcomeUnknown), false, nil
		}
		d.Outcome = OutcomePass
		return d, false, nil
	case planContextCompare:
		if !final {
			return d, true, nil
		}
		fact, exists := s.contexts[ContextKey{Name: p.key, At: at}]
		if !exists {
			d.Outcome = OutcomeUnknown
			d.Reason = "context-missing"
			return d, false, nil
		}
		// A context's unavailable evidence is useful even on an incomplete
		// traversal, but incomplete parent/document enumeration cannot prove
		// that a known value has no later conflicting provider.
		if fact.Value.State != EvidenceKnown {
			d = contextEvidenceDecision(d, fact.Value, OutcomeUnknown)
			var err error
			d.Related, err = s.contextRelated(ctx, fact.Origins)
			return d, false, err
		}
		scopeComplete, err := s.contextScopeComplete(ctx, n, p.relation, p.key)
		if err != nil {
			return d, false, err
		}
		if !scopeComplete {
			d.Outcome = OutcomeUnknown
			d.Reason = "context-scope-incomplete"
			return d, false, nil
		}
		if fact.Value.Value.Kind != p.expected.Kind {
			d.Outcome = OutcomeUnsupported
			d.Reason = "atomic-family-unavailable"
			return d, false, nil
		}
		if err := s.step(ctx, 1+contextAtomicBytes(fact.Value.Value)+contextAtomicBytes(p.expected)); err != nil {
			return d, false, err
		}
		pass, err := compareAtomic(fact.Value.Value, p.expected, p.comparison)
		if err != nil {
			return d, false, err
		}
		d.Outcome = OutcomeFail
		if pass {
			d.Outcome = OutcomePass
		}
		d.Related, err = s.contextRelated(ctx, fact.Origins)
		return d, false, err
	}
	return d, false, contract("context plan", "unexpected plan family")
}

func (s *Session) identityIndex(name string) (*identityIndex, error) {
	if index := s.identities[name]; index != nil {
		return index, nil
	}
	if err := s.retain(256 + int64(len(name))); err != nil {
		return nil, err
	}
	index := &identityIndex{values: map[string]*identityEntry{}}
	s.identities[strings.Clone(name)] = index
	return index, nil
}

// markIdentityProvider is called once for every selected declaring plan at
// Begin, including statically unsupported providers. Merely having a reference
// plan does not establish that this index's declaration role is implemented.
func (s *Session) markIdentityProvider(name string) error {
	index, err := s.identityIndex(name)
	if err != nil {
		return err
	}
	index.hasProvider = true
	return nil
}

// The scheduler seeds selected indexes before input, so this marker needs no
// allocation and remains safe even after the account has exhausted a quota.
func (s *Session) markIdentityIncomplete(name string) {
	if index := s.identities[name]; index != nil {
		index.incomplete = true
	}
}

func identityRelated(entry *identityEntry) []Location {
	if entry.collision == nil {
		return []Location{cloneLocation(entry.original)}
	}
	return []Location{cloneLocation(entry.original), cloneLocation(entry.conflicting)}
}

func evaluateIdentityPlan(ctx context.Context, s *Session, n *factNode, r RuleDefinition, p Plan, final bool) (Decision, bool, error) {
	d := planDecision(s, n, r)
	if p.kind != planIdentityDeclare && p.kind != planIdentityReference {
		return d, false, contract("identity plan", "unexpected plan family")
	}
	if err := s.step(ctx, 1); err != nil {
		return d, false, err
	}
	index, err := s.identityIndex(p.key)
	if err != nil {
		return d, false, err
	}
	if p.kind == planIdentityDeclare {
		index.hasProvider = true
	}
	if !n.closed {
		if !final {
			return d, true, nil
		}
		if p.kind == planIdentityDeclare {
			index.incomplete = true
		}
		d.Outcome = OutcomeUnknown
		d.Reason = "identity-scope-incomplete"
		return d, false, nil
	}
	value, location, err := s.lookupAttribute(ctx, n, p.attribute)
	if err != nil {
		return d, false, err
	}
	if value.State == EvidenceAbsent {
		presence, _, lookupErr := s.attributeEvidence(ctx, n, p.attribute)
		if lookupErr != nil {
			return d, false, lookupErr
		}
		if presence.State == EvidenceKnown {
			value = Evidence[Atomic]{State: EvidenceUnknown, Reason: "identity-value-unavailable"}
		}
	}
	if value.State != EvidenceKnown {
		if p.kind == planIdentityDeclare && value.State != EvidenceAbsent {
			index.incomplete = true
		}
		d = contextEvidenceDecision(d, value, OutcomeNotApplicable)
		return d, false, nil
	}
	if value.Value.Kind != AtomicString {
		if p.kind == planIdentityDeclare {
			index.incomplete = true
		}
		d.Outcome = OutcomeUnsupported
		d.Reason = "identity-family-unavailable"
		return d, false, nil
	}
	if err := validateAtomic(value.Value); err != nil {
		return d, false, err
	}
	text := value.Value.Text
	if err := s.account.Check(BudgetScalarBytes, int64(len(text))); err != nil {
		return d, false, err
	}
	if err := s.step(ctx, 1+int64(len(text))); err != nil {
		return d, false, err
	}
	owner := d.Subject
	id := decisionID{rule: r.Key, node: n.header.ID}
	entry := index.values[text]
	if !s.registered[id] {
		// Both sides retain work until EOF. References must never bypass the
		// shared identity quota just because they do not introduce new keys.
		if err := s.account.Charge(BudgetIDs, 1); err != nil {
			return d, false, err
		}
		loc := n.header.Location
		if location != nil {
			loc = *location
		}
		if err := s.retain(768 + int64(len(text)+len(p.key)+len(r.Key.ID)) + 2*contextLocationBytes(loc)); err != nil {
			return d, false, err
		}
		if p.kind == planIdentityDeclare {
			if entry == nil {
				entry = &identityEntry{first: owner, original: cloneLocation(loc), owners: map[NodeRef]bool{owner: true}}
				index.values[strings.Clone(text)] = entry
			} else if !entry.owners[owner] {
				entry.owners[owner] = true
				if entry.collision == nil {
					entry.collision = &DecisionRef{Rule: cloneKey(r.Key), Subject: owner}
					entry.conflicting = cloneLocation(loc)
				}
				// A known collision is an independent proof regardless of any
				// unrelated unavailable declarations or premature document end.
				d.Outcome = OutcomeFail
				d.Reason = "identity-duplicate"
				d.Related = []Location{cloneLocation(entry.original), cloneLocation(loc)}
				s.registered[id] = true
				return d, false, nil
			}
		}
		s.registered[id] = true
	}
	if p.kind == planIdentityDeclare && entry != nil && entry.collision != nil && owner != entry.first {
		d.Outcome = OutcomeFail
		d.Reason = "identity-duplicate"
		d.Related = identityRelated(entry)
		return d, false, nil
	}
	if !final {
		return d, true, nil
	}
	if entry != nil && entry.collision != nil {
		d.Outcome = OutcomeBlocked
		d.Reason = "identity-ambiguous"
		cause := *entry.collision
		d.Cause = &cause
		d.Related = identityRelated(entry)
		return d, false, nil
	}
	if !index.hasProvider {
		d.Outcome = OutcomeUnsupported
		d.Reason = "identity-provider-unselected"
		return d, false, nil
	}
	if !s.inputComplete || s.terminalCause != nil || index.incomplete {
		d.Outcome = OutcomeUnknown
		d.Reason = "identity-index-incomplete"
		return d, false, nil
	}
	if entry == nil {
		d.Outcome = OutcomeFail
		d.Reason = "identity-target-missing"
		return d, false, nil
	}
	d.Outcome = OutcomePass
	d.Related = identityRelated(entry)
	return d, false, nil
}
