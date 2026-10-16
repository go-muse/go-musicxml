package xsd

import (
	"context"
	"testing"
)

// These fixtures use well-formed XSD 1.0 instance contracts. Compile accepts
// immutable schema metadata; these tests do not ask it to certify schemas.
// Pinned authority: REC-xmlschema-2-20041028, sections 3.2.4, 3.2.5,
// 3.2.9, 3.2.15, 3.2.16, 3.2.18 and 4.3.1; and
// REC-xmlschema-1-20041028, section 3.10.4.
func truthfulnessScalar(t *testing.T, typ TypeRef, text string, namespaces map[string]string) Result {
	t.Helper()
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: typ}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRun(c, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "n"})
	b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, Text: text, TextKnown: true, Namespaces: namespaces, AttributesComplete: true, ChildrenComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	return r.Results(b).Scalar
}
func truthfulnessRestriction(builtin string) *SimpleSchema {
	return &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: builtin}}}
}
func TestTruthfulnessTemporalBoundsDoNotInventDatatypeFailure(t *testing.T) {
	s := truthfulnessRestriction("date")
	s.HasMinInclusive, s.MinInclusive = true, "2024-01-01"
	got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, "2024-02-29", nil)
	if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) == 0 {
		t.Fatalf("valid date with unsupported ordering must be incomplete, not invalid: %+v", got)
	}
}
func TestTruthfulnessBinaryLengthsMeasureOctets(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		length     uint64
		valid      bool
	}{
		{"hexBinary", "AA", 1, true}, {"hexBinary", "AA", 2, false},
		{"base64Binary", "YQ==", 1, true}, {"base64Binary", "YQ==", 4, false},
	} {
		t.Run(tc.name+tc.text+string(rune('0'+tc.length)), func(t *testing.T) {
			s := truthfulnessRestriction(tc.name)
			s.HasLength, s.Length = true, tc.length
			got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, tc.text, nil)
			if !got.Complete || got.Valid != tc.valid {
				t.Fatalf("octet length valid=%v: %+v", tc.valid, got)
			}
		})
	}
}
func TestTruthfulnessBase64LexicalSpace(t *testing.T) {
	for _, tc := range []struct {
		text  string
		valid bool
	}{
		{"Y Q = =", true}, {" Y\tQ\n=\r= ", true}, {"YR==", false}, {"YWJ=", false},
		{"YQ==", true}, {"YWI=", true}, {"Y\u00a0Q==", false}, {"", true},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got := truthfulnessScalar(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "base64Binary"}}, tc.text, nil)
			if !got.Complete || got.Valid != tc.valid {
				t.Fatalf("base64 lexical valid=%v: %+v", tc.valid, got)
			}
		})
	}
}
func TestTruthfulnessFloatLexicalSpace(t *testing.T) {
	for _, builtin := range []string{"float", "double"} {
		for _, text := range []string{"0x1p0", "1_0", "Inf", "+INF", "nan"} {
			t.Run(builtin+text, func(t *testing.T) {
				got := truthfulnessScalar(t, TypeRef{Name: QName{Space: XSDNamespace, Local: builtin}}, text, nil)
				if !got.Complete || got.Valid {
					t.Fatalf("Go-only lexical form must be invalid: %+v", got)
				}
			})
		}
	}
}
func TestTruthfulnessFloatBoundsRemainExplicitlyUnavailable(t *testing.T) {
	for _, builtin := range []string{"float", "double"} {
		t.Run(builtin, func(t *testing.T) {
			s := truthfulnessRestriction(builtin)
			s.HasMaxInclusive, s.MaxInclusive = true, "16777216"
			got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, "16777217", nil)
			// The pinned value-space answer differs: float rounds to the bound,
			// double does not. Until that operation is elected, neither receives
			// a normative comparison result from the float64 compatibility path.
			if got.Complete || !got.Valid || len(got.Issues) != 0 || len(got.Limitations) == 0 {
				t.Fatalf("unsupported ordered comparison claimed validity: %+v", got)
			}
		})
	}
}
func TestTruthfulnessQNameUnboundCannotPass(t *testing.T) {
	got := truthfulnessScalar(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "QName"}}, "p:name", map[string]string{"xml": "http://www.w3.org/XML/1998/namespace"})
	if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) != 1 || got.Limitations[0].Reason != "xsd-assessment-incomplete" || got.Limitations[0].Cause == nil || got.Limitations[0].Cause.Error() != "validation: incomplete assessment: incomplete XSD assessment: ordinary QName namespace-value assessment is unavailable" {
		t.Fatalf("unbound prefix must retain the exact unavailable QName assessment: %+v", got)
	}
}
func TestTruthfulnessWildcardNamespaceAdmission(t *testing.T) {
	for _, ns := range []string{"##local", "urn:allowed"} {
		t.Run(ns, func(t *testing.T) {
			typ := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleAny, Occurrence: Occurrence{Min: 1, Max: 1}, Any: &AnySchema{Namespace: ns, ProcessContents: "skip"}}}
			c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
			if err != nil {
				t.Fatal(err)
			}
			r, err := NewRun(c, &testMeter{})
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			u, _ := c.Root(QName{Local: "n"})
			b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Children: []Child{{ID: 2, Name: QName{Space: "urn:forbidden", Local: "item"}}}})
			if err != nil {
				t.Fatal(err)
			}
			got := r.Results(b).Content
			if !got.Complete || got.Valid {
				t.Fatalf("skip cannot admit forbidden namespace: %+v", got)
			}
		})
	}
}
func TestTruthfulnessAttributeWildcardLocalListAdmission(t *testing.T) {
	typ := &ComplexSchema{Form: ComplexDirect, AnyAttribute: &AnyAttributeSchema{Namespace: "##local urn:allowed", ProcessContents: "skip"}}
	r, b := bindSingle(t, TypeRef{InlineComplex: typ}, "", Attribute{Attr: Attr{Name: QName{Local: "a"}, Value: "x"}, ValueKnown: true})
	got := r.Results(b).Attributes
	if !got.Complete || !got.Valid {
		t.Fatalf("namespace list includes local attributes: %+v", got)
	}
}

