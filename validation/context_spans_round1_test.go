package validation

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSpanRound1LyricInterruptionIsNeverPublished(t *testing.T) {
	for _, interruption := range []error{errors.New("reader sentinel"), context.Canceled, ErrResourceLimit} {
		r := spanUnitReader(t)
		lyric := spanUnitNode(r, spanRole_noteLyric, nil)
		r.err = interruption
		spanLyricFacts(r, RuleKey{"spans", 1}, lyric)
		if !errors.Is(r.err, interruption) || len(r.s.contextResults) != 0 {
			t.Fatalf("interruption overwritten or published: err=%v results=%+v", r.err, r.s.contextResults)
		}
	}
}

func TestSpanRound1MalformedVerticalIdentityIsUnavailable(t *testing.T) {
	for _, number := range []Evidence[Atomic]{
		{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger}},
		{State: EvidenceKnown, Value: Atomic{Kind: AtomicBoolean, Text: "true"}},
		{State: EvidenceUnknown, Reason: "number-gap", Cause: &DecisionRef{Rule: RuleKey{"number", 1}, Subject: NodeRef{"d", 2}}},
	} {
		s := rationalSession(t, Budgets{})
		a, b := spanTestEndpoint(t, 1, SpanArpeggiate, "", 1), spanTestEndpoint(t, 2, SpanArpeggiate, "", 1)
		z, _ := NewRational(big.NewRat(0, 1))
		for _, e := range []*spanEndpoint{a, b} {
			e.categoryKnown = true
			e.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: NodeRef{"d", 10}, Measure: NodeRef{"d", 20}, Offset: z}}
		}
		b.payload.Number = number
		if err := spanVerticalMembership(context.Background(), s, []*spanEndpoint{a, b}); err != nil {
			t.Fatal(err)
		}
		for _, e := range []*spanEndpoint{a, b} {
			if e.payload.Association.State != EvidenceUnknown || len(e.payload.Members) != 0 {
				t.Fatalf("invented non-association: %+v", e.payload)
			}
			if number.Cause != nil && (e.payload.Association.Cause == nil || *e.payload.Association.Cause != *number.Cause) {
				t.Fatal("lost exact cause", e.payload.Association)
			}
		}
		s.cleanup()
	}
}

func TestSpanRound1SingletonKeepsUnusedIdentityCause(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceInvalid, EvidenceUnknown, EvidenceUnsupported} {
		s := rationalSession(t, Budgets{})
		e := spanTestEndpoint(t, 1, SpanTied, "let-ring", 1)
		cause := &DecisionRef{Rule: RuleKey{"number", 1}, Subject: e.ref}
		e.payload.Number = Evidence[Atomic]{State: state, Reason: "unavailable-number", Cause: cause}
		if err := associateSpanEndpoints(context.Background(), s, []*spanEndpoint{e}, nil); err != nil {
			t.Fatal(err)
		}
		if e.payload.Association.State != EvidenceKnown || !e.payload.Association.Value || len(e.payload.Members) != 1 {
			t.Fatal(e.payload)
		}
		if e.payload.Number.State != state || e.payload.Number.Cause == nil || *e.payload.Number.Cause != *cause {
			t.Fatal("unused number evidence lost", e.payload)
		}
		s.cleanup()
	}
}

