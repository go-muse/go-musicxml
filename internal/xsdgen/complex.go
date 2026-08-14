package xsdgen

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrInvalidComplexType = errors.New("xsdgen: invalid complex type")
	ErrInvalidOccurrence  = errors.New("xsdgen: invalid occurrence")
	ErrInvalidParticle    = errors.New("xsdgen: invalid particle")
	ErrInvalidElement     = errors.New("xsdgen: invalid element")
	ErrInvalidAttribute   = errors.New("xsdgen: invalid attribute")
)

// ComplexTypeForm identifies how an XSD complex type is defined.
type ComplexTypeForm string

const (
	ComplexTypeDirect                    ComplexTypeForm = "direct"
	ComplexTypeSimpleContentExtension    ComplexTypeForm = "simple content extension"
	ComplexTypeSimpleContentRestriction  ComplexTypeForm = "simple content restriction"
	ComplexTypeComplexContentExtension   ComplexTypeForm = "complex content extension"
	ComplexTypeComplexContentRestriction ComplexTypeForm = "complex content restriction"
)

// ParticleKind identifies one XSD model-group particle.
type ParticleKind string

const (
	ParticleElement  ParticleKind = "element"
	ParticleGroup    ParticleKind = "group"
	ParticleAll      ParticleKind = "all"
	ParticleChoice   ParticleKind = "choice"
	ParticleSequence ParticleKind = "sequence"
	ParticleAny      ParticleKind = "any"
)

// AttributeUse identifies whether an XSD attribute is optional, required, or
// prohibited.
type AttributeUse string

const (
	AttributeOptional   AttributeUse = "optional"
	AttributeRequired   AttributeUse = "required"
	AttributeProhibited AttributeUse = "prohibited"
)

// ValueConstraintKind identifies an XSD default or fixed value constraint.
type ValueConstraintKind string

const (
	ValueDefault ValueConstraintKind = "default"
	ValueFixed   ValueConstraintKind = "fixed"
)

// ValueConstraint preserves the lexical XSD value of a default or fixed
// attribute constraint.
type ValueConstraint struct {
	Kind  ValueConstraintKind
	Value string
}

// OccurrenceRange is a parsed minOccurs/maxOccurs pair.
//
// Max is zero when Unbounded is true and must otherwise be greater than or
// equal to Min.
type OccurrenceRange struct {
	Min       uint64
	Max       uint64
	Unbounded bool
}

// Required reports whether at least one local occurrence is required.
//
// A containing optional choice or sequence may still make the particle
// optional in the complete content model.
func (o OccurrenceRange) Required() bool {
	return o.Min > 0
}

// Repeated reports whether more than one local occurrence is allowed.
func (o OccurrenceRange) Repeated() bool {
	return o.Unbounded || o.Max > 1
}

// ComplexTypePlan is the resolved, generation-ready representation of a named
// XSD complex type.
type ComplexTypePlan struct {
	Declaration *Declaration
	GoName      string
	Definition  *ComplexTypeDefinition
}

// ComplexTypeDefinition preserves the structure of a named or anonymous XSD
// complex type.
//
// Base is set for simpleContent and complexContent derivations. Particle keeps
// sequence and choice nesting intact instead of flattening ordered content.
type ComplexTypeDefinition struct {
	Source          *ComplexType
	Form            ComplexTypeForm
	Base            *Declaration
	Particle        *ParticlePlan
	Attributes      []AttributePlan
	AttributeGroups []AttributeGroupPlan
	AnyAttribute    *AnyAttribute
	Mixed           bool
}

// TypePlan identifies either a named XSD type or an anonymous inline type.
// Exactly one field is set.
type TypePlan struct {
	Declaration   *Declaration
	InlineSimple  *SimpleTypeDefinition
	InlineComplex *ComplexTypeDefinition
}

// ParticlePlan is a resolved XSD particle. Children are populated for all,
// choice, and sequence particles. Reference is populated for group particles
// and for compositor particles produced by group expansion.
type ParticlePlan struct {
	Source     *Particle
	Kind       ParticleKind
	Occurrence OccurrenceRange
	Element    *ElementPlan
	Reference  *Declaration
	Children   []ParticlePlan
}

