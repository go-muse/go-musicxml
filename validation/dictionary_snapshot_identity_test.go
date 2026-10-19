package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDictionaryPackageSnapshotsDoNotAliasActualCapabilities(t *testing.T) {
	ctx := context.Background()
	account, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	engine := sourceSchemaEngine(t, XSDCheckScalar)
	makeSnapshot := func(document string, selected bool) *ResourceSnapshot {
		var selections []DictionarySelection
		if selected {
			selections = []DictionarySelection{{Manifest: DictionaryManifestSMuFL14Canonical, Relations: []DictionaryRelation{DictionaryRelationCanonicalName}}}
		}
		set, err := NewDictionarySet(ctx, account, selections, nil)
		if err != nil {
			t.Fatal(err)
		}
		s, err := engine.BeginWithDictionaries(ctx, RunOptions{DocumentID: document, Profile: "source-schema-test", Account: account}, set)
		if err != nil {
			t.Fatal(err)
		}
		h := schemaPendingHeader()
		h.Location.DocumentID = document
		if err = s.OpenNode(ctx, h); err != nil {
			t.Fatal(err)
		}
		if err = s.CloseNode(ctx, schemaLexemeEnd("1")); err != nil {
			t.Fatal(err)
		}
		_ = set.Close()
		_, snap, err := s.FinishWithResourceSnapshot(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if err != nil {
			t.Fatal(err)
		}
		return snap
	}
	first, second := makeSnapshot("first", false), makeSnapshot("second", true)
	if first.dictionary == second.dictionary {
		t.Fatal("different availability states share identity")
	}
	rules, profiles := packageFixture(t, PackageCheckContainerPath)
	h := first.Header()
	profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
	catalog, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	session, err := catalog.Begin(ctx, RunOptions{DocumentID: "package", Profile: "test.package", Account: account})
	if err != nil {
		t.Fatal(err)
	}
	if err = session.OpenPackage(ctx, packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	pin := ResourcePin{CanonicalID: "same", Revision: "1", Digest: strings.Repeat("c", 64), Representation: ResourceSourceXML}
	a, err := session.AddResource(ctx, pin, first, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := session.AddResource(ctx, pin, second, false)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("actual capability variants reused")
	}
	again, err := session.AddResource(ctx, pin, second, true)
	if err != nil || again != b {
		t.Fatalf("same-pointer idempotence: %s %v", again, err)
	}
	report := first.report
	report.AssessmentComplete = false
	report.Conformance = ConformanceUndetermined
	report.Counts.Unknown++
	if err = session.RecordResourceFailure(ctx, pin, report, errors.New("failed variant without fingerprint")); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("unsealed failure shadowed transferred capability: %v", err)
	}
	_, _ = session.Finish(ctx, PackageCompletion{})
	if n, _ := account.Usage(BudgetBufferedBytes); n != 0 {
		t.Fatalf("snapshot/capability lease leak: %d", n)
	}
}
