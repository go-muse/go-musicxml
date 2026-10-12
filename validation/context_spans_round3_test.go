package validation

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// These are admitted provider streams with an independently established part
// identity. Adapter hierarchy gaps have a separate canonical-identity gate.
func TestSpanRound3LocalTieHonorsHierarchyGaps(t *testing.T) {
	for _, hierarchy := range []string{"partwise", "timewise"} {
		for _, state := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceInvalid, EvidenceUnsupported} {
			for _, axis := range []string{"children", "order"} {
				t.Run(hierarchy+"/"+string(state)+"/"+axis, func(t *testing.T) {
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
						if parent == nil {
							r.s.root = n
						} else {
							parent.children = append(parent.children, n)
						}
						return n
					}
					root := structural("score-"+hierarchy, nil)
					timelineKey := RuleKey{"timeline", 1}
					timeline := &timelineService{Final: true, Facts: map[NodeID]timelineEventFact{}}
					cause := &DecisionRef{Rule: RuleKey{"enumeration", 1}, Subject: NodeRef{r.s.documentID, 999}}
					gap := Evidence[bool]{State: state, Value: true, Reason: "missing-measure-content", Cause: cause}
					add := func(parent *factNode, part, measure int) {
						note := spanUnitNode(r, spanRole_note, parent)
						timeline.Facts[note.header.ID] = timelineEventFact{Part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, NodeID(1001 + part)}}, Position: stateUnknown[musicalPosition]("independent-position-gap")}
						if measure == 0 {
							for _, role := range []string{"start", "stop"} {
								tied := spanUnitNode(r, spanRole_tied, note)
								spanUnitAttribute(r, tied, spanRole_tied_type, spanString(role))
								spanUnitAttribute(r, tied, spanRole_tied_number, spanInteger(1))
							}
						}
					}
					applyGap := func(n *factNode) {
						if state != EvidenceKnown {
							if axis == "children" {
								n.end.ChildrenComplete = gap
							} else {
								n.end.Order = stateUnavailable[bool, OrderKind](gap)
							}
						}
					}
					if hierarchy == "partwise" {
						for p := 0; p < 2; p++ {
							part := structural("part", root)
							for m := 0; m < 2; m++ {
								measure := structural("measure", part)
								add(measure, p, m)
								if p == 0 && m == 1 {
									applyGap(measure)
								}
							}
						}
					} else {
						for m := 0; m < 2; m++ {
							measure := structural("measure", root)
							for p := 0; p < 2; p++ {
								part := structural("part", measure)
								add(part, p, m)
							}
							if m == 1 {
								applyGap(measure)
							}
						}
					}
					r.s.timeline = map[RuleKey]*timelineService{timelineKey: timeline}
					if _, _, err := evaluateContextSpans(context.Background(), r.s, root, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b, Timeline: timelineKey}, true); err != nil {
						t.Fatal(err)
					}
					count := 0
					for _, row := range r.s.contextResults {
						if row.Span == nil || row.Span.Family != SpanTied {
							continue
						}
						count++
						a := row.Span.Association
						unavailable := state != EvidenceKnown && (hierarchy == "timewise" || row.Span.Domain.Value.Node == 1001)
						if unavailable {
							if a.State != state || a.Reason != gap.Reason || a.Cause == nil || *a.Cause != *cause || len(row.Span.Members) != 0 {
								t.Errorf("stub bypassed hierarchy gap: %+v", row.Span)
							}
						} else if a.State != EvidenceKnown || !a.Value || len(row.Span.Members) != 2 {
							t.Errorf("independent stub lost: %+v", row.Span)
						}
					}
					if count != 4 {
						t.Fatal("endpoint count", count)
					}
				})
			}
		}
	}
}

