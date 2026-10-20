package validation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func originTransportEngine(t *testing.T) (*Engine, RuleKey) {
	t.Helper()
	e, s, key := sourceOriginFixture(t, Budgets{})
	s.cleanup()
	return e, key
}

func originTransportValue(kind string) SourceOriginContextResult {
	v := sourceOriginPhysical()
	v.Physical.Value.Entry = "source"
	v.Physical.Value.Position.CoordinateSpace = "source"
	if kind == "physical" {
		return v
	}
	if kind == "rich" {
		return *sourceOriginRichPublication().SourceOrigin
	}
	v.Declaration = stateKnown(Location{Target: TargetSource, DocumentID: "source", Entry: "source", Path: "/!DOCTYPE/source", Position: &SourcePosition{ByteOffset: 3, Line: 1, Column: 4, CoordinateSpace: "xml"}})
	v.Use = stateKnown(Location{Target: TargetSource, DocumentID: "source", Entry: "source", Path: "/root", Position: &SourcePosition{ByteOffset: 99, Line: 3, Column: 5, CoordinateSpace: "xml"}})
	v.Specified = stateKnown(false)
	v.Physical = stateAbsent[Location]()
	if kind != "entity-element" {
		v.Attribute = &QName{Space: "urn:source", Local: "value"}
	}
	if kind == "default" {
		v.Kind = SourceOriginDefault
		return v
	}
	v.Kind = SourceOriginEntity
	v.Ancestry = []SourceOriginStep{{Entity: "source", Declaration: stateClone(v.Declaration), Use: stateClone(v.Use)}}
	if kind == "entity-attribute" {
		v.Specified = stateKnown(true)
		v.Physical = sourceOriginPhysical().Physical
		v.Physical.Value.Path = "/root/@value"
		v.Physical.Value.Entry = "source"
	}
	return v
}

func originTransportSession(t *testing.T, e *Engine, key RuleKey, opts RunOptions, kind, seal, text string) *Session {
	t.Helper()
	ctx := context.Background()
	opts.DocumentID, opts.Profile = "source", "source-schema-test"
	s, err := e.Begin(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	v := originTransportValue(kind)
	root := v
	if v.Attribute != nil {
		root = originTransportValue("physical")
	}
	h := schemaPendingHeader()
	if root.Physical.State == EvidenceKnown {
		h.Location = stateClone(root.Physical.Value)
	}
	if err = s.OpenNode(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err = s.sourceOrigin(ctx, key, root); err != nil {
		t.Fatal(err)
	}
	if v.Attribute != nil {
		loc := Location{Target: TargetSource, DocumentID: "source", Entry: "source", Path: "/root/@value"}
		if v.Physical.State == EvidenceKnown {
			loc = stateClone(v.Physical.Value)
		}
		if err = s.Attribute(ctx, sourceOriginAttribute(*v.Attribute, loc)); err != nil {
			t.Fatal(err)
		}
		if err = s.sourceOrigin(ctx, key, v); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.CloseNode(ctx, schemaLexemeEnd(text)); err != nil {
		t.Fatal(err)
	}
	switch seal {
	case "complete":
		err = s.completeSourceOrigins(ctx, key, stateKnown(true))
	case "incomplete":
		err = s.completeSourceOrigins(ctx, key, Evidence[bool]{State: EvidenceKnown, Value: false, Reason: "source-prefix-only"})
	case "unknown":
		err = s.completeSourceOrigins(ctx, key, stateUnknown[bool]("source-seal-unavailable"))
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func originTransportCompletion() Completion {
	return Completion{InputComplete: true, Syntax: SyntaxWellFormed}
}

func originTransportClean(t *testing.T, account *Account) {
	t.Helper()
	for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetDepth} {
		if used, err := account.Usage(kind); err != nil || used != 0 {
			t.Fatalf("live %s = %d, err=%v", kind, used, err)
		}
	}
}

func TestSourceOriginSnapshotTransportKindsAndCompleteness(t *testing.T) {
	e, key := originTransportEngine(t)
	for _, kind := range []string{"physical", "default", "entity-attribute", "entity-element", "rich"} {
		for _, seal := range []string{"complete", "missing", "incomplete", "unknown"} {
			t.Run(kind+"/"+seal, func(t *testing.T) {
				ordinary := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, kind, seal, "7")
				want, err := ordinary.Finish(context.Background(), originTransportCompletion())
				if err != nil {
					t.Fatal(err)
				}
				s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, kind, seal, "7")
				r, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), originTransportCompletion())
				if err != nil || snapshot == nil {
					t.Fatalf("source-origin export unavailable: snapshot=%v err=%v", snapshot, err)
				}
				defer snapshot.Close()
				if !reflect.DeepEqual(r, want) {
					t.Fatalf("export changed intrinsic report:\ngot %+v\nwant %+v", r, want)
				}
				if r.Satisfied() != (kind != "rich" && seal == "complete") {
					t.Fatalf("origin completeness changed: %+v", r)
				}
				if snapshot.ContextCount() != len(r.Context) {
					t.Fatalf("origin projection omitted: count=%d report=%d", snapshot.ContextCount(), len(r.Context))
				}
				for i, row := range want.Context {
					got, ok, err := snapshot.ContextAt(context.Background(), i)
					if err != nil || !ok || !reflect.DeepEqual(got, row) {
						t.Fatalf("context %d: %+v, %v", i, got, err)
					}
				}
				if snapshot.Header().LinkProjectionSelected {
					t.Fatal("origin-only profile invented a selected link plan")
				}
				_ = snapshot.Close()
				originTransportClean(t, s.account)
			})
		}
	}
}

