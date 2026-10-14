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

func dictionaryGraphRequireDocument(t *testing.T, value any, document string) {
	t.Helper()
	var inspect func(reflect.Value)
	inspect = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.CanInterface() {
			switch x := v.Interface().(type) {
			case validation.NodeRef:
				if x.DocumentID != "" && x.DocumentID != document {
					t.Errorf("stale dictionary subject %+v want %s", x, document)
				}
				return
			case validation.Location:
				if x.DocumentID != "" && x.DocumentID != document {
					t.Errorf("stale dictionary location %+v want %s", x, document)
				}
				return
			}
		}
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				inspect(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				inspect(v.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				inspect(v.Index(i))
			}
		}
	}
	inspect(reflect.ValueOf(value))
}

func TestDictionaryGraphAdoptionsRemapNestedCausalEvidence(t *testing.T) {
	ctx := context.Background()
	g, err := startDictionaryResourceGraph(ctx, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}}, DictionaryOptions{}, ProfileResourceGraphDictionaries)
	if err != nil {
		t.Fatal(err)
	}
	defer g.finish(nil)
	pin := g.primaryPin(validation.ResourceSourceXML, strings.Repeat("a", 64))
	if err = g.session.OpenPackage(ctx, validation.PackageIdentity{ID: "root", Representation: pin.Representation, Pin: pin}); err != nil {
		t.Fatal(err)
	}
	p := &graphPackage{id: "root", pin: pin}
	input := []byte(resourceScore(`<note><rest/><duration>1</duration><accidental smufl="restQuarter">sharp</accidental></note>`))
	resource, _, err := g.assessPackageSource(p, "original.musicxml", input, g.profiles.source)
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{"alias-one", "alias-two"} {
		id, e := g.session.AdoptResource(ctx, resource.id, validation.ResourceAdoptionOptions{DocumentID: document, Package: "root", ArchiveEntry: document + ".musicxml"})
		if e != nil {
			t.Fatal(e)
		}
		adoption, e := g.session.Adoption(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for i := 0; i < adoption.ContextCount(); i++ {
			row, ok, e := adoption.ContextAt(ctx, i)
			if e != nil {
				t.Fatal(e)
			}
			if !ok || row.Dictionary == nil {
				continue
			}
			found = true
			dictionaryGraphRequireDocument(t, row, document)
			if row.Subject.DocumentID != document {
				t.Error("dictionary occurrence not rebound")
			}
			if row.Dictionary.Input.Cause != nil {
				row.Dictionary.Input.Cause.Subject.DocumentID = "caller mutation"
				again, _, e := adoption.ContextAt(ctx, i)
				if e != nil {
					t.Fatal(e)
				}
				dictionaryGraphRequireDocument(t, again, document)
			}
		}
		if !found {
			t.Error("dictionary context omitted from snapshot/adoption")
		}
	}
}

