package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func TestSourceASCIISpanCoordinatesAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c, err := newSourceXMLCursor(strings.NewReader("----abc\n\tZ<&"), ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.close()
	for range 4 {
		if _, err = c.take(); err != nil {
			t.Fatal(err)
		}
	}
	span := c.asciiSpan(sourceASCIIText, 0, 100)
	if string(span) != "abc\n\tZ" {
		t.Fatalf("span %q", span)
	}
	c.consumeASCII(span)
	if c.position.ByteOffset != 10 || c.position.Line != 2 || c.position.Column != 3 {
		t.Fatalf("position %+v", c.position)
	}
	u, err := c.peek()
	if err != nil || u.Rune != '<' || u.Start != c.position {
		t.Fatalf("delimiter %+v: %v", u, err)
	}
	cancel()
	if err := c.contextErr(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
}

func TestSourceASCIIFragmentationPreservesAllTokenFacts(t *testing.T) {
	input := `<root xmlns:p="urn:x" first="` + strings.Repeat("attribute", 700) + `&#x9;tail" second="two"><p:child>` + strings.Repeat("ascii\n\t", 700) + `é𝄞 &amp; ]]>` + `</p:child></root>`
	// Escaped and literal closing brackets exercise the delimiter boundary.
	input = strings.ReplaceAll(input, "]]>", "]]&#62;")
	for _, accounted := range []bool{false, true} {
		scan := func(fragmented bool) []sourceASCIIFact {
			var inputReader io.Reader = strings.NewReader(input)
			if fragmented {
				inputReader = iotest.OneByteReader(inputReader)
			}
			var account *validation.Account
			if accounted {
				var err error
				account, err = validation.NewBudgetAccount(validation.Budgets{})
				if err != nil {
					t.Fatal(err)
				}
			}
			r, err := newSourceXMLTokenReader(inputReader, sourceXMLConfig{Context: context.Background(), Account: account})
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			var facts []sourceASCIIFact
			for {
				tok, err := r.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				meta := r.Metadata()
				meta.Attributes = append([]sourceXMLAttribute(nil), meta.Attributes...)
				facts = append(facts, sourceASCIIFact{xml.CopyToken(tok), meta})
			}
			return facts
		}
		full, fragmented := scan(false), scan(true)
		if !reflect.DeepEqual(full, fragmented) {
			t.Fatalf("fragmentation changed copied token or original-byte facts (accounted=%v)", accounted)
		}
	}
}

type sourceASCIIFact struct {
	token    xml.Token
	metadata sourceXMLTokenMeta
}
