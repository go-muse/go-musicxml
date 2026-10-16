package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unsafe"
)

// Index entries are fixed NodeRef keys and uint64 orders. The doubled record
// plus control-word bound covers spare buckets and old/new growth overlap; keys
// borrow only strings already owned by the snapshot's Elements array.
var _ [resourceFragmentOrderEntryBytes - 2*int64(unsafe.Sizeof(NodeRef{})+unsafe.Sizeof(uint64(0))+unsafe.Sizeof(uint64(0)))]struct{}

func TestReviewFragmentIDLinearWork(t *testing.T) {
	for _, shape := range [][2]int{{1000, 100}, {10000, 100}, {10000, 1000}} {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			p, snap := frameworkFixture(t)
			defer p.cleanup()
			snap.fragments.Elements = nil
			snap.ids = nil
			for i := 0; i < shape[0]; i++ {
				snap.fragments.Elements = append(snap.fragments.Elements, resourceFragmentElement{Subject: NodeRef{"doc", NodeID(shape[0] - i)}, Order: uint64(i + 1)})
			}
			for i := 0; i < shape[1]; i++ {
				snap.ids = append(snap.ids, ResourceIDFact{Value: fmt.Sprintf("id-%04d", i), Subject: snap.fragments.Elements[i*(shape[0]/shape[1])].Subject})
			}
			if e := snap.indexFragmentOrder(context.Background()); e != nil {
				t.Fatal(e)
			}
			for query := 0; query < 3; query++ {
				before, _ := p.account.Usage(BudgetRuleSteps)
				r, e := p.resolveFragmentID(context.Background(), snap, "absent-id")
				after, _ := p.account.Usage(BudgetRuleSteps)
				want := int64(1 + shape[1]*(1+len("absent-id")))
				if e != nil || r.State != EvidenceAbsent || after-before != want {
					t.Fatalf("lookup steps=%d want=%d result=%+v error=%v", after-before, want, r, e)
				}
			}
		})
	}
}

func TestReviewFragmentIndexJoinedOrderAndUnknownDomains(t *testing.T) {
	p, snap := frameworkFixture(t)
	defer p.cleanup()
	// Intrinsic duplicate validity is independent. Both authorities enumerate
	// later owners before earlier ones; the smallest NodeID is not the first.
	snap.ids = []ResourceIDFact{{Value: "same", Subject: NodeRef{"doc", 8}}, {Value: "same", Subject: NodeRef{"doc", 99}, Unique: stateKnown(false)}}
	snap.fragments.DTDRequired = true
	snap.fragments.DTDIdentityComplete = stateKnown(true)
	snap.fragments.DTDIDs = []SourceDTDIDAttribute{{Value: "same", Subject: NodeRef{"doc", 99}}, {Value: "same", Subject: NodeRef{"doc", 3}}}
	got, err := p.resolveFragmentID(context.Background(), snap, "same")
	if err != nil || got != fragmentKnown(NodeRef{"doc", 3}) {
		t.Fatalf("joined first owner: %+v %v", got, err)
	}
	for _, field := range []*Evidence[bool]{&snap.header.IDScopeComplete, &snap.fragments.DTDIdentityComplete, &snap.fragments.Complete} {
		old := *field
		*field = stateUnknown[bool]("earlier-owner-unavailable")
		for _, value := range []string{"same", "absent"} {
			got, err = p.resolveFragmentID(context.Background(), snap, value)
			if err != nil || got.State != EvidenceUnknown {
				t.Fatalf("open authority inferred %s: %+v %v", value, got, err)
			}
		}
		*field = old
	}
	// Same owner in both authorities is one target, not an ambiguity.
	snap.ids = append(snap.ids, ResourceIDFact{Value: "same", Subject: NodeRef{"doc", 3}})
	got, err = p.resolveFragmentID(context.Background(), snap, "same")
	if err != nil || got.Value.Node != 3 {
		t.Fatalf("joined same owner: %+v %v", got, err)
	}
}

func TestReviewFragmentIndexExactWorkAndCancellation(t *testing.T) {
	p, snap := frameworkFixture(t)
	before, _ := p.account.Usage(BudgetRuleSteps)
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if _, err := p.resolveFragmentID(counter, snap, "anchor"); err != nil {
		t.Fatal(err)
	}
	after, _ := p.account.Usage(BudgetRuleSteps)
	cost := after - before
	p.cleanup()
	for _, delta := range []int64{0, -1} {
		p, snap = frameworkFixture(t)
		used, _ := p.account.Usage(BudgetRuleSteps)
		if err := p.account.Charge(BudgetRuleSteps, p.account.Limits().MaxRuleSteps-used-cost-delta); err != nil {
			t.Fatal(err)
		}
		result, err := p.resolveFragmentID(context.Background(), snap, "anchor")
		if delta == 0 && (err != nil || result.State != EvidenceKnown) || delta == -1 && (!errors.Is(err, ErrResourceLimit) || result.State == EvidenceKnown) {
			t.Fatalf("delta=%d result=%+v err=%v", delta, result, err)
		}
		p.cleanup()
	}
	for at := 1; at <= counter.calls; at++ {
		p, snap = frameworkFixture(t)
		result, err := p.resolveFragmentID(&rationalCancelContext{Context: context.Background(), at: at}, snap, "anchor")
		if !errors.Is(err, context.Canceled) || result.State == EvidenceKnown {
			t.Fatalf("cancel %d result=%+v err=%v", at, result, err)
		}
		p.cleanup()
		originTransportClean(t, p.account)
	}
}

