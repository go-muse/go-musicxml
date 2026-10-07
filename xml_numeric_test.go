package musicxml

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecimalXMLUsesFixedPoint(t *testing.T) {
	t.Parallel()
	for _, number := range []float64{1e6, 1e-7, -1e6, -1e-7, math.SmallestNonzeroFloat64, math.MaxFloat64} {
		t.Run(strconv.FormatFloat(number, 'g', -1, 64), func(t *testing.T) {
			want := strconv.FormatFloat(number, 'f', -1, 64)
			for _, value := range []any{Divisions(number), PositiveDivisions(number), Tenths(number)} {
				encoded, err := xml.Marshal(value)
				require.NoError(t, err)
				assert.Contains(t, string(encoded), ">"+want+"</")
			}
			original := Note{Duration: Ptr(PositiveDivisions(number)), DefaultX: Ptr(Tenths(number))}
			encoded, err := xml.Marshal(original)
			require.NoError(t, err)
			assert.Contains(t, string(encoded), `<duration>`+want+`</duration>`)
			assert.Contains(t, string(encoded), `default-x="`+want+`"`)
			var decoded Note
			require.NoError(t, xml.Unmarshal(encoded, &decoded))
			assert.Equal(t, original, decoded)

			offset := Offset{Value: Divisions(number), Sound: Ptr(YesNoYes)}
			encoded, err = xml.Marshal(offset)
			require.NoError(t, err)
			assert.Contains(t, string(encoded), ">"+want+"</")
			var decodedOffset Offset
			require.NoError(t, xml.Unmarshal(encoded, &decodedOffset))
			assert.Equal(t, offset, decodedOffset)
		})
	}
}

func TestXMLUnsignedIntegerLexicalForms(t *testing.T) {
	t.Parallel()
	for _, lexical := range []string{"+1", "+0001", " \t+1\r\n", "-0", "-000", "0", "18446744073709551615"} {
		t.Run(lexical, func(t *testing.T) {
			want, err := parseXMLUnsignedInteger(lexical, 64)
			require.NoError(t, err)
			var named StaffNumber
			require.NoError(t, xml.Unmarshal([]byte(`<staff>`+lexical+`</staff>`), &named))
			assert.Equal(t, StaffNumber(want), named)
			var attributes Attributes
			require.NoError(t, xml.Unmarshal([]byte(`<attributes><staves>`+lexical+`</staves></attributes>`), &attributes))
			require.NotNil(t, attributes.Staves)
			assert.Equal(t, want, *attributes.Staves)
			var fret Fret
			require.NoError(t, xml.Unmarshal([]byte(`<fret>`+lexical+`</fret>`), &fret))
			assert.Equal(t, want, fret.Value)
			var repeat Repeat
			require.NoError(t, xml.Unmarshal([]byte(`<repeat times="`+lexical+`"/>`), &repeat))
			require.NotNil(t, repeat.Times)
			assert.Equal(t, want, *repeat.Times)
			var beam Beam
			require.NoError(t, xml.Unmarshal([]byte(`<beam number="`+lexical+`">begin</beam>`), &beam))
			require.NotNil(t, beam.Number)
			assert.Equal(t, BeamLevel(want), *beam.Number)
		})
	}
}

func TestXMLNumericTransportRemainsNonvalidating(t *testing.T) {
	t.Parallel()
	for _, lexical := range []string{"", " \t\r\n", "-1", "1e6", "NaN", "+Inf", "-Inf"} {
		t.Run("decimal_"+lexical, func(t *testing.T) {
			var value PositiveDivisions
			require.NoError(t, xml.Unmarshal([]byte(`<duration>`+lexical+`</duration>`), &value))
			encoded, err := xml.Marshal(value)
			require.NoError(t, err)
			var again PositiveDivisions
			require.NoError(t, xml.Unmarshal(encoded, &again))
			if math.IsNaN(float64(value)) {
				assert.True(t, math.IsNaN(float64(again)))
			} else {
				assert.Equal(t, value, again)
			}
		})
	}
	for _, lexical := range []string{"", " \t\r\n", "0", "-0"} {
		var value StaffNumber
		require.NoError(t, xml.Unmarshal([]byte(`<staff>`+lexical+`</staff>`), &value))
		assert.Zero(t, value)
		var note Note
		require.NoError(t, xml.Unmarshal([]byte(`<note><staff>`+lexical+`</staff></note>`), &note))
		require.NotNil(t, note.Staff)
		assert.Zero(t, *note.Staff)
	}
}

