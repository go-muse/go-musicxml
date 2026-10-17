package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

type graphPackageKey struct {
	pin       validation.ResourcePin
	parent    validation.PackageID
	retrieval string
}
type graphPackage struct {
	id, parent                     validation.PackageID
	depth                          int64
	rootfiles                      []validation.PackageRootfileObservation
	pin                            validation.ResourcePin
	archive                        *mxlAssessmentArchive
	entries                        map[string]validation.PackageFactID
	current                        map[string][]byte
	copied                         map[string][]byte
	primary                        *graphResource
	primaryPath                    string
	archiveEntry                   string
	primaryPackage                 validation.PackageID
	primaryIndirect                bool
	primaryBase                    resourceURIBase
	entriesComplete, rootsComplete validation.Evidence[bool]
}

func packageKnown[T any](v T) validation.Evidence[T] {
	return validation.Evidence[T]{State: validation.EvidenceKnown, Value: v}
}
func packageUnknown[T any](reason validation.Code) validation.Evidence[T] {
	return validation.Evidence[T]{State: validation.EvidenceUnknown, Reason: reason}
}
func (g *resourceGraphRun) packageFact() validation.PackageFactID {
	g.sequence++
	return validation.PackageFactID(g.sequence)
}
func (g *resourceGraphRun) packageLocation(p *graphPackage, entry, path string) validation.Location {
	if p != nil && p.parent != "" {
		// Keep retrieval URIs unchanged. The exact boundary, parent and containing
		// entry qualify nested diagnostic scope without inventing a nested URL.
		path = "package[" + string(p.id) + "; parent=" + string(p.parent) + "; entry=" + p.archiveEntry + "]/" + path
	}
	return validation.Location{Target: validation.TargetPackage, DocumentID: g.namespace + "-package", Entry: entry, Path: path}
}

// ownResourceBody transfers an already closed-stream immutable byte lease to
// the graph. Archive borrowers are closed first during root cleanup.
func (g *resourceGraphRun) ownResourceBody(body *assessmentResourceBody) error {
	if err := g.reserve(64); err != nil {
		return err
	}
	g.resourceBodies = append(g.resourceBodies, body)
	return nil
}
func (g *resourceGraphRun) ownArchive(a *mxlAssessmentArchive) error {
	if err := g.reserve(64); err != nil {
		return err
	}
	a.sharedExpanded = &g.mxlExpanded
	g.archives = append(g.archives, a)
	return nil
}

// InspectMXL assesses original ZIP headers, its original container source and
// selected archive-local resources using one bounded account. The reader stays
// open. External I/O occurs only through explicitly selected resolver requests.
// options.Graph.Profile must be ProfileMXLSource; any other value, including
// the zero value, is a configuration error.
func InspectMXL(ctx context.Context, reader io.Reader, options MXLAssessmentOptions) (validation.AssessmentBundle, error) {
	if reader == nil {
		return validation.AssessmentBundle{}, ErrNilReader
	}
	if _, err := options.MXL.limits(); err != nil {
		return validation.AssessmentBundle{}, err
	}
	g, err := startResourceGraph(ctx, options.linked(), ProfileMXLSource)
	if err != nil {
		return validation.AssessmentBundle{}, err
	}
	g.mxlOptions = options.MXL
	return g.inspectRawPackage(reader)
}

