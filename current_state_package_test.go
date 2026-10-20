package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const currentStateHintedScore = `<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "http://www.musicxml.org/dtds/partwise.dtd">` +
	`<score-partwise xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:a a.xsd extra" version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>` +
	`<part id="P1"><measure number="1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration></note></measure></part></score-partwise>`

func decodedPackage(t *testing.T, archive []byte) *MXLPackage {
	t.Helper()
	pkg, err := DecodeMXLPackage(bytes.NewReader(archive))
	require.NoError(t, err)
	return pkg
}

func schemaHintFindings(r validation.Report) []validation.Finding {
	var out []validation.Finding
	for _, f := range r.Findings {
		if f.Rule.ID == "MX40-PACKAGE-schema-hint-pairs" {
			out = append(out, f)
		}
	}
	return out
}

func TestAssessMXLPackageCurrentStateIsSatisfied(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dictionaries := requiredDictionaryOptions()
	valid := decodedPackage(t, scenarioMXL(t, scenarioValid))
	// The original primary had a DOCTYPE and an odd schemaLocation; the
	// current model keeps neither and is encoded without them.
	hintedArchive := rawMXL(t, currentStateHintedScore)
	hinted := decodedPackage(t, hintedArchive)
	original, err := InspectMXL(ctx, bytes.NewReader(hintedArchive), MXLAssessmentOptions{})
	require.NoError(t, err)
	require.False(t, original.Satisfied(), "the original archive reports its odd schemaLocation")
	require.NotEmpty(t, schemaHintFindings(original.PackageReport()))

	for name, options := range map[string]MXLAssessmentOptions{
		"semantics":                       {},
		"model":                           {Graph: MXLGraphOptions{Profile: ProfileMXLModel}},
		"model dictionaries":              {Graph: MXLGraphOptions{Profile: ProfileMXLModelDictionaries, Dictionaries: &dictionaries}},
		"model internal DTD dictionaries": {Graph: MXLGraphOptions{Profile: ProfileMXLModelInternalDTDDictionaries, Dictionaries: &dictionaries}},
	} {
		for kind, pkg := range map[string]*MXLPackage{"valid": valid, "hinted source": hinted} {
			b, err := AssessMXLPackage(ctx, pkg, options)
			require.NoError(t, err, "%s/%s", name, kind)
			assert.True(t, b.Satisfied(), "%s/%s: %+v", name, kind, b.PackageReport())
			assert.Empty(t, schemaHintFindings(b.PackageReport()), "%s/%s", name, kind)
		}
	}

	// A current model root of a resource graph is assessed the same way.
	for name, options := range map[string]LinkedAssessmentOptions{
		"semantics":                       {},
		"graph":                           {Graph: GraphOptions{Profile: ProfileResourceGraph}},
		"graph dictionaries":              {Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries, Dictionaries: &dictionaries}},
		"graph internal DTD dictionaries": {Graph: GraphOptions{Profile: ProfileResourceGraphInternalDTDDictionaries, Dictionaries: &dictionaries}},
	} {
		b, err := AssessModelWithResources(ctx, hinted.Document, options)
		require.NoError(t, err, name)
		assert.True(t, b.Satisfied(), "%s: %+v", name, b.PackageReport())
		assert.Empty(t, schemaHintFindings(b.PackageReport()), name)
	}
}