func TestSpanRound3TiedAmbiguityIsIdentityScoped(t *testing.T) {
	for _, axis := range []string{"same-known-position", "same-unavailable-position", "other-number", "other-part", "other-implied-pitch"} {
		for _, permutation := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
			t.Run(axis+fmt.Sprint(permutation), func(t *testing.T) {
				s := rationalSession(t, Budgets{})
				defer s.cleanup()
				a, b := spanTestEndpoint(t, 1, SpanTied, "start", 1), spanTestEndpoint(t, 2, SpanTied, "stop", 1)
				b.payload.Anchor = a.payload.Anchor
				c, d := spanTestEndpoint(t, 3, SpanTied, "start", 1), spanTestEndpoint(t, 4, SpanTied, "stop", 1)
				for i, e := range []*spanEndpoint{a, b, c, d} {
					e.payload.Position = stateUnknown[SpanPosition]("position-gap")
					if axis == "same-known-position" {
						offset, _ := NewRational(big.NewRat(int64(i/2), 1))
						e.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: e.payload.Domain.Value, Measure: NodeRef{"d", 100}, Offset: offset}}
					}
				}
				switch axis {
				case "other-number":
					c.payload.Number = spanInteger(2)
					d.payload.Number = spanInteger(2)
				case "other-part":
					c.payload.Domain.Value.Node = 101
					d.payload.Domain = c.payload.Domain
				case "other-implied-pitch":
					for _, e := range []*spanEndpoint{a, b, c, d} {
						e.payload.Identity = SpanIdentityImpliedTie
						e.payload.Number = Evidence[Atomic]{State: EvidenceAbsent}
						e.pitch = "60"
					}
					c.pitch = "61"
					d.pitch = "61"
				}
				groups := [][]*spanEndpoint{{a, b}, {c}, {d}}
				var es []*spanEndpoint
				for _, i := range permutation {
					es = append(es, groups[i]...)
				}
				if err := spanLocalTiePairs(context.Background(), s, es); err != nil {
					t.Fatal(err)
				}
				if err := associateSpanEndpoints(context.Background(), s, es, nil); err != nil {
					t.Fatal(err)
				}
				wantKnown := strings.HasPrefix(axis, "other-")
				for _, e := range es {
					if (e.payload.Association.State == EvidenceKnown) != wantKnown {
						t.Errorf("identity contamination: %+v", e.payload)
					}
					if !wantKnown && e.payload.Association.Reason != "span-tied-local-interpretation-ambiguous" {
						t.Errorf("missing explicit ambiguity: %+v", e.payload)
					}
				}
			})
		}
	}
}

func TestSpanRound3TiedPreflightScratchAndWorkBounds(t *testing.T) {
	for _, count := range []int{100, 1000} {
		es := make([]*spanEndpoint, count+1)
		indices := make([]int, len(es))
		for i := range count {
			es[i] = spanTestEndpoint(t, NodeID(i+1), SpanTied, "start", 1)
			indices[i] = i
		}
		es[count] = spanTestEndpoint(t, NodeID(count+1), SpanTied, "stop", 1)
		es[count].payload.Anchor = es[count-1].payload.Anchor
		indices[count] = count
		cap := int64(512 + 257*count)
		for _, limit := range []int64{cap - 1, cap} {
			s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
			ambiguous, err := spanTiedRunAmbiguous(context.Background(), s, es, indices)
			if limit < cap && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("one-under count%d: %v", count, err)
			}
			if limit == cap && (err != nil || !ambiguous) {
				t.Fatalf("exact count%d: %t %v", count, ambiguous, err)
			}
			if s.retained != 0 || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("preflight scratch leaked")
			}
			if used := rationalUsage(t, s, BudgetRuleSteps); used > int64(2*len(es)+1) {
				t.Fatal("nonlinear preflight", used)
			}
			s.cleanup()
		}
		s := rationalSession(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: count / 2}
		if _, err := spanTiedRunAmbiguous(ctx, s, es, indices); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation unobserved", err)
		}
		if s.retained != 0 || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("canceled scratch leaked")
		}
		s.cleanup()
	}
}

