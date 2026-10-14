package validation

import (
	"context"
	"errors"
	"testing"
)

func TestSourceEntityDeclarationsSupportedCarriers(t *testing.T) {
	for _, builtin := range []string{"ENTITY", "ENTITIES"} {
		for _, restricted := range []bool{false, true} {
			ref := entityBuiltin(builtin)
			if restricted {
				ref = SchemaTypeRef{InlineSimple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Inline: &SchemaSimple{Form: SchemaSimpleRestriction, Base: entityMember(builtin)}}}}
			}
			e := entityDeclarationEngine(t, entityDeclarationSchema(t, ref), XSDCheckScalar)
			cases := []struct {
				name, value   string
				facts         *SourceEntityDeclarations
				fail, unknown bool
			}{
				{"missing-facts", "asset", nil, false, true},
				{"complete-positive", "asset", ptrEntitySnapshot(entitySnapshot(true, entityRow("asset", true, 7))), false, false},
				{"partial-positive", "asset", ptrEntitySnapshot(entitySnapshot(false, entityRow("asset", true, 7))), false, false},
				{"partial-parsed-negative", "asset", ptrEntitySnapshot(entitySnapshot(false, entityRow("asset", false, 7))), true, false},
				{"complete-absent", "asset", ptrEntitySnapshot(entitySnapshot(true)), true, false},
				{"partial-absent", "asset", ptrEntitySnapshot(entitySnapshot(false)), false, true},
				{"invalid-lexical", "a:b", nil, true, false},
				{"empty", "", nil, true, false},
			}
			if builtin == "ENTITIES" {
				cases = append(cases, struct {
					name, value   string
					facts         *SourceEntityDeclarations
					fail, unknown bool
				}{"every-token", "asset other", ptrEntitySnapshot(entitySnapshot(true, entityRow("asset", true, 7))), true, false})
			}
			for _, tc := range cases {
				t.Run(builtin+"/"+map[bool]string{true: "restriction", false: "direct"}[restricted]+"/"+tc.name, func(t *testing.T) {
					r := entityDeclarationAssess(t, e, tc.value, tc.facts)
					if (r.Counts.Fail > 0) != tc.fail || (r.Counts.Unknown > 0) != tc.unknown || (!tc.fail && !tc.unknown && !r.Satisfied()) {
						t.Fatalf("unexpected report: %+v", r)
					}
				})
			}
		}
	}
}
func TestSourceEntityDeclarationsSchemaLiteralIsolation(t *testing.T) {
	ref := SchemaTypeRef{InlineSimple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: entityMember("ENTITY"), Enumerations: []string{"unused", "asset"}}}
	schema := entityDeclarationSchema(t, ref)
	e := entityDeclarationEngine(t, schema, XSDCheckScalar)
	positive := entitySnapshot(true, entityRow("asset", true, 1))
	if r := entityDeclarationAssess(t, e, "asset", &positive); !r.Satisfied() {
		t.Fatalf("unused enumeration operand consulted instance: %+v", r)
	}
	negative := entitySnapshot(true, entityRow("asset", false, 1), entityRow("unused", true, 2))
	if r := entityDeclarationAssess(t, e, "asset", &negative); r.Counts.Fail != 1 {
		t.Fatalf("compiled schema leaked another document domain: %+v", r)
	}
	if r := entityDeclarationAssess(t, e, "unused", &negative); !r.Satisfied() {
		t.Fatalf("second document enumeration poisoned: %+v", r)
	}
	// A facet failure is independently known even with no declaration facts.
	if r := entityDeclarationAssess(t, e, "other", nil); r.Counts.Fail != 1 {
		t.Fatalf("missing domain masked known facet failure: %+v", r)
	}
}
func TestSourceEntityDeclarationsFixedAndAppliedDefaults(t *testing.T) {
	for _, defaulted := range []bool{false, true} {
		value := "asset"
		element := &SchemaElement{Name: SchemaName{Local: "root"}, Type: entityBuiltin("ENTITY")}
		if defaulted {
			element.Default = &value
		} else {
			element.Fixed = &value
		}
		schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: element}})
		if err != nil {
			t.Fatal(err)
		}
		e := entityDeclarationEngine(t, schema, XSDCheckScalar, XSDCheckDeclaration)
		good := entitySnapshot(true, entityRow("asset", true, 1))
		input := value
		if defaulted {
			input = ""
		}
		if r := entityDeclarationAssess(t, e, input, &good); !r.Satisfied() {
			t.Fatalf("default=%v: %+v", defaulted, r)
		}
		missing := entitySnapshot(true)
		if r := entityDeclarationAssess(t, e, input, &missing); r.Counts.Fail == 0 {
			t.Fatalf("applied value escaped membership: %+v", r)
		}
	}
}
func TestSourceEntityDeclarationsUnsettledConstructedCarriers(t *testing.T) {
	list := &SchemaSimple{Form: SchemaSimpleList, Item: entityMember("ENTITY")}
	for _, tc := range []struct {
		name    string
		typ     *SchemaSimple
		value   string
		unknown bool
	}{
		{"list", list, "asset", true}, {"empty-list", list, "", true},
		{"entity-first-union", &SchemaSimple{Form: SchemaSimpleUnion, Members: []*SchemaSimpleMember{entityMember("ENTITY"), entityMember("string")}}, "asset", true},
		{"string-first-union", &SchemaSimple{Form: SchemaSimpleUnion, Members: []*SchemaSimpleMember{entityMember("string"), entityMember("ENTITY")}}, "asset", false},
		{"invalid-entity-falls-through", &SchemaSimple{Form: SchemaSimpleUnion, Members: []*SchemaSimpleMember{entityMember("ENTITY"), entityMember("string")}}, "a:b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := entityDeclarationEngine(t, entityDeclarationSchema(t, SchemaTypeRef{InlineSimple: tc.typ}), XSDCheckScalar)
			facts := entitySnapshot(true, entityRow("asset", true, 1))
			r := entityDeclarationAssess(t, e, tc.value, &facts)
			if (r.Counts.Unsupported > 0) != tc.unknown || r.Counts.Fail != 0 || (!tc.unknown && !r.Satisfied()) {
				t.Fatalf("XSD 1.0 construction boundary: %+v", r)
			}
		})
	}
}

