package validation

import (
	"context"
	"errors"
	"testing"
)

func layoutRound1Reader(t *testing.T, nodes ...*factNode) *contextReader {
	t.Helper()
	s := rationalSession(t, Budgets{})
	t.Cleanup(s.cleanup)
	s.documentID = "score"
	s.nodes = map[NodeID]*factNode{}
	b := &contextBindings{Roles: map[contextRole][]SchemaUse{}}
	for _, n := range nodes {
		s.nodes[n.header.ID] = n
		s.sequence = append(s.sequence, n.header.ID)
		role := n.header.Declaration.Value.Occurrence
		b.Roles[layoutRole(role)] = []SchemaUse{{Declaration: localDecl(role)}}
		b.Roles[layoutStateRole(role)] = []SchemaUse{{Declaration: localDecl(role)}}
	}
	return &contextReader{ctx: context.Background(), s: s, b: b}
}

func TestLayoutRound1UnavailableDividerSelectionIsNotDefault(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		t.Run(string(state), func(t *testing.T) {
			root := localNode(1, 0, "measure")
			sys := localNode(2, 0, "system-layout")
			def := localNode(3, 0, "system-layout")
			dist := localNode(4, 3, "system-distance")
			div := localNode(5, 3, "system-dividers")
			left := localNode(6, 5, "left-divider")
			right := localNode(7, 5, "right-divider")
			def.children = []*factNode{dist, div}
			div.children = []*factNode{left, right}
			dist.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "30")}})
			cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 2}}
			sys.end.ChildrenComplete = testEvidence(Evidence[bool]{State: state, Reason: "divider-parent-unavailable", Cause: cause})
			r := layoutRound1Reader(t, root, sys, def, dist, div, left, right)
			l := layoutScopes{r: r, key: RuleKey{"layout", 1}}
			frame := layoutFrame{systemLayout: sys, systemCertain: true, first: Evidence[bool]{State: EvidenceKnown, Value: false}, page: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 1}}, system: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 1}}}
			if err := l.publishDividers(timelineMeasureBlock{measure: root, part: Evidence[NodeRef]{State: EvidenceAbsent}}, frame, def); err != nil {
				t.Fatal(err)
			}
			if len(r.s.contextResults) != 4 {
				t.Fatalf("rows: %+v", r.s.contextResults)
			}
			for _, row := range r.s.contextResults {
				got := row.Layout.Conversion
				if got.State != state || got.Cause == nil || *got.Cause != *cause {
					t.Errorf("divider lost dependency: %+v", got)
				}
			}
		})
	}
}

func TestLayoutRound1UnknownPartNameRemainsObservable(t *testing.T) {
	measure := localNode(1, 0, "measure")
	print := localNode(2, 1, "print")
	name := localNode(3, 2, "part-name-display")
	measure.children = []*factNode{print}
	print.children = []*factNode{name}
	r := layoutRound1Reader(t, measure, print, name)
	cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 1}}
	l := layoutScopes{r: r, key: RuleKey{"layout", 1}, names: map[NodeID][]layoutPendingName{}}
	b := timelineMeasureBlock{measure: measure, content: measure, part: Evidence[NodeRef]{State: EvidenceUnknown, Reason: "unresolved-part", Cause: cause}}
	if err := l.nameUpdates(b, layoutFrame{page: Evidence[NodeRef]{State: EvidenceAbsent}}, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
		t.Fatal(err)
	}
	if len(r.s.contextResults) != 1 {
		t.Fatalf("missing unknown-part name row: %+v", r.s.contextResults)
	}
	row := r.s.contextResults[0]
	if row.Layout.Part.State != EvidenceUnknown || row.Layout.System.State != EvidenceUnknown || row.Layout.System.Cause == nil || *row.Layout.System.Cause != *cause {
		t.Errorf("part/onset cause: %+v", row.Layout)
	}
}

func TestLayoutRound1UnavailableStaffSizeRangeRemainsObservable(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		t.Run(string(state), func(t *testing.T) {
			n := localNode(1, 0, "staff-details")
			cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 1}}
			n.end.ChildrenComplete = testEvidence(Evidence[bool]{State: state, Reason: "staff-children-unavailable", Cause: cause})
			r := layoutRound1Reader(t, n)
			if err := layoutStateNode(r.ctx, r.s, RuleKey{"layout", 1}, n, r, &contextPlan{}); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range r.s.contextResults {
				if row.Property == "layout.staff-size-range" {
					found = true
					if row.Value.State != state || row.Value.Cause == nil || *row.Value.Cause != *cause {
						t.Errorf("range lost cause: %+v", row)
					}
				}
			}
			if !found {
				t.Fatal("staff size lookup disappeared")
			}
		})
	}
}

