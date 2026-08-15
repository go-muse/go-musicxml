package musicxml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypedDocumentAccessors(t *testing.T) {
	t.Parallel()

	partwise := &ScorePartwise{}
	timewise := &ScoreTimewise{}
	opus := &OpusDocument{}

	actualPartwise, ok := AsScorePartwise(partwise)
	assert.True(t, ok)
	assert.Same(t, partwise, actualPartwise)
	_, ok = AsScorePartwise(timewise)
	assert.False(t, ok)
	_, ok = AsScorePartwise((*ScorePartwise)(nil))
	assert.False(t, ok)

	actualTimewise, ok := AsScoreTimewise(timewise)
	assert.True(t, ok)
	assert.Same(t, timewise, actualTimewise)
	_, ok = AsScoreTimewise(opus)
	assert.False(t, ok)
	_, ok = AsScoreTimewise((*ScoreTimewise)(nil))
	assert.False(t, ok)

	actualOpus, ok := AsOpusDocument(opus)
	assert.True(t, ok)
	assert.Same(t, opus, actualOpus)
	_, ok = AsOpusDocument(partwise)
	assert.False(t, ok)
	_, ok = AsOpusDocument((*OpusDocument)(nil))
	assert.False(t, ok)
}
