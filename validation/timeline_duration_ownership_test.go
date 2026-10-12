package validation

import (
	"context"
	"testing"
)

func TestTimelineDurationRetainsOnlyEscapingQuotient(t *testing.T) {
	for _, tc := range []struct{ raw, units int64 }{{1, 3}, {1000, 7}, {-1, 8}, {0, 9}} {
		s := rationalSession(t, Budgets{})
		t.Cleanup(s.cleanup)
		control := rationalSession(t, Budgets{})
		t.Cleanup(control.cleanup)
		raw, units := stateDecimal(t, tc.raw, 1), stateDecimal(t, tc.units, 1)
		want, err := control.rationalQuo(context.Background(), mustRational(t, tc.raw, 1), mustRational(t, tc.units, 1))
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.normalizeTimelineDuration(context.Background(), stateKnown(raw), stateKnown(units))
		if err != nil || got.State != EvidenceKnown || got.Value.String() != want.String() {
			t.Fatalf("normalization=%+v,%v", got, err)
		}
		if s.retained != control.retained {
			t.Fatalf("temporary decimal conversions retained: got%d want onlyquotient%d", s.retained, control.retained)
		}
		if raw.String() != stateDecimal(t, tc.raw, 1).String() || units.String() != stateDecimal(t, tc.units, 1).String() {
			t.Fatal("input mutated")
		}
		copy := got.Value.Value()
		copy.SetInt64(77)
		if got.Value.String() != want.String() {
			t.Fatal("quotient aliases exported mutable copy")
		}
	}
}