// ElementPlan is a resolved local element field.
//
// Reference is set for ref= elements. Type is set for locally declared
// elements.
type ElementPlan struct {
	Source    *Element
	Name      QName
	Reference *Declaration
	Type      *TypePlan
}

// AttributePlan is a resolved attribute field.
type AttributePlan struct {
	Source     *Attribute
	Name       QName
	Reference  *Declaration
	Type       TypePlan
	Use        AttributeUse
	Constraint *ValueConstraint
}

// Required reports whether the attribute has use="required".
func (a AttributePlan) Required() bool {
	return a.Use == AttributeRequired
}

// AttributeGroupPlan preserves and resolves one attributeGroup reference.
type AttributeGroupPlan struct {
	Source    *AttributeGroup
	Reference *Declaration
}

// InvalidComplexTypeError reports a structurally invalid XSD complex type.
type InvalidComplexTypeError struct {
	Name   string
	From   string
	Reason string
}

func (e *InvalidComplexTypeError) Error() string {
	return fmt.Sprintf(
		"%v %q from %q: %s",
		ErrInvalidComplexType,
		e.Name,
		e.From,
		e.Reason,
	)
}

func (e *InvalidComplexTypeError) Unwrap() error {
	return ErrInvalidComplexType
}

// InvalidOccurrenceError reports an invalid minOccurs/maxOccurs pair.
type InvalidOccurrenceError struct {
	MinOccurs string
	MaxOccurs string
	Reason    string
}

func (e *InvalidOccurrenceError) Error() string {
	return fmt.Sprintf(
		"%v minOccurs=%q maxOccurs=%q: %s",
		ErrInvalidOccurrence,
		e.MinOccurs,
		e.MaxOccurs,
		e.Reason,
	)
}

func (e *InvalidOccurrenceError) Unwrap() error {
	return ErrInvalidOccurrence
}

// PlanComplexTypes resolves all user-defined complex types in index and
// returns them in deterministic expanded-name order.
func PlanComplexTypes(index *Index) ([]ComplexTypePlan, error) {
	return planComplexTypes(index, false)
}

// PlanExpandedComplexTypes resolves all user-defined complex types and expands
// every group and attributeGroup reference.
//
// Expanded plans contain no ParticleGroup particles and no AttributeGroups.
// A particle produced from a group reference keeps that declaration in
// ParticlePlan.Reference.
func PlanExpandedComplexTypes(index *Index) ([]ComplexTypePlan, error) {
	return planComplexTypes(index, true)
}

func planComplexTypes(
	index *Index,
	expandGroups bool,
) ([]ComplexTypePlan, error) {
	names, err := NewTypeNames(index)
	if err != nil {
		return nil, err
	}

	return planComplexTypesWithNames(index, names, expandGroups)
}

func planComplexTypesWithNames(
	index *Index,
	names *TypeNames,
	expandGroups bool,
) ([]ComplexTypePlan, error) {
	if index == nil {
		return nil, ErrNilIndex
	}
	if names == nil {
		return nil, ErrInvalidComplexType
	}

	result := make([]ComplexTypePlan, 0)

	for _, declaration := range names.Declarations() {
		if declaration.Kind != DeclarationComplexType {
			continue
		}

		goName, found := names.Lookup(declaration)
		if !found {
			panic("xsdgen: missing Go name for indexed complex type")
		}

		planner := complexTypePlanner{
			index:        index,
			file:         declaration.File,
			ownerName:    declaration.Name.Local,
			ownerGoName:  goName,
			expandGroups: expandGroups,
		}

		definition, err := planner.plan(declaration.ComplexType)
		if err != nil {
			return nil, err
		}

		result = append(result, ComplexTypePlan{
			Declaration: declaration,
			GoName:      goName,
			Definition:  definition,
		})
	}

	return result, nil
}

type complexTypePlanner struct {
	index              *Index
	file               *SchemaFile
	ownerName          string
	ownerGoName        string
	expandGroups       bool
	groupPath          []*Declaration
	attributeGroupPath []*Declaration
}

