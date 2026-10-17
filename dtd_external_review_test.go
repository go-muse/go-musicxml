package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// houseDTDInput is a score whose DOCTYPE names the caller DTD house.dtd, with
// an optional internal subset.
func houseDTDInput(internal, workTitle string) string {
	doctype := `<!DOCTYPE score-partwise SYSTEM "house.dtd"`
	if internal != "" {
		doctype += " [" + internal + "]"
	}
	return dtdTestScore(doctype+">", workTitle, "")
}

func inspectHouseDTD(dtd, internal, workTitle string, budgets validation.Budgets) (validation.Report, error) {
	return InspectXML(context.Background(), strings.NewReader(houseDTDInput(internal, workTitle)),
		AssessmentOptions{Profile: ProfileSourceInternalDTD, Budgets: budgets, DTD: &DTDOptions{Source: mapDTDSource{"house.dtd": dtd}}})
}

func requireResourceKind(t *testing.T, err error, kind validation.BudgetKind, msg string) {
	t.Helper()
	var resource *validation.ResourceError
	require.ErrorAs(t, err, &resource, msg)
	assert.Equal(t, kind, resource.Kind, msg)
	assert.ErrorIs(t, err, ErrIncompleteAssessment, msg)
}

func TestExternalDTDHonoursAssessmentBudgets(t *testing.T) {
	t.Parallel()

	// A caller DTD is parsed into the document's store under its account,
	// with or without internal parameter entities.
	big := `<!ENTITY big "` + strings.Repeat("x", 1000) + `">`
	for _, internal := range []string{"", `<!ENTITY % local "x">`} {
		report, err := inspectHouseDTD(big, internal, "T", validation.Budgets{})
		require.NoError(t, err, internal)
		assert.True(t, report.Satisfied(), internal)
		_, err = inspectHouseDTD(big, internal, "T", validation.Budgets{MaxScalarBytes: 128})
		requireResourceKind(t, err, validation.BudgetScalarBytes, "unused 1000-byte entity value under MaxScalarBytes 128: "+internal)
	}

	longName := `<!ENTITY ` + strings.Repeat("n", 200) + ` "x">`
	_, err := inspectHouseDTD(longName, "", "T", validation.Budgets{MaxScalarBytes: 128})
	requireResourceKind(t, err, validation.BudgetScalarBytes, "200-byte entity name under MaxScalarBytes 128")

	model := `<!ELEMENT a ` + strings.Repeat("(", 10) + "b" + strings.Repeat(")", 10) + `><!ENTITY x "X">`
	_, err = inspectHouseDTD(model, "", "&x;", validation.Budgets{MaxDepth: 12})
	require.NoError(t, err)
	_, err = inspectHouseDTD(model, "", "&x;", validation.Budgets{MaxDepth: 8})
	requireResourceKind(t, err, validation.BudgetDepth, "content model nested ten deep under MaxDepth 8")

	nested := strings.Repeat("<![INCLUDE[", 10) + `<!ENTITY x "X">` + strings.Repeat("]]>", 10)
	for _, internal := range []string{"", `<!ENTITY % local "x">`} {
		_, err := inspectHouseDTD(nested, internal, "&x;", validation.Budgets{MaxDepth: 12})
		require.NoError(t, err, internal)
		_, err = inspectHouseDTD(nested, internal, "&x;", validation.Budgets{MaxDepth: 8})
		requireResourceKind(t, err, validation.BudgetDepth, "ten nested sections under MaxDepth 8: "+internal)
	}

	// The bundled closure is prepared once, but importing it still honours
	// the document's scalar limit; parsing it for internal parameter entities
	// is charged to the document directly.
	prepared, err := newSourceDTDResolver(nil).prepare(context.Background(), "-//Recordare//DTD MusicXML 4.0 Partwise//EN", "")
	require.NoError(t, err)
	// Without the closure the document needs fewer than 128 scalar bytes.
	require.Greater(t, prepared.longest, int64(128))
	for _, internal := range []string{"", ` [<!ENTITY % local "x">]`} {
		input := dtdTestScore(strings.TrimSuffix(partwiseDOCTYPE, ">")+internal+">", "T", "")
		_, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD, Budgets: validation.Budgets{MaxScalarBytes: 128}})
		requireResourceKind(t, err, validation.BudgetScalarBytes, "bundled closure under MaxScalarBytes 128: "+internal)
	}
	// Importing needs exactly the scalar limit that parsing for internal
	// parameter entities needs: the longest string, a PE literal included.
	for _, internal := range []string{"", ` [<!ENTITY % local "x">]`} {
		input := dtdTestScore(strings.TrimSuffix(partwiseDOCTYPE, ">")+internal+">", "T", "")
		_, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD, Budgets: validation.Budgets{MaxScalarBytes: prepared.longest}})
		require.NoError(t, err, internal)
		_, err = InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD, Budgets: validation.Budgets{MaxScalarBytes: prepared.longest - 1}})
		requireResourceKind(t, err, validation.BudgetScalarBytes, "one byte below the longest closure string: "+internal)
	}

	// The partwise closure nests seven deep; the document itself only three.
	require.Equal(t, int64(7), prepared.depth)
	for limit, fails := range map[int64]bool{6: true, 7: false} {
		_, err := InspectXML(context.Background(), strings.NewReader(dtdTestScore(partwiseDOCTYPE, "T", "")), AssessmentOptions{Profile: ProfileSourceInternalDTD, Budgets: validation.Budgets{MaxDepth: limit}})
		if fails {
			requireResourceKind(t, err, validation.BudgetDepth, "bundled closure under MaxDepth 6")
		} else {
			assert.NoError(t, err)
		}
	}

	// Parameter-entity work in a caller DTD is charged to the document's
	// events, even when it declares nothing.
	blank := `<!ENTITY % blank "` + strings.Repeat(" ", 5000) + `"><!ENTITY x "X">`
	_, err = inspectHouseDTD(blank, "", "&x;", validation.Budgets{MaxEvents: 50_000})
	require.NoError(t, err)
	_, err = inspectHouseDTD(blank+strings.Repeat("%blank;", 20), "", "&x;", validation.Budgets{MaxEvents: 50_000})
	requireResourceKind(t, err, validation.BudgetEvents, "100,000 bytes of parameter-entity text under MaxEvents 50,000")

	// The depth reached while preparing a closure is checked when importing it.
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	require.NoError(t, err)
	store, err := newSourceDTDStore(context.Background(), account, sourceDTDStandaloneAbsent)
	require.NoError(t, err)
	defer store.Close()
	deep := &sourceDTDPrepared{depth: 9}
	err = store.importExternalSubset(deep, 8)
	var resource *validation.ResourceError
	require.ErrorAs(t, err, &resource)
	assert.Equal(t, validation.BudgetDepth, resource.Kind)
	assert.NoError(t, store.importExternalSubset(&sourceDTDPrepared{depth: 8}, 8))
}

