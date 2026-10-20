package musicxml

import (
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// The validation lines split strict loading into line 2, ProfileSourceLine,
// and line 3, ProfileModelSemantics. The representability rule moved from the
// diagnostic source profiles into ProfileSourceLine, the model composition
// stopped requiring the always-empty schema hints, and the semantic package
// catalog gained ProfileMXLSourceLine. That changed the identity of every
// catalog holding one of those profiles. Earlier captures migrate through
// this one step, applied after the decimal-contract step.

// validationLinesCatalogs maps each catalog digest, as the earlier migrations
// leave it, to its current digest.
var validationLinesCatalogs = map[string]string{
	"ac97170859bc257542f0010811fb69ec55011654a50d7813afdbb7f1219684e1": "6803972039a27fd9e3ffa1afe0a541b5b0f30d2a3d5b57b737637289f8c0e400", // model semantics
	"255faf825af13849ff431a42fca3f4ec00d5c99355b19a9586309a772577a26c": "a4b321cd76cba82aebdddf4ae34d74ed7d6e1486a9bf220638630d5d069f75fe", // DTD package
	"0ab2e46211e80f8a7a569ae48f7280220bb307d8494e9c389eca338e46d4cd7d": "180ad038e9a989fcc9b35918732acb04ebb79c7610d232ebfe60baf05499197f", // semantic package
	"110b5e123d135680ad95c927c67a0c9642bb2d6ca43f5748ffd7bf118a83eff4": "af3d00c0ba64794985e4027c77536ae9b4127ab9b27da641893772975369a104", // internal-DTD source composition
	"de0f9f61c4ea183865bf80dd4377afce3aa22b0a7be2e31fa184c5ca54621471": "e7e6d4195bb87a483d410c8a4f9cbd43eff9a1d488d93ffa02d8102b50211a63", // source semantics
}

// validationLinesProfileHashes maps the complete Profile JSON hash of each
// changed profile, as the earlier migrations leave it, to its current hash.
var validationLinesProfileHashes = map[validation.ProfileID][2]string{
	ProfileModelSemantics.ID():                       {"df35e3d58b00457242ea94c671597a44baa16739c75e469a97cd06f6d3107260", "554d4e2dc84285990d66583f0e8344cc678f5f2595485fb19be531567790c4b4"},
	ProfileMXLModelInternalDTDDictionaries.ID():      {"b003724bedbade7e3b1d398fc287071f43526e7fe7bae65b67c69963f4a93503", "ba21214cb61740cab9a9d47424a61a05232f92189e9df79f16f098f02f6d0852"},
	ProfileMXLModelSemantics.ID():                    {"2410b7dad8b214651f5fc03b5efd156bc40683c90721ea7ac68b5c7ea20c5aba", "ff5c2e00a9fbee5544ffd29a773672aae189ad0baf70e1c76e0819b058a3fc8c"},
	ProfileMXLSourceInternalDTDDictionaries.ID():     {"d39f531c9424456e4172b4aef5fd9c0ec06bc3ee78fa9f4939e070cb7e21f6ce", "5574064a48fb1fa4632747c86b3e2e457b5fe0537d0b2b3c8dfa744f45bbf678"},
	ProfileMXLSourceSemantics.ID():                   {"7887d5ef7e645524909bc703bbf639c812d583af521fae280f3067caa4249fde", "3b25ec97b7e7c372bbfd19d769b8eb7036d5508cd42cb1c106e943b971aee099"},
	ProfileResourceGraphInternalDTDDictionaries.ID(): {"06c155e0a106a8c1fe928abc3919873093fc7be10b30ba380707e70c8e4cf887", "e7703d41743be9ee844261d1e64f393b521707382693bea8e420f939ec6d5d63"},
	ProfileResourceGraphSemantics.ID():               {"4ef1e127c2c9d958562903b5b7f7dd893abf9201f6772af0a013a87afa929307", "68f66f808c799fbd298451e82635727dda043b1f985263261a22d1e9b450b521"},
	ProfileSourceInternalDTDLinks.ID():               {"45bf20217d5b7349f0cabc289855026eeb610f5b81ecaf165e731ccef6a6862a", "07835100bef7210bf6898756fc2343a38acf57a15442f1537b387925131df789"},
	ProfileSourceInternalDTDDictionaries.ID():        {"1010c4222bb19b422e6714ae5105d23d976d66c2b88d53bf502bdc390e3a25f0", "2f6be241e9d35b466d2b230329e5a8f16c76557e2708951da93356146b585b1c"},
	ProfileSourceInternalDTDLocal.ID():               {"495ed5614e4b3ed35157cc5583c0ad67f00a17c99cf514d0d960e52170ed0f35", "3005295365bc5e0b572b333d43490cb33c8fdce0e5007532bce5b4fe0dbd0ea2"},
	ProfileSourceSemantics.ID():                      {"c2dc28f67a5611a6818fe952722388fcbfd4a5d335145c88d605d6a9a209c2a2", "c15a9728c81ff126911ab99d68d776c28a3d368fd89dbbe61163802654c8c475"},
}

// migrateValidationLinesCatalog returns the digest a catalog captured before
// the validation lines has now; other catalogs keep theirs.
func migrateValidationLinesCatalog(before string) string {
	if after, ok := validationLinesCatalogs[before]; ok {
		return after
	}
	return before
}

// migrateValidationLinesProfileHash returns the complete Profile JSON hash a
// profile captured before the validation lines has now; other profiles keep
// theirs.
func migrateValidationLinesProfileHash(t *testing.T, id validation.ProfileID, before string) string {
	t.Helper()
	pair, ok := validationLinesProfileHashes[id]
	if !ok {
		return before
	}
	if before != pair[0] {
		t.Fatalf("unexpected pre-validation-lines profile %s: %s", id, before)
	}
	return pair[1]
}
