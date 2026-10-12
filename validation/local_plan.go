package validation

import (
	"context"
	"slices"
	"strings"
	"unsafe"
)

const planLocal planKind = "local-relation"

type localOperation string

const (
	localDistinct          localOperation = "distinct-child-attribute"
	localScalarSet         localOperation = "scalar-one-of"
	localAttributeSet      localOperation = "present-attribute-one-of"
	localNumberTokens      localOperation = "attribute-number-or-tokens"
	localPresenceAttribute localOperation = "presence-implies-attribute"
	localAttributeChild    localOperation = "attribute-implies-related-child"
	localDocumentChild     localOperation = "document-child-presence"
	localMarkerValues      localOperation = "document-marker-values"
)

// ChildValueRequirement compares one exact child use with a validated constant.
// AllowAbsent accepts only proven absence, never an incomplete child set.
type ChildValueRequirement struct {
	Child       DeclarationID
	Expected    Atomic
	AllowAbsent bool
}

type localPlan struct {
	Operation    localOperation
	Children     []DeclarationID
	Attribute    AttributeSelector
	Values       []Atomic
	Triggers     []AttributeSelector
	Owner        Selector
	Relation     RelationKind
	Requirements []ChildValueRequirement
}

// NewDistinctChildAttributePlan requires distinct effective attribute values on
// the selected direct children. Schema defaults come only from shared XSD evidence.
func NewDistinctChildAttributePlan(children []DeclarationID, attribute AttributeSelector) (Plan, error) {
	return newLocalPlan(localPlan{Operation: localDistinct, Children: children, Attribute: attribute})
}

// NewScalarOneOfPlan compares the validated element scalar with exact constants.
func NewScalarOneOfPlan(values []Atomic) (Plan, error) {
	return newLocalPlan(localPlan{Operation: localScalarSet, Values: values})
}

// NewPresentAttributeValuePlan checks an exact, physically present attribute;
// proven omission makes this clause not applicable, even when a default exists.
func NewPresentAttributeValuePlan(attribute AttributeSelector, values []Atomic) (Plan, error) {
	return newLocalPlan(localPlan{Operation: localAttributeSet, Attribute: attribute, Values: values})
}

// NewAttributeNumberOrTokensPlan accepts validated string members in tokens or
// validated decimal members in the inclusive exact range. It never parses strings.
func NewAttributeNumberOrTokensPlan(attribute AttributeSelector, tokens []string, minimum, maximum Decimal) (Plan, error) {
	if len(tokens) > 4094 {
		return Plan{}, contract("local plan", "too many tokens")
	}
	values := make([]Atomic, 0, len(tokens)+2)
	values = append(values, Atomic{Kind: AtomicDecimal, Decimal: minimum}, Atomic{Kind: AtomicDecimal, Decimal: maximum})
	for _, v := range tokens {
		values = append(values, Atomic{Kind: AtomicString, Text: v})
	}
	return newLocalPlan(localPlan{Operation: localNumberTokens, Attribute: attribute, Values: values})
}

// NewPresenceImpliesAttributePlan requires owner and one validated attribute
// value whenever any exact trigger attribute is physically present.
func NewPresenceImpliesAttributePlan(triggers []AttributeSelector, owner Selector, attribute AttributeSelector, expected Atomic) (Plan, error) {
	return newLocalPlan(localPlan{Operation: localPresenceAttribute, Triggers: triggers, Owner: owner, Attribute: attribute, Values: []Atomic{expected}})
}

// NewAttributeImpliesRelatedChildPlan requires a selected child when an effective
// attribute equals expected. Only same-node, parent and document relations exist.
func NewAttributeImpliesRelatedChildPlan(attribute AttributeSelector, expected Atomic, relation RelationKind, children []DeclarationID) (Plan, error) {
	return newLocalPlan(localPlan{Operation: localAttributeChild, Attribute: attribute, Values: []Atomic{expected}, Relation: relation, Children: children})
}

