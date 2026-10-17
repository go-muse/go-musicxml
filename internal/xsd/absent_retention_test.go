package xsd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"unsafe"
)

func absentRetentionSchema(tb testing.TB, count int) *Compiled {
	tb.Helper()
	attrs := make([]AttributeSchema, count)
	for i := range attrs {
		attrs[i] = AttributeSchema{Name: QName{Local: fmt.Sprintf("a%02d", i)}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}, Use: AttributeOptional}
	}
	compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: attrs}}}}}, CompileLimits{})
	if err != nil {
		tb.Fatal(err)
	}
	return compiled
}

func absentRetentionSubject(id SubjectID, model bool) Subject {
	return Subject{ID: id, Name: QName{Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true, CurrentModel: model, Current: CurrentScalar{Absent: true}, ChildrenOrderKnown: true}
}

// Repeated nodes must retain compact references to common absence facts, rather
// than a full scalar and declaration result for every declared optional slot.
func TestRunAbsentAttributesRetainCompactPerNodeFacts(t *testing.T) {
	for _, model := range []bool{false, true} {
		t.Run(fmt.Sprintf("model=%v", model), func(t *testing.T) {
			retained := func(count int) int64 {
				compiled := absentRetentionSchema(t, count)
				meter := &testMeter{}
				run, err := NewRun(compiled, meter)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := run.Close(); err != nil || meter.retained != 0 {
						t.Errorf("close=%v retained=%d", err, meter.retained)
					}
				}()
				use, _ := run.Root(QName{Local: "n"})
				if _, err := run.Bind(context.Background(), use, absentRetentionSubject(1, model)); err != nil {
					t.Fatal(err)
				}
				before := meter.retained
				binding, err := run.Bind(context.Background(), use, absentRetentionSubject(2, model))
				if err != nil {
					t.Fatal(err)
				}
				delta := meter.retained - before
				facts := slices.Collect(run.Attributes(binding))
				if len(facts) != count {
					t.Fatalf("got %d facts, want %d", len(facts), count)
				}
				for _, fact := range facts {
					if fact.Present || fact.TypeValid || !fact.DeclarationValid || !fact.Complete || fact.Scalar.Known || !fact.Scalar.Complete || fact.Use.Occurrence() == "" {
						t.Fatalf("absence fact lost: %+v", fact)
					}
				}
				return delta
			}
			overhead := retained(32) - retained(0)
			if overhead > 32*32 {
				t.Fatalf("32 optional absences retain %d extra bytes per repeated node; want compact facts within %d bytes", overhead, 32*32)
			}
		})
	}
}

func BenchmarkRunAbsentAttributes(b *testing.B) {
	for _, model := range []bool{false, true} {
		for _, count := range []int{0, 32} {
			b.Run(fmt.Sprintf("model=%v/absent=%d", model, count), func(b *testing.B) {
				compiled := absentRetentionSchema(b, count)
				use, _ := compiled.Root(QName{Local: "n"})
				var retained int64
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					meter := &testMeter{}
					run, err := NewRun(compiled, meter)
					if err != nil {
						b.Fatal(err)
					}
					for id := SubjectID(1); id <= 64; id++ {
						if _, err := run.Bind(context.Background(), use, absentRetentionSubject(id, model)); err != nil {
							b.Fatal(err)
						}
					}
					retained = meter.retained
					if err := run.Close(); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(retained)/64, "retained-B/node")
			})
		}
	}
}

