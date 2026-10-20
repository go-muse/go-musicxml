package musicxml

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// A conversion error names its state and its cause; the cause still matches.
func TestConversionErrorTextIncludesCause(t *testing.T) {
	cause := &strconv.NumError{Func: "ParseFloat", Num: "x", Err: strconv.ErrSyntax}
	err := &ConversionError{State: validation.ConversionFailed, Cause: cause}
	if got, want := err.Error(), `musicxml: model conversion failed: strconv.ParseFloat: parsing "x": invalid syntax`; got != want {
		t.Fatalf("%q, want %q", got, want)
	}
	if got := (&ConversionError{State: validation.ConversionIncomplete}).Error(); got != "musicxml: model conversion incomplete" {
		t.Fatalf("without cause: %q", got)
	}
	if !errors.Is(err, strconv.ErrSyntax) {
		t.Fatal("cause no longer matches")
	}
}

// A conversion that fails on the source's own syntax error does not repeat
// that error beside the conversion error that already wraps it: the text
// names it once for the conversion and once as the assessment's cause.
func TestDecodeAssessedDoesNotRepeatTheSourceFailure(t *testing.T) {
	_, err := DecodeAssessed(context.Background(), strings.NewReader(`<score-partwise version="4.0"><part-list>`), DecodeAssessmentOptions{})
	if err == nil {
		t.Fatal("truncated source accepted")
	}
	var conversion *ConversionError
	if !errors.As(err, &conversion) || !errors.Is(err, ErrIncompleteAssessment) {
		t.Fatalf("error classes changed: %v", err)
	}
	if n := strings.Count(err.Error(), "unexpected EOF"); n != 2 {
		t.Fatalf("source failure named %d times: %q", n, err.Error())
	}
}
