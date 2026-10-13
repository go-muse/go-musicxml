package validation

import (
	"context"
	"reflect"
)

type bundleResource struct {
	id       ResourceAssessmentID
	report   Report
	required bool
}
type bundleAdoption struct {
	context    ResourceAdoptionContext
	resource   ResourceAssessmentID
	documentID string
}
type assessmentBundleData struct {
	report             Report
	omittedDiagnostics *packageDiagnosticSummaryData
	resources          []bundleResource
	adoptions          []bundleAdoption
	enumeration        bool
	failure            *ResourceFailureSummary
	failureReport      *Report
	images             []packageImageAdvice
	files              []PackageFileMetadataObservation
	mimetypes          []PackageMimetypeObservation
}

// AssessmentBundle is an immutable core-created collection of one package/graph
// report, canonical intrinsic reports and fresh adoption metadata. Counts remain
// on individual reports and never multiply a child's failure per referring edge.
// Getters return independent API copies and are safe for concurrent readers.
type AssessmentBundle struct{ data *assessmentBundleData }

// Satisfied requires every required intrinsic report, the package report and
// complete selected enumeration. A zero bundle and a failure summary cannot pass.
func (b AssessmentBundle) Satisfied() bool {
	if b.data == nil || !b.data.enumeration || b.data.failure != nil || !b.data.report.Satisfied() {
		return false
	}
	for _, r := range b.data.resources {
		if r.required && !r.report.Satisfied() {
			return false
		}
	}
	return true
}

// PackageReport returns an owned copy with its original package profile/target.
func (b AssessmentBundle) PackageReport() Report {
	if b.data == nil {
		return Report{}
	}
	return stateClone(b.data.report)
}

// ResourceCount is the number of distinct canonical intrinsic assessments.
func (b AssessmentBundle) ResourceCount() int {
	if b.data == nil {
		return 0
	}
	return len(b.data.resources)
}

// ResourceReport returns the canonical report once, preserving target/profile.
func (b AssessmentBundle) ResourceReport(i int) (ResourceAssessmentID, Report, bool) {
	if b.data == nil || i < 0 || i >= len(b.data.resources) {
		return "", Report{}, false
	}
	r := b.data.resources[i]
	return r.id, stateClone(r.report), true
}

// AdoptionCount includes the primary root and each distinct linked occurrence.
func (b AssessmentBundle) AdoptionCount() int {
	if b.data == nil {
		return 0
	}
	return len(b.data.adoptions)
}

// Adoption returns canonical assessment identity and fresh occurrence identity.
func (b AssessmentBundle) Adoption(i int) (ResourceAssessmentID, string, bool) {
	if b.data == nil || i < 0 || i >= len(b.data.adoptions) {
		return "", "", false
	}
	v := b.data.adoptions[i]
	return v.resource, v.documentID, true
}

// EnumerationComplete reports the selected finite graph/package scope boundary.
func (b AssessmentBundle) EnumerationComplete() bool { return b.data != nil && b.data.enumeration }

// Failure returns a bounded explicitly truncated child failure summary when
// full child retention was refused. It never masquerades as a complete Report.
func (b AssessmentBundle) Failure() (ResourceFailureSummary, bool) {
	if b.data == nil || b.data.failure == nil {
		return ResourceFailureSummary{}, false
	}
	return stateClone(*b.data.failure), true
}

// FailureReport returns the original retained failed-child report when ordinary
// admission succeeded before the package became terminal. Post-terminal failure
// ingress never attempts a new full copy, even if storage remains available;
// Failure retains the bounded summary instead.
func (b AssessmentBundle) FailureReport() (Report, bool) {
	if b.data == nil || b.data.failureReport == nil {
		return Report{}, false
	}
	return stateClone(*b.data.failureReport), true
}

// ResourceAdoption is a cheap package-owned immutable view. Its accessors rebind
// every NodeRef, causal reference and location inside copied values. It is valid
// only until the owning package finishes and must not be used concurrently with
// that single-goroutine session. It never owns a duplicate intrinsic assessment.
type ResourceAdoption struct {
	owner *PackageSession
	value *packageAdoption
}