func TestRunAbsentAttributesKeepOrderedIndependentFacts(t *testing.T) {
	for _, model := range []bool{false, true} {
		t.Run(fmt.Sprintf("model=%v", model), func(t *testing.T) {
			name := QName{Local: "n"}
			stringType := TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}
			integerType := TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}
			attrs := []AttributeSchema{
				{Name: QName{Local: "first"}, Type: stringType},
				{Name: QName{Local: "default"}, Type: integerType, Constraint: &ValueConstraint{Kind: "default", Value: "7"}},
				{Name: QName{Local: "present"}, Type: integerType},
				{Name: QName{Local: "last"}, Type: stringType},
				{Name: QName{Local: "fixed"}, Type: integerType, Constraint: &ValueConstraint{Kind: "fixed", Value: "9"}},
				{Name: QName{Local: "required"}, Type: stringType, Use: AttributeRequired},
				{Name: QName{Local: "prohibited"}, Type: stringType, Use: AttributeProhibited},
			}
			compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: attrs}}}}}, CompileLimits{})
			if err != nil {
				t.Fatal(err)
			}
			run, err := NewRun(compiled, &testMeter{})
			if err != nil {
				t.Fatal(err)
			}
			defer run.Close()
			use, _ := run.Root(name)
			var previous Binding
			for id := SubjectID(1); id <= 2; id++ {
				subject := absentRetentionSubject(id, model)
				text := fmt.Sprint(id)
				subject.Attributes = []Attribute{{Attr: Attr{Name: QName{Local: "present"}, Value: text}, ValueKnown: true, Current: currentNumber(t, text)}}
				binding, err := run.Bind(context.Background(), use, subject)
				if err != nil {
					t.Fatal(err)
				}
				facts := slices.Collect(run.Attributes(binding))
				if len(facts) != 5 {
					t.Fatalf("facts=%+v", facts)
				}
				for i, want := range []string{"present", "first", "default", "last", "fixed"} {
					if facts[i].Name.Local != want || facts[i].Use.Declaration() != facts[i].Name {
						t.Fatalf("ordered exact facts=%+v", facts)
					}
				}
				if facts[0].Scalar.Value.Number.Whole != text || !facts[0].Present || !facts[0].DeclarationValid {
					t.Fatalf("present fact=%+v", facts[0])
				}
				for _, i := range []int{2, 4} {
					if !facts[i].Scalar.Defaulted || !facts[i].Scalar.Known || facts[i].Present || facts[i].Scalar.Lexeme != "" {
						t.Fatalf("effective constraint=%+v", facts[i])
					}
				}
				if facts[2].Scalar.Value.Number.Whole != "7" || facts[4].Scalar.Value.Number.Whole != "9" {
					t.Fatalf("effective values=%+v", facts)
				}
				if run.Results(binding).Attributes.Valid {
					t.Fatal("required absence disappeared")
				}
				if id == 2 {
					for _, i := range []int{1, 3} {
						if binding.state.attributes[i] != previous.state.attributes[i] {
							t.Errorf("unconstrained absence %d was not shared", i)
						}
					}
					for _, i := range []int{0, 2, 4} {
						if binding.state.attributes[i] == previous.state.attributes[i] {
							t.Errorf("subject-specific fact %d was shared", i)
						}
					}
				}
				visited := 0
				for fact := range run.Attributes(binding) {
					visited++
					fact.Name.Local = "changed-copy"
					break
				}
				if visited != 1 || binding.state.attributes[0].Name.Local != "present" {
					t.Fatal("iterator failed early stop or exposed mutable record")
				}
				previous = binding
			}
			incomplete := absentRetentionSubject(3, model)
			incomplete.AttributesComplete = false
			binding, err := run.Bind(context.Background(), use, incomplete)
			if err != nil {
				t.Fatal(err)
			}
			if len(slices.Collect(run.Attributes(binding))) != 0 || run.Results(binding).Attributes.Complete {
				t.Fatal("incomplete enumeration invented known absent facts")
			}
		})
	}
}

