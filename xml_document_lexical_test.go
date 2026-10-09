package musicxml

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// XML 1.0 productions 1, 22 and 27 permit literal S, comments and PIs at
// document boundaries, not references or CDATA sections whose decoded text
// happens to be empty or whitespace. Inside the root those forms remain legal.
// https://www.w3.org/TR/REC-xml/#sec-prolog-dtd
func xmlDocumentLexicalCases() []xmlReadingCase {
	const root = `<ROOT><KNOWN>Keep</KNOWN></ROOT>`
	var cases []xmlReadingCase
	for _, spelling := range []struct{ name, text string }{
		{"decimal space", "&#32;"}, {"hex space", "&#x20;"},
		{"decimal tab", "&#9;"}, {"hex tab", "&#x9;"},
		{"decimal LF", "&#10;"}, {"hex LF", "&#xA;"},
		{"decimal CR", "&#13;"}, {"hex CR", "&#xD;"},
		{"predefined entity", "&amp;"},
		{"empty CDATA", "<![CDATA[]]>"},
		{"space CDATA", "<![CDATA[ ]]>"},
		{"multiline CDATA", "<![CDATA[\t\r\n]]>"},
		{"text CDATA", "<![CDATA[text]]>"},
	} {
		for _, position := range []struct{ name, before, after string }{
			{"before root", "", root},
			{"after declaration", "DECL", root},
			{"after doctype", "DECL<!DOCTYPE ROOT>", root},
			{"after root", "DECL" + root, ""},
			{"after tail misc", "DECL" + root + "<!--ok--><?note ok?>", ""},
		} {
			cases = append(cases, xmlReadingCase{
				name:  position.name + "/" + spelling.name,
				input: position.before + " \t" + spelling.text + "\r\n" + position.after,
				valid: false,
			})
		}
	}
	return append(cases, []xmlReadingCase{
		{"literal S", " \t\r\n" + root + " \t\r\n", true},
		{"omitted declaration", root, true},
		{"attribute reference then literal tail", "DECL<ROOT marker='&amp;'><KNOWN>Keep</KNOWN></ROOT> \t\n", true},
		{"consecutive references", "DECL" + root + "&#32;&#9;", false},
		{"consecutive CDATA", "DECL" + root + "<![CDATA[]]><![CDATA[]]>", false},
		{"self closing root reference", "DECL<ROOT/>&#32;", false},
		{"self closing root CDATA", "DECL<ROOT/><![CDATA[]]>", false},
		{"markup in misc", `DECL<!-- <![CDATA[]]> &amp; &#32; --><?note <![CDATA[]]> &amp; &#32;?><!DOCTYPE ROOT>` + root + `<!-- < & --><?note < &?>`, true},
		{"markup in DTD literal", `DECL<!DOCTYPE ROOT [<!ENTITY unused "<![CDATA[]]> &amp; &#32;">]>` + root, true},
		{"known text", `DECL<ROOT><KNOWN><![CDATA[Ke]]>&#101;&#112;</KNOWN></ROOT>`, true},
		{"root content", "DECL<ROOT>\t<![CDATA[]]>&#32;<KNOWN>Keep</KNOWN><![CDATA[ \n]]>&#9;</ROOT>", true},
		{"skipped unknown content", `DECL<ROOT><unknown><![CDATA[<>&]]>&amp;&#32;<child/></unknown><KNOWN>Keep</KNOWN></ROOT>`, true},
		{"skipped foreign content", `DECL<ROOT xmlns:v="urn:vendor"><v:extra><![CDATA[<>&]]>&amp;&#32;<child/></v:extra><KNOWN>Keep</KNOWN></ROOT>`, true},
		{"self closing child followed by text", `DECL<ROOT><unknown/><![CDATA[]]>&#32;<KNOWN>Keep</KNOWN></ROOT>`, true},
		{"Latin1 before tail reference", "DECL<ROOT><unknown>\u00ff\u00e9\u00a0</unknown><KNOWN>Keep</KNOWN></ROOT>&#32;", false},
		{"Latin1 before tail CDATA", "DECL<ROOT><unknown>\u00ff\u00e9\u00a0</unknown><KNOWN>Keep</KNOWN></ROOT><![CDATA[]]>", false},
		{"Latin1 before literal tail", "DECL<ROOT><unknown>\u00ff\u00e9\u00a0</unknown><KNOWN>Keep</KNOWN></ROOT> \t\r\n", true},
	}...)
}

func TestXMLDocumentLexicalDecodePaths(t *testing.T) {
	runXMLReadingDecodePaths(t, xmlDocumentLexicalCases(), xmlDeclarationEncodingVariants, assertXMLReadingSyntaxError)
}

func TestXMLDocumentLexicalContainer(t *testing.T) {
	runXMLReadingContainer(t, xmlDocumentLexicalCases(), xmlDeclarationEncodingVariants, assertXMLReadingSyntaxError)
}

func TestXMLDocumentLexicalValidationParser(t *testing.T) {
	runXMLReadingValidationParser(t, xmlDocumentLexicalCases(), xmlDeclarationUTF8Input, assertXMLReadingSyntaxError)
}

