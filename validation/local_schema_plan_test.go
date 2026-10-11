package validation

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
)

const localXS = "http://www.w3.org/2001/XMLSchema"

func localType(name string) SchemaTypeRef { return SchemaTypeRef{Name: SchemaName{Local: name}} }
func localBuiltin(name string) SchemaTypeRef {
	return SchemaTypeRef{Name: SchemaName{Space: localXS, Local: name}}
}
func localElement(name, typ string) *SchemaParticle {
	return &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: SchemaName{Local: name}, Type: localType(typ)}}
}
func localChoice(children ...*SchemaParticle) *SchemaParticle {
	return &SchemaParticle{Kind: SchemaParticleChoice, Occurrence: SchemaOccurrence{Min: 0, Unbounded: true}, Children: children}
}
func localSchema(t *testing.T) *CompiledSchema {
	t.Helper()
	types := map[SchemaName]*SchemaType{}
	add := func(name string, c *SchemaComplex) { types[SchemaName{Local: name}] = &SchemaType{Complex: c} }
	attr := func(name, builtin string, use SchemaAttributeUse) SchemaAttribute {
		return SchemaAttribute{Name: SchemaName{Local: name}, Type: localBuiltin(builtin), Use: use}
	}
	add("empty", &SchemaComplex{Form: SchemaComplexDirect})
	types[SchemaName{Local: "string"}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: localBuiltin("string").Name}}}
	types[SchemaName{Local: "decimal"}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: localBuiltin("decimal").Name}}}
	types[SchemaName{Local: "integer"}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: localBuiltin("integer").Name}}}
	types[SchemaName{Local: "yes-no"}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: localBuiltin("token").Name}, Enumerations: []string{"yes", "no"}}}
	types[SchemaName{Local: "pedal"}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleUnion, Members: []*SchemaSimpleMember{{Name: SchemaName{Local: "yes-no"}}, {Name: localBuiltin("decimal").Name}}}}
	number := attr("number", "integer", SchemaAttributeOptional)
	number.Constraint = &SchemaValueConstraint{Kind: "default", Value: "1"}
	add("beam", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{number}})
	add("note", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("beam", "beam"), localElement("instrument", "reference"), localElement("assess", "assess"))})
	add("registration", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("high", "empty"), localElement("middle", "integer"), localElement("low", "empty"))})
	for _, name := range []string{"slur", "tied"} {
		add(name, &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{attr("type", "token", SchemaAttributeRequired), attr("x2", "decimal", SchemaAttributeOptional), attr("y2", "decimal", SchemaAttributeOptional), attr("offset2", "decimal", SchemaAttributeOptional)}})
	}
	cancel := SchemaAttribute{Name: SchemaName{Local: "cancel"}, Type: localType("yes-no"), Use: SchemaAttributeOptional, Constraint: &SchemaValueConstraint{Kind: "default", Value: "no"}}
	add("octave", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{cancel}})
	add("key", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("cancel", "integer"), localElement("octave", "octave"))})
	add("transpose", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("chromatic", "decimal"), localElement("diatonic", "integer"), localElement("octave-change", "integer"))})
	add("defaults", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("concert", "empty"))})
	add("sound", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "dacapo"}, Type: localType("yes-no")}, {Name: SchemaName{Local: "forward-repeat"}, Type: localType("yes-no")}, {Name: SchemaName{Local: "pedal"}, Type: localType("pedal")}}})
	add("target", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{attr("id", "ID", SchemaAttributeRequired)}})
	add("reference", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{attr("id", "IDREF", SchemaAttributeOptional)}, Particle: localChoice(localElement("channel", "integer"), localElement("bank", "integer"), localElement("program", "integer"))})
	add("assess", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{attr("player", "IDREF", SchemaAttributeOptional), {Name: SchemaName{Local: "type"}, Type: localType("yes-no"), Use: SchemaAttributeRequired}}})
	add("score-part", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{attr("id", "ID", SchemaAttributeRequired)}, Particle: localChoice(localElement("score-instrument", "target"), localElement("player", "target"), localElement("assignment", "reference"), localElement("instrument-link", "reference"))})
	add("part-list", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("score-part", "score-part"))})
	add("part", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{attr("id", "IDREF", SchemaAttributeRequired)}, Particle: localChoice(localElement("note", "note"), localElement("assignment", "reference"), localElement("play", "reference"))})
	add("measure", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("part", "part"))})
	add("root", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("part-list", "part-list"), localElement("part", "part"), localElement("measure", "measure"), localElement("other", "target"), localElement("registration", "registration"), localElement("note", "note"), localElement("slur", "slur"), localElement("tied", "tied"), localElement("key", "key"), localElement("defaults", "defaults"), localElement("for-part", "empty"), localElement("transpose", "transpose"), localElement("part-transpose", "transpose"), localElement("relation", "string"), localElement("sound", "sound"))})
	schema, e := CompileSchema(AssemblyMusicXML, SchemaData{Types: types, Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: localType("root")}}})
	if e != nil {
		t.Fatal(e)
	}
	return schema
}
func localUse(t *testing.T, s *CompiledSchema, owner, name string, attribute bool) SchemaUse {
	t.Helper()
	var found []SchemaUse
	for _, u := range s.Uses() {
		if u.Name.Local == name && u.Attribute == attribute && u.OwnerTypeKnown && u.OwnerType.Name.Local == owner {
			found = append(found, u)
		}
	}
	if len(found) != 1 {
		t.Fatalf("use %s/%s attr=%v got=%+v", owner, name, attribute, found)
	}
	return found[0]
}
func localSchemaAttr(t *testing.T, s *CompiledSchema, owner, name string) AttributeSelector {
	u := localUse(t, s, owner, name, true)
	return AttributeSelector{Name: u.Name, Declaration: u.Declaration}
}

