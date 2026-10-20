package xsd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"testing"
)

var errMeterStop = errors.New("meter stopped")

type boundaryMeter struct {
	work, buffer, peak, identities        int64
	workLimit, bufferLimit, identityLimit int64
	panicValue                            any
}

func (m *boundaryMeter) Step(ctx context.Context, n int64) error {
	if m.panicValue != nil {
		panic(m.panicValue)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.workLimit > 0 && n > m.workLimit-m.work {
		return errMeterStop
	}
	m.work += n
	return nil
}
func (m *boundaryMeter) Reserve(n int64) error {
	if m.bufferLimit > 0 && n > m.bufferLimit-m.buffer {
		return errMeterStop
	}
	m.buffer += n
	if m.buffer > m.peak {
		m.peak = m.buffer
	}
	return nil
}
func (m *boundaryMeter) Release(n int64) error { m.buffer -= n; return nil }
func (m *boundaryMeter) Scalar(int64) error    { return nil }
func (m *boundaryMeter) Identity(n int64) error {
	if m.identityLimit > 0 && n > m.identityLimit-m.identities {
		return errMeterStop
	}
	m.identities += n
	return nil
}
func TestMeterConstructionReservesBeforeMaps(t *testing.T) {
	m := &boundaryMeter{bufferLimit: 1}
	r, e := NewRun(testRunSchema(t), m)
	if !errors.Is(e, errMeterStop) || r != nil {
		t.Fatalf("run=%v error=%v", r, e)
	}
	if m.buffer != 0 {
		t.Fatalf("leaked=%d", m.buffer)
	}
}
func TestMeterGrammarStopsInsidePreparation(t *testing.T) {
	m := &boundaryMeter{workLimit: 20}
	r, e := NewRun(testRunSchema(t), m)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	r.current = context.Background()
	p := &ParticleSchema{Kind: ParticleSequence, Occurrence: Occurrence{Min: 1, Max: 1}}
	for i := 0; i < 100; i++ {
		p.Children = append(p.Children, &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Max: 1}, Element: &ElementSchema{Name: QName{Local: "item"}}})
	}
	var stop error
	func() { defer recoverRun(&stop); r.context.MatchParticle(p, nil, 0) }()
	if !errors.Is(stop, errMeterStop) {
		t.Fatalf("grammar bypassed limit: %v work=%d", stop, m.work)
	}
}
func TestMeterIdentityCountsRetainedSlots(t *testing.T) {
	m := &boundaryMeter{identityLimit: 2}
	r, e := NewRun(testRunSchema(t), m)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	r.current = context.Background()
	var stop error
	func() {
		defer recoverRun(&stop)
		for i := 0; i < 3; i++ {
			r.context.SeedIdentity(string(rune('a'+i)), "/n", "ID")
		}
	}()
	if !errors.Is(stop, errMeterStop) || len(r.context.IdentityEntries) != 2 {
		t.Fatalf("error=%v entries=%d ids=%d", stop, len(r.context.IdentityEntries), m.identities)
	}
}
func TestMeterArbitraryPanicPropagates(t *testing.T) {
	m := &boundaryMeter{panicValue: "application panic"}
	r, e := NewRun(testRunSchema(t), m)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	u, _ := r.Root(QName{Local: "root"})
	defer func() {
		if p := recover(); p != "application panic" {
			t.Fatalf("panic=%v", p)
		}
	}()
	_, _ = r.Bind(context.Background(), u, Subject{ID: 1})
	t.Fatal("panic swallowed")
}

func TestMeterDiagnosticFailurePrecedesRetention(t *testing.T) {
	m := &boundaryMeter{}
	r, e := NewRun(testRunSchema(t), m)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	s := &boundState{bindWork: &bindWork{results: freshResults(), family: "scalar"}, run: r}
	m.bufferLimit = m.buffer + 1
	var stop error
	func() { defer recoverRun(&stop); s.detail("/n", "datatype", "invalid integer") }()
	if !errors.Is(stop, errMeterStop) || s.results.Scalar.Valid || len(s.results.Scalar.Issues) != 0 {
		t.Fatalf("stop=%v partial=%+v", stop, s.results.Scalar)
	}
}

func TestMeterMetadataAllocationPrecedesCache(t *testing.T) {
	m := &boundaryMeter{}
	r, e := NewRun(testRunSchema(t), m)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	r.current = context.Background()
	m.bufferLimit = m.buffer + 1
	var stop error
	func() { defer recoverRun(&stop); r.context.ValidTypeMetadata(BuiltinType("integer")) }()
	if !errors.Is(stop, errMeterStop) || r.context.TypeMetadata != nil {
		t.Fatalf("stop=%v cache=%v", stop, r.context.TypeMetadata)
	}
}

