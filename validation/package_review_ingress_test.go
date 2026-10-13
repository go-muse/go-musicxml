package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPackageReviewRejectsNonRuleDependencies(t *testing.T) {
	for _, kind := range []DependencyKind{DependencyFact, DependencyExternal} {
		t.Run(string(kind), func(t *testing.T) {
			rules, profiles := packageFixture(t, PackageCheckContainerPath)
			rules[0].Rule.Requires = []Dependency{{Kind: kind, Key: "unavailable", Relation: RelationDocument}}
			if _, err := NewPackageCatalog(rules, profiles); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("non-rule dependency accepted: %v", err)
			}
		})
	}
}

func TestPackageReviewHeaderPresenceContracts(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceKnown} {
		for _, field := range []string{"method", "flags", "extra"} {
			t.Run(string(state)+"/"+field, func(t *testing.T) {
				absent := Evidence[uint16]{State: EvidenceAbsent}
				h := ZIPHeaderObservation{Present: Evidence[bool]{State: state}, Method: absent, Flags: absent, ExtraLength: Evidence[uint64]{State: EvidenceAbsent}}
				if state == EvidenceKnown {
					h.Present.Value = true
				}
				if state == EvidenceUnknown {
					h.Present.Reason = "framing-incomplete"
				}
				switch field {
				case "method":
					h.Method = Evidence[uint16]{State: EvidenceKnown, Value: 8}
				case "flags":
					h.Flags = Evidence[uint16]{State: EvidenceKnown, Value: 1}
				case "extra":
					h.ExtraLength = Evidence[uint64]{State: EvidenceKnown, Value: 4}
				}
				err := validateZIPHeader(h)
				if (err != nil) != (state == EvidenceAbsent) {
					t.Errorf("presence %s independent %s: %v", state, field, err)
				}
			})
		}
	}
}

func TestPackageReviewUnverifiedPinsDoNotProveAncestry(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckContainerPath)
	p := packageTestIdentity()
	p.PinUnavailable = true
	p.Pin.Digest = ""
	if err := s.OpenPackage(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	child := p
	child.ID = "inner"
	child.Parent = p.ID
	if err := s.OpenPackage(context.Background(), child); err != nil {
		t.Errorf("unverified identity used as ancestry proof: %v", err)
	}
	b, _ := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if b.Satisfied() {
		t.Error("unverified boundary certified completion")
	}
}

func TestPackageReviewRawNameAdmissionLinearAndOwned(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	if err := s.OpenPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 128; i++ {
		e := packageTestEntry(PackageFactID(i+1), "")
		e.RawName = []byte(fmt.Sprintf("%04d", i) + strings.Repeat("x", 4091) + "\xff")
		e.UTF8Name = Evidence[bool]{State: EvidenceKnown, Value: false}
		if err := s.ObserveEntry(context.Background(), e); err != nil {
			t.Errorf("entry %d: %v", i, err)
			break
		}
		e.RawName[0] = 'z'
	}
	used, _ := s.account.Usage(BudgetRuleSteps)
	if used > 3_000_000 {
		t.Errorf("raw-name admission used %d steps, expected linear bound", used)
	}
	duplicate := packageTestEntry(129, "")
	duplicate.RawName = []byte("0000" + strings.Repeat("x", 4091) + "\xff")
	duplicate.UTF8Name = Evidence[bool]{State: EvidenceKnown, Value: false}
	if err := s.ObserveEntry(context.Background(), duplicate); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("original raw key not retained independently: %v", err)
	}
	_, _ = s.Finish(context.Background(), PackageCompletion{})
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Errorf("leaked %d bytes", used)
	}
}

