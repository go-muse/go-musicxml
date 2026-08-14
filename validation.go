package musicxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	validationXSDNamespace = "http://www.w3.org/2001/XMLSchema"
	validationXSINamespace = "http://www.w3.org/2001/XMLSchema-instance"
)

// ValidationIssue describes one XSD validation failure.
type ValidationIssue struct {
	Path       string
	Constraint string
	Message    string
}

// String returns the issue path and message.
func (i ValidationIssue) String() string {
	if i.Path == "" {
		return i.Message
	}

	return i.Path + ": " + i.Message
}

// ValidationError reports every XSD validation issue found in a document.
type ValidationError struct {
	Issues []ValidationIssue
}

// Error summarizes the validation failure.
func (e *ValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ErrInvalidDocument.Error()
	}
	if len(e.Issues) == 1 {
		return fmt.Sprintf("%s: %s", ErrInvalidDocument, e.Issues[0])
	}

	return fmt.Sprintf(
		"%s: %d issues; first: %s",
		ErrInvalidDocument,
		len(e.Issues),
		e.Issues[0],
	)
}

// Unwrap makes ValidationError match ErrInvalidDocument.
func (e *ValidationError) Unwrap() error {
	return ErrInvalidDocument
}

// Validate checks document against the MusicXML 4.0 XSD.
//
// Validation is explicit: Decode and Encode remain lossless transport
// operations and do not invoke it automatically.
func Validate(document Document) error {
	schema, err := validationSchemaForDocument(document)
	if err != nil {
		return err
	}

	var encoded bytes.Buffer
	if err := Encode(&encoded, document); err != nil {
		return &ValidationError{Issues: []ValidationIssue{{
			Path:       validationDocumentPath(document),
			Constraint: "representation",
			Message:    fmt.Sprintf("cannot encode typed document: %v", err),
		}}}
	}

	root, err := parseValidationDocument(encoded.Bytes())
	if err != nil {
		return &ValidationError{Issues: []ValidationIssue{{
			Path:       validationDocumentPath(document),
			Constraint: "well-formed",
			Message:    err.Error(),
		}}}
	}

	rootSchema, found := schema.Elements[validationName(root.Name)]
	if !found {
		return &ValidationError{Issues: []ValidationIssue{{
			Path:       validationPath("", root.Name.Local, 1),
			Constraint: "root",
			Message: fmt.Sprintf(
				"unsupported root element %s",
				validationDisplayName(root.Name),
			),
		}}}
	}

	context := validationContext{
		schema:      schema,
		effective:   make(map[*validationComplexSchema]*validationEffectiveComplex),
		identifiers: make(map[string]string),
	}
	context.validateElement(
		root,
		rootSchema,
		validationPath("", root.Name.Local, 1),
	)
	context.validateIdentityReferences()
	if len(context.issues) == 0 {
		return nil
	}

	return &ValidationError{Issues: context.issues}
}

// Validate checks a partwise score against the MusicXML 4.0 XSD.
func (value *ScorePartwise) Validate() error {
	return Validate(value)
}

// Validate checks a timewise score against the MusicXML 4.0 XSD.
func (value *ScoreTimewise) Validate() error {
	return Validate(value)
}

// Validate checks an opus against the MusicXML 4.0 opus XSD.
func (value *OpusDocument) Validate() error {
	return Validate(value)
}

type validationQName struct {
	Space string
	Local string
}

type validationSchemaSet struct {
	Types    map[validationQName]*validationTypeSchema
	Elements map[validationQName]*validationElementSchema
}

type validationTypeSchema struct {
	Simple  *validationSimpleSchema
	Complex *validationComplexSchema
}

type validationTypeRef struct {
	Name          validationQName
	InlineSimple  *validationSimpleSchema
	InlineComplex *validationComplexSchema
}

type validationSimpleForm string

const (
	validationSimpleRestriction validationSimpleForm = "restriction"
	validationSimpleUnion       validationSimpleForm = "union"
	validationSimpleList        validationSimpleForm = "list"
)

type validationSimpleSchema struct {
	Form         validationSimpleForm
	Base         *validationSimpleMember
	Enumerations []string
	Members      []*validationSimpleMember
	Item         *validationSimpleMember
	Patterns     []string

	MinInclusive   string
	MaxInclusive   string
	MinExclusive   string
	MaxExclusive   string
	Length         uint64
	MinLength      uint64
	MaxLength      uint64
	TotalDigits    uint64
	FractionDigits uint64
	WhiteSpace     string

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

type validationSimpleMember struct {
	Name   validationQName
	Inline *validationSimpleSchema
}

type validationComplexForm string

const (
	validationComplexDirect                    validationComplexForm = "direct"
	validationComplexSimpleContentExtension    validationComplexForm = "simple content extension"
	validationComplexSimpleContentRestriction  validationComplexForm = "simple content restriction"
	validationComplexComplexContentExtension   validationComplexForm = "complex content extension"
	validationComplexComplexContentRestriction validationComplexForm = "complex content restriction"
)

type validationComplexSchema struct {
	Form         validationComplexForm
	Base         *validationTypeRef
	Particle     *validationParticleSchema
	Attributes   []validationAttributeSchema
	AnyAttribute *validationAnyAttributeSchema
	Mixed        bool
}

type validationAttributeUse string

const (
	validationAttributeOptional   validationAttributeUse = "optional"
	validationAttributeRequired   validationAttributeUse = "required"
	validationAttributeProhibited validationAttributeUse = "prohibited"
)

type validationAttributeSchema struct {
	Name       validationQName
	Type       validationTypeRef
	Use        validationAttributeUse
	Constraint *validationValueConstraint
}

type validationValueConstraint struct {
	Kind  string
	Value string
}

type validationAnyAttributeSchema struct {
	Namespace       string
	ProcessContents string
}

type validationParticleKind string

const (
	validationParticleElement  validationParticleKind = "element"
	validationParticleAll      validationParticleKind = "all"
	validationParticleChoice   validationParticleKind = "choice"
	validationParticleSequence validationParticleKind = "sequence"
	validationParticleAny      validationParticleKind = "any"
)

type validationOccurrence struct {
	Min       uint64
	Max       uint64
	Unbounded bool
}

type validationParticleSchema struct {
	Kind       validationParticleKind
	Occurrence validationOccurrence
	Element    *validationElementSchema
	Children   []*validationParticleSchema
	Any        *validationAnySchema
}

type validationAnySchema struct {
	Namespace       string
	ProcessContents string
}

type validationElementSchema struct {
	Name      validationQName
	Reference validationQName
	Type      validationTypeRef
	Default   *string
	Fixed     *string
	Nillable  bool
	Abstract  bool
}

type validationNode struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Text     strings.Builder
	Children []*validationNode
}

