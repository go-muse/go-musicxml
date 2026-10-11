package musicxml

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Derivation bits describe schema components, not a blanket prohibition on
// xsi:type. In particular, element Final governs substitution-group membership.
type validationDerivation uint8

const (
	validationDerivationExtension validationDerivation = 1 << iota
	validationDerivationRestriction
	validationDerivationList
	validationDerivationUnion
	validationDerivationSubstitution
)

type validationTypeIdentity struct {
	simple  *validationSimpleSchema
	complex *validationComplexSchema
	builtin string
}

type validationDerivationKey struct {
	derived, base validationTypeIdentity
	blocked       validationDerivation
}

func (c *validationContext) typeIdentity(ref *validationTypeRef) (validationTypeIdentity, bool) {
	simple, complex, builtin, ok := c.resolveType(ref)
	return validationTypeIdentity{simple, complex, builtin}, ok && (simple != nil || complex != nil || builtin != "")
}

func validationBuiltinType(name string) validationTypeIdentity {
	return validationTypeIdentity{builtin: name}
}
func (v validationTypeIdentity) final() validationDerivation {
	if v.simple != nil {
		return v.simple.Final
	}
	if v.complex != nil {
		return v.complex.Final
	}
	return 0
}

// Every builtin name is checked, including xs:anyType and the two ur-types.
// All simple derivations count as restriction for cos-st-derived-ok; list/union
// final bits instead constrain type construction from item/member definitions.
func validationBuiltinBase(name string) string {
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

func (c *validationContext) memberType(member *validationSimpleMember) (validationTypeIdentity, bool) {
	if member == nil {
		return validationTypeIdentity{}, false
	}
	return c.typeIdentity(&validationTypeRef{Name: member.Name, InlineSimple: member.Inline})
}

func (c *validationContext) typeBase(v validationTypeIdentity) (validationTypeIdentity, validationDerivation, bool) {
	if v.builtin != "" {
		base := validationBuiltinBase(v.builtin)
		if base == "" {
			return validationTypeIdentity{}, 0, true
		}
		return validationBuiltinType(base), validationDerivationRestriction, true
	}
	if v.simple != nil {
		switch v.simple.Form {
		case validationSimpleRestriction:
			base, ok := c.memberType(v.simple.Base)
			return base, validationDerivationRestriction, ok
		case validationSimpleList, validationSimpleUnion:
			return validationBuiltinType("anySimpleType"), validationDerivationRestriction, true
		default:
			return validationTypeIdentity{}, 0, false
		}
	}
	if v.complex != nil {
		switch v.complex.Form {
		case validationComplexDirect:
			return validationBuiltinType("anyType"), validationDerivationRestriction, true
		case validationComplexSimpleContentExtension, validationComplexComplexContentExtension:
			base, ok := c.typeIdentity(v.complex.Base)
			return base, validationDerivationExtension, ok
		case validationComplexSimpleContentRestriction, validationComplexComplexContentRestriction:
			base, ok := c.typeIdentity(v.complex.Base)
			return base, validationDerivationRestriction, ok
		}
	}
	return validationTypeIdentity{}, 0, false
}

// This graph follows only ancestry and simple item/member edges. Recursive
// element content is not a derivation cycle. States and results are per run;
// source and generated schema objects are never mutated.
func (c *validationContext) validTypeMetadata(v validationTypeIdentity) bool {
	if c.typeMetadata == nil {
		c.typeMetadata = make(map[validationTypeIdentity]uint8)
	}
	if state, seen := c.typeMetadata[v]; seen {
		if state == 1 {
			c.incomplete("cyclic type derivation metadata")
		} else if state == 3 {
			// A cached unknown is newly reached incomplete work, not a proved
			// false result. Replay its epoch so fixed/default/enumeration callers
			// cannot turn a later lookup into an ordinary document violation.
			c.incomplete("cached type metadata assessment is incomplete")
		}
		return state == 2
	}
	c.typeMetadata[v] = 1
	ok := false
	defer func() {
		if ok {
			c.typeMetadata[v] = 2
		} else {
			c.typeMetadata[v] = 3
		}
	}()
	if v.simple != nil && v.complex != nil {
		c.incomplete("type metadata has both simple and complex definitions")
		return false
	}
	base, method, found := c.typeBase(v)
	if !found {
		c.incomplete("missing or unsupported type derivation metadata")
		return false
	}
	if method != 0 {
		if !c.validTypeMetadata(base) {
			return false
		}
		if base.final()&method != 0 {
			c.incomplete("type derivation violates base final metadata")
			return false
		}
	}
	if v.simple != nil {
		switch v.simple.Form {
		case validationSimpleRestriction:
			if base.complex != nil || base.builtin == "anyType" {
				c.incomplete("simple restriction has a non-simple base")
				return false
			}
		case validationSimpleList:
			item, found := c.memberType(v.simple.Item)
			if !found || !c.validTypeMetadata(item) {
				c.incomplete("list item type metadata cannot be resolved")
				return false
			}
			if item.final()&validationDerivationList != 0 {
				c.incomplete("list item type final forbids list construction")
				return false
			}
		case validationSimpleUnion:
			if len(v.simple.Members) == 0 {
				c.incomplete("union has no member type metadata")
				return false
			}
			for _, member := range v.simple.Members {
				item, found := c.memberType(member)
				if !found || !c.validTypeMetadata(item) {
					c.incomplete("union member type metadata cannot be resolved")
					return false
				}
				if item.final()&validationDerivationUnion != 0 {
					c.incomplete("union member final forbids union construction")
					return false
				}
			}
		}
	}
	if v.complex != nil {
		switch v.complex.Form {
		case validationComplexComplexContentExtension, validationComplexComplexContentRestriction, validationComplexSimpleContentRestriction:
			if base.complex == nil && base.builtin != "anyType" {
				c.incomplete("complex derivation has a non-complex base")
				return false
			}
		}
	}
	ok = true
	return true
}

func (c *validationContext) typeDerived(derived, base validationTypeIdentity, blocked validationDerivation) bool {
	if derived == base {
		return true
	}
	key := validationDerivationKey{derived, base, blocked}
	if known, found := c.derivations[key]; found {
		return known
	}
	if c.derivations == nil {
		c.derivations = make(map[validationDerivationKey]bool)
	}
	// Metadata has already been checked acyclic, including all union members.
	c.derivations[key] = false
	before := c.assessmentVersion
	defer func() {
		// Cache only proved answers. A technical failure must be reached again
		// on subsequent lookups so its cached nil semantics replay incomplete
		// state rather than becoming an ordinary false derivation result.
		if c.assessmentVersion != before {
			delete(c.derivations, key)
		}
	}()
	parent, method, _ := c.typeBase(derived)
	if method == 0 || blocked&method != 0 {
		return false
	}
	if base.simple != nil && derived.complex == nil {
		semantics := c.resolveSimpleSemantics(base.simple)
		if semantics == nil {
			return false
		}
		if semantics.union != nil {
			for _, member := range semantics.union.Members {
				memberType, _ := c.memberType(member)
				if c.typeDerived(derived, memberType, blocked) {
					c.derivations[key] = true
					return true
				}
			}
		}
	}
	result := parent == base || c.typeDerived(parent, base, blocked)
	c.derivations[key] = result
	return result
}

func validationInstanceQName(node *validationNode, lexical string) (validationQName, bool) {
	lexical = collapseValidationWhitespace(lexical)
	if validateBuiltinValue("QName", lexical, lexical) != nil {
		return validationQName{}, false
	}
	prefix, local, qualified := strings.Cut(lexical, ":")
	if !qualified {
		return validationQName{Space: node.Namespaces[""], Local: lexical}, true
	}
	namespace, found := node.Namespaces[prefix]
	return validationQName{Space: namespace, Local: local}, found && namespace != ""
}

// cvc-elt 4 explicitly falls back to the declared type after a failed
// nomination. Keep that ordinary assessment so independent errors survive.
// The boolean reports nomination success separately from fallback type validity.
func (c *validationContext) actualElementType(node *validationNode, schema *validationElementSchema, path string) (*validationTypeRef, bool) {
	declared := &schema.Type
	var value string
	present := false
	for _, attr := range node.Attrs {
		if !attr.NamespaceDeclaration && attr.Name == (xml.Name{Space: validationXSINamespace, Local: "type"}) {
			value, present = attr.Value, true
			break
		}
	}
	if !present {
		return declared, true
	}
	issuePath := validationAttributePath(path, xml.Name{Space: validationXSINamespace, Local: "type"})
	name, ok := validationInstanceQName(node, value)
	if !ok {
		c.addIssue(issuePath, "xsi:type", fmt.Sprintf("value %q is not a bound QName", value))
		return declared, false
	}
	nominated := &validationTypeRef{Name: name}
	actual, found := c.typeIdentity(nominated)
	if !found {
		if _, exists := c.schema.Types[name]; exists {
			c.incomplete("nominated type component metadata is missing")
			return declared, false
		}
		c.addIssue(issuePath, "xsi:type", fmt.Sprintf("type %s cannot be resolved in the selected schema", validationDisplayQName(name)))
		return declared, false
	}
	base, found := c.typeIdentity(declared)
	if !found {
		c.incomplete("declared type metadata cannot be resolved for xsi:type")
		return declared, false
	}
	if !c.validTypeMetadata(actual) || !c.validTypeMetadata(base) {
		return declared, false
	}
	blocked := schema.Block
	if actual.complex != nil && base.complex != nil {
		blocked |= base.complex.Block
	}
	before := c.assessmentVersion
	derived := c.typeDerived(actual, base, blocked)
	if c.assessmentVersion != before {
		return declared, false
	}
	if !derived {
		c.addIssue(issuePath, "xsi:type", "nominated type is not an allowed derivation of the declared type")
		return declared, false
	}
	return nominated, true
}
