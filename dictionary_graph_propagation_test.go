package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func dictionaryGraphSelection() []validation.DictionarySelection {
	return []validation.DictionarySelection{{Manifest: validation.DictionaryManifestSMuFL14Canonical, Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName}}}
}

func dictionaryGraphKnownSource(t *testing.T) (DictionarySource, *int, *[]*resourceTestBody) {
	t.Helper()
	raw, err := os.ReadFile("testdata/dictionaries/smufl-1.4/glyphnames.json")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	var bodies []*resourceTestBody
	source := dictionaryGraphSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		if q.Manifest != validation.DictionaryManifestSMuFL14Canonical || q.Component.ID != validation.DictionaryComponentSMuFL14Glyphnames {
			t.Errorf("unexpected selected component: %+v", q)
		}
		body := &resourceTestBody{Reader: bytes.NewReader(raw)}
		bodies = append(bodies, body)
		return DictionaryComponentResult{Request: q, State: validation.EvidenceKnown, Body: body}, nil
	})
	return source, &calls, &bodies
}

func dictionaryGraphRequireCanonical(t *testing.T, r validation.Report, state validation.EvidenceState, value bool) {
	t.Helper()
	for _, row := range r.Context {
		if row.Property == validation.ContextDictionaryCanonical && row.Dictionary != nil && row.Dictionary.Input.Value.Text == "accidentalSharp" {
			d := row.Dictionary
			if d.Result.State != state || (state == validation.EvidenceKnown && d.Result.Value != value) {
				t.Fatalf("canonical evidence %+v", d)
			}
			if state == validation.EvidenceKnown && !d.ComponentVerified {
				t.Error("missing in-run component certificate")
			}
			return
		}
	}
	t.Fatal("missing accidentalSharp dictionary evidence")
}

