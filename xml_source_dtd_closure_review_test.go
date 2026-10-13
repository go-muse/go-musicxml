package musicxml

import (
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceDTDStoreReviewContentClosureTracksIndependentDeclarations(t *testing.T) {
	s, _ := dtdStoreParse(t, ` r [<!ENTITY first '<x/>'><!ENTITY other '&other;'>]>`, sourceDTDStandaloneAbsent)
	first, _ := s.Entity("first", false)
	if err := s.entityContentRead(first); err != nil {
		t.Fatal(err)
	}
	if !s.State().EntityWellFormednessIncomplete {
		t.Error("unverified other entity was erased")
	}
	used, _ := s.account.Usage(validation.BudgetBufferedBytes)
	if err := s.entityContentRead(first); err != nil {
		t.Fatal(err)
	}
	again, _ := s.account.Usage(validation.BudgetBufferedBytes)
	if used != again {
		t.Errorf("duplicate verification retained bytes: %d -> %d", used, again)
	}
	if !first.RequiresContentCheck {
		t.Error("verification mutated declaration facts")
	}
}

func TestSourceDTDStoreReviewContentClosureCancellationAndBudget(t *testing.T) {
	for _, cancelVerification := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		a := dtdStoreAccount(t, validation.Budgets{})
		const foreign = int64(37)
		if err := a.Charge(validation.BudgetBufferedBytes, foreign); err != nil {
			t.Fatal(err)
		}
		s, err := newSourceDTDStore(ctx, a, sourceDTDStandaloneAbsent)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = parseSourceDTDWithStore(&sourceDTDTestInput{text: ` r [<!ENTITY e '<x/>'>]>`}, s); err != nil {
			t.Fatal(err)
		}
		e, _ := s.Entity("e", false)
		held := int64(0)
		if cancelVerification {
			cancel()
		} else {
			used, _ := a.Usage(validation.BudgetBufferedBytes)
			held = a.Limits().MaxBufferedBytes - used
			if err = a.Charge(validation.BudgetBufferedBytes, held); err != nil {
				t.Fatal(err)
			}
		}
		err = s.entityContentRead(e)
		var resource *validation.ResourceError
		if cancelVerification && !errors.Is(err, context.Canceled) || !cancelVerification && !errors.As(err, &resource) {
			t.Errorf("got %v", err)
		}
		if !s.State().EntityWellFormednessIncomplete {
			t.Error("failed verification erased obligation")
		}
		if err = s.Close(); err != nil {
			t.Fatal(err)
		}
		if held != 0 {
			if err = a.Release(validation.BudgetBufferedBytes, held); err != nil {
				t.Fatal(err)
			}
		}
		cancel()
		if used, _ := a.Usage(validation.BudgetBufferedBytes); used != foreign {
			t.Errorf("foreign ownership changed: %d", used)
		}
	}
}
