package xsdgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMusicXML(t *testing.T) {
	t.Parallel()

	path := filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
		"musicxml.xsd",
	)

	file, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, file.Close())
	})

	schema, err := Parse(file)
	require.NoError(t, err)

	tests := []struct {
		name string
		got  int
		want int
	}{
		{name: "imports", got: len(schema.Imports), want: 2},
		{name: "simple types", got: len(schema.SimpleTypes), want: 145},
		{name: "complex types", got: len(schema.ComplexTypes), want: 224},
		{name: "attribute groups", got: len(schema.AttributeGroups), want: 45},
		{name: "groups", got: len(schema.Groups), want: 27},
		{name: "elements", got: len(schema.Elements), want: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.got)
		})
	}

	require.Len(t, schema.Elements, 2)
	assert.Equal(t, "score-partwise", schema.Elements[0].Name)
	assert.Equal(t, "score-timewise", schema.Elements[1].Name)
}
