package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClassifyResourceMediaTypePreservesUnknownAndErrors(t *testing.T) {
	a, err := NewBudgetAccount(Budgets{MaxScalarBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		value          string
		state          EvidenceState
		representation ResourceRepresentation
		reason         Code
	}{
		{"application/vnd.recordare.musicxml+xml; CHARSET=\"utf-8\"", EvidenceKnown, ResourceSourceXML, ""},
		{"application/vnd.recordare.musicxml", EvidenceKnown, ResourceRawMXL, ""},
		{"image/png", EvidenceKnown, ResourceBinary, ""},
		{"audio/ogg", EvidenceKnown, ResourceBinary, ""},
		{"video/mp4", EvidenceKnown, ResourceBinary, ""},
		{"application/vnd.recordare.musicxml; charset=utf-8", EvidenceKnown, ResourceRawMXL, ""},
		{"application/example", EvidenceUnknown, "", "musicxml-media-type-set-unresolved"},
		{"", EvidenceUnknown, "", "media-type-syntax-unresolved"},
		{"application/vnd.recordare.musicxml+xml; broken", EvidenceUnknown, "", "media-type-syntax-unresolved"},
	} {
		got, err := ClassifyResourceMediaType(context.Background(), a, tc.value)
		if err != nil || got.State != tc.state || got.Value != tc.representation || got.Reason != tc.reason {
			t.Errorf("%q got=%+v err=%v", tc.value, got, err)
		}
		if err := validateEvidence(got, "media classification"); err != nil {
			t.Errorf("invalid returned evidence: %v", err)
		}
		if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
			t.Errorf("leaked %d", used)
		}
	}
	if _, err := ClassifyResourceMediaType(context.Background(), a, strings.Repeat("x", 4097)); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("scalar bound err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ClassifyResourceMediaType(ctx, a, "image/png"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancel err=%v", err)
	}
	if _, err := ClassifyResourceMediaType(nil, a, "image/png"); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("nil context err=%v", err)
	}
	if _, err := ClassifyResourceMediaType(context.Background(), nil, "image/png"); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("nil account err=%v", err)
	}
}
func TestClassifyResourceMediaTypeReleasesScratchOnCancellation(t *testing.T) {
	a, _ := NewBudgetAccount(Budgets{})
	if _, err := ClassifyResourceMediaType(&reviewCancelAfterParse{}, a, "application/vnd.recordare.musicxml"); !errors.Is(err, context.Canceled) {
		t.Errorf("err=%v", err)
	}
	if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
		t.Errorf("leaked %d", used)
	}
}
