package musicxml

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDTDPackageCompositionPreflightBeforeIO(t *testing.T) {
	ctx := context.Background()
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		name     string
		old, new validation.ProfileID
	}{
		{"source", ProfileResourceGraphDictionaries.ID(), dtdGraphProfile}, {"model", ProfileResourceGraphDictionaries.ID(), dtdGraphProfile},
		{"raw", ProfileMXLSourceDictionaries.ID(), dtdMXLSourceProfile}, {"current", ProfileMXLModelDictionaries.ID(), dtdMXLModelProfile},
	} {
		for _, profile := range []validation.ProfileID{"", "unknown", ProfileResourceGraph.ID(), ProfileMXLSource.ID(), ProfileMXLModel.ID(), dtdGraphProfile, dtdMXLSourceProfile, dtdMXLModelProfile, ProfileResourceGraphDictionaries.ID(), ProfileMXLSourceDictionaries.ID(), ProfileMXLModelDictionaries.ID()} {
			if profile == entry.old || profile == entry.new {
				continue
			}
			t.Run(entry.name+"/"+string(profile), func(t *testing.T) {
				calls := 0
				r := &dictionaryGraphReader{}
				source := dictionaryGraphSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
					calls++
					panic("preflight performed dictionary I/O")
				})
				resolver := graphResolverFunc(func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
					panic("preflight performed resolver I/O")
				})
				resources := LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(profile)}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}
				d := DictionaryOptions{Source: source, Selections: dictionaryGraphSelection()}
				e := dtdCompositionRunEntry(ctx, entry.name, r, doc, resources, d, MXLOptions{})
				if !errors.Is(e, validation.ErrInvalidConfiguration) || calls != 0 || r.reads != 0 {
					t.Fatalf("error=%v reads=%d calls=%d", e, r.reads, calls)
				}
			})
		}
		for _, profile := range []validation.ProfileID{entry.old, entry.new} {
			for _, bad := range []string{"budget", "dictionary", "dependencies", "MXL"} {
				if bad == "MXL" && entry.name != "raw" && entry.name != "current" {
					continue
				}
				t.Run(entry.name+"/"+string(profile)+"/"+bad, func(t *testing.T) {
					calls := 0
					r := &dictionaryGraphReader{}
					resources := LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(profile)}}
					d := DictionaryOptions{Selections: dictionaryGraphSelection(), Source: dictionaryGraphSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
						calls++
						panic("invalid options imported dictionary")
					})}
					mxl := MXLOptions{}
					switch bad {
					case "budget":
						resources.Graph.Budgets.MaxRuleSteps = -1
					case "dictionary":
						d.Selections[0].Manifest = "unknown"
					case "dependencies":
						resources.Dependencies = []ResourceDependency{"invalid"}
					case "MXL":
						mxl.MaxXMLDepth = -1
					}
					e := dtdCompositionRunEntry(ctx, entry.name, r, doc, resources, d, mxl)
					if e == nil || calls != 0 || r.reads != 0 {
						t.Fatalf("error=%v reads=%d calls=%d", e, r.reads, calls)
					}
				})
			}
		}
	}
}

func dtdCompositionRunEntry(ctx context.Context, entry string, r *dictionaryGraphReader, doc Document, resources LinkedAssessmentOptions, d DictionaryOptions, mxl MXLOptions) error {
	var err error
	linked := linkedDictionaryAssessmentOptions{Resources: resources.internal(), Dictionaries: d}
	packaged := mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(resources.Graph.Profile), Budgets: resources.Graph.Budgets, DTD: resources.Graph.DTD, Dictionaries: resources.Graph.Dictionaries}, Identity: resources.Identity, Resolver: resources.Resolver, Dependencies: resources.Dependencies, MXL: mxl}.internal(), Dictionaries: d}
	switch entry {
	case "source":
		_, err = inspectXMLWithResourcesAndDictionaries(ctx, r, linked)
	case "model":
		_, err = assessModelWithResourcesAndDictionaries(ctx, doc, linked)
	case "raw":
		_, err = inspectMXLWithDictionaries(ctx, r, packaged)
	case "current":
		_, err = assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, packaged)
	}
	return err
}