func TestMeterOperationScratchAndSiblingCleanup(t *testing.T) {
	m := &boundaryMeter{}
	first, e := NewRun(testRunSchema(t), m)
	if e != nil {
		t.Fatal(e)
	}
	defer first.Close()
	sibling, e := NewRun(first.schema, m)
	if e != nil {
		t.Fatal(e)
	}
	siblingBytes := sibling.retained
	u, _ := first.Root(QName{Local: "root"})
	_, e = first.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "root"}, TextKnown: true, ChildrenComplete: true, AttributesComplete: true, Children: []Child{{ID: 2, Name: QName{Local: "child"}}}})
	if e != nil {
		t.Fatal(e)
	}
	if first.scratchRetained != 0 || m.buffer != first.retained+siblingBytes {
		t.Fatalf("scratch=%d buffer=%d owned=%d", first.scratchRetained, m.buffer, first.retained+siblingBytes)
	}
	if e := sibling.Close(); e != nil {
		t.Fatal(e)
	}
	if m.buffer != first.retained {
		t.Fatalf("sibling close stole retention: buffer=%d want=%d", m.buffer, first.retained)
	}
}

func TestMeterBindExactWorkBoundaryAndCause(t *testing.T) {
	schema := testRunSchema(t)
	subject := Subject{ID: 1, Name: QName{Local: "root"}, TextKnown: true, ChildrenComplete: true, AttributesComplete: true, Children: []Child{{ID: 2, Name: QName{Local: "child"}}}}
	m := &boundaryMeter{}
	r, e := NewRun(schema, m)
	if e != nil {
		t.Fatal(e)
	}
	u, _ := r.Root(subject.Name)
	_, e = r.Bind(context.Background(), u, subject)
	if e != nil {
		t.Fatal(e)
	}
	work := m.work
	_ = r.Close()
	for _, delta := range []int64{-1, 0, 1} {
		t.Run(fmt.Sprint(delta), func(t *testing.T) {
			m := &boundaryMeter{workLimit: work + delta}
			r, e := NewRun(schema, m)
			if e != nil {
				t.Fatal(e)
			}
			b, e := r.Bind(context.Background(), u, subject)
			if delta < 0 {
				if !errors.Is(e, errMeterStop) {
					t.Fatalf("error=%v", e)
				}
				if !r.owns(b) {
					t.Fatal("partial binding lost")
				}
				if r.Results(b).Content.Complete || r.Results(b).Declaration.Complete {
					t.Fatal("aborted binding claimed complete local evidence")
				}
			} else if e != nil {
				t.Fatal(e)
			}
			if r.scratchRetained != 0 {
				t.Fatal("scratch leaked")
			}
			_ = r.Close()
			if m.buffer != 0 {
				t.Fatalf("buffer leaked=%d", m.buffer)
			}
			if m.work == 0 {
				t.Fatal("work refunded")
			}
		})
	}
}

func TestMeterIdentityUnitsAndFinalScan(t *testing.T) {
	m := &boundaryMeter{}
	r, e := NewRun(testRunSchema(t), m)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	r.current = context.Background()
	id := &TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}
	ref := &TypeRef{Name: QName{Space: XSDNamespace, Local: "IDREF"}}
	for _, value := range []string{"one", "two"} {
		r.context.RecordIdentity(id, value, "/n/@id")
		r.context.RecordIdentityBinding(id, value, "/n/@id", &Node{})
	}
	r.context.RecordIdentity(ref, "one", "/n/@ref")
	if m.identities != 5 {
		t.Fatalf("identity units=%d, want two entries + two owners + one reference", m.identities)
	}
	before := m.work
	m.workLimit = before + 1
	_, e = r.Identity(context.Background(), true)
	if !errors.Is(e, errMeterStop) {
		t.Fatalf("final scan bypassed work limit: %v", e)
	}
	if m.identities != 5 {
		t.Fatal("final scan recharged identities")
	}
}

func TestMeterAllocationWidthAndOverflow(t *testing.T) {
	c := &Context{}
	for _, f := range []func(){func() { c.size(math.MaxInt64, 2) }, func() { c.sum(math.MaxInt64, 1) }, func() { c.size(-1, 8) }} {
		var e error
		func() { defer recoverRun(&e); f() }()
		if e == nil {
			t.Fatal("unchecked allocation")
		}
	}
	if strconv.IntSize == 32 {
		var e error
		func() { defer recoverRun(&e); c.size(1<<31, 1) }()
		if e == nil {
			t.Fatal("host-size allocation accepted")
		}
	}
}

type checkpointMeter struct {
	boundaryMeter
	cancel  context.CancelFunc
	trigger func() bool
}