type localRule struct {
	key  RuleKey
	decl DeclarationID
	plan Plan
}

func localBinding(t *testing.T, name string, d DeclarationID, p Plan, e error) localRule {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
	return localRule{RuleKey{ID: RuleID(name), Revision: 1}, d, p}
}
func localEngine(t *testing.T, schema *CompiledSchema, target Target, selected ...localRule) *Engine {
	t.Helper()
	var rules []RuleDefinition
	var plans []PlanBinding
	p := Profile{ID: "local.test", Target: target, MusicXMLVersion: "4.0", RegistryCommit: "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c", SupportedRoots: []QName{{Local: "root"}}, NormativeClosure: "Synthetic exact-declaration local and identity contracts."}
	add := func(k RuleKey, selector Selector, plan Plan, scope ScopeKind) {
		r := RuleDefinition{Key: k, Targets: []Target{TargetSource, TargetModel}, Scope: scope, Category: CategoryNormative, Class: ClassMandatory, Role: RolePredicate, Selector: selector, Predicate: string(k.ID), Text: RuleText{Title: "Local contract", Explanation: "Assess this exact local contract.", Message: "Local contract assessment."}, Sources: []SourceRef{{IDs: []string{"local-test"}, URL: "https://www.w3.org/TR/xmlschema-1/", Locator: "test"}}}
		rules = append(rules, r)
		plans = append(plans, PlanBinding{ID: r.Predicate, Plan: plan})
		p.Required = append(p.Required, k)
		p.Support = append(p.Support, RuleSupport{Rule: k, State: ImplementationAvailable})
	}
	for _, c := range []XSDCheck{XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar, XSDCheckContent, XSDCheckIdentity} {
		plan, e := NewXSDPlan(schema, c)
		if e != nil {
			t.Fatal(e)
		}
		assembly := AssemblyMusicXML
		scope := ScopeNode
		if c == XSDCheckIdentity {
			scope = ScopeDocument
		}
		add(RuleKey{ID: RuleID("xsd." + string(c)), Revision: 1}, Selector{Assembly: &assembly}, plan, scope)
	}
	for _, r := range selected {
		d := r.decl
		add(r.key, Selector{Declaration: &d}, r.plan, ScopeNode)
	}
	c, e := NewCatalog(rules, []Profile{p}, plans)
	if e != nil {
		t.Fatal(e)
	}
	engine, e := NewEngine(c)
	if e != nil {
		t.Fatal(e)
	}
	return engine
}

type localTree struct {
	name, text string
	attrs      map[string]string
	children   []*localTree
	value      *Atomic
	current    map[string]Atomic
	incomplete bool
}