func TestDictionaryGraphOneImportAcrossVerifiedAliases(t *testing.T) {
	ctx := context.Background()
	source, calls, bodies := dictionaryGraphKnownSource(t)
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="a.musicxml"/><score xlink:href="b.musicxml"/></opus>`
	child := resourceScore(`<note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`)
	fixture := &graphFixtureResolver{resources: map[string]string{"https://dictionary.test/a.musicxml": child, "https://dictionary.test/b.musicxml": child}, pins: map[string]string{"https://dictionary.test/a.musicxml": "same", "https://dictionary.test/b.musicxml": "same"}}
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := engine.Profile(ProfileSourceDictionaries.ID())
	resolver := graphResolverFunc(func(ctx context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		if q.Document == nil || q.Document.RequiredProfile != ProfileSourceDictionaries.ID() || q.Document.CatalogDigest != profile.CatalogDigest {
			t.Fatalf("wrong exact linked interpretation: %+v", q.Document)
		}
		return fixture.Resolve(ctx, q)
	})
	options := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}, Identity: ResourceIdentity{RetrievalURI: "https://dictionary.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}
	g, err := startDictionaryResourceGraph(ctx, options.internal(), DictionaryOptions{Selections: dictionaryGraphSelection(), Source: source}, ProfileResourceGraphDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	set := g.dictionaries
	b, err := g.inspectPrimarySource(strings.NewReader(root))
	if err != nil || !b.Satisfied() || b.ResourceCount() != 2 || b.AdoptionCount() != 3 || fixture.calls != 2 || *calls != 1 {
		t.Fatalf("err=%v satisfied=%v resources=%d adoptions=%d resolver=%d dictionary=%d", err, b.Satisfied(), b.ResourceCount(), b.AdoptionCount(), fixture.calls, *calls)
	}
	_, r, _ := b.ResourceReport(1)
	dictionaryGraphRequireCanonical(t, r, validation.EvidenceKnown, true)
	seen := map[string]bool{}
	for i := 0; i < b.AdoptionCount(); i++ {
		_, id, _ := b.Adoption(i)
		if seen[id] {
			t.Fatal("alias reused occurrence identity", id)
		}
		seen[id] = true
	}
	for _, body := range append(*bodies, fixture.bodies...) {
		if body.closed != 1 {
			t.Error("transferred body close count", body.closed)
		}
	}
	if _, err = set.Fingerprint(); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Error("root did not close set", err)
	}
	g.close()
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, e := g.options.account.Usage(kind)
		if e != nil || used != 0 {
			t.Errorf("live %s=%d %v", kind, used, e)
		}
	}
	used, _ := g.options.account.Usage(validation.BudgetExternalBytes)
	if want := int64(301683 + 2*len(child)); used != want {
		t.Errorf("shared external account=%d want %d", used, want)
	}
}

func TestDictionaryGraphProviderErrorKeepsIndependentFamilyFailure(t *testing.T) {
	ctx := context.Background()
	input := resourceScore(`<note><rest/><duration>1</duration><accidental smufl="restQuarter">sharp</accidental></note><note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`)
	doc, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("dictionary provider offline")
	for _, mode := range []string{"source", "model", "raw MXL", "current MXL"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			body := &resourceTestBody{Reader: strings.NewReader("untrusted partial response")}
			source := dictionaryGraphSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				return DictionaryComponentResult{Request: q, State: validation.EvidenceUnknown, Reason: "offline", Body: body}, boom
			})
			d := DictionaryOptions{Selections: dictionaryGraphSelection(), Source: source}
			var b validation.AssessmentBundle
			var e error
			switch mode {
			case "source":
				b, e = inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(input), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal(), Dictionaries: d})
			case "model":
				b, e = assessModelWithResourcesAndDictionaries(ctx, doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal(), Dictionaries: d})
			case "raw MXL":
				b, e = inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(input, "main.musicxml", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSourceDictionaries}}.internal(), Dictionaries: d})
			case "current MXL":
				b, e = assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModelDictionaries}}.internal(), Dictionaries: d})
			}
			if !errors.Is(e, boom) || calls != 1 || body.closed != 1 || !b.EnumerationComplete() {
				t.Fatalf("err=%v calls=%d close=%d enumeration=%v", e, calls, body.closed, b.EnumerationComplete())
			}
			found := false
			for i := 0; i < b.ResourceCount(); i++ {
				_, r, _ := b.ResourceReport(i)
				if r.Profile.ID == ProfileSourceContainer.ID() {
					continue
				}
				found = true
				if r.Conformance != validation.ConformanceViolated || r.Counts.Fail == 0 || r.AssessmentComplete || r.Terminal != nil || !r.InputComplete {
					t.Fatalf("independent invalid/incomplete lost: %+v", r)
				}
				dictionaryGraphRequireCanonical(t, r, validation.EvidenceUnknown, false)
			}
			if !found {
				t.Fatal("independent intrinsic report omitted")
			}
		})
	}
}

func TestDictionaryGraphUnselectedSourceNeverCalled(t *testing.T) {
	calls := 0
	source := dictionaryGraphSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{}, errors.New("unselected callback")
	})
	input := resourceScore(`<note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`)
	b, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal(), Dictionaries: DictionaryOptions{Source: source}})
	if err != nil || calls != 0 || b.ResourceCount() != 1 {
		t.Fatal(err, calls, b.ResourceCount())
	}
	_, r, _ := b.ResourceReport(0)
	dictionaryGraphRequireCanonical(t, r, validation.EvidenceUnknown, false)
}

func TestDictionaryGraphCacheUsesActualCapabilityFingerprint(t *testing.T) {
	ctx := context.Background()
	a, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	g := &resourceGraphRun{ctx: ctx, options: resourceRunOptions{account: a}, cache: map[graphResourceKey]*graphResource{}, pins: map[graphCanonicalKey]validation.ResourcePin{}}
	defer g.close()
	pin := validation.ResourcePin{CanonicalID: "same", Representation: validation.ResourceSourceXML, Digest: strings.Repeat("a", 64)}
	var fingerprints [2][32]byte
	for i, reason := range []validation.Code{"not-authorized", "provider-unavailable"} {
		set, e := validation.NewDictionarySet(ctx, a, dictionaryGraphSelection(), []validation.DictionaryImport{{Manifest: validation.DictionaryManifestSMuFL14Canonical, State: validation.EvidenceUnknown, Reason: reason}})
		if e != nil {
			t.Fatal(e)
		}
		fingerprints[i], e = set.Fingerprint()
		if e != nil {
			t.Fatal(e)
		}
		if e = set.Close(); e != nil {
			t.Fatal(e)
		}
		g.dictionaryFingerprint = fingerprints[i]
		id := validation.ResourceAssessmentID(reason)
		r, e := g.cacheResource(pin, ProfileSourceDictionaries.ID(), id, 1, true)
		if e != nil || r.id != id {
			t.Fatal("actual unavailable capabilities aliased", r, e)
		}
	}
	if fingerprints[0] == fingerprints[1] || len(g.cache) != 2 {
		t.Fatal("capability reasons absent from cache identity", len(g.cache))
	}
	g.dictionaryFingerprint = fingerprints[0]
	r, err := g.cacheResource(pin, ProfileSourceDictionaries.ID(), "ignored", 1, true)
	if err != nil || r.id != "not-authorized" {
		t.Fatal("same capability did not reuse", r, err)
	}
}

func TestDictionaryGraphKnownSetReachesEveryPrimaryRepresentation(t *testing.T) {
	ctx := context.Background()
	input := resourceScore(`<note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`)
	doc, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"source", "model", "raw MXL", "current MXL"} {
		t.Run(mode, func(t *testing.T) {
			source, calls, bodies := dictionaryGraphKnownSource(t)
			d := DictionaryOptions{Selections: dictionaryGraphSelection(), Source: source}
			var b validation.AssessmentBundle
			var e error
			switch mode {
			case "source":
				b, e = inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(input), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal(), Dictionaries: d})
			case "model":
				b, e = assessModelWithResourcesAndDictionaries(ctx, doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal(), Dictionaries: d})
			case "raw MXL":
				b, e = inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(input, "main.musicxml", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSourceDictionaries}}.internal(), Dictionaries: d})
			case "current MXL":
				b, e = assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModelDictionaries}}.internal(), Dictionaries: d})
			}
			if e != nil || *calls != 1 {
				t.Fatal(e, *calls)
			}
			found := false
			for i := 0; i < b.ResourceCount(); i++ {
				_, r, _ := b.ResourceReport(i)
				if r.Profile.ID == ProfileSourceContainer.ID() {
					continue
				}
				found = true
				dictionaryGraphRequireCanonical(t, r, validation.EvidenceKnown, true)
			}
			if !found {
				t.Fatal("dictionary primary omitted")
			}
			for _, body := range *bodies {
				if body.closed != 1 {
					t.Error("body closed", body.closed)
				}
			}
		})
	}
}

func TestDictionaryGraphNestedAliasPackagesShareOneVerifiedSet(t *testing.T) {
	child := resourceScore(`<note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`)
	archive := packageGraphBytes(child, "main.musicxml", "")
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://dictionary.test/a.mxl"/><score xlink:href="https://dictionary.test/b.mxl"/></opus>`
	resolver := &packageResolverFixture{data: map[string][]byte{"https://dictionary.test/a.mxl": archive, "https://dictionary.test/b.mxl": archive}, canonical: map[string]string{"https://dictionary.test/a.mxl": "same-archive", "https://dictionary.test/b.mxl": "same-archive"}}
	source, calls, bodies := dictionaryGraphKnownSource(t)
	b, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(root), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}.internal(), Dictionaries: DictionaryOptions{Selections: dictionaryGraphSelection(), Source: source}})
	if err != nil || *calls != 1 || resolver.calls != 2 || b.ResourceCount() != 3 || !b.EnumerationComplete() {
		t.Fatalf("err=%v imports=%d links=%d resources=%d complete=%v", err, *calls, resolver.calls, b.ResourceCount(), b.EnumerationComplete())
	}
	requireIndirectionContextUnknown(t, b, 2)
	found := false
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		for _, row := range r.Context {
			if row.Dictionary != nil && row.Property == validation.ContextDictionaryCanonical {
				found = true
				dictionaryGraphRequireCanonical(t, r, validation.EvidenceKnown, true)
				break
			}
		}
	}
	if !found {
		t.Fatal("nested score lost dictionary evidence")
	}
	contexts := map[string]validation.PackageID{}
	documents := map[string]bool{}
	for i := 0; i < b.AdoptionCount(); i++ {
		a, _ := b.AdoptionContext(i)
		_, doc, _ := b.Adoption(i)
		if documents[doc] {
			t.Fatal("aliased adoption identity", doc)
		}
		documents[doc] = true
		if a.ArchiveEntry == "main.musicxml" {
			contexts[a.BaseURI] = a.Package
		}
	}
	if contexts["https://dictionary.test/a.mxl"] == "" || contexts["https://dictionary.test/b.mxl"] == "" || contexts["https://dictionary.test/a.mxl"] == contexts["https://dictionary.test/b.mxl"] {
		t.Fatal("nested alias boundaries collapsed", contexts)
	}
	for _, body := range append(*bodies, resolver.bodies...) {
		if body.closed != 1 {
			t.Error("body close count", body.closed)
		}
	}
}