// Mutate every reachable mutable storage cell, including QNames, physical
// positions, ancestry slices and nested causal references. A subsequent getter
// must still match the original value exactly.
func originTransportMutate(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			originTransportMutate(v.Field(i))
		}
	case reflect.Pointer:
		if !v.IsNil() {
			originTransportMutate(v.Elem())
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			originTransportMutate(v.Index(i))
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString("mutated")
		}
	case reflect.Bool:
		if v.CanSet() {
			v.SetBool(!v.Bool())
		}
	case reflect.Int, reflect.Int64:
		if v.CanSet() {
			v.SetInt(987)
		}
	case reflect.Uint, reflect.Uint64:
		if v.CanSet() {
			v.SetUint(987)
		}
	}
}

func originTransportPackage(t *testing.T, snapshot *ResourceSnapshot, account *Account, change func(*IntrinsicProfileBinding)) *PackageSession {
	t.Helper()
	rules, profiles := packageFixture(t, PackageCheckMimetypeStored)
	h := snapshot.Header()
	intrinsic := IntrinsicProfileBinding{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}
	if change != nil {
		change(&intrinsic)
	}
	profiles[0].Intrinsic = []IntrinsicProfileBinding{intrinsic}
	catalog, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	p, err := catalog.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: profiles[0].Profile.ID, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	if err = p.openPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	return p
}

func originTransportPin() ResourcePin {
	return ResourcePin{CanonicalID: "source", Revision: "1", Representation: ResourceSourceXML, Digest: strings.Repeat("d", 64)}
}

