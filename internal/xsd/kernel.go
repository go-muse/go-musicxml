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
	"unsafe"
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
	Current CurrentScalar
	Attr
	NamespaceDeclaration bool
	ValueKnown           bool
}

type Context struct {
	sourceDeclarations      bool
	entityDeclarations      *EntityDeclarations
	entityLiteralDepth      int
	entityDeferred          int
	entityConstructedDepth  int
	entityIssueSequence     uint64
	currentTrials           int
	currentFixedUnavailable bool
	identitySource          identityOrigin
	identityDiagnostic      identityDiagnostic
	run                     *Run
	local                   *boundState
	AssessmentErr           error
	AssessmentVersion       uint64
	TypeMetadata            map[TypeIdentity]uint8
	Derivations             map[DerivationKey]bool
	Schema                  *SchemaData
	Issues                  []Issue
	Effective               map[*ComplexSchema]*EffectiveComplex
	ParticleNullable        map[*ParticleSchema]bool // prepared immutable metadata, scoped to this validation
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
	// Run-local immutable absence records, indexed by the effective schema.
	absent       absentAttributeCache
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
	c.step(1)

	c.ValidateElementOwned(node, schema, path, nil)
}

// owner is the original containing element, never an effective default-value view.
// The returned declaration-local success is distinct from type success and from
// independently assessed standard xsi attributes. Neither gate uses document-wide
// issue state: a parent/sibling failure cannot erase this item's binding facts.
func (c *Context) ValidateElementOwned(node *Node, schema *ElementSchema, path string, owner *Node) bool {
	c.step(1)

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
			format(c,
				"got %s, expected %s",
				displayName(node.Name, c),
				displayQName(schema.Name, c),
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
		if c.currentModel() && !c.local.subject.Current.Known && !c.local.subject.Current.Absent && (c.currentIdentityCapable(&schema.Type) || c.currentIdentityCapable(actualType)) {
			c.IdentityIncomplete = true
		}
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
	nilValue, nilPresent := nilAttribute(node.Attrs, c)
	nilled := false
	if nilPresent {
		nilPath := attributePath(path, QName{Space: XSINamespace, Local: "nil"}, c)
		if value, failure := parseBoolean(nilValue, c); failure != nil {
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
		characters := node.Text.Len() != 0
		if c.currentModel() {
			characters, _ = c.currentCharacters()
		}
		if len(node.Children) != 0 || characters {
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
	if c.currentModel() && !nilled && (schema.Default != nil || schema.Fixed != nil) {
		characters, _ := c.currentCharacters()
		if !characters && len(node.Children) == 0 {
			constraintBlocked = true
			c.capability("declaration", "model-element-value-constraint-unavailable")
			c.capability("scalar", "model-element-value-constraint-unavailable")
		}
	}
	if c.local != nil && !c.local.subject.ChildrenComplete {
		if nilled || schema.Fixed != nil {
			c.capability("declaration", "element-child-absence-unavailable")
		}
		if !c.currentModel() && !nilled && len(node.Children) == 0 && node.Text.Len() == 0 && (schema.Default != nil || schema.Fixed != nil) {
			constraintBlocked = true
			c.capability("declaration", "element-value-constraint-emptiness-unavailable")
			c.capability("scalar", "element-value-constraint-emptiness-unavailable")
			c.IdentityIncomplete = true
		}
	}
	if !c.currentModel() && !nilled && !constraintBlocked && (c.local == nil || c.local.subject.TextKnown) && len(node.Children) == 0 && node.Text.Len() == 0 {
		value := schema.Default
		if schema.Fixed != nil {
			value = schema.Fixed
		}
		if value != nil {
			// A fresh builder avoids copying strings.Builder's internal state.
			c.scratch(128)
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
			c.reserve(c.builderAllowance(int64(len(lexical))))
			c.bytes(len(lexical))
			effective.Text.WriteString(lexical)
			defaulted = true
			if c.local != nil {
				c.local.defaulted = true
			}
			node = effective
		}
	}

	if !nilled && !defaulted && !constraintBlocked && schema.Fixed != nil && c.scalarAvailable() {
		beforeFixed := c.AssessmentVersion
		// XSD 1.0 cvc-elt 5.2.2.1 forbids element children independently
		// of fixed-value equality, including for mixed content. Report the
		// structural failure first, but keep ordinary type assessment below.
		if len(node.Children) != 0 {
			declarationValid = false
			c.AddIssue(path, "fixed", "element with a fixed value must not contain child elements")
		} else if equal := c.elementFixedEqual(actualType, &schema.Type, node.Text.String(), *schema.Fixed); !equal && (!c.currentModel() || (c.AssessmentVersion == beforeFixed && !c.currentFixedUnavailable)) {
			declarationValid = false
			if c.currentModel() {
				c.local.result().Valid = false
			}
			c.AddIssue(
				path,
				"fixed",
				format(c,
					"value %q does not equal fixed value %q",
					node.Text.String(),
					*schema.Fixed,
				),
			)
		}
		declarationValid = declarationValid && c.AssessmentVersion == beforeFixed && (!c.currentModel() || !c.currentFixedUnavailable)
	}

	// An unavailable/invalid default blocks dependent scalar/content checks,
	// while actual-type attribute checks remain independent.
	if c.local != nil {
		c.local.effectiveText = node.Text.String()
		if c.currentModel() && c.local.subject.Current.Known && c.local.subject.Current.Value.Domain == StringDomain {
			c.local.effectiveText = c.local.subject.Current.Value.Text
		}
	}
	c.setFamily("scalar")
	typeValid := c.ValidateType(node, actualType, path, nilled || constraintBlocked, original) && typeAttributesValid
	declarationValid = declarationValid && typeValid && !constraintBlocked
	if c.local != nil {
		c.local.typeValid = typeValid
		c.local.declarationValid = declarationValid
		c.local.identityAllowed = !nilled && !constraintBlocked && c.scalarAvailable()
		// Capability is a permanent local fact available immediately at Bind;
		// postorder completion only gates registration, never discovers it late.
		if typeValid && c.local.identityAllowed {
			c.setFamily("declaration")
			if declarationValid {
				c.noteConstructedIdentity(&schema.Type)
			}
			c.noteConstructedIdentity(actualType)
		}
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
	c.step(1)

	if schema == nil {
		return nil, false
	}
	if schema.Reference.Local == "" {
		return schema, true
	}

	c.bytes(len(schema.Reference.Space))
	c.bytes(len(schema.Reference.Local))
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
	c.step(1)

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
		if !c.scalarAvailable() {
			return false
		}
		var assessment SimpleAssessment
		var failure *SimpleFailure
		if c.currentModel() {
			assessment, failure = c.assessCurrentType(reference, c.local.subject.Current)
		} else {
			assessment, failure = c.AssessSimpleValue(simple, node.Text.String(), node.Text.String())
		}
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
		if !c.scalarAvailable() {
			return false
		}
		var assessment SimpleAssessment
		var failure *SimpleFailure
		if c.currentModel() {
			assessment, failure = c.assessCurrentType(reference, c.local.subject.Current)
		} else {
			assessment, failure = c.AssessBuiltinValue(builtin, node.Text.String(), node.Text.String())
		}
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
	c.step(1)

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
	c.bytes(len(reference.Name.Space))
	c.bytes(len(reference.Name.Local))
	if reference.Name.Space == XSDNamespace {
		_, found := builtinSemanticsValue(reference.Name.Local)
		return nil, nil, reference.Name.Local, found
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
	c.step(1)

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
	characterViolation := false
	if c.currentModel() {
		characters, nonWhitespace := c.currentCharacters()
		characterViolation = c.scalarAvailable() && !effective.Mixed && characters && (effective.Particle == nil || nonWhitespace)
	} else {
		characterViolation = (c.local == nil || c.local.subject.TextKnown) && !effective.Mixed && node.Text.Len() != 0 && (effective.Particle == nil || trimXMLWhitespace(node.Text.String(), c) != "")
	}
	if characterViolation {
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
			c.childIssue(
				node, path, 0,
				"content-model",
				format(c,
					"unexpected element %s",
					displayName(node.Children[0].Name, c),
				),
			)
		}
		return attributesValid && contentValid && len(node.Children) == 0
	}

	if c.currentModel() && !c.local.subject.ChildrenOrderKnown {
		c.capability("content", "model-child-order-unavailable")
		c.IdentityIncomplete = true
		return false
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
		c.childIssue(
			node, path, index,
			"content-model",
			format(c,
				"unexpected element %s",
				displayName(node.Children[index].Name, c),
			),
		)
		return false
	}

	if c.local != nil {
		c.local.matches = result.Elements
		return attributesValid && contentValid
	}
	paths := childPaths(path, node.Children, c)
	for _, match := range result.Elements {
		c.step(1)
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
	c.step(1)

	if schema == nil {
		return nil, false
	}
	if cached, found := c.Effective[schema]; found {
		return cached, cached != nil
	}

	// Preserve the established effective-record/map allowance and separately
	// admit the host-sized fields added for the run-local absence cache.
	c.reserve(160 + int64(unsafe.Sizeof(absentAttributeCache{})))
	c.reserve(c.size(int64(len(schema.Attributes)), 192))
	c.step(int64(len(schema.Attributes)))
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
			result.Attributes = mergeAttributes(
				base.Attributes,
				result.Attributes,
				c)
			if schema.Form == ComplexSimpleContentExtension {
				result.AnyAttribute = MergeAnyAttribute(base.AnyAttribute, result.AnyAttribute)
			}
			result.Mixed = result.Mixed || base.Mixed
		default:
			return nil, false
		}

		if schema.Form == ComplexSimpleContentRestriction && schema.SimpleContent != nil {
			c.reserve(512)
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

		result.Particle = sequence(
			base.Particle,
			result.Particle,
			c)
		result.Attributes = mergeAttributes(
			base.Attributes,
			result.Attributes,
			c)
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
		result.Attributes = mergeAttributes(base.Attributes, result.Attributes, c)
		// Content and wildcard constraints belong to the restriction itself.
		return result, true

	default:
		return nil, false
	}
}

func MergeAttributes(base, derived []AttributeSchema) []AttributeSchema {
	return mergeAttributes(base, derived, nil)
}
func mergeAttributes(base, derived []AttributeSchema, c *Context) []AttributeSchema {
	c.step(1)

	c.reserve(c.size(c.sum(int64(len(base)), int64(len(derived))), 384))
	c.scratch(c.size(c.sum(int64(len(base)), int64(len(derived))), 96))
	c.step(int64(len(base)))
	result := append([]AttributeSchema(nil), base...)
	indexes := make(map[QName]int, len(result))
	for index := range result {
		c.step(1)
		indexes[result[index].Name] = index
	}

	for _, attribute := range derived {
		c.step(1)
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

func Sequence(left, right *ParticleSchema) *ParticleSchema { return sequence(left, right, nil) }
func sequence(left, right *ParticleSchema, c *Context) *ParticleSchema {
	c.step(1)

	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	default:
		c.reserve(160)
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
	c.step(1)

	valid := true
	for _, attribute := range node.Attrs {
		c.step(1)
		if NamespaceDeclaration(attribute) {
			continue
		}
		if StandardXSIName(attribute.Name, true) {
			continue
		}
		valid = false
		c.AddIssue(
			attributePath(path, attribute.Name, c),
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
	c.step(1)

	valid := true
	c.scratch(c.sum(128, c.size(int64(len(schemas)), 256), c.size(int64(len(node.Attrs)), 160)))
	known := make(map[QName]AttributeSchema, len(schemas))
	present := make(map[QName]Attr, len(node.Attrs))
	for _, schema := range schemas {
		c.step(1)
		known[schema.Name] = schema
	}

	for _, attribute := range node.Attrs {
		c.step(1)
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
			c.local.attributeLimited = false
			c.local.attributeInvalid = false
			c.local.currentAttributeSet = c.currentModel()
			if c.currentModel() {
				c.local.currentAttribute = c.checkCurrent(attribute.Current)
				c.local.attributeKnown = attribute.Current.Known && !attribute.Current.Absent
			}
			c.local.scalarAssessed = false
		}
		name := Name(attribute.Name)
		schema, found := known[name]
		if !found {
			if anyAttributeAllows(anyAttribute, attribute.Name, c) {
				if anyAttribute != nil {
					c.wildcard(anyAttribute.ProcessContents, "attributes")
				}
				continue
			}
			valid = false
			c.AddIssue(
				attributePath(path, attribute.Name, c),
				"attribute",
				"attribute is not allowed",
			)
			continue
		}
		present[name] = attribute.Attr

		if schema.Use == AttributeProhibited {
			valid = false
			c.AddIssue(
				attributePath(path, attribute.Name, c),
				"prohibited",
				"attribute is prohibited",
			)
			continue
		}

		if c.local != nil && !c.local.attributeKnown {
			c.local.limit("attributes", "attribute-value-unavailable", nil)
			if !c.currentModel() || c.currentIdentityCapable(&schema.Type) {
				c.IdentityIncomplete = true
			}
			c.attributeResult(schema, attribute.Value, true, false, false)
			valid = false
			continue
		}
		attributeValid := true
		if c.currentModel() {
			c.currentFixedUnavailable = false
		}
		beforeFixed := c.AssessmentVersion
		if schema.Constraint != nil &&
			schema.Constraint.Kind == "fixed" &&
			!c.attributeFixedEqual(
				&schema.Type,
				attribute.Value,
				schema.Constraint.Value,
			) && c.AssessmentVersion == beforeFixed && (!c.currentModel() || !c.currentFixedUnavailable) {
			attributeValid = false
			valid = false
			if c.currentModel() {
				c.local.result().Valid = false
				c.local.attributeInvalid = true
			}
			c.AddIssue(
				attributePath(path, attribute.Name, c),
				"fixed",
				format(c,
					"value %q does not equal fixed value %q",
					attribute.Value,
					schema.Constraint.Value,
				),
			)
		}

		if c.AssessmentVersion != beforeFixed || (c.currentModel() && c.currentFixedUnavailable) {
			attributeValid = false
			valid = false
		}
		if failure := c.ValidateSimpleType(
			&schema.Type,
			attribute.Value,
		); failure != nil {
			valid = false
			c.AddIssue(
				attributePath(path, attribute.Name, c),
				failure.Constraint,
				failure.Message,
			)
		} else {
			attributePath := attributePath(path, attribute.Name, c)
			if attributeValid {
				c.RecordIdentity(&schema.Type, c.attributeIdentityText(attribute.Value), attributePath)
			}
			c.RecordIdentityBinding(&schema.Type, c.attributeIdentityText(attribute.Value), attributePath, node)
		}
		c.attributeResult(schema, attribute.Value, true, c.local == nil || c.local.scalarAssessed, attributeValid && (c.local == nil || c.local.scalarAssessed))
	}

	if c.local != nil {
		c.local.attribute = QName{}
		c.local.currentAttributeSet = false
	}
	for _, schema := range schemas {
		c.step(1)
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
			attributeQNamePath(path, schema.Name, c),
			"required",
			"required attribute is missing",
		)
	}
	c.absentAttributeFacts(schemas, present)
	if c.sourceDeclarations && c.local != nil && !c.local.results.Attributes.Valid {
		return false
	}
	return valid
}

func (c *Context) ValidateSimpleType(
	reference *TypeRef,
	value string,
) *SimpleFailure {
	c.step(1)
	if current, ok := c.inputCurrent(); ok {
		assessment, failure := c.assessCurrentType(reference, current)
		c.observeScalar(assessment, failure == nil)
		return failure
	}

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
	c.step(1)

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
	if c.local != nil && !semantics.Aggregate() && !controlledSupportedEquality(atomicValue(semantics, semantics.normalize(left, c), c), c) {
		c.Incomplete("value-space equality is unavailable for xs:" + builtin)
		return false
	}
	if builtin == "ENTITY" {
		return c.AggregateValuesEqual(semantics, left, right)
	}
	if _, _, integer := IntegerLimits(builtin); integer {
		return integerValuesEqual(left, right, c)
	}
	if builtin == "decimal" {
		return decimalValuesEqual(left, right, c)
	}
	if semantics.StringValue {
		return compareText(semantics.normalize(left, c), semantics.normalize(right, c), c) == 0
	}
	// Preserve existing lexical comparison for other primitive families.
	if builtin == "" {
		builtin = "string"
	}
	return compareText(normalizeWhitespace(builtin, left, c), normalizeWhitespace(builtin, right, c), c) == 0
}

// Atomic integer/decimal/string/boolean/URI and supported list/union values
// use their value contracts. Other primitives retain their lexical path;
// mixed content compares raw text.
func (c *Context) ElementFixedValuesEqual(reference *TypeRef, left, right string) bool {
	c.step(1)

	_, complex, _, _ := c.ResolveType(reference)
	if complex != nil {
		effective, ok := c.EffectiveComplex(complex)
		if !ok {
			c.Incomplete("fixed-value complex content metadata cannot be resolved")
			return false
		}
		if effective.Simple == nil {
			return compareText(left, right, c) == 0
		}
		reference = effective.Simple
	}
	semantics := c.SimpleTypeSemantics(reference)
	if semantics == nil {
		// As in simpleValuesEqual, retain fail-closed comparison and its
		// additional fixed issue alongside ordinary schema diagnostics.
		return false
	}
	if c.local != nil && !semantics.Aggregate() && !controlledSupportedEquality(atomicValue(semantics, semantics.normalize(left, c), c), c) {
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
		return integerValuesEqual(left, right, c)
	}
	if semantics.Builtin == "decimal" {
		return decimalValuesEqual(left, right, c)
	}
	if semantics.StringValue {
		return compareText(semantics.normalize(left, c), semantics.normalize(right, c), c) == 0
	}
	return compareText(left, right, c) == 0
}

func (c *Context) ValidateSimple(
	schema *SimpleSchema,
	value string,
) *SimpleFailure {
	c.step(1)

	_, failure := c.ValidateSimpleValue(schema, value, value)
	return failure
}

// Keep the lexical-returning helper for callers that do not compare values.
func (c *Context) ValidateSimpleValue(schema *SimpleSchema, value, display string) (string, *SimpleFailure) {
	c.step(1)

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
	c.step(1)

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
	value = semantics.normalize(value, c)
	// Direct ENTITY/ENTITIES restriction chains first establish lexical/facet
	// validity. The document-domain prerequisite cannot hide an independent facet
	// failure, and only the outermost restriction performs its membership check.
	if c.sourceDeclarations && c.entityLiteralDepth == 0 && entityBuiltin(semantics.Builtin) && semantics.List == nil && semantics.Union == nil {
		c.entityDeferred++
		defer func() {
			// An interrupted lexical/facet operation has no result to assess. Restore
			// scope without converting its zero result into a declaration absence.
			stop := recover()
			c.entityDeferred--
			if stop != nil {
				panic(stop)
			}
			if failure == nil && c.entityDeferred == 0 {
				failure = c.assessEntityMembership(semantics.Builtin, result.Lexical)
			}
		}()
	}
	if c.sourceDeclarations && c.entityLiteralDepth == 0 && (schema.Form == SimpleList || schema.Form == SimpleUnion) {
		c.entityConstructedDepth++
		defer func() { c.entityConstructedDepth-- }()
	}
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
				Message: format(c,
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
			c.step(1)
			matched, err := matchPattern(pattern, normalized, c)
			if err != nil {
				return SimpleAssessment{}, &SimpleFailure{
					Constraint: "schema",
					Message: format(c,
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
				message = format(c, "value %q does not match XSD pattern %q", display, schema.Patterns[0])
			} else {
				message = format(c, "value %q does not match any XSD pattern in %q", display, schema.Patterns)
			}
			return SimpleAssessment{}, &SimpleFailure{Constraint: "pattern", Message: message}
		}

		if failure := validateBoundsValue(schema, normalized, builtin, display, c); failure != nil {
			return SimpleAssessment{}, failure
		}
		if failure := validateLengthFacets(
			schema,
			normalized,
			c.SimpleMemberList(schema.Base),
			c); failure != nil {
			return SimpleAssessment{}, failure
		}
		if failure := validateDigitFacets(schema, normalized, builtin, c); failure != nil {
			return SimpleAssessment{}, failure
		}

		return assessment, nil

	case SimpleUnion:
		for _, member := range schema.Members {
			c.step(1)
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
			Message:    format(c, "value %q does not match any union member", display),
		}

	case SimpleList:
		if c.sourceDeclarations && c.entityLiteralDepth == 0 && value == "" && c.entityBearingMember(schema.Item) {
			return SimpleAssessment{}, c.entityConstructionUnavailable()
		}
		result := SimpleAssessment{Lexical: value, Value: SimpleValue{Domain: ListDomain}}
		for index, item := range splitList(value, c) {
			c.step(1)
			assessment, failure := c.AssessSimpleMemberValue(schema.Item, item, item)
			if failure != nil {
				return SimpleAssessment{}, &SimpleFailure{Constraint: failure.Constraint, Message: format(c, "list item %d: %s", index+1, failure.Message)}
			}
			result.Value.Items = appendSimpleValue(result.Value.Items, assessment.Value, c)
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
	c.step(1)

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
	c.step(1)

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
	c.step(1)

	if reference == nil {
		return ""
	}
	if reference.InlineSimple != nil {
		return c.SimpleSchemaBuiltin(reference.InlineSimple)
	}
	c.bytes(len(reference.Name.Space))
	c.bytes(len(reference.Name.Local))
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
	c.step(1)

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
	c.step(1)

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
	c.step(1)

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
	c.step(1)

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
	return validateBuiltinValue(name, value, display, nil)
}
func validateBuiltinValue(name, value, display string, c *Context) *SimpleFailure {
	c.step(1)

	if name == "boolean" {
		if _, failure := parseBoolean(value, c); failure != nil {
			return &SimpleFailure{Constraint: "datatype", Message: format(c, "value %q is not valid for xs:boolean", display)}
		}
		return nil
	}
	normalized := normalizeWhitespace(name, value, c)
	c.bytes(len(normalized))

	switch name {
	case "anySimpleType", "string", "normalizedString", "token":
		return nil

	case "anyURI":
		if validXSDAnyURI(normalized, c) {
			return nil
		}

	case "language":
		if scalarPatternMatch(LanguagePattern, normalized, c) {
			return nil
		}

	case "Name":
		if validXSDName(normalized, c) {
			return nil
		}

	case "NCName", "ID", "IDREF", "ENTITY":
		if validXSDNCName(normalized, c) {
			return nil
		}

	case "NMTOKEN":
		if validXSDNMTOKEN(normalized, c) {
			return nil
		}

	case "NMTOKENS":
		items := splitList(normalized, c)
		if len(items) != 0 &&
			slices.ContainsFunc(items, func(item string) bool {
				return !validXSDNMTOKEN(item, c)
			}) == false {
			return nil
		}

	case "IDREFS", "ENTITIES":
		items := splitList(normalized, c)
		if len(items) != 0 &&
			slices.ContainsFunc(items, func(item string) bool {
				return !validXSDNCName(item, c)
			}) == false {
			return nil
		}

	case "decimal":
		if scalarPatternMatch(DecimalPattern, normalized, c) {
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
		if validInteger(name, normalized, c) {
			return nil
		}

	case "base64Binary":
		if scalarPatternMatch(Base64Pattern, normalized, c) {
			return nil
		}

	case "hexBinary":
		if len(normalized)%2 == 0 &&
			scalarPatternMatch(HexPattern, normalized, c) {
			return nil
		}

	case "QName", "NOTATION":
		if validXSDQName(normalized, c) {
			return nil
		}

	case "date", "dateTime", "duration", "gDay", "gMonth",
		"gMonthDay", "gYear", "gYearMonth", "time":
		if validXSDDateTime(name, normalized, c) {
			return nil
		}

	case "anyType":
		return nil

	default:
		return &SimpleFailure{
			Constraint: "schema",
			Message:    format(c, "unsupported XSD built-in type %q", name),
		}
	}

	return &SimpleFailure{
		Constraint: "datatype",
		Message: format(c,
			"value %q is not valid for xs:%s",
			display,
			name,
		),
	}
}

// parseValidationBoolean shares XSD whitespace and lexical handling between
// ordinary boolean values and the xsi:nil element constraint.
func ParseBoolean(value string) (bool, *SimpleFailure) { return parseBoolean(value, nil) }
func parseBoolean(value string, c *Context) (bool, *SimpleFailure) {
	c.step(1)

	switch normalizeWhitespace("boolean", value, c) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, &SimpleFailure{
			Constraint: "datatype",
			Message:    format(c, "value %q is not valid for xs:boolean", value),
		}
	}
}

// Reuse the concurrency-safe replacer across builtin checks and resolved
// replace-policy views instead of rebuilding it for every value or literal.
var ReplaceWhitespace = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func NormalizeWhitespace(name, value string) string { return normalizeWhitespace(name, value, nil) }
func normalizeWhitespace(name, value string, c *Context) string {
	c.scalar(int64(len(value)))
	c.step(1)

	switch name {
	case "string", "anySimpleType":
		return value
	case "normalizedString":
		c.bytes(len(value))
		c.reserve(int64(len(value)))
		out := ReplaceWhitespace.Replace(value)
		c.step(0)
		return out
	default:
		return collapseWhitespace(value, c)
	}
}

func CollapseWhitespace(value string) string { return collapseWhitespace(value, nil) }
func collapseWhitespace(value string, c *Context) string {
	c.scalar(int64(len(value)))
	c.step(1)

	var result strings.Builder
	capacity := int64(len(value))
	if c != nil && c.run != nil {
		c.bytes(len(value))
		valid := utf8.ValidString(value)
		c.step(0)
		if !valid {
			capacity = 0
			for _, character := range value {
				c.step(1)
				capacity = c.sum(capacity, int64(utf8.RuneLen(character)))
			}
		}
		c.scalar(capacity)
	}
	c.reserve(c.builderAllowance(capacity))
	result.Grow(int(capacity))
	wroteValue := false
	pendingSpace := false

	for _, character := range value {
		c.step(1)
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

func SplitList(value string) []string { return splitList(value, nil) }
func splitList(value string, c *Context) []string {
	c.step(1)

	normalized := collapseWhitespace(value, c)
	if normalized == "" {
		return nil
	}

	c.bytes(len(normalized))
	count := strings.Count(normalized, " ") + 1
	c.reserve(c.size(int64(count), 16))
	c.bytes(len(normalized))
	result := strings.Split(normalized, " ")
	c.step(0)
	return result
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
	return validateBoundsValue(schema, value, builtin, display, nil)
}
func validateBoundsValue(schema *SimpleSchema, value, builtin, display string, c *Context) *SimpleFailure {
	c.step(1)

	if !schema.HasMinInclusive &&
		!schema.HasMaxInclusive &&
		!schema.HasMinExclusive &&
		!schema.HasMaxExclusive {
		return nil
	}

	if _, _, integer := IntegerLimits(builtin); integer {
		return validateIntegerBounds(schema, value, display, c)
	}
	if builtin == "decimal" {
		return validateDecimalBounds(schema, value, display, c)
	}

	c.bytes(len(value))
	number, err := strconv.ParseFloat(value, 64)
	c.step(0)
	if err != nil || math.IsNaN(number) {
		return &SimpleFailure{
			Constraint: "datatype",
			Message:    format(c, "value %q is not numeric", display),
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
		c.step(1)
		if !test.Enabled {
			continue
		}
		c.bytes(len(test.Limit))
		limit, err := strconv.ParseFloat(test.Limit, 64)
		c.step(0)
		if err != nil {
			return &SimpleFailure{
				Constraint: "schema",
				Message: format(c,
					"invalid %s value %q",
					test.Constraint,
					test.Limit,
				),
			}
		}
		if !test.Valid(number, limit) {
			return &SimpleFailure{
				Constraint: test.Constraint,
				Message: format(c,
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

func ValidateLengthFacets(schema *SimpleSchema, value string, list bool) *SimpleFailure {
	return validateLengthFacets(schema, value, list, nil)
}
func validateLengthFacets(schema *SimpleSchema, value string, list bool, c *Context) *SimpleFailure {
	c.step(1)

	if !schema.HasLength &&
		!schema.HasMinLength &&
		!schema.HasMaxLength {
		return nil
	}

	c.bytes(len(value))
	length := uint64(utf8.RuneCountInString(value))
	c.step(0)
	if list {
		length = uint64(len(splitList(value, c)))
	}

	switch {
	case schema.HasLength && length != schema.Length:
		return &SimpleFailure{
			Constraint: "length",
			Message: format(c,
				"value length %d does not equal %d",
				length,
				schema.Length,
			),
		}
	case schema.HasMinLength && length < schema.MinLength:
		return &SimpleFailure{
			Constraint: "minLength",
			Message: format(c,
				"value length %d is less than %d",
				length,
				schema.MinLength,
			),
		}
	case schema.HasMaxLength && length > schema.MaxLength:
		return &SimpleFailure{
			Constraint: "maxLength",
			Message: format(c,
				"value length %d is greater than %d",
				length,
				schema.MaxLength,
			),
		}
	default:
		return nil
	}
}

func ValidateDigitFacets(schema *SimpleSchema, value, builtin string) *SimpleFailure {
	return validateDigitFacets(schema, value, builtin, nil)
}
func validateDigitFacets(schema *SimpleSchema, value, builtin string, c *Context) *SimpleFailure {
	c.step(1)

	if !schema.HasTotalDigits && !schema.HasFractionDigits {
		return nil
	}

	var total, fraction uint64
	if builtin == "decimal" {
		number, ok := parseDecimal(value, c)
		if !ok {
			return &SimpleFailure{Constraint: "datatype", Message: format(c, "value %q is not numeric", value)}
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
			if number, ok := parseInteger(value, c); ok {
				value = number.Digits
			}
		}
		normalized := strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
		c.bytes(len(normalized))
		c.scratch(32)
		parts := strings.SplitN(normalized, ".", 2)
		for _, character := range normalized {
			c.step(1)
			if unicode.IsDigit(character) {
				total++
			}
		}
		if len(parts) == 2 {
			c.bytes(len(parts[1]))
			fraction = uint64(utf8.RuneCountInString(parts[1]))
		}
	}

	if schema.HasTotalDigits && total > schema.TotalDigits {
		return &SimpleFailure{
			Constraint: "totalDigits",
			Message: format(c,
				"value has %d digits; maximum is %d",
				total,
				schema.TotalDigits,
			),
		}
	}
	if schema.HasFractionDigits && fraction > schema.FractionDigits {
		return &SimpleFailure{
			Constraint: "fractionDigits",
			Message: format(c,
				"value has %d fraction digits; maximum is %d",
				fraction,
				schema.FractionDigits,
			),
		}
	}

	return nil
}

func MatchPattern(pattern, value string) (bool, error) { return matchPattern(pattern, value, nil) }
func matchPattern(pattern, value string, c *Context) (bool, error) {
	c.step(1)

	c.bytes(len(pattern))
	c.scratch(c.sum(int64(len(pattern)), c.size(int64(strings.Count(pattern, `\c`)), int64(len(XMLNameCharacters)))))
	translated := strings.ReplaceAll(
		pattern,
		`\c`,
		`[`+XMLNameCharacters+`]`,
	)
	c.bytes(len(translated))
	c.scratch(c.sum(int64(len(translated)), c.size(int64(strings.Count(translated, `\i`)), int64(len(XMLNameStartCharacters)))))
	translated = strings.ReplaceAll(
		translated,
		`\i`,
		`[`+XMLNameStartCharacters+`]`,
	)

	// XSD \d denotes Unicode decimal digits, including inside character classes.
	c.bytes(len(translated))
	c.scratch(c.size(int64(len(translated)), 3))
	translated = strings.ReplaceAll(translated, `\d`, `\p{Nd}`)

	c.bytes(len(translated))
	c.scratch(c.sum(1024, c.size(int64(len(translated)), 128)))
	program := regexpAllowance(translated, c)
	compiled, err := regexp.Compile(`^(?:` + translated + `)$`)
	if err != nil {
		return false, err
	}

	c.step(0)
	c.bytes(len(value))
	if program > 0 {
		c.step(c.size(program, int64(len(value))))
	}
	c.scratch(c.sum(256, c.size(program, 16), c.size(int64(len(value)), 16)))
	matched := compiled.MatchString(value)
	c.step(0)
	return matched, nil
}

func BetterFailure(left, right MatchFailure) MatchFailure { return betterFailure(left, right, nil) }
func betterFailure(left, right MatchFailure, c *Context) MatchFailure {
	c.step(1)

	switch {
	case right.Position > left.Position:
		return right
	case right.Position < left.Position:
		return left
	default:
		c.scratch(c.size(c.sum(int64(len(left.Expected)), int64(len(right.Expected))), 32))
		left.Expected = uniqueNames(
			slices.Concat(left.Expected, right.Expected),
			c)
		return left
	}
}

func UniqueNames(values []QName) []QName { return uniqueNames(values, nil) }
func uniqueNames(values []QName, c *Context) []QName {
	c.step(1)

	c.scratch(c.sum(64, c.size(int64(len(values)), 112)))
	result := make([]QName, 0, len(values))
	seen := make(map[QName]struct{}, len(values))
	for _, value := range values {
		c.step(1)
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
		c.step(1)
		c.bytes(len(left.Space))
		c.bytes(len(right.Space))
		c.bytes(len(left.Local))
		c.bytes(len(right.Local))
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
	c.step(1)

	if failure.Schema != "" {
		c.AddIssue(path, "schema", failure.Schema)
		return
	}
	position := min(failure.Position, len(node.Children))
	expected := expectedNames(failure.Expected, c)

	if position == len(node.Children) {
		message := "required child element is missing"
		if expected != "" {
			message += "; expected " + expected
		}
		c.AddIssue(path, "content-model", message)
		return
	}

	message := format(c,
		"unexpected element %s",
		displayName(node.Children[position].Name, c),
	)
	if expected != "" {
		message += "; expected " + expected
	}
	c.childIssue(
		node, path, position,
		"content-model",
		message,
	)
}

func ExpectedNames(names []QName) string { return expectedNames(names, nil) }
func expectedNames(names []QName, c *Context) string {
	c.step(1)

	if len(names) == 0 {
		return ""
	}
	c.scratch(c.size(int64(len(names)), 32))
	values := make([]string, len(names))
	for index, name := range names {
		c.step(1)
		values[index] = displayQName(name, c)
	}
	n := int64(0)
	for _, value := range values {
		c.bytes(len(value))
		n = c.sum(n, int64(len(value)), 4)
	}
	c.scratch(n)
	return strings.Join(values, " or ")
}

func (c *Context) AddIssue(
	path string,
	constraint string,
	message string,
) {
	c.step(1)

	if constraint == "model-capability" {
		return
	}
	if constraint == "schema" || constraint == "assessment" {
		if constraint == "schema" || c.AssessmentErr == nil {
			c.reserve(c.sum(int64(len(path)), 2, int64(len(message))))
			c.bytes(len(path))
			c.bytes(len(message))
			c.Incomplete(path + ": " + message)
		}
		return
	}
	if c.local != nil {
		c.local.detail(path, constraint, message)
		return
	}
	if c.run != nil && c.run.identityOutput != nil {
		c.identityDetail(path, constraint, message)
		return
	}
	c.Issues = append(c.Issues, Issue{
		Path:       path,
		Constraint: constraint,
		Message:    message,
	})
}

func AnyAttributeAllows(schema *AnyAttributeSchema, name QName) bool {
	return anyAttributeAllows(schema, name, nil)
}
func anyAttributeAllows(schema *AnyAttributeSchema, name QName, c *Context) bool {
	c.step(1)

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
		c.bytes(len(schema.Namespace))
		c.scratch(c.size(int64(len(schema.Namespace)), 16))
		for _, namespace := range strings.Fields(schema.Namespace) {
			c.step(1)
			if namespace == name.Space {
				return true
			}
		}
		return false
	}
}

func NilAttribute(attributes []Attribute) (string, bool) { return nilAttribute(attributes, nil) }
func nilAttribute(attributes []Attribute, c *Context) (string, bool) {
	c.step(1)

	for _, attribute := range attributes {
		c.step(1)
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

func DisplayName(name QName) string             { return displayName(name, nil) }
func displayName(name QName, c *Context) string { return displayQName(Name(name), c) }

func DisplayQName(name QName) string { return displayQName(name, nil) }
func displayQName(name QName, c *Context) string {
	c.bytes(len(name.Space))
	c.bytes(len(name.Local))
	c.scratch(c.sum(int64(len(name.Space)), int64(len(name.Local)), 6))
	if name.Space == "" {
		return "<" + name.Local + ">"
	}
	return fmt.Sprintf("<{%s}%s>", name.Space, name.Local)
}

func Path(parent string, local string, index int) string {
	return controlledPath(parent, local, index, nil)
}
func controlledPath(parent, local string, index int, c *Context) string {
	c.bytes(len(parent))
	c.bytes(len(local))
	c.scratch(c.sum(int64(len(parent)), int64(len(local)), 32))
	if parent == "" {
		return "/" + local
	}
	if index <= 1 {
		return parent + "/" + local
	}
	return fmt.Sprintf("%s/%s[%d]", parent, local, index)
}

func ChildPath(parent string, child *Node, index int) string {
	return controlledChildPath(parent, child, index, nil)
}
func controlledChildPath(parent string, child *Node, index int, c *Context) string {
	return controlledPath(parent, child.Name.Local, index, c)
}

func ChildPaths(parent string, children []*Node) []string { return childPaths(parent, children, nil) }
func childPaths(parent string, children []*Node, c *Context) []string {
	c.step(1)

	c.scratch(c.sum(64, c.size(int64(len(children)), 96)))
	result := make([]string, len(children))
	counts := make(map[QName]int)
	for index, child := range children {
		c.step(1)
		counts[child.Name]++
		result[index] = controlledChildPath(parent, child, counts[child.Name], c)
	}
	return result
}

func IndexedChildPath(parent string, children []*Node, index int) string {
	return indexedChildPath(parent, children, index, nil)
}
func indexedChildPath(parent string, children []*Node, index int, c *Context) string {
	c.step(1)

	if index < 0 || index >= len(children) {
		return parent
	}
	count := 0
	for current := 0; current <= index; current++ {
		c.step(1)
		if children[current].Name == children[index].Name {
			count++
		}
	}
	return controlledChildPath(parent, children[index], count, c)
}

func AttributePath(parent string, name QName) string { return attributePath(parent, name, nil) }
func attributePath(parent string, name QName, c *Context) string {
	c.step(1)

	if name.Space == "" {
		c.scratch(c.sum(int64(len(parent)), int64(len(name.Local)), 2))
		c.bytes(len(parent))
		c.bytes(len(name.Local))
		return parent + "/@" + name.Local
	}
	return format(c, "%s/@{%s}%s", parent, name.Space, name.Local)
}

func AttributeQNamePath(parent string, name QName) string {
	return attributeQNamePath(parent, name, nil)
}
func attributeQNamePath(parent string, name QName, c *Context) string {
	c.step(1)

	return attributePath(parent, QName{
		Space: name.Space,
		Local: name.Local,
	}, c)
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
