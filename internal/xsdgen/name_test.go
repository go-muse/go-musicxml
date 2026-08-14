package xsdgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoTypeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   string
		want    string
		wantErr error
	}{
		{value: "above-below", want: "AboveBelow"},
		{value: "css-font-size", want: "CSSFontSize"},
		{value: "midi-16384", want: "MIDI16384"},
		{
			value: "smufl-accidental-glyph-name",
			want:  "SMuFLAccidentalGlyphName",
		},
		{value: "yyyy-mm-dd", want: "YYYYMMDD"},
		{value: "scorePartwise", want: "ScorePartwise"},
		{value: "xlink", want: "XLink"},
		{value: "", wantErr: ErrInvalidGoName},
		{value: "not valid", wantErr: ErrInvalidGoName},
		{value: "_", wantErr: ErrInvalidGoName},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			actual, err := GoTypeName(test.value)

			if test.wantErr != nil {
				assert.Empty(t, actual)
				assert.ErrorIs(t, err, test.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestGoEnumerationName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prefix  string
		value   string
		want    string
		wantErr error
	}{
		{
			name:   "word",
			prefix: "AboveBelow",
			value:  "above",
			want:   "AboveBelowAbove",
		},
		{
			name:   "leading number",
			prefix: "NoteTypeValue",
			value:  "1024th",
			want:   "NoteTypeValue1024th",
		},
		{
			name:   "empty",
			prefix: "FermataShape",
			want:   "FermataShapeEmpty",
		},
		{
			name:   "spaces and hyphen",
			prefix: "MembraneValue",
			value:  "Indo-American tomtom",
			want:   "MembraneValueIndoAmericanTomtom",
		},
		{
			name:    "invalid prefix",
			prefix:  "not valid",
			value:   "above",
			wantErr: ErrInvalidGoName,
		},
		{
			name:    "symbols only",
			prefix:  "Value",
			value:   "---",
			wantErr: ErrInvalidGoName,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := GoEnumerationName(
				test.prefix,
				test.value,
			)

			if test.wantErr != nil {
				assert.Empty(t, actual)
				assert.ErrorIs(t, err, test.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestTypeNames(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="midi-device"/>
	<xs:simpleType name="css-font-size">
		<xs:restriction base="xs:token"/>
	</xs:simpleType>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	names, err := NewTypeNames(index)
	require.NoError(t, err)

	declarations := names.Declarations()
	require.Len(t, declarations, 2)
	assert.Equal(t, "css-font-size", declarations[0].Name.Local)
	assert.Equal(t, "midi-device", declarations[1].Name.Local)

	actual, found := names.Lookup(declarations[0])
	assert.True(t, found)
	assert.Equal(t, "CSSFontSize", actual)

	actual, found = names.Lookup(declarations[1])
	assert.True(t, found)
	assert.Equal(t, "MIDIDevice", actual)
}

func TestTypeNamesCollision(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="foo-bar">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="foo_bar"/>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	names, err := NewTypeNames(index)

	assert.Nil(t, names)
	assert.ErrorIs(t, err, ErrGoNameCollision)

	var collisionErr *GoNameCollisionError
	if assert.ErrorAs(t, err, &collisionErr) {
		assert.Equal(t, "FooBar", collisionErr.Name)
	}
}

func TestTypeNamesOverrides(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="opus"/>
	<xs:complexType name="score"/>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	names, err := NewTypeNamesWithOverrides(
		index,
		TypeNameOverride{
			Name:   QName{Local: "opus"},
			GoName: "OpusDocument",
		},
		TypeNameOverride{
			Name:   QName{Local: "score"},
			GoName: "OpusScore",
		},
	)
	require.NoError(t, err)

	declarations := names.Declarations()
	require.Len(t, declarations, 2)

	actual, found := names.Lookup(declarations[0])
	assert.True(t, found)
	assert.Equal(t, "OpusDocument", actual)

	actual, found = names.Lookup(declarations[1])
	assert.True(t, found)
	assert.Equal(t, "OpusScore", actual)
}

func TestTypeNamesOverrideErrors(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="opus"/>
	<xs:complexType name="score"/>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	tests := []struct {
		name      string
		overrides []TypeNameOverride
		wantErr   error
	}{
		{
			name: "unknown type",
			overrides: []TypeNameOverride{{
				Name:   QName{Local: "missing"},
				GoName: "Missing",
			}},
			wantErr: ErrUnresolvedReference,
		},
		{
			name: "unexported Go name",
			overrides: []TypeNameOverride{{
				Name:   QName{Local: "opus"},
				GoName: "opusDocument",
			}},
			wantErr: ErrInvalidGoNameOverride,
		},
		{
			name: "duplicate type",
			overrides: []TypeNameOverride{
				{
					Name:   QName{Local: "opus"},
					GoName: "OpusDocument",
				},
				{
					Name:   QName{Local: "opus"},
					GoName: "OtherOpus",
				},
			},
			wantErr: ErrInvalidGoNameOverride,
		},
		{
			name: "renamed collision",
			overrides: []TypeNameOverride{
				{
					Name:   QName{Local: "opus"},
					GoName: "OpusScore",
				},
				{
					Name:   QName{Local: "score"},
					GoName: "OpusScore",
				},
			},
			wantErr: ErrGoNameCollision,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			names, err := NewTypeNamesWithOverrides(
				index,
				test.overrides...,
			)

			assert.Nil(t, names)
			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestTypeNamesNilIndex(t *testing.T) {
	t.Parallel()

	names, err := NewTypeNames(nil)

	assert.Nil(t, names)
	assert.ErrorIs(t, err, ErrNilIndex)
}
