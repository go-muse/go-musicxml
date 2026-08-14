package musicxml

import (
	"bytes"
	"encoding/xml"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArticulationsContentRoundTrip(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<articulations>` +
			`<accent/>` +
			`<staccato/>` +
			`<strong-accent type="up"/>` +
			`<accent/>` +
			`</articulations>`,
	)

	var actual Articulations
	err := xml.Unmarshal(input, &actual)

	require.NoError(t, err)
	require.Len(t, actual.Content, 4)
	assert.NotNil(t, actual.Content[0].Accent)
	assert.NotNil(t, actual.Content[1].Staccato)
	assert.NotNil(t, actual.Content[2].StrongAccent)
	assert.NotNil(t, actual.Content[3].Accent)

	encoded, err := marshalElement("articulations", actual)
	require.NoError(t, err)

	names, err := childElementNames(encoded)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"accent", "staccato", "strong-accent", "accent"},
		names,
	)
}

func TestPartListContentRoundTrip(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<part-list>` +
			`<part-group type="start" number="1"/>` +
			`<score-part id="P1"><part-name>Piano</part-name></score-part>` +
			`<part-group type="stop" number="1"/>` +
			`<score-part id="P2"><part-name>Violin</part-name></score-part>` +
			`</part-list>`,
	)

	var actual PartList
	err := xml.Unmarshal(input, &actual)

	require.NoError(t, err)
	require.Len(t, actual.Content, 4)
	assert.NotNil(t, actual.Content[0].PartGroup)
	assert.NotNil(t, actual.Content[1].ScorePart)
	assert.NotNil(t, actual.Content[2].PartGroup)
	assert.NotNil(t, actual.Content[3].ScorePart)

	encoded, err := marshalElement("part-list", actual)
	require.NoError(t, err)

	names, err := childElementNames(encoded)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"part-group", "score-part", "part-group", "score-part"},
		names,
	)
}

