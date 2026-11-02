package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// ruleStepExpansion is a document of about 500 bytes whose internal entities
// expand into 50,000 barlines.
func ruleStepExpansion(padding int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><!DOCTYPE score-partwise [<!ENTITY a "<barline/>">`)
	prev := "a"
	for _, name := range []string{"b", "c", "d"} {
		b.WriteString(`<!ENTITY ` + name + ` "` + strings.Repeat("&"+prev+";", 10) + `">`)
		prev = name
	}
	b.WriteString(`]><score-partwise version="4.0"><part-list><score-part id="P1"><part-name>x</part-name></score-part></part-list><part id="P1"><measure number="1">` + strings.Repeat("&"+prev+";", 50) + `</measure></part></score-partwise>`)
	if padding > 0 {
		b.WriteString("<!--" + strings.Repeat("x", padding) + "-->")
	}
	return b.String()
}

// Nodes from internal entity expansion buy at most the work of the document
// bytes read so far: an expanding document is refused on rule steps by the
// base plus the allowance for its bytes, even when only the buffer is raised,
// padded with a trailing comment or reached as a linked document of an
// in-memory model.
func TestDefaultRuleStepLimitIgnoresExpandedNodes(t *testing.T) {
	raised := validation.Budgets{MaxBufferedBytes: 1 << 30}
	refusedAt := func(t *testing.T, err error, documentBytes int) {
		t.Helper()
		var limit *validation.ResourceError
		most := int64(validation.DefaultRuleStepBase + validation.DefaultRuleStepsPerDocumentByte*documentBytes)
		if !errors.As(err, &limit) || limit.Kind != validation.BudgetRuleSteps || limit.Limit > most {
			t.Fatalf("want a rule-step refusal at %d or below for %d document bytes, got %v", most, documentBytes, err)
		}
	}
	for _, padding := range []int{0, 30 << 10} {
		input := ruleStepExpansion(padding)
		_, err := InspectXML(context.Background(), strings.NewReader(input), SourceOptions{Profile: ProfileSourceSemantics, Budgets: raised})
		refusedAt(t, err, len(input))
	}
	child := ruleStepExpansion(0)
	resolver := validation.ResourceResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(child)))}, Body: io.NopCloser(strings.NewReader(child))}, nil
	})
	opus, err := Decode(strings.NewReader(`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.musicxml"/></opus>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = AssessModelWithResources(context.Background(), opus, LinkedAssessmentOptions{Graph: GraphOptions{Budgets: raised}, Identity: ResourceIdentity{RetrievalURI: "https://budget.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: resolver})
	refusedAt(t, err, len(child))
}

// ruleStepMXL packs one score as the only rootfile of an MXL archive.
func ruleStepMXL(t testing.TB, score []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, entry := range []struct {
		name   string
		method uint16
		data   []byte
	}{
		{"mimetype", zip.Store, []byte("application/vnd.recordare.musicxml")},
		{"META-INF/container.xml", zip.Deflate, []byte(`<container><rootfiles><rootfile full-path="score.musicxml" media-type="application/vnd.recordare.musicxml+xml"/></rootfiles></container>`)},
		{"score.musicxml", zip.Deflate, score},
	} {
		w, err := z.CreateHeader(&zip.FileHeader{Name: entry.name, Method: entry.method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

// The default allowance keeps a margin over real documents: every official and
// test-suite file, loaded as XML and packed as MXL, loads within the default
// rule-step limit, and on files of at least 1000 nodes strict loading needs at
// most two thirds of the allowance, without counting the base. Smaller files
// spend a larger share on fixed costs, which the base covers.
func TestDefaultRuleStepAllowanceCoversTheCorpus(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("308 serial strict loads")
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(files) != 154 {
		t.Fatalf("corpus: %d files", len(files))
	}
	var account *validation.Account
	assessedLoadingAccountObserver = func(a *validation.Account) { account = a }
	t.Cleanup(func() { assessedLoadingAccountObserver = nil })
	usage := func(kind validation.BudgetKind) int64 {
		used, err := account.Usage(kind)
		if err != nil {
			t.Fatal(err)
		}
		return used
	}
	budgets := validation.Budgets{MaxBufferedBytes: 1 << 30}
	worst := map[string]float64{}
	measured := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, format := range []string{"xml", "mxl"} {
			account = nil
			if format == "xml" {
				_, err = DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Assessment: &SourceOptions{Budgets: budgets}})
			} else {
				_, err = DecodeMXLAssessed(context.Background(), bytes.NewReader(ruleStepMXL(t, data)), DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{Graph: MXLGraphOptions{Budgets: budgets}}})
			}
			if account == nil {
				t.Fatalf("%s %s: no account observed", file, format)
			}
			if limit := (*validation.ResourceError)(nil); errors.As(err, &limit) {
				t.Errorf("%s %s: default limits refused: %v", file, format, err)
				continue
			}
			steps, nodes := usage(validation.BudgetRuleSteps), usage(validation.BudgetNodes)
			allowance := ruleStepLimit(t, account) - validation.DefaultRuleStepBase
			if nodes < 1000 {
				continue
			}
			measured++
			if 3*steps > 2*allowance {
				t.Errorf("%s %s: %d steps for %d nodes leave less than a third of the allowance %d", file, format, steps, nodes, allowance)
			}
			worst[format] = max(worst[format], float64(steps)/float64(allowance))
		}
	}
	if measured < 10 {
		t.Fatalf("only %d runs of at least 1000 nodes", measured)
	}
	t.Logf("%d runs; largest share of the allowance used: %v", measured, worst)
}

