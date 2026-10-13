package musicxml

import (
	"mime"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

type graphFileMetadataKey struct {
	pkg                 validation.PackageID
	pin                 validation.ResourcePin
	base, entry, origin string
	member              validation.PackageFactID
	source              validation.ResourceAdoptionID
	template            uint64
}

// observePackageFileMetadata is the final metadata seam. File observations are
// ingressed as soon as their immutable bytes are verified, before parsing can
// fail. Nothing is fetched, reparsed or promoted to a resource by this pass.
func (g *resourceGraphRun) observePackageFileMetadata() error { return g.ctx.Err() }

func (g *resourceGraphRun) observeFileMetadata(v validation.PackageFileMetadataObservation) error {
	key := graphFileMetadataKey{v.Package, v.Pin, v.BaseURI, v.ArchiveEntry, v.Location.Path, v.Entry, v.Source, v.Template}
	if g.fileMetadata[key] {
		return nil
	}
	if err := g.session.ObserveFileMetadata(g.ctx, v); err != nil {
		return err
	}
	// Core owns its copy; this bounded key prevents a later adoption from
	// producing duplicate advice for the same occurrence and retrieval context.
	n := int64(len(key.base) + len(key.entry) + len(key.origin) + len(key.pkg) + len(key.pin.CanonicalID) + len(key.pin.Revision) + len(key.pin.Digest) + len(key.pin.SnapshotID))
	if err := g.reserve(512 + 2*n); err != nil {
		return err
	}
	key.base, key.entry, key.origin = strings.Clone(key.base), strings.Clone(key.entry), strings.Clone(key.origin)
	key.pkg = validation.PackageID(strings.Clone(string(key.pkg)))
	key.pin.CanonicalID, key.pin.Revision = strings.Clone(key.pin.CanonicalID), strings.Clone(key.pin.Revision)
	key.pin.Digest, key.pin.SnapshotID = strings.Clone(key.pin.Digest), strings.Clone(key.pin.SnapshotID)
	if g.fileMetadata == nil {
		g.fileMetadata = make(map[graphFileMetadataKey]bool)
	}
	g.fileMetadata[key] = true
	return nil
}

func (g *resourceGraphRun) observeLocalFileMetadata(p *graphPackage, path string, data []byte, retrieval string, location validation.Location, media validation.Evidence[string]) error {
	digest, err := g.packageDigest(data)
	if err != nil {
		return err
	}
	representation := validation.ResourceSourceXML
	if mxlAssessmentZIPPrefixBytes(data) {
		representation = validation.ResourceRawMXL
	}
	pin := g.entryPin(p, path, digest, representation)
	v := validation.PackageFileMetadataObservation{ID: g.packageFact(), Package: p.id, Pin: pin, Name: packageKnown(path), MediaType: media, Location: location, BaseURI: retrieval, ArchiveEntry: path, Entry: p.entries[path]}
	return g.observeFileMetadata(v)
}

// URI names preserve escaped path spelling, so %2F never becomes a different
// member boundary. This is URL presentation metadata, not a physical filename.
func (g *resourceGraphRun) fileNameFromRetrieval(retrieval string) (validation.Evidence[string], error) {
	absent := validation.Evidence[string]{State: validation.EvidenceAbsent}
	if retrieval == "" {
		return absent, nil
	}
	if err := g.options.account.Charge(validation.BudgetRuleSteps, int64(len(retrieval))+1); err != nil {
		return absent, err
	}
	scratch := int64(len(retrieval))*16 + 1024
	if err := g.options.account.Charge(validation.BudgetBufferedBytes, scratch); err != nil {
		return absent, err
	}
	defer g.options.account.Release(validation.BudgetBufferedBytes, scratch)
	u, err := url.Parse(retrieval)
	if err != nil || u.Opaque != "" || u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "file" {
		return packageUnknown[string]("retrieval-name-unavailable"), nil
	}
	path := u.EscapedPath()
	if path == "" || strings.HasSuffix(path, "/") {
		return absent, nil
	}
	name := path[strings.LastIndexByte(path, '/')+1:]
	if len(name) > 4096 {
		return packageUnknown[string]("retrieval-name-unavailable"), nil
	}
	return packageKnown(name), nil
}
func (g *resourceGraphRun) externalFileMedia(value string) (validation.Evidence[string], error) {
	if value == "" {
		return validation.Evidence[string]{State: validation.EvidenceAbsent}, nil
	}
	if err := g.options.account.Charge(validation.BudgetRuleSteps, int64(len(value))+1); err != nil {
		return validation.Evidence[string]{}, err
	}
	scratch := int64(len(value))*16 + 1024
	if err := g.options.account.Charge(validation.BudgetBufferedBytes, scratch); err != nil {
		return validation.Evidence[string]{}, err
	}
	defer g.options.account.Release(validation.BudgetBufferedBytes, scratch)
	if _, _, err := mime.ParseMediaType(value); err != nil {
		return validation.Evidence[string]{State: validation.EvidenceInvalid, Reason: "supplied-media-type-invalid"}, nil
	}
	return packageKnown(value), nil
}
func (g *resourceGraphRun) observeExternalFileMetadata(item graphExpansion, index uint64, result validation.ResourceResult, base resourceURIBase, href string) error {
	name, err := g.fileNameFromRetrieval(base.RetrievalURI)
	if err != nil {
		return err
	}
	media, err := g.externalFileMedia(result.MediaType)
	if err != nil {
		return err
	}
	v := validation.PackageFileMetadataObservation{ID: g.packageFact(), Package: item.pkg, Pin: result.Pin, Name: name, MediaType: media, Location: g.packageLocation(item.base.Entry, "external-document["+strconv.FormatUint(index, 10)+"]/retrieval-uri"), BaseURI: base.RetrievalURI, Source: item.adoption, Template: index, Href: href}
	return g.observeFileMetadata(v)
}
func (g *resourceGraphRun) observeBoundaryFileMetadata(p *graphPackage, retrieval string) error {
	if p.parent != "" || retrieval == "" || p.pin.Digest == "" && p.pin.SnapshotID == "" {
		return nil
	}
	name, err := g.fileNameFromRetrieval(retrieval)
	if err != nil {
		return err
	}
	return g.observeFileMetadata(validation.PackageFileMetadataObservation{ID: g.packageFact(), Package: p.id, Pin: p.pin, Name: name, MediaType: validation.Evidence[string]{State: validation.EvidenceAbsent}, Location: g.packageLocation("", "package/retrieval-uri"), BaseURI: retrieval})
}
func (g *resourceGraphRun) observeCurrentPrimaryFileMetadata(p *graphPackage, r validation.PackageRootfileObservation, retrieval string) error {
	if p.primary == nil || r.FullPath.State != validation.EvidenceKnown {
		return nil
	}
	return g.observeFileMetadata(validation.PackageFileMetadataObservation{ID: g.packageFact(), Package: p.id, Pin: p.primary.pin, Name: r.FullPath, MediaType: r.MediaType, Location: r.Location, BaseURI: retrieval, ArchiveEntry: r.FullPath.Value})
}