func lt(name string, children ...*localTree) *localTree {
	return &localTree{name: name, children: children}
}
func la(name, id string, children ...*localTree) *localTree {
	return &localTree{name: name, attrs: map[string]string{"id": id}, children: children}
}
func localFeed(t *testing.T, e *Engine, target Target, tree *localTree, complete bool) Report {
	t.Helper()
	ctx := context.Background()
	s, err := e.Begin(ctx, RunOptions{DocumentID: "doc", Profile: "local.test"})
	if err != nil {
		t.Fatal(err)
	}
	var next NodeID
	var visit func(*localTree, NodeID)
	visit = func(n *localTree, parent NodeID) {
		next++
		id := next
		h := schemaPendingHeader()
		h.ID = id
		h.Parent = parent
		h.Name = QName{Local: n.name}
		h.Location = Location{Target: target, DocumentID: "doc", Path: fmt.Sprintf("/%s[%d]", n.name, id)}
		if err := s.Open(ctx, h); err != nil {
			t.Fatal(err)
		}
		var names []string
		for name := range n.attrs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			a := AttributeFact{Owner: id, Name: QName{Local: name}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Location: h.Location, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: n.attrs[name]}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}}}
			if target == TargetModel {
				a.Scalar.Lexeme = Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}
				v := localString(n.attrs[name])
				if cv, ok := n.current[name]; ok {
					v = cv
				}
				a.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: v}
			}
			if err := s.Attribute(ctx, a); err != nil {
				t.Fatal(err)
			}
		}
		for _, c := range n.children {
			visit(c, id)
		}
		end := schemaLexemeEnd(n.text)
		end.ID = id
		if n.incomplete {
			end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "test-incomplete"}
		}
		if target == TargetModel {
			end.Order = Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelContent}
			end.Scalar.Lexeme = Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}
			end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: localString(n.text)}
			if n.value != nil {
				end.Scalar.Value.Value = *n.value
			}
		}
		if err := s.Close(ctx, end); err != nil {
			t.Fatal(err)
		}
	}
	visit(tree, 0)
	syntax := SyntaxWellFormed
	if target == TargetModel {
		syntax = SyntaxNotAssessed
	}
	r, err := s.Finish(ctx, Completion{InputComplete: complete, Syntax: syntax})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func localFinding(r Report, key string) (Finding, bool) {
	for _, d := range r.Findings {
		if string(d.Rule.ID) == key {
			return d, true
		}
	}
	return Finding{}, false
}
func TestLocalSchemaDefaultsAndInvalidSlotCausality(t *testing.T) {
	schema := localSchema(t)
	note := localUse(t, schema, "root", "note", false).Declaration
	number := localSchemaAttr(t, schema, "beam", "number")
	beam := localUse(t, schema, "note", "beam", false).Declaration
	plan, err := NewDistinctChildAttributePlan([]DeclarationID{beam}, number)
	binding := localBinding(t, "local.beams", note, plan, err)
	for _, target := range []Target{TargetSource, TargetModel} {
		for _, values := range [][]string{{"absent", "1"}, {"absent", "2"}, {"bad", "2"}, {"7", "8"}} {
			t.Run(string(target)+fmt.Sprint(values), func(t *testing.T) {
				n := lt("note")
				for _, v := range values {
					b := lt("beam")
					if v != "absent" {
						b.attrs = map[string]string{"number": v}
					}
					n.children = append(n.children, b)
				}
				r := localFeed(t, localEngine(t, schema, target, binding), target, lt("root", n), true)
				d, found := localFinding(r, "local.beams")
				switch values[1] {
				case "1":
					if !found || d.Outcome != OutcomeFail {
						t.Fatalf("default collision: %+v", r)
					}
				case "2":
					if values[0] == "bad" {
						if !found || d.Outcome != OutcomeBlocked || d.Cause == nil || d.Cause.Rule.ID != "xsd.attributes" {
							t.Fatalf("invalid slot cause: %+v", r)
						}
					} else if found || !r.Satisfied() {
						t.Fatalf("distinct: %+v", r)
					}
				case "8":
					if found || !r.Satisfied() {
						t.Fatalf("high levels: %+v", r)
					}
				}
			})
		}
	}
}

