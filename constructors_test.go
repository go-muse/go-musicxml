package musicxml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScorePartwise(t *testing.T) {
	t.Parallel()

	score := NewScorePartwise(
		PartDefinition{ID: "P1", Name: "Piano"},
		PartDefinition{ID: "P2", Name: "Violin"},
	)

	require.NotNil(t, score.Version)
	assert.Equal(t, MusicXMLVersion, *score.Version)
	require.Len(t, score.PartList.Content, 2)
	assert.Equal(
		t,
		"Piano",
		score.PartList.Content[0].ScorePart.PartName.Value,
	)
	assert.Equal(
		t,
		"Violin",
		score.PartList.Content[1].ScorePart.PartName.Value,
	)
	require.Len(t, score.Part, 2)
	assert.Equal(t, "P1", score.Part[0].ID)
	assert.Equal(t, "P2", score.Part[1].ID)
}

func TestNewScoreTimewise(t *testing.T) {
	t.Parallel()

	score := NewScoreTimewise(
		PartDefinition{ID: "P1", Name: "Piano"},
		PartDefinition{ID: "P2", Name: "Violin"},
	)
	measure := NewScoreTimewiseMeasure("1", "P1", "P2")
	score.AddMeasure(measure)

	require.NotNil(t, score.Version)
	assert.Equal(t, MusicXMLVersion, *score.Version)
	require.Len(t, score.PartList.Content, 2)
	require.Len(t, score.Measure, 1)
	require.Len(t, score.Measure[0].Part, 2)
	assert.Equal(t, "P1", score.Measure[0].Part[0].ID)
	assert.Equal(t, "P2", score.Measure[0].Part[1].ID)
}

func TestNewOpusDocument(t *testing.T) {
	t.Parallel()

	opus := NewOpusDocument()

	require.NotNil(t, opus.Version)
	assert.Equal(t, MusicXMLVersion, *opus.Version)
}

func TestNewNotes(t *testing.T) {
	t.Parallel()

	pitched := NewPitchedNote(StepC, 4, 2)
	require.NotNil(t, pitched.Pitch)
	assert.Equal(t, StepC, pitched.Pitch.Step)
	assert.Equal(t, Octave(4), pitched.Pitch.Octave)
	require.NotNil(t, pitched.Duration)
	assert.Equal(t, PositiveDivisions(2), *pitched.Duration)
	assert.Nil(t, pitched.Rest)

	rest := NewRestNote(3)
	require.NotNil(t, rest.Rest)
	require.NotNil(t, rest.Duration)
	assert.Equal(t, PositiveDivisions(3), *rest.Duration)
	assert.Nil(t, rest.Pitch)
}

func TestOrderedContentAddMethods(t *testing.T) {
	t.Parallel()

	measure := NewScorePartwiseMeasure("1")
	attributes := &Attributes{
		Divisions: Ptr(PositiveDivisions(1)),
	}
	note := NewPitchedNote(StepC, 4, 1)
	backup := &Backup{Duration: 1}

	assert.Same(t, attributes, measure.AddAttributes(attributes))
	assert.Same(t, note, measure.AddNote(note))
	assert.Same(t, backup, measure.AddBackup(backup))

	require.Len(t, measure.Content, 3)
	assert.Same(t, attributes, measure.Content[0].Attributes)
	assert.Same(t, note, measure.Content[1].Note)
	assert.Same(t, backup, measure.Content[2].Backup)
}

func TestConstructedScoreValidates(t *testing.T) {
	t.Parallel()

	score := NewScorePartwise(
		PartDefinition{ID: "P1", Name: "Piano"},
	)
	measure := NewScorePartwiseMeasure("1")
	measure.AddAttributes(&Attributes{
		Divisions: Ptr(PositiveDivisions(1)),
	})
	measure.AddNote(NewPitchedNote(StepC, 4, 1))
	score.Part[0].AddMeasure(measure)

	assert.NoError(t, score.Validate())
}

func TestConstructedTimewiseScoreValidates(t *testing.T) {
	t.Parallel()

	score := NewScoreTimewise(
		PartDefinition{ID: "P1", Name: "Piano"},
		PartDefinition{ID: "P2", Name: "Violin"},
	)
	measure := NewScoreTimewiseMeasure("1", "P1", "P2")
	measure.Part[0].AddNote(NewRestNote(1))
	measure.Part[1].AddNote(NewRestNote(1))
	score.AddMeasure(measure)

	assert.NoError(t, score.Validate())
}

func TestConstructedOpusValidates(t *testing.T) {
	t.Parallel()

	opus := NewOpusDocument()
	score := &OpusScore{Href: "scores/main.musicxml"}
	assert.Same(t, score, opus.AddScore(score))

	assert.NoError(t, opus.Validate())
}

func TestPtr(t *testing.T) {
	t.Parallel()

	value := Ptr("first")
	*value = "second"

	assert.Equal(t, "second", *value)
}
