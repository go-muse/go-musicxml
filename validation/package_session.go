package validation

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const packageTerminalReserveBytes int64 = 16384

type packageBoundary struct {
	identity        PackageIdentity
	subject         PackageFactID
	depth           int64
	enumeration     PackageEnumeration
	sealed          bool
	membersSealed   bool
	membersComplete Evidence[bool]
}
type packageDecisionKey struct {
	rule    RuleKey
	subject PackageFactID
}
type packageResourceKey struct {
	pin     ResourcePin
	profile ProfileID
	digest  string
}
type packageCanonicalKey struct {
	canonical, revision string
	representation      ResourceRepresentation
}
type packageResource struct {
	id       ResourceAssessmentID
	pin      ResourcePin
	snapshot *ResourceSnapshot
	required bool
	subject  PackageFactID
}
type packageAdoption struct {
	id         ResourceAdoptionID
	resource   *packageResource
	documentID string
	root       bool
	packageID  PackageID
	expansion  *resourceExpansion
}
type packageAssociation struct {
	id      PackageFactID
	request AssociationRequest
	result  AssociationResult
}

// PackageSession is a separate single-goroutine typed coordinator. It has no
// parser, resolver invocation, XML Open method, arbitrary facts or callbacks.
// Every ingress error is terminal; Finish must run once even after cancellation.
type PackageSession struct {
	catalog         *CompiledPackageCatalog
	profile         Profile
	documentID      string
	account         *Account
	lease           resourceLeaseOwner
	accumulator     *reportAccumulator
	boundaries      map[PackageID]*packageBoundary
	boundaryOrder   []PackageID
	entries         map[PackageFactID]PackageEntryObservation
	entryOrder      []PackageFactID
	entryNames      map[PackageID]map[string]PackageFactID
	rawEntryNames   map[PackageID]map[string]PackageFactID
	mimetypes       map[PackageFactID]PackageMimetypeObservation
	rootfiles       map[PackageFactID]PackageRootfileObservation
	rootfileOrder   []PackageFactID
	edges           map[PackageFactID]PackageEdgeObservation
	edgeOrder       []PackageFactID
	associations    []packageAssociation
	roles           map[PackageID]ResourceRoleResult
	resources       map[ResourceAssessmentID]*packageResource
	resourceOrder   []ResourceAssessmentID
	resourceKeys    map[packageResourceKey]ResourceAssessmentID
	pins            map[packageCanonicalKey]ResourcePin
	adoptions       map[ResourceAdoptionID]*packageAdoption
	expansions      map[resourceExpansionKey]*resourceExpansion
	adoptionOrder   []ResourceAdoptionID
	documents       map[string]bool
	subjects        map[PackageFactID]PackageSubjectKind
	subjectPackages map[PackageFactID]PackageID
	decisions       map[packageDecisionKey]Decision
	nextInternal    PackageFactID
	rootAdoption    bool
	retained        int64
	terminal        error
	finished        bool
	finalizing      bool
	output          *assessmentBundleData
	failure         *ResourceFailureSummary
}

