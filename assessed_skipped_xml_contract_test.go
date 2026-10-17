package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

// These inputs deliberately separate duplicate detection from attribute-token
// whitespace errors. The ordinary and strict paths must inspect skipped XML.
func TestAssessedDecodeSkippedSubtreeXMLContracts(t *testing.T) {
	cases := []struct {
		name, subtree, diagnostic string
	}{
		{"unknown lexical duplicate", `<unknown><child a="1" a="2"/></unknown>`, "duplicate XML attribute {}a"},
		{"foreign lexical duplicate", `<v:unknown xmlns:v="urn:vendor"><child a="1" a="2"/></v:unknown>`, "duplicate XML attribute {}a"},
		{"unknown expanded duplicate", `<unknown xmlns:p="urn:x" xmlns:q="urn:x"><child p:a="1" q:a="2"/></unknown>`, "duplicate XML attribute {urn:x}a"},
		{"foreign expanded duplicate", `<v:unknown xmlns:v="urn:vendor" xmlns:p="urn:x" xmlns:q="urn:x"><child p:a="1" q:a="2"/></v:unknown>`, "duplicate XML attribute {urn:x}a"},
		{"unknown nested doctype", `<unknown><child><!DOCTYPE child></child></unknown>`, "DOCTYPE"},
		{"foreign nested doctype", `<v:unknown xmlns:v="urn:vendor"><child><!DOCTYPE child></child></v:unknown>`, "DOCTYPE"},
		{"unknown undeclared prefix", `<unknown><p:child/></unknown>`, "undeclared XML namespace prefix p"},
		{"foreign undeclared prefix", `<v:unknown xmlns:v="urn:vendor"><p:child/></v:unknown>`, "undeclared XML namespace prefix p"},
		{"unknown mismatched close", `<unknown><child></different></unknown>`, ""},
		{"foreign mismatched close", `<v:unknown xmlns:v="urn:vendor"><child></different></v:unknown>`, ""},
	}
	for _, tc := range cases {
		for _, mode := range []string{"decode", "assessed default", "assessed explicit"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				input := `<score-partwise>` + tc.subtree + `</score-partwise>`
				var document Document
				var err error
				if mode == "decode" {
					document, err = Decode(strings.NewReader(input))
				} else {
					options := DecodeAssessmentOptions{}
					if mode == "assessed explicit" {
						options = strictSourceOptions()
					}
					result, decodeErr := DecodeAssessed(context.Background(), strings.NewReader(input), options)
					document, err = result.Document, decodeErr
				}
				var syntax *xml.SyntaxError
				if !errors.As(err, &syntax) || document != nil {
					t.Fatalf("skipped malformed XML must retain its syntax error and discard the model: %T %v", document, err)
				}
				if tc.diagnostic != "" && !strings.Contains(syntax.Msg, tc.diagnostic) {
					t.Fatalf("wrong XML failure: got %q, want %q", syntax.Msg, tc.diagnostic)
				}
			})
		}
	}
}

func TestAssessedDecodeSkippedDistinctAttributesRemainWellFormed(t *testing.T) {
	for _, subtree := range []string{
		`<unknown><child a="1" b="2"/></unknown>`,
		`<v:unknown xmlns:v="urn:vendor" xmlns:p="urn:x" xmlns:q="urn:y"><child a="0" p:a="1" q:a="2"/></v:unknown>`,
	} {
		input := `<score-partwise>` + subtree + `</score-partwise>`
		document, err := Decode(strings.NewReader(input))
		if err != nil || document == nil {
			t.Fatalf("distinct skipped attributes rejected: %T %v", document, err)
		}
		result, err := DecodeAssessed(context.Background(), strings.NewReader(input), strictSourceOptions())
		var syntax *xml.SyntaxError
		if errors.As(err, &syntax) {
			t.Fatalf("strict assessment manufactured malformed XML: %v", err)
		}
		// The intentionally minimal score still has independent XSD violations.
		if !errors.Is(err, ErrInvalidDocument) || result.Document != nil {
			t.Fatalf("strict source acceptance lost its independent XSD failure: %+v %v", result, err)
		}
	}
}
