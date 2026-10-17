package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-muse/go-musicxml/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cancelingReader returns at most chunk bytes per Read and cancels its
// context after the first Read, so the next context check must stop decoding.
type cancelingReader struct {
	data   []byte
	chunk  int
	reads  int
	cancel context.CancelFunc
}

func (r *cancelingReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	r.reads++
	if r.reads == 1 {
		defer r.cancel()
	}
	n := min(len(p), r.chunk, len(r.data))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func contextTestScore(t *testing.T) []byte {
	t.Helper()

	score := NewScorePartwise(PartDefinition{ID: "P1", Name: "Music"})
	measure := NewScorePartwiseMeasure("1")
	for range 200 {
		measure.AddNote(NewRestNote(1))
	}
	score.Part[0].AddMeasure(measure)

	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, score))
	return encoded.Bytes()
}

func contextTestArchive(t *testing.T, data []byte) []byte {
	t.Helper()

	document, err := Decode(bytes.NewReader(data))
	require.NoError(t, err)
	var archive bytes.Buffer
	require.NoError(t, EncodeMXL(&archive, document))
	return archive.Bytes()
}

func TestDecodeContextMatchesDecodeWithOptions(t *testing.T) {
	t.Parallel()

	data := contextTestScore(t)
	expected, err := DecodeWithOptions(bytes.NewReader(data), DecodeOptions{})
	require.NoError(t, err)

	actual, err := DecodeContext(
		context.Background(),
		bytes.NewReader(data),
		DecodeOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)

	_, err = DecodeContext(
		context.Background(),
		bytes.NewReader(data),
		DecodeOptions{MaxXMLDepth: 2},
	)
	assert.ErrorIs(t, err, ErrXMLTooDeep)

	_, err = DecodeContext(
		context.Background(),
		bytes.NewReader(data),
		DecodeOptions{MaxXMLDepth: -1},
	)
	assert.ErrorIs(t, err, ErrInvalidDecodeOptions)

	_, err = DecodeContext(context.Background(), nil, DecodeOptions{})
	assert.ErrorIs(t, err, ErrNilReader)
}

func TestDecodeContextCancellation(t *testing.T) {
	t.Parallel()

	data := contextTestScore(t)

	t.Run("already canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		document, err := DecodeContext(ctx, bytes.NewReader(data), DecodeOptions{})
		assert.Nil(t, document)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("canceled while reading", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		reader := &cancelingReader{data: data, chunk: 64, cancel: cancel}
		document, err := DecodeContext(ctx, reader, DecodeOptions{})
		assert.Nil(t, document)
		assert.ErrorIs(t, err, context.Canceled)
		assert.NotEmpty(t, reader.data, "decoding must stop before EOF")
	})

	t.Run("nil context", func(t *testing.T) {
		t.Parallel()

		var ctx context.Context
		document, err := DecodeContext(ctx, bytes.NewReader(data), DecodeOptions{})
		assert.Nil(t, document)
		var configuration *validation.ConfigurationError
		assert.ErrorAs(t, err, &configuration)
		assert.ErrorIs(t, err, validation.ErrInvalidConfiguration)
	})
}

func TestDecodeMXLContext(t *testing.T) {
	t.Parallel()

	archive := contextTestArchive(t, contextTestScore(t))
	expected, err := DecodeMXL(bytes.NewReader(archive))
	require.NoError(t, err)

	actual, err := DecodeMXLContext(
		context.Background(),
		bytes.NewReader(archive),
		MXLOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)

	expectedPackage, err := DecodeMXLPackage(bytes.NewReader(archive))
	require.NoError(t, err)
	actualPackage, err := DecodeMXLPackageContext(
		context.Background(),
		bytes.NewReader(archive),
		MXLOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, expectedPackage, actualPackage)

	_, err = DecodeMXLContext(
		context.Background(),
		bytes.NewReader(archive),
		MXLOptions{MaxArchiveBytes: -1},
	)
	assert.ErrorIs(t, err, ErrInvalidMXLOptions)
	_, err = DecodeMXLPackageContext(
		context.Background(),
		bytes.NewReader(archive),
		MXLOptions{MaxXMLDepth: 5000},
	)
	assert.ErrorIs(t, err, ErrInvalidMXLOptions)
	_, err = DecodeMXLContext(context.Background(), nil, MXLOptions{})
	assert.ErrorIs(t, err, ErrNilReader)
}

func TestDecodeMXLContextCancellation(t *testing.T) {
	t.Parallel()

	archive := contextTestArchive(t, contextTestScore(t))
	decoders := map[string]func(context.Context, io.Reader) (any, error){
		"DecodeMXLContext": func(ctx context.Context, reader io.Reader) (any, error) {
			return DecodeMXLContext(ctx, reader, MXLOptions{})
		},
		"DecodeMXLPackageContext": func(ctx context.Context, reader io.Reader) (any, error) {
			return DecodeMXLPackageContext(ctx, reader, MXLOptions{})
		},
	}

	for name, decode := range decoders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err := decode(ctx, bytes.NewReader(archive))
			assert.ErrorIs(t, err, context.Canceled)
			assert.NotErrorIs(t, err, ErrInvalidMXL)

			ctx, cancel = context.WithCancel(context.Background())
			defer cancel()
			reader := &cancelingReader{data: archive, chunk: 64, cancel: cancel}
			_, err = decode(ctx, reader)
			assert.ErrorIs(t, err, context.Canceled)
			assert.NotErrorIs(t, err, ErrInvalidMXL)
			assert.NotEmpty(t, reader.data, "archive reading must stop before EOF")

			var nilContext context.Context
			_, err = decode(nilContext, bytes.NewReader(archive))
			assert.ErrorIs(t, err, validation.ErrInvalidConfiguration)
		})
	}
}

