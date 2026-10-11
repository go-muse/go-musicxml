package validation

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

// Comparison is a bounded exact comparison over one already-typed value.
type Comparison string

// Exact comparison operators; ordering is restricted to integer and decimal values.
const (
	ComparisonEqual        Comparison = "equal"
	ComparisonNotEqual     Comparison = "not-equal"
	ComparisonLess         Comparison = "less"
	ComparisonLessEqual    Comparison = "less-equal"
	ComparisonGreater      Comparison = "greater"
	ComparisonGreaterEqual Comparison = "greater-equal"
)

// AttributeSelector names one exact attribute declaration/use on an owner.
// It is not a wildcard scalar-subject ABI; each plan assesses at most this slot.
type AttributeSelector struct {
	Name        QName
	Declaration DeclarationID
}

type planKind string

const (
	planXSD               planKind = "xsd-composite"
	planChildren          planKind = "child-count"
	planAttribute         planKind = "attribute-presence"
	planAtomic            planKind = "atomic-comparison"
	planContextValue      planKind = "context-value"
	planContextDefault    planKind = "context-default"
	planContextCompare    planKind = "context-comparison"
	planIdentityDeclare   planKind = "identity-declaration"
	planIdentityReference planKind = "identity-reference"
)

// Plan is an immutable sealed typed program. Its zero value is invalid; use a
// constructor. Plans contain no callback, XML parser, runtime expression or DSL.
type Plan struct {
	schema           *CompiledSchema
	xsdCheck         XSDCheck
	kind             planKind
	children         []DeclarationID
	minimum, maximum int64
	attribute        AttributeSelector
	required         bool
	comparison       Comparison
	expected         Atomic
	key              string
	relation         RelationKind
}

// NewChildCountPlan counts only exact child declarations. Maximum -1 means no
// normative maximum, while session node/work budgets always remain finite.
func NewChildCountPlan(children []DeclarationID, minimum, maximum int64) (Plan, error) {
	p := Plan{kind: planChildren, children: children, minimum: minimum, maximum: maximum}
	return checkedPlan(p)
}

// NewAttributePresencePlan checks one exact present/absent attribute slot.
func NewAttributePresencePlan(attribute AttributeSelector, required bool) (Plan, error) {
	return checkedPlan(Plan{kind: planAttribute, attribute: attribute, required: required})
}

// NewAtomicComparisonPlan compares an element's already-typed value. It does not
// parse lexical values or implement an XSD datatype/facet/union validator.
func NewAtomicComparisonPlan(comparison Comparison, expected Atomic) (Plan, error) {
	return checkedPlan(Plan{kind: planAtomic, comparison: comparison, expected: expected})
}

// NewContextValuePlan publishes a copied constant when its exact node selector
// matches. It does not invent a default for unknown/absent input or musical time.
func NewContextValuePlan(key string, relation RelationKind, value Atomic) (Plan, error) {
	return checkedPlan(Plan{kind: planContextValue, key: key, relation: relation, expected: value})
}

// NewContextDefaultPlan publishes the exact attribute value when present, or
// the reviewed fallback only when the exact attribute slot is proven absent.
// Unknown or unsupported presence/value evidence never activates the fallback.
func NewContextDefaultPlan(key string, relation RelationKind, attribute AttributeSelector, fallback Atomic) (Plan, error) {
	return checkedPlan(Plan{kind: planContextDefault, key: key, relation: relation, attribute: attribute, expected: fallback})
}

// NewContextComparisonPlan compares one exact generic context fact; absent
// providers, conflicting values and missing evidence cannot become defaults.
func NewContextComparisonPlan(key string, relation RelationKind, comparison Comparison, expected Atomic) (Plan, error) {
	return checkedPlan(Plan{kind: planContextCompare, key: key, relation: relation, comparison: comparison, expected: expected})
}

// NewIdentityDeclarationPlan indexes one upstream-domain-validated string slot.
// This is generic document identity, not XSD actual-type/ID lexical assessment.
func NewIdentityDeclarationPlan(index string, attribute AttributeSelector) (Plan, error) {
	return checkedPlan(Plan{kind: planIdentityDeclare, key: index, attribute: attribute})
}