// Begin validates configuration/cancellation before allocating a package run.
// A supplied Account is shared by identity, never copied or replaced.
func (c *CompiledPackageCatalog) Begin(ctx context.Context, opts RunOptions) (*PackageSession, error) {
	if ctx == nil {
		return nil, contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, &IncompleteError{Reason: "canceled-before-package", Cause: err}
	}
	if c == nil || !validID(opts.DocumentID) {
		return nil, contract("package begin", "compiled catalog and explicit document identity required")
	}
	binding, ok := c.profiles[opts.Profile]
	if !ok {
		return nil, contract("package profile", "unknown explicit profile")
	}
	a := opts.Account
	if a == nil {
		var err error
		a, err = NewBudgetAccount(opts.Budgets)
		if err != nil {
			return nil, err
		}
	} else {
		if opts.Budgets != (Budgets{}) {
			return nil, contract("package account", "shared account forbids replacement limits")
		}
		if _, err := a.Usage(BudgetNodes); err != nil {
			return nil, err
		}
	}
	reserve := packageTerminalReserveBytes + packageDiagnosticReserveBytes + 4096 + profileLogicalBytes(binding.Profile)
	if err := a.Charge(BudgetBufferedBytes, reserve); err != nil {
		return nil, err
	}
	s := &PackageSession{catalog: c, profile: binding.Profile, documentID: strings.Clone(opts.DocumentID), account: a, retained: reserve, nextInternal: ^PackageFactID(0), boundaries: map[PackageID]*packageBoundary{}, entries: map[PackageFactID]PackageEntryObservation{}, entryNames: map[PackageID]map[string]PackageFactID{}, rawEntryNames: map[PackageID]map[string]PackageFactID{}, mimetypes: map[PackageFactID]PackageMimetypeObservation{}, rootfiles: map[PackageFactID]PackageRootfileObservation{}, edges: map[PackageFactID]PackageEdgeObservation{}, roles: map[PackageID]ResourceRoleResult{}, resources: map[ResourceAssessmentID]*packageResource{}, resourceKeys: map[packageResourceKey]ResourceAssessmentID{}, pins: map[packageCanonicalKey]ResourcePin{}, adoptions: map[ResourceAdoptionID]*packageAdoption{}, expansions: map[resourceExpansionKey]*resourceExpansion{}, documents: map[string]bool{opts.DocumentID: true}, subjects: map[PackageFactID]PackageSubjectKind{}, subjectPackages: map[PackageFactID]PackageID{}, decisions: map[packageDecisionKey]Decision{}}
	s.accumulator = newReportAccumulator(binding.Profile, a)
	s.output = &assessmentBundleData{omittedDiagnostics: &packageDiagnosticSummaryData{}}
	return s, nil
}

// TerminalCause returns the accumulated terminal error without charging work or
// changing session state. It remains available after Finish; nil means no terminal
// error was recorded. Ordinary ingress is still refused after a terminal error.
func (s *PackageSession) TerminalCause() error {
	if s == nil {
		return nil
	}
	return s.terminal
}