func TestListeningContentRoundTrip(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<listening>` +
			`<sync type="none"/>` +
			`<other-listening type="custom">value</other-listening>` +
			`<offset>2</offset>` +
			`</listening>`,
	)

	var actual Listening
	err := xml.Unmarshal(input, &actual)

	require.NoError(t, err)
	require.Len(t, actual.Content, 2)
	assert.NotNil(t, actual.Content[0].Sync)
	assert.NotNil(t, actual.Content[1].OtherListening)
	assert.NotNil(t, actual.Offset)

	encoded, err := marshalElement("listening", actual)
	require.NoError(t, err)

	names, err := childElementNames(encoded)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"sync", "other-listening", "offset"},
		names,
	)
}

func TestRepeatedSequenceContentRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("key", func(t *testing.T) {
		t.Parallel()

		input := []byte(
			`<key>` +
				`<key-step>C</key-step><key-alter>1</key-alter>` +
				`<key-accidental>sharp</key-accidental>` +
				`<key-step>D</key-step><key-alter>-1</key-alter>` +
				`</key>`,
		)
		var actual Key
		require.NoError(t, xml.Unmarshal(input, &actual))
		require.Len(t, actual.Content, 5)
		require.NotNil(t, actual.Content[0].KeyStep)
		assert.Equal(t, StepC, *actual.Content[0].KeyStep)
		require.NotNil(t, actual.Content[1].KeyAlter)
		assert.Equal(t, Semitones(1), *actual.Content[1].KeyAlter)
		require.NotNil(t, actual.Content[2].KeyAccidental)
		require.NotNil(t, actual.Content[3].KeyStep)
		assert.Equal(t, StepD, *actual.Content[3].KeyStep)

		encoded, err := marshalElement("key", actual)
		require.NoError(t, err)
		names, err := childElementNames(encoded)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"key-step",
			"key-alter",
			"key-accidental",
			"key-step",
			"key-alter",
		}, names)
	})

	t.Run("lyric", func(t *testing.T) {
		t.Parallel()

		input := []byte(
			`<lyric>` +
				`<syllabic>begin</syllabic><text>Hel</text>` +
				`<elision>‿</elision>` +
				`<syllabic>end</syllabic><text>lo</text>` +
				`</lyric>`,
		)
		var actual Lyric
		require.NoError(t, xml.Unmarshal(input, &actual))
		require.Len(t, actual.Content, 5)
		require.NotNil(t, actual.Content[1].Text)
		assert.Equal(t, "Hel", actual.Content[1].Text.Value)
		require.NotNil(t, actual.Content[4].Text)
		assert.Equal(t, "lo", actual.Content[4].Text.Value)

		encoded, err := marshalElement("lyric", actual)
		require.NoError(t, err)
		names, err := childElementNames(encoded)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"syllabic",
			"text",
			"elision",
			"syllabic",
			"text",
		}, names)
	})

	t.Run("time", func(t *testing.T) {
		t.Parallel()

		input := []byte(
			`<time>` +
				`<beats>3</beats><beat-type>8</beat-type>` +
				`<beats>2</beats><beat-type>4</beat-type>` +
				`</time>`,
		)
		var actual Time
		require.NoError(t, xml.Unmarshal(input, &actual))
		require.Len(t, actual.Content, 4)
		require.NotNil(t, actual.Content[0].Beats)
		assert.Equal(t, "3", *actual.Content[0].Beats)
		require.NotNil(t, actual.Content[3].BeatType)
		assert.Equal(t, "4", *actual.Content[3].BeatType)

		encoded, err := marshalElement("time", actual)
		require.NoError(t, err)
		names, err := childElementNames(encoded)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"beats",
			"beat-type",
			"beats",
			"beat-type",
		}, names)
	})
}

func TestDistinctRepeatedSequenceContentDoesNotCollapse(t *testing.T) {
	t.Parallel()

	var first Lyric
	require.NoError(t, xml.Unmarshal(
		[]byte(`<lyric><text>first</text></lyric>`),
		&first,
	))
	var second Lyric
	require.NoError(t, xml.Unmarshal(
		[]byte(`<lyric><text>second</text></lyric>`),
		&second,
	))

	assert.NotEqual(t, first, second)
}

func TestNoteEncodingUsesSchemaOrder(t *testing.T) {
	t.Parallel()

	note := Note{
		Rest:     &Rest{},
		Duration: Ptr(PositiveDivisions(1)),
		Tie:      []Tie{{Type: StartStopStart}},
	}
	encoded, err := marshalElement("note", note)
	require.NoError(t, err)
	names, err := childElementNames(encoded)
	require.NoError(t, err)
	assert.Equal(t, []string{"rest", "duration", "tie"}, names)
}

func TestChoiceContentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   Articulations
		wantErr string
	}{
		{
			name: "empty variant",
			value: Articulations{
				Content: []ArticulationsContent{{}},
			},
			wantErr: "must contain exactly one value, got 0",
		},
		{
			name: "multiple variants",
			value: Articulations{
				Content: []ArticulationsContent{{
					Accent:   &EmptyPlacement{},
					Staccato: &EmptyPlacement{},
				}},
			},
			wantErr: "must contain exactly one value, got 2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := marshalElement("articulations", test.value)

			assert.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestChoiceContentUnknownElement(t *testing.T) {
	t.Parallel()

	var actual Articulations
	err := xml.Unmarshal(
		[]byte(`<articulations><future-articulation/></articulations>`),
		&actual,
	)

	assert.EqualError(
		t,
		err,
		"musicxml: unsupported ArticulationsContent element {}future-articulation",
	)
}

func marshalElement(
	name string,
	value any,
) ([]byte, error) {
	var result bytes.Buffer
	encoder := xml.NewEncoder(&result)

	err := encoder.EncodeElement(value, xml.StartElement{
		Name: xml.Name{Local: name},
	})
	if err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

func childElementNames(value []byte) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(value))
	depth := 0
	var result []string

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}

		switch tokenValue := token.(type) {
		case xml.StartElement:
			if depth == 1 {
				result = append(result, tokenValue.Name.Local)
			}
			depth++

		case xml.EndElement:
			depth--
		}
	}
}