func TestLocalSchemaClauseMatrixBothTargets(t *testing.T) {
	schema := localSchema(t)
	decl := func(owner, name string) DeclarationID { return localUse(t, schema, owner, name, false).Declaration }
	attr := func(owner, name string) AttributeSelector { return localSchemaAttr(t, schema, owner, name) }
	type clause struct {
		name    string
		binding localRule
		cases   []struct {
			name string
			tree *localTree
			want Outcome
		}
	}
	var clauses []clause
	add := func(name string, d DeclarationID, p Plan, e error, cases []struct {
		name string
		tree *localTree
		want Outcome
	}) {
		clauses = append(clauses, clause{name, localBinding(t, "clause."+name, d, p, e), cases})
	}
	p, e := NewChildCountPlan([]DeclarationID{decl("registration", "high"), decl("registration", "middle"), decl("registration", "low")}, 1, -1)
	add("accordion", decl("root", "registration"), p, e, []struct {
		name string
		tree *localTree
		want Outcome
	}{{"empty", lt("root", lt("registration")), OutcomeFail}, {"high", lt("root", lt("registration", lt("high"))), OutcomePass}, {"middle zero", lt("root", lt("registration", &localTree{name: "middle", text: "0"})), OutcomePass}, {"low", lt("root", lt("registration", lt("low"))), OutcomePass}, {"partial", lt("root", &localTree{name: "registration", incomplete: true}), OutcomeUnknown}})
	p, e = NewScalarOneOfPlan([]Atomic{localString(""), localString("equals")})
	var relationCases []struct {
		name string
		tree *localTree
		want Outcome
	}
	for _, value := range []string{"", "equals", " ", " equals ", "Equals", "not-equals"} {
		want := OutcomeFail
		if value == "" || value == "equals" {
			want = OutcomePass
		}
		relationCases = append(relationCases, struct {
			name string
			tree *localTree
			want Outcome
		}{value, lt("root", &localTree{name: "relation", text: value}), want})
	}
	add("relation", decl("root", "relation"), p, e, relationCases)
	for _, flag := range []string{"dacapo", "forward-repeat"} {
		p, e = NewPresentAttributeValuePlan(attr("sound", flag), []Atomic{localString("yes")})
		var cases []struct {
			name string
			tree *localTree
			want Outcome
		}
		for _, v := range []string{"absent", "yes", " yes ", "no", "maybe"} {
			n := lt("sound")
			if v != "absent" {
				n.attrs = map[string]string{flag: v}
			}
			want := OutcomePass
			switch v {
			case "absent":
				want = OutcomeNotApplicable
			case "no":
				want = OutcomeFail
			case "maybe":
				want = OutcomeBlocked
			}
			cases = append(cases, struct {
				name string
				tree *localTree
				want Outcome
			}{v, lt("root", n), want})
		}
		add(flag, decl("root", "sound"), p, e, cases)
	}
	p, e = NewAttributeNumberOrTokensPlan(attr("sound", "pedal"), []string{"yes", "no"}, localDecimal(t, "0"), localDecimal(t, "100"))
	var pedalCases []struct {
		name string
		tree *localTree
		want Outcome
	}
	for _, v := range []string{"yes", " no ", "0", "-0", " +100.00 ", "37.5", "100.0000000000001", "-0.01", "1e2", "NaN", "maybe"} {
		want := OutcomePass
		switch v {
		case "100.0000000000001", "-0.01":
			want = OutcomeFail
		case "1e2", "NaN", "maybe":
			want = OutcomeBlocked
		}
		pedalCases = append(pedalCases, struct {
			name string
			tree *localTree
			want Outcome
		}{v, lt("root", &localTree{name: "sound", attrs: map[string]string{"pedal": v}}), want})
	}
	add("pedal", decl("root", "sound"), p, e, pedalCases)
	for _, owner := range []string{"slur", "tied"} {
		slurType := ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "slur"}}
		p, e = NewPresenceImpliesAttributePlan([]AttributeSelector{attr(owner, "x2"), attr(owner, "y2"), attr(owner, "offset2")}, Selector{Component: &slurType}, attr(owner, "type"), localString("continue"))
		var cases []struct {
			name string
			tree *localTree
			want Outcome
		}
		for _, kind := range []string{"start", "stop", "continue"} {
			for _, trigger := range []string{"x2", "y2", "offset2"} {
				want := OutcomeFail
				if owner == "slur" && kind == "continue" {
					want = OutcomePass
				}
				cases = append(cases, struct {
					name string
					tree *localTree
					want Outcome
				}{kind + trigger, lt("root", &localTree{name: owner, attrs: map[string]string{"type": kind, trigger: "0"}}), want})
			}
		}
		cases = append(cases, struct {
			name string
			tree *localTree
			want Outcome
		}{"no controls", lt("root", &localTree{name: owner, attrs: map[string]string{"type": "continue"}}), OutcomeNotApplicable})
		add(owner, decl("root", owner), p, e, cases)
	}
	p, e = NewAttributeImpliesRelatedChildPlan(attr("octave", "cancel"), localString("yes"), RelationParent, []DeclarationID{decl("key", "cancel")})
	var keyCases []struct {
		name string
		tree *localTree
		want Outcome
	}
	for _, v := range []string{"absent", "no", "yes", "bad"} {
		for _, present := range []bool{false, true} {
			oct := lt("octave")
			if v != "absent" {
				oct.attrs = map[string]string{"cancel": v}
			}
			key := lt("key", oct)
			if present {
				key.children = append(key.children, &localTree{name: "cancel", text: "0"})
			}
			want := OutcomeNotApplicable
			if v == "yes" {
				want = OutcomeFail
				if present {
					want = OutcomePass
				}
			}
			if v == "bad" {
				want = OutcomeBlocked
			}
			keyCases = append(keyCases, struct {
				name string
				tree *localTree
				want Outcome
			}{fmt.Sprint(v, present), lt("root", key, lt("key", &localTree{name: "cancel", text: "0"})), want})
		}
	}
	add("cancel", decl("key", "octave"), p, e, keyCases)
	p, e = NewDocumentChildPresencePlan([]DeclarationID{decl("defaults", "concert")})
	add("for-part", decl("root", "for-part"), p, e, []struct {
		name string
		tree *localTree
		want Outcome
	}{{"absent", lt("root", lt("for-part")), OutcomeFail}, {"present", lt("root", lt("defaults", lt("concert")), lt("for-part")), OutcomePass}, {"incomplete", lt("root", &localTree{name: "defaults", incomplete: true}, lt("for-part")), OutcomeUnknown}})
	p, e = NewDocumentMarkerValuesPlan([]DeclarationID{decl("defaults", "concert")}, []ChildValueRequirement{{Child: decl("transpose", "chromatic"), Expected: Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "0")}}, {Child: decl("transpose", "diatonic"), Expected: localInteger(t, 0), AllowAbsent: true}})
	var transposeCases []struct {
		name string
		tree *localTree
		want Outcome
	}
	for _, value := range []string{"0", "-0.00", "2", "bad"} {
		want := OutcomePass
		if value == "2" {
			want = OutcomeFail
		}
		if value == "bad" {
			want = OutcomeBlocked
		}
		transposeCases = append(transposeCases, struct {
			name string
			tree *localTree
			want Outcome
		}{value, lt("root", lt("defaults", lt("concert")), lt("transpose", &localTree{name: "chromatic", text: value}, &localTree{name: "octave-change", text: "1"})), want})
	}
	transposeCases = append(transposeCases, struct {
		name string
		tree *localTree
		want Outcome
	}{"diatonic nonzero", lt("root", lt("defaults", lt("concert")), lt("transpose", &localTree{name: "chromatic", text: "0"}, &localTree{name: "diatonic", text: "1"})), OutcomeFail}, struct {
		name string
		tree *localTree
		want Outcome
	}{"no marker", lt("root", lt("transpose", &localTree{name: "chromatic", text: "2"})), OutcomeNotApplicable}, struct {
		name string
		tree *localTree
		want Outcome
	}{"part transpose excluded", lt("root", lt("defaults", lt("concert")), lt("part-transpose", &localTree{name: "chromatic", text: "2"})), OutcomeNotApplicable})
	add("transpose", decl("root", "transpose"), p, e, transposeCases)
	for _, target := range []Target{TargetSource, TargetModel} {
		for _, clause := range clauses {
			engine := localEngine(t, schema, target, clause.binding)
			for _, tc := range clause.cases {
				t.Run(string(target)+"/"+clause.name+"/"+tc.name, func(t *testing.T) {
					r := localFeed(t, engine, target, tc.tree, true)
					d, found := localFinding(r, string(clause.binding.key.ID))
					if tc.want == OutcomePass || tc.want == OutcomeNotApplicable {
						if found || !r.Satisfied() {
							t.Fatalf("unexpected finding/report: %+v", r)
						}
					} else if !found || d.Outcome != tc.want {
						t.Fatalf("want=%s got=%+v report=%+v", tc.want, d, r)
					}
					if tc.want == OutcomeFail && (d.Expected == "" || d.Actual == "") {
						t.Fatalf("missing actionable expected/actual: %+v", d)
					}
					if tc.want == OutcomeBlocked && d.Cause == nil {
						t.Fatalf("missing exact prerequisite cause %+v", d)
					}
					if found && d.Location.Target != target {
						t.Fatalf("wrong target location %+v", d.Location)
					}
				})
			}
		}
	}
}

