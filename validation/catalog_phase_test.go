package validation

import (
	"errors"
	"reflect"
	"testing"
)

func catalogIdentityPhaseFixture(t *testing.T) ([]RuleDefinition, []Profile, []PlanBinding) {
	t.Helper()
	slot := AttributeSelector{Name: QName{Local: "id"}, Declaration: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "id"}}
	declaration, _ := NewIdentityDeclarationPlan("index", slot)
	reference, _ := NewIdentityReferencePlan("index", slot)
	prerequisite, _ := NewChildCountPlan([]DeclarationID{slot.Declaration}, 0, -1)
	r, p, b := catalogScheduleFixture(t, []PlanBinding{
		{ID: "a-declaration", Plan: declaration},
		{ID: "b-declaration", Plan: declaration},
		{ID: "c-reference", Plan: reference},
		{ID: "z-prerequisite", Plan: prerequisite},
	})
	r[1].Requires = []Dependency{{Kind: DependencyRule, Rule: r[3].Key, Relation: RelationDocument}}
	return r, p, b
}

func catalogPhasePositions(t *testing.T, c *CompiledCatalog, id ProfileID) map[catalogPhase]int {
	t.Helper()
	out := make(map[catalogPhase]int)
	for i, phase := range c.phases[id] {
		if _, exists := out[phase]; exists {
			t.Fatalf("duplicate phase: %+v", phase)
		}
		out[phase] = i
	}
	return out
}

func TestCatalogIdentityRegistrationBarrierPrecedesEveryVerdict(t *testing.T) {
	r, p, b := catalogIdentityPhaseFixture(t)
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	pos := catalogPhasePositions(t, c, p[0].ID)
	if len(pos) != 6 {
		t.Fatalf("phase count=%d, want 2 registrations and 4 evaluations", len(pos))
	}
	for _, declaration := range r[:2] {
		registration := catalogPhase{rule: declaration.Key, registration: true}
		registered, ok := pos[registration]
		if !ok {
			t.Fatalf("missing registration %v", declaration.Key)
		}
		for _, verdict := range r[:3] {
			evaluated, ok := pos[catalogPhase{rule: verdict.Key}]
			if !ok || registered >= evaluated {
				t.Errorf("%v must register before %v verdict: phases=%v", declaration.Key, verdict.Key, c.phases[p[0].ID])
			}
		}
	}
	if pos[catalogPhase{rule: r[3].Key}] >= pos[catalogPhase{rule: r[1].Key, registration: true}] {
		t.Fatal("deferred declaration registration bypassed prerequisite verdict")
	}
	schedule, _ := c.Schedule(p[0].ID)
	projection := make([]RuleKey, 0, len(schedule))
	for _, phase := range c.phases[p[0].ID] {
		if !phase.registration {
			projection = append(projection, phase.rule)
		}
	}
	if len(schedule) != len(r) || !reflect.DeepEqual(schedule, projection) {
		t.Fatalf("Schedule must contain final-evaluation projection exactly once: got %v, projection %v", schedule, projection)
	}
	for i := range r {
		got, _ := c.Rule(r[i].Key)
		if !reflect.DeepEqual(got.Requires, r[i].Requires) {
			t.Fatal("phase inference modified semantic dependencies")
		}
	}
}

func TestCatalogRejectsIdentityRegistrationVerdictCycle(t *testing.T) {
	for _, unsupported := range []bool{false, true} {
		t.Run(map[bool]string{false: "available", true: "unsupported"}[unsupported], func(t *testing.T) {
			r, p, b := catalogIdentityPhaseFixture(t)
			// B's registration requires A's uniqueness verdict. A's uniqueness
			// verdict requires B to have registered, even without a Rule DAG cycle.
			r[1].Requires = []Dependency{{Kind: DependencyRule, Rule: r[0].Key, Relation: RelationDocument}}
			if unsupported {
				for i := range p[0].Support {
					p[0].Support[i].State = ImplementationUnsupported
					p[0].Support[i].Reason = "unavailable"
				}
			}
			if _, err := NewCatalog(r, p, b); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("identity phase cycle accepted: %v", err)
			}
		})
	}
}

