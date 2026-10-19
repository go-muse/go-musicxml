package musicxml

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestAuxiliaryContainerMetadataPublished(t *testing.T) {
	r, err := InspectXML(context.Background(), strings.NewReader(`<container><rootfiles><rootfile full-path="score.musicxml"/><rootfile full-path="score.pdf" media-type="application/pdf"/></rootfiles></container>`), AssessmentOptions{Profile: ProfileSourceContainer})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Satisfied() {
		t.Fatalf("valid container failed: %+v", r)
	}
	var count int
	for _, row := range r.Context {
		if row.Property == validation.ContextProperty("container.metadata") {
			count++
		}
	}
	if count != 4 {
		t.Fatalf("same-source container/rootfiles/rootfile metadata count = %d, want 4", count)
	}
}

func inspectContainerMetadata(t *testing.T, source string) (validation.Report, []validation.ContextResult) {
	t.Helper()
	r, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContainer})
	if err != nil {
		t.Fatal(err)
	}
	var rows []validation.ContextResult
	for _, row := range r.Context {
		if row.Property == validation.ContextPropertyContainerMetadata {
			if row.Container == nil {
				t.Fatal("missing typed container payload")
			}
			rows = append(rows, row)
		}
	}
	return r, rows
}

func TestAuxiliaryContainerMetadataTokensOrderAndIdentities(t *testing.T) {
	r, rows := inspectContainerMetadata(t, `<container><rootfiles><rootfile full-path="  dir/score&#x9;name.musicxml &#xA;" media-type=" application/vnd.recordare.musicxml+xml  "/><rootfile full-path="" media-type=""/><rootfile full-path="score.pdf"/></rootfiles></container>`)
	if !r.Satisfied() || len(rows) != 5 {
		t.Fatalf("container projection: %+v", r)
	}
	root, files := rows[0].Subject, rows[1].Subject
	if rows[0].Container.Role != validation.ContainerRoot || rows[0].Container.Parent != (validation.NodeRef{}) || rows[1].Container.Role != validation.ContainerRootfiles || rows[1].Container.Parent != root {
		t.Fatal("wrong exact root identities")
	}
	for i, row := range rows {
		v := row.Container
		if v.Container != root || v.Content.State != validation.EvidenceKnown || !v.Content.Value {
			t.Errorf("row %d content/identity: %+v", i, v)
		}
		if len(row.Origins) != 1 || row.Origins[0].Position == nil || row.Origins[0].DocumentID != row.Subject.DocumentID {
			t.Errorf("row %d lost source origin", i)
		}
		if i < 2 {
			if v.Ordinal.State != validation.EvidenceAbsent || v.FullPath.State != validation.EvidenceAbsent || v.FullPathRaw.State != validation.EvidenceAbsent {
				t.Errorf("non-rootfile invented fields: %+v", v)
			}
		} else if v.Role != validation.ContainerRootfile || v.Parent != files || v.Ordinal.State != validation.EvidenceKnown || v.Ordinal.Value != uint64(i-1) {
			t.Errorf("rootfile %d order/parent: %+v", i, v)
		}
	}
	first := rows[2].Container
	if first.FullPathRaw.State != validation.EvidenceKnown || first.FullPathRaw.Value != "  dir/score\tname.musicxml \n" || first.FullPath.State != validation.EvidenceKnown || first.FullPath.Value.Kind != validation.AtomicString || first.FullPath.Value.Text != "dir/score name.musicxml" || first.MediaTypeRaw.Value != " application/vnd.recordare.musicxml+xml  " || first.MediaType.Value.Text != "application/vnd.recordare.musicxml+xml" {
		t.Fatalf("raw/effective token semantics: %+v", first)
	}
	empty, absent := rows[3].Container, rows[4].Container
	if empty.FullPath.State != validation.EvidenceKnown || empty.FullPath.Value.Text != "" || empty.MediaType.State != validation.EvidenceKnown || empty.MediaType.Value.Text != "" || empty.MediaTypeRaw.State != validation.EvidenceKnown || absent.MediaType.State != validation.EvidenceAbsent || absent.MediaTypeRaw.State != validation.EvidenceAbsent {
		t.Fatalf("empty and omission conflated: %+v %+v", empty, absent)
	}
}

