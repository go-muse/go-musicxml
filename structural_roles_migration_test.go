package musicxml

import (
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Revision 2 of MX40-PACKAGE-score-before-parts decides the score and part
// roles from the MusicXML link semantics when no role provider decides them.
// That changed the identity of every package catalog and package profile.
// Earlier captures migrate through this one step, applied after the
// validation-lines step.

// structuralRolesCatalogs maps each package catalog digest, as the earlier
// migrations leave it, to its current digest.
var structuralRolesCatalogs = map[string]string{
	"a24c8a02cfed1292280dee4ea745a406b9c9e9961052b4419337e021d82d39f4": "6f86484d27217e2f5160ffc4dd42e7cee3b20e9e3fb7194ce0cf78c1e668a4ec", // ordinary package
	"de2d27f3919711316c9a484ffe8f4711bad096327d40bcaa1e727644ac7f2f8d": "3f3907dc195ccc438f8762fa1b90c485cdce1722b52afebabe558afd73ef8b6e", // dictionary package
	"a4b321cd76cba82aebdddf4ae34d74ed7d6e1486a9bf220638630d5d069f75fe": "f141f3243747620aaaa974954d19215c506a39a66260820aaa59fa1d79964fad", // DTD package
	"180ad038e9a989fcc9b35918732acb04ebb79c7610d232ebfe60baf05499197f": "394776602607f246bc899a562190c17c67a7c84a85656b26cbc92bd8157e72be", // semantic package
}

// structuralRolesProfileHashes maps the complete Profile JSON hash of each
// package profile, as the earlier migrations leave it, to its current hash.
var structuralRolesProfileHashes = map[validation.ProfileID][2]string{
	ProfileMXLModel:                             {"9ce6b701af9158e6a405b545ddf22fd42e45d797cfd4fe22a39477a7d9abfd61", "f296fb28f3dca50cf32b1a31c2947b46f76fbd66f857c6a3d685411b24355038"},
	ProfileMXLModelDictionaries:                 {"e879aa4d0cb388f7ca767959360037c798e4f7dd87e20343bb0891bc6b4204da", "683853361c25b1674598dbc9b5528ec03a4b00b9fd9fa14eb5af95c7d1dce1ca"},
	ProfileMXLModelInternalDTDDictionaries:      {"ba21214cb61740cab9a9d47424a61a05232f92189e9df79f16f098f02f6d0852", "489acd007e49fa71e736262d8140f4cc7d2d105efc9a6a3359252ce18ef022a3"},
	ProfileMXLModelSemantics:                    {"ff5c2e00a9fbee5544ffd29a773672aae189ad0baf70e1c76e0819b058a3fc8c", "d397e78d7934e5e38d949bb3da9b3fab4a21dd969dcd05bb77594ecf9258ff66"},
	ProfileMXLSource:                            {"6a34a664baf9f0e768ded55af8053ef0d4bf9a4cf5c5806cd135f375d482797b", "e6fefcb79ad9ae28f1e2ee636326ac5b3f085ddae949d0ac089861877a522cf5"},
	ProfileMXLSourceDictionaries:                {"708c34a08d3ecc3b42e68cca37209de8908584e339511aee1b3e7fc0c86a09a8", "edeb81267367baeeaaa01c34b352092f291e40d6bddc2ea6b33af28dbd94b7de"},
	ProfileMXLSourceInternalDTDDictionaries:     {"5574064a48fb1fa4632747c86b3e2e457b5fe0537d0b2b3c8dfa744f45bbf678", "15269384c3c6f9da5ad1e6e0ae4b001484a6fc45e779c2849c983f0439ef75f1"},
	ProfileMXLSourceLine:                        {"ef9df24aece789802884c65bc20820c45561172d92ecdd4f2da8f0f25567a28b", "b10217ff6d9a8aee889d7543f323dcc5d95696a7712727e2110b58c2ab5e061b"},
	ProfileMXLSourceSemantics:                   {"3b25ec97b7e7c372bbfd19d769b8eb7036d5508cd42cb1c106e943b971aee099", "9894eb1d013bf204d0fc650578f806d2b4924b637db51fac638947bf8c3619cc"},
	ProfileResourceGraph:                        {"7b9c47b64b7be5d3aa8d344d9e5c46d71988cd4b6d0b8ef030927b06ca641b1e", "889a7b1cb7a7d2b0b7e3dae6ec834a157c7430d10eb34f6c2149bbc5875fb362"},
	ProfileResourceGraphDictionaries:            {"3c1479fac8375036587d1342ae14fae853aadb5d8b1dc9e0305f5d68c9895070", "a2b4c37d0ed8c52ba3e57d33f02966267c7f98cdd52087eff6dde4dd123f7a41"},
	ProfileResourceGraphInternalDTDDictionaries: {"e7703d41743be9ee844261d1e64f393b521707382693bea8e420f939ec6d5d63", "d3946f2ecd518923ce7d53be2e49093cc043bcbb1a7b53519b8768d722560e18"},
	ProfileResourceGraphSemantics:               {"68f66f808c799fbd298451e82635727dda043b1f985263261a22d1e9b450b521", "d2545f2fb948d6f0ff501fe627aa81b8a79127a3b52ece3563bc133f04fb79e5"},
}

// migrateStructuralRolesCatalog returns the digest a catalog captured before
// the structural roles has now; other catalogs keep theirs.
func migrateStructuralRolesCatalog(before string) string {
	if after, ok := structuralRolesCatalogs[before]; ok {
		return after
	}
	return before
}

// migrateStructuralRolesProfileHash returns the complete Profile JSON hash a
// profile captured before the structural roles has now; other profiles keep
// theirs.
func migrateStructuralRolesProfileHash(t *testing.T, id validation.ProfileID, before string) string {
	t.Helper()
	pair, ok := structuralRolesProfileHashes[id]
	if !ok {
		return before
	}
	if before != pair[0] {
		t.Fatalf("unexpected pre-structural-roles profile %s: %s", id, before)
	}
	return pair[1]
}
