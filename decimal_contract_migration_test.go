package musicxml

import (
	"strings"
	"testing"
)

// Model decimals changed from the exact binary value of each float64 to its
// shortest decimal. The model profiles name this contract in their closure,
// so their catalog identity changed. This is the only migration permitted for
// historical model-catalog snapshots.
const (
	decimalContractCatalogBefore = "6f1950afff8e1d5a3054bd08edab85af182a7b50e7baca92b2547220dcd08881"
	decimalContractCatalogAfter  = "d0ab66b597650fcce62ff00b51b26d137a9f2d8c6a6bbfac1216b3d3505686d4"
	decimalContractTextBefore    = "Finite float64 decimal fields mean their exact current binary rational value."
	decimalContractTextAfter     = "Finite float64 decimal fields mean the shortest decimal that reads back to the same float64."
)

func migrateFrozenDecimalContractReport(t *testing.T, value any) {
	t.Helper()
	profile := value.(map[string]any)["Profile"].(map[string]any)
	if profile["CatalogDigest"] != decimalContractCatalogBefore {
		return
	}
	profile["CatalogDigest"] = decimalContractCatalogAfter
	if closure := profile["NormativeClosure"].(string); strings.Contains(closure, decimalContractTextBefore) {
		profile["NormativeClosure"] = strings.Replace(closure, decimalContractTextBefore, decimalContractTextAfter, 1)
	}
}