func TestExternalDTDDefaultReferenceOrder(t *testing.T) {
	t.Parallel()

	// XML 4.1: a general entity must be declared before a direct reference in
	// an attribute default. The external defect is a DTD error.
	forward := `<!ATTLIST score-partwise version CDATA "&v;"><!ENTITY v "4.0">`
	_, err := inspectHouseDTD(forward, "", "T", validation.Budgets{})
	assert.ErrorIs(t, err, ErrDTDUnavailable)
	var syntax *xml.SyntaxError
	assert.False(t, errors.As(err, &syntax), "an external DTD defect is not a document syntax error")

	// The internal subset comes first (XML 2.8), so its binding precedes the
	// external default.
	report, err := inspectHouseDTD(forward, `<!ENTITY v "4.0">`, "T", validation.Budgets{})
	require.NoError(t, err)
	assert.True(t, report.Satisfied())

	// An internal default cannot use an entity that only the later external
	// subset declares; that defect is in the document.
	_, err = inspectHouseDTD(`<!ENTITY v "4.0">`, `<!ATTLIST score-partwise version CDATA "&v;">`, "T", validation.Budgets{})
	assert.ErrorAs(t, err, &syntax)
	assert.NotErrorIs(t, err, ErrDTDUnavailable)

	// Unparsed and external entities are never allowed in a default.
	for _, dtd := range []string{
		`<!NOTATION n SYSTEM "n"><!ENTITY u SYSTEM "u" NDATA n><!ATTLIST score-partwise version CDATA "&u;">`,
		`<!ENTITY e SYSTEM "e.ent"><!ATTLIST score-partwise version CDATA "&e;">`,
	} {
		_, err = inspectHouseDTD(dtd, "", "T", validation.Budgets{})
		assert.ErrorIs(t, err, ErrDTDUnavailable, dtd)
	}

	// The same order check covers the imported bundled closure: eacute is
	// declared by isolat1.ent, after the internal subset.
	input := dtdTestScore(strings.TrimSuffix(partwiseDOCTYPE, ">")+` [<!ATTLIST score-partwise version CDATA "&eacute;">]>`, "T", "")
	_, err = InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
	assert.ErrorAs(t, err, &syntax)
}