func TestSourceOriginSnapshotTransportIndependentAdoptions(t *testing.T) {
	e, key := originTransportEngine(t)
	for _, kind := range []string{"physical", "default", "entity-attribute", "entity-element", "rich"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, kind, "complete", "bad")
			r, snapshot, err := s.FinishWithResourceSnapshot(ctx, originTransportCompletion())
			if err != nil || snapshot == nil || r.Counts.Fail != 1 {
				t.Fatalf("snapshot=%v report=%+v err=%v", snapshot, r, err)
			}
			want := stateClone(r)
			originTransportMutate(reflect.ValueOf(&r).Elem())
			copied, err := snapshot.Report(ctx)
			if err != nil || !reflect.DeepEqual(copied, want) {
				t.Fatalf("returned report aliases snapshot: %v", err)
			}
			originTransportMutate(reflect.ValueOf(&copied).Elem())
			for i := range want.Context {
				copied, ok, err := snapshot.ContextAt(ctx, i)
				if err != nil || !ok {
					t.Fatal(err)
				}
				originTransportMutate(reflect.ValueOf(&copied).Elem())
			}
			p := originTransportPackage(t, snapshot, s.account, nil)
			defer p.cleanup()
			rid, err := p.addResource(ctx, originTransportPin(), snapshot, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = p.addResource(ctx, originTransportPin(), snapshot, true); err != nil {
				t.Fatalf("same-owner duplicate transfer: %v", err)
			}
			if err = snapshot.Close(); !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("caller closed transferred snapshot: %v", err)
			}
			inner := packageTestIdentity()
			inner.ID, inner.Parent, inner.Pin.CanonicalID = "inner", "outer", "inner-archive"
			if err = p.openPackage(ctx, inner); err != nil {
				t.Fatal(err)
			}
			var views []*ResourceAdoption
			for _, opts := range []ResourceAdoptionOptions{
				{Package: "outer", DocumentID: "source", Root: true, BaseURI: "https://example.test/root/", ArchiveEntry: "root.xml"},
				{Package: "outer", DocumentID: "source-alias-with-a-longer-identity", BaseURI: "https://example.test/a/", ArchiveEntry: "a.xml"},
				{Package: "outer", DocumentID: "source-alias-two", BaseURI: "https://example.test/b/", ArchiveEntry: "b.xml"},
				{Package: "inner", DocumentID: "source-nested", BaseURI: "https://example.test/a/", ArchiveEntry: "a.xml"},
			} {
				aid, err := p.adoptResource(ctx, rid, opts)
				if err != nil {
					t.Fatal(err)
				}
				view, err := p.adoption(ctx, aid)
				if err != nil {
					t.Fatal(err)
				}
				views = append(views, view)
				expected := stateClone(want)
				originTransportRebind(reflect.ValueOf(&expected).Elem(), "source", opts.DocumentID)
				got, err := view.Report(ctx)
				if err != nil || !reflect.DeepEqual(got, expected) {
					t.Fatalf("adopted report changed origin sites or metadata: %v", err)
				}
				originTransportMutate(reflect.ValueOf(&got).Elem())
				for i := range want.Context {
					copied, ok, err := view.ContextAt(ctx, i)
					if err != nil || !ok || !reflect.DeepEqual(copied, expected.Context[i]) {
						t.Fatalf("adopted context %d: %v", i, err)
					}
					originTransportMutate(reflect.ValueOf(&copied).Elem())
				}
			}
			// Earlier views remain independent after all later copies/mutations.
			for _, view := range views {
				expected := stateClone(want)
				originTransportRebind(reflect.ValueOf(&expected).Elem(), "source", view.Header().DocumentID)
				got, err := view.Report(ctx)
				if err != nil || !reflect.DeepEqual(got, expected) {
					t.Fatalf("cross-adoption mutation: %v", err)
				}
			}
			sealFixtureExpansions(t, p)
			for _, id := range []PackageID{"outer", "inner"} {
				if err = p.sealEnumeration(ctx, completePackage(id)); err != nil {
					t.Fatal(err)
				}
			}
			bundle, err := p.finish(ctx, PackageCompletion{InputComplete: true})
			if err != nil || bundle.ResourceCount() != 1 || bundle.AdoptionCount() != 4 || bundle.Satisfied() {
				t.Fatalf("bundle lost canonical failure: resources=%d adoptions=%d err=%v", bundle.ResourceCount(), bundle.AdoptionCount(), err)
			}
			_, canonical, ok := bundle.ResourceReport(0)
			if !ok || !reflect.DeepEqual(canonical, want) {
				t.Fatal("bundle changed canonical origin/report identity")
			}
			originTransportMutate(reflect.ValueOf(&canonical).Elem())
			_, again, _ := bundle.ResourceReport(0)
			if !reflect.DeepEqual(again, want) {
				t.Fatal("bundle getter aliases canonical origin")
			}
			for _, view := range views {
				if _, err = view.Report(ctx); !errors.Is(err, ErrInvalidSession) {
					t.Fatalf("view survived package finish: %v", err)
				}
			}
			if err = snapshot.Close(); err != nil {
				t.Fatal(err)
			}
			originTransportClean(t, s.account)
		})
	}
}