// TestAssessMXLPackageCurrentStateMatchesEncodedArchive assesses a package as
// EncodeMXLPackage would write it: the package-level outcome agrees with
// InspectMXL on the encoded bytes, also after edits, and a retained raw
// rootfile keeps its source assessment.
func TestAssessMXLPackageCurrentStateMatchesEncodedArchive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	oddAlternate := strings.Replace(currentStateHintedScore, `<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "http://www.musicxml.org/dtds/partwise.dtd">`, "", 1)
	violated, err := Decode(strings.NewReader(scenarioModelViolated))
	require.NoError(t, err)
	opus, err := Decode(strings.NewReader(`<opus version="4.0"><title>T</title></opus>`))
	require.NoError(t, err)
	timewise, err := Decode(strings.NewReader(`<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><measure number="1"><part id="P1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration></note></part></measure></score-timewise>`))
	require.NoError(t, err)
	inner := scenarioMXL(t, scenarioValid)
	unchecked := func(bool) bool { return true }
	expect := func(want bool) func(bool) bool { return func(got bool) bool { return got == want } }
	for _, tc := range []struct {
		name      string
		edit      func(*MXLPackage)
		satisfied func(bool) bool
	}{
		{name: "unedited", satisfied: expect(true)},
		{name: "opus primary", satisfied: unchecked, edit: func(p *MXLPackage) {
			p.Document, p.RootFiles = opus, []MXLRootFile{{FullPath: "root.opus"}}
		}},
		{name: "timewise primary", satisfied: expect(true), edit: func(p *MXLPackage) { p.Document = timewise }},
		{name: "no rootfiles", satisfied: expect(true), edit: func(p *MXLPackage) { p.RootFiles = nil }},
		{name: "PDF rootfile", satisfied: unchecked, edit: func(p *MXLPackage) {
			p.RootFiles = append(p.RootFiles, MXLRootFile{FullPath: "score.pdf", MediaType: Ptr("application/pdf")})
			p.Resources = append(p.Resources, MXLResource{Path: "score.pdf", Data: []byte("%PDF-1.4\n")})
		}},
		{name: "nested MXL rootfile", satisfied: unchecked, edit: func(p *MXLPackage) {
			p.RootFiles = append(p.RootFiles, MXLRootFile{FullPath: "inner.mxl", MediaType: Ptr("application/vnd.recordare.musicxml")})
			p.Resources = append(p.Resources, MXLResource{Path: "inner.mxl", Data: inner})
		}},
		{name: "unreferenced resource with an odd schemaLocation", satisfied: unchecked, edit: func(p *MXLPackage) {
			p.Resources = append(p.Resources, MXLResource{Path: "notes.xml", Data: []byte(oddAlternate)})
		}},
		{name: "retitled", satisfied: expect(true), edit: func(p *MXLPackage) {
			score, ok := AsScorePartwise(p.Document)
			require.True(t, ok)
			title := "Edited"
			score.Work = &Work{WorkTitle: &title}
		}},
		{name: "primary violation", satisfied: expect(false), edit: func(p *MXLPackage) { p.Document = violated }},
		{name: "retained raw rootfile with an odd schemaLocation", satisfied: expect(false), edit: func(p *MXLPackage) {
			p.RootFiles = append(p.RootFiles, MXLRootFile{FullPath: "alternate.musicxml", MediaType: Ptr("application/vnd.recordare.musicxml+xml")})
			p.Resources = append(p.Resources, MXLResource{Path: "alternate.musicxml", Data: []byte(oddAlternate)})
		}},
	} {
		pkg := decodedPackage(t, scenarioMXL(t, scenarioValid))
		if tc.edit != nil {
			tc.edit(pkg)
		}
		current, err := AssessMXLPackage(ctx, pkg, MXLAssessmentOptions{})
		require.NoError(t, err, tc.name)
		var encoded bytes.Buffer
		require.NoError(t, EncodeMXLPackage(&encoded, pkg), tc.name)
		written, err := InspectMXL(ctx, bytes.NewReader(encoded.Bytes()), MXLAssessmentOptions{})
		require.NoError(t, err, tc.name)

		assert.True(t, tc.satisfied(current.Satisfied()), "%s: %+v", tc.name, summarizeBundle(current))
		assert.Equal(t, written.Satisfied(), current.Satisfied(), tc.name)
		assert.Equal(t, summarizeReport(written.PackageReport()).Findings, summarizeReport(current.PackageReport()).Findings, "%s: package findings agree with the encoded archive", tc.name)
		assert.Equal(t, written.PackageReport().Conformance, current.PackageReport().Conformance, tc.name)
		// The encoded archive also has its container document, which the
		// encoder writes from RootFiles and the current state does not assess.
		require.Equal(t, current.ResourceCount()+1, written.ResourceCount(), tc.name)
		_, container, _ := written.ResourceReport(0)
		assert.True(t, strings.HasSuffix(string(container.Profile.ID), "-container"), "%s: %s", tc.name, container.Profile.ID)
	}

	// An unreferenced resource is recorded as an entry only: it gets no
	// report and its hints are not paired.
	unreferenced := decodedPackage(t, scenarioMXL(t, scenarioValid))
	unreferenced.Resources = append(unreferenced.Resources, MXLResource{Path: "notes.xml", Data: []byte(oddAlternate)})
	b, err := AssessMXLPackage(ctx, unreferenced, MXLAssessmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, b.ResourceCount())
	assert.Empty(t, schemaHintFindings(b.PackageReport()))

	// The retained raw rootfile is assessed from its bytes with the source
	// profile, not as a model without hints.
	pkg := decodedPackage(t, scenarioMXL(t, scenarioValid))
	pkg.RootFiles = append(pkg.RootFiles, MXLRootFile{FullPath: "alternate.musicxml", MediaType: Ptr("application/vnd.recordare.musicxml+xml")})
	pkg.Resources = append(pkg.Resources, MXLResource{Path: "alternate.musicxml", Data: []byte(oddAlternate)})
	b, err = AssessMXLPackage(ctx, pkg, MXLAssessmentOptions{})
	require.NoError(t, err)
	findings := schemaHintFindings(b.PackageReport())
	require.Len(t, findings, 1)
	assert.Equal(t, validation.OutcomeFail, findings[0].Outcome)
	assert.Equal(t, validation.Code("schema-location-unmatched-token"), findings[0].Reason)
	targets := map[validation.Target]int{}
	for i := 0; i < b.ResourceCount(); i++ {
		_, report, _ := b.ResourceReport(i)
		targets[report.Target]++
	}
	assert.Equal(t, map[validation.Target]int{validation.TargetModel: 1, validation.TargetSource: 1}, targets)
}

