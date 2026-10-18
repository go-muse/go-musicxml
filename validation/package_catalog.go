package validation

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

// PackageCheck names one sealed package/resource predicate or observation.
type PackageCheck string

// The closed package check vocabulary distinguishes clear package predicates,
// linked facts, optional advice and schema-hint discovery usability.
const (
	PackageCheckContainerPath          PackageCheck = "container-path"
	PackageCheckMimetypeStored         PackageCheck = "mimetype-stored"
	PackageCheckMimetypeNoExtra        PackageCheck = "mimetype-no-extra"
	PackageCheckMimetypeNoPrefix       PackageCheck = "mimetype-no-prefix"
	PackageCheckFirstRootMusicXML      PackageCheck = "first-root-musicxml"
	PackageCheckRootfileRoot           PackageCheck = "rootfile-root"
	PackageCheckFirstMediaType         PackageCheck = "first-media-type"
	PackageCheckScoreBeforeParts       PackageCheck = "score-before-parts"
	PackageCheckLinkTarget             PackageCheck = "link-target"
	PackageCheckInstrumentAssociation  PackageCheck = "instrument-association"
	PackageCheckGroupMatch             PackageCheck = "group-match"
	PackageCheckMimetypeAdvice         PackageCheck = "mimetype-advice"
	PackageCheckEntryNameAdvice        PackageCheck = "entry-name-advice"
	PackageCheckCompressionObservation PackageCheck = "compression-observation"
	// PackageCheckSchemaHintPairs pairs the schema hints of source
	// resources. A model snapshot publishes no hints, so the check does not
	// apply to model resources; it used to leave them unknown. The plan
	// carries no revision, so a catalog that binds this check should revise
	// its rule to make the change visible in its digest, as the root
	// package's MX40-PACKAGE-schema-hint-pairs revision 2 does.
	PackageCheckSchemaHintPairs PackageCheck = "schema-hint-pairs"
)

// PackageCheckImageScaling and PackageCheckFileMetadataAdvice expose optional
// intrinsic-resource ratio and file naming/media advice without rendering.
const (
	PackageCheckImageScaling       PackageCheck = "image-scaling"
	PackageCheckFileMetadataAdvice PackageCheck = "file-metadata-advice"
)

// PackageSubjectKind identifies a typed package subject, never an XML selector.
type PackageSubjectKind string

// Supported package subject families.
const (
	PackageSubjectPackage     PackageSubjectKind = "package"
	PackageSubjectEntry       PackageSubjectKind = "entry"
	PackageSubjectRootfile    PackageSubjectKind = "rootfile"
	PackageSubjectEdge        PackageSubjectKind = "edge"
	PackageSubjectAssociation PackageSubjectKind = "association"
	PackageSubjectResource    PackageSubjectKind = "resource"
)

// PackageSubjectImage and PackageSubjectMetadata select typed optional evidence.
const (
	PackageSubjectImage    PackageSubjectKind = "image"
	PackageSubjectMetadata PackageSubjectKind = "metadata"
)

// PackageSelector selects the exact closed subject family required by its plan.
type PackageSelector struct{ Kind PackageSubjectKind }

// PackagePlan is an immutable constructor-produced sealed check, not executable
// caller code, a predicate callback, or a runtime expression.
type PackagePlan struct{ check PackageCheck }

// NewPackagePlan validates a member of the closed package-check vocabulary.
func NewPackagePlan(check PackageCheck) (PackagePlan, error) {
	if packageCheckSubject(check) == "" {
		return PackagePlan{}, contract("package check", "unknown check")
	}
	return PackagePlan{check: check}, nil
}

// Check reports the sealed semantic operation.
func (p PackagePlan) Check() PackageCheck { return p.check }

// PackageRuleBinding keeps existing RuleDefinition layout unchanged. Its XML
// Selector must be empty; only NewPackageCatalog accepts this target contract.
type PackageRuleBinding struct {
	Rule     RuleDefinition
	Selector PackageSelector
	Plan     PackagePlan
}

