package musicxml

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Literal public IDs also let these feature tests compile against the pre-feature
// baseline, where rejection is an executable red result rather than a build error.
const (
	independentDTDContainerProfile  validation.ProfileID = "musicxml-4.0/source-internal-dtd-container"
	independentDTDSoundsProfile     validation.ProfileID = "musicxml-4.0/source-internal-dtd-sounds-dictionaries"
	independentDTDGraphProfile      validation.ProfileID = "musicxml-4.0/resource-graph-internal-dtd-dictionaries"
	independentDTDRawMXLProfile     validation.ProfileID = "musicxml-4.0/mxl-source-internal-dtd-dictionaries"
	independentDTDCurrentMXLProfile validation.ProfileID = "musicxml-4.0/mxl-model-internal-dtd-dictionaries"
)

// These expectations are derived from the composition receipt and public
// evidence/ownership contracts, independently of the implementation tests.
func independentDTDAccount(t *testing.T, limits validation.Budgets) *validation.Account {
	t.Helper()
	a, err := validation.NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func independentDTDNoLive(t *testing.T, a *validation.Account) {
	t.Helper()
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		n, err := a.Usage(kind)
		if err != nil || n != 0 {
			t.Errorf("live %s=%d, err=%v", kind, n, err)
		}
	}
}
func independentDTDSource(t *testing.T, a *validation.Account, id string, profile validation.ProfileID, xml string) resourceIntrinsicResult {
	t.Helper()
	e, err := sourceAssessmentEngineForProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	return assessResourceSource(context.Background(), strings.NewReader(xml), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: a, DocumentID: id, Entry: "META-INF/independent.xml", Profile: profile, Engine: e}, ByteOwner: sourceBytesPrimary})
}
func independentDTDOrigin(t *testing.T, report validation.Report, subject validation.NodeRef, attr string) *validation.SourceOriginContextResult {
	t.Helper()
	for _, c := range report.Context {
		if c.SourceOrigin != nil && c.Subject == subject && c.SourceOrigin.Attribute != nil && c.SourceOrigin.Attribute.Local == attr {
			return c.SourceOrigin
		}
	}
	t.Fatalf("missing %s origin for %+v", attr, subject)
	return nil
}

func TestDTDPkgIndependentContainerPhysicalEffectiveMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, declarations, element, value, raw string
		state                                   validation.EvidenceState
		kind                                    validation.SourceOriginKind
		specified                               bool
	}{
		{"physical-tokenized", `<!ATTLIST rootfile full-path NMTOKENS #IMPLIED>`, `<rootfile full-path="  score&#x9; .xml  "/>`, "score .xml", "  score\t .xml  ", validation.EvidenceKnown, validation.SourceOriginPhysical, true},
		{"default-tokenized", `<!ATTLIST rootfile full-path NMTOKENS "  score&#x9; .xml  ">`, `<rootfile/>`, "score .xml", "", validation.EvidenceAbsent, validation.SourceOriginDefault, false},
		{"physical-entity", `<!ENTITY path "score.xml"><!ATTLIST rootfile full-path CDATA #IMPLIED>`, `<rootfile full-path="&path;"/>`, "score.xml", "", validation.EvidenceUnknown, validation.SourceOriginPhysical, true},
		{"entity-markup", `<!ENTITY file '<rootfile full-path="score.xml"/>'>`, `&file;`, "score.xml", "", validation.EvidenceAbsent, validation.SourceOriginEntity, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xml := "<!DOCTYPE container [" + tc.declarations + "]><container><rootfiles>" + tc.element + "</rootfiles></container>"
			r, err := InspectXML(context.Background(), strings.NewReader(xml), SourceOptions{Context: validation.ContextFamilyAll, Profile: SourceProfile(independentDTDContainerProfile)})
			if err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, c := range r.Context {
				if c.Container == nil || c.Container.Role != validation.ContainerRootfile {
					continue
				}
				found++
				v := c.Container
				if v.FullPath.State != validation.EvidenceKnown || v.FullPath.Value.Text != tc.value || v.FullPathRaw.State != tc.state || v.FullPathRaw.Value != tc.raw {
					t.Fatalf("effective/physical conflated: %+v", v)
				}
				if v.MediaType.State != validation.EvidenceAbsent || v.MediaTypeRaw.State != validation.EvidenceAbsent {
					t.Fatalf("invented media type: %+v", v)
				}
				origin := independentDTDOrigin(t, r, c.Subject, "full-path")
				if origin.Kind != tc.kind || origin.Specified.State != validation.EvidenceKnown || origin.Specified.Value != tc.specified {
					t.Fatalf("origin=%+v", origin)
				}
				if !tc.specified && origin.Physical.State != validation.EvidenceAbsent {
					t.Fatalf("invented physical site: %+v", origin)
				}
			}
			if found != 1 {
				t.Fatalf("rootfile contexts=%d", found)
			}
		})
	}
}

