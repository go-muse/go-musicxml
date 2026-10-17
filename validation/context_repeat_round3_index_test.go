package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

// This lower-level fixture isolates enumeration from identity-table completeness.
// Canonical part availability in the public model regression is deliberately
// unavailable; the all-note roster must still preserve its container's cause.
func repeatR3ContainerFixture(t *testing.T, count int, timewise bool, limits Budgets) (*Session, *contextReader, *timelineService, *timelinePlan, *DecisionRef) {
	t.Helper()
	s := rationalSession(t, limits)
	s.documentID = "d"
	s.nodes = map[NodeID]*factNode{}
	h := timelineHierarchy{Root: SchemaUse{Declaration: localDecl("score-root")}, Part: SchemaUse{Declaration: localDecl("part")}, Measure: SchemaUse{Declaration: localDecl("measure")}}
	p := &timelinePlan{Bindings: &timelineBindings{}}
	i := 0
	if timewise {
		i = 1
	}
	p.Bindings.Hierarchies[i] = h
	add := func(id, parent NodeID, d DeclarationID) *factNode {
		n := &factNode{header: testRetainedHeader(NodeHeader{ID: id, Parent: parent, Declaration: stateKnown(d)}), closed: true, end: testRetainedEnd(NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderModelContent)})}
		s.nodes[id] = n
		s.sequence = append(s.sequence, id)
		return n
	}
	s.root = add(1, 0, h.Root.Declaration)
	outer, inner := h.Part.Declaration, h.Measure.Declaration
	if timewise {
		outer, inner = inner, outer
	}
	cause := &DecisionRef{Rule: RuleKey{"model-representation", 1}, Subject: NodeRef{"d", 3}}
	for j := 0; j < count; j++ {
		id := NodeID(2 + 2*j)
		add(id, 1, outer)
		n := add(id+1, id, inner)
		n.end.ChildrenComplete = testEvidence(Evidence[bool]{State: EvidenceInvalid, Reason: "model-choice-cardinality", Cause: cause})
	}
	return s, &contextReader{ctx: context.Background(), s: s, b: &contextBindings{}}, &timelineService{}, p, cause
}

func TestRound3RepeatContainerExactBindings(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, mutation := range []string{"exact", "wrong-owner", "wrong-assembly", "wrong-parent", "unknown-declaration"} {
			t.Run(map[bool]string{false: "partwise", true: "timewise"}[tw]+"/"+mutation, func(t *testing.T) {
				s, r, timeline, p, cause := repeatR3ContainerFixture(t, 1, tw, Budgets{})
				defer s.cleanup()
				n := s.nodes[3]
				switch mutation {
				case "wrong-owner":
					n.header.Declaration.Value.OwnerOccurrence = "unrelated"
				case "wrong-assembly":
					n.header.Declaration.Value.Assembly = "other"
				case "wrong-parent":
					n.header.Parent = 1
				case "unknown-declaration":
					n.header.Declaration = testEvidence(stateUnknown[DeclarationID]("unknown-container"))
				}
				index, free, err := contextRepeatBuildProofIndex(r, timeline, nil, p)
				if err != nil {
					t.Fatal(err)
				}
				defer free()
				got := index.partCoverage(stateUnknown[NodeRef]("row-part-unavailable"))
				relevant := mutation == "exact" || mutation == "unknown-declaration"
				if relevant {
					if got.State != EvidenceInvalid || got.Cause != cause || got.Reason != "model-choice-cardinality" {
						t.Errorf("gap lost %+v", got)
					}
				} else if got.State != EvidenceKnown || !got.Value {
					t.Errorf("unrelated declaration/parent broadened: %+v", got)
				}
			})
		}
	}
}

