package musicxml

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

// AssessMXLPackage assesses the current primary model directly and selected
// retained resource bytes without encoding the package. Empty RootFiles uses
// the encoder's current-state score.musicxml default; it proves no original
// container, header, lexical media-type presence or source order. Callers must
// not mutate the package or its resource bytes until this call returns.
func AssessMXLPackage(ctx context.Context, value *MXLPackage, options MXLAssessmentOptions) (validation.AssessmentBundle, error) {
	if value == nil {
		return validation.AssessmentBundle{}, ErrNilMXLPackage
	}
	if _, err := options.MXL.limits(); err != nil {
		return validation.AssessmentBundle{}, err
	}
	if _, err := validationSchemaForDocument(value.Document); err != nil {
		return validation.AssessmentBundle{}, err
	}
	g, err := startResourceGraph(ctx, options.linked(), ProfileMXLModel)
	if err != nil {
		return validation.AssessmentBundle{}, err
	}
	g.mxlOptions = options.MXL
	p, err := g.newPackage(g.primaryPin(validation.ResourceCurrentMXL, ""), "", options.Identity.RetrievalURI, nil)
	if err != nil {
		return g.finish(err)
	}
	err = g.importCurrentPackage(p, value, options.Identity.RetrievalURI)
	if err == nil {
		err = g.walkResources()
	}
	if err == nil {
		err = g.sealPackages()
	}
	return g.finish(err)
}
func (g *resourceGraphRun) importCurrentPackage(p *graphPackage, value *MXLPackage, retrieval string) error {
	limits, err := g.mxlOptions.limits()
	if err != nil {
		return err
	}
	if err = g.reserve(256); err != nil {
		return err
	}
	p.current = make(map[string][]byte)
	p.copied = make(map[string][]byte)
	count := len(value.RootFiles)
	if count == 0 {
		count = 1
	}
	// Bound all field work and reserve overlapping normalization copies before
	// making them. No source container is synthesized or assessed.
	roots := value.RootFiles
	if len(roots) == 0 {
		roots = []MXLRootFile{{FullPath: mxlRootFilePath, MediaType: musicXMLMIMEType}}
	}
	metadataBytes := int64(0)
	for _, r := range roots {
		n := int64(len(r.FullPath)) + int64(len(r.MediaType))
		if n > limits.metadataSize-metadataBytes {
			return ErrMXLTooLarge
		}
		metadataBytes += n
		if err = g.options.account.Charge(validation.BudgetRuleSteps, n+1); err != nil {
			return err
		}
		if !validXMLText(r.FullPath) || !validXMLText(r.MediaType) {
			return &mxlAssessmentZIPPolicyError{reason: "current rootfile fields are not representable XML text"}
		}
		if err = g.options.account.Check(validation.BudgetScalarBytes, int64(len(r.FullPath))); err != nil {
			return err
		}
		if err = g.options.account.Check(validation.BudgetScalarBytes, int64(len(r.MediaType))); err != nil {
			return err
		}
	}
	if err = g.reserve(int64(count)*256 + metadataBytes*4); err != nil {
		return err
	}
	normalized := make([]MXLRootFile, count)
	for i, r := range roots {
		normalized[i] = MXLRootFile{FullPath: collapseValidationWhitespace(r.FullPath), MediaType: collapseValidationWhitespace(r.MediaType)}
		if !validMXLRootFilePath(normalized[i].FullPath) {
			return &mxlAssessmentZIPPolicyError{reason: "unsafe current rootfile path"}
		}
	}
	primaryPath := normalized[0].FullPath
	if err = g.observeCurrentEntry(p, primaryPath); err != nil {
		return err
	}
	for _, r := range value.Resources {
		if err = g.ctx.Err(); err != nil {
			return err
		}
		if err = g.options.account.Check(validation.BudgetScalarBytes, int64(len(r.Path))); err != nil {
			return err
		}
		if err = g.options.account.Charge(validation.BudgetRuleSteps, int64(len(r.Path))+1); err != nil {
			return err
		}
		if !validMXLContentPath(r.Path) {
			return &mxlAssessmentZIPPolicyError{reason: "unsafe or reserved current resource path"}
		}
		if _, exists := p.entries[r.Path]; exists {
			return &mxlAssessmentZIPPolicyError{reason: "duplicate current resource path"}
		}
		if err = g.observeCurrentEntry(p, r.Path); err != nil {
			return err
		}
		// This is a temporary immutable borrow under the public no-mutation contract.
		// Selection below admits and owns an independent measured byte copy.
		p.current[strings.Clone(r.Path)] = r.Data
	}
	p.entriesComplete = packageKnown(true)
	p.rootsComplete = packageKnown(true)
	engine, err := modelAssessmentEngineForProfile(ProfileModelLinks)
	if err != nil {
		return err
	}
	doc := g.next("model")
	pin := g.entryPin(p, primaryPath, "", validation.ResourceCurrentModel)
	intrinsic := assessResourceModel(g.ctx, value.Document, assessmentRunConfig{Account: g.options.account, DocumentID: doc, Entry: primaryPath, Profile: ProfileModelLinks, Engine: engine})
	if intrinsic.Snapshot == nil {
		if intrinsic.Report.Requested {
			err = g.session.RecordResourceFailure(g.ctx, pin, intrinsic.Report, intrinsic.Err)
		}
		return errors.Join(intrinsic.Err, err)
	}
	id, err := g.session.AddResource(g.ctx, pin, intrinsic.Snapshot, true)
	if err != nil {
		intrinsic.Snapshot.Close()
		return errors.Join(intrinsic.Err, err)
	}
	primary, err := g.cacheResource(pin, ProfileModelLinks, id)
	if err != nil {
		return errors.Join(intrinsic.Err, err)
	}
	g.packageDocuments[id] = doc
	base := resourceURIBase{RetrievalURI: retrieval, Archive: true, Entry: primaryPath}
	p.primary, p.primaryBase, p.primaryPath, p.primaryPackage = primary, base, primaryPath, p.id
	if err = g.adoptPackageResource(primary, doc, p.id, base, true); err != nil {
		return errors.Join(intrinsic.Err, err)
	}
	if intrinsic.Err != nil {
		return intrinsic.Err
	}
	for i, r := range normalized {
		media := packageKnown(r.MediaType)
		// The current encoder omits empty media-type fields; this is a current-state
		// rule, not a reconstruction of their original lexical presence.
		if r.MediaType == "" {
			media = validation.Evidence[string]{State: validation.EvidenceAbsent}
		}
		root := validation.PackageRootfileObservation{ID: g.packageFact(), Package: p.id, Location: g.packageLocation("", "RootFiles["+strconv.Itoa(i)+"]"), Ordinal: uint64(i + 1), FullPath: packageKnown(r.FullPath), MediaType: media, Entry: packageUnknown[validation.PackageFactID]("current-entry-unavailable"), Resource: packageUnknown[validation.ResourceAssessmentID]("current-resource-unavailable"), Indirection: packageKnown(false)}
		if i == 0 {
			root.Entry = packageKnown(p.entries[primaryPath])
			root.Resource = packageKnown(primary.id)
		} else {
			if err = g.loadRootfile(p, &root, retrieval, false); err != nil {
				return err
			}
		}
		if err = g.session.ObserveRootfile(g.ctx, root); err != nil {
			return err
		}
	}
	return nil
}
func (g *resourceGraphRun) observeCurrentEntry(p *graphPackage, path string) error {
	if err := g.options.account.Charge(validation.BudgetNodes, 1); err != nil {
		return err
	}
	e := validation.PackageEntryObservation{ID: g.packageFact(), Package: p.id, Name: path, Location: g.packageLocation(path, "Resources"), Local: unavailableZIPHeader(), Central: unavailableZIPHeader(), HeaderAssociation: packageUnknown[bool]("current-object-has-no-original-headers"), UTF8Name: packageUnknown[bool]("current-object-has-no-original-encoding"), PhysicalOrder: packageUnknown[uint64]("current-object-has-no-original-order")}
	return g.recordPackageEntry(p, e)
}
func (g *resourceGraphRun) currentPackageEntryBytes(p *graphPackage, path string, role mxlAssessmentEntryRole) ([]byte, error) {
	limits, err := g.mxlOptions.limits()
	if err != nil {
		return nil, err
	}
	limit := limits.resourceSize
	switch role {
	case mxlAssessmentMetadata:
		limit = limits.metadataSize
	case mxlAssessmentDocument:
		limit = limits.documentSize
	}
	data, exists := p.current[path]
	if !exists {
		return nil, ErrMXLRootFileNotFound
	}
	if int64(len(data)) > limit {
		return nil, ErrMXLTooLarge
	}
	if copied, ok := p.copied[path]; ok {
		return copied, nil
	}
	if int64(len(data)) > limits.resourcesSize-g.mxlExpanded {
		return nil, ErrMXLTooLarge
	}
	if err = g.options.account.Charge(validation.BudgetExternalBytes, int64(len(data))); err != nil {
		return nil, err
	}
	if err = g.reserve(int64(len(data)) + 256); err != nil {
		return nil, err
	}
	copied := make([]byte, len(data))
	for at := 0; at < len(data); {
		if err = g.ctx.Err(); err != nil {
			return nil, err
		}
		n := min(4096, len(data)-at)
		if err = g.options.account.Charge(validation.BudgetRuleSteps, int64(n)); err != nil {
			return nil, err
		}
		copy(copied[at:at+n], data[at:at+n])
		at += n
	}
	g.mxlExpanded += int64(len(data))
	p.copied[path] = copied
	return copied, nil
}
