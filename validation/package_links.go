package validation

import (
	"context"
	"github.com/go-muse/go-musicxml/internal/xsd"
	"strconv"
)

// ObserveEdge binds one finalized local link template to exact adopted target
// evidence. Caller resolution cannot turn an unavailable local prerequisite into
// success or use an unrelated template. The target must be a fresh nonroot
// occurrence, but may belong to another admitted package boundary.
func (s *PackageSession) ObserveEdge(ctx context.Context, e PackageEdgeObservation) error {
	if err := s.event(ctx, "ObserveEdge"); err != nil {
		return err
	}
	source := s.adoptions[e.Source]
	if source == nil || e.Template >= uint64(len(source.resource.snapshot.contexts)) {
		return s.fail(contract("package edge", "exact source adoption/template required"))
	}
	if s.boundaries[source.packageID].sealed || source.expansion.owner != source.id || source.expansion.sealed {
		return s.fail(contract("edge package", "source scope already sealed"))
	}
	template := source.resource.snapshot.contexts[e.Template]
	if template.Resource == nil || template.Resource.Link == nil || template.Resource.Link.Kind == ResourceBookmark {
		return s.fail(contract("package edge", "selected outgoing link template required"))
	}
	for _, id := range s.edgeOrder {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		old := s.edges[id]
		if old.Source == e.Source && old.Template == e.Template {
			return s.fail(contract("edge template", "one selected edge per adopted template"))
		}
	}
	if err := validateEvidence(e.Target, "edge target"); err != nil {
		return s.fail(err)
	}
	if err := validateEvidence(e.Fragment, "edge fragment"); err != nil {
		return s.fail(err)
	}
	if err := validateEvidence(e.FragmentTarget, "edge fragment target"); err != nil {
		return s.fail(err)
	}
	if e.Fragment.State == EvidenceKnown && !validText(e.Fragment.Value, 4096) {
		return s.fail(contract("edge fragment", "bounded exact text required"))
	}
	if e.Target.State == EvidenceKnown {
		target := s.adoptions[e.Target.Value]
		if target == nil {
			return s.fail(contract("edge target", "unknown adoption"))
		}
		if target.root || target.id == source.id {
			return s.fail(contract("edge target", "fresh nonroot adoption required"))
		}
		for _, id := range s.edgeOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			if err := s.event(ctx, "edge adoption scan"); err != nil {
				return err
			}
			old := s.edges[id]
			if old.Target.State == EvidenceKnown && old.Target.Value == e.Target.Value {
				return s.fail(contract("edge target", "target adoption already belongs to an edge"))
			}
		}
		if e.FragmentTarget.State == EvidenceKnown {
			if e.Fragment.State != EvidenceKnown {
				return s.fail(contract("fragment target", "known fragment component required"))
			}
			if e.Fragment.Value == "" && (e.FragmentTarget.Value.Node == 0 || e.FragmentTarget.Value.Node != target.resource.snapshot.header.Root.Node) {
				return s.fail(contract("fragment target", "empty fragment addresses exact adopted root"))
			}
			if e.FragmentTarget.Value.DocumentID != target.documentID || !s.adoptedNodeKnown(ctx, target, e.FragmentTarget.Value.Node) {
				return s.fail(contract("fragment target", "target outside exact adopted snapshot"))
			}
		}
	} else if e.FragmentTarget.State == EvidenceKnown {
		return s.fail(contract("fragment target", "known fragment requires known target"))
	}
	if e.Target.State != EvidenceKnown {
		if err := s.account.Charge(BudgetLinkedDocuments, 1); err != nil {
			return s.fail(err)
		}
	}
	if err := s.admit(ctx, e, 512); err != nil {
		return err
	}
	if err := s.subject(e.ID, PackageSubjectEdge); err != nil {
		return s.fail(err)
	}
	if e.Target.State == EvidenceKnown {
		s.adoptions[e.Target.Value].resource.required = true
	}
	source.expansion.edges[e.Template] = e.ID
	s.edges[e.ID] = stateClone(e)
	s.subjectPackages[e.ID] = source.packageID
	s.edgeOrder = append(s.edgeOrder, e.ID)
	return s.evaluateEdge(ctx, e, false)
}
func (s *PackageSession) adoptedNodeKnown(ctx context.Context, a *packageAdoption, node NodeID) bool {
	if node != 0 && node == a.resource.snapshot.header.Root.Node {
		return true
	}
	for _, id := range a.resource.snapshot.ids {
		if err := s.scan(ctx, 1); err != nil {
			return false
		}
		if id.Subject.Node == node {
			return true
		}
	}
	for _, v := range a.resource.snapshot.contexts {
		if err := s.scan(ctx, 1); err != nil {
			return false
		}
		if v.Subject.Node == node {
			return true
		}
	}
	return false
}
func (s *PackageSession) adoptionForPin(ctx context.Context, pin ResourcePin, doc string) *packageAdoption {
	for _, id := range s.adoptionOrder {
		if err := s.scan(ctx, 1); err != nil {
			return nil
		}
		a := s.adoptions[id]
		if a.documentID == doc && a.resource.pin == pin {
			return a
		}
	}
	return nil
}
func (s *PackageSession) resourceForPin(ctx context.Context, pin ResourcePin) *packageResource {
	for _, id := range s.resourceOrder {
		if err := s.scan(ctx, 1); err != nil {
			return nil
		}
		r := s.resources[id]
		if r.pin == pin {
			return r
		}
	}
	return nil
}
func snapshotExactID(ctx context.Context, s *PackageSession, snapshot *ResourceSnapshot, value string, node NodeID, kind string, part NodeID) bool {
	for _, v := range snapshot.ids {
		if err := s.scan(ctx, 1); err != nil {
			return false
		}
		if v.Value == value && v.Subject.Node == node && v.Kind.State == EvidenceKnown && v.Kind.Value == (QName{Local: kind}) && v.Unique.State == EvidenceKnown && v.Unique.Value && (part == 0 || v.Part.State == EvidenceKnown && v.Part.Value.Node == part) {
			return true
		}
	}
	return false
}
func snapshotIDByValue(ctx context.Context, s *PackageSession, snapshot *ResourceSnapshot, value string, kind string) (ResourceIDFact, bool) {
	for _, v := range snapshot.ids {
		if err := s.scan(ctx, 1); err != nil {
			return ResourceIDFact{}, false
		}
		if v.Value == value && v.Kind.State == EvidenceKnown && v.Kind.Value == (QName{Local: kind}) && v.Unique.State == EvidenceKnown && v.Unique.Value {
			return v, true
		}
	}
	return ResourceIDFact{}, false
}