// NewDocumentChildPresencePlan requires a node with one of the exact declaration
// uses in this document. A complete document is needed to establish absence.
func NewDocumentChildPresencePlan(markers []DeclarationID) (Plan, error) {
	return newLocalPlan(localPlan{Operation: localDocumentChild, Children: markers})
}

// NewDocumentMarkerValuesPlan checks exact direct-child values when a selected
// marker occurs in this document. Complete marker absence makes it inapplicable.
func NewDocumentMarkerValuesPlan(markers []DeclarationID, requirements []ChildValueRequirement) (Plan, error) {
	return newLocalPlan(localPlan{Operation: localMarkerValues, Children: markers, Requirements: requirements})
}
func newLocalPlan(p localPlan) (Plan, error) { return checkedPlan(Plan{kind: planLocal, local: &p}) }
func validDeclarationSet(ds []DeclarationID) bool {
	if len(ds) == 0 || len(ds) > 4096 {
		return false
	}
	seen := make(map[DeclarationID]bool, len(ds))
	for _, d := range ds {
		if !validDeclaration(d) || seen[d] {
			return false
		}
		seen[d] = true
	}
	return true
}
func exactNodeSelector(s Selector) bool {
	if s.Assembly != nil {
		return false
	}
	if s.Declaration != nil {
		return s.Component == nil && validDeclaration(*s.Declaration)
	}
	return s.Component != nil && s.Component.Kind == ComponentType && validComponent(*s.Component)
}
func (p *localPlan) bounded() bool {
	return p != nil && len(p.Values) <= 4096 && len(p.Triggers) <= 4096 && len(p.Requirements) <= 4096 && len(p.Children) <= 4096
}
func (p *localPlan) valid() error {
	if p == nil || len(p.Values) > 4096 || len(p.Triggers) > 4096 || len(p.Requirements) > 4096 || len(p.Children) > 4096 {
		return contract("local plan", "invalid finite parameter collection")
	}
	for _, v := range p.Values {
		if err := validateAtomic(v); err != nil {
			return err
		}
	}
	switch p.Operation {
	case localDistinct, localAttributeChild, localDocumentChild, localMarkerValues:
		if !validDeclarationSet(p.Children) {
			return contract("local plan", "exact nonduplicate child declarations required")
		}
	case localScalarSet, localAttributeSet, localNumberTokens, localPresenceAttribute:
	default:
		return contract("local plan", "unknown operator")
	}
	switch p.Operation {
	case localDistinct, localAttributeSet, localNumberTokens, localPresenceAttribute, localAttributeChild:
		if !validAttributeSelector(p.Attribute) {
			return contract("local plan", "exact attribute required")
		}
	}
	switch p.Operation {
	case localScalarSet, localAttributeSet:
		if len(p.Values) == 0 {
			return contract("local plan", "empty scalar choices")
		}
		for _, v := range p.Values {
			if v.Kind != p.Values[0].Kind {
				return contract("local plan", "choices must share an atomic family")
			}
		}
	case localNumberTokens:
		if len(p.Values) < 2 || p.Values[0].Kind != AtomicDecimal || p.Values[1].Kind != AtomicDecimal {
			return contract("local plan", "exact decimal bounds required")
		}
		c, e := p.Values[0].Decimal.Cmp(p.Values[1].Decimal)
		if e != nil || c > 0 {
			return contract("local plan", "invalid decimal range")
		}
		for _, v := range p.Values[2:] {
			if v.Kind != AtomicString {
				return contract("local plan", "token choices must be strings")
			}
		}
	case localPresenceAttribute:
		if len(p.Triggers) == 0 || !exactNodeSelector(p.Owner) || len(p.Values) != 1 {
			return contract("local plan", "exact triggers, owner and value required")
		}
		seen := map[AttributeSelector]bool{}
		for _, a := range p.Triggers {
			if !validAttributeSelector(a) || seen[a] {
				return contract("local plan", "invalid duplicate trigger")
			}
			seen[a] = true
		}
	case localAttributeChild:
		if !validLocalRelation(p.Relation) || len(p.Values) != 1 {
			return contract("local plan", "unsupported relation or expected value")
		}
	case localMarkerValues:
		if len(p.Requirements) == 0 {
			return contract("local plan", "child values required")
		}
		seen := map[DeclarationID]bool{}
		for _, r := range p.Requirements {
			if !validDeclaration(r.Child) || seen[r.Child] {
				return contract("local plan", "exact nonduplicate child value required")
			}
			seen[r.Child] = true
			if err := validateAtomic(r.Expected); err != nil {
				return err
			}
		}
	}
	return nil
}
func localAttributeSize(a AttributeSelector) int64 {
	return int64(128 + len(a.Name.Space) + len(a.Name.Local) + len(a.Declaration.OwnerOccurrence) + len(a.Declaration.Occurrence))
}
func (p *localPlan) size() int64 {
	if p == nil {
		return 0
	}
	n := int64(256)
	for _, d := range p.Children {
		n += int64(64 + len(d.OwnerOccurrence) + len(d.Occurrence))
	}
	for _, v := range p.Values {
		n += 128 + atomicLogicalBytes(v)
	}
	n += localAttributeSize(p.Attribute)
	for _, a := range p.Triggers {
		n += localAttributeSize(a)
	}
	for _, r := range p.Requirements {
		n += int64(192+len(r.Child.OwnerOccurrence)+len(r.Child.Occurrence)) + atomicLogicalBytes(r.Expected)
	}
	if p.Owner.Declaration != nil {
		n += int64(96 + len(p.Owner.Declaration.OwnerOccurrence) + len(p.Owner.Declaration.Occurrence))
	}
	if p.Owner.Component != nil {
		n += int64(128 + len(p.Owner.Component.Name.Space) + len(p.Owner.Component.Name.Local) + len(p.Owner.Component.Occurrence))
	}
	return n
}
func cloneLocalAttribute(a AttributeSelector) AttributeSelector {
	a.Name = QName{strings.Clone(a.Name.Space), strings.Clone(a.Name.Local)}
	a.Declaration = cloneDeclaration(a.Declaration)
	return a
}
func cloneLocalSelector(s Selector) Selector {
	if s.Declaration != nil {
		d := cloneDeclaration(*s.Declaration)
		s.Declaration = &d
	}
	if s.Component != nil {
		c := *s.Component
		c.Name = QName{strings.Clone(c.Name.Space), strings.Clone(c.Name.Local)}
		c.Occurrence = strings.Clone(c.Occurrence)
		s.Component = &c
	}
	return s
}
func cloneDeclarationSet(ds []DeclarationID) []DeclarationID {
	out := make([]DeclarationID, len(ds))
	for i, d := range ds {
		out[i] = cloneDeclaration(d)
	}
	slices.SortFunc(out, compareLocalDeclaration)
	return out
}

