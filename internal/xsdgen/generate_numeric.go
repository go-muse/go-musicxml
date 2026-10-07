package xsdgen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Defined Go types do not inherit XML methods from their underlying named
// type, so each restriction in a numeric chain needs its own adapter.
func (r *simpleTypeRenderer) numericXMLType(definition *SimpleTypeDefinition) string {
	return r.numericXMLTypeSeen(definition, make(map[*SimpleTypeDefinition]bool))
}

func (r *simpleTypeRenderer) numericXMLTypeSeen(definition *SimpleTypeDefinition, seen map[*SimpleTypeDefinition]bool) string {
	if seen[definition] {
		return ""
	}
	seen[definition] = true
	if definition == nil || definition.Form != SimpleTypeRestriction || definition.Base == nil {
		return ""
	}
	member := definition.Base
	if member.Inline != nil {
		return r.numericXMLTypeSeen(member.Inline, seen)
	}
	if member.Declaration == nil {
		return ""
	}
	if member.Declaration.Kind == DeclarationBuiltinSimpleType {
		value, _ := BuiltinGoType(member.Declaration)
		return value.xmlType
	}
	return r.numericXMLTypeSeen(r.plans[member.Declaration], seen)
}

func (r *simpleTypeRenderer) renderNumericXMLMethods(target *bytes.Buffer, name string, definition *SimpleTypeDefinition) {
	codec := r.numericXMLType(definition)
	if codec == "" {
		return
	}
	if codec == "xmlDecimal" {
		fmt.Fprintf(target, "// MarshalXML encodes an XSD decimal without exponent notation.\n")
		fmt.Fprintf(target, "func (value %s) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {\n", name)
		fmt.Fprintf(target, "return encoder.EncodeElement(%s(value), start)\n}\n\n", codec)
		fmt.Fprintf(target, "// MarshalXMLAttr encodes an XSD decimal attribute without exponent notation.\n")
		fmt.Fprintf(target, "func (value %s) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {\n", name)
		fmt.Fprintf(target, "text, err := %s(value).MarshalText()\n", codec)
		target.WriteString("return xml.Attr{Name: name, Value: string(text)}, err\n}\n\n")
	}
	fmt.Fprintf(target, "// UnmarshalXML decodes numeric XML content.\n")
	fmt.Fprintf(target, "func (value *%s) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {\n", name)
	fmt.Fprintf(target, "return decoder.DecodeElement((*%s)(value), &start)\n}\n\n", codec)
	fmt.Fprintf(target, "// UnmarshalXMLAttr decodes a numeric XML attribute.\n")
	fmt.Fprintf(target, "func (value *%s) UnmarshalXMLAttr(attribute xml.Attr) error {\n", name)
	fmt.Fprintf(target, "return (*%s)(value).UnmarshalText([]byte(attribute.Value))\n}\n\n", codec)
}

func (r *simpleTypeRenderer) declarationXMLType(declaration *Declaration) string {
	if declaration.Kind == DeclarationBuiltinSimpleType {
		value, _ := BuiltinGoType(declaration)
		return value.xmlType
	}
	return r.numericXMLType(r.plans[declaration])
}

func (r *complexTypeRenderer) registerStructure(structure *complexStructure) {
	if r.structures == nil {
		r.structures = make(map[string]*complexStructure)
	}
	r.structures[structure.owner] = structure
}

func (r *complexTypeRenderer) registerNamedStructure(structure *complexStructure) {
	r.registerStructure(structure)
	if r.namedStructures == nil {
		r.namedStructures = make(map[string]bool)
	}
	r.namedStructures[structure.owner] = true
}

func (r *complexTypeRenderer) elementXMLType(element *ElementPlan, owner string) (string, *complexStructure, error) {
	plan := element.Type
	if element.Reference != nil {
		declaration := element.Reference
		name, err := GoTypeName(declaration.Name.Local)
		if err != nil {
			return "", nil, err
		}
		planner := complexTypePlanner{index: r.index, file: declaration.File,
			ownerName: declaration.Name.Local, ownerGoName: name, expandGroups: true}
		plan, err = planner.planElementType(declaration.Element)
		if err != nil {
			return "", nil, err
		}
	}
	if plan == nil {
		return "", nil, ErrInvalidElement
	}
	if plan.InlineComplex != nil {
		if r.nameInline {
			return "", nil, nil
		}
		name, err := GoTypeName(element.Name.Local)
		if err != nil {
			return "", nil, err
		}
		return "", r.structures[owner+name], nil
	}
	if plan.Declaration != nil && plan.Declaration.Kind != DeclarationBuiltinSimpleType && plan.Declaration.Kind != DeclarationSimpleType {
		return "", nil, nil
	}
	value, err := r.simpleTypePlanGoType(plan)
	return value.xmlType, nil, err
}

