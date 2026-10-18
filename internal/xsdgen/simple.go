package xsdgen

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidSimpleType = errors.New("xsdgen: invalid simple type")

// SimpleTypeForm identifies the defining XSD construct of a simple type.
type SimpleTypeForm string

const (
	SimpleTypeRestriction SimpleTypeForm = "restriction"
	SimpleTypeUnion       SimpleTypeForm = "union"
	SimpleTypeList        SimpleTypeForm = "list"
)

// SimpleTypePlan is the resolved, generation-ready representation of a named
// XSD simple type.
type SimpleTypePlan struct {
	Declaration *Declaration
	GoName      string
	Definition  *SimpleTypeDefinition
}

// SimpleTypeDefinition is a resolved named or anonymous XSD simple type.
//
// Restriction uses Base and Enumerations. Union uses Members. List uses Item.
type SimpleTypeDefinition struct {
	Form         SimpleTypeForm
	Source       *SimpleType
	Final        derivationSet
	Base         *SimpleTypeMember
	Enumerations []EnumerationPlan
	Members      []SimpleTypeMember
	Item         *SimpleTypeMember
}

// SimpleTypeMember identifies either a named type declaration or an anonymous
// inline simple type. Exactly one field is set.
type SimpleTypeMember struct {
	Declaration *Declaration
	Inline      *SimpleTypeDefinition
}

// EnumerationPlan preserves an XSD enumeration value and assigns its Go
// constant name.
type EnumerationPlan struct {
	Value  string
	GoName string
	Facet  *Facet
}

// InvalidSimpleTypeError reports a structurally invalid XSD simple type.
type InvalidSimpleTypeError struct {
	Name   string
	From   string
	Reason string
}

func (e *InvalidSimpleTypeError) Error() string {
	return fmt.Sprintf(
		"%v %q from %q: %s",
		ErrInvalidSimpleType,
		e.Name,
		e.From,
		e.Reason,
	)
}

func (e *InvalidSimpleTypeError) Unwrap() error {
	return ErrInvalidSimpleType
}

// PlanSimpleTypes resolves all user-defined simple types in index and returns
// them in deterministic expanded-name order.
func PlanSimpleTypes(index *Index) ([]SimpleTypePlan, error) {
	names, err := NewTypeNames(index)
	if err != nil {
		return nil, err
	}

	return planSimpleTypesWithNames(index, names)
}

func planSimpleTypesWithNames(
	index *Index,
	names *TypeNames,
) ([]SimpleTypePlan, error) {
	if index == nil {
		return nil, ErrNilIndex
	}
	if names == nil {
		return nil, ErrInvalidSimpleType
	}

	result := make([]SimpleTypePlan, 0)

	for _, declaration := range names.Declarations() {
		if declaration.Kind != DeclarationSimpleType {
			continue
		}

		goName, found := names.Lookup(declaration)
		if !found {
			panic("xsdgen: missing Go name for indexed simple type")
		}

		planner := simpleTypePlanner{
			index:            index,
			file:             declaration.File,
			ownerName:        declaration.Name.Local,
			constantPrefix:   goName,
			enumerationNames: make(map[string]string),
			manifestKey:      index.typeManifestKey(declaration),
		}

		definition, err := planner.plan(declaration.SimpleType)
		if err != nil {
			return nil, err
		}

		result = append(result, SimpleTypePlan{
			Declaration: declaration,
			GoName:      goName,
			Definition:  definition,
		})
	}

	return result, nil
}

type simpleTypePlanner struct {
	index            *Index
	file             *SchemaFile
	ownerName        string
	constantPrefix   string
	enumerationNames map[string]string
	// manifestKey, when set, assigns the constants through the index's name
	// manifest. Inline types whose constants are never emitted leave it empty.
	manifestKey string
}

func (p *simpleTypePlanner) plan(
	source *SimpleType,
) (*SimpleTypeDefinition, error) {
	if source == nil {
		return nil, p.invalid("missing simpleType")
	}

	formCount := countTrue(
		source.Restriction != nil,
		source.Union != nil,
		source.List != nil,
	)
	if formCount != 1 {
		return nil, p.invalid(
			"expected exactly one of restriction, union, or list",
		)
	}

	_, fallback := schemaDerivationDefaults(p.file)
	final, err := effectiveDerivations(source.Final, fallback, simpleDerivations, schemaFinals)
	if err != nil {
		return nil, p.invalid(err.Error())
	}
	var definition *SimpleTypeDefinition
	switch {
	case source.Restriction != nil:
		definition, err = p.planRestriction(source)
	case source.Union != nil:
		definition, err = p.planUnion(source)
	default:
		definition, err = p.planList(source)
	}
	if err != nil {
		return nil, err
	}
	definition.Final = final
	return definition, nil
}

