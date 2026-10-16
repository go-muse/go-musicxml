package validation

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"
)

func TestContextReviewLayoutTypedCost(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	p := &LayoutContextResult{Scope: NodeRef{"d", 1}, Part: stateKnown(NodeRef{"d", 2}), Raw: stateKnown(localString("x")), Applicability: stateKnown(true)}
	q, err := cloneLayoutContext(context.Background(), s, p)
	if err != nil || q == nil {
		t.Fatalf("copy: %v", err)
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes); got > 1536 {
		t.Errorf("flat layout payload retained %d bytes, want at most 1536", got)
	}
	if got := rationalUsage(t, s, BudgetRuleSteps); got > 64 {
		t.Errorf("flat layout copy consumed %d steps, want at most 64", got)
	}
}

func TestContextReviewEmptyAttributeCacheIsCompact(t *testing.T) {
	s, b := independentContextFallbackFixture(t)
	defer s.cleanup()
	b.AttributeRoles = map[DeclarationID][]contextAttributeRole{}
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	before := s.retained
	for _, id := range []NodeID{2, 3} {
		if _, ok := r.attributeSelector(s.nodes[id], "unselected"); ok || r.err != nil {
			t.Fatalf("empty lookup: %v", r.err)
		}
	}
	if got := s.retained - before; got > 768 {
		t.Errorf("empty assessed-use caches retain %d bytes; want only compact binding sentinels", got)
	}
}

func TestContextReviewCatalogSharedBindingsCountedOnce(t *testing.T) {
	schema := localSchema(t)
	b := contextCoreBindings(t)
	b.bytes = 3 << 20
	plans := make([]PlanBinding, 40)
	for i := range plans {
		plans[i] = PlanBinding{ID: fmt.Sprintf("context-%d", i), Plan: Plan{kind: planContextLocal, context: &contextPlan{Schema: schema, Bindings: b, revision: contextSemanticRevision}}}
	}
	if err := catalogCheckBounds(nil, nil, plans); err != nil {
		t.Errorf("real shared immutable table counted once per plan: %v", err)
	}
	for i := range plans {
		duplicate := *b
		plans[i].Plan.context.Bindings = &duplicate
	}
	if err := catalogCheckBounds(nil, nil, plans); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("distinct retained tables escaped finite configuration cap: %v", err)
	}
	b.bytes = 1 << 20
	ownBytes := RuleID(strings.Repeat("o", 2<<20))
	for i := range plans {
		plans[i].Plan.context.Bindings = b
		plans[i].Plan.context.Provider.ID = ownBytes
	}
	if err := catalogCheckBounds(nil, nil, plans); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("per-plan bytes escaped sharing accounting: %v", err)
	}
}

func TestContextReviewRoleDispatchExactAndInterrupted(t *testing.T) {
	b := contextCoreBindings(t)
	n := localNode(1, 0, "beam")
	n.header.Declaration = testEvidence(stateKnown(b.Roles["core-beam"][0].Declaration))
	want := []contextRole{"core-all-beam", "core-beam"}
	cost := int64(1) + contextDeclarationBytes(n.header.Declaration.Value)
	for _, role := range want {
		cost += 1 + int64(len(role))
	}
	for _, limit := range []int64{cost - 1, cost} {
		s := rationalSession(t, Budgets{MaxRuleSteps: limit})
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		got := r.roles(n)
		if limit == cost {
			if r.err != nil || !slices.Equal(got, want) {
				t.Fatalf("exact dispatch: %v %v", got, r.err)
			}
		} else {
			if !errors.Is(r.err, ErrResourceLimit) || got != nil {
				t.Fatalf("one-under exposed roles: %v %v", got, r.err)
			}
			// A refused Account charge is not sticky; the reader must be.
			if got = r.roles(nil); got != nil || !errors.Is(r.err, ErrResourceLimit) {
				t.Fatal("first quota refusal reset")
			}
		}
		s.cleanup()
	}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceAbsent} {
		node := testCopyFactNode(n)
		node.header.Declaration.State = state
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		if len(r.roles(&node)) != 0 || r.err != nil {
			t.Fatal("unavailable declaration invented roles")
		}
	}
	for _, change := range []func(*DeclarationID){func(d *DeclarationID) { d.Assembly = AssemblyOpus }, func(d *DeclarationID) { d.OwnerOccurrence += "-different" }, func(d *DeclarationID) { d.Occurrence += "-different" }} {
		node := testCopyFactNode(n)
		change(&node.header.Declaration.Value)
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		if len(r.roles(&node)) != 0 || r.err != nil {
			t.Fatal("declaration identity contaminated")
		}
	}
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	r := &contextReader{ctx: probe, s: s, b: b}
	_ = r.roles(n)
	for at := 1; at <= probe.calls; at++ {
		r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
		if got := r.roles(n); got != nil || !errors.Is(r.err, context.Canceled) {
			t.Fatalf("poll %d: %v %v", at, got, r.err)
		}
	}
	if s.retained != 0 {
		t.Fatal("compiled role dispatch allocated per-node index")
	}
}

