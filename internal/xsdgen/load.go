package xsdgen

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"strings"
)

var (
	ErrNilFS                    = errors.New("xsdgen: nil filesystem")
	ErrInvalidSchemaPath        = errors.New("xsdgen: invalid schema path")
	ErrUnresolvedSchemaLocation = errors.New("xsdgen: unresolved schema location")
	ErrNamespaceMismatch        = errors.New("xsdgen: schema namespace mismatch")
)

// SchemaFile associates a parsed schema with its path in the source filesystem.
type SchemaFile struct {
	Path   string
	Schema *Schema
}

// Set contains a root schema and every locally resolved import and include.
type Set struct {
	Root  *SchemaFile
	Files []*SchemaFile

	filesByPath map[string]*SchemaFile
}

// Lookup returns a loaded schema by its path in the source filesystem.
func (s *Set) Lookup(schemaPath string) (*SchemaFile, bool) {
	if s == nil {
		return nil, false
	}

	file, found := s.filesByPath[schemaPath]
	return file, found
}

// NamespaceMismatchError reports an import or include whose target namespace
// does not match the namespace required by the referring schema.
type NamespaceMismatchError struct {
	Kind     string
	From     string
	Location string
	Expected string
	Actual   string
}

func (e *NamespaceMismatchError) Error() string {
	return fmt.Sprintf(
		"%v: %s %q from %q: expected %q, got %q",
		ErrNamespaceMismatch,
		e.Kind,
		e.Location,
		e.From,
		e.Expected,
		e.Actual,
	)
}

func (e *NamespaceMismatchError) Unwrap() error {
	return ErrNamespaceMismatch
}

// Load reads rootPath and all imports and includes reachable from it.
//
// catalogPath identifies an OASIS XML catalog used to map absolute
// schemaLocation URIs to files in fsys. Network access is never performed.
func Load(
	fsys fs.FS,
	rootPath string,
	catalogPath string,
) (*Set, error) {
	if fsys == nil {
		return nil, ErrNilFS
	}

	rootPath, err := validateInputPath(rootPath)
	if err != nil {
		return nil, err
	}

	catalogPath, err = validateInputPath(catalogPath)
	if err != nil {
		return nil, err
	}

	basePath := path.Dir(catalogPath)
	if !isWithin(basePath, rootPath) {
		return nil, fmt.Errorf(
			"%w: root %q is outside %q",
			ErrInvalidSchemaPath,
			rootPath,
			basePath,
		)
	}

	catalogFile, err := fsys.Open(catalogPath)
	if err != nil {
		return nil, fmt.Errorf(
			"xsdgen: open catalog %q: %w",
			catalogPath,
			err,
		)
	}

	catalog, parseErr := parseCatalog(catalogFile)
	closeErr := catalogFile.Close()

	if parseErr != nil {
		return nil, parseErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf(
			"xsdgen: close catalog %q: %w",
			catalogPath,
			closeErr,
		)
	}

	set := &Set{
		filesByPath: make(map[string]*SchemaFile),
	}

	loader := schemaLoader{
		fsys:        fsys,
		catalog:     catalog,
		catalogPath: catalogPath,
		basePath:    basePath,
		set:         set,
	}

	root, err := loader.load(rootPath)
	if err != nil {
		return nil, err
	}

	set.Root = root
	return set, nil
}

type schemaLoader struct {
	fsys        fs.FS
	catalog     *catalog
	catalogPath string
	basePath    string
	set         *Set
}

