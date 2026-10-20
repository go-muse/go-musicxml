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

	report, err := InspectXML(context.Background(), strings.NewReader(input), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceSemantics, DTD: &DTDOptions{Source: source}})
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
	report, err := InspectXML(context.Background(), strings.NewReader(input), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceInternalDTD})
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

// houseDTDArchive is an MXL archive whose score names a caller-owned DTD.
func houseDTDArchive(t *testing.T) []byte {
	t.Helper()
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
	return archive.Bytes()
}

func TestDecodeMXLUsesDTDOptions(t *testing.T) {
	t.Parallel()

	archive := houseDTDArchive(t)
	_, err := DecodeMXL(bytes.NewReader(archive))
	assert.ErrorIs(t, err, ErrUnsupportedXMLFeature)

	options := MXLOptions{DTD: &DTDOptions{Source: mapDTDSource{"house.dtd": `<!ENTITY house "H">`}}}
	document, err := DecodeMXLWithOptions(bytes.NewReader(archive), options)
	require.NoError(t, err)
	score, ok := AsScorePartwise(document)
	require.True(t, ok)
	assert.Equal(t, "H", *score.Work.WorkTitle)
}

func TestMXLAssessmentDTDOptions(t *testing.T) {
	t.Parallel()

	archive := houseDTDArchive(t)
	house := &DTDOptions{Source: mapDTDSource{"house.dtd": `<!ENTITY house "H">`}}
	failing := &DTDOptions{Source: DTDSourceFunc(func(context.Context, DTDRequest) ([]byte, error) { return nil, os.ErrPermission })}
	inspect := func(o MXLAssessmentOptions) (validation.AssessmentBundle, error) {
		o.Graph.Profile = ProfileMXLSourceSemantics
		return inspectMXLWithDictionaries(context.Background(), bytes.NewReader(archive), mxlDictionaryAssessmentOptions{
			Resources:    o.internal(),
			Dictionaries: DictionaryOptions{Selections: mandatoryDictionarySelections(), Source: BundledDictionarySource()},
		})
	}

	_, err := inspect(MXLAssessmentOptions{})
	assert.ErrorIs(t, err, ErrIncompleteAssessment, "the house DTD stays unread without a source")

	// Graph.DTD configures every assessed child; MXL.DTD is the fallback.
	for name, o := range map[string]MXLAssessmentOptions{
		"Graph.DTD": {Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, DTD: house}},
		"MXL.DTD":   {MXL: MXLOptions{DTD: house}},
	} {
		bundle, err := inspect(o)
		require.NoError(t, err, name)
		assert.True(t, bundle.Satisfied(), name)
	}

	_, err = inspect(MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, DTD: failing}, MXL: MXLOptions{DTD: house}})
	assert.ErrorIs(t, err, ErrDTDUnavailable, "Graph.DTD takes precedence over MXL.DTD")
	assert.ErrorIs(t, err, os.ErrPermission)
	assert.NotErrorIs(t, err, validation.ErrInvalidConfiguration, "a DTD source failure is not a resource transfer contract failure")

	// Profiles that do not read DTD declarations leave the external subset
	// unread whatever the options say.
	_, err = InspectMXL(context.Background(), bytes.NewReader(archive), MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileMXLSource, DTD: house}})
	assert.ErrorIs(t, err, ErrIncompleteAssessment)
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