func reviewEdgeFixture(t *testing.T) (*PackageSession, ResourceAdoptionID, ResourceAdoptionID) {
	t.Helper()
	s, root, _ := packageAdviceFixtureCheck(t, PackageCheckLinkTarget)
	a := s.adoptions[root]
	a.resource.snapshot.contexts = []ContextResult{{Subject: NodeRef{"source", 2}, Resource: &ResourceContextResult{Link: &ResourceLinkContext{Kind: ResourceScoreLink, Href: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "child.xml#"}}}}}}}
	source, err := s.AdoptResource(context.Background(), a.resource.id, ResourceAdoptionOptions{Package: "outer", DocumentID: "source-view", BaseURI: "source-view"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := s.AdoptResource(context.Background(), a.resource.id, ResourceAdoptionOptions{Package: "outer", DocumentID: "target-view", BaseURI: "target-view"})
	if err != nil {
		t.Fatal(err)
	}
	return s, source, target
}
func TestPackageReviewEdgeFragmentContracts(t *testing.T) {
	for _, tc := range []struct {
		name          string
		state         EvidenceState
		text          string
		node          NodeID
		self, wantErr bool
	}{
		{name: "known-empty-root", state: EvidenceKnown, node: 1},
		{name: "known-empty-arbitrary-node", state: EvidenceKnown, node: 2, wantErr: true},
		{name: "known-empty-zero-node", state: EvidenceKnown, node: 0, wantErr: true},
		{name: "absent-with-node", state: EvidenceAbsent, node: 1, wantErr: true},
		{name: "unknown-with-node", state: EvidenceUnknown, node: 1, wantErr: true},
		{name: "known-id-node", state: EvidenceKnown, text: "target", node: 2},
		{name: "self-target", state: EvidenceKnown, node: 1, self: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, source, target := reviewEdgeFixture(t)
			if tc.self {
				target = source
			}
			node := NodeRef{s.adoptions[target].documentID, tc.node}
			fragment := Evidence[string]{State: tc.state, Value: tc.text}
			if tc.state == EvidenceUnknown {
				fragment.Reason = "unresolved"
			}
			e := PackageEdgeObservation{ID: 1, Source: source, Target: Evidence[ResourceAdoptionID]{State: EvidenceKnown, Value: target}, Fragment: fragment, FragmentTarget: Evidence[NodeRef]{State: EvidenceKnown, Value: node}}
			err := s.ObserveEdge(context.Background(), e)
			if (err != nil) != tc.wantErr {
				t.Errorf("error=%v wantErr=%t", err, tc.wantErr)
			}
			_, _ = s.Finish(context.Background(), PackageCompletion{})
		})
	}
}
func TestPackageReviewEdgeFindingsIdentifyTemplate(t *testing.T) {
	s, source, _ := reviewEdgeFixture(t)
	a := s.adoptions[source]
	a.resource.snapshot.contexts = append(a.resource.snapshot.contexts, stateClone(a.resource.snapshot.contexts[0]))
	for i := 0; i < 2; i++ {
		e := PackageEdgeObservation{ID: PackageFactID(i + 1), Source: source, Template: uint64(i), Target: Evidence[ResourceAdoptionID]{State: EvidenceUnknown, Reason: "unresolved"}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceAbsent}}
		if err := s.ObserveEdge(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	b, err := s.Finish(context.Background(), PackageCompletion{})
	if err != nil {
		t.Fatal(err)
	}
	f := b.PackageReport().Findings
	if len(f) != 2 {
		t.Fatalf("findings=%+v", f)
	}
	if f[0].Location.Path == f[1].Location.Path {
		t.Errorf("different templates share location %q", f[0].Location.Path)
	}
}

func TestPackageReviewRoleEchoNilEmptyAndCausalEquality(t *testing.T) {
	scope := Evidence[bool]{State: EvidenceInvalid, Reason: "scope-invalid", Cause: &DecisionRef{Rule: RuleKey{ID: "scope", Revision: 1}, Subject: NodeRef{"source", 1}}}
	q := ResourceRoleRequest{ID: "roles", Package: "outer", ScopeComplete: scope}
	echoed := stateClone(q)
	echoed.Resources = []ResourcePin{}
	echoed.Members = []ResourceRoleMember{}
	r := ResourceRoleResult{Request: echoed, ScopeComplete: Evidence[bool]{State: EvidenceUnknown, Reason: "scope-unavailable"}}
	request := ResourceRequest{ID: q.ID, Roles: &q}
	result := ResourceResult{RequestID: q.ID, State: EvidenceKnown, Roles: &r}
	if err := ValidateResourceResult(request, result); err != nil {
		t.Errorf("semantically identical echo rejected: %v", err)
	}
	r.Request.ScopeComplete.Cause.Subject.Node++
	if err := ValidateResourceResult(request, result); err == nil {
		t.Error("altered causal subject accepted")
	}
	r.Request = stateClone(q)
	r.Request.Package = "different"
	if err := ValidateResourceResult(request, result); err == nil {
		t.Error("altered package scope accepted")
	}
}

func TestPackageReviewRejectedRelationsDoNotRequireResource(t *testing.T) {
	for _, kind := range []string{"edge", "rootfile"} {
		t.Run(kind, func(t *testing.T) {
			s, source, _ := reviewEdgeFixture(t)
			snapshot, _ := interruptionSnapshot(t, s.account, "optional", "1")
			id, err := s.AddResource(context.Background(), interruptionPin("optional"), snapshot, false)
			if err != nil {
				t.Fatal(err)
			}
			resource := s.resources[id]
			if kind == "edge" {
				target, err := s.AdoptResource(context.Background(), id, ResourceAdoptionOptions{Package: "outer", DocumentID: "optional-view"})
				if err != nil {
					t.Fatal(err)
				}
				err = s.ObserveEdge(context.Background(), PackageEdgeObservation{ID: 10, Source: source, Target: Evidence[ResourceAdoptionID]{State: EvidenceKnown, Value: target}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"optional-view", 1}}})
				if err == nil {
					t.Fatal("contradictory edge accepted")
				}
			} else {
				// An existing entry owns ID 10, so the rootfile fails final subject admission.
				if err = s.ObserveEntry(context.Background(), packageTestEntry(10, "optional.xml")); err != nil {
					t.Fatal(err)
				}
				err = s.ObserveRootfile(context.Background(), PackageRootfileObservation{ID: 10, Package: "outer", Source: NodeRef{"container", 1}, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfile[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown, Value: "optional.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceKnown, Value: 10}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceKnown, Value: id}, Indirection: Evidence[bool]{State: EvidenceKnown}})
				if err == nil {
					t.Fatal("duplicate subject accepted")
				}
			}
			if resource.required {
				t.Error("rejected relation marked optional resource required")
			}
			_, _ = s.Finish(context.Background(), PackageCompletion{})
		})
	}
}
