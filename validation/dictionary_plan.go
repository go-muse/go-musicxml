package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
)

const planDictionary planKind = "pinned-dictionary"
const dictionaryConsumerRevision uint32 = 1
const dictionaryIPARepresentationRevision uint32 = 2

// DictionaryOperation is a closed consumer policy, not an expression language.
type DictionaryOperation string

// Independent operations preserve normative membership versus partial evidence
// and rendering advice. Standard sounds remain open strings.
const (
	DictionaryCanonical      DictionaryOperation = "canonical-name"
	DictionaryStandardSounds DictionaryOperation = "standard-sounds"
	DictionaryIPA            DictionaryOperation = "ipa-positive"
	DictionaryRendering      DictionaryOperation = "rendering"
)

type dictionaryPlan struct {
	Schema    *CompiledSchema
	Bindings  *contextBindings
	Operation DictionaryOperation
	Types     []ComponentID
	Pins      []DictionaryManifest
	Revision  uint32
	Digest    string
}

// newDictionaryPlan binds one exact dictionary consumer to the compiled
// MusicXML or source-only sounds assembly. It neither selects nor loads data.
// The canonical and IPA operations require mandatory node predicates; sounds
// and rendering are context providers. No timeline service is selected.
func newDictionaryPlan(schema *CompiledSchema, operation DictionaryOperation) (Plan, error) {
	if schema == nil || schema.kernel == nil || (schema.assembly != AssemblyMusicXML && schema.assembly != AssemblySounds) {
		return Plan{}, contract("dictionary schema", "compiled MusicXML or sounds assembly required")
	}
	expectedSchema := "52a61387995feb5c632a3660836a6ddaaf1e101d2a12c2f87d5f0bfb5c3191af"
	if schema.assembly == AssemblySounds {
		expectedSchema = "13712e67b078d2de3db7e55cab04d2ef87aa49eb176e5a86e4f9f520cd7bc4ce"
	}
	if schema.Digest() != expectedSchema {
		return Plan{}, contract("dictionary schema", "reviewed pinned assembly digest required")
	}
	if schema.assembly == AssemblySounds && operation != DictionaryStandardSounds {
		return Plan{}, contract("dictionary operation", "sounds assembly supports standard identifiers only")
	}
	switch operation {
	case DictionaryCanonical, DictionaryStandardSounds, DictionaryIPA, DictionaryRendering:
	default:
		return Plan{}, contract("dictionary operation", "unknown closed operation")
	}
	b, err := compileContextBindingSpecs(schema, dictionaryBindingSpecs(schema.assembly))
	if err != nil {
		return Plan{}, err
	}
	d := &dictionaryPlan{Schema: schema, Bindings: b, Operation: operation, Revision: dictionaryConsumerRevision}
	if operation == DictionaryIPA {
		d.Revision = dictionaryIPARepresentationRevision
	}
	if schema.assembly == AssemblyMusicXML {
		for _, name := range dictionarySMuFLTypeNames {
			d.Types = append(d.Types, contextNamedType(name))
		}
	}
	for _, id := range []DictionaryManifestID{DictionaryManifestSMuFL14Canonical, DictionaryManifestSMuFL14Ranges, DictionaryManifestStandardSounds40, DictionaryManifestIPAPositive2015Unicode13} {
		m, ok := dictionaryManifest(id)
		if !ok {
			return Plan{}, contract("dictionary pins", "reviewed compiled manifest unavailable")
		}
		d.Pins = append(d.Pins, m)
	}
	if operation == DictionaryIPA {
		m, _ := dictionaryManifest(DictionaryManifestIPARepresentation2015Unicode13)
		d.Pins = append(d.Pins, m)
	}
	encoded, err := json.Marshal(struct {
		Revision         uint32
		Schema, Bindings string
		Types            []ComponentID
		Pins             []DictionaryManifest
	}{d.Revision, schema.Digest(), b.digest(), d.Types, d.Pins})
	if err != nil {
		return Plan{}, err
	}
	h := sha256.Sum256(encoded)
	d.Digest = hex.EncodeToString(h[:])
	return checkedPlan(Plan{kind: planDictionary, dictionary: d})
}
func (p *dictionaryPlan) valid() error {
	if p == nil || p.Schema == nil || p.Schema.kernel == nil || p.Bindings == nil || len(p.Digest) != 64 {
		return contract("dictionary plan", "sealed exact schema and policy required")
	}
	revision := dictionaryConsumerRevision
	if p.Operation == DictionaryIPA {
		revision = dictionaryIPARepresentationRevision
	}
	if p.Revision != revision {
		return contract("dictionary plan", "sealed exact policy revision required")
	}
	switch p.Operation {
	case DictionaryCanonical, DictionaryStandardSounds, DictionaryIPA, DictionaryRendering:
	default:
		return contract("dictionary operation", "unknown operation")
	}
	if p.Schema.assembly != AssemblyMusicXML && (p.Schema.assembly != AssemblySounds || p.Operation != DictionaryStandardSounds) {
		return contract("dictionary assembly", "exact operation assembly required")
	}
	return nil
}
func (p *dictionaryPlan) clone() *dictionaryPlan {
	if p == nil {
		return nil
	}
	q := *p
	q.Schema = p.Schema.snapshot()
	return &q
}
func (p *dictionaryPlan) size() int64 {
	if p == nil {
		return 0
	}
	n := int64(1024 + len(p.Digest))
	n += p.Bindings.size()
	for _, v := range p.Types {
		n += contextComponentBytes(v) + 128
	}
	for _, v := range p.Pins {
		// Fixed manifest/component headers, numeric fields and slice overhead,
		// plus all retained text and relation entries. Sizing never serializes
		// pins; the independently typed digest encoding above is unchanged.
		n += 1024
		for _, text := range [...]string{string(v.ID), string(v.Kind), v.Edition,
			string(v.Component.ID), v.Component.SHA256, v.Component.URL,
			string(v.Component.SourceKind), v.ComponentName, v.TransformRevision,
			v.PolicyRevision, v.DataSHA256} {
			n += int64(len(text))
		}
		for _, relation := range v.Relations {
			n += 32 + int64(len(relation))
		}
		for _, relation := range v.NegativeRelations {
			n += 32 + int64(len(relation))
		}
	}
	return n
}
func (p *dictionaryPlan) digest() any {
	return struct {
		Kind          planKind
		Operation     DictionaryOperation
		BindingPolicy string
	}{planDictionary, p.Operation, p.Digest}
}
func (p *dictionaryPlan) validateFor(r RuleDefinition) error {
	if err := p.valid(); err != nil {
		return err
	}
	if r.Selector.Assembly == nil || *r.Selector.Assembly != p.Schema.assembly || r.Selector.Component != nil || r.Selector.Declaration != nil || r.Scope != ScopeNode || r.Class != ClassMandatory || r.Category != CategoryNormative || len(r.Requires) != 0 {
		return contract("dictionary rule", "independent exact assembly node rule required")
	}
	role := RoleContextProvider
	if p.Operation == DictionaryCanonical || p.Operation == DictionaryIPA {
		role = RolePredicate
	}
	if r.Role != role {
		return contract("dictionary role", "operation requires exact predicate/provider role")
	}
	for _, target := range r.Targets {
		if target != TargetSource && (target != TargetModel || p.Schema.assembly == AssemblySounds) {
			return contract("dictionary target", "exact source/current-model target required")
		}
	}
	return nil
}
func validateDictionaryProfile(c *CompiledCatalog, closure map[RuleKey]bool, schemas map[Assembly]*CompiledSchema) error {
	seen := map[string]bool{}
	for key := range closure {
		p := c.plans[c.rules[key].Predicate]
		if p.kind != planDictionary {
			continue
		}
		d := p.dictionary
		schema := schemas[d.Schema.assembly]
		if schema == nil || schema.Digest() != d.Schema.Digest() {
			return contract("dictionary schema", "matching selected XSD assembly required")
		}
		id := string(d.Schema.assembly) + "/" + string(d.Operation)
		if seen[id] {
			return contract("dictionary profile", "one exact operation per assembly required")
		}
		seen[id] = true
	}
	return nil
}
func (p *dictionaryPlan) actualSMuFL(t ComponentID) bool { return slices.Contains(p.Types, t) }
