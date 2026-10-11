package xsd

import (
	"context"
	"fmt"
	"testing"
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
				facts := run.Attributes(binding)
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