// ObserveAssociation admits exact scoped caller evidence only after validating
// source local ID/owner/link prerequisites and optional target ID kind/ownership.
// A partial table miss is Unknown; negative membership needs exact authority.
func (s *PackageSession) ObserveAssociation(ctx context.Context, q AssociationRequest, r AssociationResult) error {
	if err := s.event(ctx, "ObserveAssociation"); err != nil {
		return err
	}
	release, err := s.temporary(ctx, struct {
		Request AssociationRequest
		Result  AssociationResult
	}{q, r})
	if err != nil {
		return err
	}
	defer release()
	request := ResourceRequest{ID: q.ID, Association: &q}
	result := ResourceResult{RequestID: q.ID, State: EvidenceKnown, Association: &r}
	if err := ValidateResourceResult(request, result); err != nil {
		return s.fail(err)
	}
	source := s.adoptionForPin(ctx, q.Source, q.SourcePart.DocumentID)
	target := s.resourceForPin(ctx, q.Target)
	if source == nil || target == nil {
		return s.fail(contract("association scope", "exact admitted source adoption and target pin required"))
	}
	if s.boundaries[source.packageID].sealed || source.expansion.owner != source.id || source.expansion.sealed {
		return s.fail(contract("association package", "source scope already sealed"))
	}
	if !snapshotExactID(ctx, s, source.resource.snapshot, q.PartID, q.SourcePart.Node, "score-part", q.SourcePart.Node) || !snapshotExactID(ctx, s, source.resource.snapshot, q.InstrumentID, q.SourceInstrument.Node, "score-instrument", q.SourcePart.Node) {
		return s.fail(contract("association source", "unique schema-proved local part/instrument required"))
	}
	var localLink, localInstrument bool
	for _, v := range source.resource.snapshot.contexts {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		p := v.Resource
		if p == nil {
			continue
		}
		if v.Subject.Node == q.PartLink.Node && p.Link != nil && p.Link.Kind == ResourcePartLink && p.Part.State == EvidenceKnown && p.Part.Value.Node == q.SourcePart.Node && p.Link.Href.Value.State == EvidenceKnown {
			localLink = true
		}
		if v.Subject.Node == q.InstrumentLink.Node && p.Instrument != nil && p.Instrument.Target.State == EvidenceKnown && p.Instrument.Target.Value.Node == q.SourceInstrument.Node && p.Part.State == EvidenceKnown && p.Part.Value.Node == q.SourcePart.Node {
			localInstrument = true
		}
	}
	matchedEdge := false
	for _, edgeID := range s.edgeOrder {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		edge := s.edges[edgeID]
		if edge.Source != source.id || edge.Target.State != EvidenceKnown {
			continue
		}
		template := source.resource.snapshot.contexts[edge.Template]
		targetAdoption := s.adoptions[edge.Target.Value]
		if template.Subject.Node == q.PartLink.Node && targetAdoption != nil && targetAdoption.resource.pin == q.Target {
			matchedEdge = true
			break
		}
	}
	if !matchedEdge {
		return s.fail(contract("association target", "selected part-link edge must bind this target pin"))
	}
	if !localLink || !localInstrument {
		return s.fail(contract("association source", "exact proved part-link/instrument-link prerequisites required"))
	}
	if target.snapshot.header.Name.State != EvidenceKnown || !packageMusicRoot(target.snapshot.header.Name.Value) {
		return s.fail(contract("association target", "known musical target scope required"))
	}
	var targetPart ResourceIDFact
	if r.TargetPartID.State == EvidenceKnown {
		var ok bool
		targetPart, ok = snapshotIDByValue(ctx, s, target.snapshot, r.TargetPartID.Value, "score-part")
		if !ok {
			return s.fail(contract("association target part", "supplied ID must resolve uniquely to score-part"))
		}
	}
	if r.TargetInstrumentID.State == EvidenceKnown {
		instrument, ok := snapshotIDByValue(ctx, s, target.snapshot, r.TargetInstrumentID.Value, "score-instrument")
		if !ok || r.TargetPartID.State == EvidenceKnown && (instrument.Part.State != EvidenceKnown || instrument.Part.Value.Node != targetPart.Subject.Node) {
			return s.fail(contract("association target instrument", "unique score-instrument with optional supplied owner required"))
		}
	}
	for _, old := range s.associations {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if old.request.ID == q.ID || old.request.Source == q.Source && old.request.InstrumentLink == q.InstrumentLink {
			return s.fail(contract("association request", "duplicate response"))
		}
	}
	if err := s.admit(ctx, r, 1024); err != nil {
		return err
	}
	id := s.internalSubject(PackageSubjectAssociation)
	s.subjectPackages[id] = source.packageID
	association := packageAssociation{id: id, request: stateClone(q), result: stateClone(r)}
	s.associations = append(s.associations, association)
	target.required = true
	for _, key := range s.catalog.orders[s.profile.ID] {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		binding := s.catalog.rules[key]
		if binding.Selector.Kind != PackageSubjectAssociation {
			continue
		}
		d := packageEvidenceDecision(s.baseDecision(key, id), r.Membership)
		if r.Membership.State == EvidenceKnown && !r.Membership.Value {
			d.Outcome = OutcomeFail
			d.Reason = "instrument-not-in-linked-part"
		}
		if err := s.record(ctx, binding.Rule, id, d, Location{Target: TargetPackage, DocumentID: s.documentID, Path: "/association[" + source.documentID + "]"}, false); err != nil {
			return err
		}
	}
	return nil
}