// Document bytes are counted after decompression and across linked documents:
// a rest-heavy score whose MXL archive deflates 54 times loads like its XML, a
// small opus primary does not starve the large score it links, and a model
// keeps its allowance when a tiny linked document is read after it.
func TestDefaultRuleStepLimitCountsCompressedAndLinkedDocuments(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	raised := validation.Budgets{MaxBufferedBytes: 1 << 30}
	var score strings.Builder
	score.WriteString(`<score-partwise version="4.0"><part-list>`)
	for p := 1; p <= 14; p++ {
		fmt.Fprintf(&score, `<score-part id="P%d"><part-name>P</part-name></score-part>`, p)
	}
	score.WriteString(`</part-list>`)
	for p := 1; p <= 14; p++ {
		fmt.Fprintf(&score, `<part id="P%d">`, p)
		for m := 1; m <= 70; m++ {
			fmt.Fprintf(&score, `<measure number="%d">`, m)
			if m == 1 {
				score.WriteString(`<attributes><divisions>1</divisions><time><beats>4</beats><beat-type>4</beat-type></time><clef><sign>G</sign><line>2</line></clef></attributes>`)
			}
			score.WriteString(`<note><rest measure="yes"/><duration>4</duration></note></measure>`)
		}
		score.WriteString(`</part>`)
	}
	score.WriteString(`</score-partwise>`)
	archive := ruleStepMXL(t, []byte(score.String()))
	if ratio := score.Len() / len(archive); ratio < 50 {
		t.Fatalf("fixture deflates only %d times", ratio)
	}
	packed, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(archive), DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{Graph: MXLGraphOptions{Budgets: raised}}})
	if err != nil || !packed.Accepted {
		t.Errorf("compressible MXL: accepted=%v error=%v", packed.Accepted, err)
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	child := reviewBudgetRemoveDOCTYPE(t, raw)
	resolver := validation.ResourceResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256(child))}, Body: io.NopCloser(bytes.NewReader(child))}, nil
	})
	opus := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.musicxml"/></opus>`
	bundle, err := InspectXMLWithResources(context.Background(), strings.NewReader(opus), LinkedAssessmentOptions{Graph: GraphOptions{Budgets: raised}, Identity: ResourceIdentity{RetrievalURI: "https://budget.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: resolver})
	var limit *validation.ResourceError
	if errors.As(err, &limit) {
		t.Errorf("linked MozartTrio under a %d-byte opus: %v", len(opus), err)
	}
	if bundle.ResourceCount() < 2 {
		t.Errorf("linked score not assessed: resources=%d error=%v", bundle.ResourceCount(), err)
	}

	// An in-memory model keeps the allowance its nodes bought when a linked
	// document of a few bytes is read after it: the limit never shrinks.
	linking := strings.Replace(string(child), "<score-partwise", `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"`, 1)
	end := strings.Index(linking, "</measure>")
	linking = linking[:end] + `<link xlink:href="child.opus"/>` + linking[end:]
	document, err := Decode(strings.NewReader(linking))
	if err != nil {
		t.Fatal(err)
	}
	tiny := []byte(`<opus/>`)
	tinyResolver := validation.ResourceResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256(tiny))}, Body: io.NopCloser(bytes.NewReader(tiny))}, nil
	})
	_, err = AssessModelWithResources(context.Background(), document, LinkedAssessmentOptions{Graph: GraphOptions{Budgets: raised}, Identity: ResourceIdentity{RetrievalURI: "https://budget.test/root.musicxml"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: tinyResolver})
	if errors.As(err, &limit) {
		t.Errorf("model with a %d-byte linked document: %v", len(tiny), err)
	}
}

// Nodes charged before the first document byte keep their allowance, and later
// work on the account may spend it. For ZIP entries that allowance costs
// archive bytes: every central-directory record takes at least 46 of them, so
// an archive padded with empty entries buys at most 10,000 steps per 46 archive
// bytes before the expanding member's own document bytes.
func TestDefaultRuleStepLimitBoundsPreReadEntries(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("slow under the race detector")
	}
	member, container := []byte(ruleStepExpansion(0)), []byte(`<container><rootfiles><rootfile full-path="score.musicxml" media-type="application/vnd.recordare.musicxml+xml"/></rootfiles></container>`)
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	add := func(name string, method uint16, data []byte) {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	add("mimetype", zip.Store, []byte("application/vnd.recordare.musicxml"))
	add("META-INF/container.xml", zip.Deflate, container)
	add("score.musicxml", zip.Deflate, member)
	for i := 0; i < 2000; i++ {
		add(fmt.Sprintf("p/%d", i), zip.Store, nil)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := InspectMXL(context.Background(), bytes.NewReader(archive.Bytes()), MXLAssessmentOptions{Graph: MXLGraphOptions{Budgets: validation.Budgets{MaxBufferedBytes: 1 << 30}}})
	var limit *validation.ResourceError
	most := int64(validation.DefaultRuleStepBase + validation.DefaultRuleStepsPerNode*archive.Len()/46 + validation.DefaultRuleStepsPerDocumentByte*(len(member)+len(container)))
	if !errors.As(err, &limit) || limit.Kind != validation.BudgetRuleSteps || limit.Limit > most || limit.Limit <= validation.DefaultRuleStepBase+validation.DefaultRuleStepsPerNode*2000 {
		t.Fatalf("padded %d-byte archive: want a rule-step refusal above the entries' allowance and at %d or below, got %v", archive.Len(), most, err)
	}
}