func TestRound3RepeatUnknownRosterCoverageAndCausalOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		s := rationalSession(t, Budgets{})
		s.nodes = map[NodeID]*factNode{}
		s.sequence = []NodeID{10, 11}
		if reverse {
			s.sequence = []NodeID{11, 10}
		}
		parts := []NodeRef{{"d", 1}, {"other-document", 1}}
		causes := []*DecisionRef{{Rule: RuleKey{"first", 1}, Subject: NodeRef{"d", 10}}, {Rule: RuleKey{"second", 1}, Subject: NodeRef{"d", 11}}}
		timeline := &timelineService{Facts: map[NodeID]timelineEventFact{}}
		for i, id := range []NodeID{10, 11} {
			s.nodes[id] = &factNode{header: testRetainedHeader(NodeHeader{ID: id, Declaration: Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "note-declaration", Cause: causes[i]}}), closed: true, end: testRetainedEnd(NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)})}
			timeline.Facts[id] = timelineEventFact{Part: stateKnown(parts[i])}
		}
		index, free, err := contextRepeatBuildProofIndex(&contextReader{ctx: context.Background(), s: s, b: &contextBindings{}}, timeline, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i, part := range parts {
			got := index.partCoverage(stateKnown(part))
			if got.State != EvidenceInvalid || got.Cause != causes[i] {
				t.Errorf("exact part/document %+v: %+v", part, got)
			}
		}
		want := causes[0]
		if reverse {
			want = causes[1]
		}
		got := index.partCoverage(stateUnknown[NodeRef]("row-part"))
		if got.State != EvidenceInvalid || got.Cause != want {
			t.Errorf("all parts source order lost %+v", got)
		}
		if got := index.partCoverage(stateKnown(NodeRef{"unrelated", 1})); got.State != EvidenceKnown || !got.Value {
			t.Errorf("unrelated part broadened %+v", got)
		}
		free()
		s.cleanup()
	}
}

func TestRound3RepeatContainerResources(t *testing.T) {
	for _, tw := range []bool{false, true} {
		run := func(ctx context.Context, limits Budgets) (int64, int64, error) {
			s, r, timeline, p, _ := repeatR3ContainerFixture(t, 3, tw, limits)
			r.ctx = ctx
			index, free, err := contextRepeatBuildProofIndex(r, timeline, nil, p)
			steps, bytes := rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes)
			if err == nil {
				if index.partCoverage(stateUnknown[NodeRef]("row")).State != EvidenceInvalid {
					t.Fatal("lost causal gap")
				}
				free()
			} else if index != nil || free != nil {
				t.Fatal("partial index escaped")
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("scratch leaked")
			}
			s.cleanup()
			return steps, bytes, err
		}
		counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
		steps, bytes, err := run(counter, Budgets{})
		if err != nil {
			t.Fatal(err)
		}
		if bytes != 7*640 {
			t.Fatalf("unused origin-map reservation retained: %d", bytes)
		}
		for _, delta := range []int64{0, -1} {
			_, _, err := run(context.Background(), Budgets{MaxRuleSteps: steps + delta})
			if delta == 0 && err != nil || delta < 0 && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("steps %d: %v", delta, err)
			}
			_, _, err = run(context.Background(), Budgets{MaxBufferedBytes: bytes + delta})
			if delta == 0 && err != nil || delta < 0 && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("bytes %d: %v", delta, err)
			}
		}
		for at := 1; at <= counter.calls; at++ {
			if _, _, err := run(&rationalCancelContext{Context: context.Background(), at: at}, Budgets{}); !errors.Is(err, context.Canceled) {
				t.Fatalf("poll %d: %v", at, err)
			}
		}
		t.Logf("timewise=%v exact steps=%d bytes=%d; all %d polls clean", tw, steps, bytes, counter.calls)
	}
}

func TestRound3RepeatContainerAndAllScopeLinearWork(t *testing.T) {
	var previous int64
	for _, count := range []int{100, 1000} {
		s, r, timeline, p, cause := repeatR3ContainerFixture(t, count, false, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		index, free, err := contextRepeatBuildProofIndex(r, timeline, nil, p)
		if err != nil {
			t.Fatal(err)
		}
		before := rationalUsage(t, s, BudgetRuleSteps)
		bytes := rationalUsage(t, s, BudgetBufferedBytes)
		for i := 0; i < count; i++ {
			if got := index.partCoverage(stateUnknown[NodeRef]("row")); got.Cause != cause {
				t.Fatal("cached all-scope result changed")
			}
		}
		work := rationalUsage(t, s, BudgetRuleSteps)
		if work != before || rationalUsage(t, s, BudgetBufferedBytes) != bytes {
			t.Fatal("per-row scope rescan/retention")
		}
		if previous != 0 && work > 11*previous {
			t.Fatalf("nonlinear %d -> %d", previous, work)
		}
		previous = work
		free()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("scratch leaked")
		}
		s.cleanup()
		t.Logf("%d eventless containers/all-scope rows: %d steps, %d bytes", count, work, bytes)
	}
}

