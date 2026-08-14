package xsdgen

import (
	"encoding/xml"
	"fmt"
)

const Namespace = "http://www.w3.org/2001/XMLSchema"

type Schema struct {
	XMLName              xml.Name          `xml:"http://www.w3.org/2001/XMLSchema schema"`
	TargetNamespace      string            `xml:"targetNamespace,attr"`
	ElementFormDefault   string            `xml:"elementFormDefault,attr"`
	AttributeFormDefault string            `xml:"attributeFormDefault,attr"`
	Namespaces           NamespaceBindings `xml:"-"`

	Annotations     []Annotation     `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	Imports         []Import         `xml:"http://www.w3.org/2001/XMLSchema import"`
	Includes        []Include        `xml:"http://www.w3.org/2001/XMLSchema include"`
	SimpleTypes     []SimpleType     `xml:"http://www.w3.org/2001/XMLSchema simpleType"`
	ComplexTypes    []ComplexType    `xml:"http://www.w3.org/2001/XMLSchema complexType"`
	Attributes      []Attribute      `xml:"http://www.w3.org/2001/XMLSchema attribute"`
	AttributeGroups []AttributeGroup `xml:"http://www.w3.org/2001/XMLSchema attributeGroup"`
	Groups          []Group          `xml:"http://www.w3.org/2001/XMLSchema group"`
	Elements        []Element        `xml:"http://www.w3.org/2001/XMLSchema element"`
}

func (s *Schema) UnmarshalXML(
	decoder *xml.Decoder,
	start xml.StartElement,
) error {
	type schemaXML Schema

	var value schemaXML
	if err := decoder.DecodeElement(&value, &start); err != nil {
		return err
	}

	*s = Schema(value)
	s.Namespaces = namespaceBindings(start)

	return nil
}

type Import struct {
	Namespace      string `xml:"namespace,attr"`
	SchemaLocation string `xml:"schemaLocation,attr"`
}

type Include struct {
	SchemaLocation string `xml:"schemaLocation,attr"`
}

type Annotation struct {
	Documentation []Documentation `xml:"http://www.w3.org/2001/XMLSchema documentation"`
	AppInfo       []AppInfo       `xml:"http://www.w3.org/2001/XMLSchema appinfo"`
}

type Documentation struct {
	Source   string `xml:"source,attr"`
	Language string `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	Content  string `xml:",innerxml"`
}

type AppInfo struct {
	Source  string `xml:"source,attr"`
	Content string `xml:",innerxml"`
}

type SimpleType struct {
	Name        string       `xml:"name,attr"`
	Annotation  *Annotation  `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	Restriction *Restriction `xml:"http://www.w3.org/2001/XMLSchema restriction"`
	Union       *Union       `xml:"http://www.w3.org/2001/XMLSchema union"`
	List        *List        `xml:"http://www.w3.org/2001/XMLSchema list"`
}

type Restriction struct {
	Base       string      `xml:"base,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	SimpleType *SimpleType `xml:"http://www.w3.org/2001/XMLSchema simpleType"`

	Enumerations   []Facet `xml:"http://www.w3.org/2001/XMLSchema enumeration"`
	Patterns       []Facet `xml:"http://www.w3.org/2001/XMLSchema pattern"`
	MinInclusive   *Facet  `xml:"http://www.w3.org/2001/XMLSchema minInclusive"`
	MaxInclusive   *Facet  `xml:"http://www.w3.org/2001/XMLSchema maxInclusive"`
	MinExclusive   *Facet  `xml:"http://www.w3.org/2001/XMLSchema minExclusive"`
	MaxExclusive   *Facet  `xml:"http://www.w3.org/2001/XMLSchema maxExclusive"`
	Length         *Facet  `xml:"http://www.w3.org/2001/XMLSchema length"`
	MinLength      *Facet  `xml:"http://www.w3.org/2001/XMLSchema minLength"`
	MaxLength      *Facet  `xml:"http://www.w3.org/2001/XMLSchema maxLength"`
	TotalDigits    *Facet  `xml:"http://www.w3.org/2001/XMLSchema totalDigits"`
	FractionDigits *Facet  `xml:"http://www.w3.org/2001/XMLSchema fractionDigits"`
	WhiteSpace     *Facet  `xml:"http://www.w3.org/2001/XMLSchema whiteSpace"`

	Group           *Group           `xml:"http://www.w3.org/2001/XMLSchema group"`
	All             *All             `xml:"http://www.w3.org/2001/XMLSchema all"`
	Choice          *Choice          `xml:"http://www.w3.org/2001/XMLSchema choice"`
	Sequence        *Sequence        `xml:"http://www.w3.org/2001/XMLSchema sequence"`
	Attributes      []Attribute      `xml:"http://www.w3.org/2001/XMLSchema attribute"`
	AttributeGroups []AttributeGroup `xml:"http://www.w3.org/2001/XMLSchema attributeGroup"`
	AnyAttribute    *AnyAttribute    `xml:"http://www.w3.org/2001/XMLSchema anyAttribute"`
}

