package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func FuzzSourceInternalDTDFrames(f *testing.F) {
	for _, source := range []string{
		`<!DOCTYPE r [<!ENTITY e "<x a='v'/>">]><r>&e;&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e "&#38;e;">]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e "<!-- &e; -->">]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e '"&#xD;&#x9;'><!ENTITY b '&e;'>]><r a="&b;"/>`,
		`<!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'v'>">%p;]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e "<x">]><r>&e; /></r>`,
		`<!DOCTYPE r [<!ENTITY e "&amp;">]><r>&e;#60;</r>`,
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		a, err := validation.NewBudgetAccount(validation.Budgets{MaxInputBytes: 1 << 16, MaxBufferedBytes: 1 << 20, MaxEvents: 1 << 15, MaxScalarBytes: 1 << 15, MaxDepth: 128})
		if err != nil {
			t.Fatal(err)
		}
		r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = r.Token()
		}
		r.Close()
		used, err := a.Usage(validation.BudgetBufferedBytes)
		if err != nil || used != 0 {
			t.Fatalf("retention=%d err=%v", used, err)
		}
	})
}
