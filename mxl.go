package musicxml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
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
	archive, err := openMXL(reader, limits)
	if err != nil {
		return nil, err
	}

	return archive.decodeDocument(limits)
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

	value, err := readMXLFile(file, limit)
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

	mediaType := strings.TrimSpace(value.MediaType)
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
	decoder, err := newXMLDecoder(bytes.NewReader(data))
	if err != nil {
		return mxlContainer{}, fmt.Errorf(
			"%w: initialize XML decoder: %w",
			ErrMXLInvalidContainer,
			err,
		)
	}

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

	return result, nil
}

func readMXLFile(
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

	result, readErr := readMXLStream(reader, limit, file.Name)
	closeErr := reader.Close()
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
	result, err := io.ReadAll(io.LimitReader(reader, limit+1))
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