func TestAuxiliaryContainerMetadataMissingRequiredPath(t *testing.T) {
	r, rows := inspectContainerMetadata(t, `<container><rootfiles><rootfile media-type="application/pdf"/></rootfiles></container>`)
	if r.Conformance != validation.ConformanceViolated || len(rows) != 3 {
		t.Fatalf("required path: %+v", r)
	}
	v := rows[2].Container
	if v.FullPath.State != validation.EvidenceInvalid || v.FullPath.Cause == nil || v.FullPathRaw.State != validation.EvidenceAbsent || v.MediaType.State != validation.EvidenceKnown || v.MediaType.Value.Text != "application/pdf" || v.Ordinal.State != validation.EvidenceKnown || v.Ordinal.Value != 1 {
		t.Fatalf("missing required prerequisite vs independent data: %+v", v)
	}
	if v.FullPath.Cause.Subject != rows[2].Subject {
		t.Fatalf("wrong prerequisite subject: %+v", v.FullPath.Cause)
	}
}

func TestAuxiliaryContainerMetadataMalformedInventory(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		role         validation.ContainerContextRole
	}{
		{"missing rootfiles", `<container/>`, validation.ContainerRoot},
		{"foreign rootfiles", `<container><rootfiles xmlns="urn:foreign"><rootfile full-path="hidden"/></rootfiles></container>`, validation.ContainerRoot},
		{"multiple rootfiles", `<container><rootfiles><rootfile full-path="a"/></rootfiles><rootfiles><rootfile full-path="b"/></rootfiles></container>`, validation.ContainerRoot},
		{"empty rootfiles", `<container><rootfiles/></container>`, validation.ContainerRootfiles},
		{"unknown leading child", `<container><rootfiles><unknown/><rootfile full-path="a"/></rootfiles></container>`, validation.ContainerRootfiles},
		{"foreign rootfile", `<container><rootfiles><rootfile xmlns="urn:foreign" full-path="hidden"/><rootfile full-path="a"/></rootfiles></container>`, validation.ContainerRootfiles},
		{"text in rootfiles", `<container><rootfiles>bad<rootfile full-path="a"/></rootfiles></container>`, validation.ContainerRootfiles},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, rows := inspectContainerMetadata(t, tc.source)
			if r.Conformance != validation.ConformanceViolated {
				t.Fatalf("grammar failure lost: %+v", r)
			}
			found := false
			for _, row := range rows {
				v := row.Container
				if v.Role == tc.role {
					found = true
					if v.Content.State != validation.EvidenceInvalid || v.Content.Cause == nil {
						t.Errorf("broken inventory claimed complete or lost cause: %+v", v)
					}
				}
				if v.Role == validation.ContainerRootfile && tc.role == validation.ContainerRootfiles && v.Ordinal.State == validation.EvidenceKnown {
					t.Errorf("subset fabricated ordinal: %+v", v)
				}
				if v.FullPath.State == validation.EvidenceKnown && v.FullPath.Value.Text == "hidden" {
					t.Error("foreign homonym was exported")
				}
			}
			if !found {
				t.Fatalf("missing inventory guard %s: %+v", tc.role, rows)
			}
		})
	}
}