func TestExternalDTDIgnoredDefaultsAreChecked(t *testing.T) {
	t.Parallel()

	// A duplicate definition does not bind, but its default is still an
	// attribute-literal context with its own WFCs.
	for name, dtd := range map[string]string{
		"external entity": `<!ATTLIST score-partwise version CDATA "4.0"><!ENTITY bad SYSTEM "other.ent"><!ATTLIST score-partwise version CDATA "&bad;">`,
		"forward entity":  `<!ATTLIST score-partwise version CDATA "4.0"><!ATTLIST score-partwise version CDATA "&v;"><!ENTITY v "4.0">`,
		"less-than":       `<!ENTITY lt2 "&#60;"><!ATTLIST score-partwise version CDATA "4.0"><!ATTLIST score-partwise version CDATA "&lt2;">`,
	} {
		_, err := inspectHouseDTD(dtd, "", "T", validation.Budgets{})
		assert.ErrorIs(t, err, ErrDTDUnavailable, name)

		_, err = newSourceDTDResolver(&DTDOptions{Source: mapDTDSource{"house.dtd": dtd}}).prepare(context.Background(), "", "house.dtd")
		if name != "less-than" {
			assert.ErrorIs(t, err, ErrDTDUnavailable, "prepared: "+name)
		}
	}

	// Prepared closures keep every read definition in document order.
	prepared, err := newSourceDTDResolver(&DTDOptions{Source: mapDTDSource{"house.dtd": `<!ATTLIST a b CDATA "1"><!ATTLIST a b CDATA "2">`}}).prepare(context.Background(), "", "house.dtd")
	require.NoError(t, err)
	require.Len(t, prepared.declarations, 2)
	assert.True(t, prepared.declarations[0].Binding)
	assert.False(t, prepared.declarations[1].Binding)
	assert.Equal(t, "2", prepared.declarations[1].RawValue)
}

func TestExternalDTDDefaultDefectIsDTDError(t *testing.T) {
	t.Parallel()

	_, err := inspectHouseDTD(`<!ENTITY bad "&#60;"><!ATTLIST score-partwise version CDATA "&bad;">`, "", "T", validation.Budgets{})
	assert.ErrorIs(t, err, ErrDTDUnavailable)
	var syntax *xml.SyntaxError
	assert.False(t, errors.As(err, &syntax), "a deferred external default check is not a document syntax error")
	assert.Contains(t, err.Error(), "house.dtd")

	// The same defect in the internal subset stays a document error.
	_, err = inspectHouseDTD(``, `<!ENTITY bad "&#60;"><!ATTLIST score-partwise version CDATA "&bad;">`, "T", validation.Budgets{})
	assert.ErrorAs(t, err, &syntax)
}

func TestExternalDTDTextDeclarationProductions(t *testing.T) {
	t.Parallel()

	prepare := func(header string) error {
		_, err := newSourceDTDResolver(&DTDOptions{Source: mapDTDSource{"x.dtd": header + `<!ENTITY a "b">`}}).prepare(context.Background(), "", "x.dtd")
		return err
	}
	for _, header := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<?xml encoding='utf-8' ?>`,
		"<?xml\tversion='1.10'\nencoding=\"UTF-8\"?>",
	} {
		assert.NoError(t, prepare(header), header)
	}
	for _, header := range []string{
		`<?xml version="1.foo" encoding="UTF-8"?>`,
		`<?xml version="1." encoding="UTF-8"?>`,
		`<?xml version="" encoding="UTF-8"?>`,
		`<?xml version="2.0" encoding="UTF-8"?>`,
		`<?xml encoding=" UTF-8 "?>`,
		`<?xml encoding="UTF-8 "?>`,
		`<?xml encoding="8UTF"?>`,
		`<?xml encoding=""?>`,
		`<?xml version="1.0"?>`,
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`,
		`<?xml encoding="UTF-8" version="1.0"?>`,
	} {
		err := prepare(header)
		assert.ErrorIs(t, err, ErrDTDUnavailable, header)
		var syntax *xml.SyntaxError
		assert.False(t, errors.As(err, &syntax), header)
	}
}

