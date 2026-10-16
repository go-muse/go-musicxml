package finalreconciliation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Deliberate policy changes require a reviewed ledger edit and corresponding
// pin update. These hashes freeze evidence classifications and residual text;
// they do not establish runtime conformance or interpret arbitrary prose.
const reviewedRequirementPolicySHA256 = "d90dc8cab1ce1ff7c46eae6d481e9cfb723b9a855065176997ca470363ed67ce"
const reviewedCasePolicySHA256 = "d7d0366dc7a58e878416b0108b69018a56474d34c4200115eeeb4f518cfd3f30"
const reviewedAxisPolicySHA256 = "1be0a4b736277adda9eb9a366897b9f1dc340dccd51283932b6e152d7fa460b7"

var preservedSourcePins = map[string]string{
	"docs/validation/plan/implementation-plan.json":               "d8dc0030f58f8f84647d099ef441adab436af5167ebaa5bf86fab4fb959ba044",
	"docs/validation/plan/open-questions.json":                    "d154bc903a294f91ff2a0c658b01a70918aa1cc9b79ed8aec31b250b7c34774f",
	"docs/validation/plan/context-dispositions.json":              "f5402c473dad21fd747013d60d746e1fad7e6ad323959cd9a1773071cf151272",
	"docs/validation/plan/prose-contracts.json":                   "f623fa5c731876b28e4fdcc99d0750e665e071bc5721848fa2739dfbe5d91b2c",
	"docs/validation/plan/xsd-contracts.json":                     "5b954dc5f5c0e3bbac44c7e4743e251e1230fc5ff1e7a6ea9bb0619032c2cf29",
	"docs/validation/plan/xsd-test-contracts.json":                "b568e5f21e7ebbd40131dcf6d4391b475f2e8eae97ce6bcfd0d0afd6dc15e94e",
	"docs/validation/plan/external-snapshot.json":                 "8d87cbbd2e8dd3a1f87e9d3adcb32fbb32650edb326aa45244a9768606d1dbde",
	"docs/validation/evidence/context-reviewed-translations.json": "e2626410faa79bce51535e230d18e754c9fb8e10df1f08bc89aaa6a0e1ad4168",
}

func checkReviewedPolicy(kind string, value any, want string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("reviewed %s policy changed: got %s", kind, got)
	}
	return nil
}
