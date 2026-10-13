package musicxml

import (
	"archive/zip"
	"compress/flate"
	"errors"
	"hash/crc32"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

type mxlAssessmentEntryRole uint8

const (
	mxlAssessmentMetadata mxlAssessmentEntryRole = iota
	mxlAssessmentDocument
	mxlAssessmentResource
	mxlAssessmentLinkedDocument
	mxlAssessmentPrimaryDocument
)

// This includes the DEFLATE history window, Huffman tables, input buffering,
// checksum/descriptor readers and stream wrappers. In Go 1.27 each of two
// Huffman tables has at most 512 overflow links of 64 uint32 values, plus slice
// headers; their conservative combined bound is below 288 KiB. Another 32 KiB
// covers history and the remaining structs, bit arrays and input buffering fit
// comfortably in the balance. This deliberately loose 512 KiB reservation is
// logical admission, not measured heap residency or a default-budget increase.
const mxlZIPDecompressorScratch int64 = 512 << 10

// readEntry returns immutable archive-owned bytes. Callers must neither mutate
// them nor retain them beyond Close; a nested borrower must close first. A
// repeated read reuses the same expanded representation and never recharges its
// ExternalBytes. Even partial bytes/errors are cached so failure cannot become
// a silent retry outside the accounting contract. All selected entry bytes,
// including mimetype, container and primary, count toward MaxResourcesBytes.
func (a *mxlAssessmentArchive) readEntry(index int, role mxlAssessmentEntryRole) (data []byte, err error) {
	if a == nil || a.closed {
		return nil, &validation.ConfigurationError{Field: "archive", Reason: "is closed or unavailable"}
	}
	if index < 0 || index >= len(a.entries) {
		return nil, &validation.ConfigurationError{Field: "entry", Reason: "index outside admitted inventory"}
	}
	var limit int64
	documentLimit, archiveLimit, automatic := mxlAssessmentRepresentationLimits(role, a.limits)
	switch role {
	case mxlAssessmentMetadata:
		limit = a.limits.metadataSize
	case mxlAssessmentDocument:
		limit = a.limits.documentSize
	case mxlAssessmentResource:
		limit = a.limits.resourceSize
	case mxlAssessmentLinkedDocument, mxlAssessmentPrimaryDocument:
		limit = max(documentLimit, archiveLimit)
	default:
		return nil, &validation.ConfigurationError{Field: "entry role", Reason: "unknown role"}
	}
	if err = a.step(1); err != nil {
		return nil, err
	}
	if !a.ready {
		return nil, &mxlAssessmentZIPCapabilityError{feature: "archive inventory is not ready for entry reads"}
	}
	entry := &a.entries[index]
	if entry.directory {
		return nil, &mxlAssessmentZIPPolicyError{reason: "selected entry is a directory"}
	}
	if entry.central.uncompressed > limit {
		return nil, ErrMXLTooLarge
	}
	content := &a.contents[index]
	if content.attempted {
		if automatic {
			cap := mxlAssessmentRepresentationLimit(content.prefix[:content.prefixLength], documentLimit, archiveLimit)
			if entry.central.uncompressed > cap || int64(len(content.data)) > cap {
				return content.data, errors.Join(ErrMXLTooLarge, content.err)
			}
		}
		return content.data, content.err
	}
	content.attempted = true
	defer func() { content.data = data; content.err = err }()
	if entry.central.uncompressed > a.expandedRemaining() {
		return nil, ErrMXLTooLarge
	}
	// Advertised bytes are a rejection hint, never the actual byte-charge owner.
	used, e := a.account.Usage(validation.BudgetExternalBytes)
	if e != nil {
		return nil, e
	}
	if entry.central.uncompressed > a.account.Limits().MaxExternalBytes-used {
		return nil, &validation.ResourceError{Kind: validation.BudgetExternalBytes, Limit: a.account.Limits().MaxExternalBytes, Used: used, Requested: entry.central.uncompressed}
	}
	if err = a.reserve(mxlZIPDecompressorScratch); err != nil {
		return nil, err
	}
	defer a.release(mxlZIPDecompressorScratch)
	if automatic {
		a.activeEntryLimit = limit
		defer func() { a.activeEntryLimit = 0 }()
	}
	// Meter below archive/zip's checksum reader: on a size mismatch that reader
	// can hide the final underlying n. We still charge those expanded bytes once.
	// These fixed built-ins are not caller-provided executable overrides.
	a.reader.RegisterDecompressor(entry.central.method, func(raw io.Reader) io.ReadCloser {
		raw = &mxlAssessmentCompressedReader{archive: a, raw: raw}
		var body io.ReadCloser
		if entry.central.method == zip.Store {
			body = io.NopCloser(raw)
		} else {
			body = flate.NewReader(raw)
		}
		return &mxlAssessmentExpandedReader{archive: a, body: body, limit: limit, content: content, automatic: automatic, documentLimit: documentLimit, archiveLimit: archiveLimit, declared: entry.central.uncompressed}
	})
	stream, err := a.reader.File[index].Open()
	if err != nil {
		return nil, err
	}
	data, err = a.readOpenedEntry(stream, limit)
	if err != nil {
		return data, err
	}
	// archive/zip deliberately skips a zero CRC without a descriptor. Assessment
	// verifies that case too, without mistaking checksum corruption for a clause.
	hash := crc32.NewIEEE()
	for p := 0; p < len(data); {
		n := len(data) - p
		if n > 4096 {
			n = 4096
		}
		if err = a.step(1); err != nil {
			return data, err
		}
		_, _ = hash.Write(data[p : p+n])
		p += n
	}
	if hash.Sum32() != entry.central.crc {
		return data, zip.ErrChecksum
	}
	return data, nil
}

// readOpenedEntry is the sole stream owner after Open. Close runs on all paths,
// including resource/cancellation/transport failure, and preserves both causes.
func (a *mxlAssessmentArchive) readOpenedEntry(stream io.ReadCloser, limit int64) (data []byte, err error) {
	defer func() { err = errors.Join(err, stream.Close()) }()
	return a.readBounded(stream, limit, validation.BudgetExternalBytes, true)
}

type mxlAssessmentExpandedReader struct {
	content                               *mxlAssessmentZIPContent
	archive                               *mxlAssessmentArchive
	body                                  io.ReadCloser
	limit, seen                           int64
	automatic, classified                 bool
	documentLimit, archiveLimit, declared int64
	prefix                                [4]byte
	prefixLength, prefixAt                int
	prefixErr                             error
}

func (r *mxlAssessmentExpandedReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, r.archive.step(1)
	}
	if r.automatic && !r.classified {
		empty := 0
		for r.prefixLength < len(r.prefix) {
			n, err := r.readData(r.prefix[r.prefixLength:])
			r.prefixLength += n
			if err != nil {
				r.prefixErr = err
				break
			}
			if n == 0 {
				empty++
				if empty >= 100 {
					r.prefixErr = io.ErrNoProgress
					break
				}
			} else {
				empty = 0
			}
		}
		r.limit = mxlAssessmentRepresentationLimit(r.prefix[:r.prefixLength], r.documentLimit, r.archiveLimit)
		r.archive.activeEntryLimit = r.limit
		r.classified = true
		if r.seen > r.limit || r.declared > r.limit {
			return 0, errors.Join(ErrMXLTooLarge, r.prefixErr)
		}
	}
	if r.automatic && r.prefixAt < r.prefixLength {
		n := copy(p, r.prefix[r.prefixAt:r.prefixLength])
		r.prefixAt += n
		if r.prefixAt == r.prefixLength {
			return n, r.prefixErr
		}
		return n, nil
	}
	if r.automatic && r.prefixErr != nil {
		return 0, r.prefixErr
	}
	return r.readData(p)
}
func (r *mxlAssessmentExpandedReader) readData(p []byte) (int, error) {
	a := r.archive
	if err := a.step(1); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	remaining := r.limit - r.seen
	if a.expandedRemaining() < remaining {
		remaining = a.expandedRemaining()
	}
	used, err := a.account.Usage(validation.BudgetExternalBytes)
	if err != nil {
		return 0, err
	}
	if a.account.Limits().MaxExternalBytes-used < remaining {
		remaining = a.account.Limits().MaxExternalBytes - used
	}
	want := min(int64(len(p)), mxlZIPReadScratch)
	if want > remaining {
		want = remaining
	}
	if want == 0 {
		want = 1
	}
	n, readErr := r.body.Read(p[:int(want)])
	if n < 0 || n > int(want) {
		return 0, mxlZIPFormat("invalid decompressor count")
	}
	if n > 0 {
		if int64(n) > r.limit-r.seen || int64(n) > a.expandedRemaining() {
			return 0, ErrMXLTooLarge
		}
		if err = a.account.Charge(validation.BudgetExternalBytes, int64(n)); err != nil {
			return 0, err
		}
		if r.content != nil && r.content.prefixLength < len(r.content.prefix) {
			r.content.prefixLength += copy(r.content.prefix[r.content.prefixLength:], p[:n])
		}
		r.seen += int64(n)
		a.expanded += int64(n)
		if a.sharedExpanded != nil {
			*a.sharedExpanded += int64(n)
		}
		if err = a.step(1); err != nil {
			return 0, err
		}
	}
	return n, readErr
}
func (r *mxlAssessmentExpandedReader) Close() error { return r.body.Close() }

