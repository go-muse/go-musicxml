package xsd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func newIdentityResolverRun(t *testing.T) (*Run, *testMeter) {
	t.Helper()
	meter := &testMeter{}
	run, err := NewRun(testRunSchema(t), meter)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !run.closed {
			if err := run.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	run.current = context.Background()
	return run, meter
}

func registerResolverOwner(run *Run, value string, owner *Node) {
	run.context.RecordIdentityBinding(&TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}, value, "/owner", owner)
}

func TestRunResolveIdentityClosureAndCandidate(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			run, _ := newIdentityResolverRun(t)
			for i := range count {
				registerResolverOwner(run, "target", &Node{ID: SubjectID(10 + i)})
			}
			got, err := run.ResolveIdentity(context.Background(), "target")
			if err != nil {
				t.Fatal(err)
			}
			want := IdentityIncomplete
			if count >= 2 {
				want = IdentityAmbiguous
			}
			if got.Status != want || got.ScopeComplete {
				t.Fatalf("open scope: %+v", got)
			}
			if count == 1 && (got.Owner != 10 || got.FirstOwner != 10 || got.Reason == "") {
				t.Errorf("candidate lost: %+v", got)
			}
			if count >= 2 && (got.FirstOwner != 10 || got.SecondOwner != 11 || got.Owner != 0) {
				t.Errorf("unstable bounded ambiguity: %+v", got)
			}
			if _, err := run.Finalize(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			got, err = run.ResolveIdentity(context.Background(), "target")
			if err != nil {
				t.Fatal(err)
			}
			want = IdentityMissing
			if count == 1 {
				want = IdentityUnique
			}
			if count >= 2 {
				want = IdentityAmbiguous
			}
			if got.Status != want || !got.ScopeComplete || got.Reason != "" {
				t.Errorf("final scope: %+v", got)
			}
		})
	}
}

func TestRunResolveIdentityUsesActualContainingOwner(t *testing.T) {
	run, _ := newIdentityResolverRun(t)
	owner := &Node{ID: 42}
	// The ID-valued element has occurrence 99, but its actual-ID owner is its
	// containing element. Display paths and scalar origins are not owner IDs.
	run.context.local = &boundState{bindWork: &bindWork{subject: Subject{ID: 99}}, id: 99}
	registerResolverOwner(run, " \ttarget\n ", owner)
	registerResolverOwner(run, "target", owner)
	run.context.local = nil
	if _, err := run.Identity(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	got, err := run.ResolveIdentity(context.Background(), "target")
	if err != nil || got.Status != IdentityUnique || got.Owner != 42 || got.FirstOwner != 42 || got.SecondOwner != 0 {
		t.Fatalf("actual owner lost: %+v, %v", got, err)
	}
	// Lookup consumes already validated normalized values; it does not silently
	// turn arbitrary strings into valid reference values.
	got, err = run.ResolveIdentity(context.Background(), " target ")
	if err != nil || got.Status != IdentityMissing {
		t.Errorf("lookup fabricated normalization: %+v, %v", got, err)
	}
}

func TestRunResolveIdentityIgnoresUnrelatedIdentityFailures(t *testing.T) {
	for _, failure := range []string{"missing-reference", "duplicate-owner"} {
		t.Run(failure, func(t *testing.T) {
			run, _ := newIdentityResolverRun(t)
			registerResolverOwner(run, "target", &Node{ID: 42})
			if failure == "missing-reference" {
				run.context.RecordIdentity(&TypeRef{Name: QName{Space: XSDNamespace, Local: "IDREF"}}, "unrelated-missing", "/reference")
			} else {
				id := &TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}
				run.context.RecordIdentity(id, "unrelated-duplicate", "/first")
				registerResolverOwner(run, "unrelated-duplicate", &Node{ID: 1})
				registerResolverOwner(run, "unrelated-duplicate", &Node{ID: 2})
			}
			identity, err := run.Finalize(context.Background(), true)
			if err != nil || identity.Valid || !identity.Complete {
				t.Fatalf("failure control: %+v, %v", identity, err)
			}
			got, err := run.ResolveIdentity(context.Background(), "target")
			if err != nil || got.Status != IdentityUnique || got.Owner != 42 || !got.ScopeComplete {
				t.Fatalf("unrelated failure gated owner: %+v, %v", got, err)
			}
		})
	}
}

