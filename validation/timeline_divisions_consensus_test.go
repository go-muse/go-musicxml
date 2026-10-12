package validation

import (
	"context"
	"testing"
)

func TestTimelineUnplacedDivisionsPreserveOnlyIndependentSameValue(t *testing.T) {
	for _, mode := range []string{"same", "different", "missing-baseline", "invalid", "hidden", "repaired-different", "excluded-different"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s, svc, b := runtimeFixture(t)
			h := &svc.State
			stateClose(t, s, h, 3)
			at := statePosition(t, 1, 1, 1)
			if mode != "missing-baseline" {
				statePublish(t, s, &h.Divisions, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 1, 1)), 20)
			}
			value := stateAtomicDecimal(t, 1, 1)
			if mode == "different" || mode == "repaired-different" || mode == "excluded-different" {
				value = stateAtomicDecimal(t, 2, 1)
			}
			cause := &DecisionRef{Rule: RuleKey{"invalid-divisions", 1}, Subject: NodeRef{"d", 31}}
			if mode == "invalid" {
				value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-divisions", Cause: cause}
			}
			child := stateNode(b, timelineRoleAttributesDivisions, 31, value)
			attrs := stateNode(b, timelineRoleAttributes, 30, stateAbsent[Atomic](), child)
			if mode == "hidden" {
				child.header.Declaration = Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "hidden-divisions"}
			}
			placement := stateGapPlacement{Displaced: true}
			if mode == "excluded-different" {
				future := statePosition(t, 2, 0, 1)
				placement.Lower = &future
			}
			if mode == "repaired-different" {
				prior := statePosition(t, 0, 1, 1)
				placement.Upper = &prior
				statePublish(t, s, &h.Divisions, statePosition(t, 1, 0, 1), stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 1, 1)), 40)
			}
			if err := h.gapAttributes(ctx, s, b, attrs, at.Part, 1, Evidence[musicalPosition]{State: EvidenceUnsupported, Reason: "unplaced-divisions"}, placement); err != nil {
				t.Fatal(err)
			}
			got, err := h.effectiveDivisions(ctx, s, at)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := h.effectiveDivisionsEvidence(ctx, s, at)
			if err != nil {
				t.Fatal(err)
			}
			wantKnown := mode == "same" || mode == "repaired-different" || mode == "excluded-different"
			if (got.Value.State == EvidenceKnown) != wantKnown || got.Value.State != evidence.State {
				t.Fatalf("unexpected consensus: %+v evidence=%+v", got, evidence)
			}
			if wantKnown && got.Value.Value.String() != "1" {
				t.Fatalf("invented value: %+v", got)
			}
			if mode == "invalid" && (got.Value.State != EvidenceInvalid || got.Value.Cause == nil || *got.Value.Cause != *cause) {
				t.Fatalf("lost invalid cause: %+v", got)
			}
			if mode == "same" {
				if !got.InheritanceUnproved || got.Inherited || len(got.Provenance) != 2 || got.Provenance[1].Node.Node != 31 {
					t.Fatalf("unplaced source or uncertain inheritance lost: %+v", got)
				}
				got.Provenance[1].Node.Node = 999
				again, err := h.effectiveDivisions(ctx, s, at)
				if err != nil || again.Provenance[1].Node.Node != 31 {
					t.Fatalf("borrowed witness alias: %+v %v", again, err)
				}
			}
		})
	}
}
