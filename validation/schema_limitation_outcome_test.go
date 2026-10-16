package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func TestSourceSchemaUnknownLimitationOrderAndPrecedence(t *testing.T) {
	unavailable := xsd.Limitation{Reason: "xsd-assessment-incomplete"}
	unknown := xsd.Limitation{Unknown: true, Reason: "source-entity-declarations-unavailable"}
	for _, reverse := range []bool{false, true} {
		for _, mode := range []string{"unknown", "fail", "blocked", "unsupported"} {
			t.Run(mode+map[bool]string{true: "/unknown-first", false: "/unknown-last"}[reverse], func(t *testing.T) {
				s, rule := schemaDiagnosticSession(t, Budgets{})
				defer s.cleanup()
				n := schemaDiagnosticNode(s, 1)
				limitations := []xsd.Limitation{unavailable, unknown}
				if reverse {
					limitations[0], limitations[1] = limitations[1], limitations[0]
				}
				result := xsd.Result{Applicable: true, Valid: true, Complete: false, Limitations: limitations}
				want, reason := OutcomeUnknown, Code(unknown.Reason)
				switch mode {
				case "fail":
					result.Valid = false
					want, reason = OutcomeFail, "xsd-constraint-violation"
				case "blocked":
					s.schema.nodes = map[NodeID]schemaNode{1: {cause: &DecisionRef{Rule: rule.Key, Subject: NodeRef{DocumentID: "source", Node: 99}}}}
					want = OutcomeBlocked
				case "unsupported":
					result.Limitations = []xsd.Limitation{unavailable}
					want, reason = OutcomeUnsupported, Code(unavailable.Reason)
				}
				if err := s.recordXSDResult(context.Background(), n, rule, result); err != nil {
					t.Fatal(err)
				}
				d, ok := s.lookupDecision(decisionID{rule.Key, n.header.ID})
				if !ok || d.Outcome != want || d.Reason != reason || (d.Cause != nil) != (mode == "blocked") {
					t.Fatalf("limitation order changed result, want %s/%s: %+v", want, reason, d)
				}
			})
		}
	}
}

func TestSourceSchemaUnknownLimitationScanCancellationAndWork(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		s, rule := schemaDiagnosticSession(t, Budgets{})
		n := schemaDiagnosticNode(s, 1)
		result := xsd.Result{Applicable: true, Valid: true, Limitations: []xsd.Limitation{{Reason: "xsd-assessment-incomplete"}, {Unknown: true, Reason: "source-entity-declarations-unavailable"}}}
		ctx, cancel := context.WithCancel(context.Background())
		want := ErrResourceLimit
		if canceled {
			cancel()
			want = context.Canceled
		} else {
			used, _ := s.account.Usage(BudgetRuleSteps)
			if err := s.account.Charge(BudgetRuleSteps, s.account.Limits().MaxRuleSteps-used-1); err != nil {
				t.Fatal(err)
			}
		}
		err := s.recordXSDResult(ctx, n, rule, result)
		cancel()
		if !errors.Is(err, want) {
			t.Fatalf("canceled=%t: want %v, got %v", canceled, want, err)
		}
		if s.accumulator.report.Counts.Unsupported != 0 || s.accumulator.report.Counts.Unknown != 0 {
			t.Fatal("interrupted scan invented a completed capability outcome")
		}
		s.cleanup()
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("cleanup retained %d bytes", used)
		}
	}
}

func TestSourceSchemaEntityUnknownSurvivesEarlierQNameLimitation(t *testing.T) {
	for _, entityFirst := range []bool{false, true} {
		s, rule := schemaDiagnosticSession(t, Budgets{})
		n := schemaDiagnosticNode(s, 1)
		typ := &xsd.ComplexSchema{Form: xsd.ComplexDirect, Attributes: []xsd.AttributeSchema{
			{Name: xsd.QName{Local: "q"}, Type: xsd.TypeRef{Name: xsd.QName{Space: xsd.XSDNamespace, Local: "QName"}}},
			{Name: xsd.QName{Local: "e"}, Type: xsd.TypeRef{Name: xsd.QName{Space: xsd.XSDNamespace, Local: "ENTITY"}}},
		}}
		compiled, err := xsd.Compile(xsd.SchemaData{Elements: map[xsd.QName]*xsd.ElementSchema{{Local: "n"}: {Name: xsd.QName{Local: "n"}, Type: xsd.TypeRef{InlineComplex: typ}}}}, xsd.CompileLimits{})
		if err != nil {
			t.Fatal(err)
		}
		run, err := xsd.NewRunWithSourceDeclarations(compiled, schemaMeter{s}, nil)
		if err != nil {
			t.Fatal(err)
		}
		attrs := []xsd.Attribute{
			{Attr: xsd.Attr{Name: xsd.QName{Local: "q"}, Value: "p:name"}, ValueKnown: true},
			{Attr: xsd.Attr{Name: xsd.QName{Local: "e"}, Value: "asset"}, ValueKnown: true},
		}
		if entityFirst {
			attrs[0], attrs[1] = attrs[1], attrs[0]
		}
		use, _ := compiled.Root(xsd.QName{Local: "n"})
		bound, err := run.Bind(context.Background(), use, xsd.Subject{ID: 1, Name: xsd.QName{Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: attrs})
		if err != nil {
			t.Fatal(err)
		}
		result := run.Results(bound).Attributes
		if !result.Valid || result.Complete || len(result.Limitations) != 2 {
			t.Fatalf("mixed attribute kernel result: %+v", result)
		}
		if err := s.recordXSDResult(context.Background(), n, rule, result); err != nil {
			t.Fatal(err)
		}
		d, ok := s.lookupDecision(decisionID{rule.Key, n.header.ID})
		if !ok || d.Outcome != OutcomeUnknown || d.Reason != "source-entity-declarations-unavailable" {
			t.Fatalf("entity-first=%t: %+v", entityFirst, d)
		}
		if err := run.Close(); err != nil {
			t.Fatal(err)
		}
		s.cleanup()
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("cleanup retained %d", used)
		}
	}
}