func TestRunResolveIdentityIncompleteDoesNotProveAbsenceOrUniqueness(t *testing.T) {
	for _, reason := range []string{"input", "identity", "assessment"} {
		t.Run(reason, func(t *testing.T) {
			run, _ := newIdentityResolverRun(t)
			registerResolverOwner(run, "target", &Node{ID: 42})
			registerResolverOwner(run, "duplicate", &Node{ID: 1})
			registerResolverOwner(run, "duplicate", &Node{ID: 2})
			if reason == "identity" {
				run.context.IdentityIncomplete = true
			}
			if reason == "assessment" {
				run.context.AssessmentErr = errors.New("source capability incomplete")
			}
			if _, err := run.Finalize(context.Background(), reason != "input"); err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{"target", "absent", "duplicate"} {
				got, err := run.ResolveIdentity(context.Background(), value)
				if err != nil {
					t.Fatal(err)
				}
				want := IdentityIncomplete
				if value == "duplicate" {
					want = IdentityAmbiguous
				}
				if got.Status != want || got.ScopeComplete {
					t.Errorf("%s: %+v", value, got)
				}
			}
		})
	}
}

func TestRunResolveIdentityLifecycleAndMeter(t *testing.T) {
	run, meter := newIdentityResolverRun(t)
	registerResolverOwner(run, "target", &Node{ID: 42})
	if _, err := run.Finalize(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	before := meter.retained
	if _, err := run.ResolveIdentity(context.Background(), "target"); err != nil {
		t.Fatal(err)
	}
	if meter.retained != before {
		t.Fatal("read-only resolution retained storage")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := run.ResolveIdentity(ctx, "target"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancellation ignored: %v", err)
	}
	if _, err := run.ResolveIdentity(nil, "target"); err == nil {
		t.Error("accepted nil context")
	}
	meter.limit = meter.steps + 2
	if _, err := run.ResolveIdentity(context.Background(), strings.Repeat("x", 4096)); err == nil {
		t.Error("unmetered lookup scan")
	}
	meter.limit = 0
	if err := run.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := run.ResolveIdentity(context.Background(), "target"); err == nil {
		t.Error("accepted closed run")
	}
	if _, err := (*Run)(nil).ResolveIdentity(context.Background(), "target"); err == nil {
		t.Error("accepted nil run")
	}
	if meter.retained != 0 {
		t.Errorf("retained storage leaked: %d", meter.retained)
	}
}

func TestRunResolveIdentitySurvivesUnrelatedScalarFailure(t *testing.T) {
	for _, idValue := range []string{"target", "invalid id"} {
		t.Run(idValue, func(t *testing.T) {
			name := QName{Local: "root"}
			data := SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{
				{Name: QName{Local: "id"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}},
				{Name: QName{Local: "number"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}},
			}}}}}}
			compiled, err := Compile(data, CompileLimits{})
			if err != nil {
				t.Fatal(err)
			}
			run, err := NewRun(compiled, &testMeter{})
			if err != nil {
				t.Fatal(err)
			}
			defer run.Close()
			use, _ := compiled.Root(name)
			bound, err := run.Bind(context.Background(), use, Subject{ID: 42, Name: name, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{
				{Attr: Attr{Name: QName{Local: "id"}, Value: idValue}, ValueKnown: true},
				{Attr: Attr{Name: QName{Local: "number"}, Value: "bad"}, ValueKnown: true},
			}})
			if err != nil {
				t.Fatal(err)
			}
			completion, err := run.Complete(context.Background(), bound, nil)
			if err != nil || completion.TypeValid || !completion.Complete {
				t.Fatalf("ordinary failure control: %+v, %v", completion, err)
			}
			if _, err := run.Finalize(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			got, err := run.ResolveIdentity(context.Background(), "target")
			want := IdentityUnique
			if idValue != "target" {
				want = IdentityMissing
			}
			if err != nil || got.Status != want || !got.ScopeComplete {
				t.Fatalf("ordinary failure blocked resolution: %+v, %v", got, err)
			}
			if want == IdentityUnique && got.Owner != 42 {
				t.Errorf("wrong attribute owner: %+v", got)
			}
		})
	}
}

func TestRunResolveIdentityReadWorkIgnoresOtherOwners(t *testing.T) {
	var baseline int64
	for _, count := range []int{1, 2, 1000} {
		run, meter := newIdentityResolverRun(t)
		for i := range count {
			registerResolverOwner(run, "target", &Node{ID: SubjectID(i + 1)})
		}
		if _, err := run.Identity(context.Background(), true); err != nil {
			t.Fatal(err)
		}
		before := meter.steps
		if _, err := run.ResolveIdentity(context.Background(), "target"); err != nil {
			t.Fatal(err)
		}
		work := meter.steps - before
		if count == 1 {
			baseline = work
		} else if work != baseline {
			t.Errorf("owners=%d work=%d baseline=%d", count, work, baseline)
		}
	}
}

