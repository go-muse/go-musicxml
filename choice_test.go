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
