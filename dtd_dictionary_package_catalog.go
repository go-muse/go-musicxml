package musicxml

import (
	"strings"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileResourceGraphInternalDTDDictionaries selects the bounded graph with
// explicitly DTD-enabled source children and supplied dictionary capabilities.
// Current model roots retain their existing model-only interpretation.
const ProfileResourceGraphInternalDTDDictionaries validation.ProfileID = "musicxml-4.0/resource-graph-internal-dtd-dictionaries"

// ProfileMXLSourceInternalDTDDictionaries selects original MXL assessment with
// bounded internal-DTD source interpretations, including its source container.
const ProfileMXLSourceInternalDTDDictionaries validation.ProfileID = "musicxml-4.0/mxl-source-internal-dtd-dictionaries"

// ProfileMXLModelInternalDTDDictionaries assesses the current primary model
// directly, with explicitly DTD-enabled retained source children. No original
// container, ZIP header, lexical spelling or DTD history is reconstructed.
const ProfileMXLModelInternalDTDDictionaries validation.ProfileID = "musicxml-4.0/mxl-model-internal-dtd-dictionaries"

func dtdDictionaryResourceIntrinsicProfiles() resourceIntrinsicProfiles {
	return resourceIntrinsicProfiles{source: ProfileSourceInternalDTDDictionaries, model: ProfileModelDictionaries, container: ProfileSourceInternalDTDContainer, sounds: ProfileSourceInternalDTDSoundsDictionaries}
}

var dtdDictionaryPackageAssessmentCatalog = sync.OnceValues(buildDTDDictionaryPackageAssessmentCatalog)

func buildDTDDictionaryPackageAssessmentCatalog() (*validation.CompiledPackageCatalog, error) {
	rules, profiles, err := buildDTDDictionaryPackageAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	return validation.NewPackageCatalog(rules, profiles)
}

func buildDTDDictionaryPackageAssessmentDefinitions() ([]validation.PackageRuleBinding, []validation.PackageProfileBinding, error) {
	// Do not append profiles to any previously compiled catalog: catalog digests
	// identify exact interpretations, including every selected intrinsic row.
	rules, profiles, err := buildPackageAssessmentDefinitions()
	if err != nil {
		return nil, nil, err
	}
	bundle := dtdDictionaryResourceIntrinsicProfiles()
	for i := range profiles {
		p := &profiles[i]
		switch p.Profile.ID {
		case ProfileResourceGraph:
			p.Profile.ID = ProfileResourceGraphInternalDTDDictionaries
		case ProfileMXLSource:
			p.Profile.ID = ProfileMXLSourceInternalDTDDictionaries
		case ProfileMXLModel:
			p.Profile.ID = ProfileMXLModelInternalDTDDictionaries
		}
		p.Profile.NormativeClosure = strings.Replace(p.Profile.NormativeClosure, "renderer, dictionary, automatic", "renderer, automatic", 1) + " Explicit composition adds bounded internal-DTD processing only to actually assessed source XML, using only actually-read internal declarations. Direct XML shorthand targets join actually read effective DTD IDs with independently established XSD IDs and select the first document-order owner, requiring complete elected identity domains before proving a miss or excluding an earlier candidate. Current-model identity remains current-state XSD-only; DTD declaration validity is not asserted. Exact source-origin snapshots retain independently qualified provenance through adoption without asserting complete origin evidence. Effective Container metadata and schema hints remain separate from physical Raw; effective Sounds mappings and supplied standard-sound recognition are supported only when explicitly assessed, with no auxiliary discovery. The current primary model keeps its existing model dictionary interpretation and cannot establish discarded source, DTD, original container or ZIP history. One root-owned dictionary set and account are shared by every intrinsic child. Pinned canonical membership, open-string sound recognition and partial IPA positive evidence preserve independent failures and unavailable capabilities. All existing package, ZIP, path, dependency, indirection, unread-external, unsupported-capability and incomplete-IPA gates remain active. No external declaration, schema or dictionary is fetched automatically."
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
				return nil, nil, &validation.ConfigurationError{Field: "DTD dictionary package intrinsic profile", Reason: "missing exact target/profile catalog interpretation"}
			}
			row.CatalogDigest = profile.CatalogDigest
		}
	}
	return rules, profiles, nil
}

// selectDictionaryPackageProfile is a closed entry-point dispatch. Neither input
// content nor profile suffixes may grant DTD parsing or auxiliary discovery.
func selectDictionaryPackageProfile(selected, ordinary validation.ProfileID) (validation.ProfileID, bool, error) {
	var composed, semantic validation.ProfileID
	switch ordinary {
	case ProfileResourceGraphDictionaries:
		composed = ProfileResourceGraphInternalDTDDictionaries
		semantic = ProfileResourceGraphSemantics
	case ProfileMXLSourceDictionaries:
		composed = ProfileMXLSourceInternalDTDDictionaries
		semantic = ProfileMXLSourceSemantics
	case ProfileMXLModelDictionaries:
		composed = ProfileMXLModelInternalDTDDictionaries
		semantic = ProfileMXLModelSemantics
	default:
		return "", false, &validation.ConfigurationError{Field: "graph profile", Reason: "unknown dictionary entry point"}
	}
	switch selected {
	case ordinary:
		return selected, false, nil
	case composed, semantic:
		return selected, true, nil
	default:
		return "", false, &validation.ConfigurationError{Field: "graph profile", Reason: "explicit entry-point compatible dictionary graph profile required"}
	}
}
