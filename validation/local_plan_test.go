package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"testing"
)

func localDecl(n string) DeclarationID {
	return DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "local.owner", Occurrence: n}
}
func localAttr(n string) AttributeSelector {
	return AttributeSelector{Name: QName{Local: n}, Declaration: localDecl("@" + n)}
}
func localString(v string) Atomic { return Atomic{Kind: AtomicString, Text: v} }
func localInteger(t *testing.T, v int64) Atomic {
	t.Helper()
	x, e := NewInteger(big.NewInt(v))
	if e != nil {
		t.Fatal(e)
	}
	return Atomic{Kind: AtomicInteger, Integer: x}
}
func localDecimal(t *testing.T, v string) Decimal {
	t.Helper()
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		t.Fatal(v)
	}
	d, e := NewDecimal(r)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func localNode(id, parent NodeID, d string) *factNode {
	h := factHeader(id, parent, d, d)
	h.Declaration.Value = localDecl(d)
	return &factNode{header: testRetainedHeader(h), end: testRetainedEnd(factEnd(id)), closed: true}
}
func localSetAttr(n *factNode, name string, v Atomic) {
	a := localAttr(name)
	n.attributes = append(n.attributes, AttributeFact{Owner: n.header.ID, Name: a.Name, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: a.Declaration}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Value: Evidence[Atomic]{State: EvidenceKnown, Value: v}}, Location: n.header.Location})
}
func localRun(t *testing.T, p Plan, root, n *factNode, nodes ...*factNode) Decision {
	t.Helper()
	a, e := NewBudgetAccount(Budgets{})
	if e != nil {
		t.Fatal(e)
	}
	s := &Session{account: a, documentID: "doc", root: root, nodes: map[NodeID]*factNode{}, inputComplete: true}
	for _, v := range append([]*factNode{root}, nodes...) {
		s.nodes[v.header.ID] = v
		s.sequence = append(s.sequence, v.header.ID)
	}
	d, pending, e := s.evaluatePlan(context.Background(), n, RuleDefinition{Key: RuleKey{ID: "local.test", Revision: 1}}, p, true)
	if e != nil || pending {
		t.Fatalf("evaluate %+v pending=%v err=%v", d, pending, e)
	}
	return d
}
func TestLocalScalarAndPresentAttributePlans(t *testing.T) {
	p, e := NewScalarOneOfPlan([]Atomic{localString(""), localString("equals")})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"", "equals", " ", " equals ", "Equals"} {
		n := localNode(1, 0, "relation")
		n.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceKnown, Value: localString(v)})
		want := OutcomeFail
		if v == "" || v == "equals" {
			want = OutcomePass
		}
		if d := localRun(t, p, n, n); d.Outcome != want {
			t.Errorf("%q: %+v", v, d)
		}
	}
	q, e := NewPresentAttributeValuePlan(localAttr("dacapo"), []Atomic{localString("yes")})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"absent", "yes", "no"} {
		n := localNode(1, 0, "sound")
		if v != "absent" {
			localSetAttr(n, "dacapo", localString(v))
		}
		want := OutcomeFail
		if v == "absent" {
			want = OutcomeNotApplicable
		}
		if v == "yes" {
			want = OutcomePass
		}
		if d := localRun(t, q, n, n); d.Outcome != want {
			t.Errorf("%s: %+v", v, d)
		}
	}
}
func TestLocalPedalUsesExactValidatedUnionValue(t *testing.T) {
	p, e := NewAttributeNumberOrTokensPlan(localAttr("pedal"), []string{"yes", "no"}, localDecimal(t, "0"), localDecimal(t, "100"))
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"yes", "no", "0", "-0", "100", "37.5", "100.0000000000000000000001", "-0.0000000000000000000001"} {
		n := localNode(1, 0, "sound")
		a := localString(v)
		if v != "yes" && v != "no" {
			a = Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, v)}
		}
		localSetAttr(n, "pedal", a)
		want := OutcomePass
		if v == "100.0000000000000000000001" || v == "-0.0000000000000000000001" {
			want = OutcomeFail
		}
		if d := localRun(t, p, n, n); d.Outcome != want {
			t.Errorf("%s: %+v", v, d)
		}
	}
}
func TestLocalBeamDistinctAndIncomplete(t *testing.T) {
	p, e := NewDistinctChildAttributePlan([]DeclarationID{localDecl("beam")}, localAttr("number"))
	if e != nil {
		t.Fatal(e)
	}
	for _, values := range [][]int64{{1, 2}, {7, 8}, {1, 3}, {2, 2}} {
		n := localNode(1, 0, "note")
		for i, v := range values {
			c := localNode(NodeID(i+2), 1, "beam")
			localSetAttr(c, "number", localInteger(t, v))
			n.children = append(n.children, c)
		}
		want := OutcomePass
		if values[0] == values[1] {
			want = OutcomeFail
		}
		if d := localRun(t, p, n, n, n.children...); d.Outcome != want {
			t.Errorf("%v: %+v", values, d)
		}
	}
	n := localNode(1, 0, "note")
	n.end.ChildrenComplete = testEvidence(Evidence[bool]{State: EvidenceUnknown, Reason: "partial"})
	if d := localRun(t, p, n, n); d.Outcome != OutcomeUnknown {
		t.Fatal(d)
	}
}
func TestLocalPresenceImplicationAndParentScope(t *testing.T) {
	slur := localDecl("slur")
	p, e := NewPresenceImpliesAttributePlan([]AttributeSelector{localAttr("x2"), localAttr("y2"), localAttr("offset2")}, Selector{Declaration: &slur}, localAttr("type"), localString("continue"))
	if e != nil {
		t.Fatal(e)
	}
	for _, owner := range []string{"slur", "tied"} {
		for _, kind := range []string{"start", "stop", "continue"} {
			for _, trigger := range []string{"x2", "y2", "offset2"} {
				n := localNode(1, 0, owner)
				localSetAttr(n, "type", localString(kind))
				localSetAttr(n, trigger, localInteger(t, 0))
				want := OutcomeFail
				if owner == "slur" && kind == "continue" {
					want = OutcomePass
				}
				if d := localRun(t, p, n, n); d.Outcome != want {
					t.Errorf("%s/%s/%s: %+v", owner, kind, trigger, d)
				}
			}
		}
	}
	parent := localNode(1, 0, "key")
	oct := localNode(2, 1, "octave")
	cancel := localNode(3, 1, "cancel")
	parent.children = []*factNode{oct, cancel}
	localSetAttr(oct, "cancel", localString("yes"))
	q, e := NewAttributeImpliesRelatedChildPlan(localAttr("cancel"), localString("yes"), RelationParent, []DeclarationID{localDecl("cancel")})
	if e != nil {
		t.Fatal(e)
	}
	if d := localRun(t, q, parent, oct, oct, cancel); d.Outcome != OutcomePass {
		t.Fatal(d)
	}
	parent.children = []*factNode{oct}
	if d := localRun(t, q, parent, oct, oct, cancel); d.Outcome != OutcomeFail {
		t.Fatal(d)
	}
	parent.end.ChildrenComplete = testEvidence(Evidence[bool]{State: EvidenceUnknown, Reason: "partial"})
	if d := localRun(t, q, parent, oct, oct, cancel); d.Outcome != OutcomeUnknown {
		t.Fatal(d)
	}
}
func TestLocalDocumentMarkerAndExactChildValues(t *testing.T) {
	root := localNode(1, 0, "root")
	marker := localNode(2, 1, "concert")
	tr := localNode(3, 1, "transpose")
	ch := localNode(4, 3, "chromatic")
	ch.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "0")}})
	root.children = []*factNode{marker, tr}
	tr.children = []*factNode{ch}
	p, e := NewDocumentMarkerValuesPlan([]DeclarationID{localDecl("concert")}, []ChildValueRequirement{{Child: localDecl("chromatic"), Expected: Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "0")}}, {Child: localDecl("diatonic"), Expected: localInteger(t, 0), AllowAbsent: true}})
	if e != nil {
		t.Fatal(e)
	}
	if d := localRun(t, p, root, tr, marker, tr, ch); d.Outcome != OutcomePass {
		t.Fatal(d)
	}
	ch.end.Scalar.Value.Value.Decimal = localDecimal(t, "2")
	if d := localRun(t, p, root, tr, marker, tr, ch); d.Outcome != OutcomeFail {
		t.Fatal(d)
	}
	root.children = []*factNode{tr}
	if d := localRun(t, p, root, tr, tr, ch); d.Outcome != OutcomeNotApplicable {
		t.Fatal(d)
	}
	q, e := NewDocumentChildPresencePlan([]DeclarationID{localDecl("concert")})
	if e != nil {
		t.Fatal(e)
	}
	if d := localRun(t, q, root, tr, tr, ch); d.Outcome != OutcomeFail {
		t.Fatal(d)
	}
	root.end.ChildrenComplete = testEvidence(Evidence[bool]{State: EvidenceUnknown, Reason: "partial"})
	if d := localRun(t, q, root, tr, tr, ch); d.Outcome != OutcomeUnknown {
		t.Fatal(d)
	}
}