type complexTypeBody struct {
	group           *Group
	all             *All
	choice          *Choice
	sequence        *Sequence
	attributes      []Attribute
	attributeGroups []AttributeGroup
	anyAttribute    *AnyAttribute
}

func (p *complexTypePlanner) plan(
	source *ComplexType,
) (*ComplexTypeDefinition, error) {
	if source == nil {
		return nil, p.invalid("missing complexType")
	}
	if source.SimpleContent != nil && source.ComplexContent != nil {
		return nil, p.invalid(
			"simpleContent and complexContent are mutually exclusive",
		)
	}
	if (source.SimpleContent != nil || source.ComplexContent != nil) &&
		hasDirectComplexTypeBody(source) {
		return nil, p.invalid(
			"derived content cannot be combined with a direct body",
		)
	}

	form := ComplexTypeDirect
	body := bodyFromComplexType(source)
	var base *Declaration

	switch {
	case source.SimpleContent != nil:
		var lexical string

		switch {
		case source.SimpleContent.Extension != nil &&
			source.SimpleContent.Restriction != nil:
			return nil, p.invalid(
				"simpleContent has both extension and restriction",
			)
		case source.SimpleContent.Extension != nil:
			form = ComplexTypeSimpleContentExtension
			extension := source.SimpleContent.Extension
			lexical = extension.Base
			body = bodyFromExtension(extension)
		case source.SimpleContent.Restriction != nil:
			form = ComplexTypeSimpleContentRestriction
			restriction := source.SimpleContent.Restriction
			lexical = restriction.Base
			body = bodyFromRestriction(restriction)
		default:
			return nil, p.invalid(
				"simpleContent has neither extension nor restriction",
			)
		}

		if hasParticle(body) {
			return nil, p.invalid(
				"simpleContent cannot contain an element particle",
			)
		}

		var err error
		base, err = p.resolveBase(lexical, true)
		if err != nil {
			return nil, err
		}

	case source.ComplexContent != nil:
		var lexical string

		switch {
		case source.ComplexContent.Extension != nil &&
			source.ComplexContent.Restriction != nil:
			return nil, p.invalid(
				"complexContent has both extension and restriction",
			)
		case source.ComplexContent.Extension != nil:
			form = ComplexTypeComplexContentExtension
			extension := source.ComplexContent.Extension
			lexical = extension.Base
			body = bodyFromExtension(extension)
		case source.ComplexContent.Restriction != nil:
			form = ComplexTypeComplexContentRestriction
			restriction := source.ComplexContent.Restriction
			lexical = restriction.Base
			body = bodyFromRestriction(restriction)
		default:
			return nil, p.invalid(
				"complexContent has neither extension nor restriction",
			)
		}

		var err error
		base, err = p.resolveBase(lexical, false)
		if err != nil {
			return nil, err
		}
	}

	particle, err := p.planBodyParticle(body)
	if err != nil {
		return nil, p.wrap("plan content particle", err)
	}

	attributes, err := p.planAttributes(body.attributes)
	if err != nil {
		return nil, p.wrap("plan attributes", err)
	}

	attributeGroups := []AttributeGroupPlan(nil)
	anyAttribute := body.anyAttribute

	if p.expandGroups {
		expandedAttributes, expandedAnyAttribute, err :=
			p.expandAttributeGroups(body.attributeGroups)
		if err != nil {
			return nil, p.wrap("expand attribute groups", err)
		}

		attributes = append(attributes, expandedAttributes...)

		anyAttribute, err = mergeAnyAttributes(
			anyAttribute,
			expandedAnyAttribute,
		)
		if err != nil {
			return nil, p.wrap("merge anyAttribute", err)
		}
	} else {
		attributeGroups, err = p.planAttributeGroups(
			body.attributeGroups,
		)
		if err != nil {
			return nil, p.wrap("plan attribute groups", err)
		}
	}

	mixed := source.Mixed
	if source.ComplexContent != nil {
		mixed = mixed || source.ComplexContent.Mixed
	}

	return &ComplexTypeDefinition{
		Source:          source,
		Form:            form,
		Base:            base,
		Particle:        particle,
		Attributes:      attributes,
		AttributeGroups: attributeGroups,
		AnyAttribute:    anyAttribute,
		Mixed:           mixed,
	}, nil
}

