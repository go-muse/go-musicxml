package xsd

import (
	"fmt"

	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	XSDNamespace = "http://www.w3.org/2001/XMLSchema"
	XSINamespace = "http://www.w3.org/2001/XMLSchema-instance"
)

// ValidationIssue describes one XSD validation failure.
type Issue struct {
	Path       string
	Constraint string
	Message    string
}

// String returns the issue path and message.
func (i Issue) String() string {
	if i.Path == "" {
		return i.Message
	}

	return i.Path + ": " + i.Message
}

type QName = struct {
	Space string
	Local string
}

type SchemaData struct {
	Types    map[QName]*TypeSchema
	Elements map[QName]*ElementSchema
}

type TypeSchema struct {
	Simple  *SimpleSchema
	Complex *ComplexSchema
}

type TypeRef struct {
	Name          QName
	InlineSimple  *SimpleSchema
	InlineComplex *ComplexSchema
}

type SimpleForm string

const (
	SimpleRestriction SimpleForm = "restriction"
	SimpleUnion       SimpleForm = "union"
	SimpleList        SimpleForm = "list"
)

type SimpleSchema struct {
	Final        Derivation
	Form         SimpleForm
	Base         *SimpleMember
	Enumerations []string
	Members      []*SimpleMember
	Item         *SimpleMember
	Patterns     []string

	MinInclusive   string
	MaxInclusive   string
	MinExclusive   string
	MaxExclusive   string
	Length         uint64
	MinLength      uint64
	MaxLength      uint64
	TotalDigits    uint64
	FractionDigits uint64
	WhiteSpace     string

	HasMinInclusive   bool
	HasMaxInclusive   bool
	HasMinExclusive   bool
	HasMaxExclusive   bool
	HasLength         bool
	HasMinLength      bool
	HasMaxLength      bool
	HasTotalDigits    bool
	HasFractionDigits bool
}

type SimpleMember struct {
	Name   QName
	Inline *SimpleSchema
}

type ComplexForm string

const (
	ComplexDirect                    ComplexForm = "direct"
	ComplexSimpleContentExtension    ComplexForm = "simple content extension"
	ComplexSimpleContentRestriction  ComplexForm = "simple content restriction"
	ComplexComplexContentExtension   ComplexForm = "complex content extension"
	ComplexComplexContentRestriction ComplexForm = "complex content restriction"
)

type ComplexSchema struct {
	Block, Final Derivation
	Abstract     bool
	// SimpleContent is a local restriction layer. A missing scalar Base is
	// bound to the inherited effective scalar in a per-context copy.
	SimpleContent *SimpleSchema
	Form          ComplexForm
	Base          *TypeRef
	Particle      *ParticleSchema
	Attributes    []AttributeSchema
	AnyAttribute  *AnyAttributeSchema
	Mixed         bool
}

type AttributeUse string

const (
	AttributeOptional   AttributeUse = "optional"
	AttributeRequired   AttributeUse = "required"
	AttributeProhibited AttributeUse = "prohibited"
)

type AttributeSchema struct {
	Name       QName
	Type       TypeRef
	Use        AttributeUse
	Constraint *ValueConstraint
}

type ValueConstraint struct {
	Kind  string
	Value string
}

type AnyAttributeSchema struct {
	Namespace       string
	ProcessContents string
}

type ParticleKind string

const (
	ParticleElement  ParticleKind = "element"
	ParticleAll      ParticleKind = "all"
	ParticleChoice   ParticleKind = "choice"
	ParticleSequence ParticleKind = "sequence"
	ParticleAny      ParticleKind = "any"
)

type Occurrence struct {
	Min       uint64
	Max       uint64
	Unbounded bool
}

type ParticleSchema struct {
	Kind       ParticleKind
	Occurrence Occurrence
	Element    *ElementSchema
	Children   []*ParticleSchema
	Any        *AnySchema
}

type AnySchema struct {
	Namespace       string
	ProcessContents string
}

type ElementSchema struct {
	Block, Final Derivation
	Name         QName
	Reference    QName
	Type         TypeRef
	Default      *string
	Fixed        *string
	Nillable     bool
	Abstract     bool
}

type Node struct {
	ID         SubjectID
	Namespaces map[string]string
	Name       QName
	Attrs      []Attribute
	Text       strings.Builder
	Children   []*Node
}

// validationAttribute retains lexical declaration provenance because an ordinary
// attribute in a namespace whose literal URI is "xmlns" has the same expanded
// xml.Name as a namespace declaration.
type Attribute struct {
	Attr
	NamespaceDeclaration bool
	ValueKnown           bool
}

type Context struct {
	identitySource    identityOrigin
	run               *Run
	local             *boundState
	AssessmentErr     error
	AssessmentVersion uint64
	TypeMetadata      map[TypeIdentity]uint8
	Derivations       map[DerivationKey]bool
	Schema            *SchemaData
	Issues            []Issue
	Effective         map[*ComplexSchema]*EffectiveComplex
	ParticleNullable  map[*ParticleSchema]bool // prepared immutable metadata, scoped to this validation
	// These per-context, schema-keyed caches are lazy: simpleSemantics is
	// needed only when a simple schema is reached; enumerationValues only
	// when enumeration literals need immutable lexical and value views.
	SimpleSemantics    map[*SimpleSchema]*SimpleSemantics
	EnumerationValues  map[EnumerationKey]EnumerationValue
	Identifiers        map[string]string
	References         []IdentityReference
	IdentityEntries    []IdentityEntry
	IdentityEntrySeen  map[string]bool
	IdentityIncomplete bool
	IdentityBindings   map[string]*IdentityBindings
}

type IdentityReference struct {
	origin identityOrigin
	Value  string
	Path   string
}

type EffectiveComplex struct {
	Particle     *ParticleSchema
	Attributes   []AttributeSchema
	AnyAttribute *AnyAttributeSchema
	Simple       *TypeRef
	Mixed        bool
}

type ElementMatch struct {
	Particle *ParticleSchema
	Index    int
	Schema   *ElementSchema
}

type MatchResult struct {
	Next     int
	Elements []ElementMatch
}

type MatchFailure struct {
	Schema   string
	Position int
	Expected []QName
}

type SimpleFailure struct {
	Constraint string
	Message    string
}

func (c *Context) ValidateElement(
	node *Node,
	schema *ElementSchema,
	path string,
) {
	c.ValidateElementOwned(node, schema, path, nil)
}

