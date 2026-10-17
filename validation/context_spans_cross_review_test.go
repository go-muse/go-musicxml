package validation

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestCrossReviewSpanUnknownCategoryCannotExcludeGraceRun(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	a := spanTestEndpoint(t, 1, SpanArpeggiate, "", 1)
	b := spanTestEndpoint(t, 2, SpanArpeggiate, "", 1)
	cause := &DecisionRef{Rule: RuleKey{"category", 1}, Subject: b.ref}
	for _, e := range []*spanEndpoint{a, b} {
		e.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: NodeRef{"d", 10}, Measure: NodeRef{"d", 20}, Offset: timelineZero}}
	}
	a.categoryKnown = true
	a.grace = true
	b.categoryKnown = false
	b.complete = Evidence[bool]{State: EvidenceUnknown, Reason: "note-category-unavailable", Cause: cause}
	if err := spanVerticalMembership(context.Background(), s, []*spanEndpoint{a, b}); err != nil {
		t.Fatal(err)
	}
	if a.payload.Association.State == EvidenceKnown {
		t.Fatalf("unknown category could belong to grace run: known grace=%+v unknown=%+v", a.payload.Association, b.payload.Association)
	}
}

func TestCrossReviewSpanCategoryWildcardKeepsKnownAxes(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceUnsupported} {
		for _, category := range []struct {
			name       string
			grace, cue bool
		}{{"principal", false, false}, {"grace", true, false}, {"cue", false, true}} {
			for _, change := range []string{"same", "part", "measure", "ordinal", "offset", "number", "family"} {
				t.Run(string(state)+"/"+category.name+"/"+change, func(t *testing.T) {
					s := rationalSession(t, Budgets{})
					defer s.cleanup()
					known := spanTestEndpoint(t, 1, SpanArpeggiate, "", 1)
					gap := spanTestEndpoint(t, 2, SpanArpeggiate, "", 1)
					for _, e := range []*spanEndpoint{known, gap} {
						e.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: NodeRef{"d", 10}, Measure: NodeRef{"d", 20}, Offset: timelineZero}}
					}
					known.categoryKnown = true
					known.grace = category.grace
					known.cue = category.cue
					cause := &DecisionRef{Rule: RuleKey{"category", 1}, Subject: gap.ref}
					gap.categoryKnown = false
					gap.complete = Evidence[bool]{State: state, Reason: "category-cause", Cause: cause}
					switch change {
					case "part":
						gap.payload.Position.Value.Part.Node++
					case "measure":
						gap.payload.Position.Value.Measure.Node++
					case "ordinal":
						gap.payload.Position.Value.MeasureOrdinal++
					case "offset":
						gap.payload.Position.Value.Offset = mustRational(t, 1, 1)
					case "number":
						gap.payload.Number = spanInteger(2)
					case "family":
						gap.payload.Family = SpanNonArpeggiate
					}
					if err := spanVerticalMembership(context.Background(), s, []*spanEndpoint{known, gap}); err != nil {
						t.Fatal(err)
					}
					got := known.payload.Association
					if change == "same" {
						if got.State != state || got.Reason != gap.complete.Reason || got.Cause == nil || *got.Cause != *cause || len(known.payload.Members) != 0 {
							t.Fatalf("category gap lost: %+v", known.payload)
						}
					} else if got.State != EvidenceKnown || !got.Value || len(known.payload.Members) != 1 {
						t.Fatalf("unrelated known axis contaminated: %+v", known.payload)
					}
					if gap.payload.Association.State != state || gap.payload.Association.Cause == nil || *gap.payload.Association.Cause != *cause {
						t.Fatal("unknown-category endpoint cause lost", gap.payload)
					}
				})
			}
		}
	}
}

func TestCrossReviewSpanUnknownCategoryAndNumberInterfereTogether(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	var endpoints []*spanEndpoint
	for i, category := range []struct{ grace, cue bool }{{false, false}, {true, false}, {false, true}} {
		e := spanTestEndpoint(t, NodeID(i+1), SpanArpeggiate, "", int64(i+1))
		e.categoryKnown = true
		e.grace = category.grace
		e.cue = category.cue
		e.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: NodeRef{"d", 10}, Measure: NodeRef{"d", 20}, Offset: timelineZero}}
		endpoints = append(endpoints, e)
	}
	gap := spanTestEndpoint(t, 4, SpanArpeggiate, "", 1)
	gap.categoryKnown = false
	gap.payload.Position = endpoints[0].payload.Position
	cause := &DecisionRef{Rule: RuleKey{"category", 1}, Subject: gap.ref}
	gap.complete = Evidence[bool]{State: EvidenceUnknown, Reason: "category-gap", Cause: cause}
	gap.payload.Number = stateUnknown[Atomic]("number-gap")
	endpoints = append(endpoints, gap)
	if err := spanVerticalMembership(context.Background(), s, endpoints); err != nil {
		t.Fatal(err)
	}
	for _, e := range endpoints {
		a := e.payload.Association
		if a.State != EvidenceUnknown || a.Cause == nil || *a.Cause != *cause || len(e.payload.Members) != 0 {
			t.Fatal("two unavailable axes fabricated exclusion", e.payload)
		}
	}
}

