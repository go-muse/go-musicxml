package validation

import "github.com/go-muse/go-musicxml/internal/xsd"

// SchemaData is XML-neutral XSD metadata. CompileSchema snapshots it; successful
// compilation is not a claim of general XSD schema validity or UPA closure.
type SchemaData = xsd.SchemaData

// SchemaName is an expanded name in a schema metadata graph.
type SchemaName = xsd.QName

// SchemaType describes a named simple or complex type.
type SchemaType = xsd.TypeSchema

// SchemaTypeRef names a type or contains its anonymous definition.
type SchemaTypeRef = xsd.TypeRef

// SchemaSimple describes one restriction, list or ordered union definition.
type SchemaSimple = xsd.SimpleSchema

// SchemaSimpleMember names or embeds a simple type.
type SchemaSimpleMember = xsd.SimpleMember

// SchemaComplex describes complex content and attribute declarations.
type SchemaComplex = xsd.ComplexSchema

// SchemaAttribute describes one exact declared attribute use.
type SchemaAttribute = xsd.AttributeSchema

// SchemaValueConstraint records a default or fixed schema value.
type SchemaValueConstraint = xsd.ValueConstraint

// SchemaAnyAttribute describes an attribute wildcard and its processing mode.
type SchemaAnyAttribute = xsd.AnyAttributeSchema

// SchemaParticle describes a nested sequence, choice, all, element or wildcard.
type SchemaParticle = xsd.ParticleSchema

// SchemaOccurrence records exact particle occurrence bounds.
type SchemaOccurrence = xsd.Occurrence

// SchemaAny describes an element wildcard and its processing mode.
type SchemaAny = xsd.AnySchema

// SchemaElement describes an element declaration or reference use.
type SchemaElement = xsd.ElementSchema

// SchemaSimpleForm identifies an XSD simple-type composition.
type SchemaSimpleForm = xsd.SimpleForm

// SchemaComplexForm identifies an XSD complex-type composition.
type SchemaComplexForm = xsd.ComplexForm

// SchemaAttributeUse identifies an attribute use constraint.
type SchemaAttributeUse = xsd.AttributeUse

// SchemaParticleKind identifies an XSD particle operator.
type SchemaParticleKind = xsd.ParticleKind

// SchemaDerivation carries XSD derivation prohibition bits.
type SchemaDerivation = xsd.Derivation

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
}

// CompileSchema copies bounded schema metadata into an immutable handle. It
// preserves lazy reached-metadata assessment rather than accepting arbitrary
// schema graphs as normative XSD schemas. Unknown assemblies fail preflight.
func CompileSchema(assembly Assembly, data SchemaData) (*CompiledSchema, error) {
	if !validAssembly(assembly) {
		return nil, contract("schema assembly", "unknown assembly")
	}
	kernel, err := xsd.Compile(data, xsd.CompileLimits{})
	if err != nil {
		return nil, &ConfigurationError{Field: "schema", Reason: err.Error()}
	}
	return &CompiledSchema{assembly: assembly, kernel: kernel}, nil
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
	use, ok := s.kernel.Root(SchemaName{Space: name.Space, Local: name.Local})
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
	return checkedPlan(Plan{kind: planXSD, schema: schema, xsdCheck: check})
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
