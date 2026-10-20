package musicxml

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// These tests compose fresh intrinsic definitions and the public package core.
// They intentionally do not replace cached engines or relax the production
// adapter's built-in-engine identity check.
func fragmentCompositionEngine(t *testing.T, sourceInfoset bool) *validation.Engine {
	t.Helper()
	var d *assessmentDefinitions
	var err error
	if sourceInfoset {
		d, err = buildSourceCompositionDefinitions()
	} else {
		d, err = buildLinkAssessmentDefinitions()
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = electXMLFragmentBaseline(d, sourceInfoset); err != nil {
		t.Fatal(err)
	}
	catalog, err := newCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := validation.NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func fragmentCompositionAccount(t *testing.T) *validation.Account {
	t.Helper()
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		used, err := account.Usage(validation.BudgetBufferedBytes)
		if err != nil || used != 0 {
			t.Errorf("fragment composition retained %d bytes: %v", used, err)
		}
	})
	return account
}

type fragmentCompositionResource struct {
	snapshot *validation.ResourceSnapshot
	pin      validation.ResourcePin
	report   validation.Report
}

func fragmentCompositionSource(t *testing.T, engine *validation.Engine, profile validation.ProfileID, account *validation.Account, documentID, source string, sourceInfoset bool) fragmentCompositionResource {
	t.Helper()
	ctx := context.Background()
	session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: documentID, Profile: profile, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	run := &sourceAssessmentRun{ctx: ctx, account: account, session: session, documentID: documentID}
	finished := false
	defer func() {
		if !finished {
			_, _ = run.finish(nil)
		}
	}()
	dtdIDs := validationEngineSourceDTDIDsSelected(engine, profile)
	lexical, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{
		Context: ctx, Account: account, ByteOwner: sourceBytesPrimary,
		InternalDTD: sourceInfoset, CaptureAttributeLexical: sourceInfoset, CaptureDTDIDs: dtdIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	run.source = &observingXMLSource{ctx: ctx, lexical: lexical, decoder: xml.NewTokenDecoder(lexical)}
	observer, err := newSourceFactObserverWithIdentity(ctx, session, account, documentID, "")
	if err != nil {
		t.Fatal(err)
	}
	run.source.observer = observer
	observer.sourceDeclarations, observer.sourceDTDIDs = sourceInfoset, dtdIDs
	observer.captureAttributeLexical = sourceInfoset
	if sourceInfoset {
		if err = account.Charge(validation.BudgetBufferedBytes, 512); err != nil {
			t.Fatal(err)
		}
		observer.retained += 512
		observer.origins = &sourceOriginObserver{engine: engine, session: session, profile: profile}
	}
	cause := run.drain()
	completion, err := run.finishInput(cause)
	if err != nil {
		t.Fatal(err)
	}
	finished = true
	report, snapshot, err := session.FinishWithResourceSnapshot(ctx, completion)
	if err != nil || cause != nil || snapshot == nil {
		t.Fatalf("source snapshot: input=%v finish=%v snapshot=%v", cause, err, snapshot)
	}
	t.Cleanup(func() { _ = snapshot.Close() })
	if !report.InputComplete || report.Syntax != validation.SyntaxWellFormed {
		t.Fatalf("source did not reach its real EOF: %+v", report)
	}
	return fragmentCompositionResource{snapshot, validation.ResourcePin{
		CanonicalID: documentID, Representation: validation.ResourceSourceXML,
		Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(source))),
	}, report}
}

func fragmentCompositionModel(t *testing.T, engine *validation.Engine, account *validation.Account, documentID string, document Document) fragmentCompositionResource {
	t.Helper()
	ctx := context.Background()
	session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: documentID, Profile: ProfileModelLinks.ID(), Account: account})
	if err != nil {
		t.Fatal(err)
	}
	status, cause := walkModel(ctx, document, session, account, documentID, modelWalkOptions{})
	report, snapshot, err := session.FinishWithResourceSnapshot(ctx, validation.Completion{InputComplete: cause == nil, Syntax: validation.SyntaxNotAssessed, Cause: cause})
	if err != nil || cause != nil || snapshot == nil || status.HasProblems {
		t.Fatalf("model snapshot: status=%+v walk=%v finish=%v", status, cause, err)
	}
	t.Cleanup(func() { _ = snapshot.Close() })
	return fragmentCompositionResource{snapshot, validation.ResourcePin{
		CanonicalID: documentID, Representation: validation.ResourceCurrentModel, SnapshotID: documentID,
	}, report}
}

