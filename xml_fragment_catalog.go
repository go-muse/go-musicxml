package musicxml

import (
	"fmt"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

// electXMLFragmentBaseline changes only fresh caller-owned definitions. The
// caller names and composes its final profiles; published catalogs are never
// mutated. Invoke after source specialization when sourceInfoset is true.
func electXMLFragmentBaseline(d *assessmentDefinitions, sourceInfoset bool) error {
	if d == nil {
		return &validation.ConfigurationError{Field: "XML fragment definitions", Reason: "owned definition set required"}
	}
	replacements := make(map[validation.RuleKey]validation.RuleKey)
	selected := 0
	for i := range d.rules {
		r := &d.rules[i]
		var assembly validation.Assembly
		for _, candidate := range []validation.Assembly{validation.AssemblyMusicXML, validation.AssemblyOpus} {
			if r.Key.ID == validation.RuleID("MX40-RESOURCE-local-links/"+string(candidate)) {
				assembly = candidate
				break
			}
		}
		if assembly == "" {
			continue
		}
		if strings.HasSuffix(r.Predicate, "/xml-fragments-v1") {
			return &validation.ConfigurationError{Field: "XML fragment definitions", Reason: "baseline already elected"}
		}
		plan, err := validationNewXMLFragmentLinkContextPlan(d.schemas[assembly], sourceInfoset)
		if err != nil {
			return err
		}
		oldPredicate, oldKey := r.Predicate, r.Key
		r.Key.Revision++
		r.Predicate += "/xml-fragments-v1"
		r.Text.Explanation += " Resource snapshots elect the registered XML-media fragment baseline: XPointer Framework, shorthand and element(), with complete all-element child order and first-document-order identity selection. Unknown supported results block fallback; unsupported schemes are skipped. No XPath, namespace-binding scheme, external declaration retrieval or compressed-primary substitution is implied."
		r.Sources = append(r.Sources,
			validation.SourceRef{IDs: []string{"MX40-PROSE-xlink-href", "XLink11-target-media"}, URL: "https://www.w3.org/TR/2010/REC-xlink11-20100506/#link-locators", Locator: "Section 5.4: pinned MusicXML href reference assigns fragments to the target media type"},
			validation.SourceRef{IDs: []string{"IANA-MusicXML-XML-media"}, URL: "https://www.iana.org/assignments/media-types/application/vnd.recordare.musicxml+xml", Locator: "MusicXML XML media registration: +xml with no alternate fragment syntax"},
			validation.SourceRef{IDs: []string{"IANA-XML-suffix-fragments"}, URL: "https://www.iana.org/assignments/media-type-structured-suffix/media-type-structured-suffix.xhtml", Locator: "+xml suffix fragment identifier considerations reference RFC 7303"},
			validation.SourceRef{IDs: []string{"RFC7303-XML-fragments"}, URL: "https://www.rfc-editor.org/rfc/rfc7303#section-5", Locator: "Sections 5 and 9.6.1: Framework and mandatory element() for XML media and +xml"},
			validation.SourceRef{IDs: []string{"XPointer-Framework"}, URL: "https://www.w3.org/TR/2003/REC-xptr-framework-20030325/", Locator: "Sections 3.1-3.4 and 4.1: full syntax, first-owner shorthand, fallback and escaping"},
			validation.SourceRef{IDs: []string{"XPointer-element"}, URL: "https://www.w3.org/TR/2003/REC-xptr-element-20030325/", Locator: "Section 3: ID anchors, child-element ordinals and malformed-part empty result"})
		found := false
		for j := range d.plans {
			if d.plans[j].ID == oldPredicate {
				d.plans[j] = validation.PlanBinding{ID: r.Predicate, Plan: plan}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("musicxml: XML fragment link plan %s unavailable", oldPredicate)
		}
		replacements[oldKey] = r.Key
		selected++
	}
	if selected != 2 {
		return &validation.ConfigurationError{Field: "XML fragment definitions", Reason: "exact MusicXML and Opus link plans required"}
	}
	for i := range d.rules {
		for j := range d.rules[i].Requires {
			dep := &d.rules[i].Requires[j]
			if dep.Kind == validation.DependencyRule {
				if next, ok := replacements[dep.Rule]; ok {
					dep.Rule = next
				}
			}
		}
	}
	for i := range d.profiles {
		p := &d.profiles[i]
		elected := false
		for j, key := range p.Required {
			if next, ok := replacements[key]; ok {
				p.Required[j] = next
				elected = true
			}
		}
		for j := range p.Support {
			if next, ok := replacements[p.Support[j].Rule]; ok {
				p.Support[j].Rule = next
			}
		}
		if !elected {
			continue
		}
		p.NormativeClosure += " Direct XML resource fragments select the registered Framework, shorthand and element() baseline. All-element order is owned and bounded; text, comments and processing instructions do not count as child elements. Unsupported schemes are skipped, while unavailable supported results cannot be skipped. Existing source/current-model identity domains, independent XSD validity and direct-target indirection gates remain active. No document xml:base/xml:id, XPath or external declaration processing is introduced."
	}
	return nil
}
