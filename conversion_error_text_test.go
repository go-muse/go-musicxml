package musicxml

import (
	"errors"
	"strconv"
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
