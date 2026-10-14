package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DictionaryKind names a closed reviewed family of dictionary evidence.
type DictionaryKind string

// Reviewed dictionary kinds do not imply a complete normative grammar.
const (
	DictionaryKindSMuFL          DictionaryKind = "smufl"
	DictionaryKindStandardSounds DictionaryKind = "standard-sounds"
	DictionaryKindIPAPositive    DictionaryKind = "ipa-positive"
)

// DictionaryManifestID selects one immutable reviewed transformation and policy.
type DictionaryManifestID string

// Supported dictionary manifests pin independent certifiable components.
const (
	DictionaryManifestSMuFL14Canonical         DictionaryManifestID = "smufl-1.4-canonical-v1"
	DictionaryManifestSMuFL14Ranges            DictionaryManifestID = "smufl-1.4-ranges-v1"
	DictionaryManifestStandardSounds40         DictionaryManifestID = "musicxml-4.0-standard-sounds-v1"
	DictionaryManifestIPAPositive2015Unicode13 DictionaryManifestID = "ipa-2015-unicode13-positive-v1"
)

// DictionaryRelation selects an exact relation within a reviewed manifest.
type DictionaryRelation string

// Dictionary relations are independent of MusicXML XSD lexical validity.
const (
	DictionaryRelationCanonicalName          DictionaryRelation = "canonical-name"
	DictionaryRelationPrimaryCodepoint       DictionaryRelation = "primary-codepoint"
	DictionaryRelationRangeMembership        DictionaryRelation = "range-membership"
	DictionaryRelationStandardSound          DictionaryRelation = "standard-sound"
	DictionaryRelationIPASymbol              DictionaryRelation = "ipa-symbol"
	DictionaryRelationIPAAttestedExample     DictionaryRelation = "ipa-attested-example"
	DictionaryRelationIPACanonicalEquivalent DictionaryRelation = "ipa-canonical-equivalent"
)

// DictionarySelection explicitly selects nonempty relations from one manifest.
// It never implicitly selects another source component or authorizes transport.
type DictionarySelection struct {
	Manifest  DictionaryManifestID
	Relations []DictionaryRelation
}

// DictionaryComponentID identifies the exact resident bytes a verifier accepts.
type DictionaryComponentID string

// Supported components are fixed sources; no arbitrary URI can be verified.
const (
	DictionaryComponentSMuFL14Glyphnames        DictionaryComponentID = "smufl-1.4-glyphnames-json"
	DictionaryComponentSMuFL14Ranges            DictionaryComponentID = "smufl-1.4-ranges-json"
	DictionaryComponentStandardSounds40         DictionaryComponentID = "musicxml-4.0-sounds-xml"
	DictionaryComponentIPAPositive2015Unicode13 DictionaryComponentID = "ipa-2015-unicode13-positive-bundle-json"
)

// DictionarySourceKind distinguishes original source bytes from reviewed derived data.
type DictionarySourceKind string

// A verified derived bundle does not prove original publication bytes were read.
const (
	DictionarySourceOriginalComponent     DictionarySourceKind = "original-component"
	DictionarySourceReviewedDerivedBundle DictionarySourceKind = "reviewed-derived-bundle"
)

// DictionaryComponentPin is fixed corpus metadata, not permission to fetch it.
type DictionaryComponentPin struct {
	ID         DictionaryComponentID
	Bytes      int64
	SHA256     string
	URL        string
	SourceKind DictionarySourceKind
}

// DictionaryManifest describes a fixed reviewed transformation. Relations and
// NegativeRelations are owned slices. NegativeRelations excludes all IPA facts;
// complete positive-table identity never certifies a complete IPA grammar.
type DictionaryManifest struct {
	ID                DictionaryManifestID
	Kind              DictionaryKind
	Edition           string
	Component         DictionaryComponentPin
	ComponentName     string
	TransformRevision string
	PolicyRevision    string
	DataSHA256        string
	DataBytes         int64
	Rows              int64
	Relations         []DictionaryRelation
	NegativeRelations []DictionaryRelation
}