func (p *simpleTypePlanner) planRestriction(
	source *SimpleType,
) (*SimpleTypeDefinition, error) {
	restriction := source.Restriction
	base, err := p.planSingleMember(
		restriction.Base,
		restriction.SimpleType,
		"restriction base",
	)
	if err != nil {
		return nil, err
	}

	if err := checkSimpleMemberDerivation(base, derivationRestriction); err != nil {
		return nil, p.invalid(err.Error())
	}

	enumerations := make(
		[]EnumerationPlan,
		len(restriction.Enumerations),
	)

	for index := range restriction.Enumerations {
		facet := &restriction.Enumerations[index]
		goName, err := GoEnumerationName(
			p.constantPrefix,
			facet.Value,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"xsdgen: name enumeration %q of %q: %w",
				facet.Value,
				p.ownerName,
				err,
			)
		}

		if p.manifestKey != "" {
			goName, err = p.index.names.Assign(enumerationKey(p.manifestKey, facet.Value), goName)
			if err != nil {
				return nil, err
			}
		}

		if existing, found := p.enumerationNames[goName]; found {
			return nil, &GoNameCollisionError{
				Name: goName,
				First: fmt.Sprintf(
					"enumeration %q of simple type %q",
					existing,
					p.ownerName,
				),
				Second: fmt.Sprintf(
					"enumeration %q of simple type %q",
					facet.Value,
					p.ownerName,
				),
			}
		}

		p.enumerationNames[goName] = facet.Value
		enumerations[index] = EnumerationPlan{
			Value:  facet.Value,
			GoName: goName,
			Facet:  facet,
		}
	}

	return &SimpleTypeDefinition{
		Form:         SimpleTypeRestriction,
		Source:       source,
		Base:         base,
		Enumerations: enumerations,
	}, nil
}

func (p *simpleTypePlanner) planUnion(
	source *SimpleType,
) (*SimpleTypeDefinition, error) {
	union := source.Union
	lexicalMembers := strings.Fields(union.MemberTypes)
	memberCount := len(lexicalMembers) + len(union.SimpleTypes)
	if memberCount == 0 {
		return nil, p.invalid("union has no member types")
	}

	members := make([]SimpleTypeMember, 0, memberCount)

	for _, lexical := range lexicalMembers {
		member, err := p.planNamedMember(lexical)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
	}

	for index := range union.SimpleTypes {
		inline, err := p.plan(&union.SimpleTypes[index])
		if err != nil {
			return nil, err
		}

		members = append(members, SimpleTypeMember{Inline: inline})
	}

	for index := range members {
		if err := checkSimpleMemberDerivation(&members[index], derivationUnion); err != nil {
			return nil, p.invalid(err.Error())
		}
	}

	return &SimpleTypeDefinition{
		Form:    SimpleTypeUnion,
		Source:  source,
		Members: members,
	}, nil
}

func (p *simpleTypePlanner) planList(
	source *SimpleType,
) (*SimpleTypeDefinition, error) {
	item, err := p.planSingleMember(
		source.List.ItemType,
		source.List.SimpleType,
		"list item type",
	)
	if err != nil {
		return nil, err
	}

	if err := checkSimpleMemberDerivation(item, derivationList); err != nil {
		return nil, p.invalid(err.Error())
	}

	return &SimpleTypeDefinition{
		Form:   SimpleTypeList,
		Source: source,
		Item:   item,
	}, nil
}

func (p *simpleTypePlanner) planSingleMember(
	lexical string,
	inline *SimpleType,
	description string,
) (*SimpleTypeMember, error) {
	count := 0
	if lexical != "" {
		count++
	}
	if inline != nil {
		count++
	}
	if count != 1 {
		return nil, p.invalid(
			"expected exactly one " + description,
		)
	}

	if lexical != "" {
		member, err := p.planNamedMember(lexical)
		if err != nil {
			return nil, err
		}

		return &member, nil
	}

	definition, err := p.plan(inline)
	if err != nil {
		return nil, err
	}

	return &SimpleTypeMember{Inline: definition}, nil
}

func (p *simpleTypePlanner) planNamedMember(
	lexical string,
) (SimpleTypeMember, error) {
	declaration, err := p.index.ResolveType(p.file, lexical)
	if err != nil {
		return SimpleTypeMember{}, err
	}
	if declaration.Kind != DeclarationSimpleType &&
		declaration.Kind != DeclarationBuiltinSimpleType {
		return SimpleTypeMember{}, p.invalid(
			fmt.Sprintf(
				"%q resolves to non-simple %s",
				lexical,
				declaration.Kind,
			),
		)
	}

	return SimpleTypeMember{Declaration: declaration}, nil
}

func (p *simpleTypePlanner) invalid(
	reason string,
) *InvalidSimpleTypeError {
	return &InvalidSimpleTypeError{
		Name:   p.ownerName,
		From:   declarationFilePath(p.file),
		Reason: reason,
	}
}

func countTrue(values ...bool) int {
	result := 0

	for _, value := range values {
		if value {
			result++
		}
	}

	return result
}

func declarationFilePath(file *SchemaFile) string {
	if file == nil {
		return ""
	}

	return file.Path
}
