package musicxml

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttributeDefaultsPreserveOmission(t *testing.T) {
	t.Parallel()

	document, err := Decode(strings.NewReader(`
<score-partwise>
	<part-list/>
</score-partwise>`))
	require.NoError(t, err)

	score, ok := document.(*ScorePartwise)
	require.True(t, ok)
	assert.Nil(t, score.Version)
	assert.Equal(t, "1.0", score.EffectiveVersion())

	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, score))
	assert.NotContains(t, encoded.String(), "version=")

	explicit := "4.0"
	score.Version = &explicit
	assert.Equal(t, "4.0", score.EffectiveVersion())
}

func TestTypedAttributeDefaults(t *testing.T) {
	t.Parallel()

	barline := &Barline{}
	assert.Nil(t, barline.Location)
	assert.Equal(
		t,
		RightLeftMiddleRight,
		barline.EffectiveLocation(),
	)

	explicit := RightLeftMiddleMiddle
	barline.Location = &explicit
	assert.Equal(t, explicit, barline.EffectiveLocation())

	beam := &Beam{}
	assert.Equal(t, BeamLevel(1), beam.EffectiveNumber())

	octaveShift := &OctaveShift{}
	assert.Equal(t, uint64(8), octaveShift.EffectiveSize())
}

func TestFixedAttributeHelpers(t *testing.T) {
	t.Parallel()

	link := &Link{}
	assert.Nil(t, link.Type)
	assert.Equal(t, "simple", link.EffectiveType())
	assert.True(t, link.TypeMatchesFixed())
	assert.Equal(t, "replace", link.EffectiveShow())
	assert.Equal(t, "onRequest", link.EffectiveActuate())

	invalid := "extended"
	link.Type = &invalid
	assert.Equal(t, invalid, link.EffectiveType())
	assert.False(t, link.TypeMatchesFixed())

	valid := "simple"
	link.Type = &valid
	assert.True(t, link.TypeMatchesFixed())
}

func TestOpusAttributeValueConstraints(t *testing.T) {
	t.Parallel()

	document, err := Decode(strings.NewReader(`
<opus xmlns:xlink="http://www.w3.org/1999/xlink">
	<score xlink:href="score.musicxml"/>
</opus>`))
	require.NoError(t, err)

	opus, ok := document.(*OpusDocument)
	require.True(t, ok)
	assert.Nil(t, opus.Version)
	assert.Equal(t, "1.0", opus.EffectiveVersion())
	require.Len(t, opus.Content, 1)
	require.NotNil(t, opus.Content[0].Score)

	score := opus.Content[0].Score
	assert.Equal(t, "simple", score.EffectiveType())
	assert.True(t, score.TypeMatchesFixed())
	assert.Equal(t, "replace", score.EffectiveShow())
	assert.Equal(t, "onRequest", score.EffectiveActuate())

	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, opus))
	assert.NotContains(t, encoded.String(), "version=")
	assert.NotContains(t, encoded.String(), "type=")
	assert.NotContains(t, encoded.String(), "show=")
	assert.NotContains(t, encoded.String(), "actuate=")
}