func TestSpanRound3UnnamedLyricScopePreservesIdentityEvidence(t *testing.T) {
	for _, axis := range []string{"duplicate-known-domain", "duplicate-unknown-domain", "invalid", "unknown", "unsupported", "malformed-known"} {
		t.Run(axis, func(t *testing.T) {
			r := spanUnitReader(t)
			note := spanUnitNode(r, spanRole_note, nil)
			part := NodeRef{r.s.documentID, 1001}
			scratch := spanScratch{s: r.s}
			defer scratch.release()
			index := spanLyricIndex{scratch: &scratch, domains: map[NodeID]Evidence[NodeRef]{note.header.ID: {State: EvidenceKnown, Value: part}}}
			if axis == "duplicate-unknown-domain" {
				index.domains = nil
				part = NodeRef{}
			}
			cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: NodeRef{r.s.documentID, 9001}}
			want := Evidence[bool]{State: EvidenceUnknown, Reason: "span-lyric-line-ambiguous"}
			lyric := spanUnitNode(r, spanRole_noteLyric, note)
			if strings.HasPrefix(axis, "duplicate-") {
				index.record(r, lyric)
				lyric = spanUnitNode(r, spanRole_noteLyric, note)
			} else {
				value := Evidence[Atomic]{State: EvidenceState(axis), Reason: "identity-gap", Cause: cause}
				want = stateUnavailable[Atomic, bool](value)
				if axis == "malformed-known" {
					value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicBoolean}}
					want = stateUnknown[bool]("span-lyric-identity-unavailable")
				}
				spanUnitAttribute(r, lyric, spanRole_lyric_number, value)
			}
			index.record(r, lyric)
			if r.err != nil {
				t.Fatal(r.err)
			}
			got := index.unnamed[part]
			if got.State != want.State || got.Reason != want.Reason {
				t.Fatalf("gap=%+v want=%+v", got, want)
			}
			if want.Cause != nil && (got.Cause == nil || *got.Cause != *want.Cause) {
				t.Fatal("identity cause lost", got)
			}
		})
	}
}

// A duplicate whose admitted hierarchy has no canonical part can compete with
// either musical part. This deliberately supplies a valid timeline: authentic
// adapter hierarchy failures retain their earlier identity-scope-incomplete gate.
func TestSpanRound3UnknownDomainUnnamedAmbiguityReachesOnlyUnnamed(t *testing.T) {
	r := spanUnitReader(t)
	root := localNode(1, 0, "score")
	root.header.Location.DocumentID = r.s.documentID
	r.s.root = root
	r.s.nodes[1] = root
	r.s.sequence = []NodeID{1}
	timelineKey := RuleKey{"timeline", 1}
	timeline := &timelineService{Final: true, Facts: map[NodeID]timelineEventFact{}}
	addLyric := func(note *factNode, number, role string) {
		lyric := spanUnitNode(r, spanRole_noteLyric, note)
		if number != "" {
			spanUnitAttribute(r, lyric, spanRole_lyric_number, spanString(number))
		}
		if role != "" {
			extend := spanUnitNode(r, spanRole_lyricExtend, lyric)
			spanUnitAttribute(r, extend, spanRole_extend_type, spanString(role))
		}
	}
	for p := 0; p < 2; p++ {
		for _, role := range []string{"start", "stop"} {
			note := spanUnitNode(r, spanRole_note, root)
			timeline.Facts[note.header.ID] = timelineEventFact{Part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, NodeID(1001 + p)}}, Position: stateUnknown[musicalPosition]("independent-position-gap")}
			addLyric(note, "", role)
			addLyric(note, "1", role)
		}
	}
	unknown := spanUnitNode(r, spanRole_note, root)
	addLyric(unknown, "", "")
	addLyric(unknown, "", "")
	r.s.timeline = map[RuleKey]*timelineService{timelineKey: timeline}
	if _, _, err := evaluateContextSpans(context.Background(), r.s, root, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b, Timeline: timelineKey}, true); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range r.s.contextResults {
		if row.Span == nil || row.Span.Family != SpanLyricExtend {
			continue
		}
		e := row.Span
		if count%2 == 0 {
			if e.Association.State != EvidenceUnknown || e.Association.Reason != "span-lyric-line-ambiguous" || len(e.Members) != 0 {
				t.Errorf("unknown-domain gap missing: %+v", e)
			}
		} else if e.Association.State != EvidenceKnown || len(e.Members) != 2 {
			t.Errorf("named line contaminated: %+v", e)
		}
		count++
	}
	if count != 8 {
		t.Fatal("endpoint count", count)
	}
}