// IntrinsicProfileBinding fixes an assembly/representation interpretation in the
// package catalog digest; individual cache hits cannot weaken it. Snapshot
// transfer requires a known matching assembly, even for failed reports. If the
// assembly is unavailable, preserve failure through RecordResourceFailure.
type IntrinsicProfileBinding struct {
	Assembly      Assembly
	Target        Target
	Profile       ProfileID
	CatalogDigest string
}

// PackageProfileBinding gives a package profile its fixed intrinsic table.
// Profile.Target is package and SupportedRoots is empty, with no fake XML root.
type PackageProfileBinding struct {
	Profile   Profile
	Intrinsic []IntrinsicProfileBinding
}

// CompiledPackageCatalog owns immutable bounded metadata and sealed plans.
// It is safe for concurrent readers; its sessions are independently owned.
type CompiledPackageCatalog struct {
	rules    map[RuleKey]PackageRuleBinding
	profiles map[ProfileID]PackageProfileBinding
	orders   map[ProfileID][]RuleKey
	supports map[ProfileID]map[RuleKey]RuleSupport
}

func packageCheckSubject(c PackageCheck) PackageSubjectKind {
	switch c {
	case PackageCheckContainerPath, PackageCheckScoreBeforeParts, PackageCheckMimetypeAdvice:
		return PackageSubjectPackage
	case PackageCheckMimetypeStored, PackageCheckMimetypeNoExtra, PackageCheckMimetypeNoPrefix, PackageCheckEntryNameAdvice, PackageCheckCompressionObservation:
		return PackageSubjectEntry
	case PackageCheckFirstRootMusicXML, PackageCheckRootfileRoot, PackageCheckFirstMediaType:
		return PackageSubjectRootfile
	case PackageCheckLinkTarget, PackageCheckGroupMatch:
		return PackageSubjectEdge
	case PackageCheckInstrumentAssociation:
		return PackageSubjectAssociation
	case PackageCheckSchemaHintPairs:
		return PackageSubjectResource
	case PackageCheckImageScaling:
		return PackageSubjectImage
	case PackageCheckFileMetadataAdvice:
		return PackageSubjectMetadata
	}
	return ""
}