func TestCatalogContextPrerequisitePrecedesIdentityRegistration(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "present"}
	writer, _ := NewContextValuePlan("ready", RelationDocument, value)
	r, p, b := catalogIdentityPhaseFixture(t)
	b[3].Plan = writer
	r[3].Role = RoleContextProvider
	r[1].Requires = []Dependency{{Kind: DependencyFact, Key: "ready", Relation: RelationDocument}}
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	pos := catalogPhasePositions(t, c, p[0].ID)
	if pos[catalogPhase{rule: r[3].Key}] >= pos[catalogPhase{rule: r[1].Key, registration: true}] {
		t.Fatal("context provider evaluated after identity registration")
	}
	// The reverse edge closes a phase-only mixed fact/identity cycle.
	r[3].Requires = []Dependency{{Kind: DependencyRule, Rule: r[0].Key, Relation: RelationDocument}}
	if _, err := NewCatalog(r, p, b); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("mixed context/identity phase cycle accepted: %v", err)
	}
}

func TestCatalogIdentityRegistrationSeparatesIndexesAndProfiles(t *testing.T) {
	r, p, b := catalogIdentityPhaseFixture(t)
	slot := b[1].Plan.attribute
	other, _ := NewIdentityDeclarationPlan("other-index", slot)
	b[1].Plan = other
	r[1].Requires = []Dependency{{Kind: DependencyRule, Rule: r[0].Key, Relation: RelationDocument}}
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatalf("different-index dependency is not circular: %v", err)
	}
	pos := catalogPhasePositions(t, c, p[0].ID)
	if pos[catalogPhase{rule: r[0].Key}] >= pos[catalogPhase{rule: r[1].Key, registration: true}] {
		t.Fatal("different-index prerequisite bypassed")
	}
	// A profile selecting only the reference cannot acquire declarations or
	// registration phases merely because the catalog knows their typed plans.
	second := cloneProfile(p[0])
	second.ID = "reference-only"
	second.Required = []RuleKey{r[2].Key}
	second.Support = []RuleSupport{{Rule: r[2].Key, State: ImplementationAvailable}}
	p = append(p, second)
	c, err = NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.phases[second.ID]; !reflect.DeepEqual(got, []catalogPhase{{rule: r[2].Key}}) {
		t.Fatalf("unselected declarations acquired phases: %v", got)
	}
}

func TestCatalogIdentityPhaseEdgesShareFiniteBudget(t *testing.T) {
	r, p, b := catalogIdentityPhaseFixture(t)
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	closure := make(map[RuleKey]bool)
	for _, rule := range r {
		closure[rule.Key] = true
	}
	// Four declaring-verdict -> registration edges, four reference ->
	// registration/verdict edges, and the deferred declaring prerequisite.
	exact := catalogBudget{remaining: 9 * catalogSlotBytes}
	if _, err := catalogProfileOrder(c, closure, &exact); err != nil {
		t.Fatal(err)
	}
	if exact.remaining != 0 {
		t.Fatalf("phase edge accounting left %d, want 0", exact.remaining)
	}
	short := catalogBudget{remaining: 8 * catalogSlotBytes}
	if _, err := catalogProfileOrder(c, closure, &short); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("phase expansion bypassed finite edge budget: %v", err)
	}
}

func TestCatalogIdentityPhasesArePermutationIndependent(t *testing.T) {
	r, p, b := catalogIdentityPhaseFixture(t)
	original, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := original.Profile(p[0].ID)
	p[0].CatalogDigest = first.CatalogDigest
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
		b[i], b[j] = b[j], b[i]
		p[0].Required[i], p[0].Required[j] = p[0].Required[j], p[0].Required[i]
		p[0].Support[i], p[0].Support[j] = p[0].Support[j], p[0].Support[i]
	}
	reordered, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original.phases[p[0].ID], reordered.phases[p[0].ID]) {
		t.Fatal("phase order depends on input/map iteration order")
	}
	before, _ := original.Schedule(p[0].ID)
	after, _ := reordered.Schedule(p[0].ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("final evaluation projection is nondeterministic")
	}
}
