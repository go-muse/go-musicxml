package validation

import (
	"fmt"
	"testing"
)

func TestLayoutRound3DisputedBreakUnavailableCauses(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 1}}
		gap := Evidence[bool]{State: state, Reason: "encoded-break-unavailable", Cause: cause}
		for _, peer := range []Evidence[bool]{{State: EvidenceKnown, Value: false}, {State: EvidenceKnown, Value: true}, {State: EvidenceAbsent}, stateUnknown[bool]("other-gap")} {
			for _, pair := range [][2]Evidence[bool]{{gap, peer}, {peer, gap}} {
				got := layoutDisputedBreak(pair[0], pair[1])
				if got.State != state || got.Cause != cause || got.Reason != gap.Reason {
					t.Errorf("disagreement erased cause: %+v", got)
				}
			}
		}
	}
	for _, pair := range [][2]Evidence[bool]{{{State: EvidenceKnown, Value: false}, {State: EvidenceKnown, Value: true}}, {{State: EvidenceKnown, Value: true}, {State: EvidenceKnown, Value: false}}} {
		if got := layoutDisputedBreak(pair[0], pair[1]); got.State != EvidenceUnknown || got.Cause != nil || got.Reason != "parallel-print-break-disagreement" {
			t.Errorf("plain disagreement fabricated defect: %+v", got)
		}
	}
}

func TestLayoutRound3ConflictingAdmittedBreaksEveryOrder(t *testing.T) {
	permutations := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, initial := range []bool{false, true} {
			for _, order := range permutations {
				t.Run(fmt.Sprintf("%s/initial=%t/order=%v", state, initial, order), func(t *testing.T) {
					r, key, p := layoutRound1CachedScope(t)
					r.s.nodes = map[NodeID]*factNode{}
					r.s.sequence = nil
					typ := contextNamedType("print")
					r.b.Attributes = map[contextRole]map[ComponentID]AttributeSelector{}
					for _, name := range []string{"new-page", "new-system", "blank-page"} {
						r.b.Attributes[layoutRole("@"+name)] = map[ComponentID]AttributeSelector{typ: localAttr(name)}
					}
					blocks := []timelineMeasureBlock{}
					ordinal := uint64(0)
					id := NodeID(1)
					add := func(index int, statement int) {
						measure := localNode(id, 0, "measure")
						id++
						r.s.nodes[measure.header.ID] = measure
						r.s.sequence = append(r.s.sequence, measure.header.ID)
						if statement >= 0 {
							print := localNode(id, measure.header.ID, "print")
							id++
							print.header.Type = Evidence[ComponentID]{State: EvidenceKnown, Value: typ}
							value := "yes"
							if statement == 1 {
								value = "no"
							}
							localSetAttr(print, "new-page", localString(value))
							if statement == 2 {
								print.attributes[0].Scalar.Value = Evidence[Atomic]{State: state, Reason: "admitted-break-unavailable", Cause: &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", print.header.ID}}}
							}
							measure.children = []*factNode{print}
							r.s.nodes[print.header.ID] = print
							r.s.sequence = append(r.s.sequence, print.header.ID)
						}
						blocks = append(blocks, timelineMeasureBlock{measure: measure, content: measure, ordinal: ordinal, part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", NodeID(100 + index)}}, complete: Evidence[bool]{State: EvidenceKnown, Value: true}, coverage: Evidence[bool]{State: EvidenceKnown, Value: true}})
					}
					if !initial {
						for i := 0; i < 3; i++ {
							add(i, -1)
						}
						ordinal++
					}
					for i, statement := range order {
						add(i, statement)
					}
					r.s.contextServices[key].Layout.Blocks = blocks
					if err := evaluateLayoutScopes(r.ctx, r.s, key, p, nil); err != nil {
						t.Fatal(err)
					}
					widths := 0
					for _, row := range r.s.contextResults {
						if row.Property != "layout.width-outcome" || row.Layout.System.State == EvidenceKnown {
							continue
						}
						widths++
						if row.Layout.Provenance != ContextExplicit || row.Layout.Raw.State != EvidenceAbsent {
							t.Errorf("conflict provenance/raw: %+v", row.Layout)
						}
						wantReason := Code("admitted-break-unavailable")
						if state == EvidenceUnknown {
							wantReason = "parallel-print-rendering-unspecified"
						}
						if row.Value.State != state || row.Value.Cause == nil || row.Value.Cause.Rule.ID != "fixture" || row.Value.Reason != wantReason {
							t.Errorf("conflict outcome lost cause: %+v", row.Value)
						}
					}
					if widths != 3 {
						t.Errorf("conflict width rows=%d", widths)
					}
					count := 0
					for _, row := range r.s.contextResults {
						if row.Property != "layout.page-height" {
							continue
						}
						count++
						if !initial && count <= 3 {
							continue
						}
						for _, got := range []Evidence[NodeRef]{row.Layout.Page, row.Layout.System} {
							if got.State != state || got.Cause == nil || got.Cause.Rule.ID != "fixture" || got.Reason != "admitted-break-unavailable" {
								t.Errorf("admitted conflict lost cause: %+v", got)
							}
						}
					}
					want := 3
					if !initial {
						want = 6
					}
					if count != want {
						t.Errorf("page rows=%d want=%d", count, want)
					}
				})
			}
		}
	}
}