func currentStateOpusArchive(t *testing.T) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, entry := range []struct{ name, content string }{
		{"META-INF/container.xml", `<container><rootfiles><rootfile full-path="root.opus" media-type="application/vnd.recordare.musicxml+xml"/></rootfiles></container>`},
		{"root.opus", `<opus xmlns:xlink="http://www.w3.org/1999/xlink" version="4.0"><title>T</title><score xlink:href="https://external.test/score.musicxml"/></opus>`},
	} {
		w, err := writer.Create(entry.name)
		require.NoError(t, err)
		_, err = io.WriteString(w, entry.content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return archive.Bytes()
}

// TestDecodeMXLAssessedModelStageUsesLoaderOptions runs the model stage with
// the loader's resources, resolver, dependencies and dictionaries, as
// AssessMXLPackage does with the same options.
func TestDecodeMXLAssessedModelStageUsesLoaderOptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	archive := currentStateOpusArchive(t)
	var resolved, opened atomic.Int32
	resolver := validation.ResourceResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		resolved.Add(1)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "not-supplied"}, nil
	})
	source := DictionarySourceFunc(func(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
		opened.Add(1)
		return BundledDictionarySource().Open(ctx, request)
	})
	options := MXLAssessmentOptions{
		Graph:        MXLGraphOptions{Dictionaries: &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()}},
		Resolver:     resolver,
		Dependencies: []ResourceDependency{ResourceDocuments},
	}

	_, err := InspectMXL(ctx, bytes.NewReader(archive), options)
	require.NoError(t, err)
	perStage := [2]int32{resolved.Swap(0), opened.Swap(0)}
	require.Positive(t, perStage[0], "the fixture asks the resolver")
	require.Positive(t, perStage[1], "the fixture opens dictionaries")

	loaded, err := DecodeMXLAssessed(ctx, bytes.NewReader(archive), DecodeMXLAssessmentOptions{Assessment: options, Lenient: true})
	require.NoError(t, err)
	require.NotNil(t, loaded.Package)
	assert.False(t, loaded.Accepted, "the unresolved score keeps both bundles incomplete")
	assert.Equal(t, perStage[0], resolved.Swap(0), "only line 3 follows resource links and asks the caller's resolver")
	assert.Equal(t, perStage[1], opened.Swap(0), "only line 3 opens the caller's dictionaries")

	// Line 2 alone asks neither the resolver nor a dictionary source.
	line := options
	line.Graph = MXLGraphOptions{Profile: ProfileMXLSourceLine}
	inspected, err := InspectMXL(ctx, bytes.NewReader(archive), line)
	require.NoError(t, err)
	assert.Equal(t, ProfileMXLSourceLine.ID(), inspected.PackageReport().Profile.ID)
	assert.Zero(t, resolved.Load(), "line 2 follows no resource links")
	line.Graph.Dictionaries = options.Graph.Dictionaries
	_, err = InspectMXL(ctx, bytes.NewReader(archive), line)
	assert.ErrorIs(t, err, validation.ErrInvalidConfiguration, "line 2 takes no dictionaries")
	assert.Zero(t, opened.Load())

	current, err := AssessMXLPackage(ctx, loaded.Package, options)
	require.NoError(t, err)
	assert.Equal(t, summarizeBundle(current), summarizeBundle(loaded.ModelLine), "AssessMXLPackage with the same options repeats the model stage")
	assert.Equal(t, perStage[0], resolved.Load())
}

