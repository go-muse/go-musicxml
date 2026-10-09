package musicxml

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf16"
	"unicode/utf8"

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
	}
	// BOM-less UTF-16 requires a first declaration naming its byte order.
	// Do not prepend one and accidentally turn valid fixtures into repeats.
	if strings.HasPrefix(input, "DECL") {
		variants["UTF16BE-no-BOM"] = be[2:]
		variants["UTF16LE-no-BOM"] = le[2:]
	}
	// ASCII bytes are also valid undeclared UTF-8, preserving omitted and
	// misplaced-declaration fixtures. Non-ASCII Latin1 needs a first declaration;
	// omit it when any character is unrepresentable rather than relabeling UTF-8.
	maximum := rune(0x7f)
	if strings.HasPrefix(input, "DECL") {
		maximum = 0xff
	}
	var latin1 []byte
	for _, character := range declaration("ISO-8859-1") {
		if character > maximum {
			return variants
		}
		latin1 = append(latin1, byte(character))
	}
	variants["Latin1"] = latin1
	return variants
}

func xmlDeclarationUTF8Input(input string) []byte {
	return []byte(strings.ReplaceAll(input, "DECL", `<?xml version="1.0"?>`))
}

// Check source bytes independently of the runtime reader: a malformed encoding
// must not make a negative XML-reading fixture pass for the wrong reason.
func TestXMLDeclarationEncodingVariants(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input     string
		latin1, bomless bool
	}{
		{"ASCII declared", `DECL<opus><title>Keep</title></opus>`, true, true},
		{"ASCII omitted", `<opus><title>Keep</title></opus>`, true, false},
		{"ASCII misplaced", ` DECL<opus/>`, true, false},
		{"ASCII repeated", `DECLDECL<opus/>`, true, true},
		{"Latin1 declared", "DECL<opus><title>\u0085\u00a0\u00ff</title></opus>", true, true},
		{"Latin1 omitted", "<opus><title>\u0085\u00a0\u00ff</title></opus>", false, false},
		{"Latin1 misplaced", " DECL<opus><title>\u00a0</title></opus>", false, false},
		{"beyond Latin1", "DECL<opus><title>\u0100</title></opus>", false, true},
		{"Unicode declared", "DECL<opus><title>\u1680\u3000\U0001d11e</title></opus>", false, true},
		{"Unicode omitted", "<opus><title>\u1680\u3000\U0001d11e</title></opus>", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			variants := xmlDeclarationEncodingVariants(test.input)
			wantNames := []string{"UTF8", "UTF8-BOM", "UTF16BE", "UTF16LE"}
			if test.latin1 {
				wantNames = append(wantNames, "Latin1")
			}
			if test.bomless {
				wantNames = append(wantNames, "UTF16BE-no-BOM", "UTF16LE-no-BOM")
			}
			var names []string
			for name := range variants {
				names = append(names, name)
			}
			assert.ElementsMatch(t, wantNames, names)
			for name, data := range variants {
				t.Run(name, func(t *testing.T) {
					encoding, decoded := "UTF-8", ""
					switch name {
					case "UTF8", "UTF8-BOM":
						if name == "UTF8-BOM" {
							require.True(t, bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}))
							data = data[3:]
						}
						assert.True(t, utf8.Valid(data))
						decoded = string(data)
					case "UTF16BE", "UTF16LE", "UTF16BE-no-BOM", "UTF16LE-no-BOM":
						var order binary.ByteOrder = binary.BigEndian
						encoding = "UTF-16BE"
						bom := []byte{0xfe, 0xff}
						if strings.HasPrefix(name, "UTF16LE") {
							encoding, order, bom = "UTF-16LE", binary.LittleEndian, []byte{0xff, 0xfe}
						}
						if !strings.HasSuffix(name, "-no-BOM") {
							require.True(t, bytes.HasPrefix(data, bom))
							data = data[2:]
						}
						require.Zero(t, len(data)%2)
						var units []uint16
						for offset := 0; offset < len(data); offset += 2 {
							units = append(units, order.Uint16(data[offset:]))
						}
						decoded = string(utf16.Decode(units))
					case "Latin1":
						encoding = "ISO-8859-1"
						var characters []rune
						for _, character := range data {
							characters = append(characters, rune(character))
						}
						decoded = string(characters)
					default:
						t.Fatalf("unexpected encoding %q", name)
					}
					want := strings.ReplaceAll(test.input, "DECL", `<?xml version="1.0" encoding="`+encoding+`"?>`)
					assert.Equal(t, want, decoded)
				})
			}
		})
	}
}