// Complete nodes resolve to the same actual type. Present required uses, omitted
// required uses, optional known absences, and incomplete enumeration deliberately
// expose different Run.Attributes tables. A type-only first-node cache is unsound.
func contextReviewSameTypeFixture(t *testing.T, limits ...Budgets) (*Session, *contextBindings) {
	t.Helper()
	xs := SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}
	child := &SchemaElement{Name: SchemaName{Local: "a"}, Type: SchemaTypeRef{Name: SchemaName{Local: "derived"}}}
	root := &SchemaElement{Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 4, Max: 4}, Element: child}}}}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{
		Types: map[SchemaName]*SchemaType{
			{Local: "A"}:       {Complex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "required"}, Type: xs, Use: SchemaAttributeRequired}, {Name: SchemaName{Local: "optional"}, Type: xs}}}},
			{Local: "derived"}: {Complex: &SchemaComplex{Form: SchemaComplexExtension, Base: &SchemaTypeRef{Name: SchemaName{Local: "A"}}}},
		},
		Elements: map[SchemaName]*SchemaElement{{Local: "root"}: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := compileContextBindingSpecs(schema, []contextBindingSpec{{Role: "required", Owner: "A", Name: "required", Type: "xs:string", Attribute: true, Count: 1}, {Role: "optional", Owner: "A", Name: "optional", Type: "xs:string", Attribute: true, Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	opts := RunOptions{DocumentID: "source", Profile: "source-schema-test"}
	if len(limits) != 0 {
		opts.Budgets = limits[0]
	}
	s, err := sourceSchemaEngineFor(t, schema, XSDCheckAttributes, XSDCheckContent).Begin(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Open(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		h := schemaPendingHeader()
		h.ID = NodeID(i + 2)
		h.Parent = 1
		h.Name.Local = "a"
		if err = s.Open(context.Background(), h); err != nil {
			t.Fatal(err)
		}
		if i == 0 || i == 3 {
			a := AttributeFact{Owner: h.ID, Name: QName{Local: "required"}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Presence: stateKnown(true), Scalar: Scalar{Lexeme: stateKnown("value"), Value: stateUnknown[Atomic]("schema-binding-pending")}, Location: Location{Target: TargetSource, DocumentID: "source", Path: "/root/a/@required"}}
			if err = s.Attribute(context.Background(), a); err != nil {
				t.Fatal(err)
			}
		}
		end := schemaLexemeEnd("")
		end.ID = h.ID
		end.AttributesComplete.Value = i != 2
		if err = s.Close(context.Background(), end); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Close(context.Background(), schemaLexemeEnd("")); err != nil {
		t.Fatal(err)
	}
	return s, b
}
func TestContextReviewSameTypePreservesAssessedUses(t *testing.T) {
	s, b := contextReviewSameTypeFixture(t)
	defer s.cleanup()
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	actual, ok := s.schema.run.ActualType(s.schema.nodes[2].binding)
	if !ok {
		t.Fatal("actual type unavailable")
	}
	var tables []*contextAttributeTable
	for _, tc := range []struct {
		id                 NodeID
		required, optional bool
	}{{2, true, true}, {3, false, true}, {4, false, false}, {5, true, true}} {
		next, ok := s.schema.run.ActualType(s.schema.nodes[tc.id].binding)
		if tc.id == 4 {
			if ok {
				t.Fatal("incomplete attribute axis unexpectedly selected actual type")
			}
		} else if !ok || next != actual {
			t.Fatalf("node %d fixture types differ", tc.id)
		}
		for role, want := range map[contextRole]bool{"required": tc.required, "optional": tc.optional} {
			_, got := r.attributeSelector(s.nodes[tc.id], role)
			if got != want || r.err != nil {
				t.Fatalf("node%d %s got%t want%t: %v", tc.id, role, got, want, r.err)
			}
		}
		table := r.attributeTable(s.nodes[tc.id])
		tables = append(tables, table)
		if got := r.attributeRoles(s.nodes[tc.id]); len(got) != btoi(tc.required)+btoi(tc.optional) {
			t.Fatalf("node%d owner dispatch leaked another use: %v", tc.id, got)
		}
	}
	if tables[0] != tables[3] || tables[0] == tables[1] || tables[2] != nil {
		t.Fatal("exact table interning or empty sentinel incorrect")
	}
}
func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestContextReviewLayoutTypedReferenceAndShape(t *testing.T) {
	// A new field requires explicit copy/measurement review rather than silently
	// disappearing from this typed resource path.
	typ := reflect.TypeFor[LayoutContextResult]()
	fields := []string{"Scope", "Origin", "Part", "Page", "System", "Staff", "Conversion", "Basis", "Raw", "Applicability", "Unit", "Provenance"}
	if typ.NumField() != len(fields) {
		t.Fatal("layout shape changed; update typed measure/copy and this full-field reference test")
	}
	for i, name := range fields {
		if typ.Field(i).Name != name {
			t.Fatalf("layout field %d changed", i)
		}
	}
	cause := &DecisionRef{Rule: RuleKey{"causal-rule", 9}, Subject: NodeRef{"causal-doc", 7}}
	p := LayoutContextResult{Scope: NodeRef{"scope-doc", 1}, Origin: NodeRef{"origin-doc", 2}, Part: Evidence[NodeRef]{State: EvidenceInvalid, Value: NodeRef{"part-doc", 3}, Reason: "part-reason", Cause: cause}, Page: Evidence[NodeRef]{State: EvidenceUnknown, Value: NodeRef{"page-doc", 4}, Reason: "page-reason", Cause: cause}, System: Evidence[NodeRef]{State: EvidenceUnsupported, Value: NodeRef{"system-doc", 5}, Reason: "system-reason", Cause: cause}, Staff: Evidence[Integer]{State: EvidenceKnown, Value: stateInt(t, 3), Reason: "staff-reason", Cause: cause}, Conversion: Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 4, 3), Reason: "conversion-reason", Cause: cause}, Basis: "coordinate-basis", Raw: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "text", Integer: stateInt(t, 12), Decimal: localDecimal(t, "2.5"), Name: QName{Space: "namespace", Local: "name"}}, Reason: "raw-reason", Cause: cause}, Applicability: Evidence[bool]{State: EvidenceKnown, Value: true, Reason: "applicable-reason", Cause: cause}, Unit: ContextUnitTenths, Provenance: ContextInherited}
	want := stateClone(p)
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	got, err := cloneLayoutContext(context.Background(), s, &p)
	if err != nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("typed/reference mismatch: %v", err)
	}
	if got.Raw.Cause == cause || got.Part.Cause == cause || got.Raw.Cause == got.Part.Cause {
		t.Fatal("mutable causes alias")
	}
	// Numeric wrappers intentionally share immutable storage; public Value copies
	// must remain independent even after mutating the returned accessors.
	got.Staff.Value.Value().SetInt64(99)
	got.Conversion.Value.Value().SetInt64(99)
	if got.Staff.Value.String() != "3" || got.Conversion.Value.String() != "4/3" {
		t.Fatal("numeric wrapper ownership changed")
	}
	var checkStrings func(reflect.Value, reflect.Value)
	checkStrings = func(a, b reflect.Value) {
		if a.Type() == reflect.TypeFor[Integer]() || a.Type() == reflect.TypeFor[Decimal]() || a.Type() == reflect.TypeFor[Rational]() {
			return
		}
		switch a.Kind() {
		case reflect.Struct:
			for i := 0; i < a.NumField(); i++ {
				checkStrings(a.Field(i), b.Field(i))
			}
		case reflect.Pointer:
			if !a.IsNil() {
				checkStrings(a.Elem(), b.Elem())
			}
		case reflect.String:
			if a.Len() != 0 && unsafe.StringData(a.String()) == unsafe.StringData(b.String()) {
				t.Fatalf("owned string aliases %s", a.Type())
			}
		}
	}
	// Evidence states are canonical immutable constants, which cloneEvidence may
	// safely intern, unlike all free-form strings. Compare those separately.
	checkStrings(reflect.ValueOf(p.Raw.Value), reflect.ValueOf(got.Raw.Value))
	if unsafe.StringData(p.Basis) == unsafe.StringData(got.Basis) || unsafe.StringData(p.Scope.DocumentID) == unsafe.StringData(got.Scope.DocumentID) {
		t.Fatal("layout strings alias")
	}
	cause.Rule.ID = "changed"
	if !reflect.DeepEqual(*got, want) {
		t.Fatal("input mutation affected projection")
	}
}

