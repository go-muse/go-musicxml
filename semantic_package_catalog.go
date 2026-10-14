package musicxml

import (
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

var semanticPackageAssessmentCatalog = sync.OnceValues(buildSemanticPackageAssessmentCatalog)

func buildSemanticPackageAssessmentCatalog() (*validation.CompiledPackageCatalog, error) {
	rules, profiles, err := buildPackageAssessmentDefinitions()
	if err != nil {
		return nil, err
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
	bundle := semanticResourceIntrinsicProfiles()
	for i := range profiles {
		p := &profiles[i]
		switch p.Profile.ID {
		case ProfileResourceGraph:
			p.Profile.ID = ProfileResourceGraphSemantics
		case ProfileMXLSource:
			p.Profile.ID = ProfileMXLSourceSemantics
		case ProfileMXLModel:
			p.Profile.ID = ProfileMXLModelSemantics
		default:
			return nil, &validation.ConfigurationError{Field: "semantic package profile", Reason: "unexpected base profile"}
		}
		p.Profile.NormativeClosure = strings.Replace(p.Profile.NormativeClosure, "renderer, dictionary, automatic", "renderer, automatic", 1)
		p.Profile.NormativeClosure += " Exact intrinsic composition selects bounded encoded MusicXML semantics, local and timeline checks, barline and graphical-repeat proofs, caller-supplied canonical and IPA evidence, and the registered XML Framework/shorthand/element fragment interpretation. Source children use actually read internal declarations and owned source origins; current models retain only current-state authority. Source Container metadata distinguishes physical and effective values. No auxiliary discovery, dictionary auto-loading, external declaration fetch, renderer or desired-score authority is introduced. Missing capabilities, incomplete identity domains, compressed-target indirection and other original package limits remain explicit. No universal conformance claim is made."
		for j := range p.Intrinsic {
			row := &p.Intrinsic[j]
			switch row.Assembly {
			case validation.AssemblyContainer, validation.AssemblySounds:
				if row.Target != validation.TargetSource {
					return nil, &validation.ConfigurationError{Field: "semantic auxiliary target", Reason: "source target required"}
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
					return nil, &validation.ConfigurationError{Field: "semantic intrinsic target", Reason: "source or current-model target required"}
				}
			default:
				return nil, &validation.ConfigurationError{Field: "semantic intrinsic assembly", Reason: "unrecognized assembly"}
			}
			var engine *validation.Engine
			if row.Target == validation.TargetSource {
				engine, err = sourceAssessmentEngineForProfile(row.Profile)
			} else {
				engine, err = modelAssessmentEngineForProfile(row.Profile)
			}
			if err != nil {
				return nil, err
			}
			profile, ok := engine.Profile(row.Profile)
			if !ok || profile.Target != row.Target || profile.CatalogDigest == "" {
				return nil, &validation.ConfigurationError{Field: "semantic package intrinsic profile", Reason: "missing exact target/profile catalog interpretation"}
			}
			row.CatalogDigest = profile.CatalogDigest
		}
	}
	return validation.NewPackageCatalog(rules, profiles)
}
