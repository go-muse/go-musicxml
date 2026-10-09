package musicxml

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var validationAnyURICases = []struct {
	name, href string
	valid      bool
}{
	{"relative", "scores/part.musicxml", true},
	{"empty", "", true},
	{"fragment", "score.musicxml#measure-1", true},
	{"escaped fragment marker", "score.musicxml#measure%231", true},
	{"escaped path brackets", "scores/%5Bpart%5D.musicxml", true},
	{"escaped authority", "http://host%2Eexample/score.musicxml", true},
	{"space in authority", "http://host name/score.musicxml", true},
	{"backslash in authority", `http://host\name/score.musicxml`, true},
	{"Unicode authority and path", "https://例.example/楽譜.musicxml", true},
	{"IPv6", "http://[2001:db8::1]:80/score.musicxml", true},
	{"IPv6 embedded IPv4 leading zeros", "http://[::ffff:192.168.001.001]/score.musicxml", true},
	{"IPv6 embedded zero IPv4", "http://[::000.000.000.000]", true},
	{"IPv6 full embedded IPv4", "http://[1:2:3:4:5:6:192.000.002.001]", true},
	{"whitespace normalization", " \thttps://example.org/score.musicxml\n", true},
	{"opaque", "urn:musicxml:score", true},
	{"opaque escaped percent", "urn:musicxml:score%25", true},
	{"multiple fragments", "score.musicxml#first#second", false},
	{"relative path brackets", "[score].musicxml", false},
	{"absolute path brackets", "https://example.org/[score].musicxml", false},
	{"malformed query escape", "score.musicxml?part=%GG", false},
	{"truncated query escape", "score.musicxml?part=%2", false},
	{"malformed opaque escape", "urn:musicxml:%GG", false},
	{"truncated opaque escape", "urn:musicxml:%", false},
	{"invalid scheme", "1scheme:score", false},
	{"unclosed IPv6", "http://[2001:db8::1/score.musicxml", false},
}

// The expectations here agree with the bundled opus.xsd and an independent
// XSD 1.0 validator. anyURI's lexical rules are specified in XSD Part 2,
// section 3.2.17, including the XLink 1.0 section 5.4 escaping procedure.
func TestValidateAnyURILexicalForms(t *testing.T) {
	t.Parallel()
	for _, test := range validationAnyURICases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := &OpusDocument{Content: OpusDocumentContents{{Score: &OpusScore{Href: test.href}}}}
			// Decode and Encode are transport; scalar checks belong to Validate.
			var output bytes.Buffer
			require.NoError(t, Encode(&output, document))
			decoded, err := DecodeOpusDocument(bytes.NewReader(output.Bytes()))
			require.NoError(t, err)
			assert.Equal(t, test.href, decoded.Content[0].Score.Href)
			err = Validate(decoded)
			if test.valid {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			var failure *ValidationError
			require.ErrorAs(t, err, &failure)
			require.Len(t, failure.Issues, 1)
			assert.Equal(t, "/opus/score/@{http://www.w3.org/1999/xlink}href", failure.Issues[0].Path)
			assert.Equal(t, "datatype", failure.Issues[0].Constraint)
		})
	}
}

// Keep the public validator's expectations checked independently against the
// official schema in Linux CI, where xmllint is required and installed.
func TestAnyURILexicalFormsAgainstOpusSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI runs the external XSD conformance test")
	}
	directory, err := filepath.Abs(filepath.Join("schema", "musicxml-4.0"))
	require.NoError(t, err)
	for _, test := range validationAnyURICases {
		t.Run(test.name, func(t *testing.T) {
			document := &OpusDocument{Content: OpusDocumentContents{{Score: &OpusScore{Href: test.href}}}}
			var encoded bytes.Buffer
			require.NoError(t, Encode(&encoded, document))
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "opus.xsd"), filepath.Join(directory, "catalog.xml"), encoded.String(), test.valid)
		})
	}
}