func (p *complexTypePlanner) resolveBase(
	lexical string,
	simpleContent bool,
) (*Declaration, error) {
	if lexical == "" {
		return nil, p.invalid("missing derivation base")
	}

	declaration, err := p.index.ResolveType(p.file, lexical)
	if err != nil {
		return nil, p.wrap(
			fmt.Sprintf("resolve derivation base %q", lexical),
			err,
		)
	}

	if simpleContent {
		switch declaration.Kind {
		case DeclarationBuiltinSimpleType,
			DeclarationSimpleType,
			DeclarationComplexType:
			return declaration, nil
		default:
			return nil, p.invalid(
				fmt.Sprintf(
					"simpleContent base %q resolves to %s",
					lexical,
					declaration.Kind,
				),
			)
		}
	}

	if declaration.Kind != DeclarationComplexType &&
		declaration.Kind != DeclarationBuiltinComplexType {
		return nil, p.invalid(
			fmt.Sprintf(
				"complexContent base %q resolves to %s",
				lexical,
				declaration.Kind,
			),
		)
	}

	return declaration, nil
}

func (p *complexTypePlanner) planBodyParticle(
	body complexTypeBody,
) (*ParticlePlan, error) {
	count := countTrue(
		body.group != nil,
		body.all != nil,
		body.choice != nil,
		body.sequence != nil,
	)
	if count == 0 {
		return nil, nil
	}
	if count != 1 {
		return nil, fmt.Errorf(
			"%w: expected at most one top-level model group",
			ErrInvalidParticle,
		)
	}

	source := &Particle{
		Group:    body.group,
		All:      body.all,
		Choice:   body.choice,
		Sequence: body.sequence,
	}

	return p.planParticle(source)
}

func (p *complexTypePlanner) planParticle(
	source *Particle,
) (*ParticlePlan, error) {
	if source == nil {
		return nil, fmt.Errorf(
			"%w: nil particle",
			ErrInvalidParticle,
		)
	}

	count := countTrue(
		source.Element != nil,
		source.Group != nil,
		source.All != nil,
		source.Choice != nil,
		source.Sequence != nil,
		source.Any != nil,
	)
	if count != 1 {
		return nil, fmt.Errorf(
			"%w: expected exactly one particle kind",
			ErrInvalidParticle,
		)
	}

	result := &ParticlePlan{Source: source}
	var occurs Occurs

	switch {
	case source.Element != nil:
		result.Kind = ParticleElement
		occurs = source.Element.Occurs

		element, err := p.planElement(source.Element)
		if err != nil {
			return nil, err
		}
		result.Element = element

	case source.Group != nil:
		if source.Group.Ref == "" ||
			source.Group.Name != "" ||
			source.Group.All != nil ||
			source.Group.Choice != nil ||
			source.Group.Sequence != nil {
			return nil, fmt.Errorf(
				"%w: local group must contain only ref",
				ErrInvalidParticle,
			)
		}

		declaration, err := p.index.ResolveGroup(
			p.file,
			source.Group.Ref,
		)
		if err != nil {
			return nil, err
		}

		if p.expandGroups {
			return p.expandGroup(source, declaration)
		}

		result.Kind = ParticleGroup
		occurs = source.Group.Occurs
		result.Reference = declaration

	case source.All != nil:
		result.Kind = ParticleAll
		occurs = source.All.Occurs

		children, err := p.planParticles(source.All.Particles)
		if err != nil {
			return nil, err
		}
		result.Children = children

	case source.Choice != nil:
		result.Kind = ParticleChoice
		occurs = source.Choice.Occurs

		children, err := p.planParticles(source.Choice.Particles)
		if err != nil {
			return nil, err
		}
		result.Children = children

	case source.Sequence != nil:
		result.Kind = ParticleSequence
		occurs = source.Sequence.Occurs

		children, err := p.planParticles(source.Sequence.Particles)
		if err != nil {
			return nil, err
		}
		result.Children = children

	case source.Any != nil:
		result.Kind = ParticleAny
		occurs = source.Any.Occurs
	}

	occurrence, err := parseOccurrence(occurs)
	if err != nil {
		return nil, err
	}
	result.Occurrence = occurrence

	return result, nil
}