func (l *schemaLoader) load(schemaPath string) (*SchemaFile, error) {
	if existing, found := l.set.filesByPath[schemaPath]; found {
		return existing, nil
	}

	reader, err := l.fsys.Open(schemaPath)
	if err != nil {
		return nil, fmt.Errorf(
			"xsdgen: open schema %q: %w",
			schemaPath,
			err,
		)
	}

	schema, parseErr := Parse(reader)
	closeErr := reader.Close()

	if parseErr != nil {
		return nil, fmt.Errorf(
			"xsdgen: parse schema %q: %w",
			schemaPath,
			parseErr,
		)
	}
	if closeErr != nil {
		return nil, fmt.Errorf(
			"xsdgen: close schema %q: %w",
			schemaPath,
			closeErr,
		)
	}

	file := &SchemaFile{
		Path:   schemaPath,
		Schema: schema,
	}

	// Register before following dependencies so import cycles terminate.
	l.set.filesByPath[schemaPath] = file
	l.set.Files = append(l.set.Files, file)

	if err := l.loadImports(file); err != nil {
		return nil, err
	}
	if err := l.loadIncludes(file); err != nil {
		return nil, err
	}

	return file, nil
}

func (l *schemaLoader) loadImports(file *SchemaFile) error {
	for _, value := range file.Schema.Imports {
		if value.SchemaLocation == "" {
			continue
		}

		dependency, err := l.loadDependency(
			file.Path,
			value.SchemaLocation,
		)
		if err != nil {
			return err
		}

		if dependency.Schema.TargetNamespace != value.Namespace {
			return &NamespaceMismatchError{
				Kind:     "import",
				From:     file.Path,
				Location: value.SchemaLocation,
				Expected: value.Namespace,
				Actual:   dependency.Schema.TargetNamespace,
			}
		}
	}

	return nil
}

func (l *schemaLoader) loadIncludes(file *SchemaFile) error {
	for _, value := range file.Schema.Includes {
		if value.SchemaLocation == "" {
			continue
		}

		dependency, err := l.loadDependency(
			file.Path,
			value.SchemaLocation,
		)
		if err != nil {
			return err
		}

		actual := dependency.Schema.TargetNamespace
		expected := file.Schema.TargetNamespace
		if actual != "" && actual != expected {
			return &NamespaceMismatchError{
				Kind:     "include",
				From:     file.Path,
				Location: value.SchemaLocation,
				Expected: expected,
				Actual:   actual,
			}
		}
	}

	return nil
}

func (l *schemaLoader) loadDependency(
	from string,
	location string,
) (*SchemaFile, error) {
	schemaPath, err := l.resolve(from, location)
	if err != nil {
		return nil, err
	}

	return l.load(schemaPath)
}

func (l *schemaLoader) resolve(
	from string,
	location string,
) (string, error) {
	if catalogTarget, found := l.catalog.resolve(location); found {
		return l.resolveLocal(
			path.Dir(l.catalogPath),
			catalogTarget,
			from,
			location,
		)
	}

	parsed, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf(
			"%w: %q from %q: %v",
			ErrInvalidSchemaPath,
			location,
			from,
			err,
		)
	}

	if parsed.IsAbs() || parsed.Host != "" {
		return "", fmt.Errorf(
			"%w: %q from %q",
			ErrUnresolvedSchemaLocation,
			location,
			from,
		)
	}

	return l.resolveLocal(
		path.Dir(from),
		location,
		from,
		location,
	)
}

func (l *schemaLoader) resolveLocal(
	base string,
	reference string,
	from string,
	location string,
) (string, error) {
	parsed, err := url.Parse(reference)
	if err != nil ||
		parsed.IsAbs() ||
		parsed.Host != "" ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		parsed.Path == "" {
		return "", fmt.Errorf(
			"%w: %q from %q",
			ErrInvalidSchemaPath,
			location,
			from,
		)
	}

	schemaPath := path.Clean(path.Join(base, parsed.Path))
	if !fs.ValidPath(schemaPath) || !isWithin(l.basePath, schemaPath) {
		return "", fmt.Errorf(
			"%w: %q from %q resolves to %q outside %q",
			ErrInvalidSchemaPath,
			location,
			from,
			schemaPath,
			l.basePath,
		)
	}

	return schemaPath, nil
}

func validateInputPath(value string) (string, error) {
	if value == "" || value == "." || !fs.ValidPath(value) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSchemaPath, value)
	}

	return value, nil
}

func isWithin(base string, target string) bool {
	if base == "." {
		return true
	}

	return target == base || strings.HasPrefix(target, base+"/")
}