func dictionaryManifest(id DictionaryManifestID) (DictionaryManifest, bool) {
	switch id {
	case DictionaryManifestSMuFL14Canonical:
		return DictionaryManifest{ID: id, Kind: DictionaryKindSMuFL, Edition: "1.4", Component: DictionaryComponentPin{DictionaryComponentSMuFL14Glyphnames, 301683, "1d05352599a20983d1c901635dc75d76f063c0987a7bee65f145325fc3e0d29f", "https://raw.githubusercontent.com/w3c-cg/smufl/753f551ca6f96bf24ba38aa2d8a317e706238c51/metadata/glyphnames.json", DictionarySourceOriginalComponent}, ComponentName: "canonical", TransformRevision: "smufl-glyphnames-primary-v1", PolicyRevision: "smufl14-exact-canonical-primary-v1", DataSHA256: "00be19819ecac90e2463287a7f76d29e2333c6d1d71c95fb9ffdda4d4ecd1bcf", DataBytes: 137769, Rows: 2932, Relations: []DictionaryRelation{DictionaryRelationCanonicalName, DictionaryRelationPrimaryCodepoint}, NegativeRelations: []DictionaryRelation{DictionaryRelationCanonicalName}}, true
	case DictionaryManifestSMuFL14Ranges:
		return DictionaryManifest{ID: id, Kind: DictionaryKindSMuFL, Edition: "1.4", Component: DictionaryComponentPin{DictionaryComponentSMuFL14Ranges, 98796, "a22be610edadf823de387f4c11716beee0ba2a11534c2bae1897b23fb6869531", "https://raw.githubusercontent.com/w3c-cg/smufl/753f551ca6f96bf24ba38aa2d8a317e706238c51/metadata/ranges.json", DictionarySourceOriginalComponent}, ComponentName: "ranges", TransformRevision: "smufl-ranges-members-v1", PolicyRevision: "smufl14-exact-ranges-v1", DataSHA256: "f3cde9fd6b9deb403c315efcf6096b0738c83bfd407bddddfc1f9ae3f78aa54a", DataBytes: 214363, Rows: 3063, Relations: []DictionaryRelation{DictionaryRelationRangeMembership}, NegativeRelations: []DictionaryRelation{DictionaryRelationRangeMembership}}, true
	case DictionaryManifestStandardSounds40:
		return DictionaryManifest{ID: id, Kind: DictionaryKindStandardSounds, Edition: "4.0", Component: DictionaryComponentPin{DictionaryComponentStandardSounds40, 33703, "a73d52b7215ea567dd54c5f2ea3e9d4f7ea167a40d5770a8fe0e3cb458858b35", "https://raw.githubusercontent.com/w3c-cg/musicxml/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/sounds.xml", DictionarySourceOriginalComponent}, ComponentName: "standard-ids", TransformRevision: "musicxml40-pinned-literal-ids-v1", PolicyRevision: "musicxml40-exact-standard-id-v1", DataSHA256: "b6d3d12d9e33c47bb793fe7769190476bc8def765469b699a477aa51771edfaf", DataBytes: 30909, Rows: 894, Relations: []DictionaryRelation{DictionaryRelationStandardSound}, NegativeRelations: []DictionaryRelation{DictionaryRelationStandardSound}}, true
	case DictionaryManifestIPAPositive2015Unicode13:
		return DictionaryManifest{ID: id, Kind: DictionaryKindIPAPositive, Edition: "IPA2015/Unicode13.0.0", Component: DictionaryComponentPin{DictionaryComponentIPAPositive2015Unicode13, 26435, "ee4d39dbf8e999d12ae4e0680fc1b55a9704b81a4f049a110bcf5a21c1da2901", "", DictionarySourceReviewedDerivedBundle}, ComponentName: "reviewed-positive-ledger", TransformRevision: "ipa-chart-positive-v1", PolicyRevision: "ipa2015-unicode13-partial-positive-v1", DataSHA256: "47b0a8812835e56e0f8b6527cca13b6d55bb4b3ee231186868e8ec8590804ec6", DataBytes: 9836, Rows: 197, Relations: []DictionaryRelation{DictionaryRelationIPASymbol, DictionaryRelationIPAAttestedExample, DictionaryRelationIPACanonicalEquivalent}}, true
	}
	return DictionaryManifest{}, false
}