func TestRunResolveIdentityNewBindingReopensScope(t *testing.T) {
	run, _ := newIdentityResolverRun(t)
	registerResolverOwner(run, "target", &Node{ID: 42})
	if _, err := run.Identity(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	use, _ := run.Root(QName{Local: "root"})
	bound, err := run.Bind(context.Background(), use, Subject{ID: 1, Name: QName{Local: "root"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Complete(context.Background(), bound, nil); err != nil {
		t.Fatal(err)
	}
	got, err := run.ResolveIdentity(context.Background(), "target")
	if err != nil || got.Status != IdentityIncomplete || got.Owner != 42 || got.ScopeComplete {
		t.Fatalf("stale closure: %+v, %v", got, err)
	}
	if _, err := run.Identity(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	got, err = run.ResolveIdentity(context.Background(), "target")
	if err != nil || got.Status != IdentityUnique {
		t.Fatalf("reclosed scope: %+v, %v", got, err)
	}
}

type identityOwnerStopMeter struct {
	testMeter
	stopOwner bool
}

func (m *identityOwnerStopMeter) Reserve(n int64) error {
	if m.stopOwner && n == 32+nodeBytes { // the owner entry and its node
		return errors.New("owner entry storage exhausted")
	}
	return m.testMeter.Reserve(n)
}

func TestRunResolveIdentityStoppedRegistrationHasNoUnsuccessfulOwner(t *testing.T) {
	for _, existing := range []bool{false, true} {
		meter := &identityOwnerStopMeter{}
		run, err := NewRun(testRunSchema(t), meter)
		if err != nil {
			t.Fatal(err)
		}
		run.current = context.Background()
		if existing {
			registerResolverOwner(run, "target", &Node{ID: 1})
		}
		meter.stopOwner = true
		err = func() (err error) {
			defer recoverRun(&err)
			registerResolverOwner(run, "target", &Node{ID: 2})
			return nil
		}()
		if err == nil {
			t.Fatal("registration stop control did not trigger")
		}
		got, err := run.ResolveIdentity(context.Background(), "target")
		if err != nil {
			t.Fatal(err)
		}
		if got.SecondOwner != 0 || (!existing && (got.FirstOwner != 0 || got.Owner != 0)) {
			t.Errorf("unsuccessful owner escaped: existing=%v resolution=%+v", existing, got)
		}
		if err := run.Close(); err != nil {
			t.Error(err)
		}
	}
}

func TestRunResolveIdentityCurrentLexicalCapabilityIsLocal(t *testing.T) {
	for _, knownID := range []bool{false, true} {
		name := QName{Local: "root"}
		pattern := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "decimal"}}, Patterns: []string{"[0-9]+"}}
		data := SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{
			{Name: QName{Local: "id"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}},
			{Name: QName{Local: "number"}, Type: TypeRef{InlineSimple: pattern}},
		}}}}}}
		compiled, err := Compile(data, CompileLimits{})
		if err != nil {
			t.Fatal(err)
		}
		run, err := NewRun(compiled, &testMeter{})
		if err != nil {
			t.Fatal(err)
		}
		use, _ := compiled.Root(name)
		id := CurrentScalar{}
		if knownID {
			id = CurrentScalar{Known: true, Value: SimpleValue{Domain: StringDomain, Text: "target"}}
		}
		bound, err := run.Bind(context.Background(), use, Subject{ID: 42, Name: name, Path: "/root", CurrentModel: true, Current: CurrentScalar{Absent: true}, ChildrenOrderKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{
			{Attr: Attr{Name: QName{Local: "id"}}, Current: id},
			{Attr: Attr{Name: QName{Local: "number"}}, Current: CurrentScalar{Known: true, Value: SimpleValue{Domain: NumberDomain, Number: Decimal{Whole: "4"}}}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		completion, err := run.Complete(context.Background(), bound, nil)
		if err != nil || completion.Complete {
			t.Fatalf("missing lexical capability control: %+v, %v", completion, err)
		}
		if _, err := run.Finalize(context.Background(), true); err != nil {
			t.Fatal(err)
		}
		got, err := run.ResolveIdentity(context.Background(), "target")
		want := IdentityIncomplete
		if knownID {
			want = IdentityUnique
		}
		if err != nil || got.Status != want || got.ScopeComplete != knownID {
			t.Errorf("knownID=%v got=%+v err=%v", knownID, got, err)
		}
		if err := run.Close(); err != nil {
			t.Error(err)
		}
	}
}