func TestRunAbsentAttributesPreserveActualTypeUse(t *testing.T) {
	name := QName{Local: "n"}
	baseName, derivedName := QName{Local: "Base"}, QName{Local: "Derived"}
	inheritedName := QName{Local: "Inherited"}
	attr := AttributeSchema{Name: QName{Local: "a"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}
	data := SchemaData{
		Elements: map[QName]*ElementSchema{name: {Name: name, Type: TypeRef{Name: baseName}}},
		Types: map[QName]*TypeSchema{
			baseName:      {Complex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{attr}}},
			derivedName:   {Complex: &ComplexSchema{Form: ComplexComplexContentRestriction, Base: &TypeRef{Name: baseName}, Attributes: []AttributeSchema{attr}}},
			inheritedName: {Complex: &ComplexSchema{Form: ComplexComplexContentExtension, Base: &TypeRef{Name: baseName}}},
		},
	}
	compiled, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	run, err := NewRun(compiled, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	use, _ := run.Root(name)
	for id, actual := range []QName{baseName, derivedName, inheritedName, baseName, derivedName, inheritedName} {
		owner := actual
		if actual == inheritedName {
			owner = baseName
		}
		subject := absentRetentionSubject(SubjectID(id+1), false)
		if actual != baseName {
			subject.Attributes = []Attribute{{Attr: Attr{Name: QName{Space: XSINamespace, Local: "type"}, Value: actual.Local}, ValueKnown: true}}
		}
		binding, err := run.Bind(context.Background(), use, subject)
		if err != nil {
			t.Fatal(err)
		}
		facts := slices.Collect(run.Attributes(binding))
		if len(facts) != 1 {
			t.Fatalf("facts=%+v", facts)
		}
		actualOwner, ok := facts[0].Use.OwnerType()
		if !ok || actualOwner.Name() != owner {
			t.Fatalf("actual %v absent fact acquired owner %v", owner, actualOwner.Name())
		}
	}
}

type absentPrefixStopMeter struct {
	boundaryMeter
	run    *Run
	mode   string
	cancel context.CancelFunc
}

func (m *absentPrefixStopMeter) prefixReached() bool {
	return m.run != nil && m.run.context.local != nil && len(m.run.context.local.attributes) >= 3
}
func (m *absentPrefixStopMeter) Step(ctx context.Context, n int64) error {
	if m.prefixReached() {
		switch m.mode {
		case "work":
			return errMeterStop
		case "cancel":
			m.cancel()
		}
	}
	return m.boundaryMeter.Step(ctx, n)
}
func (m *absentPrefixStopMeter) Reserve(n int64) error {
	if m.mode == "buffer" && m.prefixReached() {
		return errMeterStop
	}
	return m.boundaryMeter.Reserve(n)
}

func TestRunAbsentAttributesStopAtProvedPrefix(t *testing.T) {
	for _, warmed := range []bool{false, true} {
		for _, mode := range []string{"buffer", "work", "cancel"} {
			t.Run(fmt.Sprintf("warmed=%v/%s", warmed, mode), func(t *testing.T) {
				compiled := absentRetentionSchema(t, 256)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				meter := &absentPrefixStopMeter{cancel: cancel}
				run, err := NewRun(compiled, meter)
				if err != nil {
					t.Fatal(err)
				}
				meter.run = run
				defer func() {
					if err := run.Close(); err != nil || meter.buffer != 0 {
						t.Errorf("close=%v buffer=%d", err, meter.buffer)
					}
				}()
				use, _ := run.Root(QName{Local: "n"})
				if warmed {
					if _, err := run.Bind(ctx, use, absentRetentionSubject(1, false)); err != nil {
						t.Fatal(err)
					}
				}
				meter.mode = mode
				binding, err := run.Bind(ctx, use, absentRetentionSubject(2, false))
				want := errMeterStop
				if mode == "cancel" {
					want = context.Canceled
				}
				if !errors.Is(err, want) {
					t.Fatalf("stop=%v, want %v", err, want)
				}
				facts := slices.Collect(run.Attributes(binding))
				if len(facts) < 3 || len(facts) > 4 {
					t.Fatalf("got %d facts beyond stopped prefix", len(facts))
				}
				for i, fact := range facts {
					if fact.Name.Local != fmt.Sprintf("a%02d", i) || !fact.Complete || fact.Present {
						t.Fatalf("prefix fact=%+v", fact)
					}
				}
				if run.Results(binding).Attributes.Complete {
					t.Fatal("interrupted enumeration reported complete")
				}
				meter.mode = ""
				binding, err = run.Bind(context.Background(), use, absentRetentionSubject(3, false))
				if err != nil || len(slices.Collect(run.Attributes(binding))) != 256 {
					t.Fatalf("later binding=%+v err=%v", binding, err)
				}
			})
		}
	}
}

func BenchmarkRunAbsentAttributeIteration(b *testing.B) {
	compiled := absentRetentionSchema(b, 32)
	run, err := NewRun(compiled, &testMeter{})
	if err != nil {
		b.Fatal(err)
	}
	defer run.Close()
	use, _ := run.Root(QName{Local: "n"})
	binding, err := run.Bind(context.Background(), use, absentRetentionSubject(1, false))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		for fact := range run.Attributes(binding) {
			if !fact.Present {
				count++
			}
		}
		if count != 32 {
			b.Fatal(count)
		}
	}
}