type fragmentCompositionPackage struct {
	session   *validation.PackageSession
	adoptions []validation.ResourceAdoptionID
	finished  bool
}

func newFragmentCompositionPackage(t *testing.T, account *validation.Account, resources ...fragmentCompositionResource) *fragmentCompositionPackage {
	t.Helper()
	plan, err := validation.NewPackagePlan(validation.PackageCheckLinkTarget)
	if err != nil {
		t.Fatal(err)
	}
	key := validation.RuleKey{ID: "test.xml-fragment-link-target", Revision: 1}
	rule := validation.PackageRuleBinding{
		Rule: validation.RuleDefinition{Key: key, Scope: validation.ScopePackage, Targets: []validation.Target{validation.TargetPackage},
			Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative,
			Predicate: string(key.ID), Text: validation.RuleText{Title: "XML fragment target", Explanation: "Assess the direct adopted target without fetching.", Message: "Fragment target unavailable."},
			Sources: []validation.SourceRef{{IDs: []string{"XPointer-element"}, URL: "https://www.w3.org/TR/xptr-element/", Locator: "Section 3"}},
		}, Selector: validation.PackageSelector{Kind: validation.PackageSubjectEdge}, Plan: plan,
	}
	profile := validation.PackageProfileBinding{Profile: validation.Profile{
		ID: "test.xml-fragment-package", MusicXMLVersion: "4.0", RegistryCommit: assessmentRegistryCommit, Target: validation.TargetPackage,
		Required: []validation.RuleKey{key}, Support: []validation.RuleSupport{{Rule: key, State: validation.ImplementationAvailable}},
		NormativeClosure: "Only direct XML fragment target interpretation and explicit nested-indirection gating; no retrieval.",
	}}
	seen := make(map[validation.IntrinsicProfileBinding]bool)
	for _, resource := range resources {
		h := resource.snapshot.Header()
		row := validation.IntrinsicProfileBinding{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}
		if !seen[row] {
			profile.Intrinsic = append(profile.Intrinsic, row)
			seen[row] = true
		}
	}
	catalog, err := validation.NewPackageCatalog([]validation.PackageRuleBinding{rule}, []validation.PackageProfileBinding{profile})
	if err != nil {
		t.Fatal(err)
	}
	session, err := validationCompiledPackageCatalogBegin(catalog, context.Background(), validation.RunOptions{DocumentID: "fragment-package", Profile: profile.Profile.ID, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	p := &fragmentCompositionPackage{session: session}
	t.Cleanup(func() {
		if !p.finished {
			_, _ = validationPackageSessionFinish(session, context.Background(), validation.PackageCompletion{})
		}
	})
	if err = validationPackageSessionOpenPackage(session, context.Background(), validation.PackageIdentity{ID: "outer", Representation: validation.ResourceSourceXML,
		Pin: validation.ResourcePin{CanonicalID: "fragment-boundary", Representation: validation.ResourceSourceXML, Digest: strings.Repeat("a", 64)},
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

func (p *fragmentCompositionPackage) add(t *testing.T, resource fragmentCompositionResource) validation.ResourceAssessmentID {
	t.Helper()
	id, err := validationPackageSessionAddResource(p.session, context.Background(), resource.pin, resource.snapshot, true)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (p *fragmentCompositionPackage) adopt(t *testing.T, resource validation.ResourceAssessmentID, documentID string, root bool) (validation.ResourceAdoptionID, *validation.ResourceAdoption) {
	t.Helper()
	id, err := validationPackageSessionAdoptResource(p.session, context.Background(), resource, validation.ResourceAdoptionOptions{Package: "outer", DocumentID: documentID, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	p.adoptions = append(p.adoptions, id)
	view, err := validationPackageSessionAdoption(p.session, context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return id, view
}

func (p *fragmentCompositionPackage) finish(t *testing.T) validation.AssessmentBundle {
	t.Helper()
	ctx := context.Background()
	complete := validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
	sealed := make(map[validation.ResourceAdoptionID]bool)
	for _, id := range p.adoptions {
		owner, err := validationPackageSessionExpansionOwner(p.session, ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if !sealed[owner] {
			if err = validationPackageSessionSealResourceExpansion(p.session, ctx, owner, complete); err != nil {
				t.Fatal(err)
			}
			sealed[owner] = true
		}
	}
	if err := validationPackageSessionSealEnumeration(p.session, ctx, validation.PackageEnumeration{Package: "outer", Entries: complete, Rootfiles: complete, Edges: complete, Roles: complete}); err != nil {
		t.Fatal(err)
	}
	p.finished = true
	bundle, err := validationPackageSessionFinish(p.session, ctx, validation.PackageCompletion{InputComplete: true})
	if err != nil || !bundle.EnumerationComplete() {
		t.Fatalf("package finish: complete=%t err=%v", bundle.EnumerationComplete(), err)
	}
	return bundle
}

func fragmentCompositionResolve(t *testing.T, view *validation.ResourceAdoption, pointer string, state validation.EvidenceState, node validation.NodeID) validation.Evidence[validation.NodeRef] {
	t.Helper()
	got, selected, err := view.ResolveXMLFragment(context.Background(), pointer)
	if err != nil || !selected || got.State != state || got.Value.Node != node {
		t.Fatalf("fragment %q: selected=%t result=%+v err=%v; want %s node %d", pointer, selected, got, err, state, node)
	}
	if state == validation.EvidenceKnown && got.Value.DocumentID != view.Header().DocumentID {
		t.Fatalf("fragment %q escaped adoption identity: %+v", pointer, got)
	}
	return got
}

const fragmentCompositionScore = `<score-partwise>
  <!-- before first child --><?before work?>
  <work><work-number>10</work-number><!-- between children --><work-title>Ti<?inside value?>tle</work-title></work>
  <movement-number>1</movement-number><?between movements?><movement-title>First</movement-title>
  <part-list><score-part id="P1"><part-name>Part</part-name></score-part></part-list>
  <!-- before last child --><part id="P1"><measure number="1"/></part>
</score-partwise>`

func TestXMLFragmentCompositionSourceOrderAndRepeatedAdoption(t *testing.T) {
	engine := fragmentCompositionEngine(t, false)
	account := fragmentCompositionAccount(t)
	const rootXML = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="target.musicxml#element(/1/4/1/1)"/><score xlink:href="target.musicxml#element(/1/4/1/1)"/></opus>`
	root := fragmentCompositionSource(t, engine, ProfileSourceLinks.ID(), account, "source-order-root", rootXML, false)
	target := fragmentCompositionSource(t, engine, ProfileSourceLinks.ID(), account, "source-order-target", fragmentCompositionScore, false)
	canonicalTarget := target.snapshot.Header()
	// Node 9 is the part-name scalar leaf. It has neither an ID nor a retained
	// resource-context projection, so an ID/context-only transport cannot find it.
	for i := 0; i < target.snapshot.IDCount(); i++ {
		id, ok, err := target.snapshot.IDAt(context.Background(), i)
		if err != nil || !ok || id.Subject.Node == 9 {
			t.Fatalf("fixture ID unexpectedly projects part-name: %+v %v", id, err)
		}
	}
	for i := 0; i < target.snapshot.ContextCount(); i++ {
		row, ok, err := target.snapshot.ContextAt(context.Background(), i)
		if err != nil || !ok || row.Subject.Node == 9 {
			t.Fatalf("fixture context unexpectedly projects part-name: %+v %v", row, err)
		}
	}
	p := newFragmentCompositionPackage(t, account, root, target)
	rootID, sourceView := p.adopt(t, p.add(t, root), root.pin.CanonicalID, true)
	targetID := p.add(t, target)
	firstID, first := p.adopt(t, targetID, "target-first", false)
	secondID, second := p.adopt(t, targetID, "target-second", false)
	for _, tc := range []struct {
		pointer string
		node    validation.NodeID
	}{
		{"element(/1)", 1}, {"element(/1/1)", 2}, {"element(/1/1/1)", 3}, {"element(/1/1/2)", 4},
		{"element(/1/2)", 5}, {"element(/1/3)", 6}, {"element(/1/4)", 7}, {"element(/1/4/1)", 8},
		{"element(/1/4/1/1)", 9}, {"element(/1/5)", 10}, {"element(/1/5/1)", 11},
		{"unknown(a^(b^)c^^d)element(/1/4/1/1)", 9}, {"element(/1/99) element(/1/4/1/1)", 9},
	} {
		fragmentCompositionResolve(t, first, tc.pointer, validation.EvidenceKnown, tc.node)
	}
	fragmentCompositionResolve(t, first, "element(/1/6)", validation.EvidenceAbsent, 0)
	fragmentCompositionResolve(t, first, "element(/1/1/3)", validation.EvidenceAbsent, 0)
	a := fragmentCompositionResolve(t, first, "element(/1/4/1/1)", validation.EvidenceKnown, 9)
	b := fragmentCompositionResolve(t, second, "element(/1/4/1/1)", validation.EvidenceKnown, 9)
	if a.Value.DocumentID == b.Value.DocumentID || canonicalTarget.DocumentID != "source-order-target" {
		t.Fatalf("repeated adoption changed canonical or occurrence identity: %+v %+v", a, b)
	}
	// Mutating an owned result must not alter either the other adoption or a
	// later result, and the public edge ingress must admit this unprojected leaf.
	a.Value.Node, a.Value.DocumentID = 1, "caller-mutated"
	fragmentCompositionResolve(t, first, "element(/1/4/1/1)", validation.EvidenceKnown, 9)
	edges := 0
	for i := 0; i < sourceView.ContextCount(); i++ {
		row, ok, err := sourceView.ContextAt(context.Background(), i)
		if err != nil || !ok {
			t.Fatalf("source template: %v", err)
		}
		if row.Resource == nil || row.Resource.Link == nil {
			continue
		}
		adoption, view := firstID, first
		if edges == 1 {
			adoption, view = secondID, second
		}
		resolved := fragmentCompositionResolve(t, view, "element(/1/4/1/1)", validation.EvidenceKnown, 9)
		err = validationPackageSessionObserveEdge(p.session, context.Background(), validation.PackageEdgeObservation{
			ID: validation.PackageFactID(edges + 1), Source: rootID, Template: uint64(i),
			Target:      validation.Evidence[validation.ResourceAdoptionID]{State: validation.EvidenceKnown, Value: adoption},
			Indirection: validation.Evidence[bool]{State: validation.EvidenceKnown, Value: false},
			Fragment:    validation.Evidence[string]{State: validation.EvidenceKnown, Value: "element(/1/4/1/1)"}, FragmentTarget: resolved,
		})
		if err != nil {
			t.Fatal(err)
		}
		edges++
	}
	bundle := p.finish(t)
	if edges != 2 || bundle.ResourceCount() != 2 || bundle.AdoptionCount() != 3 || !bundle.Satisfied() || bundle.PackageReport().Counts.Pass != 2 {
		t.Fatalf("public composition: edges=%d resources=%d adoptions=%d report=%+v", edges, bundle.ResourceCount(), bundle.AdoptionCount(), bundle.PackageReport())
	}
}

func TestXMLFragmentCompositionCurrentModelElementOrder(t *testing.T) {
	engine := fragmentCompositionEngine(t, false)
	account := fragmentCompositionAccount(t)
	document, err := Decode(strings.NewReader(fragmentCompositionScore))
	if err != nil {
		t.Fatal(err)
	}
	resource := fragmentCompositionModel(t, engine, account, "model-order", document)
	p := newFragmentCompositionPackage(t, account, resource)
	_, view := p.adopt(t, p.add(t, resource), resource.pin.CanonicalID, true)
	for _, tc := range []struct {
		pointer string
		node    validation.NodeID
	}{
		{"element(/1)", 1}, {"element(/1/1)", 2}, {"element(/1/1/1)", 3}, {"element(/1/1/2)", 4},
		{"element(/1/2)", 5}, {"element(/1/3)", 6}, {"element(/1/4)", 7}, {"element(/1/4/1)", 8},
		{"element(/1/4/1/1)", 9}, {"element(/1/5)", 10}, {"element(/1/5/1)", 11},
	} {
		fragmentCompositionResolve(t, view, tc.pointer, validation.EvidenceKnown, tc.node)
	}
	fragmentCompositionResolve(t, view, "element(/1/6)", validation.EvidenceAbsent, 0)
	fragmentCompositionResolve(t, view, "element(/1/1/3)", validation.EvidenceAbsent, 0)
	if resource.report.Syntax != validation.SyntaxNotAssessed {
		t.Fatal("model child order acquired source syntax evidence")
	}
	p.finish(t)
}

func TestXMLFragmentCompositionElementContentIDOwner(t *testing.T) {
	engine := fragmentCompositionEngine(t, false)
	account := fragmentCompositionAccount(t)
	const source = `<score-partwise xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"><movement-title xsi:type="xs:ID">RootID</movement-title><part-list><score-part id="P1"><part-name>Part</part-name></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	resource := fragmentCompositionSource(t, engine, ProfileSourceLinks.ID(), account, "element-id-source", source, false)
	found := false
	for i := 0; i < resource.snapshot.IDCount(); i++ {
		id, ok, err := resource.snapshot.IDAt(context.Background(), i)
		if err != nil || !ok {
			t.Fatalf("element-content ID: %v", err)
		}
		if id.Value == "RootID" {
			found = true
			if id.Subject.Node != 1 || id.Source.Node != 2 {
				t.Fatalf("element-content ID lost its parent owner: %+v", id)
			}
		}
	}
	if !found {
		t.Fatal("fixture did not establish the element-content xs:ID")
	}
	p := newFragmentCompositionPackage(t, account, resource)
	_, view := p.adopt(t, p.add(t, resource), resource.pin.CanonicalID, true)
	fragmentCompositionResolve(t, view, "RootID", validation.EvidenceKnown, 1)
	fragmentCompositionResolve(t, view, "element(RootID)", validation.EvidenceKnown, 1)
	fragmentCompositionResolve(t, view, "element(RootID/1)", validation.EvidenceKnown, 2)
	fragmentCompositionResolve(t, view, "element(RootID/2)", validation.EvidenceKnown, 3)
	p.finish(t)
}

func TestXMLFragmentCompositionURIIsDecodedOnce(t *testing.T) {
	engine := fragmentCompositionEngine(t, false)
	account := fragmentCompositionAccount(t)
	resource := fragmentCompositionSource(t, engine, ProfileSourceLinks.ID(), account, "uri-target", fragmentCompositionScore, false)
	p := newFragmentCompositionPackage(t, account, resource)
	_, view := p.adopt(t, p.add(t, resource), resource.pin.CanonicalID, true)
	for _, tc := range []struct {
		href, fragment string
		state          validation.EvidenceState
		node           validation.NodeID
	}{
		{"#element(%2F1%2F4%2F1%2F1)", "element(/1/4/1/1)", validation.EvidenceKnown, 9},
		{"#element(%252F1%252F4%252F1%252F1)", "element(%2F1%2F4%2F1%2F1)", validation.EvidenceAbsent, 0},
		{"#%50%31", "P1", validation.EvidenceKnown, 8},
		{"#%2550%2531", "%50%31", validation.EvidenceAbsent, 0},
		{"#unknown(%00)element(%2F1)", "unknown(\x00)element(/1)", validation.EvidenceKnown, 1},
		{"#unknown(a%5E%28b%5E%29)%20element(%2F1%2F3)", "unknown(a^(b^)) element(/1/3)", validation.EvidenceKnown, 6},
	} {
		resolved, err := resolveAssessmentResourceURI(context.Background(), account, resourceURIBase{RetrievalURI: "https://example.test/score.musicxml"}, tc.href)
		if err != nil || !resolved.SameEntity || !resolved.FragmentPresent || resolved.Fragment != tc.fragment || resolved.RetrievalURI != "https://example.test/score.musicxml" {
			t.Fatalf("URI %q: %+v %v", tc.href, resolved, err)
		}
		fragmentCompositionResolve(t, view, resolved.Fragment, tc.state, tc.node)
	}
	if bundle := p.finish(t); !bundle.Satisfied() {
		t.Fatalf("URI fixture did not satisfy intrinsic profile: %+v", bundle.PackageReport())
	}
}

func TestXMLFragmentCompositionSourceAndModelIdentityDomains(t *testing.T) {
	sourceEngine := fragmentCompositionEngine(t, true)
	modelEngine := fragmentCompositionEngine(t, false)
	for _, duplicate := range []bool{false, true} {
		name, number, bookmarks := "dtd-only-anchor", "anchor", `<bookmark id="B"/>`
		if duplicate {
			name, number, bookmarks = "first-owner-independent-of-xsd-validity", "B", `<bookmark id="B"/><bookmark id="B"/>`
		}
		t.Run(name, func(t *testing.T) {
			account := fragmentCompositionAccount(t)
			xmlSource := `<!DOCTYPE score-partwise [<!ATTLIST measure number ID #IMPLIED>]><score-partwise><part-list><score-part id="P1"><part-name>Part</part-name></score-part></part-list><part id="P1"><measure number="` + number + `">` + bookmarks + `</measure></part></score-partwise>`
			source := fragmentCompositionSource(t, sourceEngine, ProfileSourceInternalDTDLinks.ID(), account, "domain-source", xmlSource, true)
			document, err := Decode(strings.NewReader(xmlSource))
			if err != nil {
				t.Fatal(err)
			}
			model := fragmentCompositionModel(t, modelEngine, account, "domain-model", document)
			p := newFragmentCompositionPackage(t, account, source, model)
			_, sourceView := p.adopt(t, p.add(t, source), source.pin.CanonicalID, true)
			_, modelView := p.adopt(t, p.add(t, model), "current-model-view", false)
			fragmentCompositionResolve(t, sourceView, number, validation.EvidenceKnown, 6)
			fragmentCompositionResolve(t, sourceView, "element("+number+"/1)", validation.EvidenceKnown, 7)
			fragmentCompositionResolve(t, modelView, "element(/1/2/1/1)", validation.EvidenceKnown, 7)
			fragmentCompositionResolve(t, modelView, "B", validation.EvidenceKnown, 7)
			fragmentCompositionResolve(t, modelView, "element("+number+"/1)", validation.EvidenceAbsent, 0)
			if !duplicate {
				fragmentCompositionResolve(t, modelView, "anchor", validation.EvidenceAbsent, 0)
			} else if source.report.Conformance != validation.ConformanceViolated || model.report.Conformance != validation.ConformanceViolated {
				t.Fatalf("first-owner selection erased independent XSD duplicate-ID failure: source=%s model=%s", source.report.Conformance, model.report.Conformance)
			}
			if model.report.Syntax != validation.SyntaxNotAssessed {
				t.Fatal("current model acquired source syntax authority")
			}
			for _, row := range model.report.Context {
				if row.SourceOrigin != nil {
					t.Fatal("current model acquired historical source provenance")
				}
			}
			bundle := p.finish(t)
			if duplicate && bundle.Satisfied() {
				t.Fatal("successful fragment selection certified duplicate XSD IDs")
			}
		})
	}
}

func TestXMLFragmentCompositionOldProfilesAndIndirectionStayGated(t *testing.T) {
	plainEngine, err := newLinkAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	dtdEngine, err := sourceCompositionEngine()
	if err != nil {
		t.Fatal(err)
	}
	electedEngine := fragmentCompositionEngine(t, false)
	for _, tc := range []struct {
		name     string
		engine   *validation.Engine
		profile  validation.ProfileID
		source   string
		infoset  bool
		selected bool
	}{
		{"legacy-source", plainEngine, ProfileSourceLinks.ID(), `<opus/>`, false, false},
		{"joined-shorthand-source", dtdEngine, ProfileSourceInternalDTDLinks.ID(), `<!DOCTYPE opus [<!ATTLIST opus version ID #IMPLIED>]><opus version="anchor"/>`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := fragmentCompositionAccount(t)
			resource := fragmentCompositionSource(t, tc.engine, tc.profile, account, "old-profile", tc.source, tc.infoset)
			p := newFragmentCompositionPackage(t, account, resource)
			_, view := p.adopt(t, p.add(t, resource), resource.pin.CanonicalID, true)
			result, selected, err := view.ResolveXMLFragment(context.Background(), "element(/1)")
			if err != nil || selected != tc.selected || tc.selected && result.State != validation.EvidenceUnsupported {
				t.Fatalf("old profile elected Framework: selected=%t result=%+v err=%v", selected, result, err)
			}
			if tc.infoset {
				fragmentCompositionResolve(t, view, "anchor", validation.EvidenceKnown, 1)
			}
			p.finish(t)
		})
	}
	t.Run("legacy-model", func(t *testing.T) {
		account := fragmentCompositionAccount(t)
		resource := fragmentCompositionModel(t, plainEngine, account, "old-model-profile", &OpusDocument{})
		p := newFragmentCompositionPackage(t, account, resource)
		_, view := p.adopt(t, p.add(t, resource), resource.pin.CanonicalID, true)
		result, selected, err := view.ResolveXMLFragment(context.Background(), "element(/1)")
		if err != nil || selected {
			t.Fatalf("legacy model elected Framework: selected=%t result=%+v err=%v", selected, result, err)
		}
		p.finish(t)
	})
	oldProfile, _ := plainEngine.Profile(ProfileSourceLinks.ID())
	newProfile, _ := electedEngine.Profile(ProfileSourceLinks.ID())
	if oldProfile.CatalogDigest == newProfile.CatalogDigest {
		t.Fatal("new fragment interpretation did not change the intrinsic catalog digest")
	}
	t.Run("nested-compressed-destination", func(t *testing.T) {
		account := fragmentCompositionAccount(t)
		root := fragmentCompositionSource(t, electedEngine, ProfileSourceLinks.ID(), account, "indirect-root", `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="score.mxl#element(/1/4/1/1)"/></opus>`, false)
		target := fragmentCompositionSource(t, electedEngine, ProfileSourceLinks.ID(), account, "indirect-primary", fragmentCompositionScore, false)
		p := newFragmentCompositionPackage(t, account, root, target)
		sourceID, sourceView := p.adopt(t, p.add(t, root), root.pin.CanonicalID, true)
		targetID, targetView := p.adopt(t, p.add(t, target), "nested-primary-view", false)
		resolved := fragmentCompositionResolve(t, targetView, "element(/1/4/1/1)", validation.EvidenceKnown, 9)
		edges := 0
		for i := 0; i < sourceView.ContextCount(); i++ {
			row, ok, err := sourceView.ContextAt(context.Background(), i)
			if err != nil || !ok {
				t.Fatalf("source template: %v", err)
			}
			if row.Resource == nil || row.Resource.Link == nil {
				continue
			}
			err = validationPackageSessionObserveEdge(p.session, context.Background(), validation.PackageEdgeObservation{
				ID: 1, Source: sourceID, Template: uint64(i),
				Target:      validation.Evidence[validation.ResourceAdoptionID]{State: validation.EvidenceKnown, Value: targetID},
				Indirection: validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true},
				Fragment:    validation.Evidence[string]{State: validation.EvidenceKnown, Value: "element(/1/4/1/1)"}, FragmentTarget: resolved,
			})
			if err != nil {
				t.Fatal(err)
			}
			edges++
		}
		bundle := p.finish(t)
		report := bundle.PackageReport()
		if edges != 1 || bundle.Satisfied() || report.Counts.Pass != 0 || report.Counts.Unknown != 1 {
			t.Fatalf("outer compressed fragment incorrectly became direct: %+v", report)
		}
		found := false
		for _, finding := range report.Findings {
			if finding.Reason == "nested-link-indirection-unresolved" && finding.Outcome == validation.OutcomeUnknown {
				found = true
			}
		}
		if !found {
			t.Fatal("lost exact compressed-indirection reason")
		}
	})
}
