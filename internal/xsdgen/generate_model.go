package xsdgen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// renderModelMetadata uses the final model renderer plans, including naming,
// flattened bases and ordered choices. It deliberately does not generate XSD
// use identities: only the shared compiled grammar can choose those witnesses.
func (r *complexTypeRenderer) renderModelMetadata(target *bytes.Buffer) error {
	if err := r.completeNumericStructures(); err != nil {
		return err
	}
	names := make([]string, 0, len(r.structures))
	for name := range r.structures {
		if !r.externalStructures[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	target.WriteString("// The closed current-model map is generated from the same final field plans.\nfunc init() {\n")
	for _, name := range names {
		s := r.structures[name]
		goFields := len(s.attributes) + len(s.elements)
		if s.embedded != "" {
			goFields++
		}
		if s.valueType != "" {
			goFields++
		}
		if s.xmlName != nil {
			goFields++
		}
		fmt.Fprintf(target, "modelDescriptors[%q] = &modelDescriptor{Name: %q, GoFields:%d, MappedFields:%d,", name, name, goFields, r.modelMappedFields(s))
		for _, d := range r.names.Declarations() {
			n, _ := r.names.Lookup(d)
			if n == name && d.Kind == DeclarationComplexType {
				fmt.Fprintf(target, "SchemaType: %s,", modelNameExpression(d.Name))
				break
			}
		}
		if s.xmlName != nil {
			fmt.Fprintf(target, "Root: %s,", modelNameExpression(*s.xmlName))
		}
		var scalar, attrs, elements bytes.Buffer
		if err := r.renderModelFields(s, nil, "", &scalar, &attrs, &elements); err != nil {
			return err
		}
		if scalar.Len() != 0 {
			fmt.Fprintf(target, "Scalar: &modelField{%s},", scalar.String())
		}
		fmt.Fprintf(target, "Attributes: []modelField{%s}, Elements: []modelField{%s}}\n", attrs.String(), elements.String())
	}
	target.WriteString("}\n\n")
	return nil
}

func modelNameExpression(name QName) string {
	if name.Namespace == "" {
		return fmt.Sprintf("modelName{Local: %q}", name.Local)
	}
	return fmt.Sprintf("modelName{Space: %q, Local: %q}", name.Namespace, name.Local)
}

func modelIndexExpression(path []int) string {
	var b strings.Builder
	b.WriteString("[]int{")
	for _, i := range path {
		fmt.Fprintf(&b, "%d,", i)
	}
	b.WriteByte('}')
	return b.String()
}

func (r *complexTypeRenderer) renderModelFields(s *complexStructure, path []int, prefix string, scalar, attrs, elements *bytes.Buffer) error {
	index := 0
	if s.xmlName != nil {
		index++
	}
	at := func(i int) []int { return append(append([]int(nil), path...), i) }
	if s.embedded != "" {
		base := r.structures[s.embedded]
		if base == nil {
			return fmt.Errorf("%w: model base %s", ErrInvalidComplexType, s.embedded)
		}
		if err := r.renderModelFields(base, at(index), prefix+s.embedded+".", scalar, attrs, elements); err != nil {
			return err
		}
		index++
	}
	if s.valueType != "" {
		if scalar.Len() != 0 {
			return fmt.Errorf("%w: duplicate model scalar in %s", ErrInvalidComplexType, s.owner)
		}
		kind, child, err := r.modelRepresentation(s.valueType, s.valueXMLType, nil)
		if err != nil {
			return err
		}
		if child != "" {
			return ErrInvalidComplexType
		}
		fmt.Fprintf(scalar, "Name:%q, Index:%s, Kind:%s", prefix+"Value", modelIndexExpression(at(index)), kind)
		index++
	}
	for _, f := range s.elements {
		fmt.Fprintf(elements, "{Name:%q, Index:%s,", prefix+f.goName, modelIndexExpression(at(index)))
		index++
		if f.kind == complexFieldChoice {
			fmt.Fprintf(elements, "Repeated:true, Variants: []modelField{")
			for vi, v := range f.choice.variants {
				fmt.Fprintf(elements, "{Name:%q, Index:[]int{%d}, XML:%s, Optional:true,", v.goName, vi, modelNameExpression(v.name))
				if err := r.renderModelKind(elements, v.goType, v.xmlType, v.inline); err != nil {
					return err
				}
				elements.WriteString("},")
			}
			elements.WriteString("}},")
			continue
		}
		fmt.Fprintf(elements, "XML:%s, Optional:%t, Repeated:%t,", modelNameExpression(f.name), (!f.required || f.nillable) && !f.repeated, f.repeated)
		if err := r.renderModelKind(elements, f.goType, f.xmlType, f.inline); err != nil {
			return err
		}
		elements.WriteString("},")
	}
	for _, f := range s.attributes {
		fmt.Fprintf(attrs, "{Name:%q, Index:%s, XML:%s, Optional:%t,", prefix+f.goName, modelIndexExpression(at(index)), modelNameExpression(f.name), !f.required)
		index++
		if err := r.renderModelKind(attrs, f.goType, f.xmlType, nil); err != nil {
			return err
		}
		attrs.WriteString("},")
	}
	return nil
}

func (r *complexTypeRenderer) renderModelKind(b *bytes.Buffer, goType, xmlType string, inline *complexStructure) error {
	kind, child, err := r.modelRepresentation(goType, xmlType, inline)
	if err != nil {
		return err
	}
	if child != "" {
		fmt.Fprintf(b, "Child:%q,", child)
	} else {
		fmt.Fprintf(b, "Kind:%s,", kind)
	}
	return nil
}

func (r *complexTypeRenderer) modelRepresentation(goType, xmlType string, inline *complexStructure) (string, string, error) {
	if inline != nil {
		return "", inline.owner, nil
	}
	for _, d := range r.names.Declarations() {
		name, _ := r.names.Lookup(d)
		if name != goType {
			continue
		}
		if d.Kind == DeclarationComplexType {
			return "", goType, nil
		}
		value, err := r.simpleRenderer.memberGoType(SimpleTypeMember{Declaration: d})
		if err != nil {
			return "", "", err
		}
		if value.Kind == GoTypeFloat {
			xmlType = r.simpleRenderer.declarationXMLType(d)
		}
		return modelKind(value.Kind, xmlType)
	}
	if r.structures[goType] != nil {
		return "", goType, nil
	}
	if kind, found := r.simpleRenderer.builtinKind(goType); found {
		return modelKind(kind, "")
	}
	for _, value := range builtinGoTypes {
		if value.Expression == goType {
			return modelKind(value.Kind, xmlType)
		}
	}
	return "", "", fmt.Errorf("%w: model representation %s", ErrUnsupportedComplexGeneration, goType)
}

func modelKind(kind GoTypeKind, xmlType string) (string, string, error) {
	switch kind {
	case GoTypeString:
		return "modelString", "", nil
	case GoTypeBoolean:
		return "modelBoolean", "", nil
	case GoTypeSignedInteger:
		return "modelSigned", "", nil
	case GoTypeUnsignedInteger:
		return "modelUnsigned", "", nil
	case GoTypeFloat:
		if xmlType == "xmlDecimal" {
			return "modelDecimal", "", nil
		}
	}
	return "", "", fmt.Errorf("%w: model scalar kind %s/%s", ErrUnsupportedComplexGeneration, kind, xmlType)
}

// The independent shape seal detects an incomplete descriptor without inferring
// fields or XML names from the caller's runtime struct.
func (r *complexTypeRenderer) modelMappedFields(s *complexStructure) int {
	count := len(s.attributes) + len(s.elements)
	if s.valueType != "" {
		count++
	}
	if s.embedded != "" {
		count += r.modelMappedFields(r.structures[s.embedded])
	}
	return count
}