// owner is the original containing element, never an effective default-value view.
// The returned declaration-local success is distinct from type success and from
// independently assessed standard xsi attributes. Neither gate uses document-wide
// issue state: a parent/sibling failure cannot erase this item's binding facts.
func (c *Context) ValidateElementOwned(node *Node, schema *ElementSchema, path string, owner *Node) bool {
	original := node
	c.setFamily("declaration")
	c.ValidateSchemaLocationHints(node, path)

	resolved, found := c.ResolveElement(schema)
	if !found {
		c.AddIssue(
			path,
			"schema",
			"element declaration cannot be resolved",
		)
		return false
	}
	schema = resolved

	if node.Name.Space != schema.Name.Space ||
		node.Name.Local != schema.Name.Local {
		c.AddIssue(
			path,
			"name",
			fmt.Sprintf(
				"got %s, expected %s",
				DisplayName(node.Name),
				DisplayQName(schema.Name),
			),
		)
		return false
	}

	c.setFamily("declaration")
	if schema.Abstract {
		c.AddIssue(path, "abstract", "abstract element cannot be used")
	}

	// Nomination validity belongs to cvc-elt.4. The returned fallback type can
	// still independently validate an ID-bearing item when nomination fails.
	c.setFamily("declaration")
	actualType, declarationValid := c.ActualElementType(node, schema, path)
	declarationValid = declarationValid && !schema.Abstract
	if c.local != nil {
		c.local.actual = actualType
		c.local.declared = &schema.Type
	}

	// This is an element constraint, not a scalar constraint: validateType is
	// also reused for complex simple-content bases whose attributes are legal.
	simple, _, builtin, _ := c.ResolveType(actualType)
	c.setFamily("attributes")
	typeAttributesValid := true
	if simple != nil || (builtin != "" && builtin != "anyType") {
		typeAttributesValid = c.ValidateSimpleElementAttributes(node, path)
	}

	// xsi:nil is an xs:boolean regardless of the declared element type.
	// XSD 1.0 cvc-elt 3 requires nillability even for false/0, and true
	// forbids all character/element content as well as a fixed constraint.
	// https://www.w3.org/TR/xmlschema-1/#cvc-elt
	c.setFamily("declaration")
	nilValue, nilPresent := NilAttribute(node.Attrs)
	nilled := false
	if nilPresent {
		nilPath := AttributePath(path, QName{Space: XSINamespace, Local: "nil"})
		if value, failure := ParseBoolean(nilValue); failure != nil {
			c.AddIssue(nilPath, failure.Constraint, failure.Message)
		} else {
			nilled = schema.Nillable && value
		}
		if !schema.Nillable {
			declarationValid = false
			c.AddIssue(nilPath, "nillable", "element is not nillable")
		}
	}

	if nilled {
		if schema.Fixed != nil {
			declarationValid = false
			c.AddIssue(path, "fixed", "nil element must not have a fixed value constraint")
		}
		if len(node.Children) != 0 || node.Text.Len() != 0 {
			c.SkipIdentityChildren(node)
			declarationValid = false
			c.AddIssue(path, "nillable", "nil element must not contain character data or child elements")
		}
	}

	// XSD 1.0 cvc-elt 5.1/5.1.2 applies an element value constraint only
	// when there are no character or element children and the element is not
	// nilled. Whitespace is character content; comments and PIs are not.
	// Use a validation-only view: the source text and model stay unchanged.
	defaulted := false
	constraintBlocked := false
	if !nilled && (c.local == nil || c.local.subject.TextKnown) && len(node.Children) == 0 && node.Text.Len() == 0 {
		value := schema.Default
		if schema.Fixed != nil {
			value = schema.Fixed
		}
		if value != nil {
			// A fresh builder avoids copying strings.Builder's internal state.
			effective := &Node{Name: node.Name, Attrs: node.Attrs, Children: node.Children, Namespaces: node.Namespaces}
			lexical := *value
			if actualType != &schema.Type {
				var ok bool
				lexical, ok = c.CanonicalElementConstraint(&schema.Type, *value)
				if !ok {
					constraintBlocked = true
				}
				before := c.AssessmentVersion
				if !c.ElementDefaultImmediate(actualType) {
					constraintBlocked = true
					if c.AssessmentVersion == before {
						c.AddIssue(path, "default", "value constraint is not valid for the actual content type")
					}
				}
			}
			effective.Text.WriteString(lexical)
			defaulted = true
			if c.local != nil {
				c.local.defaulted = true
			}
			node = effective
		}
	}

	if !nilled && !defaulted && schema.Fixed != nil && (c.local == nil || c.local.subject.TextKnown) {
		beforeFixed := c.AssessmentVersion
		// XSD 1.0 cvc-elt 5.2.2.1 forbids element children independently
		// of fixed-value equality, including for mixed content. Report the
		// structural failure first, but keep ordinary type assessment below.
		if len(node.Children) != 0 {
			declarationValid = false
			c.AddIssue(path, "fixed", "element with a fixed value must not contain child elements")
		} else if !c.ElementConstraintEqual(actualType, &schema.Type, node.Text.String(), *schema.Fixed) {
			declarationValid = false
			c.AddIssue(
				path,
				"fixed",
				fmt.Sprintf(
					"value %q does not equal fixed value %q",
					node.Text.String(),
					*schema.Fixed,
				),
			)
		}
		declarationValid = declarationValid && c.AssessmentVersion == beforeFixed
	}

	// An unavailable/invalid default blocks dependent scalar/content checks,
	// while actual-type attribute checks remain independent.
	if c.local != nil {
		c.local.effectiveText = node.Text.String()
	}
	c.setFamily("scalar")
	typeValid := c.ValidateType(node, actualType, path, nilled || constraintBlocked, original) && typeAttributesValid
	declarationValid = declarationValid && typeValid && !constraintBlocked
	if c.local != nil {
		c.local.typeValid = typeValid
		c.local.declarationValid = declarationValid
		c.local.identityAllowed = !nilled && !constraintBlocked && c.local.subject.TextKnown
		return declarationValid
	}
	if typeValid && !nilled && !constraintBlocked {
		c.RecordElementIdentity(&schema.Type, actualType, node.Text.String(), path, owner, declarationValid)
	}
	return declarationValid
}

func (c *Context) ResolveElement(
	schema *ElementSchema,
) (*ElementSchema, bool) {
	if schema == nil {
		return nil, false
	}
	if schema.Reference.Local == "" {
		return schema, true
	}

	resolved, found := c.Schema.Elements[schema.Reference]
	return resolved, found && resolved != nil
}

// nilled suppresses scalar and element content validation only. Type resolution
// and complex attribute validation (including identity recording) still apply.
func (c *Context) ValidateType(
	node *Node,
	reference *TypeRef,
	path string,
	nilled bool,
	subject *Node,
) bool {
	simple, complex, builtin, found := c.ResolveType(reference)
	if !found {
		c.AddIssue(path, "schema", "type declaration cannot be resolved")
		return false
	}

	if !c.ValidTypeMetadata(TypeIdentity{Simple: simple, Complex: complex, Builtin: builtin}) {
		return false
	}

	switch {
	case simple != nil:
		if nilled {
			return true
		}
		if len(node.Children) != 0 {
			c.IdentityIncomplete = true
			c.AddIssue(
				path,
				"simple-content",
				"simple value must not contain child elements",
			)
			return false
		}
		if c.local != nil && !c.local.subject.TextKnown {
			return false
		}
		assessment, failure := c.AssessSimpleValue(simple, node.Text.String(), node.Text.String())
		c.observeScalar(assessment, failure == nil)
		if failure != nil {
			c.AddIssue(path, failure.Constraint, failure.Message)
		} else {
			return true
		}

	case complex != nil:
		return c.ValidateComplex(node, complex, path, nilled, subject)

	case builtin == "anyType":
		c.opaqueType()
		return true

	case builtin != "":
		if nilled {
			return true
		}
		if len(node.Children) != 0 {
			c.IdentityIncomplete = true
			c.AddIssue(
				path,
				"simple-content",
				"simple value must not contain child elements",
			)
			return false
		}
		if c.local != nil && !c.local.subject.TextKnown {
			return false
		}
		assessment, failure := c.AssessBuiltinValue(builtin, node.Text.String(), node.Text.String())
		c.observeScalar(assessment, failure == nil)
		if failure != nil {
			c.AddIssue(path, failure.Constraint, failure.Message)
		} else {
			return true
		}

	default:
		c.AddIssue(path, "schema", "unsupported type declaration")
	}
	return false
}

