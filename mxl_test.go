package musicxml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeMXL(t *testing.T) {
	t.Parallel()

	version := Version
	title := "Fidelio"
	tests := []struct {
		name     string
		document Document
	}{
		{
			name: "opus",
			document: &OpusDocument{
				XMLName: xml.Name{Local: "opus"},
				Title:   &title,
				Version: &version,
			},
		},
		{
			name: "score partwise",
			document: &ScorePartwise{
				XMLName: xml.Name{Local: "score-partwise"},
				Version: &version,
			},
		},
		{
			name: "score timewise",
			document: &ScoreTimewise{
				XMLName: xml.Name{Local: "score-timewise"},
				Version: &version,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var encoded bytes.Buffer
			err := EncodeMXL(&encoded, test.document)
			require.NoError(t, err)

			archive := openMXLTestArchive(t, encoded.Bytes())
			require.Len(t, archive.File, 3)
			assert.Equal(t, mxlMIMETypePath, archive.File[0].Name)
			assert.Equal(t, mxlContainerPath, archive.File[1].Name)
			assert.Equal(t, mxlRootFilePath, archive.File[2].Name)

			mimetype := archive.File[0]
			assert.Equal(t, uint16(zip.Store), mimetype.Method)
			assert.Empty(t, mimetype.Extra)
			assert.Equal(
				t,
				mxlMIMEType,
				string(readMXLTestFile(t, mimetype)),
			)

			container, err := decodeMXLContainer(
				readMXLTestFile(t, archive.File[1]),
			)
			require.NoError(t, err)
			require.Len(t, container.RootFiles.Files, 1)
			assert.Equal(
				t,
				mxlRootFile{
					FullPath:  mxlRootFilePath,
					MediaType: musicXMLMIMEType,
				},
				container.RootFiles.Files[0],
			)

			root, err := Decode(bytes.NewReader(
				readMXLTestFile(t, archive.File[2]),
			))
			require.NoError(t, err)
			assert.Equal(t, test.document, root)

			decoded, err := DecodeMXL(
				bytes.NewReader(encoded.Bytes()),
			)
			require.NoError(t, err)
			assert.Equal(t, test.document, decoded)
		})
	}
}

func TestEncodeMXLErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		writer   io.Writer
		document Document
		wantErr  error
	}{
		{
			name:     "nil writer",
			document: &ScorePartwise{},
			wantErr:  ErrNilWriter,
		},
		{
			name:    "nil document",
			writer:  &bytes.Buffer{},
			wantErr: ErrNilDocument,
		},
		{
			name:     "typed nil document",
			writer:   &bytes.Buffer{},
			document: (*ScorePartwise)(nil),
			wantErr:  ErrNilDocument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := EncodeMXL(test.writer, test.document)

			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestDecodeMXL(t *testing.T) {
	t.Parallel()

	partwise := `<score-partwise version="4.0">` +
		`<part-list></part-list>` +
		`</score-partwise>`
	timewise := `<score-timewise version="4.0">` +
		`<part-list></part-list>` +
		`</score-timewise>`
	opus := `<opus version="4.0"><title>Fidelio</title></opus>`
	tests := []struct {
		name    string
		input   []byte
		entries []mxlTestEntry
		want    Document
		wantErr error
	}{
		{
			name: "opus",
			entries: []mxlTestEntry{
				mxlTestMIMETypeEntry(),
				mxlTestContainerEntry(
					`<rootfile full-path="opus.musicxml" ` +
						`media-type="` + musicXMLMIMEType + `"/>`,
				),
				mxlTestFileEntry("opus.musicxml", opus),
			},
			want: &OpusDocument{
				XMLName: xml.Name{Local: "opus"},
				Title:   stringPointer("Fidelio"),
				Version: stringPointer(Version),
			},
		},
		{
			name: "score partwise",
			entries: []mxlTestEntry{
				mxlTestMIMETypeEntry(),
				mxlTestDirectoryEntry("META-INF/"),
				mxlTestContainerEntry(
					`<rootfile full-path="scores/main.musicxml" ` +
						`media-type="` + musicXMLMIMEType + `"/>` +
						`<rootfile full-path="score.pdf" ` +
						`media-type="application/pdf"/>`,
				),
				mxlTestFileEntry(
					"scores/main.musicxml",
					partwise,
				),
				mxlTestFileEntry("score.pdf", "PDF"),
			},
			want: &ScorePartwise{
				XMLName: xml.Name{Local: "score-partwise"},
				Version: stringPointer(Version),
			},
		},
		{
			name: "score timewise without mimetype",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(
					`<rootfile full-path="score.xml"/>`,
				),
				mxlTestFileEntry("score.xml", timewise),
			},
			want: &ScoreTimewise{
				XMLName: xml.Name{Local: "score-timewise"},
				Version: stringPointer(Version),
			},
		},
		{
			name:    "nil reader",
			wantErr: ErrNilReader,
		},
		{
			name:    "invalid ZIP",
			input:   []byte("not a ZIP archive"),
			wantErr: ErrInvalidMXL,
		},
		{
			name: "missing container",
			entries: []mxlTestEntry{
				mxlTestFileEntry("score.musicxml", partwise),
			},
			wantErr: ErrMXLContainerNotFound,
		},
		{
			name: "invalid container",
			entries: []mxlTestEntry{
				mxlTestFileEntry(
					mxlContainerPath,
					`<package></package>`,
				),
			},
			wantErr: ErrMXLInvalidContainer,
		},
		{
			name: "missing root file declaration",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(""),
			},
			wantErr: ErrMXLRootFileNotFound,
		},
		{
			name: "unsupported first root file media type",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(
					`<rootfile full-path="score.pdf" ` +
						`media-type="application/pdf"/>` +
						`<rootfile full-path="score.musicxml" ` +
						`media-type="` + musicXMLMIMEType + `"/>`,
				),
				mxlTestFileEntry("score.pdf", "PDF"),
				mxlTestFileEntry(
					"score.musicxml",
					partwise,
				),
			},
			wantErr: ErrMXLUnsupportedMediaType,
		},
		{
			name: "unsafe root file path",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(
					`<rootfile full-path="../score.musicxml"/>`,
				),
				mxlTestFileEntry(
					"score.musicxml",
					partwise,
				),
			},
			wantErr: ErrMXLInvalidPath,
		},
		{
			name: "missing root file entry",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(
					`<rootfile full-path="missing.musicxml"/>`,
				),
			},
			wantErr: ErrMXLRootFileNotFound,
		},
		{
			name: "invalid mimetype value",
			entries: []mxlTestEntry{
				mxlTestFileEntry(
					mxlMIMETypePath,
					" application/vnd.recordare.musicxml",
				),
				mxlTestContainerEntry(
					`<rootfile full-path="score.musicxml"/>`,
				),
				mxlTestFileEntry(
					"score.musicxml",
					partwise,
				),
			},
			wantErr: ErrMXLInvalidMIMEType,
		},
		{
			name: "compressed mimetype",
			entries: []mxlTestEntry{
				{
					name:    mxlMIMETypePath,
					content: []byte(mxlMIMEType),
					method:  zip.Deflate,
				},
				mxlTestContainerEntry(
					`<rootfile full-path="score.musicxml"/>`,
				),
				mxlTestFileEntry(
					"score.musicxml",
					partwise,
				),
			},
			wantErr: ErrMXLInvalidMIMEType,
		},
		{
			name: "mimetype with extra field",
			entries: []mxlTestEntry{
				{
					name:    mxlMIMETypePath,
					content: []byte(mxlMIMEType),
					method:  zip.Store,
					extra:   []byte{0x55, 0x54, 0x00, 0x00},
				},
				mxlTestContainerEntry(
					`<rootfile full-path="score.musicxml"/>`,
				),
				mxlTestFileEntry(
					"score.musicxml",
					partwise,
				),
			},
			wantErr: ErrMXLInvalidMIMEType,
		},
		{
			name: "duplicate archive entry",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(
					`<rootfile full-path="score.musicxml"/>`,
				),
				mxlTestContainerEntry(
					`<rootfile full-path="other.musicxml"/>`,
				),
			},
			wantErr: ErrMXLDuplicateEntry,
		},
		{
			name: "unsafe archive entry",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(
					`<rootfile full-path="score.musicxml"/>`,
				),
				mxlTestFileEntry(
					"score.musicxml",
					partwise,
				),
				mxlTestFileEntry("../cover.png", "PNG"),
			},
			wantErr: ErrMXLInvalidPath,
		},
		{
			name: "unsupported root document",
			entries: []mxlTestEntry{
				mxlTestContainerEntry(
					`<rootfile full-path="score.musicxml"/>`,
				),
				mxlTestFileEntry(
					"score.musicxml",
					`<sounds></sounds>`,
				),
			},
			wantErr: ErrUnsupportedRoot,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := test.input
			if test.entries != nil {
				input = makeMXLTestArchive(t, test.entries)
			}

			var reader io.Reader
			if input != nil {
				reader = bytes.NewReader(input)
			}

			actual, err := DecodeMXL(reader)

			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
				assert.Nil(t, actual)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestDecodeMXLLimits(t *testing.T) {
	t.Parallel()

	container := mxlTestContainerEntry(
		`<rootfile full-path="score.musicxml"/>`,
	)
	root := mxlTestFileEntry(
		"score.musicxml",
		`<score-partwise><part-list></part-list>`+
			`</score-partwise>`,
	)
	archive := makeMXLTestArchive(
		t,
		[]mxlTestEntry{container, root},
	)
	tests := []struct {
		name   string
		limits mxlLimits
	}{
		{
			name: "archive",
			limits: mxlLimits{
				archiveSize:  int64(len(archive) - 1),
				metadataSize: maxMXLMetadataSize,
				documentSize: maxMXLDocumentSize,
			},
		},
		{
			name: "container",
			limits: mxlLimits{
				archiveSize:  maxMXLArchiveSize,
				metadataSize: int64(len(container.content) - 1),
				documentSize: maxMXLDocumentSize,
			},
		},
		{
			name: "document",
			limits: mxlLimits{
				archiveSize:  maxMXLArchiveSize,
				metadataSize: maxMXLMetadataSize,
				documentSize: int64(len(root.content) - 1),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document, err := decodeMXL(
				bytes.NewReader(archive),
				test.limits,
			)

			assert.ErrorIs(t, err, ErrMXLTooLarge)
			assert.Nil(t, document)
		})
	}
}

func TestDecodeMXLXMLDepthLimit(t *testing.T) {
	t.Parallel()

	archive := makeMXLTestArchive(t, []mxlTestEntry{
		mxlTestContainerEntry(
			`<rootfile full-path="score.musicxml"/>`,
		),
		mxlTestFileEntry(
			"score.musicxml",
			nestedOpusXML(defaultMaxXMLDepth+1),
		),
	})

	document, err := DecodeMXL(bytes.NewReader(archive))
	assert.ErrorIs(t, err, ErrXMLTooDeep)
	assert.Nil(t, document)

	value, err := DecodeMXLPackage(bytes.NewReader(archive))
	assert.ErrorIs(t, err, ErrXMLTooDeep)
	assert.Nil(t, value)
}

func TestDecodeMXLWithOptions(t *testing.T) {
	t.Parallel()

	archive := makeMXLTestArchive(t, []mxlTestEntry{
		mxlTestContainerEntry(
			`<rootfile full-path="score.musicxml"/>`,
		),
		mxlTestFileEntry(
			"score.musicxml",
			nestedOpusXML(3),
		),
	})

	document, err := DecodeMXLWithOptions(
		bytes.NewReader(archive),
		MXLOptions{MaxXMLDepth: 2},
	)
	assert.ErrorIs(t, err, ErrXMLTooDeep)
	assert.Nil(t, document)

	document, err = DecodeMXLWithOptions(
		bytes.NewReader(archive),
		MXLOptions{MaxArchiveBytes: int64(len(archive) - 1)},
	)
	assert.ErrorIs(t, err, ErrMXLTooLarge)
	assert.Nil(t, document)

	document, err = DecodeMXLWithOptions(
		bytes.NewReader(archive),
		MXLOptions{MaxDocumentBytes: -1},
	)
	assert.ErrorIs(t, err, ErrInvalidMXLOptions)
	assert.Nil(t, document)
}

func TestValidMXLPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		directory bool
		want      bool
	}{
		{
			name:  "root file",
			value: "score.musicxml",
			want:  true,
		},
		{
			name:  "nested file",
			value: "scores/main.musicxml",
			want:  true,
		},
		{
			name:      "directory",
			value:     "META-INF/",
			directory: true,
			want:      true,
		},
		{
			name:  "empty",
			value: "",
		},
		{
			name:  "absolute",
			value: "/score.musicxml",
		},
		{
			name:  "parent",
			value: "../score.musicxml",
		},
		{
			name:  "embedded parent",
			value: "scores/../score.musicxml",
		},
		{
			name:  "backslash",
			value: `scores\score.musicxml`,
		},
		{
			name:  "trailing slash on file",
			value: "scores/",
		},
		{
			name:  "root directory",
			value: ".",
		},
		{
			name:  "invalid UTF-8",
			value: string([]byte{0xff}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := validMXLPath(test.value, test.directory)

			assert.Equal(t, test.want, actual)
		})
	}
}

