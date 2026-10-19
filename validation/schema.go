package validation

import "github.com/go-muse/go-musicxml/internal/xsd"

// The Schema* types describe XML-neutral XSD metadata for CompileSchema. Build
// values with keyed composite literals, because fields may be added. Expanded
// names are QName values; a built-in type is named in SchemaXSDNamespace.
// Pointers may be shared, and sharing is significant: a type or element reached
// through one pointer is one schema component.

// SchemaData is XML-neutral XSD metadata. CompileSchema snapshots it; successful
// compilation is not a claim of general XSD schema validity or UPA closure.
type SchemaData struct {
	// Types holds the named simple and complex types.
	Types map[QName]*SchemaType
	// Elements holds the global element declarations.
	Elements map[QName]*SchemaElement
}

// SchemaType describes a named simple or complex type. Exactly one of Simple
// and Complex is set.
type SchemaType struct {
	Simple  *SchemaSimple
	Complex *SchemaComplex
}

// SchemaTypeRef names a type or contains its anonymous definition. Exactly one
// of Name, InlineSimple and InlineComplex is set.
type SchemaTypeRef struct {
	Name          QName
	InlineSimple  *SchemaSimple
	InlineComplex *SchemaComplex
}

// SchemaSimple describes one restriction, list or ordered union definition.
type SchemaSimple struct {
	// Final prohibits the listed derivations from this type.
	Final SchemaDerivation
	Form  SchemaSimpleForm
	// Base is the restricted type of a restriction.
	Base *SchemaSimpleMember
	// Enumerations lists the permitted lexical values; empty permits all.
	Enumerations []string
	// Members lists the member types of a union, in order.
	Members []*SchemaSimpleMember
	// Item is the item type of a list.
	Item *SchemaSimpleMember
	// Patterns lists the XSD regular expressions of this restriction layer;
	// empty permits all. Only a restriction uses them; a list or union ignores
	// them. Within one layer they are alternatives: a value must match at least
	// one. Layers intersect: a value must also satisfy the patterns of every
	// base type it restricts.
	Patterns []string

	// Bound facets are lexical values in the base type's value space. Each
	// one applies only when its Has field is true.
	MinInclusive   string
	MaxInclusive   string
	MinExclusive   string
	MaxExclusive   string
	Length         uint64
	MinLength      uint64
	MaxLength      uint64
	TotalDigits    uint64
	FractionDigits uint64
	// WhiteSpace is "preserve", "replace", "collapse" or "" to inherit.
	WhiteSpace string

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

// SchemaSimpleMember names or embeds a simple type. Exactly one of Name and
// Inline is set.
type SchemaSimpleMember struct {
	Name   QName
	Inline *SchemaSimple
}

// SchemaComplex describes complex content and attribute declarations.
type SchemaComplex struct {
	// Block and Final prohibit the listed derivations and substitutions.
	Block, Final SchemaDerivation
	Abstract     bool
	// SimpleContent is a local restriction layer. A missing scalar Base is
	// bound to the inherited effective scalar in a per-context copy.
	SimpleContent *SchemaSimple
	Form          SchemaComplexForm
	// Base is the base type of an extension or restriction.
	Base *SchemaTypeRef
	// Particle is the content model, or nil for empty or simple content.
	Particle     *SchemaParticle
	Attributes   []SchemaAttribute
	AnyAttribute *SchemaAnyAttribute
	Mixed        bool
}

// SchemaAttribute describes one exact declared attribute use.
type SchemaAttribute struct {
	Name       QName
	Type       SchemaTypeRef
	Use        SchemaAttributeUse
	Constraint *SchemaValueConstraint
}

// SchemaValueConstraint records a default or fixed schema value.
type SchemaValueConstraint struct {
	// Kind is SchemaConstraintDefault or SchemaConstraintFixed.
	Kind  string
	Value string
}

// SchemaAnyAttribute describes an attribute wildcard and its processing mode.
type SchemaAnyAttribute struct {
	// Namespace is the XSD namespace constraint, such as "##other".
	Namespace string
	// ProcessContents is one of the SchemaProcess constants, or "" for strict.
	ProcessContents string
}

// SchemaParticle describes a nested sequence, choice, all, element or wildcard.
type SchemaParticle struct {
	Kind       SchemaParticleKind
	Occurrence SchemaOccurrence
	// Element is set for an element particle.
	Element *SchemaElement
	// Children are the nested particles of a sequence, choice or all.
	Children []*SchemaParticle
	// Any is set for a wildcard particle.
	Any *SchemaAny
}

// SchemaOccurrence records exact particle occurrence bounds. Unbounded
// replaces Max.
type SchemaOccurrence struct {
	Min       uint64
	Max       uint64
	Unbounded bool
}

// SchemaAny describes an element wildcard and its processing mode.
type SchemaAny struct {
	// Namespace is the XSD namespace constraint, such as "##any".
	Namespace string
	// ProcessContents is one of the SchemaProcess constants, or "" for strict.
	ProcessContents string
}

// SchemaElement describes an element declaration or reference use.
type SchemaElement struct {
	// Block and Final prohibit the listed derivations and substitutions.
	Block, Final SchemaDerivation
	Name         QName
	// Reference names the global declaration that this use refers to.
	Reference QName
	Type      SchemaTypeRef
	Default   *string
	Fixed     *string
	Nillable  bool
	Abstract  bool
}

// SchemaSimpleForm identifies an XSD simple-type composition.
type SchemaSimpleForm string

// SchemaComplexForm identifies an XSD complex-type composition.
type SchemaComplexForm string

// SchemaAttributeUse identifies an attribute use constraint.
type SchemaAttributeUse string

// SchemaParticleKind identifies an XSD particle operator.
type SchemaParticleKind string

// SchemaDerivation carries XSD derivation prohibition bits.
type SchemaDerivation uint8

// SchemaXSDNamespace is the XML Schema namespace that qualifies built-in type
// names in SchemaData, for example {http://www.w3.org/2001/XMLSchema}string.
const SchemaXSDNamespace = xsd.XSDNamespace

// Untyped vocabulary for string fields of the schema metadata types:
// SchemaValueConstraint.Kind and the ProcessContents field of SchemaAny and
// SchemaAnyAttribute. An empty ProcessContents selects the XSD default,
// strict processing.
const (
	SchemaConstraintDefault = "default"
	SchemaConstraintFixed   = "fixed"

	SchemaProcessStrict = "strict"
	SchemaProcessLax    = "lax"
	SchemaProcessSkip   = "skip"
)

// Supported schema vocabulary retains the authoritative XSD metadata values.
const (
	SchemaSimpleRestriction        SchemaSimpleForm   = "restriction"
	SchemaSimpleUnion              SchemaSimpleForm   = "union"
	SchemaSimpleList               SchemaSimpleForm   = "list"
	SchemaComplexDirect            SchemaComplexForm  = "direct"
	SchemaComplexSimpleExtension   SchemaComplexForm  = "simple content extension"
	SchemaComplexSimpleRestriction SchemaComplexForm  = "simple content restriction"
	SchemaComplexExtension         SchemaComplexForm  = "complex content extension"
	SchemaComplexRestriction       SchemaComplexForm  = "complex content restriction"
	SchemaAttributeOptional        SchemaAttributeUse = "optional"
	SchemaAttributeRequired        SchemaAttributeUse = "required"
	SchemaAttributeProhibited      SchemaAttributeUse = "prohibited"
	SchemaParticleElement          SchemaParticleKind = "element"
	SchemaParticleAll              SchemaParticleKind = "all"
	SchemaParticleChoice           SchemaParticleKind = "choice"
	SchemaParticleSequence         SchemaParticleKind = "sequence"
	SchemaParticleAny              SchemaParticleKind = "any"
	SchemaDerivationExtension      SchemaDerivation   = 1
	SchemaDerivationRestriction    SchemaDerivation   = 2
	SchemaDerivationList           SchemaDerivation   = 4
	SchemaDerivationUnion          SchemaDerivation   = 8
	SchemaDerivationSubstitution   SchemaDerivation   = 16
)

// CompiledSchema is an immutable assembly-qualified snapshot of bounded schema
// data. Its zero value is invalid. It can be shared by concurrent sessions.
type CompiledSchema struct {
	assembly Assembly
	kernel   *xsd.Compiled
	// Construction-only memoization is shared by value-copied caller handles,
	// but detached from immutable plan/catalog snapshots by snapshot.
	timeline *timelineBindingsCache
	context  *contextBindingsCache
}

// CompileSchema copies bounded schema metadata into an immutable handle. It
// preserves lazy reached-metadata assessment rather than accepting arbitrary
// schema graphs as normative XSD schemas. Unknown assemblies fail preflight.
func CompileSchema(assembly Assembly, data SchemaData) (*CompiledSchema, error) {
	if !validAssembly(assembly) {
		return nil, contract("schema assembly", "unknown assembly")
	}
	kernel, err := xsd.CompileGraph(data, xsd.CompileLimits{})
	if err != nil {
		return nil, &ConfigurationError{Field: "schema", Reason: err.Error()}
	}
	return &CompiledSchema{assembly: assembly, kernel: kernel, timeline: &timelineBindingsCache{}, context: &contextBindingsCache{}}, nil
}

// snapshot retains only the immutable schema graph. A later constructor on the
// caller's handle must not grow an already admitted catalog's retained state.
func (s *CompiledSchema) snapshot() *CompiledSchema {
	if s == nil {
		return nil
	}
	return &CompiledSchema{assembly: s.assembly, kernel: s.kernel}
}

// Digest returns the deterministic schema graph digest, or empty for a zero
// handle. Plans additionally bind its assembly into the catalog digest.
func (s *CompiledSchema) Digest() string {
	if s == nil || s.kernel == nil {
		return ""
	}
	return s.kernel.Digest()
}

// Assembly returns the exact assembly identity, or empty for a zero handle.
func (s *CompiledSchema) Assembly() Assembly {
	if s == nil || s.kernel == nil {
		return ""
	}
	return s.assembly
}

// Root returns the exact global declaration identity of a schema root.
func (s *CompiledSchema) Root(name QName) (DeclarationID, bool) {
	if s == nil || s.kernel == nil {
		return DeclarationID{}, false
	}
	use, ok := s.kernel.Root(xsd.QName{Space: name.Space, Local: name.Local})
	if !ok {
		return DeclarationID{}, false
	}
	return s.declaration(use), true
}

func (s *CompiledSchema) declaration(use xsd.Use) DeclarationID {
	return DeclarationID{Assembly: s.assembly, OwnerOccurrence: use.OwnerOccurrence(), Occurrence: use.Occurrence()}
}

// XSDCheck selects one sealed composite XSD operation. Counts record composite
// predicate instances, not each displayed issue or planning-registry record.
type XSDCheck string

// Supported XSD composite operations; identity finalization is document-scoped.
const (
	XSDCheckBinding     XSDCheck = "binding"
	XSDCheckDeclaration XSDCheck = "declaration"
	XSDCheckAttributes  XSDCheck = "attributes"
	XSDCheckScalar      XSDCheck = "scalar"
	XSDCheckContent     XSDCheck = "content"
	XSDCheckIdentity    XSDCheck = "identity"
)

func validXSDCheck(check XSDCheck) bool {
	switch check {
	case XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar, XSDCheckContent, XSDCheckIdentity:
		return true
	}
	return false
}

// NewXSDPlan creates a sealed shared-schema operation for source or current-model
// facts. Current values never reconstruct unavailable original source evidence.
// Its selector must be the exact Assembly branch matching the compiled schema.
func NewXSDPlan(schema *CompiledSchema, check XSDCheck) (Plan, error) {
	if !validXSDCheck(check) {
		return Plan{}, contract("XSD plan", "compiled schema and supported operation required")
	}
	return checkedPlan(Plan{kind: planXSD, schema: schema, xsdCheck: check})
}

// newXSDPlanWithSourceDeclarations selects the bounded source-document
// ENTITY/ENTITIES declaration prerequisite for this composite operation. Direct
// built-in types and restriction descendants use effective general-entity facts;
// reached arbitrary constructed ENTITY-bearing list/union semantics remain
// unavailable under the pinned XSD 1.0 contract. All selected XSD operations in
// an assembly must agree on this mode. Only source-target rules are accepted.
// The Session must receive SourceEntityDeclarations before root attribute facts.
func newXSDPlanWithSourceDeclarations(schema *CompiledSchema, check XSDCheck) (Plan, error) {
	return checkedPlan(Plan{kind: planXSD, schema: schema, xsdCheck: check, xsdMode: xsdSourceDeclarationsV1})
}

// Components returns independent assembly-qualified type identities. The index
// identifies metadata components without asserting their runtime support.
func (s *CompiledSchema) Components() []ComponentID {
	if s == nil || s.kernel == nil {
		return nil
	}
	components := s.kernel.Components()
	out := make([]ComponentID, len(components))
	for i, component := range components {
		out[i] = s.component(component)
	}
	return out
}

// Declarations returns independent exact element/reference/attribute use IDs.
// Repeated source occurrences of one schema use share its declaration identity.
func (s *CompiledSchema) Declarations() []DeclarationID {
	if s == nil || s.kernel == nil {
		return nil
	}
	uses := s.kernel.Uses()
	out := make([]DeclarationID, len(uses))
	for i, use := range uses {
		out[i] = s.declaration(use)
	}
	return out
}

func (s *CompiledSchema) component(kind xsd.TypeID) ComponentID {
	name := kind.Name()
	out := ComponentID{Assembly: s.assembly, Kind: ComponentType}
	if name.Local != "" {
		out.Name = QName{Space: name.Space, Local: name.Local}
	} else {
		out.Occurrence = kind.Occurrence()
	}
	return out
}

// SchemaUse describes one exact immutable declaration use and its type
// relationships. A QName alone does not distinguish repeated schema positions.
// The Known fields distinguish unavailable metadata from valid zero-valued names
// on anonymous types. Global declarations have no OwnerType.
type SchemaUse struct {
	Declaration       DeclarationID
	Name              QName
	Attribute         bool
	OwnerType         ComponentID
	OwnerTypeKnown    bool
	DeclaredType      ComponentID
	DeclaredTypeKnown bool
}

// Uses returns independent value descriptors in the same order as Declarations.
// Descriptors contain exact existing identities, never mutable schema metadata.
// Inherited attributes retain their original declaring type as OwnerType.
func (s *CompiledSchema) Uses() []SchemaUse {
	if s == nil || s.kernel == nil {
		return nil
	}
	uses := s.kernel.Uses()
	out := make([]SchemaUse, len(uses))
	for i, use := range uses {
		name := use.Declaration()
		out[i] = SchemaUse{Declaration: s.declaration(use), Name: QName{Space: name.Space, Local: name.Local}, Attribute: use.IsAttribute()}
		if owner, ok := use.OwnerType(); ok {
			out[i].OwnerType = s.component(owner)
			out[i].OwnerTypeKnown = true
		}
		if declared, ok := use.DeclaredType(); ok {
			out[i].DeclaredType = s.component(declared)
			out[i].DeclaredTypeKnown = true
		}
	}
	return out
}