func TestSourceEntityDeclarationsAdmissionAndOwnership(t *testing.T) {
	ctx := context.Background()
	schema := entityDeclarationSchema(t, entityBuiltin("ENTITY"))
	e := entityDeclarationEngine(t, schema, XSDCheckScalar)
	for _, tc := range []struct {
		name   string
		mutate func(*SourceEntityDeclarations)
	}{
		{"wrong-document", func(f *SourceEntityDeclarations) { f.DocumentID = "other" }},
		{"duplicate-name", func(f *SourceEntityDeclarations) {
			f.Declarations = append(f.Declarations, entityRow("asset", false, 2))
		}},
		{"duplicate-sequence", func(f *SourceEntityDeclarations) {
			f.Declarations = append(f.Declarations, entityRow("other", true, 1))
		}},
		{"zero-sequence", func(f *SourceEntityDeclarations) { f.Declarations[0].Sequence = 0 }},
		{"invalid-name", func(f *SourceEntityDeclarations) { f.Declarations[0].Name = "a:b" }},
		{"absent-completeness", func(f *SourceEntityDeclarations) { f.Complete = Evidence[bool]{State: EvidenceAbsent} }},
		{"incomplete-without-reason", func(f *SourceEntityDeclarations) { f.Complete = Evidence[bool]{State: EvidenceKnown} }},
		{"known-location-without-position", func(f *SourceEntityDeclarations) {
			f.Declarations[0].Origin = Evidence[Location]{State: EvidenceKnown, Value: Location{Target: TargetSource, DocumentID: "source"}}
		}},
		{"wrong-location-document", func(f *SourceEntityDeclarations) {
			f.Declarations[0].Origin = Evidence[Location]{State: EvidenceKnown, Value: Location{Target: TargetSource, DocumentID: "other", Position: &SourcePosition{CoordinateSpace: "original-bytes"}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := entityDeclarationSession(t, e)
			if err := s.Open(ctx, schemaPendingHeader()); err != nil {
				t.Fatal(err)
			}
			f := entitySnapshot(true, entityRow("asset", true, 1))
			tc.mutate(&f)
			err := s.SourceEntityDeclarations(ctx, f)
			if err == nil {
				t.Fatal("invalid declaration facts admitted")
			}
			_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("retained %d bytes", used)
			}
		})
	}
	s := entityDeclarationSession(t, e)
	if err := s.Open(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	f := entitySnapshot(true, entityRow("asset", false, 1))
	f.Declarations[0].Origin = Evidence[Location]{State: EvidenceKnown, Value: Location{Target: TargetSource, DocumentID: "source", Path: "/!DOCTYPE/!ENTITY[1]", Position: &SourcePosition{ByteOffset: 17, CoordinateSpace: "original-bytes"}}}
	if err := s.SourceEntityDeclarations(ctx, f); err != nil {
		t.Fatal(err)
	}
	f.Declarations[0].Unparsed = true
	f.Declarations[0].Origin.Value.Position.ByteOffset = 999
	f.Declarations[0].Origin.Value.Path = "mutated"
	if err := s.Close(ctx, schemaLexemeEnd("asset")); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Fail != 1 || len(report.Findings) != 1 || len(report.Findings[0].Related) != 1 || report.Findings[0].Related[0].Position.ByteOffset != 17 {
		t.Fatalf("snapshot/provenance not owned: %+v", report)
	}
}

func TestSourceEntityDeclarationsRootCloseAndPrefixFinalization(t *testing.T) {
	ctx := context.Background()
	schema := entityDeclarationSchema(t, entityBuiltin("ENTITY"))
	e := entityDeclarationEngine(t, schema, XSDCheckScalar)
	s := entityDeclarationSession(t, e)
	if err := s.Open(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err := s.SourceEntityDeclarations(ctx, entitySnapshot(true)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, schemaLexemeEnd("missing")); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("malformed trailing bytes")
	report, err := s.Finish(ctx, Completion{Syntax: SyntaxMalformed, Cause: cause})
	if !errors.Is(err, cause) || report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("root-close proof lost: %+v / %v", report, err)
	}
	child := &SchemaElement{Name: SchemaName{Local: "child"}, Type: entityBuiltin("ENTITY")}
	ref := SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: child}}}
	e = entityDeclarationEngine(t, entityDeclarationSchema(t, ref), XSDCheckScalar)
	s = entityDeclarationSession(t, e)
	if err := s.Open(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err := s.SourceEntityDeclarations(ctx, entitySnapshot(true)); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteAttributes(ctx, 1, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
		t.Fatal(err)
	}
	h := schemaPendingHeader()
	h.ID = 2
	h.Parent = 1
	h.Name = QName{Local: "child"}
	h.Location.Path = "/root/child"
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	end := schemaLexemeEnd("missing")
	end.ID = 2
	if err := s.Close(ctx, end); err != nil {
		t.Fatal(err)
	}
	report, err = s.Finish(ctx, Completion{Syntax: SyntaxMalformed, Cause: cause})
	if !errors.Is(err, cause) || report.Counts.Fail != 1 {
		t.Fatalf("prefix proof lost: %+v / %v", report, err)
	}
}

