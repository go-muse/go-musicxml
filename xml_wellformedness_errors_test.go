package musicxml

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXMLWellFormednessWithoutPosition(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		input   string
		message string
	}{
		{"late XML declaration", ` <?xml version="1.0"?><root/>`, "XML declaration must be at the start of the document"},
		{"reserved XML target", `<?XmL version="1.0"?><root/>`, `reserved XML processing instruction target "XmL"`},
		{"lexical duplicate", `<root a="1" a="2"/>`, "duplicate XML attribute {}a"},
		{"expanded duplicate", `<root xmlns:p="urn:x" xmlns:q="urn:x" p:a="1" q:a="2"/>`, "duplicate XML attribute {urn:x}a"},
		{"prefix undeclaration", `<root xmlns:p=""/>`, `namespace prefix undeclaring is not allowed: "p"`},
		{"in-root doctype", `<root><!DOCTYPE root></root>`, "DOCTYPE must precede the root element"},
		{"repeated doctype", `<!DOCTYPE root><!DOCTYPE root><root/>`, "multiple DOCTYPE declarations"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &wellFormedXMLTokenReader{
				source: rawXMLTokenReader{xml.NewDecoder(strings.NewReader(test.input))},
			}
			var err error
			if !assert.NotPanics(t, func() {
				for err == nil {
					_, err = reader.Token()
				}
			}) {
				return
			}
			var syntaxError *xml.SyntaxError
			require.ErrorAs(t, err, &syntaxError)
			assert.Zero(t, syntaxError.Line)
			assert.Equal(t, test.message, syntaxError.Msg)
		})
	}
}

func TestXMLWellFormednessErrorPositions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		input   string
		message string
		line    int
	}{
		{"late XML declaration", "<!-- comment -->\n\n<?xml version='1.0'?><ROOT/>", "XML declaration must be at the start of the document", 3},
		{"multiline XML declaration", "<ROOT>\n<unknown><?xml\n version='1.0'\n?></unknown></ROOT>", "XML declaration must be at the start of the document", 4},
		{"reserved XML target", "<ROOT/>\n\n<?XmL version='1.0'?>", `reserved XML processing instruction target "XmL"`, 3},
		{"lexical duplicate", "<ROOT>\n\n<KNOWN a='1' a='2'>Keep</KNOWN></ROOT>", "duplicate XML attribute {}a", 3},
		{"expanded duplicate", "<ROOT xmlns:p='urn:x' xmlns:q='urn:x'>\n<unknown>\n<child p:a='1' q:a='2'/></unknown></ROOT>", "duplicate XML attribute {urn:x}a", 3},
		{"foreign subtree", "<ROOT xmlns:p='urn:x'>\n<p:extension>\n<child a='1' a='2'/></p:extension></ROOT>", "duplicate XML attribute {}a", 3},
		{"multiline start tag", "<ROOT>\n<KNOWN a='1'\n a='2'\n>Keep</KNOWN></ROOT>", "duplicate XML attribute {}a", 4},
		{"in-root doctype", "<ROOT>\n<unknown>\n<!DOCTYPE ROOT></unknown></ROOT>", "DOCTYPE must precede the root element", 3},
		{"post-root doctype", "<ROOT/>\n\n<!DOCTYPE ROOT>", "DOCTYPE must precede the root element", 3},
		{"repeated doctype", "<!DOCTYPE ROOT>\n\n<!DOCTYPE ROOT><ROOT/>", "multiple DOCTYPE declarations", 3},
		{"multiline doctype", "<ROOT>\n<!DOCTYPE\n ROOT\n><KNOWN>Keep</KNOWN></ROOT>", "DOCTYPE must precede the root element", 4},
		{"prefix undeclaration", "<ROOT>\n\n<unknown xmlns:p='' a='1' p:a='2'/></ROOT>", "prefix undeclaring is not allowed", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := func(t *testing.T, err error) {
				t.Helper()
				var syntaxError *xml.SyntaxError
				require.ErrorAs(t, err, &syntaxError)
				assert.Equal(t, test.line, syntaxError.Line)
				assert.Contains(t, syntaxError.Msg, test.message)
			}
			for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
				t.Run(root, func(t *testing.T) {
					input := xmlWellFormednessInput(test.input, root)
					_, err := parseValidationDocument([]byte(input))
					check(t, err)
					for encoding, data := range xmlReadingEncodingVariants(input) {
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
									_, err := path.decode(iotest.OneByteReader(bytes.NewReader(content)))
									check(t, err)
								})
							}
							linked := &MXLPackage{
								Document:  &OpusDocument{Content: []OpusDocumentContent{{OpusLink: &OpusLink{Href: "linked.musicxml"}}}},
								RootFiles: []MXLRootFile{{FullPath: "main.musicxml"}},
								Resources: []MXLResource{{Path: "linked.musicxml", Data: data}},
							}
							_, err := linked.ResolveOpus()
							assert.ErrorIs(t, err, ErrMXLLinkedDocumentInvalid)
							check(t, err)
						})
					}
				})
			}
			t.Run("container", func(t *testing.T) {
				for encoding, data := range xmlReadingEncodingVariants(xmlWellFormednessInput(test.input, "container")) {
					t.Run(encoding, func(t *testing.T) {
						_, err := decodeMXLContainer(data)
						assert.ErrorIs(t, err, ErrMXLInvalidContainer)
						check(t, err)
					})
				}
			})
		})
	}
}
