package musicxml

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"

	"github.com/go-muse/go-musicxml/validation"
)

// This catalog binds frozen clauses to descriptors returned by the authoritative
// compiled schema. Names are used only to select and verify configuration-time
// handles; runtime plans contain exact assembly-qualified identities.
type localCatalogBuilder struct {
	schema *validation.CompiledSchema
	uses   []validation.SchemaUse
	rules  []validation.RuleDefinition
	plans  []validation.PlanBinding
}

func newLocalCatalogBuilder(schema *validation.CompiledSchema) *localCatalogBuilder {
	return &localCatalogBuilder{schema: schema, uses: schema.Uses()}
}

func localType(name string) validation.Selector {
	c := validation.ComponentID{Assembly: validation.AssemblyMusicXML, Kind: validation.ComponentType, Name: validation.QName{Local: name}}
	return validation.Selector{Component: &c}
}
func localDeclaration(d validation.DeclarationID) validation.Selector {
	return validation.Selector{Declaration: &d}
}
func localString(s string) validation.Atomic {
	return validation.Atomic{Kind: validation.AtomicString, Text: s}
}

func (b *localCatalogBuilder) ownedUses(owner, name string, attribute bool) []validation.SchemaUse {
	var out []validation.SchemaUse
	for _, u := range b.uses {
		if u.Attribute == attribute && u.Name == (validation.QName{Local: name}) && u.OwnerTypeKnown && u.OwnerType.Name == (validation.QName{Local: owner}) {
			out = append(out, u)
		}
	}
	return out
}
func (b *localCatalogBuilder) children(owner string, names ...string) ([]validation.DeclarationID, error) {
	var out []validation.DeclarationID
	for _, name := range names {
		uses := b.ownedUses(owner, name, false)
		if len(uses) == 0 {
			return nil, fmt.Errorf("musicxml: local catalog: missing exact %s/%s use", owner, name)
		}
		for _, u := range uses {
			out = append(out, u.Declaration)
		}
	}
	return out, nil
}
func (b *localCatalogBuilder) attribute(owner, name string) (validation.AttributeSelector, error) {
	uses := b.ownedUses(owner, name, true)
	if len(uses) != 1 {
		return validation.AttributeSelector{}, fmt.Errorf("musicxml: local catalog: %s/@%s has %d exact uses", owner, name, len(uses))
	}
	return validation.AttributeSelector{Name: uses[0].Name, Declaration: uses[0].Declaration}, nil
}
func (b *localCatalogBuilder) add(id, suffix, title, explanation, source string, selector validation.Selector, plan validation.Plan, err error, extraIDs ...string) error {
	if err != nil {
		return err
	}
	key := validation.RuleKey{ID: validation.RuleID("MX40-LOCAL-" + id + suffix), Revision: 1}
	ids := []string{"MX40-PROSE-" + id}
	r := validation.RuleDefinition{Key: key, Selector: selector, Scope: validation.ScopeNode,
		Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative,
		Predicate: string(key.ID), Text: validation.RuleText{Title: title, Explanation: explanation, Message: "The selected bounded local MusicXML requirement was not satisfied."},
		Sources: []validation.SourceRef{{IDs: ids, URL: source, Locator: "Frozen local clause; exact declaration-use binding"}},
	}
	for _, extra := range extraIDs {
		if extra != "MX40-PROSE-midi-requires-score-instrument" {
			return fmt.Errorf("musicxml: unknown shared local provenance %s", extra)
		}
		r.Sources = append(r.Sources, validation.SourceRef{IDs: []string{extra}, URL: localSchemaSource(6048), Locator: "Corresponding instrument existence shares typed assignment consequence"})
	}
	b.rules = append(b.rules, r)
	b.plans = append(b.plans, validation.PlanBinding{ID: r.Predicate, Plan: plan})
	return nil
}
func localSchemaSource(line int) string {
	return "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L" + strconv.Itoa(line)
}