func (g *resourceGraphRun) inspectRawPackage(reader io.Reader) (validation.AssessmentBundle, error) {
	a, readErr := openMXLAssessmentArchive(g.ctx, reader, g.mxlOptions, g.options.account)
	var err error
	if a == nil {
		return g.finish(readErr)
	}
	if err = g.ownArchive(a); err != nil {
		a.Close()
		return g.finish(errors.Join(readErr, err))
	}
	digest := ""
	if a.inputComplete {
		digest, err = g.packageDigest(a.data)
		if err != nil {
			return g.finish(errors.Join(readErr, err))
		}
	}
	pin := g.primaryPin(validation.ResourceRawMXL, digest)
	_, _, _, err = g.importRawPackage(a, pin, "", g.options.identity.RetrievalURI, true)
	err = errors.Join(readErr, err)
	if err == nil {
		err = g.walkResources()
	}
	if err == nil {
		err = g.observePackageMetadata()
	}
	if err == nil {
		err = g.sealPackages()
	}
	return g.finish(err)
}
func (g *resourceGraphRun) packageDigest(data []byte) (string, error) {
	if err := g.ctx.Err(); err != nil {
		return "", err
	}
	// Initialization/finalization also performs bounded work for an empty body.
	if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
		return "", err
	}
	h := sha256.New()
	for at := 0; at < len(data); {
		if err := g.ctx.Err(); err != nil {
			return "", err
		}
		n := min(4096, len(data)-at)
		if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return "", err
		}
		_, _ = h.Write(data[at : at+n])
		at += n
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (g *resourceGraphRun) entryPin(p *graphPackage, path, digest string, representation validation.ResourceRepresentation) validation.ResourcePin {
	// Length-prefixed fields avoid namespace ambiguity; encoded identity is
	// bounded even when the caller's canonical package identity is long.
	h := sha256.New()
	for _, s := range []string{p.pin.CanonicalID, p.pin.Revision, p.pin.Digest, p.pin.SnapshotID, string(p.pin.Representation), path} {
		io.WriteString(h, strconv.Itoa(len(s)))
		io.WriteString(h, ":")
		io.WriteString(h, s)
	}
	pin := validation.ResourcePin{CanonicalID: "mxl-entry-" + hex.EncodeToString(h.Sum(nil)), Representation: representation, Digest: digest}
	if representation == validation.ResourceCurrentModel {
		pin.SnapshotID = g.next("model-snapshot")
	}
	return pin
}
func (g *resourceGraphRun) newPackage(pin validation.ResourcePin, parent validation.PackageID, retrieval string, a *mxlAssessmentArchive) (*graphPackage, error) {
	return g.newPackageAtEntry(pin, parent, retrieval, "", a)
}
func (g *resourceGraphRun) newPackageAtEntry(pin validation.ResourcePin, parent validation.PackageID, retrieval, entry string, a *mxlAssessmentArchive) (*graphPackage, error) {
	completePin := pin.Digest != "" || pin.SnapshotID != ""
	canonical := graphCanonicalKey{pin.CanonicalID, pin.Representation}
	if old, ok := g.pins[canonical]; completePin && ok && old != pin {
		return nil, &validation.ConfigurationError{Field: "package pin", Reason: "canonical identity changed revision or content"}
	}
	if err := g.reserve(2048 + 2*int64(len(retrieval)+len(entry)+len(pin.CanonicalID)+len(pin.Revision)+len(pin.Digest)+len(pin.SnapshotID))); err != nil {
		return nil, err
	}
	id := validation.PackageID(g.next("boundary"))
	if parent == "" {
		id = "root"
	}
	if err := g.session.OpenPackage(g.ctx, validation.PackageIdentity{ID: id, Parent: parent, Representation: pin.Representation, Pin: pin, BaseURI: retrieval, ArchiveEntry: entry, PinUnavailable: pin.Representation == validation.ResourceRawMXL && pin.Digest == ""}); err != nil {
		return nil, err
	}
	p := &graphPackage{id: id, parent: parent, depth: 1, pin: pin, archive: a, archiveEntry: strings.Clone(entry), entries: make(map[string]validation.PackageFactID), entriesComplete: packageUnknown[bool]("entry-inventory-unavailable"), rootsComplete: packageUnknown[bool]("rootfile-inventory-unavailable")}
	g.packages[id] = p
	if completePin {
		g.pins[canonical] = pin
	}
	g.packageList = append(g.packageList, p)
	if pin.Digest != "" || pin.SnapshotID != "" {
		g.packageContexts[graphPackageKey{pin, parent, retrieval}] = p
	}
	if err := g.observeBoundaryFileMetadata(p, retrieval); err != nil {
		return nil, err
	}
	return p, nil
}

