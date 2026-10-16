package musicxml

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceDTDIDEffectiveLexicalMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, decl, element, attribute, value string
		id, complete                          bool
	}{
		{"implied", `<!ATTLIST r a ID #IMPLIED>`, "r", "a", "  O1  ", true, true},
		{"required", `<!ATTLIST r a ID #REQUIRED>`, "r", "a", "O1", true, true},
		{"id-first", `<!ATTLIST r a ID #IMPLIED><!ATTLIST r a CDATA #IMPLIED>`, "r", "a", "O1", true, true},
		{"cdata-first", `<!ATTLIST r a CDATA #IMPLIED><!ATTLIST r a ID #IMPLIED>`, "r", "a", "O1", false, true},
		{"lexical-prefix", `<!ATTLIST p:r p:a ID #IMPLIED>`, "p:r", "p:a", "O1", true, true},
		{"expanded-homonym", `<!ATTLIST q:r q:a ID #IMPLIED>`, "p:r", "p:a", "O1", false, true},
		{"cutoff-before", `<!ENTITY % unread SYSTEM "missing.dtd">%unread;<!ATTLIST r a ID #IMPLIED>`, "r", "a", "O1", false, false},
		{"cutoff-after", `<!ATTLIST r a ID #IMPLIED><!ENTITY % unread SYSTEM "missing.dtd">%unread;`, "r", "a", "O1", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := validation.NewBudgetAccount(validation.Budgets{})
			if err != nil {
				t.Fatal(err)
			}
			source := `<!DOCTYPE ` + tc.element + ` [` + tc.decl + `]><` + tc.element + ` xmlns:p="urn:same" xmlns:q="urn:same" ` + tc.attribute + `="` + tc.value + `"/>`
			r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true, CaptureDTDIDs: true})
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			for {
				tok, err := r.Token()
				if err != nil {
					t.Fatal(err)
				}
				start, ok := tok.(xml.StartElement)
				if !ok {
					continue
				}
				for i, at := range start.Attr {
					if at.Name.Local != "a" {
						continue
					}
					m := r.metadata.Attributes[i]
					if (m.DTDID != nil) != tc.id || m.DTDIDDomainComplete != tc.complete {
						t.Fatalf("id=%v complete=%v metadata=%+v", m.DTDID != nil, m.DTDIDDomainComplete, m)
					}
					if tc.id {
						if !m.DTDID.Binding || !m.DTDID.ActuallyRead || at.Value != "O1" {
							t.Fatalf("not effective normalized ID: %+v %q", m.DTDID, at.Value)
						}
					}
				}
				break
			}
		})
	}
}