func TestTruthfulnessGuardsPreserveLexicalFailures(t *testing.T) {
	for _, tc := range []struct{ builtin, text string }{
		{"date", "2026-02-30"}, {"float", "0x1p0"}, {"double", "nan"}, {"QName", "a:b:c"},
	} {
		t.Run(tc.builtin, func(t *testing.T) {
			typ := TypeRef{Name: QName{Space: XSDNamespace, Local: tc.builtin}}
			if tc.builtin != "QName" {
				s := truthfulnessRestriction(tc.builtin)
				s.HasMinInclusive = true
				s.MinInclusive = "0"
				if tc.builtin == "date" {
					s.MinInclusive = "2024-01-01"
				}
				typ = TypeRef{InlineSimple: s}
			}
			got := truthfulnessScalar(t, typ, tc.text, nil)
			if got.Valid || !got.Complete || len(got.Limitations) != 0 || len(got.Issues) != 1 || got.Issues[0].Constraint != "datatype" {
				t.Fatalf("lexical invalidity hidden by guard: %+v", got)
			}
		})
	}
}
func TestTruthfulnessFloatLexicalControls(t *testing.T) {
	for _, builtin := range []string{"float", "double"} {
		for _, text := range []string{"1", "1.0", ".5", "1.", "+1.2e-3", "-0", "INF", "-INF", "NaN", "1e9999", "1e-9999"} {
			t.Run(builtin+text, func(t *testing.T) {
				got := truthfulnessScalar(t, TypeRef{Name: QName{Space: XSDNamespace, Local: builtin}}, text, nil)
				if !got.Complete || !got.Valid {
					t.Fatalf("valid lexical form rejected: %+v", got)
				}
			})
		}
	}
}
func TestTruthfulnessBoundsUseActualBuiltinFamily(t *testing.T) {
	for _, builtin := range []string{"integer", "decimal"} {
		s := truthfulnessRestriction(builtin)
		s.HasMaxInclusive, s.MaxInclusive = true, "9007199254740992"
		got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, "9007199254740993", nil)
		if !got.Complete || got.Valid || len(got.Issues) != 1 || got.Issues[0].Constraint != "maxInclusive" {
			t.Fatalf("exact %s bound regressed: %+v", builtin, got)
		}
	}
	s := truthfulnessRestriction("string")
	s.HasMaxInclusive, s.MaxInclusive = true, "1"
	got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, "9", nil)
	if got.Complete || !got.Valid || len(got.Issues) != 0 {
		t.Fatalf("inapplicable bound became instance invalidity: %+v", got)
	}
}
func TestTruthfulnessIndependentFactsSurviveScalarGuard(t *testing.T) {
	s := truthfulnessRestriction("date")
	s.HasMinInclusive, s.MinInclusive = true, "2024-01-01"
	typ := &ComplexSchema{Form: ComplexSimpleContentExtension, Base: &TypeRef{InlineSimple: s}, Attributes: []AttributeSchema{{Name: QName{Local: "bad"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}
	r, b := bindSingle(t, TypeRef{InlineComplex: typ}, "2024-02-29", Attribute{Attr: Attr{Name: QName{Local: "bad"}, Value: "x"}, ValueKnown: true})
	got := r.Results(b)
	if got.Attributes.Valid || !got.Attributes.Complete || got.Scalar.Complete || !got.Scalar.Valid {
		t.Fatalf("independent attribute failure erased: %+v", got)
	}
}
func TestTruthfulnessUnknownUnionMemberCannotFallThrough(t *testing.T) {
	qname := &SimpleMember{Name: QName{Space: XSDNamespace, Local: "QName"}}
	stringMember := &SimpleMember{Name: QName{Space: XSDNamespace, Local: "string"}}
	for _, reverse := range []bool{false, true} {
		members := []*SimpleMember{qname, stringMember}
		if reverse {
			members[0], members[1] = members[1], members[0]
		}
		got := truthfulnessScalar(t, TypeRef{InlineSimple: &SimpleSchema{Form: SimpleUnion, Members: members}}, "p:name", map[string]string{"p": "urn:bound"})
		if !got.Valid || got.Complete != reverse || len(got.Issues) != 0 {
			t.Fatalf("union reverse=%v fabricated member choice: %+v", reverse, got)
		}
	}
}

func truthfulnessWildcard(t *testing.T, namespace, mode, space string, attribute bool) Result {
	t.Helper()
	typ := &ComplexSchema{Form: ComplexDirect}
	subject := Subject{ID: 1, Name: QName{Space: "urn:owner", Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
	if attribute {
		typ.AnyAttribute = &AnyAttributeSchema{Namespace: namespace, ProcessContents: mode}
		subject.Attributes = []Attribute{{Attr: Attr{Name: QName{Space: space, Local: "a"}, Value: "x"}, ValueKnown: true}}
	} else {
		typ.Particle = &ParticleSchema{Kind: ParticleAny, Occurrence: Occurrence{Min: 1, Max: 1}, Any: &AnySchema{Namespace: namespace, ProcessContents: mode}}
		subject.Children = []Child{{ID: 2, Name: QName{Space: space, Local: "item"}}}
	}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{subject.Name: {Name: subject.Name, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRun(c, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	u, _ := c.Root(subject.Name)
	b, err := r.Bind(context.Background(), u, subject)
	if err != nil {
		t.Fatal(err)
	}
	if attribute {
		return r.Results(b).Attributes
	}
	return r.Results(b).Content
}
func TestTruthfulnessWildcardExplicitNamespacesAndModes(t *testing.T) {
	for _, attribute := range []bool{false, true} {
		for _, ns := range []string{"##local urn:allowed", "urn:allowed ##local"} {
			for _, space := range []string{"", "urn:allowed", "urn:forbidden"} {
				for _, mode := range []string{"skip", "lax", "strict"} {
					got := truthfulnessWildcard(t, ns, mode, space, attribute)
					valid := space != "urn:forbidden"
					complete := !valid || mode == "skip"
					if got.Valid != valid || got.Complete != complete {
						t.Fatalf("attribute=%v ns=%q space=%q mode=%s: %+v", attribute, ns, space, mode, got)
					}
				}
			}
		}
	}
}
func TestTruthfulnessWildcardMissingTargetContextIsUnavailable(t *testing.T) {
	for _, attribute := range []bool{false, true} {
		for _, ns := range []string{"##other", "##targetNamespace"} {
			got := truthfulnessWildcard(t, ns, "skip", "urn:owner", attribute)
			if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) == 0 {
				t.Fatalf("attribute=%v %s guessed defining namespace: %+v", attribute, ns, got)
			}
		}
		got := truthfulnessWildcard(t, "##other", "skip", "", attribute)
		if got.Valid || !got.Complete {
			t.Fatalf("##other must exclude absent namespace even without target context: %+v", got)
		}
		got = truthfulnessWildcard(t, "##targetNamespace urn:allowed", "skip", "urn:allowed", attribute)
		if !got.Valid || !got.Complete {
			t.Fatalf("explicit namespace admission unnecessarily unknown: %+v", got)
		}
	}
}
func TestTruthfulnessWildcardInvalidMetadataIsNotInstanceFailure(t *testing.T) {
	for _, attribute := range []bool{false, true} {
		for _, ns := range []string{"##any urn:allowed", "##other ##local", "##unknown"} {
			got := truthfulnessWildcard(t, ns, "skip", "urn:forbidden", attribute)
			if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) == 0 {
				t.Fatalf("attribute=%v invalid ns %q became instance result: %+v", attribute, ns, got)
			}
		}
		got := truthfulnessWildcard(t, "##any", "typo", "urn:forbidden", attribute)
		if !got.Valid || got.Complete || len(got.Issues) != 0 {
			t.Fatalf("invalid processContents became instance result: %+v", got)
		}
	}
}

func TestTruthfulnessBinaryFacetKindsAndLayers(t *testing.T) {
	for _, builtin := range []string{"hexBinary", "base64Binary"} {
		value := "AA"
		if builtin == "base64Binary" {
			value = " Y Q = = "
		}
		for _, kind := range []string{"minLength", "maxLength"} {
			for _, limit := range []uint64{0, 1, 2} {
				s := truthfulnessRestriction(builtin)
				want := true
				if kind == "minLength" {
					s.HasMinLength, s.MinLength = true, limit
					want = limit <= 1
				} else {
					s.HasMaxLength, s.MaxLength = true, limit
					want = limit >= 1
				}
				outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: s}}
				got := truthfulnessScalar(t, TypeRef{InlineSimple: outer}, value, nil)
				if !got.Complete || got.Valid != want {
					t.Fatalf("%s %s=%d: %+v", builtin, kind, limit, got)
				}
			}
		}
	}
	for _, builtin := range []string{"hexBinary", "base64Binary"} {
		item := &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Name: QName{Space: XSDNamespace, Local: builtin}}}
		outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: item}, HasLength: true, Length: 2}
		text := "AA BB"
		if builtin == "base64Binary" {
			text = "YQ== Yg=="
		}
		got := truthfulnessScalar(t, TypeRef{InlineSimple: outer}, text, nil)
		if !got.Complete || !got.Valid {
			t.Fatalf("binary list length did not count items: %+v", got)
		}
	}
}
func TestTruthfulnessBinaryEqualityRemainsUnavailable(t *testing.T) {
	for _, builtin := range []string{"hexBinary", "base64Binary"} {
		s := truthfulnessRestriction(builtin)
		value := "aa"
		s.Enumerations = []string{"AA"}
		if builtin == "base64Binary" {
			value = "Y Q = ="
			s.Enumerations = []string{"YQ=="}
		}
		got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, value, nil)
		if got.Complete || !got.Valid || len(got.Issues) != 0 {
			t.Fatalf("binary equality was silently elected: %+v", got)
		}
	}
}

