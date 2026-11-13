package validation

import (
	"context"
	"errors"
	"testing"
)

// The source origin, attribute lexical and DTD ID tests run again under the
// windowed strategy, which keeps a full origin only while its node is open.
func TestSourceProvenanceUnderWindowedStrategy(t *testing.T) {
	previous := setWindowedStrategy(true)
	defer setWindowedStrategy(previous)
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"TestSourceOriginOwnedIngressAndReport", TestSourceOriginOwnedIngressAndReport},
		{"TestSourceOriginMissingSealCannotPass", TestSourceOriginMissingSealCannotPass},
		{"TestSourceOriginOldJSONUnchanged", TestSourceOriginOldJSONUnchanged},
		{"TestSourceOriginSnapshotExplicitlyExportsOwnedOrigins", TestSourceOriginSnapshotExplicitlyExportsOwnedOrigins},
		{"TestSourceOriginDefaultAndEntityAttributeOwnership", TestSourceOriginDefaultAndEntityAttributeOwnership},
		{"TestSourceOriginRejectsUnownedAndContradictoryIngress", TestSourceOriginRejectsUnownedAndContradictoryIngress},
		{"TestSourceOriginProviderAndPhaseGates", TestSourceOriginProviderAndPhaseGates},
		{"TestSourceOriginSealCoverageAndAvailability", TestSourceOriginSealCoverageAndAvailability},
		{"TestSourceOriginPublicationExactBudgetsAndCancellation", TestSourceOriginPublicationExactBudgetsAndCancellation},
		{"TestSourceOriginCatalogSelectionIsSealed", TestSourceOriginCatalogSelectionIsSealed},
		{"TestSourceOriginIngressCancellationAndExactContextFactLimit", TestSourceOriginIngressCancellationAndExactContextFactLimit},
		{"TestSourceOriginPublicationRejectsMismatchedPayload", TestSourceOriginPublicationRejectsMismatchedPayload},
		{"TestSourceOriginUnavailableComponentsRemainIncomplete", TestSourceOriginUnavailableComponentsRemainIncomplete},
		{"TestSourceOriginPhysicalDeclaredAttributeAndEntityValue", TestSourceOriginPhysicalDeclaredAttributeAndEntityValue},
		{"TestSourceOriginAncestryBoundaryAndComponentCauses", TestSourceOriginAncestryBoundaryAndComponentCauses},
		{"TestSourceOriginEventAndScalarBoundaries", TestSourceOriginEventAndScalarBoundaries},
		{"TestSourceOriginSyntheticPositionAndClosedOwnerAreRejected", TestSourceOriginSyntheticPositionAndClosedOwnerAreRejected},
		{"TestSourceOriginTerminalPrefixSurvivesFinish", TestSourceOriginTerminalPrefixSurvivesFinish},
		{"TestSourceOriginRequiresCompleteAttributeEnumeration", TestSourceOriginRequiresCompleteAttributeEnumeration},
		{"TestSourceOriginIngressValidatesPayloadOnce", TestSourceOriginIngressValidatesPayloadOnce},
		{"TestSourceOriginRejectsContradictoryEntityAndDefaultShapes", TestSourceOriginRejectsContradictoryEntityAndDefaultShapes},
		{"TestSourceOriginTypedRetentionAvoidsPerFieldReserves", TestSourceOriginTypedRetentionAvoidsPerFieldReserves},
		{"TestSourceOriginSnapshotTransportKindsAndCompleteness", TestSourceOriginSnapshotTransportKindsAndCompleteness},
		{"TestSourceOriginSnapshotTransportIndependentAdoptions", TestSourceOriginSnapshotTransportIndependentAdoptions},
		{"TestSourceOriginTransportExactTypedRemapping", TestSourceOriginTransportExactTypedRemapping},
		{"TestResourceSnapshotTransportCancellationAfterAdmission", TestResourceSnapshotTransportCancellationAfterAdmission},
		{"TestSourceOriginSnapshotTransportExactOverlapAdmission", TestSourceOriginSnapshotTransportExactOverlapAdmission},
		{"TestSourceOriginSnapshotTransportEveryPostChargeCancellation", TestSourceOriginSnapshotTransportEveryPostChargeCancellation},
		{"TestSourceOriginSnapshotTransportGetterBudgetsAndCancellation", TestSourceOriginSnapshotTransportGetterBudgetsAndCancellation},
		{"TestSourceOriginSnapshotTransportRejectedTransferKeepsOwner", TestSourceOriginSnapshotTransportRejectedTransferKeepsOwner},
		{"TestSourceOriginSnapshotTransportSourceErrorPreservesPrefix", TestSourceOriginSnapshotTransportSourceErrorPreservesPrefix},
		{"TestSourceOriginSnapshotTransportProjectionOwnsSeparateStorage", TestSourceOriginSnapshotTransportProjectionOwnsSeparateStorage},
		{"TestIndependentR1PhysicalEntityAttribute", TestIndependentR1PhysicalEntityAttribute},
		{"TestIndependentR1OriginCanceledBeforeMalformed", TestIndependentR1OriginCanceledBeforeMalformed},
		{"TestSourceDTDIDIngressRejectsForgedOccurrence", TestSourceDTDIDIngressRejectsForgedOccurrence},
		{"TestSourceDTDIDIngressOwnershipSealsAndDuplicateBinding", TestSourceDTDIDIngressOwnershipSealsAndDuplicateBinding},
		{"TestSourceDTDIDSnapshotOwnershipAndAccounting", TestSourceDTDIDSnapshotOwnershipAndAccounting},
		{"TestJoinedIDFirstOwnerAndIncompleteAuthorities", TestJoinedIDFirstOwnerAndIncompleteAuthorities},
		{"TestSourceDTDIDValueAdmissionBeforeComparison", TestSourceDTDIDValueAdmissionBeforeComparison},
		{"TestSourceDTDIDDeclarationDiagnosticNamesExactField", TestSourceDTDIDDeclarationDiagnosticNamesExactField},
		{"TestSourceDTDIDSnapshotCancellation", TestSourceDTDIDSnapshotCancellation},
		{"TestSourceDTDIDSnapshotExactBoundary", TestSourceDTDIDSnapshotExactBoundary},
		{"TestSourceDTDIDSelectionQueryExactActiveAssembly", TestSourceDTDIDSelectionQueryExactActiveAssembly},
		{"TestSourceLocalPresenceCatalogCompatibilityBoundary", TestSourceLocalPresenceCatalogCompatibilityBoundary},
		{"TestSourceLocalPresenceExplicitCompositionExecutes", TestSourceLocalPresenceExplicitCompositionExecutes},
		{"TestSourceContainerMetadataSeparatesPhysicalRaw", TestSourceContainerMetadataSeparatesPhysicalRaw},
		{"TestSourceAuxiliaryHintsAdditivePlanContract", TestSourceAuxiliaryHintsAdditivePlanContract},
		{"TestSourceSoundsMappingRemainsEffective", TestSourceSoundsMappingRemainsEffective},
		{"TestSourceAuxiliaryHintOnlyLexicalElection", TestSourceAuxiliaryHintOnlyLexicalElection},
		{"TestSourceAuxiliaryHintOnlyEffectiveAndPhysicalValues", TestSourceAuxiliaryHintOnlyEffectiveAndPhysicalValues},
		{"TestSourceContainerRawExactAttributeAndCausalEvidence", TestSourceContainerRawExactAttributeAndCausalEvidence},
		{"TestSourceContainerMetadataCancellationAndWorkAdmission", TestSourceContainerMetadataCancellationAndWorkAdmission},
	} {
		t.Run(tc.name, tc.run)
	}
}

