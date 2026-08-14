package musicxml

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

// MXLPackage is a compressed MusicXML document and its related files.
//
// The first RootFiles entry identifies Document. Resources contains every
// other regular archive file except mimetype and META-INF/container.xml.
// Resource paths and bytes are preserved; ZIP compression metadata is not.
type MXLPackage struct {
	Document  Document
	RootFiles []MXLRootFile
	Resources []MXLResource
}

// MXLRootFile describes one rootfile entry from META-INF/container.xml.
//
// The first entry must identify the package's MusicXML document. Later entries
// may identify alternate renditions such as PDF or audio files.
type MXLRootFile struct {
	FullPath  string
	MediaType string
}

// MXLResource is a non-primary file stored in an MXL package.
type MXLResource struct {
	Path string
	Data []byte
}

// DecodeMXLPackage reads a compressed MusicXML document together with all
// related files stored in its ZIP archive.
func DecodeMXLPackage(reader io.Reader) (*MXLPackage, error) {
	return decodeMXLPackage(reader, defaultMXLLimits())
}

// EncodeMXLPackage writes a compressed MusicXML document together with its
// related files.
//
// If RootFiles is empty, score.musicxml is used as the primary root file.
func EncodeMXLPackage(
	writer io.Writer,
	value *MXLPackage,
) error {
	if writer == nil {
		return ErrNilWriter
	}
	if value == nil {
		return ErrNilMXLPackage
	}

	rootFiles, err := prepareMXLPackage(value)
	if err != nil {
		return err
	}

	documentXML, err := encodeMXLDocument(value.Document)
	if err != nil {
		return err
	}

	containerXML, err := encodeMXLContainerRootFiles(rootFiles)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(writer)
	if err := writeMXLFile(
		archive,
		mxlMIMETypePath,
		zip.Store,
		[]byte(mxlMIMEType),
	); err != nil {
		return err
	}
	if err := writeMXLFile(
		archive,
		mxlContainerPath,
		zip.Deflate,
		containerXML,
	); err != nil {
		return err
	}
	if err := writeMXLFile(
		archive,
		rootFiles[0].FullPath,
		zip.Deflate,
		documentXML,
	); err != nil {
		return err
	}

	for _, resource := range value.Resources {
		if err := writeMXLFile(
			archive,
			resource.Path,
			zip.Deflate,
			resource.Data,
		); err != nil {
			return err
		}
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("musicxml: close MXL archive: %w", err)
	}

	return nil
}

type openedMXL struct {
	archive      *zip.Reader
	entries      map[string]*zip.File
	container    mxlContainer
	rootFile     mxlRootFile
	documentFile *zip.File
}

func defaultMXLLimits() mxlLimits {
	return mxlLimits{
		archiveSize:   maxMXLArchiveSize,
		metadataSize:  maxMXLMetadataSize,
		documentSize:  maxMXLDocumentSize,
		resourceSize:  maxMXLResourceSize,
		resourcesSize: maxMXLResourcesSize,
	}
}

func openMXL(
	reader io.Reader,
	limits mxlLimits,
) (*openedMXL, error) {
	if reader == nil {
		return nil, ErrNilReader
	}

	data, err := readMXLStream(
		reader,
		limits.archiveSize,
		"archive",
	)
	if err != nil {
		return nil, err
	}

	archive, err := zip.NewReader(
		bytes.NewReader(data),
		int64(len(data)),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: open ZIP archive: %w",
			ErrInvalidMXL,
			err,
		)
	}

	entries, err := indexMXLEntries(archive.File)
	if err != nil {
		return nil, err
	}
	if err := validateMXLMIMEType(
		entries[mxlMIMETypePath],
		limits.metadataSize,
	); err != nil {
		return nil, err
	}

	containerFile := entries[mxlContainerPath]
	if containerFile == nil {
		return nil, ErrMXLContainerNotFound
	}

	containerData, err := readMXLFile(
		containerFile,
		limits.metadataSize,
	)
	if err != nil {
		return nil, err
	}

	container, err := decodeMXLContainer(containerData)
	if err != nil {
		return nil, err
	}
	if len(container.RootFiles.Files) == 0 {
		return nil, ErrMXLRootFileNotFound
	}

	rootFile := container.RootFiles.Files[0]
	if err := validateMXLRootFile(rootFile); err != nil {
		return nil, err
	}

	documentFile := entries[rootFile.FullPath]
	if documentFile == nil {
		return nil, fmt.Errorf(
			"%w: %q",
			ErrMXLRootFileNotFound,
			rootFile.FullPath,
		)
	}

	return &openedMXL{
		archive:      archive,
		entries:      entries,
		container:    container,
		rootFile:     rootFile,
		documentFile: documentFile,
	}, nil
}

func (a *openedMXL) decodeDocument(
	limit int64,
) (Document, error) {
	documentData, err := readMXLFile(a.documentFile, limit)
	if err != nil {
		return nil, err
	}

	document, err := Decode(bytes.NewReader(documentData))
	if err != nil {
		return nil, fmt.Errorf(
			"musicxml: decode MXL root file %q: %w",
			a.rootFile.FullPath,
			err,
		)
	}

	return document, nil
}