func TestRound3RepeatTerminalExtentProjections(t *testing.T) {
	for _, kind := range []string{"note", "forward", "backup"} {
		for _, witness := range []bool{false, true} {
			s := rationalSession(t, Budgets{})
			part := NodeRef{"d", 1}
			origin := statePosition(t, 0, 3, 1)
			duration := mustRational(t, 1, 1)
			want := "4"
			if kind == "forward" {
				origin.Offset = mustRational(t, 4, 1)
				duration = mustRational(t, 2, 1)
				want = "6"
			}
			if kind == "backup" {
				origin.Offset = mustRational(t, 4, 1)
				duration = mustRational(t, 4, 1)
				want = "4"
			}
			d := localDecl(kind)
			n := &factNode{header: testRetainedHeader(NodeHeader{ID: 10, Declaration: stateKnown(d)}), closed: true, end: testRetainedEnd(NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)})}
			s.nodes = map[NodeID]*factNode{10: n}
			s.sequence = []NodeID{10}
			timeline := &timelineService{Facts: map[NodeID]timelineEventFact{10: {Part: stateKnown(part), Origin: stateKnown(origin), Duration: stateKnown(duration)}}}
			timeline.State.Closures = []stateClosure{{Part: part, Through: 0, Complete: stateKnown(true)}}
			if err := timeline.State.observeFrameOrigin(context.Background(), s, origin, false); err != nil {
				t.Fatal(err)
			}
			if witness {
				pos := origin
				switch kind {
				case "note":
					pos.Offset = mustRational(t, 4, 1)
				case "forward":
					pos.Offset = mustRational(t, 6, 1)
				}
				if err := timeline.State.observeFrameOrigin(context.Background(), s, pos, false); err != nil {
					t.Fatal(err)
				}
			}
			r := &contextReader{ctx: context.Background(), s: s, b: &contextBindings{Elements: map[contextRole]map[DeclarationID]struct{}{contextRepeatCursorBackup: {localDecl("backup"): {}}}}}
			index, free, err := contextRepeatBuildProofIndex(r, timeline, nil)
			if err != nil {
				t.Fatal(err)
			}
			end := index.partEnd(origin)
			if end.State != EvidenceKnown || end.Value.Offset.String() != want {
				t.Fatalf("%s endpoint %+v", kind, end)
			}
			if kind != "backup" && !end.Value.UnprovedFrame {
				t.Errorf("%s synthesized endpoint falsely claims frame membership: %+v", kind, end.Value)
			}
			extent := end.Value
			extent.UnprovedFrame = true
			member, err := timeline.State.frameMember(context.Background(), s, extent)
			if err != nil {
				t.Fatal(err)
			}
			if member != (witness || kind == "backup") {
				t.Fatalf("%s witness=%v member=%v", kind, witness, member)
			}
			t.Logf("%s witness=%v origin=%s extent=%s stored UnprovedFrame=%v independently contained=%v", kind, witness, origin.Offset.String(), end.Value.Offset.String(), end.Value.UnprovedFrame, member)
			free()
			s.cleanup()
		}
	}
}

func TestRound3RepeatTerminalExtentUnavailable(t *testing.T) {
	part := NodeRef{"d", 1}
	start := statePosition(t, 0, 1, 1)
	cause := &DecisionRef{Rule: RuleKey{"duration", 1}, Subject: NodeRef{"d", 20}}
	for _, tc := range []struct {
		name    string
		closure stateClosure
		gap     Evidence[bool]
		want    EvidenceState
		reason  Code
	}{
		{"later-final-measure", stateClosure{Part: part, Through: 1, Complete: stateKnown(true)}, Evidence[bool]{}, EvidenceUnknown, "context-repeat-beat-measure-local-boundary-unresolved"},
		{"incomplete-part", stateClosure{Part: part, Through: 0, Complete: stateUnknown[bool]("part-open")}, Evidence[bool]{}, EvidenceUnknown, "context-repeat-beat-measure-local-boundary-unresolved"},
		{"empty-final-measure", stateClosure{Part: part, Through: 0, Complete: stateKnown(true)}, Evidence[bool]{}, EvidenceUnknown, "context-repeat-part-end-position-unavailable"},
		{"unknown-extent", stateClosure{Part: part, Through: 0, Complete: stateKnown(true)}, stateUnknown[bool]("origin-unavailable"), EvidenceUnknown, "origin-unavailable"},
		{"invalid-duration", stateClosure{Part: part, Through: 0, Complete: stateKnown(true)}, Evidence[bool]{State: EvidenceInvalid, Reason: "duration-invalid", Cause: cause}, EvidenceInvalid, "duration-invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := &contextRepeatProofIndex{closures: map[NodeRef]stateClosure{part: tc.closure}, endGaps: map[NodeRef]Evidence[bool]{}}
			if tc.gap.State != "" {
				x.endGaps[part] = tc.gap
			}
			got := x.partEnd(start)
			if got.State != tc.want || got.Reason != tc.reason || got.Cause != tc.gap.Cause {
				t.Errorf("extent=%+v", got)
			}
		})
	}
}

