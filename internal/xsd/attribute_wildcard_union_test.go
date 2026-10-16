package xsd

import (
	"context"
	"strings"
	"testing"
)

func wildcardUnionType(simple bool, base, derived *AnyAttributeSchema, restriction bool) *ComplexSchema {
	parent := &ComplexSchema{Form: ComplexDirect, AnyAttribute: base}
	form := ComplexComplexContentExtension
	if simple {
		parent.Form = ComplexSimpleContentExtension
		parent.Base = &TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}
		form = ComplexSimpleContentExtension
	}
	if restriction {
		form = ComplexComplexContentRestriction
		if simple {
			form = ComplexSimpleContentRestriction
		}
	}
	return &ComplexSchema{Form: form, Base: &TypeRef{InlineComplex: parent}, AnyAttribute: derived}
}
func unionWildcard(namespace, mode string) *AnyAttributeSchema {
	return &AnyAttributeSchema{Namespace: namespace, ProcessContents: mode}
}
func wildcardUnionResult(t *testing.T, typ *ComplexSchema, namespace string) Result {
	t.Helper()
	r, b := bindSingle(t, TypeRef{InlineComplex: typ}, "", Attribute{Attr: Attr{Name: QName{Space: namespace, Local: "a"}, Value: "x"}, ValueKnown: true})
	return r.Results(b).Attributes
}
func TestAttributeWildcardUnionFiniteAndAbsent(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, tc := range []struct {
			name          string
			base, derived *AnyAttributeSchema
			allowed       []string
			forbidden     []string
		}{
			{"different", unionWildcard("urn:base", "skip"), unionWildcard("urn:derived", "skip"), []string{"urn:base", "urn:derived"}, []string{"", "urn:outside"}},
			{"local-list", unionWildcard("urn:base ##local urn:base", "skip"), unionWildcard("urn:derived ##local", "skip"), []string{"", "urn:base", "urn:derived"}, []string{"urn:outside"}},
			{"base-absent", nil, unionWildcard("urn:derived", "skip"), []string{"urn:derived"}, []string{"urn:base", ""}},
			{"derived-absent", unionWildcard("urn:base", "skip"), nil, []string{"urn:base"}, []string{"urn:derived", ""}},
			{"both-absent", nil, nil, nil, []string{"urn:base", ""}},
			{"base-any", unionWildcard("##any", "skip"), unionWildcard("urn:derived", "skip"), []string{"", "urn:base", "urn:derived", "urn:outside"}, nil},
			{"derived-any", unionWildcard("urn:base", "skip"), unionWildcard("", "skip"), []string{"", "urn:base", "urn:derived", "urn:outside"}, nil},
			{"other-any", unionWildcard("##other", "skip"), unionWildcard("##any", "skip"), []string{"", "urn:outside"}, nil},
		} {
			t.Run(tc.name+map[bool]string{false: "/complex", true: "/simple"}[simple], func(t *testing.T) {
				typ := wildcardUnionType(simple, tc.base, tc.derived, false)
				for _, ns := range tc.allowed {
					if got := wildcardUnionResult(t, typ, ns); !got.Valid || !got.Complete {
						t.Fatalf("namespace %q: %+v", ns, got)
					}
				}
				for _, ns := range tc.forbidden {
					if got := wildcardUnionResult(t, typ, ns); got.Valid || !got.Complete || len(got.Issues) != 1 {
						t.Fatalf("namespace %q: %+v", ns, got)
					}
				}
			})
		}
	}
}
func TestAttributeWildcardUnionDerivedProcessContents(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, baseMode := range []string{"skip", "lax", "strict", ""} {
			for _, derivedMode := range []string{"skip", "lax", "strict", ""} {
				typ := wildcardUnionType(simple, unionWildcard("urn:base", baseMode), unionWildcard("urn:derived", derivedMode), false)
				for _, ns := range []string{"urn:base", "urn:derived"} {
					got := wildcardUnionResult(t, typ, ns)
					if !got.Valid || got.Complete != (derivedMode == "skip") || len(got.Issues) != 0 {
						t.Fatalf("simple=%v base=%q derived=%q ns=%q: %+v", simple, baseMode, derivedMode, ns, got)
					}
					if derivedMode != "skip" {
						reason := "wildcard-strict-unavailable"
						if derivedMode == "lax" {
							reason = "wildcard-lax-unavailable"
						}
						found := false
						for _, limitation := range got.Limitations {
							found = found || limitation.Reason == reason
						}
						if !found {
							t.Fatalf("lost derived mode %q: %+v", derivedMode, got)
						}
					}
				}
			}
		}
	}
}
func TestAttributeWildcardRestrictionDoesNotUnion(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, local := range []*AnyAttributeSchema{nil, unionWildcard("urn:derived", "skip")} {
			typ := wildcardUnionType(simple, unionWildcard("urn:base urn:derived", "skip"), local, true)
			for _, ns := range []string{"urn:base", "urn:derived"} {
				want := local != nil && ns == "urn:derived"
				if got := wildcardUnionResult(t, typ, ns); got.Valid != want || !got.Complete {
					t.Fatalf("simple=%v local=%v ns=%q: %+v", simple, local, ns, got)
				}
			}
			extended := &ComplexSchema{Form: typ.Form, Base: &TypeRef{InlineComplex: typ}, AnyAttribute: unionWildcard("urn:last", "skip")}
			if simple {
				extended.Form = ComplexSimpleContentExtension
			} else {
				extended.Form = ComplexComplexContentExtension
			}
			for _, ns := range []string{"urn:base", "urn:derived", "urn:last"} {
				want := ns == "urn:last" || (local != nil && ns == "urn:derived")
				if got := wildcardUnionResult(t, extended, ns); got.Valid != want || !got.Complete {
					t.Fatalf("inherited simple=%v ns=%q: %+v", simple, ns, got)
				}
			}
		}
	}
}
func TestAttributeWildcardUnionUnresolvedIsNotInvalid(t *testing.T) {
	for _, pair := range [][2]string{{"##other", "##local"}, {"urn:base", "##other"}, {"##other", "##other"}, {"##any urn:bad", "urn:derived"}} {
		for _, simple := range []bool{false, true} {
			typ := wildcardUnionType(simple, unionWildcard(pair[0], "skip"), unionWildcard(pair[1], "skip"), false)
			for _, ns := range []string{"", "urn:base", "urn:outside"} {
				got := wildcardUnionResult(t, typ, ns)
				if !got.Valid || got.Complete || len(got.Issues) != 0 || len(got.Limitations) == 0 {
					t.Fatalf("pair=%q simple=%v ns=%q: %+v", pair, simple, ns, got)
				}
			}
			form := ComplexComplexContentExtension
			if simple {
				form = ComplexSimpleContentExtension
			}
			typ = &ComplexSchema{Form: form, Base: &TypeRef{InlineComplex: typ}, AnyAttribute: unionWildcard("urn:last", "skip")}
			if got := wildcardUnionResult(t, typ, "urn:outside"); !got.Valid || got.Complete || len(got.Issues) != 0 {
				t.Fatalf("inherited unavailable %+v", got)
			}
		}
	}
}
func TestAttributeWildcardUnionRetainsKnownExplicitAdmission(t *testing.T) {
	typ := wildcardUnionType(false, unionWildcard("##targetNamespace urn:base", "skip"), unionWildcard("urn:derived", "skip"), false)
	for _, ns := range []string{"urn:base", "urn:derived"} {
		if got := wildcardUnionResult(t, typ, ns); !got.Valid || !got.Complete {
			t.Fatalf("%q: %+v", ns, got)
		}
	}
	if got := wildcardUnionResult(t, typ, "urn:unknown"); !got.Valid || got.Complete || len(got.Issues) != 0 {
		t.Fatalf("target context fabricated: %+v", got)
	}
}
func TestAttributeWildcardUnionCanonicalSetAndOwnership(t *testing.T) {
	base, derived := unionWildcard(" urn:base\t##local urn:base ", "strict"), unionWildcard("urn:derived ##local urn:base", "skip")
	got := MergeAnyAttribute(base, derived)
	if got == nil || got.Namespace != "urn:base ##local urn:derived" || got.ProcessContents != "skip" {
		t.Fatalf("finite set union: %+v", got)
	}
	if !strings.HasPrefix(base.Namespace, " ") || derived.Namespace != "urn:derived ##local urn:base" {
		t.Fatal("mutated input")
	}
	for _, ns := range []string{"", "urn:base", "urn:derived"} {
		if !AnyAttributeAllows(got, QName{Space: ns, Local: "a"}) {
			t.Fatalf("compatibility admission lost %q", ns)
		}
	}
}
func TestAttributeWildcardUnionPreservesContentWitness(t *testing.T) {
	typ := wildcardUnionType(false, unionWildcard("urn:base", "skip"), unionWildcard("urn:derived", "skip"), false)
	typ.Particle = &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1}, Element: &ElementSchema{Name: QName{Local: "tail"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}}
	compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	run, err := NewRun(compiled, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	use, _ := compiled.Root(QName{Local: "n"})
	b, err := run.Bind(context.Background(), use, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Space: "urn:base", Local: "a"}, Value: "x"}, ValueKnown: true}}, Children: []Child{{ID: 2, Name: QName{Local: "tail"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := run.Results(b); !got.Attributes.Valid || !got.Attributes.Complete || !got.Content.Valid || !got.Content.Complete {
		t.Fatalf("result: %+v", got)
	}
	witnesses := run.Witnesses(b)
	if len(witnesses) != 1 || witnesses[0].Mode != WitnessDeclared || witnesses[0].Use.Declaration() != (QName{Local: "tail"}) {
		t.Fatalf("witnesses: %+v", witnesses)
	}
}

func TestAttributeWildcardUnionUnavailablePreservesDeclaredFacts(t *testing.T) {
	typ := wildcardUnionType(false, unionWildcard("##other", "skip"), unionWildcard("##local", "skip"), false)
	typ.Attributes = []AttributeSchema{{Name: QName{Local: "known"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}
	for _, unknown := range []bool{false, true} {
		attrs := []Attribute{{Attr: Attr{Name: QName{Local: "known"}, Value: "bad"}, ValueKnown: true}}
		if unknown {
			attrs = append(attrs, Attribute{Attr: Attr{Name: QName{Local: "unresolved"}, Value: "x"}, ValueKnown: true})
		}
		r, b := bindSingle(t, TypeRef{InlineComplex: typ}, "", attrs...)
		got := r.Results(b).Attributes
		if got.Valid || got.Complete == unknown || len(got.Issues) != 1 || got.Issues[0].Constraint != "datatype" {
			t.Fatalf("unknown=%v: %+v", unknown, got)
		}
	}
	// No undeclared attribute reaches the unresolved composition at all.
	r, b := bindSingle(t, TypeRef{InlineComplex: typ}, "")
	if got := r.Results(b).Attributes; !got.Valid || !got.Complete {
		t.Fatalf("unused wildcard: %+v", got)
	}
	// Restriction owns its wildcard and does not inherit union uncertainty.
	restricted := &ComplexSchema{Form: ComplexComplexContentRestriction, Base: &TypeRef{InlineComplex: typ}, AnyAttribute: unionWildcard("urn:only", "skip")}
	if got := wildcardUnionResult(t, restricted, "urn:only"); !got.Valid || !got.Complete {
		t.Fatalf("restriction: %+v", got)
	}
	if got := wildcardUnionResult(t, restricted, "urn:outside"); got.Valid || !got.Complete {
		t.Fatalf("restriction rejection: %+v", got)
	}
}

func TestAttributeWildcardUnionCurrentModel(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		base := "urn:base"
		if unavailable {
			base = "##other"
		}
		typ := wildcardUnionType(false, unionWildcard(base, "skip"), unionWildcard("urn:derived", "skip"), false)
		compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
		if err != nil {
			t.Fatal(err)
		}
		r, err := NewRun(compiled, &testMeter{})
		if err != nil {
			t.Fatal(err)
		}
		u, _ := compiled.Root(QName{Local: "n"})
		b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, CurrentModel: true, Current: CurrentScalar{Known: true, Value: SimpleValue{Domain: StringDomain, Text: ""}}, ChildrenOrderKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Space: "urn:base", Local: "a"}}, Current: CurrentScalar{Known: true, Value: SimpleValue{Domain: StringDomain, Text: "x"}}}}})
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Results(b).Attributes; !got.Valid || got.Complete == unavailable || len(got.Issues) != 0 {
			t.Fatalf("unavailable=%v: %+v", unavailable, got)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAttributeWildcardUnionInheritedModeAndUnavailable(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, mode := range []string{"skip", "lax", "strict", ""} {
			typ := wildcardUnionType(simple, unionWildcard("urn:base", mode), nil, false)
			form := ComplexComplexContentExtension
			if simple {
				form = ComplexSimpleContentExtension
			}
			typ = &ComplexSchema{Form: form, Base: &TypeRef{InlineComplex: typ}}
			if got := wildcardUnionResult(t, typ, "urn:base"); !got.Valid || got.Complete != (mode == "skip") {
				t.Fatalf("simple=%v inherited mode=%q: %+v", simple, mode, got)
			}
		}
		typ := wildcardUnionType(simple, unionWildcard("##other", "skip"), unionWildcard("##local", "skip"), false)
		form := ComplexComplexContentExtension
		if simple {
			form = ComplexSimpleContentExtension
		}
		for _, outer := range []*AnyAttributeSchema{nil, unionWildcard("##any", "skip")} {
			inherited := &ComplexSchema{Form: form, Base: &TypeRef{InlineComplex: typ}, AnyAttribute: outer}
			if got := wildcardUnionResult(t, inherited, "urn:outside"); !got.Valid || got.Complete || len(got.Issues) != 0 {
				t.Fatalf("unavailable base recovered via %v: %+v", outer, got)
			}
		}
	}
}