type validationContext struct {
	schema      *validationSchemaSet
	issues      []ValidationIssue
	effective   map[*validationComplexSchema]*validationEffectiveComplex
	identifiers map[string]string
	references  []validationIdentityReference
}

type validationIdentityReference struct {
	value string
	path  string
}

type validationEffectiveComplex struct {
	particle     *validationParticleSchema
	attributes   []validationAttributeSchema
	anyAttribute *validationAnyAttributeSchema
	simple       *validationTypeRef
	mixed        bool
}

type validationElementMatch struct {
	index  int
	schema *validationElementSchema
}

type validationMatchResult struct {
	next     int
	elements []validationElementMatch
}

type validationMatchFailure struct {
	position int
	expected []validationQName
}

type validationSimpleFailure struct {
	constraint string
	message    string
}

func validationSchemaForDocument(
	document Document,
) (*validationSchemaSet, error) {
	switch value := document.(type) {
	case nil:
		return nil, ErrNilDocument
	case *ScorePartwise:
		if value == nil {
			return nil, ErrNilDocument
		}
		return &scoreValidationSchema, nil
	case *ScoreTimewise:
		if value == nil {
			return nil, ErrNilDocument
		}
		return &scoreValidationSchema, nil
	case *OpusDocument:
		if value == nil {
			return nil, ErrNilDocument
		}
		return &opusValidationSchema, nil
	default:
		return nil, fmt.Errorf(
			"%w: %T",
			ErrUnsupportedDocument,
			document,
		)
	}
}

func validationDocumentPath(document Document) string {
	switch document.(type) {
	case *ScorePartwise:
		return "/score-partwise"
	case *ScoreTimewise:
		return "/score-timewise"
	case *OpusDocument:
		return "/opus"
	default:
		return "/"
	}
}

func parseValidationDocument(source []byte) (*validationNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(source))

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil, ErrEmptyDocument
		}
		if err != nil {
			return nil, fmt.Errorf("read document root: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			root, err := readValidationNode(decoder, value)
			if err != nil {
				return nil, err
			}
			return root, readValidationTail(decoder)
		case xml.CharData:
			if len(bytes.TrimSpace(value)) != 0 {
				return nil, errors.New("unexpected character data before root")
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
		default:
			return nil, fmt.Errorf("unexpected %T before root", token)
		}
	}
}

func readValidationNode(
	decoder *xml.Decoder,
	start xml.StartElement,
) (*validationNode, error) {
	result := &validationNode{
		Name:  start.Name,
		Attrs: append([]xml.Attr(nil), start.Attr...),
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf(
				"read %s: %w",
				validationDisplayName(start.Name),
				err,
			)
		}

		switch value := token.(type) {
		case xml.StartElement:
			child, err := readValidationNode(decoder, value)
			if err != nil {
				return nil, err
			}
			result.Children = append(result.Children, child)
		case xml.EndElement:
			if value.Name != start.Name {
				return nil, fmt.Errorf(
					"unexpected closing element %s",
					validationDisplayName(value.Name),
				)
			}
			return result, nil
		case xml.CharData:
			result.Text.Write([]byte(value))
		case xml.Comment, xml.ProcInst, xml.Directive:
		}
	}
}

func readValidationTail(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read document tail: %w", err)
		}

		switch value := token.(type) {
		case xml.CharData:
			if len(bytes.TrimSpace(value)) == 0 {
				continue
			}
		case xml.Comment, xml.ProcInst:
			continue
		}

		return fmt.Errorf("unexpected %T after root element", token)
	}
}

func (c *validationContext) validateElement(
	node *validationNode,
	schema *validationElementSchema,
	path string,
) {
	resolved, found := c.resolveElement(schema)
	if !found {
		c.addIssue(
			path,
			"schema",
			"element declaration cannot be resolved",
		)
		return
	}
	schema = resolved

	if node.Name.Space != schema.Name.Space ||
		node.Name.Local != schema.Name.Local {
		c.addIssue(
			path,
			"name",
			fmt.Sprintf(
				"got %s, expected %s",
				validationDisplayName(node.Name),
				validationDisplayQName(schema.Name),
			),
		)
		return
	}

	if schema.Abstract {
		c.addIssue(path, "abstract", "abstract element cannot be used")
		return
	}

	nilValue, nilPresent := validationNilAttribute(node.Attrs)
	if nilPresent {
		if !schema.Nillable {
			c.addIssue(
				path+"/@xsi:nil",
				"nillable",
				"element is not nillable",
			)
		} else if nilValue {
			if len(node.Children) != 0 ||
				strings.TrimSpace(node.Text.String()) != "" {
				c.addIssue(
					path,
					"nillable",
					"nil element must not contain a value",
				)
			}
			return
		}
	}

	if schema.Fixed != nil &&
		node.Text.String() != *schema.Fixed {
		c.addIssue(
			path,
			"fixed",
			fmt.Sprintf(
				"value %q does not equal fixed value %q",
				node.Text.String(),
				*schema.Fixed,
			),
		)
	}

	c.validateType(node, &schema.Type, path)
}

func (c *validationContext) resolveElement(
	schema *validationElementSchema,
) (*validationElementSchema, bool) {
	if schema == nil {
		return nil, false
	}
	if schema.Reference.Local == "" {
		return schema, true
	}

	resolved, found := c.schema.Elements[schema.Reference]
	return resolved, found
}

