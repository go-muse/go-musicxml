package musicxml

import (
	"context"
	"strings"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileResourceGraphSemantics composes bounded encoded semantic assessments
// for source and current-model resources, retaining their distinct authorities.
const ProfileResourceGraphSemantics validation.ProfileID = "musicxml-4.0/resource-graph-semantics"

// ProfileMXLSourceSemantics adds the encoded semantic interpretation to original
// MXL assessment without granting transport or external-declaration authority.
const ProfileMXLSourceSemantics validation.ProfileID = "musicxml-4.0/mxl-source-semantics"

// ProfileMXLModelSemantics assesses a current package and retained source children
// without reconstructing original ZIP, container or discarded source history.
const ProfileMXLModelSemantics validation.ProfileID = "musicxml-4.0/mxl-model-semantics"

// ProfileMXLSourceLine is validation line 2 of strict MXL loading: the
// original archive bytes that the converted MXLPackage does not keep (the
// container path, the mimetype entry, entry names and compression), the
// first rootfile's media type as the original container wrote it, and the
// original schema hints, with the first rootfile document, the one conversion
// decodes, assessed by ProfileSourceLine. DecodeMXLAssessed runs it on the
// archive it reads; line 3, ProfileMXLModelSemantics, checks the converted
// package, including the files it retains as bytes. Line 2 asks the caller's
// resolver for nothing.
const ProfileMXLSourceLine validation.ProfileID = "musicxml-4.0/mxl-source-line"

func isSemanticPackageProfile(id validation.ProfileID) bool {
	switch id {
	case ProfileResourceGraphSemantics, ProfileMXLSourceSemantics, ProfileMXLModelSemantics:
		return true
	}
	return false
}
func semanticResourceIntrinsicProfiles() resourceIntrinsicProfiles {
	return resourceIntrinsicProfiles{source: ProfileSourceSemantics, model: ProfileModelSemantics, container: ProfileSourceInternalDTDContainer, sounds: ProfileSourceInternalDTDSoundsDictionaries}
}

// semanticPackageIntrinsicProfiles returns the intrinsic profiles of a
// semantic package profile: line 2 assesses its documents with
// ProfileSourceLine.
func semanticPackageIntrinsicProfiles(id validation.ProfileID) resourceIntrinsicProfiles {
	bundle := semanticResourceIntrinsicProfiles()
	if id == ProfileMXLSourceLine {
		bundle.source = ProfileSourceLine
	}
	return bundle
}

// startSourceLineResourceGraph starts validation line 2 of strict MXL
// loading. ProfileMXLSourceLine reads no dictionaries, so the graph imports
// none; account may be nil.
func startSourceLineResourceGraph(ctx context.Context, resources LinkedAssessmentOptions, account *validation.Account) (*resourceGraphRun, error) {
	cfg, err := validateLinkedAssessmentOptionsAccount(ctx, resources, ProfileMXLSourceLine, account)
	if err != nil {
		return nil, err
	}
	// The caller's resources were validated above, but line 2 has no rule
	// that consumes resolved documents, roles, associations or images: line 3
	// asks the caller's resolver for them.
	cfg.resolver, cfg.selected = nil, nil
	catalog, err := semanticPackageAssessmentCatalog()
	if err != nil {
		return nil, err
	}
	return startConfiguredResourceGraph(ctx, cfg, ProfileMXLSourceLine, catalog, semanticPackageIntrinsicProfiles(ProfileMXLSourceLine))
}

// sourceLinePackageCheck reports whether a package check reads original
// archive bytes, the first rootfile's media type as the original container
// wrote it or original schema hints, which only line 2 sees.
func sourceLinePackageCheck(id validation.RuleID) bool {
	for _, check := range []validation.PackageCheck{validation.PackageCheckContainerPath, validation.PackageCheckFirstMediaType, validation.PackageCheckMimetypeStored, validation.PackageCheckMimetypeNoExtra, validation.PackageCheckMimetypeNoPrefix, validation.PackageCheckMimetypeAdvice, validation.PackageCheckEntryNameAdvice, validation.PackageCheckCompressionObservation, validation.PackageCheckSchemaHintPairs} {
		if id == validation.RuleID("MX40-PACKAGE-"+string(check)) {
			return true
		}
	}
	return false
}

var semanticPackageAssessmentCatalog = sync.OnceValues(buildSemanticPackageAssessmentCatalog)

func buildSemanticPackageAssessmentCatalog() (*validation.CompiledPackageCatalog, error) {
	rules, profiles, err := semanticPackageAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	return validation.NewPackageCatalog(rules, profiles)
}

// semanticPackageAssessmentDefinitions composes the semantic package profiles
// and binds each intrinsic row to its engine's current catalog digest.
func semanticPackageAssessmentDefinitions() ([]validation.PackageRuleBinding, []validation.PackageProfileBinding, error) {
	rules, profiles, err := buildPackageAssessmentDefinitions()
	if err != nil {
		return nil, nil, err
	}
	for i := range rules {
		r := &rules[i].Rule
		if r.Key.ID != "MX40-PACKAGE-link-target" {
			continue
		}
		r.Text.Explanation += " In this composition, exact intrinsic snapshot capabilities supply the registered Framework, shorthand and element() evidence; current-model IDs remain XSD-only, while source identity joins effective DTD and XSD authority."
		for j := range r.Sources {
			if r.Sources[j].URL == "https://www.w3.org/TR/2003/REC-xptr-framework-20030325/#shorthand" {
				r.Sources[j].Locator = "Sections 3.1-3.4: selected Framework and first-owner shorthand; source/current-model identity domains remain distinct"
			}
		}
		r.Sources = append(r.Sources, validation.SourceRef{IDs: []string{"XPointer-element"}, URL: "https://www.w3.org/TR/2003/REC-xptr-element-20030325/", Locator: "Section 3: selected element-child addressing from exact intrinsic snapshot capability"})
	}
	for i := range profiles {
		if profiles[i].Profile.ID == ProfileMXLSource {
			line := profiles[i]
			line.Profile = compositionProfileCopy(line.Profile)
			line.Profile.ID = ProfileMXLSourceLine
			// Line 2 follows no resource links, so it reaches no Sounds
			// document; the Sounds row would bind a dictionary profile.
			line.Intrinsic = nil
			for _, row := range profiles[i].Intrinsic {
				if row.Assembly != validation.AssemblySounds {
					line.Intrinsic = append(line.Intrinsic, row)
				}
			}
			profiles = append(profiles, line)
			break
		}
	}
	for i := range profiles {
		p := &profiles[i]
		bundle := semanticPackageIntrinsicProfiles(p.Profile.ID)
		switch p.Profile.ID {
		case ProfileResourceGraph:
			p.Profile.ID = ProfileResourceGraphSemantics
		case ProfileMXLSourceLine:
		case ProfileMXLSource:
			p.Profile.ID = ProfileMXLSourceSemantics
		case ProfileMXLModel:
			p.Profile.ID = ProfileMXLModelSemantics
		default:
			return nil, nil, &validation.ConfigurationError{Field: "semantic package profile", Reason: "unexpected base profile"}
		}
		p.Profile.NormativeClosure = strings.Replace(p.Profile.NormativeClosure, "renderer, dictionary, automatic", "renderer, automatic", 1)
		p.Profile.NormativeClosure += " Exact intrinsic composition selects bounded encoded MusicXML semantics, local and timeline checks, barline and graphical-repeat proofs, caller-supplied canonical and IPA evidence, and the registered XML Framework/shorthand/element fragment interpretation. Source children use actually read internal declarations, a DOCTYPE external subset read from the bundled official MusicXML DTDs or a caller DTD source and owned source origins; current models retain only current-state authority. Source Container metadata distinguishes physical and effective values. No auxiliary discovery, dictionary auto-loading, external declaration fetch, renderer or desired-score authority is introduced. Missing capabilities, incomplete identity domains, compressed-target indirection and other original package limits remain explicit. No universal conformance claim is made."
		if p.Profile.ID == ProfileMXLSourceLine {
			p.Profile.NormativeClosure = "Validation line 2 of strict MXL loading: the bounded original raw-MXL archive facts that a converted MXLPackage does not keep. Original local/central headers, mimetype storage, extra fields, leading bytes and content advice, UTF-8 entry names, the container path and observed compression remain source evidence, the first rootfile's media type is decided as the original container wrote it, including absent, empty and DTD-defaulted values, and original schema hints are paired. The first rootfile document, which conversion decodes, is assessed with the source line, and the original container with its bounded DTD/effective metadata. Later rootfiles and resource links are not followed and no dictionary is read: line 3 assesses the converted package, including the files it keeps as bytes. Complete ZIP/JAR/DEFLATE normative closure and other original package limits remain unresolved when reached. No universal conformance claim is made."
			required := p.Profile.Required[:0]
			for _, key := range p.Profile.Required {
				if sourceLinePackageCheck(key.ID) {
					required = append(required, key)
				}
			}
			support := p.Profile.Support[:0]
			for _, s := range p.Profile.Support {
				if sourceLinePackageCheck(s.Rule.ID) {
					support = append(support, s)
				}
			}
			p.Profile.Required, p.Profile.Support = required, support
		}
		for j := range p.Intrinsic {
			row := &p.Intrinsic[j]
			switch row.Assembly {
			case validation.AssemblyContainer, validation.AssemblySounds:
				if row.Target != validation.TargetSource {
					return nil, nil, &validation.ConfigurationError{Field: "semantic auxiliary target", Reason: "source target required"}
				}
				row.Profile = bundle.container
				if row.Assembly == validation.AssemblySounds {
					row.Profile = bundle.sounds
				}
			case validation.AssemblyMusicXML, validation.AssemblyOpus:
				switch row.Target {
				case validation.TargetSource:
					row.Profile = bundle.source
				case validation.TargetModel:
					row.Profile = bundle.model
				default:
					return nil, nil, &validation.ConfigurationError{Field: "semantic intrinsic target", Reason: "source or current-model target required"}
				}
			default:
				return nil, nil, &validation.ConfigurationError{Field: "semantic intrinsic assembly", Reason: "unrecognized assembly"}
			}
			var engine *validation.Engine
			if row.Target == validation.TargetSource {
				engine, err = sourceAssessmentEngineForProfile(row.Profile)
			} else {
				engine, err = modelAssessmentEngineForProfile(row.Profile)
			}
			if err != nil {
				return nil, nil, err
			}
			profile, ok := engine.Profile(row.Profile)
			if !ok || profile.Target != row.Target || profile.CatalogDigest == "" {
				return nil, nil, &validation.ConfigurationError{Field: "semantic package intrinsic profile", Reason: "missing exact target/profile catalog interpretation"}
			}
			row.CatalogDigest = profile.CatalogDigest
		}
	}
	return rules, profiles, nil
}