// Test oracle changes only the exact typed identity fields; it never calls the
// production remapper/copier and never treats an equal arbitrary string as an ID.
func originTransportRebind(v reflect.Value, from, to string) {
	if v.Type() == reflect.TypeFor[NodeRef]() {
		n := v.Addr().Interface().(*NodeRef)
		if n.DocumentID == from {
			n.DocumentID = to
		}
		return
	}
	if v.Type() == reflect.TypeFor[Location]() {
		loc := v.Addr().Interface().(*Location)
		if loc.DocumentID == from {
			loc.DocumentID = to
		}
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			originTransportRebind(v.Field(i), from, to)
		}
	case reflect.Pointer:
		if !v.IsNil() {
			originTransportRebind(v.Elem(), from, to)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			originTransportRebind(v.Index(i), from, to)
		}
	}
}

func TestSourceOriginTransportExactTypedRemapping(t *testing.T) {
	// Known source-origin sites must be local at ingress. Exercise the sealed
	// copier directly for nonlocal/empty control sites without relaxing ingress.
	for _, doc := range []string{"source", "source-prefix", "external", ""} {
		row := sourceOriginRichPublication()
		row.Subject.DocumentID, row.SourceOrigin.Subject.DocumentID = doc, doc
		row.Origins = []Location{{Target: TargetSource, DocumentID: doc, Entry: "source", Path: "source", Position: &SourcePosition{ByteOffset: 7, Line: 2, Column: 3, CoordinateSpace: "source"}}}
		v := row.SourceOrigin
		for _, site := range []*Evidence[Location]{&v.Physical, &v.Declaration, &v.Use, &v.Ancestry[0].Declaration, &v.Ancestry[0].Use, &v.Ancestry[1].Declaration, &v.Ancestry[1].Use, &v.Ancestry[2].Use} {
			if site.State == EvidenceKnown {
				site.Value.DocumentID = doc
			}
			if site.Cause != nil {
				site.Cause.Subject.DocumentID = doc
			}
		}
		v.Specified.Cause.Subject.DocumentID = doc
		v.AncestryComplete.Cause.Subject.DocumentID = doc
		row.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "source", Cause: &DecisionRef{Rule: RuleKey{"source", 1}, Subject: NodeRef{doc, 1}}}
		value := Report{Context: []ContextResult{row}, Findings: []Finding{{
			Decision: Decision{Rule: RuleKey{"source", 1}, Subject: NodeRef{doc, 1}, Cause: &DecisionRef{Rule: RuleKey{"source", 1}, Subject: NodeRef{doc, 2}}, Reason: "source", Actual: "source", Related: stateClone(row.Origins)},
			Location: stateClone(row.Origins[0]),
		}}}
		want := stateClone(value)
		originTransportRebind(reflect.ValueOf(&want).Elem(), "source", "adopted-source")
		a, err := NewBudgetAccount(Budgets{})
		if err != nil {
			t.Fatal(err)
		}
		got, err := resourceLiveCopy(context.Background(), a, value, "source", "adopted-source")
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("doc=%q changed unavailable state, unrelated string or coordinate: got=%+v err=%v", doc, got, err)
		}
		originTransportMutate(reflect.ValueOf(&got).Elem())
		again, err := resourceLiveCopy(context.Background(), a, value, "source", "adopted-source")
		if err != nil || !reflect.DeepEqual(again, want) {
			t.Fatalf("doc=%q shared mutable origin storage: %v", doc, err)
		}
		originTransportClean(t, a)
	}
}

// Observe live bytes at cancellation checkpoints without changing the account.
type originTransportContext struct {
	context.Context
	account         *Account
	peak            int64
	threshold       int64
	calls, cancelAt int
}

func (c *originTransportContext) Err() error {
	used, _ := c.account.Usage(BudgetBufferedBytes)
	if used > c.peak {
		c.peak = used
	}
	if used >= c.threshold {
		c.calls++
		if c.cancelAt > 0 && c.calls >= c.cancelAt {
			return context.Canceled
		}
	}
	return c.Context.Err()
}