func TestLocalConstructorsRejectUnsafeOrInexactParameters(t *testing.T) {
	tests := []struct {
		name string
		make func() (Plan, error)
	}{
		{"empty choices", func() (Plan, error) { return NewScalarOneOfPlan(nil) }},
		{"mixed choices", func() (Plan, error) { return NewScalarOneOfPlan([]Atomic{localString("a"), localInteger(t, 1)}) }},
		{"inexact attribute", func() (Plan, error) {
			return NewPresentAttributeValuePlan(AttributeSelector{Name: QName{Local: "x"}}, []Atomic{localString("a")})
		}},
		{"too many choices", func() (Plan, error) { return NewScalarOneOfPlan(make([]Atomic, 4097)) }},
		{"too large value", func() (Plan, error) { return NewScalarOneOfPlan([]Atomic{localString(string(make([]byte, 4<<20)))}) }},
		{"duplicate children", func() (Plan, error) {
			return NewDistinctChildAttributePlan([]DeclarationID{localDecl("x"), localDecl("x")}, localAttr("id"))
		}},
		{"backward bounds", func() (Plan, error) {
			return NewAttributeNumberOrTokensPlan(localAttr("p"), nil, localDecimal(t, "100"), localDecimal(t, "0"))
		}},
		{"zero decimal", func() (Plan, error) {
			return NewAttributeNumberOrTokensPlan(localAttr("p"), nil, Decimal{}, localDecimal(t, "0"))
		}},
		{"unimplemented relation", func() (Plan, error) {
			return NewAttributeImpliesRelatedChildPlan(localAttr("x"), localString("yes"), RelationMusicalPosition, []DeclarationID{localDecl("x")})
		}},
		{"empty marker values", func() (Plan, error) { return NewDocumentMarkerValuesPlan([]DeclarationID{localDecl("marker")}, nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, e := tt.make(); e == nil {
				t.Fatal("accepted invalid parameters")
			}
		})
	}
}
func TestLocalPlanCopyDigestAndInvalidPrerequisites(t *testing.T) {
	values := []Atomic{localString("equals"), localString("")}
	p, e := NewScalarOneOfPlan(values)
	if e != nil {
		t.Fatal(e)
	}
	before := fmt.Sprint(p.digestValue())
	values[0].Text = "changed"
	if before != fmt.Sprint(p.digestValue()) {
		t.Fatal("retained choices")
	}
	q, e := NewScalarOneOfPlan([]Atomic{localString("other")})
	if e != nil {
		t.Fatal(e)
	}
	if before == fmt.Sprint(q.digestValue()) {
		t.Fatal("digest omitted values")
	}
	n := localNode(1, 0, "relation")
	cause := &DecisionRef{Rule: RuleKey{ID: "schema.scalar", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: 1}}
	n.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceInvalid, Reason: "bad-scalar", Cause: cause})
	d := localRun(t, p, n, n)
	if d.Outcome != OutcomeBlocked || d.Cause == nil || *d.Cause != *cause {
		t.Fatalf("lost cause %+v", d)
	}
	n.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceUnsupported, Reason: "unknown-domain"})
	if d := localRun(t, p, n, n); d.Outcome != OutcomeUnsupported {
		t.Fatal(d)
	}
}
func TestLocalPlanScopeAndTargetBoundary(t *testing.T) {
	p, e := NewScalarOneOfPlan([]Atomic{localString("x")})
	if e != nil {
		t.Fatal(e)
	}
	d := localDecl("x")
	r := RuleDefinition{Scope: ScopeNode, Category: CategoryNormative, Class: ClassMandatory, Role: RolePredicate, Targets: []Target{TargetSource, TargetModel}, Selector: Selector{Declaration: &d}}
	if e := p.validateFor(r); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*RuleDefinition){func(r *RuleDefinition) { r.Scope = ScopeDocument }, func(r *RuleDefinition) { r.Category = CategoryModel }, func(r *RuleDefinition) { r.Targets = []Target{TargetPackage} }, func(r *RuleDefinition) { r.Role = RoleContextProvider }} {
		bad := r
		change(&bad)
		if e := p.validateFor(bad); e == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}
