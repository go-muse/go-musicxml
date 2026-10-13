package validation

import (
	"context"
	"strings"
)

// PackageImageAdvice owns optional scoped metadata and exact dimensionless
// ratios. No dimensions in pixels or physical units are converted to tenths.
type PackageImageAdvice struct {
	Source                                     ResourceAdoptionID
	Template                                   uint64
	Request                                    ImageMetadataRequest
	Availability                               Evidence[bool]
	Metadata                                   *ImageMetadataResult
	IntrinsicAspectRatio, SpecifiedAspectRatio Evidence[Rational]
	PreservesAspectRatio                       Evidence[bool]
}
type packageImageAdvice struct {
	subject PackageFactID
	value   PackageImageAdvice
}

// PackageFileMetadataObservation supplies optional presentation metadata for an
// exact admitted boundary/resource pin. Name never proves content validity.
type PackageFileMetadataObservation struct {
	ID              PackageFactID
	Package         PackageID
	Pin             ResourcePin
	Name, MediaType Evidence[string]
	// Context is optional for legacy boundary/adoption observations. Entry and
	// Source are mutually exclusive metadata-only occurrence capabilities; they
	// never admit a parsed resource, adoption, role, ID or graph closure.
	Location              Location
	BaseURI, ArchiveEntry string
	Entry                 PackageFactID
	Source                ResourceAdoptionID
	Template              uint64
	Href                  string
}

// ObserveImageMetadata consumes only an explicitly selected image template and
// exact echoed request. An unavailable optional reply cannot block conformance.
func (s *PackageSession) ObserveImageMetadata(ctx context.Context, source ResourceAdoptionID, template uint64, q ImageMetadataRequest, result ResourceResult) error {
	if err := s.event(ctx, "ObserveImageMetadata"); err != nil {
		return err
	}
	a := s.adoptions[source]
	if a == nil || template >= uint64(len(a.resource.snapshot.contexts)) || s.boundaries[a.packageID].sealed || a.expansion.owner != a.id || a.expansion.sealed {
		return s.fail(contract("image template", "exact source adoption/template required"))
	}
	v := a.resource.snapshot.contexts[template]
	if v.Resource == nil || v.Resource.Image == nil {
		return s.fail(contract("image template", "selected intrinsic image required"))
	}
	local := v.Resource.Image
	if q.Package != a.packageID || q.BaseURI != a.expansion.key.baseURI || q.ArchiveEntry != a.expansion.key.archiveEntry || q.Source != a.resource.pin || q.Subject != (NodeRef{a.documentID, v.Subject.Node}) || local.Source.Value.State != EvidenceKnown || q.Href != local.Source.Value.Value.Text {
		return s.fail(contract("image request", "exact source pin/subject/href required"))
	}
	if local.MediaType.Value.State == EvidenceKnown {
		if q.MediaType != local.MediaType.Value.Value.Text {
			return s.fail(contract("image request", "exact media-type required"))
		}
	} else if q.MediaType != "" {
		return s.fail(contract("image request", "cannot invent unavailable media-type"))
	}
	release, err := s.temporary(ctx, struct {
		Request   ImageMetadataRequest
		Metadata  *ImageMetadataResult
		Reason    Code
		Authority EvidenceAuthority
	}{q, result.Image, result.Reason, result.Authority})
	if err != nil {
		return err
	}
	defer release()
	if err := ValidateResourceResult(ResourceRequest{ID: q.ID, Image: &q}, result); err != nil {
		return s.fail(err)
	}
	if m := result.Image; m != nil {
		for _, field := range []Evidence[Decimal]{m.Width, m.Height, m.PixelAspectRatio} {
			if field.State == EvidenceKnown {
				if err := s.account.Check(BudgetScalarBytes, field.Value.ByteSize()); err != nil {
					return s.fail(err)
				}
			}
		}
	}
	for _, old := range s.output.images {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if err := s.event(ctx, "image uniqueness"); err != nil {
			return err
		}
		if old.value.Request.ID == q.ID || old.value.Source == source && old.value.Template == template {
			return s.fail(contract("image response", "duplicate selected image response"))
		}
	}
	advice := PackageImageAdvice{Source: source, Template: template, Request: q, Availability: Evidence[bool]{State: result.State, Reason: result.Reason}, IntrinsicAspectRatio: Evidence[Rational]{State: EvidenceUnknown, Reason: "image-dimensions-unavailable"}, SpecifiedAspectRatio: Evidence[Rational]{State: EvidenceUnknown, Reason: "specified-dimensions-unavailable"}, PreservesAspectRatio: Evidence[bool]{State: EvidenceUnknown, Reason: "image-aspect-ratio-unavailable"}}
	if result.State == EvidenceKnown {
		advice.Availability.Value = true
		advice.Metadata = result.Image
	}
	arithmetic := &Session{account: s.account}
	defer func() {
		if arithmetic.retained > 0 {
			_ = s.account.Release(BudgetBufferedBytes, arithmetic.retained)
		}
	}()
	if m := result.Image; m != nil {
		if m.Width.State == EvidenceKnown && m.Height.State == EvidenceKnown && (m.Unit != ImageUnitPixel || m.PixelAspectRatio.State == EvidenceKnown) {
			ratio, err := arithmetic.rationalQuoDecimals(ctx, m.Width.Value, m.Height.Value)
			if err != nil {
				return s.fail(err)
			}
			if m.Unit == ImageUnitPixel {
				pixel, err := arithmetic.rationalFromDecimal(ctx, m.PixelAspectRatio.Value)
				if err != nil {
					return s.fail(err)
				}
				ratio, err = arithmetic.rationalMul(ctx, ratio, pixel)
				if err != nil {
					return s.fail(err)
				}
			}
			advice.IntrinsicAspectRatio = Evidence[Rational]{State: EvidenceKnown, Value: ratio}
		}
	}
	width, height := local.Width.Value, local.Height.Value
	if width.State == EvidenceKnown && height.State == EvidenceKnown && width.Value.Kind == AtomicDecimal && height.Value.Kind == AtomicDecimal && width.Value.Decimal.Valid() && height.Value.Decimal.Valid() && width.Value.Decimal.value.Sign() > 0 && height.Value.Decimal.value.Sign() > 0 {
		ratio, err := arithmetic.rationalQuoDecimals(ctx, width.Value.Decimal, height.Value.Decimal)
		if err != nil {
			return s.fail(err)
		}
		advice.SpecifiedAspectRatio = Evidence[Rational]{State: EvidenceKnown, Value: ratio}
		if advice.IntrinsicAspectRatio.State == EvidenceKnown {
			comparison, err := arithmetic.rationalCmp(ctx, ratio, advice.IntrinsicAspectRatio.Value)
			if err != nil {
				return s.fail(err)
			}
			advice.PreservesAspectRatio = Evidence[bool]{State: EvidenceKnown, Value: comparison == 0}
		}
	} else if width.State == EvidenceAbsent || height.State == EvidenceAbsent {
		advice.SpecifiedAspectRatio = Evidence[Rational]{State: EvidenceAbsent}
		advice.PreservesAspectRatio = Evidence[bool]{State: EvidenceAbsent}
	}
	if err := s.admit(ctx, advice, 512); err != nil {
		return err
	}
	subject := s.internalSubject(PackageSubjectImage)
	s.subjectPackages[subject] = a.packageID
	s.output.images = append(s.output.images, packageImageAdvice{subject: subject, value: stateClone(advice)})
	return s.evaluateNewAdvice(ctx, s.output.images[len(s.output.images)-1:], nil)
}