// ObserveRoles validates the full exact echoed request and bounded package scope.
// Caller assertions remain visibly authoritative metadata, never filename or
// same-ID inference. Unknown/partial resource-role tables cannot prove exclusion.
func (s *PackageSession) ObserveRoles(ctx context.Context, r ResourceRoleResult) error {
	if err := s.event(ctx, "ObserveRoles"); err != nil {
		return err
	}
	release, err := s.temporary(ctx, r)
	if err != nil {
		return err
	}
	defer release()
	q := r.Request
	if s.boundaries[q.Package] == nil || s.boundaries[q.Package].sealed {
		return s.fail(contract("role scope", "unknown package"))
	}
	if _, ok := s.roles[q.Package]; ok {
		return s.fail(contract("role scope", "duplicate role result"))
	}
	if err := ValidateResourceResult(ResourceRequest{ID: q.ID, Roles: &q}, ResourceResult{RequestID: q.ID, State: EvidenceKnown, Roles: &r}); err != nil {
		return s.fail(err)
	}

	for _, member := range q.Members {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		scoped := false
		for _, id := range s.adoptionOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			a := s.adoptions[id]
			base := Evidence[string]{State: EvidenceAbsent}
			if a.expansion.key.baseURI != "" {
				base = Evidence[string]{State: EvidenceKnown, Value: a.expansion.key.baseURI}
			}
			if a.packageID == q.Package && a.resource.pin == member.Resource && a.expansion.key.archiveEntry == member.ArchiveEntry && base.State == member.BaseURI.State && base.Value == member.BaseURI.Value {
				scoped = true
				break
			}
		}
		if !scoped {
			return s.fail(contract("role member", "exact same-package context was not admitted"))
		}
	}
	for _, pin := range q.Resources {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		scoped := false
		for _, id := range s.adoptionOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			a := s.adoptions[id]
			if a.packageID == q.Package && a.resource.pin == pin {
				scoped = true
				break
			}
		}
		if !scoped {
			for _, id := range s.rootfileOrder {
				if err := s.scan(ctx, 1); err != nil {
					return err
				}
				r := s.rootfiles[id]
				if r.Package == q.Package && r.Resource.State == EvidenceKnown && s.resources[r.Resource.Value].pin == pin {
					scoped = true
					break
				}
			}
		}
		if !scoped {
			return s.fail(contract("role scope", "resource pin was not admitted"))
		}
	}
	if err := s.admit(ctx, r, 512); err != nil {
		return err
	}
	s.roles[q.Package] = stateClone(r)
	return s.evaluateBoundary(ctx, s.boundaries[q.Package])
}