// Adoption obtains a package-owned view without exposing a transferred snapshot.
func (s *PackageSession) Adoption(ctx context.Context, id ResourceAdoptionID) (*ResourceAdoption, error) {
	if err := s.event(ctx, "Adoption"); err != nil {
		return nil, err
	}
	a := s.adoptions[id]
	if a == nil {
		return nil, s.fail(contract("adoption", "unknown adoption identity"))
	}
	return &ResourceAdoption{owner: s, value: a}, nil
}
func (a *ResourceAdoption) live() error {
	if a == nil || a.owner == nil || a.owner.finished || a.value == nil || a.value.resource.snapshot.closed {
		return &SessionError{Operation: "adoption read", Reason: "closed or uninitialized adoption"}
	}
	return nil
}

// Header returns zero after package completion; otherwise it is an owned copy.
func (a *ResourceAdoption) Header() ResourceSnapshotHeader {
	if a.live() != nil {
		return ResourceSnapshotHeader{}
	}
	h := stateClone(a.value.resource.snapshot.header)
	from := h.DocumentID
	h.DocumentID = a.value.documentID
	resourceRemap(reflect.ValueOf(&h).Elem(), from, a.value.documentID)
	return h
}

// ContextCount returns selected template count, or zero after completion.
func (a *ResourceAdoption) ContextCount() int {
	if a.live() != nil {
		return 0
	}
	return len(a.value.resource.snapshot.contexts)
}

// IDCount returns selected ID occurrence count, or zero after completion.
func (a *ResourceAdoption) IDCount() int {
	if a.live() != nil {
		return 0
	}
	return len(a.value.resource.snapshot.ids)
}

// ContextAt returns an owned, fully document-rebound projection copy.
func (a *ResourceAdoption) ContextAt(ctx context.Context, i int) (ContextResult, bool, error) {
	if e := a.live(); e != nil {
		return ContextResult{}, false, e
	}
	if ctx == nil {
		return ContextResult{}, false, contract("context", "must not be nil")
	}
	if e := ctx.Err(); e != nil {
		return ContextResult{}, false, e
	}
	v := a.value.resource.snapshot
	if i < 0 || i >= len(v.contexts) {
		return ContextResult{}, false, nil
	}
	out, err := resourceLiveCopy(ctx, a.owner.account, v.contexts[i], v.header.DocumentID, a.value.documentID)
	return out, err == nil, err
}

// IDAt returns a copied exact ID state with remapped owners and causal origins.
func (a *ResourceAdoption) IDAt(ctx context.Context, i int) (ResourceIDFact, bool, error) {
	if e := a.live(); e != nil {
		return ResourceIDFact{}, false, e
	}
	if ctx == nil {
		return ResourceIDFact{}, false, contract("context", "must not be nil")
	}
	if e := ctx.Err(); e != nil {
		return ResourceIDFact{}, false, e
	}
	v := a.value.resource.snapshot
	if i < 0 || i >= len(v.ids) {
		return ResourceIDFact{}, false, nil
	}
	out, err := resourceLiveCopy(ctx, a.owner.account, v.ids[i], v.header.DocumentID, a.value.documentID)
	return out, err == nil, err
}

// Report returns an owned remapped display report without changing canonical
// counts or storing a report copy for every adoption.
func (a *ResourceAdoption) Report(ctx context.Context) (Report, error) {
	if e := a.live(); e != nil {
		return Report{}, e
	}
	if ctx == nil {
		return Report{}, contract("context", "must not be nil")
	}
	if e := ctx.Err(); e != nil {
		return Report{}, e
	}
	v := a.value.resource.snapshot
	return resourceLiveCopy(ctx, a.owner.account, v.report, v.header.DocumentID, a.value.documentID)
}

