// Minimal Unicode13-derived representation policy.
// Unicode copyright and permission: ../testdata/dictionaries/ipa-representation/
// LICENSE-UNICODE-V3.txt (preserved1991–2026 snapshot, not a2020 license text).
// See NOTICES.txt and source-ledger.md there for separate adapted-data notices.
package validation

import "context"

const ipaRepresentationPolicySHA256 = "abf354d8847b1eb4233ac2d7e835d025ebd9d99dee903737aa0e921004a65bd2"

var ipaRepresentationCCCValues = [...]struct {
	scalar rune
	ccc    uint8
}{
	{0x0300, 230},
	{0x0301, 230},
	{0x0302, 230},
	{0x0303, 230},
	{0x0304, 230},
	{0x0306, 230},
	{0x0308, 230},
	{0x030A, 230},
	{0x030B, 230},
	{0x030C, 230},
	{0x030D, 230},
	{0x030F, 230},
	{0x0311, 230},
	{0x0315, 232},
	{0x0318, 220},
	{0x0319, 220},
	{0x031A, 232},
	{0x031C, 220},
	{0x031D, 220},
	{0x031E, 220},
	{0x031F, 220},
	{0x0320, 220},
	{0x0322, 202},
	{0x0324, 220},
	{0x0325, 220},
	{0x0327, 202},
	{0x0329, 220},
	{0x032A, 220},
	{0x032C, 220},
	{0x032F, 220},
	{0x0330, 220},
	{0x0334, 1},
	{0x0339, 220},
	{0x033A, 220},
	{0x033B, 220},
	{0x033C, 220},
	{0x033D, 230},
	{0x0346, 230},
	{0x0351, 230},
	{0x0357, 230},
	{0x035C, 233},
	{0x0361, 234},
	{0x1DC4, 230},
	{0x1DC5, 230},
	{0x1DC6, 230},
	{0x1DC7, 230},
	{0x1DC8, 230},
	{0x1DC9, 230},
	{0x1DF5, 230},
	{0xFE20, 230},
	{0xFE21, 230},
	{0xFE27, 220},
	{0xFE28, 220},
}

func ipaRepresentationCCC(ctx context.Context, a *Account, cp rune) (uint8, error) {
	lo, hi := 0, len(ipaRepresentationCCCValues)
	for lo < hi {
		if err := dictionaryStep(ctx, a, 1); err != nil {
			return 0, err
		}
		mid := lo + (hi-lo)/2
		row := ipaRepresentationCCCValues[mid]
		if row.scalar < cp {
			lo = mid + 1
		} else if row.scalar > cp {
			hi = mid
		} else {
			return row.ccc, nil
		}
	}
	return 0, nil
}
