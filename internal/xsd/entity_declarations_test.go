package xsd

import (
	"context"
	"errors"
	"testing"
)

type entityAdmissionMeter struct {
	testMeter
	reservations, failAt int
}

func (m *entityAdmissionMeter) Reserve(n int64) error {
	m.reservations++
	if m.reservations == m.failAt {
		return errMeterStop
	}
	return m.testMeter.Reserve(n)
}
func entityRestrictedKernel(t *testing.T) *Compiled {
	t.Helper()
	schema, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineSimple: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "ENTITY"}}, Patterns: []string{"[a-z]+"}}}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}
func entityKernelSubject() Subject {
	return Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", Text: "asset", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
}
func TestEntityDomainEveryReservationInterruptionKeepsTruth(t *testing.T) {
	ctx := context.Background()
	schema := entityRestrictedKernel(t)
	use, _ := schema.Root(QName{Local: "root"})
	domain := &EntityDeclarations{Complete: true, Bindings: map[string]EntityDeclaration{"asset": {Unparsed: true, Sequence: 1}}}
	baseline := &entityAdmissionMeter{}
	run, err := NewRunWithSourceDeclarations(schema, baseline, domain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run.Bind(ctx, use, entityKernelSubject()); err != nil {
		t.Fatal(err)
	}
	count := baseline.reservations
	_ = run.Close()
	for stop := 1; stop <= count; stop++ {
		m := &entityAdmissionMeter{failAt: stop}
		r, err := NewRunWithSourceDeclarations(schema, m, domain)
		if err == nil {
			var b Binding
			b, err = r.Bind(ctx, use, entityKernelSubject())
			if r.owns(b) && !r.Results(b).Scalar.Valid {
				t.Fatalf("reservation %d invented an entity failure: %+v", stop, r.Results(b).Scalar)
			}
			if r.context.entityDeferred != 0 || r.context.entityLiteralDepth != 0 || r.context.entityConstructedDepth != 0 {
				t.Fatalf("scope leaked at %d", stop)
			}
			_ = r.Close()
		}
		if !errors.Is(err, errMeterStop) {
			t.Fatalf("stop %d: %v", stop, err)
		}
		if m.retained != 0 {
			t.Fatalf("stop %d retained %d", stop, m.retained)
		}
	}
}
func TestEntityDomainNegativeProofBeforeDiagnosticReservation(t *testing.T) {
	schema := entityRestrictedKernel(t)
	use, _ := schema.Root(QName{Local: "root"})
	domain := &EntityDeclarations{Complete: true, Bindings: map[string]EntityDeclaration{"asset": {Unparsed: false, Sequence: 8}}}
	baseline := &entityAdmissionMeter{}
	run, err := NewRunWithSourceDeclarations(schema, baseline, domain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := run.Bind(context.Background(), use, entityKernelSubject())
	if err != nil || run.Results(b).Scalar.Valid {
		t.Fatalf("negative witness missing: %v", err)
	}
	count := baseline.reservations
	_ = run.Close()
	found := false
	for stop := 1; stop <= count; stop++ {
		m := &entityAdmissionMeter{failAt: stop}
		r, err := NewRunWithSourceDeclarations(schema, m, domain)
		if err == nil {
			b, bindErr := r.Bind(context.Background(), use, entityKernelSubject())
			err = bindErr
			if r.owns(b) && !r.Results(b).Scalar.Valid && len(r.Results(b).Scalar.Issues) == 0 {
				found = true
			}
			_ = r.Close()
		}
		if !errors.Is(err, errMeterStop) || m.retained != 0 {
			t.Fatalf("stop %d: %v retained %d", stop, err, m.retained)
		}
	}
	if !found {
		t.Fatal("negative proof depended on retained diagnostic")
	}
}
func TestEntityDomainIncompleteAttributesKeepBindingUnavailable(t *testing.T) {
	schema := entityRestrictedKernel(t)
	run, err := NewRunWithSourceDeclarations(schema, &testMeter{}, &EntityDeclarations{Complete: true})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	use, _ := schema.Root(QName{Local: "root"})
	subject := entityKernelSubject()
	subject.AttributesComplete = false
	b, err := run.Bind(context.Background(), use, subject)
	if err != nil {
		t.Fatal(err)
	}
	r := run.Results(b)
	if !r.Scalar.Valid || r.Scalar.Complete || r.Binding.Complete {
		t.Fatalf("domain bypassed xsi attribute closure: %+v", r)
	}
}

func TestEntityDomainEveryCancellationCheckpointRestoresScopes(t *testing.T) {
	schema := entityRestrictedKernel(t)
	use, _ := schema.Root(QName{Local: "root"})
	domain := &EntityDeclarations{Complete: true, Bindings: map[string]EntityDeclaration{"asset": {Unparsed: true, Sequence: 1}}}
	baseline := &scalarMeter{}
	r, err := NewRunWithSourceDeclarations(schema, baseline, domain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Bind(context.Background(), use, entityKernelSubject()); err != nil {
		t.Fatal(err)
	}
	calls, steps := baseline.calls, baseline.steps
	_ = r.Close()
	for stop := int64(1); stop <= calls; stop++ {
		ctx, cancel := context.WithCancel(context.Background())
		m := &scalarMeter{cancel: cancel, cancelAt: stop}
		r, err := NewRunWithSourceDeclarations(schema, m, domain)
		if err != nil {
			t.Fatal(err)
		}
		b, err := r.Bind(ctx, use, entityKernelSubject())
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("checkpoint %d: %v", stop, err)
		}
		if r.owns(b) && !r.Results(b).Scalar.Valid {
			t.Fatalf("checkpoint %d invented failure", stop)
		}
		if r.context.entityDeferred != 0 || r.context.entityLiteralDepth != 0 || r.context.entityConstructedDepth != 0 {
			t.Fatalf("checkpoint %d leaked scope", stop)
		}
		_ = r.Close()
		if m.retained != 0 {
			t.Fatalf("checkpoint %d retained %d", stop, m.retained)
		}
	}
	for _, short := range []int64{0, 1} {
		m := &scalarMeter{limit: steps - short, cause: errMeterStop}
		r, err := NewRunWithSourceDeclarations(schema, m, domain)
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Bind(context.Background(), use, entityKernelSubject())
		if short == 0 && err != nil || short == 1 && !errors.Is(err, errMeterStop) {
			t.Fatalf("one-short=%d: %v", short, err)
		}
		_ = r.Close()
		if m.retained != 0 {
			t.Fatalf("one-short=%d retention=%d", short, m.retained)
		}
	}
}
func TestEntityDomainsShareCompiledSchemaConcurrently(t *testing.T) {
	schema := entityRestrictedKernel(t)
	for _, unparsed := range []bool{false, true} {
		for _, name := range []string{"first", "second", "third", "fourth"} {
			t.Run(name+map[bool]string{true: "/unparsed", false: "/parsed"}[unparsed], func(t *testing.T) {
				t.Parallel()
				m := &testMeter{}
				domain := &EntityDeclarations{Complete: true, Bindings: map[string]EntityDeclaration{"asset": {Unparsed: unparsed, Sequence: 1}}}
				r, err := NewRunWithSourceDeclarations(schema, m, domain)
				if err != nil {
					t.Fatal(err)
				}
				use, _ := schema.Root(QName{Local: "root"})
				b, err := r.Bind(context.Background(), use, entityKernelSubject())
				if err != nil || r.Results(b).Scalar.Valid != unparsed || !r.Results(b).Scalar.Complete {
					t.Fatalf("cross-run domain: %+v / %v", r.Results(b).Scalar, err)
				}
				_ = r.Close()
				if m.retained != 0 || r.context.entityDeclarations != nil {
					t.Fatal("closed run retained borrowed domain")
				}
			})
		}
	}
}
