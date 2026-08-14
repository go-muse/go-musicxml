package xsdgen

import (
	"errors"
	"fmt"
)

var (
	ErrNilSet               = errors.New("xsdgen: nil schema set")
	ErrNilIndex             = errors.New("xsdgen: nil declaration index")
	ErrInvalidSchemaFile    = errors.New("xsdgen: invalid schema file")
	ErrUnnamedDeclaration   = errors.New("xsdgen: unnamed global declaration")
	ErrDuplicateDeclaration = errors.New("xsdgen: duplicate global declaration")
	ErrUnresolvedReference  = errors.New("xsdgen: unresolved declaration reference")
)

// DeclarationKind identifies one kind of global XSD declaration.
type DeclarationKind string

const (
	DeclarationBuiltinSimpleType  DeclarationKind = "built-in simple type"
	DeclarationBuiltinComplexType DeclarationKind = "built-in complex type"
	DeclarationSimpleType         DeclarationKind = "simple type"
	DeclarationComplexType        DeclarationKind = "complex type"
	DeclarationElement            DeclarationKind = "element"
	DeclarationAttribute          DeclarationKind = "attribute"
	DeclarationGroup              DeclarationKind = "group"
	DeclarationAttributeGroup     DeclarationKind = "attribute group"
)

// SymbolSpace identifies an XSD declaration namespace.
//
// Simple and complex types share the same symbol space. The remaining
// declaration kinds each have their own symbol space.
type SymbolSpace string

const (
	TypeSymbolSpace           SymbolSpace = "type"
	ElementSymbolSpace        SymbolSpace = "element"
	AttributeSymbolSpace      SymbolSpace = "attribute"
	GroupSymbolSpace          SymbolSpace = "group"
	AttributeGroupSymbolSpace SymbolSpace = "attribute group"
)

// Declaration associates a global XSD declaration with its expanded name and
// source schema. Exactly one declaration pointer is set for non-built-in
// declarations.
type Declaration struct {
	Name QName
	Kind DeclarationKind
	File *SchemaFile

	SimpleType     *SimpleType
	ComplexType    *ComplexType
	Element        *Element
	Attribute      *Attribute
	Group          *Group
	AttributeGroup *AttributeGroup
}

// Builtin reports whether the declaration is supplied by XML Schema itself.
func (d *Declaration) Builtin() bool {
	if d == nil {
		return false
	}

	return d.Kind == DeclarationBuiltinSimpleType ||
		d.Kind == DeclarationBuiltinComplexType
}

// DuplicateDeclarationError reports two declarations in the same XSD symbol
// space with the same expanded name.
type DuplicateDeclarationError struct {
	Space     SymbolSpace
	Name      QName
	Existing  *Declaration
	Duplicate *Declaration
}

func (e *DuplicateDeclarationError) Error() string {
	return fmt.Sprintf(
		"%v: %s %s in %q and %q",
		ErrDuplicateDeclaration,
		e.Space,
		formatExpandedName(e.Name),
		declarationPath(e.Existing),
		declarationPath(e.Duplicate),
	)
}

func (e *DuplicateDeclarationError) Unwrap() error {
	return ErrDuplicateDeclaration
}

// UnresolvedReferenceError reports a QName that does not identify a loaded
// declaration in the required symbol space.
type UnresolvedReferenceError struct {
	Space   SymbolSpace
	Lexical string
	Name    QName
	From    string
}

func (e *UnresolvedReferenceError) Error() string {
	return fmt.Sprintf(
		"%v: %s %q (%s) from %q",
		ErrUnresolvedReference,
		e.Space,
		e.Lexical,
		formatExpandedName(e.Name),
		e.From,
	)
}

func (e *UnresolvedReferenceError) Unwrap() error {
	return ErrUnresolvedReference
}

// Index contains all global declarations in a loaded schema set.
type Index struct {
	types           map[expandedName]*Declaration
	elements        map[expandedName]*Declaration
	attributes      map[expandedName]*Declaration
	groups          map[expandedName]*Declaration
	attributeGroups map[expandedName]*Declaration
}

type expandedName struct {
	namespace string
	local     string
}

