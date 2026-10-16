package xsd

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"unsafe"
)

func witnessAllocationBuild(r *Run, s *boundState) (err error) {
	defer recoverRun(&err)
	r.buildWitnesses(s)
	return nil
}

func TestWitnessAllocationExactBufferAndWorkBoundaries(t *testing.T) {
	const count = 9
	width := int64(unsafe.Sizeof(Witness{}))
	m := &boundaryMeter{}
	r, s := witnessAllocationFixture(t, count, m)
	if err := witnessAllocationBuild(r, s); err != nil {
		t.Fatal(err)
	}
	steps := m.work
	_ = r.Close()
	for _, kind := range []string{"buffer", "work"} {
		for _, short := range []int64{0, 1} {
			m := &boundaryMeter{}
			r, s := witnessAllocationFixture(t, count, m)
			before := m.buffer
			if kind == "buffer" {
				m.bufferLimit = before + count*width - short
			} else {
				m.workLimit = steps - short
			}
			err := witnessAllocationBuild(r, s)
			if short == 0 && err != nil || short != 0 && !errors.Is(err, errMeterStop) {
				t.Fatalf("%s short=%d: %v", kind, short, err)
			}
			if kind == "buffer" && short != 0 && (s.witnesses != nil || m.buffer != before) {
				t.Fatal("witness allocation preceded admission")
			}
			if err := r.Close(); err != nil || m.buffer != 0 {
				t.Fatalf("Close=%v buffered=%d", err, m.buffer)
			}
		}
	}
	t.Logf("%d witnesses: exact %d bytes and %d steps", count, count*width, steps)
}

func TestWitnessAllocationEveryCancellationCleansOwnership(t *testing.T) {
	probe := &scalarMeter{}
	r, s := witnessAllocationFixture(t, 9, probe)
	if err := witnessAllocationBuild(r, s); err != nil {
		t.Fatal(err)
	}
	calls := probe.calls
	_ = r.Close()
	for at := int64(1); at <= calls; at++ {
		ctx, cancel := context.WithCancel(context.Background())
		m := &scalarMeter{cancel: cancel, cancelAt: at}
		r, s := witnessAllocationFixture(t, 9, m)
		r.current = ctx
		err := witnessAllocationBuild(r, s)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d/%d: %v", at, calls, err)
		}
		if r.scratchRetained != 0 || m.retained != r.retained {
			t.Fatal("witness lease ownership lost")
		}
		if err := r.Close(); err != nil || m.retained != 0 {
			t.Fatalf("poll %d Close=%v retained=%d", at, err, m.retained)
		}
	}
	t.Logf("all %d witness-builder polls preserve cleanup", calls)
}

func TestWitnessAllocationModesAndUnavailableUse(t *testing.T) {
	r, s := witnessAllocationFixture(t, 5, &boundaryMeter{})
	s.matches[1].Particle = &ParticleSchema{Kind: ParticleAny, Any: &AnySchema{ProcessContents: "skip"}}
	s.matches[2].Particle = nil // No exact or inherited owner use: do not invent one.
	s.matches[3].Particle = &ParticleSchema{Kind: ParticleAny, Any: &AnySchema{ProcessContents: "lax"}}
	s.matches[4].Particle = &ParticleSchema{Kind: ParticleAny, Any: &AnySchema{ProcessContents: "strict"}}
	if err := witnessAllocationBuild(r, s); err != nil {
		t.Fatal(err)
	}
	if len(s.witnesses) != 4 || cap(s.witnesses) != 4 {
		t.Fatalf("unavailable use consumed a witness slot: %d/%d", len(s.witnesses), cap(s.witnesses))
	}
	for i, want := range []struct {
		child SubjectID
		mode  WitnessMode
	}{{2, WitnessDeclared}, {3, WitnessSkip}, {5, WitnessLax}, {6, WitnessStrict}} {
		if w := s.witnesses[i]; w.Child != want.child || w.Mode != want.mode {
			t.Fatalf("witness %d: %+v", i, w)
		}
	}
	if s.results.Content.Complete || !s.results.Content.Valid || !r.context.IdentityIncomplete || len(s.results.Content.Limitations) != 3 {
		t.Fatalf("lost truthful limitations: %+v", s.results.Content)
	}
}

func witnessAllocationSchema(t *testing.T, count int, required bool) (*Compiled, Subject) {
	t.Helper()
	typ := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: uint64(count), Max: uint64(count)}, Element: &ElementSchema{Name: QName{Local: "child"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}}}
	if required {
		typ.Attributes = []AttributeSchema{{Name: QName{Local: "required"}, Use: AttributeRequired, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}}
	}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	subject := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
	for i := 0; i < count; i++ {
		subject.Children = append(subject.Children, Child{ID: SubjectID(i + 2), Name: QName{Local: "child"}, Path: "/root/child"})
	}
	return c, subject
}

