package xsdgen

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"strconv"
)

type complexChoice struct {
	owner          string
	goType         string
	listGoType     string
	variants       []complexChoiceVariant
	variantIndexes map[expandedName]int
}

type complexChoiceVariant struct {
	name        QName
	goName      string
	goType      string
	description string
}

func (r *complexTypeRenderer) planRepeatedChoices(
	structure *complexStructure,
	particle *ParticlePlan,
	orderedContent bool,
) error {
	if orderedContent {
		if particle == nil {
			return fmt.Errorf(
				"%w: %s has no particle for ordered content",
				ErrUnsupportedComplexGeneration,
				structure.owner,
			)
		}

		choice, err := r.buildChoice(
			structure.owner,
			particle,
			false,
		)
		if err != nil {
			return err
		}
		return registerComplexChoice(
			structure,
			particle,
			choice,
		)
	}

	var particles []*ParticlePlan
	collectOrderedParticles(particle, &particles)

	if len(particles) > 1 {
		return fmt.Errorf(
			"%w: %s contains %d independently ordered repeated particles; configure the type as ordered content",
			ErrUnsupportedComplexGeneration,
			structure.owner,
			len(particles),
		)
	}
	if len(particles) == 0 {
		return nil
	}

	choice, err := r.buildChoice(
		structure.owner,
		particles[0],
		false,
	)
	if err != nil {
		return err
	}

	return registerComplexChoice(
		structure,
		particles[0],
		choice,
	)
}

func registerComplexChoice(
	structure *complexStructure,
	particle *ParticlePlan,
	choice *complexChoice,
) error {
	structure.choices[particle] = choice
	for key := range choice.variantIndexes {
		if existing, found := structure.choiceByElement[key]; found {
			return fmt.Errorf(
				"%w: %s and %s both contain element {%s}%s",
				ErrUnsupportedComplexGeneration,
				existing.goType,
				choice.goType,
				key.namespace,
				key.local,
			)
		}
		structure.choiceByElement[key] = choice
	}

	return nil
}

func collectOrderedParticles(
	particle *ParticlePlan,
	result *[]*ParticlePlan,
) {
	if particle == nil {
		return
	}
	if particle.Kind != ParticleElement &&
		particle.Occurrence.Repeated() &&
		particleHasMultipleElementNames(particle) {
		*result = append(*result, particle)
		return
	}

	for childIndex := range particle.Children {
		collectOrderedParticles(
			&particle.Children[childIndex],
			result,
		)
	}
}

func particleHasMultipleElementNames(particle *ParticlePlan) bool {
	var elements []*ElementPlan
	if err := collectChoiceElements(particle, &elements); err != nil {
		return false
	}

	names := make(map[expandedName]struct{})
	for _, element := range elements {
		if element == nil {
			continue
		}
		names[expandedName{
			namespace: element.Name.Namespace,
			local:     element.Name.Local,
		}] = struct{}{}
		if len(names) > 1 {
			return true
		}
	}

	return false
}

func (r *complexTypeRenderer) buildChoice(
	owner string,
	particle *ParticlePlan,
	requireRepeatedChoice bool,
) (*complexChoice, error) {
	if particle == nil {
		return nil, ErrInvalidParticle
	}
	if requireRepeatedChoice &&
		(particle.Kind != ParticleChoice ||
			!particle.Occurrence.Repeated()) {
		return nil, ErrInvalidParticle
	}

	goType := owner + "Content"
	description := "ordered content for " + owner
	if err := r.reserveTypeName(goType, description); err != nil {
		return nil, err
	}
	listGoType := owner + "Contents"
	if err := r.reserveTypeName(
		listGoType,
		"ordered content list for "+owner,
	); err != nil {
		return nil, err
	}

	var elements []*ElementPlan
	for childIndex := range particle.Children {
		if err := collectChoiceElements(
			&particle.Children[childIndex],
			&elements,
		); err != nil {
			return nil, fmt.Errorf(
				"%w: build %s: %w",
				ErrUnsupportedComplexGeneration,
				goType,
				err,
			)
		}
	}
	if len(elements) == 0 {
		return nil, fmt.Errorf(
			"%w: %s has no element alternatives",
			ErrUnsupportedComplexGeneration,
			goType,
		)
	}

	result := &complexChoice{
		owner:          owner,
		goType:         goType,
		listGoType:     listGoType,
		variantIndexes: make(map[expandedName]int),
	}
	usedFieldNames := make(map[string]string)

	for _, element := range elements {
		if element == nil {
			return nil, ErrInvalidElement
		}

		goName, err := GoTypeName(element.Name.Local)
		if err != nil {
			return nil, err
		}
		goTypeValue, err := r.elementGoType(element, goType)
		if err != nil {
			return nil, fmt.Errorf(
				"xsdgen: resolve ordered element %q type: %w",
				element.Name.Local,
				err,
			)
		}

		description := "element " + formatExpandedName(element.Name)
		key := expandedName{
			namespace: element.Name.Namespace,
			local:     element.Name.Local,
		}
		if existingIndex, found := result.variantIndexes[key]; found {
			existing := &result.variants[existingIndex]
			if existing.goType != goTypeValue {
				return nil, &GoFieldNameCollisionError{
					Owner:  goType,
					Name:   goName,
					First:  existing.description + " of type " + existing.goType,
					Second: description + " of type " + goTypeValue,
				}
			}

			continue
		}
		if existing, found := usedFieldNames[goName]; found {
			return nil, &GoFieldNameCollisionError{
				Owner:  goType,
				Name:   goName,
				First:  existing,
				Second: description,
			}
		}

		usedFieldNames[goName] = description
		result.variantIndexes[key] = len(result.variants)
		result.variants = append(
			result.variants,
			complexChoiceVariant{
				name:        element.Name,
				goName:      goName,
				goType:      goTypeValue,
				description: description,
			},
		)
	}

	r.choices = append(r.choices, result)

	return result, nil
}

