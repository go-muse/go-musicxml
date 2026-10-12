package validation

import "strings"

func cloneLocation(v Location) Location {
	v.Target = Target(strings.Clone(string(v.Target)))
	v.DocumentID = strings.Clone(v.DocumentID)
	v.Entry = strings.Clone(v.Entry)
	v.Path = strings.Clone(v.Path)
	if v.Position != nil {
		p := *v.Position
		p.CoordinateSpace = strings.Clone(p.CoordinateSpace)
		v.Position = &p
	}
	return v
}
func cloneLocations(v []Location) []Location {
	if v == nil {
		return nil
	}
	out := make([]Location, len(v))
	for i := range v {
		out[i] = cloneLocation(v[i])
	}
	return out
}
func cloneDecision(v Decision) Decision {
	v.Outcome = Outcome(strings.Clone(string(v.Outcome)))
	v.Rule = cloneKey(v.Rule)
	v.Subject.DocumentID = strings.Clone(v.Subject.DocumentID)
	v.Reason = Code(strings.Clone(string(v.Reason)))
	v.Expected = strings.Clone(v.Expected)
	v.Actual = strings.Clone(v.Actual)
	v.Related = cloneLocations(v.Related)
	if v.Cause != nil {
		c := *v.Cause
		c.Rule = cloneKey(c.Rule)
		c.Subject.DocumentID = strings.Clone(c.Subject.DocumentID)
		v.Cause = &c
	}
	return v
}
func cloneKey(v RuleKey) RuleKey { v.ID = RuleID(strings.Clone(string(v.ID))); return v }
func cloneDeclaration(v DeclarationID) DeclarationID {
	v.Assembly = Assembly(strings.Clone(string(v.Assembly)))
	v.OwnerOccurrence = strings.Clone(v.OwnerOccurrence)
	v.Occurrence = strings.Clone(v.Occurrence)
	return v
}
func cloneComponent(v ComponentID) ComponentID {
	v.Assembly = Assembly(strings.Clone(string(v.Assembly)))
	v.Kind = ComponentKind(strings.Clone(string(v.Kind)))
	v.Name = QName{strings.Clone(v.Name.Space), strings.Clone(v.Name.Local)}
	v.Occurrence = strings.Clone(v.Occurrence)
	return v
}
func cloneRule(v RuleDefinition) RuleDefinition {
	if v.Selector.Assembly != nil {
		assembly := *v.Selector.Assembly
		v.Selector.Assembly = &assembly
	}
	v.Class = RuleClass(strings.Clone(string(v.Class)))
	v.Role = RuleRole(strings.Clone(string(v.Role)))
	v.Category = DiagnosticCategory(strings.Clone(string(v.Category)))
	v.Scope = ScopeKind(strings.Clone(string(v.Scope)))
	v.Key = cloneKey(v.Key)
	v.Predicate = strings.Clone(v.Predicate)
	v.Text = RuleText{strings.Clone(v.Text.Title), strings.Clone(v.Text.Explanation), strings.Clone(v.Text.Message)}
	v.Targets = append([]Target(nil), v.Targets...)
	for i := range v.Targets {
		v.Targets[i] = Target(strings.Clone(string(v.Targets[i])))
	}
	v.Requires = append([]Dependency(nil), v.Requires...)
	for i := range v.Requires {
		v.Requires[i].Kind = DependencyKind(strings.Clone(string(v.Requires[i].Kind)))
		v.Requires[i].Relation = RelationKind(strings.Clone(string(v.Requires[i].Relation)))
		v.Requires[i].Key = strings.Clone(v.Requires[i].Key)
		v.Requires[i].Rule = cloneKey(v.Requires[i].Rule)
	}
	if v.Selector.Component != nil {
		c := cloneComponent(*v.Selector.Component)
		v.Selector.Component = &c
	}
	if v.Selector.Declaration != nil {
		d := cloneDeclaration(*v.Selector.Declaration)
		v.Selector.Declaration = &d
	}
	sources := make([]SourceRef, len(v.Sources))
	for i, s := range v.Sources {
		sources[i] = SourceRef{URL: strings.Clone(s.URL), Locator: strings.Clone(s.Locator), IDs: make([]string, len(s.IDs))}
		for j, id := range s.IDs {
			sources[i].IDs[j] = strings.Clone(id)
		}
	}
	v.Sources = sources
	return v
}
func cloneProfile(v Profile) Profile {
	v.Target = Target(strings.Clone(string(v.Target)))
	v.ID = ProfileID(strings.Clone(string(v.ID)))
	v.MusicXMLVersion = strings.Clone(v.MusicXMLVersion)
	v.CatalogDigest = strings.Clone(v.CatalogDigest)
	v.RegistryCommit = strings.Clone(v.RegistryCommit)
	v.NormativeClosure = strings.Clone(v.NormativeClosure)
	v.Required = append([]RuleKey(nil), v.Required...)
	for i := range v.Required {
		v.Required[i] = cloneKey(v.Required[i])
	}
	v.Support = append([]RuleSupport(nil), v.Support...)
	for i := range v.Support {
		v.Support[i].State = ImplementationState(strings.Clone(string(v.Support[i].State)))
		v.Support[i].Rule = cloneKey(v.Support[i].Rule)
		v.Support[i].Reason = Code(strings.Clone(string(v.Support[i].Reason)))
	}
	v.SupportedRoots = append([]QName(nil), v.SupportedRoots...)
	for i := range v.SupportedRoots {
		v.SupportedRoots[i] = QName{strings.Clone(v.SupportedRoots[i].Space), strings.Clone(v.SupportedRoots[i].Local)}
	}
	return v
}
func cloneEvidence[T any](v Evidence[T]) Evidence[T] {
	// Use the closed vocabulary's owned constants. This detaches even enum
	// substrings from caller backing storage without allocating for every fact.
	switch v.State {
	case EvidenceKnown:
		v.State = EvidenceKnown
	case EvidenceAbsent:
		v.State = EvidenceAbsent
	case EvidenceUnknown:
		v.State = EvidenceUnknown
	case EvidenceUnsupported:
		v.State = EvidenceUnsupported
	case EvidenceInvalid:
		v.State = EvidenceInvalid
	default:
		v.State = EvidenceState(strings.Clone(string(v.State)))
	}
	if v.Cause != nil {
		c := *v.Cause
		c.Rule = cloneKey(c.Rule)
		c.Subject.DocumentID = strings.Clone(c.Subject.DocumentID)
		v.Cause = &c
	}
	v.Reason = Code(strings.Clone(string(v.Reason)))
	return v
}
func cloneAtomic(v Atomic) Atomic {
	v.Kind = AtomicKind(strings.Clone(string(v.Kind)))
	v.Text = strings.Clone(v.Text)
	v.Name.Space = strings.Clone(v.Name.Space)
	v.Name.Local = strings.Clone(v.Name.Local)
	return v
}
func cloneScalar(v Scalar) Scalar {
	v.Lexeme = cloneEvidence(v.Lexeme)
	v.Lexeme.Value = strings.Clone(v.Lexeme.Value)
	v.Value = cloneEvidence(v.Value)
	v.Value.Value = cloneAtomic(v.Value.Value)
	return v
}
func cloneHeader(v NodeHeader) NodeHeader {
	v.Name.Space = strings.Clone(v.Name.Space)
	v.Name.Local = strings.Clone(v.Name.Local)
	v.Location = cloneLocation(v.Location)
	v.Type = cloneEvidence(v.Type)
	v.Type.Value = cloneComponent(v.Type.Value)
	v.Declaration = cloneEvidence(v.Declaration)
	v.Declaration.Value = cloneDeclaration(v.Declaration.Value)
	bindings := make([]Binding, len(v.Bindings))
	for i, b := range v.Bindings {
		bindings[i] = Binding{strings.Clone(b.Prefix), strings.Clone(b.URI)}
	}
	v.Bindings = bindings
	return v
}
func cloneAttribute(v AttributeFact) AttributeFact {
	v.Location = cloneLocation(v.Location)
	v.Declaration = cloneEvidence(v.Declaration)
	v.Declaration.Value = cloneDeclaration(v.Declaration.Value)
	v.Presence = cloneEvidence(v.Presence)
	v.Scalar = cloneScalar(v.Scalar)
	v.Name = QName{strings.Clone(v.Name.Space), strings.Clone(v.Name.Local)}
	return v
}
func cloneEnd(v NodeEnd) NodeEnd {
	v.Scalar = cloneScalar(v.Scalar)
	v.ChildrenComplete = cloneEvidence(v.ChildrenComplete)
	v.AttributesComplete = cloneEvidence(v.AttributesComplete)
	v.Order = cloneEvidence(v.Order)
	v.Order.Value = OrderKind(strings.Clone(string(v.Order.Value)))
	return v
}
func cloneContext(v ContextFact) ContextFact {
	v.Value = cloneEvidence(v.Value)
	v.Value.Value = cloneAtomic(v.Value.Value)
	v.Origins = cloneLocations(v.Origins)
	v.Providers = append([]RuleKey(nil), v.Providers...)
	for i := range v.Providers {
		v.Providers[i] = cloneKey(v.Providers[i])
	}
	return v
}