func (r *complexTypeRenderer) choiceXMLType(variant complexChoiceVariant) string {
	if r.needsNumericXML(variant.inline) {
		return "xml" + variant.inline.owner
	}
	if variant.xmlType != "" {
		return variant.xmlType
	}
	return variant.goType
}

func (r *complexTypeRenderer) choiceXMLDecoded(variant complexChoiceVariant) string {
	if r.needsNumericXML(variant.inline) {
		return "xmlConvertPointer(&decoded, xml" + variant.inline.owner + ".model)"
	}
	if variant.xmlType != "" {
		return "(*" + variant.goType + ")(&decoded)"
	}
	return "&decoded"
}

func (r *complexTypeRenderer) choiceXMLValue(variant complexChoiceVariant) string {
	value := "value." + variant.goName
	if r.needsNumericXML(variant.inline) {
		return "xmlConvertPointer(" + value + ", xml" + variant.inline.owner + "From)"
	}
	if variant.xmlType != "" {
		return "(*" + variant.xmlType + ")(" + value + ")"
	}
	return value
}

func (r *complexTypeRenderer) needsNumericXML(structure *complexStructure) bool {
	if structure == nil {
		return false
	}
	if structure.valueXMLType != "" || r.needsNumericXML(r.structures[structure.embedded]) {
		return true
	}
	for _, fields := range [][]complexField{structure.elements, structure.attributes} {
		for _, field := range fields {
			if field.xmlType != "" || r.needsNumericXML(field.inline) {
				return true
			}
		}
	}
	return false
}

// Wire structs keep the exported field types unchanged, while encoding/xml
// sees text adapters for direct built-in numeric fields. Embedded wire structs
// have no XML methods, avoiding accidental promotion of a base type's methods.
func (r *complexTypeRenderer) renderNumericXML(target *bytes.Buffer) (bool, error) {
	if err := r.completeNumericStructures(); err != nil {
		return false, err
	}
	var names []string
	for name, structure := range r.structures {
		if !r.externalStructures[name] && r.needsNumericXML(structure) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		structure := r.structures[name]
		wire := "xml" + name
		public := name
		if !r.namedStructures[name] {
			var source bytes.Buffer
			r.renderStructure(&source, structure)
			public = source.String()
		}
		converted := r.numericWireStructure(structure)
		fmt.Fprintf(target, "type %s struct {\n", wire)
		if structure.xmlName != nil {
			fmt.Fprintf(target, "XMLName xml.Name %s\n", xmlStructTag(*structure.xmlName, false, false))
		}
		r.renderStructureFields(target, &converted)
		target.WriteString("}\n\n")
		fmt.Fprintf(target, "func %sFrom(value %s) %s {\nreturn %s{\n", wire, public, wire, wire)
		r.renderNumericConversions(target, structure, true)
		target.WriteString("}\n}\n\n")
		fmt.Fprintf(target, "func (value %s) model() %s {\nreturn %s{\n", wire, public, public)
		r.renderNumericConversions(target, structure, false)
		target.WriteString("}\n}\n\n")
		if !r.namedStructures[name] {
			continue
		}
		fmt.Fprintf(target, "// MarshalXML encodes numeric fields using their XSD lexical forms.\n")
		fmt.Fprintf(target, "func (value %s) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {\n", name)
		if structure.xmlName != nil {
			fmt.Fprintf(target, "if start.Name == (xml.Name{Local: %q}) { start.Name = %s }\n", name, xmlNameExpression(*structure.xmlName))
		}
		fmt.Fprintf(target, "return encoder.EncodeElement(%sFrom(value), start)\n}\n\n", wire)
		fmt.Fprintf(target, "// UnmarshalXML decodes numeric fields using their XSD lexical forms.\n")
		fmt.Fprintf(target, "func (value *%s) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {\n", name)
		fmt.Fprintf(target, "decoded := %sFrom(*value)\n", wire)
		target.WriteString("if err := decoder.DecodeElement(&decoded, &start); err != nil { return err }\n")
		target.WriteString("*value = decoded.model()\nreturn nil\n}\n\n")
	}
	return len(names) != 0, nil
}

