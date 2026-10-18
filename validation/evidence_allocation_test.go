package validation

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func checkUnavailableEvidence[T comparable](t *testing.T, nonzero T) {
	t.Helper()
	checkUnavailableEvidenceWith(t, nonzero, validateEvidence[T])
}

func checkUnavailableEvidenceWith[T any](t *testing.T, nonzero T, validate func(Evidence[T], string) error) {
	t.Helper()
	for _, state := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		evidence := Evidence[T]{State: state}
		if state != EvidenceAbsent {
			evidence.Reason = "unavailable"
		}
		if state == EvidenceInvalid {
			evidence.Cause = &DecisionRef{Rule: RuleKey{ID: "cause", Revision: 1}, Subject: NodeRef{DocumentID: "document", Node: 1}}
		}
		if allocations := testing.AllocsPerRun(10, func() {
			if err := validate(evidence, "test"); err != nil {
				t.Fatal(err)
			}
		}); allocations != 0 {
			t.Errorf("%T %s validation allocated %v; want 0", nonzero, state, allocations)
		}
		evidence.Value = nonzero
		if err := validate(evidence, "test"); err == nil {
			t.Errorf("%T %s accepted a nonzero unavailable value", nonzero, state)
		}
	}
}

func TestEvidenceZeroChecksAvoidReflectionAllocations(t *testing.T) {
	// Atomic payloads are not comparable and use explicit zero checks. In
	// particular, an initialized exact numeric zero is not an absent payload.
	checkUnavailableEvidence(t, true)
	checkUnavailableEvidence(t, "value")
	checkUnavailableEvidence(t, OrderModelFields)
	checkUnavailableEvidence(t, ComponentID{Assembly: AssemblyMusicXML})
	checkUnavailableEvidence(t, DeclarationID{Assembly: AssemblyMusicXML})
	checkUnavailableEvidenceWith(t, Atomic{Kind: AtomicString}, validateAtomicEvidence)
	integer, err := NewInteger(new(big.Int))
	if err != nil {
		t.Fatal(err)
	}
	decimal, err := NewDecimal(new(big.Rat))
	if err != nil {
		t.Fatal(err)
	}
	checkUnavailableEvidenceWith(t, Atomic{Integer: integer}, validateAtomicEvidence)
	checkUnavailableEvidenceWith(t, Atomic{Decimal: decimal}, validateAtomicEvidence)
	checkUnavailableEvidenceWith(t, Atomic{Integer: Integer{text: "0"}}, validateAtomicEvidence)
	checkUnavailableEvidenceWith(t, Atomic{Decimal: Decimal{bytes: 1}}, validateAtomicEvidence)
}

func TestEvidenceStateCopiesDetachWithoutAllocation(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceKnown, EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		evidence := Evidence[bool]{State: EvidenceState(strings.Clone(string(state)))}
		if allocations := testing.AllocsPerRun(10, func() {
			if got := cloneEvidence(evidence); got.State != state {
				t.Fatal("state changed")
			}
		}); allocations != 0 {
			t.Errorf("%s copy allocated %v; want 0", state, allocations)
		}
		catalogPadMetadataStrings(reflect.ValueOf(&evidence).Elem())
		owned := cloneEvidence(evidence)
		catalogCheckDetachedStrings(t, reflect.ValueOf(evidence), reflect.ValueOf(owned), "evidence")
	}
	// Clone remains defensive for strings outside the closed state vocabulary.
	evidence := Evidence[bool]{State: "future-state", Reason: "caller-reason", Cause: &DecisionRef{Rule: RuleKey{ID: "rule", Revision: 1}, Subject: NodeRef{DocumentID: "document", Node: 1}}}
	catalogPadMetadataStrings(reflect.ValueOf(&evidence).Elem())
	owned := cloneEvidence(evidence)
	catalogCheckDetachedStrings(t, reflect.ValueOf(evidence), reflect.ValueOf(owned), "extensible evidence")
	owned.Cause.Subject.Node = 2
	if evidence.Cause.Subject.Node != 1 {
		t.Fatal("cause ownership lost")
	}
}