func windowedOriginFixture(t *testing.T, limits Budgets, fam ContextFamilies) (*Session, RuleKey) {
	t.Helper()
	base := sourceSchemaEngine(t, XSDCheckScalar)
	profile, _ := base.Profile("source-schema-test")
	profile.CatalogDigest = ""
	var rules []RuleDefinition
	var plans []PlanBinding
	for _, key := range profile.Required {
		r, _ := base.catalog.Rule(key)
		rules = append(rules, r)
		plans = append(plans, PlanBinding{ID: r.Predicate, Plan: base.catalog.plans[r.Predicate]})
	}
	plan, _ := newSourceOriginPlan(AssemblyMusicXML)
	key := RuleKey{"test.source-origin", 1}
	assembly := AssemblyMusicXML
	rules = append(rules, RuleDefinition{Key: key, Targets: []Target{TargetSource}, Scope: ScopeDocument, Category: CategoryNormative, Class: ClassMandatory, Role: RoleContextProvider, Selector: Selector{Assembly: &assembly}, Predicate: "source-origin", Text: RuleText{Title: "Source origin", Explanation: "Retain explicitly supplied source origins.", Message: "Source origin evidence is incomplete."}, Sources: []SourceRef{{IDs: []string{"XML-1.0"}, URL: "https://www.w3.org/TR/xml/", Locator: "3.3.2"}}})
	plans = append(plans, PlanBinding{ID: "source-origin", Plan: plan})
	profile.Required = append(profile.Required, key)
	profile.Support = append(profile.Support, RuleSupport{Rule: key, State: ImplementationAvailable})
	c, err := NewCatalog(rules, []Profile{profile}, plans)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := NewEngine(c)
	s, err := e.Begin(context.Background(), RunOptions{Context: fam, DocumentID: "source", Profile: profile.ID, Budgets: limits})
	if err != nil {
		t.Fatal(err)
	}
	return s, key
}

