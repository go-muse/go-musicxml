package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

const (
	mxlMIMEType      = "application/vnd.recordare.musicxml"
	musicXMLMIMEType = "application/vnd.recordare.musicxml+xml"

	mxlMIMETypePath  = "mimetype"
	mxlContainerPath = "META-INF/container.xml"
	mxlRootFilePath  = "score.musicxml"

	maxMXLArchiveSize   int64 = 64 << 20
	maxMXLMetadataSize  int64 = 1 << 20
	maxMXLDocumentSize  int64 = 256 << 20
	maxMXLResourceSize  int64 = 256 << 20
	maxMXLResourcesSize int64 = 512 << 20
)

// DecodeMXL is scenario 1 for MXL archives: loading without MusicXML
// validation.
//
// DecodeMXL reads a compressed MusicXML document.
//
// Archives without the optional legacy-compatible mimetype entry are accepted.
// Archive paths and the sizes of the archive, container, and root document are
// validated before the root document is decoded.
func DecodeMXL(reader io.Reader) (Document, error) {
	return decodeMXL(reader, defaultMXLLimits())
}

// DecodeMXLWithOptions reads a compressed MusicXML document using explicit
// resource limits. Zero option fields use the package defaults.
func DecodeMXLWithOptions(
	reader io.Reader,
	options MXLOptions,
) (Document, error) {
	limits, err := options.limits()
	if err != nil {
		return nil, err
	}

	return decodeMXL(reader, limits)
}

// DecodeMXLContext reads a compressed MusicXML document like
// DecodeMXLWithOptions and stops when ctx is canceled or its deadline passes.
//
// Cancellation is cooperative, as for DecodeContext: ctx is checked before
// each read of the archive, while each entry is expanded, while the root
// document is decoded and before a result is returned. The returned error then
// matches ctx.Err() with errors.Is, not ErrInvalidMXL. A Read call that blocks
// cannot be interrupted; give a blocking reader its own deadline or close it.
// A nil ctx is rejected with a *validation.ConfigurationError. Use
// AsScorePartwise, AsScoreTimewise or AsOpusDocument to narrow the result.
func DecodeMXLContext(
	ctx context.Context,
	reader io.Reader,
	options MXLOptions,
) (Document, error) {
	limits, err := options.limits()
	if err != nil {
		return nil, err
	}

	return decodeMXLContext(ctx, reader, limits)
}

// DecodeMXLScorePartwise reads a compressed score-partwise document.
//
// It returns ErrUnsupportedRoot when the archive contains another root type.
func DecodeMXLScorePartwise(reader io.Reader) (*ScorePartwise, error) {
	return DecodeMXLScorePartwiseWithOptions(reader, MXLOptions{})
}

// DecodeMXLScorePartwiseWithOptions reads a compressed score-partwise
// document using explicit resource limits.
func DecodeMXLScorePartwiseWithOptions(
	reader io.Reader,
	options MXLOptions,
) (*ScorePartwise, error) {
	document, err := DecodeMXLWithOptions(reader, options)
	if err != nil {
		return nil, err
	}
	value, ok := AsScorePartwise(document)
	if !ok {
		return nil, &UnsupportedRootError{Name: documentRootName(document)}
	}

	return value, nil
}

// DecodeMXLScoreTimewise reads a compressed score-timewise document.
//
// It returns ErrUnsupportedRoot when the archive contains another root type.
func DecodeMXLScoreTimewise(reader io.Reader) (*ScoreTimewise, error) {
	return DecodeMXLScoreTimewiseWithOptions(reader, MXLOptions{})
}

// DecodeMXLScoreTimewiseWithOptions reads a compressed score-timewise
// document using explicit resource limits.
func DecodeMXLScoreTimewiseWithOptions(
	reader io.Reader,
	options MXLOptions,
) (*ScoreTimewise, error) {
	document, err := DecodeMXLWithOptions(reader, options)
	if err != nil {
		return nil, err
	}
	value, ok := AsScoreTimewise(document)
	if !ok {
		return nil, &UnsupportedRootError{Name: documentRootName(document)}
	}

	return value, nil
}

// DecodeMXLOpusDocument reads a compressed opus document.
//
// It returns ErrUnsupportedRoot when the archive contains another root type.
func DecodeMXLOpusDocument(reader io.Reader) (*OpusDocument, error) {
	return DecodeMXLOpusDocumentWithOptions(reader, MXLOptions{})
}