func TestDecodeUnsupportedXMLFeature(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"internal DTD entity": `<!DOCTYPE score-partwise [<!ENTITY title "Title">]>` +
			`<score-partwise><work><work-title>&title;</work-title></work>` +
			`<part-list/></score-partwise>`,
		"unavailable encoding": `<?xml version="1.0" encoding="Shift_JIS"?>` +
			`<score-partwise><part-list/></score-partwise>`,
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			document, err := Decode(strings.NewReader(input))
			assert.Nil(t, document)
			assert.ErrorIs(t, err, ErrUnsupportedXMLFeature)
			assert.ErrorIs(t, err, ErrIncompleteAssessment)
		})
	}

	_, err := Decode(strings.NewReader(`<score-partwise><part-list/>`))
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnsupportedXMLFeature)
}

func TestErrorTypesTolerateNilReceivers(t *testing.T) {
	t.Parallel()

	var root *UnsupportedRootError
	assert.NotPanics(t, func() { _ = root.Error() })
	assert.NoError(t, root.Unwrap())

	var link *MXLLinkError
	assert.NotPanics(t, func() { _ = link.Error() })
	assert.NoError(t, link.Unwrap())

	assert.True(t, errors.Is(&UnsupportedRootError{}, ErrUnsupportedRoot))
}

// mxlTestCancelMethod is a private ZIP method used only by the final-entry
// cancellation test. Its decompressor cancels the registered context on Read.
const mxlTestCancelMethod uint16 = 0xC0DE

var (
	registerMXLTestCancelMethod sync.Once
	mxlTestCancelOnRead         atomic.Pointer[context.CancelFunc]
)

type mxlTestWriteCloser struct{ io.Writer }

func (mxlTestWriteCloser) Close() error { return nil }

type mxlTestCancelingDecompressor struct{ reader io.Reader }

func (d mxlTestCancelingDecompressor) Read(p []byte) (int, error) {
	if cancel := mxlTestCancelOnRead.Load(); cancel != nil {
		(*cancel)()
	}
	return d.reader.Read(p)
}

func (mxlTestCancelingDecompressor) Close() error { return nil }

// finalEntryCancelingArchive returns a valid package whose last archive entry
// uses mxlTestCancelMethod.
func finalEntryCancelingArchive(t *testing.T) []byte {
	t.Helper()

	registerMXLTestCancelMethod.Do(func() {
		zip.RegisterCompressor(mxlTestCancelMethod, func(w io.Writer) (io.WriteCloser, error) {
			return mxlTestWriteCloser{w}, nil
		})
		zip.RegisterDecompressor(mxlTestCancelMethod, func(r io.Reader) io.ReadCloser {
			return mxlTestCancelingDecompressor{reader: r}
		})
	})

	document, err := Decode(bytes.NewReader(contextTestScore(t)))
	require.NoError(t, err)
	var original bytes.Buffer
	require.NoError(t, EncodeMXLPackage(&original, &MXLPackage{
		Document:  document,
		Resources: []MXLResource{{Path: "media/last.bin", Data: []byte("final resource")}},
	}))

	source, err := zip.NewReader(bytes.NewReader(original.Bytes()), int64(original.Len()))
	require.NoError(t, err)
	var rewritten bytes.Buffer
	writer := zip.NewWriter(&rewritten)
	for index, file := range source.File {
		if index < len(source.File)-1 {
			require.NoError(t, writer.Copy(file))
			continue
		}
		require.Equal(t, "media/last.bin", file.Name)
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: file.Name, Method: mxlTestCancelMethod})
		require.NoError(t, err)
		_, err = entry.Write([]byte("final resource"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return rewritten.Bytes()
}

// TestDecodeMXLPackageContextCancellationDuringFinalEntry is not parallel: it
// installs the process-wide cancel hook of mxlTestCancelMethod.
func TestDecodeMXLPackageContextCancellationDuringFinalEntry(t *testing.T) {
	archive := finalEntryCancelingArchive(t)

	value, err := DecodeMXLPackageContext(context.Background(), bytes.NewReader(archive), MXLOptions{})
	require.NoError(t, err)
	require.Len(t, value.Resources, 1)
	assert.Equal(t, "final resource", string(value.Resources[0].Data))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mxlTestCancelOnRead.Store(&cancel)
	defer mxlTestCancelOnRead.Store(nil)

	value, err = DecodeMXLPackageContext(ctx, bytes.NewReader(archive), MXLOptions{})
	assert.Nil(t, value)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, ErrInvalidMXL)
}

// blockingReader blocks its first Read until release is closed.
type blockingReader struct {
	data    []byte
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *blockingReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.entered)
		<-r.release
	})
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestDecodeContextDoesNotInterruptBlockedRead(t *testing.T) {
	t.Parallel()

	reader := &blockingReader{
		data:    contextTestScore(t),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := DecodeContext(ctx, reader, DecodeOptions{})
		done <- err
	}()

	<-reader.entered
	cancel()
	select {
	case err := <-done:
		t.Fatalf("DecodeContext returned while Read was blocked: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(reader.release)
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("DecodeContext did not return after the blocked Read was released")
	}
}
