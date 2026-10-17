package musicxml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundledDTDFilesMatchPins(t *testing.T) {
	t.Parallel()

	entries, err := bundledDTDFiles.ReadDir(bundledDTDDirectory)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, entry := range entries {
		data, err := bundledDTDFiles.ReadFile(bundledDTDDirectory + "/" + entry.Name())
		require.NoError(t, err)
		pin, ok := bundledDTDPins[entry.Name()]
		require.True(t, ok, "embedded file %s has no pin", entry.Name())
		sum := sha256.Sum256(data)
		assert.Equal(t, pin.Size, len(data), entry.Name())
		assert.Equal(t, pin.SHA256, hex.EncodeToString(sum[:]), entry.Name())
		seen[entry.Name()] = true
	}
	for name := range bundledDTDPins {
		assert.True(t, seen[name], "pinned file %s is not embedded", name)
	}
}

// TestBundledDTDPublicIDsMatchOfficialCatalog keeps the resolution table equal
// to the official MusicXML catalog shipped beside the XSD files.
func TestBundledDTDPublicIDsMatchOfficialCatalog(t *testing.T) {
	t.Parallel()

	catalog, err := os.ReadFile("schema/musicxml-4.0/catalog.xml")
	require.NoError(t, err)
	want := map[string]string{}
	for _, match := range regexp.MustCompile(`publicId="([^"]+)"\s+uri="([^"]+)"`).FindAllStringSubmatch(string(catalog), -1) {
		if match[2] == "midixml.dtd" {
			continue
		}
		want[match[1]] = match[2]
	}
	require.NotEmpty(t, want)
	assert.Equal(t, want, bundledDTDPublicIDs)
}

func TestBundledDTDResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, publicID, systemID, base, want string
	}{
		{"current public", "-//Recordare//DTD MusicXML 4.0 Partwise//EN", "", "", "partwise.dtd"},
		{"older public", "-//Recordare//DTD MusicXML 1.0 Timewise//EN", "anything.dtd", "", "timewise.dtd"},
		{"public whitespace", "  -//Recordare//DTD  MusicXML 3.1\nOpus//EN ", "", "", "opus.dtd"},
		{"public wins over unknown system", "-//Recordare//DTD MusicXML 2.0 Container//EN", "file:///not-read.dtd", "", "container.dtd"},
		{"official system", "", "http://www.musicxml.org/dtds/sounds.dtd", "", "sounds.dtd"},
		{"https system", "-//Unknown//EN", "https://www.musicxml.org/dtds/partwise.dtd", "", "partwise.dtd"},
		{"relative module", "", "common.mod", "http://www.musicxml.org/dtds/partwise.dtd", "common.mod"},
		{"unknown public and system", "-//Unknown//EN", "partwise.dtd", "", ""},
		{"unbundled official file", "", "http://www.musicxml.org/dtds/midixml.dtd", "", ""},
		{"relative without official base", "", "common.mod", "file:///x/partwise.dtd", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			name, text, ok := bundledDTD(test.publicID, test.systemID, test.base)
			assert.Equal(t, test.want != "", ok)
			assert.Equal(t, test.want, name)
			if ok {
				assert.Equal(t, bundledDTDText(test.want), text)
			}
		})
	}
}

func TestPrepareBundledDTDs(t *testing.T) {
	t.Parallel()

	resolver := newSourceDTDResolver(nil)
	for _, root := range []string{"partwise.dtd", "timewise.dtd", "opus.dtd", "container.dtd", "sounds.dtd"} {
		t.Run(root, func(t *testing.T) {
			t.Parallel()

			prepared, err := resolver.prepare(context.Background(), "", "http://www.musicxml.org/dtds/"+root)
			require.NoError(t, err)
			require.NotNil(t, prepared)
			assert.NotEmpty(t, prepared.declarations)
		})
	}

	prepared, err := resolver.prepare(context.Background(), "-//Recordare//DTD MusicXML 4.0 Partwise//EN", "")
	require.NoError(t, err)
	again, err := resolver.prepare(context.Background(), "-//Recordare//DTD MusicXML 3.0 Partwise//EN", "http://example.invalid/partwise.dtd")
	require.NoError(t, err)
	assert.Same(t, prepared, again, "bundled closures are prepared once")

	attribute := func(element, name string) (sourceDTDDeclaration, bool) {
		for _, d := range prepared.declarations {
			if d.Kind == sourceDTDAttributeDeclaration && d.Element == element && d.Name == name {
				return d, true
			}
		}
		return sourceDTDDeclaration{}, false
	}
	version, ok := attribute("score-partwise", "version")
	require.True(t, ok, "score-partwise version comes from the INCLUDE section")
	assert.Equal(t, sourceDTDDefaultValue, version.DefaultKind)
	assert.Equal(t, "1.0", version.RawValue)
	_, timewise := attribute("score-timewise", "version")
	assert.False(t, timewise, "the IGNORE section is skipped")

	defaultX, ok := attribute("note", "default-x")
	require.True(t, ok, "attribute groups expand through parameter entities")
	assert.Equal(t, "CDATA", defaultX.AttributeType)

	assert.Equal(t, 'é', prepared.characters["eacute"])
	assert.Equal(t, 'Ł', prepared.characters["Lstrok"])
	assert.Equal(t, []xml.Attr{{Name: xml.Name{Space: "xmlns", Local: "xlink"}, Value: "http://www.w3.org/1999/xlink"}}, prepared.namespaces["link"])
	assert.Positive(t, prepared.bytes)
	for _, d := range prepared.declarations {
		assert.NotEqual(t, sourceDTDParameterEntityDeclaration, d.Kind)
		assert.True(t, d.Binding)
	}
}