func (b *localCatalogBuilder) bindLocalRelations() error {
	children, err := b.children("accordion-registration", "accordion-high", "accordion-middle", "accordion-low")
	if err != nil {
		return err
	}
	p, err := validation.NewChildCountPlan(children, 1, -1)
	if err := b.add("accordion-at-least-one", "", "Accordion register presence", "Accordion registration must contain at least one high, middle or low register child; each child retains its own XSD value and occurrence constraints.", localSchemaSource(3290), localType("accordion-registration"), p, err); err != nil {
		return err
	}

	children, err = b.children("note", "beam")
	if err != nil {
		return err
	}
	a, err := b.attribute("beam", "number")
	if err != nil {
		return err
	}
	p, err = validation.NewDistinctChildAttributePlan(children, a)
	if err := b.add("beam-number-distinct", "", "Distinct beam numbers", "Within one note, beam effective numbers must be distinct. A proven absent number uses the schema default 1; no contiguous-level or six-level restriction is added.", "https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/beam/", localType("note"), p, err); err != nil {
		return err
	}

	for _, owner := range []string{"slur", "tied"} {
		var triggers []validation.AttributeSelector
		for _, name := range []string{"bezier-x2", "bezier-y2", "bezier-offset2"} {
			a, e := b.attribute(owner, name)
			if e != nil {
				return e
			}
			triggers = append(triggers, a)
		}
		a, err = b.attribute(owner, "type")
		if err != nil {
			return err
		}
		p, err = validation.NewPresenceImpliesAttributePlan(triggers, localType("slur"), a, localString("continue"))
		if err := b.add("bezier-second-control", "/"+owner, "Second Bézier controls", "Any second-control attribute requires a slur with type continue. Tied never qualifies; the controls need not all be present together.", localSchemaSource(1948), localType(owner), p, err); err != nil {
			return err
		}
	}

	markers, err := b.children("defaults", "concert-score")
	if err != nil {
		return err
	}
	p, err = validation.NewDocumentChildPresencePlan(markers)
	if err := b.add("for-part-requires-concert", "", "Concert score for part transposition", "A for-part element requires an explicit concert-score marker in this document's defaults.", localSchemaSource(2877), localType("for-part"), p, err); err != nil {
		return err
	}

	children, err = b.children("key", "cancel")
	if err != nil {
		return err
	}
	a, err = b.attribute("key-octave", "cancel")
	if err != nil {
		return err
	}
	p, err = validation.NewAttributeImpliesRelatedChildPlan(a, localString("yes"), validation.RelationParent, children)
	if err := b.add("key-octave-cancel-exists", "", "Key cancellation presence", "A key-octave with cancel=yes requires a cancel child in the same key. Glyph ordering and number indexing are separate obligations.", localSchemaSource(3025), localType("key-octave"), p, err); err != nil {
		return err
	}

	uses := b.ownedUses("metronome", "metronome-relation", false)
	if len(uses) == 0 {
		return fmt.Errorf("musicxml: local catalog: missing metronome-relation use")
	}
	for i, u := range uses {
		p, err = validation.NewScalarOneOfPlan([]validation.Atomic{localString(""), localString("equals")})
		suffix := ""
		if len(uses) > 1 {
			suffix = "/use-" + strconv.Itoa(i+1)
		}
		if err := b.add("metronome-relation-value", suffix, "Metronome relation value", "The current metronome relation string must be exactly empty or equals. Whitespace and case are significant.", localSchemaSource(3821), localDeclaration(u.Declaration), p, err); err != nil {
			return err
		}
	}
	for _, spec := range []struct{ id, attribute string }{{"sound-dacapo-yes", "dacapo"}, {"sound-forward-repeat-yes", "forward-repeat"}} {
		a, err = b.attribute("sound", spec.attribute)
		if err != nil {
			return err
		}
		p, err = validation.NewPresentAttributeValuePlan(a, []validation.Atomic{localString("yes")})
		if err := b.add(spec.id, "", "Sound navigation flag", "When this sound attribute is present its validated yes-no value must be yes. No is a local prose violation; omission is not. This does not execute playback navigation.", localSchemaSource(4125), localType("sound"), p, err); err != nil {
			return err
		}
	}
	zero, err := validation.NewDecimal(big.NewRat(0, 1))
	if err != nil {
		return err
	}
	hundred, err := validation.NewDecimal(big.NewRat(100, 1))
	if err != nil {
		return err
	}
	for _, name := range []string{"damper-pedal", "soft-pedal", "sostenuto-pedal"} {
		a, err = b.attribute("sound", name)
		if err != nil {
			return err
		}
		p, err = validation.NewAttributeNumberOrTokensPlan(a, []string{"yes", "no"}, zero, hundred)
		if err := b.add("sound-pedal-range", "/"+name, "Sound pedal range", "After shared ordered union validation, this pedal accepts yes/no or an exact decimal from 0 through 100 inclusive. It does not parse or guess a union member independently.", localSchemaSource(4125), localType("sound"), p, err); err != nil {
			return err
		}
	}
	chromatic, err := b.children("transpose", "chromatic")
	if err != nil {
		return err
	}
	diatonic, err := b.children("transpose", "diatonic")
	if err != nil {
		return err
	}
	integerZero, err := validation.NewInteger(big.NewInt(0))
	if err != nil {
		return err
	}
	var requirements []validation.ChildValueRequirement
	for _, d := range chromatic {
		requirements = append(requirements, validation.ChildValueRequirement{Child: d, Expected: validation.Atomic{Kind: validation.AtomicDecimal, Decimal: zero}})
	}
	for _, d := range diatonic {
		requirements = append(requirements, validation.ChildValueRequirement{Child: d, Expected: validation.Atomic{Kind: validation.AtomicInteger, Integer: integerZero}, AllowAbsent: true})
	}
	uses = b.ownedUses("attributes", "transpose", false)
	if len(uses) == 0 {
		return fmt.Errorf("musicxml: local catalog: missing attributes/transpose use")
	}
	for i, u := range uses {
		p, err = validation.NewDocumentMarkerValuesPlan(markers, requirements)
		suffix := ""
		if len(uses) > 1 {
			suffix = "/use-" + strconv.Itoa(i+1)
		}
		if err := b.add("concert-score-transpose", suffix, "Concert-score transposition", "With this document's concert-score marker, attributes/transpose requires chromatic zero and absent or zero diatonic. Octave-change and double remain allowed; for-part/part-transpose is excluded.", localSchemaSource(5871), localDeclaration(u.Declaration), p, err); err != nil {
			return err
		}
	}
	return nil
}

