package musicxml

import (
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileResourceGraph selects bounded resource-graph assessment with a package
// target, independently scoped intrinsic source/model reports and no XML root
// selector. Authorized nested MXL boundaries receive their applicable package
// checks. A current model root is assessed as it would be encoded: it carries
// no xsi attributes, so schema-hint discovery does not apply to it. The profile
// grants no I/O authority and does not assert complete MusicXML or
// external-spec conformance.
const ProfileResourceGraph validation.ProfileID = "musicxml-4.0/resource-graph"

// ProfileMXLSource selects bounded original-MXL package/resource assessment.
// Original ZIP evidence and source-only intrinsic reports remain distinct; known
// residual ZIP, media-type and nested-rootfile interpretation gaps are explicit.
// It has TargetPackage, no XML roots and no implicit resolver authority.
const ProfileMXLSource validation.ProfileID = "musicxml-4.0/mxl-source"

// ProfileMXLModel selects bounded assessment of the current state of a whole
// MXLPackage, as EncodeMXLPackage would write it. Its current Document is
// assessed directly and carries no xsi attributes, so schema-hint discovery
// does not apply to it; retained raw resources that are reached keep their
// source assessment. Original ZIP headers, container lexical form and
// discarded xsi attributes are never reconstructed: raw checks apply only to
// separately reached raw-MXL boundaries. The graph report has TargetPackage
// and no XML roots.
const ProfileMXLModel validation.ProfileID = "musicxml-4.0/mxl-model"

var packageAssessmentCatalog = sync.OnceValues(buildPackageAssessmentCatalog)

func newPackageAssessmentCatalog() (*validation.CompiledPackageCatalog, error) {
	return packageAssessmentCatalog()
}

func buildPackageAssessmentCatalog() (*validation.CompiledPackageCatalog, error) {
	rules, profiles, err := buildPackageAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	return validation.NewPackageCatalog(rules, profiles)
}

// A catalog pins interpretations, not fetched data. These definitions use only
// compiled intrinsic metadata and never start an assessment or consult a URI.
func buildPackageAssessmentDefinitions() ([]validation.PackageRuleBinding, []validation.PackageProfileBinding, error) {
	intrinsic, err := packageIntrinsicProfiles()
	if err != nil {
		return nil, nil, err
	}
	rules, err := packageAssessmentRules()
	if err != nil {
		return nil, nil, err
	}
	var profiles []validation.PackageProfileBinding
	for _, input := range []struct {
		id      validation.ProfileID
		closure string
	}{
		{ProfileResourceGraph, "Finite resource graph rooted in original XML or a direct current MusicXML/Opus model. The graph report has package target; plain XML/model boundaries do not assert ZIP/container obligations. Authorized nested raw MXL boundaries receive the selected applicable package checks. Source and current-model intrinsic interpretations are fixed separately."},
		{ProfileMXLSource, "Bounded original raw-MXL package and selected resource graph. Original local/central headers, mimetype prefixes, physical order and independently assessed original container XML remain source evidence. Only source intrinsic interpretations are accepted; a decoded current model cannot replace original XML proof. Each authorized nested raw MXL is a separate encapsulation boundary."},
		{ProfileMXLModel, "Bounded current MXLPackage and selected resource graph. Assess the current primary Document directly and separately selected retained XML bytes as source. Do not encode/reparse to recover original source. Current package fields do not prove original compression, encryption, header extras, physical order, original mimetype bytes or container lexical presence. Those original-byte checks are outside the current-boundary claim; they remain selected for separately reached authorized raw-MXL boundaries."},
	} {
		p := validation.PackageProfileBinding{Profile: validation.Profile{
			ID: input.id, MusicXMLVersion: "4.0", RegistryCommit: assessmentRegistryCommit,
			Target: validation.TargetPackage, NormativeClosure: input.closure + " " + packageAssessmentClosure,
		}}
		for _, r := range rules {
			p.Profile.Required = append(p.Profile.Required, r.Rule.Key)
			p.Profile.Support = append(p.Profile.Support, validation.RuleSupport{Rule: r.Rule.Key, State: validation.ImplementationAvailable})
		}
		for _, row := range intrinsic {
			if input.id != ProfileMXLSource || row.Target == validation.TargetSource {
				p.Intrinsic = append(p.Intrinsic, row)
			}
		}
		profiles = append(profiles, p)
	}
	return rules, profiles, nil
}

const packageAssessmentClosure = "The selected sealed checks preserve exact container path, ordered rootfile and no-namespace musical-root evidence, reviewed first media-type cases, conditional score-before-parts with roles from a provider or the MusicXML link semantics, and exact authorized instrument-part-file association. Local XSD/reference facts are prerequisites, not duplicated package datatype rules. Independent intrinsic source/model reports keep their own profiles, digests, targets and counts; successful target-kind evidence never certifies child conformance. Selected schema-hint namespace/location pairing is a capability policy separate from the accepted list(anyURI) datatype. Unmatched hints or missing discovery evidence prevent selected discovery completion. A current model resource is assessed as it would be encoded: it cannot represent xsi attributes and carries none, so the pairing does not apply to it; discarded original attributes are never reconstructed. No hinted schema is loaded or selected. Group, image scaling, mimetype presence/first placement/exact content, UTF-8 entry names and file/MIME conventions remain optional advice. Observed compression is capability context, not an invented DEFLATE-only rejection rule. Raw mimetype checks distinguish mandatory stored/unencrypted, local-header no-extra and reviewed leading padding/five standard Unicode BOM cases from optional exact content; central-only extras satisfy this local-header predicate, while independent ZIP framing/association obligations and other padding families retain their own limits. Complete ZIP/JAR/DEFLATE normative closure, the complete MusicXML media-type set, outer compressed-rootfile and linked-target indirection and unproved association or role interpretations remain unresolved when reached. Bounded technical nested traversal does not resolve outer indirection wording. Missing authority, unavailable target/root/fragment evidence and incomplete graph enumeration cannot certify success. The fixed intrinsic table includes source Container/Sounds for selected auxiliary resources without adding Document model roots. MusicXML 4.0 identifies the pinned schema interpretation, not a blanket version=4.0 instance restriction. No filename-based role inference, cross-file ID equality, universal XPointer, renderer, dictionary, automatic network/filesystem access or full MusicXML conformance claim. Existing intrinsic syntax, unread-DTD, representation and resource limits remain active."

func packageIntrinsicProfiles() ([]validation.IntrinsicProfileBinding, error) {
	var result []validation.IntrinsicProfileBinding
	for _, input := range []struct {
		assembly validation.Assembly
		target   validation.Target
		profile  validation.ProfileID
	}{
		{validation.AssemblyMusicXML, validation.TargetSource, ProfileSourceLinks},
		{validation.AssemblyOpus, validation.TargetSource, ProfileSourceLinks},
		{validation.AssemblyContainer, validation.TargetSource, ProfileSourceContainer},
		{validation.AssemblySounds, validation.TargetSource, ProfileSourceSounds},
		{validation.AssemblyMusicXML, validation.TargetModel, ProfileModelLinks},
		{validation.AssemblyOpus, validation.TargetModel, ProfileModelLinks},
	} {
		var engine *validation.Engine
		var err error
		if input.target == validation.TargetSource {
			engine, err = sourceAssessmentEngineForProfile(input.profile)
		} else {
			engine, err = modelAssessmentEngineForProfile(input.profile)
		}
		if err != nil {
			return nil, err
		}
		p, ok := engine.Profile(input.profile)
		if !ok || p.Target != input.target || p.CatalogDigest == "" {
			return nil, &validation.ConfigurationError{Field: "package intrinsic profile", Reason: "missing exact target/profile catalog interpretation"}
		}
		result = append(result, validation.IntrinsicProfileBinding{Assembly: input.assembly, Target: input.target, Profile: input.profile, CatalogDigest: p.CatalogDigest})
	}
	return result, nil
}

func packageAssessmentRules() ([]validation.PackageRuleBinding, error) {
	// Original research IDs are provenance. Runtime identities denote these
	// reviewed sealed composites, not one predicate per research record.
	container := func(line string, ids ...string) []validation.SourceRef {
		return []validation.SourceRef{{IDs: ids, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd#L" + line, Locator: "MusicXML 4.0 container schema annotation, line " + line}}
	}
	score := func(line int, ids ...string) []validation.SourceRef {
		return []validation.SourceRef{{IDs: ids, URL: localSchemaSource(line), Locator: "MusicXML 4.0 exact score schema annotation/use"}}
	}
	type spec struct {
		check                       validation.PackageCheck
		subject                     validation.PackageSubjectKind
		title, explanation, message string
		sources                     []validation.SourceRef
	}
	specs := []spec{
		{validation.PackageCheckContainerPath, validation.PackageSubjectPackage, "Exact container entry", "A represented MXL package needs the exact case-sensitive META-INF/container.xml entry. Its original XML schema assessment is a separate required intrinsic report; an incomplete inventory cannot prove absence.", "The exact package container entry was not established.", append(container("140", "MX40-PROSE-mxl-container-path"), validation.SourceRef{IDs: []string{"MX40-PROSE-mxl-container-path"}, URL: "https://www.w3.org/2021/06/musicxml40/tutorial/compressed-mxl-files/", Locator: "Dated MusicXML 4.0 compressed MXL container path; actual published XSD governs schema assessment"})},
		{validation.PackageCheckMimetypeStored, validation.PackageSubjectEntry, "Stored and unencrypted mimetype", "For an existing raw mimetype entry, establish STORE and no encryption from independently retained local and central headers. An independently proved local defect survives later transport failure. Missing mimetype is separate optional advice.", "The raw mimetype entry is not proved stored and unencrypted.", container("46", "MX40-PROSE-mxl-mimetype-stored")},
		{validation.PackageCheckMimetypeNoExtra, validation.PackageSubjectEntry, "Mimetype local-header extra field", "A proved local-header extra field violates the early-byte identification clause. With independently associated headers, zero local extra length passes even with central-only extras. Central ZIP framing and capability checks remain independent.", "The mimetype no-extra-field condition was not established.", container("46", "MX40-PROSE-mxl-mimetype-no-extra")},
		{validation.PackageCheckMimetypeNoPrefix, validation.PackageSubjectEntry, "Mimetype leading bytes", "Inspect original mimetype bytes for reviewed leading ASCII/NUL padding and UTF-8, UTF-16BE/LE and UTF-32BE/LE BOMs. Other padding families remain unavailable. This condition does not require exact MIME content or prohibit trailing whitespace.", "The mimetype leading-byte condition was not established.", container("46", "MX40-PROSE-mxl-mimetype-no-prefix")},
		{validation.PackageCheckFirstRootMusicXML, validation.PackageSubjectRootfile, "Primary MusicXML representation", "The first encoded rootfile occurrence identifies the primary musical representation. Resolve actual target kind independently of filename, sort order or child conformance. Nested compressed indirection retains its interpretation gate.", "The first rootfile was not established as a MusicXML representation.", container("117", "MX40-PROSE-mxl-first-root-musicxml")},
		{validation.PackageCheckRootfileRoot, validation.PackageSubjectRootfile, "Musical rootfile document kind", "A MusicXML XML rootfile has an unqualified score-partwise, score-timewise or opus document element. Known non-MusicXML later alternates are outside this condition. Target-root truth is separate from required intrinsic acceptance and nested compressed indirection.", "A selected MusicXML rootfile document kind was not established.", container("128", "MX40-PROSE-mxl-rootfile-document-root")},
		{validation.PackageCheckFirstMediaType, validation.PackageSubjectRootfile, "Primary rootfile media type", "The first rootfile cannot declare an established non-MusicXML media type. Preserve omitted versus present-empty token values and the no-value MusicXML default. Registered types and reviewed parameters are bounded evidence; the complete allowed set remains unresolved.", "The first rootfile media type was not established as MusicXML.", container("130", "MX40-PROSE-mxl-first-media-type", "MX40-PROSE-mxl-media-type-default")},
		{validation.PackageCheckScoreBeforeParts, validation.PackageSubjectPackage, "Score precedes individual parts", "When exact scoped role evidence establishes both a score and separate part files, the first rootfile names the score. Linked-only parts count. Without a role provider decision, the MusicXML link semantics of a complete package decide: part-link targets are part files and their source the score, opus score targets are scores, opus-link and work/opus targets are opus documents (link-target checks their root kind), and a later rootfile with an established non-MusicXML media type is not MusicXML: the declared media type is authoritative. A known role-provider fact is never overridden. A complete minimal package may prove the antecedent false; filenames, part counts and matching IDs cannot classify roles.", "The conditional score-before-parts requirement was not established.", container("119", "MX40-PROSE-mxl-score-before-parts")},
		{validation.PackageCheckLinkTarget, validation.PackageSubjectEdge, "Selected linked target kind", "Use the exact local template family, authorized adopted target and supported fragment evidence. Opus-link and score work-opus select opus; Opus score and part-link select a score root. Inaccessible or unsupported targets remain unavailable; successful root-kind evidence never replaces child assessment.", "The selected linked target was not established.", []validation.SourceRef{
			{IDs: []string{"MX40-PROSE-opus-link-target", "MX40-PROSE-opus-score-target"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/opus.xsd#L76", Locator: "opus-link and score declarations/annotations"},
			{IDs: []string{"MX40-PROSE-part-link-archive-root"}, URL: localSchemaSource(5994), Locator: "part-link target and ZIP-root-relative context"},
			{IDs: []string{"MX40-PROSE-compressed-relative-links"}, URL: localSchemaSource(4541), Locator: "Exact score link family ZIP-root base; not a universal Opus rule"},
			{IDs: []string{"MusicXML-4.0-musicxml.xsd"}, URL: localSchemaSource(5953), Locator: "Score work opus reference targets an opus document"},
			{IDs: []string{"XPointer-Framework-shorthand"}, URL: "https://www.w3.org/TR/2003/REC-xptr-framework-20030325/#shorthand", Locator: "Bounded schema-ID shorthand interpretation; arbitrary schemes excluded"},
		}},
		{validation.PackageCheckInstrumentAssociation, validation.PackageSubjectAssociation, "Instrument membership in linked part file", "After the existing local same-part IDREF prerequisite, require exact instrument-to-part-file association evidence bound to both resource pins and exact subjects. Missing authority or partial table misses stay unknown; there is no ID-spelling bijection.", "The instrument association with its linked part file was not established.", score(5928, "MX40-PROSE-instrument-link-target")},
		{validation.PackageCheckGroupMatch, validation.PackageSubjectEdge, "Linked group correspondence advice", "Compare exact xs:string group data through a resolved part-link using authoritative target context. A mismatch or unavailable comparison is optional advice, not normative rejection or fabricated agreement.", "Linked group correspondence advice is unavailable or differs.", score(6000, "MX40-PROSE-group-link-match")},
		{validation.PackageCheckMimetypeAdvice, validation.PackageSubjectPackage, "Recommended mimetype conventions", "Presence, first physical placement and exact US-ASCII MIME content are recommendations. Older containers may omit mimetype. These conventions are separate from mandatory existing-entry header and leading-byte conditions.", "A recommended mimetype convention was not established.", container("42", "MX40-PROSE-mxl-mimetype-first")},
		{validation.PackageCheckEntryNameAdvice, validation.PackageSubjectEntry, "UTF-8 entry-name advice", "Retain observed entry-name encoding and recommend UTF-8 interoperability. Legacy encoding does not become a normative MusicXML violation; path-security processing policy is separate.", "UTF-8 entry-name advice was not established.", container("28", "MX40-PROSE-mxl-entry-name-utf8")},
		{validation.PackageCheckCompressionObservation, validation.PackageSubjectEntry, "Observed archive compression", "Expose bounded original compression-method evidence. The DEFLATE mandatory-versus-description gate and complete ZIP/JAR closure remain open. A transport implementation's unsupported algorithm is not a MusicXML violation.", "Original archive compression evidence was not available.", container("30", "MX40-PROSE-mxl-deflate", "MX40-PROSE-mxl-zip-format")},
		{validation.PackageCheckSchemaHintPairs, validation.PackageSubjectResource, "Selected schema discovery pairing", "Consume separately projected namespace/location pairs and unmatched trailing URI as capability evidence. Odd list length is not an XSD datatype failure; noNamespaceSchemaLocation creates no pair requirement. A current model resource cannot represent xsi attributes and is encoded without them, so the pairing does not apply to it; original source hints are not reconstructed. Hints never load or select a schema.", "Selected schema discovery evidence is incomplete.", []validation.SourceRef{{IDs: []string{"REQ-DEF-XSI-SCHEMALOC", "REQ-DEF-XSI-NONAMESPACE"}, URL: "https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema-loc", Locator: "XML Schema 1.0 Second Edition Part 1 section 4.3.2; pairing consumer separate from datatype validity"}}},
		{validation.PackageCheckImageScaling, validation.PackageSubjectImage, "Image scaling advice", "Use exact encoded width/height and authorized positive image dimensions in their declared common unit. One dimension preserves proportion; two dimensions are independent. Missing metadata is optional unavailable advice; never infer pixel density, MusicXML tenths conversion or mandatory geometry.", "Image scaling advice was not established.", score(2326, "MX40-PROSE-image-scaling")},
		{validation.PackageCheckFileMetadataAdvice, validation.PackageSubjectMetadata, "MusicXML file and MIME conventions", "Expose supplied filename/MIME metadata against published .mxl/.musicxml and legacy .xml conventions. A suffix never proves actual resource kind or normative validity; metadata conventions remain optional advice.", "A recommended file or media-type convention was not established.", container("34", "MX40-PROSE-musicxml-file-types")},
	}
	var rules []validation.PackageRuleBinding
	for _, s := range specs {
		plan, err := validation.NewPackagePlan(s.check)
		if err != nil {
			return nil, err
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-PACKAGE-" + string(s.check)), Revision: 1}
		if s.check == validation.PackageCheckSchemaHintPairs {
			// Revision 2 applies the pairing to current model resources as
			// they would be encoded, instead of leaving them unknown.
			key.Revision = 2
		}
		if s.check == validation.PackageCheckScoreBeforeParts {
			// Revision 2 also decides the roles from the MusicXML link
			// semantics when no role provider decides them.
			key.Revision = 2
		}
		rule := validation.RuleDefinition{
			Key: key, Scope: validation.ScopePackage, Targets: []validation.Target{validation.TargetPackage},
			Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative,
			Predicate: string(key.ID), Sources: s.sources,
			Text: validation.RuleText{Title: s.title, Explanation: s.explanation, Message: s.message},
		}
		switch s.check {
		case validation.PackageCheckGroupMatch, validation.PackageCheckMimetypeAdvice, validation.PackageCheckEntryNameAdvice, validation.PackageCheckImageScaling, validation.PackageCheckFileMetadataAdvice:
			rule.Class, rule.Role, rule.Category = validation.ClassRecommendation, validation.RoleAdvisory, validation.CategoryAdvisory
		case validation.PackageCheckCompressionObservation:
			rule.Class, rule.Role, rule.Category = validation.ClassPolicy, validation.RoleContextProvider, validation.CategoryCapability
		case validation.PackageCheckSchemaHintPairs:
			rule.Class, rule.Role, rule.Category = validation.ClassPolicy, validation.RolePredicate, validation.CategoryCapability
		}
		rules = append(rules, validation.PackageRuleBinding{Rule: rule, Selector: validation.PackageSelector{Kind: s.subject}, Plan: plan})
	}
	return rules, nil
}