type Facet struct {
	Value      string      `xml:"value,attr"`
	Fixed      string      `xml:"fixed,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
}

type Union struct {
	MemberTypes string       `xml:"memberTypes,attr"`
	Annotation  *Annotation  `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	SimpleTypes []SimpleType `xml:"http://www.w3.org/2001/XMLSchema simpleType"`
}

type List struct {
	ItemType   string      `xml:"itemType,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	SimpleType *SimpleType `xml:"http://www.w3.org/2001/XMLSchema simpleType"`
}

type ComplexType struct {
	Name       string      `xml:"name,attr"`
	Mixed      bool        `xml:"mixed,attr"`
	Abstract   bool        `xml:"abstract,attr"`
	Block      string      `xml:"block,attr"`
	Final      string      `xml:"final,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`

	SimpleContent  *SimpleContent  `xml:"http://www.w3.org/2001/XMLSchema simpleContent"`
	ComplexContent *ComplexContent `xml:"http://www.w3.org/2001/XMLSchema complexContent"`
	Group          *Group          `xml:"http://www.w3.org/2001/XMLSchema group"`
	All            *All            `xml:"http://www.w3.org/2001/XMLSchema all"`
	Choice         *Choice         `xml:"http://www.w3.org/2001/XMLSchema choice"`
	Sequence       *Sequence       `xml:"http://www.w3.org/2001/XMLSchema sequence"`

	Attributes      []Attribute      `xml:"http://www.w3.org/2001/XMLSchema attribute"`
	AttributeGroups []AttributeGroup `xml:"http://www.w3.org/2001/XMLSchema attributeGroup"`
	AnyAttribute    *AnyAttribute    `xml:"http://www.w3.org/2001/XMLSchema anyAttribute"`
}

type SimpleContent struct {
	Annotation  *Annotation  `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	Restriction *Restriction `xml:"http://www.w3.org/2001/XMLSchema restriction"`
	Extension   *Extension   `xml:"http://www.w3.org/2001/XMLSchema extension"`
}

type ComplexContent struct {
	Mixed       bool         `xml:"mixed,attr"`
	Annotation  *Annotation  `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	Restriction *Restriction `xml:"http://www.w3.org/2001/XMLSchema restriction"`
	Extension   *Extension   `xml:"http://www.w3.org/2001/XMLSchema extension"`
}

type Extension struct {
	Base       string      `xml:"base,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`

	Group    *Group    `xml:"http://www.w3.org/2001/XMLSchema group"`
	All      *All      `xml:"http://www.w3.org/2001/XMLSchema all"`
	Choice   *Choice   `xml:"http://www.w3.org/2001/XMLSchema choice"`
	Sequence *Sequence `xml:"http://www.w3.org/2001/XMLSchema sequence"`

	Attributes      []Attribute      `xml:"http://www.w3.org/2001/XMLSchema attribute"`
	AttributeGroups []AttributeGroup `xml:"http://www.w3.org/2001/XMLSchema attributeGroup"`
	AnyAttribute    *AnyAttribute    `xml:"http://www.w3.org/2001/XMLSchema anyAttribute"`
}