func TestSpanRound1HierarchyGapWithEstablishedPartIdentity(t *testing.T) {
	for _, hierarchy := range []string{"partwise", "timewise"} {
		for _, axis := range []string{"complete", "children", "order"} {
			t.Run(hierarchy+"/"+axis, func(t *testing.T) {
				r := spanUnitReader(t)
				structural := func(name string, parent *factNode) *factNode {
					id := NodeID(len(r.s.sequence) + 1)
					var pid NodeID
					if parent != nil {
						pid = parent.header.ID
					}
					n := localNode(id, pid, name)
					n.header.Location.DocumentID = r.s.documentID
					r.s.nodes[id] = n
					r.s.sequence = append(r.s.sequence, id)
					if parent != nil {
						parent.children = append(parent.children, n)
					} else {
						r.s.root = n
					}
					return n
				}
				root := structural("score-"+hierarchy, nil)
				partList := spanUnitNode(r, spanRole_partList, root)
				for _, role := range []string{"start", "stop"} {
					group := spanUnitNode(r, spanRole_partGroup, partList)
					spanUnitAttribute(r, group, spanRole_part_group_type, spanString(role))
				}
				parts := []NodeRef{{r.s.documentID, 1001}, {r.s.documentID, 1002}}
				timelineKey := RuleKey{"timeline", 1}
				timeline := &timelineService{Final: true, Facts: map[NodeID]timelineEventFact{}}
				cause := &DecisionRef{Rule: RuleKey{"enumeration", 1}, Subject: NodeRef{r.s.documentID, 999}}
				gap := Evidence[bool]{State: EvidenceUnknown, Reason: "missing-middle-measure", Cause: cause}
				addNotes := func(parent, measure *factNode, part int, index int) {
					note := spanUnitNode(r, spanRole_note, parent)
					timeline.Facts[note.header.ID] = timelineEventFact{Part: Evidence[NodeRef]{State: EvidenceKnown, Value: parts[part]}, Position: stateUnknown[musicalPosition]("independent-position-gap")}
					if index != 1 {
						n := spanUnitNode(r, spanRole_slur, note)
						role := "start"
						if index == 2 {
							role = "stop"
						}
						spanUnitAttribute(r, n, spanRole_slur_type, spanString(role))
					}
					if index == 1 {
						if axis == "children" {
							measure.end.ChildrenComplete = testEvidence(gap)
						}
						if axis == "order" {
							measure.end.Order = testEvidence(stateUnavailable[bool, OrderKind](gap))
						}
					}
				}
				if hierarchy == "partwise" {
					for part := range parts {
						p := structural("part", root)
						for i := 0; i < 3; i++ {
							m := structural("measure", p)
							addNotes(m, m, part, i)
						}
					}
				} else {
					for i := 0; i < 3; i++ {
						m := structural("measure", root)
						for part := range parts {
							p := structural("part", m)
							addNotes(p, m, part, i)
						}
					}
				}
				r.s.timeline = map[RuleKey]*timelineService{timelineKey: timeline}
				if _, _, err := evaluateContextSpans(context.Background(), r.s, root, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b, Timeline: timelineKey}, true); err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, row := range r.s.contextResults {
					if row.Span == nil || row.Span.Family != SpanSlur {
						continue
					}
					count++
					a := row.Span.Association
					if axis == "complete" {
						if a.State != EvidenceKnown || !a.Value || len(row.Span.Members) != 2 {
							t.Fatal("control association", row.Span)
						}
					} else if a.State != EvidenceUnknown || a.Reason != gap.Reason || a.Cause == nil || *a.Cause != *cause || len(row.Span.Members) != 0 {
						t.Fatal("gap did not reach known part domain", row.Span)
					}
				}
				if count != 4 {
					t.Fatal("missing endpoints", count)
				}
			})
		}
	}
}

func TestSpanRound1LocalTieCompetitorsCannotBeRepairedGreedily(t *testing.T) {
	for _, axis := range []string{"isolated", "other-number", "same-number-before", "same-number-after", "unknown-number", "unknown-part", "three-local-boundaries"} {
		t.Run(axis, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			a, b := spanTestEndpoint(t, 1, SpanTied, "start", 1), spanTestEndpoint(t, 2, SpanTied, "stop", 1)
			b.payload.Anchor = a.payload.Anchor
			es := []*spanEndpoint{a, b}
			wantKnown := axis == "isolated" || axis == "other-number"
			if axis != "isolated" {
				c := spanTestEndpoint(t, 3, SpanTied, "start", 1)
				switch axis {
				case "other-number":
					c.payload.Number = spanInteger(2)
				case "unknown-number":
					c.payload.Number = stateUnknown[Atomic]("number-unavailable")
				case "unknown-part":
					c.payload.Domain = stateUnknown[NodeRef]("part-unavailable")
				case "three-local-boundaries":
					c.payload.Anchor = a.payload.Anchor
				}
				if axis == "same-number-after" {
					es = append(es, c)
				} else {
					es = append([]*spanEndpoint{c}, es...)
				}
			}
			if err := spanLocalTiePairs(context.Background(), s, es); err != nil {
				t.Fatal(err)
			}
			if err := associateSpanEndpoints(context.Background(), s, es, nil); err != nil {
				t.Fatal(err)
			}
			for _, e := range []*spanEndpoint{a, b} {
				if (e.payload.Association.State == EvidenceKnown) != wantKnown {
					t.Fatalf("unproved local pair after ordinary association: %+v", e.payload)
				}
			}
		})
	}
}