func TestDecodeMXLContainerRejectsTail(t *testing.T) {
	t.Parallel()

	_, err := decodeMXLContainer([]byte(
		`<container><rootfiles></rootfiles></container>` +
			strings.Repeat(" ", 2) +
			`<second></second>`,
	))

	assert.ErrorIs(t, err, ErrMXLInvalidContainer)
}

type mxlTestEntry struct {
	name      string
	content   []byte
	method    uint16
	extra     []byte
	directory bool
}

func mxlTestMIMETypeEntry() mxlTestEntry {
	return mxlTestFileEntry(mxlMIMETypePath, mxlMIMEType)
}

func mxlTestContainerEntry(
	rootFiles string,
) mxlTestEntry {
	return mxlTestFileEntry(
		mxlContainerPath,
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<container><rootfiles>`+
			rootFiles+
			`</rootfiles></container>`,
	)
}

func mxlTestFileEntry(
	name string,
	content string,
) mxlTestEntry {
	return mxlTestEntry{
		name:    name,
		content: []byte(content),
		method:  zip.Store,
	}
}

func mxlTestDirectoryEntry(name string) mxlTestEntry {
	return mxlTestEntry{
		name:      name,
		method:    zip.Store,
		directory: true,
	}
}

func makeMXLTestArchive(
	t *testing.T,
	entries []mxlTestEntry,
) []byte {
	t.Helper()

	var result bytes.Buffer
	archive := zip.NewWriter(&result)

	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:   entry.name,
			Method: entry.method,
			Extra:  entry.extra,
		}
		if entry.directory {
			header.SetMode(fs.ModeDir | 0o755)
		}

		writer, err := archive.CreateHeader(header)
		require.NoError(t, err)

		_, err = writer.Write(entry.content)
		require.NoError(t, err)
	}

	require.NoError(t, archive.Close())

	return result.Bytes()
}

func openMXLTestArchive(
	t *testing.T,
	data []byte,
) *zip.Reader {
	t.Helper()

	result, err := zip.NewReader(
		bytes.NewReader(data),
		int64(len(data)),
	)
	require.NoError(t, err)

	return result
}

func readMXLTestFile(
	t *testing.T,
	file *zip.File,
) []byte {
	t.Helper()

	reader, err := file.Open()
	require.NoError(t, err)

	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return result
}

func stringPointer(value string) *string {
	return &value
}
