package musicxml

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// MXLResolvedOpus is an opus document whose ordered content has been resolved
// against the related files in an MXL package.
//
// Path is the archive path of the document that contains Document. Inline
// nested opus elements therefore have the same Path as their parent. Linked
// opus documents are memoized by path, so a linked Target may point to an
// ancestor when the package contains a cycle.
type MXLResolvedOpus struct {
	Path     string
	Document *OpusDocument
	Content  []MXLResolvedOpusContent

	state *mxlResolvedOpusState
}

// MXLResolvedOpusContent is one ordered child of an MXLResolvedOpus.
//
// Exactly one field is non-nil.
type MXLResolvedOpusContent struct {
	Opus     *MXLResolvedOpus
	OpusLink *MXLResolvedOpusLink
	Score    *MXLResolvedScore
}

// MXLResolvedOpusLink is an opus-link and its resolved target.
type MXLResolvedOpusLink struct {
	Link     *OpusLink
	Path     string
	Fragment string
	Target   *MXLResolvedOpus
}

// MXLResolvedScore is a score link and its resolved target.
type MXLResolvedScore struct {
	Link     *OpusScore
	Path     string
	Fragment string
	Target   ScoreDocument
}

// ResolveOpus resolves every score and opus-link reachable from the package's
// root opus document.
//
// Relative XLink references are resolved against the archive path of the opus
// document containing the link. Only documents stored inside the MXL package
// are resolved. Linked documents are decoded without changing Resources;
// call SyncResolvedOpus after editing to update their resource bytes.
//
// Opus-link cycles between archive documents are supported, but a cyclic or
// excessively deep in-memory opus model is rejected before traversal.
func (value *MXLPackage) ResolveOpus() (*MXLResolvedOpus, error) {
	if value == nil {
		return nil, ErrNilMXLPackage
	}

	document, ok := value.Document.(*OpusDocument)
	if !ok {
		if value.Document == nil {
			return nil, ErrNilDocument
		}

		return nil, fmt.Errorf(
			"%w: got %T",
			ErrMXLNotOpus,
			value.Document,
		)
	}
	if document == nil {
		return nil, ErrNilDocument
	}
	if err := checkDocumentNesting(document); err != nil {
		return nil, err
	}

	rootFiles, err := prepareMXLPackage(value)
	if err != nil {
		return nil, err
	}

	resources := make(map[string][]byte, len(value.Resources))
	for _, resource := range value.Resources {
		resources[resource.Path] = resource.Data
	}

	rootPath := rootFiles[0].FullPath
	resolver := mxlOpusResolver{
		resources: resources,
		documents: map[string]Document{
			rootPath: document,
		},
		opuses: make(map[string]*MXLResolvedOpus),
	}

	resolved, err := resolver.resolveOpus(rootPath, document)
	if err != nil {
		return nil, err
	}

	state, err := newMXLResolvedOpusState(
		value,
		rootPath,
		resolved,
		resolver.documents,
		resources,
	)
	if err != nil {
		return nil, err
	}
	resolved.state = state

	return resolved, nil
}

type mxlOpusResolver struct {
	resources map[string][]byte
	documents map[string]Document
	opuses    map[string]*MXLResolvedOpus
}

func (r *mxlOpusResolver) resolveOpus(
	documentPath string,
	document *OpusDocument,
) (*MXLResolvedOpus, error) {
	if resolved := r.opuses[documentPath]; resolved != nil {
		return resolved, nil
	}

	result := &MXLResolvedOpus{
		Path:     documentPath,
		Document: document,
	}
	r.opuses[documentPath] = result

	content, err := r.resolveContent(documentPath, document.Content)
	if err != nil {
		return nil, err
	}
	result.Content = content

	return result, nil
}

func (r *mxlOpusResolver) resolveInlineOpus(
	documentPath string,
	document *OpusDocument,
) (*MXLResolvedOpus, error) {
	result := &MXLResolvedOpus{
		Path:     documentPath,
		Document: document,
	}

	content, err := r.resolveContent(documentPath, document.Content)
	if err != nil {
		return nil, err
	}
	result.Content = content

	return result, nil
}

func (r *mxlOpusResolver) resolveContent(
	sourcePath string,
	values []OpusDocumentContent,
) ([]MXLResolvedOpusContent, error) {
	result := make([]MXLResolvedOpusContent, len(values))

	for index, value := range values {
		selected := 0
		if value.Opus != nil {
			selected++
		}
		if value.OpusLink != nil {
			selected++
		}
		if value.Score != nil {
			selected++
		}
		if selected != 1 {
			return nil, fmt.Errorf(
				"musicxml: resolve opus content at index %d: "+
					"must contain exactly one value, got %d",
				index,
				selected,
			)
		}

		switch {
		case value.Opus != nil:
			resolved, err := r.resolveInlineOpus(
				sourcePath,
				value.Opus,
			)
			if err != nil {
				return nil, err
			}
			result[index].Opus = resolved

		case value.OpusLink != nil:
			resolved, err := r.resolveOpusLink(
				sourcePath,
				value.OpusLink,
			)
			if err != nil {
				return nil, err
			}
			result[index].OpusLink = resolved

		case value.Score != nil:
			resolved, err := r.resolveScore(
				sourcePath,
				value.Score,
			)
			if err != nil {
				return nil, err
			}
			result[index].Score = resolved
		}
	}

	return result, nil
}