func TestLayoutRound1MalformedBlankPageParity(t *testing.T) {
	for _, atom := range []Atomic{{Kind: AtomicString, Text: "1"}, {Kind: AtomicInteger}, localInteger(t, -1)} {
		got := layoutBlankPageParity(Evidence[bool]{State: EvidenceKnown, Value: true}, Evidence[Atomic]{State: EvidenceKnown, Value: atom})
		if got.State != EvidenceUnknown {
			t.Errorf("malformed atom chose parity: %+v", got)
		}
	}
	for _, v := range []int64{0, 1, 2, 3} {
		got := layoutBlankPageParity(Evidence[bool]{State: EvidenceKnown, Value: true}, Evidence[Atomic]{State: EvidenceKnown, Value: localInteger(t, v)})
		if got.State != EvidenceKnown || got.Value != (v%2 == 0) {
			t.Errorf("%d: %+v", v, got)
		}
	}
}

func TestLayoutRound1PercussionDisplayReferenceIgnoresGlyphLine(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	for _, line := range []Evidence[Integer]{{State: EvidenceAbsent}, {State: EvidenceKnown, Value: localByteIntegers[1]}, {State: EvidenceKnown, Value: localByteIntegers[5]}} {
		clef := clefState{Sign: Evidence[string]{State: EvidenceKnown, Value: "percussion"}, Line: line, Additional: Evidence[bool]{State: EvidenceAbsent}, OctaveChange: Evidence[Integer]{State: EvidenceAbsent}}
		got, err := layoutStateStaffStep(context.Background(), s, Evidence[string]{State: EvidenceKnown, Value: "G"}, Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[4]}, clef)
		if err != nil || got.State != EvidenceKnown || got.Value.String() != "2" {
			t.Errorf("tutorial G4 line2, encoded line %+v: %+v, %v", line, got, err)
		}
	}
}

func TestLayoutRound1PublishersPreserveInterruption(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget", true: "cancel"}[cancel], func(t *testing.T) {
			n := localNode(1, 0, "staff-details")
			r := layoutRound1Reader(t, n)
			account, err := NewBudgetAccount(Budgets{MaxRuleSteps: 1})
			if err != nil {
				t.Fatal(err)
			}
			r.s.account = account
			sentinel := r.s.step(r.ctx, 2)
			if !errors.Is(sentinel, ErrResourceLimit) {
				t.Fatal(sentinel)
			}
			if cancel {
				ctx, stop := context.WithCancel(context.Background())
				stop()
				r.ctx = ctx
				r.err = context.Canceled
			} else {
				r.err = sentinel
			}
			l := layoutScopes{r: r, key: RuleKey{"layout", 1}}
			if err := l.publishName(n, layoutFrame{}, Evidence[bool]{State: EvidenceKnown, Value: true}, Evidence[NodeRef]{State: EvidenceAbsent}); err != r.err {
				t.Errorf("interruption overwritten: %v", err)
			}
			if len(r.s.contextResults) != 0 {
				t.Errorf("interrupted payload published: %+v", r.s.contextResults)
			}
		})
	}
}

func layoutRound1CachedScope(t *testing.T) (*contextReader, RuleKey, *contextPlan) {
	t.Helper()
	measure := localNode(1, 0, "measure")
	print := localNode(2, 1, "print")
	measure.children = []*factNode{print}
	localSetAttr(print, "new-system", localString("yes"))
	print.header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextNamedType("print")})
	r := layoutRound1Reader(t, measure, print)
	r.b.Attributes = map[contextRole]map[ComponentID]AttributeSelector{layoutRole("@new-system"): {print.header.Type.Value: localAttr("new-system")}}
	key := RuleKey{"layout", 1}
	timeline := RuleKey{"timeline", 1}
	p := &contextPlan{Timeline: timeline, Bindings: r.b}
	r.s.catalog = &CompiledCatalog{rules: map[RuleKey]RuleDefinition{timeline: {Key: timeline, Predicate: "timeline"}}, plans: map[string]Plan{"timeline": {timeline: &timelinePlan{Bindings: &timelineBindings{}}}}}
	blocks := []timelineMeasureBlock{{measure: measure, content: measure, part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 10}}, complete: Evidence[bool]{State: EvidenceKnown, Value: true}, coverage: Evidence[bool]{State: EvidenceKnown, Value: true}}}
	r.s.contextServices = map[RuleKey]*contextService{key: {Layout: &contextLayoutService{Blocks: blocks, BlocksReady: true}}}
	return r, key, p
}