func TestSpanRound3LyricDomainIndexScratchAndCancellation(t *testing.T) {
	run := func(count int, limits Budgets, cancelAt int) (int64, int64, error) {
		r := spanUnitReader(t)
		account, err := NewBudgetAccount(limits)
		if err != nil {
			t.Fatal(err)
		}
		r.s.account = account
		if cancelAt > 0 {
			r.ctx = &rationalCancelContext{Context: context.Background(), at: cancelAt}
		}
		scratch := spanScratch{s: r.s}
		index := spanLyricIndex{scratch: &scratch, domains: map[NodeID]Evidence[NodeRef]{}}
		for i := 0; i < count && r.err == nil; i++ {
			note := spanUnitNode(r, spanRole_note, nil)
			index.domains[note.header.ID] = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, 1001}}
			for j := 0; j < 2 && r.err == nil; j++ {
				index.record(r, spanUnitNode(r, spanRole_noteLyric, note))
			}
		}
		bytes, steps := rationalUsage(t, r.s, BudgetBufferedBytes), rationalUsage(t, r.s, BudgetRuleSteps)
		if r.s.retained != 0 {
			t.Fatal("lyric index charged permanent storage", r.s.retained)
		}
		scratch.release()
		if rationalUsage(t, r.s, BudgetBufferedBytes) != 0 {
			t.Fatal("lyric index scratch leaked")
		}
		return bytes, steps, r.err
	}
	var previousWork int64
	for _, count := range []int{100, 1000} {
		exact, work, err := run(count, Budgets{}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if previousWork > 0 && work > previousWork*11 {
			t.Fatal("nonlinear lyric domain index", previousWork, work)
		}
		previousWork = work
		if _, _, err := run(count, Budgets{MaxBufferedBytes: exact}, 0); err != nil {
			t.Fatal("exact lyric cap", err)
		}
		if _, _, err := run(count, Budgets{MaxBufferedBytes: exact - 1}, 0); !errors.Is(err, ErrResourceLimit) {
			t.Fatal("one-under lyric cap", err)
		}
		if _, _, err := run(count, Budgets{}, count); !errors.Is(err, context.Canceled) {
			t.Fatal("lyric cancellation unobserved", err)
		}
	}
}

func TestSpanRound3PartialLyricIdentityRetainsKnownAxis(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceInvalid, EvidenceUnknown, EvidenceUnsupported} {
		for _, knownAxis := range []contextRole{spanRole_lyric_name, spanRole_lyric_number} {
			texts := []string{"verse", ""}
			if knownAxis == spanRole_lyric_number {
				texts = []string{"1", "01"}
			}
			for _, knownText := range texts {
				t.Run(string(state)+"/"+string(knownAxis)+"/"+knownText, func(t *testing.T) {
					r := spanUnitReader(t)
					note := spanUnitNode(r, spanRole_note, nil)
					lyric := spanUnitNode(r, spanRole_noteLyric, note)
					part := NodeRef{r.s.documentID, 1001}
					scratch := spanScratch{s: r.s}
					defer scratch.release()
					index := spanLyricIndex{scratch: &scratch, domains: map[NodeID]Evidence[NodeRef]{note.header.ID: {State: EvidenceKnown, Value: part}}}
					unavailableAxis := spanRole_lyric_number
					if knownAxis == spanRole_lyric_number {
						unavailableAxis = spanRole_lyric_name
					}
					cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: NodeRef{r.s.documentID, lyric.header.ID}}
					value := Evidence[Atomic]{State: state, Reason: "partial-identity-gap", Cause: cause}
					spanUnitAttribute(r, lyric, knownAxis, spanString(knownText))
					spanUnitAttribute(r, lyric, unavailableAxis, value)
					index.record(r, lyric)
					if r.err != nil {
						t.Fatal(r.err)
					}
					if gap, ok := index.unnamed[part]; ok {
						t.Errorf("independent known axis discarded: %+v", gap)
					}
					identity, available := index.endpoint(r, lyric)
					if identity != "" || available.State != state || available.Reason != value.Reason || available.Cause == nil || *available.Cause != *cause {
						t.Errorf("affected exact identity lost its cause: %q %+v", identity, available)
					}
				})
			}
		}
	}
}