func (r *mxlOpusResolver) resolveOpusLink(
	sourcePath string,
	link *OpusLink,
) (*MXLResolvedOpusLink, error) {
	targetPath, fragment, err := resolveMXLLinkPath(
		sourcePath,
		link.Href,
	)
	if err != nil {
		return nil, newMXLLinkError(
			sourcePath,
			link.Href,
			"",
			err,
		)
	}

	document, err := r.loadDocument(targetPath)
	if err != nil {
		return nil, newMXLLinkError(
			sourcePath,
			link.Href,
			targetPath,
			err,
		)
	}

	opus, ok := document.(*OpusDocument)
	if !ok || opus == nil {
		return nil, newMXLLinkError(
			sourcePath,
			link.Href,
			targetPath,
			fmt.Errorf(
				"%w: opus-link resolved to %T",
				ErrMXLLinkedDocumentType,
				document,
			),
		)
	}

	target, err := r.resolveOpus(targetPath, opus)
	if err != nil {
		return nil, err
	}

	return &MXLResolvedOpusLink{
		Link:     link,
		Path:     targetPath,
		Fragment: fragment,
		Target:   target,
	}, nil
}

func (r *mxlOpusResolver) resolveScore(
	sourcePath string,
	link *OpusScore,
) (*MXLResolvedScore, error) {
	targetPath, fragment, err := resolveMXLLinkPath(
		sourcePath,
		link.Href,
	)
	if err != nil {
		return nil, newMXLLinkError(
			sourcePath,
			link.Href,
			"",
			err,
		)
	}

	document, err := r.loadDocument(targetPath)
	if err != nil {
		return nil, newMXLLinkError(
			sourcePath,
			link.Href,
			targetPath,
			err,
		)
	}

	score, ok := document.(ScoreDocument)
	if !ok || score == nil {
		return nil, newMXLLinkError(
			sourcePath,
			link.Href,
			targetPath,
			fmt.Errorf(
				"%w: score link resolved to %T",
				ErrMXLLinkedDocumentType,
				document,
			),
		)
	}

	return &MXLResolvedScore{
		Link:     link,
		Path:     targetPath,
		Fragment: fragment,
		Target:   score,
	}, nil
}

func (r *mxlOpusResolver) loadDocument(
	documentPath string,
) (Document, error) {
	if document := r.documents[documentPath]; document != nil {
		return document, nil
	}

	data, ok := r.resources[documentPath]
	if !ok {
		return nil, ErrMXLLinkedDocumentNotFound
	}

	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %w",
			ErrMXLLinkedDocumentInvalid,
			err,
		)
	}
	r.documents[documentPath] = document

	return document, nil
}

func resolveMXLLinkPath(
	sourcePath string,
	href string,
) (string, string, error) {
	reference, err := url.Parse(href)
	if err != nil {
		return "", "", fmt.Errorf(
			"%w: %v",
			ErrMXLInvalidLink,
			err,
		)
	}
	if reference.IsAbs() ||
		reference.Host != "" ||
		reference.User != nil ||
		reference.Opaque != "" {
		return "", "", ErrMXLExternalLink
	}
	if reference.RawQuery != "" || reference.ForceQuery {
		return "", "", fmt.Errorf(
			"%w: query parameters are not archive paths",
			ErrMXLInvalidLink,
		)
	}

	targetPath := reference.Path
	switch {
	case targetPath == "":
		targetPath = sourcePath
	case strings.HasPrefix(targetPath, "/"):
		targetPath = strings.TrimPrefix(targetPath, "/")
	default:
		targetPath = path.Join(path.Dir(sourcePath), targetPath)
	}

	if !validMXLContentPath(targetPath) {
		return "", "", fmt.Errorf(
			"%w: %w",
			ErrMXLInvalidLink,
			ErrMXLInvalidPath,
		)
	}

	return targetPath, reference.Fragment, nil
}

func newMXLLinkError(
	sourcePath string,
	href string,
	targetPath string,
	err error,
) *MXLLinkError {
	return &MXLLinkError{
		SourcePath: sourcePath,
		Href:       href,
		TargetPath: targetPath,
		Err:        err,
	}
}
