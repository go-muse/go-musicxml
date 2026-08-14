package xsdgen

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/token"
)

var ErrInvalidDocumentGeneration = errors.New(
	"xsdgen: invalid document generation",
)

// DocumentGenerationOptions selects one global document element and controls
// how declarations from a standalone root schema are named in Go.
//
// ExternalTypes identifies declarations already generated in the destination
// package. They remain available to generated fields but are not emitted.
type DocumentGenerationOptions struct {
	Element       QName
	GoName        string
	TypeNames     []TypeNameOverride
	ExternalTypes []QName
}

// GenerateDocument generates one root element together with the named simple
// and complex types declared by its schema set.
//
// If the root element references a named complex type, that type is flattened
// into the root structure and receives GoName. Recursive references to the XSD
// type therefore use the root Go type without introducing a duplicate wrapper.
func GenerateDocument(
	index *Index,
	packageName string,
	options DocumentGenerationOptions,
) ([]byte, error) {
	if index == nil {
		return nil, ErrNilIndex
	}
	if !token.IsIdentifier(packageName) {
		return nil, fmt.Errorf(
			"%w %q",
			ErrInvalidPackageName,
			packageName,
		)
	}
	if !validNCName(options.Element.Local) ||
		!exportedGoIdentifier(options.GoName) {
		return nil, fmt.Errorf(
			"%w: element %s as %q",
			ErrInvalidDocumentGeneration,
			formatExpandedName(options.Element),
			options.GoName,
		)
	}

	elementDeclaration, found := index.LookupElement(QName{
		Namespace: options.Element.Namespace,
		Local:     options.Element.Local,
	})
	if !found {
		return nil, &UnresolvedReferenceError{
			Space:   ElementSymbolSpace,
			Lexical: options.Element.String(),
			Name:    options.Element,
			From:    "document generation",
		}
	}
	if elementDeclaration.Element == nil ||
		elementDeclaration.File == nil {
		return nil, ErrInvalidElement
	}

	planner := complexTypePlanner{
		index:        index,
		file:         elementDeclaration.File,
		ownerName:    elementDeclaration.Name.Local,
		ownerGoName:  options.GoName,
		expandGroups: true,
	}
	rootType, err := planner.planElementType(
		elementDeclaration.Element,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"xsdgen: plan document element %q: %w",
			elementDeclaration.Name.Local,
			err,
		)
	}
	if rootType == nil {
		return nil, ErrInvalidElement
	}

	overrides, rootDeclaration, err := documentTypeOverrides(
		options,
		rootType,
	)
	if err != nil {
		return nil, err
	}

	names, err := NewTypeNamesWithOverrides(index, overrides...)
	if err != nil {
		return nil, err
	}

	simplePlans, err := planSimpleTypesWithNames(index, names)
	if err != nil {
		return nil, err
	}
	complexPlans, err := planComplexTypesWithNames(
		index,
		names,
		true,
	)
	if err != nil {
		return nil, err
	}

	external, err := documentExternalTypes(
		index,
		options.ExternalTypes,
	)
	if err != nil {
		return nil, err
	}
	if rootDeclaration != nil {
		if _, found := external[rootDeclaration]; found {
			return nil, fmt.Errorf(
				"%w: root type %s cannot be external",
				ErrInvalidDocumentGeneration,
				formatExpandedName(rootDeclaration.Name),
			)
		}
	}

	simpleRenderer := &simpleTypeRenderer{
		names: names,
		plans: make(
			map[*Declaration]*SimpleTypeDefinition,
			len(simplePlans),
		),
		kindStates: make(
			map[*Declaration]resolveState,
			len(simplePlans),
		),
		kinds: make(
			map[*Declaration]GoTypeKind,
			len(simplePlans),
		),
	}
	for planIndex := range simplePlans {
		plan := &simplePlans[planIndex]
		simpleRenderer.plans[plan.Declaration] = plan.Definition
	}

	renderer := complexTypeRenderer{
		index:          index,
		names:          names,
		simpleRenderer: simpleRenderer,
		packageName:    packageName,
		usedTypeNames:  make(map[string]string),
		nameInline:     true,
	}
	for _, declaration := range names.Declarations() {
		name, found := names.Lookup(declaration)
		if !found {
			panic("xsdgen: missing Go name for indexed type")
		}
		renderer.usedTypeNames[name] = describeDeclaration(declaration)
	}

	var body bytes.Buffer
	for planIndex := range simplePlans {
		plan := &simplePlans[planIndex]
		if _, skipped := external[plan.Declaration]; skipped {
			continue
		}
		if err := simpleRenderer.renderPlan(&body, plan); err != nil {
			return nil, err
		}
	}

	var rootDefinition *ComplexTypeDefinition
	for planIndex := range complexPlans {
		plan := &complexPlans[planIndex]
		if plan.Declaration == rootDeclaration {
			rootDefinition = plan.Definition
			continue
		}
		if _, skipped := external[plan.Declaration]; skipped {
			continue
		}
		if err := renderer.renderPlan(&body, plan); err != nil {
			return nil, err
		}
	}

	rootStructure, err := buildDocumentStructure(
		&renderer,
		options.GoName,
		rootType,
		rootDefinition,
	)
	if err != nil {
		return nil, err
	}
	if err := ensureXMLNameAvailable(rootStructure); err != nil {
		return nil, err
	}
	renderer.renderElement(&body, &generatedElement{
		declaration: elementDeclaration,
		goName:      options.GoName,
		structure:   rootStructure,
	})

	for _, inlineType := range renderer.inlineTypes {
		if inlineType == nil || inlineType.structure == nil {
			return nil, ErrInvalidComplexType
		}

		fmt.Fprintf(
			&body,
			"// %s represents an anonymous nested XSD complex type.\n",
			inlineType.name,
		)
		fmt.Fprintf(&body, "type %s ", inlineType.name)
		renderer.renderStructure(&body, inlineType.structure)
		body.WriteString("\n\n")
		renderer.renderValueConstraintMethods(
			&body,
			inlineType.structure,
		)
	}
	for _, choice := range renderer.choices {
		renderer.renderChoice(&body, choice)
	}

	var source bytes.Buffer
	source.WriteString("// Code generated by xsdgen; DO NOT EDIT.\n\n")
	fmt.Fprintf(&source, "package %s\n\n", packageName)
	source.WriteString("import (\n")
	source.WriteString("\t\"encoding/xml\"\n")
	if len(renderer.choices) != 0 {
		source.WriteString("\t\"fmt\"\n")
	}
	source.WriteString(")\n\n")
	source.Write(body.Bytes())

	result, err := format.Source(source.Bytes())
	if err != nil {
		return nil, fmt.Errorf(
			"xsdgen: format generated document: %w",
			err,
		)
	}

	return result, nil
}