func TestDTDPackageCompositionPinsExactIntrinsicTable(t *testing.T) {
	_, profiles, err := buildDTDDictionaryPackageAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Fatal("profile count", len(profiles))
	}
	for _, binding := range profiles {
		want := map[[2]string]validation.ProfileID{
			{string(validation.AssemblyMusicXML), string(validation.TargetSource)}:  ProfileSourceInternalDTDDictionaries.ID(),
			{string(validation.AssemblyOpus), string(validation.TargetSource)}:      ProfileSourceInternalDTDDictionaries.ID(),
			{string(validation.AssemblyContainer), string(validation.TargetSource)}: dtdContainerProfile,
			{string(validation.AssemblySounds), string(validation.TargetSource)}:    dtdSoundsProfile,
		}
		if binding.Profile.ID != dtdMXLSourceProfile {
			want[[2]string{string(validation.AssemblyMusicXML), string(validation.TargetModel)}] = ProfileModelDictionaries.ID()
			want[[2]string{string(validation.AssemblyOpus), string(validation.TargetModel)}] = ProfileModelDictionaries.ID()
		}
		if len(binding.Intrinsic) != len(want) {
			t.Fatal("intrinsic count", binding.Profile.ID, len(binding.Intrinsic))
		}
		for _, row := range binding.Intrinsic {
			key := [2]string{string(row.Assembly), string(row.Target)}
			if row.Profile != want[key] {
				t.Fatalf("wrong exact profile %+v", row)
			}
			var e *validation.Engine
			if row.Target == validation.TargetSource {
				e, err = sourceAssessmentEngineForProfile(row.Profile)
			} else {
				e, err = modelAssessmentEngineForProfile(row.Profile)
			}
			if err != nil {
				t.Fatal(err)
			}
			p, ok := e.Profile(row.Profile)
			if !ok || p.CatalogDigest != row.CatalogDigest {
				t.Fatal("unpinned intrinsic", row)
			}
			delete(want, key)
		}
		if len(want) != 0 {
			t.Fatal("missing rows", want)
		}
	}
}

func TestDTDPackageCompositionEffectiveContainerAndRetainedChildren(t *testing.T) {
	container := `<!DOCTYPE container [<!ATTLIST rootfile full-path CDATA "main.opus" media-type CDATA "application/vnd.recordare.musicxml+xml">]><container><rootfiles><rootfile/></rootfiles></container>`
	root := `<!DOCTYPE opus [<!ATTLIST opus-link xlink:href CDATA "child.opus">]><opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link/><opus-link/></opus>`
	child := `<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`
	wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}, {name: "main.opus", body: root}, {name: "child.opus", body: child}}, nil, "")
	b, err := inspectMXLWithDictionaries(context.Background(), bytes.NewReader(wire), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(dtdMXLSourceProfile)}}.internal()})
	if err != nil || b.PackageReport().Conformance == validation.ConformanceViolated || b.ResourceCount() != 3 || b.AdoptionCount() != 4 || !b.EnumerationComplete() {
		t.Fatalf("err=%v resources=%d adoptions=%d package=%+v", err, b.ResourceCount(), b.AdoptionCount(), b.PackageReport())
	}
	found := false
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		if r.Profile.ID != dtdContainerProfile {
			continue
		}
		for _, row := range r.Context {
			if row.Container != nil && row.Container.Role == validation.ContainerRootfile {
				found = true
				c := row.Container
				if c.FullPathRaw.State != validation.EvidenceAbsent || c.FullPath.Value.Text != "main.opus" || c.MediaTypeRaw.State != validation.EvidenceAbsent || c.MediaType.Value.Text != "application/vnd.recordare.musicxml+xml" {
					t.Fatalf("effective routing reconstructed Raw: %+v", c)
				}
			}
		}
	}
	if !found {
		t.Fatal("missing source container")
	}
}