func TestTruthfulnessQNamePatternRejectionPrecedesContextGuard(t *testing.T) {
	for _, enumeration := range []bool{false, true} {
		restricted := truthfulnessRestriction("QName")
		restricted.Patterns = []string{"allowed"}
		if enumeration {
			restricted.Enumerations = []string{"allowed"}
		}
		got := truthfulnessScalar(t, TypeRef{InlineSimple: restricted}, "p:forbidden", map[string]string{"p": "urn:bound"})
		if got.Valid || !got.Complete || len(got.Issues) != 1 || got.Issues[0].Constraint != "pattern" || len(got.Limitations) != 0 {
			t.Fatalf("QName lexical facet hidden by context/equality prerequisite: %+v", got)
		}
	}
}
func TestTruthfulnessRejectedQNameMemberAllowsProvedFallback(t *testing.T) {
	restricted := truthfulnessRestriction("QName")
	restricted.Patterns = []string{"allowed"}
	chained := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: restricted}}
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: chained}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}
	got := truthfulnessScalar(t, TypeRef{InlineSimple: union}, "p:forbidden", map[string]string{"p": "urn:bound"})
	if !got.Valid || !got.Complete || len(got.Issues) != 0 || len(got.Limitations) != 0 {
		t.Fatalf("proved member rejection did not permit fallback: %+v", got)
	}
	got = truthfulnessScalar(t, TypeRef{InlineSimple: union}, "allowed", nil)
	if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) == 0 {
		t.Fatalf("viable QName member incorrectly fell through: %+v", got)
	}
}