// TestDecodeMXLAssessedModelStageSharesAccount takes the rule steps a
// successful load of a valid archive uses, which is the smallest budget that
// loads it. One step less still accepts the archive and converts it, so the
// model stage is refused on the shared account: an operational error and no
// package in both modes, with everything released. It sets a package hook, so
// it does not run in parallel.
func TestDecodeMXLAssessedModelStageSharesAccount(t *testing.T) {
	ctx := context.Background()
	archive := scenarioMXL(t, scenarioValid)
	load := func(steps int64, lenient bool) (DecodeMXLResult, error) {
		return DecodeMXLAssessed(ctx, bytes.NewReader(archive), DecodeMXLAssessmentOptions{Lenient: lenient, Assessment: MXLAssessmentOptions{Graph: MXLGraphOptions{Budgets: validation.Budgets{MaxRuleSteps: steps}}}})
	}
	var account *validation.Account
	decodeMXLStageHook = func(stage string, a *validation.Account) { account = a }
	t.Cleanup(func() { decodeMXLStageHook = nil })
	result, err := load(1<<40, false)
	require.NoError(t, err)
	require.True(t, result.Accepted)
	minimum, err := account.Usage(validation.BudgetRuleSteps)
	require.NoError(t, err)
	result, err = load(minimum, false)
	require.NoError(t, err, "the steps a load uses are enough for it")
	require.True(t, result.Accepted)
	inspected, err := InspectMXL(ctx, bytes.NewReader(archive), MXLAssessmentOptions{Graph: MXLGraphOptions{Budgets: validation.Budgets{MaxRuleSteps: minimum - 1}}})
	require.NoError(t, err)
	require.True(t, inspected.Satisfied(), "the source stage alone fits one step under the loader's minimum")

	for _, lenient := range []bool{false, true} {
		result, err := load(minimum-1, lenient)
		var resource *validation.ResourceError
		require.ErrorAs(t, err, &resource, "lenient=%v", lenient)
		assert.Equal(t, validation.BudgetRuleSteps, resource.Kind)
		assert.ErrorIs(t, err, ErrIncompleteAssessment, "lenient=%v", lenient)
		assert.Equal(t, validation.ConversionConverted, result.Conversion.State, "lenient=%v: the archive was accepted and converted", lenient)
		assert.True(t, result.ModelLine.PackageReport().Requested, "lenient=%v: the model stage ran", lenient)
		assert.Nil(t, result.Package, "lenient=%v", lenient)
		assert.False(t, result.Accepted, "lenient=%v", lenient)
		used, err := account.Usage(validation.BudgetBufferedBytes)
		require.NoError(t, err)
		assert.Zero(t, used, "lenient=%v: everything is released", lenient)
	}
}

