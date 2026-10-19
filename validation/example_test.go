package validation_test

import (
	"context"
	"fmt"

	v "github.com/go-muse/go-musicxml/validation"
)

// This is a caller-defined synthetic fact profile, not a MusicXML conformance
// profile or a model/source adapter. Both the plan and its selector are explicit.
func ExampleEngine() {
	root := v.DeclarationID{Assembly: v.AssemblyMusicXML, OwnerOccurrence: "example", Occurrence: "root"}
	child := v.DeclarationID{Assembly: v.AssemblyMusicXML, OwnerOccurrence: "example", Occurrence: "child"}
	key := v.RuleKey{ID: "example.required-child", Revision: 1}
	plan, _ := v.NewChildCountPlan([]v.DeclarationID{child}, 1, -1)
	catalog, err := v.NewCatalog([]v.RuleDefinition{{Key: key, Class: v.ClassMandatory, Role: v.RolePredicate, Category: v.CategoryNormative, Targets: []v.Target{v.TargetModel}, Scope: v.ScopeNode, Selector: v.Selector{Declaration: &root}, Predicate: "count", Sources: []v.SourceRef{{IDs: []string{"example-contract"}, URL: "https://example.org/contract", Locator: "required child"}}, Text: v.RuleText{Title: "One child", Explanation: "This example requires one child.", Message: "Child count"}}}, []v.Profile{{ID: "example.model", MusicXMLVersion: "4.0", RegistryCommit: "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c", Target: v.TargetModel, Required: []v.RuleKey{key}, Support: []v.RuleSupport{{Rule: key, State: v.ImplementationAvailable}}, SupportedRoots: []v.QName{{Local: "root"}}, NormativeClosure: "Synthetic example only; no MusicXML profile claim."}}, []v.PlanBinding{{ID: "count", Plan: plan}})
	if err != nil {
		panic(err)
	}
	engine, _ := v.NewEngine(catalog)
	ctx := context.Background()
	session, _ := engine.Begin(ctx, v.RunOptions{DocumentID: "example-document", Profile: "example.model"})
	open := func(id, parent v.NodeID, name string, declaration v.DeclarationID) {
		err := session.OpenNode(ctx, v.NodeHeader{ID: id, Parent: parent, Name: v.QName{Local: name}, Type: v.Evidence[v.ComponentID]{State: v.EvidenceUnknown, Reason: "not-needed"}, Declaration: v.Evidence[v.DeclarationID]{State: v.EvidenceKnown, Value: declaration}, Location: v.Location{Target: v.TargetModel, DocumentID: "example-document", Path: "/" + name}})
		if err != nil {
			panic(err)
		}
	}
	closeNode := func(id v.NodeID) {
		err := session.CloseNode(ctx, v.NodeEnd{ID: id, Scalar: v.Scalar{Lexeme: v.Evidence[string]{State: v.EvidenceAbsent}, Value: v.Evidence[v.Atomic]{State: v.EvidenceAbsent}}, ChildrenComplete: v.Evidence[bool]{State: v.EvidenceKnown, Value: true}, AttributesComplete: v.Evidence[bool]{State: v.EvidenceKnown, Value: true}, Order: v.Evidence[v.OrderKind]{State: v.EvidenceKnown, Value: v.OrderModelFields}})
		if err != nil {
			panic(err)
		}
	}
	open(1, 0, "root", root)
	open(2, 1, "child", child)
	closeNode(2)
	closeNode(1)
	report, err := session.Finish(ctx, v.Completion{InputComplete: true, Syntax: v.SyntaxNotAssessed})
	fmt.Println(report.Satisfied(), err)
	// Output: true <nil>
}
