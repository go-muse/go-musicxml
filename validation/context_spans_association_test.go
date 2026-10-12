package validation

import (
	"context"
	"testing"
)

func spanTestEndpoint(t *testing.T, id NodeID, family SpanFamily, role string, number int64) *spanEndpoint {
	t.Helper()
	return &spanEndpoint{ref: NodeRef{"d", id}, complete: Evidence[bool]{State: EvidenceKnown, Value: true}, payload: SpanContextResult{Family: family, Role: Evidence[string]{State: EvidenceKnown, Value: role}, Domain: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"d", 100}}, Anchor: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"d", id + 1000}}, Number: Evidence[Atomic]{State: EvidenceKnown, Value: localInteger(t, number)}, Identity: SpanIdentityNumber, Voice: Evidence[string]{State: EvidenceAbsent}, Staff: Evidence[Integer]{State: EvidenceAbsent}, Association: stateUnknown[bool]("span-not-associated")}}
}
func TestContextSpanDocumentOrderAssociation(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	var es []*spanEndpoint
	for i, role := range []string{"start", "stop", "stop", "start", "start", "stop"} {
		es = append(es, spanTestEndpoint(t, NodeID(i+1), SpanSlur, role, 1))
	}
	if err := associateSpanEndpoints(context.Background(), s, es, nil); err != nil {
		t.Fatal(err)
	}
	for i, e := range es {
		if e.payload.Association.State != EvidenceKnown || !e.payload.Association.Value || len(e.payload.Members) != 2 {
			t.Fatalf("endpoint %d = %+v", i, e.payload)
		}
	}
	if es[2].payload.Members[0].Node != 4 || es[2].payload.Members[1].Node != 3 {
		t.Fatalf("semantic endpoint order lost: %+v", es[2].payload.Members)
	}
}
func TestContextSpanIsolationAndUnknownGap(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]*spanEndpoint)
	}{
		{"different-number", func(e []*spanEndpoint) { e[1].payload.Number.Value = localInteger(t, 2) }},
		{"different-part", func(e []*spanEndpoint) { e[1].payload.Domain.Value.Node = 101 }},
		{"different-family", func(e []*spanEndpoint) { e[1].payload.Family = SpanSlide }},
		{"eligible-gap", func(e []*spanEndpoint) { e[0].complete = stateUnknown[bool]("test-gap") }},
		{"same-role", func(e []*spanEndpoint) { e[1].payload.Role.Value = "start" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			es := []*spanEndpoint{spanTestEndpoint(t, 1, SpanSlur, "start", 1), spanTestEndpoint(t, 2, SpanSlur, "stop", 1)}
			tc.change(es)
			if err := associateSpanEndpoints(context.Background(), s, es, nil); err != nil {
				t.Fatal(err)
			}
			for _, e := range es {
				if e.payload.Association.State == EvidenceKnown {
					t.Fatalf("fabricated relation: %+v", e.payload)
				}
			}
		})
	}
}
func TestContextSpanFamilyExceptions(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	e := spanTestEndpoint(t, 1, SpanTied, "let-ring", 1)
	if err := associateSpanEndpoints(context.Background(), s, []*spanEndpoint{e}, nil); err != nil {
		t.Fatal(err)
	}
	if e.payload.Association.State != EvidenceKnown || len(e.payload.Members) != 1 {
		t.Fatalf("let-ring %+v", e.payload)
	}
	a, b := spanTestEndpoint(t, 2, SpanTied, "start", 1), spanTestEndpoint(t, 3, SpanTied, "stop", 1)
	a.payload.Identity = SpanIdentityImpliedTie
	a.payload.Number = Evidence[Atomic]{State: EvidenceAbsent}
	a.pitch = "60"
	b.pitch = "60"
	if err := associateSpanEndpoints(context.Background(), s, []*spanEndpoint{a, b}, nil); err != nil {
		t.Fatal(err)
	}
	if a.payload.Association.State == EvidenceKnown || b.payload.Association.State == EvidenceKnown {
		t.Fatal("implicit tied collapsed onto explicit 1")
	}
}
func TestContextBeamGroupSeparatesGraceCueAndVoice(t *testing.T) {
	for _, change := range []func(*spanEndpoint){func(e *spanEndpoint) { e.grace = true }, func(e *spanEndpoint) { e.cue = true }, func(e *spanEndpoint) { e.payload.Voice = Evidence[string]{State: EvidenceKnown, Value: "2"} }} {
		s := rationalSession(t, Budgets{})
		a, b := spanTestEndpoint(t, 1, SpanBeam, "begin", 1), spanTestEndpoint(t, 2, SpanBeam, "end", 1)
		a.payload.Identity = SpanIdentityBeamLevel
		b.payload.Identity = SpanIdentityBeamLevel
		change(b)
		if err := associateSpanEndpoints(context.Background(), s, []*spanEndpoint{a, b}, nil); err != nil {
			t.Fatal(err)
		}
		if a.payload.Association.State == EvidenceKnown {
			t.Fatal("beam groups merged")
		}
		s.cleanup()
	}
}
func TestContextSpanCloneOwnsMembersAndCause(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	e := spanTestEndpoint(t, 1, SpanSlur, "start", 1).payload
	e.Members = []NodeRef{{"d", 2}}
	e.Association = Evidence[bool]{State: EvidenceInvalid, Reason: "bad", Cause: &DecisionRef{Rule: RuleKey{"r", 1}, Subject: NodeRef{"d", 3}}}
	c, err := cloneSpanContext(context.Background(), s, &e)
	if err != nil {
		t.Fatal(err)
	}
	e.Members[0].Node = 9
	e.Association.Cause.Subject.Node = 8
	if c.Members[0].Node != 2 || c.Association.Cause.Subject.Node != 3 {
		t.Fatal("borrowed output")
	}
}