func TestSpanRound3PartialLyricIdentityNeedsKnownStringAxis(t *testing.T) {
	for _, axis := range []contextRole{spanRole_lyric_name, spanRole_lyric_number} {
		for _, kind := range []AtomicKind{AtomicBoolean, AtomicInteger, ""} {
			t.Run(string(axis)+"/"+string(kind), func(t *testing.T) {
				r := spanUnitReader(t)
				note := spanUnitNode(r, spanRole_note, nil)
				lyric := spanUnitNode(r, spanRole_noteLyric, note)
				part := NodeRef{r.s.documentID, 1001}
				scratch := spanScratch{s: r.s}
				defer scratch.release()
				index := spanLyricIndex{scratch: &scratch, domains: map[NodeID]Evidence[NodeRef]{note.header.ID: {State: EvidenceKnown, Value: part}}}
				unavailableAxis := spanRole_lyric_number
				if axis == spanRole_lyric_number {
					unavailableAxis = spanRole_lyric_name
				}
				cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: NodeRef{r.s.documentID, lyric.header.ID}}
				spanUnitAttribute(r, lyric, axis, Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: kind}})
				spanUnitAttribute(r, lyric, unavailableAxis, Evidence[Atomic]{State: EvidenceInvalid, Reason: "partial-invalid", Cause: cause})
				index.record(r, lyric)
				if r.err != nil {
					t.Fatal(r.err)
				}
				gap, ok := index.unnamed[part]
				if !ok || gap.State != EvidenceInvalid || gap.Cause == nil || *gap.Cause != *cause {
					t.Fatalf("mistyped Known axis invented exclusion: %+v present=%t", gap, ok)
				}
			})
		}
	}
}

func TestSpanRound3ExcludedUnavailableSiblingKeepsUnnamedCauseIndependent(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceUnsupported} {
		for _, excludedFirst := range []bool{false, true} {
			t.Run(string(state)+fmt.Sprint(excludedFirst), func(t *testing.T) {
				r := spanUnitReader(t)
				note := spanUnitNode(r, spanRole_note, nil)
				part := NodeRef{r.s.documentID, 1001}
				scratch := spanScratch{s: r.s}
				defer scratch.release()
				index := spanLyricIndex{scratch: &scratch, domains: map[NodeID]Evidence[NodeRef]{note.header.ID: {State: EvidenceKnown, Value: part}}}
				var excluded []*factNode
				for _, named := range []struct {
					axis contextRole
					text string
				}{{spanRole_lyric_name, "verse"}, {spanRole_lyric_name, ""}, {spanRole_lyric_number, "1"}} {
					n := spanUnitNode(r, spanRole_noteLyric, note)
					unavailableAxis := spanRole_lyric_number
					if named.axis == spanRole_lyric_number {
						unavailableAxis = spanRole_lyric_name
					}
					cause := &DecisionRef{Rule: RuleKey{"excluded-identity", 1}, Subject: NodeRef{r.s.documentID, n.header.ID}}
					spanUnitAttribute(r, n, unavailableAxis, Evidence[Atomic]{State: EvidenceInvalid, Reason: "excluded-identity", Cause: cause})
					spanUnitAttribute(r, n, named.axis, spanString(named.text))
					excluded = append(excluded, n)
				}
				possible := spanUnitNode(r, spanRole_noteLyric, note)
				wantedCause := &DecisionRef{Rule: RuleKey{"possible-number", 1}, Subject: NodeRef{r.s.documentID, possible.header.ID}}
				spanUnitAttribute(r, possible, spanRole_lyric_number, Evidence[Atomic]{State: state, Reason: "possibly-unnamed", Cause: wantedCause})
				nodes := append([]*factNode{possible}, excluded...)
				if excludedFirst {
					nodes = append(excluded, possible)
				}
				for _, n := range nodes {
					index.record(r, n)
				}
				if r.err != nil {
					t.Fatal(r.err)
				}
				got := index.unnamed[part]
				if got.State != state || got.Reason != "possibly-unnamed" || got.Cause == nil || *got.Cause != *wantedCause {
					t.Fatalf("excluded sibling leaked into unnamed scope: %+v", got)
				}
			})
		}
	}
}
