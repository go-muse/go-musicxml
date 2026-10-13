package validation

import "testing"

func TestResourceReviewLinkFreeIndexRetainsOnlyPartOwnership(t *testing.T) {
	root := resourceTestNode(1, 0, "score-part")
	child := resourceTestNode(2, 1, "group")
	r, index := resourceTestReader(t, root, child)
	if len(index.position) != 0 || len(index.names) != 0 || len(index.ordinals) != 0 || len(index.complete) != 0 {
		t.Error("link-free document built attachment indexes")
	}
	if index.parts[2].State != EvidenceKnown || index.parts[2].Value.Node != 1 {
		t.Errorf("lightweight part ownership lost: %+v", index.parts)
	}
	if index.bytes >= int64(len(r.s.sequence)+1)*512 {
		t.Errorf("link-free index retains %d bytes", index.bytes)
	}
}

func TestResourceReviewSubjectDeclarationCauseIsOwnedAndRemapped(t *testing.T) {
	node := resourceTestNode(1, 0, "root")
	cause := &DecisionRef{Rule: RuleKey{ID: "declaration", Revision: 1}, Subject: NodeRef{"d", 1}}
	node.header.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "declaration-unavailable", Cause: cause}
	r, _ := resourceTestReader(t, node)
	shell := resourceShell(r.s, node, Evidence[NodeRef]{State: EvidenceAbsent})
	shell.Group = &ResourceGroupContext{Value: Evidence[string]{State: EvidenceAbsent}}
	owned := cloneResourceContextValue(shell)
	remapResourceContextValue(owned, "d", "adopted")
	if owned.SubjectDeclaration.State != EvidenceInvalid || owned.SubjectDeclaration.Reason != "declaration-unavailable" || owned.SubjectDeclaration.Cause == nil || owned.SubjectDeclaration.Cause.Subject.DocumentID != "adopted" {
		t.Fatalf("declaration evidence lost: %+v", owned.SubjectDeclaration)
	}
	if cause.Subject.DocumentID != "d" || shell.SubjectDeclaration.Cause != cause || owned.SubjectDeclaration.Cause == cause {
		t.Fatal("declaration cause aliased source evidence")
	}
}
