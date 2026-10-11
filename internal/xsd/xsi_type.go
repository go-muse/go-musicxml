package xsd

import (
	"fmt"
	"strings"
)

// Derivation bits describe schema components, not a blanket prohibition on
// xsi:type. In particular, element Final governs substitution-group membership.
type Derivation uint8

const (
	DerivationExtension Derivation = 1 << iota
	DerivationRestriction
	DerivationList
	DerivationUnion
	DerivationSubstitution
)

type TypeIdentity struct {
	Simple  *SimpleSchema
	Complex *ComplexSchema
	Builtin string
}

type DerivationKey struct {
	Derived, Base TypeIdentity
	Blocked       Derivation
}

func (c *Context) TypeIdentity(ref *TypeRef) (TypeIdentity, bool) {
	simple, complex, builtin, ok := c.ResolveType(ref)
	return TypeIdentity{simple, complex, builtin}, ok && (simple != nil || complex != nil || builtin != "")
}

func BuiltinType(name string) TypeIdentity {
	return TypeIdentity{Builtin: name}
}
func (v TypeIdentity) Final() Derivation {
	if v.Simple != nil {
		return v.Simple.Final
	}
	if v.Complex != nil {
		return v.Complex.Final
	}
	return 0
}

// Every builtin name is checked, including xs:anyType and the two ur-types.
// All simple derivations count as restriction for cos-st-derived-ok; list/union
// final bits instead constrain type construction from item/member definitions.
func BuiltinBase(name string) string {
	switch name {
	case "anyType":
		return ""
	case "anySimpleType":
		return "anyType"
	case "normalizedString":
		return "string"
	case "token":
		return "normalizedString"
	case "language", "Name", "NMTOKEN":
		return "token"
	case "NCName":
		return "Name"
	case "ID", "IDREF", "ENTITY":
		return "NCName"
	case "integer":
		return "decimal"
	case "nonPositiveInteger", "nonNegativeInteger", "long":
		return "integer"
	case "negativeInteger":
		return "nonPositiveInteger"
	case "positiveInteger", "unsignedLong":
		return "nonNegativeInteger"
	case "int":
		return "long"
	case "short":
		return "int"
	case "byte":
		return "short"
	case "unsignedInt":
		return "unsignedLong"
	case "unsignedShort":
		return "unsignedInt"
	case "unsignedByte":
		return "unsignedShort"
	default:
		return "anySimpleType"
	}
}

func (c *Context) MemberType(member *SimpleMember) (TypeIdentity, bool) {
	if member == nil {
		return TypeIdentity{}, false
	}
	return c.TypeIdentity(&TypeRef{Name: member.Name, InlineSimple: member.Inline})
}

func (c *Context) TypeBase(v TypeIdentity) (TypeIdentity, Derivation, bool) {
	if v.Builtin != "" {
		base := BuiltinBase(v.Builtin)
		if base == "" {
			return TypeIdentity{}, 0, true
		}
		return BuiltinType(base), DerivationRestriction, true
	}
	if v.Simple != nil {
		switch v.Simple.Form {
		case SimpleRestriction:
			base, ok := c.MemberType(v.Simple.Base)
			return base, DerivationRestriction, ok
		case SimpleList, SimpleUnion:
			return BuiltinType("anySimpleType"), DerivationRestriction, true
		default:
			return TypeIdentity{}, 0, false
		}
	}
	if v.Complex != nil {
		switch v.Complex.Form {
		case ComplexDirect:
			return BuiltinType("anyType"), DerivationRestriction, true
		case ComplexSimpleContentExtension, ComplexComplexContentExtension:
			base, ok := c.TypeIdentity(v.Complex.Base)
			return base, DerivationExtension, ok
		case ComplexSimpleContentRestriction, ComplexComplexContentRestriction:
			base, ok := c.TypeIdentity(v.Complex.Base)
			return base, DerivationRestriction, ok
		}
	}
	return TypeIdentity{}, 0, false
}

// This graph follows only ancestry and simple item/member edges. Recursive
// element content is not a derivation cycle. States and results are per run;
// source and generated schema objects are never mutated.
func (c *Context) ValidTypeMetadata(v TypeIdentity) bool {
	if c.TypeMetadata == nil {
		c.TypeMetadata = make(map[TypeIdentity]uint8)
	}
	if state, seen := c.TypeMetadata[v]; seen {
		if state == 1 {
			c.Incomplete("cyclic type derivation metadata")
		} else if state == 3 {
			// A cached unknown is newly reached incomplete work, not a proved
			// false result. Replay its epoch so fixed/default/enumeration callers
			// cannot turn a later lookup into an ordinary document violation.
			c.IncompleteAgain("cached type metadata assessment is incomplete")
		}
		return state == 2
	}
	c.TypeMetadata[v] = 1
	ok := false
	defer func() {
		if ok {
			c.TypeMetadata[v] = 2
		} else {
			c.TypeMetadata[v] = 3
		}
	}()
	if v.Simple != nil && v.Complex != nil {
		c.Incomplete("type metadata has both simple and complex definitions")
		return false
	}
	base, method, found := c.TypeBase(v)
	if !found {
		c.Incomplete("missing or unsupported type derivation metadata")
		return false
	}
	if method != 0 {
		if !c.ValidTypeMetadata(base) {
			return false
		}
		if base.Final()&method != 0 {
			c.Incomplete("type derivation violates base final metadata")
			return false
		}
	}
	if v.Simple != nil {
		switch v.Simple.Form {
		case SimpleRestriction:
			if base.Complex != nil || base.Builtin == "anyType" {
				c.Incomplete("simple restriction has a non-simple base")
				return false
			}
		case SimpleList:
			item, found := c.MemberType(v.Simple.Item)
			if !found || !c.ValidTypeMetadata(item) {
				c.Incomplete("list item type metadata cannot be resolved")
				return false
			}
			if item.Final()&DerivationList != 0 {
				c.Incomplete("list item type final forbids list construction")
				return false
			}
		case SimpleUnion:
			if len(v.Simple.Members) == 0 {
				c.Incomplete("union has no member type metadata")
				return false
			}
			for _, member := range v.Simple.Members {
				item, found := c.MemberType(member)
				if !found || !c.ValidTypeMetadata(item) {
					c.Incomplete("union member type metadata cannot be resolved")
					return false
				}
				if item.Final()&DerivationUnion != 0 {
					c.Incomplete("union member final forbids union construction")
					return false
				}
			}
		}
	}
	if v.Complex != nil {
		switch v.Complex.Form {
		case ComplexComplexContentExtension, ComplexComplexContentRestriction, ComplexSimpleContentRestriction:
			if base.Complex == nil && base.Builtin != "anyType" {
				c.Incomplete("complex derivation has a non-complex base")
				return false
			}
		}
	}
	ok = true
	return true
}

