package musicxml

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Model decimals changed from the exact binary value of each float64 to its
// shortest decimal. Every model profile names this contract in its closure,
// so the identity of each catalog that holds a model profile changed, and the
// source profiles of those catalogs changed catalog digest with it. This is
// the only migration permitted for historical snapshots of those catalogs.
const (
	decimalContractCatalogBefore = "6f1950afff8e1d5a3054bd08edab85af182a7b50e7baca92b2547220dcd08881"
	decimalContractCatalogAfter  = "d0ab66b597650fcce62ff00b51b26d137a9f2d8c6a6bbfac1216b3d3505686d4"
	decimalContractTextBefore    = "Finite float64 decimal fields mean their exact current binary rational value."
	decimalContractTextAfter     = modelDecimalContract
)

// decimalContractCatalogs maps each catalog digest the contract changed, as
// the earlier migrations leave it, to its current digest: the base model,
// timeline, links, dictionaries and model-semantics catalogs, the context
// catalog after its figure-number migration, and the four package catalogs,
// whose intrinsic model bindings carry the new model identities.
var decimalContractCatalogs = map[string]string{
	decimalContractCatalogBefore:                                       decimalContractCatalogAfter,
	"cf62be51ed16c6a08609a30d78ae513e22f4f3dd3398ce4213251dd80ac043ad": "ba3068de73e238b43a8cf3ee43937f276f2c3b198d896a3eaf590627f4222677", // timeline
	"c3e4d66f82ce776a6f8cfdd62295c124e352bc2ade1b0218ef70c8837c7f1506": "bd5b2d15acf69fac926093567e8f7fcd6cb3e85638b9d8f7665e76a39f562c33", // context
	"5fb200a688a054add8a551d288486d2724ff7d1530b42a3e0fbed40f3df46cb3": "4988798850674ee92c815d98adf5ef81c0a06bb99c565ce6be63bb88c071af3b", // links
	"d8199be864c2baa081aca5887ca0ac188052ba22f6a690ea9bb3b436dc054c57": "3a96efabf9a7d72b639141fef20f285e6981b2de90c0e25e964d9167fe9d554f", // dictionaries
	"e85bf8df6094ca313a4f36523eafc6bad2cd4140bfce933b9c3fe026d70bd8f9": "ac97170859bc257542f0010811fb69ec55011654a50d7813afdbb7f1219684e1", // model semantics
	"bcc5d047e4de9f02a61cfdd86f57297f9655006ea0fb6f04ef38628d05dcc28f": "a24c8a02cfed1292280dee4ea745a406b9c9e9961052b4419337e021d82d39f4", // ordinary package
	"bbb60bcd61359b41fda394d95e29d93c42c8c0526b101e98f375b698204d6658": "de2d27f3919711316c9a484ffe8f4711bad096327d40bcaa1e727644ac7f2f8d", // dictionary package
	"948b7a2d4d16e42dae71c648bf566012e4aeb39591f6093eb196259b275c5dbb": "255faf825af13849ff431a42fca3f4ec00d5c99355b19a9586309a772577a26c", // DTD package
	"94392cf23145a815149b494559174e9c3524d01d1cf13fe0a55c0ebb4b0520aa": "0ab2e46211e80f8a7a569ae48f7280220bb307d8494e9c389eca338e46d4cd7d", // semantic package
}

