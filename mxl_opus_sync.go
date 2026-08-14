package musicxml

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

// SyncResolvedOpus writes edits made through a resolved opus graph back to the
// package.
//
// The graph must be the root returned by ResolveOpus for this package.
// Documents may be edited in place, but changing the graph topology requires a
// new ResolveOpus call. Unchanged linked documents retain their original bytes,
// including formatting and character encoding. Resource updates are committed
// only after every reachable document has been checked and encoded.
func (value *MXLPackage) SyncResolvedOpus(
	resolved *MXLResolvedOpus,
) error {
	if value == nil {
		return ErrNilMXLPackage
	}
	if resolved == nil {
		return ErrNilMXLResolvedOpus
	}

	state := resolved.state
	if state == nil ||
		state.root != resolved ||
		state.packageValue != value {
		return ErrMXLResolvedOpusMismatch
	}

	rootFiles, err := prepareMXLPackage(value)
	if err != nil {
		return err
	}
	if rootFiles[0].FullPath != state.rootPath {
		return fmt.Errorf(
			"%w: root path changed from %q to %q",
			ErrMXLResolvedOpusStale,
			state.rootPath,
			rootFiles[0].FullPath,
		)
	}
	if value.Document != state.documents[state.rootPath] {
		return fmt.Errorf(
			"%w: root document %q was replaced",
			ErrMXLResolvedOpusStale,
			state.rootPath,
		)
	}

	documents, err := state.collectDocuments()
	if err != nil {
		return err
	}

	resourceIndexes := make(map[string]int, len(value.Resources))
	for index := range value.Resources {
		resourceIndexes[value.Resources[index].Path] = index
	}

	paths := make([]string, 0, len(documents))
	for documentPath := range documents {
		paths = append(paths, documentPath)
	}
	sort.Strings(paths)

	for _, documentPath := range paths {
		if documentPath == state.rootPath {
			continue
		}

		index, ok := resourceIndexes[documentPath]
		if !ok {
			return fmt.Errorf(
				"%w: linked document %q is missing",
				ErrMXLResolvedOpusStale,
				documentPath,
			)
		}

		actual := sha256.Sum256(value.Resources[index].Data)
		if actual != state.resourceHashes[documentPath] {
			return fmt.Errorf(
				"%w: linked document %q changed outside the resolved graph",
				ErrMXLResolvedOpusStale,
				documentPath,
			)
		}
	}

	encoded := make(map[string][]byte, len(documents))
	for _, documentPath := range paths {
		data, err := encodeMXLDocument(documents[documentPath])
		if err != nil {
			return fmt.Errorf(
				"musicxml: sync resolved document %q: %w",
				documentPath,
				err,
			)
		}
		encoded[documentPath] = data
	}

	for _, documentPath := range paths {
		if documentPath == state.rootPath ||
			sha256.Sum256(encoded[documentPath]) ==
				state.canonicalHashes[documentPath] {
			continue
		}

		index := resourceIndexes[documentPath]
		value.Resources[index].Data = encoded[documentPath]
	}

	value.Document = resolved.Document

	for _, documentPath := range paths {
		state.canonicalHashes[documentPath] = sha256.Sum256(
			encoded[documentPath],
		)
		if documentPath == state.rootPath {
			continue
		}

		index := resourceIndexes[documentPath]
		state.resourceHashes[documentPath] = sha256.Sum256(
			value.Resources[index].Data,
		)
	}

	return nil
}

type mxlResolvedOpusState struct {
	packageValue    *MXLPackage
	root            *MXLResolvedOpus
	rootPath        string
	documents       map[string]Document
	canonicalHashes map[string][sha256.Size]byte
	resourceHashes  map[string][sha256.Size]byte
}

func newMXLResolvedOpusState(
	packageValue *MXLPackage,
	rootPath string,
	root *MXLResolvedOpus,
	documents map[string]Document,
	resources map[string][]byte,
) (*mxlResolvedOpusState, error) {
	result := &mxlResolvedOpusState{
		packageValue: packageValue,
		root:         root,
		rootPath:     rootPath,
		documents:    make(map[string]Document, len(documents)),
		canonicalHashes: make(
			map[string][sha256.Size]byte,
			len(documents),
		),
		resourceHashes: make(map[string][sha256.Size]byte, len(documents)-1),
	}

	for documentPath, document := range documents {
		data, err := encodeMXLDocument(document)
		if err != nil {
			return nil, fmt.Errorf(
				"musicxml: capture resolved document %q: %w",
				documentPath,
				err,
			)
		}

		result.documents[documentPath] = document
		result.canonicalHashes[documentPath] = sha256.Sum256(data)

		if documentPath == rootPath {
			continue
		}

		resource, ok := resources[documentPath]
		if !ok {
			return nil, fmt.Errorf(
				"%w: linked document %q is missing",
				ErrMXLResolvedOpusStale,
				documentPath,
			)
		}
		result.resourceHashes[documentPath] = sha256.Sum256(resource)
	}

	return result, nil
}