func (c *Context) ResolveType(
	reference *TypeRef,
) (
	*SimpleSchema,
	*ComplexSchema,
	string,
	bool,
) {
	if reference == nil {
		return nil, nil, "", false
	}
	if reference.InlineSimple != nil && reference.InlineComplex != nil {
		return nil, nil, "", false
	}
	if reference.InlineSimple != nil {
		return reference.InlineSimple, nil, "", true
	}
	if reference.InlineComplex != nil {
		return nil, reference.InlineComplex, "", true
	}
	if reference.Name.Local == "" {
		return nil, nil, "", false
	}
	if reference.Name.Space == XSDNamespace {
		return nil, nil, reference.Name.Local, BuiltinSemantics(reference.Name.Local) != nil
	}

	resolved, found := c.Schema.Types[reference.Name]
	if !found || resolved == nil {
		return nil, nil, "", false
	}

	return resolved.Simple, resolved.Complex, "", true
}

func (c *Context) ValidateComplex(
	node *Node,
	schema *ComplexSchema,
	path string,
	nilled bool,
	subject *Node,
) bool {
	if schema.Abstract {
		c.AddIssue(path, "abstract", "abstract type cannot be used")
	}
	effective, ok := c.EffectiveComplex(schema)
	if !ok {
		c.AddIssue(path, "schema", "complex type cannot be resolved")
		return false
	}

	c.setFamily("attributes")
	attributesValid := c.ValidateAttributes(
		subject,
		effective.Attributes,
		effective.AnyAttribute,
		path,
	)
	c.setFamily("content")
	if schema.Abstract {
		// Attribute declarations are independently assessable even when their
		// owner's actual type is abstract. Content recovery remains separate.
		c.SkipIdentityChildren(node)
		return false
	}
	if nilled {
		return attributesValid
	}

	if effective.Simple != nil {
		if len(node.Children) != 0 {
			c.IdentityIncomplete = true
			c.AddIssue(
				path,
				"simple-content",
				"simple-content value must not contain child elements",
			)
			return false
		}
		c.setFamily("scalar")
		return c.ValidateType(node, effective.Simple, path, false, subject) && attributesValid
	}

	contentValid := true
	// XSD 1.0 cvc-complex-type 2.1 forbids every character in empty
	// content; 2.3 permits XML S in element-only content. An absent
	// effective particle identifies the empty types represented here, not
	// nullable particles or the generator's unnormalized empty groups.
	// Keep the legacy diagnostic and leave the source text unchanged.
	if (c.local == nil || c.local.subject.TextKnown) && !effective.Mixed && node.Text.Len() != 0 &&
		(effective.Particle == nil || strings.TrimFunc(node.Text.String(), IsWhitespace) != "") {
		c.AddIssue(
			path,
			"element-only",
			"character data is not allowed",
		)
		contentValid = false
	}

	if effective.Particle == nil {
		if len(node.Children) != 0 {
			c.IdentityIncomplete = true
			c.AddIssue(
				ChildPath(path, node.Children[0], 1),
				"content-model",
				fmt.Sprintf(
					"unexpected element %s",
					DisplayName(node.Children[0].Name),
				),
			)
		}
		return attributesValid && contentValid && len(node.Children) == 0
	}

	result, failure, matched := c.MatchParticle(
		effective.Particle,
		node.Children,
		0,
	)
	if c.local != nil && !c.local.subject.ChildrenComplete {
		// A closed prefix may already fully satisfy this grammar. Recover only
		// leaves whose exact declaration cannot change with an unknown suffix;
		// never turn failure of this tentative match into an absence verdict.
		if matched && result.Next == len(node.Children) {
			c.local.matches = c.stablePrefixMatches(effective.Particle, result.Elements)
		}
		return false
	}
	if !matched || result.Next != len(node.Children) {
		c.SkipIdentityChildren(node)
		if !matched {
			c.AddContentIssue(node, path, failure)
			return false
		}

		index := result.Next
		c.AddIssue(
			IndexedChildPath(path, node.Children, index),
			"content-model",
			fmt.Sprintf(
				"unexpected element %s",
				DisplayName(node.Children[index].Name),
			),
		)
		return false
	}

	if c.local != nil {
		c.local.matches = result.Elements
		return attributesValid && contentValid
	}
	paths := ChildPaths(path, node.Children)
	for _, match := range result.Elements {
		if !c.ValidateElementOwned(
			node.Children[match.Index],
			match.Schema,
			paths[match.Index],
			subject,
		) {
			contentValid = false
		}
	}
	return attributesValid && contentValid
}

func (c *Context) EffectiveComplex(
	schema *ComplexSchema,
) (result *EffectiveComplex, ok bool) {
	if schema == nil {
		return nil, false
	}
	if cached, found := c.Effective[schema]; found {
		return cached, cached != nil
	}

	result = &EffectiveComplex{
		Particle:     schema.Particle,
		Attributes:   append([]AttributeSchema(nil), schema.Attributes...),
		AnyAttribute: schema.AnyAttribute,
		Mixed:        schema.Mixed,
	}
	// A nil entry marks pending or failed resolution. Publish only a complete
	// result: repeated or cyclic lookups must never accept a partial type.
	c.Effective[schema] = nil
	defer func() {
		if ok {
			c.Effective[schema] = result
		}
	}()

	switch schema.Form {
	case ComplexDirect:
		return result, true

	case ComplexSimpleContentExtension,
		ComplexSimpleContentRestriction:
		if schema.Base == nil {
			return nil, false
		}

		simple, baseComplex, builtin, found := c.ResolveType(schema.Base)
		if !found {
			return nil, false
		}
		switch {
		case simple != nil || builtin != "":
			result.Simple = schema.Base
		case baseComplex != nil:
			base, ok := c.EffectiveComplex(baseComplex)
			if !ok || base.Simple == nil {
				return nil, false
			}
			result.Simple = base.Simple
			result.Attributes = MergeAttributes(
				base.Attributes,
				result.Attributes,
			)
			if schema.Form == ComplexSimpleContentExtension {
				result.AnyAttribute = MergeAnyAttribute(base.AnyAttribute, result.AnyAttribute)
			}
			result.Mixed = result.Mixed || base.Mixed
		default:
			return nil, false
		}

		if schema.Form == ComplexSimpleContentRestriction && schema.SimpleContent != nil {
			local := *schema.SimpleContent
			if local.Base == nil {
				local.Base = &SimpleMember{
					Name:   result.Simple.Name,
					Inline: result.Simple.InlineSimple,
				}
			}
			result.Simple = &TypeRef{InlineSimple: &local}
		}

		return result, true

	case ComplexComplexContentExtension:
		if schema.Base == nil {
			return nil, false
		}
		_, baseComplex, _, found := c.ResolveType(schema.Base)
		if !found || baseComplex == nil {
			return nil, false
		}
		base, ok := c.EffectiveComplex(baseComplex)
		if !ok || base.Simple != nil {
			return nil, false
		}

		result.Particle = Sequence(
			base.Particle,
			result.Particle,
		)
		result.Attributes = MergeAttributes(
			base.Attributes,
			result.Attributes,
		)
		result.AnyAttribute = MergeAnyAttribute(
			base.AnyAttribute,
			result.AnyAttribute,
		)
		result.Mixed = result.Mixed || base.Mixed

		return result, true

	case ComplexComplexContentRestriction:
		_, baseComplex, builtin, found := c.ResolveType(schema.Base)
		if !found {
			return nil, false
		}
		if builtin == "anyType" {
			return result, true
		}
		if baseComplex == nil {
			return nil, false
		}
		base, ok := c.EffectiveComplex(baseComplex)
		if !ok {
			return nil, false
		}
		// Attribute uses omitted by restriction are inherited; explicit local
		// uses (including prohibited ones) replace the corresponding base uses.
		result.Attributes = MergeAttributes(base.Attributes, result.Attributes)
		// Content and wildcard constraints belong to the restriction itself.
		return result, true

	default:
		return nil, false
	}
}

