package musicxml

import (
	"strings"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileResourceGraphDictionaries selects the bounded resource graph with
// dictionary-enabled source/current-model intrinsic profiles. It grants no
// transport authority and does not enable internal-DTD or full IPA assessment.
const ProfileResourceGraphDictionaries validation.ProfileID = "musicxml-4.0/resource-graph-dictionaries"

// ProfileMXLSourceDictionaries selects original-MXL package checks and exact
// source dictionary interpretations. Original container assessment is unchanged.
const ProfileMXLSourceDictionaries validation.ProfileID = "musicxml-4.0/mxl-source-dictionaries"

// ProfileMXLModelDictionaries selects current-MXL package checks with a directly
// assessed current primary model and dictionary-enabled retained source children.
// It never reconstructs discarded source or original ZIP evidence.
const ProfileMXLModelDictionaries validation.ProfileID = "musicxml-4.0/mxl-model-dictionaries"

func dictionaryResourceIntrinsicProfiles() resourceIntrinsicProfiles {
	return resourceIntrinsicProfiles{source: ProfileSourceDictionaries, model: ProfileModelDictionaries, container: ProfileSourceContainer, sounds: ProfileSourceSoundsDictionaries}
}

var dictionaryPackageAssessmentCatalog = sync.OnceValues(buildDictionaryPackageAssessmentCatalog)

func newDictionaryPackageAssessmentCatalog() (*validation.CompiledPackageCatalog, error) {
	return dictionaryPackageAssessmentCatalog()
}

func buildDictionaryPackageAssessmentCatalog() (*validation.CompiledPackageCatalog, error) {
	rules, profiles, err := buildDictionaryPackageAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	return validation.NewPackageCatalog(rules, profiles)
}

func buildDictionaryPackageAssessmentDefinitions() ([]validation.PackageRuleBinding, []validation.PackageProfileBinding, error) {
	// Fresh ordinary definitions own all their slices. The compiled ordinary
	// catalog and its profile/digest/serialization contracts remain untouched.
	rules, profiles, err := buildPackageAssessmentDefinitions()
	if err != nil {
		return nil, nil, err
	}
	bundle := dictionaryResourceIntrinsicProfiles()
	for i := range profiles {
		p := &profiles[i]
		switch p.Profile.ID {
		case ProfileResourceGraph:
			p.Profile.ID = ProfileResourceGraphDictionaries
		case ProfileMXLSource:
			p.Profile.ID = ProfileMXLSourceDictionaries
		case ProfileMXLModel:
			p.Profile.ID = ProfileMXLModelDictionaries
		}
		p.Profile.NormativeClosure = strings.Replace(p.Profile.NormativeClosure, "renderer, dictionary, automatic", "renderer, automatic", 1) + " Selected dictionary intrinsic profiles add exact pinned SMuFL membership, independently retained MusicXML family constraints, local defaults and bounded range advice, open-string standard-sound recognition and explicitly partial IPA positive evidence. Missing capabilities remain unavailable without suppressing independent failures. Full IPA sequence, font rendering, internal-DTD and source-origin composition are outside this closure. A root imports only explicitly selected components once, sharing the verified set and finite account with every intrinsic child."
		for j := range p.Intrinsic {
			row := &p.Intrinsic[j]
			switch row.Assembly {
			case validation.AssemblyContainer:
				row.Profile = bundle.container
			case validation.AssemblySounds:
				row.Profile = bundle.sounds
			default:
				if row.Target == validation.TargetSource {
					row.Profile = bundle.source
				} else {
					row.Profile = bundle.model
				}
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
				return nil, nil, &validation.ConfigurationError{Field: "dictionary package intrinsic profile", Reason: "missing exact target/profile catalog interpretation"}
			}
			row.CatalogDigest = profile.CatalogDigest
		}
	}
	return rules, profiles, nil
}
