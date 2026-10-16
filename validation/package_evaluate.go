package validation

import (
	"bytes"
	"context"
)

func packageEvidenceDecision[T comparable](d Decision, e Evidence[T]) Decision {
	d.Reason = e.Reason
	switch e.State {
	case EvidenceKnown:
		d.Outcome = OutcomePass
	case EvidenceAbsent:
		d.Outcome = OutcomeNotApplicable
	case EvidenceUnknown:
		d.Outcome = OutcomeUnknown
	case EvidenceUnsupported:
		d.Outcome = OutcomeUnsupported
	case EvidenceInvalid:
		d.Outcome = OutcomeBlocked
		d.Cause = e.Cause
	}
	return d
}
func (s *PackageSession) baseDecision(key RuleKey, id PackageFactID) Decision {
	return Decision{Rule: key, Subject: NodeRef{s.documentID, NodeID(id)}, Outcome: OutcomeUnknown, Reason: "package-evidence-unavailable"}
}
func (s *PackageSession) record(ctx context.Context, r RuleDefinition, id PackageFactID, d Decision, location Location, final bool) error {
	if s.terminal != nil {
		return s.terminal
	}
	k := packageDecisionKey{r.Key, id}
	if _, ok := s.decisions[k]; ok {
		return nil
	}
	if support := s.catalog.supports[s.profile.ID][r.Key]; support.State == ImplementationUnsupported {
		d.Outcome = OutcomeUnsupported
		d.Reason = support.Reason
		d.Cause = nil
	} else if pre, ok := s.prerequisites(ctx, r, id); !ok {
		d = pre
	}
	return s.recordPrepared(ctx, r, id, d, location, final)
}