// Finish is terminal even when canceled. It preserves admitted known findings,
// publishes one immutable bundle and releases all live owners exactly once.
func (s *PackageSession) Finish(ctx context.Context, end PackageCompletion) (AssessmentBundle, error) {
	if s == nil || s.finished || s.account == nil || s.accumulator == nil {
		return AssessmentBundle{}, &SessionError{Operation: "package Finish", Reason: "already finished or uninitialized"}
	}
	defer s.cleanup()
	s.finalizing = true
	if ctx == nil {
		s.fail(contract("context", "must not be nil"))
	} else {
		s.fail(ctx.Err())
	}
	s.fail(end.Cause)
	if s.terminal == nil {
		for _, id := range s.boundaryOrder {
			if err := s.evaluateBoundary(ctx, s.boundaries[id]); err != nil {
				break
			}
		}
	}
	if s.terminal == nil {
		for _, id := range s.entryOrder {
			if err := s.evaluateEntry(ctx, s.entries[id], true); err != nil {
				break
			}
		}
	}
	if s.terminal == nil {
		for _, key := range s.catalog.orders[s.profile.ID] {
			r := s.catalog.rules[key]
			switch r.Selector.Kind {
			case PackageSubjectRootfile:
				for _, id := range s.rootfileOrder {
					if _, ok := s.decisions[packageDecisionKey{r.Rule.Key, id}]; ok {
						continue
					}
					if err := s.scan(ctx, 1); err != nil {
						break
					}
					if err := s.evaluateRootfileRule(ctx, r, s.rootfiles[id], true); err != nil {
						break
					}
				}
			case PackageSubjectImage, PackageSubjectMetadata:
				if err := s.evaluateAdvice(ctx, r); err != nil {
					s.fail(err)
				}
			case PackageSubjectEdge, PackageSubjectAssociation, PackageSubjectResource:
				if err := s.evaluateResourceRule(ctx, r); err != nil {
					s.fail(err)
				}
			}
			if s.terminal != nil {
				break
			}
		}
	}
	complete := end.InputComplete && len(s.boundaries) > 0 && s.terminal == nil
	for _, id := range s.boundaryOrder {
		b := s.boundaries[id]
		complete = complete && !b.identity.PinUnavailable && b.sealed && knownComplete(b.enumeration.Entries) && knownComplete(b.enumeration.Rootfiles) && knownComplete(b.enumeration.Edges) && knownComplete(b.enumeration.Roles)
	}
	for _, id := range s.adoptionOrder {
		a := s.adoptions[id]
		complete = complete && a.expansion.sealed && knownComplete(a.expansion.complete)
	}
	if !complete {
		s.accumulator.incomplete()
	}
	if s.terminal != nil {
		category, reason := terminalClassification(s.terminal, false)
		s.accumulator.terminal(s.terminal, category, reason)
	}
	out := s.output
	out.report = s.accumulator.finish(end.InputComplete, SyntaxNotAssessed)
	out.enumeration = complete
	out.failure = s.failure
	// Canonical report payloads were independently owned and admitted at resource
	// finalization. These private immutable records transfer to the final bundle;
	// release clears the snapshot field, never mutates the transferred backing data.
	for i, id := range s.resourceOrder {
		out.resources[i].required = s.resources[id].required
	}
	for i, id := range s.adoptionOrder {
		a := s.adoptions[id]
		out.adoptions[i].context.ExpansionComplete = a.expansion.complete
	}
	bundle := AssessmentBundle{data: out}
	s.finished = true
	if s.terminal != nil {
		return bundle, &IncompleteError{Reason: "package-terminated", Cause: s.terminal}
	}
	return bundle, nil
}
func (s *PackageSession) cleanup() {
	if s == nil {
		return
	}
	s.finished = true
	for _, r := range s.resources {
		r.snapshot.release()
	}
	if s.retained > 0 {
		_ = s.account.Release(BudgetBufferedBytes, s.retained)
		s.retained = 0
	}
	s.accumulator = nil
	s.boundaries = nil
	s.entries = nil
	s.entryNames = nil
	s.rawEntryNames = nil
	s.mimetypes = nil
	s.rootfiles = nil
	s.edges = nil
	s.associations = nil
	s.roles = nil
	s.resources = nil
	s.resourceKeys = nil
	s.pins = nil
	s.adoptions = nil
	s.expansions = nil
	s.documents = nil
	s.subjects = nil
	s.subjectPackages = nil
	s.decisions = nil
	s.output = nil
	s.failure = nil
}