func (c *validationContext) validateType(
	node *validationNode,
	reference *validationTypeRef,
	path string,
) {
	simple, complex, builtin, found := c.resolveType(reference)
	if !found {
		c.addIssue(path, "schema", "type declaration cannot be resolved")
		return
	}

	switch {
	case simple != nil:
		if len(node.Children) != 0 {
			c.addIssue(
				path,
				"simple-content",
				"simple value must not contain child elements",
			)
			return
		}
		if failure := c.validateSimple(simple, node.Text.String()); failure != nil {
			c.addIssue(path, failure.constraint, failure.message)
		} else {
			c.recordIdentity(reference, node.Text.String(), path)
		}

	case complex != nil:
		c.validateComplex(node, complex, path)

	case builtin == "anyType":
		return

	case builtin != "":
		if len(node.Children) != 0 {
			c.addIssue(
				path,
				"simple-content",
				"simple value must not contain child elements",
			)
			return
		}
		if failure := validateBuiltin(builtin, node.Text.String()); failure != nil {
			c.addIssue(path, failure.constraint, failure.message)
		} else {
			c.recordIdentity(reference, node.Text.String(), path)
		}

	default:
		c.addIssue(path, "schema", "unsupported type declaration")
	}
}

func (c *validationContext) resolveType(
	reference *validationTypeRef,
) (
	*validationSimpleSchema,
	*validationComplexSchema,
	string,
	bool,
) {
	if reference == nil {
		return nil, nil, "", false
	}
	if reference.InlineSimple != nil {
		return reference.InlineSimple, nil, "", true
	}
	if reference.InlineComplex != nil {
		return nil, reference.InlineComplex, "", true
	}
	if reference.Name.Local == "" {
		return nil, nil, "", false
	}
	if reference.Name.Space == validationXSDNamespace {
		return nil, nil, reference.Name.Local, true
	}

	resolved, found := c.schema.Types[reference.Name]
	if !found || resolved == nil {
		return nil, nil, "", false
	}

	return resolved.Simple, resolved.Complex, "", true
}

func (c *validationContext) validateComplex(
	node *validationNode,
	schema *validationComplexSchema,
	path string,
) {
	effective, ok := c.effectiveComplex(schema)
	if !ok {
		c.addIssue(path, "schema", "complex type cannot be resolved")
		return
	}

	c.validateAttributes(
		node,
		effective.attributes,
		effective.anyAttribute,
		path,
	)

	if effective.simple != nil {
		if len(node.Children) != 0 {
			c.addIssue(
				path,
				"simple-content",
				"simple-content value must not contain child elements",
			)
			return
		}
		c.validateType(node, effective.simple, path)
		return
	}

	if !effective.mixed &&
		strings.TrimSpace(node.Text.String()) != "" {
		c.addIssue(
			path,
			"element-only",
			"character data is not allowed",
		)
	}

	if effective.particle == nil {
		if len(node.Children) != 0 {
			c.addIssue(
				validationChildPath(path, node.Children[0], 1),
				"content-model",
				fmt.Sprintf(
					"unexpected element %s",
					validationDisplayName(node.Children[0].Name),
				),
			)
		}
		return
	}

	result, failure, matched := c.matchParticle(
		effective.particle,
		node.Children,
		0,
	)
	if !matched || result.next != len(node.Children) {
		if !matched {
			c.addContentIssue(node, path, failure)
			return
		}

		index := result.next
		c.addIssue(
			validationIndexedChildPath(path, node.Children, index),
			"content-model",
			fmt.Sprintf(
				"unexpected element %s",
				validationDisplayName(node.Children[index].Name),
			),
		)
		return
	}

	paths := validationChildPaths(path, node.Children)
	for _, match := range result.elements {
		c.validateElement(
			node.Children[match.index],
			match.schema,
			paths[match.index],
		)
	}
}

func (c *validationContext) effectiveComplex(
	schema *validationComplexSchema,
) (*validationEffectiveComplex, bool) {
	if schema == nil {
		return nil, false
	}
	if cached, found := c.effective[schema]; found {
		return cached, true
	}

	result := &validationEffectiveComplex{
		particle:     schema.Particle,
		attributes:   append([]validationAttributeSchema(nil), schema.Attributes...),
		anyAttribute: schema.AnyAttribute,
		mixed:        schema.Mixed,
	}
	// Install before resolving a base so malformed cyclic derivations cannot
	// recurse forever.
	c.effective[schema] = result

	switch schema.Form {
	case validationComplexDirect:
		return result, true

	case validationComplexSimpleContentExtension,
		validationComplexSimpleContentRestriction:
		if schema.Base == nil {
			return nil, false
		}

		simple, baseComplex, builtin, found := c.resolveType(schema.Base)
		if !found {
			return nil, false
		}
		switch {
		case simple != nil || builtin != "":
			result.simple = schema.Base
		case baseComplex != nil:
			base, ok := c.effectiveComplex(baseComplex)
			if !ok || base.simple == nil {
				return nil, false
			}
			result.simple = base.simple
			result.attributes = mergeValidationAttributes(
				base.attributes,
				result.attributes,
			)
			result.anyAttribute = mergeValidationAnyAttribute(
				base.anyAttribute,
				result.anyAttribute,
			)
			result.mixed = result.mixed || base.mixed
		default:
			return nil, false
		}

		return result, true

	case validationComplexComplexContentExtension:
		if schema.Base == nil {
			return nil, false
		}
		_, baseComplex, _, found := c.resolveType(schema.Base)
		if !found || baseComplex == nil {
			return nil, false
		}
		base, ok := c.effectiveComplex(baseComplex)
		if !ok || base.simple != nil {
			return nil, false
		}

		result.particle = validationSequence(
			base.particle,
			result.particle,
		)
		result.attributes = mergeValidationAttributes(
			base.attributes,
			result.attributes,
		)
		result.anyAttribute = mergeValidationAnyAttribute(
			base.anyAttribute,
			result.anyAttribute,
		)
		result.mixed = result.mixed || base.mixed

		return result, true

	case validationComplexComplexContentRestriction:
		return result, true

	default:
		return nil, false
	}
}

