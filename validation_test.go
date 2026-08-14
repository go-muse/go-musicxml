package musicxml

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOfficialExamples(t *testing.T) {
	t.Parallel()

	files := []string{
		"Chant.musicxml",
		"MozartTrio.musicxml",
		"MozaChloSample.musicxml",
		"OpusLink.musicxml",
		"OpusScore.musicxml",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			source, err := os.ReadFile(filepath.Join(
				"testdata",
				"official",
				file,
			))
			require.NoError(t, err)

			document, err := Decode(bytes.NewReader(source))
			require.NoError(t, err)
			assert.NoError(t, Validate(document))
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	valid := decodeValidationScore(t)
	invalidChoice := decodeValidationScore(t)
	invalidChoiceNote := validationFirstNote(t, invalidChoice)
	invalidChoiceNote.Pitch = &Pitch{
		Step:   StepC,
		Octave: 4,
	}

	invalidStep := decodeValidationScore(t)
	invalidStepNote := validationFirstNote(t, invalidStep)
	invalidStepNote.Rest = nil
	invalidStepNote.Pitch = &Pitch{
		Step:   Step("H"),
		Octave: 4,
	}

	invalidOctave := decodeValidationScore(t)
	invalidOctaveNote := validationFirstNote(t, invalidOctave)
	invalidOctaveNote.Rest = nil
	invalidOctaveNote.Pitch = &Pitch{
		Step:   StepC,
		Octave: 10,
	}

	noParts := decodeValidationScore(t)
	noParts.Part = nil

	unresolvedPart := decodeValidationScore(t)
	unresolvedPart.Part[0].ID = "P2"

	invalidFixed := "extended"
	opus := &OpusDocument{
		Content: []OpusDocumentContent{{
			Score: &OpusScore{
				Href: "score.musicxml",
				Type: &invalidFixed,
			},
		}},
	}

	tests := []struct {
		name           string
		document       Document
		wantConstraint string
		wantPath       string
	}{
		{
			name:     "valid score",
			document: valid,
		},
		{
			name:           "missing required part",
			document:       noParts,
			wantConstraint: "content-model",
			wantPath:       "/score-partwise",
		},
		{
			name:           "mutually exclusive note branches",
			document:       invalidChoice,
			wantConstraint: "content-model",
			wantPath:       "/score-partwise/part/measure/note/rest",
		},
		{
			name:           "enumeration",
			document:       invalidStep,
			wantConstraint: "enumeration",
			wantPath:       "/score-partwise/part/measure/note/pitch/step",
		},
		{
			name:           "numeric bound",
			document:       invalidOctave,
			wantConstraint: "maxInclusive",
			wantPath:       "/score-partwise/part/measure/note/pitch/octave",
		},
		{
			name:           "fixed attribute",
			document:       opus,
			wantConstraint: "fixed",
			wantPath:       "/opus/score/@{http://www.w3.org/1999/xlink}type",
		},
		{
			name:           "unresolved IDREF",
			document:       unresolvedPart,
			wantConstraint: "IDREF",
			wantPath:       "/score-partwise/part/@id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(test.document)
			if test.wantConstraint == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidDocument)

			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			require.NotEmpty(t, validationErr.Issues)
			assert.Equal(
				t,
				test.wantConstraint,
				validationErr.Issues[0].Constraint,
			)
			assert.Equal(
				t,
				test.wantPath,
				validationErr.Issues[0].Path,
			)
		})
	}
}

func TestValidateErrors(t *testing.T) {
	t.Parallel()

	var nilPartwise *ScorePartwise

	tests := []struct {
		name     string
		document Document
		want     error
	}{
		{
			name: "nil interface",
			want: ErrNilDocument,
		},
		{
			name:     "typed nil",
			document: nilPartwise,
			want:     ErrNilDocument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, Validate(test.document), test.want)
		})
	}
}

func TestValidateRejectsOutOfOrderContent(t *testing.T) {
	t.Parallel()

	document, err := Decode(strings.NewReader(`
<score-partwise version="4.0">
	<part-list>
		<score-part id="P1"><part-name>Music</part-name></score-part>
	</part-list>
	<part id="P1">
		<measure number="1">
			<attributes>
				<key>
					<key-step>C</key-step>
					<key-accidental>natural</key-accidental>
					<key-alter>0</key-alter>
				</key>
			</attributes>
		</measure>
	</part>
</score-partwise>`))
	require.NoError(t, err)

	validationErr := Validate(document)
	require.ErrorIs(t, validationErr, ErrInvalidDocument)
	var details *ValidationError
	require.ErrorAs(t, validationErr, &details)
	require.NotEmpty(t, details.Issues)
	assert.Equal(t, "content-model", details.Issues[0].Constraint)
}

func TestValidateRegularNoteWithTie(t *testing.T) {
	t.Parallel()

	score := NewScorePartwise(
		PartDefinition{ID: "P1", Name: "Piano"},
	)
	measure := NewScorePartwiseMeasure("1")
	note := NewRestNote(1)
	note.Tie = []Tie{{Type: StartStopStart}}
	measure.AddNote(note)
	score.Part[0].AddMeasure(measure)

	assert.NoError(t, score.Validate())
}

