package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func sourcePresenceCatalogFixture(t *testing.T, trigger, explicit bool) ([]RuleDefinition, []Profile, []PlanBinding, *CompiledSchema) {
	t.Helper()
	data := SchemaData{Types: map[SchemaName]*SchemaType{{Local: "root-type"}: {Complex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{
		{Name: SchemaName{Local: "dacapo"}, Type: localBuiltin("token")},
		{Name: SchemaName{Local: "type"}, Type: localBuiltin("token")},
		{Name: SchemaName{Local: "x2"}, Type: localBuiltin("decimal")},
	}}}}, Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: localType("root-type")}}}
	schema, err := CompileSchema(AssemblyMusicXML, data)
	if err != nil {
		t.Fatal(err)
	}
	r, p, b := entityDeclarationDefinitions(t, schema, XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckContent)
	root, _ := schema.Root(QName{Local: "root"})
	attr := localSchemaAttr(t, schema, "root-type", "dacapo")
	var plan Plan
	if explicit {
		plan, err = NewSourceAttributeValuePlan(attr, []Atomic{localString("yes")})
	} else {
		plan, err = NewPresentAttributeValuePlan(attr, []Atomic{localString("yes")})
	}
	if trigger {
		attrs := []AttributeSelector{localSchemaAttr(t, schema, "root-type", "x2")}
		target := localSchemaAttr(t, schema, "root-type", "type")
		if explicit {
			plan, err = NewSourcePresenceImpliesAttributePlan(attrs, Selector{Declaration: &root}, target, localString("continue"))
		} else {
			plan, err = NewPresenceImpliesAttributePlan(attrs, Selector{Declaration: &root}, target, localString("continue"))
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	rr, _, _ := catalogFixture(t)
	rule := rr[0]
	rule.Key = RuleKey{"test.source-presence", 1}
	rule.Predicate = "source-presence"
	rule.Selector.Declaration = &root
	r = append(r, rule)
	p[0].Required = append(p[0].Required, rule.Key)
	p[0].Support = append(p[0].Support, RuleSupport{Rule: rule.Key, State: ImplementationAvailable})
	b = append(b, PlanBinding{ID: rule.Predicate, Plan: plan})
	return r, p, b, schema
}

func TestSourceLocalPresenceCatalogCompatibilityBoundary(t *testing.T) {
	for _, trigger := range []bool{false, true} {
		for _, mode := range []string{"legacy-rejected", "explicit-accepted", "legacy-xsd-accepted", "local-unsupported", "local-unselected", "xsd-unsupported", "other-assembly"} {
			t.Run(mode+map[bool]string{false: "-value", true: "-trigger"}[trigger], func(t *testing.T) {
				r, p, b, schema := sourcePresenceCatalogFixture(t, trigger, mode == "explicit-accepted")
				last := len(r) - 1
				switch mode {
				case "legacy-xsd-accepted":
					for i := 0; i < len(b)-1; i++ {
						var err error
						b[i].Plan, err = NewXSDPlan(schema, b[i].Plan.xsdCheck)
						if err != nil {
							t.Fatal(err)
						}
					}
				case "local-unsupported":
					p[0].Support[len(p[0].Support)-1].State = ImplementationUnsupported
					p[0].Support[len(p[0].Support)-1].Reason = "reviewed-unavailable"
				case "local-unselected":
					p[0].Required = p[0].Required[:len(p[0].Required)-1]
					p[0].Support = p[0].Support[:len(p[0].Support)-1]
				case "xsd-unsupported":
					for i := 0; i < len(p[0].Support)-1; i++ {
						p[0].Support[i].State = ImplementationUnsupported
						p[0].Support[i].Reason = "reviewed-unavailable"
					}
				case "other-assembly":
					r[last].Selector.Declaration.Assembly = AssemblyOpus
				}
				_, err := NewCatalog(r, p, b)
				if mode == "legacy-rejected" {
					replacement := "NewSourceAttributeValuePlan"
					if trigger {
						replacement = "NewSourcePresenceImpliesAttributePlan"
					}
					if !errors.Is(err, ErrInvalidConfiguration) || !strings.Contains(err.Error(), replacement) || !strings.Contains(err.Error(), string(r[last].Key.ID)) {
						t.Fatalf("legacy physical plan was not refused with actionable replacement: %v", err)
					}
				} else if err != nil {
					t.Fatalf("compatible %s rejected: %v", mode, err)
				}
			})
		}
	}
}

func TestSourceLocalPresenceExplicitCompositionExecutes(t *testing.T) {
	for _, trigger := range []bool{false, true} {
		r, p, b, _ := sourcePresenceCatalogFixture(t, trigger, true)
		c, err := NewCatalog(r, p, b)
		if err != nil {
			t.Fatal(err)
		}
		e, err := NewEngine(c)
		if err != nil {
			t.Fatal(err)
		}
		s, err := e.Begin(context.Background(), RunOptions{DocumentID: "source", Profile: p[0].ID})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Open(context.Background(), schemaPendingHeader()); err != nil {
			t.Fatal(err)
		}
		if err = s.SourceEntityDeclarations(context.Background(), entitySnapshot(true)); err != nil {
			t.Fatal(err)
		}
		attrs := []struct{ name, value string }{{"dacapo", "no"}}
		if trigger {
			attrs = []struct{ name, value string }{{"x2", "1"}, {"type", "start"}}
		}
		for _, v := range attrs {
			a := sourceOriginAttribute(QName{Local: v.name}, schemaPendingHeader().Location)
			a.Scalar.Lexeme = stateKnown(v.value)
			if err = s.Attribute(context.Background(), a); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.Close(context.Background(), schemaLexemeEnd("")); err != nil {
			t.Fatal(err)
		}
		got, ok := s.lookupDecision(decisionID{rule: r[len(r)-1].Key, node: 1})
		if !ok || got.Outcome != OutcomeFail {
			t.Fatalf("effective source presence did not execute: %+v/%v", got, ok)
		}
		_, err = s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if err != nil {
			t.Fatal(err)
		}
	}
}