func TestParseXMLUnsignedIntegerRejectsInvalidForms(t *testing.T) {
	t.Parallel()
	for _, lexical := range []string{"", " ", "-1", "++1", "+-0", "--0", "1.0", "1e1", "1 0", "0x1", "1_0", "\u00a01", "1\u00a0", "18446744073709551616"} {
		_, err := parseXMLUnsignedInteger(lexical, 64)
		assert.Error(t, err, lexical)
	}
	_, err := parseXMLUnsignedInteger("+256", 8)
	assert.Error(t, err)
}

func TestNumericXMLPreservesEmbeddedFieldsAndOmissions(t *testing.T) {
	t.Parallel()
	var value MetronomeTuplet
	require.NoError(t, xml.Unmarshal([]byte(`<metronome-tuplet type="start" bracket="yes"><actual-notes>+3</actual-notes><normal-notes>+2</normal-notes><normal-type>eighth</normal-type></metronome-tuplet>`), &value))
	assert.Equal(t, uint64(3), value.ActualNotes)
	assert.Equal(t, uint64(2), value.NormalNotes)
	assert.Equal(t, StartStopStart, value.Type)
	require.NotNil(t, value.Bracket)
	assert.Equal(t, YesNoYes, *value.Bracket)
	encoded, err := xml.Marshal(value)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `type="start"`)
	assert.Contains(t, string(encoded), `<actual-notes>3</actual-notes><normal-notes>2</normal-notes><normal-type>eighth</normal-type>`)
	var again MetronomeTuplet
	require.NoError(t, xml.Unmarshal(encoded, &again))
	assert.Equal(t, value, again)

	var attributes Attributes
	require.NoError(t, xml.Unmarshal([]byte(`<attributes/>`), &attributes))
	assert.Nil(t, attributes.Staves)
	assert.Nil(t, attributes.Instruments)
	encoded, err = xml.Marshal(attributes)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "staves")
	assert.NotContains(t, string(encoded), "instruments")
}

func TestNumericXMLDoesNotChangeJSONRepresentation(t *testing.T) {
	t.Parallel()
	value := struct {
		Duration PositiveDivisions
		Staff    StaffNumber
	}{Duration: 1e6, Staff: 1}
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	assert.JSONEq(t, `{"Duration":1000000,"Staff":1}`, string(encoded))
	var decoded struct {
		Duration PositiveDivisions
		Staff    StaffNumber
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, value, decoded)
}

func TestNumericDocumentRoundTrip(t *testing.T) {
	t.Parallel()
	for _, decimal := range []string{"1000000", "0.0000001"} {
		source := `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Test</part-name></score-part></part-list><part id="P1"><measure number="1"><attributes><divisions>` + decimal + `</divisions><staves>+1</staves></attributes><note default-x="` + decimal + `"><rest/><duration>` + decimal + `</duration><staff>+1</staff></note></measure></part></score-partwise>`
		document, err := Decode(strings.NewReader(source))
		require.NoError(t, err)
		require.NoError(t, Validate(document))
		var encoded bytes.Buffer
		require.NoError(t, Encode(&encoded, document))
		assert.Contains(t, encoded.String(), `<duration>`+decimal+`</duration>`)
		again, err := Decode(bytes.NewReader(encoded.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, document, again)
		assert.NoError(t, Validate(again))
	}
}

func TestParseXMLUnsignedIntegerLexicalForms(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		lexical string
		want    uint64
	}{
		{"+1", 1},
		{"+0001", 1},
		{" \t+1\r\n", 1},
		{"-0", 0},
		{"-000", 0},
		{"0", 0},
		{"18446744073709551615", 18446744073709551615},
	} {
		actual, err := parseXMLUnsignedInteger(test.lexical, 64)
		require.NoError(t, err, test.lexical)
		assert.Equal(t, test.want, actual, test.lexical)
	}
	actual, err := parseXMLUnsignedInteger("+255", 8)
	require.NoError(t, err)
	assert.Equal(t, uint64(255), actual)
}