func TestLocalIncompleteAntecedentAndExactDeclarations(t *testing.T) {
	slur := localDecl("slur")
	p, e := NewPresenceImpliesAttributePlan([]AttributeSelector{localAttr("x2")}, Selector{Declaration: &slur}, localAttr("type"), localString("continue"))
	if e != nil {
		t.Fatal(e)
	}
	n := localNode(1, 0, "slur")
	n.end.AttributesComplete = testEvidence(Evidence[bool]{State: EvidenceUnknown, Reason: "unknown-presence"})
	if d := localRun(t, p, n, n); d.Outcome != OutcomeUnknown {
		t.Fatal(d)
	}
	n.end.AttributesComplete = testEvidence(Evidence[bool]{State: EvidenceKnown, Value: true})
	if d := localRun(t, p, n, n); d.Outcome != OutcomeNotApplicable {
		t.Fatal(d)
	}
	parent := localNode(1, 0, "key")
	oct := localNode(2, 1, "octave")
	other := localNode(3, 1, "cancel")
	other.header.Declaration.Value.Assembly = AssemblyOpus
	parent.children = []*factNode{oct, other}
	localSetAttr(oct, "cancel", localString("yes"))
	q, e := NewAttributeImpliesRelatedChildPlan(localAttr("cancel"), localString("yes"), RelationParent, []DeclarationID{localDecl("cancel")})
	if e != nil {
		t.Fatal(e)
	}
	if d := localRun(t, q, parent, oct, oct, other); d.Outcome != OutcomeFail {
		t.Fatalf("homonym satisfied exact declaration %+v", d)
	}
}