func collectChoiceElements(
	particle *ParticlePlan,
	result *[]*ElementPlan,
) error {
	if particle == nil {
		return ErrInvalidParticle
	}

	switch particle.Kind {
	case ParticleElement:
		if particle.Element == nil {
			return ErrInvalidElement
		}
		*result = append(*result, particle.Element)

		return nil

	case ParticleAll, ParticleChoice, ParticleSequence:
		for childIndex := range particle.Children {
			if err := collectChoiceElements(
				&particle.Children[childIndex],
				result,
			); err != nil {
				return err
			}
		}

		return nil

	case ParticleGroup:
		return errors.New("unexpanded group")

	case ParticleAny:
		return errors.New("xs:any alternative")

	default:
		return fmt.Errorf(
			"%w: unknown particle kind %q",
			ErrInvalidParticle,
			particle.Kind,
		)
	}
}

func (r *complexTypeRenderer) reserveTypeName(
	name string,
	description string,
) error {
	if !token.IsIdentifier(name) {
		return fmt.Errorf("%w %q", ErrInvalidGoName, name)
	}
	if existing, found := r.usedTypeNames[name]; found {
		return &GoNameCollisionError{
			Name:   name,
			First:  existing,
			Second: description,
		}
	}

	r.usedTypeNames[name] = description
	return nil
}

func (s *complexStructure) addChoice(
	choice *complexChoice,
) error {
	if choice == nil {
		return ErrInvalidParticle
	}
	if s.insertedChoices[choice] {
		return nil
	}

	s.choiceIndexes[choice] = len(s.elements)
	s.elements = append(s.elements, complexField{
		kind:        complexFieldChoice,
		goType:      choice.goType,
		description: "ordered content " + choice.listGoType,
		choice:      choice,
	})
	s.insertedChoices[choice] = true

	return nil
}

