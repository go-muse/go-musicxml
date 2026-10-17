package finalreconciliation

import "testing"

// These are every ledger route that reaches either mandatory assessed loader,
// including routes changed through strictSourceOptions rather than their bodies.
var reviewedAssessedLoadingRoutes = map[string]string{
	"TestSourceAssessmentSyntaxUnderSkippedContent":               "baseline",
	"TestSourceTruncatedMarkupPublic":                             "baseline",
	"TestDecodeAssessedSourceAndConversionDomains":                "baseline",
	"TestDecodeAssessedDrainsAfterOverflowAndPreservesTailCauses": "baseline",
	"TestDecodeAssessedRetainsOverflowDuringInterruptedDrain":     "baseline",
	"TestSourceAssessmentPreflightDoesNotRead":                    "baseline",
	"TestDecodeAssessedPermissiveAndStrictSourceAreSeparate":      "baseline",
	"TestBoundedLocalStrictAccordionIntegration":                  "baseline",
	"TestDecodeAssessedObservesFilteredSource":                    "baseline",
	"TestDecodeAssessedTruncatedSummaryPreservesJoinedCauses":     "baseline",
	"TestDecodeAssessedUsesTighterDepthLimit":                     "baseline",
	"TestIPARepresentationPublicSourceModelAndAssessedReader":     "baseline",
	"TestAssessedDecodeSkippedSubtreeXMLContracts":                "pr42-added",
	"TestAssessedDecodeSkippedDistinctAttributesRemainWellFormed": "pr42-added",
}

func TestFinalReconciliationAssessedLoadingRouteLayers(t *testing.T) {
	_, ledger := loadLedger(t)
	for id, prior := range reviewedAssessedLoadingRoutes {
		t.Run(id, func(t *testing.T) {
			route := rowWith(ledger, "cases", "id", id)
			if route["evidence_layer"] != "pr46-assessed-loading" || route["prior_evidence_layer"] != prior {
				t.Fatalf("mandatory-loader route has evidence layer %v, prior %v; want pr46-assessed-loading, %s", route["evidence_layer"], route["prior_evidence_layer"], prior)
			}
		})
	}
}

func TestFinalReconciliationRejectsEveryMigratedRouteAsPriorEvidence(t *testing.T) {
	for id, prior := range reviewedAssessedLoadingRoutes {
		t.Run(id, func(t *testing.T) {
			root, ledger := loadLedger(t)
			route := rowWith(ledger, "cases", "id", id)
			route["evidence_layer"] = prior
			rejectLedger(t, root, ledger, "current assessed-loading route")
		})
	}
}

func TestFinalReconciliationRejectsMissingPriorRouteLayer(t *testing.T) {
	root, ledger := loadLedger(t)
	route := rowWith(ledger, "cases", "id", "TestIPARepresentationPublicSourceModelAndAssessedReader")
	delete(route, "prior_evidence_layer")
	rejectLedger(t, root, ledger, "current assessed-loading route")
}
