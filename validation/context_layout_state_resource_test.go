package validation

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestLayoutStateExactCoordinateAndResources(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	clef := clefState{Sign: Evidence[string]{State: EvidenceKnown, Value: "G"}, Line: Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[2]}, Additional: Evidence[bool]{State: EvidenceAbsent}, OctaveChange: Evidence[Integer]{State: EvidenceAbsent}}
	for _, c := range []struct {
		step   string
		octave int
		want   string
	}{{"G", 4, "2"}, {"A", 4, "3"}, {"C", 4, "-2"}, {"G", 3, "-5"}} {
		got, e := layoutStateStaffStep(context.Background(), s, Evidence[string]{State: EvidenceKnown, Value: c.step}, Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[c.octave]}, clef)
		if e != nil || got.State != EvidenceKnown || got.Value.String() != c.want {
			t.Fatalf("%+v %+v %v", c, got, e)
		}
	}
	huge, _ := new(big.Int).SetString("1000000000000000000000000000000", 10)
	line, _ := NewInteger(huge)
	clef.Line.Value = line
	got, e := layoutStateStaffStep(context.Background(), s, Evidence[string]{State: EvidenceKnown, Value: "G"}, Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[4]}, clef)
	if e != nil || got.Value.String() != "1999999999999999999999999999998" {
		t.Fatalf("huge line %+v %v", got, e)
	}
	for _, cancel := range []bool{false, true} {
		limits := Budgets{MaxBufferedBytes: 1}
		if cancel {
			limits = Budgets{}
		}
		limited := rationalSession(t, limits)
		ctx, stop := context.WithCancel(context.Background())
		if cancel {
			stop()
		}
		_, err := layoutStatePrecedingStaff(ctx, limited, line)
		if cancel {
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		} else if !errors.Is(err, ErrResourceLimit) {
			t.Fatal(err)
		}
		limited.cleanup()
		used, _ := limited.account.Usage(BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("transient leak %d", used)
		}
		stop()
	}
}
func TestLayoutStateCoordinateMissingClefLineNotStandardGuess(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	clef := clefState{Sign: Evidence[string]{State: EvidenceKnown, Value: "G"}, Line: Evidence[Integer]{State: EvidenceAbsent}, Additional: Evidence[bool]{State: EvidenceAbsent}, OctaveChange: Evidence[Integer]{State: EvidenceAbsent}}
	got, err := layoutStateStaffStep(context.Background(), s, Evidence[string]{State: EvidenceKnown, Value: "G"}, Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[4]}, clef)
	if err != nil || got.State != EvidenceUnknown {
		t.Fatalf("absent G line guessed %+v %v", got, err)
	}
	clef.Sign.Value = "percussion"
	got, err = layoutStateStaffStep(context.Background(), s, Evidence[string]{State: EvidenceKnown, Value: "G"}, Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[4]}, clef)
	if err != nil || got.State != EvidenceKnown || got.Value.String() != "2" {
		t.Fatalf("percussion reference %+v %v", got, err)
	}
}

func TestLayoutStateMissingDeclarationBecomesGapNotConfigurationFailure(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	r := &contextReader{ctx: context.Background(), s: s}
	index := layoutStateIndex{}
	b := timelineMeasureBlock{part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 1}}, ordinal: 2}
	if err := index.symbolGap(r, b, Evidence[bool]{State: EvidenceAbsent}); err != nil {
		t.Fatal(err)
	}
	if len(index.Symbols.Gaps) != 1 || index.Symbols.Gaps[0].Availability.State != EvidenceUnknown {
		t.Fatal(index.Symbols.Gaps)
	}
}
