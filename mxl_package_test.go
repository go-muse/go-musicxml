package musicxml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLPackageRoundTrip(t *testing.T) {
	t.Parallel()

	partwise := `<score-partwise version="4.0">` +
		`<part-list></part-list>` +
		`</score-partwise>`
	pdf := []byte{0x25, 0x50, 0x44, 0x46, 0x00, 0xff}
	image := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xfe}
	linkedPart := []byte(
		`<score-partwise><part-list></part-list></score-partwise>`,
	)
	input := makeMXLTestArchive(t, []mxlTestEntry{
		mxlTestMIMETypeEntry(),
		mxlTestDirectoryEntry("META-INF/"),
		mxlTestContainerEntry(
			`<rootfile full-path="scores/main.musicxml" ` +
				`media-type="` + musicXMLMIMEType + `"/>` +
				`<rootfile full-path="alternate/score.pdf" ` +
				`media-type="application/pdf"/>`,
		),
		mxlTestFileEntry("scores/main.musicxml", partwise),
		{
			name:    "alternate/score.pdf",
			content: pdf,
			method:  zip.Store,
		},
		{
			name:    "images/cover.png",
			content: image,
			method:  zip.Deflate,
		},
		{
			name:    "parts/violin.musicxml",
			content: linkedPart,
			method:  zip.Deflate,
		},
		mxlTestFileEntry("metadata/source.txt", ""),
	})

	actual, err := DecodeMXLPackage(bytes.NewReader(input))
	require.NoError(t, err)

	want := &MXLPackage{
		Document: &ScorePartwise{
			XMLName: xml.Name{Local: "score-partwise"},
			Version: stringPointer(Version),
		},
		RootFiles: []MXLRootFile{
			{
				FullPath:  "scores/main.musicxml",
				MediaType: musicXMLMIMEType,
			},
			{
				FullPath:  "alternate/score.pdf",
				MediaType: "application/pdf",
			},
		},
		Resources: []MXLResource{
			{
				Path: "alternate/score.pdf",
				Data: pdf,
			},
			{
				Path: "images/cover.png",
				Data: image,
			},
			{
				Path: "parts/violin.musicxml",
				Data: linkedPart,
			},
			{
				Path: "metadata/source.txt",
				Data: []byte{},
			},
		},
	}
	assert.Equal(t, want, actual)

	var encoded bytes.Buffer
	err = EncodeMXLPackage(&encoded, actual)
	require.NoError(t, err)

	archive := openMXLTestArchive(t, encoded.Bytes())
	require.Len(t, archive.File, 7)
	assert.Equal(t, mxlMIMETypePath, archive.File[0].Name)
	assert.Equal(t, mxlContainerPath, archive.File[1].Name)
	assert.Equal(t, "scores/main.musicxml", archive.File[2].Name)
	assert.Equal(t, "alternate/score.pdf", archive.File[3].Name)
	assert.Equal(t, pdf, readMXLTestFile(t, archive.File[3]))
	assert.Equal(t, image, readMXLTestFile(t, archive.File[4]))
	assert.Equal(t, linkedPart, readMXLTestFile(t, archive.File[5]))

	roundTripped, err := DecodeMXLPackage(
		bytes.NewReader(encoded.Bytes()),
	)
	require.NoError(t, err)
	assert.Equal(t, actual, roundTripped)
}

func TestEncodeMXLPackageUsesDefaultRootFile(t *testing.T) {
	t.Parallel()

	version := Version
	value := &MXLPackage{
		Document: &ScoreTimewise{
			XMLName: xml.Name{Local: "score-timewise"},
			Version: &version,
		},
		Resources: []MXLResource{
			{
				Path: "images/cover.png",
				Data: []byte("PNG"),
			},
		},
	}

	var encoded bytes.Buffer
	err := EncodeMXLPackage(&encoded, value)
	require.NoError(t, err)

	actual, err := DecodeMXLPackage(
		bytes.NewReader(encoded.Bytes()),
	)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]MXLRootFile{
			{
				FullPath:  mxlRootFilePath,
				MediaType: musicXMLMIMEType,
			},
		},
		actual.RootFiles,
	)
	assert.Equal(t, value.Document, actual.Document)
	assert.Equal(t, value.Resources, actual.Resources)
	assert.Empty(t, value.RootFiles)
}

