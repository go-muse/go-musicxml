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

type xmlReadingCase struct {
	name  string
	input string
	valid bool
}

// Each rule family supplies its fixtures and encoding policy. Shared drivers
// keep the public decode, container and deferred-link paths identical while
// the invalid callback preserves each family's error-type expectations.
func runXMLReadingDecodePaths(t *testing.T, cases []xmlReadingCase, variants func(string) map[string][]byte, assertInvalid func(*testing.T, error)) {
	t.Helper()
	t.Parallel()
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		for _, test := range cases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				input := xmlWellFormednessInput(test.input, root)
				for encoding, data := range variants(input) {
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
									assertInvalid(t, err)
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

func runXMLReadingContainer(t *testing.T, cases []xmlReadingCase, variants func(string) map[string][]byte, assertInvalid func(*testing.T, error)) {
	t.Helper()
	t.Parallel()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := xmlWellFormednessInput(test.input, "container")
			// Preserve the malformed fragment while supplying usable metadata.
			input = strings.Replace(input, `</container>`, `<rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`, 1)
			for encoding, data := range variants(input) {
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
								assertInvalid(t, err)
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

func runXMLReadingValidationParser(t *testing.T, cases []xmlReadingCase, prepare func(string) []byte, assertInvalid func(*testing.T, error)) {
	t.Helper()
	t.Parallel()
	// This internal Encode/reparse helper takes plain UTF-8 only. Public
	// decoding owns BOM handling and other supported encodings.
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseValidationDocument(prepare(xmlWellFormednessInput(test.input, "opus")))
			if test.valid {
				assert.NoError(t, err)
			} else {
				assertInvalid(t, err)
			}
		})
	}
}

func runXMLReadingLinkedResources(t *testing.T, cases []xmlReadingCase, variants func(string) map[string][]byte, assertInvalid func(*testing.T, error)) {
	t.Helper()
	t.Parallel()
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		for _, test := range cases {
			t.Run(root+"/"+test.name, func(t *testing.T) {
				input := xmlWellFormednessInput(test.input, root)
				for encoding, data := range variants(input) {
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
							assertInvalid(t, err)
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

func xmlReadingUTF8Input(input string) []byte {
	return []byte(input)
}

func assertXMLReadingError(t *testing.T, err error) {
	t.Helper()
	assert.Error(t, err)
}

func assertXMLReadingSyntaxError(t *testing.T, err error) {
	t.Helper()
	var syntaxError *xml.SyntaxError
	assert.ErrorAs(t, err, &syntaxError)
}