func TestTruthfulnessQNameLengthNeverUsesLexicalRuneCount(t *testing.T) {
	for _, length := range []uint64{0, 1, 100} {
		s := truthfulnessRestriction("QName")
		s.HasLength, s.Length = true, length
		got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, "p:name", map[string]string{"p": "urn:bound"})
		if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) == 0 {
			t.Fatalf("QName length=%d invented lexical-count invalidity: %+v", length, got)
		}
	}
}

func TestTruthfulnessInheritedBoundsCannotHideOuterPattern(t *testing.T) {
	for _, tc := range []struct{ builtin, bound, pattern, value string }{
		{"float", "0", "2", "1"}, {"double", "0", "2", "1"}, {"date", "2024-01-01", "2025-.*", "2024-02-29"},
	} {
		inner := truthfulnessRestriction(tc.builtin)
		inner.HasMinInclusive, inner.MinInclusive = true, tc.bound
		outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: inner}, Patterns: []string{tc.pattern}}
		got := truthfulnessScalar(t, TypeRef{InlineSimple: outer}, tc.value, nil)
		if got.Valid || !got.Complete || len(got.Issues) != 1 || got.Issues[0].Constraint != "pattern" || len(got.Limitations) != 0 {
			t.Fatalf("%s inner Unsupported hid outer rejection: %+v", tc.builtin, got)
		}
		union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: outer}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}
		got = truthfulnessScalar(t, TypeRef{InlineSimple: union}, tc.value, nil)
		if !got.Valid || !got.Complete || len(got.Issues) != 0 || len(got.Limitations) != 0 {
			t.Fatalf("%s proved rejection blocked union fallback: %+v", tc.builtin, got)
		}
	}
}