func TestLocalDocumentCacheBudgetAndCancellation(t *testing.T) {
	p, e := NewDocumentChildPresencePlan([]DeclarationID{localDecl("marker")})
	if e != nil {
		t.Fatal(e)
	}
	root := localNode(1, 0, "root")
	marker := localNode(2, 1, "marker")
	root.children = []*factNode{marker}
	for _, limit := range []int64{383, 384} {
		a, _ := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
		s := &Session{account: a, root: root, inputComplete: true, nodes: map[NodeID]*factNode{1: root, 2: marker}, sequence: []NodeID{1, 2}}
		_, _, err := s.evaluateLocalPlan(context.Background(), root, p.local, true)
		if limit == 383 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("first excess accepted %v", err)
		}
		if limit == 384 && err != nil {
			t.Fatal(err)
		}
		if err == nil {
			before, _ := a.Usage(BudgetRuleSteps)
			_, _, err = s.evaluateLocalPlan(context.Background(), root, p.local, true)
			after, _ := a.Usage(BudgetRuleSteps)
			if err != nil || after-before != 1 {
				t.Fatalf("repeated document scan: steps %d -> %d err %v", before, after, err)
			}
		}
		s.cleanup()
		if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("buffer leak %d", used)
		}
	}
	a, _ := NewBudgetAccount(Budgets{})
	s := &Session{account: a, root: root, inputComplete: true, nodes: map[NodeID]*factNode{1: root, 2: marker}, sequence: []NodeID{1, 2}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e = s.evaluateLocalPlan(ctx, root, p.local, true); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation ignored %v", e)
	}
	a, _ = NewBudgetAccount(Budgets{MaxRuleSteps: 1})
	s.account = a
	if _, _, e = s.evaluateLocalPlan(context.Background(), root, p.local, true); !errors.Is(e, ErrResourceLimit) {
		t.Fatalf("work limit ignored %v", e)
	}
}

func TestLocalPlanCanonicalSetDigestAndQNameIdentity(t *testing.T) {
	p, e := NewScalarOneOfPlan([]Atomic{localString("equals"), localString("")})
	if e != nil {
		t.Fatal(e)
	}
	q, e := NewScalarOneOfPlan([]Atomic{localString(""), localString("equals")})
	if e != nil {
		t.Fatal(e)
	}
	left, _ := json.Marshal(p.digestValue())
	right, _ := json.Marshal(q.digestValue())
	if string(left) != string(right) {
		t.Errorf("set order changed digest %s != %s", left, right)
	}
	p, e = NewScalarOneOfPlan([]Atomic{{Kind: AtomicQName, Name: QName{Space: "a}b", Local: "c"}}})
	if e != nil {
		t.Fatal(e)
	}
	q, e = NewScalarOneOfPlan([]Atomic{{Kind: AtomicQName, Name: QName{Space: "a", Local: "b}c"}}})
	if e != nil {
		t.Fatal(e)
	}
	left, _ = json.Marshal(p.digestValue())
	right, _ = json.Marshal(q.digestValue())
	if string(left) == string(right) {
		t.Fatal("different QName identities collided")
	}
}