func TestWitnessAllocationBorrowedViewSurvivesDescendantBinding(t *testing.T) {
	c, subject := witnessAllocationSchema(t, 9, false)
	m := &boundaryMeter{}
	r, err := NewRun(c, m)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	u, _ := r.Root(subject.Name)
	b, err := r.Bind(context.Background(), u, subject)
	if err != nil {
		t.Fatal(err)
	}
	view := r.Witnesses(b)
	want := append([]Witness(nil), view...)
	var completions []Completion
	for _, w := range view {
		child, err := r.Bind(context.Background(), w.Use, Subject{ID: w.Child, Parent: 1, Name: QName{Local: "child"}, Path: "/root/child", Text: "ok", TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
		if err != nil {
			t.Fatal(err)
		}
		completion, err := r.Complete(context.Background(), child, nil)
		if err != nil {
			t.Fatal(err)
		}
		completions = append(completions, completion)
	}
	completion, err := r.Complete(context.Background(), b, completions)
	if err != nil || !completion.Complete || !completion.TypeValid || !completion.DeclarationValid || !reflect.DeepEqual(view, want) || !reflect.DeepEqual(r.Witnesses(b), want) {
		t.Fatalf("retained witness view/results changed: %+v %v", completion, err)
	}
}

type witnessAllocationStopContext struct {
	context.Context
	when func() bool
}

func (c *witnessAllocationStopContext) Err() error {
	if c.when() {
		return context.Canceled
	}
	return nil
}

type witnessAllocationRefusalMeter struct {
	boundaryMeter
	when func(int64) bool
}

func (m *witnessAllocationRefusalMeter) Reserve(n int64) error {
	if m.when != nil && m.when(n) {
		return errMeterStop
	}
	return m.boundaryMeter.Reserve(n)
}

func TestWitnessAllocationInterruptedBindPreservesPriorProof(t *testing.T) {
	for _, mode := range []string{"count-cancellation", "allocation-refusal"} {
		t.Run(mode, func(t *testing.T) {
			const count = 9
			c, subject := witnessAllocationSchema(t, count, true)
			m := &witnessAllocationRefusalMeter{}
			r, err := NewRun(c, m)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Context(context.Background())
			ready := func() bool {
				return r.context.local != nil && len(r.context.local.matches) == count && !r.context.local.results.Attributes.Valid
			}
			if mode == "count-cancellation" {
				ctx = &witnessAllocationStopContext{Context: ctx, when: ready}
			} else {
				m.when = func(n int64) bool { return ready() && n == count*int64(unsafe.Sizeof(Witness{})) }
			}
			u, _ := r.Root(subject.Name)
			b, err := r.Bind(ctx, u, subject)
			want := error(context.Canceled)
			if mode == "allocation-refusal" {
				want = errMeterStop
			}
			result := r.Results(b)
			if !errors.Is(err, want) || !r.owns(b) || result.Attributes.Valid || len(result.Attributes.Issues) == 0 || result.Attributes.Complete || result.Content.Complete || len(r.Witnesses(b)) != 0 {
				t.Fatalf("partial proof lost or completeness invented: %v / %+v", err, result)
			}
			if err := r.Close(); err != nil || m.buffer != 0 {
				t.Fatalf("Close=%v buffered=%d", err, m.buffer)
			}
		})
	}
}

// Frozen pre-fix emitter is an output oracle only. Its obsolete allocation
// policy is deliberately not used as a resource or interrupted-prefix oracle.
func witnessAllocationLegacyOutput(r *Run, s *boundState) {
	owner, _ := r.typeID(s.actual)
	for _, m := range s.matches {
		r.step(1)
		if m.Particle != nil && m.Particle.Kind == ParticleAny {
			mode := WitnessStrict
			if m.Particle.Any.ProcessContents == "skip" {
				mode = WitnessSkip
			} else if m.Particle.Any.ProcessContents == "lax" {
				mode = WitnessLax
			}
			r.context.wildcard(m.Particle.Any.ProcessContents, "content")
			s.witnesses = appendRetained(&r.context, s.witnesses, Witness{Index: m.Index, Child: s.subject.Children[m.Index].ID, Mode: mode})
			continue
		}
		uses := r.schema.particleUses[particleUseKey{owner, m.Particle}]
		var use Use
		if len(uses) == 1 {
			use = uses[0]
		} else if len(uses) == 0 {
			r.step(1)
			use = r.schema.inheritedParticleUses[m.Particle]
		}
		if use.schema == nil {
			s.limit("content", "exact-declaration-use-unavailable", nil)
			r.context.IdentityIncomplete = true
			continue
		}
		resolved := use
		if resolved.original.Reference.Local != "" {
			if target, ok := r.schema.Root(resolved.original.Reference); ok {
				resolved = target
			}
		}
		s.witnesses = appendRetained(&r.context, s.witnesses, Witness{Index: m.Index, Child: s.subject.Children[m.Index].ID, Use: use, ResolvedDeclaration: resolved, Mode: WitnessDeclared})
	}
}

func TestWitnessAllocationSuccessfulOutputMatchesPriorEmitter(t *testing.T) {
	for _, count := range []int{0, 1, 3, 9, 33} {
		for _, mixed := range []bool{false, true} {
			r, s := witnessAllocationFixture(t, count, &boundaryMeter{})
			if mixed {
				for i := range s.matches {
					switch i % 5 {
					case 1:
						s.matches[i].Particle = &ParticleSchema{Kind: ParticleAny, Any: &AnySchema{ProcessContents: "skip"}}
					case 2:
						s.matches[i].Particle = nil
					case 3:
						s.matches[i].Particle = &ParticleSchema{Kind: ParticleAny, Any: &AnySchema{ProcessContents: "lax"}}
					case 4:
						s.matches[i].Particle = &ParticleSchema{Kind: ParticleAny, Any: &AnySchema{ProcessContents: "strict"}}
					}
				}
			}
			old, err := NewRun(r.schema, &boundaryMeter{})
			if err != nil {
				t.Fatal(err)
			}
			old.current = context.Background()
			prior := &boundState{run: old, actual: s.actual, subject: s.subject, matches: s.matches, results: freshResults(), family: "content"}
			old.context.local = prior
			witnessAllocationLegacyOutput(old, prior)
			if err := witnessAllocationBuild(r, s); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.witnesses, prior.witnesses) || !reflect.DeepEqual(s.results, prior.results) || r.context.IdentityIncomplete != old.context.IdentityIncomplete {
				t.Fatalf("successful output changed: count=%d mixed=%t", count, mixed)
			}
			_ = old.Close()
		}
	}
}
