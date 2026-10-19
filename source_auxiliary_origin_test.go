package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func auxiliaryOriginTestEngine(t *testing.T, assembly validation.Assembly, projections ...validation.Plan) (*validation.Engine, validation.RuleKey) {
	t.Helper()
	data := containerValidationSchema
	if assembly == validation.AssemblySounds {
		data = soundsValidationSchema
	}
	schema, err := compileValidationSchema(assembly, data)
	if err != nil {
		t.Fatal(err)
	}
	xsd, err := validation.NewXSDPlan(schema, validation.XSDCheckScalar)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := validationNewSourceOriginPlan(assembly)
	if err != nil {
		t.Fatal(err)
	}
	profile := validation.Profile{ID: "auxiliary-origin-test", Target: validation.TargetSource, MusicXMLVersion: "4.0", RegistryCommit: "test", SupportedRoots: []validation.QName{{Local: string(assembly)}}, NormativeClosure: "Test exact auxiliary source-origin root routing."}
	var rules []validation.RuleDefinition
	var plans []validation.PlanBinding
	key := validation.RuleKey{ID: "test.auxiliary-origin", Revision: 1}
	for i, plan := range append([]validation.Plan{xsd, origin}, projections...) {
		k, role, scope := validation.RuleKey{ID: "test.auxiliary-scalar", Revision: 1}, validation.RolePredicate, validation.ScopeNode
		if i == 1 {
			k, role, scope = key, validation.RoleContextProvider, validation.ScopeDocument
		} else if i > 1 {
			k, role, scope = validation.RuleKey{ID: "test.auxiliary-metadata", Revision: 1}, validation.RoleContextProvider, validation.ScopeDocument
		}
		rule := validation.RuleDefinition{Key: k, Targets: []validation.Target{validation.TargetSource}, Scope: scope, Category: validation.CategoryNormative, Class: validation.ClassMandatory, Role: role, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(k.ID), Text: validation.RuleText{Title: "Auxiliary origin", Explanation: "Test exact source assembly routing.", Message: "Source evidence unavailable."}, Sources: []validation.SourceRef{{IDs: []string{"XML-1.0"}, URL: "https://www.w3.org/TR/xml/", Locator: "3.3.2"}}}
		rules = append(rules, rule)
		plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
		profile.Required = append(profile.Required, k)
		profile.Support = append(profile.Support, validation.RuleSupport{Rule: k, State: validation.ImplementationAvailable})
	}
	catalog, err := validation.NewCatalog(rules, []validation.Profile{profile}, plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := validation.NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine, key
}

func TestSourceAuxiliaryOriginExactRootRouting(t *testing.T) {
	for _, assembly := range []validation.Assembly{validation.AssemblyContainer, validation.AssemblySounds} {
		for _, foreign := range []bool{false, true} {
			name := string(assembly)
			if foreign {
				name += "/foreign"
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				engine, key := auxiliaryOriginTestEngine(t, assembly)
				account := dtdStoreAccount(t, validation.Budgets{})
				session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: "source", Profile: "auxiliary-origin-test", Account: account})
				if err != nil {
					t.Fatal(err)
				}
				observer, err := newSourceFactObserver(ctx, session, account)
				if err != nil {
					t.Fatal(err)
				}
				observer.origins = &sourceOriginObserver{engine: engine, session: session, profile: "auxiliary-origin-test"}
				input := "<" + string(assembly)
				if foreign {
					input += ` xmlns="urn:foreign"`
				}
				input += "/>"
				reader, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: ctx, Account: account, InternalDTD: true})
				if err != nil {
					t.Fatal(err)
				}
				decoder := xml.NewTokenDecoder(reader)
				var readErr error
				for {
					token, err := decoder.Token()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						readErr = err
						break
					}
					if err := observer.observeSource(token, reader.Metadata(), reader); err != nil {
						readErr = err
						break
					}
				}
				if readErr == nil {
					readErr = observer.sealSourceOrigins(ctx)
				}
				report, finishErr := session.Finish(ctx, validation.Completion{InputComplete: readErr == nil, Syntax: validation.SyntaxWellFormed, Cause: readErr})
				selected := observer.origins.key
				reader.Close()
				observer.close()
				if foreign {
					if readErr == nil || selected != (validation.RuleKey{}) || len(report.Context) != 0 {
						t.Fatalf("foreign root borrowed auxiliary origin provider: %v %+v", readErr, report.Context)
					}
				} else {
					if readErr != nil || finishErr != nil || selected != key || len(report.Context) != 1 || report.Context[0].SourceOrigin == nil || report.Context[0].SourceOrigin.Kind != validation.SourceOriginPhysical {
						t.Fatalf("auxiliary origin route: read=%v finish=%v provider=%+v context=%+v", readErr, finishErr, selected, report.Context)
					}
				}
				if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
					t.Fatalf("retained source buffers: %d", used)
				}
			})
		}
	}
}