func TestLayoutRound1ScopesWithUnavailableTimelineDoNotPanic(t *testing.T) {
	r, key, p := layoutRound1CachedScope(t)
	if err := evaluateLayoutScopes(r.ctx, r.s, key, p, nil); err != nil {
		t.Fatal(err)
	}
	if len(r.s.contextResults) == 0 {
		t.Fatal("no local layout projections")
	}
	for _, row := range r.s.contextResults {
		if row.Property == "layout.top-system-distance" && row.Value.State != EvidenceUnknown {
			t.Errorf("missing timeline fabricated system field: %+v", row)
		}
	}
}

func TestLayoutRound1BlocksAreReusedWithoutRetention(t *testing.T) {
	r, key, p := layoutRound1CachedScope(t)
	before := rationalUsage(t, r.s, BudgetBufferedBytes)
	first, available, err := layoutGetBlocks(r, key, p)
	if err != nil || available.State != EvidenceKnown || len(first) != 1 {
		t.Fatalf("first blocks: %+v %+v %v", first, available, err)
	}
	second, _, err := layoutGetBlocks(r, key, p)
	if err != nil || &first[0] != &second[0] {
		t.Fatalf("blocks copied/rebuilt: %v", err)
	}
	if got := rationalUsage(t, r.s, BudgetBufferedBytes); got != before {
		t.Errorf("cached block retention %d -> %d", before, got)
	}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	r.ctx = ctx
	if _, _, err := layoutGetBlocks(r, key, p); !errors.Is(err, context.Canceled) {
		t.Errorf("cached blocks bypass cancellation: %v", err)
	}
}

func TestLayoutRound1BlockCacheAdmissionBeforeAllocation(t *testing.T) {
	for _, budget := range []Budgets{{MaxBufferedBytes: 1}, {MaxRuleSteps: 1}} {
		r, key, p := layoutRound1CachedScope(t)
		account, err := NewBudgetAccount(budget)
		if err != nil {
			t.Fatal(err)
		}
		r.s.account = account
		r.s.contextServices = nil
		blocks, _, err := layoutGetBlocks(r, key, p)
		if !errors.Is(err, ErrResourceLimit) || blocks != nil || len(r.s.contextServices) != 0 || r.s.retained != 0 {
			t.Errorf("unadmitted block cache: blocks=%v err=%v retained=%d", blocks, err, r.s.retained)
		}
	}
}

func TestLayoutRound1InterruptedStateIndexIsNotCached(t *testing.T) {
	r, key, p := layoutRound1CachedScope(t)
	account, err := NewBudgetAccount(Budgets{MaxRuleSteps: 15})
	if err != nil {
		t.Fatal(err)
	}
	r.s.account = account
	index, err := layoutGetStateIndex(r, key, p)
	if !errors.Is(err, ErrResourceLimit) || index != nil {
		t.Fatalf("expected interrupted build: index=%+v err=%v", index, err)
	}
	if r.s.contextServices[key].Layout.State != nil {
		t.Error("partial state index published")
	}
	index, again := layoutGetStateIndex(r, key, p)
	if again != err || index != nil {
		t.Errorf("reader retry escaped interruption: index=%+v err=%v want=%v", index, again, err)
	}
}

func TestLayoutRound1BreakAvailabilityConsensus(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 4}}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		gap := Evidence[bool]{State: state, Reason: "encoded-break-unavailable", Cause: cause}
		for _, positive := range []bool{false, true} {
			known := Evidence[bool]{State: EvidenceKnown, Value: positive}
			for _, pair := range [][2]Evidence[bool]{{known, gap}, {gap, known}} {
				got, conflict := layoutMergeBreak(pair[0], pair[1])
				if conflict || got.State != state || got.Cause != cause {
					t.Errorf("availability lost: %+v => %+v conflict=%t", pair, got, conflict)
				}
			}
		}
	}
	omitted := stateUnknown[bool]("encoded-break-completeness-unavailable")
	for _, pair := range [][2]Evidence[bool]{{{State: EvidenceKnown, Value: true}, omitted}, {omitted, {State: EvidenceKnown, Value: true}}} {
		got, conflict := layoutMergeBreak(pair[0], pair[1])
		if conflict || got.State != EvidenceKnown || !got.Value {
			t.Errorf("unencoded omission contradicted explicit break: %+v", got)
		}
	}
}