// decimalContractProfileHashes maps the complete Profile JSON hash of each
// profile the contract changed, as the earlier migrations leave it, to its
// current hash. Model profiles name the contract; the others changed catalog
// digest with a model profile of their catalog or an intrinsic binding.
var decimalContractProfileHashes = map[validation.ProfileID][2]string{
	ProfileSourceLinks.ID():                          {"ff0ec0dd30142325e24d84c36c75bad9b2034092d0d11089c1b6a692f2df9b3d", "3386acc792320b6b6bf142fb07e5ae04f0f037adfb1f92d373d9e7a5f708a63f"},
	ProfileSourceTimeline.ID():                       {"701abcfe0421aa62a1d14bcdcef5a1f0ff01a568b68094aff43d723ce4777cc5", "347e388915991e709d0eca826564cf8784415139eb4db165523c8bea02343d05"},
	ProfileSourceContext.ID():                        {"43a78f538defd0fbbc4e5331eecf41688204f12742a5900e49672128cd320155", "deef89259ebd1347c61320a67750144c37e73f58d85d6e2455e10a39e3df3b63"},
	ProfileSourceDictionaries.ID():                   {"7713fd75714d24bcb49ac71dffc3e9ec9e8ae642cccffcb310a2ca211bfd5fc6", "b105b6914c4916b818d296463d21b68373e3f4d5c2dd566d962bfebf27c5f30c"},
	ProfileSourceSoundsDictionaries.ID():             {"94a540d749c6bd05a10c2990efcfcbd43e388044ebc8a6b381cd684e243d5962", "b8140d1fe022f3f2652e63307bcbd5062d35fb2f84e89128b021350a6769aa66"},
	ProfileModelLinks.ID():                           {"dcb0c770a09165e69c95dc65244c0a863ed27f605eb255c641c29eaa54d3e9a5", "65eed7fcf28bd59b43db5abb14208b29f144f115a47e88a539f6b946c3a02aa3"},
	ProfileModelTimeline.ID():                        {"38c486cdfc6d23be969a3f5dc6f2f8666452d6745d7627547ee4d74966924831", "c76bac7529900c96971982a4fb4257527aabeff317ba2ba36da5845da02c124c"},
	ProfileModelContext.ID():                         {"ea6744501e0555255420471e8b8d5a0e4fb9371d915332777bfffdac77fd2511", "26e971cb0e87faad189db7e53e3a8740fda81ff8537e33cb41389e8027b3e8a0"},
	ProfileModelDictionaries.ID():                    {"8bddd6162b1c8aefcecb0514fa35dcc42da251008917e335763a2ac02f2dc16b", "5e414ef85171beb5b70f849141123bbd1c08613e225056ae9eba097ccc0e1729"},
	ProfileModelSemantics.ID():                       {"3ec86e6676086552bdf57492671eee6f7ec2101d805d44aac960116174782ab4", "df35e3d58b00457242ea94c671597a44baa16739c75e469a97cd06f6d3107260"},
	ProfileResourceGraph.ID():                        {"0c4ce4ce5969151bf75380d76039a7db7461c0597b844ece5866a76de6198e93", "7b9c47b64b7be5d3aa8d344d9e5c46d71988cd4b6d0b8ef030927b06ca641b1e"},
	ProfileMXLSource.ID():                            {"e04420acfbf1ecb5003447144996a5c6dbb7bbfc381d752fc76667fd64303a17", "6a34a664baf9f0e768ded55af8053ef0d4bf9a4cf5c5806cd135f375d482797b"},
	ProfileMXLModel.ID():                             {"ed04026dfc05a64ee427cfedb7c3a933cb8b51929c0b7cb76f2508ac0868d3ed", "9ce6b701af9158e6a405b545ddf22fd42e45d797cfd4fe22a39477a7d9abfd61"},
	ProfileResourceGraphDictionaries.ID():            {"08830a06118f1fe882ea7211aa4b5552a97c7653cbb2a0fefa3a6e112c9e7c6f", "3c1479fac8375036587d1342ae14fae853aadb5d8b1dc9e0305f5d68c9895070"},
	ProfileMXLSourceDictionaries.ID():                {"22c1d06c778e820b6baaa51802e0bbb9b43612e3a08d56a4b15ba0cc47853849", "708c34a08d3ecc3b42e68cca37209de8908584e339511aee1b3e7fc0c86a09a8"},
	ProfileMXLModelDictionaries.ID():                 {"c3e9922ed26ebe889306e07769d57f9aa5c07946a84caa1bacaae20106695b49", "e879aa4d0cb388f7ca767959360037c798e4f7dd87e20343bb0891bc6b4204da"},
	ProfileResourceGraphInternalDTDDictionaries.ID(): {"7dbc72d33fa734b1dc9cf49c74272b7fd2cffb88ceeac1bc301692841101a497", "06c155e0a106a8c1fe928abc3919873093fc7be10b30ba380707e70c8e4cf887"},
	ProfileMXLSourceInternalDTDDictionaries.ID():     {"2dfa360e102b8301cf53c893eae7ee2d9d73c76fc5254bae07dddeea98f95f2e", "d39f531c9424456e4172b4aef5fd9c0ec06bc3ee78fa9f4939e070cb7e21f6ce"},
	ProfileMXLModelInternalDTDDictionaries.ID():      {"4d1b782b8277c653820898e2abae51d8dc0bce42301d057ecd4bf7a74a44d08e", "b003724bedbade7e3b1d398fc287071f43526e7fe7bae65b67c69963f4a93503"},
	ProfileResourceGraphSemantics.ID():               {"16ce6ba1ca182022b2de75bcb2ce1a6177284186a9d918bdef5edeb1a96612e2", "4ef1e127c2c9d958562903b5b7f7dd893abf9201f6772af0a013a87afa929307"},
	ProfileMXLSourceSemantics.ID():                   {"afaaf944f27a5a02b8beccac1f4c11eb685690ce3fd789f5aa64f43b0076b616", "7887d5ef7e645524909bc703bbf639c812d583af521fae280f3067caa4249fde"},
	ProfileMXLModelSemantics.ID():                    {"12bfdb571374eca60c2dccd68634fedfcde96a858b36d436c59d3b3d0614aa64", "2410b7dad8b214651f5fc03b5efd156bc40683c90721ea7ac68b5c7ea20c5aba"},
}

