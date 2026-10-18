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
// An empty options.Graph.Profile selects ProfileMXLModelSemantics with the
// bundled required dictionaries, or options.Graph.Dictionaries. Otherwise it
// must be ProfileMXLModel, or a dictionary MXL model profile when
// options.Graph.Dictionaries is set. A decoded package cannot carry
// source-only evidence such as discarded schema hints, so its package report
// can stay incomplete; DecodeMXLAssessed proves package rules from the
// original archive instead.
func AssessMXLPackage(ctx context.Context, value *MXLPackage, options MXLAssessmentOptions) (validation.AssessmentBundle, error) {
	if graph, dictionaries, ok := options.Graph.scenario(ProfileMXLModelSemantics); ok {
		options.Graph = graph
		return assessMXLPackageWithDictionaries(ctx, value, MXLDictionaryAssessmentOptions{Resources: options, Dictionaries: dictionaries})
	}
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
	return g.assessCurrentPackage(value)
}

func (g *resourceGraphRun) assessCurrentPackage(value *MXLPackage) (validation.AssessmentBundle, error) {
	p, err := g.newPackage(g.primaryPin(validation.ResourceCurrentMXL, ""), "", g.options.identity.RetrievalURI, nil)
	if err != nil {
		return g.finish(err)
	}
	err = g.importCurrentPackage(p, value, g.options.identity.RetrievalURI)
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
		if err = packageEvidenceTextLimit("current rootfile full-path", r.FullPath); err != nil {
			return err
		}
		if err = packageEvidenceTextLimit("current rootfile media-type", r.MediaType); err != nil {
			return err
		}
		n := int64(len(r.FullPath)) + int64(len(r.MediaType))
		if n > limits.metadataSize-metadataBytes {
			return ErrMXLTooLarge
		}
		metadataBytes += n
		if err = g.options.account.Charge(validation.BudgetRuleSteps, n+1); err != nil {
			return err
		}
		if !validXMLText(r.FullPath) {
			return &mxlAssessmentZIPPolicyError{reason: "current rootfile path is not representable XML text", cause: ErrMXLInvalidPath}
		}
		if !validXMLText(r.MediaType) {
			return &mxlAssessmentZIPPolicyError{reason: "current rootfile media type is not representable XML text", cause: ErrMXLInvalidContainer}
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
	seenRoots := make(map[string]bool, count)
	for i, r := range roots {
		normalized[i] = MXLRootFile{FullPath: collapseValidationWhitespace(r.FullPath), MediaType: collapseValidationWhitespace(r.MediaType)}
		if !validMXLRootFilePath(normalized[i].FullPath) {
			return &mxlAssessmentZIPPolicyError{reason: "unsafe current rootfile path", cause: ErrMXLInvalidPath}
		}
		if seenRoots[normalized[i].FullPath] {
			return &mxlAssessmentZIPPolicyError{reason: "duplicate current rootfile path", cause: ErrMXLDuplicateEntry}
		}
		seenRoots[normalized[i].FullPath] = true
	}
	primaryPath := normalized[0].FullPath
	if err = g.observeCurrentEntry(p, primaryPath, "RootFiles[0]"); err != nil {
		return err
	}
	for i, r := range value.Resources {
		if err = g.ctx.Err(); err != nil {
			return err
		}
		if err = packageEvidenceTextLimit("current resource path", r.Path); err != nil {
			return err
		}
		if err = g.options.account.Check(validation.BudgetScalarBytes, int64(len(r.Path))); err != nil {
			return err
		}
		if err = g.options.account.Charge(validation.BudgetRuleSteps, int64(len(r.Path))+1); err != nil {
			return err
		}
		if !validMXLContentPath(r.Path) || strings.ContainsRune(r.Path, 0) {
			return &mxlAssessmentZIPPolicyError{reason: "unsafe or reserved current resource path", cause: ErrMXLInvalidPath}
		}
		if _, exists := p.entries[r.Path]; exists {
			return &mxlAssessmentZIPPolicyError{reason: "duplicate current resource path", cause: ErrMXLDuplicateEntry}
		}
		if err = g.observeCurrentEntry(p, r.Path, "Resources["+strconv.Itoa(i)+"]"); err != nil {
			return err
		}
		// This is a temporary immutable borrow under the public no-mutation contract.
		// Selection below admits and owns an independent measured byte copy.
		p.current[strings.Clone(r.Path)] = r.Data
	}
	p.entriesComplete = packageKnown(true)
	p.rootsComplete = packageKnown(true)
	engine, err := modelAssessmentEngineForProfile(g.profiles.model)
	if err != nil {
		return err
	}
	doc := g.next("model")
	pin := g.entryPin(p, primaryPath, "", validation.ResourceCurrentModel)
	intrinsic := assessResourceModel(g.ctx, value.Document, assessmentRunConfig{Account: g.options.account, Dictionaries: g.dictionaries, DocumentID: doc, Entry: primaryPath, XMLDepth: g.intrinsicXMLDepth(p.id), Profile: g.profiles.model, Engine: engine})
	if intrinsic.Snapshot == nil {
		return g.retainUntransferredIntrinsic(intrinsic, pin, nil)
	}
	id, err := g.session.AddResource(g.ctx, pin, intrinsic.Snapshot, true)
	if err != nil {
		return g.retainUntransferredIntrinsic(intrinsic, pin, err)
	}
	primary, err := g.cacheResource(pin, g.profiles.model, id, intrinsic.XMLDepth, intrinsic.Err == nil)
	if err != nil {
		return errors.Join(intrinsic.Err, err)
	}
	g.packageDocuments[id] = doc
	base := resourceURIBase{RetrievalURI: retrieval, Archive: true, Entry: primaryPath}
	p.primary, p.primaryBase, p.primaryPath, p.primaryPackage = primary, base, primaryPath, p.id
	if err = g.adoptPackageResource(primary, doc, p.id, base, true); err != nil {
		return errors.Join(intrinsic.Err, err)
	}
	for i, r := range normalized {
		media := packageKnown(r.MediaType)
		// The current encoder omits empty media-type fields; this is a current-state
		// rule, not a reconstruction of their original lexical presence.
		if r.MediaType == "" {
			media = validation.Evidence[string]{State: validation.EvidenceAbsent}
		}
		root := validation.PackageRootfileObservation{ID: g.packageFact(), Package: p.id, Location: g.packageLocation(p, "", "RootFiles["+strconv.Itoa(i)+"]"), Ordinal: uint64(i + 1), FullPath: packageKnown(r.FullPath), MediaType: media, Entry: packageUnknown[validation.PackageFactID]("current-entry-unavailable"), Resource: packageUnknown[validation.ResourceAssessmentID]("current-resource-unavailable"), Indirection: packageKnown(false)}
		if i == 0 {
			root.Entry = packageKnown(p.entries[primaryPath])
			root.Resource = packageKnown(primary.id)
			if err = g.observeCurrentPrimaryFileMetadata(p, root, retrieval); err != nil {
				return errors.Join(intrinsic.Err, err)
			}
		} else {
			err = g.loadRootfile(p, &root, retrieval, false)
		}
		err = errors.Join(err, g.recordRootfile(p, root))
		if err != nil || intrinsic.Err != nil {
			// The admitted primary identity and its independent metadata remain
			// available even when intrinsic model assessment was interrupted.
			return errors.Join(intrinsic.Err, err)
		}
	}
	return nil
}
func (g *resourceGraphRun) observeCurrentEntry(p *graphPackage, path, origin string) error {
	if err := g.options.account.Charge(validation.BudgetNodes, 1); err != nil {
		return err
	}
	e := validation.PackageEntryObservation{ID: g.packageFact(), Package: p.id, Name: path, Location: g.packageLocation(p, path, origin), Local: unavailableZIPHeader(), Central: unavailableZIPHeader(), HeaderAssociation: packageUnknown[bool]("current-object-has-no-original-headers"), UTF8Name: packageUnknown[bool]("current-object-has-no-original-encoding"), PhysicalOrder: packageUnknown[uint64]("current-object-has-no-original-order")}
	return g.recordPackageEntry(p, e)
}
func (g *resourceGraphRun) currentPackageEntryBytes(p *graphPackage, path string, role mxlAssessmentEntryRole) ([]byte, error) {
	limits, err := g.mxlOptions.limits()
	if err != nil {
		return nil, err
	}
	if err = g.ctx.Err(); err != nil {
		return nil, err
	}
	documentLimit, archiveLimit, automatic := mxlAssessmentRepresentationLimits(role, limits)
	limit := limits.resourceSize
	switch role {
	case mxlAssessmentMetadata:
		limit = limits.metadataSize
	case mxlAssessmentDocument:
		limit = limits.documentSize
	case mxlAssessmentPrimaryDocument, mxlAssessmentLinkedDocument:
		limit = max(documentLimit, archiveLimit)
	}
	// Once copied, classify only our independent immutable copy, even if a caller
	// violates the no-mutation contract after this particular selection.
	data, exists := p.copied[path]
	cached := exists
	if !cached {
		data, exists = p.current[path]
	}
	if !exists {
		return nil, ErrMXLRootFileNotFound
	}
	if int64(len(data)) > limit {
		return nil, ErrMXLTooLarge
	}
	if cached {
		if automatic && int64(len(data)) > mxlAssessmentRepresentationLimit(data[:min(4, len(data))], documentLimit, archiveLimit) {
			return nil, ErrMXLTooLarge
		}
		return data, nil
	}
	if int64(len(data)) > limits.resourcesSize-g.mxlExpanded {
		return nil, ErrMXLTooLarge
	}
	used, err := g.options.account.Usage(validation.BudgetExternalBytes)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > g.options.account.Limits().MaxExternalBytes-used {
		return nil, &validation.ResourceError{Kind: validation.BudgetExternalBytes, Limit: g.options.account.Limits().MaxExternalBytes, Used: used, Requested: int64(len(data))}
	}
	prefixLength := 0
	if automatic {
		prefixLength = min(4, len(data))
		if err = g.options.account.Charge(validation.BudgetExternalBytes, int64(prefixLength)); err != nil {
			return nil, err
		}
		g.mxlExpanded += int64(prefixLength)
		if err = g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return nil, err
		}
		limit = mxlAssessmentRepresentationLimit(data[:prefixLength], documentLimit, archiveLimit)
		if int64(len(data)) > limit {
			return nil, ErrMXLTooLarge
		}
	}
	// Admit bounded copy setup, including an empty selected member.
	if err = g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
		return nil, err
	}
	if err = g.reserve(int64(len(data)) + 256); err != nil {
		return nil, err
	}
	if err = g.options.account.Charge(validation.BudgetExternalBytes, int64(len(data)-prefixLength)); err != nil {
		return nil, err
	}
	g.mxlExpanded += int64(len(data) - prefixLength)
	copied := make([]byte, len(data))
	for at := 0; at < len(data); {
		if err = g.ctx.Err(); err != nil {
			return nil, err
		}
		n := min(4096, len(data)-at)
		if err = g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return nil, err
		}
		copy(copied[at:at+n], data[at:at+n])
		at += n
	}
	p.copied[path] = copied
	return copied, nil
}