func TestCrossReviewBeamUnknownCategoryKeepsKnownAxes(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceUnsupported} {
		for _, category := range []struct {
			name       string
			grace, cue bool
		}{{"principal", false, false}, {"grace", true, false}, {"cue", false, true}} {
			for _, axis := range []string{"same", "part", "number", "voice-value", "voice-presence"} {
				t.Run(string(state)+"/"+category.name+"/"+axis, func(t *testing.T) {
					s := rationalSession(t, Budgets{})
					defer s.cleanup()
					a, b, gap := spanTestEndpoint(t, 1, SpanBeam, "begin", 1), spanTestEndpoint(t, 2, SpanBeam, "end", 1), spanTestEndpoint(t, 3, SpanBeam, "continue", 1)
					for _, e := range []*spanEndpoint{a, b, gap} {
						e.payload.Identity = SpanIdentityBeamLevel
						e.categoryKnown = true
						e.grace = category.grace
						e.cue = category.cue
					}
					gap.categoryKnown = false
					gap.grace = false
					gap.cue = false
					cause := &DecisionRef{Rule: RuleKey{"category", 1}, Subject: gap.ref}
					gap.complete = Evidence[bool]{State: state, Reason: "beam-category-gap", Cause: cause}
					switch axis {
					case "part":
						gap.payload.Domain.Value.Node++
					case "number":
						gap.payload.Number = spanInteger(2)
					case "voice-value":
						gap.payload.Voice = Evidence[string]{State: EvidenceKnown, Value: "second"}
					case "voice-presence":
						gap.payload.Voice = Evidence[string]{State: EvidenceKnown, Value: ""}
					}
					if err := associateSpanEndpoints(context.Background(), s, []*spanEndpoint{a, gap, b}, nil); err != nil {
						t.Fatal(err)
					}
					for _, e := range []*spanEndpoint{a, b} {
						got := e.payload.Association
						if axis == "same" {
							if got.State != state || got.Reason != gap.complete.Reason || got.Cause == nil || *got.Cause != *cause || len(e.payload.Members) != 0 {
								t.Fatal("category gap excluded compatible beam", e.payload)
							}
						} else if got.State != EvidenceKnown || !got.Value || len(e.payload.Members) != 2 {
							t.Fatal("unrelated beam scope contaminated", e.payload)
						}
					}
				})
			}
		}
	}
}

func TestCrossReviewBeamCategoryWildcardPreservesRemainingIdentity(t *testing.T) {
	for _, missing := range []string{"number", "voice", "part"} {
		t.Run(missing, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			var endpoints []*spanEndpoint
			for group := 0; group < 2; group++ {
				for _, role := range []string{"begin", "end"} {
					e := spanTestEndpoint(t, NodeID(len(endpoints)+1), SpanBeam, role, 1)
					e.payload.Identity = SpanIdentityBeamLevel
					e.categoryKnown = true
					e.grace = true
					e.payload.Voice = Evidence[string]{State: EvidenceKnown, Value: "one"}
					if group == 1 {
						if missing == "voice" {
							e.payload.Number = spanInteger(2)
						} else {
							e.payload.Voice.Value = "two"
						}
					}
					endpoints = append(endpoints, e)
				}
			}
			gap := spanTestEndpoint(t, 5, SpanBeam, "continue", 1)
			gap.payload.Identity = SpanIdentityBeamLevel
			gap.categoryKnown = false
			gap.payload.Voice = Evidence[string]{State: EvidenceKnown, Value: "one"}
			cause := &DecisionRef{Rule: RuleKey{"category", 1}, Subject: gap.ref}
			gap.complete = Evidence[bool]{State: EvidenceUnknown, Reason: "category-gap", Cause: cause}
			switch missing {
			case "number":
				gap.payload.Number = stateUnknown[Atomic]("number-gap")
			case "voice":
				gap.payload.Voice = stateUnknown[string]("voice-gap")
			case "part":
				gap.payload.Domain = stateUnknown[NodeRef]("part-gap")
			}
			endpoints = append(endpoints, gap)
			if err := associateSpanEndpoints(context.Background(), s, endpoints, nil); err != nil {
				t.Fatal(err)
			}
			for i, e := range endpoints[:4] {
				got := e.payload.Association
				if i < 2 {
					if got.State != EvidenceUnknown || got.Cause == nil || *got.Cause != *cause {
						t.Fatal("compatible partial identity excluded", missing, e.payload)
					}
				} else if got.State != EvidenceKnown {
					t.Fatal("known identity axis ignored", missing, e.payload)
				}
			}
		})
	}
}