func TestTruthfulnessQNameListIndependentRejections(t *testing.T) {
	list := &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "QName"}}}
	for _, kind := range []string{"later-item", "length", "pattern"} {
		t.Run(kind, func(t *testing.T) {
			typ := list
			text := "p:name a:b:c"
			if kind == "length" {
				text = "p:name"
				typ = &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: list}, HasLength: true, Length: 2}
			}
			if kind == "pattern" {
				text = "p:name"
				typ = &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: list}, Patterns: []string{"allowed"}}
			}
			got := truthfulnessScalar(t, TypeRef{InlineSimple: typ}, text, map[string]string{"p": "urn:bound"})
			if got.Valid || !got.Complete || len(got.Issues) != 1 || len(got.Limitations) != 0 {
				t.Fatalf("independent list rejection hidden: %+v", got)
			}
			union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: typ}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}
			got = truthfulnessScalar(t, TypeRef{InlineSimple: union}, text, nil)
			if !got.Valid || !got.Complete || len(got.Issues) != 0 || len(got.Limitations) != 0 {
				t.Fatalf("proved list rejection blocked fallback: %+v", got)
			}
		})
	}
}
func TestTruthfulnessListUnionPrecheckNeverSelectsCandidate(t *testing.T) {
	item := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Name: QName{Space: XSDNamespace, Local: "integer"}}}}
	list := &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Inline: item}}
	got := truthfulnessScalar(t, TypeRef{InlineSimple: list}, "p:name a:b:c", nil)
	if got.Valid || !got.Complete || len(got.Issues) != 1 || len(got.Limitations) != 0 {
		t.Fatalf("later union item known rejection hidden: %+v", got)
	}
	got = truthfulnessScalar(t, TypeRef{InlineSimple: list}, "p:name 1", nil)
	if !got.Valid || got.Complete || len(got.Issues) != 0 {
		t.Fatalf("viable unknown item became selected success: %+v", got)
	}
}

func TestTruthfulnessListLaterSupportedAtomicFailureSurvives(t *testing.T) {
	for _, kind := range []string{"bound", "enumeration", "string-length"} {
		t.Run(kind, func(t *testing.T) {
			atomic := truthfulnessRestriction("integer")
			word := "-1"
			if kind == "bound" {
				atomic.HasMinInclusive, atomic.MinInclusive = true, "0"
			}
			if kind == "enumeration" {
				atomic.Enumerations = []string{"0"}
			}
			if kind == "string-length" {
				atomic = truthfulnessRestriction("string")
				atomic.HasLength, atomic.Length = true, 1
				word = "a:b:c"
			}
			item := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Inline: atomic}}}
			list := &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Inline: item}}
			got := truthfulnessScalar(t, TypeRef{InlineSimple: list}, "p:name "+word, map[string]string{"p": "urn:bound"})
			if got.Valid || !got.Complete || len(got.Issues) != 1 || len(got.Limitations) != 0 {
				t.Fatalf("supported later item failure hidden: %+v", got)
			}
		})
	}
}