func mergeValidationAttributes(
	base []validationAttributeSchema,
	derived []validationAttributeSchema,
) []validationAttributeSchema {
	result := append([]validationAttributeSchema(nil), base...)
	indexes := make(map[validationQName]int, len(result))
	for index := range result {
		indexes[result[index].Name] = index
	}

	for _, attribute := range derived {
		if index, found := indexes[attribute.Name]; found {
			result[index] = attribute
			continue
		}
		indexes[attribute.Name] = len(result)
		result = append(result, attribute)
	}

	return result
}

func mergeValidationAnyAttribute(
	base *validationAnyAttributeSchema,
	derived *validationAnyAttributeSchema,
) *validationAnyAttributeSchema {
	if derived != nil {
		return derived
	}
	return base
}

func validationSequence(
	left *validationParticleSchema,
	right *validationParticleSchema,
) *validationParticleSchema {
	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	default:
		return &validationParticleSchema{
			Kind: validationParticleSequence,
			Occurrence: validationOccurrence{
				Min: 1,
				Max: 1,
			},
			Children: []*validationParticleSchema{left, right},
		}
	}
}

func (c *validationContext) validateAttributes(
	node *validationNode,
	schemas []validationAttributeSchema,
	anyAttribute *validationAnyAttributeSchema,
	path string,
) {
	known := make(map[validationQName]validationAttributeSchema, len(schemas))
	present := make(map[validationQName]xml.Attr, len(node.Attrs))
	for _, schema := range schemas {
		known[schema.Name] = schema
	}

	for _, attribute := range node.Attrs {
		if validationNamespaceDeclaration(attribute) {
			continue
		}
		if attribute.Name.Space == validationXSINamespace &&
			attribute.Name.Local == "nil" {
			continue
		}

		name := validationName(attribute.Name)
		schema, found := known[name]
		if !found {
			if validationAnyAttributeAllows(anyAttribute, attribute.Name) {
				continue
			}
			c.addIssue(
				validationAttributePath(path, attribute.Name),
				"attribute",
				"attribute is not allowed",
			)
			continue
		}
		present[name] = attribute

		if schema.Use == validationAttributeProhibited {
			c.addIssue(
				validationAttributePath(path, attribute.Name),
				"prohibited",
				"attribute is prohibited",
			)
			continue
		}

		if schema.Constraint != nil &&
			schema.Constraint.Kind == "fixed" &&
			!c.simpleValuesEqual(
				&schema.Type,
				attribute.Value,
				schema.Constraint.Value,
			) {
			c.addIssue(
				validationAttributePath(path, attribute.Name),
				"fixed",
				fmt.Sprintf(
					"value %q does not equal fixed value %q",
					attribute.Value,
					schema.Constraint.Value,
				),
			)
		}

		if failure := c.validateSimpleType(
			&schema.Type,
			attribute.Value,
		); failure != nil {
			c.addIssue(
				validationAttributePath(path, attribute.Name),
				failure.constraint,
				failure.message,
			)
		} else {
			c.recordIdentity(
				&schema.Type,
				attribute.Value,
				validationAttributePath(path, attribute.Name),
			)
		}
	}

	for _, schema := range schemas {
		if schema.Use != validationAttributeRequired {
			continue
		}
		if _, found := present[schema.Name]; found {
			continue
		}

		c.addIssue(
			validationAttributeQNamePath(path, schema.Name),
			"required",
			"required attribute is missing",
		)
	}
}

func (c *validationContext) validateSimpleType(
	reference *validationTypeRef,
	value string,
) *validationSimpleFailure {
	simple, complex, builtin, found := c.resolveType(reference)
	if !found || complex != nil {
		return &validationSimpleFailure{
			constraint: "schema",
			message:    "simple type cannot be resolved",
		}
	}
	if simple != nil {
		return c.validateSimple(simple, value)
	}
	return validateBuiltin(builtin, value)
}

func (c *validationContext) simpleValuesEqual(
	reference *validationTypeRef,
	left string,
	right string,
) bool {
	builtin := c.simpleBuiltin(reference)
	return normalizeValidationWhitespace(
		builtin,
		left,
	) == normalizeValidationWhitespace(
		builtin,
		right,
	)
}

func (c *validationContext) recordIdentity(
	reference *validationTypeRef,
	value string,
	path string,
) {
	builtin := c.simpleBuiltin(reference)
	normalized := normalizeValidationWhitespace(builtin, value)

	switch builtin {
	case "ID":
		if firstPath, found := c.identifiers[normalized]; found {
			c.addIssue(
				path,
				"ID",
				fmt.Sprintf(
					"duplicate ID %q; first declared at %s",
					normalized,
					firstPath,
				),
			)
			return
		}
		c.identifiers[normalized] = path

	case "IDREF":
		c.references = append(
			c.references,
			validationIdentityReference{
				value: normalized,
				path:  path,
			},
		)

	case "IDREFS":
		for _, item := range strings.Fields(normalized) {
			c.references = append(
				c.references,
				validationIdentityReference{
					value: item,
					path:  path,
				},
			)
		}
	}
}

func (c *validationContext) validateIdentityReferences() {
	for _, reference := range c.references {
		if _, found := c.identifiers[reference.value]; found {
			continue
		}
		c.addIssue(
			reference.path,
			"IDREF",
			fmt.Sprintf(
				"IDREF %q does not identify an element in the document",
				reference.value,
			),
		)
	}
}

