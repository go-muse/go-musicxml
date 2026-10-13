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

func TestDTDTokenizedNormalizationPreservesReferenceWhitespace(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", ""}, {"   ", ""}, {"a", "a"}, {" a  b ", "a b"},
		{" a\tb\nc\rd\u00a0e ", "a\tb\nc\rd\u00a0e"},
		{" \t  \n ", "\t \n"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			account, err := validation.NewBudgetAccount(validation.Budgets{})
			if err != nil {
				t.Fatal(err)
			}
			r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: account})
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.normalizeDTDTokenized(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("got=%q want=%q error=%v", got, tc.want, err)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			used, err := account.Usage(validation.BudgetBufferedBytes)
			if err != nil || used != 0 {
				t.Fatalf("live retention=%d error=%v", used, err)
			}
		})
	}
}

func TestDTDAttributesNormalizeBeforeNamespaceExpansion(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        map[xml.Name]string
	}{
		{"default-versus-character-reference-whitespace", `<!DOCTYPE r [<!ENTITY w "&#xA;"><!ATTLIST r a NMTOKENS "  x  y " b CDATA "&#x9;&w;">]><r/>`, map[xml.Name]string{{Local: "a"}: "x y", {Local: "b"}: "\t "}},
		{"explicit-numeric-whitespace", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS "default">]><r a=" x&#x9;  y "/>`, map[xml.Name]string{{Local: "a"}: "x\t y"}},
		{"first-default", `<!DOCTYPE r [<!ATTLIST r a CDATA "before"><!ATTLIST r a CDATA "after">]><r/>`, map[xml.Name]string{{Local: "a"}: "before"}},
		{"namespace-default-on-same-tag", `<!DOCTYPE r [<!ATTLIST r xmlns:p CDATA "urn:p" p:a CDATA "value">]><r/>`, map[xml.Name]string{{Space: "urn:p", Local: "a"}: "value"}},
		{"lexical-declaration-name", `<!DOCTYPE r [<!ATTLIST p:r a CDATA "not-applied">]><r xmlns:p="urn:p"/>`, map[xml.Name]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account, err := validation.NewBudgetAccount(validation.Budgets{})
			if err != nil {
				t.Fatal(err)
			}
			r, err := newSourceXMLTokenReader(strings.NewReader(tc.input), sourceXMLConfig{Context: context.Background(), Account: account, InternalDTD: true})
			if err != nil {
				t.Fatal(err)
			}
			decoder := xml.NewTokenDecoder(r)
			var got map[xml.Name]string
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if start, ok := token.(xml.StartElement); ok {
					got = make(map[xml.Name]string)
					for _, attr := range start.Attr {
						if attr.Name.Space != "xmlns" && attr.Name.Local != "xmlns" {
							got[attr.Name] = attr.Value
						}
					}
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("attributes=%v want=%v", got, tc.want)
			}
			for name, value := range tc.want {
				if got[name] != value {
					t.Fatalf("attribute=%v got=%q want=%q", name, got[name], value)
				}
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			if used, err := account.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
				t.Fatalf("live=%d error=%v", used, err)
			}
		})
	}
}

func TestDTDTokenizedNormalizationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: ctx, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cancel()
	if _, err := r.normalizeDTDTokenized(" a  b "); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestDTDTokenizedNormalizationExactWorkAdmission(t *testing.T) {
	const value = " a  b "
	for _, limit := range []int64{int64(len(value)) - 1, int64(len(value))} {
		account, err := validation.NewBudgetAccount(validation.Budgets{MaxEvents: limit})
		if err != nil {
			t.Fatal(err)
		}
		r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: account})
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.normalizeDTDTokenized(value)
		if limit == int64(len(value)) {
			if err != nil || got != "a b" {
				t.Fatalf("exact: value=%q error=%v", got, err)
			}
		} else if !errors.Is(err, validation.ErrResourceLimit) || got != "" {
			t.Fatalf("one-short: value=%q error=%v", got, err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if used, err := account.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
			t.Fatalf("live=%d error=%v", used, err)
		}
	}
}

func TestDTDTokenizedNormalizationChunkBoundaries(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{strings.Repeat("a", 4094) + "    b ", strings.Repeat("a", 4094) + " b"},
		{strings.Repeat("a", 4095) + "é  b ", strings.Repeat("a", 4095) + "é b"},
	} {
		account, err := validation.NewBudgetAccount(validation.Budgets{})
		if err != nil {
			t.Fatal(err)
		}
		r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: account})
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.normalizeDTDTokenized(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("chunk boundary: got length=%d want=%d error=%v", len(got), len(tc.want), err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if used, err := account.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
			t.Fatalf("live=%d error=%v", used, err)
		}
	}
}

func TestDTDTokenizedNormalizationExactBufferAdmission(t *testing.T) {
	const value = " a  b "
	probe, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: probe})
	if err != nil {
		t.Fatal(err)
	}
	base, err := probe.Usage(validation.BudgetBufferedBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	for _, short := range []int64{0, 1} {
		limit := base + 64 + 2*int64(len(value)) - short
		account, err := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: limit})
		if err != nil {
			t.Fatal(err)
		}
		r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: account})
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.normalizeDTDTokenized(value)
		if short == 0 {
			if err != nil || got != "a b" {
				t.Fatalf("exact buffer: value=%q error=%v", got, err)
			}
		} else if !errors.Is(err, validation.ErrResourceLimit) || got != "" {
			t.Fatalf("short buffer: value=%q error=%v", got, err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if used, err := account.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
			t.Fatalf("live=%d error=%v", used, err)
		}
	}
}
