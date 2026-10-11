package musicxml

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

func TestValidationDocumentUsesSharedSourceSyntax(t *testing.T) {
	for _, input := range []string{`<p:r/>`, `<r xmlns:p=""/>`, `<r xmlns:xml="urn:bad"/>`, `<?xml?><r/>`, `<r a="x"b="y"/>`, `<!DOCTYPE><r/>`, `<!garbage><r/>`, `<r/><tail/>`} {
		if _, err := parseValidationDocument([]byte(input)); err == nil {
			t.Errorf("accepted malformed original XML: %s", input)
		}
	}
	for _, input := range []string{`<𐀀/>`, `<r xmlns:a="xmlns" a:x="ordinary"/>`, `<?xml version="1.0"?><r/>`, `<!DOCTYPE r [<!-- empty -->]><r/>`} {
		if _, err := parseValidationDocument([]byte(input)); err != nil {
			t.Errorf("rejected valid original XML %s: %v", input, err)
		}
	}
}

func TestValidationDocumentPreservesLexicalNamespaceAndNormalization(t *testing.T) {
	input := "<r xmlns:p=\"xmlns\" xmlns:q=\"urn:old\" p:x=\"ordinary\" literal=\"a\tb\r\nc\" refs=\"a&#x9;b&#xA;c&#xD;d\"><q:c xmlns:q=\"urn:new\"/><q:c/></r>"
	root, err := parseValidationDocument([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Attrs) != 5 || !root.Attrs[0].NamespaceDeclaration || !root.Attrs[1].NamespaceDeclaration || root.Attrs[2].NamespaceDeclaration || root.Attrs[2].Name.Space != "xmlns" {
		t.Fatalf("physical namespace provenance lost: %+v", root.Attrs)
	}
	if root.Attrs[3].Value != "a b c" || root.Attrs[4].Value != "a\tb\nc\rd" {
		t.Errorf("literal/reference distinction lost: %+v", root.Attrs)
	}
	if len(root.Children) != 2 || root.Children[0].Name.Space != "urn:new" || root.Children[1].Name.Space != "urn:old" || root.Namespaces["q"] != "urn:old" {
		t.Errorf("scope restoration lost: %+v", root)
	}
}

func TestValidationDocumentPreservesDepthSentinel(t *testing.T) {
	exact := strings.Repeat("<r>", maximumDocumentDepth) + strings.Repeat("</r>", maximumDocumentDepth)
	if _, err := parseValidationDocument([]byte(exact)); err != nil {
		t.Fatalf("exact depth: %v", err)
	}
	over := "<r>" + exact + "</r>"
	if _, err := parseValidationDocument([]byte(over)); !errors.Is(err, ErrDocumentTooDeep) {
		t.Fatalf("legacy depth sentinel lost: %v", err)
	}
}

func TestValidationDocumentRejectsMalformedSourceTail(t *testing.T) {
	_, err := parseValidationDocument([]byte(`<r/><`))
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("malformed tail was not checked: %v", err)
	}
}