func TestExternalDTDHonoursInternalParameterEntities(t *testing.T) {
	t.Parallel()

	// The internal subset is read first, so its parameter entities win over
	// the external declarations (XML 2.8, 4.2) and select the timewise
	// sections of score.mod: the partwise version default disappears.
	doctype := strings.TrimSuffix(partwiseDOCTYPE, ">") + ` [<!ENTITY % partwise "IGNORE"><!ENTITY % timewise "INCLUDE">]>`
	result, err := DecodeAssessed(context.Background(), strings.NewReader(dtdTestScore(doctype, "Title", "")), DecodeAssessmentOptions{})
	require.NoError(t, err)
	score, ok := AsScorePartwise(result.Document)
	require.True(t, ok)
	assert.Nil(t, score.Version, "the overridden sections declare no partwise default")
	report, err := InspectXML(context.Background(), strings.NewReader(dtdTestScore(doctype, "Title", "")), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceInternalDTD})
	require.NoError(t, err)
	for _, row := range report.Context {
		origin := row.SourceOrigin
		if origin != nil && origin.Attribute != nil && origin.Attribute.Local == "version" {
			assert.NotEqual(t, validation.SourceOriginDefault, origin.Kind, "no partwise version default after the override")
		}
	}

	// Reading the external subset is charged to the document's budget, with
	// or without internal parameter entities.
	for _, d := range []string{partwiseDOCTYPE, doctype} {
		_, err = InspectXML(context.Background(), strings.NewReader(dtdTestScore(d, "Title", "")),
			SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceInternalDTD, Budgets: validation.Budgets{MaxBufferedBytes: 1 << 20}})
		var resource *validation.ResourceError
		require.ErrorAs(t, err, &resource, d)
		assert.Equal(t, validation.BudgetBufferedBytes, resource.Kind, d)
		assert.ErrorIs(t, err, ErrIncompleteAssessment, d)
	}

	// Ordinary decoding does not interpret parameter entities, so a document
	// whose internal subset declares one keeps the historical result.
	opus := `<!DOCTYPE opus PUBLIC "-//Recordare//DTD MusicXML 4.0 Opus//EN" "http://www.musicxml.org/dtds/opus.dtd" ` +
		`[<!ENTITY % link-attributes "xlink:href CDATA #REQUIRED">]><opus><score xlink:href="s.musicxml"/></opus>`
	_, err = Decode(strings.NewReader(opus))
	assert.ErrorIs(t, err, ErrUnsupportedXMLFeature)
}

func TestExternalDTDLongIdentifiers(t *testing.T) {
	t.Parallel()

	long := "house/" + strings.Repeat("d", 5000) + ".dtd"
	source := DTDSourceFunc(func(_ context.Context, request DTDRequest) ([]byte, error) {
		if request.SystemID == long {
			return []byte(`<!ENTITY house "H"><!ATTLIST score-partwise version CDATA "4.0">`), nil
		}
		return nil, ErrDTDNotFound
	})
	input := dtdTestScore(`<!DOCTYPE score-partwise SYSTEM "`+long+`">`, "&house;", "")

	// A declaration site in an entity whose identifier exceeds the reported
	// limit is unknown evidence, which makes the assessment incomplete rather
	// than a configuration or contract failure.
	_, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Decode: DecodeOptions{DTD: &DTDOptions{Source: source}}})
	assert.ErrorIs(t, err, ErrIncompleteAssessment)
	assert.NotErrorIs(t, err, validation.ErrInvalidConfiguration)
	assert.NotErrorIs(t, err, validation.ErrInvalidSession)

	// Ordinary decoding does not resolve identifiers longer than 4096 bytes.
	_, err = DecodeWithOptions(strings.NewReader(input), DecodeOptions{DTD: &DTDOptions{Source: source}})
	assert.ErrorIs(t, err, ErrUnsupportedXMLFeature)
}

func TestExternalDTDTypedNilSource(t *testing.T) {
	t.Parallel()

	var source DTDSourceFunc
	score, err := DecodeScorePartwiseWithOptions(strings.NewReader(dtdTestScore(partwiseDOCTYPE, "&eacute;", "")), DecodeOptions{DTD: &DTDOptions{Source: source}})
	require.NoError(t, err)
	assert.Equal(t, "é", *score.Work.WorkTitle)

	_, err = DecodeAssessed(context.Background(), strings.NewReader(dtdTestScore(partwiseDOCTYPE, "&eacute;", "")),
		DecodeAssessmentOptions{Decode: DecodeOptions{DTD: &DTDOptions{Source: source}}})
	require.NoError(t, err)
}

func TestDecodeStandaloneDocumentIgnoresExternalEntities(t *testing.T) {
	t.Parallel()

	input := strings.Replace(dtdTestScore(partwiseDOCTYPE, "&eacute;", ""), `encoding="UTF-8"?>`, `encoding="UTF-8" standalone="yes"?>`, 1)
	_, err := Decode(strings.NewReader(input))
	assert.ErrorIs(t, err, ErrUnsupportedXMLFeature, "XML 4.1 Entity Declared: a standalone document may not use external declarations")

	input = strings.Replace(dtdTestScore(partwiseDOCTYPE, "Title", ""), `encoding="UTF-8"?>`, `encoding="UTF-8" standalone="no"?>`, 1)
	_, err = Decode(strings.NewReader(strings.Replace(input, "Title", "&eacute;", 1)))
	require.NoError(t, err)
}