func (c *validationContext) validateSimple(
	schema *validationSimpleSchema,
	value string,
) *validationSimpleFailure {
	if schema == nil {
		return &validationSimpleFailure{
			constraint: "schema",
			message:    "simple type is missing",
		}
	}

	switch schema.Form {
	case validationSimpleRestriction:
		if schema.Base == nil {
			return &validationSimpleFailure{
				constraint: "schema",
				message:    "restriction base is missing",
			}
		}
		if failure := c.validateSimpleMember(schema.Base, value); failure != nil {
			return failure
		}

		builtin := c.simpleMemberBuiltin(schema.Base)
		normalized := normalizeValidationWhitespace(builtin, value)

		if len(schema.Enumerations) != 0 &&
			!slices.ContainsFunc(
				schema.Enumerations,
				func(candidate string) bool {
					return normalizeValidationWhitespace(
						builtin,
						candidate,
					) == normalized
				},
			) {
			return &validationSimpleFailure{
				constraint: "enumeration",
				message: fmt.Sprintf(
					"value %q is not one of the allowed values",
					value,
				),
			}
		}

		for _, pattern := range schema.Patterns {
			matched, err := matchValidationPattern(pattern, normalized)
			if err != nil {
				return &validationSimpleFailure{
					constraint: "schema",
					message: fmt.Sprintf(
						"invalid generated XSD pattern %q: %v",
						pattern,
						err,
					),
				}
			}
			if !matched {
				return &validationSimpleFailure{
					constraint: "pattern",
					message: fmt.Sprintf(
						"value %q does not match XSD pattern %q",
						value,
						pattern,
					),
				}
			}
		}

		if failure := validateBounds(schema, normalized); failure != nil {
			return failure
		}
		if failure := validateLengthFacets(
			schema,
			normalized,
			c.simpleMemberList(schema.Base),
		); failure != nil {
			return failure
		}
		if failure := validateDigitFacets(schema, normalized); failure != nil {
			return failure
		}

		return nil

	case validationSimpleUnion:
		for index := range schema.Members {
			if c.validateSimpleMember(
				schema.Members[index],
				value,
			) == nil {
				return nil
			}
		}

		return &validationSimpleFailure{
			constraint: "union",
			message: fmt.Sprintf(
				"value %q does not match any union member",
				value,
			),
		}

	case validationSimpleList:
		if schema.Item == nil {
			return &validationSimpleFailure{
				constraint: "schema",
				message:    "list item type is missing",
			}
		}
		for index, item := range strings.Fields(value) {
			if failure := c.validateSimpleMember(
				schema.Item,
				item,
			); failure != nil {
				return &validationSimpleFailure{
					constraint: failure.constraint,
					message: fmt.Sprintf(
						"list item %d: %s",
						index+1,
						failure.message,
					),
				}
			}
		}
		return nil

	default:
		return &validationSimpleFailure{
			constraint: "schema",
			message:    "unsupported simple type form",
		}
	}
}

func (c *validationContext) validateSimpleMember(
	member *validationSimpleMember,
	value string,
) *validationSimpleFailure {
	if member == nil {
		return &validationSimpleFailure{
			constraint: "schema",
			message:    "simple type member is missing",
		}
	}
	if member.Inline != nil {
		return c.validateSimple(member.Inline, value)
	}
	if member.Name.Space == validationXSDNamespace {
		return validateBuiltin(member.Name.Local, value)
	}

	resolved, found := c.schema.Types[member.Name]
	if !found || resolved == nil || resolved.Simple == nil {
		return &validationSimpleFailure{
			constraint: "schema",
			message:    "simple type member cannot be resolved",
		}
	}

	return c.validateSimple(resolved.Simple, value)
}

func (c *validationContext) simpleBuiltin(
	reference *validationTypeRef,
) string {
	if reference == nil {
		return ""
	}
	if reference.InlineSimple != nil {
		return c.simpleSchemaBuiltin(reference.InlineSimple)
	}
	if reference.Name.Space == validationXSDNamespace {
		return reference.Name.Local
	}

	resolved, found := c.schema.Types[reference.Name]
	if !found || resolved == nil || resolved.Simple == nil {
		return ""
	}
	return c.simpleSchemaBuiltin(resolved.Simple)
}

func (c *validationContext) simpleSchemaBuiltin(
	schema *validationSimpleSchema,
) string {
	if schema == nil {
		return ""
	}

	switch schema.Form {
	case validationSimpleRestriction:
		return c.simpleMemberBuiltin(schema.Base)
	case validationSimpleUnion, validationSimpleList:
		return "string"
	default:
		return ""
	}
}

func (c *validationContext) simpleMemberBuiltin(
	member *validationSimpleMember,
) string {
	if member == nil {
		return ""
	}
	if member.Inline != nil {
		return c.simpleSchemaBuiltin(member.Inline)
	}
	if member.Name.Space == validationXSDNamespace {
		return member.Name.Local
	}

	resolved, found := c.schema.Types[member.Name]
	if !found || resolved == nil || resolved.Simple == nil {
		return ""
	}
	return c.simpleSchemaBuiltin(resolved.Simple)
}

func (c *validationContext) simpleMemberList(
	member *validationSimpleMember,
) bool {
	if member == nil {
		return false
	}
	if member.Inline != nil {
		return c.simpleSchemaList(member.Inline)
	}
	if member.Name.Space == validationXSDNamespace {
		return false
	}

	resolved, found := c.schema.Types[member.Name]
	return found &&
		resolved != nil &&
		resolved.Simple != nil &&
		c.simpleSchemaList(resolved.Simple)
}

func (c *validationContext) simpleSchemaList(
	schema *validationSimpleSchema,
) bool {
	if schema == nil {
		return false
	}
	switch schema.Form {
	case validationSimpleList:
		return true
	case validationSimpleRestriction:
		return c.simpleMemberList(schema.Base)
	default:
		return false
	}
}