func TestCrossReviewSpanExtractionPreservesCategoryCause(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceUnsupported} {
		r := spanUnitReader(t)
		note := spanUnitNode(r, spanRole_note, nil)
		child := spanUnitNode(r, spanRole_notePitch, note)
		cause := &DecisionRef{Rule: RuleKey{"declaration", 1}, Subject: NodeRef{r.s.documentID, child.header.ID}}
		child.header.Declaration = testEvidence(Evidence[DeclarationID]{State: state, Reason: "category-declaration-gap", Cause: cause})
		mark := spanUnitNode(r, spanRole_arpeggiate, note)
		e := spanExtract(r, RuleKey{"spans", 1}, mark, spanFamilyBinding{family: SpanArpeggiate, node: spanRole_arpeggiate, number: spanRole_arpeggiate_number, identity: SpanIdentityNumber}, nil, nil, 1, nil)
		if r.err != nil {
			t.Fatal(r.err)
		}
		if e.categoryKnown || e.complete.State != state || e.complete.Reason != "category-declaration-gap" || e.complete.Cause == nil || *e.complete.Cause != *cause {
			t.Fatalf("original category cause replaced: %+v", e.complete)
		}
	}
}

func TestCrossReviewBeamCategoryGapScratchIsReleased(t *testing.T) {
	for _, count := range []int{100, 1000} {
		endpoints := make([]*spanEndpoint, count)
		for i := range endpoints {
			e := spanTestEndpoint(t, NodeID(i+1), SpanBeam, "continue", 1)
			e.payload.Identity = SpanIdentityBeamLevel
			e.categoryKnown = false
			e.complete = stateUnknown[bool]("category-gap")
			endpoints[i] = e
		}
		for _, limit := range []int64{3073, 3074} {
			s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
			err := associateSpanEndpoints(context.Background(), s, endpoints, nil)
			if limit == 3073 && !errors.Is(err, ErrResourceLimit) {
				t.Fatal("one-under scratch", err)
			}
			if limit == 3074 && err != nil {
				t.Fatal("repeated keys grew scratch", count, err)
			}
			if s.retained != 512 || rationalUsage(t, s, BudgetBufferedBytes) != 512 {
				t.Fatal("category scratch was not released", s.retained, rationalUsage(t, s, BudgetBufferedBytes))
			}
			s.cleanup()
		}
		s := rationalSession(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: 50}
		if err := associateSpanEndpoints(ctx, s, endpoints, nil); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if s.retained != 512 || rationalUsage(t, s, BudgetBufferedBytes) != 512 {
			t.Fatal("canceled category scratch leaked")
		}
		s.cleanup()
	}
}

func TestCrossReviewVerticalCategoryGapKeepsNumberWithoutPosition(t *testing.T) {
	for _, placement := range []string{"unavailable", "unproved-frame"} {
		for _, same := range []bool{false, true} {
			t.Run(placement+"/"+strconv.FormatBool(same), func(t *testing.T) {
				s := rationalSession(t, Budgets{})
				defer s.cleanup()
				a, gap := spanTestEndpoint(t, 1, SpanArpeggiate, "", 1), spanTestEndpoint(t, 2, SpanArpeggiate, "", 2)
				a.categoryKnown = true
				a.grace = true
				gap.categoryKnown = false
				a.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: NodeRef{"d", 10}, Measure: NodeRef{"d", 20}, Offset: timelineZero}}
				gap.payload.Domain = Evidence[NodeRef]{State: EvidenceKnown, Value: a.payload.Position.Value.Part}
				cause := &DecisionRef{Rule: RuleKey{"category", 1}, Subject: gap.ref}
				gap.complete = Evidence[bool]{State: EvidenceUnknown, Reason: "category-gap", Cause: cause}
				if same {
					gap.payload.Number = spanInteger(1)
				}
				if placement == "unavailable" {
					gap.payload.Position = stateUnknown[SpanPosition]("position-gap")
				} else {
					gap.payload.Position = a.payload.Position
					gap.payload.Position.Value.UnprovedFrame = true
				}
				if err := spanVerticalMembership(context.Background(), s, []*spanEndpoint{a, gap}); err != nil {
					t.Fatal(err)
				}
				got := a.payload.Association
				if same {
					if got.State != EvidenceUnknown || got.Cause == nil || *got.Cause != *cause {
						t.Fatal("category cause lost", a.payload)
					}
				} else if got.State != EvidenceKnown {
					t.Fatal("established number ignored", a.payload)
				}
			})
		}
	}
}
