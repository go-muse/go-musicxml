package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const partwiseDOCTYPE = `<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "http://www.musicxml.org/dtds/partwise.dtd">`

func dtdTestScore(doctype, workTitle, extra string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` + doctype +
		`<score-partwise><work><work-title>` + workTitle + `</work-title></work>` +
		`<part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list>` +
		`<part id="P1"><measure number="1">` + extra + `</measure></part></score-partwise>`
}

func TestDecodeExpandsBundledDTDCharacterEntities(t *testing.T) {
	t.Parallel()

	input := dtdTestScore(partwiseDOCTYPE, "Caf&eacute; &Lstrok;&oacute;d&zacute;",
		`<direction><direction-type><words font-family="&Ccedil;a&tab;">x</words></direction-type></direction>`)
	input = strings.Replace(input, "&tab;", "&#9;", 1)
	score, err := DecodeScorePartwise(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, "Café Łódź", *score.Work.WorkTitle)

	// The same references without a resolvable external subset keep the
	// historical unavailable result.
	for _, doctype := range []string{
		`<!DOCTYPE score-partwise SYSTEM "partwise.dtd">`,
		`<!DOCTYPE score-partwise PUBLIC "-//Unknown//DTD Score//EN" "http://example.invalid/score.dtd">`,
		strings.TrimSuffix(partwiseDOCTYPE, ">") + ` [<!ENTITY x "y">]>`,
	} {
		_, err := Decode(strings.NewReader(dtdTestScore(doctype, "Caf&eacute;", "")))
		assert.ErrorIs(t, err, ErrUnsupportedXMLFeature, doctype)
	}

	_, err = Decode(strings.NewReader(dtdTestScore(partwiseDOCTYPE, "&undeclared;", "")))
	assert.Error(t, err)
}

func TestDecodeBindsBundledDTDNamespaceDefaults(t *testing.T) {
	t.Parallel()

	opus := `<opus><score xlink:href="score.musicxml"/></opus>`
	document, err := Decode(strings.NewReader(`<!DOCTYPE opus PUBLIC "-//Recordare//DTD MusicXML 4.0 Opus//EN" "http://www.musicxml.org/dtds/opus.dtd">` + opus))
	require.NoError(t, err)
	value, ok := AsOpusDocument(document)
	require.True(t, ok)
	require.Len(t, value.Content, 1)
	assert.Equal(t, "score.musicxml", value.Content[0].Score.Href)

	// Without a DOCTYPE the prefix is still undeclared.
	_, err = Decode(strings.NewReader(opus))
	var syntax *xml.SyntaxError
	assert.ErrorAs(t, err, &syntax)
}

func TestDecodeDoesNotApplyDTDAttributeDefaults(t *testing.T) {
	t.Parallel()

	input := dtdTestScore(partwiseDOCTYPE, "Title", "")
	plain, err := DecodeScorePartwise(strings.NewReader(input))
	require.NoError(t, err)
	assert.Nil(t, plain.Version, "ordinary decoding reports only physical attributes")
	assert.Equal(t, "1.0", plain.EffectiveVersion())

	// Assessed loading reads the full DTD infoset, as it already did for an
	// internal subset, so the defaulted version reaches its model.
	result, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{})
	require.NoError(t, err)
	strict, ok := AsScorePartwise(result.Document)
	require.True(t, ok)
	require.NotNil(t, strict.Version)
	assert.Equal(t, "1.0", *strict.Version)
}

// TestDecodeExternalDTDKeepsExistingTokens shows that resolving bundled DTDs
// changes nothing for files that ordinary decoding already read: every token of
// the corpus and official files is identical with and without the resolver.
func TestDecodeExternalDTDKeepsExistingTokens(t *testing.T) {
	t.Parallel()

	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		require.NoError(t, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}))
	}
	require.NotEmpty(t, files)
	tokens := func(data []byte, resolver *sourceDTDResolver) ([]xml.Token, error) {
		reader, err := newSourceXMLTokenReader(bytes.NewReader(data), sourceXMLConfig{Context: context.Background(), DTD: resolver})
		if err != nil {
			return nil, err
		}
		var out []xml.Token
		for {
			token, err := reader.Token()
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			if err != nil {
				return out, err
			}
			out = append(out, xml.CopyToken(token))
		}
	}
	compared := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		legacy, legacyErr := tokens(data, nil)
		if legacyErr != nil {
			continue
		}
		resolved, err := tokens(data, newSourceDTDResolver(nil))
		require.NoError(t, err, file)
		assert.True(t, reflect.DeepEqual(legacy, resolved), file)
		compared++
	}
	assert.Greater(t, compared, 140)
}

func TestDecodeCallerDTDSource(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	source := DTDSourceFunc(func(_ context.Context, request DTDRequest) ([]byte, error) {
		calls.Add(1)
		if request.SystemID == "house.dtd" {
			return []byte(`<!ENTITY house "H">`), nil
		}
		return nil, ErrDTDNotFound
	})
	options := DecodeOptions{DTD: &DTDOptions{Source: source}}

	score, err := DecodeScorePartwiseWithOptions(strings.NewReader(dtdTestScore(`<!DOCTYPE score-partwise SYSTEM "house.dtd">`, "&house;", "")), options)
	require.NoError(t, err)
	assert.Equal(t, "H", *score.Work.WorkTitle)

	// A source that declines falls back to the bundled DTDs.
	score, err = DecodeScorePartwiseWithOptions(strings.NewReader(dtdTestScore(partwiseDOCTYPE, "&eacute;", "")), options)
	require.NoError(t, err)
	assert.Equal(t, "é", *score.Work.WorkTitle)
	assert.Positive(t, calls.Load())

	// The DTD is resolved for every document with an external DOCTYPE, but a
	// source error matters only when a character entity needs it.
	failing := DecodeOptions{DTD: &DTDOptions{Source: DTDSourceFunc(func(context.Context, DTDRequest) ([]byte, error) {
		return nil, os.ErrPermission
	})}}
	_, err = DecodeWithOptions(strings.NewReader(dtdTestScore(partwiseDOCTYPE, "plain", "")), failing)
	require.NoError(t, err)
	_, err = DecodeWithOptions(strings.NewReader(dtdTestScore(partwiseDOCTYPE, "&eacute;", "")), failing)
	assert.ErrorIs(t, err, ErrDTDUnavailable)
	assert.ErrorIs(t, err, os.ErrPermission)
	var syntax *xml.SyntaxError
	assert.False(t, errors.As(err, &syntax), "a source failure is not an XML syntax error")
}

func TestAssessedLoadingCallerDTDSource(t *testing.T) {
	t.Parallel()

	source := mapDTDSource{"house.dtd": `<!ENTITY % parent PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "partwise.dtd">%parent;<!ENTITY house "H">`}
	input := dtdTestScore(`<!DOCTYPE score-partwise SYSTEM "house.dtd">`, "&house; &eacute;", "")

	result, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Decode: DecodeOptions{DTD: &DTDOptions{Source: source}}})
	require.NoError(t, err)
	score, ok := AsScorePartwise(result.Document)
	require.True(t, ok)
	assert.Equal(t, "H é", *score.Work.WorkTitle)

	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceSemantics, DTD: &DTDOptions{Source: source}})
	require.NoError(t, err)
	assert.True(t, report.Satisfied())

	// Without the caller source the house DTD stays unread.
	_, err = DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{})
	assert.ErrorIs(t, err, ErrIncompleteAssessment)

	failing := DTDSourceFunc(func(context.Context, DTDRequest) ([]byte, error) { return nil, os.ErrPermission })
	_, err = DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Decode: DecodeOptions{DTD: &DTDOptions{Source: failing}}})
	assert.ErrorIs(t, err, ErrDTDUnavailable)
	assert.ErrorIs(t, err, os.ErrPermission)
}

func TestExternalDTDDeclarationOrigins(t *testing.T) {
	t.Parallel()

	input := dtdTestScore(partwiseDOCTYPE, "Title", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>eighth</type><beam>begin</beam></note>`)
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
	require.NoError(t, err)
	require.True(t, report.Satisfied())

	found := false
	for _, row := range report.Context {
		origin := row.SourceOrigin
		if origin == nil || origin.Attribute == nil || origin.Attribute.Local != "number" || origin.Kind != validation.SourceOriginDefault {
			continue
		}
		found = true
		require.Equal(t, validation.EvidenceKnown, origin.Declaration.State)
		location := origin.Declaration.Value
		assert.Equal(t, "http://www.musicxml.org/dtds/note.mod", location.Entry)
		require.NotNil(t, location.Position)
		assert.Equal(t, validation.CoordinateSpaceExternalDTD, location.Position.CoordinateSpace)
		assert.Positive(t, location.Position.Line)
	}
	assert.True(t, found, "the defaulted beam number carries its external declaration site")
}