// NewIndex indexes every global declaration in set.
func NewIndex(set *Set) (*Index, error) {
	if set == nil {
		return nil, ErrNilSet
	}

	result := &Index{
		types:           make(map[expandedName]*Declaration),
		elements:        make(map[expandedName]*Declaration),
		attributes:      make(map[expandedName]*Declaration),
		groups:          make(map[expandedName]*Declaration),
		attributeGroups: make(map[expandedName]*Declaration),
	}

	result.addBuiltinTypes()

	for _, file := range set.Files {
		if err := result.addFile(file); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// LookupType returns a simple, complex, or built-in type by expanded name.
func (i *Index) LookupType(name QName) (*Declaration, bool) {
	return lookup(i, TypeSymbolSpace, name)
}

// LookupElement returns a global element by expanded name.
func (i *Index) LookupElement(name QName) (*Declaration, bool) {
	return lookup(i, ElementSymbolSpace, name)
}

// LookupAttribute returns a global attribute by expanded name.
func (i *Index) LookupAttribute(name QName) (*Declaration, bool) {
	return lookup(i, AttributeSymbolSpace, name)
}

// LookupGroup returns a global model group by expanded name.
func (i *Index) LookupGroup(name QName) (*Declaration, bool) {
	return lookup(i, GroupSymbolSpace, name)
}

// LookupAttributeGroup returns a global attribute group by expanded name.
func (i *Index) LookupAttributeGroup(name QName) (*Declaration, bool) {
	return lookup(i, AttributeGroupSymbolSpace, name)
}

// ResolveType resolves a type or base QName in the namespace context of from.
func (i *Index) ResolveType(
	from *SchemaFile,
	value string,
) (*Declaration, error) {
	return i.resolve(from, value, TypeSymbolSpace)
}

// ResolveElement resolves an element ref QName in the namespace context of
// from.
func (i *Index) ResolveElement(
	from *SchemaFile,
	value string,
) (*Declaration, error) {
	return i.resolve(from, value, ElementSymbolSpace)
}

// ResolveAttribute resolves an attribute ref QName in the namespace context of
// from.
func (i *Index) ResolveAttribute(
	from *SchemaFile,
	value string,
) (*Declaration, error) {
	return i.resolve(from, value, AttributeSymbolSpace)
}

// ResolveGroup resolves a model-group ref QName in the namespace context of
// from.
func (i *Index) ResolveGroup(
	from *SchemaFile,
	value string,
) (*Declaration, error) {
	return i.resolve(from, value, GroupSymbolSpace)
}

// ResolveAttributeGroup resolves an attribute-group ref QName in the namespace
// context of from.
func (i *Index) ResolveAttributeGroup(
	from *SchemaFile,
	value string,
) (*Declaration, error) {
	return i.resolve(from, value, AttributeGroupSymbolSpace)
}

func (i *Index) addFile(file *SchemaFile) error {
	if file == nil || file.Schema == nil || file.Path == "" {
		return ErrInvalidSchemaFile
	}

	namespace := file.Schema.TargetNamespace

	for declarationIndex := range file.Schema.SimpleTypes {
		value := &file.Schema.SimpleTypes[declarationIndex]
		declaration, err := newDeclaration(
			file,
			namespace,
			value.Name,
			DeclarationSimpleType,
		)
		if err != nil {
			return err
		}

		declaration.SimpleType = value
		if err := i.add(TypeSymbolSpace, declaration); err != nil {
			return err
		}
	}

	for declarationIndex := range file.Schema.ComplexTypes {
		value := &file.Schema.ComplexTypes[declarationIndex]
		declaration, err := newDeclaration(
			file,
			namespace,
			value.Name,
			DeclarationComplexType,
		)
		if err != nil {
			return err
		}

		declaration.ComplexType = value
		if err := i.add(TypeSymbolSpace, declaration); err != nil {
			return err
		}
	}

	for declarationIndex := range file.Schema.Elements {
		value := &file.Schema.Elements[declarationIndex]
		declaration, err := newDeclaration(
			file,
			namespace,
			value.Name,
			DeclarationElement,
		)
		if err != nil {
			return err
		}

		declaration.Element = value
		if err := i.add(ElementSymbolSpace, declaration); err != nil {
			return err
		}
	}

	for declarationIndex := range file.Schema.Attributes {
		value := &file.Schema.Attributes[declarationIndex]
		declaration, err := newDeclaration(
			file,
			namespace,
			value.Name,
			DeclarationAttribute,
		)
		if err != nil {
			return err
		}

		declaration.Attribute = value
		if err := i.add(AttributeSymbolSpace, declaration); err != nil {
			return err
		}
	}

	for declarationIndex := range file.Schema.Groups {
		value := &file.Schema.Groups[declarationIndex]
		declaration, err := newDeclaration(
			file,
			namespace,
			value.Name,
			DeclarationGroup,
		)
		if err != nil {
			return err
		}

		declaration.Group = value
		if err := i.add(GroupSymbolSpace, declaration); err != nil {
			return err
		}
	}

	for declarationIndex := range file.Schema.AttributeGroups {
		value := &file.Schema.AttributeGroups[declarationIndex]
		declaration, err := newDeclaration(
			file,
			namespace,
			value.Name,
			DeclarationAttributeGroup,
		)
		if err != nil {
			return err
		}

		declaration.AttributeGroup = value
		if err := i.add(AttributeGroupSymbolSpace, declaration); err != nil {
			return err
		}
	}

	return nil
}

func (i *Index) addBuiltinTypes() {
	for _, name := range builtinSimpleTypeNames {
		declaration := &Declaration{
			Name: QName{
				Namespace: Namespace,
				Local:     name,
				Prefix:    "xs",
			},
			Kind: DeclarationBuiltinSimpleType,
		}

		i.types[declarationKey(declaration.Name)] = declaration
	}

	declaration := &Declaration{
		Name: QName{
			Namespace: Namespace,
			Local:     "anyType",
			Prefix:    "xs",
		},
		Kind: DeclarationBuiltinComplexType,
	}

	i.types[declarationKey(declaration.Name)] = declaration
}

func (i *Index) add(
	space SymbolSpace,
	declaration *Declaration,
) error {
	declarations := i.declarations(space)
	key := declarationKey(declaration.Name)

	if existing, found := declarations[key]; found {
		return &DuplicateDeclarationError{
			Space:     space,
			Name:      declaration.Name,
			Existing:  existing,
			Duplicate: declaration,
		}
	}

	declarations[key] = declaration
	return nil
}

func (i *Index) resolve(
	from *SchemaFile,
	value string,
	space SymbolSpace,
) (*Declaration, error) {
	if i == nil {
		return nil, ErrNilIndex
	}
	if from == nil || from.Schema == nil || from.Path == "" {
		return nil, ErrInvalidSchemaFile
	}

	name, err := from.Schema.ResolveQName(value)
	if err != nil {
		return nil, fmt.Errorf(
			"xsdgen: resolve %s reference %q from %q: %w",
			space,
			value,
			from.Path,
			err,
		)
	}

	declaration, found := lookup(i, space, name)
	if !found {
		return nil, &UnresolvedReferenceError{
			Space:   space,
			Lexical: value,
			Name:    name,
			From:    from.Path,
		}
	}

	return declaration, nil
}

func (i *Index) declarations(
	space SymbolSpace,
) map[expandedName]*Declaration {
	switch space {
	case TypeSymbolSpace:
		return i.types
	case ElementSymbolSpace:
		return i.elements
	case AttributeSymbolSpace:
		return i.attributes
	case GroupSymbolSpace:
		return i.groups
	case AttributeGroupSymbolSpace:
		return i.attributeGroups
	default:
		panic(fmt.Sprintf("xsdgen: unsupported symbol space %q", space))
	}
}

func lookup(
	index *Index,
	space SymbolSpace,
	name QName,
) (*Declaration, bool) {
	if index == nil {
		return nil, false
	}

	declaration, found := index.declarations(space)[declarationKey(name)]
	return declaration, found
}

func newDeclaration(
	file *SchemaFile,
	namespace string,
	name string,
	kind DeclarationKind,
) (*Declaration, error) {
	if name == "" {
		return nil, fmt.Errorf(
			"%w: %s in %q",
			ErrUnnamedDeclaration,
			kind,
			file.Path,
		)
	}
	if !validNCName(name) {
		return nil, fmt.Errorf(
			"%w %q: %s in %q",
			ErrInvalidQName,
			name,
			kind,
			file.Path,
		)
	}

	return &Declaration{
		Name: QName{
			Namespace: namespace,
			Local:     name,
		},
		Kind: kind,
		File: file,
	}, nil
}

func declarationKey(name QName) expandedName {
	return expandedName{
		namespace: name.Namespace,
		local:     name.Local,
	}
}

func declarationPath(declaration *Declaration) string {
	if declaration == nil || declaration.File == nil {
		return "<built-in>"
	}

	return declaration.File.Path
}

func formatExpandedName(name QName) string {
	if name.Namespace == "" {
		return fmt.Sprintf("%q", name.Local)
	}

	return fmt.Sprintf("{%s}%s", name.Namespace, name.Local)
}

var builtinSimpleTypeNames = []string{
	"ENTITIES",
	"ENTITY",
	"ID",
	"IDREF",
	"IDREFS",
	"NCName",
	"NMTOKEN",
	"NMTOKENS",
	"NOTATION",
	"Name",
	"QName",
	"anySimpleType",
	"anyURI",
	"base64Binary",
	"boolean",
	"byte",
	"date",
	"dateTime",
	"decimal",
	"double",
	"duration",
	"float",
	"gDay",
	"gMonth",
	"gMonthDay",
	"gYear",
	"gYearMonth",
	"hexBinary",
	"int",
	"integer",
	"language",
	"long",
	"negativeInteger",
	"nonNegativeInteger",
	"nonPositiveInteger",
	"normalizedString",
	"positiveInteger",
	"short",
	"string",
	"time",
	"token",
	"unsignedByte",
	"unsignedInt",
	"unsignedLong",
	"unsignedShort",
}
