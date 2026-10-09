package musicxml

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// XML 1.0 productions 1, 3, 22 and 27 allow only XML S, comments and PIs
// around the root (plus the declaration/DOCTYPE in their prolog positions).
// https://www.w3.org/TR/REC-xml/#sec-prolog-dtd
// These are literal-character tests, independent of MusicXML/XSD validity.
// Lexical handling of references and CDATA outside the root is a separate
// reader limitation: decoded CharData alone cannot distinguish their spelling.
func xmlDocumentWhitespaceCases() []xmlReadingCase {
	const root = `<ROOT><KNOWN>Keep</KNOWN></ROOT>`
	var cases []xmlReadingCase
	for _, character := range []rune{
		' ', '\t', '\r', '\n',
		'\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005',
		'\u2006', '\u2007', '\u2008', '\u2009', '\u200a',
		'\u2028', '\u2029', '\u202f', '\u205f', '\u3000',
	} {
		valid := character == ' ' || character == '\t' || character == '\r' || character == '\n'
		for _, position := range []struct{ name, before, after string }{
			{"before root", "", root},
			{"post declaration", "DECL", root},
			{"after root", "DECL" + root, ""},
		} {
			cases = append(cases, xmlReadingCase{
				name:  fmt.Sprintf("%s/U+%04X", position.name, character),
				input: position.before + " \t" + string(character) + "\r\n" + position.after,
				valid: valid,
			})
		}
	}
	return append(cases, []xmlReadingCase{
		{"empty boundaries", root, true},
		{"XML S and misc", "DECL \t\r\n<!--\u00a0--><?note \u0085?><!DOCTYPE ROOT>\n" + root + "\r<!--\u1680--><?note \u3000?> \t\n", true},
		{"omitted declaration and misc", " \t<!--\u00a0--><?note \u0085?>\r\n" + root + "\n<?note end?>\t", true},
		{"Unicode after prolog misc", "DECL <!--ok--><?note ok?> \u00a0\n" + root, false},
		{"Unicode after doctype", "DECL<!DOCTYPE ROOT> \u0085\n" + root, false},
		{"Unicode after tail misc", "DECL" + root + " <!--ok--><?note ok?> \u00a0\n", false},
	}...)
}

func TestXMLDocumentWhitespaceDecodePaths(t *testing.T) {
	runXMLReadingDecodePaths(t, xmlDocumentWhitespaceCases(), xmlDocumentWhitespaceEncodingVariants, assertXMLReadingError)
}

func TestXMLDocumentWhitespaceContainer(t *testing.T) {
	runXMLReadingContainer(t, xmlDocumentWhitespaceCases(), xmlDocumentWhitespaceEncodingVariants, assertXMLReadingError)
}

func TestXMLDocumentWhitespaceValidationParser(t *testing.T) {
	runXMLReadingValidationParser(t, xmlDocumentWhitespaceCases(), xmlDeclarationUTF8Input, assertXMLReadingError)
}

func TestXMLDocumentWhitespaceLinkedResources(t *testing.T) {
	runXMLReadingLinkedResources(t, xmlDocumentWhitespaceCases(), xmlDocumentWhitespaceEncodingVariants, assertXMLReadingError)
}

func TestXMLDocumentWhitespaceErrors(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, decodeError, validationError string }{
		{"ordinary before", "text<opus/>", "musicxml: unexpected xml.CharData before root element", "unexpected character data before root"},
		{"ordinary after", "<opus/>text", "musicxml: unexpected xml.CharData after root element", "unexpected xml.CharData after root element"},
		{"Unicode before", "\u00a0<opus/>", "musicxml: unexpected xml.CharData before root element", "unexpected character data before root"},
		{"Unicode post declaration", "<?xml version='1.0'?>\u0085<opus/>", "musicxml: unexpected xml.CharData before root element", "unexpected character data before root"},
		{"Unicode after", "<opus/>\u0085", "musicxml: unexpected xml.CharData after root element", "unexpected xml.CharData after root element"},
		{"Unicode without root", " \t\u00a0\r\n", "musicxml: unexpected xml.CharData before root element", "unexpected character data before root"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(test.input))
			assert.EqualError(t, err, test.decodeError)
			assert.NotErrorIs(t, err, ErrEmptyDocument)
			_, err = parseValidationDocument([]byte(test.input))
			assert.EqualError(t, err, test.validationError)
			assert.NotErrorIs(t, err, ErrEmptyDocument)
		})
	}
	for _, input := range []string{"", " \t\r\n"} {
		t.Run(fmt.Sprintf("empty %q", input), func(t *testing.T) {
			_, err := Decode(strings.NewReader(input))
			assert.ErrorIs(t, err, ErrEmptyDocument)
			_, err = parseValidationDocument([]byte(input))
			assert.ErrorIs(t, err, ErrEmptyDocument)
		})
	}
}