func TestExternalDTDIsNeverFetched(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `<!ENTITY eacute "X">`)
	}))
	defer server.Close()

	for _, doctype := range []string{
		`<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "` + server.URL + `/partwise.dtd">`,
		`<!DOCTYPE score-partwise SYSTEM "` + server.URL + `/partwise.dtd">`,
	} {
		input := dtdTestScore(doctype, "&eacute;", "")
		_, _ = Decode(strings.NewReader(input))
		_, _ = DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{})
	}
	assert.Zero(t, requests.Load())
}

func TestDecodeMXLUsesDTDOptions(t *testing.T) {
	t.Parallel()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, content := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="score.musicxml" media-type="application/vnd.recordare.musicxml+xml"/></rootfiles></container>`,
		"score.musicxml":         dtdTestScore(`<!DOCTYPE score-partwise SYSTEM "house.dtd">`, "&house;", ""),
	} {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	_, err := DecodeMXL(bytes.NewReader(archive.Bytes()))
	assert.ErrorIs(t, err, ErrUnsupportedXMLFeature)

	options := MXLOptions{DTD: &DTDOptions{Source: mapDTDSource{"house.dtd": `<!ENTITY house "H">`}}}
	document, err := DecodeMXLWithOptions(bytes.NewReader(archive.Bytes()), options)
	require.NoError(t, err)
	score, ok := AsScorePartwise(document)
	require.True(t, ok)
	assert.Equal(t, "H", *score.Work.WorkTitle)
}

func TestDecodeExternalDTDDocumentedLimits(t *testing.T) {
	t.Parallel()

	opus := `<opus><score xlink:href="score.musicxml"/></opus>`
	opusDOCTYPE := `<!DOCTYPE opus PUBLIC "-//Recordare//DTD MusicXML 4.0 Opus//EN" "http://www.musicxml.org/dtds/opus.dtd"`

	// A caller source error surfaces when a namespace binding needs the DTD.
	failing := DecodeOptions{DTD: &DTDOptions{Source: DTDSourceFunc(func(context.Context, DTDRequest) ([]byte, error) {
		return nil, os.ErrPermission
	})}}
	_, err := DecodeWithOptions(strings.NewReader(opusDOCTYPE+">"+opus), failing)
	assert.ErrorIs(t, err, ErrDTDUnavailable)
	assert.ErrorIs(t, err, os.ErrPermission)

	// An internal subset that ordinary decoding does not interpret keeps the
	// historical unavailable result for namespace bindings.
	for _, subset := range []string{` [<!ATTLIST score a CDATA #IMPLIED>]`, ` [<!ENTITY % p "x">%p;]`} {
		_, err = Decode(strings.NewReader(opusDOCTYPE + subset + ">" + opus))
		assert.ErrorIs(t, err, ErrUnsupportedXMLFeature, subset)
	}

	// One caller entity is limited to 16 MiB.
	huge := DecodeOptions{DTD: &DTDOptions{Source: DTDSourceFunc(func(context.Context, DTDRequest) ([]byte, error) {
		return bytes.Repeat([]byte(" "), sourceDTDExternalTextBytes+1), nil
	})}}
	_, err = DecodeWithOptions(strings.NewReader(dtdTestScore(`<!DOCTYPE score-partwise SYSTEM "house.dtd">`, "&house;", "")), huge)
	assert.ErrorIs(t, err, ErrDTDUnavailable)
	assert.ErrorIs(t, err, validation.ErrResourceLimit)
}
