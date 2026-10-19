package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func frameworkFixture(t *testing.T) (*PackageSession, *ResourceSnapshot) {
	t.Helper()
	p := beginPackageFixture(t, PackageCheckLinkTarget)
	f := &resourceFragmentSnapshot{Mode: fragmentFramework, Complete: stateKnown(true), Elements: []resourceFragmentElement{
		{Subject: NodeRef{"doc", 70}, Parent: NodeRef{"doc", 0}, Order: 1},
		{Subject: NodeRef{"doc", 3}, Parent: NodeRef{"doc", 70}, Order: 2},
		{Subject: NodeRef{"doc", 99}, Parent: NodeRef{"doc", 3}, Order: 3},
		{Subject: NodeRef{"doc", 8}, Parent: NodeRef{"doc", 70}, Order: 4},
	}}
	snap := &ResourceSnapshot{account: p.account, header: ResourceSnapshotHeader{DocumentID: "doc", Root: NodeRef{"doc", 70}, IDScopeComplete: stateKnown(true)}, fragments: f, ids: []ResourceIDFact{{Value: "anchor", Subject: NodeRef{"doc", 3}}}}
	if err := snap.indexFragmentOrder(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p, snap
}
func TestXMLFragmentFrameworkAndElement(t *testing.T) {
	for _, tc := range []struct {
		pointer string
		state   EvidenceState
		node    NodeID
		reason  Code
	}{
		{"anchor", EvidenceKnown, 3, ""}, {"absent", EvidenceAbsent, 0, ""},
		{"element(/1)", EvidenceKnown, 70, ""}, {"element(/1/2)", EvidenceKnown, 8, ""},
		{"element(/1/1/1)", EvidenceKnown, 99, ""}, {"element(anchor/1)", EvidenceKnown, 99, ""},
		{"element(anchor)", EvidenceKnown, 3, ""}, {"element(absent)element(/1)", EvidenceKnown, 70, ""},
		{"element(/2) element(/1/2)", EvidenceKnown, 8, ""},
		{"element(/1/99999999999999999999999999999999999999999999999999)element(/1)", EvidenceKnown, 70, ""},
		{"element(/0)element(/1)", EvidenceKnown, 70, ""}, {"element(/01)element(/1)", EvidenceKnown, 70, ""},
		{"element(/1/)element(/1)", EvidenceKnown, 70, ""}, {"element( /1)element(/1)", EvidenceKnown, 70, ""},
		{"element(/1//2)element(/1)", EvidenceKnown, 70, ""}, {"element(a:b)element(/1)", EvidenceKnown, 70, ""},
		{"unknown(data)element(/1)", EvidenceKnown, 70, ""}, {"xpointer(id('absent')) element(/1)", EvidenceKnown, 70, ""},
		{"xmlns(e=urn:scheme)e:element(/1)element(/1/2)", EvidenceKnown, 8, ""},
		{"xml:element(/1)element(/1/2)", EvidenceKnown, 8, ""},
		{"unknown(a^(b^)c^^d)element(/1)", EvidenceKnown, 70, ""},
		{"unknown((a)(b(c)))element(/1)", EvidenceKnown, 70, ""},
		{"element(^(bad^))element(/1)", EvidenceKnown, 70, ""},
		{"element(/1)element(/2)", EvidenceKnown, 70, ""},
		{"element(/2)", EvidenceAbsent, 0, "fragment-pointer-empty"},
		{"unknown(a)", EvidenceUnsupported, 0, "fragment-schemes-unimplemented"},
		{"element(/1)unknown(^q)", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{"element(/1)unknown(()", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{"element(/1))", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{"element(/1) trailing", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{" element(/1)", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{"element(/1) ", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{"element (/1)", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{"a:b:c(/1)", EvidenceAbsent, 0, "fragment-framework-malformed"},
		{"element(%2F1)", EvidenceAbsent, 0, "fragment-pointer-empty"},
		{"%61nchor", EvidenceAbsent, 0, "fragment-framework-malformed"},
	} {
		t.Run(tc.pointer, func(t *testing.T) {
			p, snap := frameworkFixture(t)
			defer p.cleanup()
			r, e := p.resolveXMLFragment(context.Background(), snap, tc.pointer)
			if e != nil || r.State != tc.state || r.Value.Node != tc.node || r.Reason != tc.reason {
				t.Fatalf("got %+v %v want %s node %d reason %s", r, e, tc.state, tc.node, tc.reason)
			}
		})
	}
}
func TestXMLFragmentUnknownCannotSkipToFallback(t *testing.T) {
	p, snap := frameworkFixture(t)
	defer p.cleanup()
	snap.header.IDScopeComplete = stateUnknown[bool]("incomplete")
	r, e := p.resolveXMLFragment(context.Background(), snap, "element(anchor)element(/1)")
	if e != nil || r.State != EvidenceUnknown {
		t.Fatalf("unknown anchor skipped: %+v %v", r, e)
	}
	r, e = p.resolveXMLFragment(context.Background(), snap, "unknown(anchor)element(/1)")
	if e != nil || r.State != EvidenceKnown {
		t.Fatalf("unsupported scheme did not skip: %+v %v", r, e)
	}
	for _, pointer := range []string{"element(missing/0)element(/1)", "element(missing//1)element(/1)", "element(missing/1/)element(/1)", "element()element(/1)", "element( )element(/1)"} {
		r, e = p.resolveXMLFragment(context.Background(), snap, pointer)
		if e != nil || r.State != EvidenceKnown || r.Value.Node != 70 {
			t.Fatalf("invalid scheme data did not fall back before unavailable identity: %+v %v", r, e)
		}
	}
	r, e = p.resolveXMLFragment(context.Background(), snap, "element(missing)unknown(^q)")
	if e != nil || r.State != EvidenceAbsent || r.Reason != "fragment-framework-malformed" {
		t.Fatalf("unavailable anchor hid malformed Framework: %+v %v", r, e)
	}
	snap.fragments.Complete = stateUnknown[bool]("incomplete")
	for _, pointer := range []string{"element(/1/9999999999999999999999999)", "element(/1/1)element(/1)"} {
		r, e = p.resolveXMLFragment(context.Background(), snap, pointer)
		if e != nil || r.State != EvidenceUnknown {
			t.Fatalf("incomplete tree proved a result: %+v %v", r, e)
		}
	}
}
func TestXMLFragmentParserBoundedCancellationAndBudget(t *testing.T) {
	pointer := "unknown(" + strings.Repeat("a", 3000) + ")element(/1/" + strings.Repeat("9", 900) + ")element(/1)"
	p, snap := frameworkFixture(t)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	r, e := p.resolveXMLFragment(probe, snap, pointer)
	if e != nil || r.State != EvidenceKnown {
		t.Fatalf("large pointer: %+v %v", r, e)
	}
	p.cleanup()
	for _, at := range []int{1, 2, 20, 100, probe.calls / 2, probe.calls} {
		p, snap = frameworkFixture(t)
		_, e = p.resolveXMLFragment(&rationalCancelContext{Context: context.Background(), at: at}, snap, pointer)
		if !errors.Is(e, context.Canceled) {
			t.Fatalf("poll %d: %v", at, e)
		}
		p.cleanup()
		if n, _ := p.account.Usage(BudgetBufferedBytes); n != 0 {
			t.Fatalf("poll %d leaked %d", at, n)
		}
	}
}

func TestXMLFragmentAdoptionOwnedRemappingAndWorkAdmission(t *testing.T) {
	p, snap := frameworkFixture(t)
	defer p.cleanup()
	a := &ResourceAdoption{owner: p, value: &packageAdoption{documentID: "adopted-a", resource: &packageResource{snapshot: snap}}}
	b := &ResourceAdoption{owner: p, value: &packageAdoption{documentID: "adopted-b", resource: &packageResource{snapshot: snap}}}
	first, selected, e := a.ResolveXMLFragment(context.Background(), "element(/1/1)")
	second, _, e2 := b.ResolveXMLFragment(context.Background(), "element(/1/1)")
	if e != nil || e2 != nil || !selected || first.Value.DocumentID != "adopted-a" || second.Value.DocumentID != "adopted-b" || first.Value.Node != 3 || second.Value.Node != 3 || snap.fragments.Elements[1].Subject.DocumentID != "doc" {
		t.Fatalf("adoption ownership: %+v %+v %v %v", first, second, e, e2)
	}
	if r, selected, e := a.ResolveXMLFragment(context.Background(), "unknown(\x00)element(/1)"); e != nil || !selected || r.State != EvidenceKnown {
		t.Fatalf("Unicode scheme data rejected: %+v %v", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e = a.ResolveXMLFragment(ctx, "anchor"); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation: %v", e)
	}
	// Exact pre-scan work is charged before inspecting the bounded input. One
	// below the charge refuses without manufacturing a syntactic classification.
	used, _ := p.account.Usage(BudgetRuleSteps)
	hold := p.account.Limits().MaxRuleSteps - used - (1 + 3*int64(len("anchor"))) + 1
	if e = p.account.Charge(BudgetRuleSteps, hold); e != nil {
		t.Fatal(e)
	}
	if _, _, e = a.ResolveXMLFragment(context.Background(), "anchor"); !errors.Is(e, ErrResourceLimit) {
		t.Fatalf("unmetered input scan: %v", e)
	}
}
func TestXMLFragmentSnapshotExactParserLease(t *testing.T) {
	run := func(available int64) (Evidence[NodeRef], error) {
		p, snap := frameworkFixture(t)
		used, _ := p.account.Usage(BudgetBufferedBytes)
		hold := p.account.Limits().MaxBufferedBytes - used - available
		if e := p.account.Charge(BudgetBufferedBytes, hold); e != nil {
			t.Fatal(e)
		}
		r, e := p.resolveXMLFragment(context.Background(), snap, "element(/1)")
		p.account.Release(BudgetBufferedBytes, hold)
		p.cleanup()
		if n, _ := p.account.Usage(BudgetBufferedBytes); n != 0 {
			t.Fatalf("leaked %d", n)
		}
		return r, e
	}
	exact := int64(len("element(/1)"))*80 + 1024
	if r, e := run(exact); e != nil || r.State != EvidenceKnown {
		t.Fatalf("exact lease: %+v %v", r, e)
	}
	if _, e := run(exact - 1); !errors.Is(e, ErrResourceLimit) {
		t.Fatalf("one under: %v", e)
	}
}

func TestXMLFragmentUnavailableTargetRetainsEvidenceWithUnicodeData(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceAbsent} {
		t.Run(string(state), func(t *testing.T) {
			s, source, _ := reviewEdgeFixture(t)
			defer s.cleanup()
			s.adoptions[source].resource.snapshot.fragments = &resourceFragmentSnapshot{Mode: fragmentFramework}
			target := Evidence[ResourceAdoptionID]{State: state}
			wantOutcome, wantReason := OutcomeFail, Code("linked-resource-authoritatively-absent")
			if state == EvidenceUnknown {
				target.Reason = "resolver-unavailable"
				wantOutcome, wantReason = OutcomeUnknown, target.Reason
			}
			edge := PackageEdgeObservation{ID: 1, Source: source, Target: target, Indirection: stateKnown(false), Fragment: stateKnown("unknown(\x00)element(/1)"), FragmentTarget: stateUnknown[NodeRef]("target-unavailable")}
			if err := s.observeEdge(context.Background(), edge); err != nil {
				t.Fatalf("valid Unicode scheme data lost unavailable-target evidence: %v", err)
			}
			bundle, err := s.finish(context.Background(), PackageCompletion{})
			if err != nil {
				t.Fatal(err)
			}
			findings := bundle.PackageReport().Findings
			if len(findings) != 1 || findings[0].Outcome != wantOutcome || findings[0].Reason != wantReason {
				t.Fatalf("changed unavailable target: %+v", findings)
			}
		})
	}
}

func TestXMLFragmentNameScratchAdmittedBeforeValidation(t *testing.T) {
	p, snap := frameworkFixture(t)
	used, _ := p.account.Usage(BudgetBufferedBytes)
	hold := p.account.Limits().MaxBufferedBytes - used
	if err := p.account.Charge(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	_, err := p.resolveXMLFragment(context.Background(), snap, "anchor")
	p.account.Release(BudgetBufferedBytes, hold)
	p.cleanup()
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("name validator ran without admitted scratch: %v", err)
	}
	if used, _ := p.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("scratch refusal retained %d", used)
	}
}