func TestDTDPackageCompositionGraphDefaultLinksAndNoFetch(t *testing.T) {
	source, calls, bodies := dictionaryGraphKnownSource(t)
	root := `<!DOCTYPE opus [<!ATTLIST score xlink:href CDATA "child.musicxml">]><opus xmlns:xlink="http://www.w3.org/1999/xlink"><score/><score/></opus>`
	child := `<!DOCTYPE score-partwise [<!ENTITY glyph "accidentalSharp">]>` + resourceScore(`<note><rest/><duration>1</duration><accidental smufl="&glyph;">sharp</accidental></note>`)
	engine, e := sourceAssessmentEngineForProfile(ProfileSourceInternalDTDDictionaries.ID())
	if e != nil {
		t.Fatal(e)
	}
	profile, _ := engine.Profile(ProfileSourceInternalDTDDictionaries.ID())
	fixture := &graphFixtureResolver{resources: map[string]string{"https://dtd.test/child.musicxml": child}, pins: map[string]string{"https://dtd.test/child.musicxml": "child"}}
	resolver := graphResolverFunc(func(ctx context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		if q.Document == nil || q.Document.RequiredProfile != profile.ID || q.Document.CatalogDigest != profile.CatalogDigest {
			t.Fatalf("wrong interpretation %+v", q)
		}
		return fixture.Resolve(ctx, q)
	})
	options := LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(dtdGraphProfile)}, Identity: ResourceIdentity{RetrievalURI: "https://dtd.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}
	g, err := startDictionaryResourceGraph(context.Background(), options.internal(), DictionaryOptions{Source: source, Selections: dictionaryGraphSelection()}, ProfileResourceGraphDictionaries.ID())
	if err != nil {
		t.Fatal(err)
	}
	b, err := g.inspectPrimarySource(strings.NewReader(root))
	if err != nil || !b.Satisfied() || b.ResourceCount() != 2 || b.AdoptionCount() != 3 || *calls != 1 {
		t.Fatalf("err=%v resources=%d adoptions=%d calls=%d package=%+v", err, b.ResourceCount(), b.AdoptionCount(), *calls, b.PackageReport())
	}
	_, r, _ := b.ResourceReport(1)
	dictionaryGraphRequireCanonical(t, r, validation.EvidenceKnown, true)
	for _, body := range append(*bodies, fixture.bodies...) {
		if body.closed != 1 {
			t.Fatal("body closes", body.closed)
		}
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, e := g.options.account.Usage(kind)
		if e != nil || used != 0 {
			t.Fatal("live gauge", kind, used, e)
		}
	}
	for _, dependencies := range [][]ResourceDependency{nil, {ResourceDocuments}} {
		calls := 0
		opts := linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(dtdGraphProfile)}, Dependencies: dependencies, Resolver: graphResolverFunc(func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
			calls++
			panic("DTD/schema hint fetched")
		})}.internal()}
		input := `<!DOCTYPE opus SYSTEM "https://untrusted.test/external.dtd" [<!ENTITY remote SYSTEM "https://untrusted.test/entity">]><opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="https://untrusted.test/schema"/>`
		b, e := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), opts)
		if e != nil || calls != 0 || b.Satisfied() {
			t.Fatalf("external gap: err=%v calls=%d satisfied=%v", e, calls, b.Satisfied())
		}
	}
}