func TestRunAbsentAttributeIteratorLifecycle(t *testing.T) {
	compiled := absentRetentionSchema(t, 2)
	meter := &testMeter{}
	run, err := NewRun(compiled, meter)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := NewRun(compiled, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	use, _ := run.Root(QName{Local: "n"})
	binding, err := run.Bind(context.Background(), use, absentRetentionSubject(1, false))
	if err != nil {
		t.Fatal(err)
	}
	var nilRun *Run
	for _, test := range []struct {
		name    string
		run     *Run
		binding Binding
	}{
		{"nil-run", nilRun, binding},
		{"zero-binding", run, Binding{}},
		{"foreign-binding", foreign, binding},
		{"stale-state", run, Binding{run: run, state: &boundState{subject: binding.state.subject}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for range test.run.Attributes(test.binding) {
				t.Fatal("invalid binding yielded facts")
			}
		})
	}
	delayed := run.Attributes(binding)
	count := 0
	for range run.Attributes(binding) {
		count++
		if err := run.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Errorf("iterator yielded %d facts across Close", count)
	}
	for range delayed {
		t.Error("iterator obtained before Close exposed stale facts")
	}
	for range run.Attributes(binding) {
		t.Error("closed run exposed stale facts")
	}
	if meter.retained != 0 {
		t.Fatalf("retained=%d", meter.retained)
	}
}

type absentAdmissionMeter struct {
	boundaryMeter
	reject int64
}

func (m *absentAdmissionMeter) Reserve(n int64) error {
	if n == m.reject {
		return errMeterStop
	}
	return m.boundaryMeter.Reserve(n)
}

func TestRunAbsentAttributeCacheAdmission(t *testing.T) {
	for _, stage := range []string{"cache", "record", "node-pointer"} {
		t.Run(stage, func(t *testing.T) {
			compiled := absentRetentionSchema(t, 256)
			meter := &absentAdmissionMeter{}
			run, err := NewRun(compiled, meter)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := run.Close(); err != nil || meter.buffer != 0 {
					t.Errorf("close=%v buffer=%d", err, meter.buffer)
				}
			}()
			run.current = context.Background()
			use, _ := run.Root(QName{Local: "n"})
			effective, ok := run.context.EffectiveComplex(use.original.Type.InlineComplex)
			if !ok {
				t.Fatal("effective type unavailable")
			}
			state := &boundState{run: run, actual: &use.original.Type}
			run.context.local = state
			switch stage {
			case "cache":
				meter.reject = int64(256 * unsafe.Sizeof((*AttributeResult)(nil)))
			case "record":
				meter.reject = int64(unsafe.Sizeof(AttributeResult{}))
			case "node-pointer":
				meter.reject = int64(unsafe.Sizeof((*AttributeResult)(nil)))
			}
			func() {
				defer recoverRun(&err)
				owner, _ := run.typeID(state.actual)
				run.context.absentAttributeResult(effective, owner.Occurrence(), effective.Attributes, 0)
			}()
			if !errors.Is(err, errMeterStop) {
				t.Fatalf("stop=%v", err)
			}
			if len(state.attributes) != 0 {
				t.Fatal("unadmitted per-node pointer was published")
			}
			switch stage {
			case "cache":
				if effective.absent.records != nil {
					t.Fatal("cache array allocated before admission")
				}
			case "record":
				if effective.absent.records[0] != nil {
					t.Fatal("shared record allocated before admission")
				}
			case "node-pointer":
				if effective.absent.records[0] == nil {
					t.Fatal("already admitted immutable shared record disappeared")
				}
			}
		})
	}
}

func TestRunAbsentAttributesConcurrentIndependentRuns(t *testing.T) {
	compiled := absentRetentionSchema(t, 32)
	failures := make(chan error, 16)
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			model := worker%2 == 1
			meter := &testMeter{}
			run, err := NewRun(compiled, meter)
			if err != nil {
				failures <- err
				return
			}
			defer func() {
				if err := run.Close(); err != nil || meter.retained != 0 {
					failures <- fmt.Errorf("close=%v retained=%d", err, meter.retained)
				}
			}()
			use, _ := run.Root(QName{Local: "n"})
			for id := SubjectID(1); id <= 16; id++ {
				binding, err := run.Bind(context.Background(), use, absentRetentionSubject(id, model))
				if err != nil {
					failures <- err
					return
				}
				count := 0
				for fact := range run.Attributes(binding) {
					count++
					if fact.Present || !fact.Complete || fact.Use.Declaration() != fact.Name {
						failures <- fmt.Errorf("fact=%+v", fact)
						return
					}
				}
				if count != 32 {
					failures <- fmt.Errorf("model=%v facts=%d", model, count)
					return
				}
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}

func TestRunAbsentAttributeCacheKeepsAliasedNamedUses(t *testing.T) {
	attribute := AttributeSchema{Name: QName{Local: "a"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}
	shared := &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{attribute}}
	data := SchemaData{Elements: map[QName]*ElementSchema{}, Types: map[QName]*TypeSchema{}}
	for _, name := range []string{"one", "two"} {
		q := QName{Local: name}
		data.Types[q] = &TypeSchema{Complex: shared}
		data.Elements[q] = &ElementSchema{Name: q, Type: TypeRef{Name: q}}
	}
	compiled, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	run, err := NewRun(compiled, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	for i, name := range []string{"one", "two", "two", "one"} {
		q := QName{Local: name}
		subject := absentRetentionSubject(SubjectID(i+1), i >= 2)
		subject.Name = q
		use, _ := run.Root(q)
		binding, err := run.Bind(context.Background(), use, subject)
		if err != nil {
			t.Fatal(err)
		}
		facts := slices.Collect(run.Attributes(binding))
		if len(facts) != 1 {
			t.Fatalf("facts=%+v", facts)
		}
		owner, ok := facts[0].Use.OwnerType()
		if !ok || owner.Name() != q {
			t.Errorf("alias %q acquired another named declaration owner %q", name, owner.Name().Local)
		}
	}
}
