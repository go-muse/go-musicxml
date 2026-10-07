package xsdgen

// constraintWhitespace resolves the whitespace normalization applied before
// comparing a string-valued fixed attribute. Named and inline restrictions can
// strengthen the built-in type's whiteSpace facet without changing its Go type.
func (r *simpleTypeRenderer) constraintWhitespace(plan *TypePlan) string {
	if plan == nil {
		return "string"
	}
	return r.memberConstraintWhitespace(SimpleTypeMember{
		Declaration: plan.Declaration,
		Inline:      plan.InlineSimple,
	})
}

func (r *simpleTypeRenderer) memberConstraintWhitespace(member SimpleTypeMember) string {
	if member.Inline != nil {
		return r.definitionConstraintWhitespace(member.Inline)
	}
	if member.Declaration == nil {
		return "string"
	}
	if member.Declaration.Kind == DeclarationBuiltinSimpleType {
		switch member.Declaration.Name.Local {
		case "string", "anySimpleType":
			return "string"
		case "normalizedString":
			return "normalizedString"
		default:
			return "token"
		}
	}
	return r.definitionConstraintWhitespace(r.plans[member.Declaration])
}

func (r *simpleTypeRenderer) definitionConstraintWhitespace(definition *SimpleTypeDefinition) string {
	if definition == nil {
		return "string"
	}
	if definition.Form == SimpleTypeList || definition.Form == SimpleTypeUnion {
		return "token"
	}
	if definition.Source != nil && definition.Source.Restriction != nil {
		if facet := definition.Source.Restriction.WhiteSpace; facet != nil {
			switch facet.Value {
			case "preserve":
				return "string"
			case "replace":
				return "normalizedString"
			case "collapse":
				return "token"
			}
		}
	}
	if definition.Base != nil {
		return r.memberConstraintWhitespace(*definition.Base)
	}
	return "string"
}