func TestLocalFailureSurvivesDiagnosticTruncationAndReleasesAccount(t *testing.T) {
	schema := localSchema(t)
	p, e := NewScalarOneOfPlan([]Atomic{localString("equals")})
	rule := localBinding(t, "local.literal", localUse(t, schema, "root", "relation", false).Declaration, p, e)
	engine := localEngine(t, schema, TargetSource, rule)
	account, e := NewBudgetAccount(Budgets{MaxDiagnosticBytes: 1})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	s, e := engine.Begin(ctx, RunOptions{DocumentID: "doc", Profile: "local.test", Account: account})
	if e != nil {
		t.Fatal(e)
	}
	root := schemaPendingHeader()
	root.Location.DocumentID = "doc"
	if e = s.Open(ctx, root); e != nil {
		t.Fatal(e)
	}
	child := schemaPendingHeader()
	child.ID = 2
	child.Parent = 1
	child.Name.Local = "relation"
	child.Location.DocumentID = "doc"
	child.Location.Path = "/root/relation"
	if e = s.Open(ctx, child); e != nil {
		t.Fatal(e)
	}
	end := schemaLexemeEnd("wrong")
	end.ID = 2
	if e = s.Close(ctx, end); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(ctx, schemaLexemeEnd("")); e != nil {
		t.Fatal(e)
	}
	report, e := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if e != nil {
		t.Fatal(e)
	}
	if report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || !report.DiagnosticsTruncated || len(report.Findings) != 0 {
		t.Fatalf("lost authoritative failure %+v", report)
	}
	for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetDepth} {
		if used, _ := account.Usage(kind); used != 0 {
			t.Fatalf("leaked %s: %d", kind, used)
		}
	}
}