func MergeAttributes(
	base []AttributeSchema,
	derived []AttributeSchema,
) []AttributeSchema {
	result := append([]AttributeSchema(nil), base...)
	indexes := make(map[QName]int, len(result))
	for index := range result {
		indexes[result[index].Name] = index
	}

	for _, attribute := range derived {
		if index, found := indexes[attribute.Name]; found {
			result[index] = attribute
			continue
		}
		indexes[attribute.Name] = len(result)
		result = append(result, attribute)
	}

	return result
}

func MergeAnyAttribute(
	base *AnyAttributeSchema,
	derived *AnyAttributeSchema,
) *AnyAttributeSchema {
	if derived != nil {
		return derived
	}
	return base
}

func Sequence(
	left *ParticleSchema,
	right *ParticleSchema,
) *ParticleSchema {
	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	default:
		return &ParticleSchema{
			Kind: ParticleSequence,
			Occurrence: Occurrence{
				Min: 1,
				Max: 1,
			},
			Children: []*ParticleSchema{left, right},
		}
	}
}

// validationStandardXSIName recognizes standard schema-instance names permitted
// by cvc-type and cvc-complex-type. validateElement checks type, nil and hint
// values; hints do not select or fetch schemas.
func StandardXSIName(name QName, allowType bool) bool {
	if name.Space != XSINamespace {
		return false
	}
	switch name.Local {
	case "nil", "schemaLocation", "noNamespaceSchemaLocation":
		return true
	case "type":
		return allowType
	}
	return false
}

// validateSimpleElementAttributes enforces XSD 1.0 cvc-type 3.1.1. The four
// standard xsi names are permitted here; validateElement separately checks
// xsi:nil, xsi:type and hint datatypes.
// https://www.w3.org/TR/xmlschema-1/#cvc-type
func (c *Context) ValidateSimpleElementAttributes(node *Node, path string) bool {
	valid := true
	for _, attribute := range node.Attrs {
		if NamespaceDeclaration(attribute) {
			continue
		}
		if StandardXSIName(attribute.Name, true) {
			continue
		}
		valid = false
		c.AddIssue(
			AttributePath(path, attribute.Name),
			"attribute",
			"attribute is not allowed",
		)
	}
	return valid
}

func (c *Context) ValidateAttributes(
	node *Node,
	schemas []AttributeSchema,
	anyAttribute *AnyAttributeSchema,
	path string,
) bool {
	valid := true
	known := make(map[QName]AttributeSchema, len(schemas))
	present := make(map[QName]Attr, len(node.Attrs))
	for _, schema := range schemas {
		known[schema.Name] = schema
	}

	for _, attribute := range node.Attrs {
		if NamespaceDeclaration(attribute) {
			continue
		}
		if StandardXSIName(attribute.Name, true) {
			continue
		}

		if c.local != nil {
			c.local.attribute = attribute.Name
			c.local.attributeEpoch = c.AssessmentVersion
			c.local.attributeKnown = attribute.ValueKnown
			c.local.scalarAssessed = false
		}
		name := Name(attribute.Name)
		schema, found := known[name]
		if !found {
			if AnyAttributeAllows(anyAttribute, attribute.Name) {
				if anyAttribute != nil {
					c.wildcard(anyAttribute.ProcessContents, "attributes")
				}
				continue
			}
			valid = false
			c.AddIssue(
				AttributePath(path, attribute.Name),
				"attribute",
				"attribute is not allowed",
			)
			continue
		}
		present[name] = attribute.Attr

		if schema.Use == AttributeProhibited {
			valid = false
			c.AddIssue(
				AttributePath(path, attribute.Name),
				"prohibited",
				"attribute is prohibited",
			)
			continue
		}

		if c.local != nil && !attribute.ValueKnown {
			c.local.limit("attributes", "attribute-value-unavailable", nil)
			c.IdentityIncomplete = true
			c.attributeResult(schema, attribute.Value, true, false, false)
			valid = false
			continue
		}
		attributeValid := true
		beforeFixed := c.AssessmentVersion
		if schema.Constraint != nil &&
			schema.Constraint.Kind == "fixed" &&
			!c.SimpleValuesEqual(
				&schema.Type,
				attribute.Value,
				schema.Constraint.Value,
			) && c.AssessmentVersion == beforeFixed {
			attributeValid = false
			valid = false
			c.AddIssue(
				AttributePath(path, attribute.Name),
				"fixed",
				fmt.Sprintf(
					"value %q does not equal fixed value %q",
					attribute.Value,
					schema.Constraint.Value,
				),
			)
		}

		if c.AssessmentVersion != beforeFixed {
			attributeValid = false
			valid = false
		}
		if failure := c.ValidateSimpleType(
			&schema.Type,
			attribute.Value,
		); failure != nil {
			valid = false
			c.AddIssue(
				AttributePath(path, attribute.Name),
				failure.Constraint,
				failure.Message,
			)
		} else {
			attributePath := AttributePath(path, attribute.Name)
			if attributeValid {
				c.RecordIdentity(&schema.Type, attribute.Value, attributePath)
			}
			c.RecordIdentityBinding(&schema.Type, attribute.Value, attributePath, node)
		}
		c.attributeResult(schema, attribute.Value, true, c.local == nil || c.local.scalarAssessed, attributeValid && (c.local == nil || c.local.scalarAssessed))
	}

	if c.local != nil {
		c.local.attribute = QName{}
	}
	for _, schema := range schemas {
		if schema.Use != AttributeRequired {
			continue
		}
		if _, found := present[schema.Name]; found {
			continue
		}
		if c.local != nil && !c.local.subject.AttributesComplete {
			valid = false
			continue
		}

		valid = false
		c.AddIssue(
			AttributeQNamePath(path, schema.Name),
			"required",
			"required attribute is missing",
		)
	}
	c.absentAttributeFacts(schemas, present)
	return valid
}