func (s *PackageSession) evaluateResourceRule(ctx context.Context, r PackageRuleBinding) error {
	switch r.Selector.Kind {
	case PackageSubjectEdge:
		for _, id := range s.edgeOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			if err := s.evaluateEdgeRule(ctx, r, s.edges[id], true); err != nil {
				return err
			}
		}
	case PackageSubjectAssociation:
		// Every selected instrument-link template creates an obligation even when no
		// provider reply arrived. Missing capability is never an empty-table pass.
		for _, aid := range s.adoptionOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			a := s.adoptions[aid]
			if a.expansion.owner != aid {
				continue
			}
			for _, v := range a.resource.snapshot.contexts {
				if err := s.scan(ctx, 1); err != nil {
					return err
				}
				if v.Resource == nil || v.Resource.Instrument == nil {
					continue
				}
				id := PackageFactID(0)
				for _, association := range s.associations {
					if err := s.scan(ctx, 1); err != nil {
						return err
					}
					q := association.request
					if q.Source == a.resource.pin && q.InstrumentLink.DocumentID == a.documentID && q.InstrumentLink.Node == v.Subject.Node {
						id = association.id
						break
					}
				}
				if id == 0 {
					if err := s.admit(ctx, v.Subject, 256); err != nil {
						return err
					}
					id = s.internalSubject(PackageSubjectAssociation)
					s.subjectPackages[id] = a.packageID
				}
				d := s.baseDecision(r.Rule.Key, id)
				local := v.Resource.Instrument.Target
				if local.State != EvidenceKnown {
					d = packageEvidenceDecision(d, local)
				} else {
					d.Outcome = OutcomeUnknown
					d.Reason = "instrument-association-evidence-unavailable"
					for _, association := range s.associations {
						if err := s.scan(ctx, 1); err != nil {
							return err
						}
						q := association.request
						if q.Source == a.resource.pin && q.InstrumentLink.DocumentID == a.documentID && q.InstrumentLink.Node == v.Subject.Node {
							d = packageEvidenceDecision(d, association.result.Membership)
							if association.result.Membership.State == EvidenceKnown && !association.result.Membership.Value {
								d.Outcome = OutcomeFail
								d.Reason = "instrument-not-in-linked-part"
							}
							break
						}
					}
				}
				if d.Cause != nil && d.Cause.Subject.DocumentID == a.resource.snapshot.header.DocumentID {
					cause := *d.Cause
					cause.Subject.DocumentID = a.documentID
					d.Cause = &cause
				}
				if err := s.record(ctx, r.Rule, id, d, Location{Target: TargetPackage, DocumentID: s.documentID, Path: "/association[" + a.documentID + "]"}, true); err != nil {
					return err
				}
			}
		}
	case PackageSubjectResource:
		for _, id := range s.resourceOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			resource := s.resources[id]
			d := s.baseDecision(r.Rule.Key, resource.subject)
			d.Outcome = OutcomeNotApplicable
			d.Reason = "schema-hints-proven-absent"
			found := false
			for _, v := range resource.snapshot.contexts {
				if err := s.scan(ctx, 1); err != nil {
					return err
				}
				if v.Resource == nil || v.Resource.SchemaHints == nil {
					continue
				}
				found = true
				h := v.Resource.SchemaHints
				if h.Raw.State == EvidenceAbsent && h.NoNamespace.Presence.State == EvidenceAbsent {
					continue
				}
				if h.Complete.State != EvidenceKnown || !h.Complete.Value {
					d = packageEvidenceDecision(d, h.Complete)
					if d.Outcome == OutcomePass || d.Outcome == OutcomeNotApplicable {
						d.Outcome = OutcomeUnknown
					}
					d.Reason = "schema-discovery-pairing-unavailable"
					break
				}
				d.Outcome = OutcomePass
				d.Reason = "schema-discovery-complete-pairs"
			}
			if !found {
				d.Outcome = OutcomeUnsupported
				d.Reason = "schema-discovery-projection-unavailable"
			}
			if err := s.record(ctx, r.Rule, resource.subject, d, Location{Target: TargetPackage, DocumentID: s.documentID, Path: "/resource[" + string(id) + "]"}, true); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *PackageSession) linkTargetDecision(ctx context.Context, d Decision, e PackageEdgeObservation, v ContextResult) Decision {
	link := v.Resource.Link
	if link.Href.Value.State != EvidenceKnown {
		return packageAdoptDecision(packageRequiredEvidenceDecision(d, link.Href.Value), s.adoptions[e.Source])
	}
	if e.Target.State != EvidenceKnown {
		d = packageEvidenceDecision(d, e.Target)
		if d.Outcome == OutcomeNotApplicable {
			d.Outcome = OutcomeFail
			d.Reason = "linked-resource-authoritatively-absent"
		}
		return d
	}
	target := s.adoptions[e.Target.Value]
	root := target.resource.snapshot.header.Name
	if root.State != EvidenceKnown {
		return packageAdoptDecision(packageRequiredEvidenceDecision(d, root), target)
	}
	wantScore := link.Kind == ResourceOpusScore || link.Kind == ResourcePartLink
	wantOpus := link.Kind == ResourceOpusLink || link.Kind == ResourceScoreWorkOpus
	if wantScore && (root.Value.Space != "" || root.Value.Local != "score-partwise" && root.Value.Local != "score-timewise") || wantOpus && root.Value != (QName{Local: "opus"}) {
		d.Outcome = OutcomeFail
		d.Reason = "wrong-linked-document-kind"
		return d
	}
	if e.Fragment.State == EvidenceKnown && e.Fragment.Value != "" {
		if !xsd.ValidXSDNCName(e.Fragment.Value) {
			d.Outcome = OutcomeUnsupported
			d.Reason = "fragment-grammar-unimplemented"
			return d
		}
		matching := false
		for _, id := range target.resource.snapshot.ids {
			if err := s.scan(ctx, 1); err != nil {
				return d
			}
			if id.Value == e.Fragment.Value {
				if id.Unique.State != EvidenceKnown || !id.Unique.Value {
					return packageAdoptDecision(packageRequiredEvidenceDecision(d, id.Unique), target)
				}
				if e.FragmentTarget.State == EvidenceKnown && id.Subject.Node != e.FragmentTarget.Value.Node {
					d.Outcome = OutcomeUnknown
					d.Reason = "fragment-target-identity-mismatch"
					return d
				}
				matching = true
				break
			}
		}
		if !matching {
			d.Outcome = OutcomeUnknown
			d.Reason = "fragment-id-scope-incomplete"
			if knownComplete(target.resource.snapshot.header.IDScopeComplete) {
				d.Outcome = OutcomeFail
				d.Reason = "fragment-id-absent"
			}
			return d
		}
	} else if e.Fragment.State != EvidenceAbsent && e.Fragment.State != EvidenceKnown {
		return packageRequiredEvidenceDecision(d, e.Fragment)
	}
	d.Outcome = OutcomePass
	d.Reason = "linked-target-kind-established"
	return d
}
func (s *PackageSession) groupDecision(ctx context.Context, d Decision, e PackageEdgeObservation, v ContextResult) Decision {
	if e.Target.State != EvidenceKnown {
		return packageEvidenceDecision(d, e.Target)
	}
	source := s.adoptions[e.Source]
	target := s.adoptions[e.Target.Value]
	if !knownComplete(source.resource.snapshot.header.ScopeComplete) || !knownComplete(target.resource.snapshot.header.ScopeComplete) {
		d.Outcome = OutcomeUnknown
		d.Reason = "group-scope-incomplete"
		return d
	}
	found := false
	for _, c := range source.resource.snapshot.contexts {
		if err := s.scan(ctx, 1); err != nil {
			return d
		}
		if s.scan(ctx, 1) != nil {
			return d
		}
		if c.Resource == nil || c.Resource.Group == nil || !c.Resource.Group.Linked || c.Resource.Owner.Node != v.Subject.Node {
			continue
		}
		found = true
		value := c.Resource.Group.Value
		if value.State != EvidenceKnown {
			return packageAdoptDecision(packageRequiredEvidenceDecision(d, value), source)
		}
		matched, uncertain := false, false
		for _, candidate := range target.resource.snapshot.contexts {
			if s.scan(ctx, 1+int64(len(value.Value))) != nil {
				return d
			}
			if candidate.Resource == nil || candidate.Resource.Group == nil || candidate.Resource.Group.Linked {
				continue
			}
			other := candidate.Resource.Group.Value
			if other.State != EvidenceKnown {
				uncertain = true
				continue
			}
			if other.Value == value.Value {
				matched = true
				break
			}
		}
		if !matched {
			d.Outcome = OutcomeFail
			d.Reason = "group-link-string-mismatch"
			if uncertain {
				d.Outcome = OutcomeUnknown
				d.Reason = "target-group-value-unavailable"
			}
			return d
		}
	}
	if !found {
		d.Outcome = OutcomeNotApplicable
		d.Reason = "no-group-link-advice"
		return d
	}
	d.Outcome = OutcomePass
	d.Reason = "group-link-strings-match"
	return d
}