func TestExternalDTDParameterCompletenessAfterExternalRead(t *testing.T) {
	t.Parallel()

	// The internal PE is consumed by the external subset in a declaration
	// context, so its replacement has been processed.
	dtd := `<!ENTITY % choice "INCLUDE"><![%selected;[<!ENTITY x "X">]]>`
	report, err := inspectHouseDTD(dtd, `<!ENTITY % selected "&#37;choice;">`, "&x;", validation.Budgets{})
	require.NoError(t, err)
	assert.Equal(t, validation.SyntaxWellFormed, report.Syntax)
	assert.True(t, report.Satisfied())

	// Literal inclusion processes it as well.
	dtd = `<!ENTITY % choice "Y"><!ENTITY x "%selected;">`
	report, err = inspectHouseDTD(dtd, `<!ENTITY % selected "&#37;choice;">`, "&x;", validation.Budgets{})
	require.NoError(t, err)
	assert.Equal(t, validation.SyntaxWellFormed, report.Syntax)
	assert.True(t, report.Satisfied())

	// A replacement that nothing processes keeps its uncertainty.
	report, err = inspectHouseDTD(`<!ENTITY x "X">`, `<!ENTITY % unused "&#37;nothing;">`, "&x;", validation.Budgets{})
	require.NoError(t, err)
	assert.Equal(t, validation.SyntaxIncomplete, report.Syntax)
}

func TestExternalDTDDeclarationSitesThroughParameterEntities(t *testing.T) {
	t.Parallel()

	// note's default-x comes from %print-style; in note.mod, whose
	// replacement includes the %position; literal of common.mod.
	input := dtdTestScore(partwiseDOCTYPE, "Title", `<note default-x="1"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type>quarter</type></note>`)
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
	require.NoError(t, err)
	require.True(t, report.Satisfied())
	found := false
	for _, row := range report.Context {
		origin := row.SourceOrigin
		if origin == nil || origin.Attribute == nil || origin.Attribute.Local != "default-x" {
			continue
		}
		found = true
		require.Equal(t, validation.EvidenceKnown, origin.Declaration.State)
		location := origin.Declaration.Value
		assert.Equal(t, "http://www.musicxml.org/dtds/common.mod", location.Entry)
		require.NotNil(t, location.Position)
		want := bundledDTDPosition(t, "common.mod", "<!ENTITY % position", "default-x")
		assert.Equal(t, want.ByteOffset, location.Position.ByteOffset)
		assert.Equal(t, int64(want.Line), int64(location.Position.Line))
		assert.Equal(t, int64(want.Column), int64(location.Position.Column))
	}
	assert.True(t, found)

	// Text from an internal-subset parameter entity has no site in an
	// external entity, so its declarations are unknown evidence.
	report, err = inspectHouseDTD(`%local;`, `<!ENTITY % local "<!ATTLIST score-partwise local CDATA 'x'>">`, "T", validation.Budgets{})
	require.NoError(t, err)
	assert.False(t, report.Satisfied())
	found = false
	for _, row := range report.Context {
		if origin := row.SourceOrigin; origin != nil && origin.Attribute != nil && origin.Attribute.Local == "local" {
			found = true
			assert.Equal(t, validation.EvidenceUnknown, origin.Declaration.State)
		}
	}
	assert.True(t, found)

	// Every published site of the prepared closures is where the declaration
	// text physically starts in its entity.
	for _, publicID := range []string{"-//Recordare//DTD MusicXML 4.0 Partwise//EN", "-//Recordare//DTD MusicXML 4.0 Timewise//EN", "-//Recordare//DTD MusicXML 4.0 Opus//EN"} {
		prepared, err := newSourceDTDResolver(nil).prepare(context.Background(), publicID, "")
		require.NoError(t, err)
		sites := 0
		for _, d := range prepared.declarations {
			if !d.Origin.SpanAvailable {
				continue
			}
			sites++
			file := strings.TrimPrefix(d.Origin.Entity, bundledDTDSystemPrefixes[0])
			text, err := bundledDTDEntityText(file)
			require.NoError(t, err)
			start := d.Origin.Span.Start
			require.Less(t, int(start.ByteOffset), len(text))
			at := text[start.ByteOffset:]
			if d.Kind == sourceDTDAttributeDeclaration {
				assert.True(t, strings.HasPrefix(at, d.Name), "%s %s %s at %s:%d", publicID, d.Element, d.Name, file, start.Line)
			} else {
				assert.True(t, strings.HasPrefix(at, "<!"), "%s %s at %s:%d", d.Kind, d.Name, file, start.Line)
			}
			before := text[:start.ByteOffset]
			assert.Equal(t, 1+strings.Count(before, "\n"), start.Line, d.Name)
			assert.Equal(t, 1+utf8.RuneCountInString(before[strings.LastIndex(before, "\n")+1:]), start.Column, d.Name)
			assert.GreaterOrEqual(t, d.Origin.Span.End.ByteOffset, start.ByteOffset, d.Name)
		}
		assert.Greater(t, sites, 100, publicID)
	}
}

