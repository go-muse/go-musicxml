package xsdgen

import "fmt"

// constraintWhitespace resolves the whitespace normalization applied before
// comparing a string-valued fixed attribute. Named and inline restrictions can
// strengthen the built-in type's whiteSpace facet without changing its Go type.
func (r *simpleTypeRenderer) constraintWhitespace(plan *TypePlan) (string, error) {
	if plan == nil {
		return "string", nil
	}
	return r.memberConstraintWhitespace(SimpleTypeMember{
		Declaration: plan.Declaration,
		Inline:      plan.InlineSimple,
	}, make(map[*SimpleTypeDefinition]string))
}

func (r *simpleTypeRenderer) memberConstraintWhitespace(member SimpleTypeMember, policies map[*SimpleTypeDefinition]string) (string, error) {
	if member.Inline != nil {
		return r.definitionConstraintWhitespace(member.Inline, policies)
	}
	if member.Declaration == nil {
		return "string", nil
	}
	if member.Declaration.Kind == DeclarationBuiltinSimpleType {
		switch member.Declaration.Name.Local {
		case "string", "anySimpleType":
			return "string", nil
		case "normalizedString":
			return "normalizedString", nil
		default:
			return "token", nil
		}
	}
	return r.definitionConstraintWhitespace(r.plans[member.Declaration], policies)
}

func (r *simpleTypeRenderer) definitionConstraintWhitespace(definition *SimpleTypeDefinition, policies map[*SimpleTypeDefinition]string) (policy string, err error) {
	if definition == nil {
		return "string", nil
	}
	if known, found := policies[definition]; found {
		if known == "" {
			return "", fmt.Errorf("%w: cyclic fixed-attribute whitespace dependency", ErrUnsupportedComplexGeneration)
		}
		return known, nil
	}
	policies[definition] = ""
	defer func() {
		if err == nil {
			policies[definition] = policy
		} else {
			delete(policies, definition)
		}
	}()
	if definition.Form == SimpleTypeList {
		return "token", nil
	}
	if definition.Form == SimpleTypeUnion {
		// XSD 1.0 section 4.3.6 makes union normalization depend on the
		// successfully matched member. String comparison requires matching
		// policies and genuinely string-valued members, not merely Go strings.
		policy := ""
		for _, member := range definition.Members {
			memberPolicy, err := r.memberConstraintWhitespace(member, policies)
			if err != nil {
				return "", err
			}
			if policy != "" && policy != memberPolicy {
				return "", fmt.Errorf(
					"%w: fixed union has mixed whitespace policies %q and %q",
					ErrUnsupportedComplexGeneration, policy, memberPolicy,
				)
			}
			policy = memberPolicy
		}
		stringValues := make(map[*SimpleTypeDefinition]bool)
		for _, member := range definition.Members {
			if !r.constraintMemberHasStringValue(member, stringValues) {
				return "", fmt.Errorf(
					"%w: fixed union includes a non-string value space requiring value-aware comparison",
					ErrUnsupportedComplexGeneration,
				)
			}
		}
		return policy, nil
	}
	if definition.Source != nil && definition.Source.Restriction != nil {
		if facet := definition.Source.Restriction.WhiteSpace; facet != nil {
			switch facet.Value {
			case "preserve":
				return "string", nil
			case "replace":
				return "normalizedString", nil
			case "collapse":
				return "token", nil
			}
		}
	}
	if definition.Base != nil {
		return r.memberConstraintWhitespace(*definition.Base, policies)
	}
	return "string", nil
}

// constraintMemberHasStringValue identifies XSD string-derived value spaces.
// A Go string can also store dates, QNames, lists and union numerics, whose
// distinct lexical forms can have equal values. Fixed-union helpers do not
// implement those value-space comparisons.
func (r *simpleTypeRenderer) constraintMemberHasStringValue(member SimpleTypeMember, known map[*SimpleTypeDefinition]bool) bool {
	definition := member.Inline
	if definition == nil && member.Declaration != nil {
		if member.Declaration.Kind == DeclarationBuiltinSimpleType {
			switch member.Declaration.Name.Local {
			case "string", "normalizedString", "token", "language",
				"Name", "NCName", "NMTOKEN", "ID", "IDREF", "ENTITY":
				return true
			default:
				return false
			}
		}
		definition = r.plans[member.Declaration]
	}
	if definition == nil {
		return false
	}
	if value, found := known[definition]; found {
		return value
	}
	// False also marks active definitions, so invalid cycles fail closed.
	known[definition] = false
	switch definition.Form {
	case SimpleTypeRestriction:
		if definition.Base != nil {
			known[definition] = r.constraintMemberHasStringValue(*definition.Base, known)
		}
	case SimpleTypeUnion:
		for _, member := range definition.Members {
			if !r.constraintMemberHasStringValue(member, known) {
				return false
			}
		}
		known[definition] = true
	}
	return known[definition]
}