func compareLocalDeclaration(a, b DeclarationID) int {
	if c := strings.Compare(string(a.Assembly), string(b.Assembly)); c != 0 {
		return c
	}
	if c := strings.Compare(a.OwnerOccurrence, b.OwnerOccurrence); c != 0 {
		return c
	}
	return strings.Compare(a.Occurrence, b.Occurrence)
}
func compareLocalAtomic(a, b Atomic) int {
	if c := strings.Compare(string(a.Kind), string(b.Kind)); c != 0 {
		return c
	}
	if c := strings.Compare(a.Name.Space, b.Name.Space); c != 0 {
		return c
	}
	if c := strings.Compare(a.Name.Local, b.Name.Local); c != 0 {
		return c
	}
	return strings.Compare(atomicDisplay(a), atomicDisplay(b))
}
func (p *localPlan) clone() *localPlan {
	if p == nil {
		return nil
	}
	q := *p
	q.Children = cloneDeclarationSet(p.Children)
	q.Attribute = cloneLocalAttribute(p.Attribute)
	q.Owner = cloneLocalSelector(p.Owner)
	q.Values = make([]Atomic, len(p.Values))
	for i, v := range p.Values {
		q.Values[i] = cloneAtomic(v)
	}
	q.Triggers = make([]AttributeSelector, len(p.Triggers))
	for i, a := range p.Triggers {
		q.Triggers[i] = cloneLocalAttribute(a)
	}
	q.Requirements = make([]ChildValueRequirement, len(p.Requirements))
	for i, r := range p.Requirements {
		q.Requirements[i] = ChildValueRequirement{Child: cloneDeclaration(r.Child), Expected: cloneAtomic(r.Expected), AllowAbsent: r.AllowAbsent}
	}
	switch q.Operation {
	case localScalarSet, localAttributeSet:
		slices.SortFunc(q.Values, compareLocalAtomic)
	case localNumberTokens:
		slices.SortFunc(q.Values[2:], compareLocalAtomic)
	}
	slices.SortFunc(q.Triggers, func(a, b AttributeSelector) int {
		if c := compareLocalDeclaration(a.Declaration, b.Declaration); c != 0 {
			return c
		}
		if c := strings.Compare(a.Name.Space, b.Name.Space); c != 0 {
			return c
		}
		return strings.Compare(a.Name.Local, b.Name.Local)
	})
	slices.SortFunc(q.Requirements, func(a, b ChildValueRequirement) int { return compareLocalDeclaration(a.Child, b.Child) })
	return &q
}
func (p *localPlan) digest() any {
	if p == nil {
		return nil
	}
	type value struct {
		Kind  AtomicKind
		Value string
		Name  QName
	}
	type requirement struct {
		Child       DeclarationID
		Value       value
		AllowAbsent bool
	}
	out := struct {
		Operation    localOperation
		Children     []DeclarationID
		Attribute    AttributeSelector
		Values       []value
		Triggers     []AttributeSelector
		Owner        Selector
		Relation     RelationKind
		Requirements []requirement
	}{Operation: p.Operation, Children: p.Children, Attribute: p.Attribute, Triggers: p.Triggers, Owner: p.Owner, Relation: p.Relation}
	for _, v := range p.Values {
		out.Values = append(out.Values, value{v.Kind, localDigestText(v), v.Name})
	}
	for _, r := range p.Requirements {
		out.Requirements = append(out.Requirements, requirement{r.Child, value{r.Expected.Kind, localDigestText(r.Expected), r.Expected.Name}, r.AllowAbsent})
	}
	return out
}

