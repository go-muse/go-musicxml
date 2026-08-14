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
