package validation

import "testing"

// Every published property belongs to the family its namespace names, which
// the ContextFamilies documentation lists; an unknown property has none.
func TestContextFamilyOfProperties(t *testing.T) {
	for property, want := range map[ContextProperty]ContextFamilies{
		ContextPropertyTiming:              ContextFamilyTiming,
		ContextPrintObject:                 ContextFamilyLayout,
		ContextNoteheadFilled:              ContextFamilyLayout,
		"layout.opus-version":              ContextFamilyLayout,
		ContextPropertySpan:                ContextFamilySpans,
		ContextPropertySpanLyricSyllable:   ContextFamilySpans,
		ContextPropertySpanLineType:        ContextFamilySpans,
		ContextPropertyRepeat:              ContextFamilyRepeats,
		ContextPropertyNavigation:          ContextFamilyNavigation,
		ContextPropertyNavigationTraversal: ContextFamilyNavigation,
		"performance-figure-prefix":        ContextFamilyPerformance,
		ContextPropertySoundMapping:        ContextFamilySounds,
		ContextResourceImage:               ContextFamilyResources,
		ContextPropertyContainerMetadata:   ContextFamilyContainer,
		ContextPropertySourceOrigin:        ContextFamilySourceOrigins,
		ContextDictionaryCanonical:         ContextFamilyDictionaries,
		"unknown":                          0,
	} {
		if got := (ContextResult{Property: property}).Family(); got != want {
			t.Errorf("%s: family %b, want %b", property, got, want)
		}
	}
	if ContextFamilyAll != contextFamilyEnd-1 || ContextFamilyAll&ContextFamilyDictionaries == 0 {
		t.Fatalf("ContextFamilyAll = %b", ContextFamilyAll)
	}
}