func TestXMLDocumentLexicalLinkedResources(t *testing.T) {
	runXMLReadingLinkedResources(t, xmlDocumentLexicalCases(), xmlDeclarationEncodingVariants, assertXMLReadingSyntaxError)
}

func TestXMLDocumentLexicalAgainstXMLLint(t *testing.T) {
	runXMLReadingAgainstXMLLint(t, xmlDocumentLexicalCases())
}

func TestXMLDocumentLexicalErrors(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input string
		line        int
	}{
		{"before root", "\n\n&#32;<opus/>", 3},
		{"after root", "DECL<opus/>\n\n&#32;", 3},
		{"empty CDATA", "DECL<opus/>\n\n<![CDATA[]]>", 3},
		{"multiline CDATA", "DECL\n<![CDATA[\n\n]]><opus/>", 4},
		{"no root reference", "&#32;", 1},
		{"no root CDATA", "<![CDATA[]]>", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := func(t *testing.T, err error) {
				t.Helper()
				var syntaxError *xml.SyntaxError
				require.ErrorAs(t, err, &syntaxError)
				assert.Equal(t, test.line, syntaxError.Line)
				assert.Contains(t, syntaxError.Msg, "outside the root element")
				assert.NotErrorIs(t, err, ErrEmptyDocument)
			}
			_, err := parseValidationDocument(xmlDeclarationUTF8Input(test.input))
			check(t, err)
			for encoding, data := range xmlDeclarationEncodingVariants(test.input) {
				t.Run(encoding, func(t *testing.T) {
					_, err := Decode(iotest.OneByteReader(bytes.NewReader(data)))
					check(t, err)
				})
			}
		})
	}
}

func TestXMLDocumentLexicalPreservesText(t *testing.T) {
	t.Parallel()
	const input = `DECL<opus><title><![CDATA[<&]]>&amp;&#32;&#xD;&#xA;é</title></opus>`
	const want = "<&& \r\né"
	node, err := parseValidationDocument(xmlDeclarationUTF8Input(input))
	require.NoError(t, err)
	require.Len(t, node.Children, 1)
	assert.Equal(t, want, node.Children[0].Text.String())
	for encoding, data := range xmlDeclarationEncodingVariants(input) {
		t.Run(encoding, func(t *testing.T) {
			document, err := Decode(iotest.OneByteReader(bytes.NewReader(data)))
			require.NoError(t, err)
			assert.Equal(t, Ptr(want), document.(*OpusDocument).Title)
			var encoded bytes.Buffer
			require.NoError(t, Encode(&encoded, document))
			roundTrip, err := Decode(strings.NewReader(encoded.String()))
			require.NoError(t, err)
			assert.Equal(t, document, roundTrip)
		})
	}
}

// Exercise marker spans across buffer boundaries and charset conversion, with
// readers that split source bytes differently or return data together with EOF.
func TestXMLDocumentLexicalStreaming(t *testing.T) {
	t.Parallel()
	padding := strings.Repeat(" ", 8193)
	for _, test := range []struct {
		name, input string
		valid       bool
	}{
		{"long prolog CDATA", "DECL" + padding + "<![CDATA[]]><opus/>", false},
		{"long tail reference", "DECL<opus/>" + padding + "&#32;", false},
		{"long legal boundaries", "DECL" + padding + "<opus/>" + padding, true},
		{"supplementary tail CDATA", "DECL<opus><title>𝄞</title></opus><![CDATA[]]>", false},
		{"non ASCII tail reference", "DECL<opus><title>" + strings.Repeat("é", 4097) + "</title></opus>&#32;", false},
		{"non ASCII literal tail", "DECL<opus><title>" + strings.Repeat("é", 4097) + "</title></opus> \n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseValidationDocument(xmlDeclarationUTF8Input(test.input))
			assert.Equal(t, test.valid, err == nil, "internal parser: %v", err)
			for encoding, data := range xmlDeclarationEncodingVariants(test.input) {
				for _, reader := range []struct {
					name string
					wrap func(io.Reader) io.Reader
				}{
					{"ordinary", func(r io.Reader) io.Reader { return r }},
					{"one byte", iotest.OneByteReader},
					{"half", iotest.HalfReader},
					{"data with EOF", iotest.DataErrReader},
				} {
					t.Run(encoding+"/"+reader.name, func(t *testing.T) {
						_, err := Decode(reader.wrap(bytes.NewReader(data)))
						if test.valid {
							assert.NoError(t, err)
						} else {
							assertXMLReadingSyntaxError(t, err)
						}
					})
				}
			}
		})
	}
}

func TestXMLDocumentLexicalBoundedReadAhead(t *testing.T) {
	t.Parallel()
	// A root token must be available without consuming its large text body.
	input := strings.NewReader("<opus><title>" + strings.Repeat("x", 1<<20) + "</title></opus>")
	decoder, err := newXMLDecoder(input)
	require.NoError(t, err)
	token, err := decoder.Token()
	require.NoError(t, err)
	assert.IsType(t, xml.StartElement{}, token)
	assert.Greater(t, input.Len(), 1<<19, "reader buffered the document before returning its root")
}