func TestDictionaryGraphAdmitsExplicitSourceSoundsProfile(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/dictionaries/musicxml-4.0/sounds.xml")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	body := &resourceTestBody{Reader: bytes.NewReader(raw)}
	d := DictionaryOptions{Selections: []validation.DictionarySelection{{Manifest: validation.DictionaryManifestStandardSounds40, Relations: []validation.DictionaryRelation{validation.DictionaryRelationStandardSound}}}, Source: dictionaryGraphSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{Request: q, State: validation.EvidenceKnown, Body: body}, nil
	})}
	g, err := startDictionaryResourceGraph(ctx, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}}, d, ProfileResourceGraphDictionaries)
	if err != nil {
		t.Fatal(err)
	}
	defer g.finish(nil)
	pin := g.primaryPin(validation.ResourceSourceXML, strings.Repeat("a", 64))
	if err = g.session.OpenPackage(ctx, validation.PackageIdentity{ID: "root", Representation: pin.Representation, Pin: pin}); err != nil {
		t.Fatal(err)
	}
	resource, _, err := g.assessPackageSource(&graphPackage{id: "root", pin: pin}, "sounds.xml", []byte(`<sounds><sound id="brass.alphorn"/><sound id="custom.sound"/></sounds>`), g.profiles.sounds)
	if err != nil {
		t.Fatal(err)
	}
	aid, err := g.session.AdoptResource(ctx, resource.id, validation.ResourceAdoptionOptions{DocumentID: "sound-adoption", Package: "root", ArchiveEntry: "sounds.xml"})
	if err != nil {
		t.Fatal(err)
	}
	adoption, err := g.session.Adoption(ctx, aid)
	if err != nil {
		t.Fatal(err)
	}
	report, err := adoption.Report(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Target != validation.TargetSource || report.Profile.ID != ProfileSourceSoundsDictionaries || report.Conformance == validation.ConformanceViolated {
		t.Fatalf("sounds recognition changed open string semantics: %+v", report)
	}
	found := map[string]bool{}
	for i := 0; i < adoption.ContextCount(); i++ {
		row, ok, e := adoption.ContextAt(ctx, i)
		if e != nil {
			t.Fatal(e)
		}
		if !ok || row.Dictionary == nil || row.Property != validation.ContextDictionarySounds {
			continue
		}
		d := row.Dictionary
		found[d.Input.Value.Text] = true
		dictionaryGraphRequireDocument(t, row, "sound-adoption")
		if d.Result.State != validation.EvidenceKnown || d.Result.Value != (d.Input.Value.Text == "brass.alphorn") || !d.ComponentVerified {
			t.Errorf("sounds evidence %+v", d)
		}
	}
	if !found["brass.alphorn"] || !found["custom.sound"] || calls != 1 || body.closed != 1 {
		t.Fatal("missing exact sounds propagation", found, calls, body.closed)
	}
}

func TestDictionaryGraphTerminalImportStopsBeforePrimaryAndReleases(t *testing.T) {
	for _, mode := range []string{"external budget", "cancel callback"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			source, calls, bodies := dictionaryGraphKnownSource(t)
			resources := LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}}
			want := validation.ErrResourceLimit
			if mode == "external budget" {
				resources.Graph.Budgets.MaxExternalBytes = 301682
			} else {
				want = context.Canceled
				inner := source
				source = dictionaryGraphSourceFunc(func(ctx context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
					r, e := inner.Open(ctx, q)
					cancel()
					return r, e
				})
			}
			g, err := startDictionaryResourceGraph(ctx, resources, DictionaryOptions{Selections: dictionaryGraphSelection(), Source: source}, ProfileResourceGraphDictionaries)
			if g == nil {
				t.Fatal("no graph to inspect cleanup", err)
			}
			if !errors.Is(err, want) {
				t.Fatal("wrong terminal cause", err)
			}
			b, err := g.finish(err)
			if !errors.Is(err, want) || b.ResourceCount() != 0 || *calls != 1 {
				t.Fatal("terminal import continued", err, b.ResourceCount(), *calls)
			}
			for _, body := range *bodies {
				if body.closed != 1 {
					t.Error("transferred body close count", body.closed)
				}
			}
			for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
				used, e := g.options.account.Usage(kind)
				if e != nil || used != 0 {
					t.Errorf("live %s=%d %v", kind, used, e)
				}
			}
		})
	}
}

func TestDictionaryGraphDoesNotEnableInternalDTD(t *testing.T) {
	input := `<!DOCTYPE score-partwise [<!ENTITY glyph "accidentalSharp">]>` + resourceScore(`<note><rest/><duration>1</duration><accidental smufl="&glyph;">sharp</accidental></note>`)
	b, err := InspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}}})
	if err == nil && b.Satisfied() {
		t.Fatal("dictionary profile silently enabled internal DTD")
	}
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		for _, row := range r.Context {
			if row.Dictionary != nil && row.Dictionary.Input.State == validation.EvidenceKnown && row.Dictionary.Input.Value.Text == "accidentalSharp" {
				t.Fatal("DTD entity became dictionary evidence")
			}
		}
	}
}
