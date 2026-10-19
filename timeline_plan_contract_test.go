package musicxml

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// These contract tests use the pinned generated MusicXML graph, including its
// anonymous partwise/timewise owners and repeated full-note branches.
func timelineContractSchema(t *testing.T) *validation.CompiledSchema {
	t.Helper()
	s, err := validation.CompileSchema(validation.AssemblyMusicXML, scoreValidationSchema)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func timelineContractOwned(s *validation.CompiledSchema, owner validation.ComponentID, name string, attribute bool) []validation.SchemaUse {
	var out []validation.SchemaUse
	for _, u := range s.Uses() {
		if u.OwnerTypeKnown && u.OwnerType == owner && u.Name == (validation.QName{Local: name}) && u.Attribute == attribute {
			out = append(out, u)
		}
	}
	return out
}
func timelineContractOwners(t *testing.T, s *validation.CompiledSchema) []validation.ReferenceOwner {
	t.Helper()
	var owners []validation.ReferenceOwner
	for _, root := range []string{"score-partwise", "score-timewise"} {
		d, ok := s.Root(validation.QName{Local: root})
		if !ok {
			t.Fatal(root)
		}
		var u validation.SchemaUse
		for _, candidate := range s.Uses() {
			if candidate.Declaration == d {
				u = candidate
			}
		}
		if root == "score-timewise" {
			u = timelineContractOwned(s, u.DeclaredType, "measure", false)[0]
		}
		part := timelineContractOwned(s, u.DeclaredType, "part", false)[0]
		a := timelineContractOwned(s, part.DeclaredType, "id", true)[0]
		owners = append(owners, validation.ReferenceOwner{Declaration: part.Declaration, Attribute: validation.AttributeSelector{Name: a.Name, Declaration: a.Declaration}, Rule: validation.RuleKey{ID: validation.RuleID(root + ".part-owner"), Revision: 1}})
	}
	return owners
}
func timelineContractCatalog(t *testing.T, s *validation.CompiledSchema, provider validation.Plan, consumer validation.Plan, numeral bool) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
	t.Helper()
	assembly := validation.AssemblyMusicXML
	profile := validation.Profile{ID: "timeline.contract", Target: validation.TargetSource, MusicXMLVersion: "4.0", RegistryCommit: "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c", SupportedRoots: []validation.QName{{Local: "score-partwise"}, {Local: "score-timewise"}}, NormativeClosure: "Sealed timeline plan contracts only."}
	var rules []validation.RuleDefinition
	var bindings []validation.PlanBinding
	add := func(key validation.RuleKey, selector validation.Selector, scope validation.ScopeKind, role validation.RuleRole, p validation.Plan) {
		r := validation.RuleDefinition{Key: key, Selector: selector, Scope: scope, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Class: validation.ClassMandatory, Role: role, Category: validation.CategoryNormative, Predicate: string(key.ID), Text: validation.RuleText{Title: "Contract", Explanation: "Contract fixture", Message: "Contract fixture"}, Sources: []validation.SourceRef{{IDs: []string{"contract"}, URL: "https://www.w3.org/2021/06/musicxml40/", Locator: "Contract fixture"}}}
		rules = append(rules, r)
		bindings = append(bindings, validation.PlanBinding{ID: r.Predicate, Plan: p})
		profile.Required = append(profile.Required, key)
		profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
	}
	// Deliberately lexical consumer-before-provider naming verifies dependency order.
	typ := "note"
	if numeral {
		typ = "numeral"
	}
	component := validation.ComponentID{Assembly: assembly, Kind: validation.ComponentType, Name: validation.QName{Local: typ}}
	add(validation.RuleKey{ID: "a-consumer", Revision: 1}, validation.Selector{Component: &component}, validation.ScopeNode, validation.RolePredicate, consumer)
	rules[0].Requires = []validation.Dependency{{Kind: validation.DependencyRule, Rule: validation.RuleKey{ID: "z-timeline", Revision: 1}, Relation: validation.RelationDocument}}
	add(validation.RuleKey{ID: "z-timeline", Revision: 1}, validation.Selector{Assembly: &assembly}, validation.ScopeDocument, validation.RoleContextProvider, provider)
	var scoreParts, partLists []validation.DeclarationID
	for _, u := range s.Uses() {
		if !u.Attribute && u.OwnerTypeKnown && u.OwnerType.Name == (validation.QName{Local: "part-list"}) && u.Name.Local == "score-part" {
			scoreParts = append(scoreParts, u.Declaration)
		}
		if !u.Attribute && u.Name.Local == "part-list" && u.DeclaredTypeKnown && u.DeclaredType.Name == (validation.QName{Local: "part-list"}) {
			partLists = append(partLists, u.Declaration)
		}
	}
	for _, owner := range timelineContractOwners(t, s) {
		p, err := validation.NewTypedReferencePlan(validation.ReferencePlanSpec{Attribute: owner.Attribute, Targets: scoreParts, TargetAncestors: partLists})
		if err != nil {
			t.Fatal(err)
		}
		d := owner.Declaration
		add(owner.Rule, validation.Selector{Declaration: &d}, validation.ScopeNode, validation.RolePredicate, p)
	}
	for _, check := range []validation.XSDCheck{validation.XSDCheckBinding, validation.XSDCheckDeclaration, validation.XSDCheckAttributes, validation.XSDCheckScalar, validation.XSDCheckContent, validation.XSDCheckIdentity} {
		p, err := validation.NewXSDPlan(s, check)
		if err != nil {
			t.Fatal(err)
		}
		scope := validation.ScopeNode
		if check == validation.XSDCheckIdentity {
			scope = validation.ScopeDocument
		}
		add(validation.RuleKey{ID: validation.RuleID("xsd." + string(check)), Revision: 1}, validation.Selector{Assembly: &assembly}, scope, validation.RolePredicate, p)
	}
	return rules, []validation.Profile{profile}, bindings
}
func timelineContractPlans(t *testing.T, s *validation.CompiledSchema, numeral bool) (validation.Plan, validation.Plan) {
	t.Helper()
	p, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: s, PartOwners: timelineContractOwners(t, s)})
	if err != nil {
		t.Fatal(err)
	}
	spec := validation.ConsumerSpec{Schema: s, Provider: validation.RuleKey{ID: "z-timeline", Revision: 1}}
	var c validation.Plan
	if numeral {
		c, err = validation.NewNumeralKeyPlan(spec)
	} else {
		c, err = validation.NewChordDurationPlan(spec)
	}
	if err != nil {
		t.Fatal(err)
	}
	return p, c
}
func TestTimelinePlanSealedConfiguration(t *testing.T) {
	s := timelineContractSchema(t)
	good := validation.TimelinePlanSpec{Schema: s, PartOwners: timelineContractOwners(t, s)}
	for _, tc := range []struct {
		name   string
		change func(*validation.TimelinePlanSpec)
	}{
		{"nil schema", func(p *validation.TimelinePlanSpec) { p.Schema = nil }},
		{"zero schema", func(p *validation.TimelinePlanSpec) { p.Schema = &validation.CompiledSchema{} }},
		{"owners absent", func(p *validation.TimelinePlanSpec) { p.PartOwners = nil }},
		{"owner missing", func(p *validation.TimelinePlanSpec) { p.PartOwners = p.PartOwners[:1] }},
		{"duplicate owner", func(p *validation.TimelinePlanSpec) { p.PartOwners[1] = p.PartOwners[0] }},
		{"wrong owner declaration", func(p *validation.TimelinePlanSpec) {
			p.PartOwners[0].Declaration = p.PartOwners[1].Attribute.Declaration
		}},
		{"wrong owner slot", func(p *validation.TimelinePlanSpec) { p.PartOwners[0].Attribute = p.PartOwners[1].Attribute }},
		{"zero owner revision", func(p *validation.TimelinePlanSpec) { p.PartOwners[0].Rule.Revision = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := good
			p.PartOwners = append([]validation.ReferenceOwner(nil), good.PartOwners...)
			tc.change(&p)
			if _, err := validation.NewTimelinePlan(p); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted invalid contract: %v", err)
			}
		})
	}
	for _, makePlan := range []func(validation.ConsumerSpec) (validation.Plan, error){validation.NewChordDurationPlan, validation.NewNumeralKeyPlan} {
		for _, spec := range []validation.ConsumerSpec{{}, {Schema: s}, {Schema: s, Provider: validation.RuleKey{ID: "z-timeline"}}, {Provider: validation.RuleKey{ID: "z-timeline", Revision: 1}}} {
			if _, err := makePlan(spec); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted consumer %+v: %v", spec, err)
			}
		}
	}
}
func TestTimelineCatalogRequiresExactProvider(t *testing.T) {
	s := timelineContractSchema(t)
	for _, numeral := range []bool{false, true} {
		for _, unavailable := range []bool{false, true} {
			p, c := timelineContractPlans(t, s, numeral)
			for _, tc := range []struct {
				name   string
				change func([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding)
			}{
				{"missing dependency", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					r[0].Requires = nil
				}},
				{"wrong dependency revision", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					r[0].Requires[0].Rule.Revision = 2
				}},
				{"wrong dependency relation", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					r[0].Requires[0].Relation = validation.RelationMusicalPosition
				}},
				{"wrong provider role", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					r[1].Role = validation.RolePredicate
				}},
				{"wrong provider scope", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					r[1].Scope = validation.ScopeNode
				}},
				{"wrong provider assembly", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					a := validation.AssemblyOpus
					r[1].Selector.Assembly = &a
				}},
				{"wrong provider kind", func(_ []validation.RuleDefinition, _ []validation.Profile, b []validation.PlanBinding) {
					b[1].Plan, _ = validation.NewContextValuePlan("musical-fact", validation.RelationDocument, validation.Atomic{Kind: validation.AtomicString, Text: "unsafe"})
				}},
				{"wrong consumer selector", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					x := *r[0].Selector.Component
					x.Name.Local = "sound"
					r[0].Selector.Component = &x
				}},
				{"wrong provider targets", func(r []validation.RuleDefinition, _ []validation.Profile, _ []validation.PlanBinding) {
					r[1].Targets = []validation.Target{validation.TargetSource, validation.TargetPackage}
				}},
				{"wrong owner provider", func(_ []validation.RuleDefinition, _ []validation.Profile, b []validation.PlanBinding) {
					b[2].Plan, _ = validation.NewChildCountPlan([]validation.DeclarationID{timelineContractOwners(t, s)[0].Declaration}, 0, -1)
				}},
			} {
				t.Run(tc.name+map[bool]string{true: "/numeral", false: "/chord"}[numeral]+map[bool]string{true: "/unsupported", false: "/available"}[unavailable], func(t *testing.T) {
					r, ps, b := timelineContractCatalog(t, s, p, c, numeral)
					if unavailable {
						ps[0].Support[1].State = validation.ImplementationUnsupported
						ps[0].Support[1].Reason = "not-selected"
					}
					tc.change(r, ps, b)
					if _, err := validation.NewCatalog(r, ps, b); !errors.Is(err, validation.ErrInvalidConfiguration) {
						t.Fatalf("accepted invalid binding: %v", err)
					}
				})
			}
		}
	}
}
func TestTimelineCatalogOwnershipDigestAndOrder(t *testing.T) {
	s := timelineContractSchema(t)
	spec := validation.TimelinePlanSpec{Schema: s, PartOwners: timelineContractOwners(t, s)}
	p, err := validation.NewTimelinePlan(spec)
	if err != nil {
		t.Fatal(err)
	}
	_, consumer := timelineContractPlans(t, s, false)
	r, ps, b := timelineContractCatalog(t, s, p, consumer, false)
	first, err := validation.NewCatalog(r, ps, b)
	if err != nil {
		t.Fatal(err)
	}
	spec.PartOwners[0].Rule.ID = "caller mutation"
	spec.PartOwners[1].Attribute.Name.Local = "changed"
	again, err := validation.NewCatalog(r, ps, b)
	if err != nil {
		t.Fatal(err)
	}
	if timelineContractDigest(first) != timelineContractDigest(again) {
		t.Fatal("caller mutation changed compiled plan")
	}
	owners := timelineContractOwners(t, s)
	slices.Reverse(owners)
	b[1].Plan, err = validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: s, PartOwners: owners})
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := validation.NewCatalog(r, ps, b)
	if err != nil {
		t.Fatal(err)
	}
	if timelineContractDigest(first) != timelineContractDigest(reordered) {
		t.Fatal("set order changed digest")
	}
	order, _ := first.Schedule(ps[0].ID)
	provider := slices.Index(order, r[1].Key)
	consumerIndex := slices.Index(order, r[0].Key)
	if provider < 0 || provider >= consumerIndex {
		t.Fatalf("provider not before consumer: %+v", order)
	}
	for _, o := range timelineContractOwners(t, s) {
		if at := slices.Index(order, o.Rule); at < 0 || at >= provider {
			t.Fatalf("owner not before service: %+v", order)
		}
	}
	stored, _ := first.Rule(r[0].Key)
	if !reflect.DeepEqual(stored.Requires, r[0].Requires) {
		t.Fatal("compile rewrote public dependencies")
	}
}