func (r *complexTypeRenderer) renderChoice(
	target *bytes.Buffer,
	choice *complexChoice,
) {
	fmt.Fprintf(
		target,
		"// %s stores the recognized ordered child elements of %s.\n",
		choice.listGoType,
		choice.owner,
	)
	fmt.Fprintf(
		target,
		"type %s []%s\n\n",
		choice.listGoType,
		choice.goType,
	)

	fmt.Fprintf(
		target,
		"// UnmarshalXML decodes and appends one recognized %s child.\n",
		choice.owner,
	)
	fmt.Fprintf(
		target,
		"func (values *%s) UnmarshalXML(\n",
		choice.listGoType,
	)
	target.WriteString("\tdecoder *xml.Decoder,\n")
	target.WriteString("\tstart xml.StartElement,\n")
	target.WriteString(") error {\n")
	fmt.Fprintf(target, "\tvar decoded %s\n", choice.goType)
	target.WriteString(
		"\tif err := decoded.UnmarshalXML(decoder, start); err != nil {\n",
	)
	target.WriteString("\t\treturn err\n")
	target.WriteString("\t}\n")
	fmt.Fprintf(
		target,
		"\tif decoded == (%s{}) {\n",
		choice.goType,
	)
	target.WriteString("\t\treturn nil\n")
	target.WriteString("\t}\n")
	target.WriteString("\t*values = append(*values, decoded)\n")
	target.WriteString("\treturn nil\n")
	target.WriteString("}\n\n")

	fmt.Fprintf(
		target,
		"// %s is one ordered child element of %s.\n",
		choice.goType,
		choice.owner,
	)
	fmt.Fprintf(target, "type %s struct {\n", choice.goType)
	for _, variant := range choice.variants {
		fmt.Fprintf(
			target,
			"\t%s *%s\n",
			variant.goName,
			variant.goType,
		)
	}
	target.WriteString("}\n\n")

	for _, variant := range choice.variants {
		fmt.Fprintf(
			target,
			"// Add%s appends the %q child while preserving content order.\n",
			variant.goName,
			variant.name.Local,
		)
		fmt.Fprintf(
			target,
			"func (value *%s) Add%s(element *%s) *%s {\n",
			choice.owner,
			variant.goName,
			variant.goType,
			variant.goType,
		)
		fmt.Fprintf(
			target,
			"\tvalue.Content = append(value.Content, %s{%s: element})\n",
			choice.goType,
			variant.goName,
		)
		target.WriteString("\treturn element\n")
		target.WriteString("}\n\n")
	}

	fmt.Fprintf(
		target,
		"// UnmarshalXML decodes one %s variant.\n",
		choice.goType,
	)
	fmt.Fprintf(
		target,
		"func (value *%s) UnmarshalXML(\n",
		choice.goType,
	)
	target.WriteString("\tdecoder *xml.Decoder,\n")
	target.WriteString("\tstart xml.StartElement,\n")
	target.WriteString(") error {\n")
	fmt.Fprintf(target, "\t*value = %s{}\n\n", choice.goType)
	target.WriteString("\tswitch start.Name {\n")
	for _, variant := range choice.variants {
		fmt.Fprintf(
			target,
			"\tcase (%s):\n",
			xmlNameExpression(variant.name),
		)
		fmt.Fprintf(
			target,
			"\t\tvar decoded %s\n",
			variant.goType,
		)
		target.WriteString(
			"\t\tif err := decoder.DecodeElement(&decoded, &start); err != nil {\n",
		)
		fmt.Fprintf(
			target,
			"\t\t\treturn fmt.Errorf(%s, err)\n",
			strconv.Quote(fmt.Sprintf(
				"%s: decode %s.%s: %%w",
				r.packageName,
				choice.goType,
				variant.goName,
			)),
		)
		target.WriteString("\t\t}\n")
		fmt.Fprintf(
			target,
			"\t\tvalue.%s = &decoded\n",
			variant.goName,
		)
		target.WriteString("\t\treturn nil\n\n")
	}
	target.WriteString("\tdefault:\n")
	fmt.Fprintf(
		target,
		"\t\tif err := decoder.Skip(); err != nil {\n"+
			"\t\t\treturn fmt.Errorf(%s, err)\n"+
			"\t\t}\n",
		strconv.Quote(fmt.Sprintf(
			"%s: skip unsupported %s element: %%w",
			r.packageName,
			choice.goType,
		)),
	)
	target.WriteString("\t\treturn nil\n")
	target.WriteString("\t}\n")
	target.WriteString("}\n\n")

	fmt.Fprintf(
		target,
		"// MarshalXML encodes the selected %s variant.\n",
		choice.goType,
	)
	fmt.Fprintf(
		target,
		"func (value %s) MarshalXML(\n",
		choice.goType,
	)
	target.WriteString("\tencoder *xml.Encoder,\n")
	target.WriteString("\t_ xml.StartElement,\n")
	target.WriteString(") error {\n")
	target.WriteString("\tselected := 0\n")
	for _, variant := range choice.variants {
		fmt.Fprintf(
			target,
			"\tif value.%s != nil {\n",
			variant.goName,
		)
		target.WriteString("\t\tselected++\n")
		target.WriteString("\t}\n")
	}
	target.WriteString("\tif selected != 1 {\n")
	fmt.Fprintf(
		target,
		"\t\treturn fmt.Errorf(%s, selected)\n",
		strconv.Quote(fmt.Sprintf(
			"%s: %s must contain exactly one value, got %%d",
			r.packageName,
			choice.goType,
		)),
	)
	target.WriteString("\t}\n\n")

	target.WriteString("\tswitch {\n")
	for _, variant := range choice.variants {
		fmt.Fprintf(
			target,
			"\tcase value.%s != nil:\n",
			variant.goName,
		)
		fmt.Fprintf(
			target,
			"\t\tstart := xml.StartElement{Name: %s}\n",
			xmlNameExpression(variant.name),
		)
		fmt.Fprintf(
			target,
			"\t\tif err := encoder.EncodeElement(value.%s, start); err != nil {\n",
			variant.goName,
		)
		fmt.Fprintf(
			target,
			"\t\t\treturn fmt.Errorf(%s, err)\n",
			strconv.Quote(fmt.Sprintf(
				"%s: encode %s.%s: %%w",
				r.packageName,
				choice.goType,
				variant.goName,
			)),
		)
		target.WriteString("\t\t}\n")
		target.WriteString("\t\treturn nil\n")
	}
	target.WriteString("\t}\n\n")
	target.WriteString("\treturn nil\n")
	target.WriteString("}\n\n")
}

func xmlNameExpression(name QName) string {
	var result bytes.Buffer
	result.WriteString("xml.Name{")
	if name.Namespace != "" {
		fmt.Fprintf(
			&result,
			"Space: %s, ",
			strconv.Quote(name.Namespace),
		)
	}
	fmt.Fprintf(
		&result,
		"Local: %s}",
		strconv.Quote(name.Local),
	)

	return result.String()
}
