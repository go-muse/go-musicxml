package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDTDPackageCompositionTransferIdentityAndCallerOwnership(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"exact", "legacy profile", "different digest", "different account", "wrong target"} {
		t.Run(mode, func(t *testing.T) {
			cfg, err := validateLinkedAssessmentOptions(ctx, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: dtdGraphProfile}}, dtdGraphProfile)
			if err != nil {
				t.Fatal(err)
			}
			rules, profiles, err := buildDTDDictionaryPackageAssessmentDefinitions()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "different digest" {
				for i := range profiles {
					for j := range profiles[i].Intrinsic {
						if profiles[i].Intrinsic[j].Target == validation.TargetSource {
							profiles[i].Intrinsic[j].CatalogDigest = strings.Repeat("f", 64)
						}
					}
				}
			}
			catalog, err := validation.NewPackageCatalog(rules, profiles)
			if err != nil {
				t.Fatal(err)
			}
			g, err := startConfiguredResourceGraph(ctx, cfg, dtdGraphProfile, catalog, dtdDictionaryResourceIntrinsicProfiles())
			if err != nil {
				t.Fatal(err)
			}
			account := cfg.account
			profile := ProfileSourceInternalDTDDictionaries
			if mode == "different account" {
				account = adapterAccount(t, validation.Budgets{})
			}
			if mode == "legacy profile" {
				profile = ProfileSourceDictionaries
			}
			result := assessResourceSource(ctx, strings.NewReader(`<opus/>`), adapterSourceConfig(t, account, "source", "source.opus", profile, sourceBytesPrimary))
			if result.Err != nil || result.Snapshot == nil {
				t.Fatal("snapshot", result.Err)
			}
			pin := g.primaryPin(validation.ResourceSourceXML, strings.Repeat("a", 64))
			if mode == "wrong target" {
				pin.Representation = validation.ResourceCurrentModel
				pin.Digest = ""
				pin.SnapshotID = "current"
			}
			id, err := validationPackageSessionAddResource(g.session, ctx, pin, result.Snapshot, true)
			if mode == "exact" {
				if err != nil || id == "" {
					t.Fatal(err)
				}
				if err = result.Snapshot.Close(); !errors.Is(err, validation.ErrInvalidSession) {
					t.Fatal("caller closed transferred snapshot", err)
				}
				g.finish(nil)
			} else {
				if err == nil || id != "" {
					t.Fatal("wrong identity admitted")
				}
				g.finish(err)
				if _, e := result.Snapshot.Report(ctx); e != nil {
					t.Fatal("failed transfer stole caller snapshot", e)
				}
				if e := result.Snapshot.Close(); e != nil {
					t.Fatal("caller cannot close rejected snapshot", e)
				}
			}
			adapterClean(t, cfg.account)
			if account != cfg.account {
				adapterClean(t, account)
			}
		})
	}
}

// There is no public Sounds traversal route. This exercises explicit, same-account
// auxiliary admission only, without adding filename or musical-link inference.
func TestDTDPackageCompositionExplicitSoundsAdoption(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/dictionaries/musicxml-4.0/sounds.xml")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	body := &resourceTestBody{Reader: bytes.NewReader(raw)}
	dictionaries := DictionaryOptions{Selections: []validation.DictionarySelection{{Manifest: validation.DictionaryManifestStandardSounds40, Relations: []validation.DictionaryRelation{validation.DictionaryRelationStandardSound}}}, Source: dictionaryGraphSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{Request: q, State: validation.EvidenceKnown, Body: body}, nil
	})}
	g, err := startDictionaryResourceGraph(ctx, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: dtdGraphProfile}}, dictionaries, ProfileResourceGraphDictionaries)
	if err != nil {
		t.Fatal(err)
	}
	pin := g.primaryPin(validation.ResourceSourceXML, strings.Repeat("a", 64))
	if err = validationPackageSessionOpenPackage(g.session, ctx, validation.PackageIdentity{ID: "root", Representation: pin.Representation, Pin: pin}); err != nil {
		t.Fatal(err)
	}
	input := []byte(`<!DOCTYPE sounds [<!ATTLIST sound id CDATA "brass.alphorn"><!ENTITY alternatives "<any primary='yes'>A</any><any>B</any>">]><sounds><sound>&alternatives;</sound><sound id="custom.sound"/></sounds>`)
	resource, _, err := g.assessPackageSource(&graphPackage{id: "root", pin: pin}, "original-sounds.xml", input, g.profiles.sounds)
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{"sounds-first", "sounds-second"} {
		aid, e := validationPackageSessionAdoptResource(g.session, ctx, resource.id, validation.ResourceAdoptionOptions{DocumentID: document, Package: "root", ArchiveEntry: document + ".xml", BaseURI: "https://sounds.test/" + document + "/"})
		if e != nil {
			t.Fatal(e)
		}
		adoption, e := validationPackageSessionAdoption(g.session, ctx, aid)
		if e != nil {
			t.Fatal(e)
		}
		seen := map[string]bool{}
		origins := 0
		for i := 0; i < adoption.ContextCount(); i++ {
			row, ok, e := adoption.ContextAt(ctx, i)
			if e != nil || !ok {
				t.Fatal("context", e)
			}
			dictionaryGraphRequireDocument(t, row, document)
			if row.Dictionary != nil && row.Property == validation.ContextDictionarySounds {
				d := row.Dictionary
				seen[d.Input.Value.Text] = true
				if d.Result.State != validation.EvidenceKnown || d.Result.Value != (d.Input.Value.Text == "brass.alphorn") {
					t.Fatal("recognition", d)
				}
			}
			if row.SourceOrigin == nil {
				continue
			}
			origins++
			original, _, e := adoption.ContextAt(ctx, i)
			if e != nil {
				t.Fatal(e)
			}
			o := row.SourceOrigin
			o.Subject.DocumentID = "mutated"
			if o.Attribute != nil {
				o.Attribute.Local = "mutated"
			}
			for _, site := range []*validation.Evidence[validation.Location]{&o.Physical, &o.Declaration, &o.Use} {
				if site.State == validation.EvidenceKnown {
					if site.Value.Entry != "original-sounds.xml" {
						t.Fatal("original entry was rewritten", site.Value)
					}
					site.Value.DocumentID = "mutated"
					if site.Value.Position != nil {
						site.Value.Position.ByteOffset = 999999
					}
				}
			}
			for j := range o.Ancestry {
				o.Ancestry[j].Entity = "mutated"
				o.Ancestry[j].Declaration.Value.DocumentID = "mutated"
			}
			again, _, e := adoption.ContextAt(ctx, i)
			if e != nil || !reflect.DeepEqual(again, original) {
				t.Fatal("caller mutation reached later getter", e)
			}
		}
		if origins == 0 || !seen["brass.alphorn"] || !seen["custom.sound"] {
			t.Fatal("missing auxiliary origins or recognition", origins, seen)
		}
	}
	if calls != 1 || body.closed != 1 {
		t.Fatal("component ownership", calls, body.closed)
	}
	g.finish(nil)
	adapterClean(t, g.options.account)
}

