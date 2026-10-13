package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"strconv"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

type graphPackageKey struct {
	pin       validation.ResourcePin
	parent    validation.PackageID
	retrieval string
}
type graphPackage struct {
	id, parent                     validation.PackageID
	pin                            validation.ResourcePin
	archive                        *mxlAssessmentArchive
	entries                        map[string]validation.PackageFactID
	current                        map[string][]byte
	copied                         map[string][]byte
	primary                        *graphResource
	primaryPath                    string
	primaryPackage                 validation.PackageID
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
func (g *resourceGraphRun) packageLocation(entry, path string) validation.Location {
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
	a, readErr := openMXLAssessmentArchive(ctx, reader, options.MXL, g.options.account)
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
	_, _, _, err = g.importRawPackage(a, pin, "", options.Identity.RetrievalURI, true)
	err = errors.Join(readErr, err)
	if err == nil {
		err = g.walkResources()
	}
	if err == nil {
		err = g.sealPackages()
	}
	return g.finish(err)
}
func (g *resourceGraphRun) packageDigest(data []byte) (string, error) {
	h := sha256.New()
	for at := 0; at < len(data); {
		if err := g.ctx.Err(); err != nil {
			return "", err
		}
		n := min(4096, len(data)-at)
		if err := g.options.account.Charge(validation.BudgetRuleSteps, int64(n)); err != nil {
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
	if err := g.reserve(2048 + 2*int64(len(retrieval)+len(pin.CanonicalID)+len(pin.Revision)+len(pin.Digest)+len(pin.SnapshotID))); err != nil {
		return nil, err
	}
	id := validation.PackageID(g.next("boundary"))
	if parent == "" {
		id = "root"
	}
	if err := g.session.OpenPackage(g.ctx, validation.PackageIdentity{ID: id, Parent: parent, Representation: pin.Representation, Pin: pin, PinUnavailable: pin.Representation == validation.ResourceRawMXL && pin.Digest == ""}); err != nil {
		return nil, err
	}
	p := &graphPackage{id: id, parent: parent, pin: pin, archive: a, entries: make(map[string]validation.PackageFactID), entriesComplete: packageUnknown[bool]("entry-inventory-unavailable"), rootsComplete: packageUnknown[bool]("rootfile-inventory-unavailable")}
	g.packages[id] = p
	g.packageList = append(g.packageList, p)
	if pin.Digest != "" || pin.SnapshotID != "" {
		g.packageContexts[graphPackageKey{pin, parent, retrieval}] = p
	}
	return p, nil
}

// importRawPackage uses only observed bytes and the container's same-parse
// snapshot. The supplied archive owner must outlive every graph expansion.
// An existing ancestor is a legal linked backreference, never new containment.
func (g *resourceGraphRun) importRawPackage(a *mxlAssessmentArchive, pin validation.ResourcePin, parent validation.PackageID, retrieval string, isPrimary bool) (*graphResource, resourceURIBase, validation.PackageID, error) {
	for ancestor := g.packages[parent]; ancestor != nil; ancestor = g.packages[ancestor.parent] {
		if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return nil, resourceURIBase{}, "", err
		}
		if ancestor.pin == pin {
			base := ancestor.primaryBase
			base.RetrievalURI = retrieval
			return ancestor.primary, base, ancestor.primaryPackage, nil
		}
	}
	key := graphPackageKey{pin, parent, retrieval}
	if p := g.packageContexts[key]; p != nil {
		return p.primary, p.primaryBase, p.primaryPackage, nil
	}
	p, err := g.newPackage(pin, parent, retrieval, a)
	if err != nil {
		return nil, resourceURIBase{}, "", err
	}
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
	container, _, err := g.assessPackageSource(p, mxlContainerPath, data, ProfileSourceContainer)
	if err != nil || container == nil {
		return nil, resourceURIBase{}, p.id, err
	}
	aid, err := g.session.AdoptResource(g.ctx, container.id, validation.ResourceAdoptionOptions{DocumentID: g.next("container"), Package: p.id, BaseURI: retrieval, ArchiveEntry: mxlContainerPath})
	if err != nil {
		return nil, resourceURIBase{}, p.id, err
	}
	base := resourceURIBase{RetrievalURI: retrieval, Archive: true, Entry: mxlContainerPath}
	if err = g.enqueue(container, aid, p.id, base, 1); err != nil {
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
		if _, exists := p.entries[entry.central.name]; exists {
			return nil
		}
		e := validation.PackageEntryObservation{ID: g.packageFact(), Package: p.id, Name: entry.central.name, Location: g.packageLocation(entry.central.name, ""), Local: unavailableZIPHeader(), Central: observedZIPHeader(entry.central), HeaderAssociation: packageUnknown[bool]("header-association-unavailable"), UTF8Name: packageKnown(entry.utf8Name), PhysicalOrder: packageUnknown[uint64]("physical-order-unavailable")}
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
		e := validation.PackageEntryObservation{ID: g.packageFact(), Package: p.id, Name: mxlMIMETypePath, Location: g.packageLocation(mxlMIMETypePath, ""), Local: observedZIPHeader(a.prefix.header), Central: unavailableZIPHeader(), HeaderAssociation: packageUnknown[bool]("header-association-unavailable"), UTF8Name: packageKnown(true), PhysicalOrder: packageKnown(uint64(0))}
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
	p.entries[strings.Clone(e.Name)] = e.ID
	return nil
}
func (g *resourceGraphRun) assessPackageSource(p *graphPackage, path string, data []byte, profile validation.ProfileID) (*graphResource, string, error) {
	digest, err := g.packageDigest(data)
	if err != nil {
		return nil, "", err
	}
	pin := g.entryPin(p, path, digest, validation.ResourceSourceXML)
	if cached := g.cache[graphResourceKey{pin, profile}]; cached != nil {
		return cached, g.packageDocuments[cached.id], nil
	}
	engine, err := sourceAssessmentEngineForProfile(profile)
	if err != nil {
		return nil, "", err
	}
	doc := g.next("source")
	result := assessResourceSource(g.ctx, bytes.NewReader(data), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, DocumentID: doc, Entry: path, Profile: profile, Engine: engine}, ByteOwner: sourceBytesAccounted})
	if result.Snapshot == nil {
		if result.Report.Requested {
			err = g.session.RecordResourceFailure(g.ctx, pin, result.Report, result.Err)
		}
		return nil, doc, errors.Join(result.Err, err)
	}
	id, err := g.session.AddResource(g.ctx, pin, result.Snapshot, true)
	if err != nil {
		result.Snapshot.Close()
		return nil, doc, errors.Join(result.Err, err)
	}
	resource, err := g.cacheResource(pin, profile, id)
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
			if c.Parent == root {
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
		r := validation.PackageRootfileObservation{ID: g.packageFact(), Package: p.id, Source: row.Subject, Location: g.packageLocation(mxlContainerPath, "rootfile["+strconv.FormatUint(c.Ordinal.Value, 10)+"]"), Ordinal: c.Ordinal.Value, FullPath: atomicPackageString(c.FullPath), MediaType: atomicPackageString(c.MediaType), Entry: packageUnknown[validation.PackageFactID]("rootfile-entry-unavailable"), Resource: packageUnknown[validation.ResourceAssessmentID]("rootfile-resource-unavailable"), Indirection: packageKnown(false)}
		err = g.loadRootfile(p, &r, retrieval, isPrimary && r.Ordinal == 1)
		if err == nil {
			err = g.session.ObserveRootfile(g.ctx, r)
		}
		release()
		if err != nil {
			return err
		}
	}
	return nil
}
func (g *resourceGraphRun) packageMediaKind(v validation.Evidence[string]) (mxl bool, other bool, err error) {
	if v.State != validation.EvidenceKnown {
		return false, false, nil
	}
	if len(v.Value) > 4096 {
		return false, false, &validation.ConfigurationError{Field: "rootfile media type", Reason: "exceeds bounded package evidence"}
	}
	if err = g.options.account.Charge(validation.BudgetRuleSteps, int64(len(v.Value))+1); err != nil {
		return
	}
	if err = g.ctx.Err(); err != nil {
		return
	}
	scratch := int64(len(v.Value))*16 + 1024
	if err = g.options.account.Charge(validation.BudgetBufferedBytes, scratch); err != nil {
		return
	}
	defer g.options.account.Release(validation.BudgetBufferedBytes, scratch)
	typ, _, parseErr := mime.ParseMediaType(v.Value)
	if parseErr != nil {
		return false, false, nil
	}
	return typ == mxlMIMEType, typ == "application/pdf" || strings.HasPrefix(typ, "image/") || strings.HasPrefix(typ, "audio/") || strings.HasPrefix(typ, "video/"), nil
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
	nested, other, err := g.packageMediaKind(r.MediaType)
	if err != nil {
		return err
	}
	if other {
		return nil
	}
	if !validMXLContentPath(path) {
		r.Resource = packageUnknown[validation.ResourceAssessmentID]("rootfile-path-policy")
		return nil
	}
	if nested {
		if err := g.checkNestedArchiveSize(p, path); err != nil {
			return err
		}
	}
	data, err := g.packageEntryBytes(p, path, mxlAssessmentDocument)
	if err != nil {
		return err
	}
	var resource *graphResource
	var base resourceURIBase
	targetPackage := p.id
	if nested || bytes.HasPrefix(data, []byte{'P', 'K', 3, 4}) {
		r.Indirection = packageKnown(true)
		resource, base, targetPackage, err = g.importNestedPackage(p, path, data, retrieval, isPrimary)
	} else {
		var doc string
		resource, doc, err = g.assessPackageSource(p, path, data, ProfileSourceLinks)
		base = resourceURIBase{RetrievalURI: retrieval, Archive: true, Entry: path}
		if resource != nil && err == nil {
			err = g.adoptPackageResource(resource, doc, p.id, base, isPrimary)
		}
	}
	if resource != nil {
		r.Resource = packageKnown(resource.id)
		if r.Ordinal == 1 {
			p.primary, p.primaryBase, p.primaryPath, p.primaryPackage = resource, base, path, targetPackage
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
	return g.enqueue(resource, aid, pkg, base, 1)
}
func (g *resourceGraphRun) importNestedPackage(p *graphPackage, path string, data []byte, retrieval string, primary bool) (*graphResource, resourceURIBase, validation.PackageID, error) {
	digest, err := g.packageDigest(data)
	if err != nil {
		return nil, resourceURIBase{}, "", err
	}
	pin := g.entryPin(p, path, digest, validation.ResourceRawMXL)
	key := graphPackageKey{pin, p.id, retrieval}
	if old := g.packageContexts[key]; old != nil {
		return old.primary, old.primaryBase, old.primaryPackage, nil
	}
	archive, readErr := borrowMXLAssessmentArchive(g.ctx, data, g.mxlOptions, g.options.account)
	if archive == nil {
		return nil, resourceURIBase{}, "", readErr
	}
	if err = g.ownArchive(archive); err != nil {
		archive.Close()
		return nil, resourceURIBase{}, "", errors.Join(readErr, err)
	}
	resource, base, pkg, err := g.importRawPackage(archive, pin, p.id, retrieval, primary)
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

// loadArchiveDocument binds literal resolved paths to their exact encapsulation.
// No filename or href spelling can alias another package's resource inventory.
func (g *resourceGraphRun) loadArchiveDocument(item graphExpansion, path string) (*graphResource, resourceURIBase, validation.PackageID, validation.Evidence[validation.ResourceAdoptionID], error) {
	unavailable := unavailableTarget(validation.EvidenceUnknown, "archive-resource-unavailable", nil)
	p := g.packages[item.pkg]
	if p == nil {
		return nil, resourceURIBase{}, item.pkg, unavailable, nil
	}
	if p.primary != nil && path == p.primaryPath {
		return p.primary, p.primaryBase, p.primaryPackage, unavailable, nil
	}
	if _, ok := p.entries[path]; !ok {
		if p.entriesComplete.State == validation.EvidenceKnown && p.entriesComplete.Value {
			unavailable = unavailableTarget(validation.EvidenceAbsent, "", nil)
		}
		return nil, resourceURIBase{}, p.id, unavailable, nil
	}
	data, err := g.packageEntryBytes(p, path, mxlAssessmentResource)
	if err != nil {
		return nil, resourceURIBase{}, p.id, unavailable, err
	}
	if bytes.HasPrefix(data, []byte{'P', 'K', 3, 4}) {
		resource, base, pkg, err := g.importNestedPackage(p, path, data, item.base.RetrievalURI, false)
		return resource, base, pkg, unavailable, err
	}
	resource, _, err := g.assessPackageSource(p, path, data, ProfileSourceLinks)
	return resource, resourceURIBase{RetrievalURI: item.base.RetrievalURI, Archive: true, Entry: path}, p.id, unavailable, err
}
func (g *resourceGraphRun) sealPackages() error {
	// IDs are assigned and appended through deterministic entry/rootfile order;
	// walking the context map would randomize diagnostics and quota boundaries.
	for _, item := range g.packageList {
		if err := g.session.SealEnumeration(g.ctx, validation.PackageEnumeration{Package: item.id, Entries: item.entriesComplete, Rootfiles: item.rootsComplete, Edges: packageKnown(true), Roles: packageKnown(true)}); err != nil {
			return err
		}
	}
	return nil
}

func (g *resourceGraphRun) checkNestedArchiveSize(p *graphPackage, path string) error {
	limits, err := g.mxlOptions.limits()
	if err != nil {
		return err
	}
	if p.archive != nil {
		if index, ok := p.archive.index[path]; ok && p.archive.entries[index].central.uncompressed > limits.archiveSize {
			return ErrMXLTooLarge
		}
	} else if int64(len(p.current[path])) > limits.archiveSize {
		return ErrMXLTooLarge
	}
	return nil
}