func TestDTDPackageCompositionModelPrimaryAndRetainedSourceHonesty(t *testing.T) {
	ctx := context.Background()
	child := `<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`
	for _, mode := range []string{"graph", "current MXL"} {
		t.Run(mode, func(t *testing.T) {
			var b validation.AssessmentBundle
			var err error
			if mode == "graph" {
				doc, e := Decode(strings.NewReader(`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://dtd.test/child.opus"/></opus>`))
				if e != nil {
					t.Fatal(e)
				}
				resolver := &graphFixtureResolver{resources: map[string]string{"https://dtd.test/child.opus": child}, pins: map[string]string{"https://dtd.test/child.opus": "child"}}
				b, err = assessModelWithResourcesAndDictionaries(ctx, doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(dtdGraphProfile)}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}.internal()})
			} else {
				doc, e := Decode(strings.NewReader(`<opus/>`))
				if e != nil {
					t.Fatal(e)
				}
				value := &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}, {FullPath: "child.opus"}}, Resources: []MXLResource{{Path: "child.opus", Data: []byte(child)}}}
				b, err = assessMXLPackageWithDictionaries(ctx, value, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(dtdMXLModelProfile)}}.internal()})
				if string(value.Resources[0].Data) != child || len(value.RootFiles) != 2 {
					t.Fatal("current input mutated")
				}
			}
			if err != nil || b.ResourceCount() != 2 || !b.EnumerationComplete() {
				t.Fatal("children unavailable", err, b.ResourceCount())
			}
			model, source := false, false
			for i := 0; i < b.ResourceCount(); i++ {
				_, r, _ := b.ResourceReport(i)
				switch r.Target {
				case validation.TargetModel:
					model = true
					if r.Profile.ID != ProfileModelDictionaries.ID() {
						t.Fatal("model interpretation changed", r.Profile.ID)
					}
					for _, row := range r.Context {
						if row.SourceOrigin != nil || row.Container != nil {
							t.Fatal("current model invented source evidence")
						}
						if row.Resource != nil && row.Resource.SchemaHints != nil && row.Resource.SchemaHints.Raw.State != validation.EvidenceUnsupported {
							t.Fatal("model reconstructed discarded hints")
						}
					}
				case validation.TargetSource:
					source = true
					if r.Profile.ID != ProfileSourceInternalDTDDictionaries.ID() {
						t.Fatal("unexpected auxiliary or source profile", r.Profile.ID)
					}
					found := false
					for _, row := range r.Context {
						if row.SourceOrigin != nil && row.SourceOrigin.Kind == validation.SourceOriginDefault {
							found = true
						}
					}
					if !found {
						t.Fatal("retained XML lost independent DTD history")
					}
				}
			}
			if !model || !source {
				t.Fatal("representation collapsed")
			}
		})
	}
}

func TestDTDPackageCompositionNestedAliasesPreserveBoundariesAndOrigins(t *testing.T) {
	child := `<!DOCTYPE score-partwise [<!ENTITY glyph "accidentalSharp">]>` + resourceScore(`<note><rest/><duration>1</duration><accidental smufl="&glyph;">sharp</accidental></note>`)
	container := `<!DOCTYPE container [<!ATTLIST rootfile full-path CDATA "main.musicxml">]><container><rootfiles><rootfile/></rootfiles></container>`
	archive := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}, {name: "main.musicxml", body: child}}, nil, "")
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://dtd.test/a.mxl"/><score xlink:href="https://dtd.test/b.mxl"/></opus>`
	resolver := &packageResolverFixture{data: map[string][]byte{"https://dtd.test/a.mxl": archive, "https://dtd.test/b.mxl": archive}, canonical: map[string]string{"https://dtd.test/a.mxl": "same-archive", "https://dtd.test/b.mxl": "same-archive"}}
	source, calls, bodies := dictionaryGraphKnownSource(t)
	b, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(root), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(dtdGraphProfile)}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}.internal(), Dictionaries: DictionaryOptions{Source: source, Selections: dictionaryGraphSelection()}})
	if err != nil || *calls != 1 || resolver.calls != 2 || b.ResourceCount() != 3 || !b.EnumerationComplete() {
		t.Fatal("nested traversal", err, *calls, resolver.calls, b.ResourceCount())
	}
	requireIndirectionContextUnknown(t, b, 2)
	boundaries := map[string]validation.PackageID{}
	documents := map[string]bool{}
	for i := 0; i < b.AdoptionCount(); i++ {
		c, _ := b.AdoptionContext(i)
		_, doc, _ := b.Adoption(i)
		if documents[doc] {
			t.Fatal("adoption identity reused", doc)
		}
		documents[doc] = true
		if c.ArchiveEntry == "main.musicxml" {
			boundaries[c.BaseURI] = c.Package
		}
	}
	if boundaries["https://dtd.test/a.mxl"] == "" || boundaries["https://dtd.test/b.mxl"] == "" || boundaries["https://dtd.test/a.mxl"] == boundaries["https://dtd.test/b.mxl"] {
		t.Fatal("base/package identities collapsed", boundaries)
	}
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		origins := 0
		for _, row := range r.Context {
			if row.SourceOrigin != nil {
				origins++
			}
		}
		if origins == 0 {
			t.Fatal("nested source origins omitted", r.Profile.ID)
		}
	}
	for _, body := range append(*bodies, resolver.bodies...) {
		if body.closed != 1 {
			t.Fatal("body ownership", body.closed)
		}
	}
}