func TestResourceSnapshotTransportCancellationAfterAdmission(t *testing.T) {
	// The legacy ordinary path also needs a check after the admission charge.
	// No evaluation is involved: this isolates copying from assessment outcomes.
	s := snapshotTestSession(t, "bad")
	defer s.cleanup()
	before, _ := s.account.Usage(BudgetBufferedBytes)
	ctx := &originTransportContext{Context: context.Background(), account: s.account, threshold: before + 129, cancelAt: 1}
	report := Report{Context: []ContextResult{sourceOriginRichPublication()}}
	snapshot, err := s.makeResourceSnapshot(ctx, report)
	if snapshot != nil {
		_ = snapshot.Close()
	}
	if !errors.Is(err, context.Canceled) || snapshot != nil {
		t.Fatalf("copy did not check cancellation after admission: snapshot=%v err=%v", snapshot, err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != before+128 {
		t.Fatalf("canceled snapshot copy leaked beyond the session-owned ID scratch: %d bytes", used-before-128)
	}
}

func TestSourceOriginSnapshotTransportExactOverlapAdmission(t *testing.T) {
	e, key := originTransportEngine(t)
	probe := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "complete", "bad")
	counter := &originTransportContext{Context: context.Background(), account: probe.account}
	_, snapshot, err := probe.FinishWithResourceSnapshot(counter, originTransportCompletion())
	if err != nil || snapshot == nil {
		t.Fatalf("probe: snapshot=%v err=%v", snapshot, err)
	}
	// Independent concrete retained storage lower bound includes both copies of
	// origins, not just the canonical report or selected projection in isolation.
	owned := sourceOriginClonedStorage(reflect.ValueOf(snapshot.report), true) + sourceOriginClonedStorage(reflect.ValueOf(snapshot.contexts), true) + sourceOriginClonedStorage(reflect.ValueOf(snapshot.header), true) + sourceOriginClonedStorage(reflect.ValueOf(snapshot.ids), true)
	if snapshot.retained < owned || counter.peak <= snapshot.retained {
		t.Fatalf("snapshot failed to admit owned overlapping storage: retained=%d owned=%d peak=%d", snapshot.retained, owned, counter.peak)
	}
	_ = snapshot.Close()
	originTransportClean(t, probe.account)
	for _, delta := range []int64{0, -1} {
		t.Run(map[bool]string{true: "exact", false: "one-short"}[delta == 0], func(t *testing.T) {
			s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll, Budgets: Budgets{MaxBufferedBytes: counter.peak + delta}}, "rich", "complete", "bad")
			r, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), originTransportCompletion())
			if delta == 0 {
				if err != nil || snapshot == nil {
					t.Fatalf("exact overlap refused: %v", err)
				}
				_ = snapshot.Close()
			} else if !errors.Is(err, ErrResourceLimit) || snapshot != nil || r.AssessmentComplete || r.Terminal == nil {
				t.Fatalf("one-short overlap: snapshot=%v err=%v report=%+v", snapshot, err, r)
			}
			if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || len(r.Context) != 2 {
				t.Fatalf("snapshot refusal erased known intrinsic result: %+v", r)
			}
			originTransportClean(t, s.account)
		})
	}
}

func TestSourceOriginSnapshotTransportEveryPostChargeCancellation(t *testing.T) {
	e, key := originTransportEngine(t)
	probe := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "complete", "bad")
	counter := &originTransportContext{Context: context.Background(), account: probe.account}
	_, snapshot, err := probe.FinishWithResourceSnapshot(counter, originTransportCompletion())
	if err != nil || snapshot == nil {
		t.Fatalf("probe: snapshot=%v err=%v", snapshot, err)
	}
	peak := counter.peak
	_ = snapshot.Close()
	// Count only checks while the snapshot's overlapping lease is live, then
	// cancel at every one of those checks. Earlier evaluation has already proved
	// the independent scalar violation and that result must survive each refusal.
	probe = originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "complete", "bad")
	counter = &originTransportContext{Context: context.Background(), account: probe.account, threshold: peak}
	_, snapshot, err = probe.FinishWithResourceSnapshot(counter, originTransportCompletion())
	if err != nil || snapshot == nil || counter.calls < 20 {
		t.Fatalf("copy lacks bounded post-charge checks: calls=%d err=%v", counter.calls, err)
	}
	_ = snapshot.Close()
	for at := 1; at <= counter.calls; at++ {
		s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "complete", "bad")
		ctx := &originTransportContext{Context: context.Background(), account: s.account, threshold: peak, cancelAt: at}
		r, snapshot, err := s.FinishWithResourceSnapshot(ctx, originTransportCompletion())
		if !errors.Is(err, context.Canceled) || snapshot != nil || r.AssessmentComplete || r.Terminal == nil || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || len(r.Context) != 2 {
			t.Fatalf("post-charge boundary %d: snapshot=%v err=%v report=%+v", at, snapshot, err, r)
		}
		originTransportClean(t, s.account)
	}
}