func validateBuiltin(
	name string,
	value string,
) *validationSimpleFailure {
	normalized := normalizeValidationWhitespace(name, value)

	switch name {
	case "anySimpleType", "string", "normalizedString", "token":
		return nil

	case "anyURI":
		if _, err := url.Parse(normalized); err == nil {
			return nil
		}

	case "language":
		if validationLanguagePattern.MatchString(normalized) {
			return nil
		}

	case "Name":
		if validXMLName(normalized) {
			return nil
		}

	case "NCName", "ID", "IDREF", "ENTITY":
		if validXMLNCName(normalized) {
			return nil
		}

	case "NMTOKEN":
		if validXMLNMTOKEN(normalized) {
			return nil
		}

	case "NMTOKENS":
		items := strings.Fields(normalized)
		if len(items) != 0 &&
			slices.ContainsFunc(items, func(item string) bool {
				return !validXMLNMTOKEN(item)
			}) == false {
			return nil
		}

	case "IDREFS", "ENTITIES":
		items := strings.Fields(normalized)
		if len(items) != 0 &&
			slices.ContainsFunc(items, func(item string) bool {
				return !validXMLNCName(item)
			}) == false {
			return nil
		}

	case "boolean":
		switch normalized {
		case "true", "false", "1", "0":
			return nil
		}

	case "decimal":
		if validationDecimalPattern.MatchString(normalized) {
			return nil
		}

	case "float", "double":
		if normalized == "INF" ||
			normalized == "-INF" ||
			normalized == "NaN" {
			return nil
		}
		if _, err := strconv.ParseFloat(normalized, 64); err == nil {
			return nil
		}

	case "integer", "long", "int", "short", "byte":
		if _, err := strconv.ParseInt(
			normalized,
			10,
			validationSignedBits(name),
		); err == nil {
			return nil
		}

	case "nonPositiveInteger", "negativeInteger":
		number, err := strconv.ParseInt(normalized, 10, 64)
		if err == nil &&
			((name == "nonPositiveInteger" && number <= 0) ||
				(name == "negativeInteger" && number < 0)) {
			return nil
		}

	case "nonNegativeInteger", "positiveInteger", "unsignedLong",
		"unsignedInt", "unsignedShort", "unsignedByte":
		number, err := strconv.ParseUint(
			normalized,
			10,
			validationUnsignedBits(name),
		)
		if err == nil &&
			(name != "positiveInteger" || number > 0) {
			return nil
		}

	case "base64Binary":
		if validationBase64Pattern.MatchString(normalized) {
			return nil
		}

	case "hexBinary":
		if len(normalized)%2 == 0 &&
			validationHexPattern.MatchString(normalized) {
			return nil
		}

	case "QName", "NOTATION":
		if validXMLQName(normalized) {
			return nil
		}

	case "date", "dateTime", "duration", "gDay", "gMonth",
		"gMonthDay", "gYear", "gYearMonth", "time":
		if validXSDDateTime(name, normalized) {
			return nil
		}

	case "anyType":
		return nil

	default:
		return &validationSimpleFailure{
			constraint: "schema",
			message:    fmt.Sprintf("unsupported XSD built-in type %q", name),
		}
	}

	return &validationSimpleFailure{
		constraint: "datatype",
		message: fmt.Sprintf(
			"value %q is not valid for xs:%s",
			value,
			name,
		),
	}
}

func validationSignedBits(name string) int {
	switch name {
	case "byte":
		return 8
	case "short":
		return 16
	case "int":
		return 32
	default:
		return 64
	}
}

func validationUnsignedBits(name string) int {
	switch name {
	case "unsignedByte":
		return 8
	case "unsignedShort":
		return 16
	case "unsignedInt":
		return 32
	default:
		return 64
	}
}

func normalizeValidationWhitespace(name string, value string) string {
	switch name {
	case "string", "anySimpleType":
		return value
	case "normalizedString":
		return strings.NewReplacer(
			"\t", " ",
			"\n", " ",
			"\r", " ",
		).Replace(value)
	default:
		return strings.Join(strings.Fields(value), " ")
	}
}

func validateBounds(
	schema *validationSimpleSchema,
	value string,
) *validationSimpleFailure {
	if !schema.HasMinInclusive &&
		!schema.HasMaxInclusive &&
		!schema.HasMinExclusive &&
		!schema.HasMaxExclusive {
		return nil
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) {
		return &validationSimpleFailure{
			constraint: "datatype",
			message:    fmt.Sprintf("value %q is not numeric", value),
		}
	}

	tests := []struct {
		enabled    bool
		constraint string
		limit      string
		valid      func(float64, float64) bool
	}{
		{schema.HasMinInclusive, "minInclusive", schema.MinInclusive, func(value, limit float64) bool {
			return value >= limit
		}},
		{schema.HasMaxInclusive, "maxInclusive", schema.MaxInclusive, func(value, limit float64) bool {
			return value <= limit
		}},
		{schema.HasMinExclusive, "minExclusive", schema.MinExclusive, func(value, limit float64) bool {
			return value > limit
		}},
		{schema.HasMaxExclusive, "maxExclusive", schema.MaxExclusive, func(value, limit float64) bool {
			return value < limit
		}},
	}
	for _, test := range tests {
		if !test.enabled {
			continue
		}
		limit, err := strconv.ParseFloat(test.limit, 64)
		if err != nil {
			return &validationSimpleFailure{
				constraint: "schema",
				message: fmt.Sprintf(
					"invalid %s value %q",
					test.constraint,
					test.limit,
				),
			}
		}
		if !test.valid(number, limit) {
			return &validationSimpleFailure{
				constraint: test.constraint,
				message: fmt.Sprintf(
					"value %q violates %s=%q",
					value,
					test.constraint,
					test.limit,
				),
			}
		}
	}

	return nil
}

func validateLengthFacets(
	schema *validationSimpleSchema,
	value string,
	list bool,
) *validationSimpleFailure {
	if !schema.HasLength &&
		!schema.HasMinLength &&
		!schema.HasMaxLength {
		return nil
	}

	length := uint64(utf8.RuneCountInString(value))
	if list {
		length = uint64(len(strings.Fields(value)))
	}

	switch {
	case schema.HasLength && length != schema.Length:
		return &validationSimpleFailure{
			constraint: "length",
			message: fmt.Sprintf(
				"value length %d does not equal %d",
				length,
				schema.Length,
			),
		}
	case schema.HasMinLength && length < schema.MinLength:
		return &validationSimpleFailure{
			constraint: "minLength",
			message: fmt.Sprintf(
				"value length %d is less than %d",
				length,
				schema.MinLength,
			),
		}
	case schema.HasMaxLength && length > schema.MaxLength:
		return &validationSimpleFailure{
			constraint: "maxLength",
			message: fmt.Sprintf(
				"value length %d is greater than %d",
				length,
				schema.MaxLength,
			),
		}
	default:
		return nil
	}
}

