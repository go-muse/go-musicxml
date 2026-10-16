package validation

import (
	"context"
	"testing"
)

func TestResourceMediaDuplicateReasonIndependentOfValue(t *testing.T) {
	for _, value := range []string{
		`application/vnd.recordare.musicxml+xml; charset=utf-8; charset=utf-8`,
		`application/vnd.recordare.musicxml+xml; CHARSET=utf-8; Charset=latin1`,
		`image/png; foo="a;b"; FOO="different"`,
	} {
		a, err := NewBudgetAccount(Budgets{})
		if err != nil {
			t.Fatal(err)
		}
		got, err := ClassifyResourceMediaType(context.Background(), a, value)
		if err != nil || got.State != EvidenceUnknown || got.Reason != "media-type-duplicate-parameter" {
			t.Errorf("%q: got %+v, error %v", value, got, err)
		}
		if live, _ := a.Usage(BudgetBufferedBytes); live != 0 {
			t.Errorf("retained %d bytes", live)
		}
	}
}