func originTransportGetterBoundary(t *testing.T, account *Account, value any, to string, get func(context.Context) error) {
	t.Helper()
	ctx := context.Background()
	need, err := resourceMeasureAs(ctx, account, reflect.ValueOf(value), to)
	if err != nil {
		t.Fatal(err)
	}
	used, _ := account.Usage(BudgetBufferedBytes)
	for _, delta := range []int64{0, 1} {
		hold := account.Limits().MaxBufferedBytes - used - need + delta
		if err = account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		err = get(ctx)
		_ = account.Release(BudgetBufferedBytes, hold)
		if delta == 0 && err != nil || delta == 1 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("copy boundary delta=%d: %v", delta, err)
		}
		if now, _ := account.Usage(BudgetBufferedBytes); now != used {
			t.Fatalf("copy boundary leaked %d", now-used)
		}
	}
	counter := &rationalCancelContext{Context: ctx, at: 1 << 30}
	if err = get(counter); err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= counter.calls; at++ {
		err = get(&rationalCancelContext{Context: ctx, at: at})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("getter cancellation boundary %d: %v", at, err)
		}
		if now, _ := account.Usage(BudgetBufferedBytes); now != used {
			t.Fatalf("getter cancellation boundary %d leaked %d", at, now-used)
		}
	}
}

func TestSourceOriginSnapshotTransportGetterBudgetsAndCancellation(t *testing.T) {
	e, key := originTransportEngine(t)
	s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll, Budgets: Budgets{MaxRuleSteps: 1 << 40}}, "rich", "complete", "bad")
	_, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), originTransportCompletion())
	if err != nil || snapshot == nil {
		t.Fatalf("snapshot=%v err=%v", snapshot, err)
	}
	originTransportGetterBoundary(t, s.account, snapshot.contexts[1], "", func(ctx context.Context) error { _, _, err := snapshot.ContextAt(ctx, 1); return err })
	originTransportGetterBoundary(t, s.account, snapshot.report, "", func(ctx context.Context) error { _, err := snapshot.Report(ctx); return err })
	p := originTransportPackage(t, snapshot, s.account, nil)
	rid, err := p.addResource(context.Background(), originTransportPin(), snapshot, true)
	if err != nil {
		t.Fatal(err)
	}
	doc := "adopted-source-with-a-much-longer-identity-than-canonical"
	aid, err := p.adoptResource(context.Background(), rid, ResourceAdoptionOptions{Package: "outer", DocumentID: doc})
	if err != nil {
		t.Fatal(err)
	}
	view, err := p.adoption(context.Background(), aid)
	if err != nil {
		t.Fatal(err)
	}
	originTransportGetterBoundary(t, s.account, snapshot.contexts[1], doc, func(ctx context.Context) error { _, _, err := view.ContextAt(ctx, 1); return err })
	originTransportGetterBoundary(t, s.account, snapshot.report, doc, func(ctx context.Context) error { _, err := view.Report(ctx); return err })
	_, _ = p.finish(context.Background(), PackageCompletion{})
	originTransportClean(t, s.account)
}