func timelineContractDigest(c *validation.CompiledCatalog) string {
	p, _ := c.Profile("timeline.contract")
	return p.CatalogDigest
}

func timelineContractVariantSchema(t *testing.T, change func(*validation.SchemaData)) *validation.CompiledSchema {
	t.Helper()
	data := scoreValidationSchema
	data.Types = make(map[validation.SchemaName]*validation.SchemaType, len(scoreValidationSchema.Types))
	for k, v := range scoreValidationSchema.Types {
		data.Types[k] = v
	}
	change(&data)
	s, err := validation.CompileSchema(validation.AssemblyMusicXML, data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func timelineContractChangedSchema(t *testing.T) *validation.CompiledSchema {
	return timelineContractVariantSchema(t, func(data *validation.SchemaData) {
		key := validation.SchemaName{Local: "mode"}
		typ := *data.Types[key]
		simple := *typ.Simple
		simple.Enumerations = []string{"contract-variant"}
		typ.Simple = &simple
		data.Types[key] = &typ
	})
}
func timelineContractRewriteParticle(p *validation.SchemaParticle, name, typ string, duplicate bool) *validation.SchemaParticle {
	if p == nil {
		return nil
	}
	q := *p
	q.Children = make([]*validation.SchemaParticle, 0, len(p.Children))
	for _, child := range p.Children {
		copy := timelineContractRewriteParticle(child, name, typ, duplicate)
		q.Children = append(q.Children, copy)
		if duplicate && child.Element != nil && child.Element.Name.Local == name {
			q.Children = append(q.Children, copy)
		}
	}
	if p.Element != nil {
		e := *p.Element
		if e.Name.Local == name && typ != "" {
			e.Type = validation.SchemaTypeRef{Name: validation.SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: typ}}
		}
		q.Element = &e
	}
	return &q
}
func TestTimelinePlanRejectsMalformedPinnedRoleShapes(t *testing.T) {
	for _, tc := range []struct {
		name, owner, child, typ string
		duplicate               bool
	}{
		{name: "wrong scalar type", owner: "note", child: "duration", typ: "string"},
		{name: "wrong state type", owner: "attributes", child: "key", typ: "string"},
		{name: "ambiguous same-owner role", owner: "time", child: "beats", duplicate: true},
		{name: "wrong local-key type", owner: "numeral-key", child: "numeral-mode", typ: "string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := timelineContractVariantSchema(t, func(data *validation.SchemaData) {
				key := validation.SchemaName{Local: tc.owner}
				typ := *data.Types[key]
				complex := *typ.Complex
				complex.Particle = timelineContractRewriteParticle(complex.Particle, tc.child, tc.typ, tc.duplicate)
				typ.Complex = &complex
				data.Types[key] = &typ
			})
			if _, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: s, PartOwners: timelineContractOwners(t, s)}); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted malformed role shape: %v", err)
			}
		})
	}
}
func TestTimelineCatalogSelectionSchemaAndImplicitCycles(t *testing.T) {
	s := timelineContractSchema(t)
	other := timelineContractChangedSchema(t)
	for _, tc := range []struct {
		name   string
		change func([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding)
	}{
		{"unselected owner", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			p[0].Required = slices.Delete(p[0].Required, 2, 3)
			p[0].Support = slices.Delete(p[0].Support, 2, 3)
			return r, p, b
		}},
		{"owner revision", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			owners := timelineContractOwners(t, s)
			owners[0].Rule.Revision++
			b[1].Plan, _ = validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: s, PartOwners: owners})
			return r, p, b
		}},
		{"owner target mismatch", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			r[2].Targets = []validation.Target{validation.TargetSource}
			return r, p, b
		}},
		{"implicit owner cycle", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			r[2].Requires = []validation.Dependency{{Kind: validation.DependencyRule, Rule: r[1].Key, Relation: validation.RelationDocument}}
			return r, p, b
		}},
		{"unselected schema", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			p[0].Required = p[0].Required[:4]
			p[0].Support = p[0].Support[:4]
			return r, p, b
		}},
		{"consumer schema mismatch", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			b[0].Plan, _ = validation.NewChordDurationPlan(validation.ConsumerSpec{Schema: other, Provider: r[1].Key})
			return r, p, b
		}},
		{"selected XSD schema mismatch", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			checks := []validation.XSDCheck{validation.XSDCheckBinding, validation.XSDCheckDeclaration, validation.XSDCheckAttributes, validation.XSDCheckScalar, validation.XSDCheckContent, validation.XSDCheckIdentity}
			for i, check := range checks {
				b[4+i].Plan, _ = validation.NewXSDPlan(other, check)
			}
			return r, p, b
		}},
		{"two selected services", func(r []validation.RuleDefinition, p []validation.Profile, b []validation.PlanBinding) ([]validation.RuleDefinition, []validation.Profile, []validation.PlanBinding) {
			copy := r[1]
			copy.Key.ID = "second-service"
			copy.Predicate = "second-service"
			r = append(r, copy)
			b = append(b, validation.PlanBinding{ID: copy.Predicate, Plan: b[1].Plan})
			p[0].Required = append(p[0].Required, copy.Key)
			p[0].Support = append(p[0].Support, validation.RuleSupport{Rule: copy.Key, State: validation.ImplementationAvailable})
			return r, p, b
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider, consumer := timelineContractPlans(t, s, false)
			r, p, b := timelineContractCatalog(t, s, provider, consumer, false)
			r, p, b = tc.change(r, p, b)
			if _, err := validation.NewCatalog(r, p, b); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted invalid profile: %v", err)
			}
		})
	}
}
func TestTimelinePlansSnapshotSchemaHandlesAndProviderRevision(t *testing.T) {
	s := timelineContractSchema(t)
	provider, consumer := timelineContractPlans(t, s, false)
	r, p, b := timelineContractCatalog(t, s, provider, consumer, false)
	c, err := validation.NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	replacement := r[1].Key
	replacement.Revision++
	b[0].Plan, err = validation.NewChordDurationPlan(validation.ConsumerSpec{Schema: s, Provider: replacement})
	if err != nil {
		t.Fatal(err)
	}
	r[1].Key = replacement
	r[0].Requires[0].Rule = replacement
	p[0].Required[1] = replacement
	p[0].Support[1].Rule = replacement
	revised, err := validation.NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	if timelineContractDigest(c) == timelineContractDigest(revised) {
		t.Fatal("exact provider revision absent from digest")
	}
	*s = validation.CompiledSchema{}
	independent, err := validation.NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	if timelineContractDigest(revised) != timelineContractDigest(independent) {
		t.Fatal("plan retained caller schema handle")
	}
}

