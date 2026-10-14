package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourcePhysicalLexicalCapture(t *testing.T) {
	for _, tc := range []struct {
		name, input, element, effective, raw string
		state                                validation.EvidenceState
		enabled                              bool
	}{
		{"physical-before-tokenized", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS #IMPLIED>]><r a="  one   two  "/>`, "r", "one two", "  one   two  ", validation.EvidenceKnown, true},
		{"physical-xml-normalized", "<!DOCTYPE r [<!ATTLIST r a CDATA #IMPLIED>]><r a=\"a\r\nb&#x9;&amp;c\"/>", "r", "a b\t&c", "a b\t&c", validation.EvidenceKnown, true},
		{"physical-general-replacement", `<!DOCTYPE r [<!ENTITY e 'one'><!ATTLIST r a NMTOKENS #IMPLIED>]><r a="  &e;  two "/>`, "r", "one two", "", validation.EvidenceUnknown, true},
		{"defaulted", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS ' one   two '>]><r/>`, "r", "one two", "", validation.EvidenceAbsent, true},
		{"entity-generated", `<!DOCTYPE r [<!ENTITY e '<child a="one"/>'>]><r>&e;</r>`, "child", "one", "", validation.EvidenceAbsent, true},
		{"unselected", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS #IMPLIED>]><r a=" one   two "/>`, "r", "one two", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := dtdStoreAccount(t, validation.Budgets{})
			config := sourceXMLConfig{Context: context.Background(), Account: account, InternalDTD: true}
			config.CaptureAttributeLexical = tc.enabled
			reader, err := newSourceXMLTokenReader(strings.NewReader(tc.input), config)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for {
				token, err := reader.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				start, ok := token.(xml.StartElement)
				if !ok || start.Name.Local != tc.element {
					continue
				}
				for i, attribute := range start.Attr {
					if attribute.Name.Local != "a" {
						continue
					}
					found = true
					raw := reader.Metadata().Attributes[i].Lexical
					if attribute.Value != tc.effective || raw.State != tc.state || raw.Value != tc.raw {
						t.Errorf("effective=%q raw=%+v, want effective=%q raw=%s/%q", attribute.Value, raw, tc.effective, tc.state, tc.raw)
					}
				}
			}
			if !found {
				t.Error("expected attribute missing")
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
				t.Errorf("live buffer charge=%d", used)
			}
		})
	}
}