// importRawPackage uses only observed bytes and the container's same-parse
// snapshot. The supplied archive owner must outlive every graph expansion.
// An existing ancestor is a legal linked backreference, never new containment.
func (g *resourceGraphRun) importRawPackage(a *mxlAssessmentArchive, pin validation.ResourcePin, parent validation.PackageID, retrieval string, isPrimary bool) (*graphResource, resourceURIBase, validation.PackageID, error) {
	return g.importRawPackageAtDepth(a, pin, parent, retrieval, isPrimary, 1)
}
func (g *resourceGraphRun) importRawPackageAtDepth(a *mxlAssessmentArchive, pin validation.ResourcePin, parent validation.PackageID, retrieval string, isPrimary bool, depth int64) (*graphResource, resourceURIBase, validation.PackageID, error) {
	return g.importRawPackageAtEntry(a, pin, parent, retrieval, "", isPrimary, depth)
}
func (g *resourceGraphRun) importRawPackageAtEntry(a *mxlAssessmentArchive, pin validation.ResourcePin, parent validation.PackageID, retrieval, entry string, isPrimary bool, depth int64) (*graphResource, resourceURIBase, validation.PackageID, error) {
	if resource, base, pkg, known, err := g.existingRawPackage(pin, parent, retrieval); known || err != nil {
		return resource, base, pkg, err
	}
	if depth > g.options.account.Limits().MaxDepth {
		return nil, resourceURIBase{}, "", &validation.ResourceError{Kind: validation.BudgetDepth, Limit: g.options.account.Limits().MaxDepth, Used: depth - 1, Requested: 1}
	}
	p, err := g.newPackageAtEntry(pin, parent, retrieval, entry, a)
	if err != nil {
		return nil, resourceURIBase{}, "", err
	}
	p.depth = depth
	if err = g.observeRawEntries(p); err != nil {
		return nil, resourceURIBase{}, p.id, err
	}
	if !a.ready {
		return nil, resourceURIBase{}, p.id, nil
	}
	if at, ok := a.index[mxlMIMETypePath]; ok {
		_, readErr := a.readEntry(at, mxlAssessmentMetadata)
		content := &a.contents[at]
		m := validation.PackageMimetypeObservation{Entry: p.entries[mxlMIMETypePath], Prefix: content.prefix[:content.prefixLength], ByteLength: packageUnknown[uint64]("mimetype-read-incomplete"), ContentComplete: packageUnknown[bool]("mimetype-read-incomplete")}
		if readErr == nil {
			m.ByteLength = packageKnown(uint64(len(content.data)))
			m.ContentComplete = packageKnown(true)
		}
		observationErr := g.session.ObserveMimetype(g.ctx, m)
		if err = errors.Join(readErr, observationErr); err != nil {
			return nil, resourceURIBase{}, p.id, err
		}
	}
	at, exists := a.index[mxlContainerPath]
	if !exists {
		return nil, resourceURIBase{}, p.id, nil
	}
	data, err := a.readEntry(at, mxlAssessmentMetadata)
	if err != nil {
		return nil, resourceURIBase{}, p.id, err
	}
	container, _, err := g.assessPackageSource(p, mxlContainerPath, data, g.profiles.container)
	if err != nil || container == nil {
		return nil, resourceURIBase{}, p.id, err
	}
	aid, err := g.session.AdoptResource(g.ctx, container.id, validation.ResourceAdoptionOptions{DocumentID: g.next("container"), Package: p.id, BaseURI: retrieval, ArchiveEntry: mxlContainerPath})
	if err != nil {
		return nil, resourceURIBase{}, p.id, err
	}
	base := resourceURIBase{RetrievalURI: retrieval, Archive: true, Entry: mxlContainerPath}
	if err = g.enqueue(container, aid, p.id, base, p.depth); err != nil {
		return nil, resourceURIBase{}, p.id, err
	}
	err = g.consumeContainer(p, aid, retrieval, isPrimary)
	return p.primary, p.primaryBase, p.primaryPackage, err
}
func unavailableZIPHeader() validation.ZIPHeaderObservation {
	return validation.ZIPHeaderObservation{Present: packageUnknown[bool]("original-header-unavailable"), Method: packageUnknown[uint16]("original-header-unavailable"), Flags: packageUnknown[uint16]("original-header-unavailable"), ExtraLength: packageUnknown[uint64]("original-header-unavailable")}
}
func observedZIPHeader(h mxlAssessmentZIPHeader) validation.ZIPHeaderObservation {
	return validation.ZIPHeaderObservation{Present: packageKnown(true), Method: packageKnown(h.method), Flags: packageKnown(h.flags), ExtraLength: packageKnown(uint64(len(h.extra)))}
}
func (g *resourceGraphRun) observeRawEntries(p *graphPackage) error {
	a := p.archive
	// Preserve independently observed local mimetype defects before another
	// partially observed member can terminate ordinary inventory ingress.
	observe := func(entry mxlAssessmentZIPEntry) error {
		if err := g.ctx.Err(); err != nil {
			return err
		}
		if entry.central.name == "" {
			return nil
		}
		if err := packageEvidenceTextLimit("archive entry name", entry.central.name); err != nil {
			return err
		}
		if strings.ContainsRune(entry.central.name, 0) {
			return &mxlAssessmentZIPPolicyError{reason: "NUL in archive entry name", cause: ErrMXLInvalidPath}
		}
		if _, exists := p.entries[entry.central.name]; exists {
			return nil
		}
		e := validation.PackageEntryObservation{ID: g.packageFact(), Package: p.id, Name: entry.central.name, Location: g.packageLocation(p, entry.central.name, ""), Local: unavailableZIPHeader(), Central: observedZIPHeader(entry.central), HeaderAssociation: packageUnknown[bool]("header-association-unavailable"), UTF8Name: packageKnown(entry.utf8Name), PhysicalOrder: packageUnknown[uint64]("physical-order-unavailable")}
		if !utf8.ValidString(entry.central.name) {
			if err := g.options.account.Check(validation.BudgetScalarBytes, int64(len(entry.central.name))); err != nil {
				return err
			}
			if err := g.options.account.Charge(validation.BudgetRuleSteps, int64(len(entry.central.name))+128); err != nil {
				return err
			}
			// Preserve bytes without treating an ASCII diagnostic label as identity.
			// The original transport policy still rejects this member for reading.
			n := int64(len(entry.central.name))*2 + 512
			if err := g.options.account.Charge(validation.BudgetBufferedBytes, n); err != nil {
				return err
			}
			defer g.options.account.Release(validation.BudgetBufferedBytes, n)
			e.Name = ""
			e.RawName = []byte(entry.central.name)
			label := hex.EncodeToString(e.RawName[:min(64, len(e.RawName))])
			if len(e.RawName) > 64 {
				label += "..."
			}
			e.Location = g.packageLocation(p, "", "raw-entry-name-hex["+label+"]")
		}
		if entry.localObserved && entry.local.name == entry.central.name {
			e.Local = observedZIPHeader(entry.local)
		}
		if entry.trustedIdentity {
			e.HeaderAssociation = packageKnown(entry.associated)
			e.PhysicalOrder = packageKnown(uint64(entry.physicalOrder))
		}
		return g.recordPackageEntry(p, e)
	}
	for _, entry := range a.entries {
		if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return err
		}
		if err := g.ctx.Err(); err != nil {
			return err
		}
		if entry.central.name == mxlMIMETypePath {
			if err := observe(entry); err != nil {
				return err
			}
			break
		}
	}
	if _, exists := p.entries[mxlMIMETypePath]; !exists && a.prefix.framed {
		e := validation.PackageEntryObservation{ID: g.packageFact(), Package: p.id, Name: mxlMIMETypePath, Location: g.packageLocation(p, mxlMIMETypePath, ""), Local: observedZIPHeader(a.prefix.header), Central: unavailableZIPHeader(), HeaderAssociation: packageUnknown[bool]("header-association-unavailable"), UTF8Name: packageKnown(true), PhysicalOrder: packageKnown(uint64(0))}
		if err := g.recordPackageEntry(p, e); err != nil {
			return err
		}
	}
	for _, entry := range a.entries {
		if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return err
		}
		if err := observe(entry); err != nil {
			return err
		}
	}
	if a.inventoryComplete {
		p.entriesComplete = packageKnown(true)
	}
	return nil
}
func (g *resourceGraphRun) recordPackageEntry(p *graphPackage, e validation.PackageEntryObservation) error {
	if err := g.reserve(256 + 2*int64(len(e.Name))); err != nil {
		return err
	}
	if err := g.session.ObserveEntry(g.ctx, e); err != nil {
		return err
	}
	if len(e.RawName) == 0 {
		p.entries[strings.Clone(e.Name)] = e.ID
	}
	return nil
}
func (g *resourceGraphRun) assessPackageSource(p *graphPackage, path string, data []byte, profile validation.ProfileID) (*graphResource, string, error) {
	digest, err := g.packageDigest(data)
	if err != nil {
		return nil, "", err
	}
	pin := g.entryPin(p, path, digest, validation.ResourceSourceXML)
	return g.assessVerifiedPackageSource(p, path, data, profile, pin)
}

