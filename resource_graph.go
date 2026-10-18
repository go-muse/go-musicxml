package musicxml

import (
	"context"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"strconv"

	"github.com/go-muse/go-musicxml/validation"
)

// resourceIntrinsicProfiles fixes the interpretation of each child assembly for
// one root run. A child never infers its profile from external data or resets the
// account. The ordinary bundle is kept separate from opt-in dictionary profiles.
type resourceIntrinsicProfiles struct {
	source, model, container, sounds validation.ProfileID
}

func ordinaryResourceIntrinsicProfiles() resourceIntrinsicProfiles {
	return resourceIntrinsicProfiles{source: ProfileSourceLinks, model: ProfileModelLinks, container: ProfileSourceContainer, sounds: ProfileSourceSounds}
}

type resourceGraphRun struct {
	ctx                   context.Context
	options               resourceRunOptions
	profiles              resourceIntrinsicProfiles
	dictionaries          *validation.DictionarySet
	dictionaryFingerprint [32]byte
	dictionaryErr         error
	session               *validation.PackageSession
	namespace             string
	sequence              uint64
	retained              int64
	cache                 map[graphResourceKey]*graphResource
	pins                  map[graphCanonicalKey]validation.ResourcePin
	expanded              map[graphExpansionKey]bool
	queue                 []graphExpansion
	packageLeases         []func()
	fileMetadata          map[graphFileMetadataKey]bool
	packageList           []*graphPackage
	packages              map[validation.PackageID]*graphPackage
	packageContexts       map[graphPackageKey]*graphPackage
	archives              []*mxlAssessmentArchive
	resourceBodies        []*assessmentResourceBody
	packageDocuments      map[validation.ResourceAssessmentID]string
	mxlOptions            MXLOptions
	mxlExpanded           int64
	linked                map[graphLinkKey]graphLinkedTarget
}

func startResourceGraph(ctx context.Context, options LinkedAssessmentOptions, profile validation.ProfileID) (*resourceGraphRun, error) {
	cfg, err := validateLinkedAssessmentOptions(ctx, options, profile)
	if err != nil {
		return nil, err
	}
	catalog, err := newPackageAssessmentCatalog()
	if err != nil {
		return nil, err
	}
	return startConfiguredResourceGraph(ctx, cfg, profile, catalog, ordinaryResourceIntrinsicProfiles())
}

func startConfiguredResourceGraph(ctx context.Context, cfg resourceRunOptions, profile validation.ProfileID, catalog *validation.CompiledPackageCatalog, profiles resourceIntrinsicProfiles) (*resourceGraphRun, error) {
	namespace, err := newResourceRunIdentity()
	if err != nil {
		return nil, err
	}
	g := &resourceGraphRun{ctx: ctx, options: cfg, namespace: namespace, profiles: profiles}
	if err = g.reserve(2048 + 2*int64(len(namespace)+len(cfg.identity.CanonicalID)+len(cfg.identity.Revision)+len(cfg.identity.RetrievalURI))); err != nil {
		return nil, err
	}
	g.linked = make(map[graphLinkKey]graphLinkedTarget)
	g.cache = make(map[graphResourceKey]*graphResource)
	g.pins = make(map[graphCanonicalKey]validation.ResourcePin)
	g.expanded = make(map[graphExpansionKey]bool)
	g.packages = make(map[validation.PackageID]*graphPackage)
	g.packageContexts = make(map[graphPackageKey]*graphPackage)
	g.packageDocuments = make(map[validation.ResourceAssessmentID]string)
	g.session, err = catalog.Begin(ctx, validation.RunOptions{DocumentID: namespace + "-package", Profile: profile, Account: cfg.account})
	if err != nil {
		g.close()
		return nil, err
	}
	return g, nil
}
func (g *resourceGraphRun) reserve(n int64) error {
	if err := g.ctx.Err(); err != nil {
		return err
	}
	if _, err := validation.AllocationSize(n); err != nil {
		return err
	}
	if err := g.options.account.Charge(validation.BudgetBufferedBytes, n); err != nil {
		return err
	}
	g.retained += n
	return nil
}
func (g *resourceGraphRun) close() {
	// Intrinsic sessions have all finalized before root cleanup; only the root
	// owns this set. Clearing the pointer prevents a second Close on retry.
	if g.dictionaries != nil {
		g.dictionaryErr = joinResourceErrors(g.dictionaryErr, g.dictionaries.Close())
		g.dictionaries = nil
	}
	for i := len(g.archives) - 1; i >= 0; i-- {
		g.archives[i].Close()
	}
	g.archives = nil
	for _, body := range g.resourceBodies {
		body.close()
	}
	g.resourceBodies = nil
	for _, release := range g.packageLeases {
		release()
	}
	g.packageLeases = nil
	g.fileMetadata = nil
	g.packageList = nil
	g.packages = nil
	g.packageContexts = nil
	g.packageDocuments = nil
	if g.retained != 0 {
		_ = g.options.account.Release(validation.BudgetBufferedBytes, g.retained)
		g.retained = 0
	}
}
func (g *resourceGraphRun) next(prefix string) string {
	g.sequence++
	return g.namespace + "-" + prefix + "-" + strconv.FormatUint(g.sequence, 10)
}
func (g *resourceGraphRun) primaryPin(representation validation.ResourceRepresentation, digest string) validation.ResourcePin {
	id := g.options.identity.CanonicalID
	if id == "" {
		id = g.namespace + "-primary"
	}
	p := validation.ResourcePin{CanonicalID: id, Revision: g.options.identity.Revision, Representation: representation, Digest: digest}
	if representation == validation.ResourceCurrentModel || representation == validation.ResourceCurrentMXL {
		p.SnapshotID = g.next("snapshot")
	}
	return p
}
func (g *resourceGraphRun) finish(err error) (validation.AssessmentBundle, error) {
	b, e := g.session.Finish(g.ctx, validation.PackageCompletion{InputComplete: err == nil, Cause: err})
	g.close()
	return b, joinResourceErrors(joinResourceErrors(err, e), g.dictionaryErr)
}

