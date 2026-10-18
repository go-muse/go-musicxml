package musicxml

import "github.com/go-muse/go-musicxml/validation"

// requiredDictionarySelections are the dictionary relations that the semantic
// profiles require: canonical SMuFL names and IPA representation.
func requiredDictionarySelections() []validation.DictionarySelection {
	return []validation.DictionarySelection{
		{Manifest: validation.DictionaryManifestSMuFL14Canonical, Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName}},
		{Manifest: validation.DictionaryManifestIPARepresentation2015Unicode13, Relations: []validation.DictionaryRelation{validation.DictionaryRelationIPARepresentation}},
	}
}

// requiredDictionaryOptions imports the required relations from the bundled
// pinned data.
func requiredDictionaryOptions() DictionaryOptions {
	return DictionaryOptions{Source: BundledDictionarySource(), Selections: requiredDictionarySelections()}
}

// scenario resolves options for an entry point whose empty Profile selects the
// full semantic profile. It reports whether the dictionary path applies: an
// empty Profile or explicit Dictionaries. With an empty Profile, nil
// Dictionaries or a value with a nil Source and no selections selects the
// bundled required data, as DecodeAssessed does. The returned options have no
// Dictionaries.
func (o AssessmentOptions) scenario(semantic validation.ProfileID) (AssessmentOptions, DictionaryOptions, bool) {
	dictionaries := o.Dictionaries
	o.Dictionaries = nil
	if o.Profile == "" {
		o.Profile = semantic
		if dictionaries == nil || dictionaries.Source == nil && len(dictionaries.Selections) == 0 {
			required := requiredDictionaryOptions()
			dictionaries = &required
		}
	}
	if dictionaries == nil {
		return o, DictionaryOptions{}, false
	}
	return o, *dictionaries, true
}