func TestDTDPkgIndependentAuxiliaryHintsKeepOddEffectiveTail(t *testing.T) {
	for _, name := range []string{"container", "sounds"} {
		for _, physical := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/physical=%v", name, physical), func(t *testing.T) {
				attrs := ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
				decl := `<!ATTLIST ` + name + ` xsi:schemaLocation NMTOKENS " urn:first&#x9; first.xsd  urn:orphan " xsi:noNamespaceSchemaLocation CDATA " lone.xsd ">`
				if physical {
					attrs += ` xsi:schemaLocation=" urn:first&#x9; first.xsd  urn:orphan " xsi:noNamespaceSchemaLocation=" lone.xsd "`
				}
				body := `<sound id="custom"/>`
				profile := independentDTDSoundsProfile
				if name == "container" {
					body = `<rootfiles><rootfile full-path="main.xml"/></rootfiles>`
					profile = independentDTDContainerProfile
				}
				xml := "<!DOCTYPE " + name + " [" + decl + "]><" + name + attrs + ">" + body + "</" + name + ">"
				r, err := InspectXML(context.Background(), strings.NewReader(xml), SourceOptions{Context: validation.ContextFamilyAll, Profile: SourceProfile(profile)})
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, c := range r.Context {
					if c.Resource == nil || c.Resource.SchemaHints == nil {
						continue
					}
					count++
					h := c.Resource.SchemaHints
					if len(h.Pairs) != 1 || h.Pairs[0].Namespace.State != validation.EvidenceKnown || h.Pairs[0].Namespace.Value != "urn:first" || h.Pairs[0].Location.Value != "first.xsd" || h.Trailing.State != validation.EvidenceKnown || h.Trailing.Value != "urn:orphan" || h.Complete.State != validation.EvidenceKnown || h.Complete.Value {
						t.Fatalf("odd effective hint=%+v", h)
					}
					if h.NoNamespace.Value.State != validation.EvidenceKnown || h.NoNamespace.Value.Value.Text != "lone.xsd" {
						t.Fatalf("lost effective noNamespace=%+v", h.NoNamespace)
					}
					want := validation.EvidenceAbsent
					if physical {
						want = validation.EvidenceKnown
					}
					if h.Raw.State != want || h.NoNamespace.Presence.State != want || h.NoNamespace.Raw.State != want {
						t.Fatalf("physical hint confused with default: %+v", h)
					}
					if physical && (h.Raw.Value != " urn:first\t first.xsd  urn:orphan " || h.NoNamespace.Raw.Value != " lone.xsd ") {
						t.Fatalf("original hint whitespace lost: %+v", h)
					}
				}
				if count != 1 {
					t.Fatalf("hint rows=%d", count)
				}
				if r.Conformance == validation.ConformanceViolated {
					t.Fatalf("odd discovery tail became schema violation: %+v", r.Findings)
				}
			})
		}
	}
}