// NewPackageCatalog validates exact package selectors, metadata, dependency
// closure and fixed intrinsic profile digests before copying. Ordinary NewCatalog
// retains its XML root and exclusive XML selector invariants unchanged. All
// selected/transitive checks require exactly one support row. Compilation uses
// the existing 65,536-rule/4,096-profile/64 MiB bounded catalog helpers.
func NewPackageCatalog(rules []PackageRuleBinding, profiles []PackageProfileBinding) (*CompiledPackageCatalog, error) {
	if len(rules) > catalogMaxRules || len(profiles) > catalogMaxProfiles || len(profiles) == 0 {
		return nil, contract("package catalog", "invalid collection length")
	}
	// These fixed-size views are bounded before allocation by the hard catalog
	// collection limits and retain no copied nested caller storage.
	rr := make([]RuleDefinition, len(rules))
	pp := make([]Profile, len(profiles))
	for i, r := range rules {
		rr[i] = r.Rule
	}
	extra := catalogBudget{remaining: catalogMaxBytes}
	extra.slots(len(rules))
	extra.slots(len(profiles))
	for i, p := range profiles {
		pp[i] = p.Profile
		extra.slots(len(p.Intrinsic))
		if !extra.ok() {
			break
		}
		for _, b := range p.Intrinsic {
			extra.identity(string(b.Assembly))
			extra.identity(string(b.Target))
			extra.identity(string(b.Profile))
			extra.text(b.CatalogDigest, 64)
		}
	}
	if !extra.ok() {
		return nil, contract("package catalog size", "intrinsic table exceeds compilation bounds")
	}
	if err := catalogCheckBoundsRemaining(rr, pp, nil, extra.remaining); err != nil {
		return nil, err
	}
	c := &CompiledPackageCatalog{rules: make(map[RuleKey]PackageRuleBinding, len(rules)), profiles: make(map[ProfileID]PackageProfileBinding, len(profiles)), orders: make(map[ProfileID][]RuleKey, len(profiles)), supports: make(map[ProfileID]map[RuleKey]RuleSupport, len(profiles))}
	metadata := make(map[RuleKey]RuleDefinition, len(rules))
	plans := make(map[string]PackageCheck)
	for _, b := range rules {
		r := b.Rule
		if err := catalogValidateRuleMode(r, false); err != nil {
			return nil, err
		}
		if r.Selector != (Selector{}) || r.Scope != ScopePackage || len(r.Targets) != 1 || r.Targets[0] != TargetPackage {
			return nil, contract("package rule", "empty XML selector, package scope and exclusive package target required")
		}
		if b.Selector.Kind == "" || b.Selector.Kind != packageCheckSubject(b.Plan.check) {
			return nil, contract("package selector", "selector/check mismatch or uninitialized plan")
		}
		if err := packageValidateRole(b); err != nil {
			return nil, err
		}
		if _, ok := c.rules[r.Key]; ok {
			return nil, contract("package rule", "duplicate exact rule")
		}
		if previous, ok := plans[r.Predicate]; ok && previous != b.Plan.check {
			return nil, contract("package plan", "one predicate identity denotes conflicting plans")
		}
		plans[r.Predicate] = b.Plan.check
		for _, d := range r.Requires {
			if d.Kind != DependencyRule {
				return nil, contract("package dependency", "only compiled rule dependencies supported")
			}
			if d.Relation != RelationSameNode && d.Relation != RelationDocument {
				return nil, contract("package dependency", "only exact subject or package scope relations supported")
			}
		}
		b.Rule = cloneRule(r)
		c.rules[b.Rule.Key] = b
		metadata[b.Rule.Key] = b.Rule
	}
	for _, rule := range c.rules {
		for _, dep := range rule.Rule.Requires {
			provider, ok := c.rules[dep.Rule]
			if !ok {
				continue
			}
			if dep.Relation == RelationSameNode && provider.Selector.Kind != rule.Selector.Kind {
				return nil, contract("package dependency", "same subject needs identical selector kinds")
			}
			if dep.Relation == RelationDocument && (provider.Selector.Kind != PackageSubjectPackage || rule.Selector.Kind == PackageSubjectResource) {
				return nil, contract("package dependency", "document relation requires package-boundary provider and scoped consumer")
			}
		}
	}
	order, err := catalogOrder(metadata)
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if err = catalogValidateProfileMode(p.Profile, false); err != nil {
			return nil, err
		}
		if p.Profile.Target != TargetPackage || len(p.Profile.SupportedRoots) != 0 {
			return nil, contract("package profile", "package target and no XML roots required")
		}
		if _, ok := c.profiles[p.Profile.ID]; ok {
			return nil, contract("package profile", "duplicate profile")
		}
		seen := map[[2]string]bool{}
		for _, i := range p.Intrinsic {
			k := [2]string{string(i.Assembly), string(i.Target)}
			if !validAssembly(i.Assembly) || (i.Target != TargetSource && i.Target != TargetModel) || !catalogID(string(i.Profile)) || !validResourceDigest(i.CatalogDigest) || seen[k] {
				return nil, contract("package intrinsic profile", "invalid, duplicate or unpinned interpretation")
			}
			seen[k] = true
		}
		closure, e := catalogClosure(p.Profile.Required, metadata)
		if e != nil {
			return nil, e
		}
		supports := make(map[RuleKey]RuleSupport, len(p.Profile.Support))
		for _, v := range p.Profile.Support {
			if _, ok := supports[v.Rule]; ok {
				return nil, contract("package support", "duplicate rule")
			}
			supports[v.Rule] = v
		}
		if len(closure) != len(supports) {
			return nil, contract("package support", "support must exactly cover closure")
		}
		for key := range closure {
			if _, ok := supports[key]; !ok {
				return nil, contract("package support", "missing rule")
			}
		}
		p.Profile = cloneProfile(p.Profile)
		p.Intrinsic = stateClone(p.Intrinsic)
		c.profiles[p.Profile.ID] = p
		c.supports[p.Profile.ID] = supports
		for _, key := range order {
			if closure[key] {
				c.orders[p.Profile.ID] = append(c.orders[p.Profile.ID], key)
			}
		}
	}
	digest, err := packageCatalogDigest(c, metadata)
	if err != nil {
		return nil, err
	}
	for id, p := range c.profiles {
		if p.Profile.CatalogDigest != "" && p.Profile.CatalogDigest != digest {
			return nil, contract("package catalog digest", "supplied digest differs from canonical artifact")
		}
		p.Profile.CatalogDigest = digest
		c.profiles[id] = p
	}
	return c, nil
}
func packageValidateRole(b PackageRuleBinding) error {
	r := b.Rule
	switch b.Plan.check {
	case PackageCheckGroupMatch, PackageCheckMimetypeAdvice, PackageCheckEntryNameAdvice, PackageCheckImageScaling, PackageCheckFileMetadataAdvice:
		if r.Role != RoleAdvisory || r.Category != CategoryAdvisory || r.Class != ClassRecommendation {
			return contract("package role", "advice cannot become mandatory rejection")
		}
	case PackageCheckCompressionObservation:
		if r.Role != RoleContextProvider && r.Role != RoleAdvisory {
			return contract("package role", "compression observation is not normative exclusivity")
		}
	case PackageCheckSchemaHintPairs:
		if r.Category != CategoryCapability || r.Class != ClassPolicy || r.Role != RolePredicate {
			return contract("package role", "schema discovery usability is a capability predicate")
		}
	default:
		if r.Category != CategoryNormative || r.Class != ClassMandatory || r.Role != RolePredicate {
			return contract("package role", "normative package check requires mandatory predicate metadata")
		}
	}
	return nil
}
func packageCatalogDigest(c *CompiledPackageCatalog, rules map[RuleKey]RuleDefinition) (string, error) {
	profiles := make(map[ProfileID]Profile, len(c.profiles))
	for id, p := range c.profiles {
		profiles[id] = p.Profile
	}
	base, err := catalogDigest(&CompiledCatalog{rules: rules, profiles: profiles})
	if err != nil {
		return "", err
	}
	type binding struct {
		Key      RuleKey
		Selector PackageSelector
		Check    PackageCheck
	}
	type intrinsic struct {
		Profile ProfileID
		Table   []IntrinsicProfileBinding
	}
	artifact := struct {
		Format, Metadata string
		Bindings         []binding
		Intrinsic        []intrinsic
	}{Format: "validation-package-catalog-v1", Metadata: base}
	for k, b := range c.rules {
		artifact.Bindings = append(artifact.Bindings, binding{k, b.Selector, b.Plan.check})
	}
	slices.SortFunc(artifact.Bindings, func(a, b binding) int { return catalogCompareKeys(a.Key, b.Key) })
	for id, p := range c.profiles {
		table := stateClone(p.Intrinsic)
		slices.SortFunc(table, func(a, b IntrinsicProfileBinding) int {
			if n := cmp.Compare(a.Assembly, b.Assembly); n != 0 {
				return n
			}
			return cmp.Compare(a.Target, b.Target)
		})
		artifact.Intrinsic = append(artifact.Intrinsic, intrinsic{id, table})
	}
	slices.SortFunc(artifact.Intrinsic, func(a, b intrinsic) int { return cmp.Compare(a.Profile, b.Profile) })
	data, err := json.Marshal(artifact)
	if err != nil {
		return "", contract("package digest", "cannot encode closed artifact")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Profile returns a defensive metadata copy.
func (c *CompiledPackageCatalog) Profile(id ProfileID) (Profile, bool) {
	if c == nil {
		return Profile{}, false
	}
	p, ok := c.profiles[id]
	if !ok {
		return Profile{}, false
	}
	return cloneProfile(p.Profile), true
}

// Rule returns owned metadata with its exact package selector and sealed plan.
func (c *CompiledPackageCatalog) Rule(key RuleKey) (PackageRuleBinding, bool) {
	if c == nil {
		return PackageRuleBinding{}, false
	}
	r, ok := c.rules[key]
	if !ok {
		return PackageRuleBinding{}, false
	}
	r.Rule = cloneRule(r.Rule)
	return r, true
}
func validResourceDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return false
			}
		}
	}
	return true
}
func packageIdentityText(v string) bool {
	return v != "" && validText(v, 4096) && !strings.ContainsAny(v, "\r\n\t")
}