// ObserveFileMetadata retains optional filename/MIME advice for an exact scope.
func (s *PackageSession) ObserveFileMetadata(ctx context.Context, v PackageFileMetadataObservation) error {
	if err := s.event(ctx, "ObserveFileMetadata"); err != nil {
		return err
	}
	b := s.boundaries[v.Package]
	if b == nil {
		return s.fail(contract("file metadata", "exact package scope required"))
	}
	if err := validateResourcePin(v.Pin); err != nil {
		return s.fail(err)
	}
	canonical := packageCanonicalKey{v.Pin.CanonicalID, v.Pin.Revision, v.Pin.Representation}
	prior, pinned := s.pins[canonical]
	if pinned && prior != v.Pin {
		return s.fail(contract("file metadata pin", "canonical identity content conflict"))
	}
	if b.sealed || !validText(v.BaseURI, 16384) || !validText(v.ArchiveEntry, 4096) || !validText(v.Href, 16384) {
		return s.fail(contract("file metadata", "live bounded exact context required"))
	}
	if v.Location != (Location{}) {
		if err := validateLocation(v.Location, TargetPackage, s.documentID); err != nil {
			return s.fail(err)
		}
	}
	if err := s.validateFileMetadataScope(ctx, b, v); err != nil {
		return s.fail(err)
	}
	for _, e := range []Evidence[string]{v.Name, v.MediaType} {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if err := validateEvidence(e, "file metadata"); err != nil {
			return s.fail(err)
		}
		if e.State == EvidenceKnown && !validText(e.Value, 4096) {
			return s.fail(contract("file metadata", "bounded exact text required"))
		}
	}
	extra := int64(256)
	if !pinned {
		extra += 256
	} // canonical lookup entry, separately from metadata
	if err := s.admit(ctx, v, extra); err != nil {
		return err
	}
	if err := s.subject(v.ID, PackageSubjectMetadata); err != nil {
		return s.fail(err)
	}
	s.subjectPackages[v.ID] = v.Package
	owned := stateClone(v)
	// Pin consistency grants no parsed capabilities. Register only after all
	// validation, storage and subject admission succeed; later promotion must
	// use these exact immutable bytes while acquiring its own resource lease.
	canonical = packageCanonicalKey{owned.Pin.CanonicalID, owned.Pin.Revision, owned.Pin.Representation}
	s.pins[canonical] = owned.Pin
	s.output.files = append(s.output.files, owned)
	return s.evaluateNewAdvice(ctx, nil, s.output.files[len(s.output.files)-1:])
}