func (p *complexTypePlanner) planParticles(
	sources []Particle,
) ([]ParticlePlan, error) {
	result := make([]ParticlePlan, len(sources))

	for index := range sources {
		particle, err := p.planParticle(&sources[index])
		if err != nil {
			return nil, fmt.Errorf(
				"xsdgen: plan particle at index %d: %w",
				index,
				err,
			)
		}

		result[index] = *particle
	}

	return result, nil
}

func (p *complexTypePlanner) planElement(
	source *Element,
) (*ElementPlan, error) {
	if source.Ref != "" {
		if source.Name != "" ||
			source.Type != "" ||
			source.SimpleType != nil ||
			source.ComplexType != nil {
			return nil, fmt.Errorf(
				"%w: ref cannot be combined with name or type",
				ErrInvalidElement,
			)
		}

		declaration, err := p.index.ResolveElement(p.file, source.Ref)
		if err != nil {
			return nil, err
		}

		return &ElementPlan{
			Source:    source,
			Name:      declaration.Name,
			Reference: declaration,
		}, nil
	}

	if !validNCName(source.Name) {
		return nil, fmt.Errorf(
			"%w: invalid local name %q",
			ErrInvalidElement,
			source.Name,
		)
	}

	name, err := localDeclarationName(
		p.file.Schema,
		source.Name,
		source.Form,
		p.file.Schema.ElementFormDefault,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidElement, err)
	}

	typePlan, err := p.planElementType(source)
	if err != nil {
		return nil, err
	}

	return &ElementPlan{
		Source: source,
		Name:   name,
		Type:   typePlan,
	}, nil
}

func (p *complexTypePlanner) planElementType(
	source *Element,
) (*TypePlan, error) {
	count := countTrue(
		source.Type != "",
		source.SimpleType != nil,
		source.ComplexType != nil,
	)
	if count > 1 {
		return nil, fmt.Errorf(
			"%w: expected at most one element type",
			ErrInvalidElement,
		)
	}

	prefix, err := p.inlineTypePrefix(source.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidElement, err)
	}

	switch {
	case source.Type != "":
		declaration, err := p.index.ResolveType(p.file, source.Type)
		if err != nil {
			return nil, err
		}

		return &TypePlan{Declaration: declaration}, nil

	case source.SimpleType != nil:
		definition, err := p.planInlineSimpleType(
			source.SimpleType,
			prefix,
		)
		if err != nil {
			return nil, err
		}

		return &TypePlan{InlineSimple: definition}, nil

	case source.ComplexType != nil:
		child := *p
		child.ownerName = p.ownerName + "." + source.Name
		child.ownerGoName = prefix

		definition, err := child.plan(source.ComplexType)
		if err != nil {
			return nil, err
		}

		return &TypePlan{InlineComplex: definition}, nil

	default:
		declaration, found := p.index.LookupType(QName{
			Namespace: Namespace,
			Local:     "anyType",
		})
		if !found {
			panic("xsdgen: missing built-in anyType")
		}

		return &TypePlan{Declaration: declaration}, nil
	}
}

func (p *complexTypePlanner) planAttributes(
	sources []Attribute,
) ([]AttributePlan, error) {
	result := make([]AttributePlan, len(sources))

	for index := range sources {
		attribute, err := p.planAttribute(&sources[index])
		if err != nil {
			return nil, fmt.Errorf(
				"xsdgen: plan attribute at index %d: %w",
				index,
				err,
			)
		}

		result[index] = *attribute
	}

	return result, nil
}