// DecodeMXLOpusDocumentWithOptions reads a compressed opus document using
// explicit resource limits.
func DecodeMXLOpusDocumentWithOptions(
	reader io.Reader,
	options MXLOptions,
) (*OpusDocument, error) {
	document, err := DecodeMXLWithOptions(reader, options)
	if err != nil {
		return nil, err
	}
	value, ok := AsOpusDocument(document)
	if !ok {
		return nil, &UnsupportedRootError{Name: documentRootName(document)}
	}

	return value, nil
}

// EncodeMXL writes a compressed MusicXML document.
func EncodeMXL(
	writer io.Writer,
	document Document,
) error {
	return EncodeMXLPackage(writer, &MXLPackage{
		Document: document,
	})
}

type mxlLimits struct {
	archiveSize   int64
	metadataSize  int64
	documentSize  int64
	resourceSize  int64
	resourcesSize int64
	xmlDepth      int
	dtd           *DTDOptions
	// infoset, when set, decodes the root document from the full DTD
	// infoset on this account, as DecodeAssessed does: DTD defaults reach the
	// model and internal-subset entities expand. Ordinary decoding leaves it nil.
	infoset *validation.Account
}

type mxlContainer struct {
	XMLName   xml.Name     `xml:"container"`
	RootFiles mxlRootFiles `xml:"rootfiles"`
}

type mxlRootFiles struct {
	Files []mxlRootFile `xml:"rootfile"`
}

type mxlRootFile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr,omitempty"`
}

func decodeMXL(
	reader io.Reader,
	limits mxlLimits,
) (Document, error) {
	return decodeMXLContext(context.Background(), reader, limits)
}

func decodeMXLContext(
	ctx context.Context,
	reader io.Reader,
	limits mxlLimits,
) (Document, error) {
	archive, err := openMXL(ctx, reader, limits)
	if err != nil {
		return nil, err
	}

	document, err := archive.decodeDocument(ctx, limits)
	if err != nil {
		return nil, err
	}
	if err := mxlContextErr(ctx); err != nil {
		return nil, err
	}

	return document, nil
}

func indexMXLEntries(
	files []*zip.File,
) (map[string]*zip.File, error) {
	result := make(map[string]*zip.File, len(files))
	seen := make(map[string]struct{}, len(files))

	for _, file := range files {
		if !validMXLPath(file.Name, file.FileInfo().IsDir()) {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLInvalidPath,
				file.Name,
			)
		}
		if _, ok := seen[file.Name]; ok {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLDuplicateEntry,
				file.Name,
			)
		}
		seen[file.Name] = struct{}{}

		if file.FileInfo().IsDir() {
			continue
		}

		result[file.Name] = file
	}

	return result, nil
}

func validMXLPath(
	value string,
	directory bool,
) bool {
	if strings.ContainsRune(value, '\\') {
		return false
	}

	if directory {
		value = strings.TrimSuffix(value, "/")
	}

	return value != "" &&
		value != "." &&
		path.Clean(value) == value &&
		fs.ValidPath(value)
}

func validateMXLMIMEType(
	ctx context.Context,
	file *zip.File,
	limit int64,
) error {
	if file == nil {
		return nil
	}
	if file.Method != zip.Store ||
		file.Flags&1 != 0 ||
		len(file.Extra) != 0 {
		return ErrMXLInvalidMIMEType
	}

	value, err := readMXLFile(ctx, file, limit)
	if err != nil {
		return err
	}
	if string(value) != mxlMIMEType {
		return fmt.Errorf(
			"%w: got %q",
			ErrMXLInvalidMIMEType,
			value,
		)
	}

	return nil
}

func validateMXLRootFile(value mxlRootFile) error {
	if !validMXLPath(value.FullPath, false) {
		return fmt.Errorf(
			"%w: %q",
			ErrMXLInvalidPath,
			value.FullPath,
		)
	}

	mediaType := collapseValidationWhitespace(value.MediaType)
	if mediaType != "" && mediaType != musicXMLMIMEType {
		return fmt.Errorf(
			"%w: %q",
			ErrMXLUnsupportedMediaType,
			value.MediaType,
		)
	}

	return nil
}

