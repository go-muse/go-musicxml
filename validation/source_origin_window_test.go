package validation

import "testing"

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