func TestSourceEntityDeclarationsAppliedAttributeConstraints(t *testing.T) {
	for _, kind := range []string{"default", "fixed"} {
		for _, unparsed := range []bool{false, true} {
			t.Run(kind+map[bool]string{true: "/unparsed", false: "/parsed"}[unparsed], func(t *testing.T) {
				ref := SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "asset"}, Type: entityBuiltin("ENTITY"), Constraint: &SchemaValueConstraint{Kind: kind, Value: "image"}}}}}
				e := entityDeclarationEngine(t, entityDeclarationSchema(t, ref), XSDCheckAttributes)
				facts := entitySnapshot(true, entityRow("image", unparsed, 1))
				r := entityDeclarationAssess(t, e, "", &facts)
				if unparsed {
					if !r.Satisfied() {
						t.Fatalf("applied positive: %+v", r)
					}
				} else if r.Counts.Fail != 1 || !r.AssessmentComplete || len(r.Findings) != 1 || r.Findings[0].Reason != "entity-declaration" {
					t.Fatalf("applied negative: %+v", r)
				}
			})
		}
	}
}
func TestSourceEntityDeclarationsEnumerationAndFixedOperands(t *testing.T) {
	ref := SchemaTypeRef{InlineSimple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: entityMember("ENTITIES"), Enumerations: []string{"unused", "asset other"}}}
	e := entityDeclarationEngine(t, entityDeclarationSchema(t, ref), XSDCheckScalar)
	facts := entitySnapshot(true, entityRow("asset", true, 2), entityRow("other", true, 1))
	if r := entityDeclarationAssess(t, e, "asset other", &facts); !r.Satisfied() {
		t.Fatalf("unused list literal: %+v", r)
	}
	expected := "unused"
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: entityBuiltin("ENTITIES"), Fixed: &expected}}})
	if err != nil {
		t.Fatal(err)
	}
	e = entityDeclarationEngine(t, schema, XSDCheckScalar, XSDCheckDeclaration)
	r := entityDeclarationAssess(t, e, "asset", &facts)
	if r.Counts.Fail != 1 || r.Counts.Pass != 1 || !r.AssessmentComplete || len(r.Findings) != 1 || r.Findings[0].Reason != "fixed" {
		t.Fatalf("fixed operand borrowed instance domain: %+v", r)
	}
}
func TestSourceEntityDeclarationsPermutationAndUnknownReason(t *testing.T) {
	e := entityDeclarationEngine(t, entityDeclarationSchema(t, entityBuiltin("ENTITIES")), XSDCheckScalar)
	first := entityRow("first", true, 19)
	second := entityRow("second", false, 3)
	for _, rows := range [][]SourceEntityDeclaration{{first, second}, {second, first}} {
		facts := entitySnapshot(false, rows...)
		r := entityDeclarationAssess(t, e, "missing second", &facts)
		if r.Counts.Fail != 1 || !r.AssessmentComplete {
			t.Fatalf("partial absence hid later known negative: %+v", r)
		}
	}
	for _, state := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceUnsupported} {
		facts := SourceEntityDeclarations{DocumentID: "source", Complete: Evidence[bool]{State: state, Reason: "scanner-cutoff-reason"}}
		r := entityDeclarationAssess(t, e, "missing", &facts)
		if r.Counts.Unknown != 1 || len(r.Findings) != 1 || r.Findings[0].Reason != "scanner-cutoff-reason" {
			t.Fatalf("domain uncertainty reason lost: %+v", r)
		}
	}
	for _, reason := range []Code{"", "explanatory only"} {
		facts := entitySnapshot(true, first)
		facts.Complete.Reason = reason
		if r := entityDeclarationAssess(t, e, "first", &facts); !r.Satisfied() {
			t.Fatalf("known true explanation changed domain: %+v", r)
		}
	}
}
func TestSourceEntityDeclarationsDiagnosticCapKeepsFailure(t *testing.T) {
	ctx := context.Background()
	e := entityDeclarationEngine(t, entityDeclarationSchema(t, entityBuiltin("ENTITY")), XSDCheckScalar)
	s, err := e.Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test", Budgets: Budgets{MaxDiagnosticBytes: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Open(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err = s.SourceEntityDeclarations(ctx, entitySnapshot(true)); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(ctx, schemaLexemeEnd("missing")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || !r.DiagnosticsTruncated || len(r.Findings) != 0 {
		t.Fatalf("diagnostic cap erased proof: %+v / %v", r, err)
	}
}
