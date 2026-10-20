package xsd

import "unsafe"

// This is a rejection-only pass for lists which can reach a newly guarded
// primitive. A nil failure means only that no rejection has been established;
// it never promotes a viable guarded candidate to an assessed value. Existing
// supported atomic operations may prove negatives, while the ordinary assessor
// remains responsible for the final ordered selection and value-side result.
func (c *Context) memberHasGuardedAtomic(member *SimpleMember) bool {
	c.step(1)
	semantics := c.MemberSemantics(member)
	if semantics == nil {
		return false
	}
	if semantics.List != nil {
		return c.memberHasGuardedAtomic(semantics.List.Item)
	}
	if semantics.Union != nil {
		return c.unionHasGuardedAtomic(semantics.Union)
	}
	return guardedAtomicBuiltin(semantics.Builtin)
}

func (c *Context) precheckListLexical(item *SimpleMember, value string) *SimpleFailure {
	for index, word := range splitList(value, c) {
		c.step(1)
		if failure := c.precheckMemberLexical(item, word, word, false); failure != nil {
			return &SimpleFailure{Constraint: failure.Constraint, Message: format(c, "list item %d: %s", index+1, failure.Message)}
		}
	}
	return nil
}
func (c *Context) precheckMemberLexical(member *SimpleMember, value, display string, flattenUnion bool) *SimpleFailure {
	c.step(1)
	semantics := c.MemberSemantics(member)
	if semantics == nil {
		return &SimpleFailure{Constraint: "schema", Message: "simple type member cannot be resolved"}
	}
	if flattenUnion && semantics.Union != nil {
		return c.precheckSchemaLexical(semantics.Union, value, display)
	}
	if supportedAtomicPrecheck(semantics) {
		// Preserve existing exact/value-side negatives for supported atomics.
		// Discard the value: this pass still cannot elect an aggregate member.
		_, failure := c.AssessSimpleMemberValue(member, value, display)
		return failure
	}
	if member.Inline != nil {
		return c.precheckSchemaLexical(member.Inline, value, display)
	}
	if member.Name.Space == XSDNamespace {
		return validateBuiltinValue(member.Name.Local, value, display, c)
	}
	return c.precheckSchemaLexical(c.Schema.Types[member.Name].Simple, value, display)
}
func (c *Context) precheckSchemaLexical(schema *SimpleSchema, value, display string) *SimpleFailure {
	c.step(1)
	semantics := c.ResolveSimpleSemantics(schema)
	if semantics == nil {
		return &SimpleFailure{Constraint: "schema", Message: "simple type cannot be resolved"}
	}
	value = semantics.normalize(value, c)
	switch schema.Form {
	case SimpleRestriction:
		if semantics.Union != nil {
			if failure := rejectUnionPattern(schema, value, display, c); failure != nil {
				return failure
			}
			known, failure := c.precheckKnownUnionCandidate(semantics.Union, value, display)
			if failure != nil {
				return failure
			}
			if known {
				// Earlier members are proved rejected and a supported atomic member
				// is proved valid. Reuse existing outer facet/value assessment.
				_, failure := c.AssessSimpleValue(schema, value, display)
				return failure
			}
			// Preserve actual restriction wrappers. Only composition through a
			// union member (the flattenUnion argument) may discard them.
			return c.precheckMemberLexical(schema.Base, value, display, false)
		}
		if failure := c.precheckMemberLexical(schema.Base, value, display, false); failure != nil {
			return failure
		}
		if failure := validatePatternFacets(schema, value, display, c); failure != nil {
			return failure
		}
		if semantics.List != nil || BuiltinListItem(semantics.Builtin) != "" {
			return validateLengthFacets(schema, value, true, semantics.Builtin, c)
		}
		if semantics.StringValue || semantics.Builtin == "anyURI" || semantics.Builtin == "hexBinary" || semantics.Builtin == "base64Binary" {
			return validateLengthFacets(schema, value, false, semantics.Builtin, c)
		}
		// QName length is facet-valid in XSD 1.0. Equality and ordered facets
		// deliberately remain the responsibility of the ordinary value assessor.
		return nil
	case SimpleList:
		return c.precheckListLexical(schema.Item, value)
	case SimpleUnion:
		for _, member := range schema.Members {
			c.step(1)
			failure := c.precheckMemberLexical(member, value, display, true)
			if failure == nil {
				return nil
			}
			if TechnicalFailure(failure) {
				return failure
			}
		}
		return &SimpleFailure{Constraint: "union", Message: format(c, "value %q does not match any union member", display)}
	default:
		return &SimpleFailure{Constraint: "schema", Message: "unsupported simple type form"}
	}
}