func TestAttributeWildcardUnionExplicitNamesStayExact(t *testing.T) {
	left := "urn:A urn:a%2F urn:non\u00a0breaking"
	right := "urn:a urn:a%2f urn:non\u00a0breaking"
	merged := MergeAnyAttribute(unionWildcard(left, "skip"), unionWildcard(right, "skip"))
	if merged.Namespace != "urn:A urn:a%2F urn:non\u00a0breaking urn:a urn:a%2f" {
		t.Fatalf("changed namespace identity: %q", merged.Namespace)
	}
	for _, ns := range []string{"urn:A", "urn:a", "urn:a%2F", "urn:a%2f", "urn:non\u00a0breaking"} {
		if !AnyAttributeAllows(merged, QName{Space: ns, Local: "a"}) {
			t.Fatalf("legacy lost %q", ns)
		}
		if got := wildcardUnionResult(t, &ComplexSchema{Form: ComplexDirect, AnyAttribute: merged}, ns); !got.Valid || !got.Complete {
			t.Fatalf("controlled lost %q: %+v", ns, got)
		}
	}
}

// Documented pre-existing limitation: empty metadata means the absent/default
// namespace attribute. The generator cannot distinguish an explicit empty XML
// attribute from omission. A nonempty whitespace spelling represents the empty
// namespace set; these are characterization tests, not generic-XSD closure.
func TestAttributeWildcardEmptyNamespaceMetadataLimitation(t *testing.T) {
	for _, namespace := range []string{"", " "} {
		for _, attributeSpace := range []string{"", "urn:other"} {
			want := namespace == ""
			for _, typ := range []*ComplexSchema{
				{Form: ComplexDirect, AnyAttribute: unionWildcard(namespace, "skip")},
				wildcardUnionType(false, unionWildcard(namespace, "skip"), unionWildcard(namespace, "skip"), false),
			} {
				got := wildcardUnionResult(t, typ, attributeSpace)
				if got.Valid != want || !got.Complete {
					t.Fatalf("namespace metadata %q, attribute space %q: %+v", namespace, attributeSpace, got)
				}
			}
		}
	}
}

func TestAttributeWildcardNamespaceTokenAdmissionConsistency(t *testing.T) {
	for _, namespace := range []string{"", " ", "##any", "##other", "##local urn:one", "##targetNamespace", "##any ##local", "urn:one ##other", "urn:%"} {
		_, kind := wildcardUnionTokens(namespace, nil)
		admission := wildcardNamespace(namespace, "urn:one", nil)
		if admission.valid != (kind != "invalid") {
			t.Fatalf("namespace %q union kind %q disagrees with admission %+v", namespace, kind, admission)
		}
	}
}