func (c *Context) ValidateSimpleType(
	reference *TypeRef,
	value string,
) *SimpleFailure {
	simple, complex, builtin, found := c.ResolveType(reference)
	if !found || complex != nil {
		return c.IncompleteFailure("simple type cannot be resolved")
	}
	if !c.ValidTypeMetadata(TypeIdentity{Simple: simple, Builtin: builtin}) {
		return c.AssessmentFailure("simple type metadata incomplete")
	}
	if simple != nil {
		assessment, failure := c.AssessSimpleValue(simple, value, value)
		c.observeScalar(assessment, failure == nil)
		return failure
	}
	assessment, failure := c.AssessBuiltinValue(builtin, value, value)
	c.observeScalar(assessment, failure == nil)
	return failure
}

func (c *Context) SimpleValuesEqual(
	reference *TypeRef,
	left string,
	right string,
) bool {
	semantics := c.SimpleTypeSemantics(reference)
	if semantics == nil {
		// Deliberately fail closed, even for identical spellings. Ordinary
		// value validation reports the schema failure; the additional fixed
		// issue is retained until diagnostic deduplication is addressed.
		return false
	}
	if semantics.Aggregate() || semantics.Builtin == "boolean" || semantics.Builtin == "anyURI" {
		return c.AggregateValuesEqual(semantics, left, right)
	}
	builtin := semantics.Builtin
	if c.local != nil && !semantics.Aggregate() && !supportedEquality(AtomicValue(semantics, semantics.Normalize(left))) {
		c.Incomplete("value-space equality is unavailable for xs:" + builtin)
		return false
	}
	if builtin == "ENTITY" {
		return c.AggregateValuesEqual(semantics, left, right)
	}
	if _, _, integer := IntegerLimits(builtin); integer {
		return IntegerValuesEqual(left, right)
	}
	if builtin == "decimal" {
		return DecimalValuesEqual(left, right)
	}
	if semantics.StringValue {
		return semantics.Normalize(left) == semantics.Normalize(right)
	}
	// Preserve existing lexical comparison for other primitive families.
	if builtin == "" {
		builtin = "string"
	}
	return NormalizeWhitespace(builtin, left) == NormalizeWhitespace(builtin, right)
}

// Atomic integer/decimal/string/boolean/URI and supported list/union values
// use their value contracts. Other primitives retain their lexical path;
// mixed content compares raw text.
func (c *Context) ElementFixedValuesEqual(reference *TypeRef, left, right string) bool {
	_, complex, _, _ := c.ResolveType(reference)
	if complex != nil {
		effective, ok := c.EffectiveComplex(complex)
		if !ok {
			c.Incomplete("fixed-value complex content metadata cannot be resolved")
			return false
		}
		if effective.Simple == nil {
			return left == right
		}
		reference = effective.Simple
	}
	semantics := c.SimpleTypeSemantics(reference)
	if semantics == nil {
		// As in simpleValuesEqual, retain fail-closed comparison and its
		// additional fixed issue alongside ordinary schema diagnostics.
		return false
	}
	if c.local != nil && !semantics.Aggregate() && !supportedEquality(AtomicValue(semantics, semantics.Normalize(left))) {
		c.Incomplete("value-space equality is unavailable for xs:" + semantics.Builtin)
		return false
	}
	if semantics.Aggregate() || semantics.Builtin == "boolean" || semantics.Builtin == "anyURI" {
		return c.AggregateValuesEqual(semantics, left, right)
	}
	if semantics.Builtin == "ENTITY" {
		return c.AggregateValuesEqual(semantics, left, right)
	}
	if _, _, integer := IntegerLimits(semantics.Builtin); integer {
		return IntegerValuesEqual(left, right)
	}
	if semantics.Builtin == "decimal" {
		return DecimalValuesEqual(left, right)
	}
	if semantics.StringValue {
		return semantics.Normalize(left) == semantics.Normalize(right)
	}
	return left == right
}

func (c *Context) ValidateSimple(
	schema *SimpleSchema,
	value string,
) *SimpleFailure {
	_, failure := c.ValidateSimpleValue(schema, value, value)
	return failure
}

// Keep the lexical-returning helper for callers that do not compare values.
func (c *Context) ValidateSimpleValue(schema *SimpleSchema, value, display string) (string, *SimpleFailure) {
	assessment, failure := c.AssessSimpleValue(schema, value, display)
	return assessment.Lexical, failure
}

// Return both the normalized lexical view and its actual value. The most
// derived atomic/list policy runs before inherited lexical facets. A union
// instead supplies each member the same input and returns the first success's
// view; a later OUTER facet failure never restarts union member selection.
// Display is kept separately so inherited diagnostics retain source spelling.
func (c *Context) AssessSimpleValue(
	schema *SimpleSchema,
	value, display string,
) (result SimpleAssessment, failure *SimpleFailure) {
	defer func() {
		if failure != nil && failure.Constraint == "schema" {
			c.Incomplete(failure.Message)
		}
	}()
	if !c.ValidTypeMetadata(TypeIdentity{Simple: schema}) {
		return SimpleAssessment{}, c.AssessmentFailure("simple derivation metadata incomplete")
	}
	semantics := c.ResolveSimpleSemantics(schema)
	if semantics == nil {
		return SimpleAssessment{}, &SimpleFailure{Constraint: "schema", Message: "simple type cannot be resolved"}
	}
	value = semantics.Normalize(value)
	switch schema.Form {
	case SimpleRestriction:
		assessment, failure := c.AssessSimpleMemberValue(schema.Base, value, display)
		if failure != nil {
			return SimpleAssessment{}, failure
		}
		normalized := assessment.Lexical
		builtin := semantics.Builtin
		beforeEnumeration := c.AssessmentVersion
		enumerationMatch := len(schema.Enumerations) == 0 || c.MatchesSimpleEnumeration(schema, assessment)
		if c.AssessmentVersion != beforeEnumeration {
			return SimpleAssessment{}, c.AssessmentFailure("enumeration literal assessment incomplete")
		}
		if !enumerationMatch {
			return SimpleAssessment{}, &SimpleFailure{
				Constraint: "enumeration",
				Message: fmt.Sprintf(
					"value %q is not one of the allowed values",
					display,
				),
			}
		}

		// Patterns in one restriction are alternatives; the base assessment above
		// preserves intersection across restriction layers. Check every alternative
		// so an earlier match or miss cannot conceal an invalid generated pattern.
		patternsMatched := len(schema.Patterns) == 0
		for _, pattern := range schema.Patterns {
			matched, err := MatchPattern(pattern, normalized)
			if err != nil {
				return SimpleAssessment{}, &SimpleFailure{
					Constraint: "schema",
					Message: fmt.Sprintf(
						"invalid generated XSD pattern %q: %v",
						pattern,
						err,
					),
				}
			}
			patternsMatched = patternsMatched || matched
		}
		if !patternsMatched {
			var message string
			if len(schema.Patterns) == 1 {
				message = fmt.Sprintf("value %q does not match XSD pattern %q", display, schema.Patterns[0])
			} else {
				message = fmt.Sprintf("value %q does not match any XSD pattern in %q", display, schema.Patterns)
			}
			return SimpleAssessment{}, &SimpleFailure{Constraint: "pattern", Message: message}
		}

		if failure := ValidateBoundsValue(schema, normalized, builtin, display); failure != nil {
			return SimpleAssessment{}, failure
		}
		if failure := ValidateLengthFacets(
			schema,
			normalized,
			c.SimpleMemberList(schema.Base),
		); failure != nil {
			return SimpleAssessment{}, failure
		}
		if failure := ValidateDigitFacets(schema, normalized, builtin); failure != nil {
			return SimpleAssessment{}, failure
		}

		return assessment, nil

	case SimpleUnion:
		for _, member := range schema.Members {
			normalized, failure := c.AssessUnionMemberValue(member, value, display)
			if failure == nil {
				return normalized, nil
			}
			if TechnicalFailure(failure) {
				return SimpleAssessment{}, failure
			}
		}
		return SimpleAssessment{}, &SimpleFailure{
			Constraint: "union",
			Message:    fmt.Sprintf("value %q does not match any union member", display),
		}

	case SimpleList:
		result := SimpleAssessment{Lexical: value, Value: SimpleValue{Domain: ListDomain}}
		for index, item := range SplitList(value) {
			assessment, failure := c.AssessSimpleMemberValue(schema.Item, item, item)
			if failure != nil {
				return SimpleAssessment{}, &SimpleFailure{Constraint: failure.Constraint, Message: fmt.Sprintf("list item %d: %s", index+1, failure.Message)}
			}
			result.Value.Items = append(result.Value.Items, assessment.Value)
		}
		return result, nil

	default:
		return SimpleAssessment{}, &SimpleFailure{Constraint: "schema", Message: "unsupported simple type form"}
	}
}

