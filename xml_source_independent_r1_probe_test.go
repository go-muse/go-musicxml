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

// WFC checks at every later occurrence must remain active after proof discharge.
func TestIndependentR1ContentProofScope(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		complete    bool
		malformed   bool
	}{
		{"verified-then-invalid-attribute", `<!DOCTYPE r [<!ENTITY e "<x/>">]><r>&e;<x a="&e;"/></r>`, false, true},
		{"verified-then-namespace-invalid", `<!DOCTYPE r [<!ENTITY e "<p:x/>">]><r><x xmlns:p="urn:x">&e;</x>&e;</r>`, false, true},
		{"unused-other-record", `<!DOCTYPE r [<!ENTITY e "<x/>"><!ENTITY unused "&unknown;">]><r>&e;</r>`, false, false},
		{"same-name-ignored-recursive", `<!DOCTYPE r [<!ENTITY e "<x/>"><!ENTITY e "&e;">]><r>&e;</r>`, true, false},
		{"numeric-formed-invalid-content", `<!DOCTYPE r [<!ENTITY e "&#38;#0;">]><r>&e;</r>`, false, true},
		{"attribute-nested-closure-unproved", `<!DOCTYPE r [<!ENTITY e "]]>"><!ENTITY alias "&e;">]><r a="&alias;"/>`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, s, err := sourceEntityScan(t, tc.input)
			var syntax *xml.SyntaxError
			if errors.As(err, &syntax) != tc.malformed || !tc.malformed && (err != nil || s.SyntaxComplete != tc.complete) {
				t.Fatalf("error=%v summary=%+v", err, s)
			}
		})
	}
}

func TestIndependentR1BorrowedOriginNestedATTLIST(t *testing.T) {
	input := `<!DOCTYPE r [<!ENTITY % leaf "<!ATTLIST r a CDATA 'a' b CDATA 'b'>"><!ENTITY % middle "&#37;leaf;<!ENTITY e 'e'>"><!ENTITY % outer "&#37;middle;"><!ENTITY % other "<!ATTLIST r c CDATA 'c'>">%outer;%other;]><r/>`
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		d, ok := r.dtdStore.Attribute("r", name)
		if !ok {
			t.Fatal(name)
		}
		if len(d.Origin.Ancestry) != 3 || d.Origin.Ancestry[0].Entity != "outer" || d.Origin.Ancestry[1].Entity != "middle" || d.Origin.Ancestry[2].Entity != "leaf" {
			t.Fatalf("%s origin=%+v", name, d.Origin)
		}
	}
	e, _ := r.dtdStore.Entity("e", false)
	if len(e.Origin.Ancestry) != 2 || e.Origin.Ancestry[1].Entity != "middle" {
		t.Fatalf("entity origin=%+v", e.Origin)
	}
	c, _ := r.dtdStore.Attribute("r", "c")
	if len(c.Origin.Ancestry) != 1 || c.Origin.Ancestry[0].Entity != "other" {
		t.Fatalf("c origin=%+v", c.Origin)
	}
	if r.dtdOriginScratch != nil {
		t.Fatal("scratch retained past doctype")
	}
	for err == nil {
		_, err = r.Token()
	}
	if err != io.EOF {
		t.Fatal(err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if used, _ := a.Usage(validation.BudgetBufferedBytes); used != 0 {
		t.Fatal(used)
	}
}

func TestIndependentR1ContentProofCancellationPrecedence(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	ctx, cancel := context.WithCancel(context.Background())
	r, err := newSourceXMLTokenReader(strings.NewReader(`<!DOCTYPE r [<!ENTITY e "<x/>">]><r>&e;</r>`), sourceXMLConfig{Context: ctx, Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	for {
		tok, e := r.Token()
		if e != nil {
			t.Fatal(e)
		}
		if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "x" {
			break
		}
	}
	r.releaseToken()
	used, _ := a.Usage(validation.BudgetBufferedBytes)
	held := a.Limits().MaxBufferedBytes - used
	if err = a.Charge(validation.BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
	cancel()
	_, err = r.Token()
	if !errors.Is(err, context.Canceled) || errors.Is(err, validation.ErrResourceLimit) || r.entity == nil || !r.dtdStore.State().EntityWellFormednessIncomplete {
		t.Fatalf("err=%v state=%+v", err, r.dtdStore.State())
	}
	r.Close()
	if used, _ := a.Usage(validation.BudgetBufferedBytes); used != held {
		t.Fatalf("foreign allowance lost: %d/%d", used, held)
	}
	a.Release(validation.BudgetBufferedBytes, held)
}