// Poll compressed input too: an arbitrarily long sequence of empty DEFLATE
// blocks must not spend work or delay cancellation behind one output Read.
// Compressed bytes were already cumulatively admitted by their upstream owner;
// only processing work is charged here.
type mxlAssessmentCompressedReader struct {
	archive *mxlAssessmentArchive
	raw     io.Reader
}

func (r *mxlAssessmentCompressedReader) Read(p []byte) (int, error) {
	if err := r.archive.step(1); err != nil {
		return 0, err
	}
	if len(p) > 4096 {
		p = p[:4096]
	}
	n, err := r.raw.Read(p)
	if n < 0 || n > len(p) {
		return 0, mxlZIPFormat("invalid compressed reader count")
	}
	if n > 0 {
		if chargeErr := r.archive.step(1); chargeErr != nil {
			return 0, chargeErr
		}
	}
	return n, err
}

// expandedRemaining preserves the private observer's standalone cap while a
// graph-owned shared counter prevents nested boundaries from resetting it.
func (a *mxlAssessmentArchive) expandedRemaining() int64 {
	used := a.expanded
	if a.sharedExpanded != nil {
		used = *a.sharedExpanded
	}
	return a.limits.resourcesSize - used
}

// Auto classification consumes a fixed four-byte prefix in the same bounded
// decompressor stream. The prefix is never separately recharged or retained in
// a growing body before the representation-specific cap is selected.
func mxlAssessmentRepresentationLimits(role mxlAssessmentEntryRole, limits mxlLimits) (document, archive int64, automatic bool) {
	switch role {
	case mxlAssessmentPrimaryDocument:
		return limits.documentSize, min(limits.archiveSize, limits.resourceSize), true
	case mxlAssessmentLinkedDocument:
		return min(limits.documentSize, limits.resourceSize), min(limits.archiveSize, limits.resourceSize), true
	}
	return 0, 0, false
}
func mxlAssessmentZIPPrefixBytes(prefix []byte) bool {
	return len(prefix) >= 4 && prefix[0] == 'P' && prefix[1] == 'K' && ((prefix[2] == 3 && prefix[3] == 4) || (prefix[2] == 5 && prefix[3] == 6))
}
func mxlAssessmentRepresentationLimit(prefix []byte, document, archive int64) int64 {
	if mxlAssessmentZIPPrefixBytes(prefix) {
		return archive
	}
	return document
}
