package musicxml

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"strconv"

	"github.com/go-muse/go-musicxml/validation"
)

type resourceGraphRun struct {
	ctx       context.Context
	options   resourceRunOptions
	session   *validation.PackageSession
	namespace string
	sequence  uint64
	retained  int64
	cache     map[graphResourceKey]*graphResource
	pins      map[graphCanonicalKey]validation.ResourcePin
	expanded  map[graphExpansionKey]bool
	queue     []graphExpansion
	linked    map[graphLinkKey]graphLinkedTarget
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
	namespace, err := newResourceRunIdentity()
	if err != nil {
		return nil, err
	}
	g := &resourceGraphRun{ctx: ctx, options: cfg, namespace: namespace}
	if err = g.reserve(2048 + 2*int64(len(namespace)+len(cfg.identity.CanonicalID)+len(cfg.identity.Revision)+len(cfg.identity.RetrievalURI))); err != nil {
		return nil, err
	}
	g.linked = make(map[graphLinkKey]graphLinkedTarget)
	g.cache = make(map[graphResourceKey]*graphResource)
	g.pins = make(map[graphCanonicalKey]validation.ResourcePin)
	g.expanded = make(map[graphExpansionKey]bool)
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
	return b, errors.Join(err, e)
}

// InspectXMLWithResources assesses original MusicXML/Opus bytes and explicitly
// selected resource dependencies in one finite account. The caller reader stays
// open. Ordinary normative findings have nil operation error; inspect Satisfied.
// The resolver owns transport authorization and blocked-I/O cancellation.
func InspectXMLWithResources(ctx context.Context, reader io.Reader, options LinkedAssessmentOptions) (validation.AssessmentBundle, error) {
	if reader == nil {
		return validation.AssessmentBundle{}, ErrNilReader
	}
	g, err := startResourceGraph(ctx, options, ProfileResourceGraph)
	if err != nil {
		return validation.AssessmentBundle{}, err
	}
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks)
	if err != nil {
		return g.finish(err)
	}
	id := g.next("source")
	result := assessResourceSource(ctx, reader, sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks, Engine: engine}, ByteOwner: sourceBytesPrimary})
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
func AssessModelWithResources(ctx context.Context, document Document, options LinkedAssessmentOptions) (validation.AssessmentBundle, error) {
	if _, err := validationSchemaForDocument(document); err != nil {
		return validation.AssessmentBundle{}, err
	}
	g, err := startResourceGraph(ctx, options, ProfileResourceGraph)
	if err != nil {
		return validation.AssessmentBundle{}, err
	}
	engine, err := modelAssessmentEngineForProfile(ProfileModelLinks)
	if err != nil {
		return g.finish(err)
	}
	id := g.next("model")
	pin := g.primaryPin(validation.ResourceCurrentModel, "")
	result := assessResourceModel(ctx, document, assessmentRunConfig{Account: g.options.account, DocumentID: id, Profile: ProfileModelLinks, Engine: engine})
	return g.finishPrimary(result, pin, id)
}
func (g *resourceGraphRun) finishPrimary(result resourceIntrinsicResult, pin validation.ResourcePin, id string) (validation.AssessmentBundle, error) {
	if result.Snapshot == nil || pin.Representation == validation.ResourceSourceXML && !result.DigestComplete {
		if result.Snapshot != nil {
			_ = result.Snapshot.Close()
		}
		err := result.Err
		if result.Report.Requested {
			err = errors.Join(err, g.session.RecordResourceFailure(g.ctx, pin, result.Report, err))
		}
		return g.finish(err)
	}
	if err := g.session.OpenPackage(g.ctx, validation.PackageIdentity{ID: "root", Representation: pin.Representation, Pin: pin}); err != nil {
		result.Snapshot.Close()
		return g.finish(errors.Join(result.Err, err))
	}
	rid, err := g.session.AddResource(g.ctx, pin, result.Snapshot, true)
	if err != nil {
		result.Snapshot.Close()
		return g.finish(errors.Join(result.Err, err))
	}
	aid, err := g.session.AdoptResource(g.ctx, rid, validation.ResourceAdoptionOptions{DocumentID: id, Root: true, Package: "root", BaseURI: g.options.identity.RetrievalURI})
	if err != nil {
		return g.finish(errors.Join(result.Err, err))
	}
	if result.Err != nil {
		return g.finish(result.Err)
	}
	resource, cacheErr := g.cacheResource(pin, result.Report.Profile.ID, rid)
	if cacheErr != nil {
		return g.finish(cacheErr)
	}
	err = g.enqueue(resource, aid, "root", resourceURIBase{RetrievalURI: g.options.identity.RetrievalURI}, 1)
	if err == nil {
		err = g.walkResources()
	}
	if err == nil {
		known := validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
		err = g.session.SealEnumeration(g.ctx, validation.PackageEnumeration{Package: "root", Entries: known, Rootfiles: known, Edges: known, Roles: known})
	}
	return g.finish(err)
}