func TestRound3RepeatKnownPartEventlessContainerIsolation(t *testing.T) {
	schema := localSchema(t)
	for _, target := range []Target{TargetSource, TargetModel} {
		t.Run(string(target), func(t *testing.T) {
			// Finalize the authoritative identity table first, then inject an unavailable
			// provider enumeration axis. Public malformed models instead also invalidate
			// identity completeness; this isolates the known-part branch explicitly.
			tree := lt("root", referenceHeader(), lt("measure", la("part", "P1"), la("part", "P2")), lt("measure", la("part", "P2")))
			s := localAdmit(t, localEngine(t, schema, target, referenceBindings(t, schema, false)...), target, tree, nil)
			defer s.cleanup()
			if !s.schema.bound {
				if err := s.bindSourceSchema(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			s.inputComplete = true
			if err := s.finalizeSourceSchema(context.Background()); err != nil {
				t.Fatal(err)
			}
			rootDecl, _ := schema.Root(QName{Local: "root"})
			partUse := localUse(t, schema, "measure", "part", false)
			p := &timelinePlan{Bindings: &timelineBindings{}, PartOwners: []ReferenceOwner{{Declaration: partUse.Declaration, Attribute: localSchemaAttr(t, schema, "part", "id"), Rule: RuleKey{"part.measure", 1}}}}
			p.Bindings.Hierarchies[1] = timelineHierarchy{Root: SchemaUse{Declaration: rootDecl}, Measure: localUse(t, schema, "root", "measure", false), Part: partUse}
			var containers []*factNode
			var parts []Evidence[NodeRef]
			for _, id := range s.sequence {
				n := s.nodes[id]
				if n.header.Declaration.State == EvidenceKnown && n.header.Declaration.Value == partUse.Declaration {
					part, err := s.timelinePart(context.Background(), n, p)
					if err != nil || part.State != EvidenceKnown {
						t.Fatalf("canonical part=%+v err=%v", part, err)
					}
					containers = append(containers, n)
					parts = append(parts, part)
				}
			}
			if len(parts) != 3 || parts[0].Value == parts[1].Value || parts[1].Value != parts[2].Value {
				t.Fatal("invalid identity fixture")
			}
			cause := &DecisionRef{Rule: RuleKey{"provider-enumeration", 1}, Subject: NodeRef{"doc", containers[1].header.ID}}
			containers[1].end.ChildrenComplete = testEvidence(Evidence[bool]{State: EvidenceInvalid, Reason: "eventless-container", Cause: cause})
			index, free, err := contextRepeatBuildProofIndex(&contextReader{ctx: context.Background(), s: s, b: &contextBindings{}}, &timelineService{}, nil, p)
			if err != nil {
				t.Fatal(err)
			}
			defer free()
			if got := index.partCoverage(parts[0]); got.State != EvidenceKnown || !got.Value {
				t.Errorf("P2 broadened to P1: %+v", got)
			}
			for _, part := range []Evidence[NodeRef]{parts[1], stateUnknown[NodeRef]("all-notes")} {
				if got := index.partCoverage(part); got.State != EvidenceInvalid || got.Cause != cause {
					t.Errorf("eventless scope lost: %+v", got)
				}
			}
			if len(index.coverage) != 1 {
				t.Errorf("repeated canonical P2 duplicated coverage: %d", len(index.coverage))
			}
		})
	}
}

func TestRound3RepeatEverySemanticContainer(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, id := range []NodeID{1, 2, 3} {
			s, r, timeline, p, cause := repeatR3ContainerFixture(t, 1, tw, Budgets{})
			s.nodes[3].end.ChildrenComplete = testEvidence(stateKnown(true))
			s.nodes[id].end.ChildrenComplete = testEvidence(Evidence[bool]{State: EvidenceInvalid, Reason: "container-enumeration", Cause: cause})
			index, free, err := contextRepeatBuildProofIndex(r, timeline, nil, p)
			if err != nil {
				t.Fatal(err)
			}
			got := index.partCoverage(stateUnknown[NodeRef]("all-notes"))
			if got.State != EvidenceInvalid || got.Cause != cause || got.Reason != "container-enumeration" {
				t.Errorf("timewise=%v container%d lost: %+v", tw, id, got)
			}
			free()
			s.cleanup()
		}
	}
}