func (c *Context) TypeDerived(derived, base TypeIdentity, blocked Derivation) bool {
	if derived == base {
		return true
	}
	key := DerivationKey{derived, base, blocked}
	if known, found := c.Derivations[key]; found {
		return known
	}
	if c.Derivations == nil {
		c.Derivations = make(map[DerivationKey]bool)
	}
	// Metadata has already been checked acyclic, including all union members.
	c.Derivations[key] = false
	before := c.AssessmentVersion
	defer func() {
		// Cache only proved answers. A technical failure must be reached again
		// on subsequent lookups so its cached nil semantics replay incomplete
		// state rather than becoming an ordinary false derivation result.
		if c.AssessmentVersion != before {
			delete(c.Derivations, key)
		}
	}()
	parent, method, _ := c.TypeBase(derived)
	if method == 0 || blocked&method != 0 {
		return false
	}
	if base.Simple != nil && derived.Complex == nil {
		semantics := c.ResolveSimpleSemantics(base.Simple)
		if semantics == nil {
			return false
		}
		if semantics.Union != nil {
			for _, member := range semantics.Union.Members {
				memberType, _ := c.MemberType(member)
				if c.TypeDerived(derived, memberType, blocked) {
					c.Derivations[key] = true
					return true
				}
			}
		}
	}
	result := parent == base || c.TypeDerived(parent, base, blocked)
	c.Derivations[key] = result
	return result
}

func InstanceQName(node *Node, lexical string) (QName, bool) {
	lexical = CollapseWhitespace(lexical)
	if ValidateBuiltinValue("QName", lexical, lexical) != nil {
		return QName{}, false
	}
	prefix, local, qualified := strings.Cut(lexical, ":")
	if !qualified {
		return QName{Space: node.Namespaces[""], Local: lexical}, true
	}
	namespace, found := node.Namespaces[prefix]
	return QName{Space: namespace, Local: local}, found && namespace != ""
}

// cvc-elt 4 explicitly falls back to the declared type after a failed
// nomination. Keep that ordinary assessment so independent errors survive.
// The boolean reports nomination success separately from fallback type validity.
func (c *Context) ActualElementType(node *Node, schema *ElementSchema, path string) (*TypeRef, bool) {
	declared := &schema.Type
	var value string
	present := false
	for _, attr := range node.Attrs {
		if !attr.NamespaceDeclaration && attr.Name == (QName{Space: XSINamespace, Local: "type"}) {
			value, present = attr.Value, true
			break
		}
	}
	if !present {
		return declared, true
	}
	issuePath := AttributePath(path, QName{Space: XSINamespace, Local: "type"})
	name, ok := InstanceQName(node, value)
	if !ok {
		c.AddIssue(issuePath, "xsi:type", fmt.Sprintf("value %q is not a bound QName", value))
		return declared, false
	}
	nominated := &TypeRef{Name: name}
	actual, found := c.TypeIdentity(nominated)
	if !found {
		if _, exists := c.Schema.Types[name]; exists {
			c.Incomplete("nominated type component metadata is missing")
			return declared, false
		}
		c.AddIssue(issuePath, "xsi:type", fmt.Sprintf("type %s cannot be resolved in the selected schema", DisplayQName(name)))
		return declared, false
	}
	base, found := c.TypeIdentity(declared)
	if !found {
		c.Incomplete("declared type metadata cannot be resolved for xsi:type")
		return declared, false
	}
	if !c.ValidTypeMetadata(actual) || !c.ValidTypeMetadata(base) {
		return declared, false
	}
	blocked := schema.Block
	if actual.Complex != nil && base.Complex != nil {
		blocked |= base.Complex.Block
	}
	before := c.AssessmentVersion
	derived := c.TypeDerived(actual, base, blocked)
	if c.AssessmentVersion != before {
		return declared, false
	}
	if !derived {
		c.AddIssue(issuePath, "xsi:type", "nominated type is not an allowed derivation of the declared type")
		return declared, false
	}
	return nominated, true
}