func (p *complexTypePlanner) planAttribute(
	source *Attribute,
) (*AttributePlan, error) {
	use, err := parseAttributeUse(source.Use)
	if err != nil {
		return nil, err
	}

	localConstraint, err := parseValueConstraint(
		source.Default,
		source.Fixed,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidAttribute, err)
	}

	if source.Ref != "" {
		if source.Name != "" ||
			source.Type != "" ||
			source.SimpleType != nil {
			return nil, fmt.Errorf(
				"%w: ref cannot be combined with name or type",
				ErrInvalidAttribute,
			)
		}

		declaration, err := p.index.ResolveAttribute(
			p.file,
			source.Ref,
		)
		if err != nil {
			return nil, err
		}

		typePlan, err := p.planReferencedAttributeType(declaration)
		if err != nil {
			return nil, err
		}

		inheritedConstraint, err := parseValueConstraint(
			declaration.Attribute.Default,
			declaration.Attribute.Fixed,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: referenced attribute %s: %w",
				ErrInvalidAttribute,
				formatExpandedName(declaration.Name),
				err,
			)
		}

		constraint, err := mergeValueConstraints(
			localConstraint,
			inheritedConstraint,
		)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidAttribute, err)
		}
		constraint, err = validateAttributeConstraint(use, constraint)
		if err != nil {
			return nil, err
		}

		return &AttributePlan{
			Source:     source,
			Name:       declaration.Name,
			Reference:  declaration,
			Type:       typePlan,
			Use:        use,
			Constraint: constraint,
		}, nil
	}

	if !validNCName(source.Name) {
		return nil, fmt.Errorf(
			"%w: invalid local name %q",
			ErrInvalidAttribute,
			source.Name,
		)
	}

	name, err := localDeclarationName(
		p.file.Schema,
		source.Name,
		source.Form,
		p.file.Schema.AttributeFormDefault,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidAttribute, err)
	}

	prefix, err := p.inlineTypePrefix(source.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidAttribute, err)
	}

	typePlan, err := p.planAttributeType(
		p.file,
		source,
		prefix,
	)
	if err != nil {
		return nil, err
	}

	constraint, err := validateAttributeConstraint(
		use,
		localConstraint,
	)
	if err != nil {
		return nil, err
	}

	return &AttributePlan{
		Source:     source,
		Name:       name,
		Type:       typePlan,
		Use:        use,
		Constraint: constraint,
	}, nil
}

func (p *complexTypePlanner) planReferencedAttributeType(
	declaration *Declaration,
) (TypePlan, error) {
	if declaration == nil ||
		declaration.Kind != DeclarationAttribute ||
		declaration.Attribute == nil ||
		declaration.File == nil {
		return TypePlan{}, fmt.Errorf(
			"%w: invalid referenced declaration",
			ErrInvalidAttribute,
		)
	}

	prefix, err := p.inlineTypePrefix(declaration.Name.Local)
	if err != nil {
		return TypePlan{}, fmt.Errorf("%w: %w", ErrInvalidAttribute, err)
	}

	return p.planAttributeType(
		declaration.File,
		declaration.Attribute,
		prefix,
	)
}

func (p *complexTypePlanner) planAttributeType(
	file *SchemaFile,
	source *Attribute,
	inlinePrefix string,
) (TypePlan, error) {
	if source.Type != "" && source.SimpleType != nil {
		return TypePlan{}, fmt.Errorf(
			"%w: expected at most one attribute type",
			ErrInvalidAttribute,
		)
	}

	if source.SimpleType != nil {
		planner := *p
		planner.file = file

		definition, err := planner.planInlineSimpleType(
			source.SimpleType,
			inlinePrefix,
		)
		if err != nil {
			return TypePlan{}, err
		}

		return TypePlan{InlineSimple: definition}, nil
	}

	if source.Type == "" {
		declaration, found := p.index.LookupType(QName{
			Namespace: Namespace,
			Local:     "anySimpleType",
		})
		if !found {
			panic("xsdgen: missing built-in anySimpleType")
		}

		return TypePlan{Declaration: declaration}, nil
	}

	declaration, err := p.index.ResolveType(file, source.Type)
	if err != nil {
		return TypePlan{}, err
	}
	if declaration.Kind != DeclarationBuiltinSimpleType &&
		declaration.Kind != DeclarationSimpleType {
		return TypePlan{}, fmt.Errorf(
			"%w: type %q resolves to %s",
			ErrInvalidAttribute,
			source.Type,
			declaration.Kind,
		)
	}

	return TypePlan{Declaration: declaration}, nil
}