func TestSourceContainerMetadataConstructorContract(t *testing.T) {
	if _, err := validationNewSourceContainerMetadataContextPlan(nil); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("nil schema accepted: %v", err)
	}
	for _, input := range []struct {
		assembly validation.Assembly
		data     validationSchemaSet
	}{
		{validation.AssemblyMusicXML, scoreValidationSchema},
		{validation.AssemblyOpus, opusValidationSchema},
		{validation.AssemblySounds, soundsValidationSchema},
		{validation.AssemblyContainer, containerValidationSchema},
	} {
		schema, err := compileValidationSchema(input.assembly, input.data)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := validationNewSourceContainerMetadataContextPlan(schema)
		if input.assembly != validation.AssemblyContainer {
			if !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("wrong assembly %s accepted: %v", input.assembly, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		legacy, err := validationNewContainerMetadataContextPlan(schema)
		if err != nil {
			t.Fatal(err)
		}
		oldEngine, _ := auxiliaryOriginTestEngine(t, input.assembly, legacy)
		sourceEngine, originKey := auxiliaryOriginTestEngine(t, input.assembly, plan)
		oldProfile, _ := oldEngine.Profile("auxiliary-origin-test")
		sourceProfile, _ := sourceEngine.Profile("auxiliary-origin-test")
		if oldProfile.CatalogDigest == sourceProfile.CatalogDigest {
			t.Fatal("source Container mode omitted from catalog digest")
		}
		if key, ok := validationEngineSourceAttributeLexicalProvider(sourceEngine, sourceProfile.ID, input.assembly); !ok || key != originKey {
			t.Fatal("source metadata alone did not elect lexical ingress")
		}
		if _, ok := validationEngineSourceAttributeLexicalProvider(oldEngine, oldProfile.ID, input.assembly); ok {
			t.Fatal("legacy metadata elected lexical ingress")
		}
		assembly := validation.AssemblyContainer
		key := validation.RuleKey{ID: "test-source-container", Revision: 1}
		rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetModel}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Source container", Explanation: "Test source-only container contract.", Message: "Source evidence unavailable."}, Sources: []validation.SourceRef{{IDs: []string{"XML-1.0"}, URL: "https://www.w3.org/TR/xml/", Locator: "3.3.2"}}}
		profile := validation.Profile{ID: "source-container-model-invalid", Target: validation.TargetModel, MusicXMLVersion: "4.0", RegistryCommit: "test", SupportedRoots: []validation.QName{{Local: "container"}}, NormativeClosure: "Test source-only container target.", Required: []validation.RuleKey{key}, Support: []validation.RuleSupport{{Rule: key, State: validation.ImplementationAvailable}}}
		_, err = validation.NewCatalog([]validation.RuleDefinition{rule}, []validation.Profile{profile}, []validation.PlanBinding{{ID: rule.Predicate, Plan: plan}})
		var config *validation.ConfigurationError
		if !errors.As(err, &config) || config.Field != "source resource target" {
			t.Fatalf("source container accepted model target: %v", err)
		}
	}
}