// InspectXMLWithResources assesses original MusicXML/Opus bytes and explicitly
// selected resource dependencies in one finite account. The caller reader stays
// open. Ordinary normative findings have nil operation error; inspect Satisfied.
// The resolver owns transport authorization and blocked-I/O cancellation.
// An empty options.Graph.Profile selects ProfileResourceGraphSemantics with
// the bundled required dictionaries, or options.Graph.Dictionaries. Otherwise
// it must be ProfileResourceGraph, or a dictionary graph profile when
// options.Graph.Dictionaries is set.
func InspectXMLWithResources(ctx context.Context, reader io.Reader, options LinkedAssessmentOptions) (validation.AssessmentBundle, error) {
	if graph, dictionaries, ok := options.Graph.scenario(ProfileResourceGraphSemantics); ok {
		options.Graph = graph
		return inspectXMLWithResourcesAndDictionaries(ctx, reader, LinkedDictionaryAssessmentOptions{Resources: options, Dictionaries: dictionaries})
	}
	if reader == nil {
		return validation.AssessmentBundle{}, ErrNilReader
	}
	g, err := startResourceGraph(ctx, options, ProfileResourceGraph)
	if err != nil {
		return validation.AssessmentBundle{}, err
	}
	return g.inspectPrimarySource(reader)
}

func (g *resourceGraphRun) inspectPrimarySource(reader io.Reader) (validation.AssessmentBundle, error) {
	engine, err := sourceAssessmentEngineForProfile(g.profiles.source)
	if err != nil {
		return g.finish(err)
	}
	id := g.next("source")
	result := assessResourceSource(g.ctx, reader, sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, DTD: g.options.dtd, Dictionaries: g.dictionaries, DocumentID: id, Profile: g.profiles.source, Engine: engine}, ByteOwner: sourceBytesPrimary})
	digest := ""
	if result.DigestComplete {
		digest = hex.EncodeToString(result.Digest[:])
	}
	pin := g.primaryPin(validation.ResourceSourceXML, digest)
	return g.finishPrimary(result, pin, id)
}

// AssessModelWithResources walks the current model directly and never encodes it
// to recover source history. Its opaque representation identity is fresh each
// run. Lost xsi discovery metadata remains explicitly unavailable.
// An empty options.Graph.Profile selects ProfileResourceGraphSemantics with
// the bundled required dictionaries, or options.Graph.Dictionaries. Otherwise
// it must be ProfileResourceGraph, or a dictionary graph profile when
// options.Graph.Dictionaries is set.
func AssessModelWithResources(ctx context.Context, document Document, options LinkedAssessmentOptions) (validation.AssessmentBundle, error) {
	if graph, dictionaries, ok := options.Graph.scenario(ProfileResourceGraphSemantics); ok {
		options.Graph = graph
		return assessModelWithResourcesAndDictionaries(ctx, document, LinkedDictionaryAssessmentOptions{Resources: options, Dictionaries: dictionaries})
	}
	if _, err := validationSchemaForDocument(document); err != nil {
		return validation.AssessmentBundle{}, err
	}
	g, err := startResourceGraph(ctx, options, ProfileResourceGraph)
	if err != nil {
		return validation.AssessmentBundle{}, err
	}
	return g.assessPrimaryModel(document)
}