func (p *complexTypePlanner) planInlineSimpleType(
	source *SimpleType,
	constantPrefix string,
) (*SimpleTypeDefinition, error) {
	planner := simpleTypePlanner{
		index:            p.index,
		file:             p.file,
		ownerName:        p.ownerName,
		constantPrefix:   constantPrefix,
		enumerationNames: make(map[string]string),
	}

	return planner.plan(source)
}

func (p *complexTypePlanner) planAttributeGroups(
	sources []AttributeGroup,
) ([]AttributeGroupPlan, error) {
	result := make([]AttributeGroupPlan, len(sources))

	for index := range sources {
		source := &sources[index]
		declaration, err := p.resolveAttributeGroup(source)
		if err != nil {
			return nil, err
		}

		result[index] = AttributeGroupPlan{
			Source:    source,
			Reference: declaration,
		}
	}

	return result, nil
}

func (p *complexTypePlanner) resolveAttributeGroup(
	source *AttributeGroup,
) (*Declaration, error) {
	if source == nil ||
		source.Ref == "" ||
		source.Name != "" ||
		len(source.Attributes) != 0 ||
		len(source.AttributeGroups) != 0 ||
		source.AnyAttribute != nil {
		return nil, fmt.Errorf(
			"%w: local attributeGroup must contain only ref",
			ErrInvalidAttribute,
		)
	}

	return p.index.ResolveAttributeGroup(p.file, source.Ref)
}

func (p *complexTypePlanner) inlineTypePrefix(
	localName string,
) (string, error) {
	suffix, err := GoTypeName(localName)
	if err != nil {
		return "", err
	}

	return p.ownerGoName + suffix, nil
}

func (p *complexTypePlanner) invalid(
	reason string,
) *InvalidComplexTypeError {
	return &InvalidComplexTypeError{
		Name:   p.ownerName,
		From:   declarationFilePath(p.file),
		Reason: reason,
	}
}

func (p *complexTypePlanner) wrap(
	reason string,
	cause error,
) error {
	return fmt.Errorf("%w: %w", p.invalid(reason), cause)
}

func parseOccurrence(source Occurs) (OccurrenceRange, error) {
	minimum, err := parseOccurrenceValue(source.MinOccurs, 1)
	if err != nil {
		return OccurrenceRange{}, &InvalidOccurrenceError{
			MinOccurs: source.MinOccurs,
			MaxOccurs: source.MaxOccurs,
			Reason:    "minOccurs must be a non-negative integer",
		}
	}

	if source.MaxOccurs == "unbounded" {
		return OccurrenceRange{
			Min:       minimum,
			Unbounded: true,
		}, nil
	}

	maximum, err := parseOccurrenceValue(source.MaxOccurs, 1)
	if err != nil {
		return OccurrenceRange{}, &InvalidOccurrenceError{
			MinOccurs: source.MinOccurs,
			MaxOccurs: source.MaxOccurs,
			Reason:    "maxOccurs must be a non-negative integer or unbounded",
		}
	}
	if maximum < minimum {
		return OccurrenceRange{}, &InvalidOccurrenceError{
			MinOccurs: source.MinOccurs,
			MaxOccurs: source.MaxOccurs,
			Reason:    "maxOccurs is less than minOccurs",
		}
	}

	return OccurrenceRange{
		Min: minimum,
		Max: maximum,
	}, nil
}

func parseOccurrenceValue(
	value string,
	defaultValue uint64,
) (uint64, error) {
	if value == "" {
		return defaultValue, nil
	}

	return strconv.ParseUint(value, 10, 64)
}

func parseAttributeUse(value string) (AttributeUse, error) {
	switch value {
	case "", string(AttributeOptional):
		return AttributeOptional, nil
	case string(AttributeRequired):
		return AttributeRequired, nil
	case string(AttributeProhibited):
		return AttributeProhibited, nil
	default:
		return "", fmt.Errorf(
			"%w: unsupported use %q",
			ErrInvalidAttribute,
			value,
		)
	}
}