// Metadata-only admission proves only an observed file occurrence and its
// supplied presentation fields. It deliberately does not populate resources,
// adoptions, roots, roles or expansions.
func (s *PackageSession) validateFileMetadataScope(ctx context.Context, b *packageBoundary, v PackageFileMetadataObservation) error {
	if v.Entry != 0 && v.Source != "" {
		return contract("file metadata", "entry and source occurrence modes are exclusive")
	}
	if v.Entry != 0 {
		e, ok := s.entries[v.Entry]
		if !ok || len(e.RawName) != 0 || e.Package != v.Package || e.Name != v.ArchiveEntry || v.Location == (Location{}) || v.Source != "" || v.Template != 0 || v.Href != "" {
			return contract("file metadata", "exact admitted member occurrence required")
		}
		base := b.identity.BaseURI == v.BaseURI
		for _, id := range s.adoptionOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			if err := s.event(ctx, "file member context"); err != nil {
				return err
			}
			a := s.adoptions[id]
			base = base || a.packageID == v.Package && a.expansion.key.baseURI == v.BaseURI
		}
		if !base {
			return contract("file metadata", "member base must be an admitted package context")
		}
		if v.Pin.Representation != ResourceSourceXML && v.Pin.Representation != ResourceRawMXL && v.Pin.Representation != ResourceBinary {
			return contract("file metadata", "member observation requires immutable byte pin")
		}
		if v.Name.State == EvidenceKnown && v.Name.Value != e.Name {
			return contract("file metadata", "member name must be exact literal entry")
		}
		return nil
	}
	if v.Source != "" {
		a := s.adoptions[v.Source]
		if a == nil || a.packageID != v.Package || a.expansion.owner != a.id || a.expansion.sealed || v.Template >= uint64(len(a.resource.snapshot.contexts)) || v.ArchiveEntry != "" || v.Location == (Location{}) || v.Location.Entry != a.expansion.key.archiveEntry {
			return contract("file metadata", "exact selected external occurrence required")
		}
		t := a.resource.snapshot.contexts[v.Template]
		if t.Resource == nil || t.Resource.Link == nil || t.Resource.Link.Kind == ResourceBookmark || t.Resource.Link.Href.Value.State != EvidenceKnown || t.Resource.Link.Href.Value.Value.Kind != AtomicString || t.Resource.Link.Href.Value.Value.Text != v.Href {
			return contract("file metadata", "external occurrence must match link template")
		}
		if v.Pin.Representation != ResourceSourceXML && v.Pin.Representation != ResourceRawMXL {
			return contract("file metadata", "external document requires immutable XML or MXL pin")
		}
		return nil
	}
	if v.Template != 0 || v.Href != "" {
		return contract("file metadata", "orphan source context")
	}
	context := v.BaseURI != "" || v.ArchiveEntry != ""
	scoped := b.identity.Pin == v.Pin && (!context || b.identity.BaseURI == v.BaseURI && b.identity.ArchiveEntry == v.ArchiveEntry)
	for _, id := range s.adoptionOrder {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if err := s.event(ctx, "file metadata scope"); err != nil {
			return err
		}
		a := s.adoptions[id]
		if a.packageID == v.Package && a.resource.pin == v.Pin && (!context || a.expansion.key.baseURI == v.BaseURI && a.expansion.key.archiveEntry == v.ArchiveEntry) {
			scoped = true
		}
	}
	if !scoped {
		return contract("file metadata", "pin outside exact package/context scope")
	}
	return nil
}