func TestSpanRound1NonTemporalPositionsAreAbsentWithUnavailableTimeline(t *testing.T) {
	for _, tc := range []struct {
		family            SpanFamily
		owner, node, role contextRole
	}{
		{SpanPartGroup, spanRole_partList, spanRole_partGroup, spanRole_part_group_type},
		{SpanBarre, spanRole_frame, spanRole_barre, spanRole_barre_type},
		{SpanMetronomeTied, spanRole_metronome, spanRole_metronomeTied, spanRole_metronome_tied_type},
	} {
		t.Run(string(tc.family), func(t *testing.T) {
			r := spanUnitReader(t)
			root := spanUnitNode(r, tc.owner, nil)
			parent := root
			if tc.family == SpanBarre {
				parent = spanUnitNode(r, spanRole_frameNote, root)
				f := spanUnitNode(r, spanRole_frameFret, parent)
				f.end.Scalar.Value = testEvidence(spanInteger(1))
			}
			n := spanUnitNode(r, tc.node, parent)
			spanUnitAttribute(r, n, tc.role, spanString("start"))
			key := RuleKey{"spans", 1}
			cause := &DecisionRef{Rule: RuleKey{"timeline", 1}, Subject: spanRef(r.s, root).Value}
			r.s.contextServices = map[RuleKey]*contextService{key: {Timeline: Evidence[bool]{State: EvidenceUnsupported, Reason: "timeline-not-selected", Cause: cause}}}
			if _, _, err := evaluateContextSpans(context.Background(), r.s, root, key, &contextPlan{Bindings: r.b}, true); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range r.s.contextResults {
				if row.Span == nil {
					continue
				}
				found = true
				p := row.Span
				if p.Position.State != EvidenceAbsent {
					t.Fatalf("non-temporal position inherited timeline failure: %+v", p)
				}
				for name, state := range map[string]EvidenceState{"role": p.Role.State, "domain": p.Domain.State, "anchor": p.Anchor.State, "number": p.Number.State, "number-explicit": p.NumberExplicit.State, "staff": p.Staff.State, "voice": p.Voice.State, "association": p.Association.State} {
					if state == "" {
						t.Fatal("public zero evidence", name, p)
					}
				}
			}
			if !found {
				t.Fatal("missing projection")
			}
		})
	}
}

func TestSpanRound1UnavailableRunWritesPollCancellation(t *testing.T) {
	const count = 100
	for _, vertical := range []bool{false, true} {
		s := rationalSession(t, Budgets{})
		var es []*spanEndpoint
		z, _ := NewRational(big.NewRat(0, 1))
		for i := 0; i < count; i++ {
			e := spanTestEndpoint(t, NodeID(i+1), SpanSlur, "start", 1)
			e.complete = stateUnknown[bool]("run-gap")
			if vertical {
				e.payload.Family = SpanArpeggiate
				e.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: NodeRef{"d", 10}, Measure: NodeRef{"d", 20}, Offset: z}}
			}
			es = append(es, e)
		}
		// Extraction uses three polls per numbered span (two per vertical span),
		// plus setup and run lookup. The next poll must occur in the write loop.
		at := 3*count + 3
		if vertical {
			at = 2*count + 3
		}
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		var err error
		if vertical {
			err = spanVerticalMembership(ctx, s, es)
		} else {
			err = associateSpanEndpoints(ctx, s, es, nil)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unmetered run writes: vertical=%t polls=%d err=%v", vertical, ctx.calls, err)
		}
		s.cleanup()
	}
}