type Attribute struct {
	Name       string      `xml:"name,attr"`
	Ref        string      `xml:"ref,attr"`
	Type       string      `xml:"type,attr"`
	Use        string      `xml:"use,attr"`
	Default    *string     `xml:"default,attr"`
	Fixed      *string     `xml:"fixed,attr"`
	Form       string      `xml:"form,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	SimpleType *SimpleType `xml:"http://www.w3.org/2001/XMLSchema simpleType"`
}

type AttributeGroup struct {
	Name       string      `xml:"name,attr"`
	Ref        string      `xml:"ref,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`

	Attributes      []Attribute      `xml:"http://www.w3.org/2001/XMLSchema attribute"`
	AttributeGroups []AttributeGroup `xml:"http://www.w3.org/2001/XMLSchema attributeGroup"`
	AnyAttribute    *AnyAttribute    `xml:"http://www.w3.org/2001/XMLSchema anyAttribute"`
}

type Occurs struct {
	MinOccurs string `xml:"minOccurs,attr"`
	MaxOccurs string `xml:"maxOccurs,attr"`
}

type Group struct {
	Occurs

	Name       string      `xml:"name,attr"`
	Ref        string      `xml:"ref,attr"`
	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	All        *All        `xml:"http://www.w3.org/2001/XMLSchema all"`
	Choice     *Choice     `xml:"http://www.w3.org/2001/XMLSchema choice"`
	Sequence   *Sequence   `xml:"http://www.w3.org/2001/XMLSchema sequence"`
}

type All struct {
	Occurs

	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	Particles  []Particle  `xml:",any"`
}

type Choice struct {
	Occurs

	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	Particles  []Particle  `xml:",any"`
}

type Sequence struct {
	Occurs

	Annotation *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	Particles  []Particle  `xml:",any"`
}

type Element struct {
	Occurs

	Name              string       `xml:"name,attr"`
	Ref               string       `xml:"ref,attr"`
	Type              string       `xml:"type,attr"`
	Default           *string      `xml:"default,attr"`
	Fixed             *string      `xml:"fixed,attr"`
	Form              string       `xml:"form,attr"`
	Block             string       `xml:"block,attr"`
	Final             string       `xml:"final,attr"`
	SubstitutionGroup string       `xml:"substitutionGroup,attr"`
	Nillable          bool         `xml:"nillable,attr"`
	Abstract          bool         `xml:"abstract,attr"`
	Annotation        *Annotation  `xml:"http://www.w3.org/2001/XMLSchema annotation"`
	SimpleType        *SimpleType  `xml:"http://www.w3.org/2001/XMLSchema simpleType"`
	ComplexType       *ComplexType `xml:"http://www.w3.org/2001/XMLSchema complexType"`
}

type Any struct {
	Occurs

	Namespace       string      `xml:"namespace,attr"`
	ProcessContents string      `xml:"processContents,attr"`
	Annotation      *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
}

type AnyAttribute struct {
	Namespace       string      `xml:"namespace,attr"`
	ProcessContents string      `xml:"processContents,attr"`
	Annotation      *Annotation `xml:"http://www.w3.org/2001/XMLSchema annotation"`
}

type Particle struct {
	Element  *Element
	Group    *Group
	All      *All
	Choice   *Choice
	Sequence *Sequence
	Any      *Any
}

func (p *Particle) UnmarshalXML(
	decoder *xml.Decoder,
	start xml.StartElement,
) error {
	if start.Name.Space != Namespace {
		return fmt.Errorf(
			"xsdgen: unsupported particle {%s}%s",
			start.Name.Space,
			start.Name.Local,
		)
	}

	switch start.Name.Local {
	case "element":
		p.Element = &Element{}
		return decoder.DecodeElement(p.Element, &start)
	case "group":
		p.Group = &Group{}
		return decoder.DecodeElement(p.Group, &start)
	case "all":
		p.All = &All{}
		return decoder.DecodeElement(p.All, &start)
	case "choice":
		p.Choice = &Choice{}
		return decoder.DecodeElement(p.Choice, &start)
	case "sequence":
		p.Sequence = &Sequence{}
		return decoder.DecodeElement(p.Sequence, &start)
	case "any":
		p.Any = &Any{}
		return decoder.DecodeElement(p.Any, &start)
	default:
		return fmt.Errorf(
			"xsdgen: unsupported particle %q",
			start.Name.Local,
		)
	}
}
