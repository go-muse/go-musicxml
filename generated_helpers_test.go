package musicxml

import (
	"encoding/xml"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnumerationHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []YesNo{YesNoYes, YesNoNo}, YesNo("").Values())
	assert.True(t, YesNoYes.IsKnown())
	assert.False(t, YesNo("maybe").IsKnown())
	assert.False(t, YesNo(" yes").IsKnown(), "the match is exact")

	// An empty value is known exactly when the schema enumerates it.
	assert.True(t, BreathMarkValueEmpty.IsKnown())
	assert.False(t, YesNo("").IsKnown())

	// Values returns a new slice each time.
	values := NoteTypeValue("").Values()
	require.NotEmpty(t, values)
	values[0] = "changed"
	assert.NotEqual(t, NoteTypeValue("changed"), NoteTypeValue("").Values()[0])
	for _, value := range NoteTypeValue("").Values() {
		assert.True(t, value.IsKnown(), value)
	}

	// A restriction of another enumeration lists only its own values.
	assert.NotContains(t, SwingTypeValue("").Values(), SwingTypeValue(NoteTypeValueWhole))
	assert.False(t, SwingTypeValue(NoteTypeValueWhole).IsKnown())

	// A union's constants are only its enumerated members, so it has no
	// helpers.
	for _, union := range []any{NumberOrNormal(""), PositiveIntegerOrEmpty("")} {
		_, isKnown := reflect.TypeOf(union).MethodByName("IsKnown")
		_, values := reflect.TypeOf(union).MethodByName("Values")
		assert.False(t, isKnown || values, "%T", union)
	}
}

func TestAttributeEnumerationConstants(t *testing.T) {
	t.Parallel()

	show, actuate, kind, space := XLinkShowNew, XLinkActuateOnRequest, XLinkTypeSimple, XMLSpacePreserve
	assert.Equal(t, "new", show)
	assert.Equal(t, "onRequest", actuate)
	assert.Equal(t, "simple", kind)
	assert.Equal(t, "preserve", space)

	link := Link{Href: "score.musicxml", Show: &show, Actuate: &actuate}
	encoded, err := xml.Marshal(link)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `show="new"`)
	assert.Contains(t, string(encoded), `actuate="onRequest"`)
}

func TestContentValueAndElementName(t *testing.T) {
	t.Parallel()

	// Variants that share a Go type are told apart by ElementName.
	p, f := DynamicsContent{P: &Empty{}}, DynamicsContent{F: &Empty{}}
	assert.Same(t, p.P, p.Value())
	assert.Same(t, f.F, f.Value())
	assert.Equal(t, "p", p.ElementName())
	assert.Equal(t, "f", f.ElementName())

	// Nothing or more than one variant is not a value.
	assert.Nil(t, DynamicsContent{}.Value())
	assert.Empty(t, DynamicsContent{}.ElementName())
	both := DynamicsContent{P: &Empty{}, F: &Empty{}}
	assert.Nil(t, both.Value())
	assert.Empty(t, both.ElementName())

	// Every decoded child reports its variant and element name.
	document, err := Decode(strings.NewReader(`<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list>` +
		`<part id="P1"><measure number="1"><direction><direction-type><dynamics><pp/><sfz/></dynamics></direction-type></direction></measure></part></score-partwise>`))
	require.NoError(t, err)
	score, ok := AsScorePartwise(document)
	require.True(t, ok)
	var names []string
	for _, content := range score.Part[0].Measure[0].Content {
		direction, ok := content.Value().(*Direction)
		if !ok {
			continue
		}
		for _, dynamics := range direction.DirectionType[0].Dynamics {
			for _, child := range dynamics.Content {
				require.NotNil(t, child.Value())
				names = append(names, child.ElementName())
			}
		}
	}
	assert.Equal(t, []string{"pp", "sfz"}, names)
}
