package musicxml

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeIgnoresForeignNamespaceLookalikes(t *testing.T) {
	t.Parallel()
	input := `<score-partwise version="4.0" xmlns:v="urn:vendor" v:version="1.0" xmlns:version="urn:not-a-version">` +
		`<movement-title>Original</movement-title><v:movement-title>Extension</v:movement-title>` +
		`<part-list><score-part id="P1" v:id="OTHER" xmlns:id="urn:not-an-id"><part-name xml:lang="en">Piano</part-name></score-part></part-list>` +
		`<v:part id="OTHER"><measure number="2"/></v:part>` +
		`<part id="P1" v:id="OTHER"><measure number="1">` +
		`<note><pitch><step>C</step><v:step>D</v:step><octave>4</octave></pitch><duration>1</duration><v:duration>2</v:duration></note>` +
		`<direction><direction-type><wedge type="crescendo" xmlns:l="http://www.w3.org/1999/xlink" l:type="simple"/></direction-type></direction>` +
		`</measure></part></score-partwise>`
	document, err := DecodeScorePartwise(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, document.MovementTitle)
	assert.Equal(t, "Original", *document.MovementTitle)
	require.NotNil(t, document.Version)
	assert.Equal(t, "4.0", *document.Version)
	require.Len(t, document.Part, 1)
	assert.Equal(t, "P1", document.Part[0].ID)
	assert.Equal(t, "P1", document.PartList.Content[0].ScorePart.ID)
	content := document.Part[0].Measure[0].Content
	assert.Equal(t, StepC, content[0].Note.Pitch.Step)
	assert.Equal(t, PositiveDivisions(1), *content[0].Note.Duration)
	require.NoError(t, document.Validate())
	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, document))
	assert.NotContains(t, encoded.String(), "Extension")
	assert.NotContains(t, encoded.String(), "OTHER")
}

func TestDecodePreservesQualifiedSchemaAttributes(t *testing.T) {
	t.Parallel()
	input := `<score-partwise xmlns:l="http://www.w3.org/1999/xlink">` +
		`<work><opus l:href="collection.musicxml" l:type="simple" l:role="collection" l:title="Collection" l:show="new" l:actuate="onRequest"/></work>` +
		`<credit><credit-words xml:lang="en" xml:space="preserve">Title</credit-words></credit>` +
		`<part-list><score-part id="P1"><part-name>Piano</part-name>` +
		`<part-link l:href="part.musicxml"/></score-part></part-list>` +
		`<part id="P1"><measure number="1"><link l:href="linked.musicxml"/></measure></part></score-partwise>`
	document, err := DecodeScorePartwise(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, "collection.musicxml", document.Work.Opus.Href)
	assert.Equal(t, "simple", *document.Work.Opus.Type)
	assert.Equal(t, "collection", *document.Work.Opus.Role)
	assert.Equal(t, "Collection", *document.Work.Opus.Title)
	assert.Equal(t, "new", *document.Work.Opus.Show)
	assert.Equal(t, "onRequest", *document.Work.Opus.Actuate)
	assert.Equal(t, "part.musicxml", document.PartList.Content[0].ScorePart.PartLink[0].Href)
	assert.Equal(t, "en", *document.Credit[0].Content[0].CreditWords.Lang)
	assert.Equal(t, "preserve", *document.Credit[0].Content[0].CreditWords.Space)
	assert.Equal(t, "linked.musicxml", document.Part[0].Measure[0].Content[0].Link.Href)
	opus, err := DecodeOpusDocument(strings.NewReader(`<opus xmlns:l="http://www.w3.org/1999/xlink"><opus-link l:href="other.musicxml"/><score l:href="score.musicxml"/></opus>`))
	require.NoError(t, err)
	assert.Equal(t, "other.musicxml", opus.Content[0].OpusLink.Href)
	assert.Equal(t, "score.musicxml", opus.Content[1].Score.Href)
}

func TestDecodeNamespaceFilteringKeepsDocumentChecks(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`<opus/><v:opus xmlns:v="urn:vendor"/>`,
		`<opus><v:extension xmlns:v="urn:vendor"></opus>`,
		`<v:opus xmlns:v="urn:vendor"/>`,
	} {
		document, err := Decode(strings.NewReader(input))
		assert.Error(t, err, input)
		assert.Nil(t, document, input)
	}
	document, err := DecodeWithOptions(strings.NewReader(`<opus><v:extension xmlns:v="urn:vendor"><child/></v:extension></opus>`), DecodeOptions{MaxXMLDepth: 2})
	assert.ErrorIs(t, err, ErrXMLTooDeep)
	assert.Nil(t, document)
}

func TestDecodeIgnoresNamespaceDeclarations(t *testing.T) {
	t.Parallel()

	// Namespace declarations are legal on schema-valid input; they are not
	// attributes named version or id in the MusicXML model.
	input := `<score-partwise version="4.0" xmlns:version="urn:vendor">` +
		`<part-list><score-part id="P1" xmlns:id="urn:vendor"><part-name>Piano</part-name></score-part></part-list>` +
		`<part id="P1" xmlns:id="urn:vendor"><measure number="1"/></part></score-partwise>`
	document, err := DecodeScorePartwise(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, Ptr("4.0"), document.Version)
	assert.Equal(t, "P1", document.PartList.Content[0].ScorePart.ID)
	assert.Equal(t, "P1", document.Part[0].ID)
	assert.NoError(t, document.Validate())
}