// XSD 1.0 section 4.1.2.3 replaces union-variety member definitions with
// their own ordered member definitions. Only this union-composition boundary
// skips restriction wrappers: direct union restrictions, list item types and
// atomic/list members still take their ordinary full assessment paths.
func (c *Context) AssessUnionMemberValue(member *SimpleMember, value, display string) (SimpleAssessment, *SimpleFailure) {
	semantics := c.MemberSemantics(member)
	if semantics != nil && semantics.Union != nil {
		return c.AssessSimpleValue(semantics.Union, value, display)
	}
	return c.AssessSimpleMemberValue(member, value, display)
}

func (c *Context) AssessSimpleMemberValue(
	member *SimpleMember,
	value, display string,
) (SimpleAssessment, *SimpleFailure) {
	semantics := c.MemberSemantics(member)
	if semantics == nil {
		return SimpleAssessment{}, c.IncompleteFailure("simple type member cannot be resolved")
	}
	if member.Inline != nil {
		return c.AssessSimpleValue(member.Inline, value, display)
	}
	if member.Name.Space == XSDNamespace {
		return c.AssessBuiltinValue(member.Name.Local, value, display)
	}
	return c.AssessSimpleValue(c.Schema.Types[member.Name].Simple, value, display)
}

func (c *Context) SimpleBuiltin(
	reference *TypeRef,
) string {
	if reference == nil {
		return ""
	}
	if reference.InlineSimple != nil {
		return c.SimpleSchemaBuiltin(reference.InlineSimple)
	}
	if reference.Name.Space == XSDNamespace {
		return reference.Name.Local
	}

	resolved, found := c.Schema.Types[reference.Name]
	if !found || resolved == nil || resolved.Simple == nil {
		return ""
	}
	return c.SimpleSchemaBuiltin(resolved.Simple)
}

func (c *Context) SimpleSchemaBuiltin(
	schema *SimpleSchema,
) string {
	if schema == nil {
		return ""
	}

	switch schema.Form {
	case SimpleRestriction:
		return c.SimpleMemberBuiltin(schema.Base)
	case SimpleUnion, SimpleList:
		return "string"
	default:
		return ""
	}
}

func (c *Context) SimpleMemberBuiltin(
	member *SimpleMember,
) string {
	if member == nil {
		return ""
	}
	if member.Inline != nil {
		return c.SimpleSchemaBuiltin(member.Inline)
	}
	if member.Name.Space == XSDNamespace {
		return member.Name.Local
	}

	resolved, found := c.Schema.Types[member.Name]
	if !found || resolved == nil || resolved.Simple == nil {
		return ""
	}
	return c.SimpleSchemaBuiltin(resolved.Simple)
}

func (c *Context) SimpleMemberList(
	member *SimpleMember,
) bool {
	if member == nil {
		return false
	}
	if member.Inline != nil {
		return c.SimpleSchemaList(member.Inline)
	}
	if member.Name.Space == XSDNamespace {
		// Built-in list varieties retain item-count facets through restrictions
		// and the effective complex simple-content scalar bridge.
		return BuiltinListItem(member.Name.Local) != ""
	}

	resolved, found := c.Schema.Types[member.Name]
	return found &&
		resolved != nil &&
		resolved.Simple != nil &&
		c.SimpleSchemaList(resolved.Simple)
}

func (c *Context) SimpleSchemaList(
	schema *SimpleSchema,
) bool {
	if schema == nil {
		return false
	}
	switch schema.Form {
	case SimpleList:
		return true
	case SimpleRestriction:
		return c.SimpleMemberList(schema.Base)
	default:
		return false
	}
}

func ValidateBuiltin(
	name string,
	value string,
) *SimpleFailure {
	return ValidateBuiltinValue(name, value, value)
}