func TestEncodeMXLPackageErrors(t *testing.T) {
	t.Parallel()

	document := &ScorePartwise{}
	tests := []struct {
		name    string
		writer  io.Writer
		value   *MXLPackage
		wantErr error
	}{
		{
			name:    "nil writer",
			value:   &MXLPackage{Document: document},
			wantErr: ErrNilWriter,
		},
		{
			name:    "nil package",
			writer:  &bytes.Buffer{},
			wantErr: ErrNilMXLPackage,
		},
		{
			name:    "nil document",
			writer:  &bytes.Buffer{},
			value:   &MXLPackage{},
			wantErr: ErrNilDocument,
		},
		{
			name:   "unsafe primary root path",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				RootFiles: []MXLRootFile{
					{FullPath: "../score.musicxml"},
				},
			},
			wantErr: ErrMXLInvalidPath,
		},
		{
			name:   "unsupported primary media type",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				RootFiles: []MXLRootFile{
					{
						FullPath:  "score.musicxml",
						MediaType: "application/pdf",
					},
				},
			},
			wantErr: ErrMXLUnsupportedMediaType,
		},
		{
			name:   "duplicate root file",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				RootFiles: []MXLRootFile{
					{FullPath: "score.musicxml"},
					{FullPath: "score.musicxml"},
				},
			},
			wantErr: ErrMXLDuplicateEntry,
		},
		{
			name:   "missing alternate root file",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				RootFiles: []MXLRootFile{
					{FullPath: "score.musicxml"},
					{
						FullPath:  "alternate.pdf",
						MediaType: "application/pdf",
					},
				},
			},
			wantErr: ErrMXLRootFileNotFound,
		},
		{
			name:   "unsafe resource path",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				Resources: []MXLResource{
					{Path: "../cover.png"},
				},
			},
			wantErr: ErrMXLInvalidPath,
		},
		{
			name:   "reserved resource path",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				Resources: []MXLResource{
					{Path: mxlContainerPath},
				},
			},
			wantErr: ErrMXLInvalidPath,
		},
		{
			name:   "resource replaces primary root",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				Resources: []MXLResource{
					{Path: mxlRootFilePath},
				},
			},
			wantErr: ErrMXLDuplicateEntry,
		},
		{
			name:   "duplicate resource",
			writer: &bytes.Buffer{},
			value: &MXLPackage{
				Document: document,
				Resources: []MXLResource{
					{Path: "cover.png"},
					{Path: "cover.png"},
				},
			},
			wantErr: ErrMXLDuplicateEntry,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := EncodeMXLPackage(test.writer, test.value)

			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestDecodeMXLPackageRootFileErrors(t *testing.T) {
	t.Parallel()

	partwise := `<score-partwise>` +
		`<part-list></part-list>` +
		`</score-partwise>`
	tests := []struct {
		name      string
		rootFiles string
		wantErr   error
	}{
		{
			name: "unsafe alternate root file",
			rootFiles: `<rootfile full-path="score.musicxml"/>` +
				`<rootfile full-path="../alternate.pdf" ` +
				`media-type="application/pdf"/>`,
			wantErr: ErrMXLInvalidPath,
		},
		{
			name: "missing alternate root file",
			rootFiles: `<rootfile full-path="score.musicxml"/>` +
				`<rootfile full-path="alternate.pdf" ` +
				`media-type="application/pdf"/>`,
			wantErr: ErrMXLRootFileNotFound,
		},
		{
			name: "duplicate root file",
			rootFiles: `<rootfile full-path="score.musicxml"/>` +
				`<rootfile full-path="score.musicxml"/>`,
			wantErr: ErrMXLDuplicateEntry,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := makeMXLTestArchive(t, []mxlTestEntry{
				mxlTestContainerEntry(test.rootFiles),
				mxlTestFileEntry("score.musicxml", partwise),
			})

			actual, err := DecodeMXLPackage(
				bytes.NewReader(input),
			)

			assert.ErrorIs(t, err, test.wantErr)
			assert.Nil(t, actual)
		})
	}
}

func TestDecodeMXLPackageResourceLimits(t *testing.T) {
	t.Parallel()

	archive := makeMXLTestArchive(t, []mxlTestEntry{
		mxlTestContainerEntry(
			`<rootfile full-path="score.musicxml"/>`,
		),
		mxlTestFileEntry(
			"score.musicxml",
			`<score-partwise><part-list></part-list>`+
				`</score-partwise>`,
		),
		mxlTestFileEntry("first.bin", "1234"),
		mxlTestFileEntry("second.bin", "5678"),
	})
	tests := []struct {
		name   string
		limits mxlLimits
	}{
		{
			name: "individual resource",
			limits: mxlLimits{
				archiveSize:   maxMXLArchiveSize,
				metadataSize:  maxMXLMetadataSize,
				documentSize:  maxMXLDocumentSize,
				resourceSize:  3,
				resourcesSize: 8,
			},
		},
		{
			name: "all resources",
			limits: mxlLimits{
				archiveSize:   maxMXLArchiveSize,
				metadataSize:  maxMXLMetadataSize,
				documentSize:  maxMXLDocumentSize,
				resourceSize:  4,
				resourcesSize: 7,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := decodeMXLPackage(
				bytes.NewReader(archive),
				test.limits,
			)

			assert.ErrorIs(t, err, ErrMXLTooLarge)
			assert.Nil(t, actual)
		})
	}
}

func TestDecodeMXLPackageNilReader(t *testing.T) {
	t.Parallel()

	actual, err := DecodeMXLPackage(nil)

	assert.ErrorIs(t, err, ErrNilReader)
	assert.Nil(t, actual)
}