// TestAssessMXLPackageCurrentStateCorpus wraps every official and test-suite
// file as an MXL archive. Each package whose archive InspectMXL accepts is
// satisfied by AssessMXLPackage while unedited, and agrees with InspectMXL on
// the bytes EncodeMXLPackage writes for it. The test is sequential, so a
// -race build only samples every fortieth file, within the race job's time
// limit.
func TestAssessMXLPackageCurrentStateCorpus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		require.NoError(t, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}))
	}
	require.GreaterOrEqual(t, len(files), 154)
	source, model := MXLAssessmentOptions{}, MXLAssessmentOptions{}
	accepted, floor := 0, 139
	if raceEnabled {
		sampled := files[:0]
		for i := 0; i < len(files); i += 40 {
			sampled = append(sampled, files[i])
		}
		files, floor = sampled, 1
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		archive := rawMXL(t, string(data))
		original, err := InspectMXL(ctx, bytes.NewReader(archive), source)
		if err != nil || !original.Satisfied() {
			continue
		}
		accepted++
		pkg, err := DecodeMXLPackage(bytes.NewReader(archive))
		require.NoError(t, err, file)
		current, err := AssessMXLPackage(ctx, pkg, model)
		require.NoError(t, err, file)
		assert.True(t, current.Satisfied(), "%s: %+v", file, current.PackageReport())
		var encoded bytes.Buffer
		require.NoError(t, EncodeMXLPackage(&encoded, pkg), file)
		written, err := InspectMXL(ctx, bytes.NewReader(encoded.Bytes()), source)
		require.NoError(t, err, file)
		assert.Equal(t, written.Satisfied(), current.Satisfied(), file)
		assert.Equal(t, summarizeReport(written.PackageReport()).Findings, summarizeReport(current.PackageReport()).Findings, file)
	}
	t.Logf("%d of %d archives accepted", accepted, len(files))
	assert.GreaterOrEqual(t, accepted, floor, "a floor that may rise")
}

// TestAssessMXLPackageCurrentStateAfterLossyConversion keeps the limit of the
// satisfaction claim: a part id that only an internal-DTD default supplies is
// accepted in the original archive, but conversion does not apply the default,
// so the current state violates the model and strict loading refuses it at
// the model stage.
func TestAssessMXLPackageCurrentStateAfterLossyConversion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	archive := rawMXL(t, `<!DOCTYPE score-partwise [<!ATTLIST part id CDATA "P1">]>`+
		`<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`+
		`<part><measure number="1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration></note></measure></part></score-partwise>`)
	original, err := InspectMXL(ctx, bytes.NewReader(archive), MXLAssessmentOptions{})
	require.NoError(t, err)
	require.True(t, original.Satisfied(), "the DTD default supplies the part id")

	current, err := AssessMXLPackage(ctx, decodedPackage(t, archive), MXLAssessmentOptions{})
	require.NoError(t, err)
	assert.False(t, current.Satisfied(), "the converted part has no id")

	// Strict loading decodes the model from the assessed DTD infoset, so the
	// default supplies the id there too and the package is accepted.
	result, err := DecodeMXLAssessed(ctx, bytes.NewReader(archive), DecodeMXLAssessmentOptions{})
	require.NoError(t, err)
	assert.True(t, result.SourceLine.Satisfied())
	assert.True(t, result.ModelLine.Satisfied())
	assert.Equal(t, validation.ConversionConverted, result.Conversion.State)
	require.NotNil(t, result.Package)
	part := result.Package.Document.(*ScorePartwise).Part[0]
	assert.Equal(t, "P1", part.ID)
}
