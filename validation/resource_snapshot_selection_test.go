package validation

import (
	"context"
	"reflect"
	"testing"
)

func TestResourceSnapshotLinkProjectionSelectionWitness(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     planKind
		selected bool
	}{
		{"schema-only", "", false},
		{"hints-only", planContextMusicXMLHints, false},
		{"other-assembly-links", planContextOpusLinks, false},
		{"active-assembly-links", planContextMusicXMLLinks, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := snapshotTestSession(t, "7")
			if tc.kind != "" {
				// Install only a selected-plan witness; no projected row is fabricated.
				key := RuleKey{ID: "selection-witness", Revision: 1}
				s.catalog.orders[s.profile.ID] = append(s.catalog.orders[s.profile.ID], key)
				s.catalog.rules[key] = RuleDefinition{Key: key, Predicate: "selection-witness"}
				s.catalog.plans["selection-witness"] = Plan{kind: tc.kind}
			}
			_, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil || snapshot == nil {
				t.Fatalf("snapshot=%v err=%v", snapshot, err)
			}
			defer snapshot.Close()
			field := reflect.ValueOf(snapshot.Header()).FieldByName("LinkProjectionSelected")
			if !field.IsValid() || field.Kind() != reflect.Bool {
				t.Fatal("snapshot lacks a typed selected-link-plan witness")
			}
			if field.Bool() != tc.selected {
				t.Fatalf("selected=%t, want %t", field.Bool(), tc.selected)
			}
		})
	}
}