func (b *localCatalogBuilder) namedElementUses(name string) []validation.SchemaUse {
	var uses []validation.SchemaUse
	for _, u := range b.uses {
		if !u.Attribute && u.Name == (validation.QName{Local: name}) {
			uses = append(uses, u)
		}
	}
	sort.Slice(uses, func(i, j int) bool { return uses[i].Declaration.Occurrence < uses[j].Declaration.Occurrence })
	return uses
}
func (b *localCatalogBuilder) typeAttribute(owner validation.ComponentID, name string) (validation.AttributeSelector, error) {
	var found []validation.SchemaUse
	for _, u := range b.uses {
		if u.Attribute && u.OwnerTypeKnown && u.OwnerType == owner && u.Name == (validation.QName{Local: name}) {
			found = append(found, u)
		}
	}
	if len(found) != 1 {
		return validation.AttributeSelector{}, fmt.Errorf("musicxml: local catalog: anonymous attribute %s has %d uses", name, len(found))
	}
	return validation.AttributeSelector{Name: found[0].Name, Declaration: found[0].Declaration}, nil
}

func (b *localCatalogBuilder) bindTypedReferences() error {
	scoreParts, err := b.children("part-list", "score-part")
	if err != nil {
		return err
	}
	instruments, err := b.children("score-part", "score-instrument")
	if err != nil {
		return err
	}
	players, err := b.children("score-part", "player")
	if err != nil {
		return err
	}
	partLists := b.namedElementUses("part-list")
	if len(partLists) != 2 {
		return fmt.Errorf("musicxml: local catalog: expected two score-root part-list uses, got %d", len(partLists))
	}
	var listDeclarations []validation.DeclarationID
	for _, u := range partLists {
		listDeclarations = append(listDeclarations, u.Declaration)
	}
	parts := b.namedElementUses("part")
	if len(parts) != 2 {
		return fmt.Errorf("musicxml: local catalog: expected two score part uses, got %d", len(parts))
	}
	var owners []validation.ReferenceOwner
	for i, u := range parts {
		if !u.DeclaredTypeKnown {
			return fmt.Errorf("musicxml: local catalog: part type unavailable")
		}
		a, err := b.typeAttribute(u.DeclaredType, "id")
		if err != nil {
			return err
		}
		suffix := "/use-" + strconv.Itoa(i+1)
		key := validation.RuleKey{ID: validation.RuleID("MX40-LOCAL-part-id-target" + suffix), Revision: 1}
		p, err := validation.NewTypedReferencePlan(validation.ReferencePlanSpec{Attribute: a, Targets: scoreParts, TargetAncestors: listDeclarations})
		if err := b.add("part-id-target", suffix, "Score-part reference target", "A part IDREF must target a unique score-part in this document's part-list. That reference defines part identity; repeated timewise references are permitted.", localSchemaSource(2407), localDeclaration(u.Declaration), p, err); err != nil {
			return err
		}
		owners = append(owners, validation.ReferenceOwner{Declaration: u.Declaration, Attribute: a, Rule: key})
	}
	for _, spec := range []struct {
		id, owner, attribute string
		line                 int
		player               bool
	}{
		{"midi-device-target", "midi-device", "id", 2664, false},
		{"midi-instrument-target", "midi-instrument", "id", 2676, false},
		{"instrument-change-target", "instrument-change", "id", 3687, false},
		{"instrument-note-target", "instrument", "id", 5037, false},
		{"play-instrument-target", "play", "id", 4125, false},
		{"assess-player-target", "assess", "player", 4699, true},
		{"instrument-link-target", "instrument-link", "id", 5928, false},
	} {
		a, err := b.attribute(spec.owner, spec.attribute)
		if err != nil {
			return err
		}
		targets := instruments
		if spec.player {
			targets = players
		}
		p, err := validation.NewTypedReferencePlan(validation.ReferencePlanSpec{Attribute: a, Targets: targets, Owners: scoreParts, OwnerReferences: owners})
		extra := []string(nil)
		if spec.id == "midi-instrument-target" {
			extra = []string{"MX40-PROSE-midi-requires-score-instrument"}
		}
		explanation := "The present validated reference must resolve uniquely to the required target kind in the owning or matching score-part. Generic missing, invalid and ambiguous references block this extra clause with their existing cause; proven optional omission is not an invalid reference."
		suffix := ""
		if spec.id == "instrument-link-target" {
			suffix = "/local"
			explanation = "The local instrument-link IDREF must target a unique score-instrument in its owning score-part. This local-only clause does not establish linked-file existence, content or instrument association, and never assumes identical IDs across documents."
		}
		source := localSchemaSource(spec.line)
		if spec.id == "play-instrument-target" {
			source = "https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/play/#attributes"
		}
		if err := b.add(spec.id, suffix, "Typed and scoped reference", explanation, source, localType(spec.owner), p, err, extra...); err != nil {
			return err
		}
	}
	return nil
}
