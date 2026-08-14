package xsdgen

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidGroup          = errors.New("xsdgen: invalid group")
	ErrInvalidAttributeGroup = errors.New("xsdgen: invalid attribute group")
	ErrReferenceCycle        = errors.New("xsdgen: declaration reference cycle")
	ErrMultipleAnyAttributes = errors.New("xsdgen: multiple anyAttribute declarations")
)

// ReferenceCycleError reports a cycle between model groups or attribute
// groups. Path includes the repeated declaration at both ends.
type ReferenceCycleError struct {
	Space SymbolSpace
	Path  []QName
}

func (e *ReferenceCycleError) Error() string {
	path := make([]string, len(e.Path))
	for index, name := range e.Path {
		path[index] = formatExpandedName(name)
	}

	return fmt.Sprintf(
		"%v in %s space: %s",
		ErrReferenceCycle,
		e.Space,
		strings.Join(path, " -> "),
	)
}

func (e *ReferenceCycleError) Unwrap() error {
	return ErrReferenceCycle
}

func (p *complexTypePlanner) expandGroup(
	source *Particle,
	declaration *Declaration,
) (*ParticlePlan, error) {
	if declaration == nil ||
		declaration.Kind != DeclarationGroup ||
		declaration.Group == nil ||
		declaration.File == nil {
		return nil, fmt.Errorf(
			"%w: invalid referenced declaration",
			ErrInvalidGroup,
		)
	}

	if cycle := referenceCycle(p.groupPath, declaration); cycle != nil {
		return nil, &ReferenceCycleError{
			Space: GroupSymbolSpace,
			Path:  cycle,
		}
	}

	group := declaration.Group
	if group.Ref != "" ||
		group.Name == "" ||
		group.MinOccurs != "" ||
		group.MaxOccurs != "" {
		return nil, fmt.Errorf(
			"%w %s from %q: global group must have a name and no ref or occurrence constraints",
			ErrInvalidGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
		)
	}

	count := countTrue(
		group.All != nil,
		group.Choice != nil,
		group.Sequence != nil,
	)
	if count != 1 {
		return nil, fmt.Errorf(
			"%w %s from %q: expected exactly one compositor",
			ErrInvalidGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
		)
	}

	occurrence, err := parseOccurrence(source.Group.Occurs)
	if err != nil {
		return nil, err
	}

	child := *p
	child.file = declaration.File
	child.groupPath = appendDeclaration(
		p.groupPath,
		declaration,
	)

	expanded, err := child.planBodyParticle(complexTypeBody{
		all:      group.All,
		choice:   group.Choice,
		sequence: group.Sequence,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w %s from %q: %w",
			ErrInvalidGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
			err,
		)
	}
	if expanded == nil {
		panic("xsdgen: group compositor produced no particle")
	}
	if expanded.Occurrence != (OccurrenceRange{Min: 1, Max: 1}) {
		return nil, fmt.Errorf(
			"%w %s from %q: global group compositor cannot have minOccurs or maxOccurs",
			ErrInvalidGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
		)
	}

	expanded.Source = source
	expanded.Reference = declaration
	expanded.Occurrence = occurrence

	return expanded, nil
}

func (p *complexTypePlanner) expandAttributeGroups(
	sources []AttributeGroup,
) ([]AttributePlan, *AnyAttribute, error) {
	var attributes []AttributePlan
	var anyAttribute *AnyAttribute

	for index := range sources {
		source := &sources[index]
		declaration, err := p.resolveAttributeGroup(source)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"xsdgen: resolve attribute group at index %d: %w",
				index,
				err,
			)
		}

		groupAttributes, groupAnyAttribute, err :=
			p.expandAttributeGroup(declaration)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"xsdgen: expand attribute group at index %d: %w",
				index,
				err,
			)
		}

		attributes = append(attributes, groupAttributes...)

		anyAttribute, err = mergeAnyAttributes(
			anyAttribute,
			groupAnyAttribute,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	return attributes, anyAttribute, nil
}

func (p *complexTypePlanner) expandAttributeGroup(
	declaration *Declaration,
) ([]AttributePlan, *AnyAttribute, error) {
	if declaration == nil ||
		declaration.Kind != DeclarationAttributeGroup ||
		declaration.AttributeGroup == nil ||
		declaration.File == nil {
		return nil, nil, fmt.Errorf(
			"%w: invalid referenced declaration",
			ErrInvalidAttributeGroup,
		)
	}

	if cycle := referenceCycle(
		p.attributeGroupPath,
		declaration,
	); cycle != nil {
		return nil, nil, &ReferenceCycleError{
			Space: AttributeGroupSymbolSpace,
			Path:  cycle,
		}
	}

	group := declaration.AttributeGroup
	if group.Ref != "" || group.Name == "" {
		return nil, nil, fmt.Errorf(
			"%w %s from %q: global attributeGroup must have a name and no ref",
			ErrInvalidAttributeGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
		)
	}

	child := *p
	child.file = declaration.File
	child.attributeGroupPath = appendDeclaration(
		p.attributeGroupPath,
		declaration,
	)

	attributes, err := child.planAttributes(group.Attributes)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w %s from %q: plan attributes: %w",
			ErrInvalidAttributeGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
			err,
		)
	}

	nestedAttributes, nestedAnyAttribute, err :=
		child.expandAttributeGroups(group.AttributeGroups)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w %s from %q: %w",
			ErrInvalidAttributeGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
			err,
		)
	}

	attributes = append(attributes, nestedAttributes...)

	anyAttribute, err := mergeAnyAttributes(
		group.AnyAttribute,
		nestedAnyAttribute,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w %s from %q: %w",
			ErrInvalidAttributeGroup,
			formatExpandedName(declaration.Name),
			declaration.File.Path,
			err,
		)
	}

	return attributes, anyAttribute, nil
}

func mergeAnyAttributes(
	first *AnyAttribute,
	second *AnyAttribute,
) (*AnyAttribute, error) {
	if first != nil && second != nil {
		return nil, ErrMultipleAnyAttributes
	}
	if first != nil {
		return first, nil
	}

	return second, nil
}

func referenceCycle(
	path []*Declaration,
	next *Declaration,
) []QName {
	for index, declaration := range path {
		if declaration != next {
			continue
		}

		result := make([]QName, 0, len(path)-index+1)
		for _, item := range path[index:] {
			result = append(result, item.Name)
		}
		result = append(result, next.Name)

		return result
	}

	return nil
}

func appendDeclaration(
	path []*Declaration,
	declaration *Declaration,
) []*Declaration {
	result := make([]*Declaration, len(path)+1)
	copy(result, path)
	result[len(path)] = declaration

	return result
}
