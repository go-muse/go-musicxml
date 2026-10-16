package xsdgen

import (
	"encoding/xml"
	"testing"
)

// Accepted pre-existing limitation, not a claim of full namespace-list support:
// the string DTO loses presence for an explicit empty namespace attribute.
func TestAnyAttributeEmptyNamespacePresenceLimitation(t *testing.T) {
	for _, tc := range []struct{ attribute, want string }{
		{"", ""}, {` namespace=""`, ""}, {` namespace=" "`, " "},
	} {
		var value AnyAttribute
		if err := xml.Unmarshal([]byte(`<anyAttribute`+tc.attribute+`/>`), &value); err != nil {
			t.Fatal(err)
		}
		if value.Namespace != tc.want {
			t.Fatalf("attribute %q: namespace=%q, want %q; update the documented limitation if presence is preserved", tc.attribute, value.Namespace, tc.want)
		}
	}
}