func TestTruthfulnessListPrecheckDoesNotVisitLaterViableUnionAlternatives(t *testing.T) {
	bad := truthfulnessRestriction("QName")
	bad.Patterns = []string{"["}
	for _, first := range []string{"string", "QName"} {
		item := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: first}}, {Inline: bad}}}
		list := &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Inline: item}}
		got := truthfulnessScalar(t, TypeRef{InlineSimple: list}, "p:name", nil)
		if !got.Valid || got.Complete != (first == "string") || len(got.Issues) != 0 {
			t.Fatalf("first %s was polluted by later member: %+v", first, got)
		}
		if first == "QName" && (len(got.Limitations) != 1 || got.Limitations[0].Cause == nil || got.Limitations[0].Cause.Error() != "validation: incomplete assessment: incomplete XSD assessment: ordinary QName namespace-value assessment is unavailable") {
			t.Fatalf("later pattern was assessed: %+v", got)
		}
	}
}

func TestTruthfulnessGuardedUnionRestrictionKeepsIndependentPattern(t *testing.T) {
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}
	outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: union}, Patterns: []string{"allowed"}}
	for _, text := range []string{"p:forbidden", " p:forbidden ", "\tp:forbidden\n"} {
		got := truthfulnessScalar(t, TypeRef{InlineSimple: outer}, text, map[string]string{"p": "urn:bound"})
		if got.Valid || !got.Complete || len(got.Issues) != 1 || len(got.Limitations) != 0 {
			t.Fatalf("normalization-independent union pattern rejection hidden for %q: %+v", text, got)
		}
	}
	outer.Patterns = []string{" p:name "}
	got := truthfulnessScalar(t, TypeRef{InlineSimple: outer}, " p:name ", nil)
	if !got.Valid || got.Complete || len(got.Issues) != 0 {
		t.Fatalf("selection-dependent normalization was guessed: %+v", got)
	}
}
func TestTruthfulnessListRestrictedUnionKeepsSupportedFailures(t *testing.T) {
	for _, kind := range []string{"pattern", "enumeration"} {
		t.Run(kind, func(t *testing.T) {
			union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Name: QName{Space: XSDNamespace, Local: "integer"}}}}
			restricted := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: union}}
			if kind == "pattern" {
				restricted.Patterns = []string{"p:.*|0"}
			} else {
				restricted.Enumerations = []string{"0"}
			}
			list := &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Inline: restricted}}
			got := truthfulnessScalar(t, TypeRef{InlineSimple: list}, "p:name -1", map[string]string{"p": "urn:bound"})
			if got.Valid || !got.Complete || len(got.Issues) != 1 || len(got.Limitations) != 0 {
				t.Fatalf("later restricted-union %s rejection hidden: %+v", kind, got)
			}
		})
	}
}

func TestTruthfulnessUnionPatternUsesOnlyViableNormalizations(t *testing.T) {
	for _, kind := range []string{"common-collapse", "rejected-preserve", "rejected-replace"} {
		t.Run(kind, func(t *testing.T) {
			member := &SimpleMember{Name: QName{Space: XSDNamespace, Local: "token"}}
			if kind != "common-collapse" {
				builtin := "string"
				if kind == "rejected-replace" {
					builtin = "normalizedString"
				}
				s := truthfulnessRestriction(builtin)
				s.Patterns = []string{"allowed"}
				member = &SimpleMember{Inline: s}
			}
			union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, member}}
			outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: union}, Patterns: []string{" .* "}}
			got := truthfulnessScalar(t, TypeRef{InlineSimple: outer}, " p:name ", map[string]string{"p": "urn:bound"})
			if got.Valid || !got.Complete || len(got.Issues) != 1 || len(got.Limitations) != 0 {
				t.Fatalf("impossible normalization hid a known pattern rejection: %+v", got)
			}
		})
	}
}

