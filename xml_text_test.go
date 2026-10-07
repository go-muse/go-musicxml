package musicxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeRejectsUnrepresentableText(t *testing.T) {
	t.Parallel()

	values := []string{"a\x00b", "a\x01b", "a\x0bb", "a\x0cb", "a\x1fb", "a\ufffeb", "a\uffffb", "a\xffb", "a\xed\xa0\x80b", "a\xf4\x90\x80\x80b"}
	for _, value := range values {
		t.Run(fmt.Sprintf("%x", value), func(t *testing.T) {
			t.Parallel()
			for _, test := range []struct {
				name     string
				document Document
				path     string
			}{
				{"element", &ScorePartwise{MovementTitle: &value}, "ScorePartwise.MovementTitle"},
				{"timewise attribute", &ScoreTimewise{Version: &value}, "ScoreTimewise.Version"},
				{"opus attribute", &OpusDocument{Content: OpusDocumentContents{{Score: &OpusScore{Href: value}}}}, "OpusDocument.Content[0].Score.Href"},
				{"named string", &ScorePartwise{Part: []ScorePartwisePart{{Measure: []ScorePartwisePartMeasure{{Text: Ptr(MeasureText(value))}}}}}, "ScorePartwise.Part[0].Measure[0].Text"},
				{"simple content", &ScorePartwise{Credit: []Credit{{Content: CreditContents{{CreditWords: &FormattedTextID{Value: value}}}}}}, "ScorePartwise.Credit[0].Content[0].CreditWords.Value"},
			} {
				t.Run(test.name, func(t *testing.T) {
					var encoded bytes.Buffer
					err := Encode(&encoded, test.document)
					require.Error(t, err)
					assert.Contains(t, err.Error(), test.path)
					assert.Empty(t, encoded.Bytes())
				})
			}
		})
	}
}

func TestValidateUnrepresentableText(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"a\x00b", "a\xffb", "a\ufffeb"} {
		document := decodeValidationScore(t)
		document.MovementTitle = &value
		err := Validate(document)
		require.ErrorIs(t, err, ErrInvalidDocument)
		var validation *ValidationError
		require.ErrorAs(t, err, &validation)
		require.Len(t, validation.Issues, 1)
		assert.Equal(t, "representation", validation.Issues[0].Constraint)
		assert.Contains(t, validation.Issues[0].Message, "ScorePartwise.MovementTitle")
		assert.Equal(t, value, *document.MovementTitle)
	}
}

func TestEncodeRepresentableTextRoundTrip(t *testing.T) {
	t.Parallel()

	// XML 1.0 allows these control and noncharacter code points, as well as
	// literal replacement characters and the maximum Unicode value.
	value := "\t\n\r !<&\"'\u007f\u0085\ud7ff\ue000\ufdd0\ufdef\ufffd\U00010000\U0001fffe\U0001ffff\U0010fffe\U0010ffff"
	document := decodeValidationScore(t)
	document.MovementTitle = &value
	document.Part[0].Measure[0].Text = Ptr(MeasureText(value))
	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, document))
	decoded, err := DecodeScorePartwise(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, value, *decoded.MovementTitle)
	assert.Equal(t, MeasureText(value), *decoded.Part[0].Measure[0].Text)
}

func TestEncodeIgnoresXMLNameTextAndAllowsIncompleteModels(t *testing.T) {
	t.Parallel()

	name := xml.Name{Local: "\x00", Space: "\xff"}
	for _, document := range []Document{
		&ScorePartwise{XMLName: name},
		&ScoreTimewise{XMLName: name},
		&OpusDocument{XMLName: name},
	} {
		var encoded bytes.Buffer
		require.NoError(t, Encode(&encoded, document))
		_, err := Decode(bytes.NewReader(encoded.Bytes()))
		require.NoError(t, err)
	}
}

func TestMXLEncodersRejectUnrepresentableDocumentText(t *testing.T) {
	t.Parallel()

	value := "title\x00"
	document := &ScorePartwise{MovementTitle: &value}
	var encoded bytes.Buffer
	require.Error(t, EncodeMXL(&encoded, document))
	assert.Empty(t, encoded.Bytes())
	require.Error(t, EncodeMXLPackage(&encoded, &MXLPackage{Document: document}))
	assert.Empty(t, encoded.Bytes())
}

func TestMXLRejectsUnrepresentableMetadataBeforeNormalization(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"\x00", "\xff", "\ufffe", "\uffff"} {
		t.Run(fmt.Sprintf("%x", value), func(t *testing.T) {
			t.Parallel()
			for _, primary := range []bool{true, false} {
				for _, mediaType := range []bool{true, false} {
					root := MXLRootFile{FullPath: "score.musicxml"}
					wantErr := ErrMXLInvalidPath
					if mediaType {
						root.MediaType = "application/" + value
						wantErr = ErrMXLInvalidContainer
					} else {
						root.FullPath = "score" + value + ".musicxml"
					}
					value := &MXLPackage{Document: &ScorePartwise{}, RootFiles: []MXLRootFile{root}}
					if !primary {
						value.RootFiles = []MXLRootFile{{FullPath: "primary.musicxml"}, root}
						value.Resources = []MXLResource{{Path: root.FullPath, Data: []byte("alternate")}}
					}
					var encoded bytes.Buffer
					err := EncodeMXLPackage(&encoded, value)
					require.ErrorIs(t, err, wantErr, "primary=%v mediaType=%v", primary, mediaType)
					assert.Empty(t, encoded.Bytes())
				}
			}
		})
	}
}

func TestMXLRepresentableAlternateMediaTypeRoundTrip(t *testing.T) {
	t.Parallel()

	mediaType := "application/\ufffd\ufdd0\U0001fffe\U0010ffff"
	value := &MXLPackage{
		Document:  &ScorePartwise{},
		RootFiles: []MXLRootFile{{FullPath: "score.musicxml"}, {FullPath: "alternate.bin", MediaType: mediaType}},
		Resources: []MXLResource{{Path: "alternate.bin", Data: []byte("alternate")}},
	}
	var encoded bytes.Buffer
	require.NoError(t, EncodeMXLPackage(&encoded, value))
	decoded, err := DecodeMXLPackage(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, value.RootFiles, decoded.RootFiles)
}