func validateDigitFacets(
	schema *validationSimpleSchema,
	value string,
) *validationSimpleFailure {
	if !schema.HasTotalDigits && !schema.HasFractionDigits {
		return nil
	}

	normalized := strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
	parts := strings.SplitN(normalized, ".", 2)
	total := uint64(0)
	for _, character := range normalized {
		if unicode.IsDigit(character) {
			total++
		}
	}
	fraction := uint64(0)
	if len(parts) == 2 {
		fraction = uint64(utf8.RuneCountInString(parts[1]))
	}

	if schema.HasTotalDigits && total > schema.TotalDigits {
		return &validationSimpleFailure{
			constraint: "totalDigits",
			message: fmt.Sprintf(
				"value has %d digits; maximum is %d",
				total,
				schema.TotalDigits,
			),
		}
	}
	if schema.HasFractionDigits && fraction > schema.FractionDigits {
		return &validationSimpleFailure{
			constraint: "fractionDigits",
			message: fmt.Sprintf(
				"value has %d fraction digits; maximum is %d",
				fraction,
				schema.FractionDigits,
			),
		}
	}

	return nil
}

func matchValidationPattern(pattern string, value string) (bool, error) {
	translated := strings.ReplaceAll(
		pattern,
		`\c`,
		`[A-Za-z0-9_.:-]`,
	)
	translated = strings.ReplaceAll(
		translated,
		`\i`,
		`[A-Za-z_:]`,
	)

	compiled, err := regexp.Compile(`^(?:` + translated + `)$`)
	if err != nil {
		return false, err
	}

	return compiled.MatchString(value), nil
}

func (c *validationContext) matchParticle(
	particle *validationParticleSchema,
	children []*validationNode,
	position int,
) (validationMatchResult, validationMatchFailure, bool) {
	if particle == nil {
		return validationMatchResult{next: position}, validationMatchFailure{}, true
	}

	result := validationMatchResult{next: position}
	bestFailure := validationMatchFailure{position: position}
	count := uint64(0)
	maximum := particle.Occurrence.Max
	if particle.Occurrence.Unbounded {
		maximum = uint64(len(children) + 1)
	}

	for count < maximum {
		next, failure, matched := c.matchParticleBody(
			particle,
			children,
			result.next,
		)
		if !matched {
			bestFailure = betterValidationFailure(bestFailure, failure)
			break
		}
		if next.next == result.next {
			// A required sequence whose children are all optional represents
			// one valid empty occurrence. Count it once, then stop so an
			// unbounded empty particle cannot loop forever.
			count++
			break
		}

		result.next = next.next
		result.elements = append(result.elements, next.elements...)
		count++
	}

	if count < particle.Occurrence.Min {
		if len(bestFailure.expected) == 0 {
			bestFailure.expected = validationParticleFirstNames(particle)
		}
		bestFailure.position = max(bestFailure.position, result.next)
		return validationMatchResult{}, bestFailure, false
	}

	return result, bestFailure, true
}

func (c *validationContext) matchParticleBody(
	particle *validationParticleSchema,
	children []*validationNode,
	position int,
) (validationMatchResult, validationMatchFailure, bool) {
	switch particle.Kind {
	case validationParticleElement:
		element, found := c.resolveElement(particle.Element)
		if !found {
			return validationMatchResult{}, validationMatchFailure{
				position: position,
			}, false
		}
		if position >= len(children) ||
			validationName(children[position].Name) != element.Name {
			return validationMatchResult{}, validationMatchFailure{
				position: position,
				expected: []validationQName{element.Name},
			}, false
		}

		return validationMatchResult{
			next: position + 1,
			elements: []validationElementMatch{{
				index:  position,
				schema: element,
			}},
		}, validationMatchFailure{}, true

	case validationParticleSequence:
		result := validationMatchResult{next: position}
		bestFailure := validationMatchFailure{position: position}
		for _, child := range particle.Children {
			next, failure, matched := c.matchParticle(
				child,
				children,
				result.next,
			)
			bestFailure = betterValidationFailure(bestFailure, failure)
			if !matched {
				return validationMatchResult{}, bestFailure, false
			}
			result.next = next.next
			result.elements = append(
				result.elements,
				next.elements...,
			)
		}
		return result, bestFailure, true

	case validationParticleChoice:
		bestFailure := validationMatchFailure{position: position}
		var empty *validationMatchResult
		for _, child := range particle.Children {
			result, failure, matched := c.matchParticle(
				child,
				children,
				position,
			)
			bestFailure = betterValidationFailure(bestFailure, failure)
			if !matched {
				continue
			}
			if result.next > position {
				return result, bestFailure, true
			}
			if empty == nil {
				copy := result
				empty = &copy
			}
		}
		if empty != nil {
			return *empty, bestFailure, true
		}
		if len(bestFailure.expected) == 0 {
			bestFailure.expected = validationParticleFirstNames(particle)
		}
		return validationMatchResult{}, bestFailure, false

	case validationParticleAll:
		return c.matchAllParticle(particle, children, position)

	case validationParticleAny:
		if position >= len(children) {
			return validationMatchResult{}, validationMatchFailure{
				position: position,
			}, false
		}
		return validationMatchResult{
			next: position + 1,
		}, validationMatchFailure{}, true

	default:
		return validationMatchResult{}, validationMatchFailure{
			position: position,
		}, false
	}
}

func (c *validationContext) matchAllParticle(
	particle *validationParticleSchema,
	children []*validationNode,
	position int,
) (validationMatchResult, validationMatchFailure, bool) {
	result := validationMatchResult{next: position}
	used := make([]bool, len(particle.Children))

	for result.next < len(children) {
		matchedChild := false
		for index, child := range particle.Children {
			if used[index] {
				continue
			}
			next, _, matched := c.matchParticle(
				child,
				children,
				result.next,
			)
			if !matched || next.next == result.next {
				continue
			}

			used[index] = true
			result.next = next.next
			result.elements = append(
				result.elements,
				next.elements...,
			)
			matchedChild = true
			break
		}
		if !matchedChild {
			break
		}
	}

	var expected []validationQName
	for index, child := range particle.Children {
		if used[index] || child.Occurrence.Min == 0 {
			continue
		}
		expected = append(
			expected,
			validationParticleFirstNames(child)...,
		)
	}
	if len(expected) != 0 {
		return validationMatchResult{}, validationMatchFailure{
			position: result.next,
			expected: expected,
		}, false
	}

	return result, validationMatchFailure{}, true
}