var xmlDocumentWhitespaceTextCases = []struct{ name, text string }{
	{"Latin1", "\u00a0\u0085Keep\u00a0"},
	{"Unicode", "\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200aKeep\u2028\u2029\u202f\u205f\u3000"},
}

func TestXMLDocumentWhitespacePreservesText(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		for _, test := range xmlDocumentWhitespaceTextCases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				input := xmlWellFormednessInput(`DECL<ROOT><KNOWN>`+test.text+`</KNOWN></ROOT>`, root)
				node, err := parseValidationDocument(xmlDeclarationUTF8Input(input))
				require.NoError(t, err)
				require.Len(t, node.Children, 1)
				assert.Equal(t, test.text, node.Children[0].Text.String())
				for encoding, data := range xmlDocumentWhitespaceEncodingVariants(input) {
					t.Run(encoding, func(t *testing.T) {
						document, err := Decode(iotest.OneByteReader(bytes.NewReader(data)))
						require.NoError(t, err)
						switch value := document.(type) {
						case *ScorePartwise:
							assert.Equal(t, Ptr(test.text), value.MovementTitle)
						case *ScoreTimewise:
							assert.Equal(t, Ptr(test.text), value.MovementTitle)
						case *OpusDocument:
							assert.Equal(t, Ptr(test.text), value.Title)
						default:
							t.Fatalf("unexpected document %T", document)
						}
						var encoded bytes.Buffer
						require.NoError(t, Encode(&encoded, document))
						roundTrip, err := Decode(&encoded)
						require.NoError(t, err)
						assert.Equal(t, document, roundTrip)
					})
				}
			})
		}
	}
}

// Do not reuse the ASCII-only Latin1 fixture helpers for Unicode cases.
// Omitted declarations stay omitted; Latin1 requires a declaration and every
// character must fit in one byte. UTF-16 fixtures contain real code units.
func xmlDocumentWhitespaceEncodingVariants(input string) map[string][]byte {
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
	if strings.HasPrefix(input, "DECL") {
		variants["UTF16BE-no-BOM"] = be[2:]
		variants["UTF16LE-no-BOM"] = le[2:]
		var latin1 []byte
		for _, character := range declaration("ISO-8859-1") {
			if character > 0xff {
				return variants
			}
			latin1 = append(latin1, byte(character))
		}
		variants["Latin1"] = latin1
	}
	return variants
}

// Compare original bytes, including supported encodings, without round-tripping
// through the model. This is XML well-formedness only: no --schema or --valid.
func TestXMLDocumentWhitespaceAgainstXMLLint(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this XML well-formedness test")
	}
	cases := xmlDocumentWhitespaceCases()
	for _, test := range xmlDocumentWhitespaceTextCases {
		cases = append(cases, xmlReadingCase{"preserved text " + test.name, `DECL<ROOT><KNOWN>` + test.text + `</KNOWN></ROOT>`, true})
	}
	for _, root := range []string{"score-partwise", "score-timewise", "opus", "container"} {
		for _, test := range cases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				input := xmlWellFormednessInput(test.input, root)
				if root == "container" {
					input = strings.Replace(input, `</container>`, `<rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`, 1)
				}
				for encoding, data := range xmlDocumentWhitespaceEncodingVariants(input) {
					t.Run(encoding, func(t *testing.T) {
						command := exec.Command(xmllint, "--nonet", "--noout", "-")
						command.Stdin = bytes.NewReader(data)
						output, err := command.CombinedOutput()
						if test.valid {
							assert.NoErrorf(t, err, "independent XML well-formedness: %s", output)
						} else {
							var exitError *exec.ExitError
							if assert.ErrorAsf(t, err, &exitError, "independent XML parser accepted source: %s", output) {
								assert.Equalf(t, 1, exitError.ExitCode(), "expected XML parser failure: %s", output)
							}
						}
						if root != "container" {
							_, err = Decode(bytes.NewReader(data))
							assert.Equal(t, test.valid, err == nil, "Decode: %v", err)
						} else {
							_, err = decodeMXLContainer(data)
							assert.Equal(t, test.valid, err == nil, "container XML reader: %v", err)
						}
						if encoding == "UTF8" {
							_, err = parseValidationDocument(data)
							assert.Equal(t, test.valid, err == nil, "internal UTF-8 parser: %v", err)
						}
					})
				}
			})
		}
	}
}
