package validation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestSourceContainerMetadataSeparatesPhysicalRaw(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		specified, origin, lexical bool
		state                      EvidenceState
	}{
		{"physical-tokenized", true, true, true, EvidenceKnown},
		{"physical-unretained-entity-value", true, true, false, EvidenceUnknown},
		{"defaulted", false, true, false, EvidenceAbsent},
		{"entity-generated", false, true, false, EvidenceAbsent},
		{"origin-unavailable", true, false, false, EvidenceUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := containerTestFixture(t)
			defer s.cleanup()
			p.sourceInfoset = true
			n := s.nodes[3]
			slot := sourceOriginSubject{Node: n.header.ID, Attribute: n.attributes[0].Name}
			if tc.origin {
				s.sourceOriginSeen = map[sourceOriginSubject]*SourceOriginContextResult{slot: {Specified: stateKnown(tc.specified)}}
			}
			if tc.lexical {
				s.sourceAttributeLexical = map[sourceOriginSubject]Evidence[string]{slot: stateKnown("  physical\t  score.musicxml \n")}
			}
			if _, _, err := evaluateContextContainerMetadata(context.Background(), s, nil, RuleKey{"source-container", 1}, p, true); err != nil {
				t.Fatal(err)
			}
			got := s.contextResults[2].Container
			if got.FullPath.State != EvidenceKnown || got.FullPath.Value.Text != "score.musicxml" || got.FullPathRaw.State != tc.state || got.Ordinal.State != EvidenceKnown || got.Ordinal.Value != 1 {
				t.Fatalf("effective path, physical raw or order changed: %+v", got)
			}
			if tc.lexical && got.FullPathRaw.Value != "  physical\t  score.musicxml \n" {
				t.Fatalf("physical raw reconstructed from effective spelling: %+v", got.FullPathRaw)
			}
			for _, index := range []int{0, 1} {
				if e := s.contextResults[index].Container.Content; e.State != EvidenceKnown || !e.Value {
					t.Fatalf("ancestor inventory prerequisite changed: %+v", e)
				}
			}
		})
	}
}