func validationParticleFirstNames(
	particle *validationParticleSchema,
) []validationQName {
	if particle == nil {
		return nil
	}

	switch particle.Kind {
	case validationParticleElement:
		if particle.Element == nil {
			return nil
		}
		if particle.Element.Reference.Local != "" {
			return []validationQName{particle.Element.Reference}
		}
		return []validationQName{particle.Element.Name}
	case validationParticleAll, validationParticleChoice:
		var result []validationQName
		for _, child := range particle.Children {
			result = append(result, validationParticleFirstNames(child)...)
		}
		return uniqueValidationNames(result)
	case validationParticleSequence:
		var result []validationQName
		for _, child := range particle.Children {
			result = append(result, validationParticleFirstNames(child)...)
			if child.Occurrence.Min > 0 {
				break
			}
		}
		return uniqueValidationNames(result)
	default:
		return nil
	}
}

func betterValidationFailure(
	left validationMatchFailure,
	right validationMatchFailure,
) validationMatchFailure {
	switch {
	case right.position > left.position:
		return right
	case right.position < left.position:
		return left
	default:
		left.expected = uniqueValidationNames(
			append(left.expected, right.expected...),
		)
		return left
	}
}

func uniqueValidationNames(
	values []validationQName,
) []validationQName {
	result := make([]validationQName, 0, len(values))
	seen := make(map[validationQName]struct{}, len(values))
	for _, value := range values {
		if value.Local == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	slices.SortFunc(result, func(left, right validationQName) int {
		if comparison := strings.Compare(left.Space, right.Space); comparison != 0 {
			return comparison
		}
		return strings.Compare(left.Local, right.Local)
	})
	return result
}

func (c *validationContext) addContentIssue(
	node *validationNode,
	path string,
	failure validationMatchFailure,
) {
	position := min(failure.position, len(node.Children))
	expected := validationExpectedNames(failure.expected)

	if position == len(node.Children) {
		message := "required child element is missing"
		if expected != "" {
			message += "; expected " + expected
		}
		c.addIssue(path, "content-model", message)
		return
	}

	message := fmt.Sprintf(
		"unexpected element %s",
		validationDisplayName(node.Children[position].Name),
	)
	if expected != "" {
		message += "; expected " + expected
	}
	c.addIssue(
		validationIndexedChildPath(path, node.Children, position),
		"content-model",
		message,
	)
}

func validationExpectedNames(names []validationQName) string {
	if len(names) == 0 {
		return ""
	}
	values := make([]string, len(names))
	for index, name := range names {
		values[index] = validationDisplayQName(name)
	}
	return strings.Join(values, " or ")
}

func (c *validationContext) addIssue(
	path string,
	constraint string,
	message string,
) {
	c.issues = append(c.issues, ValidationIssue{
		Path:       path,
		Constraint: constraint,
		Message:    message,
	})
}

func validationAnyAttributeAllows(
	schema *validationAnyAttributeSchema,
	name xml.Name,
) bool {
	if schema == nil {
		return false
	}
	switch schema.Namespace {
	case "", "##any":
		return true
	case "##other":
		return name.Space != ""
	case "##local":
		return name.Space == ""
	default:
		for _, namespace := range strings.Fields(schema.Namespace) {
			if namespace == name.Space {
				return true
			}
		}
		return false
	}
}

func validationNilAttribute(
	attributes []xml.Attr,
) (bool, bool) {
	for _, attribute := range attributes {
		if attribute.Name.Space != validationXSINamespace ||
			attribute.Name.Local != "nil" {
			continue
		}

		value := strings.TrimSpace(attribute.Value)
		return value == "true" || value == "1", true
	}
	return false, false
}

func validationNamespaceDeclaration(attribute xml.Attr) bool {
	return attribute.Name.Space == "xmlns" ||
		(attribute.Name.Space == "" && attribute.Name.Local == "xmlns")
}

func validationName(name xml.Name) validationQName {
	return validationQName{
		Space: name.Space,
		Local: name.Local,
	}
}

func validationString(value string) *string {
	return &value
}

func validationDisplayName(name xml.Name) string {
	return validationDisplayQName(validationName(name))
}

func validationDisplayQName(name validationQName) string {
	if name.Space == "" {
		return "<" + name.Local + ">"
	}
	return fmt.Sprintf("<{%s}%s>", name.Space, name.Local)
}

func validationPath(parent string, local string, index int) string {
	if parent == "" {
		return "/" + local
	}
	if index <= 1 {
		return parent + "/" + local
	}
	return fmt.Sprintf("%s/%s[%d]", parent, local, index)
}

func validationChildPath(
	parent string,
	child *validationNode,
	index int,
) string {
	return validationPath(parent, child.Name.Local, index)
}

func validationChildPaths(
	parent string,
	children []*validationNode,
) []string {
	result := make([]string, len(children))
	counts := make(map[xml.Name]int)
	for index, child := range children {
		counts[child.Name]++
		result[index] = validationChildPath(
			parent,
			child,
			counts[child.Name],
		)
	}
	return result
}

func validationIndexedChildPath(
	parent string,
	children []*validationNode,
	index int,
) string {
	if index < 0 || index >= len(children) {
		return parent
	}
	count := 0
	for current := 0; current <= index; current++ {
		if children[current].Name == children[index].Name {
			count++
		}
	}
	return validationChildPath(parent, children[index], count)
}

func validationAttributePath(parent string, name xml.Name) string {
	if name.Space == "" {
		return parent + "/@" + name.Local
	}
	return fmt.Sprintf("%s/@{%s}%s", parent, name.Space, name.Local)
}

func validationAttributeQNamePath(
	parent string,
	name validationQName,
) string {
	return validationAttributePath(parent, xml.Name{
		Space: name.Space,
		Local: name.Local,
	})
}

var (
	validationLanguagePattern = regexp.MustCompile(
		`^[A-Za-z]{1,8}(?:-[A-Za-z0-9]{1,8})*$`,
	)
	validationDecimalPattern = regexp.MustCompile(
		`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`,
	)
	validationBase64Pattern = regexp.MustCompile(
		`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`,
	)
	validationHexPattern = regexp.MustCompile(`^[0-9A-Fa-f]*$`)
)
