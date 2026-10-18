package xsdgen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
)

// ErrNameManifest reports an invalid or incomplete name manifest.
var ErrNameManifest = errors.New("xsdgen: name manifest")

// NameManifest pins the Go names of the exported package-scope types and
// constants that the generator emits: named, anonymous and choice types and
// enumeration constants. Struct fields and methods are scoped to their type,
// and unexported helpers are private to the package; neither is listed.
//
// The manifest is authoritative: a listed component keeps its name even when
// the derivation would produce another one, and no name belongs to two
// components. A component that is not listed gets its derived name, with a
// numeric suffix when that name is taken by any listed or new component, and
// is recorded as new. Every model generation of one Go package shares one
// manifest, so collisions between generations, such as a new constant named
// like an existing type, are resolved in the generator rather than by the
// compiler. A manifest records what generation asks for, so it is not safe
// for concurrent generations.
type NameManifest struct {
	byKey  map[string]string
	byName map[string]string
	added  map[string]string
	used   map[string]bool
}

// NewNameManifest returns an empty manifest.
func NewNameManifest() *NameManifest {
	return &NameManifest{
		byKey:  make(map[string]string),
		byName: make(map[string]string),
		added:  make(map[string]string),
		used:   make(map[string]bool),
	}
}

// ReadNameManifest parses lines of a component key and its Go name separated
// by a tab. It rejects a duplicate key, a name assigned to two keys and an
// unexported or invalid identifier.
func ReadNameManifest(reader io.Reader) (*NameManifest, error) {
	manifest := NewNameManifest()
	scanner := bufio.NewScanner(reader)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSuffix(scanner.Text(), "\r")
		if text == "" {
			continue
		}
		key, name, found := strings.Cut(text, "\t")
		if !found || key == "" || strings.Contains(name, "\t") {
			return nil, fmt.Errorf("%w: line %d: want key<TAB>name", ErrNameManifest, line)
		}
		if !exportedGoIdentifier(name) {
			return nil, fmt.Errorf("%w: line %d: invalid Go name %q", ErrNameManifest, line, name)
		}
		if existing, found := manifest.byKey[key]; found {
			return nil, fmt.Errorf("%w: line %d: %s is listed twice, as %s and %s", ErrNameManifest, line, key, existing, name)
		}
		if existing, found := manifest.byName[name]; found {
			return nil, fmt.Errorf("%w: line %d: %s is assigned to both %s and %s", ErrNameManifest, line, name, existing, key)
		}
		manifest.byKey[key] = name
		manifest.byName[name] = key
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNameManifest, err)
	}
	return manifest, nil
}

// LoadNameManifest reads the manifest file at filePath. A missing file is an
// empty manifest, so that the first generation can create it.
func LoadNameManifest(filePath string) (*NameManifest, error) {
	data, err := os.ReadFile(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return NewNameManifest(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNameManifest, err)
	}
	return ReadNameManifest(bytes.NewReader(data))
}

// Assign returns the Go name of the component identified by key: its listed
// name, or derived, suffixed while taken, for a new component. A nil manifest
// returns derived unchanged.
func (m *NameManifest) Assign(key, derived string) (string, error) {
	if m == nil {
		return derived, nil
	}
	if !exportedGoIdentifier(derived) {
		return "", fmt.Errorf("%w %q", ErrInvalidGoName, derived)
	}
	if name, found := m.byKey[key]; found {
		m.used[key] = true
		return name, nil
	}
	// A name that already ends in a digit takes its suffix after an
	// underscore, so that Flat1 and its second holder Flat1_2 stay distinct
	// from a Flat12.
	separator := ""
	if last := derived[len(derived)-1]; last >= '0' && last <= '9' {
		separator = "_"
	}
	name := derived
	for suffix := 2; ; suffix++ {
		if _, taken := m.byName[name]; !taken {
			break
		}
		name = derived + separator + strconv.Itoa(suffix)
	}
	m.byKey[key] = name
	m.byName[name] = key
	m.added[key] = name
	m.used[key] = true
	return name, nil
}

// External returns the listed name of a type that a generation names as
// external rather than owning it: the component's own entry when another
// generation of the same schema listed it, otherwise the type that exactly one
// other schema lists under the same local name. derived only describes a
// failure.
func (m *NameManifest) External(declaration *Declaration, derived string) (string, error) {
	// The same component emitted by another generation keeps its entry.
	if name, found := m.byKey[componentKey(declaration, "type")]; found {
		return name, nil
	}
	own := path.Base(declarationPath(declaration))
	suffix := " type " + declaration.Name.Local
	var found []string
	for key := range m.byKey {
		file, rest, ok := strings.Cut(key, " ")
		if ok && file != own && " "+rest == suffix {
			found = append(found, key)
		}
	}
	if len(found) != 1 {
		slices.Sort(found)
		return "", fmt.Errorf("%w: external type %s (%s) needs exactly one listed owner in another schema, found %d: %s", ErrNameManifest, declaration.Name.Local, derived, len(found), strings.Join(found, ", "))
	}
	return m.byKey[found[0]], nil
}

// Added returns the components that the manifest did not list, by key.
func (m *NameManifest) Added() map[string]string {
	result := make(map[string]string, len(m.added))
	for key, name := range m.added {
		result[key] = name
	}
	return result
}

// Used returns the sorted keys that generation asked for.
func (m *NameManifest) Used() []string {
	keys := make([]string, 0, len(m.used))
	for key := range m.used {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// Keys returns the sorted keys of the manifest, including added ones.
func (m *NameManifest) Keys() []string {
	keys := make([]string, 0, len(m.byKey))
	for key := range m.byKey {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// Name returns the listed or added name of key.
func (m *NameManifest) Name(key string) (string, bool) {
	name, found := m.byKey[key]
	return name, found
}

// Format renders the manifest sorted by key.
func (m *NameManifest) Format() []byte {
	var result bytes.Buffer
	for _, key := range m.Keys() {
		fmt.Fprintf(&result, "%s\t%s\n", key, m.byKey[key])
	}
	return result.Bytes()
}

// Incomplete reports the components that the manifest did not list, which
// a check must reject until the manifest is updated.
func (m *NameManifest) Incomplete() error {
	if m == nil || len(m.added) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m.added))
	for key := range m.added {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var lines []string
	for _, key := range keys {
		lines = append(lines, key+" as "+m.added[key])
	}
	return fmt.Errorf("%w: %d components are not listed; regenerate with -update-names after reviewing their names:\n%s", ErrNameManifest, len(keys), strings.Join(lines, "\n"))
}

// componentKey identifies a schema component by its file and path, such as
// "musicxml.xsd type yes-no".
func componentKey(declaration *Declaration, kind string) string {
	return path.Base(declarationPath(declaration)) + " " + kind + " " + declaration.Name.Local
}

// enumerationKey identifies one enumeration value of the component owner.
func enumerationKey(owner, value string) string {
	return owner + " enumeration " + strconv.Quote(value)
}