// NewIdentityReferencePlan resolves one upstream-domain-validated string slot
// only after document closure. It adds no musical target-kind or scope rule.
func NewIdentityReferencePlan(index string, attribute AttributeSelector) (Plan, error) {
	return checkedPlan(Plan{kind: planIdentityReference, key: index, attribute: attribute})
}
func checkedPlan(p Plan) (Plan, error) {
	if err := p.valid(); err != nil {
		return Plan{}, err
	}
	return p.clone(), nil
}
func validComparison(c Comparison) bool {
	switch c {
	case ComparisonEqual, ComparisonNotEqual, ComparisonLess, ComparisonLessEqual, ComparisonGreater, ComparisonGreaterEqual:
		return true
	}
	return false
}
func validLocalRelation(r RelationKind) bool {
	return r == RelationSameNode || r == RelationParent || r == RelationDocument
}
func validAttributeSelector(a AttributeSelector) bool {
	return validQName(a.Name) && validDeclaration(a.Declaration)
}
func (p Plan) valid() error {
	if len(p.children) > 4096 {
		return contract("plan", "too many child declarations")
	}
	if p.configSize() > 4<<20 {
		return contract("plan", "parameters exceed the finite configuration bound")
	}
	switch p.kind {
	case planXSD:
		if p.schema == nil || p.schema.kernel == nil || !validAssembly(p.schema.assembly) || !validXSDCheck(p.xsdCheck) {
			return contract("XSD plan", "compiled schema and supported operation required")
		}
	case planChildren:
		if len(p.children) == 0 || len(p.children) > 4096 || p.minimum < 0 || p.maximum < -1 || (p.maximum >= 0 && p.maximum < p.minimum) {
			return contract("child-count plan", "invalid declarations or bounds")
		}
		seen := map[DeclarationID]bool{}
		for _, d := range p.children {
			if !validDeclaration(d) || seen[d] {
				return contract("child-count plan", "invalid or duplicate declaration")
			}
			seen[d] = true
		}
	case planAttribute:
		if !validAttributeSelector(p.attribute) {
			return contract("attribute plan", "exact attribute declaration/name required")
		}
	case planAtomic:
		if err := validateComparison(p.comparison, p.expected); err != nil {
			return err
		}
	case planContextValue, planContextDefault:
		if !validID(p.key) || !validLocalRelation(p.relation) {
			return contract("context provider", "invalid key or unsupported relation")
		}
		if p.kind == planContextDefault && !validAttributeSelector(p.attribute) {
			return contract("context default", "exact attribute selector required")
		}
		if err := validateAtomic(p.expected); err != nil {
			return err
		}
	case planContextCompare:
		if !validID(p.key) || !validLocalRelation(p.relation) {
			return contract("context comparison", "invalid key or unsupported relation")
		}
		if err := validateComparison(p.comparison, p.expected); err != nil {
			return err
		}
	case planIdentityDeclare, planIdentityReference:
		if !validID(p.key) || !validAttributeSelector(p.attribute) {
			return contract("identity plan", "exact index/attribute declaration required")
		}
	default:
		return contract("plan", "zero or unknown typed plan")
	}
	return nil
}
func validateComparison(c Comparison, a Atomic) error {
	if !validComparison(c) {
		return contract("comparison", "unknown operator")
	}
	if err := validateAtomic(a); err != nil {
		return err
	}
	if c != ComparisonEqual && c != ComparisonNotEqual && a.Kind != AtomicInteger && a.Kind != AtomicDecimal {
		return contract("comparison", "ordering is limited to exact numbers")
	}
	return nil
}
func (p Plan) clone() Plan {
	p.children = append([]DeclarationID(nil), p.children...)
	for i := range p.children {
		p.children[i] = cloneDeclaration(p.children[i])
	}
	slices.SortFunc(p.children, func(a, b DeclarationID) int {
		if n := cmp.Compare(a.Assembly, b.Assembly); n != 0 {
			return n
		}
		if n := cmp.Compare(a.OwnerOccurrence, b.OwnerOccurrence); n != 0 {
			return n
		}
		return cmp.Compare(a.Occurrence, b.Occurrence)
	})
	p.kind = planKind(strings.Clone(string(p.kind)))
	p.comparison = Comparison(strings.Clone(string(p.comparison)))
	p.relation = RelationKind(strings.Clone(string(p.relation)))
	p.key = strings.Clone(p.key)
	p.attribute.Name = QName{strings.Clone(p.attribute.Name.Space), strings.Clone(p.attribute.Name.Local)}
	p.attribute.Declaration = cloneDeclaration(p.attribute.Declaration)
	p.expected = cloneAtomic(p.expected)
	return p
}
func (p Plan) validateFor(r RuleDefinition) error {
	if p.kind == planXSD {
		if r.Selector.Assembly == nil || p.schema == nil || *r.Selector.Assembly != p.schema.assembly || r.Category != CategoryNormative || r.Class != ClassMandatory || r.Role != RolePredicate {
			return contract("XSD plan", "matching assembly selector and required normative predicate role required")
		}
		for _, target := range r.Targets {
			if target != TargetSource {
				return contract("XSD plan", "only source evidence is implemented")
			}
		}
		if (p.xsdCheck == XSDCheckIdentity) != (r.Scope == ScopeDocument) {
			return contract("XSD plan", "identity requires document scope; other operations require node scope")
		}
	} else if r.Selector.Assembly != nil {
		return contract("plan selector", "assembly selection is reserved for XSD plans")
	}
	if err := p.valid(); err != nil {
		return err
	}
	if r.Scope != ScopeNode && r.Scope != ScopeDocument {
		return contract("plan scope", "this scope is not executable")
	}
	if r.Selector.Component != nil && r.Selector.Component.Kind != ComponentType {
		return contract("plan selector", "only exact type components can select a node")
	}
	if p.kind == planContextValue || p.kind == planContextDefault {
		if r.Role != RoleContextProvider {
			return contract("context provider", "role must be context-provider")
		}
	} else if r.Role == RoleContextProvider {
		return contract("plan role", "only a context-value plan produces context")
	}
	if (p.kind == planIdentityDeclare || p.kind == planIdentityReference) && r.Scope != ScopeNode {
		return contract("identity plan", "identity slots require node scope")
	}
	return nil
}
func (p Plan) digestValue() any {
	if p.kind == planXSD {
		return struct {
			Kind           planKind
			SchemaDigest   string
			SchemaAssembly Assembly
			Check          XSDCheck
		}{p.kind, p.schema.Digest(), p.schema.Assembly(), p.xsdCheck}
	}
	type atomicDigest struct {
		Kind AtomicKind
		Text string
		Name QName
	}
	a := atomicDigest{Kind: p.expected.Kind, Text: p.expected.Text, Name: p.expected.Name}
	if p.expected.Kind == AtomicInteger {
		a.Text = p.expected.Integer.String()
	}
	if p.expected.Kind == AtomicDecimal {
		a.Text = p.expected.Decimal.String()
	}
	return struct {
		Kind             planKind
		Children         []DeclarationID
		Minimum, Maximum int64
		Attribute        AttributeSelector
		Required         bool
		Comparison       Comparison
		Expected         atomicDigest
		Key              string
		Relation         RelationKind
	}{p.kind, p.children, p.minimum, p.maximum, p.attribute, p.required, p.comparison, a, p.key, p.relation}
}
func (p Plan) configSize() int64 {
	n := int64(256+len(p.key)+len(p.attribute.Name.Space)+len(p.attribute.Name.Local)+len(p.attribute.Declaration.OwnerOccurrence)+len(p.attribute.Declaration.Occurrence)+len(p.expected.Text)+len(p.expected.Name.Space)+len(p.expected.Name.Local)) + p.expected.Integer.ByteSize() + p.expected.Decimal.ByteSize()
	for _, d := range p.children {
		n += int64(64 + len(d.OwnerOccurrence) + len(d.Occurrence))
	}
	return n
}
func (p Plan) metadataBytes() int64 {
	encoded, _ := json.Marshal(p.digestValue())
	return int64(len(encoded))
}
func compareAtomic(actual, expected Atomic, comparison Comparison) (bool, error) {
	if err := validateAtomic(actual); err != nil {
		return false, err
	}
	if actual.Kind != expected.Kind {
		return false, contract("atomic comparison", "different exact value families")
	}
	cmp := 0
	switch actual.Kind {
	case AtomicInteger:
		var err error
		cmp, err = actual.Integer.Cmp(expected.Integer)
		if err != nil {
			return false, err
		}
	case AtomicDecimal:
		var err error
		cmp, err = actual.Decimal.Cmp(expected.Decimal)
		if err != nil {
			return false, err
		}
	case AtomicString, AtomicBoolean:
		cmp = strings.Compare(actual.Text, expected.Text)
	case AtomicQName:
		if actual.Name != expected.Name {
			cmp = 1
		}
	}
	switch comparison {
	case ComparisonEqual:
		return cmp == 0, nil
	case ComparisonNotEqual:
		return cmp != 0, nil
	case ComparisonLess:
		return cmp < 0, nil
	case ComparisonLessEqual:
		return cmp <= 0, nil
	case ComparisonGreater:
		return cmp > 0, nil
	case ComparisonGreaterEqual:
		return cmp >= 0, nil
	}
	return false, contract("comparison", "unknown operator")
}
