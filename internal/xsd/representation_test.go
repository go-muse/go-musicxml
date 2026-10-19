package xsd

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestModelRepresentationRepresents(t *testing.T) {
	decimal, _ := ModelRepresentationOf("decimal")
	positive, _ := ModelRepresentationOf("positiveInteger")
	signed, _ := ModelRepresentationOf("integer")
	small, _ := ModelRepresentationOf("unsignedByte")
	for _, test := range []struct {
		representation ModelRepresentation
		value          string
		want           bool
	}{
		{decimal, "0", true}, {decimal, "-0.000", true}, {decimal, "0.1", true}, {decimal, "+01.50", true},
		{decimal, "123456.789", true}, {decimal, "0.30000000000000004", true},
		{decimal, "1.00000000000000000001", false}, {decimal, "0.1000000000000000055511151231257827021181583404541015625", false},
		{decimal, "1" + strings.Repeat("0", 400), false}, {decimal, "0." + strings.Repeat("0", 400) + "1", false},
		{decimal, "1e0", false},
		{decimal, "10000000000000000000000", true}, {decimal, "1000000000000000000", true}, {decimal, "100000000000000000000", true},
		{decimal, strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64), true}, {decimal, "-" + strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64), true},
		{decimal, "0." + strings.Repeat("0", 323) + "5", true}, {decimal, "9007199254740993", false}, {decimal, "9007199254740992", true}, {decimal, "12345678901234567", false},
		{decimal, "1.0000000000000001", false}, {decimal, "1" + strings.Repeat("0", 309), false},
		{positive, "18446744073709551615", true}, {positive, "18446744073709551616", false}, {positive, "-1", false}, {positive, "1.5", false},
		{signed, "-9223372036854775808", true}, {signed, "9223372036854775808", false},
		{small, "255", true}, {small, "256", false},
	} {
		value, ok := ParseDecimal(test.value)
		if got := ok && test.representation.Represents(value); got != test.want {
			t.Errorf("%q: got %t, want %t", test.value, got, test.want)
		}
	}
	if _, ok := ModelRepresentationOf("string"); ok {
		t.Error("a string type has a numeric representation")
	}
}
