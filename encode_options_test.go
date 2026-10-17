package musicxml

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeOptionsTestScore() *ScorePartwise {
	score := NewScorePartwise(PartDefinition{ID: "P1", Name: "Music"})
	score.Work = &Work{WorkTitle: Ptr("Title with  two spaces")}
	measure := NewScorePartwiseMeasure("1")
	measure.AddNote(NewPitchedNote(StepC, 4, 1))
	measure.AddNote(NewRestNote(1))
	score.Part[0].AddMeasure(measure)
	return score
}

func TestEncodeWithOptionsZeroMatchesEncode(t *testing.T) {
	t.Parallel()

	score := encodeOptionsTestScore()
	var expected, actual bytes.Buffer
	require.NoError(t, Encode(&expected, score))
	require.NoError(t, EncodeWithOptions(&actual, score, EncodeOptions{}))
	assert.Equal(t, expected.String(), actual.String())
}

func TestEncodeWithOptions(t *testing.T) {
	t.Parallel()

	score := encodeOptionsTestScore()
	var compact bytes.Buffer
	require.NoError(t, Encode(&compact, score))

	tests := map[string]EncodeOptions{
		"declaration":             {XMLDeclaration: true},
		"indent":                  {Indent: "  "},
		"tab indent":              {Indent: "\t"},
		"declaration with indent": {XMLDeclaration: true, Indent: "  "},
	}
	for name, options := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var encoded bytes.Buffer
			require.NoError(t, EncodeWithOptions(&encoded, score, options))
			text := encoded.String()

			assert.Equal(t, options.XMLDeclaration, strings.HasPrefix(text, xml.Header))
			if options.Indent != "" {
				assert.Contains(t, text, "\n"+options.Indent+"<part-list>")
			}
			assert.Contains(t, text, "<work-title>Title with  two spaces</work-title>")

			decoded, err := Decode(&encoded)
			require.NoError(t, err)
			var reencoded bytes.Buffer
			require.NoError(t, Encode(&reencoded, decoded))
			assert.Equal(t, compact.String(), reencoded.String())
		})
	}
}

func TestEncodeWithOptionsErrors(t *testing.T) {
	t.Parallel()

	score := encodeOptionsTestScore()
	for _, indent := range []string{"x", " \n", " "} {
		var encoded bytes.Buffer
		err := EncodeWithOptions(&encoded, score, EncodeOptions{Indent: indent})
		assert.ErrorIs(t, err, ErrInvalidEncodeOptions, "indent %q", indent)
		assert.Empty(t, encoded.Bytes())
	}

	assert.ErrorIs(t, EncodeWithOptions(nil, score, EncodeOptions{}), ErrNilWriter)
	assert.ErrorIs(
		t,
		EncodeWithOptions(&bytes.Buffer{}, nil, EncodeOptions{XMLDeclaration: true}),
		ErrNilDocument,
	)

	cyclic := NewOpusDocument()
	cyclic.AddOpus(cyclic)
	var encoded bytes.Buffer
	err := EncodeWithOptions(&encoded, cyclic, EncodeOptions{XMLDeclaration: true})
	assert.ErrorIs(t, err, ErrDocumentCycle)
	assert.Empty(t, encoded.Bytes(), "checks run before the declaration is written")

	invalidText := encodeOptionsTestScore()
	invalidText.Work.WorkTitle = Ptr("bad\x01")
	encoded.Reset()
	err = EncodeWithOptions(&encoded, invalidText, EncodeOptions{XMLDeclaration: true})
	assert.Error(t, err)
	assert.Empty(t, encoded.Bytes())
}

func TestResolveOpusWithOptions(t *testing.T) {
	t.Parallel()

	deep := `<score-partwise><work><work-title>T</work-title></work>` +
		`<part-list/></score-partwise>`
	value := opusPackageWithLink(
		&OpusScore{Href: "scores/linked.musicxml"},
		[]MXLResource{{Path: "scores/linked.musicxml", Data: []byte(deep)}},
	)

	resolved, err := value.ResolveOpus()
	require.NoError(t, err)
	withZero, err := value.ResolveOpusWithOptions(DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, resolved.Content[0].Score.Target, withZero.Content[0].Score.Target)

	_, err = value.ResolveOpusWithOptions(DecodeOptions{MaxXMLDepth: 2})
	assert.ErrorIs(t, err, ErrMXLLinkedDocumentInvalid)
	assert.ErrorIs(t, err, ErrXMLTooDeep)

	_, err = value.ResolveOpusWithOptions(DecodeOptions{MaxXMLDepth: 3})
	assert.NoError(t, err)

	_, err = value.ResolveOpusWithOptions(DecodeOptions{MaxXMLDepth: -1})
	assert.ErrorIs(t, err, ErrInvalidDecodeOptions)

	var missing *MXLPackage
	_, err = missing.ResolveOpusWithOptions(DecodeOptions{})
	assert.ErrorIs(t, err, ErrNilMXLPackage)
}

func TestValidateRejectsFlattenedChoiceConflict(t *testing.T) {
	t.Parallel()

	score := encodeOptionsTestScore()
	note := NewPitchedNote(StepD, 4, 1)
	note.Rest = &Rest{}
	score.Part[0].Measure[0].AddNote(note)

	err := Validate(score)
	assert.ErrorIs(t, err, ErrInvalidDocument)
}