func (r *complexTypeRenderer) numericWireStructure(structure *complexStructure) complexStructure {
	wire := *structure
	if r.needsNumericXML(r.structures[structure.embedded]) {
		wire.embedded = "xml" + structure.embedded
	}
	if structure.valueXMLType != "" {
		wire.valueType = structure.valueXMLType
	}
	wire.elements = append([]complexField(nil), structure.elements...)
	wire.attributes = append([]complexField(nil), structure.attributes...)
	for _, fields := range [][]complexField{wire.elements, wire.attributes} {
		for index := range fields {
			field := &fields[index]
			if field.xmlType != "" {
				field.goType = field.xmlType
			} else if r.needsNumericXML(field.inline) {
				field.goType = "xml" + field.inline.owner
			}
		}
	}
	return wire
}

func (r *complexTypeRenderer) renderNumericConversions(target *bytes.Buffer, structure *complexStructure, toWire bool) {
	if structure.xmlName != nil {
		target.WriteString("XMLName: value.XMLName,\n")
	}
	if structure.embedded != "" {
		name := structure.embedded
		key, expression := name, "value."+name
		if r.needsNumericXML(r.structures[name]) {
			if toWire {
				key, expression = "xml"+name, "xml"+name+"From("+expression+")"
			} else {
				expression = "value.xml" + name + ".model()"
			}
		}
		fmt.Fprintf(target, "%s: %s,\n", key, expression)
	}
	if structure.valueType != "" {
		expression := "value.Value"
		if structure.valueXMLType != "" {
			kind := structure.valueType
			if toWire {
				kind = structure.valueXMLType
			}
			expression = kind + "(" + expression + ")"
		}
		fmt.Fprintf(target, "Value: %s,\n", expression)
	}
	for _, fields := range [][]complexField{structure.elements, structure.attributes} {
		for _, field := range fields {
			expression := "value." + field.goName
			kind := field.goType
			wireKind := field.xmlType
			if r.needsNumericXML(field.inline) {
				wireKind = "xml" + field.inline.owner
			}
			if wireKind != "" {
				from, to := kind, wireKind
				if !toWire {
					from, to = wireKind, kind
				}
				fieldType := elementFieldType(&field)
				if field.kind == complexFieldAttribute {
					fieldType = attributeFieldType(&field)
				}
				if field.inline == nil {
					switch {
					case strings.HasPrefix(fieldType, "[]"):
						expression = fmt.Sprintf("xmlConvertSlice(%s, func(value %s) %s { return %s(value) })", expression, from, to, to)
					case strings.HasPrefix(fieldType, "*"):
						expression = "(*" + to + ")(" + expression + ")"
					default:
						expression = to + "(" + expression + ")"
					}
				} else {
					convert := wireKind + "From"
					if !toWire {
						convert = wireKind + ".model"
					}
					switch {
					case strings.HasPrefix(fieldType, "[]"):
						expression = "xmlConvertSlice(" + expression + ", " + convert + ")"
					case strings.HasPrefix(fieldType, "*"):
						expression = "xmlConvertPointer(" + expression + ", " + convert + ")"
					default:
						expression = convert + "(" + expression + ")"
					}
				}
			}
			fmt.Fprintf(target, "%s: %s,\n", field.goName, expression)
		}
	}
}

// Element generation can embed complex types emitted in a separate generated
// file. Inspect their structures to avoid inheriting their XML methods, but
// leave their wire declarations in that file.
func (r *complexTypeRenderer) completeNumericStructures() error {
	for {
		var missing []string
		for _, structure := range r.structures {
			if structure.embedded != "" && r.structures[structure.embedded] == nil {
				missing = append(missing, structure.embedded)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		plans, err := planComplexTypesWithNames(r.index, r.names, true)
		if err != nil {
			return err
		}
		sort.Strings(missing)
		for _, name := range missing {
			if r.structures[name] != nil {
				continue
			}
			var definition *ComplexTypeDefinition
			for _, plan := range plans {
				if plan.GoName == name {
					definition = plan.Definition
					break
				}
			}
			if definition == nil {
				return fmt.Errorf("%w: missing embedded type %s", ErrInvalidComplexType, name)
			}
			previous := make(map[string]bool, len(r.structures))
			for existing := range r.structures {
				previous[existing] = true
			}
			if _, err := r.buildStructure(name, definition); err != nil {
				return err
			}
			if r.externalStructures == nil {
				r.externalStructures = make(map[string]bool)
			}
			for generated := range r.structures {
				if !previous[generated] {
					r.externalStructures[generated] = true
				}
			}
		}
	}
}