func packageAdoptDecision(d Decision, a *packageAdoption) Decision {
	if d.Cause != nil && d.Cause.Subject.DocumentID == a.resource.snapshot.header.DocumentID {
		c := *d.Cause
		c.Subject.DocumentID = a.documentID
		d.Cause = &c
	}
	return d
}

func (s *PackageSession) evaluateEdge(ctx context.Context, e PackageEdgeObservation, final bool) error {
	for _, key := range s.catalog.orders[s.profile.ID] {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		r := s.catalog.rules[key]
		if r.Selector.Kind == PackageSubjectEdge {
			if err := s.evaluateEdgeRule(ctx, r, e, final); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *PackageSession) evaluateEdgeRule(ctx context.Context, r PackageRuleBinding, edge PackageEdgeObservation, final bool) error {
	source := s.adoptions[edge.Source]
	template := source.resource.snapshot.contexts[edge.Template]
	d := s.baseDecision(r.Rule.Key, edge.ID)
	if support := s.catalog.supports[s.profile.ID][r.Rule.Key]; support.State == ImplementationUnsupported {
		d.Outcome = OutcomeUnsupported
		d.Reason = support.Reason
	} else if pre, ok := s.prerequisites(ctx, r.Rule, edge.ID); !ok {
		d = pre
	} else if r.Plan.check == PackageCheckLinkTarget {
		d = s.linkTargetDecision(ctx, d, edge, template)
	} else {
		d = s.groupDecision(ctx, d, edge, template)
	}
	location := Location{Target: TargetPackage, DocumentID: s.documentID, Path: "/edge[" + string(edge.Source) + "]/template[" + strconv.FormatUint(edge.Template, 10) + "]"}
	if len(template.Origins) > 0 {
		origin := template.Origins[0]
		if origin.DocumentID == source.resource.snapshot.header.DocumentID {
			origin.DocumentID = source.documentID
		}
		d.Related = append(d.Related, origin)
	}
	return s.record(ctx, r.Rule, edge.ID, d, location, final)
}

// roleUniverseComplete separates provider request coverage from the whole
// package universe. Unknown/unrequested physical members cannot disappear into
// a complete but smaller table. Mimetype is the one literal non-XML metadata
// record excluded by the package domain, independently of inferred roles.
func (s *PackageSession) roleUniverseComplete(ctx context.Context, b *packageBoundary, roles ResourceRoleResult) bool {
	if !b.sealed || !knownComplete(b.enumeration.Entries) || !knownComplete(b.enumeration.Rootfiles) || !knownComplete(b.enumeration.Edges) {
		return false
	}
	for _, aid := range s.adoptionOrder {
		if s.scan(ctx, 1) != nil {
			return false
		}
		a := s.adoptions[aid]
		if a.packageID != b.identity.ID || a.expansion.owner != aid || a.expansion.key.archiveEntry == "" {
			continue
		}
		covered := false
		for _, m := range roles.Request.Members {
			if s.scan(ctx, 1) != nil {
				return false
			}
			baseState := EvidenceAbsent
			if a.expansion.key.baseURI != "" {
				baseState = EvidenceKnown
			}
			covered = covered || m.Resource == a.resource.pin && m.ArchiveEntry == a.expansion.key.archiveEntry && m.BaseURI.State == baseState && m.BaseURI.Value == a.expansion.key.baseURI
		}
		if len(roles.Request.Members) == 0 {
			for _, p := range roles.Request.Resources {
				if s.scan(ctx, 1) != nil {
					return false
				}
				covered = covered || p == a.resource.pin
			}
		}
		if !covered {
			return false
		}
	}
	for _, eid := range s.entryOrder {
		if s.scan(ctx, 1) != nil {
			return false
		}
		entry := s.entries[eid]
		if entry.Package != b.identity.ID || entry.Name == "mimetype" {
			continue
		}
		if len(entry.RawName) != 0 {
			return false
		}
		covered := false
		for _, m := range roles.Request.Members {
			if s.scan(ctx, 1) != nil {
				return false
			}
			covered = covered || m.ArchiveEntry == entry.Name
		}
		if len(roles.Request.Members) == 0 {
			for _, aid := range s.adoptionOrder {
				if s.scan(ctx, 1) != nil {
					return false
				}
				a := s.adoptions[aid]
				if a.packageID != b.identity.ID || a.expansion.key.archiveEntry != entry.Name {
					continue
				}
				for _, p := range roles.Request.Resources {
					if s.scan(ctx, 1) != nil {
						return false
					}
					covered = covered || p == a.resource.pin
				}
			}
		}
		if !covered {
			return false
		}
	}
	return true
}
func (s *PackageSession) rolePinAmbiguous(ctx context.Context, pkg PackageID, pin ResourcePin) bool {
	count := 0
	for _, id := range s.adoptionOrder {
		if s.scan(ctx, 1) != nil {
			return true
		}
		a := s.adoptions[id]
		if a.packageID == pkg && a.resource.pin == pin && a.expansion.owner == id {
			count++
			if count > 1 {
				return true
			}
		}
	}
	return false
}
