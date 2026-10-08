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
var xmlDeclarationCases = []struct {
	name  string
	input string
	valid bool
}{
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
	t.Parallel()
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		for _, test := range xmlDeclarationCases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				for encoding, data := range xmlDeclarationEncodingVariants(xmlWellFormednessInput(test.input, root)) {
					t.Run(encoding, func(t *testing.T) {
						archive := makeMXLTestArchive(t, []mxlTestEntry{
							mxlTestFileEntry(mxlContainerPath, `<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`),
							mxlTestFileEntry("score.musicxml", string(data)),
						})
						for _, path := range xmlReadingPaths(root) {
							t.Run(path.name, func(t *testing.T) {
								content := data
								if path.archive {
									content = archive
								}
								document, err := path.decode(iotest.OneByteReader(bytes.NewReader(content)))
								if !test.valid {
									var syntaxError *xml.SyntaxError
									assert.ErrorAs(t, err, &syntaxError)
									assert.Nil(t, document)
									return
								}
								require.NoError(t, err)
								assertXMLReadingTitle(t, document)
							})
						}
					})
				}
			})
		}
	}
}

func TestXMLDeclarationContainer(t *testing.T) {
	t.Parallel()
	for _, test := range xmlDeclarationCases {
		t.Run(test.name, func(t *testing.T) {
			input := xmlWellFormednessInput(test.input, "container")
			input = strings.Replace(input, `</container>`, `<rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`, 1)
			for encoding, data := range xmlDeclarationEncodingVariants(input) {
				t.Run(encoding, func(t *testing.T) {
					archive := makeMXLTestArchive(t, []mxlTestEntry{
						mxlTestFileEntry(mxlContainerPath, string(data)),
						mxlTestFileEntry("score.musicxml", `<opus><title>Keep</title></opus>`),
					})
					for _, path := range xmlReadingPaths("opus") {
						if !path.archive {
							continue
						}
						t.Run(path.name, func(t *testing.T) {
							document, err := path.decode(bytes.NewReader(archive))
							if !test.valid {
								assert.ErrorIs(t, err, ErrMXLInvalidContainer)
								var syntaxError *xml.SyntaxError
								assert.ErrorAs(t, err, &syntaxError)
								assert.Nil(t, document)
								return
							}
							require.NoError(t, err)
							assertXMLReadingTitle(t, document)
						})
					}
				})
			}
		})
	}
}

func TestXMLDeclarationValidationParser(t *testing.T) {
	t.Parallel()
	for _, test := range xmlDeclarationCases {
		t.Run(test.name, func(t *testing.T) {
			// This internal Encode/reparse helper takes UTF-8 only. Public
			// decoding owns BOM handling and other supported encodings.
			input := xmlWellFormednessInput(test.input, "opus")
			data := strings.ReplaceAll(input, "DECL", `<?xml version="1.0"?>`)
			_, err := parseValidationDocument([]byte(data))
			if test.valid {
				assert.NoError(t, err)
			} else {
				var syntaxError *xml.SyntaxError
				assert.ErrorAs(t, err, &syntaxError)
			}
		})
	}
}

func TestXMLDeclarationLinkedResources(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		for _, test := range xmlDeclarationCases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				for encoding, data := range xmlDeclarationEncodingVariants(xmlWellFormednessInput(test.input, root)) {
					t.Run(encoding, func(t *testing.T) {
						link := `<score xlink:href="linked.musicxml"/>`
						if root == "opus" {
							link = `<opus-link xlink:href="linked.musicxml"/>`
						}
						archive := makeMXLTestArchive(t, []mxlTestEntry{
							mxlTestFileEntry(mxlContainerPath, `<container><rootfiles><rootfile full-path="main.musicxml"/></rootfiles></container>`),
							mxlTestFileEntry("main.musicxml", `<opus xmlns:xlink="http://www.w3.org/1999/xlink">`+link+`</opus>`),
							mxlTestFileEntry("linked.musicxml", string(data)),
						})
						value, err := DecodeMXLPackage(bytes.NewReader(archive))
						require.NoError(t, err) // Resources remain opaque until resolution.
						resolved, err := value.ResolveOpus()
						if !test.valid {
							assert.ErrorIs(t, err, ErrMXLLinkedDocumentInvalid)
							var syntaxError *xml.SyntaxError
							assert.ErrorAs(t, err, &syntaxError)
							return
						}
						require.NoError(t, err)
						if root == "opus" {
							assertXMLReadingTitle(t, resolved.Content[0].OpusLink.Target.Document)
						} else {
							assertXMLReadingTitle(t, resolved.Content[0].Score.Target)
						}
					})
				}
			})
		}
	}
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