// windowedOriginInputs are an element origin and an attribute origin of the
// node the fixture opens, with the attribute's specifiedness.
func windowedOriginInputs(t *testing.T, s *Session, specified Evidence[bool]) (SourceOriginContextResult, SourceOriginContextResult) {
	t.Helper()
	h := schemaPendingHeader()
	h.Location = sourceOriginPhysical().Physical.Value
	if err := s.OpenNode(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	a := sourceOriginAttribute(QName{Local: "x"}, h.Location)
	if err := s.Attribute(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	attribute := sourceOriginPhysical()
	attribute.Attribute = &a.Name
	attribute.Physical.Value = a.Location
	attribute.Specified = specified
	return sourceOriginPhysical(), attribute
}

// Under the windowed strategy, at every byte budget across an element and
// an attribute origin, with and without the projection and with plain and
// overflowing specifiedness: a refused origin leaves no count, no open
// record, no specifiedness and no charge behind. When both are admitted,
// closing their node gives back exactly the open record, its attribute entry
// and the private copies the origins charged, and the attribute keeps its
// specifiedness.
func TestWindowedSourceOriginCharges(t *testing.T) {
	defer setWindowedStrategy(setWindowedStrategy(true))
	ctx := context.Background()
	for _, families := range []ContextFamilies{0, ContextFamilyAll} {
		for _, specified := range []Evidence[bool]{stateKnown(true), {State: EvidenceKnown, Value: true, Reason: "why"}} {
			probe, key := windowedOriginFixture(t, Budgets{MaxBufferedBytes: 1 << 40}, families)
			element, attribute := windowedOriginInputs(t, probe, specified)
			start := probe.retained
			if err := probe.sourceOrigin(ctx, key, element); err != nil {
				t.Fatal(err)
			}
			if err := probe.sourceOrigin(ctx, key, attribute); err != nil {
				t.Fatal(err)
			}
			end := probe.retained
			want := sourceOriginOpenBytes + sourceOriginIndexEntryBytes
			if families == 0 {
				for _, in := range []SourceOriginContextResult{element, attribute} {
					size, err := measureSourceOriginContext(ctx, probe, &in)
					if err != nil {
						t.Fatal(err)
					}
					want += size
				}
			}
			before := probe.retained
			probe.closeSourceOrigins(probe.stack[len(probe.stack)-1])
			if released := before - probe.retained; released != want || len(probe.sourceOriginOpen) != 0 {
				t.Errorf("families %b, specified %+v: closing released %d, want %d", families, specified, released, want)
			}
			if got, ok := probe.sourceOriginSpecifiedOf(sourceOriginSubject{Node: 1, Attribute: *attribute.Attribute}); !ok || got.State != specified.State || got.Value != specified.Value || got.Reason != specified.Reason {
				t.Errorf("families %b: closed specifiedness %+v, want %+v", families, got, specified)
			}
			probe.cleanup()
			for limit := start; limit < end; limit++ {
				s, key := windowedOriginFixture(t, Budgets{MaxBufferedBytes: limit}, families)
				element, attribute := windowedOriginInputs(t, s, specified)
				used, interned := s.retained, internCharge(s)
				var count uint64
				open := 0
				err := s.sourceOrigin(ctx, key, element)
				if err == nil {
					used, interned = s.retained, internCharge(s)
					count, open = 1, 1
					if err = s.sourceOrigin(ctx, key, attribute); err == nil {
						t.Fatalf("limit %d: both origins admitted", limit)
					}
				}
				if !errors.Is(err, ErrResourceLimit) || s.sourceOriginCount != count || len(s.sourceOriginOpen) != open || len(s.sourceOriginSpecified) != 0 || len(s.sourceOriginSpecifiedOverflow) != 0 || s.retained != used+internCharge(s)-interned {
					t.Fatalf("families %b, limit %d: %v count %d open %d specified %d overflow %d retained %d/%d", families, limit, err, s.sourceOriginCount, len(s.sourceOriginOpen), len(s.sourceOriginSpecified), len(s.sourceOriginSpecifiedOverflow), s.retained, used)
				}
				s.cleanup()
			}
		}
	}
}