func TestTimelinePlanRejectsWrongNamedScalarFamilies(t *testing.T) {
	for _, tc := range []struct {
		name, typ string
		change    func(*validation.SchemaType)
	}{
		{"duration alias becomes string", "positive-divisions", func(t *validation.SchemaType) {
			s := *t.Simple
			s.Base = &validation.SchemaSimpleMember{Name: validation.SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}
			t.Simple = &s
		}},
		{"integer key becomes decimal", "fifths", func(t *validation.SchemaType) {
			s := *t.Simple
			s.Base = &validation.SchemaSimpleMember{Name: validation.SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "decimal"}}
			t.Simple = &s
		}},
		{"arbitrary precision staff becomes string", "staff-number", func(t *validation.SchemaType) {
			s := *t.Simple
			s.Base = &validation.SchemaSimpleMember{Name: validation.SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}
			t.Simple = &s
		}},
		{"pedal union loses numeric branch", "yes-no-number", func(t *validation.SchemaType) {
			s := *t.Simple
			s.Members = []*validation.SchemaSimpleMember{{Name: validation.SchemaName{Local: "yes-no"}}}
			t.Simple = &s
		}},
		{"complex scalar octave becomes string", "key-octave", func(t *validation.SchemaType) {
			c := *t.Complex
			c.Base = &validation.SchemaTypeRef{Name: validation.SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}
			t.Complex = &c
		}},
		{"structured key becomes mixed", "key", func(t *validation.SchemaType) { c := *t.Complex; c.Mixed = true; t.Complex = &c }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := timelineContractVariantSchema(t, func(d *validation.SchemaData) {
				key := validation.SchemaName{Local: tc.typ}
				typ := *d.Types[key]
				tc.change(&typ)
				d.Types[key] = &typ
			})
			if _, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: timelineContractOwners(t, schema)}); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted wrong underlying scalar/complex family: %v", err)
			}
		})
	}
}

func TestTimelinePlanRequiresRetainedCompositeDiscriminators(t *testing.T) {
	for _, tc := range []struct{ owner, name string }{{"other-play", "type"}, {"line-detail", "line"}, {"staff-size", "scaling"}, {"staff-details", "show-frets"}, {"staff-details", "print-object"}, {"staff-details", "print-spacing"}, {"line-detail", "width"}, {"line-detail", "color"}, {"line-detail", "line-type"}, {"line-detail", "print-object"}} {
		t.Run(tc.owner+"/"+tc.name, func(t *testing.T) {
			schema := timelineContractVariantSchema(t, func(data *validation.SchemaData) {
				key := validation.SchemaName{Local: tc.owner}
				typ := *data.Types[key]
				c := *typ.Complex
				c.Attributes = nil
				found := false
				for _, a := range typ.Complex.Attributes {
					if a.Name.Local == tc.name {
						found = true
						continue
					}
					c.Attributes = append(c.Attributes, a)
				}
				if !found {
					t.Fatal("fixture role missing")
				}
				typ.Complex = &c
				data.Types[key] = &typ
			})
			if _, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: timelineContractOwners(t, schema)}); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted missing retained discriminator: %v", err)
			}
		})
	}
}