// migrateDecimalContractCatalog returns the digest a catalog captured before
// the contract change has now; other catalogs keep theirs.
func migrateDecimalContractCatalog(before string) string {
	if after, ok := decimalContractCatalogs[before]; ok {
		return after
	}
	return before
}

// migrateDecimalContractProfileHash returns the complete Profile JSON hash a
// profile captured before the contract change has now; other profiles keep
// theirs.
func migrateDecimalContractProfileHash(t *testing.T, id validation.ProfileID, before string) string {
	t.Helper()
	pair, ok := decimalContractProfileHashes[id]
	if !ok {
		return before
	}
	if before != pair[0] {
		t.Fatalf("unexpected pre-decimal-contract profile %s: %s", id, before)
	}
	return pair[1]
}

func migrateFrozenDecimalContractReport(t *testing.T, value any) {
	t.Helper()
	profile := value.(map[string]any)["Profile"].(map[string]any)
	before := profile["CatalogDigest"].(string)
	after := migrateDecimalContractCatalog(before)
	if after == before {
		return
	}
	profile["CatalogDigest"] = after
	if profile["Target"] != string(validation.TargetModel) {
		return
	}
	closure := profile["NormativeClosure"].(string)
	switch {
	case strings.Count(closure, decimalContractTextBefore) > 1:
		t.Fatalf("historical %s closure repeats the decimal contract", profile["ID"])
	case strings.Count(closure, decimalContractTextBefore) == 1:
		closure = strings.Replace(closure, decimalContractTextBefore, decimalContractTextAfter, 1)
	case !strings.Contains(closure, decimalContractTextAfter):
		closure += " " + decimalContractTextAfter
	default:
		t.Fatalf("historical %s closure already names the current decimal contract", profile["ID"])
	}
	profile["NormativeClosure"] = closure
}

// Every current-model profile names the decimal contract in its closure, so
// that composed model profiles, whose builders replace the closure, change
// identity with the contract too, and package intrinsic bindings carry those
// identities.
func TestEveryModelProfileNamesTheDecimalContract(t *testing.T) {
	digests := map[validation.ProfileID]string{}
	for _, id := range []validation.ProfileID{ProfileModelXSD.ID(), ProfileModelLocal.ID(), ProfileModelTimeline.ID(), ProfileModelContext.ID(), ProfileModelLinks.ID(), ProfileModelDictionaries.ID(), ProfileModelSemantics.ID()} {
		engine, err := modelAssessmentEngineForProfile(id)
		if err != nil {
			t.Fatal(err)
		}
		profile, ok := engine.Profile(id)
		if !ok || profile.Target != validation.TargetModel || strings.Count(profile.NormativeClosure, modelDecimalContract) != 1 {
			t.Errorf("%s does not name the decimal contract once: %q", id, profile.NormativeClosure)
		}
		digests[id] = profile.CatalogDigest
	}
	checked := 0
	for name, build := range map[string]func() ([]validation.PackageRuleBinding, []validation.PackageProfileBinding, error){
		"ordinary":   buildPackageAssessmentDefinitions,
		"dictionary": buildDictionaryPackageAssessmentDefinitions,
		"dtd":        buildDTDDictionaryPackageAssessmentDefinitions,
		"semantic":   semanticPackageAssessmentDefinitions,
	} {
		_, packages, err := build()
		if err != nil {
			t.Fatalf("%s package definitions: %v", name, err)
		}
		for _, binding := range packages {
			for _, row := range binding.Intrinsic {
				if row.Target != validation.TargetModel {
					continue
				}
				if want, ok := digests[row.Profile]; !ok || row.CatalogDigest != want {
					t.Errorf("%s intrinsic %s binding has catalog %s, want %s", binding.Profile.ID, row.Profile, row.CatalogDigest, want)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no intrinsic model bindings checked")
	}
}

// Every root catalog goes through newCatalog, so no model profile, including
// one added later, can omit the decimal contract from its identity.
func TestCatalogsNameTheDecimalContract(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "model_decimal_contract.go" {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("validation.NewCatalog(")) {
			t.Errorf("%s builds a catalog without newCatalog", name)
		}
	}
}