// recordPrepared accepts a decision whose support and dependencies were checked
// by its evaluator. Other ingress paths use record to perform those checks once.
func (s *PackageSession) recordPrepared(ctx context.Context, r RuleDefinition, id PackageFactID, d Decision, location Location, final bool) error {
	if s.terminal != nil {
		return s.terminal
	}
	k := packageDecisionKey{r.Key, id}
	if _, ok := s.decisions[k]; ok {
		return nil
	}

	if !final && d.Outcome != OutcomeFail && d.Outcome != OutcomePass && d.Outcome != OutcomeNotApplicable {
		return nil
	}
	if ctx == nil {
		return s.fail(contract("context", "must not be nil"))
	}
	if err := ctx.Err(); err != nil {
		return s.fail(err)
	}
	if err := s.account.Charge(BudgetRuleSteps, 1); err != nil {
		return s.fail(err)
	}
	if err := s.accumulator.recordOutcome(r, d); err != nil {
		return s.fail(err)
	}
	// Established findings precede the optional dependency-cache retention.
	// A cache refusal must not erase the only explanation of an authoritative count.
	before := len(s.accumulator.report.Findings)
	if err := s.accumulator.storeDiagnostic(r, d, location); err != nil {
		return s.fail(err)
	}
	if len(s.accumulator.report.Findings) == before {
		s.output.omittedDiagnostics.capture(r, d, s.subjectPackages[id])
	}
	if err := s.admit(ctx, d, 256); err != nil {
		return err
	}
	s.decisions[k] = cloneDecision(d)
	return nil
}
func (s *PackageSession) prerequisites(ctx context.Context, r RuleDefinition, id PackageFactID) (Decision, bool) {
	for _, dep := range r.Requires {
		if err := s.scan(ctx, 1); err != nil {
			return s.baseDecision(r.Key, id), false
		}
		d := s.baseDecision(r.Key, id)
		if dep.Kind != DependencyRule {
			d.Outcome = OutcomeUnsupported
			d.Reason = "package-capability-unavailable"
			return d, false
		}
		subject := id
		if dep.Relation == RelationDocument {
			if b := s.boundaries[s.subjectPackages[id]]; b != nil {
				subject = b.subject
			} else {
				d.Outcome = OutcomeUnknown
				d.Reason = "package-prerequisite-scope-unavailable"
				return d, false
			}
		}
		predecessor, ok := s.decisions[packageDecisionKey{dep.Rule, subject}]
		if !ok {
			d.Outcome = OutcomeUnknown
			d.Reason = "package-prerequisite-pending"
			return d, false
		}
		if predecessor.Outcome != OutcomePass && predecessor.Outcome != OutcomeNotApplicable {
			d.Outcome = OutcomeBlocked
			d.Reason = "package-prerequisite-unavailable"
			d.Cause = &DecisionRef{Rule: predecessor.Rule, Subject: predecessor.Subject}
			return d, false
		}
	}
	return Decision{}, true
}
func (s *PackageSession) evaluateEntry(ctx context.Context, e PackageEntryObservation, final bool) error {
	b := s.boundaries[e.Package]
	for _, key := range s.catalog.orders[s.profile.ID] {
		if _, ok := s.decisions[packageDecisionKey{key, e.ID}]; ok {
			continue
		}
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		r := s.catalog.rules[key]
		if r.Selector.Kind != PackageSubjectEntry {
			continue
		}
		d := s.baseDecision(key, e.ID)
		if support := s.catalog.supports[s.profile.ID][key]; support.State == ImplementationUnsupported {
			d.Outcome = OutcomeUnsupported
			d.Reason = support.Reason
		} else if pre, ok := s.prerequisites(ctx, r.Rule, e.ID); !ok {
			d = pre
		} else {
			switch r.Plan.check {
			case PackageCheckMimetypeStored, PackageCheckMimetypeNoExtra, PackageCheckMimetypeNoPrefix:
				if b.identity.Representation != ResourceRawMXL || e.Name != "mimetype" {
					d.Outcome = OutcomeNotApplicable
					d.Reason = "outside-raw-mimetype-scope"
					break
				}
				switch r.Plan.check {
				case PackageCheckMimetypeStored:
					d = packageStoredDecision(d, e)
				case PackageCheckMimetypeNoExtra:
					d = packageExtraDecision(d, e)
				case PackageCheckMimetypeNoPrefix:
					if m, ok := s.mimetypes[e.ID]; ok {
						d = packagePrefixDecision(d, m)
					}
				}
			case PackageCheckEntryNameAdvice:
				if b.identity.Representation != ResourceRawMXL {
					d.Outcome = OutcomeNotApplicable
					d.Reason = "original-entry-encoding-unavailable"
					break
				}
				d = packageEvidenceDecision(d, e.UTF8Name)
				if e.UTF8Name.State == EvidenceKnown && !e.UTF8Name.Value {
					d.Outcome = OutcomeFail
					d.Reason = "legacy-entry-name-encoding"
				}
			case PackageCheckCompressionObservation:
				if b.identity.Representation != ResourceRawMXL {
					d.Outcome = OutcomeNotApplicable
					d.Reason = "original-compression-unavailable"
					break
				}
				d = packageEvidenceDecision(d, e.Central.Method)
				if e.Central.Method.State == EvidenceKnown {
					d.Outcome = OutcomePass
					d.Reason = "observed-compression-method"
				}
			}
		}
		if err := s.recordPrepared(ctx, r.Rule, e.ID, d, e.Location, final); err != nil {
			return err
		}
	}
	return nil
}
func packageStoredDecision(d Decision, e PackageEntryObservation) Decision {
	// An independently framed complete local header can already prove a failure.
	if e.Local.Present.State == EvidenceKnown && e.Local.Present.Value {
		if e.Local.Method.State == EvidenceKnown && e.Local.Method.Value != 0 {
			d.Outcome = OutcomeFail
			d.Reason = "mimetype-local-compressed"
			return d
		}
		if e.Local.Flags.State == EvidenceKnown && e.Local.Flags.Value&1 != 0 {
			d.Outcome = OutcomeFail
			d.Reason = "mimetype-local-encrypted"
			return d
		}
	}
	for _, v := range []Evidence[bool]{e.Local.Present, e.Central.Present, e.HeaderAssociation} {
		if v.State != EvidenceKnown || !v.Value {
			d = packageEvidenceDecision(d, v)
			if d.Outcome == OutcomePass || d.Outcome == OutcomeNotApplicable {
				d.Outcome = OutcomeUnknown
				d.Reason = "mimetype-header-association-unavailable"
			}
			return d
		}
	}
	for _, h := range []ZIPHeaderObservation{e.Local, e.Central} {
		if h.Method.State != EvidenceKnown {
			return packageRequiredEvidenceDecision(d, h.Method)
		}
		if h.Flags.State != EvidenceKnown {
			return packageRequiredEvidenceDecision(d, h.Flags)
		}
	}
	if e.Local.Method.Value != e.Central.Method.Value || e.Local.Flags.Value&1 != e.Central.Flags.Value&1 {
		d.Outcome = OutcomeUnknown
		d.Reason = "mimetype-header-inconsistency"
		return d
	}
	d.Outcome = OutcomePass
	d.Reason = "mimetype-stored-unencrypted"
	return d
}
func packageExtraDecision(d Decision, e PackageEntryObservation) Decision {
	if e.Local.Present.State == EvidenceKnown && e.Local.Present.Value && e.Local.ExtraLength.State == EvidenceKnown && e.Local.ExtraLength.Value > 0 {
		d.Outcome = OutcomeFail
		d.Reason = "mimetype-local-extra-field"
		return d
	}
	for _, v := range []Evidence[bool]{e.Local.Present, e.Central.Present, e.HeaderAssociation} {
		if v.State != EvidenceKnown || !v.Value {
			return packageRequiredEvidenceDecision(d, v)
		}
	}
	if e.Local.ExtraLength.State != EvidenceKnown {
		return packageRequiredEvidenceDecision(d, e.Local.ExtraLength)
	}
	// MusicXML adopts the early-byte identification mechanism: only local
	// extras move the mimetype payload. Central framing and association remain
	// prerequisites above; central extras have separate ZIP transport duties.
	d.Outcome = OutcomePass
	d.Reason = "mimetype-no-local-header-extra"
	return d
}
func packagePrefixDecision(d Decision, m PackageMimetypeObservation) Decision {
	prefix := m.Prefix
	// Check longer Unicode signatures first, preserving a complete signature even
	// when later content transport fails. Truncated signatures remain unavailable.
	for _, bom := range [][]byte{{0, 0, 0xfe, 0xff}, {0xff, 0xfe, 0, 0}, {0xef, 0xbb, 0xbf}, {0xfe, 0xff}, {0xff, 0xfe}} {
		if bytes.HasPrefix(prefix, bom) {
			d.Outcome = OutcomeFail
			d.Reason = "mimetype-leading-bom"
			return d
		}
	}
	if len(prefix) > 0 {
		switch prefix[0] {
		case 0, ' ', '\t', '\n', '\v', '\f', '\r':
			d.Outcome = OutcomeFail
			d.Reason = "mimetype-leading-padding"
			return d
		}
	}
	if bytes.HasPrefix(prefix, []byte("application/vnd.recordare.musicxml")) {
		d.Outcome = OutcomePass
		d.Reason = "mimetype-no-leading-prefix"
		return d
	}
	if m.ContentComplete.State == EvidenceKnown && m.ContentComplete.Value && m.ByteLength.State == EvidenceKnown && m.ByteLength.Value == 0 {
		d.Outcome = OutcomePass
		d.Reason = "empty-mimetype-no-prefix"
		return d
	}
	d.Outcome = OutcomeUnknown
	d.Reason = "mimetype-prefix-family-unresolved"
	return d
}
func (s *PackageSession) evaluateBoundary(ctx context.Context, b *packageBoundary) error {
	for _, key := range s.catalog.orders[s.profile.ID] {
		if _, ok := s.decisions[packageDecisionKey{key, b.subject}]; ok {
			continue
		}
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		r := s.catalog.rules[key]
		if r.Selector.Kind != PackageSubjectPackage {
			continue
		}
		d := s.baseDecision(key, b.subject)
		if support := s.catalog.supports[s.profile.ID][key]; support.State == ImplementationUnsupported {
			d.Outcome = OutcomeUnsupported
			d.Reason = support.Reason
		} else if pre, ok := s.prerequisites(ctx, r.Rule, b.subject); !ok {
			d = pre
		} else {
			switch r.Plan.check {
			case PackageCheckContainerPath:
				if b.identity.Representation != ResourceRawMXL {
					d.Outcome = OutcomeNotApplicable
					d.Reason = "outside-original-container-scope"
				} else if _, ok := s.entryNames[b.identity.ID]["META-INF/container.xml"]; ok {
					d.Outcome = OutcomePass
					d.Reason = "exact-container-entry"
				} else if b.sealed && b.enumeration.Entries.State == EvidenceKnown && b.enumeration.Entries.Value {
					d.Outcome = OutcomeFail
					d.Reason = "missing-container-entry"
				} else {
					d.Outcome = OutcomeUnknown
					d.Reason = "package-entry-enumeration-incomplete"
				}
			case PackageCheckMimetypeAdvice:
				if b.identity.Representation != ResourceRawMXL {
					d.Outcome = OutcomeNotApplicable
					d.Reason = "outside-original-mimetype-scope"
					break
				}
				id, ok := s.entryNames[b.identity.ID]["mimetype"]
				if !ok {
					if b.sealed && b.enumeration.Entries.State == EvidenceKnown && b.enumeration.Entries.Value {
						d.Outcome = OutcomeFail
						d.Reason = "mimetype-recommended-presence"
					}
				} else {
					e := s.entries[id]
					m, known := s.mimetypes[id]
					if e.PhysicalOrder.State == EvidenceKnown && e.PhysicalOrder.Value != 0 {
						d.Outcome = OutcomeFail
						d.Reason = "mimetype-recommended-first"
					} else if known && m.ContentComplete.State == EvidenceKnown && m.ContentComplete.Value && m.ByteLength.State == EvidenceKnown {
						want := []byte("application/vnd.recordare.musicxml")
						if uint64(len(m.Prefix)) != m.ByteLength.Value {
							d.Outcome = OutcomeUnknown
							d.Reason = "mimetype-prefix-family-unresolved"
						} else if m.ByteLength.Value == uint64(len(want)) && bytes.Equal(m.Prefix, want) {
							d = packageEvidenceDecision(d, e.PhysicalOrder)
							if e.PhysicalOrder.State == EvidenceKnown {
								d.Outcome = OutcomePass
								d.Reason = "mimetype-recommended-content"
							}
						} else {
							d.Outcome = OutcomeFail
							d.Reason = "mimetype-recommended-content"
						}
					}
				}
			case PackageCheckScoreBeforeParts:
				if b.identity.Representation != ResourceRawMXL && b.identity.Representation != ResourceCurrentMXL {
					d.Outcome = OutcomeNotApplicable
					d.Reason = "outside-package-role-scope"
				} else {
					d = s.scoreBeforeParts(ctx, d, b)
				}
			}
		}
		if err := s.recordPrepared(ctx, r.Rule, b.subject, d, s.location(b.identity.ID), b.sealed || s.finalizing); err != nil {
			return err
		}
	}
	return nil
}
func (s *PackageSession) scoreBeforeParts(ctx context.Context, d Decision, b *packageBoundary) Decision {
	roles, known := s.roles[b.identity.ID]
	if known && b.membersSealed && knownComplete(b.membersComplete) {
		var score, part bool
		uncertain := false
		var first ResourcePin
		var firstPath string
		firstKnown := false
		for _, id := range s.rootfileOrder {
			if err := s.scan(ctx, 1); err != nil {
				return d
			}
			r := s.rootfiles[id]
			if r.Package == b.identity.ID && r.Ordinal == 1 && r.Resource.State == EvidenceKnown {
				if resource := s.resources[r.Resource.Value]; resource != nil {
					first = resource.pin
					firstPath = r.FullPath.Value
					firstKnown = r.FullPath.State == EvidenceKnown
				}
			}
		}
		firstScore := Evidence[bool]{State: EvidenceUnknown, Reason: "primary-score-role-unavailable"}
		firstMembers := 0
		for _, id := range s.adoptionOrder {
			if s.scan(ctx, 1) != nil {
				return d
			}
			a := s.adoptions[id]
			if firstKnown && a.packageID == b.identity.ID && a.resource.pin == first && a.expansion.owner == id && a.expansion.key.archiveEntry == firstPath {
				firstMembers++
			}
		}
		// Complete provider scope cannot turn missing score/part classifications
		// into negative membership. Every requested row has independent states.
		for _, pin := range roles.Request.Resources {
			if s.scan(ctx, 1) != nil {
				return d
			}
			if len(roles.Request.Members) > 0 {
				break
			}
			scoreKnown, partKnown := false, false
			for _, f := range roles.Resources {
				if s.scan(ctx, 1) != nil {
					return d
				}
				if f.Resource == pin && f.Membership.State == EvidenceKnown {
					scoreKnown = scoreKnown || f.Role == ResourceRoleScore
					partKnown = partKnown || f.Role == ResourceRoleIndividualPart
				}
			}
			uncertain = uncertain || !scoreKnown || !partKnown
		}
		for _, m := range roles.Request.Members {
			if s.scan(ctx, 1) != nil {
				return d
			}
			scoreKnown, partKnown := false, false
			for _, f := range roles.Resources {
				if s.scan(ctx, 1) != nil {
					return d
				}
				if f.Member != nil && roleMemberKey(*f.Member) == roleMemberKey(m) && f.Membership.State == EvidenceKnown {
					scoreKnown = scoreKnown || f.Role == ResourceRoleScore
					partKnown = partKnown || f.Role == ResourceRoleIndividualPart
				}
			}
			uncertain = uncertain || !scoreKnown || !partKnown
		}
		for _, r := range roles.Resources {
			if err := s.scan(ctx, 1); err != nil {
				return d
			}
			if len(roles.Request.Members) == 0 && s.rolePinAmbiguous(ctx, b.identity.ID, r.Resource) {
				uncertain = true
				continue
			}
			uncertain = uncertain || r.Membership.State != EvidenceKnown
			if r.Membership.State == EvidenceKnown && r.Membership.Value {
				score = score || r.Role == ResourceRoleScore
				part = part || r.Role == ResourceRoleIndividualPart
			}
			if firstKnown && r.Resource == first && r.Role == ResourceRoleScore && (len(roles.Request.Members) == 0 || firstMembers == 1 && r.Member != nil && r.Member.ArchiveEntry == firstPath) {
				firstScore = r.Membership
			}
		}
		if score && part {
			d = packageEvidenceDecision(d, firstScore)
			if firstScore.State == EvidenceKnown && !firstScore.Value {
				d.Outcome = OutcomeFail
				d.Reason = "score-must-precede-parts"
			}
			return d
		}
		if roles.ScopeComplete.State == EvidenceKnown && roles.ScopeComplete.Value && !uncertain && (!score || !part) && s.roleUniverseComplete(ctx, b, roles) {
			d.Outcome = OutcomeNotApplicable
			d.Reason = "complete-role-scope-excludes-score-parts"
			return d
		}
	}
	// Narrow complete minimal-package vacuity: only container, optional mimetype,
	// and the sole known primary resource; no encoded external edge inventory.
	if b.sealed && knownComplete(b.enumeration.Entries) && knownComplete(b.enumeration.Rootfiles) && knownComplete(b.enumeration.Edges) {
		primary := PackageFactID(0)
		roots := 0
		for _, id := range s.rootfileOrder {
			if err := s.scan(ctx, 1); err != nil {
				return d
			}
			r := s.rootfiles[id]
			if r.Package == b.identity.ID {
				roots++
				if r.Ordinal == 1 && r.Entry.State == EvidenceKnown && r.Resource.State == EvidenceKnown {
					resource := s.resources[r.Resource.Value]
					if resource != nil && resource.snapshot.header.LinkProjectionSelected && knownComplete(resource.snapshot.header.ScopeComplete) && resource.snapshot.header.Name.State == EvidenceKnown && packageMusicRoot(resource.snapshot.header.Name.Value) {
						noLinks := true
						for _, v := range resource.snapshot.contexts {
							if s.scan(ctx, 1) != nil {
								return d
							}
							if v.Resource != nil && v.Resource.Link != nil && v.Resource.Link.Kind != ResourceBookmark {
								noLinks = false
								break
							}
						}
						if noLinks {
							primary = r.Entry.Value
						}
					}
				}
			}
		}
		minimal := roots == 1 && primary != 0
		for _, id := range s.entryOrder {
			if err := s.scan(ctx, 1); err != nil {
				return d
			}
			e := s.entries[id]
			if e.Package == b.identity.ID && e.Name != "META-INF/container.xml" && e.Name != "mimetype" && id != primary {
				minimal = false
			}
		}
		for _, id := range s.edgeOrder {
			if err := s.scan(ctx, 1); err != nil {
				return d
			}
			edge := s.edges[id]
			if a := s.adoptions[edge.Source]; a != nil && a.packageID == b.identity.ID {
				minimal = false
			}
		}
		if minimal {
			d.Outcome = OutcomeNotApplicable
			d.Reason = "complete-minimal-package-no-separate-parts"
			return d
		}
	}
	d.Outcome = OutcomeUnknown
	d.Reason = "package-score-part-role-evidence-unavailable"
	return d
}
func knownComplete(v Evidence[bool]) bool { return v.State == EvidenceKnown && v.Value }
func packageMusicRoot(q QName) bool {
	return q.Space == "" && (q.Local == "score-partwise" || q.Local == "score-timewise" || q.Local == "opus")
}
func (s *PackageSession) rootfileDecision(ctx context.Context, d Decision, r PackageRootfileObservation, check PackageCheck) Decision {
	if check == PackageCheckFirstRootMusicXML || check == PackageCheckFirstMediaType {
		if r.Ordinal != 1 {
			d.Outcome = OutcomeNotApplicable
			d.Reason = "not-first-rootfile"
			return d
		}
	}
	if check == PackageCheckFirstMediaType {
		return s.mediaDecision(ctx, d, r.MediaType)
	}
	if check == PackageCheckRootfileRoot && r.Ordinal != 1 {
		media := s.mediaDecision(ctx, d, r.MediaType)
		if media.Outcome == OutcomeFail {
			d.Outcome = OutcomeNotApplicable
			d.Reason = "nonmusicxml-alternate"
			return d
		}
		if media.Outcome != OutcomePass {
			return media
		}
	}
	if r.Indirection.State == EvidenceKnown && r.Indirection.Value {
		d.Outcome = OutcomeUnknown
		d.Reason = "nested-rootfile-indirection-unresolved"
		return d
	}
	if r.Resource.State != EvidenceKnown {
		d = packageEvidenceDecision(d, r.Resource)
		if d.Outcome == OutcomeNotApplicable {
			d.Outcome = OutcomeFail
			d.Reason = "rootfile-resource-missing"
		}
		return d
	}
	// ObserveRootfile admits a Known resource only when its assessment exists.
	resource := s.resources[r.Resource.Value]
	name := resource.snapshot.header.Name
	d = packageEvidenceDecision(d, name)
	if name.State == EvidenceKnown {
		d.Outcome = OutcomePass
		d.Reason = "musicxml-root-kind"
		if !packageMusicRoot(name.Value) {
			d.Outcome = OutcomeFail
			d.Reason = "wrong-rootfile-document-kind"
		}
	}
	return d
}
func (s *PackageSession) mediaDecision(ctx context.Context, d Decision, v Evidence[string]) Decision {
	if v.State == EvidenceAbsent {
		d.Outcome = OutcomePass
		d.Reason = "absent-media-type-musicxml-default"
		return d
	}
	if v.State != EvidenceKnown {
		return packageEvidenceDecision(d, v)
	}
	media, err := ClassifyResourceMediaType(ctx, s.account, v.Value)
	if err != nil {
		s.fail(err)
		return d
	}
	if media.State != EvidenceKnown {
		return packageEvidenceDecision(d, media)
	}
	switch media.Value {
	case ResourceSourceXML:
		d.Outcome, d.Reason = OutcomePass, "registered-uncompressed-musicxml-type"
	case ResourceRawMXL:
		d.Outcome, d.Reason = OutcomePass, "registered-compressed-musicxml-type"
	case ResourceBinary:
		d.Outcome, d.Reason = OutcomeFail, "established-nonmusicxml-media-type"
	}
	return d
}