func TestLocalClosedScalarFailureSurvivesLaterInterruption(t *testing.T) {
	schema := localSchema(t)
	p, e := NewScalarOneOfPlan([]Atomic{localString("equals")})
	rule := localBinding(t, "local.literal", localUse(t, schema, "root", "relation", false).Declaration, p, e)
	engine := localEngine(t, schema, TargetSource, rule)
	ctx := context.Background()
	s, e := engine.Begin(ctx, RunOptions{DocumentID: "doc", Profile: "local.test"})
	if e != nil {
		t.Fatal(e)
	}
	h := schemaPendingHeader()
	h.Location.DocumentID = "doc"
	if e = s.Open(ctx, h); e != nil {
		t.Fatal(e)
	}
	h.ID = 2
	h.Parent = 1
	h.Name.Local = "relation"
	h.Location.Path = "/root/relation"
	if e = s.Open(ctx, h); e != nil {
		t.Fatal(e)
	}
	end := schemaLexemeEnd("bad")
	end.ID = 2
	if e = s.Close(ctx, end); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(ctx, schemaLexemeEnd("")); e != nil {
		t.Fatal(e)
	}
	if d, ok := s.lookupDecision(decisionID{rule.key, 2}); !ok || d.Outcome != OutcomeFail {
		t.Errorf("closed local failure was deferred %+v", d)
	}
	interruption := errors.New("source tail interrupted")
	r, e := s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: interruption})
	if !errors.Is(e, interruption) || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || r.AssessmentComplete {
		t.Fatalf("lost established local result %+v %v", r, e)
	}
}