// The pin comes only from hashing these retained immutable entry bytes. Reuse
// that verification for metadata and intrinsic parsing; never rehash the borrow.
func (g *resourceGraphRun) assessVerifiedPackageSource(p *graphPackage, path string, data []byte, profile validation.ProfileID, pin validation.ResourcePin) (*graphResource, string, error) {
	if cached := g.cache[g.resourceKey(pin, profile)]; cached != nil {
		return cached, g.packageDocuments[cached.id], g.checkIntrinsicXMLDepth(cached, p.id)
	}
	engine, err := sourceAssessmentEngineForProfile(profile)
	if err != nil {
		return nil, "", err
	}
	doc := g.next("source")
	result := assessResourceSource(g.ctx, bytes.NewReader(data), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, Dictionaries: g.dictionaries, DocumentID: doc, Entry: path, XMLDepth: g.intrinsicXMLDepth(p.id), Profile: profile, Engine: engine}, ByteOwner: sourceBytesAccounted})
	if result.Snapshot == nil || shouldRetainMalformedIntrinsicWithoutAssembly(result) {
		return nil, doc, g.retainUntransferredIntrinsic(result, pin, nil)
	}
	id, err := g.session.AddResource(g.ctx, pin, result.Snapshot, true)
	if err != nil {
		return nil, doc, g.retainUntransferredIntrinsic(result, pin, err)
	}
	resource, err := g.cacheResource(pin, profile, id, result.XMLDepth, result.Err == nil)
	if err == nil {
		g.packageDocuments[id] = doc
	}
	return resource, doc, errors.Join(result.Err, err)
}
func atomicPackageString(v validation.Evidence[validation.Atomic]) validation.Evidence[string] {
	if v.State != validation.EvidenceKnown {
		return validation.Evidence[string]{State: v.State, Reason: v.Reason, Cause: v.Cause}
	}
	if v.Value.Kind != validation.AtomicString {
		return packageUnknown[string]("container-token-value-unavailable")
	}
	return packageKnown(v.Value.Text)
}
func (g *resourceGraphRun) consumeContainer(p *graphPackage, aid validation.ResourceAdoptionID, retrieval string, isPrimary bool) error {
	adoption, err := g.session.Adoption(g.ctx, aid)
	if err != nil {
		return err
	}
	// A complete root inventory needs both enclosing source content proofs.
	root := adoption.Header().Root
	rootComplete := false
	files := validation.NodeRef{}
	filesComplete := false
	for i := 0; i < adoption.ContextCount(); i++ {
		row, ok, err := adoption.ContextAt(g.ctx, i)
		if err != nil {
			return err
		}
		if !ok || row.Container == nil {
			continue
		}
		c := row.Container
		switch c.Role {
		case validation.ContainerRoot:
			rootComplete = row.Subject == root && c.Content.State == validation.EvidenceKnown && c.Content.Value
		case validation.ContainerRootfiles:
			if c.Parent == root && files == (validation.NodeRef{}) {
				files = row.Subject
				filesComplete = c.Content.State == validation.EvidenceKnown && c.Content.Value
			}
		}
	}
	complete := rootComplete && filesComplete
	if complete {
		p.rootsComplete = packageKnown(true)
	}
	for i := 0; i < adoption.ContextCount(); i++ {
		row, ok, err := adoption.ContextAt(g.ctx, i)
		if err != nil {
			return err
		}
		if !ok || row.Container == nil || row.Container.Role != validation.ContainerRootfile {
			continue
		}
		release, err := resourceWorkingLease(g.ctx, g.options.account, row)
		if err != nil {
			return err
		}
		c := row.Container
		if !complete || c.Parent != files || c.Ordinal.State != validation.EvidenceKnown {
			release()
			continue
		}
		r := validation.PackageRootfileObservation{ID: g.packageFact(), Package: p.id, Source: row.Subject, Location: g.packageLocation(p, mxlContainerPath, "rootfile["+strconv.FormatUint(c.Ordinal.Value, 10)+"]"), Ordinal: c.Ordinal.Value, FullPath: atomicPackageString(c.FullPath), MediaType: atomicPackageString(c.MediaType), Entry: packageUnknown[validation.PackageFactID]("rootfile-entry-unavailable"), Resource: packageUnknown[validation.ResourceAssessmentID]("rootfile-resource-unavailable"), Indirection: packageKnown(false)}
		if err = packageEvidenceTextLimit("rootfile full-path", r.FullPath.Value); err == nil {
			err = packageEvidenceTextLimit("rootfile media-type", r.MediaType.Value)
		}
		if err != nil {
			release()
			return err
		}
		err = g.loadRootfile(p, &r, retrieval, isPrimary && r.Ordinal == 1)
		if terminal := g.session.TerminalCause(); terminal != nil {
			err = joinResourceErrors(err, terminal)
		} else {
			err = errors.Join(err, g.recordRootfile(p, r))
		}
		release()
		if err != nil {
			return err
		}
	}
	return nil
}
func (g *resourceGraphRun) packageMediaKind(v validation.Evidence[string]) (validation.Evidence[validation.ResourceRepresentation], error) {
	if v.State == validation.EvidenceAbsent {
		// Absent rootfile media-type has the reviewed MusicXML default. An
		// explicitly present empty value must go through the shared classifier.
		return packageKnown(validation.ResourceSourceXML), nil
	}
	if v.State != validation.EvidenceKnown {
		return validation.Evidence[validation.ResourceRepresentation]{State: v.State, Reason: v.Reason, Cause: v.Cause}, nil
	}
	if err := packageEvidenceTextLimit("rootfile media-type", v.Value); err != nil {
		return validation.Evidence[validation.ResourceRepresentation]{}, err
	}
	return validation.ClassifyResourceMediaType(g.ctx, g.options.account, v.Value)
}
func (g *resourceGraphRun) loadRootfile(p *graphPackage, r *validation.PackageRootfileObservation, retrieval string, isPrimary bool) error {
	if r.FullPath.State != validation.EvidenceKnown {
		r.Entry = validation.Evidence[validation.PackageFactID]{State: r.FullPath.State, Reason: r.FullPath.Reason, Cause: r.FullPath.Cause}
		r.Resource = validation.Evidence[validation.ResourceAssessmentID]{State: r.FullPath.State, Reason: r.FullPath.Reason, Cause: r.FullPath.Cause}
		return nil
	}
	path := r.FullPath.Value
	entry, exists := p.entries[path]
	if !exists {
		if p.entriesComplete.State == validation.EvidenceKnown && p.entriesComplete.Value {
			r.Entry = validation.Evidence[validation.PackageFactID]{State: validation.EvidenceAbsent}
			r.Resource = validation.Evidence[validation.ResourceAssessmentID]{State: validation.EvidenceAbsent}
		}
		return nil
	}
	r.Entry = packageKnown(entry)
	media, err := g.packageMediaKind(r.MediaType)
	if err != nil {
		return err
	}
	if media.State == validation.EvidenceKnown && media.Value == validation.ResourceBinary {
		return nil
	}
	if r.Ordinal != 1 && media.State != validation.EvidenceKnown {
		r.Resource = packageUnknown[validation.ResourceAssessmentID]("rootfile-media-type-unresolved")
		return nil
	}
	if !validMXLContentPath(path) {
		r.Resource = packageUnknown[validation.ResourceAssessmentID]("rootfile-path-policy")
		return nil
	}
	role := mxlAssessmentPrimaryDocument
	if r.Ordinal != 1 {
		role = mxlAssessmentLinkedDocument
	}
	data, err := g.packageEntryBytes(p, path, role)
	if err != nil {
		return err
	}
	pin, err := g.observeLocalFileMetadata(p, path, data, retrieval, r.Location, r.MediaType)
	if err != nil {
		return err
	}
	var resource *graphResource
	var base resourceURIBase
	targetPackage := p.id
	if mxlAssessmentZIPPrefixBytes(data) {
		r.Indirection = packageKnown(true)
		resource, base, targetPackage, err = g.importNestedPackage(p, path, data, pin, retrieval, isPrimary, p.depth+1)
	} else {
		var doc string
		resource, doc, err = g.assessVerifiedPackageSource(p, path, data, g.profiles.source, pin)
		base = resourceURIBase{RetrievalURI: retrieval, Archive: true, Entry: path}
		if resource != nil && err == nil {
			err = g.adoptPackageResource(resource, doc, p.id, base, isPrimary)
		}
	}
	if resource != nil {
		r.Resource = packageKnown(resource.id)
		if r.Ordinal == 1 {
			p.primary, p.primaryBase, p.primaryPath, p.primaryPackage = resource, base, path, targetPackage
			p.primaryIndirect = r.Indirection.State == validation.EvidenceKnown && r.Indirection.Value
		}
	}
	return err
}
func (g *resourceGraphRun) adoptPackageResource(resource *graphResource, doc string, pkg validation.PackageID, base resourceURIBase, root bool) error {
	root = root && g.packages[pkg].parent == ""
	if !root {
		doc = g.next("rootfile")
	}
	aid, err := g.session.AdoptResource(g.ctx, resource.id, validation.ResourceAdoptionOptions{DocumentID: doc, Root: root, Package: pkg, BaseURI: base.RetrievalURI, ArchiveEntry: base.Entry})
	if err != nil {
		return err
	}
	return g.enqueue(resource, aid, pkg, base, g.packages[pkg].depth)
}
func (g *resourceGraphRun) importNestedPackage(p *graphPackage, path string, data []byte, pin validation.ResourcePin, retrieval string, primary bool, depth int64) (*graphResource, resourceURIBase, validation.PackageID, error) {
	key := graphPackageKey{pin, p.id, retrieval}
	if old := g.packageContexts[key]; old != nil {
		return old.primary, old.primaryBase, old.primaryPackage, nil
	}
	archive, readErr := borrowMXLAssessmentArchive(g.ctx, data, g.mxlOptions, g.options.account)
	if archive == nil {
		return nil, resourceURIBase{}, "", readErr
	}
	if err := g.ownArchive(archive); err != nil {
		archive.Close()
		return nil, resourceURIBase{}, "", errors.Join(readErr, err)
	}
	resource, base, pkg, err := g.importRawPackageAtEntry(archive, pin, p.id, retrieval, path, primary, depth)
	return resource, base, pkg, errors.Join(readErr, err)
}
func (g *resourceGraphRun) packageEntryBytes(p *graphPackage, path string, role mxlAssessmentEntryRole) ([]byte, error) {
	if p.archive != nil {
		index, ok := p.archive.index[path]
		if !ok {
			return nil, ErrMXLRootFileNotFound
		}
		return p.archive.readEntry(index, role)
	}
	return g.currentPackageEntryBytes(p, path, role)
}