func TestDTDPkgIndependentSoundsDefaultsDoNotEraseStringSpaceOrIdentityFailure(t *testing.T) {
	xml := `<!DOCTYPE sounds [<!ATTLIST sound id ID "repeated" primary (yes|no) "yes"><!ATTLIST ensemble primary (yes|no) "no" number NMTOKEN " two "><!ENTITY row '<ensemble> A&#x9; B </ensemble>'>]><sounds><sound>&row;</sound><sound><solo> C  D </solo></sound></sounds>`
	r, err := inspectXMLWithDictionaries(context.Background(), strings.NewReader(xml), dictionaryAssessmentOptions{Assessment: assessmentOptions{Context: validation.ContextFamilyAll, Profile: independentDTDSoundsProfile}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Conformance != validation.ConformanceViolated {
		t.Fatalf("duplicate effective IDs did not remain violated: %+v", r)
	}
	count := 0
	for _, c := range r.Context {
		m := c.SoundMapping
		if m == nil {
			continue
		}
		count++
		if m.SoundID.State != validation.EvidenceKnown || m.SoundID.Value.Text != "repeated" {
			t.Fatalf("effective ID=%+v", m)
		}
		switch m.Role {
		case validation.SoundMappingSound:
			if m.Primary.State != validation.EvidenceKnown || m.Primary.Value.Text != "yes" || m.Direction != validation.SoundMappingToMusicXML {
				t.Fatalf("sound effective default=%+v", m)
			}
		case validation.SoundMappingEnsemble:
			if m.Primary.State != validation.EvidenceKnown || m.Primary.Value.Text != "no" || m.Number.State != validation.EvidenceKnown || m.Number.Value.Text != "two" || m.LibraryID.Value.Text != " A\t B " || m.Direction != validation.SoundMappingFromMusicXML {
				t.Fatalf("ensemble effective mapping=%+v", m)
			}
		case validation.SoundMappingSolo:
			if m.Primary.State != validation.EvidenceAbsent || m.LibraryID.Value.Text != " C  D " {
				t.Fatalf("solo mapping=%+v", m)
			}
		default:
			t.Fatalf("unexpected role %s", m.Role)
		}
	}
	if count != 4 {
		t.Fatalf("mapping rows=%d", count)
	}
}

type independentDTDPanicReader struct{}

func (independentDTDPanicReader) Read([]byte) (int, error) { panic("preflight performed input I/O") }

type independentDTDPanicDictionary struct{}

func (independentDTDPanicDictionary) Open(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
	panic("preflight opened a dictionary")
}

type independentDTDPanicResolver struct{}

func (independentDTDPanicResolver) Resolve(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
	panic("preflight invoked resolver")
}

func TestDTDPkgIndependentAllEntryPointsRejectBeforeIO(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"source", "model", "raw-mxl", "current-mxl"} {
		for _, invalid := range []string{"empty", "unknown", "other-entry", "budget", "dictionary", "dependency", "identity", "mxl-limit"} {
			if invalid == "mxl-limit" && (entry == "source" || entry == "model") {
				continue
			}
			t.Run(entry+"/"+invalid, func(t *testing.T) {
				profile := independentDTDGraphProfile
				if entry == "raw-mxl" {
					profile = independentDTDRawMXLProfile
				}
				if entry == "current-mxl" {
					profile = independentDTDCurrentMXLProfile
				}
				o := mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(profile)}, Resolver: independentDTDPanicResolver{}, Dependencies: []ResourceDependency{ResourceDocuments}}.internal(), Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{{Manifest: validation.DictionaryManifestStandardSounds40, Relations: []validation.DictionaryRelation{validation.DictionaryRelationStandardSound}}}, Source: independentDTDPanicDictionary{}}}
				switch invalid {
				case "empty":
					o.Resources.Graph.Profile = ""
				case "unknown":
					o.Resources.Graph.Profile = "musicxml-4.0/not-a-dtd-profile"
				case "other-entry":
					o.Resources.Graph.Profile = independentDTDRawMXLProfile
					if entry == "raw-mxl" {
						o.Resources.Graph.Profile = independentDTDCurrentMXLProfile
					}
				case "budget":
					o.Resources.Graph.Budgets.MaxInputBytes = -1
				case "dictionary":
					o.Dictionaries.Selections[0].Manifest = "unknown"
				case "dependency":
					o.Resources.Dependencies = append(o.Resources.Dependencies, ResourceDocuments)
				case "identity":
					o.Resources.Identity.RetrievalURI = "relative/path"
				case "mxl-limit":
					o.Resources.MXL.MaxXMLDepth = 4097
				}
				var b validation.AssessmentBundle
				var e error
				switch entry {
				case "source":
					b, e = inspectXMLWithResourcesAndDictionaries(context.Background(), independentDTDPanicReader{}, linkedDictionaryAssessmentOptions{Resources: o.Resources.linked(), Dictionaries: o.Dictionaries})
				case "model":
					b, e = assessModelWithResourcesAndDictionaries(context.Background(), doc, linkedDictionaryAssessmentOptions{Resources: o.Resources.linked(), Dictionaries: o.Dictionaries})
				case "raw-mxl":
					b, e = inspectMXLWithDictionaries(context.Background(), independentDTDPanicReader{}, o)
				case "current-mxl":
					b, e = assessMXLPackageWithDictionaries(context.Background(), &MXLPackage{Document: doc}, o)
				}
				if e == nil || b.ResourceCount() != 0 {
					t.Fatalf("invalid request admitted: resources=%d err=%v", b.ResourceCount(), e)
				}
			})
		}
	}
}

