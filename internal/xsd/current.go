package xsd

// CurrentScalar is current object evidence, never an original XML spelling.
// Known carries a present value; Absent means the representation has no scalar
// slot. The zero value is unavailable. StringDomain supplies actual current text
// to the shared lexical machinery; numbers and booleans have no lexical subject.
type CurrentScalar struct {
	Known, Absent bool
	Value         SimpleValue
}

func (c *Context) currentModel() bool { return c.local != nil && c.local.subject.CurrentModel }
func (c *Context) scalarAvailable() bool {
	if c.currentModel() {
		return c.local.subject.Current.Known || c.local.subject.Current.Absent
	}
	return c.local == nil || c.local.subject.TextKnown
}
func (c *Context) currentCharacters() (present, nonWhitespace bool) {
	value := c.local.subject.Current
	if !value.Known {
		return false, false
	}
	if value.Value.Domain == StringDomain {
		return value.Value.Text != "", trimXMLWhitespace(value.Value.Text, c) != ""
	}
	return true, true
}
func (c *Context) inputCurrent() (CurrentScalar, bool) {
	if !c.currentModel() {
		return CurrentScalar{}, false
	}
	if c.local.attribute != (QName{}) {
		return c.local.currentAttribute, c.local.currentAttributeSet
	}
	return c.local.subject.Current, true
}
func (c *Context) assessCurrentType(reference *TypeRef, value CurrentScalar) (SimpleAssessment, *SimpleFailure) {
	simple, complex, builtin, found := c.ResolveType(reference)
	if !found || complex != nil {
		return SimpleAssessment{}, c.IncompleteFailure("current simple type cannot be resolved")
	}
	if !c.ValidTypeMetadata(TypeIdentity{Simple: simple, Builtin: builtin}) {
		return SimpleAssessment{}, c.AssessmentFailure("simple type metadata incomplete")
	}
	if !value.Known || value.Absent {
		c.capability(c.local.family, "model-scalar-value-unavailable")
		return SimpleAssessment{}, &SimpleFailure{Constraint: "assessment", Message: "current scalar value is unavailable"}
	}
	if value.Value.Domain == StringDomain {
		if simple != nil {
			return c.AssessSimpleValue(simple, value.Value.Text, value.Value.Text)
		}
		return c.AssessBuiltinValue(builtin, value.Value.Text, value.Value.Text)
	}
	if simple != nil {
		return c.assessCurrentSimple(simple, value.Value)
	}
	return c.assessCurrentBuiltin(builtin, value.Value)
}
func (c *Context) assessCurrentBuiltin(builtin string, value SimpleValue) (SimpleAssessment, *SimpleFailure) {
	c.step(1)
	bad := func() (SimpleAssessment, *SimpleFailure) {
		return SimpleAssessment{}, &SimpleFailure{Constraint: "datatype", Message: format(c, "current value is not in xs:%s", builtin)}
	}
	if builtin == "boolean" && value.Domain == BooleanDomain {
		return SimpleAssessment{Builtin: builtin, Value: value}, nil
	}
	minimum, maximum, integer := IntegerLimits(builtin)
	if value.Domain != NumberDomain || (!integer && builtin != "decimal") {
		return bad()
	}
	if integer {
		if len(value.Number.Fraction) != 0 {
			return bad()
		}
		if minimum != "" {
			n, _ := parseDecimal(minimum, c)
			if value.Number.compare(n, c) < 0 {
				return bad()
			}
		}
		if maximum != "" {
			n, _ := parseDecimal(maximum, c)
			if value.Number.compare(n, c) > 0 {
				return bad()
			}
		}
	}
	return SimpleAssessment{Builtin: builtin, Value: value}, nil
}
func (c *Context) assessCurrentMember(member *SimpleMember, value SimpleValue, flattenUnion bool) (SimpleAssessment, *SimpleFailure) {
	c.step(1)
	semantics := c.MemberSemantics(member)
	if semantics == nil {
		return SimpleAssessment{}, c.IncompleteFailure("current simple member cannot be resolved")
	}
	if flattenUnion && semantics.Union != nil {
		return c.assessCurrentSimple(semantics.Union, value)
	}
	if member.Inline != nil {
		return c.assessCurrentSimple(member.Inline, value)
	}
	if member.Name.Space == XSDNamespace {
		return c.assessCurrentBuiltin(member.Name.Local, value)
	}
	return c.assessCurrentSimple(c.Schema.Types[member.Name].Simple, value)
}
func (c *Context) assessCurrentSimple(schema *SimpleSchema, value SimpleValue) (SimpleAssessment, *SimpleFailure) {
	c.step(1)
	if !c.ValidTypeMetadata(TypeIdentity{Simple: schema}) {
		return SimpleAssessment{}, c.AssessmentFailure("simple derivation metadata incomplete")
	}
	semantics := c.ResolveSimpleSemantics(schema)
	if semantics == nil {
		return SimpleAssessment{}, c.AssessmentFailure("current simple type cannot be resolved")
	}
	switch schema.Form {
	case SimpleRestriction:
		result, baseFailure := c.assessCurrentMember(schema.Base, value, false)
		// A lexical capability hole does not erase the independently known value.
		if baseFailure != nil && (!TechnicalFailure(baseFailure) || result.Builtin == "") {
			return result, baseFailure
		}
		if len(schema.Patterns) > 0 {
			c.capability(c.local.family, "model-lexical-pattern-unavailable")
			baseFailure = &SimpleFailure{Constraint: "model-capability", Message: "current numeric or boolean value has no lexical pattern subject"}
			for _, pattern := range schema.Patterns {
				c.step(1)
				if _, err := matchPattern(pattern, "", c); err != nil {
					c.Incomplete(format(c, "invalid generated XSD pattern %q: %v", pattern, err))
				}
			}
		}
		before := c.AssessmentVersion
		if len(schema.Enumerations) > 0 && !c.MatchesSimpleEnumeration(schema, result) {
			if before != c.AssessmentVersion {
				return result, c.AssessmentFailure("enumeration literal assessment incomplete")
			}
			return result, &SimpleFailure{Constraint: "enumeration", Message: "current value is not one of the allowed values"}
		}
		if result.Value.Domain == NumberDomain {
			if failure := validateNumberBounds(schema, result.Value.Number, result.Builtin, c); failure != nil {
				return result, failure
			}
			if failure := validateNumberDigits(schema, result.Value.Number, c); failure != nil {
				return result, failure
			}
		} else if schema.HasMinInclusive || schema.HasMaxInclusive || schema.HasMinExclusive || schema.HasMaxExclusive || schema.HasTotalDigits || schema.HasFractionDigits {
			return result, c.IncompleteFailure("numeric facets require a current numeric value")
		}
		if schema.HasLength || schema.HasMinLength || schema.HasMaxLength {
			return result, c.IncompleteFailure("length facet requires textual or list evidence")
		}
		return result, baseFailure
	case SimpleUnion:
		for _, member := range schema.Members {
			c.step(1)
			result, failure := c.assessCurrentMember(member, value, true)
			if failure == nil {
				return result, nil
			}
			if TechnicalFailure(failure) {
				return result, failure
			}
		}
		return SimpleAssessment{}, &SimpleFailure{Constraint: "union", Message: "current value does not match any union member"}
	case SimpleList:
		return SimpleAssessment{}, &SimpleFailure{Constraint: "datatype", Message: "current numeric or boolean value is not a list"}
	default:
		return SimpleAssessment{}, c.IncompleteFailure("unsupported current simple type form")
	}
}
func validateNumberBounds(schema *SimpleSchema, number Decimal, builtin string, c *Context) *SimpleFailure {
	for _, test := range []struct {
		enabled             bool
		constraint, literal string
		minimum, exclusive  bool
	}{
		{schema.HasMinInclusive, "minInclusive", schema.MinInclusive, true, false}, {schema.HasMaxInclusive, "maxInclusive", schema.MaxInclusive, false, false}, {schema.HasMinExclusive, "minExclusive", schema.MinExclusive, true, true}, {schema.HasMaxExclusive, "maxExclusive", schema.MaxExclusive, false, true},
	} {
		c.step(1)
		if !test.enabled {
			continue
		}
		limit, ok := parseDecimal(test.literal, c)
		if _, _, integer := IntegerLimits(builtin); integer {
			_, ok = parseInteger(test.literal, c)
		}
		if !ok {
			return c.IncompleteFailure(format(c, "invalid %s value %q", test.constraint, test.literal))
		}
		comparison := number.compare(limit, c)
		if (test.minimum && comparison < 0) || (!test.minimum && comparison > 0) || (test.exclusive && comparison == 0) {
			return &SimpleFailure{Constraint: test.constraint, Message: format(c, "current value violates %s=%q", test.constraint, test.literal)}
		}
	}
	return nil
}
func validateNumberDigits(schema *SimpleSchema, number Decimal, c *Context) *SimpleFailure {
	c.step(1)
	fraction := uint64(len(number.Fraction))
	total := max(1, uint64(len(number.Whole))+fraction)
	if schema.HasTotalDigits && total > schema.TotalDigits {
		return &SimpleFailure{Constraint: "totalDigits", Message: format(c, "value has %d digits; maximum is %d", total, schema.TotalDigits)}
	}
	if schema.HasFractionDigits && fraction > schema.FractionDigits {
		return &SimpleFailure{Constraint: "fractionDigits", Message: format(c, "value has %d fraction digits; maximum is %d", fraction, schema.FractionDigits)}
	}
	return nil
}
func (c *Context) currentLiteralValue(reference *TypeRef, value CurrentScalar) (SimpleAssessment, bool) {
	_, complex, _, _ := c.ResolveType(reference)
	if complex != nil {
		effective, ok := c.EffectiveComplex(complex)
		if !ok || effective.Simple == nil {
			c.Incomplete("current element fixed comparison requires simple content")
			return SimpleAssessment{}, false
		}
		reference = effective.Simple
	}
	semantics := c.SimpleTypeSemantics(reference)
	if semantics == nil || !value.Known || value.Absent {
		return SimpleAssessment{}, false
	}
	if value.Value.Domain == StringDomain {
		return c.SimpleLiteralValue(semantics, value.Value.Text)
	}
	if semantics.Union != nil {
		result, failure := c.assessCurrentSimple(semantics.Union, value.Value)
		if TechnicalFailure(failure) {
			c.Incomplete("current fixed union member is unavailable")
		}
		return result, failure == nil
	}
	if semantics.List != nil {
		return SimpleAssessment{}, false
	}
	result, failure := c.assessCurrentBuiltin(semantics.Builtin, value.Value)
	return result, failure == nil
}
func (c *Context) currentFixedEqual(reference *TypeRef, value CurrentScalar, literal string) bool {
	actual, ok := c.currentLiteralValue(reference, value)
	if !ok {
		return false
	}
	_, complex, _, _ := c.ResolveType(reference)
	if complex != nil {
		effective, found := c.EffectiveComplex(complex)
		if !found || effective.Simple == nil {
			return false
		}
		reference = effective.Simple
	}
	expected, ok := c.SimpleLiteralValue(c.SimpleTypeSemantics(reference), literal)
	return ok && actual.Value.equal(expected.Value, c)
}