func TestDecodeNormalizesExternalNamespaceDefaults(t *testing.T) {
	t.Parallel()

	opus := `<!DOCTYPE opus SYSTEM "house.dtd"><opus><score xlink:href="score.musicxml"/></opus>`
	for name, dtd := range map[string]string{
		"character reference":       `<!ATTLIST score xmlns:xlink CDATA #FIXED "http://www.w3.org/1999/xlin&#107;">`,
		"internal entity":           `<!ENTITY xl "http://www.w3.org/1999/xlink"><!ATTLIST score xmlns:xlink CDATA "&xl;">`,
		"nested entities":           `<!ENTITY w3 "http://www.w3.org"><!ENTITY xl "&w3;/1999/xlink"><!ATTLIST score xmlns:xlink CDATA "&xl;">`,
		"tokenized declared type":   `<!ATTLIST score xmlns:xlink NMTOKEN " http://www.w3.org/1999/xlink ">`,
		"predefined entity in name": `<!ATTLIST score xmlns:xlink CDATA "http://www.w3.org/1999/xlink"><!ATTLIST score xmlns:other CDATA "urn:a&amp;b">`,
	} {
		document, err := DecodeWithOptions(strings.NewReader(opus), DecodeOptions{DTD: &DTDOptions{Source: mapDTDSource{"house.dtd": dtd}}})
		require.NoError(t, err, name)
		value, ok := AsOpusDocument(document)
		require.True(t, ok, name)
		require.Len(t, value.Content, 1, name)
		assert.Equal(t, "score.musicxml", value.Content[0].Score.Href, name)
	}

	prepared, err := newSourceDTDResolver(&DTDOptions{Source: mapDTDSource{"x.dtd": `<!ATTLIST score xmlns:other CDATA "urn:a&amp;b&#x9;c d">`}}).prepare(context.Background(), "", "x.dtd")
	require.NoError(t, err)
	require.Len(t, prepared.namespaces["score"], 1)
	assert.Equal(t, "urn:a&b\tc d", prepared.namespaces["score"][0].Value, "references are normalized; a referenced tab stays a tab (XML 3.3.3)")

	// A default that is not a well-formed attribute value is not applied.
	for _, dtd := range []string{
		`<!ENTITY lt2 "&#60;"><!ATTLIST score xmlns:xlink CDATA "&lt2;">`,
		`<!ENTITY ext SYSTEM "x.ent"><!ATTLIST score xmlns:xlink CDATA "&ext;">`,
		`<!ATTLIST score xmlns:xlink CDATA "&undeclared;">`,
	} {
		prepared, err := newSourceDTDResolver(&DTDOptions{Source: mapDTDSource{"x.dtd": dtd}}).prepare(context.Background(), "", "x.dtd")
		if err == nil {
			assert.Empty(t, prepared.namespaces["score"], dtd)
		}
	}
}