func TestSpanRound1TieIndexesAreBoundedScratch(t *testing.T) {
	for _, count := range []int{100, 1000} {
		es := make([]*spanEndpoint, count)
		for i := range es {
			es[i] = spanTestEndpoint(t, NodeID(i+1), SpanTied, "start", 1)
		}
		// One domain and one repeated identity cost the same scratch irrespective
		// of endpoint count; no associations or retained output are made here.
		for _, limit := range []int64{2816, 2817} {
			s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
			err := spanLocalTiePairs(context.Background(), s, es)
			if limit == 2816 && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("one-under count%d err=%v", count, err)
			}
			if limit == 2817 && err != nil {
				t.Fatalf("exact scratch count%d err=%v", count, err)
			}
			if s.retained != 0 || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("scratch leaked count%d retained%d", count, s.retained)
			}
			s.cleanup()
		}
		s := rationalSession(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: count}
		if err := spanLocalTiePairs(ctx, s, es); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if s.retained != 0 || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cancel leaked scratch")
		}
		s.cleanup()
	}
}

func TestSpanRound1LyricIndexLendsOneRetainedIdentity(t *testing.T) {
	r := spanUnitReader(t)
	note := spanUnitNode(r, spanRole_note, nil)
	lyric := spanUnitNode(r, spanRole_noteLyric, note)
	scratch := spanScratch{s: r.s}
	index := spanLyricIndex{scratch: &scratch}
	index.record(r, lyric)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.s.retained != 0 {
		t.Fatal("temporary lyric identity retained", r.s.retained)
	}
	key, available := index.endpoint(r, lyric)
	if available.State != EvidenceKnown || key == "" {
		t.Fatal(key, available)
	}
	retained := r.s.retained
	for i := 0; i < 100; i++ {
		next, v := index.endpoint(r, lyric)
		if next != key || v.State != EvidenceKnown {
			t.Fatal(next, v)
		}
	}
	if r.s.retained != retained {
		t.Fatal("repeated identity retained again", retained, r.s.retained)
	}
	scratch.release()
	if rationalUsage(t, r.s, BudgetBufferedBytes) != retained {
		t.Fatal("scratch release erased endpoint ownership")
	}
	if key != "a:a:" {
		t.Fatal("unnamed identity changed", key)
	}
}

func TestSpanRound1DispatchSkipsUnrelatedFamiliesWithinBudget(t *testing.T) {
	r := spanUnitReader(t)
	for i := 0; i < 100; i++ {
		n := localNode(NodeID(i+1), 0, "unrelated-node")
		r.s.nodes[n.header.ID] = n
		r.s.sequence = append(r.s.sequence, n.header.ID)
		if i == 0 {
			r.s.root = n
		}
	}
	// These immutable empty declaration indexes are the compiled no-role case.
	r.b.Elements = map[contextRole]map[DeclarationID]struct{}{}
	r.b.ElementRoles = map[DeclarationID][]contextRole{}
	account, err := NewBudgetAccount(Budgets{MaxRuleSteps: 10000})
	if err != nil {
		t.Fatal(err)
	}
	r.s.account = account
	if _, _, err := evaluateContextSpans(context.Background(), r.s, r.s.root, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b}, true); err != nil {
		t.Fatalf("unrelated nodes still probe every family: %v", err)
	}
	if len(r.s.contextResults) != 0 {
		t.Fatal("irrelevant endpoints", r.s.contextResults)
	}
	if service := r.s.contextServices[RuleKey{"spans", 1}]; service.Spans == nil || !service.Spans.Final {
		t.Fatal("provider did not finish")
	}
}

func TestSpanRound1DispatchQuotaErrorCannotBecomeEmptySuccess(t *testing.T) {
	r := spanUnitReader(t)
	root := spanUnitNode(r, spanRole_partList, nil)
	r.b.ElementRoles = map[DeclarationID][]contextRole{}
	account, err := NewBudgetAccount(Budgets{MaxRuleSteps: 7})
	if err != nil {
		t.Fatal(err)
	}
	r.s.account = account
	_, _, err = evaluateContextSpans(context.Background(), r.s, root, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b}, true)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("quota interruption disappeared: %v", err)
	}
	if len(r.s.contextResults) != 0 || r.s.contextServices[RuleKey{"spans", 1}].Spans != nil {
		t.Fatal("interrupted provider published")
	}
	if r.s.retained != 2053 {
		t.Fatal("provider rollback leaked output or scratch", r.s.retained)
	}
}