func auxiliaryHintSchema(t *testing.T, assembly Assembly) *CompiledSchema {
	t.Helper()
	schema, err := CompileSchema(assembly, SchemaData{Elements: map[SchemaName]*SchemaElement{
		{Local: string(assembly)}: {Name: SchemaName{Local: string(assembly)}, Type: SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestSourceAuxiliaryHintsAdditivePlanContract(t *testing.T) {
	for _, assembly := range []Assembly{AssemblyContainer, AssemblySounds} {
		t.Run(string(assembly), func(t *testing.T) {
			schema := auxiliaryHintSchema(t, assembly)
			legacy, err := NewSchemaHintContextPlan(schema)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceSchemaHintContextPlan(schema)
			if err != nil {
				t.Fatal(err)
			}
			old, _ := json.Marshal(legacy.context.digest(legacy.kind))
			current, _ := json.Marshal(source.context.digest(source.kind))
			if string(old) == string(current) {
				t.Fatal("source mode omitted from digest")
			}
			copy := source.clone()
			got, _ := json.Marshal(copy.context.digest(copy.kind))
			if string(got) != string(current) || !copy.context.sourceInfoset {
				t.Fatal("clone lost source mode")
			}
			rule := RuleDefinition{Targets: []Target{TargetModel}, Category: CategoryNormative, Class: ClassMandatory, Role: RoleContextProvider, Scope: ScopeDocument, Selector: Selector{Assembly: &assembly}}
			if err := source.validateFor(rule); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("model target accepted: %v", err)
			}
		})
	}
}

func TestSourceSoundsMappingRemainsEffective(t *testing.T) {
	for _, kind := range []SourceOriginKind{SourceOriginDefault, SourceOriginEntity} {
		t.Run(string(kind), func(t *testing.T) {
			s, p := soundsTestFixture(t)
			defer s.cleanup()
			s.sourceOriginSeen = map[sourceOriginSubject]*SourceOriginContextResult{}
			for _, n := range s.nodes {
				for _, a := range n.attributes {
					s.sourceOriginSeen[sourceOriginSubject{Node: n.header.ID, Attribute: a.Name}] = &SourceOriginContextResult{Kind: kind, Specified: stateKnown(false)}
				}
			}
			if _, _, err := evaluateContextSoundsMapping(context.Background(), s, nil, RuleKey{"sounds-effective", 1}, p, true); err != nil {
				t.Fatal(err)
			}
			root, child := s.contextResults[0].SoundMapping, s.contextResults[1].SoundMapping
			if root.SoundID.Value.Text != "example" || root.Primary.Value.Text != "yes" || child.Primary.Value.Text != "no" || child.Number.Value.Text != "many:players" || child.LibraryID.Value.Text != " library  value " || root.Direction != SoundMappingToMusicXML || child.Direction != SoundMappingFromMusicXML {
				t.Fatalf("synthetic attributes lost effective mapping values: %+v %+v", root, child)
			}
		})
	}
}

func auxiliaryHintCatalog(t *testing.T, assembly Assembly, mode string) (*CompiledCatalog, RuleKey, error) {
	t.Helper()
	schema := auxiliaryHintSchema(t, assembly)
	profile := Profile{ID: "auxiliary-hint-test", Target: TargetSource, MusicXMLVersion: "4.0", RegistryCommit: "test", SupportedRoots: []QName{{Local: string(assembly)}}, NormativeClosure: "Test hints alone elect exact source lexical ingress."}
	var rules []RuleDefinition
	var plans []PlanBinding
	add := func(key RuleKey, plan Plan, a Assembly, role RuleRole, scope ScopeKind, selected bool, support ImplementationState) {
		rule := RuleDefinition{Key: key, Targets: []Target{TargetSource}, Scope: scope, Category: CategoryNormative, Class: ClassMandatory, Role: role, Selector: Selector{Assembly: &a}, Predicate: string(key.ID), Text: RuleText{Title: "Auxiliary hints", Explanation: "Test exact hints-only source lexical admission.", Message: "Hint evidence unavailable."}, Sources: []SourceRef{{IDs: []string{"XML-1.0"}, URL: "https://www.w3.org/TR/xml/", Locator: "3.3.2"}}}
		rules = append(rules, rule)
		plans = append(plans, PlanBinding{ID: rule.Predicate, Plan: plan})
		if selected {
			profile.Required = append(profile.Required, key)
			item := RuleSupport{Rule: key, State: support}
			if support != ImplementationAvailable {
				item.Reason = "test-unavailable"
			}
			profile.Support = append(profile.Support, item)
		}
	}
	for _, check := range []XSDCheck{XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar} {
		p, err := NewXSDPlan(schema, check)
		if err != nil {
			t.Fatal(err)
		}
		add(RuleKey{RuleID("test." + string(check)), 1}, p, assembly, RolePredicate, ScopeNode, true, ImplementationAvailable)
	}
	originKey := RuleKey{"test.auxiliary-origin", 1}
	originAssembly := assembly
	if mode == "wrong-origin" {
		originAssembly = AssemblyContainer
		if originAssembly == assembly {
			originAssembly = AssemblySounds
		}
		p, err := NewXSDPlan(auxiliaryHintSchema(t, originAssembly), XSDCheckScalar)
		if err != nil {
			t.Fatal(err)
		}
		add(RuleKey{"test.other-scalar", 1}, p, originAssembly, RolePredicate, ScopeNode, true, ImplementationAvailable)
	}
	origin, err := NewSourceOriginPlan(originAssembly)
	if err != nil {
		t.Fatal(err)
	}
	originSupport := ImplementationAvailable
	if mode == "unavailable-origin" {
		originSupport = ImplementationUnsupported
	}
	add(originKey, origin, originAssembly, RoleContextProvider, ScopeDocument, mode != "missing-origin", originSupport)
	hint, err := NewSourceSchemaHintContextPlan(schema)
	if mode == "legacy" {
		hint, err = NewSchemaHintContextPlan(schema)
	}
	if err != nil {
		t.Fatal(err)
	}
	hintSupport := ImplementationAvailable
	if mode == "unavailable-hint" {
		hintSupport = ImplementationUnsupported
	}
	add(RuleKey{"test.auxiliary-hints", 1}, hint, assembly, RoleContextProvider, ScopeDocument, mode != "unselected-hint", hintSupport)
	catalog, err := NewCatalog(rules, []Profile{profile}, plans)
	return catalog, originKey, err
}

func TestSourceAuxiliaryHintOnlyLexicalElection(t *testing.T) {
	for _, assembly := range []Assembly{AssemblyContainer, AssemblySounds} {
		for _, mode := range []string{"selected", "legacy", "unselected-hint", "unavailable-hint", "missing-origin", "wrong-origin", "unavailable-origin"} {
			t.Run(string(assembly)+"/"+mode, func(t *testing.T) {
				catalog, key, err := auxiliaryHintCatalog(t, assembly, mode)
				if mode == "missing-origin" || mode == "wrong-origin" || mode == "unavailable-origin" {
					var config *ConfigurationError
					if !errors.As(err, &config) || config.Field != "source resource profile" {
						t.Fatalf("unsealed origin accepted or wrong rejection: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				engine, err := NewEngine(catalog)
				if err != nil {
					t.Fatal(err)
				}
				got, ok := engine.SourceAttributeLexicalProvider("auxiliary-hint-test", assembly)
				if mode == "selected" {
					if !ok || got != key {
						t.Fatalf("hints alone did not elect exact provider: %+v %v", got, ok)
					}
				} else {
					if ok || got != (RuleKey{}) {
						t.Fatalf("unelected lexical input enabled: %+v", got)
					}
					s, err := engine.Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "auxiliary-hint-test"})
					if err != nil {
						t.Fatal(err)
					}
					defer s.cleanup()
					if err := s.SourceAttributeLexical(context.Background(), key, NodeRef{"source", 1}, QName{Local: "id"}, stateKnown("raw")); !errors.Is(err, ErrInvalidConfiguration) {
						t.Fatalf("unelected ingress accepted: %v", err)
					}
				}
			})
		}
	}
}

func TestSourceAuxiliaryHintOnlyEffectiveAndPhysicalValues(t *testing.T) {
	for _, assembly := range []Assembly{AssemblyContainer, AssemblySounds} {
		for _, kind := range []SourceOriginKind{SourceOriginPhysical, SourceOriginDefault, SourceOriginEntity} {
			t.Run(string(assembly)+"/"+string(kind), func(t *testing.T) {
				catalog, key, err := auxiliaryHintCatalog(t, assembly, "selected")
				if err != nil {
					t.Fatal(err)
				}
				engine, err := NewEngine(catalog)
				if err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				s, err := engine.Begin(ctx, RunOptions{DocumentID: "source", Profile: "auxiliary-hint-test"})
				if err != nil {
					t.Fatal(err)
				}
				defer s.cleanup()
				h := schemaPendingHeader()
				h.Name.Local = string(assembly)
				h.Location = sourceOriginPhysical().Physical.Value
				if err := s.Open(ctx, h); err != nil {
					t.Fatal(err)
				}
				if err := s.SourceOrigin(ctx, key, sourceOriginPhysical()); err != nil {
					t.Fatal(err)
				}
				physical := kind == SourceOriginPhysical
				for _, attribute := range []struct{ name, value string }{{"schemaLocation", "urn:a\ta.xsd trailing"}, {"noNamespaceSchemaLocation", "  hints.xsd \n"}} {
					name := QName{Space: "http://www.w3.org/2001/XMLSchema-instance", Local: attribute.name}
					o := sourceResourceAdmit(t, s, key, kind, name, attribute.value, physical)
					raw := stateAbsent[string]()
					if physical {
						raw = stateKnown("  " + attribute.value + "  ")
					}
					if err := s.SourceAttributeLexical(ctx, key, o.Subject, name, raw); err != nil {
						t.Fatal(err)
					}
				}
				if err := s.Close(ctx, schemaLexemeEnd("value")); err != nil {
					t.Fatal(err)
				}
				if err := s.CompleteSourceOrigins(ctx, key, stateKnown(true)); err != nil {
					t.Fatal(err)
				}
				report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
				if err != nil || !report.Satisfied() {
					t.Fatalf("odd hint pair became datatype violation: %v %+v", err, report.Findings)
				}
				var hints *ResourceSchemaHintsContext
				for _, row := range report.Context {
					if row.Resource != nil && row.Resource.SchemaHints != nil {
						hints = row.Resource.SchemaHints
					}
				}
				if hints == nil || len(hints.Pairs) != 1 || hints.Pairs[0].Namespace.Value != "urn:a" || hints.Pairs[0].Location.Value != "a.xsd" || hints.Trailing.Value != "trailing" || hints.Complete.State != EvidenceKnown || hints.Complete.Value || hints.NoNamespace.Value.State != EvidenceKnown || hints.NoNamespace.Value.Value.Text != "hints.xsd" {
					t.Fatalf("effective hints lost: %+v", hints)
				}
				state := EvidenceAbsent
				if physical {
					state = EvidenceKnown
				}
				if hints.Raw.State != state || hints.NoNamespace.Raw.State != state || hints.NoNamespace.Presence.State != state {
					t.Fatalf("physical hints fabricated: %+v", hints)
				}
				if physical && hints.Raw.Value != "  urn:a\ta.xsd trailing  " {
					t.Fatalf("XML whitespace lost: %+v", hints.Raw)
				}
				if used := rationalUsage(t, s, BudgetBufferedBytes); used != 0 {
					t.Fatalf("retained hints buffers: %d", used)
				}
			})
		}
	}
}

func TestSourceContainerRawExactAttributeAndCausalEvidence(t *testing.T) {
	for _, mode := range []string{"wrong-declaration", "empty", "absent", "invalid-raw"} {
		t.Run(mode, func(t *testing.T) {
			s, p := containerTestFixture(t)
			defer s.cleanup()
			p.sourceInfoset = true
			n := s.nodes[3]
			slot := sourceOriginSubject{Node: n.header.ID, Attribute: n.attributes[1].Name}
			s.sourceOriginSeen = map[sourceOriginSubject]*SourceOriginContextResult{slot: {Specified: stateKnown(true)}}
			s.sourceAttributeLexical = map[sourceOriginSubject]Evidence[string]{slot: stateKnown("physical media-type")}
			cause := &DecisionRef{Rule: RuleKey{"raw-cause", 1}, Subject: NodeRef{s.documentID, n.header.ID}}
			switch mode {
			case "wrong-declaration":
				n.attributes[1].Declaration.Value.Occurrence = "different-use"
			case "absent":
				n.attributes = n.attributes[:1]
			case "empty":
				n.attributes[1].Scalar.Value = localKnownText("")
				s.sourceAttributeLexical[slot] = stateKnown("")
			case "invalid-raw":
				s.sourceAttributeLexical[slot] = Evidence[string]{State: EvidenceInvalid, Reason: "raw-gap", Cause: cause}
			}
			if _, _, err := evaluateContextContainerMetadata(context.Background(), s, nil, RuleKey{"container-source", 1}, p, true); err != nil {
				t.Fatal(err)
			}
			got := s.contextResults[2].Container
			switch mode {
			case "wrong-declaration", "absent":
				if got.MediaType.State != EvidenceAbsent || got.MediaTypeRaw.State != EvidenceAbsent {
					t.Fatalf("unqualified slot borrowed raw: %+v", got)
				}
			case "empty":
				if got.MediaType.State != EvidenceKnown || got.MediaType.Value.Text != "" || got.MediaTypeRaw.State != EvidenceKnown || got.MediaTypeRaw.Value != "" {
					t.Fatalf("empty conflated with omission: %+v", got)
				}
			case "invalid-raw":
				if got.MediaType.State != EvidenceKnown || got.MediaTypeRaw.State != EvidenceInvalid || got.MediaTypeRaw.Cause == nil || *got.MediaTypeRaw.Cause != *cause {
					t.Fatalf("independent causal raw lost: %+v", got)
				}
				cause.Rule.ID = "caller"
				if got.MediaTypeRaw.Cause.Rule.ID != "raw-cause" {
					t.Fatal("raw cause aliases source evidence")
				}
			}
		})
	}
}

func TestSourceContainerMetadataCancellationAndWorkAdmission(t *testing.T) {
	fixture := func() (*Session, *contextPlan) {
		s, p := containerTestFixture(t)
		p.sourceInfoset = true
		s.sourceOriginSeen = map[sourceOriginSubject]*SourceOriginContextResult{}
		s.sourceAttributeLexical = map[sourceOriginSubject]Evidence[string]{}
		for _, n := range s.nodes {
			for _, a := range n.attributes {
				slot := sourceOriginSubject{Node: n.header.ID, Attribute: a.Name}
				s.sourceOriginSeen[slot] = &SourceOriginContextResult{Specified: stateKnown(true)}
				s.sourceAttributeLexical[slot] = stateKnown("physical spelling")
			}
		}
		return s, p
	}
	probe, p := fixture()
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if _, _, err := evaluateContextContainerMetadata(counter, probe, nil, RuleKey{"source-container", 1}, p, true); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, probe, BudgetRuleSteps)
	probe.cleanup()
	for at := 1; at <= counter.calls; at++ {
		s, p := fixture()
		_, _, err := evaluateContextContainerMetadata(&rationalCancelContext{Context: context.Background(), at: at}, s, nil, RuleKey{"source-container", 1}, p, true)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation %d/%d lost: %v", at, counter.calls, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("canceled source metadata leaked buffers")
		}
	}
	for _, delta := range []int64{-1, 0} {
		s, p := fixture()
		s.account, _ = NewBudgetAccount(Budgets{MaxRuleSteps: work + delta})
		_, _, err := evaluateContextContainerMetadata(context.Background(), s, nil, RuleKey{"source-container", 1}, p, true)
		if delta == 0 && err != nil || delta < 0 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("work boundary %d: %v", delta, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("source metadata work boundary leaked buffers")
		}
	}
	t.Logf("checked %d source metadata cancellation polls and exact %d work boundary", counter.calls, work)
}