func decodeMXLPackage(
	reader io.Reader,
	limits mxlLimits,
) (*MXLPackage, error) {
	archive, err := openMXL(reader, limits)
	if err != nil {
		return nil, err
	}

	rootFiles, err := decodeMXLPackageRootFiles(
		archive.container.RootFiles.Files,
		archive.entries,
	)
	if err != nil {
		return nil, err
	}

	document, err := archive.decodeDocument(limits.documentSize)
	if err != nil {
		return nil, err
	}

	resources, err := archive.decodeResources(limits)
	if err != nil {
		return nil, err
	}

	return &MXLPackage{
		Document:  document,
		RootFiles: rootFiles,
		Resources: resources,
	}, nil
}

func decodeMXLPackageRootFiles(
	values []mxlRootFile,
	entries map[string]*zip.File,
) ([]MXLRootFile, error) {
	result := make([]MXLRootFile, len(values))
	seen := make(map[string]struct{}, len(values))

	for index, value := range values {
		if !validMXLContentPath(value.FullPath) {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLInvalidPath,
				value.FullPath,
			)
		}
		if index == 0 {
			if err := validateMXLRootFile(value); err != nil {
				return nil, err
			}
		}
		if _, ok := seen[value.FullPath]; ok {
			return nil, fmt.Errorf(
				"%w: rootfile %q",
				ErrMXLDuplicateEntry,
				value.FullPath,
			)
		}
		seen[value.FullPath] = struct{}{}

		if entries[value.FullPath] == nil {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLRootFileNotFound,
				value.FullPath,
			)
		}

		result[index] = MXLRootFile{
			FullPath:  value.FullPath,
			MediaType: value.MediaType,
		}
	}

	return result, nil
}

func (a *openedMXL) decodeResources(
	limits mxlLimits,
) ([]MXLResource, error) {
	var result []MXLResource
	var total int64

	for _, file := range a.archive.File {
		if file.FileInfo().IsDir() ||
			file.Name == mxlMIMETypePath ||
			file.Name == mxlContainerPath ||
			file.Name == a.rootFile.FullPath {
			continue
		}

		remaining := limits.resourcesSize - total
		if remaining < 0 ||
			file.UncompressedSize64 > uint64(remaining) {
			return nil, fmt.Errorf(
				"%w: resources exceed %d bytes",
				ErrMXLTooLarge,
				limits.resourcesSize,
			)
		}

		data, err := readMXLFile(file, limits.resourceSize)
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > remaining {
			return nil, fmt.Errorf(
				"%w: resources exceed %d bytes",
				ErrMXLTooLarge,
				limits.resourcesSize,
			)
		}
		total += int64(len(data))

		result = append(result, MXLResource{
			Path: file.Name,
			Data: data,
		})
	}

	return result, nil
}

func prepareMXLPackage(
	value *MXLPackage,
) ([]mxlRootFile, error) {
	rootFiles := make([]mxlRootFile, len(value.RootFiles))
	if len(rootFiles) == 0 {
		rootFiles = []mxlRootFile{
			{
				FullPath:  mxlRootFilePath,
				MediaType: musicXMLMIMEType,
			},
		}
	} else {
		for index, rootFile := range value.RootFiles {
			rootFiles[index] = mxlRootFile{
				FullPath:  rootFile.FullPath,
				MediaType: rootFile.MediaType,
			}
		}
	}

	available := map[string]struct{}{
		rootFiles[0].FullPath: {},
	}
	seenEntries := map[string]struct{}{
		mxlMIMETypePath:       {},
		mxlContainerPath:      {},
		rootFiles[0].FullPath: {},
	}

	for _, resource := range value.Resources {
		if !validMXLContentPath(resource.Path) {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLInvalidPath,
				resource.Path,
			)
		}
		if _, ok := seenEntries[resource.Path]; ok {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLDuplicateEntry,
				resource.Path,
			)
		}

		seenEntries[resource.Path] = struct{}{}
		available[resource.Path] = struct{}{}
	}

	seenRootFiles := make(map[string]struct{}, len(rootFiles))
	for index, rootFile := range rootFiles {
		if !validMXLContentPath(rootFile.FullPath) {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLInvalidPath,
				rootFile.FullPath,
			)
		}
		if index == 0 {
			if err := validateMXLRootFile(rootFile); err != nil {
				return nil, err
			}
		}
		if _, ok := seenRootFiles[rootFile.FullPath]; ok {
			return nil, fmt.Errorf(
				"%w: rootfile %q",
				ErrMXLDuplicateEntry,
				rootFile.FullPath,
			)
		}
		seenRootFiles[rootFile.FullPath] = struct{}{}

		if _, ok := available[rootFile.FullPath]; !ok {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrMXLRootFileNotFound,
				rootFile.FullPath,
			)
		}
	}

	return rootFiles, nil
}

func validMXLContentPath(value string) bool {
	return value != mxlMIMETypePath &&
		value != mxlContainerPath &&
		validMXLPath(value, false)
}