type independentDTDBody struct {
	io.Reader
	closes int
}

func (b *independentDTDBody) Close() error { b.closes++; return nil }

type independentDTDResolver func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error)

func (f independentDTDResolver) Resolve(ctx context.Context, r validation.ResourceRequest) (validation.ResourceResult, error) {
	return f(ctx, r)
}

func TestDTDPkgIndependentDefaultedLinksSurviveOriginIndexShift(t *testing.T) {
	root := `<!DOCTYPE opus [<!ATTLIST opus-link xlink:href CDATA "child.opus">]><opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link/><opus-link/></opus>`
	child := `<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`
	var bodies []*independentDTDBody
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceInternalDTDDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	p, _ := engine.Profile(ProfileSourceInternalDTDDictionaries.ID())
	resolver := independentDTDResolver(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		if q.Document == nil || q.Document.RequiredProfile != p.ID || q.Document.CatalogDigest != p.CatalogDigest || q.Document.Href != "child.opus" {
			t.Fatalf("wrong exact request: %+v", q.Document)
		}
		b := &independentDTDBody{Reader: strings.NewReader(child)}
		bodies = append(bodies, b)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(child)))}, RetrievalURI: "https://example.test/child.opus", Body: b}, nil
	})
	// Use the real starter while preserving visibility of the root account.
	g, err := startDictionaryResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(independentDTDGraphProfile)}, Identity: ResourceIdentity{RetrievalURI: "https://example.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: resolver}.internal(), DictionaryOptions{}, ProfileResourceGraphDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	a := g.options.account
	b, err := g.inspectPrimarySource(strings.NewReader(root))
	if err != nil || !b.EnumerationComplete() || b.ResourceCount() != 2 || b.AdoptionCount() != 3 || len(bodies) != 2 {
		t.Fatalf("err=%v enumeration=%v resources=%d adoptions=%d resolver=%d report=%+v", err, b.EnumerationComplete(), b.ResourceCount(), b.AdoptionCount(), len(bodies), b.PackageReport())
	}
	for _, body := range bodies {
		if body.closes != 1 {
			t.Errorf("external body closes=%d", body.closes)
		}
	}
	for i := 0; i < b.AdoptionCount(); i++ {
		v, ok := b.AdoptionContext(i)
		if !ok || v.ExpansionComplete.State != validation.EvidenceKnown || !v.ExpansionComplete.Value {
			t.Errorf("unsealed shifted context: %+v", v)
		}
	}
	got, _ := a.Usage(validation.BudgetInputBytes)
	if got != int64(len(root)) {
		t.Errorf("primary input=%d want=%d", got, len(root))
	}
	got, _ = a.Usage(validation.BudgetExternalBytes)
	if got != 2*int64(len(child)) {
		t.Errorf("alias input=%d want=%d", got, 2*len(child))
	}
	independentDTDNoLive(t, a)
}

func TestDTDPkgIndependentRejectedTransfersKeepSnapshotOwned(t *testing.T) {
	for _, mode := range []string{"legacy-profile", "foreign-account", "wrong-representation", "publication-budget", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			g, err := startDictionaryResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(independentDTDRawMXLProfile)}}.internal(), DictionaryOptions{}, ProfileMXLSourceDictionaries.ID())
			if err != nil {
				t.Fatal(err)
			}
			pa := g.options.account
			a := pa
			profile := independentDTDContainerProfile
			if mode == "legacy-profile" {
				profile = ProfileSourceContainer.ID()
			}
			if mode == "foreign-account" {
				a = independentDTDAccount(t, validation.Budgets{})
			}
			out := independentDTDSource(t, a, "intrinsic", profile, `<container><rootfiles><rootfile full-path="main.xml"/></rootfiles></container>`)
			if out.Err != nil || out.Snapshot == nil {
				t.Fatal(out.Err)
			}
			before := out.Snapshot.Header()
			s := g.session
			pin := validation.ResourcePin{CanonicalID: "intrinsic", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", out.Digest)}
			if mode == "wrong-representation" {
				pin.Representation = validation.ResourceCurrentModel
				pin.Digest = ""
				pin.SnapshotID = "current"
			}
			ctx := context.Background()
			ballast := int64(0)
			if mode == "publication-budget" {
				used, e := pa.Usage(validation.BudgetBufferedBytes)
				if e != nil {
					t.Fatal(e)
				}
				ballast = pa.Limits().MaxBufferedBytes - used
				if e := pa.Charge(validation.BudgetBufferedBytes, ballast); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err = validationPackageSessionAddResource(s, ctx, pin, out.Snapshot, true); err == nil {
				t.Fatal("invalid transfer accepted")
			}
			_, _ = g.finish(err)
			if ballast != 0 {
				if e := pa.Release(validation.BudgetBufferedBytes, ballast); e != nil {
					t.Fatal(e)
				}
			}
			if !reflect.DeepEqual(before, out.Snapshot.Header()) {
				t.Fatal("failed transfer closed or rewrote caller snapshot")
			}
			if _, err := out.Snapshot.Report(context.Background()); err != nil {
				t.Fatalf("caller no longer owns live snapshot: %v", err)
			}
			if err := out.Snapshot.Close(); err != nil {
				t.Fatal(err)
			}
			independentDTDNoLive(t, a)
			if pa != a {
				independentDTDNoLive(t, pa)
			}
		})
	}
}

