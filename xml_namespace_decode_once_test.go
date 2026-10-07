package musicxml

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Namespace names are compared literally, even when a (deprecated) relative
// namespace URI happens to equal a declared prefix or the reserved name xml.
func TestDecodeExpandsNamespacesOnce(t *testing.T) {
	t.Parallel()
	const attributes = ` xmlns:v="xlink" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:r="xml"`
	const content = `<work><opus xlink:href="correct.musicxml" v:href="wrong.musicxml"/></work>` +
		`<credit><credit-words xml:lang="en" r:lang="wrong">Title</credit-words></credit>`
	for _, root := range []string{"score-partwise", "score-timewise", "opus"} {
		input := `<` + root + attributes + `>` + content + `</` + root + `>`
		if root == "opus" {
			input = `<opus` + attributes + `><score xlink:href="correct.musicxml" v:href="wrong.musicxml"/></opus>`
		}
		for name, data := range namespaceEncodingVariants(input) {
			t.Run(root+"/"+name, func(t *testing.T) {
				t.Parallel()
				check := func(document Document) {
					switch value := document.(type) {
					case *ScorePartwise:
						assert.Equal(t, "correct.musicxml", value.Work.Opus.Href)
						assert.Equal(t, Ptr("en"), value.Credit[0].Content[0].CreditWords.Lang)
					case *ScoreTimewise:
						assert.Equal(t, "correct.musicxml", value.Work.Opus.Href)
						assert.Equal(t, Ptr("en"), value.Credit[0].Content[0].CreditWords.Lang)
					case *OpusDocument:
						assert.Equal(t, "correct.musicxml", value.Content[0].Score.Href)
					}
				}
				document, err := Decode(bytes.NewReader(data))
				require.NoError(t, err)
				check(document)
				switch root {
				case "score-partwise":
					document, err = DecodeScorePartwise(bytes.NewReader(data))
				case "score-timewise":
					document, err = DecodeScoreTimewise(bytes.NewReader(data))
				case "opus":
					document, err = DecodeOpusDocument(bytes.NewReader(data))
				}
				require.NoError(t, err)
				check(document)
				archive := makeMXLTestArchive(t, []mxlTestEntry{
					mxlTestFileEntry(mxlContainerPath, `<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`),
					mxlTestFileEntry("score.musicxml", string(data)),
				})
				document, err = DecodeMXL(bytes.NewReader(archive))
				require.NoError(t, err)
				check(document)
				value, err := DecodeMXLPackage(bytes.NewReader(archive))
				require.NoError(t, err)
				check(value.Document)
			})
		}
	}
}

func TestMXLContainerExpandsNamespacesOnce(t *testing.T) {
	t.Parallel()
	// After expansion, v:x is an ordinary foreign attribute, not a namespace
	// declaration. It must not redefine the namespace of y:rootfile.
	metadata := `<container xmlns:v="xmlns" xmlns:y="x" v:x=""><rootfiles>` +
		`<y:rootfile full-path="wrong.musicxml"/><rootfile full-path="score.musicxml"/>` +
		`</rootfiles></container>`
	for name, data := range namespaceEncodingVariants(metadata) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			archive := makeMXLTestArchive(t, []mxlTestEntry{
				mxlTestFileEntry(mxlContainerPath, string(data)),
				mxlTestFileEntry("score.musicxml", `<opus><title>Correct</title></opus>`),
				mxlTestFileEntry("wrong.musicxml", `<opus><title>Wrong</title></opus>`),
			})
			value, err := DecodeMXLPackage(bytes.NewReader(archive))
			require.NoError(t, err)
			assert.Equal(t, "score.musicxml", value.RootFiles[0].FullPath)
			document, err := DecodeMXL(bytes.NewReader(archive))
			require.NoError(t, err)
			assert.Equal(t, Ptr("Correct"), document.(*OpusDocument).Title)
		})
	}
}

func TestRawTokenWrappersKeepXMLChecks(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`<opus><title>text</wrong></opus>`,
		`<opus><title>text</opus>`,
		`<opus>`,
		`<opus xmlns:a="urn:vendor" xmlns:b="urn:vendor"><a:child></b:child></opus>`,
		`<opus/><opus/>`,
		`<opus/>tail`,
		`<foreign:opus xmlns:foreign="urn:vendor"/>`,
	} {
		for name, data := range namespaceEncodingVariants(input) {
			t.Run(name+"/"+input, func(t *testing.T) {
				document, err := Decode(bytes.NewReader(data))
				assert.Error(t, err)
				assert.Nil(t, document)
			})
		}
	}
	for name, data := range namespaceEncodingVariants(`<opus><v:extension xmlns:v="urn:vendor"><child/></v:extension></opus>`) {
		t.Run(name+"/depth", func(t *testing.T) {
			_, err := DecodeWithOptions(bytes.NewReader(data), DecodeOptions{MaxXMLDepth: 2})
			assert.ErrorIs(t, err, ErrXMLTooDeep)
		})
	}
	// The declaration-checking wrapper is also used without the depth wrapper
	// for container metadata, so exercise its standalone matching-tag checks.
	for name, data := range namespaceEncodingVariants(`<container><rootfiles></container>`) {
		t.Run(name+"/container", func(t *testing.T) {
			decoder, err := newXMLDecoder(bytes.NewReader(data))
			require.NoError(t, err)
			for err == nil {
				_, err = decoder.Token()
			}
			assert.NotErrorIs(t, err, io.EOF)
		})
	}
}

func namespaceEncodingVariants(input string) map[string][]byte {
	be := encodeUTF16(`<?xml version="1.0" encoding="UTF-16BE"?>`+input, binary.BigEndian)
	le := encodeUTF16(`<?xml version="1.0" encoding="UTF-16LE"?>`+input, binary.LittleEndian)
	return map[string][]byte{
		"UTF8": []byte(input), "UTF16BE": be, "UTF16LE": le,
		"UTF16BE-no-BOM": be[2:], "UTF16LE-no-BOM": le[2:],
	}
}

func TestDecodeNamespaceScopesAndIgnoredSubtrees(t *testing.T) {
	input := `<score-partwise xmlns:l="http://www.w3.org/1999/xlink"><movement-title>A<v:title xmlns:v="urn:vendor"><movement-title>wrong</movement-title></v:title>B</movement-title><part-list><score-part id="P1"><part-name xml:lang="en">Piano</part-name></score-part></part-list><part id="P1"><measure number="1"><direction xmlns:l="urn:vendor"><direction-type><wedge type="crescendo" l:type="wrong"/></direction-type></direction><link l:href="correct.musicxml"/><direction><direction-type xmlns="urn:vendor"><words xmlns="">wrong</words></direction-type><direction-type><words>right</words></direction-type></direction></measure></part></score-partwise>`
	document, err := DecodeScorePartwise(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := *document.MovementTitle; got != "AB" {
		t.Fatalf("title: %q", got)
	}
	content := document.Part[0].Measure[0].Content
	if got := content[0].Direction.DirectionType[0].Wedge.Type; got != WedgeTypeCrescendo {
		t.Fatalf("wedge: %q", got)
	}
	if got := content[1].Link.Href; got != "correct.musicxml" {
		t.Fatalf("link: %q", got)
	}
	if got := content[2].Direction.DirectionType[0].Content[0].Words.Value; got != "right" {
		t.Fatalf("words: %q", got)
	}
}