func TestDTDPackageCompositionUnavailableDictionaryKeepsIndependentFailures(t *testing.T) {
	plain := resourceScore(`<note><rest/><duration>1</duration><accidental smufl="restQuarter">sharp</accidental></note><note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`)
	input := `<!DOCTYPE score-partwise [<!ATTLIST score-partwise version CDATA "4.0">]>` + strings.Replace(plain, ` version="4.0"`, "", 1)
	doc, err := Decode(strings.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("dictionary provider offline")
	for _, mode := range []string{"source", "model", "raw", "current"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			body := &resourceTestBody{Reader: strings.NewReader("untrusted partial")}
			d := DictionaryOptions{Selections: dictionaryGraphSelection(), Source: dictionaryGraphSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				return DictionaryComponentResult{Request: q, State: validation.EvidenceUnknown, Reason: "offline", Body: body}, boom
			})}
			var b validation.AssessmentBundle
			var e error
			switch mode {
			case "source":
				b, e = inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(dtdGraphProfile)}}.internal(), Dictionaries: d})
			case "model":
				b, e = assessModelWithResourcesAndDictionaries(context.Background(), doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: GraphProfile(dtdGraphProfile)}}.internal(), Dictionaries: d})
			case "raw":
				b, e = inspectMXLWithDictionaries(context.Background(), bytes.NewReader(packageGraphBytes(input, "main.musicxml", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(dtdMXLSourceProfile)}}.internal(), Dictionaries: d})
			case "current":
				b, e = assessMXLPackageWithDictionaries(context.Background(), &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: MXLProfile(dtdMXLModelProfile)}}.internal(), Dictionaries: d})
			}
			if !errors.Is(e, boom) || calls != 1 || body.closed != 1 || !b.EnumerationComplete() {
				t.Fatalf("err=%v calls=%d closes=%d enumeration=%t", e, calls, body.closed, b.EnumerationComplete())
			}
			found := false
			for i := 0; i < b.ResourceCount(); i++ {
				_, r, _ := b.ResourceReport(i)
				if r.Profile.ID == dtdContainerProfile {
					continue
				}
				found = true
				if r.Conformance != validation.ConformanceViolated || r.Counts.Fail == 0 || r.AssessmentComplete || r.Terminal != nil || !r.InputComplete {
					t.Fatalf("independent failure lost: %+v", r)
				}
				dictionaryGraphRequireCanonical(t, r, validation.EvidenceUnknown, false)
			}
			if !found {
				t.Fatal("musical report missing")
			}
		})
	}
}