// archiveDocumentInventory returns whether a path could select a document.
// Missing targets need no read or expansion: only a complete local inventory
// proves absence. Preserve primary identity precedence and exact package/path
// spelling; an incomplete inventory carries its own unavailable evidence.
func (g *resourceGraphRun) archiveDocumentInventory(item graphExpansion, path string) (validation.Evidence[validation.ResourceAdoptionID], bool, error) {
	unavailable := unavailableTarget(validation.EvidenceUnknown, "archive-resource-unavailable", nil)
	if err := g.ctx.Err(); err != nil {
		return unavailable, false, err
	}
	if err := g.session.TerminalCause(); err != nil {
		return unavailable, false, err
	}
	if err := g.options.account.Charge(validation.BudgetRuleSteps, int64(len(path))+1); err != nil {
		return unavailable, false, err
	}
	p := g.packages[item.pkg]
	if p == nil {
		return unavailable, false, nil
	}
	if p.primary != nil && path == p.primaryPath {
		return unavailable, true, nil
	}
	if _, ok := p.entries[path]; ok {
		return unavailable, true, nil
	}
	if p.entriesComplete.State == validation.EvidenceKnown && p.entriesComplete.Value {
		return unavailableTarget(validation.EvidenceAbsent, "", nil), false, nil
	}
	switch p.entriesComplete.State {
	case validation.EvidenceUnknown, validation.EvidenceUnsupported, validation.EvidenceInvalid:
		unavailable = unavailableTarget(p.entriesComplete.State, p.entriesComplete.Reason, p.entriesComplete.Cause)
	}
	return unavailable, false, nil
}