func localDigestText(a Atomic) string {
	if a.Kind == AtomicQName {
		return ""
	}
	return atomicDisplay(a)
}
func localGap[T any](e Evidence[T]) Decision {
	return evidenceDecision(e.State, e.Reason, e.Cause, false)
}
func localIncomplete(e Evidence[bool], reason Code) Decision {
	d := localGap(e)
	if d.Outcome == OutcomePass {
		d.Outcome = OutcomeUnknown
		d.Reason = reason
	}
	return d
}
func localWorst(a, b Decision) Decision {
	if dependencyOutcomeRank(b) > dependencyOutcomeRank(a) {
		return b
	}
	return a
}
func localMatches(ctx context.Context, s *Session, n *factNode, declarations []DeclarationID) (bool, Decision, error) {
	e := n.header.Declaration
	if e.State != EvidenceKnown {
		return false, localGap(e), nil
	}
	for _, d := range declarations {
		if err := s.step(ctx, 1); err != nil {
			return false, Decision{}, err
		}
		if e.Value == d {
			return true, Decision{Outcome: OutcomePass}, nil
		}
	}
	return false, Decision{Outcome: OutcomePass}, nil
}
func (s *Session) localScalar(ctx context.Context, n *factNode, a *AttributeSelector) (Evidence[Atomic], error) {
	if a != nil {
		presence, _, e := s.attributeEvidence(ctx, n, *a)
		if e != nil {
			return Evidence[Atomic]{}, e
		}
		if presence.State != EvidenceKnown && presence.State != EvidenceAbsent {
			return Evidence[Atomic]{State: presence.State, Reason: presence.Reason, Cause: presence.Cause}, nil
		}
	}
	if s.schema != nil {
		name := QName{}
		if a != nil {
			name = a.Name
			if s.schema.run != nil {
				bound := s.schema.nodes[n.header.ID]
				found := false
				for result := range s.schema.run.Attributes(bound.binding) {
					if err := s.step(ctx, 1); err != nil {
						return Evidence[Atomic]{}, err
					}
					if result.Name.Space == a.Name.Space && result.Name.Local == a.Name.Local {
						found = s.schema.active.declaration(result.Use) == a.Declaration
						break
					}
				}
				if !found {
					return Evidence[Atomic]{State: EvidenceUnsupported, Reason: "exact-attribute-use-unavailable"}, nil
				}
			}
		}
		return s.schemaEffectiveScalar(ctx, n.header.ID, name)
	}
	// Generic fact streams already promise domain-validated typed values. They
	// have no implicit defaults; production source/model catalogs use the kernel.
	if a == nil {
		return n.end.Scalar.Value, nil
	}
	v, _, e := s.lookupAttribute(ctx, n, *a)
	return v, e
}
func (s *Session) localPresence(ctx context.Context, n *factNode, declarations []DeclarationID, document bool) (Decision, *factNode, error) {
	gap := Decision{Outcome: OutcomePass}
	nodes := n.children
	if document {
		nodes = nil
	}
	scan := func(ch *factNode) (bool, error) {
		match, d, e := localMatches(ctx, s, ch, declarations)
		if e != nil {
			return false, e
		}
		gap = localWorst(gap, d)
		return match, nil
	}
	if document {
		for _, id := range s.sequence {
			ch := s.nodes[id]
			if err := s.step(ctx, 1); err != nil {
				return Decision{}, nil, err
			}
			match, e := scan(ch)
			if e != nil {
				return Decision{}, nil, e
			}
			if match {
				return Decision{Outcome: OutcomePass}, ch, nil
			}
			if !ch.closed || ch.end.ChildrenComplete.State != EvidenceKnown || !ch.end.ChildrenComplete.Value {
				gap = localWorst(gap, localIncomplete(ch.end.ChildrenComplete, "children-incomplete"))
			}
		}
		if !s.inputComplete {
			gap = localWorst(gap, Decision{Outcome: OutcomeUnknown, Reason: "document-incomplete"})
		}
	} else {
		for _, ch := range nodes {
			match, e := scan(ch)
			if e != nil {
				return Decision{}, nil, e
			}
			if match {
				return Decision{Outcome: OutcomePass}, ch, nil
			}
		}
		if !n.closed || n.end.ChildrenComplete.State != EvidenceKnown || !n.end.ChildrenComplete.Value {
			gap = localWorst(gap, localIncomplete(n.end.ChildrenComplete, "children-incomplete"))
		}
	}
	if gap.Outcome != OutcomePass {
		return gap, nil, nil
	}
	return Decision{Outcome: OutcomeFail, Reason: "required-related-child-absent", Expected: "at least one selected child", Actual: "0"}, nil, nil
}
func (s *Session) localCompare(ctx context.Context, v Evidence[Atomic], expected Atomic) (Decision, error) {
	if v.State != EvidenceKnown {
		return localGap(v), nil
	}
	if v.Value.Kind != expected.Kind {
		return Decision{Outcome: OutcomeUnsupported, Reason: "atomic-family-unavailable"}, nil
	}
	if e := s.step(ctx, 1+atomicLogicalBytes(v.Value)/64+atomicLogicalBytes(expected)/64); e != nil {
		return Decision{}, e
	}
	ok, e := compareAtomic(v.Value, expected, ComparisonEqual)
	if e != nil {
		return Decision{}, e
	}
	d := Decision{Outcome: OutcomePass}
	if !ok {
		d = Decision{Outcome: OutcomeFail, Reason: "local-value-mismatch", Expected: atomicDisplay(expected), Actual: atomicDisplay(v.Value)}
	}
	return d, e
}
func (s *Session) evaluateLocalPlan(ctx context.Context, n *factNode, p *localPlan, final bool) (Decision, bool, error) {
	if !final && (p.Operation == localDocumentChild || p.Operation == localMarkerValues || (p.Operation == localAttributeChild && p.Relation == RelationDocument)) {
		return Decision{}, true, nil
	}
	if !final && p.Operation == localAttributeChild && p.Relation == RelationParent {
		parent := s.nodes[n.header.Parent]
		if parent != nil && !parent.closed {
			return Decision{}, true, nil
		}
	}
	d := Decision{Outcome: OutcomePass}
	switch p.Operation {
	case localScalarSet, localAttributeSet, localNumberTokens:
		var attr *AttributeSelector
		if p.Operation != localScalarSet {
			attr = &p.Attribute
			presence, _, e := s.attributeEvidence(ctx, n, *attr)
			if e != nil {
				return d, false, e
			}
			if presence.State == EvidenceAbsent || (presence.State == EvidenceKnown && !presence.Value) {
				return Decision{Outcome: OutcomeNotApplicable, Reason: "proven-absent"}, false, nil
			}
			if presence.State != EvidenceKnown {
				return localGap(presence), false, nil
			}
		}
		v, e := s.localScalar(ctx, n, attr)
		if e != nil {
			return d, false, e
		}
		if v.State != EvidenceKnown {
			return localGap(v), false, nil
		}
		if p.Operation == localNumberTokens && v.Value.Kind == AtomicDecimal {
			if e := s.step(ctx, 1+atomicLogicalBytes(v.Value)/32); e != nil {
				return d, false, e
			}
			lo, e := v.Value.Decimal.Cmp(p.Values[0].Decimal)
			if e != nil {
				return d, false, e
			}
			hi, e := v.Value.Decimal.Cmp(p.Values[1].Decimal)
			if e != nil {
				return d, false, e
			}
			if lo < 0 || hi > 0 {
				d = Decision{Outcome: OutcomeFail, Reason: "local-number-range", Expected: atomicDisplay(p.Values[0]) + ".." + atomicDisplay(p.Values[1]), Actual: atomicDisplay(v.Value)}
			}
			return d, false, nil
		}
		values := p.Values
		if p.Operation == localNumberTokens {
			values = values[2:]
		}
		for _, expected := range values {
			comparison, e := s.localCompare(ctx, v, expected)
			if e != nil {
				return d, false, e
			}
			if comparison.Outcome == OutcomePass {
				return comparison, false, nil
			}
			if comparison.Outcome != OutcomeFail {
				return comparison, false, nil
			}
		}
		return Decision{Outcome: OutcomeFail, Reason: "local-value-not-allowed", Expected: "one of the reviewed typed constants", Actual: atomicDisplay(v.Value)}, false, nil
	case localPresenceAttribute:
		present := false
		gap := Decision{Outcome: OutcomePass}
		for _, a := range p.Triggers {
			e, _, err := s.attributeEvidence(ctx, n, a)
			if err != nil {
				return d, false, err
			}
			if e.State == EvidenceKnown && e.Value {
				present = true
				break
			}
			if e.State != EvidenceAbsent && e.State != EvidenceKnown {
				gap = localWorst(gap, localGap(e))
			}
		}
		if !present {
			if gap.Outcome != OutcomePass {
				return gap, false, nil
			}
			return Decision{Outcome: OutcomeNotApplicable, Reason: "trigger-absent"}, false, nil
		}
		owner := selectorDecision(p.Owner, n.header)
		if owner.Outcome == OutcomeNotApplicable {
			return Decision{Outcome: OutcomeFail, Reason: "local-owner-mismatch", Expected: "the required exact owner selector", Actual: n.header.Name.Local}, false, nil
		}
		if owner.Outcome != OutcomePass {
			return owner, false, nil
		}
		v, e := s.localScalar(ctx, n, &p.Attribute)
		if e != nil {
			return d, false, e
		}
		d, e = s.localCompare(ctx, v, p.Values[0])
		return d, false, e
	case localAttributeChild:
		v, e := s.localScalar(ctx, n, &p.Attribute)
		if e != nil {
			return d, false, e
		}
		if v.State == EvidenceAbsent {
			return Decision{Outcome: OutcomeNotApplicable, Reason: "trigger-absent"}, false, nil
		}
		match, e := s.localCompare(ctx, v, p.Values[0])
		if e != nil {
			return d, false, e
		}
		if match.Outcome == OutcomeFail {
			return Decision{Outcome: OutcomeNotApplicable, Reason: "trigger-value-mismatch"}, false, nil
		}
		if match.Outcome != OutcomePass {
			return match, false, nil
		}
		at := n
		if p.Relation == RelationParent {
			at = s.nodes[n.header.Parent]
		}
		if p.Relation == RelationDocument {
			at = s.root
		}
		if at == nil {
			return Decision{Outcome: OutcomeUnknown, Reason: "related-owner-unavailable"}, false, nil
		}
		d, _, e = s.localPresence(ctx, at, p.Children, p.Relation == RelationDocument)
		return d, false, e
	case localDocumentChild, localMarkerValues:
		marker, markerNode, e := s.localDocumentPresence(ctx, p)
		if e != nil {
			if p.Operation == localDocumentChild && marker.Outcome == OutcomeFail {
				return marker, false, planProofStop(n, p, nil, marker, e)
			}
			return Decision{}, false, e
		}
		if p.Operation == localDocumentChild {
			return marker, false, nil
		}
		if marker.Outcome == OutcomeFail {
			return Decision{Outcome: OutcomeNotApplicable, Reason: "document-marker-absent"}, false, nil
		}
		if marker.Outcome != OutcomePass {
			return marker, false, nil
		}
		unresolved := Decision{Outcome: OutcomePass}
		for _, requirement := range p.Requirements {
			presence, ch, e := s.localPresence(ctx, n, []DeclarationID{requirement.Child}, false)
			if e != nil {
				return d, false, e
			}
			if presence.Outcome == OutcomeFail && requirement.AllowAbsent {
				continue
			}
			if presence.Outcome != OutcomePass {
				if presence.Outcome == OutcomeFail {
					return presence, false, nil
				}
				unresolved = localWorst(unresolved, presence)
				continue
			}
			v, e := s.localScalar(ctx, ch, nil)
			if e != nil {
				return d, false, e
			}
			comparison, e := s.localCompare(ctx, v, requirement.Expected)
			if e != nil {
				return d, false, e
			}
			if comparison.Outcome != OutcomePass {
				if comparison.Outcome != OutcomeFail {
					unresolved = localWorst(unresolved, comparison)
					continue
				}
				if markerNode != nil {
					comparison, e = s.withProvedPlanRelated(n, p, nil, comparison, markerNode.header.Location)
					return comparison, false, e
				}
				return comparison, false, nil
			}
		}
		return unresolved, false, nil
	case localDistinct:
		gap := Decision{Outcome: OutcomePass}
		count := 0
		// Values borrow immutable session-owned atoms. Keep the first entry
		// on the stack so a two-child duplicate can be proved even when no
		// optional storage is available. Growing scratch is independently
		// reserved and released; schemaAtomic's session charges stay owned.
		type entry struct {
			node  *factNode
			value Atomic
		}
		var first [1]entry
		values := first[:0]
		// Charge the complete host-layout entry, including pointer and padding.
		const entryBytes = int64(unsafe.Sizeof(entry{}))
		var scratchBytes int64
		defer func() {
			if scratchBytes != 0 {
				_ = s.account.Release(BudgetBufferedBytes, scratchBytes)
			}
		}()
		for i, ch := range n.children {
			match, selection, e := localMatches(ctx, s, ch, p.Children)
			if e != nil {
				return d, false, e
			}
			gap = localWorst(gap, selection)
			if !match {
				continue
			}
			count++
			v, e := s.localScalar(ctx, ch, &p.Attribute)
			if e != nil {
				return d, false, e
			}
			if v.State != EvidenceKnown {
				gap = localWorst(gap, localGap(v))
				continue
			}
			for _, previous := range values {
				comparison, e := s.localCompare(ctx, v, previous.value)
				if e != nil {
					return d, false, e
				}
				if comparison.Outcome == OutcomePass {
					duplicate := Decision{Outcome: OutcomeFail, Reason: "duplicate-effective-child-value", Expected: "distinct effective child values", Actual: atomicDisplay(v.Value)}
					duplicate, e = s.withProvedPlanRelated(n, p, nil, duplicate, previous.node.header.Location, ch.header.Location)
					return duplicate, false, e
				}
				if comparison.Outcome != OutcomeFail {
					gap = localWorst(gap, comparison)
				}
			}
			// Compare before optional growth so an established duplicate is
			// never replaced by a scratch-allocation failure. The last child
			// needs no retained comparison entry.
			if i+1 == len(n.children) {
				continue
			}
			if len(values) == cap(values) {
				capacity := cap(values) + min(cap(values), len(n.children)-cap(values))
				maxBytes := int64(^uint(0) >> 1)
				if int64(capacity) > maxBytes/entryBytes {
					return d, false, &ResourceError{Kind: BudgetBufferedBytes, Limit: maxBytes, Requested: maxBytes}
				}
				bytes := int64(capacity) * entryBytes
				if err := s.step(ctx, 1+int64(len(values))); err != nil {
					return d, false, err
				}
				// Both backing arrays are live during the checked copy.
				if err := s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
					return d, false, err
				}
				grown := make([]entry, len(values), capacity)
				copy(grown, values)
				if scratchBytes != 0 {
					_ = s.account.Release(BudgetBufferedBytes, scratchBytes)
				}
				values, scratchBytes = grown, bytes
			}
			values = append(values, entry{node: ch, value: v.Value})
		}
		if !n.closed || n.end.ChildrenComplete.State != EvidenceKnown || !n.end.ChildrenComplete.Value {
			gap = localWorst(gap, localIncomplete(n.end.ChildrenComplete, "children-incomplete"))
		}
		if gap.Outcome != OutcomePass {
			return gap, false, nil
		}
		if count == 0 {
			return Decision{Outcome: OutcomeNotApplicable, Reason: "selected-children-absent"}, false, nil
		}
		return d, false, nil
	}
	return Decision{Outcome: OutcomeUnsupported, Reason: "local-operator-unavailable"}, false, nil
}

// Document scans are cached per immutable plan only at final evaluation. The
// cache owns no facts; its entries are accounted and released with the session.
type localDocumentResult struct {
	decision Decision
	node     *factNode
}

func (s *Session) localDocumentPresence(ctx context.Context, p *localPlan) (Decision, *factNode, error) {
	if err := s.step(ctx, 1); err != nil {
		return Decision{}, nil, err
	}
	if known, ok := s.localDocuments[p]; ok {
		return known.decision, known.node, nil
	}
	d, n, err := s.localPresence(ctx, s.root, p.Children, true)
	if err != nil {
		return d, n, err
	}
	if err := s.retain(384); err != nil {
		return d, n, err
	}
	if s.localDocuments == nil {
		s.localDocuments = make(map[*localPlan]localDocumentResult)
	}
	s.localDocuments[p] = localDocumentResult{d, n}
	return d, n, nil
}