func TestExternalDTDRelativeBaseIsTheParsingEntity(t *testing.T) {
	t.Parallel()

	// XML 4.2.2: a relative identifier is relative to the external entity
	// containing the < that starts the declaration when it is parsed. Here
	// the declaration of inner is written in b/mods.ent but parsed as the
	// replacement of %decl; while a/main.dtd is read.
	var requests []DTDRequest
	source := DTDSourceFunc(func(_ context.Context, request DTDRequest) ([]byte, error) {
		requests = append(requests, request)
		switch request.SystemID + " " + request.BaseSystemID {
		case "a/main.dtd ":
			return []byte(`<!ENTITY % mods SYSTEM "../b/mods.ent"> %mods; %decl; %inner;`), nil
		case "../b/mods.ent a/main.dtd":
			return []byte(`<!ENTITY % decl "<!ENTITY &#37; inner SYSTEM 'inner.mod'>">`), nil
		case "inner.mod a/main.dtd":
			return []byte(`<!ENTITY x "from-a">`), nil
		case "inner.mod b/mods.ent":
			return []byte(`<!ENTITY x "from-b">`), nil
		}
		return nil, ErrDTDNotFound
	})
	prepared, err := newSourceDTDResolver(&DTDOptions{Source: source}).prepare(context.Background(), "", "a/main.dtd")
	require.NoError(t, err)
	for _, d := range prepared.declarations {
		if d.Name == "x" {
			assert.Equal(t, "from-a", d.Replacement)
		}
	}
	require.Len(t, requests, 3)
	assert.Equal(t, DTDRequest{SystemID: "inner.mod", BaseSystemID: "a/main.dtd"}, requests[2])
}

func TestExternalDTDSitesAfterTextDeclarations(t *testing.T) {
	t.Parallel()

	// Sites are in the entity after its byte order mark is removed and line
	// ends are normalized, the text declaration included.
	files := map[string]string{
		"m.dtd": "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"UTF-8\"?>\r\n<!ENTITY % e SYSTEM \"e.ent\"> %e;\r\n<!ENTITY x 'X'>",
		"e.ent": "<?xml encoding='UTF-8'?>\n\n<!ENTITY y 'Y'>",
	}
	prepared, err := newSourceDTDResolver(&DTDOptions{Source: mapDTDSource(files)}).prepare(context.Background(), "", "m.dtd")
	require.NoError(t, err)
	want := map[string]sourceXMLPosition{
		"x": {ByteOffset: int64(strings.Index("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!ENTITY % e SYSTEM \"e.ent\"> %e;\n<!ENTITY x", "<!ENTITY x")), Line: 3, Column: 1},
		"y": {ByteOffset: int64(len("<?xml encoding='UTF-8'?>\n\n")), Line: 3, Column: 1},
	}
	for _, d := range prepared.declarations {
		if position, ok := want[d.Name]; ok {
			assert.Equal(t, position, d.Origin.Span.Start, d.Name)
			delete(want, d.Name)
		}
	}
	assert.Empty(t, want)
}