// loadArchiveDocument binds literal resolved paths to their exact encapsulation.
// No filename or href spelling can alias another package's resource inventory.
func (g *resourceGraphRun) loadArchiveDocument(item graphExpansion, path string) (*graphResource, resourceURIBase, validation.PackageID, validation.Evidence[validation.ResourceAdoptionID], bool, error) {
	indirect := false
	unavailable, selectable, err := g.archiveDocumentInventory(item, path)
	if err != nil || !selectable {
		return nil, resourceURIBase{}, item.pkg, unavailable, indirect, err
	}
	p := g.packages[item.pkg]
	if p.primary != nil && path == p.primaryPath {
		indirect = p.primaryIndirect
		base := p.primaryBase
		base.RetrievalURI = item.base.RetrievalURI
		return p.primary, base, p.primaryPackage, unavailable, indirect, nil
	}
	data, err := g.packageEntryBytes(p, path, mxlAssessmentLinkedDocument)
	if err != nil {
		return nil, resourceURIBase{}, p.id, unavailable, indirect, err
	}
	pin, err := g.observeLocalFileMetadata(p, path, data, item.base.RetrievalURI, g.packageLocation(p, path, "selected-archive-document"), validation.Evidence[string]{State: validation.EvidenceAbsent})
	if err != nil {
		return nil, resourceURIBase{}, p.id, unavailable, indirect, err
	}
	if mxlAssessmentZIPPrefixBytes(data) {
		indirect = true
		resource, base, pkg, err := g.importNestedPackage(p, path, data, pin, item.base.RetrievalURI, false, item.depth+1)
		return resource, base, pkg, unavailable, indirect, err
	}
	resource, _, err := g.assessVerifiedPackageSource(p, path, data, g.profiles.source, pin)
	return resource, resourceURIBase{RetrievalURI: item.base.RetrievalURI, Archive: true, Entry: path}, p.id, unavailable, indirect, err
}
func (g *resourceGraphRun) sealPackages() error {
	// IDs are assigned and appended through deterministic entry/rootfile order;
	// walking the context map would randomize diagnostics and quota boundaries.
	for _, item := range g.packageList {
		// Roles seals completion of the selected ingestion pass. It does not
		// assert provider authority or broaden an incomplete role universe.
		if err := g.session.SealEnumeration(g.ctx, validation.PackageEnumeration{Package: item.id, Entries: item.entriesComplete, Rootfiles: item.rootsComplete, Edges: packageKnown(true), Roles: packageKnown(true)}); err != nil {
			return err
		}
	}
	return nil
}

