package musicxml

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DECL is replaced with a declaration matching each test's byte encoding.
// The fixtures exercise placement, not the full XML declaration grammar.
var xmlDeclarationCases = []xmlReadingCase{
	{"first", `DECL<ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"first before misc and doctype", "DECL\n<!-- comment --><?test ok?><!DOCTYPE ROOT><ROOT><KNOWN>Keep</KNOWN></ROOT>", true},
	{"omitted", `<ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"omitted before misc", " \t\r\n<!-- comment --><?test ok?><ROOT><KNOWN>Keep</KNOWN></ROOT>", true},
	{"after space", ` DECL<ROOT/>`, false},
	{"after XML whitespace", "\t\r\nDECL<ROOT/>", false},
	{"after comment", `<!-- comment -->DECL<ROOT/>`, false},
	{"after processing instruction", `<?test ok?>DECL<ROOT/>`, false},
	{"after doctype", `<!DOCTYPE ROOT>DECL<ROOT/>`, false},
	{"repeated", `DECLDECL<ROOT/>`, false},
	{"repeated after misc", "DECL\n<!-- comment -->DECL<ROOT/>", false},
	{"inside root", `DECL<ROOT>DECL</ROOT>`, false},
	{"inside known text", `DECL<ROOT><KNOWN>DECLKeep</KNOWN></ROOT>`, false},
	{"inside unknown subtree", `DECL<ROOT><unknown><child>DECL</child></unknown></ROOT>`, false},
	{"inside foreign subtree", `DECL<ROOT xmlns:v="urn:vendor"><v:extension><child>DECL</child></v:extension></ROOT>`, false},
	{"after root", "DECL<ROOT/>\nDECL", false},
	{"declaration text in comment", `DECL<!-- <?xml version="1.0"?> --><ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"declaration text in CDATA", `DECL<ROOT><unknown><![CDATA[<?xml version="1.0"?>]]></unknown><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"ordinary processing instructions", `DECL<?xml-stylesheet type="text/xsl" href="style.xsl"?><ROOT><?xml-model href="schema.rng"?><unknown><?XML-stylesheet test?></unknown><KNOWN>Keep</KNOWN></ROOT><?xmlfoo test?>`, true},
}

func TestXMLDeclarationDecodePaths(t *testing.T) {
	runXMLReadingDecodePaths(t, xmlDeclarationCases, xmlDeclarationEncodingVariants, assertXMLReadingSyntaxError)
}

func TestXMLDeclarationContainer(t *testing.T) {
	runXMLReadingContainer(t, xmlDeclarationCases, xmlDeclarationEncodingVariants, assertXMLReadingSyntaxError)
}

func TestXMLDeclarationValidationParser(t *testing.T) {
	runXMLReadingValidationParser(t, xmlDeclarationCases, xmlDeclarationUTF8Input, assertXMLReadingSyntaxError)
}

func TestXMLDeclarationLinkedResources(t *testing.T) {
	runXMLReadingLinkedResources(t, xmlDeclarationCases, xmlDeclarationEncodingVariants, assertXMLReadingSyntaxError)
}

func TestXMLDeclarationReservedTargets(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"XML", "XMl", "XmL", "Xml", "xML", "xMl", "xmL"} {
		for _, template := range []string{
			`PI<opus/>`, `DECLPI<opus/>`, `<opus>PI</opus>`,
			`<opus><unknown>PI</unknown></opus>`,
			`<opus xmlns:v="urn:vendor"><v:extension>PI</v:extension></opus>`, `<opus/>PI`,
		} {
			t.Run(target+"/"+template, func(t *testing.T) {
				input := strings.ReplaceAll(template, "PI", `<?`+target+` version="1.0"?>`)
				for encoding, data := range xmlDeclarationEncodingVariants(input) {
					t.Run(encoding, func(t *testing.T) {
						_, err := Decode(iotest.OneByteReader(bytes.NewReader(data)))
						var syntaxError *xml.SyntaxError
						require.ErrorAs(t, err, &syntaxError)
						assert.Contains(t, syntaxError.Msg, "reserved XML processing instruction target")
					})
				}
			})
		}
	}
}

func xmlDeclarationEncodingVariants(input string) map[string][]byte {
	declaration := func(encoding string) string {
		return strings.ReplaceAll(input, "DECL", `<?xml version="1.0" encoding="`+encoding+`"?>`)
	}
	utf8 := []byte(declaration("UTF-8"))
	be := encodeUTF16(declaration("UTF-16BE"), binary.BigEndian)
	le := encodeUTF16(declaration("UTF-16LE"), binary.LittleEndian)
	variants := map[string][]byte{
		"UTF8": utf8, "UTF8-BOM": append([]byte{0xef, 0xbb, 0xbf}, utf8...),
		"UTF16BE": be, "UTF16LE": le,
		"Latin1": []byte(declaration("ISO-8859-1")),
	}
	// BOM-less UTF-16 requires a first declaration naming its byte order.
	// Do not prepend one and accidentally turn valid fixtures into repeats.
	if strings.HasPrefix(input, "DECL") {
		variants["UTF16BE-no-BOM"] = be[2:]
		variants["UTF16LE-no-BOM"] = le[2:]
	}
	return variants
}

func xmlDeclarationUTF8Input(input string) []byte {
	return []byte(strings.ReplaceAll(input, "DECL", `<?xml version="1.0"?>`))
}