func TestDTDPackageCompositionCacheProfilesAndCapabilitiesStayDistinct(t *testing.T) {
	a := adapterAccount(t, validation.Budgets{})
	g := &resourceGraphRun{ctx: context.Background(), options: resourceRunOptions{account: a}, cache: map[graphResourceKey]*graphResource{}, pins: map[graphCanonicalKey]validation.ResourcePin{}}
	pin := validation.ResourcePin{CanonicalID: "same", Representation: validation.ResourceSourceXML, Digest: strings.Repeat("a", 64)}
	seen := map[*graphResource]bool{}
	for _, profile := range []validation.ProfileID{ProfileSourceDictionaries, ProfileSourceInternalDTDDictionaries} {
		for _, fingerprint := range [][32]byte{{1}, {2}} {
			g.dictionaryFingerprint = fingerprint
			r, e := g.cacheResource(pin, profile, validation.ResourceAssessmentID(string(profile)+string(rune(fingerprint[0]))), 1, true)
			if e != nil || seen[r] {
				t.Fatal("cross-capability/profile reuse", e)
			}
			seen[r] = true
		}
	}
	if len(g.cache) != 4 {
		t.Fatal("distinct identities collapsed")
	}
	g.close()
	adapterClean(t, a)
}

func TestDTDPackageCompositionBudgetsAndCancellationCleanup(t *testing.T) {
	input := `<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`
	run := func(ctx context.Context, budgets validation.Budgets) (*resourceGraphRun, validation.AssessmentBundle, error) {
		g, e := startDictionaryResourceGraph(ctx, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: dtdGraphProfile, Budgets: budgets}}, DictionaryOptions{}, ProfileResourceGraphDictionaries)
		if g == nil {
			return nil, validation.AssessmentBundle{}, e
		}
		if e != nil {
			b, e := g.finish(e)
			return g, b, e
		}
		b, e := g.inspectPrimarySource(strings.NewReader(input))
		return g, b, e
	}
	g, b, err := run(context.Background(), validation.Budgets{})
	if err != nil || !b.Satisfied() {
		t.Fatal("baseline", err, b.PackageReport())
	}
	adapterClean(t, g.options.account)
	if used := adapterUsage(t, g.options.account, validation.BudgetInputBytes); used != int64(len(input)) {
		t.Fatal("primary input counted more than once", used)
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetInputBytes, validation.BudgetContextFacts, validation.BudgetRuleSteps} {
		used := adapterUsage(t, g.options.account, kind)
		for _, delta := range []int64{0, -1} {
			limits := validation.Budgets{}
			switch kind {
			case validation.BudgetInputBytes:
				limits.MaxInputBytes = used + delta
			case validation.BudgetContextFacts:
				limits.MaxContextFacts = used + delta
			case validation.BudgetRuleSteps:
				limits.MaxRuleSteps = used + delta
			}
			r, b, e := run(context.Background(), limits)
			if delta == 0 && (e != nil || !b.Satisfied()) {
				t.Fatalf("exact %s=%d error=%v", kind, used, e)
			}
			if delta < 0 && (!errors.Is(e, validation.ErrResourceLimit) || b.Satisfied()) {
				t.Fatalf("one below %s=%d error=%v", kind, used, e)
			}
			if r != nil {
				adapterClean(t, r.options.account)
			}
		}
	}
	for _, limit := range []int64{1, 4096, 8192, 16384} {
		g, b, e := run(context.Background(), validation.Budgets{MaxBufferedBytes: limit})
		if !errors.Is(e, validation.ErrResourceLimit) || b.Satisfied() {
			t.Fatal("buffer boundary", limit, e)
		}
		if g != nil {
			adapterClean(t, g.options.account)
		}
	}
	for _, at := range []int{1, 50, 500, 2000, 10000} {
		ctx, cancel := context.WithCancel(context.Background())
		counted := &dtdAuditCountingContext{Context: ctx, cancel: cancel, at: at}
		g, _, e := run(counted, validation.Budgets{})
		if e != nil && !errors.Is(e, context.Canceled) {
			t.Fatal("cancellation", at, e)
		}
		if g != nil {
			adapterClean(t, g.options.account)
		}
		cancel()
	}
}
