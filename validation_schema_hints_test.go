package musicxml

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This matrix checks name permissibility, not the deferred value semantics of
// schema-instance hints. The same original bytes are supplied to xmllint by
// TestElementAttributeContractsAgainstSchema.
func validationSchemaHintCases() []validationAttributeCase {
	const opus = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="score.musicxml"/></opus>`
	const timewise = `<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><measure number="1"><part id="P1"/></measure></score-timewise>`
	targets := []struct{ name, source, marker, schema string }{
		{"partwise root", validationAttributeScore, "<score-partwise ", ""},
		{"complex content", validationAttributeScore, "<measure ", ""},
		{"complex simple content", validationAttributeScore, "<type ", ""},
		{"named simple", validationAttributeScore, "<step", ""},
		{"builtin simple", validationAttributeScore, "<staves", ""},
		{"timewise root", timewise, "<score-timewise ", ""},
		{"timewise measure", timewise, "<measure ", ""},
		{"opus root", opus, "<opus ", "opus.xsd"},
		{"opus score", opus, "<score ", "opus.xsd"},
	}
	var tests []validationAttributeCase
	for _, target := range targets {
		for _, hint := range []struct{ name, value string }{
			{"schemaLocation", "urn:example https://example.invalid/schema.xsd urn:other other.xsd"},
			{"noNamespaceSchemaLocation", "schemas/musicxml.xsd"},
		} {
			for _, spelling := range []struct{ name, attributes string }{
				{"xsi prefix", ` xmlns:xsi="` + validationXSINamespace + `" xsi:` + hint.name + `="` + hint.value + `"`},
				{"alias after attribute", ` other:` + hint.name + `="` + hint.value + `" xmlns:other="` + validationXSINamespace + `"`},
			} {
				tests = append(tests, validationAttributeCase{
					name: target.name + " " + hint.name + " " + spelling.name,
					source: strings.Replace(target.source, target.marker,
						strings.TrimSuffix(target.marker, " ")+spelling.attributes+" ", 1),
					schema: target.schema,
				})
			}
		}
	}
	for _, test := range []struct{ name, extra, constraint, path string }{
		{"both hints", "", "", ""},
		{"unknown xsi sibling", ` i:unrecognized="x"`, "attribute", "/@{" + validationXSINamespace + "}unrecognized"},
		{"unqualified no-namespace lookalike", ` noNamespaceSchemaLocation="schema.xsd"`, "attribute", "/@noNamespaceSchemaLocation"},
		{"foreign paired-hint lookalike", ` xmlns:p="urn:foreign" p:schemaLocation="urn:example schema.xsd"`, "attribute", "/@{urn:foreign}schemaLocation"},
		{"unqualified lookalike", ` schemaLocation="schema.xsd"`, "attribute", "/@schemaLocation"},
		{"foreign lookalike", ` xmlns:p="urn:foreign" p:noNamespaceSchemaLocation="schema.xsd"`, "attribute", "/@{urn:foreign}noNamespaceSchemaLocation"},
		{"case sensitive name", ` i:SchemaLocation="schema.xsd"`, "attribute", "/@{" + validationXSINamespace + "}SchemaLocation"},
		{"namespace declaration lookalike", ` xmlns:p="xmlns" p:schemaLocation="schema.xsd"`, "attribute", "/@{xmlns}schemaLocation"},
		{"ordinary datatype", ` width="bad"`, "datatype", "/@width"},
	} {
		tests = append(tests, validationAttributeCase{
			name:       "hint interaction " + test.name,
			source:     strings.Replace(validationAttributeScore, "<measure ", `<measure xmlns:i="`+validationXSINamespace+`" i:schemaLocation="urn:example schema.xsd" i:noNamespaceSchemaLocation="musicxml.xsd"`+test.extra+" ", 1),
			constraint: test.constraint, path: "/score-partwise/part/measure" + test.path,
		})
	}
	for _, test := range []struct{ name, old, replacement, constraint, path string }{
		{"invalid ordinary enum", `implicit="yes"`, `implicit="maybe"`, "enumeration", "/score-partwise/part/measure/@implicit"},
		{"missing ordinary required attribute", ` number="1"`, "", "required", "/score-partwise/part/measure/@number"},
		{"invalid ordinary content", "<step>C</step>", "<step>H</step>", "enumeration", "/score-partwise/part/measure/note/pitch/step"},
	} {
		source := strings.Replace(validationAttributeScore, "<score-partwise ", `<score-partwise xmlns:i="`+validationXSINamespace+`" i:noNamespaceSchemaLocation="https://example.invalid/replacement.xsd" `, 1)
		tests = append(tests, validationAttributeCase{
			name: "hint interaction " + test.name, source: strings.Replace(source, test.old, test.replacement, 1),
			constraint: test.constraint, path: test.path,
		})
	}
	for _, value := range []string{"simple", "extended"} {
		constraint := ""
		if value == "extended" {
			constraint = "fixed"
		}
		tests = append(tests, validationAttributeCase{
			name:   "hint interaction fixed " + value,
			source: `<opus xmlns:i="` + validationXSINamespace + `" xmlns:xlink="http://www.w3.org/1999/xlink"><score i:noNamespaceSchemaLocation="opus.xsd" xlink:href="score.musicxml" xlink:type="` + value + `"/></opus>`,
			schema: "opus.xsd", constraint: constraint, path: "/opus/score/@{http://www.w3.org/1999/xlink}type",
		})
	}
	// The alias is inherited, rebound to a foreign namespace for one child, and
	// restored for its sibling. Recognition must use the expanded name.
	tests = append(tests, validationAttributeCase{
		name:   "hint prefix rebinding and restoration",
		source: `<opus xmlns:i="` + validationXSINamespace + `" xmlns:xlink="http://www.w3.org/1999/xlink" i:noNamespaceSchemaLocation="opus.xsd"><score xmlns:i="urn:foreign" i:schemaLocation="urn:example schema.xsd" xlink:href="first.musicxml"/><score i:schemaLocation="urn:example schema.xsd" xlink:href="second.musicxml"/></opus>`,
		schema: "opus.xsd", constraint: "attribute", path: "/opus/score/@{urn:foreign}schemaLocation",
	})
	return tests
}

func TestSchemaLocationHintsDoNotFetch(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		// A permissive replacement would hide the ordinary-attribute failure.
		_, _ = w.Write([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="opus" type="xs:anyType"/></xs:schema>`))
	}))
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/control")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.EqualValues(t, 1, requests.Load(), "HTTP request counter must be live")
	requests.Store(0)

	for _, extra := range []string{"", ` rubbish="x"`} {
		source := `<opus xmlns:i="` + validationXSINamespace + `" i:schemaLocation="urn:example ` + server.URL + `/namespaced.xsd" i:noNamespaceSchemaLocation="` + server.URL + `/opus.xsd"` + extra + `/>`
		context := validateAttributeSource(t, &opusValidationSchema, source)
		if extra == "" {
			assert.Empty(t, context.issues)
		} else {
			require.Len(t, context.issues, 1)
			assert.Equal(t, "attribute", context.issues[0].Constraint)
			assert.Equal(t, "/opus/@rubbish", context.issues[0].Path)
		}
	}
	assert.Zero(t, requests.Load(), "schema-location hints must not trigger HTTP requests")
}

func TestSchemaLocationHintsPreservePublicModelValidation(t *testing.T) {
	t.Parallel()
	// Decode still drops unsupported source attributes. Neither accepting hint
	// names internally nor public Validate adds a strict-source validation API.
	source := `<opus xmlns:i="` + validationXSINamespace + `" i:noNamespaceSchemaLocation="https://example.invalid/opus.xsd" rubbish="x"/>`
	document, err := Decode(strings.NewReader(source))
	require.NoError(t, err)
	assert.NoError(t, Validate(document))
	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, document))
	assert.NotContains(t, encoded.String(), "SchemaLocation")
	assert.NotContains(t, encoded.String(), "rubbish")
}