func (s *mxlResolvedOpusState) collectDocuments() (
	map[string]Document,
	error,
) {
	result := make(map[string]Document, len(s.documents))
	visited := make(map[*MXLResolvedOpus]struct{})

	var walk func(*MXLResolvedOpus, bool) error
	walk = func(
		opus *MXLResolvedOpus,
		registerDocument bool,
	) error {
		if opus == nil || opus.Document == nil {
			return fmt.Errorf(
				"%w: opus at %q has a nil document",
				ErrMXLResolvedOpusInvalid,
				s.rootPath,
			)
		}

		if registerDocument {
			if err := s.addDocument(
				result,
				opus.Path,
				opus.Document,
			); err != nil {
				return err
			}
		}

		if _, ok := visited[opus]; ok {
			return nil
		}
		visited[opus] = struct{}{}

		if len(opus.Content) != len(opus.Document.Content) {
			return fmt.Errorf(
				"%w: opus %q content length changed from %d to %d",
				ErrMXLResolvedOpusInvalid,
				opus.Path,
				len(opus.Content),
				len(opus.Document.Content),
			)
		}

		for index := range opus.Content {
			source := opus.Document.Content[index]
			resolved := opus.Content[index]

			selected := 0
			if source.Opus != nil {
				selected++
			}
			if source.OpusLink != nil {
				selected++
			}
			if source.Score != nil {
				selected++
			}
			if selected != 1 {
				return fmt.Errorf(
					"%w: opus %q content at index %d "+
						"contains %d source values",
					ErrMXLResolvedOpusInvalid,
					opus.Path,
					index,
					selected,
				)
			}

			switch {
			case source.Opus != nil:
				if resolved.Opus == nil ||
					resolved.OpusLink != nil ||
					resolved.Score != nil ||
					resolved.Opus.Document != source.Opus ||
					resolved.Opus.Path != opus.Path {
					return newInvalidResolvedOpusContentError(
						opus.Path,
						index,
					)
				}
				if err := walk(resolved.Opus, false); err != nil {
					return err
				}

			case source.OpusLink != nil:
				if resolved.Opus != nil ||
					resolved.OpusLink == nil ||
					resolved.Score != nil ||
					resolved.OpusLink.Link != source.OpusLink ||
					resolved.OpusLink.Target == nil {
					return newInvalidResolvedOpusContentError(
						opus.Path,
						index,
					)
				}

				targetPath, fragment, err := resolveMXLLinkPath(
					opus.Path,
					source.OpusLink.Href,
				)
				if err != nil {
					return newMXLLinkError(
						opus.Path,
						source.OpusLink.Href,
						"",
						err,
					)
				}
				if resolved.OpusLink.Path != targetPath ||
					resolved.OpusLink.Fragment != fragment ||
					resolved.OpusLink.Target.Path != targetPath {
					return newInvalidResolvedOpusContentError(
						opus.Path,
						index,
					)
				}
				if err := walk(
					resolved.OpusLink.Target,
					true,
				); err != nil {
					return err
				}

			case source.Score != nil:
				if resolved.Opus != nil ||
					resolved.OpusLink != nil ||
					resolved.Score == nil ||
					resolved.Score.Link != source.Score ||
					resolved.Score.Target == nil {
					return newInvalidResolvedOpusContentError(
						opus.Path,
						index,
					)
				}

				targetPath, fragment, err := resolveMXLLinkPath(
					opus.Path,
					source.Score.Href,
				)
				if err != nil {
					return newMXLLinkError(
						opus.Path,
						source.Score.Href,
						"",
						err,
					)
				}
				if resolved.Score.Path != targetPath ||
					resolved.Score.Fragment != fragment {
					return newInvalidResolvedOpusContentError(
						opus.Path,
						index,
					)
				}
				if err := s.addDocument(
					result,
					targetPath,
					resolved.Score.Target,
				); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := walk(s.root, true); err != nil {
		return nil, err
	}

	if len(result) != len(s.documents) {
		return nil, fmt.Errorf(
			"%w: reachable document set changed from %d to %d",
			ErrMXLResolvedOpusInvalid,
			len(s.documents),
			len(result),
		)
	}

	return result, nil
}

func (s *mxlResolvedOpusState) addDocument(
	result map[string]Document,
	documentPath string,
	document Document,
) error {
	if !validMXLContentPath(documentPath) {
		return fmt.Errorf(
			"%w: document has invalid path %q",
			ErrMXLResolvedOpusInvalid,
			documentPath,
		)
	}

	expected, ok := s.documents[documentPath]
	if !ok {
		return fmt.Errorf(
			"%w: document %q was added after resolution",
			ErrMXLResolvedOpusInvalid,
			documentPath,
		)
	}
	if expected != document {
		return fmt.Errorf(
			"%w: document %q was replaced",
			ErrMXLResolvedOpusInvalid,
			documentPath,
		)
	}

	if existing, ok := result[documentPath]; ok &&
		existing != document {
		return fmt.Errorf(
			"%w: document %q has conflicting targets",
			ErrMXLResolvedOpusInvalid,
			documentPath,
		)
	}
	result[documentPath] = document

	return nil
}

func newInvalidResolvedOpusContentError(
	documentPath string,
	index int,
) error {
	return fmt.Errorf(
		"%w: opus %q content at index %d no longer matches its document",
		ErrMXLResolvedOpusInvalid,
		documentPath,
		index,
	)
}