func ValidateBuiltinValue(name, value, display string) *SimpleFailure {
	if name == "boolean" {
		if _, failure := ParseBoolean(value); failure != nil {
			return &SimpleFailure{Constraint: "datatype", Message: fmt.Sprintf("value %q is not valid for xs:boolean", display)}
		}
		return nil
	}
	normalized := NormalizeWhitespace(name, value)

	switch name {
	case "anySimpleType", "string", "normalizedString", "token":
		return nil

	case "anyURI":
		if ValidXSDAnyURI(normalized) {
			return nil
		}

	case "language":
		if LanguagePattern.MatchString(normalized) {
			return nil
		}

	case "Name":
		if ValidXSDName(normalized) {
			return nil
		}

	case "NCName", "ID", "IDREF", "ENTITY":
		if ValidXSDNCName(normalized) {
			return nil
		}

	case "NMTOKEN":
		if ValidXSDNMTOKEN(normalized) {
			return nil
		}

	case "NMTOKENS":
		items := SplitList(normalized)
		if len(items) != 0 &&
			slices.ContainsFunc(items, func(item string) bool {
				return !ValidXSDNMTOKEN(item)
			}) == false {
			return nil
		}

	case "IDREFS", "ENTITIES":
		items := SplitList(normalized)
		if len(items) != 0 &&
			slices.ContainsFunc(items, func(item string) bool {
				return !ValidXSDNCName(item)
			}) == false {
			return nil
		}

	case "decimal":
		if DecimalPattern.MatchString(normalized) {
			return nil
		}

	case "float", "double":
		if normalized == "INF" ||
			normalized == "-INF" ||
			normalized == "NaN" {
			return nil
		}
		if _, err := strconv.ParseFloat(normalized, 64); err == nil {
			return nil
		}

	case "integer", "long", "int", "short", "byte", "nonPositiveInteger",
		"negativeInteger", "nonNegativeInteger", "positiveInteger", "unsignedLong",
		"unsignedInt", "unsignedShort", "unsignedByte":
		if ValidInteger(name, normalized) {
			return nil
		}

	case "base64Binary":
		if Base64Pattern.MatchString(normalized) {
			return nil
		}

	case "hexBinary":
		if len(normalized)%2 == 0 &&
			HexPattern.MatchString(normalized) {
			return nil
		}

	case "QName", "NOTATION":
		if ValidXSDQName(normalized) {
			return nil
		}

	case "date", "dateTime", "duration", "gDay", "gMonth",
		"gMonthDay", "gYear", "gYearMonth", "time":
		if ValidXSDDateTime(name, normalized) {
			return nil
		}

	case "anyType":
		return nil

	default:
		return &SimpleFailure{
			Constraint: "schema",
			Message:    fmt.Sprintf("unsupported XSD built-in type %q", name),
		}
	}

	return &SimpleFailure{
		Constraint: "datatype",
		Message: fmt.Sprintf(
			"value %q is not valid for xs:%s",
			display,
			name,
		),
	}
}

// parseValidationBoolean shares XSD whitespace and lexical handling between
// ordinary boolean values and the xsi:nil element constraint.
func ParseBoolean(value string) (bool, *SimpleFailure) {
	switch NormalizeWhitespace("boolean", value) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, &SimpleFailure{
			Constraint: "datatype",
			Message:    fmt.Sprintf("value %q is not valid for xs:boolean", value),
		}
	}
}

// Reuse the concurrency-safe replacer across builtin checks and resolved
// replace-policy views instead of rebuilding it for every value or literal.
var ReplaceWhitespace = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func NormalizeWhitespace(name string, value string) string {
	switch name {
	case "string", "anySimpleType":
		return value
	case "normalizedString":
		return ReplaceWhitespace.Replace(value)
	default:
		return CollapseWhitespace(value)
	}
}

func CollapseWhitespace(value string) string {
	var result strings.Builder
	result.Grow(len(value))
	wroteValue := false
	pendingSpace := false

	for _, character := range value {
		if IsWhitespace(character) {
			pendingSpace = wroteValue
			continue
		}
		if pendingSpace {
			result.WriteByte(' ')
			pendingSpace = false
		}
		result.WriteRune(character)
		wroteValue = true
	}

	return result.String()
}

func SplitList(value string) []string {
	normalized := CollapseWhitespace(value)
	if normalized == "" {
		return nil
	}

	return strings.Split(normalized, " ")
}

func IsWhitespace(value rune) bool {
	return strings.ContainsRune(XMLWhitespace, value)
}

func ValidateBounds(
	schema *SimpleSchema,
	value string,
	builtin string,
) *SimpleFailure {
	return ValidateBoundsValue(schema, value, builtin, value)
}

func ValidateBoundsValue(schema *SimpleSchema, value, builtin, display string) *SimpleFailure {
	if !schema.HasMinInclusive &&
		!schema.HasMaxInclusive &&
		!schema.HasMinExclusive &&
		!schema.HasMaxExclusive {
		return nil
	}

	if _, _, integer := IntegerLimits(builtin); integer {
		return ValidateIntegerBounds(schema, value, display)
	}
	if builtin == "decimal" {
		return ValidateDecimalBounds(schema, value, display)
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) {
		return &SimpleFailure{
			Constraint: "datatype",
			Message:    fmt.Sprintf("value %q is not numeric", display),
		}
	}

	tests := []struct {
		Enabled    bool
		Constraint string
		Limit      string
		Valid      func(float64, float64) bool
	}{
		{schema.HasMinInclusive, "minInclusive", schema.MinInclusive, func(value, limit float64) bool {
			return value >= limit
		}},
		{schema.HasMaxInclusive, "maxInclusive", schema.MaxInclusive, func(value, limit float64) bool {
			return value <= limit
		}},
		{schema.HasMinExclusive, "minExclusive", schema.MinExclusive, func(value, limit float64) bool {
			return value > limit
		}},
		{schema.HasMaxExclusive, "maxExclusive", schema.MaxExclusive, func(value, limit float64) bool {
			return value < limit
		}},
	}
	for _, test := range tests {
		if !test.Enabled {
			continue
		}
		limit, err := strconv.ParseFloat(test.Limit, 64)
		if err != nil {
			return &SimpleFailure{
				Constraint: "schema",
				Message: fmt.Sprintf(
					"invalid %s value %q",
					test.Constraint,
					test.Limit,
				),
			}
		}
		if !test.Valid(number, limit) {
			return &SimpleFailure{
				Constraint: test.Constraint,
				Message: fmt.Sprintf(
					"value %q violates %s=%q",
					display,
					test.Constraint,
					test.Limit,
				),
			}
		}
	}

	return nil
}

func ValidateLengthFacets(
	schema *SimpleSchema,
	value string,
	list bool,
) *SimpleFailure {
	if !schema.HasLength &&
		!schema.HasMinLength &&
		!schema.HasMaxLength {
		return nil
	}

	length := uint64(utf8.RuneCountInString(value))
	if list {
		length = uint64(len(SplitList(value)))
	}

	switch {
	case schema.HasLength && length != schema.Length:
		return &SimpleFailure{
			Constraint: "length",
			Message: fmt.Sprintf(
				"value length %d does not equal %d",
				length,
				schema.Length,
			),
		}
	case schema.HasMinLength && length < schema.MinLength:
		return &SimpleFailure{
			Constraint: "minLength",
			Message: fmt.Sprintf(
				"value length %d is less than %d",
				length,
				schema.MinLength,
			),
		}
	case schema.HasMaxLength && length > schema.MaxLength:
		return &SimpleFailure{
			Constraint: "maxLength",
			Message: fmt.Sprintf(
				"value length %d is greater than %d",
				length,
				schema.MaxLength,
			),
		}
	default:
		return nil
	}
}

