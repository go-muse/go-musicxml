package validation

import (
	"context"
	"testing"
)

func selectionWitnessSnapshot(t *testing.T, kind planKind, support ImplementationState) *ResourceSnapshot {
	t.Helper()
	s := snapshotTestSession(t, "7")
	if kind != "" {
		key := RuleKey{ID: "selection-witness", Revision: 1}
		assembly := contextPlanAssembly(kind)
		s.catalog.orders[s.profile.ID] = append(s.catalog.orders[s.profile.ID], key)
		s.catalog.rules[key] = RuleDefinition{Key: key, Scope: ScopeDocument, Selector: Selector{Assembly: &assembly}, Predicate: "selection-witness", Category: CategoryNormative, Class: ClassMandatory, Role: RoleContextProvider}
		// Run the normal finalizer on a link-free miniature schema. Empty role
		// bindings let the available plan perform its traversal without emitting
		// rows; unsupported and missing support take evaluate's early return.
		s.catalog.plans["selection-witness"] = Plan{kind: kind, context: &contextPlan{Schema: s.schema.active, Bindings: &contextBindings{assembly: assembly}}}
		if support != "" {
			s.catalog.supports[s.profile.ID][key] = RuleSupport{Rule: key, State: support, Reason: "selection-witness-unsupported"}
		}
	}
	_, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || snapshot == nil {
		t.Fatalf("snapshot=%v err=%v", snapshot, err)
	}
	t.Cleanup(func() { _ = snapshot.Close() })
	return snapshot
}

func TestResourceSnapshotLinkProjectionSelectionWitness(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     planKind
		support  ImplementationState
		selected bool
	}{
		{"schema-only", "", "", false},
		{"hints-only", planContextMusicXMLHints, ImplementationAvailable, false},
		{"other-assembly-links", planContextOpusLinks, ImplementationAvailable, false},
		{"active-assembly-links", planContextMusicXMLLinks, ImplementationAvailable, true},
		{"unsupported-links", planContextMusicXMLLinks, ImplementationUnsupported, false},
		{"missing-support", planContextMusicXMLLinks, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := selectionWitnessSnapshot(t, tc.kind, tc.support)
			if snapshot.Header().LinkProjectionSelected != tc.selected {
				t.Errorf("selected=%t, want %t", snapshot.Header().LinkProjectionSelected, tc.selected)
			}
			if tc.kind == planContextMusicXMLLinks && snapshot.ContextCount() != 0 {
				t.Error("link-free fixture unexpectedly projected rows")
			}
		})
	}
}
