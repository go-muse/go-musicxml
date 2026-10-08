package musicxml

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These are XML-reading regressions, independent of MusicXML validity. ROOT
// and KNOWN are replaced with each supported root and a known text element.
var xmlWellFormednessCases = []struct {
	name  string
	input string
	valid bool
}{
	{"duplicate root attribute", `<ROOT version="4.0" version="3.0"/>`, false},
	{"duplicate known attribute", `<ROOT><KNOWN a="1" a="2">Keep</KNOWN></ROOT>`, false},
	{"duplicate unknown attribute", `<ROOT><unknown><child a="1" a="2"/></unknown></ROOT>`, false},
	{"duplicate foreign attribute", `<ROOT xmlns:v="urn:vendor"><v:unknown><child a="1" a="2"/></v:unknown></ROOT>`, false},
	{"duplicate default declaration", `<ROOT xmlns="" xmlns=""/>`, false},
	{"duplicate prefix declaration", `<ROOT xmlns:p="urn:first" xmlns:p="urn:second"/>`, false},
	{"empty prefix declaration", `<ROOT xmlns:p=""/>`, false},
	{"empty prefix declaration with attributes", `<ROOT xmlns:p="" a="1" p:a="2"/>`, false},
	{"empty prefix declaration after attributes", `<ROOT a="1" p:a="2" xmlns:p=""/>`, false},
	{"prefix undeclaration in unknown subtree", `<ROOT xmlns:p="urn:vendor"><unknown><child xmlns:p=""/></unknown></ROOT>`, false},
	{"prefix undeclaration in foreign subtree", `<ROOT xmlns:p="urn:vendor"><p:extension><child xmlns:p=""/></p:extension></ROOT>`, false},
	{"duplicate expanded root attribute", `<ROOT p:a="1" q:a="2" xmlns:p="urn:vendor" xmlns:q="urn:vendor"/>`, false},
	{"duplicate expanded known attribute", `<ROOT xmlns:p="urn:vendor" xmlns:q="urn:vendor"><KNOWN p:a="1" q:a="2">Keep</KNOWN></ROOT>`, false},
	{"duplicate expanded unknown attribute", `<ROOT xmlns:p="urn:vendor" xmlns:q="urn:vendor"><unknown><child p:a="1" q:a="2"/></unknown></ROOT>`, false},
	{"duplicate expanded foreign attribute", `<ROOT xmlns:p="urn:vendor" xmlns:q="urn:vendor"><p:unknown><child p:a="1" q:a="2"/></p:unknown></ROOT>`, false},
	{"duplicate after rebinding", `<ROOT xmlns:p="urn:first" xmlns:q="urn:second"><unknown xmlns:q="urn:first" p:a="1" q:a="2"/></ROOT>`, false},
	{"duplicate after scope restoration", `<ROOT xmlns:p="urn:first" xmlns:q="urn:first"><unknown xmlns:q="urn:second"/><unknown p:a="1" q:a="2"/></ROOT>`, false},
	{"duplicate reserved XML namespace", `<ROOT xmlns:p="http://www.w3.org/XML/1998/namespace" xml:lang="en" p:lang="fr"/>`, false},
	{"duplicate decoded namespace URI", `<ROOT xmlns:p="urn:vendor" xmlns:q="urn:vend&#x6f;r" p:a="1" q:a="2"/>`, false},
	{"duplicate literal xmlns URI", `<ROOT xmlns:p="xmlns" xmlns:q="xmlns" p:a="1" q:a="2"/>`, false},
	{"doctype inside root", `<ROOT><!DOCTYPE ROOT></ROOT>`, false},
	{"doctype inside known text", `<ROOT><KNOWN><!DOCTYPE ROOT></KNOWN></ROOT>`, false},
	{"doctype inside unknown subtree", `<ROOT><unknown><child><!DOCTYPE ROOT></child></unknown></ROOT>`, false},
	{"doctype inside foreign subtree", `<ROOT xmlns:v="urn:vendor"><v:unknown><child><!DOCTYPE ROOT></child></v:unknown></ROOT>`, false},
	{"doctype after root", `<ROOT/><!DOCTYPE ROOT>`, false},
	{"repeated prolog doctype", `<!DOCTYPE ROOT><!-- between --><?test ok?><!DOCTYPE ROOT><ROOT/>`, false},
	{"truncated doctype", `<!DOCTYPE ROOT`, false},
	{"no XML declaration", `<ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"legal prolog doctype", `<!-- before --><?test before?><!DOCTYPE ROOT><!-- after --><?test after?><ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"external DTD is not loaded", `<!DOCTYPE ROOT SYSTEM "file:///does-not-exist/musicxml.dtd"><ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"internal DTD is not applied", `<!DOCTYPE ROOT [<!ATTLIST ROOT version CDATA "3.0">]><ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"different attribute namespaces", `<ROOT xmlns:p="urn:first" xmlns:q="urn:second" a="0" p:a="1" q:a="2"><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"default namespace does not qualify attributes", `<ROOT><unknown xmlns="urn:vendor" xmlns:p="urn:vendor" a="1" p:a="2"/><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"empty default declaration remains legal", `<ROOT xmlns=""><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"default undeclaration in foreign subtree", `<ROOT><extension xmlns="urn:vendor"><child xmlns=""/></extension><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"same attributes on siblings", `<ROOT><unknown a="1"/><unknown a="2"/><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"distinct after rebinding", `<ROOT xmlns:p="urn:first" xmlns:q="urn:first"><unknown xmlns:q="urn:second" p:a="1" q:a="2"/><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"distinct after scope restoration", `<ROOT xmlns:p="urn:first" xmlns:q="urn:second"><unknown xmlns:q="urn:first"/><unknown p:a="1" q:a="2"/><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"declaration versus literal xmlns URI", `<ROOT xmlns:p="xmlns" xmlns:a="urn:vendor" p:a="1"><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"well-formed ignored extensions", `<ROOT xmlns:p="urn:first" xmlns:q="urn:second"><unknown a="1"><child a="2"/></unknown><p:extension p:a="1" q:a="2"><child/></p:extension><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"literal namespace URIs remain distinct", `<ROOT xmlns:p="urn:vendor" xmlns:q="urn:Vendor" p:a="1" q:a="2"><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"doctype internal comment", `<!DOCTYPE ROOT [<!-- <!DOCTYPE ignored> -->]><ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"doctype root name is not DTD validated", `<!DOCTYPE different><ROOT><KNOWN>Keep</KNOWN></ROOT>`, true},
	{"doctype XML whitespace", "<!DOCTYPE\tROOT><ROOT><KNOWN>Keep</KNOWN></ROOT>", true},
	{"doctype text is not a directive", `<ROOT><!-- <!DOCTYPE ROOT> --><?test DOCTYPE ROOT?><KNOWN><![CDATA[Keep]]></KNOWN></ROOT>`, true},
}

func TestXMLWellFormednessDecodePaths(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		for _, test := range xmlWellFormednessCases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				input := xmlWellFormednessInput(test.input, root)
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
								document, err := path.decode(iotest.OneByteReader(bytes.NewReader(content)))
								if !test.valid {
									assert.Error(t, err)
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

func TestXMLWellFormednessContainer(t *testing.T) {
	t.Parallel()
	for _, test := range xmlWellFormednessCases {
		t.Run(test.name, func(t *testing.T) {
			input := xmlWellFormednessInput(test.input, "container")
			// Preserve the malformed fragment while supplying usable metadata.
			input = strings.Replace(input, `</container>`, `<rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`, 1)
			for encoding, data := range xmlReadingEncodingVariants(input) {
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

func TestXMLWellFormednessValidationParser(t *testing.T) {
	t.Parallel()
	for _, test := range xmlWellFormednessCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseValidationDocument([]byte(xmlWellFormednessInput(test.input, "opus")))
			if test.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestXMLWellFormednessDepthBoundary(t *testing.T) {
	t.Parallel()
	for _, content := range []string{
		`<unknown><child a="1"/></unknown>`,
		`<v:extension xmlns:v="urn:vendor"><child a="1"/></v:extension>`,
	} {
		for encoding, data := range xmlReadingEncodingVariants(`<opus>` + content + `</opus>`) {
			t.Run(encoding+"/"+content, func(t *testing.T) {
				_, err := DecodeWithOptions(bytes.NewReader(data), DecodeOptions{MaxXMLDepth: 3})
				assert.NoError(t, err)
				_, err = DecodeWithOptions(bytes.NewReader(data), DecodeOptions{MaxXMLDepth: 2})
				assert.ErrorIs(t, err, ErrXMLTooDeep)
			})
		}
	}
}

func TestXMLWellFormednessLinkedResources(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		for _, test := range xmlWellFormednessCases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				input := xmlWellFormednessInput(test.input, root)
				for encoding, data := range xmlReadingEncodingVariants(input) {
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
							assert.Error(t, err)
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

func TestXMLDoctypeDoesNotFetchExternalDTD(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "DTD must not be fetched", http.StatusInternalServerError)
	}))
	defer server.Close()
	for _, declaration := range []string{
		` SYSTEM "` + server.URL + `/external.dtd"`,
		` PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "` + server.URL + `/external.dtd"`,
	} {
		input := `<!DOCTYPE score-partwise` + declaration + `><score-partwise><movement-title>Keep</movement-title></score-partwise>`
		document, err := Decode(strings.NewReader(input))
		require.NoError(t, err)
		assertXMLReadingTitle(t, document)
		metadata := `<!DOCTYPE container` + declaration + `><container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`
		_, err = decodeMXLContainer([]byte(metadata))
		require.NoError(t, err)
		_, err = parseValidationDocument([]byte(input))
		require.NoError(t, err)
	}
	assert.Zero(t, requests.Load())
}

func TestXMLAttributeCheckDoesNotInventNamespaceBindings(t *testing.T) {
	t.Parallel()
	// encoding/xml permits undeclared prefixes. This bounded repair must not
	// mistake that prefix for a literal URI and report an invented collision.
	_, err := Decode(strings.NewReader(`<opus xmlns:q="p" p:a="1" q:a="2"/>`))
	assert.NoError(t, err)
}

func xmlReadingEncodingVariants(input string) map[string][]byte {
	variants := namespaceEncodingVariants(input)
	variants["UTF8-BOM"] = append([]byte{0xef, 0xbb, 0xbf}, []byte(input)...)
	// The regression fixtures are ASCII, which is also valid Latin-1. An
	// explicit declaration exercises the separate CharsetReader path.
	variants["Latin1"] = []byte(`<?xml version="1.0" encoding="ISO-8859-1"?>` + input)
	return variants
}

func xmlWellFormednessInput(input, root string) string {
	known := "movement-title"
	if root == "opus" {
		known = "title"
	} else if root == "container" {
		known = "rootfiles"
	}
	return strings.NewReplacer("ROOT", root, "KNOWN", known).Replace(input)
}

func assertXMLReadingTitle(t *testing.T, document Document) {
	t.Helper()
	switch value := document.(type) {
	case *ScorePartwise:
		assert.Equal(t, Ptr("Keep"), value.MovementTitle)
		assert.Nil(t, value.Version)
	case *ScoreTimewise:
		assert.Equal(t, Ptr("Keep"), value.MovementTitle)
		assert.Nil(t, value.Version)
	case *OpusDocument:
		assert.Equal(t, Ptr("Keep"), value.Title)
		assert.Nil(t, value.Version)
	default:
		t.Fatalf("unexpected document %T", document)
	}
}

type xmlReadingPath struct {
	name    string
	archive bool
	decode  func(io.Reader) (Document, error)
}

func xmlReadingPaths(root string) []xmlReadingPath {
	paths := []xmlReadingPath{
		{"Decode", false, Decode},
		{"DecodeWithOptions", false, func(r io.Reader) (Document, error) { return DecodeWithOptions(r, DecodeOptions{MaxXMLDepth: 32}) }},
		{"DecodeMXL", true, DecodeMXL},
		{"DecodeMXLWithOptions", true, func(r io.Reader) (Document, error) { return DecodeMXLWithOptions(r, MXLOptions{MaxXMLDepth: 32}) }},
		{"DecodeMXLPackage", true, func(r io.Reader) (Document, error) {
			value, err := DecodeMXLPackage(r)
			if err != nil {
				return nil, err
			}
			return value.Document, nil
		}},
		{"DecodeMXLPackageWithOptions", true, func(r io.Reader) (Document, error) {
			value, err := DecodeMXLPackageWithOptions(r, MXLOptions{MaxXMLDepth: 32})
			if err != nil {
				return nil, err
			}
			return value.Document, nil
		}},
	}
	switch root {
	case "score-partwise":
		return append(paths,
			xmlReadingPath{"DecodeScorePartwise", false, func(r io.Reader) (Document, error) { return DecodeScorePartwise(r) }},
			xmlReadingPath{"DecodeScorePartwiseWithOptions", false, func(r io.Reader) (Document, error) {
				return DecodeScorePartwiseWithOptions(r, DecodeOptions{MaxXMLDepth: 32})
			}},
			xmlReadingPath{"DecodeMXLScorePartwise", true, func(r io.Reader) (Document, error) { return DecodeMXLScorePartwise(r) }},
			xmlReadingPath{"DecodeMXLScorePartwiseWithOptions", true, func(r io.Reader) (Document, error) {
				return DecodeMXLScorePartwiseWithOptions(r, MXLOptions{MaxXMLDepth: 32})
			}})
	case "score-timewise":
		return append(paths,
			xmlReadingPath{"DecodeScoreTimewise", false, func(r io.Reader) (Document, error) { return DecodeScoreTimewise(r) }},
			xmlReadingPath{"DecodeScoreTimewiseWithOptions", false, func(r io.Reader) (Document, error) {
				return DecodeScoreTimewiseWithOptions(r, DecodeOptions{MaxXMLDepth: 32})
			}},
			xmlReadingPath{"DecodeMXLScoreTimewise", true, func(r io.Reader) (Document, error) { return DecodeMXLScoreTimewise(r) }},
			xmlReadingPath{"DecodeMXLScoreTimewiseWithOptions", true, func(r io.Reader) (Document, error) {
				return DecodeMXLScoreTimewiseWithOptions(r, MXLOptions{MaxXMLDepth: 32})
			}})
	case "opus":
		return append(paths,
			xmlReadingPath{"DecodeOpusDocument", false, func(r io.Reader) (Document, error) { return DecodeOpusDocument(r) }},
			xmlReadingPath{"DecodeOpusDocumentWithOptions", false, func(r io.Reader) (Document, error) {
				return DecodeOpusDocumentWithOptions(r, DecodeOptions{MaxXMLDepth: 32})
			}},
			xmlReadingPath{"DecodeMXLOpusDocument", true, func(r io.Reader) (Document, error) { return DecodeMXLOpusDocument(r) }},
			xmlReadingPath{"DecodeMXLOpusDocumentWithOptions", true, func(r io.Reader) (Document, error) {
				return DecodeMXLOpusDocumentWithOptions(r, MXLOptions{MaxXMLDepth: 32})
			}})
	default:
		panic(fmt.Sprintf("unexpected root %q", root))
	}
}