func (g *resourceGraphRun) assessPrimaryModel(document Document) (validation.AssessmentBundle, error) {
	engine, err := modelAssessmentEngineForProfile(g.profiles.model)
	if err != nil {
		return g.finish(err)
	}
	id := g.next("model")
	pin := g.primaryPin(validation.ResourceCurrentModel, "")
	result := assessResourceModel(g.ctx, document, assessmentRunConfig{Account: g.options.account, DTD: g.options.dtd, Dictionaries: g.dictionaries, DocumentID: id, Profile: g.profiles.model, Engine: engine})
	return g.finishPrimary(result, pin, id)
}
func (g *resourceGraphRun) finishPrimary(result resourceIntrinsicResult, pin validation.ResourcePin, id string) (validation.AssessmentBundle, error) {
	if result.Snapshot == nil || shouldRetainMalformedIntrinsicWithoutAssembly(result) || pin.Representation == validation.ResourceSourceXML && !result.DigestComplete {
		return g.finish(g.retainUntransferredIntrinsic(result, pin, nil))
	}
	if err := g.session.OpenPackage(g.ctx, validation.PackageIdentity{ID: "root", Representation: pin.Representation, Pin: pin, BaseURI: g.options.identity.RetrievalURI}); err != nil {
		return g.finish(g.retainUntransferredIntrinsic(result, pin, err))
	}
	if err := g.observeBoundaryFileMetadata(&graphPackage{id: "root", pin: pin}, g.options.identity.RetrievalURI); err != nil {
		return g.finish(g.retainUntransferredIntrinsic(result, pin, err))
	}
	rid, err := g.session.AddResource(g.ctx, pin, result.Snapshot, true)
	if err != nil {
		return g.finish(g.retainUntransferredIntrinsic(result, pin, err))
	}
	aid, err := g.session.AdoptResource(g.ctx, rid, validation.ResourceAdoptionOptions{DocumentID: id, Root: true, Package: "root", BaseURI: g.options.identity.RetrievalURI})
	if err != nil {
		return g.finish(errors.Join(result.Err, err))
	}
	if result.Err != nil {
		return g.finish(result.Err)
	}
	resource, cacheErr := g.cacheResource(pin, result.Report.Profile.ID, rid, result.XMLDepth, true)
	if cacheErr != nil {
		return g.finish(cacheErr)
	}
	err = g.enqueue(resource, aid, "root", resourceURIBase{RetrievalURI: g.options.identity.RetrievalURI}, 1)
	if err == nil {
		err = g.walkResources()
	}
	if err == nil {
		err = g.observePackageMetadata()
	}
	if err == nil {
		err = g.sealPackages()
	}
	if err == nil {
		known := validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
		err = g.session.SealEnumeration(g.ctx, validation.PackageEnumeration{Package: "root", Entries: known, Rootfiles: known, Edges: known, Roles: known})
	}
	return g.finish(err)
}

// retainUntransferredIntrinsic preserves the independent best report when export
// or later admission fails. Call only before successful AddResource transfers
// the snapshot; afterward the session already owns its canonical report.
func (g *resourceGraphRun) retainUntransferredIntrinsic(result resourceIntrinsicResult, pin validation.ResourcePin, admissionErr error) error {
	err := joinResourceErrors(result.Err, admissionErr)
	if result.Snapshot != nil {
		err = joinResourceErrors(err, result.Snapshot.Close())
	}
	if result.Report.Requested {
		err = joinResourceErrors(err, g.session.RecordResourceFailure(g.ctx, pin, result.Report, err))
	}
	return err
}

// A malformed parse, or a DOCTYPE external subset that could not be supplied,
// can export an owned report before any root assembly exists. Retain that
// failure without manufacturing an extra transfer error. Other unadmitted-root
// contracts (including unsupported roots and empty input) keep their existing
// transfer behavior.
func shouldRetainMalformedIntrinsicWithoutAssembly(result resourceIntrinsicResult) bool {
	if result.Snapshot == nil || result.Snapshot.Header().Assembly.State == validation.EvidenceKnown {
		return false
	}
	var syntax *xml.SyntaxError
	return result.Report.Syntax == validation.SyntaxMalformed && errors.As(result.Err, &syntax) || errors.Is(result.Err, ErrDTDUnavailable)
}

// joinResourceErrors preserves unrelated causes without repeating a cause that
// the other error already wraps. In particular Finish includes its input cause.
func joinResourceErrors(a, b error) error {
	if a == nil || errors.Is(b, a) {
		return b
	}
	if b == nil || errors.Is(a, b) {
		return a
	}
	return errors.Join(a, b)
}