func (s *PackageSession) evaluateNewAdvice(ctx context.Context, images []packageImageAdvice, files []PackageFileMetadataObservation) error {
	for _, key := range s.catalog.orders[s.profile.ID] {
		if err := s.scan(ctx, 1); err != nil {
			return err
		}
		if err := s.evaluateAdviceValues(ctx, s.catalog.rules[key], images, files, false); err != nil {
			return err
		}
	}
	return nil
}
func (s *PackageSession) evaluateAdvice(ctx context.Context, r PackageRuleBinding) error {
	return s.evaluateAdviceValues(ctx, r, s.output.images, s.output.files, true)
}
func (s *PackageSession) evaluateAdviceValues(ctx context.Context, r PackageRuleBinding, images []packageImageAdvice, files []PackageFileMetadataObservation, final bool) error {
	if r.Selector.Kind == PackageSubjectImage {
		for _, v := range images {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			d := s.baseDecision(r.Rule.Key, v.subject)
			d = packageEvidenceDecision(d, v.value.PreservesAspectRatio)
			if v.value.PreservesAspectRatio.State == EvidenceKnown {
				d.Reason = "image-aspect-ratio-preserved"
				if !v.value.PreservesAspectRatio.Value {
					d.Outcome = OutcomeFail
					d.Reason = "image-aspect-ratio-changed"
				}
			}
			if err := s.record(ctx, r.Rule, v.subject, d, Location{Target: TargetPackage, DocumentID: s.documentID, Path: "/image[" + string(v.value.Request.ID) + "]"}, final); err != nil {
				return err
			}
		}
	}
	if r.Selector.Kind == PackageSubjectMetadata {
		for _, v := range files {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			d := s.baseDecision(r.Rule.Key, v.ID)
			d.Outcome = OutcomeNotApplicable
			d.Reason = "file-metadata-absent"
			compressed := v.Pin.Representation == ResourceRawMXL || v.Pin.Representation == ResourceCurrentMXL
			if v.Name.State == EvidenceKnown {
				d.Outcome = OutcomePass
				d.Reason = "recommended-file-name"
				name := v.Name.Value
				if compressed && !strings.HasSuffix(name, ".mxl") || !compressed && !strings.HasSuffix(name, ".musicxml") && !strings.HasSuffix(name, ".xml") {
					d.Outcome = OutcomeFail
					d.Reason = "nonstandard-file-suffix"
				} else if !compressed && strings.HasSuffix(name, ".xml") {
					d.Reason = "legacy-xml-file-suffix"
				}
			} else if v.Name.State != EvidenceAbsent {
				d = packageEvidenceDecision(d, v.Name)
			}
			if v.MediaType.State == EvidenceKnown {
				want := "application/vnd.recordare.musicxml+xml"
				if compressed {
					want = "application/vnd.recordare.musicxml"
				}
				if v.MediaType.Value != want {
					d.Outcome = OutcomeFail
					d.Reason = "nonstandard-file-media-type"
				} else if d.Outcome == OutcomeNotApplicable {
					d.Outcome = OutcomePass
					d.Reason = "recommended-file-media-type"
				}
			} else if v.MediaType.State != EvidenceAbsent && d.Outcome != OutcomeFail {
				d = packageEvidenceDecision(d, v.MediaType)
			}
			location := v.Location
			if location == (Location{}) {
				location = s.location(v.Package)
			}
			if err := s.record(ctx, r.Rule, v.ID, d, location, final); err != nil {
				return err
			}
		}
	}
	return nil
}

// ImageAdviceCount returns selected optional image responses, including misses.
func (b AssessmentBundle) ImageAdviceCount() int {
	if b.data == nil {
		return 0
	}
	return len(b.data.images)
}

// ImageAdviceAt returns an owned metadata/ratio copy after the account is closed.
func (b AssessmentBundle) ImageAdviceAt(i int) (PackageImageAdvice, bool) {
	if b.data == nil || i < 0 || i >= len(b.data.images) {
		return PackageImageAdvice{}, false
	}
	return stateClone(b.data.images[i].value), true
}

// FileMetadataCount returns explicitly supplied optional metadata observations.
func (b AssessmentBundle) FileMetadataCount() int {
	if b.data == nil {
		return 0
	}
	return len(b.data.files)
}

// FileMetadataAt returns one owned optional filename/media-type observation.
func (b AssessmentBundle) FileMetadataAt(i int) (PackageFileMetadataObservation, bool) {
	if b.data == nil || i < 0 || i >= len(b.data.files) {
		return PackageFileMetadataObservation{}, false
	}
	return stateClone(b.data.files[i]), true
}

// MimetypeCount returns retained bounded raw mimetype observations.
func (b AssessmentBundle) MimetypeCount() int {
	if b.data == nil {
		return 0
	}
	return len(b.data.mimetypes)
}

// MimetypeAt returns an independently owned original prefix observation.
func (b AssessmentBundle) MimetypeAt(i int) (PackageMimetypeObservation, bool) {
	if b.data == nil || i < 0 || i >= len(b.data.mimetypes) {
		return PackageMimetypeObservation{}, false
	}
	return stateClone(b.data.mimetypes[i]), true
}