func TestDecodeExternalDTDNamespaceEdges(t *testing.T) {
	t.Parallel()

	// A #FIXED namespace default in the external subset applies to an element
	// that omits the attribute, even when an ancestor binds the prefix to a
	// different namespace (XML 3.3.2). This is the one token difference that
	// resolving the bundled DTDs introduces for ordinary decoding.
	opus := `<!DOCTYPE opus PUBLIC "-//Recordare//DTD MusicXML 4.0 Opus//EN" "http://www.musicxml.org/dtds/opus.dtd">` +
		`<opus xmlns:xlink="urn:other"><score xlink:href="score.musicxml"/></opus>`
	document, err := Decode(strings.NewReader(opus))
	require.NoError(t, err)
	value, ok := AsOpusDocument(document)
	require.True(t, ok)
	require.Len(t, value.Content, 1)
	assert.Equal(t, "score.musicxml", value.Content[0].Score.Href)

	// With a resolved external subset, a prefix that neither the document nor
	// the DTD binds is an XML syntax error, as it is without a DOCTYPE.
	unbound := `<words q:a="1">x</words>`
	_, err = Decode(strings.NewReader(dtdTestScore(partwiseDOCTYPE, "Title", `<direction><direction-type>`+unbound+`</direction-type></direction>`)))
	var syntax *xml.SyntaxError
	assert.ErrorAs(t, err, &syntax)
	assert.NotErrorIs(t, err, ErrUnsupportedXMLFeature)

	// An unresolved external subset might bind it, so the historical result
	// remains.
	_, err = Decode(strings.NewReader(dtdTestScore(`<!DOCTYPE score-partwise SYSTEM "partwise.dtd">`, "Title", `<direction><direction-type>`+unbound+`</direction-type></direction>`)))
	assert.ErrorIs(t, err, ErrUnsupportedXMLFeature)
}

func TestAssessmentOptionsDTDIsHonoured(t *testing.T) {
	t.Parallel()

	source := &DTDOptions{Source: mapDTDSource{"house.dtd": `<!ENTITY house "H">`}}
	input := dtdTestScore(`<!DOCTYPE score-partwise SYSTEM "house.dtd">`, "&house;", "")
	read := func() io.Reader { return strings.NewReader(input) }
	dictionaries := DictionaryOptions{Selections: mandatoryDictionarySelections(), Source: BundledDictionarySource()}

	result, err := DecodeAssessed(context.Background(), read(), DecodeAssessmentOptions{Assessment: &SourceOptions{Context: validation.ContextFamilyAll, DTD: source}})
	require.NoError(t, err)
	score, ok := AsScorePartwise(result.Document)
	require.True(t, ok)
	assert.Equal(t, "H", *score.Work.WorkTitle)

	result, err = decodeAssessedWithDictionaries(context.Background(), read(), decodeDictionaryTestOptions{
		Assessment: &dictionaryAssessmentOptions{Assessment: assessmentOptions{DTD: source}, Dictionaries: dictionaries},
	})
	require.NoError(t, err)
	score, ok = AsScorePartwise(result.Document)
	require.True(t, ok)
	assert.Equal(t, "H", *score.Work.WorkTitle)

	report, err := inspectXMLWithDictionaries(context.Background(), read(), dictionaryAssessmentOptions{
		Assessment:   assessmentOptions{Profile: ProfileSourceSemantics.ID(), DTD: source},
		Dictionaries: dictionaries,
	})
	require.NoError(t, err)
	assert.True(t, report.Satisfied())

	// DecodeOptions.DTD takes precedence over SourceOptions.DTD.
	decline := &DTDOptions{Source: DTDSourceFunc(func(context.Context, DTDRequest) ([]byte, error) { return nil, os.ErrPermission })}
	_, err = DecodeAssessed(context.Background(), read(), DecodeAssessmentOptions{Decode: DecodeOptions{DTD: decline}, Assessment: &SourceOptions{Context: validation.ContextFamilyAll, DTD: source}})
	assert.ErrorIs(t, err, os.ErrPermission)
}

// TestAssessedLoadingCorpusMeasurement backs the documented measurement of
// default assessed loading over the official files and the MusicXML test
// suite. The accepted count is a floor that may rise; no rejection may come
// from an unread external DTD.
func TestAssessedLoadingCorpusMeasurement(t *testing.T) {
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
	require.Len(t, files, 154, "the documented denominator")
	accepted := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		result, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{})
		if err == nil {
			accepted++
			continue
		}
		assert.NotErrorIs(t, err, ErrUnsupportedXMLFeature, file)
		assert.NotErrorIs(t, err, ErrDTDUnavailable, file)
		for _, f := range result.SourceLine.Findings {
			assert.NotContains(t, string(f.Reason), "dtd-infoset-unavailable", file)
		}
	}
	assert.GreaterOrEqual(t, accepted, 140)
}
