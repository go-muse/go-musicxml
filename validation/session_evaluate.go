package validation

import (
	"context"
	"strconv"
)

func (s *Session) evaluate(ctx context.Context, n *factNode, key RuleKey, final bool) error {
	id := decisionID{rule: key, node: n.header.ID}
	if _, done := s.lookupDecision(id); done {
		return nil
	}
	r := s.catalog.rules[key]
	if r.Scope == ScopeDocument && n != s.root {
		return nil
	}
	if !n.closed {
		return nil
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	p, hasPlan := s.catalog.plans[r.Predicate]
	selected := selectorDecision(r.Selector, n.header)
	if r.Selector.Assembly != nil && p.kind == planModelRepresentation {
		selected = s.modelAssemblyDecision(*r.Selector.Assembly)
	} else if r.Selector.Assembly != nil {
		selected = Decision{Outcome: OutcomeUnsupported, Reason: "schema-binding-unavailable"}
		if s.schema != nil && s.schema.active != nil {
			selected = Decision{Outcome: OutcomeNotApplicable, Reason: "schema-assembly-mismatch"}
			if *r.Selector.Assembly == s.schema.active.assembly {
				selected.Outcome, selected.Reason = OutcomePass, ""
			}
		}
	}

	selected.Rule = key
	selected.Subject = NodeRef{DocumentID: s.documentID, Node: n.header.ID}
	if selected.Outcome != OutcomePass {
		if hasPlan && p.kind == planIdentityDeclare && selected.Outcome != OutcomeNotApplicable {
			s.markIdentityIncomplete(p.key)
		}
		if hasPlan && (p.kind == planContextValue || p.kind == planContextDefault) && selected.Outcome != OutcomeNotApplicable {
			if err := s.markContextIncomplete(ctx, n, r, p, selected); err != nil {
				return err
			}
		}
		return s.record(r, n, selected)
	}
	support := s.catalog.supports[s.profile.ID][key]
	if support.State != ImplementationAvailable {
		if hasPlan && p.kind == planIdentityDeclare {
			s.markIdentityIncomplete(p.key)
		}
		selected.Outcome = OutcomeUnsupported
		selected.Reason = support.Reason
		if hasPlan && (p.kind == planContextValue || p.kind == planContextDefault) {
			if err := s.markContextIncomplete(ctx, n, r, p, selected); err != nil {
				return err
			}
		}
		return s.record(r, n, selected)
	}
	if !final && r.Scope == ScopeDocument {
		return nil
	}
	// Reference retention is charged at first closure, even though its result
	// waits for document prerequisites and the complete identity scope.
	if p.kind == planIdentityReference && !final {
		_, _, err := evaluateIdentityPlan(ctx, s, n, r, p, false)
		return err
	}
	// A valid local numeral-key is a complete independent alternative. Retain
	// its result before waiting for document timeline finalization or resources.
	if p.kind == planNumeralKey {
		d, conclusive, err := s.localNumeralKeyProof(ctx, n, p.timelineConsumer)
		if err != nil {
			return s.recordProvedPlanInterruption(r, n, p, err)
		}
		if conclusive {
			return s.record(r, n, d)
		}
	}
	var dependency Decision
	var pending bool
	var err error
	if p.isContext() && p.kind != planTimeOnlyOrder && p.kind != planFigureNumberNumeric && p.kind != planBendReleaseSign && p.kind != planGraphicalRepeatContent {
		dependency, pending, err = s.contextDependencies(ctx, n, r, p, final)
	} else {
		dependency, pending, err = s.dependencies(ctx, n, r, final)
	}
	if err != nil {
		return err
	}
	if pending {
		return nil
	}
	if dependency.Outcome != OutcomePass {
		if p.kind == planIdentityDeclare {
			s.markIdentityIncomplete(p.key)
		}
		dependency.Rule = key
		dependency.Subject = selected.Subject
		if p.kind == planContextValue || p.kind == planContextDefault {
			if err := s.markContextIncomplete(ctx, n, r, p, dependency); err != nil {
				return err
			}
		}
		return s.record(r, n, dependency)
	}
	if p.kind == planModelRepresentation {
		return s.evaluateModelRepresentation(ctx, n, r)
	}
	if p.kind == planXSD {
		return s.evaluateXSD(ctx, n, r, p, final)
	}
	decision, pending, err := s.evaluatePlan(ctx, n, r, p, final)
	if err != nil {
		return s.recordProvedPlanInterruption(r, n, p, err)
	}
	if pending {
		return nil
	}
	return s.record(r, n, decision)
}
func selectorDecision(selector Selector, n NodeHeader) Decision {
	if selector.Component != nil {
		if selector.Component.Kind != ComponentType {
			return Decision{Outcome: OutcomeUnsupported, Reason: "selector-component-kind-unavailable"}
		}
		e := n.Type
		if e.State != EvidenceKnown {
			return evidenceDecision(e.State, e.Reason, e.Cause, true)
		}
		if e.Value == *selector.Component {
			return Decision{Outcome: OutcomePass}
		}
	} else if selector.Declaration != nil {
		e := n.Declaration
		if e.State != EvidenceKnown {
			return evidenceDecision(e.State, e.Reason, e.Cause, true)
		}
		if e.Value == *selector.Declaration {
			return Decision{Outcome: OutcomePass}
		}
	}
	return Decision{Outcome: OutcomeNotApplicable, Reason: "selector-mismatch"}
}
func evidenceDecision(state EvidenceState, reason Code, cause *DecisionRef, absenceExcludes bool) Decision {
	d := Decision{Reason: reason, Cause: cause}
	switch state {
	case EvidenceKnown:
		d.Outcome = OutcomePass
	case EvidenceAbsent:
		if absenceExcludes {
			d.Outcome = OutcomeNotApplicable
			d.Reason = "proven-absent"
		} else {
			d.Outcome = OutcomeUnknown
			d.Reason = "required-value-absent"
		}
	case EvidenceUnknown:
		d.Outcome = OutcomeUnknown
	case EvidenceUnsupported:
		d.Outcome = OutcomeUnsupported
	case EvidenceInvalid:
		d.Outcome = OutcomeBlocked
	default:
		d.Outcome = OutcomeUnsupported
		d.Reason = "invalid-evidence-contract"
	}
	return d
}

// lookupDecision is the only reader of per-subject outcomes. Reconstructed
// Pass/NA results preserve their identity without retaining diagnostic payloads.
func (s *Session) lookupDecision(id decisionID) (Decision, bool) {
	if result := s.successful[id.rule][id.node]; result != 0 {
		outcome := OutcomePass
		if result == successfulNotApplicable {
			outcome = OutcomeNotApplicable
		}
		return Decision{Rule: id.rule, Subject: NodeRef{s.documentID, id.node}, Outcome: outcome}, true
	}
	d, ok := s.decisions[id]
	return d, ok
}

func (s *Session) record(r RuleDefinition, n *factNode, d Decision) error {
	id := decisionID{r.Key, n.header.ID}
	if _, exists := s.lookupDecision(id); exists {
		return nil
	}
	d.Rule = r.Key
	d.Subject = NodeRef{s.documentID, n.header.ID}
	// Once established, a finding remains authoritative even if retaining its
	// dependency-cache entry exhausts the session buffer afterwards.
	if err := s.accumulator.record(r, d, n.header.Location); err != nil {
		return err
	}
	return s.cacheDecision(r, n, d)
}

func (s *Session) cacheDecision(r RuleDefinition, n *factNode, d Decision) error {
	id := decisionID{r.Key, n.header.ID}
	if d.Outcome == OutcomePass || d.Outcome == OutcomeNotApplicable {
		results := s.successful[r.Key]
		bytes := int64(32) // NodeID, outcome byte and conservative map allowance.
		if results == nil {
			bytes += int64(128 + len(r.Key.ID)) // Both map levels and per-rule state.
		}
		if err := s.retain(bytes); err != nil {
			return err
		}
		if results == nil {
			if s.successful == nil {
				s.successful = make(map[RuleKey]map[NodeID]successfulOutcome)
			}
			results = make(map[NodeID]successfulOutcome)
			s.successful[r.Key] = results
		}
		result := successfulPass
		if d.Outcome == OutcomeNotApplicable {
			result = successfulNotApplicable
		}
		results[n.header.ID] = result
		return nil
	}
	// Problem dependencies retain only outcome, reason and cause, not copied
	// attacker-controlled value text or diagnostic location lists.
	cached := d
	cached.Expected = ""
	cached.Actual = ""
	cached.Related = nil
	bytes := int64(384 + len(cached.Rule.ID) + len(cached.Subject.DocumentID) + len(cached.Reason))
	if cached.Cause != nil {
		bytes += int64(128 + len(cached.Cause.Rule.ID) + len(cached.Cause.Subject.DocumentID))
	}
	if err := s.retain(bytes); err != nil {
		return err
	}
	s.decisions[id] = cloneDecision(cached)
	return nil
}
func (s *Session) dependencies(ctx context.Context, n *factNode, r RuleDefinition, final bool) (Decision, bool, error) {
	result := Decision{Outcome: OutcomePass}
	pending := false
	for _, dependency := range r.Requires {
		d, wait, err := s.dependency(ctx, n, dependency, final)
		if err != nil {
			return Decision{}, false, err
		}
		pending = pending || wait
		if dependencyOutcomeRank(d) > dependencyOutcomeRank(result) || (dependencyOutcomeRank(d) == dependencyOutcomeRank(result) && d.Outcome != OutcomePass && dependencyCauseLess(d, result)) {
			result = d
		}
	}
	if result.Outcome == OutcomeBlocked {
		return result, false, nil
	}
	if pending {
		return Decision{}, true, nil
	}
	return result, false, nil
}
func dependencyOutcomeRank(d Decision) int {
	switch d.Outcome {
	case OutcomeBlocked:
		return 3
	case OutcomeUnsupported:
		return 2
	case OutcomeUnknown:
		return 1
	}
	return 0
}
func dependencyCauseLess(a, b Decision) bool {
	if a.Cause == nil {
		return b.Cause == nil && a.Reason < b.Reason
	}
	if b.Cause == nil {
		return true
	}
	if a.Cause.Rule.ID != b.Cause.Rule.ID {
		return a.Cause.Rule.ID < b.Cause.Rule.ID
	}
	if a.Cause.Rule.Revision != b.Cause.Rule.Revision {
		return a.Cause.Rule.Revision < b.Cause.Rule.Revision
	}
	if a.Cause.Subject.DocumentID != b.Cause.Subject.DocumentID {
		return a.Cause.Subject.DocumentID < b.Cause.Subject.DocumentID
	}
	return a.Cause.Subject.Node < b.Cause.Subject.Node
}
func (s *Session) dependency(ctx context.Context, n *factNode, dependency Dependency, final bool) (Decision, bool, error) {
	if err := s.step(ctx, 1); err != nil {
		return Decision{}, false, err
	}
	if !validLocalRelation(dependency.Relation) {
		return Decision{Outcome: OutcomeUnsupported, Reason: "dependency-relation-unavailable"}, false, nil
	}
	if dependency.Kind == DependencyExternal {
		return Decision{Outcome: OutcomeUnsupported, Reason: "external-provider-unavailable"}, false, nil
	}
	if dependency.Kind == DependencyFact {
		if !final {
			return Decision{}, true, nil
		}
		at, ok := contextSubject(s, n, dependency.Relation)
		if !ok {
			return Decision{Outcome: OutcomeUnknown, Reason: "dependency-scope-absent"}, false, nil
		}
		fact, ok := s.contexts[ContextKey{Name: dependency.Key, At: at}]
		if !ok {
			return Decision{Outcome: OutcomeUnknown, Reason: "context-fact-unavailable"}, false, nil
		}
		if fact.Value.State != EvidenceKnown && fact.Value.State != EvidenceAbsent {
			return evidenceDecision(fact.Value.State, fact.Value.Reason, fact.Value.Cause, false), false, nil
		}
		complete, err := s.contextScopeComplete(ctx, n, dependency.Relation, dependency.Key)
		if err != nil {
			return Decision{}, false, err
		}
		if !complete {
			return Decision{Outcome: OutcomeUnknown, Reason: "context-scope-incomplete"}, false, nil
		}
		return Decision{Outcome: OutcomePass}, false, nil
	}
	var candidates []NodeID
	switch dependency.Relation {
	case RelationSameNode:
		candidates = []NodeID{n.header.ID}
	case RelationParent:
		if n.header.Parent == 0 {
			return Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-parent-absent"}, false, nil
		}
		candidates = []NodeID{n.header.Parent}
	case RelationDocument:
		if !final {
			return Decision{}, true, nil
		}
		prerequisite := s.documentPrerequisites[dependency.Rule]
		if prerequisite == nil || !prerequisite.finalized {
			return Decision{}, false, contract("document prerequisite", "final evaluation phase not established")
		}
		if prerequisite.ready {
			return prerequisite.decision, false, nil
		}
		if s.catalog.rules[dependency.Rule].Scope == ScopeDocument {
			if s.root != nil {
				candidates = []NodeID{s.root.header.ID}
			}
		} else {
			candidates = s.sequence
		}
		d, pending, err := s.scanPrerequisite(ctx, dependency, candidates, true)
		if err == nil && !pending {
			prerequisite.decision, prerequisite.ready = d, true
		}
		return d, pending, err
	}
	return s.scanPrerequisite(ctx, dependency, candidates, final)
}

// Keep subject-order causal priority identical for singleton and document
// prerequisites. Only finalized document aggregates are memoized by the caller.
func (s *Session) scanPrerequisite(ctx context.Context, dependency Dependency, candidates []NodeID, final bool) (Decision, bool, error) {
	seenApplicable := false
	var unresolved *Decision
	for _, subject := range candidates {
		if err := s.step(ctx, 1); err != nil {
			return Decision{}, false, err
		}
		d, ok := s.lookupDecision(decisionID{dependency.Rule, subject})
		if !ok {
			if !final {
				return Decision{}, true, nil
			}
			gap := Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-not-established"}
			unresolved = &gap
			continue
		}
		if d.Outcome == OutcomeNotApplicable {
			continue
		}
		seenApplicable = true
		if d.Outcome == OutcomePass {
			continue
		}
		mapped := d
		mapped.Expected = ""
		mapped.Actual = ""
		mapped.Related = nil
		if d.Outcome == OutcomeFail {
			mapped.Outcome = OutcomeBlocked
			mapped.Reason = "failed-prerequisite"
			mapped.Cause = &DecisionRef{Rule: d.Rule, Subject: d.Subject}
			return mapped, false, nil
		}
		if d.Outcome == OutcomeBlocked {
			if mapped.Cause == nil {
				mapped.Cause = &DecisionRef{Rule: d.Rule, Subject: d.Subject}
			}
			return mapped, false, nil
		}
		if unresolved == nil || mapped.Outcome == OutcomeUnsupported {
			copy := mapped
			unresolved = &copy
		}
	}
	if dependency.Relation == RelationDocument && (!s.inputComplete || s.enumerationIncomplete) {
		return Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-document-incomplete"}, false, nil
	}
	if unresolved != nil {
		return *unresolved, false, nil
	}
	if !seenApplicable {
		return Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-not-applicable"}, false, nil
	}
	return Decision{Outcome: OutcomePass}, false, nil
}
func (s *Session) evaluatePlan(ctx context.Context, n *factNode, r RuleDefinition, p Plan, final bool) (Decision, bool, error) {
	d := Decision{Rule: r.Key, Subject: NodeRef{s.documentID, n.header.ID}, Outcome: OutcomePass}
	switch p.kind {
	case planContextContainerMetadata, planContextSoundsMapping, planContextMusicXMLLinks, planContextOpusLinks, planContextMusicXMLHints, planContextOpusHints, planContextContainerHints, planContextSoundsHints, planContextOpusLayout, planContextLocal, planContextSpans, planContextRepeats, planContextNavigation, planContextLayout, planContextPerformance, planTimeOnlyOrder, planFigureNumberNumeric, planBendReleaseSign, planGraphicalRepeatContent:
		return s.evaluateContextPlan(ctx, n, r.Key, p, final)
	case planTimeline:
		return s.evaluateTimeline(ctx, n, r.Key, p.timeline, final)
	case planChordDuration, planNumeralKey:
		return s.evaluateTimelineConsumer(ctx, n, p.timelineConsumer, p.kind, final)
	case planLocal:
		return s.evaluateLocalPlan(ctx, n, p.local, final)
	case planTypedReference:
		return s.evaluateReferencePlan(ctx, n, p.reference, final)
	case planChildren:
		count := int64(0)
		var uncertain *Decision
		for _, child := range n.children {
			if err := s.step(ctx, 1); err != nil {
				return d, false, err
			}
			e := child.header.Declaration
			if e.State != EvidenceKnown {
				gap := evidenceDecision(e.State, e.Reason, e.Cause, false)
				if uncertain == nil || gap.Outcome == OutcomeBlocked || gap.Outcome == OutcomeUnsupported {
					uncertain = &gap
				}
				continue
			}
			for _, selection := range p.children {
				if err := s.step(ctx, 1); err != nil {
					return d, false, err
				}
				if e.Value == selection {
					count++
					break
				}
			}
		}
		d.Expected = "at least " + strconv.FormatInt(p.minimum, 10)
		if p.maximum >= 0 {
			d.Expected += " and at most " + strconv.FormatInt(p.maximum, 10)
		}
		d.Actual = strconv.FormatInt(count, 10)
		if p.maximum >= 0 && count > p.maximum {
			d.Outcome = OutcomeFail
			d.Reason = "child-count-maximum"
			return d, false, nil
		}
		if uncertain != nil {
			uncertain.Rule = d.Rule
			uncertain.Subject = d.Subject
			return *uncertain, false, nil
		}
		if n.end.ChildrenComplete.State != EvidenceKnown || !n.end.ChildrenComplete.Value {
			gap := evidenceDecision(n.end.ChildrenComplete.State, n.end.ChildrenComplete.Reason, n.end.ChildrenComplete.Cause, false)
			if gap.Outcome == OutcomePass {
				gap.Outcome = OutcomeUnknown
				gap.Reason = "children-incomplete"
			}
			gap.Rule = d.Rule
			gap.Subject = d.Subject
			return gap, false, nil
		}
		if count < p.minimum {
			d.Outcome = OutcomeFail
			d.Reason = "child-count-minimum"
		}
	case planAttribute:
		presence, _, err := s.attributeEvidence(ctx, n, p.attribute)
		if err != nil {
			return d, false, err
		}
		if presence.State != EvidenceKnown && presence.State != EvidenceAbsent {
			gap := evidenceDecision(presence.State, presence.Reason, presence.Cause, false)
			gap.Rule = d.Rule
			gap.Subject = d.Subject
			return gap, false, nil
		}
		actual := presence.State == EvidenceKnown && presence.Value
		d.Expected = strconv.FormatBool(p.required)
		d.Actual = strconv.FormatBool(actual)
		if actual != p.required {
			d.Outcome = OutcomeFail
			d.Reason = "attribute-presence"
		}
	case planAtomic:
		value := n.end.Scalar.Value
		if value.State != EvidenceKnown {
			gap := evidenceDecision(value.State, value.Reason, value.Cause, false)
			gap.Rule = d.Rule
			gap.Subject = d.Subject
			return gap, false, nil
		}
		if value.Value.Kind != p.expected.Kind {
			d.Outcome = OutcomeUnsupported
			d.Reason = "atomic-family-unavailable"
			return d, false, nil
		}
		if err := s.step(ctx, 1+atomicLogicalBytes(value.Value)/64+atomicLogicalBytes(p.expected)/64); err != nil {
			return d, false, err
		}
		matches, err := compareAtomic(value.Value, p.expected, p.comparison)
		if err != nil {
			return d, false, err
		}
		d.Expected = atomicDisplay(p.expected)
		d.Actual = atomicDisplay(value.Value)
		if !matches {
			d.Outcome = OutcomeFail
			d.Reason = "atomic-comparison"
		}
	case planContextValue, planContextDefault, planContextCompare:
		return evaluateContextPlan(ctx, s, n, r, p, final)
	case planIdentityDeclare, planIdentityReference:
		// The compiled registration phase settles every eligible declaration
		// before any declaration can establish uniqueness for the index.
		return evaluateIdentityPlan(ctx, s, n, r, p, final && !s.collectingIdentity)
	default:
		d.Outcome = OutcomeUnsupported
		d.Reason = "plan-unavailable"
	}
	return d, false, nil
}
func atomicDisplay(v Atomic) string {
	switch v.Kind {
	case AtomicInteger:
		return v.Integer.String()
	case AtomicDecimal:
		return v.Decimal.String()
	case AtomicQName:
		return "{" + v.Name.Space + "}" + v.Name.Local
	default:
		return v.Text
	}
}
func (s *Session) attributeEvidence(ctx context.Context, n *factNode, selector AttributeSelector) (Evidence[bool], *AttributeFact, error) {
	var uncertain *Evidence[bool]
	for i := range n.attributes {
		if err := s.step(ctx, 1); err != nil {
			return Evidence[bool]{}, nil, err
		}
		a := &n.attributes[i]
		if a.Declaration.State == EvidenceKnown && a.Declaration.Value == selector.Declaration && a.Name != selector.Name {
			return Evidence[bool]{}, nil, contract("attribute selector", "declaration/name facts contradict plan")
		}
		if a.Name != selector.Name {
			continue
		}
		if a.Declaration.State != EvidenceKnown {
			gap := Evidence[bool]{State: a.Declaration.State, Reason: a.Declaration.Reason, Cause: a.Declaration.Cause}
			if gap.State == EvidenceAbsent {
				continue
			}
			uncertain = &gap
			continue
		}
		if a.Declaration.Value == selector.Declaration {
			return a.Presence, a, nil
		}
	}
	if uncertain != nil {
		return *uncertain, nil, nil
	}
	complete := n.end.AttributesComplete
	if complete.State == EvidenceKnown && complete.Value {
		return Evidence[bool]{State: EvidenceAbsent}, nil, nil
	}
	if complete.State == EvidenceKnown {
		return Evidence[bool]{State: EvidenceUnknown, Reason: "attributes-incomplete"}, nil, nil
	}
	return Evidence[bool]{State: complete.State, Reason: complete.Reason, Cause: complete.Cause}, nil, nil
}
func (s *Session) lookupAttribute(ctx context.Context, n *factNode, selector AttributeSelector) (Evidence[Atomic], *Location, error) {
	p, a, err := s.attributeEvidence(ctx, n, selector)
	if err != nil {
		return Evidence[Atomic]{}, nil, err
	}
	if p.State != EvidenceKnown {
		return Evidence[Atomic]{State: p.State, Reason: p.Reason, Cause: p.Cause}, nil, nil
	}
	if a == nil {
		return Evidence[Atomic]{State: EvidenceUnknown, Reason: "attribute-value-unavailable"}, nil, nil
	}
	location := a.Location
	return a.Scalar.Value, &location, nil
}