func (m *checkpointMeter) Step(ctx context.Context, n int64) error {
	if m.trigger != nil && m.trigger() {
		m.cancel()
	}
	return m.boundaryMeter.Step(ctx, n)
}
func TestMeterGrammarCancellationAtActualStates(t *testing.T) {
	for _, stage := range []string{"prepare", "repeat", "transition", "witness", "all-scan"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			meter := &checkpointMeter{cancel: cancel}
			r, e := NewRun(testRunSchema(t), meter)
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			r.current = ctx
			leaf := &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1}, Element: &ElementSchema{Name: QName{Local: "n"}}}
			p := &ParticleSchema{Kind: ParticleSequence, Occurrence: Occurrence{Min: 0, Unbounded: true}, Children: []*ParticleSchema{leaf}}
			var children []*Node
			for range 30 {
				children = append(children, &Node{Name: QName{Local: "n"}})
			}
			m := NewParticleMatcher(&r.context, children)
			m.Work = &ParticleWork{}
			var operation func()
			switch stage {
			case "prepare":
				p = &ParticleSchema{Kind: ParticleSequence, Occurrence: Occurrence{Min: 1, Max: 1}}
				for range 30 {
					p.Children = append(p.Children, &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Max: 1}, Element: &ElementSchema{Name: QName{Local: "n"}}})
				}
				meter.trigger = func() bool { return len(m.Nullable) >= 10 }
				operation = func() { m.Prepare(p, map[*ParticleSchema]bool{}) }
			case "repeat":
				meter.trigger = func() bool { return m.Work.RepeatStates >= 3 }
				operation = func() { m.Run(p, 0) }
			case "transition":
				meter.trigger = func() bool { return m.Work.Transitions >= 4 }
				operation = func() { m.Run(p, 0) }
			case "witness":
				if _, _, ok := m.Run(p, 0); !ok {
					t.Fatal("baseline did not match")
				}
				m.Work = &ParticleWork{}
				meter.trigger = func() bool { return m.Work.WitnessStates >= 2 }
				operation = func() { var out []ElementMatch; m.Witness(p, 0, len(children), &out) }
			case "all-scan":
				p = &ParticleSchema{Kind: ParticleAll}
				for range 30 {
					p.Children = append(p.Children, leaf)
				}
				m.Children = []*Node{{Name: QName{Local: "other"}}}
				meter.trigger = func() bool { return m.Work.Transitions >= 5 }
				operation = func() { m.All(p, 0, nil) }
			}
			var stop error
			func() { defer recoverRun(&stop); operation() }()
			if !errors.Is(stop, context.Canceled) {
				t.Fatalf("stage bypassed cancellation: %v; work=%+v", stop, m.Work)
			}
		})
	}
}

func TestMeterIdentityLimitPrecedesEveryIndex(t *testing.T) {
	for _, binding := range []bool{false, true} {
		t.Run(fmt.Sprint(binding), func(t *testing.T) {
			m := &boundaryMeter{identityLimit: 1}
			r, e := NewRun(testRunSchema(t), m)
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			r.current = context.Background()
			r.context.SeedIdentity("first", "/first", "ID")
			ref := &TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}
			var stop error
			func() {
				defer recoverRun(&stop)
				if binding {
					r.context.RecordIdentityBinding(ref, "second", "/second", &Node{})
				} else {
					r.context.RecordIdentity(ref, "second", "/second")
				}
			}()
			if !errors.Is(stop, errMeterStop) {
				t.Fatalf("stop=%v", stop)
			}
			if _, exists := r.context.Identifiers["second"]; exists {
				t.Fatal("declaration mirror retained before identity charge")
			}
			if _, exists := r.context.IdentityBindings["second"]; exists {
				t.Fatal("binding index retained before identity charge")
			}
		})
	}
}

func TestMeterEndpointSortWorkIsDeterministic(t *testing.T) {
	for _, predecessors := range []bool{false, true} {
		var want int64
		for trial := range 64 {
			m := &boundaryMeter{}
			r := &Run{meter: m, current: context.Background()}
			c := &Context{run: r}
			if predecessors {
				p := &ParticlePredecessors{Many: map[int]int{}}
				for i := range 37 {
					p.Many[i] = i
				}
				p.Ordered(&ParticleMatcher{Context: c})
			} else {
				seen := map[int]bool{}
				for i := range 37 {
					seen[i] = true
				}
				orderedPositions(seen, c)
			}
			r.releaseScratch()
			if trial == 0 {
				want = m.work
			} else if m.work != want {
				t.Fatalf("random map order changed work: predecessors=%v trial=%d got=%d want=%d", predecessors, trial, m.work, want)
			}
		}
	}
}