func TestContextReviewLayoutExactBudgetAndCancellation(t *testing.T) {
	p := &LayoutContextResult{Basis: strings.Repeat("long-basis", 400), Raw: stateKnown(localString("value")), Part: Evidence[NodeRef]{State: EvidenceAbsent}}
	ample := rationalSession(t, Budgets{})
	_, err := cloneLayoutContext(context.Background(), ample, p)
	if err != nil {
		t.Fatal(err)
	}
	size, work := ample.retained, rationalUsage(t, ample, BudgetRuleSteps)
	ample.cleanup()
	for _, tc := range []struct {
		kind  BudgetKind
		limit int64
	}{{BudgetBufferedBytes, size - 1}, {BudgetRuleSteps, work - 1}, {BudgetBufferedBytes, size}, {BudgetRuleSteps, work}} {
		limits := Budgets{}
		if tc.kind == BudgetBufferedBytes {
			limits.MaxBufferedBytes = tc.limit
		} else {
			limits.MaxRuleSteps = tc.limit
		}
		s := rationalSession(t, limits)
		q, err := cloneLayoutContext(context.Background(), s, p)
		exact := tc.kind == BudgetBufferedBytes && tc.limit == size || tc.kind == BudgetRuleSteps && tc.limit == work
		if exact {
			if err != nil || q == nil {
				t.Fatalf("exact %s: %v", tc.kind, err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || q != nil || s.retained != 0 {
			t.Fatalf("one-under %s published allocation: %v", tc.kind, err)
		}
		s.cleanup()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	if q, err := cloneLayoutContext(ctx, s, p); q != nil || !errors.Is(err, context.Canceled) || s.retained != 0 {
		t.Fatalf("cancel: %v", err)
	}
}

func TestContextReviewAttributeCacheAdmission(t *testing.T) {
	s, b := contextReviewSameTypeFixture(t)
	before := rationalUsage(t, s, BudgetRuleSteps)
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	r := &contextReader{ctx: probe, s: s, b: b}
	if _, ok := r.attributeSelector(s.nodes[2], "required"); !ok || r.err != nil {
		t.Fatal(r.err)
	}
	total := rationalUsage(t, s, BudgetRuleSteps)
	t.Logf("fallback admission work=%d polls=%d", total-before, probe.calls)
	for at := 1; at <= probe.calls; at++ {
		reader := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
		retained := s.retained
		_, _ = reader.attributeSelector(s.nodes[2], "required")
		if !errors.Is(reader.err, context.Canceled) || len(reader.attributeCache) != 0 || len(reader.attributeIntern) != 0 || s.retained != retained {
			t.Fatalf("poll%d published partial cache: err=%v retained=%d/%d", at, reader.err, s.retained, retained)
		}
		reader.ctx = context.Background()
		_, _ = reader.attributeSelector(s.nodes[2], "required")
		if !errors.Is(reader.err, context.Canceled) {
			t.Fatal("cache interruption reset")
		}
	}
	s.cleanup()
	for _, limit := range []int64{total - 1, total} {
		s, b := contextReviewSameTypeFixture(t, Budgets{MaxRuleSteps: limit})
		retained := s.retained
		reader := &contextReader{ctx: context.Background(), s: s, b: b}
		_, ok := reader.attributeSelector(s.nodes[2], "required")
		if limit == total {
			if !ok || reader.err != nil {
				t.Fatalf("exact cache work: %v", reader.err)
			}
		} else {
			if ok || !errors.Is(reader.err, ErrResourceLimit) || len(reader.attributeCache) != 0 || len(reader.attributeIntern) != 0 || s.retained != retained {
				t.Fatalf("one-under admitted cache: %v", reader.err)
			}
			// Demonstrate the account itself accepts a smaller charge after refusal.
			if err := s.account.Charge(BudgetRuleSteps, 0); err != nil {
				t.Fatal(err)
			}
			_, _ = reader.attributeSelector(s.nodes[2], "optional")
			if !errors.Is(reader.err, ErrResourceLimit) {
				t.Fatal("reader lost non-sticky quota interruption")
			}
		}
		s.cleanup()
	}
}

func TestContextReviewBindingHashCanonicalAndComplete(t *testing.T) {
	schema := localSchema(t)
	specs := []contextBindingSpec{{Role: "beam", Owner: "note", Name: "beam", Type: "beam", Count: 1}, {Role: "number", Owner: "beam", Name: "number", Type: "xs:integer", Attribute: true, Count: 1}}
	a, err := compileContextBindingSpecs(schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(specs)
	b, err := compileContextBindingSpecs(schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.digest()) != 64 || a.digest() != b.digest() {
		t.Fatal("canonical binding hash depends on spec/map order")
	}
	for _, change := range []func(*contextBindings){
		func(v *contextBindings) { v.assembly = AssemblyOpus },
		func(v *contextBindings) { v.requiredSchemaDigest += "other" },
		func(v *contextBindings) { v.Roles["beam-other"] = v.Roles["beam"]; delete(v.Roles, "beam") },
		func(v *contextBindings) { v.Roles["beam"][0].Declaration.Assembly = AssemblyOpus },
		func(v *contextBindings) { v.Roles["beam"][0].Declaration.Occurrence += "other" },
		func(v *contextBindings) { v.Roles["beam"][0].Declaration.OwnerOccurrence += "other" },
		func(v *contextBindings) { v.Roles["beam"][0].OwnerType.Name.Local += "other" },
		func(v *contextBindings) { v.Roles["beam"][0].DeclaredType.Name.Local += "other" },
		func(v *contextBindings) { v.Roles["beam"][0].Attribute = true },
	} {
		c, err := compileContextBindingSpecs(schema, specs)
		if err != nil {
			t.Fatal(err)
		}
		change(c)
		if c.hash() == a.digest() {
			t.Fatal("binding hash omitted exact schema/use metadata")
		}
	}
	if allocations := testing.AllocsPerRun(100, func() { _ = a.digest() }); allocations != 0 {
		t.Fatalf("cached binding digest allocated %g times", allocations)
	}
}

func TestContextReviewRoleSortBoundedAndInterrupted(t *testing.T) {
	if unsafe.Sizeof(contextRole("")) > 128 || unsafe.Sizeof(contextAttributeRole{}) > 128 {
		t.Fatal("role element shape outgrew conservative scratch allowance")
	}
	input := func(count int) []contextRole {
		out := make([]contextRole, count)
		for i := range out {
			out[i] = contextRole(fmt.Sprintf("role-%04d", count-i))
		}
		return out
	}
	run := func(count int) int64 {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		r := &contextReader{ctx: context.Background(), s: s}
		values := input(count)
		if !contextSortRoles(r, values, func(v contextRole) contextRole { return v }) || !slices.IsSorted(values) || r.err != nil {
			t.Fatal(r.err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("sort scratch retained")
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := run(100), run(1000)
	// Sorting a finite vocabulary is n log n, unlike per-document linear scans.
	// Compare normalized growth, with factor-two headroom for merge boundaries.
	if large*100*int64(bits.Len(100)) > 2*small*1000*int64(bits.Len(1000)) {
		t.Fatalf("role sort exceeds n log n growth: %d -> %d", small, large)
	}
	_ = run(4096)
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	r := &contextReader{ctx: probe, s: s}
	if !contextSortRoles(r, input(8), func(v contextRole) contextRole { return v }) {
		t.Fatal(r.err)
	}
	for at := 1; at <= probe.calls; at++ {
		r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s}
		if contextSortRoles(r, input(8), func(v contextRole) contextRole { return v }) || !errors.Is(r.err, context.Canceled) || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("sort cancellation poll%d: %v", at, r.err)
		}
	}
	cost := run(8)
	limited := rationalSession(t, Budgets{MaxRuleSteps: cost - 1})
	defer limited.cleanup()
	r = &contextReader{ctx: context.Background(), s: limited}
	if contextSortRoles(r, input(8), func(v contextRole) contextRole { return v }) || !errors.Is(r.err, ErrResourceLimit) || rationalUsage(t, limited, BudgetBufferedBytes) != 0 {
		t.Fatalf("sort one-under: %v", r.err)
	}
	r = &contextReader{ctx: context.Background(), s: s}
	if contextSortRoles(r, input(4097), func(v contextRole) contextRole { return v }) || !errors.Is(r.err, ErrInvalidConfiguration) {
		t.Fatal("role vocabulary bound not enforced")
	}
	t.Logf("100/1000-role sort work %d/%d", small, large)
}

func TestContextReviewAttributeCacheBufferedBoundary(t *testing.T) {
	source, b := contextReviewSameTypeFixture(t)
	defer source.cleanup()
	run := func(limit int64) (int64, error) {
		// This reader borrows the already-final, immutable assessed uses while its
		// own cache account isolates the cache's peak live allocation boundary.
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		s.schema = source.schema
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		_, ok := r.attributeSelector(source.nodes[2], "required")
		if r.err != nil && (ok || len(r.attributeCache) != 0 || len(r.attributeIntern) != 0 || s.retained != 0 || rationalUsage(t, s, BudgetBufferedBytes) != 0) {
			t.Fatalf("failed cache admission retained partial state: %v", r.err)
		}
		used, err := s.retained, r.err
		s.schema = nil
		s.cleanup()
		return used, err
	}
	low, high := int64(1), int64(16384)
	if _, err := run(high); err != nil {
		t.Fatal(err)
	}
	for low < high {
		mid := low + (high-low)/2
		if _, err := run(mid); err == nil {
			high = mid
		} else if errors.Is(err, ErrResourceLimit) {
			low = mid + 1
		} else {
			t.Fatal(err)
		}
	}
	retained, err := run(low)
	if err != nil || retained == 0 {
		t.Fatalf("exact cache buffer: %v", err)
	}
	if _, err := run(low - 1); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("one-under cache buffer: %v", err)
	}
	t.Logf("exact cache peak=%d retained=%d", low, retained)
}