func TestDTDPkgIndependentSourceLimitsReleaseOriginAndLexicalStorage(t *testing.T) {
	xml := `<!DOCTYPE container [<!ENTITY path "main.xml"><!ATTLIST rootfile media-type CDATA "application/vnd.recordare.musicxml+xml">]><container><rootfiles><rootfile full-path="&path;"/></rootfiles></container>`
	budgets := []validation.Budgets{{}, {MaxContextFacts: 1}, {MaxContextFacts: 3}, {MaxBufferedBytes: 4096}, {MaxBufferedBytes: 32768}, {MaxBufferedBytes: 131072}, {MaxRuleSteps: 100}, {MaxRuleSteps: 1000}, {MaxRuleSteps: 10000}, {MaxInputBytes: int64(len(xml) - 1)}}
	for i, budget := range budgets {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			a := independentDTDAccount(t, budget)
			out := independentDTDSource(t, a, "bounded", independentDTDContainerProfile, xml)
			if i == 0 && (out.Err != nil || out.Snapshot == nil) {
				t.Fatalf("control failed: %v", out.Err)
			}
			// These caps are below incontrovertible input requirements: three
			// element origins plus two attributes, startup storage, or raw bytes.
			if (i == 1 || i == 2 || i == 3 || i == 6 || i == 9) && out.Err == nil {
				t.Fatal("undersized limit silently ignored")
			}
			if i > 0 && out.Err != nil {
				var resource *validation.ResourceError
				if !errors.As(out.Err, &resource) {
					t.Errorf("non-resource rejection: %v", out.Err)
				}
			}
			if out.Err != nil && out.Report.Satisfied() {
				t.Fatal("interrupted assessment falsely satisfied")
			}
			if out.Snapshot != nil {
				if err := out.Snapshot.Close(); err != nil {
					t.Fatal(err)
				}
			}
			independentDTDNoLive(t, a)
		})
	}
}

func TestDTDPkgIndependentOrdinaryWrappersDoNotGrantDTD(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"source", "model", "raw-mxl", "current-mxl"} {
		t.Run(entry, func(t *testing.T) {
			profile := independentDTDGraphProfile
			if entry == "raw-mxl" {
				profile = independentDTDRawMXLProfile
			}
			if entry == "current-mxl" {
				profile = independentDTDCurrentMXLProfile
			}
			o := MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(profile)}, Resolver: independentDTDPanicResolver{}, Dependencies: []ResourceDependency{ResourceDocuments}}
			var err error
			switch entry {
			case "source":
				_, err = InspectXMLWithResources(context.Background(), independentDTDPanicReader{}, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(o.Graph.Profile), Budgets: o.Graph.Budgets, DTD: o.Graph.DTD, Dictionaries: o.Graph.Dictionaries}, Identity: o.Identity, Resolver: o.Resolver, Dependencies: o.Dependencies})
			case "model":
				_, err = AssessModelWithResources(context.Background(), doc, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(o.Graph.Profile), Budgets: o.Graph.Budgets, DTD: o.Graph.DTD, Dictionaries: o.Graph.Dictionaries}, Identity: o.Identity, Resolver: o.Resolver, Dependencies: o.Dependencies})
			case "raw-mxl":
				_, err = InspectMXL(context.Background(), independentDTDPanicReader{}, o)
			case "current-mxl":
				_, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc}, o)
			}
			if !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("ordinary wrapper granted composed interpretation: %v", err)
			}
		})
	}
}

