package validation

import (
	"context"
	"errors"
	"testing"
)

// Use a local capability interface so the test-first regression compiles before
// the read-only Session query exists.
type sourceDTDIDSelectionQuery interface{ SourceDTDIDsSelected() bool }

func TestSourceDTDIDSelectionQueryExactActiveAssembly(t *testing.T) {
	var missing *Session
	query, ok := any(missing).(sourceDTDIDSelectionQuery)
	if !ok {
		t.Fatal("session has no exact active-assembly DTD-ID capability query")
	}
	if query.SourceDTDIDsSelected() {
		t.Fatal("nil session selected input")
	}
	_, unopened, _ := sourceOriginFixture(t, Budgets{})
	defer unopened.cleanup()
	if any(unopened).(sourceDTDIDSelectionQuery).SourceDTDIDsSelected() {
		t.Fatal("unopened session selected input")
	}
	s, _ := dtdIDSession(t)
	defer s.cleanup()
	query = any(s).(sourceDTDIDSelectionQuery)
	work, _ := s.account.Usage(BudgetRuleSteps)
	retained, _ := s.account.Usage(BudgetBufferedBytes)
	if !query.SourceDTDIDsSelected() {
		t.Fatal("selected MusicXML assembly lost DTD-ID capability")
	}
	if knownComplete(s.dtdIDs.Complete) {
		t.Fatal("selection query must not assert occurrence completion")
	}
	// An immutable compiled profile can elect another assembly without electing
	// this active assembly. No domain ingress is allowed by the broad query.
	s.catalog.fragmentDTDs[s.profile.ID] = map[Assembly]bool{AssemblyOpus: true}
	if !(&Engine{catalog: s.catalog}).SourceDTDIDsSelected(s.profile.ID) || query.SourceDTDIDsSelected() {
		t.Fatal("profile-wide and active-assembly selection were conflated")
	}
	s.catalog.fragmentDTDs[s.profile.ID][AssemblyMusicXML] = true
	s.profile.Target = TargetModel
	if query.SourceDTDIDsSelected() {
		t.Fatal("current model acquired source DTD authority")
	}
	s.profile.Target = TargetSource
	s.catalog.sourceDeclarations[s.profile.ID][AssemblyMusicXML] = 0
	if query.SourceDTDIDsSelected() {
		t.Fatal("missing declaration capability acquired source DTD authority")
	}
	if got, _ := s.account.Usage(BudgetRuleSteps); got != work {
		t.Fatal("read-only selection query charged work")
	}
	if got, _ := s.account.Usage(BudgetBufferedBytes); got != retained {
		t.Fatal("read-only selection query retained data")
	}
	if err := s.SourceDTDIDDomain(context.Background(), SourceDTDIDDomain{"source", stateKnown(true)}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("unselected ingress gate was weakened: %v", err)
	}
}
