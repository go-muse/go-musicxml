package validation

import (
	"context"
	"math/big"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func TestModelSchemaBridgeKeepsCurrentValue(t *testing.T) {
	ctx := context.Background()
	account, _ := NewBudgetAccount(Budgets{})
	s := &Session{profile: Profile{Target: TargetModel}, account: account}
	exact, err := NewDecimal(new(big.Rat).SetFloat64(0.1))
	if err != nil {
		t.Fatal(err)
	}
	n := &factNode{header: NodeHeader{ID: 1, Name: QName{Local: "root"}}, closed: true, end: NodeEnd{Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicDecimal, Decimal: exact}}}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelFields}}}
	got, err := s.schemaSubject(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CurrentModel || !got.Current.Known || got.TextKnown || got.Current.Value.Domain != xsd.NumberDomain || !got.ChildrenComplete || !got.ChildrenOrderKnown {
		t.Fatalf("subject=%+v", got)
	}
	if got.Current.Value.Number.Compare(xsd.Decimal{Fraction: "1"}) <= 0 {
		t.Fatal("binary-exact current 0.1 was rounded")
	}
	n.end.Order = Evidence[OrderKind]{State: EvidenceUnsupported, Reason: "content-hole"}
	got, err = s.schemaSubject(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChildrenOrderKnown || got.ChildrenComplete {
		t.Fatalf("hole was compacted into ordered input: %+v", got)
	}
}