func TestExternalDTDReplacementMapsAreBounded(t *testing.T) {
	t.Parallel()

	// Each character reference starts its own segment; a long run of them
	// must not multiply the literal's storage.
	literal := strings.Repeat("a&#9;", 200_000)
	dtd := `<!ENTITY % pa "` + literal + `"><!ENTITY % pb "` + literal + `"><!ENTITY x "X">`
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	require.NoError(t, err)
	store, err := newSourceDTDStore(context.Background(), account, sourceDTDStandaloneAbsent)
	require.NoError(t, err)
	defer store.Close()
	store.parsed = true
	_, _, err = parseExternalSubset(context.Background(), store, &sourceDTDResolver{}, sourceDTDEntity{name: "big.dtd", systemID: "big.dtd", text: dtd, size: int64(len(dtd))}, false, 64)
	require.NoError(t, err)
	for d, segments := range store.segments {
		assert.LessOrEqual(t, len(segments), sourceDTDMaxTextSegments+1, d.Name)
		assert.Empty(t, segments[len(segments)-1].Entity, "text past the bound has no site")
	}

	// A declaration written past the bound has no known site; one before it
	// keeps its site.
	late := `<!ENTITY % late "` + strings.Repeat("&#32;", 300) + `<!ENTITY z 'Z'>"> %late; ` +
		`<!ENTITY % early "<!ENTITY w 'W'>` + strings.Repeat("&#32;", 300) + `"> %early;`
	prepared, err := newSourceDTDResolver(&DTDOptions{Source: mapDTDSource{"late.dtd": late}}).prepare(context.Background(), "", "late.dtd")
	require.NoError(t, err)
	sites := map[string]bool{}
	for _, d := range prepared.declarations {
		sites[d.Name] = d.Origin.SpanAvailable
	}
	assert.Equal(t, map[string]bool{"z": false, "w": true}, sites)

	doc := `<!DOCTYPE score-partwise SYSTEM "big.dtd"><score-partwise><work><work-title>&x;</work-title></work>` +
		`<part-list><score-part id="P1"><part-name>M</part-name></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	_, err = DecodeWithOptions(strings.NewReader(doc), DecodeOptions{DTD: &DTDOptions{Source: mapDTDSource{"big.dtd": dtd}}})
	assert.NoError(t, err)
}

func TestExternalDTDTextIsBuffered(t *testing.T) {
	t.Parallel()

	// The library's normalized copy of a caller entity is live storage while
	// the entity is read, whichever path reads it.
	comment := `<!--` + strings.Repeat("x", 1_000_000) + `-->`
	inspect := func(source mapDTDSource, internal string, limit int64) error {
		_, err := InspectXML(context.Background(), strings.NewReader(houseDTDInput(internal, "&x;")),
			AssessmentOptions{Profile: ProfileSourceInternalDTD, Budgets: validation.Budgets{MaxBufferedBytes: limit}, DTD: &DTDOptions{Source: source}})
		return err
	}
	small := mapDTDSource{"house.dtd": `<!ENTITY x "X">`}
	require.NoError(t, inspect(small, "", 256<<10), "the document itself fits in 256 KiB")
	for name, c := range map[string]struct {
		source   mapDTDSource
		internal string
	}{
		"root":                      {mapDTDSource{"house.dtd": comment + `<!ENTITY x "X">`}, ""},
		"root with an internal PE":  {mapDTDSource{"house.dtd": comment + `<!ENTITY x "X">`}, `<!ENTITY % local "x">`},
		"external parameter entity": {mapDTDSource{"house.dtd": `<!ENTITY % big SYSTEM "big.ent"> %big; <!ENTITY x "X">`, "big.ent": comment}, ""},
		"eight megabytes":           {mapDTDSource{"house.dtd": `<!--` + strings.Repeat("x", 8_000_000) + `--><!ENTITY x "X">`}, ""},
	} {
		requireResourceKind(t, inspect(c.source, c.internal, 256<<10), validation.BudgetBufferedBytes, name)
		assert.NoError(t, inspect(c.source, c.internal, 32<<20), name)
	}

	// The text is released when it is no longer read: after a parse only the
	// store's own declarations remain, and nothing remains after Close on any
	// failure.
	files := mapDTDSource{"big.ent": comment + `<!ENTITY y "Y">`, "lit.ent": strings.Repeat("z", 1000)}
	for _, limit := range []int64{0, 600_000, 1_100_000, 1_500_000, 2_200_000} {
		account, err := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: limit})
		require.NoError(t, err)
		store, err := newSourceDTDStore(context.Background(), account, sourceDTDStandaloneAbsent)
		require.NoError(t, err)
		store.parsed = true
		root, _, charged, err := sourceDTDExternalText(account, []byte(comment+`<!ENTITY % big SYSTEM "big.ent"> %big; <!ENTITY x "X">`+
			`<!ENTITY % lit SYSTEM "lit.ent"><!ENTITY z "%lit;">`))
		if err == nil {
			_, _, err = parseExternalSubset(context.Background(), store, newSourceDTDResolver(&DTDOptions{Source: files}), sourceDTDEntity{name: "m.dtd", systemID: "m.dtd", text: root, charged: charged}, false, 64)
		}
		used, _ := account.Usage(validation.BudgetBufferedBytes)
		if err == nil {
			assert.Equal(t, store.owned, used, "limit %d: only the store remains", limit)
		}
		require.NoError(t, store.Close())
		used, _ = account.Usage(validation.BudgetBufferedBytes)
		assert.Zero(t, used, "limit %d: err=%v", limit, err)
	}
}