// LookupDictionaryManifest returns owned fixed metadata for a supported pin.
// This finite metadata operation performs no loading or capability activation.
func LookupDictionaryManifest(id DictionaryManifestID) (DictionaryManifest, bool) {
	return dictionaryManifest(id)
}

func dictionaryComponent(id DictionaryComponentID) (DictionaryComponentPin, bool) {
	var manifest DictionaryManifestID
	switch id {
	case DictionaryComponentSMuFL14Glyphnames:
		manifest = DictionaryManifestSMuFL14Canonical
	case DictionaryComponentSMuFL14Ranges:
		manifest = DictionaryManifestSMuFL14Ranges
	case DictionaryComponentStandardSounds40:
		manifest = DictionaryManifestStandardSounds40
	case DictionaryComponentIPAPositive2015Unicode13:
		manifest = DictionaryManifestIPAPositive2015Unicode13
	default:
		return DictionaryComponentPin{}, false
	}
	m, _ := dictionaryManifest(manifest)
	return m.Component, true
}

// DictionaryComponentCertificate is a copyable, sealed proof of exact component
// bytes checked on one Account. Its zero value proves nothing. It owns no input
// bytes or dynamic storage and cannot authorize I/O or certify XML/DTD validity.
type DictionaryComponentCertificate struct {
	account   *Account
	component DictionaryComponentID
	digest    [32]byte
	verified  bool
}

func dictionaryStep(ctx context.Context, a *Account, n int64) error {
	if ctx == nil {
		return contract("dictionary context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Charge(BudgetRuleSteps, n)
}

// VerifyDictionaryComponent hashes already resident bytes against an exact pin.
// The caller must not mutate bytes during verification. Transport owners charge
// external bytes once; this verifier charges only bounded work and live scratch.
// Every failure returns a zero certificate. A success on a derived IPA bundle
// never asserts that chart or Unicode source bytes were read during this run.
func VerifyDictionaryComponent(ctx context.Context, a *Account, id DictionaryComponentID, raw []byte) (DictionaryComponentCertificate, error) {
	var zero DictionaryComponentCertificate
	if err := dictionaryStep(ctx, a, 1); err != nil {
		return zero, err
	}
	const scratch = 512
	if err := a.Charge(BudgetBufferedBytes, scratch); err != nil {
		return zero, err
	}
	defer a.Release(BudgetBufferedBytes, scratch)
	pin, ok := dictionaryComponent(id)
	if !ok {
		return zero, contract("dictionary component", "unsupported component")
	}
	if int64(len(raw)) != pin.Bytes {
		return zero, contract("dictionary component", "source byte length mismatch")
	}
	h := sha256.New()
	for start := 0; start < len(raw); {
		end := start + min(32768, len(raw)-start)
		if err := dictionaryStep(ctx, a, int64(end-start)+1); err != nil {
			return zero, err
		}
		_, _ = h.Write(raw[start:end])
		start = end
	}
	var digest [32]byte
	h.Sum(digest[:0])
	if hex.EncodeToString(digest[:]) != pin.SHA256 {
		return zero, contract("dictionary component", "source SHA-256 mismatch")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return DictionaryComponentCertificate{account: a, component: id, digest: digest, verified: true}, nil
}

func dictionaryCopyString(s string) string { return strings.Clone(s) }