func TestTruthfulnessUnionPatternProofPreservesOrderAndIsolation(t *testing.T) {
	qname := &SimpleMember{Name: QName{Space: XSDNamespace, Local: "QName"}}
	token := &SimpleMember{Name: QName{Space: XSDNamespace, Local: "token"}}
	preserve := &SimpleMember{Name: QName{Space: XSDNamespace, Local: "string"}}
	bad := truthfulnessRestriction("string")
	bad.Patterns = []string{"["}
	floating := truthfulnessRestriction("float")
	floating.HasMinInclusive, floating.MinInclusive = true, "0"
	for _, tc := range []struct {
		name    string
		members []*SimpleMember
		invalid bool
	}{
		{"known-first-does-not-retry", []*SimpleMember{token, qname, preserve}, true},
		{"known-after-guard-does-not-retry", []*SimpleMember{qname, token, preserve}, true},
		{"known-list-does-not-retry", []*SimpleMember{qname, {Inline: &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "NMTOKEN"}}}}, preserve}, true},
		{"known-per-value-list-does-not-retry", []*SimpleMember{{Inline: floating}, {Inline: &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Inline: &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{qname, {Name: QName{Space: XSDNamespace, Local: "integer"}}}}}}}, preserve}, true},
		{"known-before-bad-metadata", []*SimpleMember{qname, token, {Inline: bad}}, true},
		{"unknown-before-bad-metadata", []*SimpleMember{qname, {Inline: bad}}, false},
		{"possible-match-before-bad-metadata", []*SimpleMember{qname, preserve, {Inline: bad}}, false},
		{"nested-known-does-not-retry", []*SimpleMember{qname, {Inline: &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{token, preserve}}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			union := &SimpleSchema{Form: SimpleUnion, Members: tc.members}
			outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: union}, Patterns: []string{" .* "}}
			text := " p:name "
			if tc.name == "known-per-value-list-does-not-retry" {
				text = " -1 "
			}
			got := truthfulnessScalar(t, TypeRef{InlineSimple: outer}, text, map[string]string{"p": "urn:bound"})
			if tc.invalid {
				if got.Valid || !got.Complete || len(got.Issues) != 1 || len(got.Limitations) != 0 {
					t.Fatalf("known base candidate was retried after outer rejection: %+v", got)
				}
			} else if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) != 1 || got.Limitations[0].Cause == nil || got.Limitations[0].Cause.Error() != "validation: incomplete assessment: incomplete XSD assessment: ordinary QName namespace-value assessment is unavailable" {
				t.Fatalf("hypothetical later member polluted the real first candidate: %+v", got)
			}
		})
	}
}

func TestTruthfulnessBinaryEnumerationCannotHideFacetFailure(t *testing.T) {
	for _, tc := range []struct{ builtin, text, enumeration, pattern string }{
		{"hexBinary", "CC", "AA", "BB"},
		{"base64Binary", "YQ==", "Yg==", "Yg=="},
	} {
		for _, facet := range []string{"pattern", "length", "minLength", "maxLength"} {
			for _, shape := range []string{"same-restriction", "outer-restriction", "list-item"} {
				t.Run(tc.builtin+"/"+facet+"/"+shape, func(t *testing.T) {
					base := truthfulnessRestriction(tc.builtin)
					base.Enumerations = []string{tc.enumeration}
					s := base
					if shape == "outer-restriction" {
						s = &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: base}}
					}
					switch facet {
					case "pattern":
						s.Patterns = []string{tc.pattern}
					case "length":
						s.HasLength, s.Length = true, 3
					case "minLength":
						s.HasMinLength, s.MinLength = true, 3
					case "maxLength":
						s.HasMaxLength, s.MaxLength = true, 0
					}
					if shape == "list-item" {
						s = &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Inline: s}}
					}
					got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, tc.text, nil)
					if got.Valid || !got.Complete || len(got.Limitations) != 0 || len(got.Issues) != 1 || got.Issues[0].Constraint != facet {
						t.Fatalf("independent %s rejection hidden by binary equality: %+v", facet, got)
					}
				})
			}
		}
		// Passing independently checkable facets cannot establish equality.
		s := truthfulnessRestriction(tc.builtin)
		s.Enumerations, s.Patterns = []string{tc.text}, []string{tc.text}
		s.HasLength, s.Length = true, 1
		got := truthfulnessScalar(t, TypeRef{InlineSimple: s}, tc.text, nil)
		if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) == 0 {
			t.Fatalf("binary equality must remain unavailable: %+v", got)
		}
	}
}
