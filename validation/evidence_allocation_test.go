package validation

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func checkUnavailableEvidence[T comparable](t *testing.T, nonzero T) {
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
			if err := validateEvidence(evidence, "test"); err != nil {
				t.Fatal(err)
			}
		}); allocations != 0 {
			t.Errorf("%T %s validation allocated %v; want 0", nonzero, state, allocations)
		}
		evidence.Value = nonzero
		if err := validateEvidence(evidence, "test"); err == nil {
			t.Errorf("%T %s accepted a nonzero unavailable value", nonzero, state)
		}
	}
}

func TestEvidenceZeroChecksAvoidReflectionAllocations(t *testing.T) {
	// All session-ingress payloads have comparable representations. In
	// particular, an initialized exact numeric zero is not an absent payload.
	checkUnavailableEvidence(t, true)
	checkUnavailableEvidence(t, "value")
	checkUnavailableEvidence(t, OrderModelFields)
	checkUnavailableEvidence(t, ComponentID{Assembly: AssemblyMusicXML})
	checkUnavailableEvidence(t, DeclarationID{Assembly: AssemblyMusicXML})
	checkUnavailableEvidence(t, Atomic{Kind: AtomicString})
	integer, err := NewInteger(new(big.Int))
	if err != nil {
		t.Fatal(err)
	}
	decimal, err := NewDecimal(new(big.Rat))
	if err != nil {
		t.Fatal(err)
	}
	checkUnavailableEvidence(t, Atomic{Integer: integer})
	checkUnavailableEvidence(t, Atomic{Decimal: decimal})
	checkUnavailableEvidence(t, Atomic{Integer: Integer{text: "0"}})
	checkUnavailableEvidence(t, Atomic{Decimal: Decimal{bytes: 1}})
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