func TestValidateDateTimeBuiltins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		builtin string
		value   string
		valid   bool
	}{
		{name: "date", builtin: "date", value: "2024-02-29", valid: true},
		{name: "date timezone", builtin: "date", value: "-0001-12-31+14:00", valid: true},
		{name: "date text", builtin: "date", value: "not-a-date"},
		{name: "date non-leap", builtin: "date", value: "2023-02-29"},
		{name: "date year zero", builtin: "date", value: "0000-01-01"},
		{name: "date timezone range", builtin: "date", value: "2024-01-01+14:01"},
		{name: "dateTime", builtin: "dateTime", value: "2024-02-29T23:59:59.5Z", valid: true},
		{name: "dateTime end of day", builtin: "dateTime", value: "2024-02-29T24:00:00Z", valid: true},
		{name: "dateTime bad end of day", builtin: "dateTime", value: "2024-02-29T24:00:00.1Z"},
		{name: "dateTime bad day", builtin: "dateTime", value: "2024-04-31T00:00:00"},
		{name: "time", builtin: "time", value: "12:30:45.25-04:00", valid: true},
		{name: "time missing seconds", builtin: "time", value: "12:30"},
		{name: "duration", builtin: "duration", value: "P1Y2M3DT4H5M6.7S", valid: true},
		{name: "negative duration", builtin: "duration", value: "-PT0S", valid: true},
		{name: "empty duration", builtin: "duration", value: "P"},
		{name: "empty duration time", builtin: "duration", value: "P1DT"},
		{name: "gDay", builtin: "gDay", value: "---31Z", valid: true},
		{name: "bad gDay", builtin: "gDay", value: "---32"},
		{name: "gMonth", builtin: "gMonth", value: "--12", valid: true},
		{name: "bad gMonth", builtin: "gMonth", value: "--13"},
		{name: "gMonthDay", builtin: "gMonthDay", value: "--02-29", valid: true},
		{name: "bad gMonthDay", builtin: "gMonthDay", value: "--04-31"},
		{name: "gYear", builtin: "gYear", value: "2024", valid: true},
		{name: "bad gYear", builtin: "gYear", value: "24"},
		{name: "gYearMonth", builtin: "gYearMonth", value: "2024-12Z", valid: true},
		{name: "bad gYearMonth", builtin: "gYearMonth", value: "2024-13"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			failure := validateBuiltin(test.builtin, test.value)
			if test.valid {
				assert.Nil(t, failure)
				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, "datatype", failure.constraint)
		})
	}
}

func TestValidateRejectsInvalidEncodingDate(t *testing.T) {
	t.Parallel()

	score := NewScorePartwise(
		PartDefinition{ID: "P1", Name: "Piano"},
	)
	measure := NewScorePartwiseMeasure("1")
	measure.AddNote(NewRestNote(1))
	score.Part[0].AddMeasure(measure)
	score.Identification = &Identification{Encoding: &Encoding{}}
	date := YYYYMMDD("not-a-date")
	score.Identification.Encoding.AddEncodingDate(&date)

	err := score.Validate()
	require.ErrorIs(t, err, ErrInvalidDocument)
	var details *ValidationError
	require.ErrorAs(t, err, &details)
	require.NotEmpty(t, details.Issues)
	assert.Equal(t, "datatype", details.Issues[0].Constraint)
	assert.Equal(
		t,
		"/score-partwise/identification/encoding/encoding-date",
		details.Issues[0].Path,
	)
}

func TestValidateUnicodeXMLNames(t *testing.T) {
	t.Parallel()

	score := NewScorePartwise(
		PartDefinition{ID: "P1", Name: "Piano"},
	)
	measure := NewScorePartwiseMeasure("1")
	note := NewRestNote(1)
	note.ID = Ptr("ня1")
	measure.AddNote(note)
	score.Part[0].AddMeasure(measure)

	assert.NoError(t, score.Validate())
}

func TestValidateXMLNameBuiltins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		builtin string
		value   string
		valid   bool
	}{
		{builtin: "Name", value: "part:голос-1", valid: true},
		{builtin: "NCName", value: "голос-1", valid: true},
		{builtin: "ID", value: "樂器1", valid: true},
		{builtin: "NMTOKEN", value: "1:голос", valid: true},
		{builtin: "QName", value: "муз:голос", valid: true},
		{builtin: "NCName", value: "1голос"},
		{builtin: "NCName", value: "муз:голос"},
		{builtin: "QName", value: "a:b:c"},
		{builtin: "NMTOKEN", value: "голос value"},
	}

	for _, test := range tests {
		failure := validateBuiltin(test.builtin, test.value)
		if test.valid {
			assert.Nil(t, failure, "%s %q", test.builtin, test.value)
			continue
		}
		assert.NotNil(t, failure, "%s %q", test.builtin, test.value)
	}
}

func decodeValidationScore(t *testing.T) *ScorePartwise {
	t.Helper()

	document, err := Decode(strings.NewReader(`
<score-partwise version="4.0">
	<part-list>
		<score-part id="P1">
			<part-name>Music</part-name>
		</score-part>
	</part-list>
	<part id="P1">
		<measure number="1">
			<note>
				<rest/>
				<duration>1</duration>
			</note>
		</measure>
	</part>
</score-partwise>`))
	require.NoError(t, err)

	result, ok := document.(*ScorePartwise)
	require.True(t, ok)
	return result
}

func validationFirstNote(
	t *testing.T,
	document *ScorePartwise,
) *Note {
	t.Helper()

	require.NotEmpty(t, document.Part)
	require.NotEmpty(t, document.Part[0].Measure)
	require.NotEmpty(t, document.Part[0].Measure[0].Content)

	note := document.Part[0].Measure[0].Content[0].Note
	require.NotNil(t, note)
	return note
}

func TestValidationError(t *testing.T) {
	t.Parallel()

	err := &ValidationError{Issues: []ValidationIssue{
		{
			Path:       "/score-partwise",
			Constraint: "content-model",
			Message:    "required child element is missing",
		},
		{
			Path:       "/score-partwise/@version",
			Constraint: "datatype",
			Message:    "invalid version",
		},
	}}

	assert.True(t, errors.Is(err, ErrInvalidDocument))
	assert.Contains(t, err.Error(), "2 issues")
	assert.Contains(t, err.Error(), "/score-partwise")
}