func TestLocalMarkerValuesIndependentConjuncts(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "schema.scalar", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: 3}}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, unavailableFirst := range []bool{false, true} {
			for _, knownValue := range []int64{0, 1} {
				t.Run(fmt.Sprintf("%s/first=%v/known=%d", state, unavailableFirst, knownValue), func(t *testing.T) {
					root := localNode(1, 0, "root")
					marker := localNode(2, 1, "marker")
					owner := localNode(3, 1, "owner")
					unknownName, knownName := "b-value", "a-value"
					if unavailableFirst {
						unknownName, knownName = knownName, unknownName
					}
					unknown := localNode(4, 3, unknownName)
					known := localNode(5, 3, knownName)
					root.children = []*factNode{marker, owner}
					owner.children = []*factNode{unknown, known}
					unknown.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: state, Reason: "unavailable-scalar"})
					if state == EvidenceInvalid {
						unknown.end.Scalar.Value.Cause = cause
					}
					known.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceKnown, Value: localInteger(t, knownValue)})
					p, e := NewDocumentMarkerValuesPlan([]DeclarationID{localDecl("marker")}, []ChildValueRequirement{{Child: unknown.header.Declaration.Value, Expected: localInteger(t, 0)}, {Child: known.header.Declaration.Value, Expected: localInteger(t, 0)}})
					if e != nil {
						t.Fatal(e)
					}
					d := localRun(t, p, root, owner, marker, owner, unknown, known)
					want := OutcomeFail
					if knownValue == 0 {
						want = localGap(*unknown.end.Scalar.Value).Outcome
					}
					if d.Outcome != want {
						t.Fatalf("independent conjunct lost: want=%s got=%+v", want, d)
					}
					if want == OutcomeBlocked && (d.Cause == nil || *d.Cause != *cause) {
						t.Fatalf("blocked cause changed %+v", d)
					}
				})
			}
		}
	}
}
func TestLocalDistinctIndependentDuplicateSurvivesUnavailableNumbers(t *testing.T) {
	p, e := NewDistinctChildAttributePlan([]DeclarationID{localDecl("beam")}, localAttr("number"))
	if e != nil {
		t.Fatal(e)
	}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for unavailableAt := 0; unavailableAt < 3; unavailableAt++ {
			for _, duplicate := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/at=%d/duplicate=%v", state, unavailableAt, duplicate), func(t *testing.T) {
					n := localNode(1, 0, "note")
					count := int64(0)
					var gap Evidence[Atomic]
					for i := 0; i < 3; i++ {
						ch := localNode(NodeID(i+2), 1, "beam")
						number := int64(2)
						if !duplicate {
							number += count
						}
						localSetAttr(ch, "number", localInteger(t, number))
						if i == unavailableAt {
							gap = Evidence[Atomic]{State: state, Reason: "number-unavailable"}
							if state == EvidenceInvalid {
								gap.Cause = &DecisionRef{Rule: RuleKey{ID: "schema.attribute", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: ch.header.ID}}
							}
							ch.attributes[0].Scalar.Value = gap
						} else {
							count++
						}
						n.children = append(n.children, ch)
					}
					d := localRun(t, p, n, n, n.children...)
					want := localGap(gap).Outcome
					if duplicate {
						want = OutcomeFail
					}
					if d.Outcome != want {
						t.Fatalf("independent uniqueness lost want=%s got=%+v", want, d)
					}
					if want == OutcomeBlocked && (d.Cause == nil || *d.Cause != *gap.Cause) {
						t.Fatalf("cause changed %+v", d)
					}
				})
			}
		}
	}
}
func TestLocalTriggerDisjunctionContinuesAfterUnknownPresence(t *testing.T) {
	owner := localDecl("slur")
	p, e := NewPresenceImpliesAttributePlan([]AttributeSelector{localAttr("a-control"), localAttr("b-control")}, Selector{Declaration: &owner}, localAttr("type"), localString("continue"))
	if e != nil {
		t.Fatal(e)
	}
	n := localNode(1, 0, "slur")
	localSetAttr(n, "a-control", localInteger(t, 0))
	n.attributes[0].Presence = Evidence[bool]{State: EvidenceUnknown, Reason: "unknown-first-trigger"}
	localSetAttr(n, "b-control", localInteger(t, 0))
	localSetAttr(n, "type", localString("start"))
	if d := localRun(t, p, n, n); d.Outcome != OutcomeFail {
		t.Fatalf("known activation and counterexample suppressed %+v", d)
	}
}