func TestSourceOriginSnapshotTransportRejectedTransferKeepsOwner(t *testing.T) {
	e, key := originTransportEngine(t)
	for _, mismatch := range []string{"account", "profile", "digest", "assembly", "target", "pin", "canceled", "budget", "collision"} {
		t.Run(mismatch, func(t *testing.T) {
			s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "complete", "bad")
			r, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), originTransportCompletion())
			if err != nil || snapshot == nil {
				t.Fatalf("snapshot=%v err=%v", snapshot, err)
			}
			account := s.account
			if mismatch == "account" {
				account, err = NewBudgetAccount(Budgets{})
				if err != nil {
					t.Fatal(err)
				}
			}
			p := originTransportPackage(t, snapshot, account, func(binding *IntrinsicProfileBinding) {
				switch mismatch {
				case "profile":
					binding.Profile = "other-source"
				case "digest":
					binding.CatalogDigest = strings.Repeat("a", 64)
				case "assembly":
					binding.Assembly = AssemblyOpus
				case "target":
					binding.Target = TargetModel
				}
			})
			ctx := context.Background()
			var hold int64
			if mismatch == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if mismatch == "budget" {
				used, _ := account.Usage(BudgetBufferedBytes)
				hold = account.Limits().MaxBufferedBytes - used
				if err = account.Charge(BudgetBufferedBytes, hold); err != nil {
					t.Fatal(err)
				}
			}
			pin := originTransportPin()
			if mismatch == "pin" {
				pin.Representation = ResourceCurrentModel
				pin.Digest = ""
				pin.SnapshotID = "model-state"
			}
			if mismatch == "collision" {
				other := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll, Account: account}, "rich", "complete", "bad")
				_, owned, err := other.FinishWithResourceSnapshot(ctx, originTransportCompletion())
				if err != nil {
					t.Fatal(err)
				}
				if _, err = p.addResource(ctx, pin, owned, true); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = p.addResource(ctx, pin, snapshot, true); err == nil {
				t.Fatal("mismatched transfer accepted")
			}
			if hold != 0 {
				_ = account.Release(BudgetBufferedBytes, hold)
			}
			got, err := snapshot.Report(context.Background())
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("rejected transfer took caller ownership: %v", err)
			}
			_, _ = p.finish(context.Background(), PackageCompletion{})
			if err = snapshot.Close(); err != nil {
				t.Fatal(err)
			}
			originTransportClean(t, s.account)
			originTransportClean(t, account)
		})
	}
}

func TestSourceOriginSnapshotTransportSourceErrorPreservesPrefix(t *testing.T) {
	e, key := originTransportEngine(t)
	cause := errors.New("source read stopped")
	end := Completion{Syntax: SyntaxIncomplete, Cause: cause}
	ordinary := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "missing", "bad")
	want, err := ordinary.Finish(context.Background(), end)
	if !errors.Is(err, cause) {
		t.Fatal(err)
	}
	s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "missing", "bad")
	r, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), end)
	if !errors.Is(err, cause) || snapshot == nil || !reflect.DeepEqual(r, want) {
		t.Fatalf("source error changed intrinsic report: snapshot=%v err=%v", snapshot, err)
	}
	if snapshot.Header().ScopeComplete.State != EvidenceUnknown || r.AssessmentComplete || snapshot.ContextCount() != 2 {
		t.Fatalf("source prefix became complete: header=%+v report=%+v", snapshot.Header(), r)
	}
	copied, err := snapshot.Report(context.Background())
	if err != nil || !reflect.DeepEqual(copied, want) {
		t.Fatalf("snapshot discarded source error: %v", err)
	}
	_ = snapshot.Close()
	originTransportClean(t, s.account)
}

func TestSourceOriginSnapshotTransportProjectionOwnsSeparateStorage(t *testing.T) {
	e, key := originTransportEngine(t)
	s := originTransportSession(t, e, key, RunOptions{Context: ContextFamilyAll}, "rich", "complete", "bad")
	r, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), originTransportCompletion())
	if err != nil || snapshot == nil {
		t.Fatalf("snapshot=%v err=%v", snapshot, err)
	}
	defer snapshot.Close()
	pointers := func(value any) map[uintptr]bool {
		found := map[uintptr]bool{}
		var visit func(reflect.Value)
		visit = func(v reflect.Value) {
			switch v.Kind() {
			case reflect.Struct:
				for i := 0; i < v.NumField(); i++ {
					visit(v.Field(i))
				}
			case reflect.Pointer:
				if !v.IsNil() {
					found[v.Pointer()] = true
					visit(v.Elem())
				}
			case reflect.Slice:
				if v.Len() > 0 {
					found[v.Pointer()] = true
				}
				for i := 0; i < v.Len(); i++ {
					visit(v.Index(i))
				}
			}
		}
		visit(reflect.ValueOf(value))
		return found
	}
	selected := pointers(snapshot.contexts)
	for _, value := range []any{snapshot.report.Context, r.Context} {
		for ptr := range pointers(value) {
			if selected[ptr] {
				t.Fatal("selected projection shares mutable storage with a report")
			}
		}
	}
}