func reviewFragmentSnapshotSession(t *testing.T) *Session {
	t.Helper()
	s, key := dtdIDSession(t)
	f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
	if err := s.SourceDTDIDAttribute(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(context.Background(), schemaLexemeEnd("1")); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteSourceDTDIDs(context.Background(), "source"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestReviewFragmentIndexSnapshotAdmissionAndReuse(t *testing.T) {
	probe := reviewFragmentSnapshotSession(t)
	counter := &originTransportContext{Context: context.Background(), account: probe.account}
	_, snap, err := probe.FinishWithResourceSnapshot(counter, originTransportCompletion())
	if err != nil || snap == nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap.fragmentOrder, map[NodeRef]uint64{{"source", 1}: 1}) {
		t.Fatalf("index=%v", snap.fragmentOrder)
	}
	peak := counter.peak
	snap.Close()
	originTransportClean(t, probe.account)
	for _, delta := range []int64{0, -1} {
		s := reviewFragmentSnapshotSession(t)
		hold := s.account.Limits().MaxBufferedBytes - peak - delta
		if err = s.account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		_, snap, err = s.FinishWithResourceSnapshot(context.Background(), originTransportCompletion())
		s.account.Release(BudgetBufferedBytes, hold)
		if delta == 0 && (err != nil || snap == nil) || delta == -1 && (!errors.Is(err, ErrResourceLimit) || snap != nil) {
			t.Fatalf("snapshot delta=%d error=%v", delta, err)
		}
		if snap != nil {
			snap.Close()
			if snap.fragmentOrder != nil {
				t.Fatal("closed index retained")
			}
		}
		originTransportClean(t, s.account)
	}
	s := reviewFragmentSnapshotSession(t)
	_, snap, err = s.FinishWithResourceSnapshot(context.Background(), originTransportCompletion())
	if err != nil {
		t.Fatal(err)
	}
	p := originTransportPackage(t, snap, s.account, nil)
	rid, err := p.AddResource(context.Background(), originTransportPin(), snap, true)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := p.AddResource(context.Background(), originTransportPin(), snap, true)
	if err != nil || cached != rid {
		t.Fatalf("cached transfer %v", err)
	}
	for _, doc := range []string{"adopted-a", "adopted-b", "adopted-a-longer-identity"} {
		aid, e := p.AdoptResource(context.Background(), rid, ResourceAdoptionOptions{Package: "outer", DocumentID: doc})
		if e != nil {
			t.Fatal(e)
		}
		view, e := p.Adoption(context.Background(), aid)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 3; i++ {
			got, selected, e := view.ResolveXMLFragment(context.Background(), "value")
			if e != nil || !selected || got != fragmentKnown(NodeRef{doc, 1}) {
				t.Fatalf("adoption %s: %+v %v", doc, got, e)
			}
		}
	}
	if len(snap.fragmentOrder) != 1 || snap.fragmentOrder[NodeRef{"source", 1}] != 1 {
		t.Fatal("adoption mutated intrinsic index")
	}
	p.cleanup()
	originTransportClean(t, s.account)
	// The same Account is usable once the package has released its sole owner.
	again, e := sourceSchemaEngine(t, XSDCheckScalar).Begin(context.Background(), RunOptions{DocumentID: "reused", Profile: s.profile.ID, Account: s.account})
	if e != nil {
		t.Fatal(e)
	}
	again.cleanup()
	originTransportClean(t, s.account)
}

func TestReviewFragmentIndexSnapshotPostAdmissionCancellation(t *testing.T) {
	probe := reviewFragmentSnapshotSession(t)
	ctx := &originTransportContext{Context: context.Background(), account: probe.account}
	_, snap, err := probe.FinishWithResourceSnapshot(ctx, originTransportCompletion())
	if err != nil {
		t.Fatal(err)
	}
	peak := ctx.peak
	snap.Close()
	probe = reviewFragmentSnapshotSession(t)
	ctx = &originTransportContext{Context: context.Background(), account: probe.account, threshold: peak}
	_, snap, err = probe.FinishWithResourceSnapshot(ctx, originTransportCompletion())
	if err != nil {
		t.Fatal(err)
	}
	calls := ctx.calls
	snap.Close()
	for at := 1; at <= calls; at++ {
		s := reviewFragmentSnapshotSession(t)
		_, snap, err = s.FinishWithResourceSnapshot(&originTransportContext{Context: context.Background(), account: s.account, threshold: peak, cancelAt: at}, originTransportCompletion())
		if !errors.Is(err, context.Canceled) || snap != nil {
			t.Fatalf("cancel %d/%d snapshot=%v err=%v", at, calls, snap, err)
		}
		originTransportClean(t, s.account)
	}
}