func (c *Context) unionHasGuardedAtomic(schema *SimpleSchema) bool {
	for _, member := range schema.Members {
		c.step(1)
		if c.memberHasGuardedAtomic(member) {
			return true
		}
	}
	return false
}
func supportedAtomicPrecheck(semantics *SimpleSemantics) bool {
	_, _, integer := IntegerLimits(semantics.Builtin)
	return !semantics.Aggregate() && !entityBuiltin(semantics.Builtin) &&
		(integer || semantics.Builtin == "decimal" || semantics.StringValue || semantics.Builtin == "boolean" || semantics.Builtin == "anyURI")
}

// Certainty is separate from lexical viability. Stop at the first viable
// candidate; a guarded candidate cannot authorize looking through it or choosing
// a later member. Only an already supported, fully checked atomic can be known.
func (c *Context) precheckKnownUnionCandidate(schema *SimpleSchema, value, display string) (bool, *SimpleFailure) {
	for _, member := range schema.Members {
		c.step(1)
		semantics := c.MemberSemantics(member)
		if semantics == nil {
			return false, &SimpleFailure{Constraint: "schema", Message: "simple type member cannot be resolved"}
		}
		var failure *SimpleFailure
		known := false
		if semantics.Union != nil {
			known, failure = c.precheckKnownUnionCandidate(semantics.Union, value, display)
		} else {
			failure = c.precheckMemberLexical(member, value, display, true)
			known = supportedAtomicPrecheck(semantics)
		}
		if failure == nil {
			return known, nil
		}
		if TechnicalFailure(failure) {
			return false, failure
		}
	}
	return false, &SimpleFailure{Constraint: "union", Message: format(c, "value %q does not match any union member", display)}
}

// A union restriction can reject before an unavailable selected value only if
// every still-possible selected candidate fails. Explore in an isolated context:
// hypothetical later members must not publish technical epochs, limitations or
// cache entries into the actual ordered assessment. Work and storage still use
// the same run meter, and resource/cancellation panics are never intercepted.
func rejectUnionPattern(schema *SimpleSchema, value, display string, c *Context) *SimpleFailure {
	if len(schema.Patterns) == 0 {
		return nil
	}
	trial := c.rejectionTrial()
	semantics := trial.ResolveSimpleSemantics(schema)
	if semantics == nil || semantics.Union == nil {
		return nil
	}
	failure, _ := trial.rejectUnionCandidates(semantics.Union, schema, value, display)
	return failure
}

// A non-nil failure proves rejection across all candidates visited. Terminal
// means a known valid base candidate was reached, so later members cannot be
// selected even when its OUTER pattern fails. A viable guarded candidate is not
// known to succeed: its outer failure permits further proof, never selection.
func (c *Context) rejectUnionCandidates(union, restriction *SimpleSchema, value, display string) (*SimpleFailure, bool) {
	var rejected *SimpleFailure
	for _, member := range union.Members {
		c.step(1)
		semantics := c.MemberSemantics(member)
		if semantics == nil {
			return nil, true
		}
		if semantics.Union != nil {
			failure, terminal := c.rejectUnionCandidates(semantics.Union, restriction, value, display)
			if failure == nil || terminal {
				return failure, terminal
			}
			rejected = failure
			continue
		}
		failure := c.precheckMemberLexical(member, value, display, true)
		if TechnicalFailure(failure) {
			return nil, true
		}
		if failure != nil {
			rejected = failure
			continue
		}
		known := supportedAtomicPrecheck(semantics)
		normalized := semantics.normalize(value, c)
		if !known && !c.entityBearingMember(member) {
			// A separate ordinary assessment may prove this particular candidate
			// succeeds, even if its type contains lexically rejected guarded
			// alternatives. Its non-success supplies no proof and publishes no
			// technical state; the rejection-only checks remain authoritative.
			trial := c.rejectionTrial()
			assessment, checked := trial.AssessSimpleMemberValue(member, value, display)
			if checked == nil && trial.AssessmentVersion == 0 {
				normalized, known = assessment.Lexical, true
			}
		}
		failure = validatePatternFacets(restriction, normalized, display, c)
		if failure == nil || TechnicalFailure(failure) {
			return nil, true
		}
		rejected = failure
		if known {
			return rejected, true
		}
	}
	return rejected, false
}

// No mutable cache or technical epoch is shared with a hypothetical trial.
// Schema definitions and the execution meter remain shared and immutable.
func (c *Context) rejectionTrial() *Context {
	c.scratch(int64(unsafe.Sizeof(Context{})) + int64(unsafe.Sizeof(legacyBoundState{})))
	trial := &Context{Schema: c.Schema, run: c.run}
	if c.run != nil {
		trial.local = &boundState{bindWork: &bindWork{}, run: c.run}
	}
	return trial
}
