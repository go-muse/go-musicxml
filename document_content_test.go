package musicxml

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScorePartwiseMusicDataRoundTrip(t *testing.T) {
	t.Parallel()

	input := `<score-partwise version="4.0">` +
		`<part-list>` +
		`<score-part id="P1"><part-name>Piano</part-name></score-part>` +
		`</part-list>` +
		`<part id="P1"><measure number="1">` +
		`<note/>` +
		`<backup><duration>1</duration></backup>` +
		`<direction/>` +
		`<note/>` +
		`</measure></part>` +
		`</score-partwise>`

	document, err := Decode(strings.NewReader(input))
	require.NoError(t, err)

	actual, ok := document.(*ScorePartwise)
	require.True(t, ok)
	require.Len(t, actual.Part, 1)
	require.Len(t, actual.Part[0].Measure, 1)

	content := actual.Part[0].Measure[0].Content
	require.Len(t, content, 4)
	assert.NotNil(t, content[0].Note)
	assert.NotNil(t, content[1].Backup)
	assert.NotNil(t, content[2].Direction)
	assert.NotNil(t, content[3].Note)

	var encoded strings.Builder
	err = Encode(&encoded, actual)
	require.NoError(t, err)

	decodedAgain, err := Decode(strings.NewReader(encoded.String()))
	require.NoError(t, err)
	roundTrip := decodedAgain.(*ScorePartwise)
	roundTripContent := roundTrip.Part[0].Measure[0].Content
	require.Len(t, roundTripContent, 4)
	assert.NotNil(t, roundTripContent[0].Note)
	assert.NotNil(t, roundTripContent[1].Backup)
	assert.NotNil(t, roundTripContent[2].Direction)
	assert.NotNil(t, roundTripContent[3].Note)
}

func TestScoreTimewiseMusicDataRoundTrip(t *testing.T) {
	t.Parallel()

	input := `<score-timewise version="4.0">` +
		`<part-list>` +
		`<score-part id="P1"><part-name>Piano</part-name></score-part>` +
		`</part-list>` +
		`<measure number="1"><part id="P1">` +
		`<attributes/>` +
		`<note/>` +
		`<barline/>` +
		`</part></measure>` +
		`</score-timewise>`

	document, err := Decode(strings.NewReader(input))
	require.NoError(t, err)

	actual, ok := document.(*ScoreTimewise)
	require.True(t, ok)
	require.Len(t, actual.Measure, 1)
	require.Len(t, actual.Measure[0].Part, 1)

	content := actual.Measure[0].Part[0].Content
	require.Len(t, content, 3)
	assert.NotNil(t, content[0].Attributes)
	assert.NotNil(t, content[1].Note)
	assert.NotNil(t, content[2].Barline)

	var encoded strings.Builder
	err = Encode(&encoded, actual)
	require.NoError(t, err)

	decodedAgain, err := Decode(strings.NewReader(encoded.String()))
	require.NoError(t, err)
	roundTrip := decodedAgain.(*ScoreTimewise)
	roundTripContent := roundTrip.Measure[0].Part[0].Content
	require.Len(t, roundTripContent, 3)
	assert.NotNil(t, roundTripContent[0].Attributes)
	assert.NotNil(t, roundTripContent[1].Note)
	assert.NotNil(t, roundTripContent[2].Barline)
}
