package validation

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// A causal Blocked summary is not proof that every incomplete sub-operation
// depends on the referenced failure. One composite may contain both a proven
// representation failure and an independently unavailable capability.
func TestModelBlockedCompositeCanRetainIndependentLimitations(t *testing.T) {
	run := func(mixed bool) Report {
		t.Helper()
		attributes := []SchemaAttribute{
			{Name: QName{Local: "broken"}, Type: SchemaTypeRef{Name: QName{Space: xsd.XSDNamespace, Local: "decimal"}}},
			{Name: QName{Local: "limited"}, Type: SchemaTypeRef{InlineSimple: &SchemaSimple{
				Form:     SchemaSimpleRestriction,
				Base:     &SchemaSimpleMember{Name: QName{Space: xsd.XSDNamespace, Local: "integer"}},
				Patterns: []string{"[0-9]+"},
			}}},
		}
		root := &SchemaElement{Name: QName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: attributes}}}
		schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{{Local: "root"}: root}})
		if err != nil {
			t.Fatal(err)
		}
		session := modelSchemaSession(t, schema, Budgets{})
		ctx := context.Background()
		header := modelSchemaHeader(1, 0, "root", "Root")
		if err := session.OpenNode(ctx, header); err != nil {
			t.Fatal(err)
		}
		attribute := func(name string, value Evidence[Atomic]) {
			t.Helper()
			fact := AttributeFact{
				Owner: 1, Name: QName{Local: name},
				Presence:    Evidence[bool]{State: EvidenceKnown, Value: true},
				Declaration: Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"},
				Scalar:      Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}, Value: value},
				Location:    header.Location,
			}
			if err := session.Attribute(ctx, fact); err != nil {
				t.Fatal(err)
			}
		}
		attribute("broken", Evidence[Atomic]{State: EvidenceUnsupported, Reason: "model-nonfinite"})
		if mixed {
			integer, err := NewInteger(big.NewInt(4))
			if err != nil {
				t.Fatal(err)
			}
			attribute("limited", Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: integer}})
		}
		modelSchemaClose(t, session, modelSchemaEnd(1, Evidence[Atomic]{State: EvidenceAbsent}, OrderModelFields), ModelProblem{
			Code: "model-nonfinite", Axis: ModelAxisAttribute, Attribute: QName{Local: "broken"},
			Location: header.Location, Expected: "finite", Actual: "NaN",
		})
		result := session.schemaResult(session.nodes[1], XSDCheckAttributes)
		reasons := make([]string, 0, len(result.Limitations))
		for _, limitation := range result.Limitations {
			reasons = append(reasons, limitation.Reason)
		}
		want := []string{"attribute-value-unavailable"}
		if mixed {
			want = append(want, "model-lexical-pattern-unavailable")
		}
		if !result.Valid || result.Complete || !reflect.DeepEqual(reasons, want) {
			t.Fatalf("mixed=%t: attributes result=%+v, want limitations=%v", mixed, result, want)
		}
		report, err := session.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
		if err != nil {
			t.Fatal(err)
		}
		if report.Counts.Unknown != 0 || report.Counts.Unsupported != 0 || report.Counts.Blocked != 1 || report.BlockingProblems != 2 || report.AssessmentComplete || report.DiagnosticsTruncated {
			t.Fatalf("mixed=%t: report=%+v", mixed, report)
		}
		if len(report.Findings) != 2 {
			t.Fatalf("mixed=%t: findings=%+v", mixed, report.Findings)
		}
		failure, blocked := report.Findings[0], report.Findings[1]
		if failure.Category != CategoryModel || failure.Outcome != OutcomeFail || blocked.Outcome != OutcomeBlocked || blocked.Cause == nil || blocked.Cause.Rule != failure.Rule || blocked.Cause.Subject != failure.Subject {
			t.Fatalf("mixed=%t: real causal failure missing: %+v", mixed, report.Findings)
		}
		return report
	}
	dependent, mixed := run(false), run(true)
	// The report intentionally exposes composite outcomes, not a complete trace
	// of every capability limitation. Neither counts nor retained causal findings
	// may be used by a facade to discard incompleteness from the mixed case.
	if !reflect.DeepEqual(dependent.Counts, mixed.Counts) || dependent.BlockingProblems != mixed.BlockingProblems || !reflect.DeepEqual(dependent.Findings, mixed.Findings) {
		t.Fatalf("composite summary contract changed: dependent=%+v mixed=%+v", dependent, mixed)
	}
}
