package validation

import (
	"bytes"
	"context"
	"mime"
	"strings"
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
	if err := s.admit(ctx, d, 256); err != nil {
		return err
	}
	s.decisions[k] = cloneDecision(d)
	if err := s.accumulator.storeDiagnostic(r, d, location); err != nil {
		return s.fail(err)
	}
	return nil
}
func (s *PackageSession) prerequisites(r RuleDefinition, id PackageFactID) (Decision, bool) {
	for _, dep := range r.Requires {
		d := s.baseDecision(r.Key, id)
		if dep.Kind != DependencyRule {
			d.Outcome = OutcomeUnsupported
			d.Reason = "package-capability-unavailable"
			return d, false
		}
		predecessor, ok := s.decisions[packageDecisionKey{dep.Rule, id}]
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
		r := s.catalog.rules[key]
		if r.Selector.Kind != PackageSubjectEntry {
			continue
		}
		d := s.baseDecision(key, e.ID)
		if support := s.catalog.supports[s.profile.ID][key]; support.State == ImplementationUnsupported {
			d.Outcome = OutcomeUnsupported
			d.Reason = support.Reason
		} else if pre, ok := s.prerequisites(r.Rule, e.ID); !ok {
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
				d = packageEvidenceDecision(d, e.UTF8Name)
				if e.UTF8Name.State == EvidenceKnown && !e.UTF8Name.Value {
					d.Outcome = OutcomeFail
					d.Reason = "legacy-entry-name-encoding"
				}
			case PackageCheckCompressionObservation:
				d = packageEvidenceDecision(d, e.Central.Method)
				if e.Central.Method.State == EvidenceKnown {
					d.Outcome = OutcomePass
					d.Reason = "observed-compression-method"
				}
			}
		}
		if err := s.record(ctx, r.Rule, e.ID, d, e.Location, final); err != nil {
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
			return packageEvidenceDecision(d, h.Method)
		}
		if h.Flags.State != EvidenceKnown {
			return packageEvidenceDecision(d, h.Flags)
		}
	}
	if e.Local.Method.Value != e.Central.Method.Value || e.Local.Flags.Value&1 != e.Central.Flags.Value&1 {
		d.Outcome = OutcomeUnknown
		d.Reason = "mimetype-header-inconsistency"
		return d
	}
	d.Outcome = OutcomePass
	d.Reason = "mimetype-stored-unencrypted"
	if e.Central.Method.Value != 0 || e.Central.Flags.Value&1 != 0 {
		d.Outcome = OutcomeFail
		d.Reason = "mimetype-compressed-or-encrypted"
	}
	return d
}
func packageExtraDecision(d Decision, e PackageEntryObservation) Decision {
	if e.Local.Present.State == EvidenceKnown && e.Local.Present.Value && e.Local.ExtraLength.State == EvidenceKnown && e.Local.ExtraLength.Value > 0 {
		d.Outcome = OutcomeFail
		d.Reason = "mimetype-local-extra-field"
		return d
	}
	if e.HeaderAssociation.State != EvidenceKnown || !e.HeaderAssociation.Value {
		return packageEvidenceDecision(d, e.HeaderAssociation)
	}
	if e.Local.ExtraLength.State != EvidenceKnown {
		return packageEvidenceDecision(d, e.Local.ExtraLength)
	}
	if e.Central.ExtraLength.State != EvidenceKnown {
		return packageEvidenceDecision(d, e.Central.ExtraLength)
	}
	if e.Central.ExtraLength.Value > 0 {
		d.Outcome = OutcomeUnknown
		d.Reason = "mimetype-central-extra-scope-unresolved"
	} else {
		d.Outcome = OutcomePass
		d.Reason = "mimetype-no-header-extras"
	}
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
		r := s.catalog.rules[key]
		if r.Selector.Kind != PackageSubjectPackage {
			continue
		}
		d := s.baseDecision(key, b.subject)
		if support := s.catalog.supports[s.profile.ID][key]; support.State == ImplementationUnsupported {
			d.Outcome = OutcomeUnsupported
			d.Reason = support.Reason
		} else if pre, ok := s.prerequisites(r.Rule, b.subject); !ok {
			d = pre
		} else {
			switch r.Plan.check {
			case PackageCheckContainerPath:
				if b.identity.Representation != ResourceRawMXL && b.identity.Representation != ResourceCurrentMXL {
					d.Outcome = OutcomeNotApplicable
					d.Reason = "no-package-container"
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
						if m.ByteLength.Value == uint64(len(want)) && bytes.Equal(m.Prefix, want) {
							d.Outcome = OutcomePass
							d.Reason = "mimetype-recommended-content"
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
					d = s.scoreBeforeParts(d, b)
				}
			}
		}
		if err := s.record(ctx, r.Rule, b.subject, d, s.location(b.identity.ID), b.sealed || s.finalizing); err != nil {
			return err
		}
	}
	return nil
}
func (s *PackageSession) scoreBeforeParts(d Decision, b *packageBoundary) Decision {
	roles, known := s.roles[b.identity.ID]
	if known {
		var score, part bool
		var first ResourcePin
		firstKnown := false
		for _, id := range s.rootfileOrder {
			r := s.rootfiles[id]
			if r.Package == b.identity.ID && r.Ordinal == 1 && r.Resource.State == EvidenceKnown {
				if resource := s.resources[r.Resource.Value]; resource != nil {
					first = resource.pin
					firstKnown = true
				}
			}
		}
		firstScore := Evidence[bool]{State: EvidenceUnknown, Reason: "primary-score-role-unavailable"}
		for _, r := range roles.Resources {
			if r.Membership.State == EvidenceKnown && r.Membership.Value {
				score = score || r.Role == ResourceRoleScore
				part = part || r.Role == ResourceRoleIndividualPart
			}
			if firstKnown && r.Resource == first && r.Role == ResourceRoleScore {
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
		if roles.ScopeComplete.State == EvidenceKnown && roles.ScopeComplete.Value && (!score || !part) {
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
			r := s.rootfiles[id]
			if r.Package == b.identity.ID {
				roots++
				if r.Ordinal == 1 && r.Entry.State == EvidenceKnown && r.Resource.State == EvidenceKnown {
					resource := s.resources[r.Resource.Value]
					if resource != nil && knownComplete(resource.snapshot.header.ScopeComplete) {
						primary = r.Entry.Value
					}
				}
			}
		}
		minimal := roots == 1 && primary != 0
		for _, id := range s.entryOrder {
			e := s.entries[id]
			if e.Package == b.identity.ID && e.Name != "META-INF/container.xml" && e.Name != "mimetype" && id != primary {
				minimal = false
			}
		}
		for _, id := range s.edgeOrder {
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
	media := s.mediaDecision(ctx, d, r.MediaType)
	if check == PackageCheckRootfileRoot && r.Ordinal != 1 && media.Outcome == OutcomeFail {
		d.Outcome = OutcomeNotApplicable
		d.Reason = "nonmusicxml-alternate"
		return d
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
	resource := s.resources[r.Resource.Value]
	if resource == nil {
		return d
	}
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
	if err := s.account.Charge(BudgetRuleSteps, int64(len(v.Value))+1); err != nil {
		s.fail(err)
		return d
	}
	if ctx.Err() != nil {
		s.fail(ctx.Err())
		return d
	}
	if err := s.retain(int64(len(v.Value))*4 + 1024); err != nil {
		return d
	}
	typ, parameters, err := mime.ParseMediaType(v.Value)
	if err != nil {
		d.Outcome = OutcomeUnknown
		d.Reason = "media-type-syntax-unresolved"
		return d
	}
	typ = strings.ToLower(typ)
	if typ == "application/vnd.recordare.musicxml+xml" {
		for key := range parameters {
			if key != "charset" {
				d.Outcome = OutcomeUnknown
				d.Reason = "musicxml-media-parameter-unresolved"
				return d
			}
		}
		d.Outcome = OutcomePass
		d.Reason = "registered-uncompressed-musicxml-type"
		return d
	}
	if typ == "application/vnd.recordare.musicxml" {
		d.Outcome = OutcomePass
		d.Reason = "registered-compressed-musicxml-type"
		return d
	}
	if typ == "application/pdf" || strings.HasPrefix(typ, "image/") || strings.HasPrefix(typ, "audio/") || strings.HasPrefix(typ, "video/") {
		d.Outcome = OutcomeFail
		d.Reason = "established-nonmusicxml-media-type"
		return d
	}
	d.Outcome = OutcomeUnknown
	d.Reason = "musicxml-media-type-set-unresolved"
	return d
}
