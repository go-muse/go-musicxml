package musicxml

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDocumentDepthLimit(t *testing.T) {
	t.Parallel()

	var encoded bytes.Buffer
	require.NoError(t, Encode(
		&encoded,
		nestedOpusDocument(maximumDocumentDepth),
	))

	encoded.Reset()
	err := Encode(
		&encoded,
		nestedOpusDocument(maximumDocumentDepth+1),
	)
	assert.ErrorIs(t, err, ErrDocumentTooDeep)
	assert.Empty(t, encoded.Bytes())
}

func TestValidateDocumentDepthLimit(t *testing.T) {
	t.Parallel()

	assert.NoError(t, Validate(nestedOpusDocument(maximumDocumentDepth)))

	err := Validate(nestedOpusDocument(maximumDocumentDepth + 1))
	assert.ErrorIs(t, err, ErrInvalidDocument)
	assert.ErrorIs(t, err, ErrDocumentTooDeep)
}

func TestEncodeRejectsCyclicOpus(t *testing.T) {
	t.Parallel()

	root := NewOpusDocument()
	root.AddOpus(root)

	var encoded bytes.Buffer
	err := Encode(&encoded, root)
	assert.ErrorIs(t, err, ErrDocumentCycle)
	assert.Empty(t, encoded.Bytes())

	err = Validate(root)
	assert.ErrorIs(t, err, ErrInvalidDocument)
	assert.ErrorIs(t, err, ErrDocumentCycle)
}

func TestParseValidationDocumentDepthLimit(t *testing.T) {
	t.Parallel()

	_, err := parseValidationDocument([]byte(
		nestedOpusXML(maximumDocumentDepth + 1),
	))
	assert.ErrorIs(t, err, ErrDocumentTooDeep)
}

func TestEncodeAllowsSharedOpusSubtree(t *testing.T) {
	t.Parallel()

	shared := NewOpusDocument()
	root := NewOpusDocument()
	root.AddOpus(shared)
	root.AddOpus(shared)

	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, root))
	assert.Equal(t, 3, bytes.Count(encoded.Bytes(), []byte("<opus")))
}

func nestedOpusDocument(depth int) *OpusDocument {
	root := NewOpusDocument()
	current := root
	for index := 1; index < depth; index++ {
		child := NewOpusDocument()
		current.AddOpus(child)
		current = child
	}

	return root
}

func TestDocumentDepthIncludesOpusLeafElements(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		add  func(*OpusDocument)
	}{
		{"title", func(opus *OpusDocument) { opus.Title = Ptr("") }},
		{"score", func(opus *OpusDocument) { opus.AddScore(&OpusScore{Href: "score.musicxml"}) }},
		{"opus-link", func(opus *OpusDocument) { opus.AddOpusLink(&OpusLink{Href: "collection.musicxml"}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, depth := range []int{maximumDocumentDepth - 1, maximumDocumentDepth} {
				document := nestedOpusDocument(depth)
				leaf := document
				for len(leaf.Content) > 0 {
					leaf = leaf.Content[0].Opus
				}
				test.add(leaf)
				var output bytes.Buffer
				err := Encode(&output, document)
				if depth == maximumDocumentDepth {
					assert.ErrorIs(t, err, ErrDocumentTooDeep)
					assert.Empty(t, output.Bytes())
					assert.ErrorIs(t, Validate(document), ErrDocumentTooDeep)
				} else {
					require.NoError(t, err)
					assert.NoError(t, Validate(document))
					_, err = DecodeWithOptions(bytes.NewReader(output.Bytes()), DecodeOptions{MaxXMLDepth: maximumDocumentDepth})
					assert.NoError(t, err)
				}
			}
		})
	}
}