// These expectations follow the normative grammar, rather than libxml2's
// URI parser. It disagrees on registry-based authorities, opaque/query brackets,
// empty absolute URIs, empty ports, and some IPv6 literals. Keep these separate
// from the compatible cases checked with xmllint above.
//
// RFC 2396 sections 3, 3.2.1 and 3.2.2, Appendix C.1 and Appendix G.3:
// https://www.rfc-editor.org/rfc/rfc2396
// RFC 2732 section 3 adds brackets to reserved (and hence uric), without
// changing uric_no_slash or path-segment characters:
// https://www.rfc-editor.org/rfc/rfc2732#section-3
func TestValidateAnyURIStandardGrammar(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, href string
		valid      bool
	}{
		{"registry authority colon", "http://host:abc", true},
		{"registry authority escaped octet", "http://host:%32", true},
		{"empty server port", "http://host:", true},
		{"registry authority multiple colons", "example://host:a:b", true},
		{"registry authority at signs", "example://one@two@three", true},
		{"registry authority escaped brackets", "example://%5Bname%5D", true},
		{"opaque bracket after initial character", "mailto:foo[bar]", true},
		{"opaque question mark", "scheme:?", true},
		{"opaque question mark then brackets", "scheme:?[value]", true},
		{"opaque question mark then slash", "scheme:?/path", true},
		{"relative query", "?query", true},
		{"relative query brackets", "?[value]", true},
		{"hierarchical query brackets", "http://host/path?[value]", true},
		{"fragment brackets", "score.musicxml#[value]", true},
		{"IPv6 empty port", "http://[::1]:", true},
		{"IPv6 user information", "http://user%20name@[::1]:80/score", true},
		{"opaque leading opening bracket", "scheme:[value]", false},
		{"opaque leading closing bracket", "scheme:]value", false},
		{"empty absolute URI", "scheme:", false},
		{"empty absolute URI before fragment", "scheme:#value", false},
		{"IPv6 escaped port", "http://[::1]:%32", false},
		{"IPv6 nondecimal port", "http://[::1]:abc", false},
		{"IPv6 prefix", "http://a[::1]", false},
		{"IPv6 nonaddress", "http://[name]", false},
		{"IPv4 inside IPv6 brackets", "http://[192.0.2.1]", false},
		{"IPv6 zone outside RFC2732", "http://[fe80::1%25eth0]", false},
		{"brackets in user information", "http://user[]@[::1]", false},
		{"IPv6 embedded IPv4 boundary octets", "http://[::ffff:000.099.100.255]:80/path", true},
		{"IPv6 embedded IPv4 zero port", "http://user@[::ffff:001.002.003.004]:000", true},
		{"IPv6 embedded IPv4 octet overflow", "http://[::ffff:256.001.001.001]", false},
		{"IPv6 embedded IPv4 octet too long", "http://[::ffff:0000.1.1.1]", false},
		{"IPv6 embedded IPv4 missing octet", "http://[::ffff:001.1.1]", false},
		{"IPv6 embedded IPv4 empty octet", "http://[::ffff:001..1.1]", false},
		{"IPv6 embedded IPv4 extra octet", "http://[::ffff:001.1.1.1.1]", false},
		{"IPv6 embedded IPv4 signed octet", "http://[::ffff:+01.1.1.1]", false},
		{"IPv6 embedded IPv4 negative octet", "http://[::ffff:-00.1.1.1]", false},
		{"IPv6 embedded IPv4 nondigit octet", "http://[::ffff:0a1.1.1.1]", false},
		{"IPv6 embedded IPv4 too few groups", "http://[1:2:3:4:5:001.1.1.1]", false},
		{"IPv6 embedded IPv4 too many groups", "http://[1:2:3:4:5:6:7:001.1.1.1]", false},
		{"IPv6 embedded IPv4 double compression", "http://[1::2::001.1.1.1]", false},
		{"IPv6 embedded IPv4 misplaced dot", "http://[1.2:3::001.1.1.1]", false},
		{"IPv6 embedded IPv4 zone", "http://[::ffff:001.1.1.1%25zone]", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := &OpusDocument{Content: OpusDocumentContents{{Score: &OpusScore{Href: test.href}}}}
			err := Validate(document)
			if test.valid {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrInvalidDocument)
			}
		})
	}
}
