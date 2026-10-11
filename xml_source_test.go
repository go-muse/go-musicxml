package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"strings"
	"testing"
)

func sourceScan(t *testing.T, input string) ([]xml.Token, error) {
	t.Helper()
	s, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background()})
	if err != nil {
		return nil, err
	}
	defer s.Close()
	var out []xml.Token
	for {
		token, err := s.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, xml.CopyToken(token))
	}
}
func TestSourceLexicalClosure(t *testing.T) {
	bad := []string{`<a x="1"y="2"/>`, `<?p?x?><a/>`, `<:a/>`, `<a:/>`, `<a:b:c/>`, `<a xmlns:p="u"><p:1a/></a>`, `<a><b></a>`, `<a/><b/>`, `&#32;<a/>`, `<a/><![CDATA[ ]]>`, `<a>&#xD800;</a>`, `<a>&#0;</a>`, `<a>&#x110000;</a>`, `<a>]]></a>`, `<a><!--\x00--></a>`}
	for _, input := range bad {
		input = strings.ReplaceAll(input, `\x00`, "\x00")
		if _, err := sourceScan(t, input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	good := []string{`<a/>`, `<𐀀 a="x&#x9;y"/>`, `<a>]]&gt;</a>`, `<a><![CDATA[<x>&z;]]></a>`, `<?p?><a/>`, `<?p data?><a/>`, `<a xml:lang="en"/>`}
	for _, input := range good {
		if _, err := sourceScan(t, input); err != nil {
			t.Errorf("%q: %v", input, err)
		}
	}
}
func TestSourceAttributeNormalization(t *testing.T) {
	tokens, err := sourceScan(t, "<a v=\"a\tb\nc\r\nd&#x9;e&#xA;f&#xD;g\"/>")
	if err != nil {
		t.Fatal(err)
	}
	if got := tokens[0].(xml.StartElement).Attr[0].Value; got != "a b c d\te\nf\rg" {
		t.Fatalf("%q", got)
	}
}
func TestSourceNamespaceClosure(t *testing.T) {
	for _, input := range []string{`<a p:x="1"/>`, `<p:a/>`, `<a xmlns:xml="bad"/>`, `<a xmlns:xmlns="x"/>`, `<a xmlns:p="http://www.w3.org/XML/1998/namespace"/>`, `<a xmlns="http://www.w3.org/2000/xmlns/"/>`, `<xmlns:a/>`, `<a xmlns:p=""/>`, `<a xmlns:p="u" xmlns:q="u" p:x="1" q:x="2"/>`} {
		if _, err := sourceScan(t, input); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}
func TestSourceDeclarationGrammar(t *testing.T) {
	for _, input := range []string{`<?xml?><a/>`, `<?xml encoding="UTF-8" version="1.0"?><a/>`, `<?xml version="1.0" nonsense="x"?><a/>`, `<?xml version="1.0" standalone="maybe"?><a/>`, `<?xml version="1.0" encoding="8bad"?><a/>`, `<?xml version="1&#x2e;0"?><a/>`, `<!garbage><a/>`} {
		if _, err := sourceScan(t, input); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	for _, input := range []string{`<?xml version="1.0"?><a/>`, `<?xml version="1.23" standalone="yes"?><a/>`} {
		if _, err := sourceScan(t, input); err != nil {
			t.Error(err)
		}
	}
}
func TestSourceTokenBoundBeforeAllocation(t *testing.T) {
	for _, template := range []string{`<a x="VALUE"/>`, `<a>VALUE</a>`, `<!--VALUE--><a/>`, `<?p VALUE?><a/>`, `<VALUE/>`} {
		input := strings.ReplaceAll(template, "VALUE", strings.Repeat("x", 100000))
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxScalarBytes: 32})
		s, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background(), Account: a})
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = s.Token()
		}
		var resource *validation.ResourceError
		if !errors.As(err, &resource) {
			t.Fatalf("%s: %v", template, err)
		}
		if s.Summary().BytesRead > 4096 {
			t.Fatalf("read giant token: %d", s.Summary().BytesRead)
		}
		s.Close()
		used, _ := a.Usage(validation.BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("leak %d", used)
		}
	}
}