type mapDTDSource map[string]string

func (m mapDTDSource) OpenDTD(_ context.Context, request DTDRequest) ([]byte, error) {
	key := request.PublicID
	if key == "" {
		key = request.SystemID
	}
	text, ok := m[key]
	if !ok {
		return nil, ErrDTDNotFound
	}
	return []byte(text), nil
}

func TestPrepareCallerDTD(t *testing.T) {
	t.Parallel()

	source := mapDTDSource{
		"custom.dtd": "<?xml version='1.0' encoding='UTF-8'?>\r\n" +
			`<!ENTITY % module SYSTEM "module.ent">` + "\r\n%module;\n" +
			`<!ENTITY % kind "INCLUDE">` +
			`<![ %kind; [ <!ATTLIST score-partwise version CDATA %version-default;> ]]>` +
			`<![ IGNORE [ <!ATTLIST ignored a CDATA "x"> <![ INCLUDE [ ]]> ]]>`,
		"module.ent": `<!ENTITY composer "B">` + `<!ENTITY % version-default "'9.9'">` +
			`<!ENTITY % ns "urn:x"><!ENTITY % attrs "xmlns:x CDATA #FIXED '%ns;'">` +
			`<!ATTLIST part %attrs;><!ATTLIST measure xmlns:y CDATA #FIXED '%ns;'>`,
	}
	resolver := newSourceDTDResolver(&DTDOptions{Source: source})
	prepared, err := resolver.prepare(context.Background(), "", "custom.dtd")
	require.NoError(t, err)
	require.NotNil(t, prepared)

	var version, ignored bool
	for _, d := range prepared.declarations {
		if d.Element == "score-partwise" && d.Name == "version" {
			version = true
			assert.Equal(t, "9.9", d.RawValue)
		}
		ignored = ignored || d.Element == "ignored"
	}
	assert.True(t, version)
	assert.False(t, ignored)
	assert.Equal(t, 'B', prepared.characters["composer"])
	assert.Equal(t, []xml.Attr{{Name: xml.Name{Space: "xmlns", Local: "x"}, Value: "urn:x"}}, prepared.namespaces["part"])
	// XML 4.4.1: a parameter-entity reference is not recognized in an
	// attribute default literal.
	assert.Equal(t, []xml.Attr{{Name: xml.Name{Space: "xmlns", Local: "y"}, Value: "%ns;"}}, prepared.namespaces["measure"])

	// A source that declines the root selects the bundled closure.
	bundled, err := resolver.prepare(context.Background(), "-//Recordare//DTD MusicXML 4.0 Opus//EN", "")
	require.NoError(t, err)
	require.NotNil(t, bundled)

	unknown, err := resolver.prepare(context.Background(), "", "unknown.dtd")
	require.NoError(t, err)
	assert.Nil(t, unknown)
}

func TestPrepareCallerDTDErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"syntax":            `<!ELEMENT broken`,
		"missing module":    `<!ENTITY % m SYSTEM "missing.ent"> %m;`,
		"recursive":         `<!ENTITY % a "<!-- loop --> &#37;a;"> %a;`,
		"bare percent":      `<!ENTITY % a "% b">`,
		"bad encoding":      `<?xml version="1.0" encoding="ISO-8859-1"?><!ENTITY a "b">`,
		"unclosed section":  `<![ INCLUDE [ <!ENTITY a "b">`,
		"undeclared inline": `<!ATTLIST e a %missing; #IMPLIED>`,
	}
	for name, text := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resolver := newSourceDTDResolver(&DTDOptions{Source: mapDTDSource{"x.dtd": text}})
			prepared, err := resolver.prepare(context.Background(), "", "x.dtd")
			assert.Nil(t, prepared)
			assert.ErrorIs(t, err, ErrDTDUnavailable)
			var dtdErr *DTDError
			require.ErrorAs(t, err, &dtdErr)
			assert.Equal(t, "x.dtd", dtdErr.SystemID)
		})
	}

	failing := DTDSourceFunc(func(context.Context, DTDRequest) ([]byte, error) {
		return nil, os.ErrPermission
	})
	_, err := newSourceDTDResolver(&DTDOptions{Source: failing}).prepare(context.Background(), "-//Recordare//DTD MusicXML 4.0 Partwise//EN", "")
	assert.ErrorIs(t, err, ErrDTDUnavailable)
	assert.ErrorIs(t, err, os.ErrPermission)
	assert.False(t, strings.Contains(err.Error(), "syntax"))
}