func packageRequiredEvidenceDecision[T comparable](d Decision, e Evidence[T]) Decision {
	d = packageEvidenceDecision(d, e)
	if d.Outcome == OutcomeNotApplicable || d.Outcome == OutcomePass {
		d.Outcome = OutcomeUnknown
		d.Reason = "required-evidence-unavailable"
	}
	return d
}

func (s *PackageSession) evaluateRootfile(ctx context.Context, observation PackageRootfileObservation, final bool) error {
	for _, key := range s.catalog.orders[s.profile.ID] {
		if _, ok := s.decisions[packageDecisionKey{key, observation.ID}]; ok {
			continue
		}
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		r := s.catalog.rules[key]
		if r.Selector.Kind != PackageSubjectRootfile {
			continue
		}
		if err := s.evaluateRootfileRule(ctx, r, observation, final); err != nil {
			return err
		}
	}
	return nil
}
func (s *PackageSession) evaluateRootfileRule(ctx context.Context, r PackageRuleBinding, v PackageRootfileObservation, final bool) error {
	if _, ok := s.decisions[packageDecisionKey{r.Rule.Key, v.ID}]; ok {
		return nil
	}
	d := s.baseDecision(r.Rule.Key, v.ID)
	if support := s.catalog.supports[s.profile.ID][r.Rule.Key]; support.State == ImplementationUnsupported {
		d.Outcome = OutcomeUnsupported
		d.Reason = support.Reason
	} else if pre, ok := s.prerequisites(ctx, r.Rule, v.ID); !ok {
		d = pre
	} else {
		d = s.rootfileDecision(ctx, d, v, r.Plan.check)
	}
	return s.recordPrepared(ctx, r.Rule, v.ID, d, v.Location, final)
}