func parseValueConstraint(
	defaultValue *string,
	fixedValue *string,
) (*ValueConstraint, error) {
	if defaultValue != nil && fixedValue != nil {
		return nil, errors.New(
			"default and fixed are mutually exclusive",
		)
	}

	switch {
	case defaultValue != nil:
		return &ValueConstraint{
			Kind:  ValueDefault,
			Value: *defaultValue,
		}, nil

	case fixedValue != nil:
		return &ValueConstraint{
			Kind:  ValueFixed,
			Value: *fixedValue,
		}, nil

	default:
		return nil, nil
	}
}

func mergeValueConstraints(
	local *ValueConstraint,
	inherited *ValueConstraint,
) (*ValueConstraint, error) {
	if local == nil {
		return inherited, nil
	}
	if inherited == nil {
		return local, nil
	}

	if inherited.Kind == ValueFixed &&
		(local.Kind != ValueFixed || local.Value != inherited.Value) {
		return nil, fmt.Errorf(
			"constraint %s=%q conflicts with inherited fixed=%q",
			local.Kind,
			local.Value,
			inherited.Value,
		)
	}

	return local, nil
}

func validateAttributeConstraint(
	use AttributeUse,
	constraint *ValueConstraint,
) (*ValueConstraint, error) {
	if constraint == nil {
		return nil, nil
	}

	switch use {
	case AttributeOptional:
		return constraint, nil

	case AttributeRequired:
		if constraint.Kind == ValueDefault {
			return nil, fmt.Errorf(
				"%w: required attribute cannot have a default value",
				ErrInvalidAttribute,
			)
		}

		return constraint, nil

	case AttributeProhibited:
		return nil, fmt.Errorf(
			"%w: prohibited attribute cannot have a value constraint",
			ErrInvalidAttribute,
		)

	default:
		return nil, fmt.Errorf(
			"%w: unsupported use %q",
			ErrInvalidAttribute,
			use,
		)
	}
}

func localDeclarationName(
	schema *Schema,
	local string,
	form string,
	defaultForm string,
) (QName, error) {
	if schema == nil {
		return QName{}, ErrInvalidSchemaFile
	}

	qualified := false
	switch form {
	case "":
		switch defaultForm {
		case "", "unqualified":
		case "qualified":
			qualified = true
		default:
			return QName{}, fmt.Errorf(
				"invalid default form %q",
				defaultForm,
			)
		}
	case "unqualified":
	case "qualified":
		qualified = true
	default:
		return QName{}, fmt.Errorf("invalid form %q", form)
	}

	name := QName{Local: local}
	if qualified {
		name.Namespace = schema.TargetNamespace
	}

	return name, nil
}

func bodyFromComplexType(source *ComplexType) complexTypeBody {
	return complexTypeBody{
		group:           source.Group,
		all:             source.All,
		choice:          source.Choice,
		sequence:        source.Sequence,
		attributes:      source.Attributes,
		attributeGroups: source.AttributeGroups,
		anyAttribute:    source.AnyAttribute,
	}
}

func bodyFromExtension(source *Extension) complexTypeBody {
	return complexTypeBody{
		group:           source.Group,
		all:             source.All,
		choice:          source.Choice,
		sequence:        source.Sequence,
		attributes:      source.Attributes,
		attributeGroups: source.AttributeGroups,
		anyAttribute:    source.AnyAttribute,
	}
}

func bodyFromRestriction(source *Restriction) complexTypeBody {
	return complexTypeBody{
		group:           source.Group,
		all:             source.All,
		choice:          source.Choice,
		sequence:        source.Sequence,
		attributes:      source.Attributes,
		attributeGroups: source.AttributeGroups,
		anyAttribute:    source.AnyAttribute,
	}
}

func hasDirectComplexTypeBody(source *ComplexType) bool {
	return source.Group != nil ||
		source.All != nil ||
		source.Choice != nil ||
		source.Sequence != nil ||
		len(source.Attributes) != 0 ||
		len(source.AttributeGroups) != 0 ||
		source.AnyAttribute != nil
}

func hasParticle(body complexTypeBody) bool {
	return body.group != nil ||
		body.all != nil ||
		body.choice != nil ||
		body.sequence != nil
}