func (s *PackageSession) event(ctx context.Context, op string) error {
	if s == nil || s.finished || s.account == nil {
		return &SessionError{Operation: op, Reason: "closed or uninitialized package"}
	}
	if s.terminal != nil {
		return &SessionError{Operation: op, Reason: "package already terminated", Cause: s.terminal}
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
	return nil
}
func (s *PackageSession) fail(err error) error {
	if s != nil && err != nil {
		if s.terminal == nil {
			s.terminal = err
		} else if !errors.Is(s.terminal, err) {
			s.terminal = errors.Join(s.terminal, err)
		}
	}
	return err
}
func (s *PackageSession) retain(n int64) error {
	if _, err := AllocationSize(n); err != nil {
		return s.fail(err)
	}
	if err := s.account.Charge(BudgetBufferedBytes, n); err != nil {
		return s.fail(err)
	}
	s.retained += n
	return nil
}
func (s *PackageSession) admit(ctx context.Context, v any, extra int64) error {
	n, err := resourceMeasure(ctx, s.account, reflect.ValueOf(v))
	if err != nil {
		return s.fail(err)
	}
	n, err = rationalCheckedAdd(BudgetBufferedBytes, n, extra)
	if err != nil {
		return s.fail(err)
	}
	if err = s.account.Charge(BudgetContextFacts, 1); err != nil {
		return s.fail(err)
	}
	return s.retain(n)
}
func (s *PackageSession) subject(id PackageFactID, kind PackageSubjectKind) error {
	if id == 0 || uint64(id)>>63 != 0 {
		return contract("package subject", "adapter identity must be positive low-63-bit")
	}
	if _, ok := s.subjects[id]; ok {
		return contract("package subject", "duplicate identity")
	}
	s.subjects[id] = kind
	return nil
}
func (s *PackageSession) internalSubject(kind PackageSubjectKind) PackageFactID {
	id := s.nextInternal
	s.nextInternal--
	s.subjects[id] = kind
	return id
}
func (s *PackageSession) location(pkg PackageID) Location {
	return Location{Target: TargetPackage, DocumentID: s.documentID, Path: "/package[" + string(pkg) + "]"}
}

// OpenPackage admits one acyclic encapsulation boundary, keeping sibling entry
// namespaces distinct. Source/current graph boundaries make no raw ZIP claim.
func (s *PackageSession) OpenPackage(ctx context.Context, p PackageIdentity) error {
	if err := s.event(ctx, "OpenPackage"); err != nil {
		return err
	}
	if !validID(string(p.ID)) || (p.Parent != "" && !validID(string(p.Parent))) || p.ID == p.Parent {
		return s.fail(contract("package identity", "invalid boundary identity"))
	}
	if !validText(p.BaseURI, 16384) || !validText(p.ArchiveEntry, 4096) {
		return s.fail(contract("package provenance", "bounded exact context required"))
	}
	if p.ArchiveEntry != "" && (p.Parent == "" || s.entryNames[p.Parent][p.ArchiveEntry] == 0) {
		return s.fail(contract("package provenance", "exact admitted parent member required"))
	}
	if p.PinUnavailable {
		if p.Representation != ResourceRawMXL || p.Pin.Digest != "" || p.Pin.SnapshotID != "" || !packageIdentityText(p.Pin.CanonicalID) || !validText(p.Pin.Revision, maxIdentityBytes) {
			return s.fail(contract("package prefix identity", "raw incomplete boundary needs unavailable whole-body pin"))
		}
	} else if err := validateResourcePin(p.Pin); err != nil {
		return s.fail(err)
	}
	if p.Representation != p.Pin.Representation || p.Representation == ResourceBinary {
		return s.fail(contract("package identity", "representation mismatch"))
	}
	if _, ok := s.boundaries[p.ID]; ok {
		return s.fail(contract("package identity", "duplicate boundary"))
	}
	depth := int64(1)
	if p.Parent != "" {
		parent := s.boundaries[p.Parent]
		if parent == nil || parent.sealed {
			return s.fail(contract("package parent", "parent must already be admitted"))
		}
		depth = parent.depth + 1
		for ancestor := parent; ancestor != nil; ancestor = s.boundaries[ancestor.identity.Parent] {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			if !p.PinUnavailable && !ancestor.identity.PinUnavailable && ancestor.identity.Pin == p.Pin {
				return s.fail(contract("package ancestry", "immutable package cannot contain itself under a fresh boundary ID"))
			}
		}
	} else if len(s.boundaries) != 0 {
		return s.fail(contract("package root", "only one root boundary"))
	}
	if depth > s.account.Limits().MaxDepth {
		return s.fail(&ResourceError{Kind: BudgetDepth, Limit: s.account.Limits().MaxDepth, Requested: depth})
	}
	key := packageCanonicalKey{p.Pin.CanonicalID, p.Pin.Revision, p.Pin.Representation}
	if old, ok := s.pins[key]; !p.PinUnavailable && ok && old != p.Pin {
		return s.fail(contract("package pin", "canonical identity content conflict"))
	}
	if err := s.admit(ctx, p, 1024); err != nil {
		return err
	}
	p = stateClone(p)
	b := &packageBoundary{identity: p, depth: depth, subject: s.internalSubject(PackageSubjectPackage)}
	s.boundaries[p.ID] = b
	s.subjectPackages[b.subject] = p.ID
	s.boundaryOrder = append(s.boundaryOrder, p.ID)
	s.entryNames[p.ID] = map[string]PackageFactID{}
	s.rawEntryNames[p.ID] = map[string]PackageFactID{}
	if !p.PinUnavailable {
		s.pins[key] = p.Pin
	}
	return nil
}
func validateZIPHeader(h ZIPHeaderObservation) error {
	if err := validateEvidence(h.Present, "ZIP header presence"); err != nil {
		return err
	}
	if h.Present.State == EvidenceKnown && !h.Present.Value {
		return contract("ZIP header presence", "use absent evidence")
	}
	if err := validateEvidence(h.Method, "ZIP method"); err != nil {
		return err
	}
	if err := validateEvidence(h.Flags, "ZIP flags"); err != nil {
		return err
	}
	if err := validateEvidence(h.ExtraLength, "ZIP extra length"); err != nil {
		return err
	}
	// Unknown framing can retain independent field observations; proven absence cannot.
	if h.Present.State == EvidenceAbsent && (h.Method.State == EvidenceKnown || h.Flags.State == EvidenceKnown || h.ExtraLength.State == EvidenceKnown) {
		return contract("ZIP header", "absent header cannot carry known fields")
	}
	return nil
}

// ObserveEntry copies exact independent local/central observations. It records
// already proven local failures immediately, so later cancellation cannot erase
// them; global enumeration is assessed independently at terminal completion.
func (s *PackageSession) ObserveEntry(ctx context.Context, e PackageEntryObservation) error {
	if err := s.event(ctx, "ObserveEntry"); err != nil {
		return err
	}
	b := s.boundaries[e.Package]
	if b == nil || b.sealed {
		return s.fail(contract("entry package", "unopened or sealed scope"))
	}
	rawName := len(e.RawName) != 0
	if rawName {
		if len(e.RawName) > 4096 {
			return s.fail(contract("entry raw name", "bounded bytes required"))
		}
		if err := s.account.Check(BudgetScalarBytes, int64(len(e.RawName))); err != nil {
			return s.fail(err)
		}
		if err := s.scan(ctx, int64(len(e.RawName))); err != nil {
			return err
		}
		if e.Name != "" || utf8.Valid(e.RawName) || b.identity.Representation != ResourceRawMXL {
			return s.fail(contract("entry raw name", "bounded original non-UTF8 bytes exclusive with text required"))
		}
	} else if !validText(e.Name, 4096) || e.Name == "" {
		return s.fail(contract("entry name", "bounded nonempty exact name required"))
	}
	if err := validateLocation(e.Location, TargetPackage, s.documentID); err != nil {
		return s.fail(err)
	}
	if err := validateZIPHeader(e.Local); err != nil {
		return s.fail(err)
	}
	if err := validateZIPHeader(e.Central); err != nil {
		return s.fail(err)
	}
	if err := validateEvidence(e.HeaderAssociation, "header association"); err != nil {
		return s.fail(err)
	}
	if err := validateEvidence(e.UTF8Name, "name encoding"); err != nil {
		return s.fail(err)
	}
	if rawName && (e.UTF8Name.State != EvidenceKnown || e.UTF8Name.Value) {
		return s.fail(contract("entry raw name", "exact observed non-UTF8 evidence required"))
	}
	if err := validateEvidence(e.PhysicalOrder, "physical order"); err != nil {
		return s.fail(err)
	}
	if rawName {
		if _, ok := s.rawEntryNames[e.Package][string(e.RawName)]; ok {
			return s.fail(contract("entry identity", "duplicate exact raw name"))
		}
	} else if _, ok := s.entryNames[e.Package][e.Name]; ok {
		return s.fail(contract("entry identity", "duplicate exact member name"))
	}
	// The raw-name index owns a distinct immutable string key. Admit its bytes
	// before conversion rather than borrowing the caller's mutable byte slice.
	extra := int64(512)
	if rawName {
		extra += int64(len(e.RawName))
	}
	if err := s.admit(ctx, e, extra); err != nil {
		return err
	}
	if err := s.subject(e.ID, PackageSubjectEntry); err != nil {
		return s.fail(err)
	}
	e = stateClone(e)
	s.entries[e.ID] = e
	s.subjectPackages[e.ID] = e.Package
	s.entryOrder = append(s.entryOrder, e.ID)
	if rawName {
		s.rawEntryNames[e.Package][string(e.RawName)] = e.ID
	} else {
		s.entryNames[e.Package][e.Name] = e.ID
	}
	if err := s.evaluateEntry(ctx, e, false); err != nil {
		return err
	}
	return s.evaluateBoundary(ctx, s.boundaries[e.Package])
}

// ObserveMimetype copies at most 64 original initial bytes, never stripped or
// transcoded text. Prefix and length consistency is validated before copying.
func (s *PackageSession) ObserveMimetype(ctx context.Context, m PackageMimetypeObservation) error {
	if err := s.event(ctx, "ObserveMimetype"); err != nil {
		return err
	}
	e, ok := s.entries[m.Entry]
	if !ok || e.Name != "mimetype" || s.boundaries[e.Package].sealed {
		return s.fail(contract("mimetype", "exact admitted mimetype entry required"))
	}
	if _, ok = s.mimetypes[m.Entry]; ok {
		return s.fail(contract("mimetype", "duplicate byte observation"))
	}
	if len(m.Prefix) > 64 {
		return s.fail(contract("mimetype prefix", "maximum 64 original bytes"))
	}
	if err := validateEvidence(m.ByteLength, "mimetype length"); err != nil {
		return s.fail(err)
	}
	if err := validateEvidence(m.ContentComplete, "mimetype completeness"); err != nil {
		return s.fail(err)
	}
	if m.ByteLength.State == EvidenceKnown && uint64(len(m.Prefix)) > m.ByteLength.Value {
		return s.fail(contract("mimetype prefix", "prefix exceeds measured length"))
	}
	if err := s.admit(ctx, m, 256); err != nil {
		return err
	}
	s.mimetypes[m.Entry] = stateClone(m)
	s.output.mimetypes = append(s.output.mimetypes, s.mimetypes[m.Entry])
	if err := s.evaluateEntry(ctx, e, false); err != nil {
		return err
	}
	return s.evaluateBoundary(ctx, s.boundaries[e.Package])
}

// ObserveRootfile preserves actual encoded order and causal scalar states.
func (s *PackageSession) ObserveRootfile(ctx context.Context, r PackageRootfileObservation) error {
	if err := s.event(ctx, "ObserveRootfile"); err != nil {
		return err
	}
	b := s.boundaries[r.Package]
	if b == nil || b.sealed || r.Ordinal == 0 {
		return s.fail(contract("rootfile", "exact open scope and positive ordinal required"))
	}
	if err := validateLocation(r.Location, TargetPackage, s.documentID); err != nil {
		return s.fail(err)
	}
	if !validResourceNode(r.Source) && !(b.identity.Representation == ResourceCurrentMXL && r.Source == (NodeRef{}) && r.Location.Path != "") {
		return s.fail(contract("rootfile source", "actual container occurrence required"))
	}
	for _, v := range []Evidence[string]{r.FullPath, r.MediaType} {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if err := validateEvidence(v, "rootfile value"); err != nil {
			return s.fail(err)
		}
		if v.State == EvidenceKnown && !validText(v.Value, 4096) {
			return s.fail(contract("rootfile value", "excessive token"))
		}
	}
	if err := validateEvidence(r.Entry, "rootfile entry"); err != nil {
		return s.fail(err)
	}
	if err := validateEvidence(r.Resource, "rootfile resource"); err != nil {
		return s.fail(err)
	}
	if err := validateEvidence(r.Indirection, "rootfile indirection"); err != nil {
		return s.fail(err)
	}
	if r.Entry.State == EvidenceKnown {
		e, ok := s.entries[r.Entry.Value]
		if !ok || len(e.RawName) != 0 || e.Package != r.Package || r.FullPath.State != EvidenceKnown || e.Name != r.FullPath.Value {
			return s.fail(contract("rootfile entry", "exact same-package full-path association required"))
		}
	}
	if r.Resource.State == EvidenceKnown {
		resource := s.resources[r.Resource.Value]
		if resource == nil {
			return s.fail(contract("rootfile resource", "unknown assessment identity"))
		}
	}
	for _, id := range s.rootfileOrder {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if old := s.rootfiles[id]; old.Package == r.Package && old.Ordinal == r.Ordinal {
			return s.fail(contract("rootfile order", "duplicate encoded ordinal"))
		}
	}
	if err := s.admit(ctx, r, 512); err != nil {
		return err
	}
	if err := s.subject(r.ID, PackageSubjectRootfile); err != nil {
		return s.fail(err)
	}
	if r.Resource.State == EvidenceKnown {
		s.resources[r.Resource.Value].required = true
	}
	s.rootfiles[r.ID] = stateClone(r)
	s.subjectPackages[r.ID] = r.Package
	s.rootfileOrder = append(s.rootfileOrder, r.ID)
	if err := s.evaluateRootfile(ctx, r, false); err != nil {
		return err
	}
	return s.evaluateBoundary(ctx, b)
}

// SealEnumeration records each independent complete/incomplete scope exactly once.
func (s *PackageSession) SealEnumeration(ctx context.Context, e PackageEnumeration) error {
	if err := s.event(ctx, "SealEnumeration"); err != nil {
		return err
	}
	b := s.boundaries[e.Package]
	if b == nil || b.sealed {
		return s.fail(contract("package enumeration", "unopened or already sealed boundary"))
	}
	for _, v := range []Evidence[bool]{e.Entries, e.Rootfiles, e.Edges, e.Roles} {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if err := validateEvidence(v, "package enumeration"); err != nil {
			return s.fail(err)
		}
		if v.State == EvidenceAbsent {
			return s.fail(contract("package enumeration", "absence is not completeness"))
		}
	}
	if err := s.admit(ctx, e, 128); err != nil {
		return err
	}
	b.enumeration = stateClone(e)
	b.sealed = true
	return s.evaluateBoundary(ctx, b)
}

// AddResource transfers an immutable core-created snapshot on the same Account.
// Rejected transfers preserve its sole caller owner; no arbitrary Report can
// certify success. A duplicate identical transferred pointer is idempotent.
func (s *PackageSession) AddResource(ctx context.Context, pin ResourcePin, snapshot *ResourceSnapshot, required bool) (ResourceAssessmentID, error) {
	if err := s.event(ctx, "AddResource"); err != nil {
		return "", err
	}
	if err := validateResourcePin(pin); err != nil {
		return "", s.fail(err)
	}
	if snapshot == nil || snapshot.closed || snapshot.account != s.account {
		return "", s.fail(contract("resource transfer", "live core snapshot on identical account required"))
	}
	key := packageResourceKey{pin, snapshot.header.Profile, snapshot.header.CatalogDigest}
	if id, ok := s.resourceKeys[key]; ok {
		old := s.resources[id]
		if old.snapshot == snapshot && snapshot.owner == &s.lease {
			old.required = old.required || required
			return id, nil
		}
		return "", s.fail(contract("resource transfer", "different snapshot for canonical assessment"))
	}
	if snapshot.owner != nil {
		return "", s.fail(contract("resource transfer", "snapshot already owned"))
	}
	if pin.Representation == ResourceSourceXML && snapshot.header.Target != TargetSource || pin.Representation == ResourceCurrentModel && snapshot.header.Target != TargetModel || pin.Representation != ResourceSourceXML && pin.Representation != ResourceCurrentModel {
		return "", s.fail(contract("resource transfer", "pin and intrinsic representation mismatch"))
	}
	allowed := false
	for _, b := range s.catalog.profiles[s.profile.ID].Intrinsic {
		if err := s.scan(ctx, 1); err != nil {
			return "", err
		}
		if (snapshot.header.Assembly.State == EvidenceKnown && b.Assembly == snapshot.header.Assembly.Value || snapshot.header.Assembly.State != EvidenceKnown && !snapshot.report.Satisfied()) && b.Target == snapshot.header.Target && b.Profile == snapshot.header.Profile && b.CatalogDigest == snapshot.header.CatalogDigest {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", s.fail(contract("resource transfer", "profile/digest/assembly not in fixed intrinsic table"))
	}
	canonical := packageCanonicalKey{pin.CanonicalID, pin.Revision, pin.Representation}
	if old, ok := s.pins[canonical]; ok && old != pin {
		return "", s.fail(contract("resource pin", "canonical content changed"))
	}
	if s.documents[snapshot.header.DocumentID] {
		return "", s.fail(contract("resource document", "canonical occurrence already used"))
	}
	if err := s.admit(ctx, pin, 1024); err != nil {
		return "", err
	}
	id := ResourceAssessmentID("resource-" + strconv.Itoa(len(s.resourceOrder)+1))
	r := &packageResource{id: id, pin: stateClone(pin), snapshot: snapshot, required: required, subject: s.internalSubject(PackageSubjectResource)}
	s.resources[id] = r
	s.resourceOrder = append(s.resourceOrder, id)
	s.output.resources = append(s.output.resources, bundleResource{id: id, report: snapshot.report, required: required})
	s.resourceKeys[key] = id
	s.pins[canonical] = r.pin
	s.documents[snapshot.header.DocumentID] = true
	snapshot.owner = &s.lease
	return id, nil
}

// AdoptResource allocates one distinct occurrence view and charges every linked
// adoption, including cache hits and back-edges. Root is explicit and unique.
func (s *PackageSession) AdoptResource(ctx context.Context, id ResourceAssessmentID, opts ResourceAdoptionOptions) (ResourceAdoptionID, error) {
	if err := s.event(ctx, "AdoptResource"); err != nil {
		return "", err
	}
	r := s.resources[id]
	if s.boundaries[opts.Package] == nil || s.boundaries[opts.Package].sealed {
		return "", s.fail(contract("resource adoption", "explicit admitted package scope required"))
	}
	if r == nil || !validID(opts.DocumentID) || !validText(opts.BaseURI, 16384) || !validText(opts.ArchiveEntry, 4096) {
		return "", s.fail(contract("resource adoption", "known resource and explicit document identity required"))
	}
	if b := s.boundaries[opts.Package]; b.membersSealed {
		key := resourceExpansionKey{id, opts.Package, opts.BaseURI, opts.ArchiveEntry}
		if s.expansions[key] == nil {
			return "", s.fail(contract("resource adoption", "member inventory already sealed"))
		}
	}
	if opts.Root {
		if s.rootAdoption || opts.DocumentID != r.snapshot.header.DocumentID || s.boundaries[opts.Package].identity.Parent != "" {
			return "", s.fail(contract("root adoption", "one primary canonical occurrence required"))
		}
	} else if s.documents[opts.DocumentID] {
		return "", s.fail(contract("resource adoption", "fresh occurrence identity required"))
	}
	if !opts.Root {
		if err := s.account.Charge(BudgetLinkedDocuments, 1); err != nil {
			return "", s.fail(err)
		}
	}
	if err := s.admit(ctx, opts, 1536); err != nil {
		return "", err
	}
	aid := ResourceAdoptionID("adoption-" + strconv.Itoa(len(s.adoptionOrder)+1))
	a := &packageAdoption{id: aid, resource: r, documentID: strings.Clone(opts.DocumentID), root: opts.Root, packageID: opts.Package}
	a.expansion = s.newExpansion(aid, id, opts)
	s.adoptions[aid] = a
	s.adoptionOrder = append(s.adoptionOrder, aid)
	s.output.adoptions = append(s.output.adoptions, bundleAdoption{resource: id, documentID: a.documentID, context: ResourceAdoptionContext{Adoption: aid, Assessment: id, DocumentID: a.documentID, Package: a.packageID, BaseURI: a.expansion.key.baseURI, ArchiveEntry: a.expansion.key.archiveEntry, ExpansionOwner: a.expansion.owner}})
	s.documents[opts.DocumentID] = true
	if opts.Root {
		s.rootAdoption = true
		r.required = true
	}
	return aid, nil
}

// RecordResourceFailure is failure-only ingress after a child could not produce
// an admitted snapshot. It cannot certify success. One pre-reserved terminal
// summary preserves exact known truth when a full report copy is refused.
func (s *PackageSession) RecordResourceFailure(ctx context.Context, pin ResourcePin, report Report, cause error) error {
	if s == nil || s.finished || s.account == nil || s.failure != nil {
		return &SessionError{Operation: "RecordResourceFailure", Reason: "closed package or terminal resource already recorded"}
	}
	if report.Satisfied() && cause == nil {
		return s.fail(contract("failed resource", "successful report needs no failure ingress"))
	}
	complete := Evidence[bool]{State: EvidenceKnown, Value: true}
	if pin.Digest == "" && pin.SnapshotID == "" {
		if !packageIdentityText(pin.CanonicalID) || !validText(pin.Revision, maxIdentityBytes) || (pin.Representation != ResourceSourceXML && pin.Representation != ResourceCurrentModel && pin.Representation != ResourceRawMXL && pin.Representation != ResourceCurrentMXL) {
			return s.fail(contract("failed resource", "bounded unavailable identity required"))
		}
		complete = Evidence[bool]{State: EvidenceUnknown, Reason: "full-resource-pin-unavailable"}
	} else if err := validateResourcePin(pin); err != nil {
		return s.fail(err)
	}
	if !validTarget(report.Target) || !validID(string(report.Profile.ID)) || !validResourceDigest(report.Profile.CatalogDigest) {
		return s.fail(contract("failed resource", "exact original report target/profile/digest required"))
	}
	if cause == nil {
		cause = &IncompleteError{Reason: "intrinsic-resource-failure"}
	}
	category, reason := terminalClassification(cause, false)
	if report.Terminal != nil {
		category = DiagnosticCategory(strings.Clone(reportBoundedValue(string(report.Terminal.Category), reportTerminalBytes)))
		reason = Code(strings.Clone(reportBoundedValue(string(report.Terminal.Reason), reportTerminalBytes)))
	}
	summary := &ResourceFailureSummary{Pin: stateClone(pin), PinComplete: complete, Target: report.Target, Profile: ProfileID(strings.Clone(string(report.Profile.ID))), CatalogDigest: strings.Clone(report.Profile.CatalogDigest), Counts: report.Counts, Conformance: report.Conformance, Terminal: TerminalSummary{Category: category, Reason: reason}, DiagnosticsTruncated: true, MetadataTruncated: true}
	// Attempt ordinary full retention first. Cancellation/quota refusal cannot
	// prevent the already reserved independent count/identity summary below.
	if n, err := resourceMeasure(ctx, s.account, reflect.ValueOf(report)); err == nil {
		if err = s.account.Charge(BudgetBufferedBytes, n); err == nil {
			s.retained += n
			copy := stateClone(report)
			s.output.failureReport = &copy
			summary.DiagnosticsTruncated = false
			summary.MetadataTruncated = false
		}
	}
	s.failure = summary
	return s.fail(cause)
}

// scan charges repeated bounded iteration independently of the original ingress.
func (s *PackageSession) scan(ctx context.Context, n int64) error {
	if s.terminal != nil {
		return s.terminal
	}
	if ctx == nil {
		return s.fail(contract("context", "must not be nil"))
	}
	if err := ctx.Err(); err != nil {
		return s.fail(err)
	}
	return s.fail(s.account.Charge(BudgetRuleSteps, n))
}

func (s *PackageSession) temporary(ctx context.Context, v any) (func(), error) {
	n, err := resourceMeasure(ctx, s.account, reflect.ValueOf(v))
	if err != nil {
		return nil, s.fail(err)
	}
	n, err = rationalCheckedMul(BudgetBufferedBytes, n, 2)
	if err != nil {
		return nil, s.fail(err)
	}
	if err = s.account.Charge(BudgetBufferedBytes, n); err != nil {
		return nil, s.fail(err)
	}
	return func() { _ = s.account.Release(BudgetBufferedBytes, n) }, nil
}
