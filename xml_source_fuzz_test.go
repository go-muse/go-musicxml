package musicxml

import (
	"bytes"
	"context"
	"io"
	"testing"
	"testing/iotest"

	"github.com/go-muse/go-musicxml/validation"
)

// FuzzSourceXMLReader checks streaming progress, fragment independence, resource
// cleanup and deterministic acceptance without treating another parser's legacy
// name tables or ignored DTD semantics as a normative oracle.
func FuzzSourceXMLReader(f *testing.F) {
	for _, seed := range []string{`<a/>`, `<?xml version="1.0"?><a x="literal&#x9;reference"/>`, `<a xmlns:p="u"><p:b/></a>`, `<!DOCTYPE a [<!ATTLIST a x CDATA "default">]><a/>`, `<a><![CDATA[x]]><!--x--><?p x?></a>`, `<a>&#xD800;</a>`, `<𐀀/>`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			t.Skip()
		}
		var outcomes [2]bool
		for i := range 2 {
			var input io.Reader = bytes.NewReader(data)
			if i == 1 {
				input = iotest.OneByteReader(input)
			}
			a, _ := validation.NewBudgetAccount(validation.Budgets{MaxInputBytes: 16384, MaxScalarBytes: 1024, MaxBufferedBytes: 1 << 20, MaxDepth: 32, MaxEvents: 2048, MaxAttributes: 256})
			s, err := newSourceXMLTokenReader(input, sourceXMLConfig{Context: context.Background(), Account: a})
			if err != nil {
				t.Fatal(err)
			}
			for err == nil {
				_, err = s.Token()
			}
			outcomes[i] = err == io.EOF
			s.Close()
			for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
				used, e := a.Usage(kind)
				if e != nil || used != 0 {
					t.Fatalf("%s leak %d: %v", kind, used, e)
				}
			}
		}
		if outcomes[0] != outcomes[1] {
			t.Fatalf("fragment-dependent acceptance for %q", data)
		}
	})
}