// recordRootfile transfers a measured owned observation into the package's
// metadata inventory; cleanup releases it after all metadata consumers finish.
func (g *resourceGraphRun) recordRootfile(p *graphPackage, r validation.PackageRootfileObservation) error {
	if err := g.session.ObserveRootfile(g.ctx, r); err != nil {
		return err
	}
	release, err := resourceWorkingLease(g.ctx, g.options.account, r)
	if err != nil {
		return err
	}
	if err = g.reserve(2048); err != nil {
		release()
		return err
	}
	p.rootfiles = append(p.rootfiles, r)
	g.packageLeases = append(g.packageLeases, release)
	return nil
}

// existingRawPackage proves reuse with the same immutable pin and encapsulation
// context as importRawPackageAtEntry. It changes no target interpretation.
func (g *resourceGraphRun) existingRawPackage(pin validation.ResourcePin, parent validation.PackageID, retrieval string) (*graphResource, resourceURIBase, validation.PackageID, bool, error) {
	for ancestor := g.packages[parent]; ancestor != nil; ancestor = g.packages[ancestor.parent] {
		if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return nil, resourceURIBase{}, "", false, err
		}
		if ancestor.pin == pin {
			base := ancestor.primaryBase
			base.RetrievalURI = retrieval
			return ancestor.primary, base, ancestor.primaryPackage, true, nil
		}
	}
	key := graphPackageKey{pin, parent, retrieval}
	if p := g.packageContexts[key]; p != nil {
		return p.primary, p.primaryBase, p.primaryPackage, true, nil
	}
	return nil, resourceURIBase{}, "", false, nil
}