func documentTypeOverrides(
	options DocumentGenerationOptions,
	rootType *TypePlan,
) ([]TypeNameOverride, *Declaration, error) {
	result := append(
		[]TypeNameOverride(nil),
		options.TypeNames...,
	)

	if rootType.Declaration == nil {
		if rootType.InlineComplex == nil {
			return nil, nil, fmt.Errorf(
				"%w: root element must have complex content",
				ErrInvalidDocumentGeneration,
			)
		}

		return result, nil, nil
	}

	declaration := rootType.Declaration
	if declaration.Kind != DeclarationComplexType {
		return nil, nil, fmt.Errorf(
			"%w: root type %s is not a complex type",
			ErrInvalidDocumentGeneration,
			describeDeclaration(declaration),
		)
	}

	for _, override := range result {
		if override.Name.Namespace != declaration.Name.Namespace ||
			override.Name.Local != declaration.Name.Local {
			continue
		}
		if override.GoName != options.GoName {
			return nil, nil, fmt.Errorf(
				"%w: root type %s is named %q instead of %q",
				ErrInvalidDocumentGeneration,
				formatExpandedName(declaration.Name),
				override.GoName,
				options.GoName,
			)
		}

		return result, declaration, nil
	}

	result = append(result, TypeNameOverride{
		Name:   declaration.Name,
		GoName: options.GoName,
	})

	return result, declaration, nil
}

func documentExternalTypes(
	index *Index,
	names []QName,
) (map[*Declaration]struct{}, error) {
	result := make(map[*Declaration]struct{}, len(names))

	for _, name := range names {
		declaration, found := index.LookupType(QName{
			Namespace: name.Namespace,
			Local:     name.Local,
		})
		if !found || declaration.Builtin() {
			return nil, &UnresolvedReferenceError{
				Space:   TypeSymbolSpace,
				Lexical: name.String(),
				Name:    name,
				From:    "external document type",
			}
		}
		if _, duplicate := result[declaration]; duplicate {
			return nil, fmt.Errorf(
				"%w: duplicate external type %s",
				ErrInvalidDocumentGeneration,
				formatExpandedName(declaration.Name),
			)
		}

		result[declaration] = struct{}{}
	}

	return result, nil
}

func buildDocumentStructure(
	renderer *complexTypeRenderer,
	owner string,
	rootType *TypePlan,
	rootDefinition *ComplexTypeDefinition,
) (*complexStructure, error) {
	if rootType.Declaration != nil {
		if rootDefinition == nil {
			return nil, ErrInvalidComplexType
		}

		return renderer.buildStructure(owner, rootDefinition)
	}
	if rootType.InlineComplex != nil {
		return renderer.buildStructure(owner, rootType.InlineComplex)
	}

	return nil, fmt.Errorf(
		"%w: root element must have complex content",
		ErrInvalidDocumentGeneration,
	)
}
