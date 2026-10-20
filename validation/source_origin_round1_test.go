package validation

import (
	"context"
	"errors"
	"testing"
)

func TestSourceOriginRequiresCompleteAttributeEnumeration(t *testing.T) {
	for _, scope := range []string{"root", "child"} {
		for _, complete := range []Evidence[bool]{stateKnown(false), stateUnknown[bool]("not-read"), {State: EvidenceUnsupported, Reason: "dtd-infoset-unavailable"}, {State: EvidenceInvalid, Reason: "invalid", Cause: &DecisionRef{Rule: RuleKey{"reader", 1}, Subject: NodeRef{"source", 1}}}} {
			t.Run(scope+"/"+string(complete.State), func(t *testing.T) {
				_, s, key := sourceOriginFixture(t, Budgets{})
				ctx := context.Background()
				h := schemaPendingHeader()
				h.Location = sourceOriginPhysical().Physical.Value
				if err := s.OpenNode(ctx, h); err != nil {
					t.Fatal(err)
				}
				if err := s.sourceOrigin(ctx, key, sourceOriginPhysical()); err != nil {
					t.Fatal(err)
				}
				if scope == "child" {
					child := h
					child.ID = 2
					child.Parent = 1
					if err := s.OpenNode(ctx, child); err != nil {
						t.Fatal(err)
					}
					origin := sourceOriginPhysical()
					origin.Subject.Node = 2
					if err := s.sourceOrigin(ctx, key, origin); err != nil {
						t.Fatal(err)
					}
					end := schemaLexemeEnd("1")
					end.ID = 2
					end.AttributesComplete = complete
					if err := s.CloseNode(ctx, end); err != nil {
						t.Fatal(err)
					}
				}
				end := schemaLexemeEnd("1")
				if scope == "root" {
					end.AttributesComplete = complete
				}
				if err := s.CloseNode(ctx, end); err != nil {
					t.Fatal(err)
				}
				if err := s.completeSourceOrigins(ctx, key, stateKnown(true)); err != nil {
					t.Fatal(err)
				}
				s.inputComplete = true
				decision, pending, err := s.evaluateSourceOrigins(ctx)
				if err != nil || pending || decision.Outcome != OutcomeUnknown || decision.Reason != "source-origin-enumeration-incomplete" {
					t.Errorf("unobserved attributes allowed origin success: %+v pending=%v err=%v", decision, pending, err)
				}
				report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, f := range report.Findings {
					if f.Decision.Rule == key {
						found = true
						if f.Decision.Outcome != OutcomeUnknown {
							t.Errorf("origin finding %+v", f)
						}
					}
				}
				if !found {
					t.Error("origin provider cached success despite unobserved attributes")
				}
			})
		}
	}
}

func TestSourceOriginIngressValidatesPayloadOnce(t *testing.T) {
	for _, entity := range []bool{false, true} {
		_, s, key := sourceOriginFixture(t, Budgets{})
		ctx := context.Background()
		in := sourceOriginPhysical()
		h := schemaPendingHeader()
		h.Location = in.Physical.Value
		if entity {
			in.Kind = SourceOriginEntity
			in.Specified = stateKnown(false)
			in.Declaration = in.Physical
			in.Use = in.Physical
			in.Physical = stateAbsent[Location]()
			in.Ancestry = []SourceOriginStep{{Entity: "outer", Declaration: in.Declaration, Use: in.Use}, {Entity: "inner", Declaration: in.Declaration, Use: in.Use}}
			h.Location.Position = nil
		}
		if err := s.OpenNode(ctx, h); err != nil {
			t.Fatal(err)
		}
		before := rationalUsage(t, s, BudgetRuleSteps)
		if err := s.sourceOrigin(ctx, key, in); err != nil {
			t.Fatal(err)
		}
		ingress := rationalUsage(t, s, BudgetRuleSteps) - before
		probe := rationalSession(t, Budgets{})
		pub := ContextResult{Provider: key, Subject: in.Subject, Property: ContextPropertySourceOrigin, Value: stateAbsent[Atomic](), SourceOrigin: &in}
		if err := probe.publishContextResult(ctx, pub); err != nil {
			t.Fatal(err)
		}
		publication := rationalUsage(t, probe, BudgetRuleSteps)
		if ingress != publication {
			t.Errorf("entity=%v ingress=%d publication=%d; payload validation charged more than once", entity, ingress, publication)
		}
		s.cleanup()
		probe.cleanup()
	}
}

func TestSourceOriginRejectsContradictoryEntityAndDefaultShapes(t *testing.T) {
	for _, mode := range []string{"physical-entity-element", "entity-unknown-specified", "entity-unknown-physical", "default-ancestry"} {
		t.Run(mode, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			in := sourceOriginPublication()
			v := in.SourceOrigin
			v.Kind = SourceOriginEntity
			v.Declaration = v.Physical
			v.Use = v.Physical
			v.Ancestry = []SourceOriginStep{{Entity: "e", Declaration: v.Declaration, Use: v.Use}}
			switch mode {
			case "entity-unknown-specified":
				v.Specified = stateUnknown[bool]("unobserved")
			case "entity-unknown-physical":
				v.Attribute = &QName{Local: "x"}
				v.Physical = stateUnknown[Location]("unobserved")
			case "default-ancestry":
				v.Kind = SourceOriginDefault
				v.Attribute = &QName{Local: "x"}
				v.Specified = stateKnown(false)
				v.Physical = stateAbsent[Location]()
			}
			if err := s.publishContextResult(context.Background(), in); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("accepted contradictory %s: %v", mode, err)
			}
			if len(s.contextResults) != 0 || s.retained != 0 {
				t.Error("malformed origin partially published")
			}
		})
	}
}

func TestSourceOriginTypedRetentionAvoidsPerFieldReserves(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	in := sourceOriginPhysical()
	if _, err := cloneSourceOriginContext(context.Background(), s, &in); err != nil {
		t.Fatal(err)
	}
	// The closed fixed record plus one physical site should fit this conservative
	// allowance, which includes the allocator rounding of each owned string.
	// Every context envelope/map reserve is separate from this payload.
	bytes := rationalUsage(t, s, BudgetBufferedBytes)
	if bytes > 1280 {
		t.Errorf("physical origin payload retained %d logical bytes; fixed headers were counted repeatedly", bytes)
	}
}
