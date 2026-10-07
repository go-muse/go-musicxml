package musicxml

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLLinkURISegmentsRoundTrip(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		href string
		path string
	}{
		{"/scores//../first.musicxml", "scores/first.musicxml"},
		{"../scores//../first.musicxml", "scores/first.musicxml"},
		{"/scores///../../first.musicxml#movement", "scores/first.musicxml"},
		{"../scores///../../first.musicxml#movement", "scores/first.musicxml"},
		{"/scores//../../first.musicxml", "first.musicxml"},
		{"../scores//../../first.musicxml", "first.musicxml"},
		{"/scores%2Fsub/../first.musicxml", "first.musicxml"},
		{"../scores%2fsub/../first.musicxml", "first.musicxml"},
		{"/scores%2Fsub/%2E%2E/first.musicxml", "first.musicxml"},
	} {
		for _, opusLink := range []bool{false, true} {
			kind := "score"
			if opusLink {
				kind = "opus-link"
			}
			t.Run(kind+"/"+test.href, func(t *testing.T) {
				t.Parallel()

				child := OpusDocumentContent{Score: &OpusScore{Href: test.href}}
				first := []byte(`<score-partwise><movement-title>Root-level score</movement-title></score-partwise>`)
				second := []byte(`<score-partwise><movement-title>Nested score</movement-title></score-partwise>`)
				if opusLink {
					child = OpusDocumentContent{OpusLink: &OpusLink{Href: test.href}}
					first = []byte(`<opus><title>Root-level opus</title></opus>`)
					second = []byte(`<opus><title>Nested opus</title></opus>`)
				}
				value := &MXLPackage{
					Document:  &OpusDocument{Content: []OpusDocumentContent{child}},
					RootFiles: []MXLRootFile{{FullPath: "collections/main.musicxml"}},
					Resources: []MXLResource{
						{Path: "first.musicxml", Data: first},
						{Path: "scores/first.musicxml", Data: second},
						{Path: "image.png", Data: []byte{0, 0xff, 0x89, 0x50}},
					},
				}
				before := cloneMXLResources(value.Resources)
				resolved, err := value.ResolveOpus()
				require.NoError(t, err)
				if opusLink {
					link := resolved.Content[0].OpusLink
					require.Equal(t, test.path, link.Path)
					link.Target.Document.Title = Ptr("Edited")
				} else {
					link := resolved.Content[0].Score
					require.Equal(t, test.path, link.Path)
					link.Target.(*ScorePartwise).MovementTitle = Ptr("Edited")
				}
				require.NoError(t, value.SyncResolvedOpus(resolved))
				for index, resource := range value.Resources {
					if resource.Path != test.path {
						assert.Equal(t, before[index], resource, "unrelated resource changed")
					} else {
						assert.Contains(t, string(resource.Data), "Edited")
					}
				}

				var output bytes.Buffer
				require.NoError(t, EncodeMXLPackage(&output, value))
				decoded, err := DecodeMXLPackage(bytes.NewReader(output.Bytes()))
				require.NoError(t, err)
				assert.Equal(t, value.Resources, decoded.Resources)
				roundTripped, err := decoded.ResolveOpus()
				require.NoError(t, err)
				if opusLink {
					link := roundTripped.Content[0].OpusLink
					assert.Equal(t, test.path, link.Path)
					assert.Equal(t, test.href, link.Link.Href)
					assert.Equal(t, Ptr("Edited"), link.Target.Document.Title)
				} else {
					link := roundTripped.Content[0].Score
					assert.Equal(t, test.path, link.Path)
					assert.Equal(t, test.href, link.Link.Href)
					assert.Equal(t, Ptr("Edited"), link.Target.(*ScorePartwise).MovementTitle)
				}
			})
		}
	}
}

func TestMXLLinkURISegmentsRemainBounded(t *testing.T) {
	t.Parallel()

	for _, href := range []string{
		"/scores//first.musicxml",
		"../scores//first.musicxml",
		"/scores/..//first.musicxml",
		"../scores/..//first.musicxml",
		"/scores//../../../first.musicxml",
		"../scores//../../../first.musicxml",
		"/scores%2Fsub/../../first.musicxml",
		"../scores%2Fsub/../../first.musicxml",
		"/scores//../first.musicxml/",
		"/scores//../first.musicxml/.",
		"/scores//../first.musicxml/child/..",
		"/scores//../../mimetype",
		"/scores//../../META-INF/container.xml",
	} {
		t.Run(href, func(t *testing.T) {
			t.Parallel()
			value := opusPackageWithLink(&OpusScore{Href: href}, []MXLResource{
				{Path: "first.musicxml", Data: []byte(`<score-partwise/>`)},
				{Path: "scores/first.musicxml", Data: []byte(`<score-partwise/>`)},
			})
			value.RootFiles = []MXLRootFile{{FullPath: "collections/main.musicxml"}}
			_, err := value.ResolveOpus()
			assert.ErrorIs(t, err, ErrMXLInvalidPath)
		})
	}
}

func TestMXLLinkURISegmentsDecodeOnce(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		source string
		href   string
		path   string
	}{
		{"collection%2Fname/main.musicxml", "first.musicxml", "collection%2Fname/first.musicxml"},
		{"collection%2E/main.musicxml", "../first.musicxml", "first.musicxml"},
		{"main.musicxml", "first%252Fscore.musicxml", "first%2Fscore.musicxml"},
		{"main.musicxml", "first%252Escore.musicxml", "first%2Escore.musicxml"},
		{"main.musicxml", "scores%2Fsub/./first.musicxml", "scores/sub/first.musicxml"},
		{"main.musicxml", "scores/%2e/first.musicxml", "scores/first.musicxml"},
		{"main.musicxml", "scores/%2e%2e/first.musicxml", "first.musicxml"},
		{"main.musicxml", "%20first%20%20score.musicxml%20", " first  score.musicxml "},
	} {
		t.Run(test.source+"/"+test.href, func(t *testing.T) {
			t.Parallel()
			actual, _, err := resolveMXLLinkPath(test.source, test.href)
			require.NoError(t, err)
			assert.Equal(t, test.path, actual)
		})
	}
}