func TestAuxiliaryContainerMetadataOwnedReportAndSourceOnly(t *testing.T) {
	const source = `<container><rootfiles><rootfile/><rootfile full-path="b"/></rootfiles></container>`
	before, _ := inspectContainerMetadata(t, source)
	_, rows := inspectContainerMetadata(t, source)
	rows[0].Container.Container.DocumentID = "caller"
	rows[2].Container.FullPath.Cause.Rule.ID = "caller"
	rows[2].Origins[0].Position.Line = 999
	after, _ := inspectContainerMetadata(t, source)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("mutating output altered a subsequent report")
	}
	for _, profile := range []validation.ProfileID{ProfileSourceXSDPreview, ProfileSourceLocal, ProfileSourceTimeline, ProfileSourceContext} {
		r, err := InspectXML(context.Background(), strings.NewReader(`<opus version="4.0"/>`), AssessmentOptions{Profile: profile})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"Container":`) || strings.Contains(string(data), `container.metadata`) {
			t.Fatalf("container payload changed old profile JSON %s", profile)
		}
	}
}

func TestAuxiliaryContainerMetadataSealedPlan(t *testing.T) {
	schema, err := compileValidationSchema(validation.AssemblyContainer, containerValidationSchema)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := validation.NewContainerMetadataContextPlan(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validation.NewContainerMetadataContextPlan(nil); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("nil schema accepted: %v", err)
	}
	for _, entry := range []struct {
		assembly validation.Assembly
		data     validationSchemaSet
	}{{validation.AssemblyMusicXML, scoreValidationSchema}, {validation.AssemblyOpus, opusValidationSchema}, {validation.AssemblySounds, soundsValidationSchema}} {
		other, err := compileValidationSchema(entry.assembly, entry.data)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := validation.NewContainerMetadataContextPlan(other); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Fatalf("wrong assembly accepted: %s %v", entry.assembly, err)
		}
	}
	for _, bad := range []string{"target", "assembly", "role", "scope", "selector"} {
		a := validation.AssemblyContainer
		rule := validation.RuleDefinition{Key: validation.RuleKey{ID: "test-container", Revision: 1}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &a}, Predicate: "test-container", Text: validation.RuleText{Title: "test", Explanation: "test", Message: "test"}, Sources: []validation.SourceRef{{IDs: []string{"container-test"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd", Locator: "rootfile"}}}
		switch bad {
		case "target":
			rule.Targets = []validation.Target{validation.TargetModel}
		case "assembly":
			a = validation.AssemblySounds
		case "role":
			rule.Role = validation.RolePredicate
		case "scope":
			rule.Scope = validation.ScopeNode
		case "selector":
			rule.Selector = validation.Selector{Component: &validation.ComponentID{Assembly: a, Kind: validation.ComponentType, Name: validation.QName{Local: "container"}}}
		}
		profile := validation.Profile{ID: "container-invalid-contract", MusicXMLVersion: "4.0", RegistryCommit: "test", Target: rule.Targets[0], SupportedRoots: []validation.QName{{Local: "container"}}, NormativeClosure: "Test invalid container projection contracts.", Required: []validation.RuleKey{rule.Key}, Support: []validation.RuleSupport{{Rule: rule.Key, State: validation.ImplementationAvailable}}}
		_, err := validation.NewCatalog([]validation.RuleDefinition{rule}, []validation.Profile{profile}, []validation.PlanBinding{{ID: rule.Predicate, Plan: plan}})
		var config *validation.ConfigurationError
		field := "context provider"
		if bad == "target" {
			field = "container context target"
		}
		if !errors.As(err, &config) || config.Field != field {
			t.Fatalf("bad %s did not reach exact contract %q: %v", bad, field, err)
		}
	}
}

func TestAuxiliaryContainerMetadataSchemaAndDigestBinding(t *testing.T) {
	schema, err := compileValidationSchema(validation.AssemblyContainer, containerValidationSchema)
	if err != nil {
		t.Fatal(err)
	}
	alternateData := containerValidationSchema
	alternateData.Elements = maps.Clone(containerValidationSchema.Elements)
	root := *alternateData.Elements[validationQName{Local: "container"}]
	root.Nillable = true
	alternateData.Elements[validationQName{Local: "container"}] = &root
	alternate, err := compileValidationSchema(validation.AssemblyContainer, alternateData)
	if err != nil {
		t.Fatal(err)
	}
	makeCatalog := func(xsdSchema, contextSchema *validation.CompiledSchema) (*validation.CompiledCatalog, error) {
		xsdPlan, err := validation.NewXSDPlan(xsdSchema, validation.XSDCheckContent)
		if err != nil {
			return nil, err
		}
		projection, err := validation.NewContainerMetadataContextPlan(contextSchema)
		if err != nil {
			return nil, err
		}
		assembly := validation.AssemblyContainer
		profile := validation.Profile{ID: "container-test", MusicXMLVersion: "4.0", RegistryCommit: "test", Target: validation.TargetSource, SupportedRoots: []validation.QName{{Local: "container"}}, NormativeClosure: "Test exact container schema and projection binding."}
		var rules []validation.RuleDefinition
		var plans []validation.PlanBinding
		for i, plan := range []validation.Plan{xsdPlan, projection} {
			key, role, scope := validation.RuleKey{ID: "container-xsd", Revision: 1}, validation.RolePredicate, validation.ScopeNode
			if i == 1 {
				key.ID, role, scope = "container-context", validation.RoleContextProvider, validation.ScopeDocument
			}
			rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: role, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource}, Scope: scope, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: "test", Explanation: "test", Message: "test"}, Sources: []validation.SourceRef{{IDs: []string{"container-test"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd", Locator: "rootfile"}}}
			rules = append(rules, rule)
			plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
			profile.Required = append(profile.Required, key)
			profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
		}
		return validation.NewCatalog(rules, []validation.Profile{profile}, plans)
	}
	first, err := makeCatalog(schema, schema)
	if err != nil {
		t.Fatal(err)
	}
	second, err := makeCatalog(alternate, alternate)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := first.Profile("container-test")
	b, _ := second.Profile("container-test")
	if a.CatalogDigest == b.CatalogDigest {
		t.Fatal("schema change omitted from container catalog digest")
	}
	if _, err := makeCatalog(schema, alternate); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("context/schema digest mismatch accepted: %v", err)
	}
	root.Type.Name.Local = "rootfiles"
	wrongRoot, err := compileValidationSchema(validation.AssemblyContainer, alternateData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validation.NewContainerMetadataContextPlan(wrongRoot); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("wrong global root type accepted: %v", err)
	}
}