// Current numeric components are checked and normalized as value components,
// without ever constructing a spelling to feed the lexical validator.
func (c *Context) checkCurrent(value CurrentScalar) CurrentScalar {
	c.step(1)
	if value.Known && value.Absent {
		panic(runAbort{&TechnicalError{Message: "contradictory current scalar evidence"}})
	}
	if !value.Known {
		return value
	}
	switch value.Value.Domain {
	case StringDomain:
		c.scalar(int64(len(value.Value.Text)))
		c.bytes(len(value.Value.Text))
	case BooleanDomain:
	case NumberDomain:
		n := value.Value.Number
		c.scalar(c.sum(int64(len(n.Whole)), int64(len(n.Fraction))))
		for _, digits := range []string{n.Whole, n.Fraction} {
			for i := range len(digits) {
				c.step(1)
				if digits[i] < '0' || digits[i] > '9' {
					panic(runAbort{&TechnicalError{Message: "invalid current numeric components"}})
				}
			}
		}
		for len(n.Whole) > 0 && n.Whole[0] == '0' {
			c.step(1)
			n.Whole = n.Whole[1:]
		}
		for len(n.Fraction) > 0 && n.Fraction[len(n.Fraction)-1] == '0' {
			c.step(1)
			n.Fraction = n.Fraction[:len(n.Fraction)-1]
		}
		n.Negative = n.Negative && (n.Whole != "" || n.Fraction != "")
		value.Value.Number = n
	default:
		panic(runAbort{&TechnicalError{Message: "unsupported current scalar input domain"}})
	}
	return value
}

func (c *Context) elementFixedEqual(actual, declared *TypeRef, text, literal string) bool {
	if c.currentModel() {
		return c.currentFixedEqual(actual, c.local.subject.Current, literal)
	}
	return c.ElementConstraintEqual(actual, declared, text, literal)
}
func (c *Context) attributeFixedEqual(reference *TypeRef, left, right string) bool {
	if current, ok := c.inputCurrent(); ok {
		return c.currentFixedEqual(reference, current, right)
	}
	return c.SimpleValuesEqual(reference, left, right)
}
func (c *Context) attributeIdentityText(text string) string {
	if current, ok := c.inputCurrent(); ok && current.Known && current.Value.Domain == StringDomain {
		return current.Value.Text
	}
	return text
}