func decodeMXLContainer(
	data []byte,
) (mxlContainer, error) {
	source, err := newXMLDecoder(bytes.NewReader(data))
	if err != nil {
		return mxlContainer{}, fmt.Errorf(
			"%w: initialize XML decoder: %w",
			ErrMXLInvalidContainer,
			err,
		)
	}

	decoder := xml.NewTokenDecoder(&namespaceXMLTokenReader{
		source: source,
	})
	start, err := readRoot(decoder)
	if err != nil {
		return mxlContainer{}, fmt.Errorf(
			"%w: %w",
			ErrMXLInvalidContainer,
			err,
		)
	}
	if start.Name.Space != "" ||
		start.Name.Local != "container" {
		return mxlContainer{}, fmt.Errorf(
			"%w: unexpected root {%s}%s",
			ErrMXLInvalidContainer,
			start.Name.Space,
			start.Name.Local,
		)
	}

	var result mxlContainer
	if err := decoder.DecodeElement(&result, &start); err != nil {
		return mxlContainer{}, fmt.Errorf(
			"%w: decode: %w",
			ErrMXLInvalidContainer,
			err,
		)
	}
	if err := readDocumentTail(decoder); err != nil {
		return mxlContainer{}, fmt.Errorf(
			"%w: %w",
			ErrMXLInvalidContainer,
			err,
		)
	}
	for index := range result.RootFiles.Files {
		rootFile := &result.RootFiles.Files[index]
		// Both attributes are xs:token values, not literal ZIP entry names.
		rootFile.FullPath = collapseValidationWhitespace(rootFile.FullPath)
		rootFile.MediaType = collapseValidationWhitespace(rootFile.MediaType)
	}

	return result, nil
}

// readMXLFile expands one archive entry. A done ctx stops the expansion at the
// next read and takes precedence over any read or close error, so
// cancellation is never reported as an invalid archive.
func readMXLFile(
	ctx context.Context,
	file *zip.File,
	limit int64,
) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf(
			"%w: entry %q exceeds %d bytes",
			ErrMXLTooLarge,
			file.Name,
			limit,
		)
	}

	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf(
			"%w: open entry %q: %w",
			ErrInvalidMXL,
			file.Name,
			err,
		)
	}

	result, readErr := readMXLStream(
		mxlContextReader(ctx, reader),
		limit,
		file.Name,
	)
	closeErr := reader.Close()
	if err := mxlContextErr(ctx); err != nil {
		return nil, err
	}
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf(
			"%w: close entry %q: %w",
			ErrInvalidMXL,
			file.Name,
			closeErr,
		)
	}

	return result, nil
}

func readMXLStream(
	reader io.Reader,
	limit int64,
	name string,
) ([]byte, error) {
	// Read one byte beyond the limit to distinguish an exact fit from an
	// oversized stream, without wrapping an explicitly allowed MaxInt64.
	readLimit := limit
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	result, err := io.ReadAll(io.LimitReader(reader, readLimit))
	if err != nil {
		return nil, fmt.Errorf(
			"%w: read %s: %w",
			ErrInvalidMXL,
			name,
			err,
		)
	}
	if int64(len(result)) > limit {
		return nil, fmt.Errorf(
			"%w: %s exceeds %d bytes",
			ErrMXLTooLarge,
			name,
			limit,
		)
	}

	return result, nil
}

func encodeMXLDocument(
	document Document,
) ([]byte, error) {
	var result bytes.Buffer
	result.WriteString(xml.Header)

	if err := Encode(&result, document); err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

func encodeMXLContainer(
	rootFilePath string,
) ([]byte, error) {
	return encodeMXLContainerRootFiles([]mxlRootFile{
		{
			FullPath:  rootFilePath,
			MediaType: musicXMLMIMEType,
		},
	})
}

func encodeMXLContainerRootFiles(
	rootFiles []mxlRootFile,
) ([]byte, error) {
	value := mxlContainer{
		RootFiles: mxlRootFiles{Files: rootFiles},
	}

	var result bytes.Buffer
	result.WriteString(xml.Header)

	if err := xml.NewEncoder(&result).Encode(value); err != nil {
		return nil, fmt.Errorf(
			"musicxml: encode MXL container: %w",
			err,
		)
	}

	return result.Bytes(), nil
}

func writeMXLFile(
	archive *zip.Writer,
	name string,
	method uint16,
	data []byte,
) error {
	writer, err := archive.CreateHeader(&zip.FileHeader{
		Name:   name,
		Method: method,
	})
	if err != nil {
		return fmt.Errorf(
			"musicxml: create MXL entry %q: %w",
			name,
			err,
		)
	}
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf(
			"musicxml: write MXL entry %q: %w",
			name,
			err,
		)
	}

	return nil
}