func TestDTDPkgIndependentCurrentModelRemainsSourceHonest(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="lost.xsd"/>`))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := modelAssessmentEngineForProfile(ProfileModelDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	exact, _ := engine.Profile(ProfileModelDictionaries.ID())
	for _, mxl := range []bool{false, true} {
		t.Run(fmt.Sprint(mxl), func(t *testing.T) {
			previous := ""
			for i := 0; i < 2; i++ {
				var b validation.AssessmentBundle
				var err error
				if mxl {
					b, err = assessMXLPackageWithDictionaries(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus", MediaType: Ptr("application/vnd.recordare.musicxml+xml")}}}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(independentDTDCurrentMXLProfile)}}.internal()})
				} else {
					b, err = assessModelWithResourcesAndDictionaries(context.Background(), doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(independentDTDGraphProfile)}}.internal()})
				}
				if err != nil {
					t.Fatal(err)
				}
				if b.ResourceCount() != 1 {
					t.Fatalf("invented source container: resources=%d", b.ResourceCount())
				}
				_, r, _ := b.ResourceReport(0)
				if r.Target != validation.TargetModel || r.Profile.ID != exact.ID || r.Profile.CatalogDigest != exact.CatalogDigest || r.Syntax != validation.SyntaxNotAssessed {
					t.Fatalf("current model interpretation rewritten: %+v", r)
				}
				hints := 0
				for _, v := range r.Context {
					if v.SourceOrigin != nil || v.Container != nil {
						t.Fatal("invented source provenance/container")
					}
					for _, site := range v.Origins {
						if site.Position != nil {
							t.Fatal("invented source coordinates")
						}
					}
					if v.Resource != nil && v.Resource.SchemaHints != nil {
						hints++
						h := v.Resource.SchemaHints
						if h.Raw.State != validation.EvidenceUnsupported || h.NoNamespace.Value.State != validation.EvidenceUnsupported || h.NoNamespace.Presence.State != validation.EvidenceUnsupported {
							t.Fatalf("invented model hint: %+v", h)
						}
					}
				}
				if hints != 1 {
					t.Fatalf("model hint contexts=%d", hints)
				}
				_, identity, ok := b.Adoption(0)
				if !ok || identity == "" || identity == previous {
					t.Fatalf("model occurrence identity reused: %q", identity)
				}
				previous = identity
			}
		})
	}
}

func TestDTDPkgIndependentCapabilitiesPartitionExactSnapshotIdentity(t *testing.T) {
	ctx := context.Background()
	g, err := startDictionaryResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(independentDTDGraphProfile)}}.internal(), DictionaryOptions{}, ProfileResourceGraphDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	a := g.options.account
	set, err := validation.NewDictionarySet(ctx, a, []validation.DictionarySelection{{Manifest: validation.DictionaryManifestStandardSounds40, Relations: []validation.DictionaryRelation{validation.DictionaryRelationStandardSound}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceInternalDTDDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	var ids []validation.ResourceAssessmentID
	pin := validation.ResourcePin{CanonicalID: "same-source", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(`<opus/>`)))}
	for i, dictionary := range []*validation.DictionarySet{g.dictionaries, set} {
		out := assessResourceSource(ctx, strings.NewReader(`<opus/>`), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: a, DocumentID: fmt.Sprintf("capability-%d", i), Profile: ProfileSourceInternalDTDDictionaries.ID(), Engine: engine, Dictionaries: dictionary}, ByteOwner: sourceBytesPrimary})
		if out.Err != nil || out.Snapshot == nil {
			t.Fatal(out.Err)
		}
		// An editable returned report cannot replace the core snapshot identity.
		out.Report.Profile.CatalogDigest = "forged"
		out.Report.Profile.ID = ProfileSourceDictionaries.ID()
		id, err := validationPackageSessionAddResource(g.session, ctx, pin, out.Snapshot, true)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Fatal("same bytes under different actual capabilities merged")
	}
	b, err := g.finish(nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.ResourceCount() != 2 {
		t.Fatalf("canonical interpretations=%d", b.ResourceCount())
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	independentDTDNoLive(t, a)
}

func TestDTDPkgIndependentExactIntrinsicRowsAcceptOnlyTheirTargets(t *testing.T) {
	ctx := context.Background()
	for _, pkg := range []struct {
		id, entry validation.ProfileID
		models    bool
	}{{independentDTDGraphProfile, ProfileResourceGraphDictionaries.ID(), true}, {independentDTDRawMXLProfile, ProfileMXLSourceDictionaries.ID(), false}, {independentDTDCurrentMXLProfile, ProfileMXLModelDictionaries.ID(), true}} {
		for _, row := range []struct {
			assembly validation.Assembly
			profile  validation.ProfileID
			xml      string
			model    bool
		}{
			{validation.AssemblyMusicXML, ProfileSourceInternalDTDDictionaries.ID(), `<score-partwise version="4.0"><part-list/></score-partwise>`, false},
			{validation.AssemblyOpus, ProfileSourceInternalDTDDictionaries.ID(), `<opus/>`, false},
			{validation.AssemblyContainer, independentDTDContainerProfile, `<container><rootfiles><rootfile full-path="main.xml"/></rootfiles></container>`, false},
			{validation.AssemblySounds, independentDTDSoundsProfile, `<sounds><sound id="custom"/></sounds>`, false},
			{validation.AssemblyMusicXML, ProfileModelDictionaries.ID(), `<score-partwise version="4.0"><part-list/></score-partwise>`, true},
			{validation.AssemblyOpus, ProfileModelDictionaries.ID(), `<opus/>`, true},
		} {
			t.Run(fmt.Sprintf("%s/%s/model=%v", pkg.id, row.assembly, row.model), func(t *testing.T) {
				g, err := startDictionaryResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(pkg.id)}}.internal(), DictionaryOptions{}, pkg.entry)
				if err != nil {
					t.Fatal(err)
				}
				a := g.options.account
				var engine *validation.Engine
				target := validation.TargetSource
				if row.model {
					target = validation.TargetModel
					engine, err = modelAssessmentEngineForProfile(row.profile)
				} else {
					engine, err = sourceAssessmentEngineForProfile(row.profile)
				}
				if err != nil {
					t.Fatal(err)
				}
				exact, ok := engine.Profile(row.profile)
				if !ok || exact.CatalogDigest == "" {
					t.Fatal("missing exact intrinsic engine profile")
				}
				cfg := assessmentRunConfig{Context: validation.ContextFamilyAll, Account: a, DocumentID: "exact-row", Profile: row.profile, Engine: engine, Dictionaries: g.dictionaries}
				var out resourceIntrinsicResult
				pin := validation.ResourcePin{CanonicalID: "exact-row", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(row.xml)))}
				if row.model {
					doc, err := Decode(strings.NewReader(row.xml))
					if err != nil {
						t.Fatal(err)
					}
					out = assessResourceModel(ctx, doc, cfg)
					pin.Representation = validation.ResourceCurrentModel
					pin.Digest = ""
					pin.SnapshotID = "current"
				} else {
					out = assessResourceSource(ctx, strings.NewReader(row.xml), sourceAssessmentConfig{assessmentRunConfig: cfg, ByteOwner: sourceBytesPrimary})
				}
				if out.Err != nil || out.Snapshot == nil {
					t.Fatal(out.Err)
				}
				h := out.Snapshot.Header()
				if h.Assembly.State != validation.EvidenceKnown || h.Assembly.Value != row.assembly || h.Target != target || h.Profile != row.profile || h.CatalogDigest != exact.CatalogDigest {
					t.Fatalf("wrong intrinsic interpretation: %+v", h)
				}
				id, err := validationPackageSessionAddResource(g.session, ctx, pin, out.Snapshot, true)
				if row.model && !pkg.models {
					if err == nil || id != "" {
						t.Fatal("raw source package admitted current model")
					}
					_, _ = g.finish(err)
					if out.Snapshot.Header().DocumentID != "exact-row" {
						t.Fatal("rejected model snapshot lost ownership")
					}
					if err := out.Snapshot.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					if err != nil || id == "" {
						t.Fatalf("exact row refused: %v", err)
					}
					if err := out.Snapshot.Close(); err == nil {
						t.Fatal("successful transfer left caller release ownership")
					}
					b, err := g.finish(nil)
					if err != nil {
						t.Fatal(err)
					}
					if b.ResourceCount() != 1 {
						t.Fatalf("accepted rows=%d", b.ResourceCount())
					}
				}
				independentDTDNoLive(t, a)
			})
		}
	}
}
