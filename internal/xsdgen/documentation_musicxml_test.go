package xsdgen

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// documentationWords reduces text to its letters and digits, so that wrapping
// and list markers do not affect a comparison.
func documentationWords(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// localElementDocumentation collects the documentation of every local element
// declaration in value, walking the parsed schema.
func localElementDocumentation(value reflect.Value, index *Index, out *[]string) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			localElementDocumentation(value.Elem(), index, out)
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			localElementDocumentation(value.Index(i), index, out)
		}
	case reflect.Struct:
		if element, ok := value.Addr().Interface().(*Element); ok && element.Name != "" {
			if text := index.documentation(element.Annotation); text != "" {
				*out = append(*out, text)
			}
		}
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).IsExported() {
				localElementDocumentation(value.Field(i), index, out)
			}
		}
	}
}

// TestMusicXMLElementDocumentationIsCopied checks that the documentation of
// every documented local element in musicxml.xsd reaches a generated comment,
// whether the element becomes a field or a choice variant.
func TestMusicXMLElementDocumentationIsCopied(t *testing.T) {
	t.Parallel()

	set, err := Load(os.DirFS(filepath.Join("..", "..", "schema", "musicxml-4.0")), "musicxml.xsd", "catalog.xml")
	require.NoError(t, err)
	index := &Index{documentationNotice: "notice"}
	var entries []string
	for _, file := range set.Files {
		if filepath.Base(file.Path) != "musicxml.xsd" {
			continue
		}
		for i := range file.Schema.ComplexTypes {
			localElementDocumentation(reflect.ValueOf(&file.Schema.ComplexTypes[i]), index, &entries)
		}
		for i := range file.Schema.Groups {
			localElementDocumentation(reflect.ValueOf(&file.Schema.Groups[i]), index, &entries)
		}
	}
	require.Greater(t, len(entries), 150, "musicxml.xsd documents its local elements")

	var generated strings.Builder
	for _, name := range []string{"zz_generated_types.go", "zz_generated_documents.go"} {
		data, err := os.ReadFile(filepath.Join("..", "..", name))
		require.NoError(t, err)
		generated.Write(data)
	}
	text := documentationWords(generated.String())
	missing := 0
	for _, entry := range entries {
		if !strings.Contains(text, documentationWords(entry)) {
			missing++
			t.Errorf("not copied: %.80s", entry)
		}
	}
	assert.Zero(t, missing)
}