func ValidateDigitFacets(
	schema *SimpleSchema,
	value string,
	builtin string,
) *SimpleFailure {
	if !schema.HasTotalDigits && !schema.HasFractionDigits {
		return nil
	}

	var total, fraction uint64
	if builtin == "decimal" {
		number, ok := ParseDecimal(value)
		if !ok {
			return &SimpleFailure{Constraint: "datatype", Message: fmt.Sprintf("value %q is not numeric", value)}
		}
		// XSD 1.0 totalDigits bounds both coefficient magnitude and scale:
		// https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-totalDigits
		// Keep leading fractional zeros (.001 needs totalDigits >= 3), but the
		// immutable views omit leading whole and trailing fractional zeros.
		fraction = uint64(len(number.Fraction))
		total = max(1, uint64(len(number.Whole))+fraction)
	} else {
		// Integer digit facets count the value's significant digits; the source
		// spelling is retained separately for lexical patterns and diagnostics.
		if _, _, integer := IntegerLimits(builtin); integer {
			if number, ok := ParseInteger(value); ok {
				value = number.Digits
			}
		}
		normalized := strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
		parts := strings.SplitN(normalized, ".", 2)
		for _, character := range normalized {
			if unicode.IsDigit(character) {
				total++
			}
		}
		if len(parts) == 2 {
			fraction = uint64(utf8.RuneCountInString(parts[1]))
		}
	}

	if schema.HasTotalDigits && total > schema.TotalDigits {
		return &SimpleFailure{
			Constraint: "totalDigits",
			Message: fmt.Sprintf(
				"value has %d digits; maximum is %d",
				total,
				schema.TotalDigits,
			),
		}
	}
	if schema.HasFractionDigits && fraction > schema.FractionDigits {
		return &SimpleFailure{
			Constraint: "fractionDigits",
			Message: fmt.Sprintf(
				"value has %d fraction digits; maximum is %d",
				fraction,
				schema.FractionDigits,
			),
		}
	}

	return nil
}

func MatchPattern(pattern string, value string) (bool, error) {
	translated := strings.ReplaceAll(
		pattern,
		`\c`,
		`[`+XMLNameCharacters+`]`,
	)
	translated = strings.ReplaceAll(
		translated,
		`\i`,
		`[`+XMLNameStartCharacters+`]`,
	)

	// XSD \d denotes Unicode decimal digits, including inside character classes.
	translated = strings.ReplaceAll(translated, `\d`, `\p{Nd}`)

	compiled, err := regexp.Compile(`^(?:` + translated + `)$`)
	if err != nil {
		return false, err
	}

	return compiled.MatchString(value), nil
}

func BetterFailure(
	left MatchFailure,
	right MatchFailure,
) MatchFailure {
	switch {
	case right.Position > left.Position:
		return right
	case right.Position < left.Position:
		return left
	default:
		left.Expected = UniqueNames(
			slices.Concat(left.Expected, right.Expected),
		)
		return left
	}
}

func UniqueNames(
	values []QName,
) []QName {
	result := make([]QName, 0, len(values))
	seen := make(map[QName]struct{}, len(values))
	for _, value := range values {
		if value.Local == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	slices.SortFunc(result, func(left, right QName) int {
		if comparison := strings.Compare(left.Space, right.Space); comparison != 0 {
			return comparison
		}
		return strings.Compare(left.Local, right.Local)
	})
	return result
}

func (c *Context) AddContentIssue(
	node *Node,
	path string,
	failure MatchFailure,
) {
	if failure.Schema != "" {
		c.AddIssue(path, "schema", failure.Schema)
		return
	}
	position := min(failure.Position, len(node.Children))
	expected := ExpectedNames(failure.Expected)

	if position == len(node.Children) {
		message := "required child element is missing"
		if expected != "" {
			message += "; expected " + expected
		}
		c.AddIssue(path, "content-model", message)
		return
	}

	message := fmt.Sprintf(
		"unexpected element %s",
		DisplayName(node.Children[position].Name),
	)
	if expected != "" {
		message += "; expected " + expected
	}
	c.AddIssue(
		IndexedChildPath(path, node.Children, position),
		"content-model",
		message,
	)
}

func ExpectedNames(names []QName) string {
	if len(names) == 0 {
		return ""
	}
	values := make([]string, len(names))
	for index, name := range names {
		values[index] = DisplayQName(name)
	}
	return strings.Join(values, " or ")
}

func (c *Context) AddIssue(
	path string,
	constraint string,
	message string,
) {
	if constraint == "schema" || constraint == "assessment" {
		if constraint == "schema" || c.AssessmentErr == nil {
			c.Incomplete(path + ": " + message)
		}
		return
	}
	if c.local != nil {
		c.local.detail(path, constraint, message)
		return
	}
	if c.run != nil && c.run.identityResults != nil {
		c.identityDetail(path, constraint, message)
		return
	}
	c.Issues = append(c.Issues, Issue{
		Path:       path,
		Constraint: constraint,
		Message:    message,
	})
}

func AnyAttributeAllows(
	schema *AnyAttributeSchema,
	name QName,
) bool {
	if schema == nil {
		return false
	}
	switch schema.Namespace {
	case "", "##any":
		return true
	case "##other":
		return name.Space != ""
	case "##local":
		return name.Space == ""
	default:
		for _, namespace := range strings.Fields(schema.Namespace) {
			if namespace == name.Space {
				return true
			}
		}
		return false
	}
}

func NilAttribute(
	attributes []Attribute,
) (string, bool) {
	for _, attribute := range attributes {
		if attribute.Name.Space != XSINamespace ||
			attribute.Name.Local != "nil" {
			continue
		}

		return attribute.Value, true
	}
	return "", false
}

func NamespaceDeclaration(attribute Attribute) bool {
	return attribute.NamespaceDeclaration
}

func Name(name QName) QName {
	return QName{
		Space: name.Space,
		Local: name.Local,
	}
}

func String(value string) *string {
	return &value
}

func DisplayName(name QName) string {
	return DisplayQName(Name(name))
}

func DisplayQName(name QName) string {
	if name.Space == "" {
		return "<" + name.Local + ">"
	}
	return fmt.Sprintf("<{%s}%s>", name.Space, name.Local)
}

func Path(parent string, local string, index int) string {
	if parent == "" {
		return "/" + local
	}
	if index <= 1 {
		return parent + "/" + local
	}
	return fmt.Sprintf("%s/%s[%d]", parent, local, index)
}

func ChildPath(
	parent string,
	child *Node,
	index int,
) string {
	return Path(parent, child.Name.Local, index)
}

func ChildPaths(
	parent string,
	children []*Node,
) []string {
	result := make([]string, len(children))
	counts := make(map[QName]int)
	for index, child := range children {
		counts[child.Name]++
		result[index] = ChildPath(
			parent,
			child,
			counts[child.Name],
		)
	}
	return result
}

func IndexedChildPath(
	parent string,
	children []*Node,
	index int,
) string {
	if index < 0 || index >= len(children) {
		return parent
	}
	count := 0
	for current := 0; current <= index; current++ {
		if children[current].Name == children[index].Name {
			count++
		}
	}
	return ChildPath(parent, children[index], count)
}

func AttributePath(parent string, name QName) string {
	if name.Space == "" {
		return parent + "/@" + name.Local
	}
	return fmt.Sprintf("%s/@{%s}%s", parent, name.Space, name.Local)
}

func AttributeQNamePath(
	parent string,
	name QName,
) string {
	return AttributePath(parent, QName{
		Space: name.Space,
		Local: name.Local,
	})
}

var (
	LanguagePattern = regexp.MustCompile(
		`^[A-Za-z]{1,8}(?:-[A-Za-z0-9]{1,8})*$`,
	)
	DecimalPattern = regexp.MustCompile(
		`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`,
	)
	Base64Pattern = regexp.MustCompile(
		`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`,
	)
	HexPattern = regexp.MustCompile(`^[0-9A-Fa-f]*$`)
)
