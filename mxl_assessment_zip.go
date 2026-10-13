package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

// This is a private processing boundary, not a ZIP or MusicXML validator.
// Supported envelopes are ordinary single-volume, non-ZIP64 STORE/DEFLATE,
// including 12- and 16-byte descriptors. Errors below do not make normative
// decisions. In particular local and central mimetype extras stay independent.
// Framing failures wrap zip.ErrFormat; entry reads preserve zip.ErrChecksum,
// io.ErrUnexpectedEOF and flate.CorruptInputError as transport errors. They are
// operation failures, not MusicXML normative findings.
const (
	mxlZIPOwnerBytes  int64 = 1024
	mxlZIPReadScratch int64 = 4096
	// This conservative logical allowance covers raw records, exact physical
	// indices, content cache, map buckets, zip.File objects, growing zip.File
	// pointer slices (including overlapping growth), and per-file time metadata.
	// Four copies of variable bytes additionally cover raw strings, zip's backing
	// bytes and strings, path normalization and temporary field comparisons.
	mxlZIPEntryAllowance int64 = 2048
	// Go's EOCD search uses two buffers (1024 and 65 KiB), then bufio uses 4 KiB.
	// Both EOCD buffers plus comment copies and the failed terminal File fit here.
	mxlZIPReaderScratch int64 = 256 << 10
)

type mxlAssessmentZIPCapabilityError struct{ feature string }

func (e *mxlAssessmentZIPCapabilityError) Error() string {
	return "MXL ZIP assessment capability unavailable: " + e.feature
}

type mxlAssessmentZIPPolicyError struct {
	reason string
	cause  error
}

func (e *mxlAssessmentZIPPolicyError) Unwrap() error {
	return errors.Join(validation.ErrProcessingPolicy, e.cause)
}

func (e *mxlAssessmentZIPPolicyError) Error() string {
	return "MXL ZIP assessment processing policy: " + e.reason
}

type mxlAssessmentZIPHeader struct {
	name, extra, comment     string // Independently owned immutable raw bytes.
	flags, method            uint16
	crc                      uint32
	compressed, uncompressed int64
}
type mxlAssessmentZIPEntry struct {
	central, local                                 mxlAssessmentZIPHeader
	centralOrdinal                                 int
	localOffset, bodyStart, bodyEnd, end           int64
	physicalOrder                                  int
	associated, localObserved, utf8Name, directory bool
	trustedIdentity                                bool // Exact envelope plus all local ranges are unambiguous.
	creatorVersion                                 uint16
	externalAttrs                                  uint32
}

// A prefix is only a fully framed local header at byte zero. It has no proved
// central-directory association and never supplies complete entry inventory.
// Its distinct type prevents promoting an arbitrary PK signature to an entry.
type mxlAssessmentZIPPrefix struct {
	framed    bool
	header    mxlAssessmentZIPHeader
	bodyStart int64
}

type mxlAssessmentZIPContent struct {
	prefix       [64]byte
	prefixLength int
	attempted    bool
	data         []byte
	err          error
}

type mxlAssessmentArchive struct {
	ctx                      context.Context
	account                  *validation.Account
	limits                   mxlLimits
	data                     []byte
	entries                  []mxlAssessmentZIPEntry // Central-directory order, exact capacity.
	physical                 []int                   // Independently verified local-header order.
	index                    map[string]int
	prefix                   mxlAssessmentZIPPrefix
	reader                   *zip.Reader
	contents                 []mxlAssessmentZIPContent
	inventoryComplete        bool // Complete raw enumeration, not a processing verdict.
	declaredEntryCount       uint16
	actualEntryCount         int64
	centralDirectoryComplete bool
	countConsistent          bool
	ready                    bool
	retained, expanded       int64
	closed                   bool
	inputComplete            bool
	activeEntryLimit         int64  // Nonzero only during selected bounded prefix classification.
	sharedExpanded           *int64 // Optional root-run aggregate; does not own another Account.
}

// openMXLAssessmentArchive owns its buffered compressed representation. The
// caller's reader is never closed. Every nonnil result, including on error,
// must be closed after consuming its partial observations. Cancellation polls
// cannot interrupt an arbitrary blocked caller Read; no goroutine is leaked to
// simulate an interrupt. The private owner is single-threaded.
func openMXLAssessmentArchive(ctx context.Context, reader io.Reader, options MXLOptions, account *validation.Account) (*mxlAssessmentArchive, error) {
	a, err := newMXLAssessmentArchive(ctx, options, account)
	if err != nil {
		return nil, err
	}
	if reader == nil {
		return a, &validation.ConfigurationError{Field: "reader", Reason: "must not be nil"}
	}
	a.data, err = a.readBounded(reader, a.limits.archiveSize, validation.BudgetInputBytes, false)
	if err != nil {
		a.observePrefix()
		return a, err
	}
	a.inputComplete = true
	err = a.observe()
	return a, err
}

// borrowMXLAssessmentArchive borrows an already-accounted complete immutable
// representation. Its upstream owner MUST keep both bytes and their live-buffer
// reservation alive, unchanged, until this archive is closed. This constructor
// does not charge InputBytes/ExternalBytes or duplicate the upstream reservation.
// It still admits all observation/index/decompressor/expanded-byte ownership.
func borrowMXLAssessmentArchive(ctx context.Context, data []byte, options MXLOptions, account *validation.Account) (*mxlAssessmentArchive, error) {
	a, err := newMXLAssessmentArchive(ctx, options, account)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > a.limits.archiveSize {
		return a, ErrMXLTooLarge
	}
	a.data = data
	a.inputComplete = true
	err = a.observe()
	return a, err
}

func newMXLAssessmentArchive(ctx context.Context, options MXLOptions, account *validation.Account) (*mxlAssessmentArchive, error) {
	if ctx == nil {
		return nil, &validation.ConfigurationError{Field: "context", Reason: "must not be nil"}
	}
	limits, err := options.limits()
	if err != nil {
		return nil, err
	}
	if _, err = account.Usage(validation.BudgetBufferedBytes); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = account.Charge(validation.BudgetBufferedBytes, mxlZIPOwnerBytes); err != nil {
		return nil, err
	}
	return &mxlAssessmentArchive{ctx: ctx, account: account, limits: limits, retained: mxlZIPOwnerBytes}, nil
}

// Close releases only this owner's reservations, exactly once. It does no
// cancellable work. Neither a borrowed upstream buffer nor caller I/O is closed.
func (a *mxlAssessmentArchive) Close() {
	if a == nil || a.closed {
		return
	}
	a.closed = true
	a.data = nil
	a.entries = nil
	a.physical = nil
	a.index = nil
	a.reader = nil
	a.contents = nil
	a.prefix = mxlAssessmentZIPPrefix{}
	_ = a.account.Release(validation.BudgetBufferedBytes, a.retained)
	a.retained = 0
}
func (a *mxlAssessmentArchive) step(n int64) error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	return a.account.Charge(validation.BudgetRuleSteps, n)
}
func (a *mxlAssessmentArchive) reserve(n int64) error {
	if n < 0 || n > math.MaxInt64-a.retained {
		return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Limit: math.MaxInt64, Used: a.retained, Requested: n}
	}
	if _, err := validation.AllocationSize(n); err != nil {
		return err
	}
	if err := a.account.Charge(validation.BudgetBufferedBytes, n); err != nil {
		return err
	}
	a.retained += n
	return nil
}
func (a *mxlAssessmentArchive) release(n int64) {
	_ = a.account.Release(validation.BudgetBufferedBytes, n)
	a.retained -= n
}

// readBounded charges primary bytes; expanded streams are metered underneath
// archive/zip so its checksum wrapper cannot hide counts. Its one-byte EOF probe can
// discover an excess at an exact limit without max+1 arithmetic or retention.
// Read attempts (including empty reads/EOF probes) and append/copy work each
// charge a unit per at most 4096 bytes. Byte quotas still use exact counts.
// Capacity growth is reserved while the old backing array is still live.
func (a *mxlAssessmentArchive) readBounded(reader io.Reader, limit int64, kind validation.BudgetKind, expanded bool) ([]byte, error) {
	if err := a.reserve(mxlZIPReadScratch); err != nil {
		return nil, err
	}
	defer a.release(mxlZIPReadScratch)
	var scratch [mxlZIPReadScratch]byte
	var data []byte
	emptyReads := 0
	for {
		if err := a.step(1); err != nil {
			return data, err
		}
		remaining := limit - int64(len(data))
		if expanded && a.expandedRemaining() < remaining {
			remaining = a.expandedRemaining()
		}
		used, err := a.account.Usage(kind)
		if err != nil {
			return data, err
		}
		maximum := a.account.Limits().MaxInputBytes
		if kind == validation.BudgetExternalBytes {
			maximum = a.account.Limits().MaxExternalBytes
		}
		if maximum-used < remaining {
			remaining = maximum - used
		}
		want := int64(len(scratch))
		if want > remaining {
			want = remaining
		}
		if want == 0 {
			want = 1
		}
		n, readErr := reader.Read(scratch[:int(want)])
		if n < 0 || n > int(want) {
			return data, fmt.Errorf("%w: invalid Reader count", zip.ErrFormat)
		}
		if expanded && a.activeEntryLimit > 0 && a.activeEntryLimit < limit {
			limit = a.activeEntryLimit
		}
		if n > 0 {
			emptyReads = 0
			if int64(n) > limit-int64(len(data)) {
				return data, errors.Join(ErrMXLTooLarge, readErr)
			}
			if !expanded {
				if err = a.account.Charge(kind, int64(n)); err != nil {
					return data, err
				}
			}
			if err = a.step(1); err != nil {
				return data, err
			}
			target := int64(len(data)) + int64(n)
			if target > int64(cap(data)) {
				capacity := limit
				if int64(cap(data)) <= limit/2 {
					capacity = int64(cap(data)) * 2
				}
				if capacity < mxlZIPReadScratch {
					capacity = mxlZIPReadScratch
				}
				if capacity < target {
					capacity = target
				}
				if capacity > limit {
					capacity = limit
				}
				if err = a.reserve(capacity); err != nil {
					return data, err
				}
				size, err := validation.AllocationSize(capacity)
				if err != nil {
					a.release(capacity)
					return data, err
				}
				next := make([]byte, len(data), size)
				if err = a.copyBytes(next, data); err != nil {
					a.release(capacity)
					return data, err
				}
				a.release(int64(cap(data)))
				data = next
			}
			data = append(data, scratch[:n]...)
		} else if readErr == nil {
			emptyReads++
			if emptyReads >= 100 {
				return data, io.ErrNoProgress
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return data, nil
			}
			return data, readErr
		}
	}
}

// copyBytes meters actual relocation, rather than a rounded charge followed by
// one unbounded copy. Both backing arrays stay reserved until this completes.
func (a *mxlAssessmentArchive) copyBytes(dst, src []byte) error {
	for at := 0; at < len(src); {
		if err := a.step(1); err != nil {
			return err
		}
		n := min(int(mxlZIPReadScratch), len(src)-at)
		copy(dst[at:at+n], src[at:at+n])
		at += n
	}
	return nil
}

func mxlZIPU16(b []byte, at int) uint16 { return binary.LittleEndian.Uint16(b[at : at+2]) }
func mxlZIPU32(b []byte, at int) uint32 { return binary.LittleEndian.Uint32(b[at : at+4]) }
func mxlZIPRange(data []byte, start, size int64) ([]byte, bool) {
	if start < 0 || size < 0 || start > int64(len(data)) || size > int64(len(data))-start {
		return nil, false
	}
	return data[int(start):int(start+size)], true
}
func mxlZIPFormat(reason string) error { return fmt.Errorf("%w: %s", zip.ErrFormat, reason) }

func (a *mxlAssessmentArchive) observePrefix() {
	h, ok := mxlZIPRange(a.data, 0, 30)
	if !ok || mxlZIPU32(h, 0) != 0x04034b50 {
		return
	}
	n, x := int64(mxlZIPU16(h, 26)), int64(mxlZIPU16(h, 28))
	v, ok := mxlZIPRange(a.data, 30, n+x)
	if !ok {
		return
	}
	// Only the literal framed mimetype prefix is independently useful. Do not
	// retain arbitrary unindexed payload records or scan for later signatures.
	if n != int64(len(mxlMIMETypePath)) || !bytes.Equal(v[:int(n)], []byte(mxlMIMETypePath)) {
		return
	}
	if a.account.Check(validation.BudgetScalarBytes, n) != nil || a.account.Check(validation.BudgetScalarBytes, x) != nil {
		return
	}
	if a.step(n+x+30) != nil || a.reserve(n+x) != nil {
		return
	}
	a.prefix = mxlAssessmentZIPPrefix{framed: true, header: mxlZIPLocalHeader(h, v, n), bodyStart: 30 + n + x}
}
func mxlZIPLocalHeader(h, v []byte, n int64) mxlAssessmentZIPHeader {
	return mxlAssessmentZIPHeader{name: string(v[:int(n)]), extra: string(v[int(n):]), flags: mxlZIPU16(h, 6), method: mxlZIPU16(h, 8), crc: mxlZIPU32(h, 14), compressed: int64(mxlZIPU32(h, 18)), uncompressed: int64(mxlZIPU32(h, 22))}
}

type mxlAssessmentZIPEnd struct {
	offset, start, size int64
	count               uint16
	comment             string
}

func (a *mxlAssessmentArchive) endRecord() (mxlAssessmentZIPEnd, error) {
	var zero mxlAssessmentZIPEnd
	var envelopeFeature string
	low := len(a.data) - (65535 + 22)
	if low < 0 {
		low = 0
	}
	for p := len(a.data) - 22; p >= low; p-- {
		if err := a.step(1); err != nil {
			return zero, err
		}
		h := a.data[p : p+22]
		if mxlZIPU32(h, 0) != 0x06054b50 {
			continue
		}
		commentEnd := int64(p) + 22 + int64(mxlZIPU16(h, 20))
		if commentEnd != int64(len(a.data)) {
			start, size := int64(mxlZIPU32(h, 16)), int64(mxlZIPU32(h, 12))
			if commentEnd < int64(len(a.data)) && start <= int64(p) && size == int64(p)-start {
				envelopeFeature = "trailing ZIP envelope"
			}
			continue
		}
		start, size := int64(mxlZIPU32(h, 16)), int64(mxlZIPU32(h, 12))
		if start > int64(p) || size != int64(p)-start {
			// Sentinel fields cannot frame an ordinary directory. A ZIP64 claim
			// needs its own framed end/locator chain; arbitrary signature bytes
			// in a comment cannot identify an unsupported envelope feature.
			if a.framedZIP64End(p) {
				return zero, &mxlAssessmentZIPCapabilityError{feature: "ZIP64 envelope"}
			}
			if start <= int64(p) && size < int64(p)-start {
				envelopeFeature = "preamble or bytes outside ordinary directory envelope"
			}
			continue
		}
		if mxlZIPU16(h, 4) != 0 || mxlZIPU16(h, 6) != 0 || mxlZIPU16(h, 8) != mxlZIPU16(h, 10) {
			return zero, &mxlAssessmentZIPCapabilityError{feature: "multi-volume archive"}
		}
		if mxlZIPU16(h, 10) == 0xffff {
			return zero, &mxlAssessmentZIPCapabilityError{feature: "ZIP64 envelope"}
		}
		comment := a.data[p+22:]
		if err := a.account.Check(validation.BudgetScalarBytes, int64(len(comment))); err != nil {
			return zero, err
		}
		// archive/zip uses a different comment-signature selection rule. Verify its
		// selected EOCD BEFORE it can allocate from an unadmitted alternate envelope.
		goEnd, err := a.goEndOffset()
		if err != nil {
			return zero, err
		}
		if goEnd != p {
			return zero, &mxlAssessmentZIPCapabilityError{feature: "ambiguous EOCD signature in comment"}
		}
		if err := a.reserve(int64(len(comment))); err != nil {
			return zero, err
		}
		return mxlAssessmentZIPEnd{offset: int64(p), start: start, size: size, count: mxlZIPU16(h, 10), comment: string(comment)}, nil
	}
	if envelopeFeature != "" {
		return zero, &mxlAssessmentZIPCapabilityError{feature: envelopeFeature}
	}
	return zero, mxlZIPFormat("missing or truncated end-of-central-directory record")
}

// framedZIP64End recognizes only the bounded end-record chain and directory
// extent. It does not admit ZIP64 parsing or consult signatures inside ordinary
// directory fields. All uint64 offsets are bounded before conversion/slicing.
func (a *mxlAssessmentArchive) framedZIP64End(end int) bool {
	if end < 20+56 {
		return false
	}
	locator := a.data[end-20 : end]
	if mxlZIPU32(locator, 0) != 0x07064b50 {
		return false
	}
	offset := binary.LittleEndian.Uint64(locator[8:16])
	if offset > uint64(end-20-56) {
		return false
	}
	record := a.data[int(offset) : int(offset)+56]
	if mxlZIPU32(record, 0) != 0x06064b50 || binary.LittleEndian.Uint64(record[4:12]) != uint64(end-20)-offset-12 {
		return false
	}
	size := binary.LittleEndian.Uint64(record[40:48])
	start := binary.LittleEndian.Uint64(record[48:56])
	return start <= offset && size == offset-start
}

// Mirrors only Go 1.27's bounded EOCD choice, not its parser. Admission does not
// call archive/zip until that choice agrees with our exact envelope candidate.
func (a *mxlAssessmentArchive) goEndOffset() (int, error) {
	data := a.data
	for _, window := range []int{1024, 65 * 1024} {
		start := len(data) - window
		if start < 0 {
			start = 0
		}
		for p := len(data) - 22; p >= start; p-- {
			if err := a.step(1); err != nil {
				return -1, err
			}
			if mxlZIPU32(data[p:p+4], 0) != 0x06054b50 {
				continue
			}
			if int64(p)+22+int64(mxlZIPU16(data[p:p+22], 20)) > int64(len(data)) {
				break
			}
			return p, nil
		}
		if start == 0 {
			break
		}
	}
	return -1, nil
}

func (a *mxlAssessmentArchive) observe() error {
	a.observePrefix()
	end, err := a.endRecord()
	if err != nil {
		return err
	}
	a.declaredEntryCount = end.count
	count, variable := int64(0), int64(0)
	// First pass walks the actual exact directory range with no entry-sized
	// allocation. Wrapped EOCD counts cannot evade cumulative node admission.
	for p := end.start; p < end.offset; {
		h, ok := mxlZIPRange(a.data, p, 46)
		if !ok || p+46 > end.offset || mxlZIPU32(h, 0) != 0x02014b50 {
			return mxlZIPFormat("central directory framing")
		}
		n, x, c := int64(mxlZIPU16(h, 28)), int64(mxlZIPU16(h, 30)), int64(mxlZIPU16(h, 32))
		total := n + x + c
		if total > end.offset-p-46 {
			return mxlZIPFormat("central variable fields outside directory")
		}
		if err = a.step(46 + total); err != nil {
			return err
		}
		for _, length := range []int64{n, x, c} {
			if err = a.account.Check(validation.BudgetScalarBytes, length); err != nil {
				return err
			}
		}
		if err = a.account.Charge(validation.BudgetNodes, 1); err != nil {
			return err
		}
		count++
		variable += total
		p += 46 + total
	}
	a.actualEntryCount = count
	a.centralDirectoryComplete = true
	a.countConsistent = count == int64(end.count)
	if uint16(count) != end.count {
		return mxlZIPFormat("central record count mismatch")
	}
	allowance, err := mxlZIPInventoryAllowance(count, variable)
	if err != nil {
		return err
	}
	if err = a.reserve(allowance); err != nil {
		return err
	}
	length, err := validation.AllocationSize(count)
	if err != nil {
		return err
	}
	a.entries = make([]mxlAssessmentZIPEntry, length)
	a.physical = make([]int, length)
	a.contents = make([]mxlAssessmentZIPContent, length)
	a.index = make(map[string]int, length)
	p := end.start
	var unsupported error
	if !a.countConsistent {
		unsupported = &mxlAssessmentZIPCapabilityError{feature: "wrapped central-directory record count"}
	}
	for i := range a.entries {
		if err = a.step(1); err != nil {
			return err
		}
		h := a.data[int(p) : int(p)+46]
		n, x, c := int(mxlZIPU16(h, 28)), int(mxlZIPU16(h, 30)), int(mxlZIPU16(h, 32))
		v := a.data[int(p)+46 : int(p)+46+n+x+c]
		e := &a.entries[i]
		e.central = mxlAssessmentZIPHeader{name: string(v[:n]), extra: string(v[n : n+x]), comment: string(v[n+x:]), flags: mxlZIPU16(h, 8), method: mxlZIPU16(h, 10), crc: mxlZIPU32(h, 16), compressed: int64(mxlZIPU32(h, 20)), uncompressed: int64(mxlZIPU32(h, 24))}
		e.centralOrdinal = i
		e.physicalOrder = -1
		e.localOffset = int64(mxlZIPU32(h, 42))
		e.creatorVersion = mxlZIPU16(h, 4)
		e.externalAttrs = mxlZIPU32(h, 38)
		header := zip.FileHeader{Name: e.central.name, CreatorVersion: e.creatorVersion, ExternalAttrs: e.externalAttrs}
		mode := header.Mode()
		e.directory = mode.IsDir()
		e.utf8Name = utf8.ValidString(e.central.name)
		if err = a.step(int64(n + x + c)); err != nil {
			return err
		}
		if !validMXLPath(e.central.name, e.directory) {
			return &mxlAssessmentZIPPolicyError{reason: "unsafe or noncanonical archive path", cause: ErrMXLInvalidPath}
		}
		if _, ok := a.index[e.central.name]; ok {
			return &mxlAssessmentZIPPolicyError{reason: "duplicate archive name", cause: ErrMXLDuplicateEntry}
		}
		a.index[e.central.name] = i
		a.physical[i] = i
		if mxlZIPU16(h, 34) != 0 && unsupported == nil {
			unsupported = &mxlAssessmentZIPCapabilityError{feature: "multi-volume entry"}
		}
		if e.localOffset == math.MaxUint32 || e.central.compressed == math.MaxUint32 || e.central.uncompressed == math.MaxUint32 {
			return &mxlAssessmentZIPCapabilityError{feature: "ZIP64 entry"}
		}
		if err = a.observeLocal(e, end.start); err != nil {
			return err
		}
		if err = a.checkExtra(e.local.extra); err != nil {
			return err
		}
		if err = a.checkExtra(e.central.extra); err != nil {
			return err
		}
		if (e.local.flags|e.central.flags)&0x2041 != 0 && unsupported == nil {
			unsupported = &mxlAssessmentZIPCapabilityError{feature: "encrypted entry"}
		}
		if (e.local.flags|e.central.flags) & ^uint16(0x080e) != 0 && unsupported == nil {
			unsupported = &mxlAssessmentZIPCapabilityError{feature: "unreviewed general-purpose ZIP flags"}
		}
		if e.central.method != zip.Store && e.central.method != zip.Deflate && unsupported == nil {
			unsupported = &mxlAssessmentZIPCapabilityError{feature: "compression method outside STORE/DEFLATE"}
		}
		// Metadata only; no symlink is ever followed or extracted.
		if mode&fs.ModeSymlink != 0 {
			return &mxlAssessmentZIPPolicyError{reason: "symbolic-link archive entry"}
		}
		p += int64(46 + n + x + c)
	}
	if err = a.sortPhysical(); err != nil {
		return err
	}
	next := int64(0)
	for rank, i := range a.physical {
		if err = a.step(1); err != nil {
			return err
		}
		e := &a.entries[i]
		if e.localOffset < next {
			return &mxlAssessmentZIPPolicyError{reason: "overlapping local entry range"}
		}
		if e.localOffset != next {
			return &mxlAssessmentZIPCapabilityError{feature: "preamble or unindexed bytes between local entries"}
		}
		e.physicalOrder = rank
		next = e.end
	}
	if next != end.start {
		return &mxlAssessmentZIPCapabilityError{feature: "unindexed bytes before central directory"}
	}
	a.inventoryComplete = true
	for i := range a.entries {
		if err = a.step(1); err != nil {
			return err
		}
		a.entries[i].trustedIdentity = true
	}
	if unsupported != nil {
		return unsupported
	}
	if err = a.step(count + 1); err != nil {
		return err
	}
	// The full allowance (actual count, not EOCD count) is live before this call.
	a.reader, err = zip.NewReader(bytes.NewReader(a.data), int64(len(a.data)))
	if err != nil {
		return err
	}
	if len(a.reader.File) != len(a.entries) || a.reader.Comment != end.comment {
		return mxlZIPFormat("archive/zip inventory differential")
	}
	for i, f := range a.reader.File {
		if err = a.step(1 + int64(len(f.Name)+len(f.Extra)+len(f.Comment))); err != nil {
			return err
		}
		e := &a.entries[i]
		offset, err := f.DataOffset()
		if err != nil {
			return err
		}
		if f.Name != e.central.name || f.Flags != e.central.flags || f.Method != e.central.method || string(f.Extra) != e.central.extra || f.Comment != e.central.comment || f.CRC32 != e.central.crc || f.CompressedSize64 != uint64(e.central.compressed) || f.UncompressedSize64 != uint64(e.central.uncompressed) || offset != e.bodyStart {
			return mxlZIPFormat("archive/zip entry differential")
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			return &mxlAssessmentZIPPolicyError{reason: "symbolic-link archive entry"}
		}
	}
	a.ready = true
	return nil
}

func (a *mxlAssessmentArchive) observeLocal(e *mxlAssessmentZIPEntry, centralStart int64) error {
	h, ok := mxlZIPRange(a.data, e.localOffset, 30)
	if !ok || e.localOffset+30 > centralStart || mxlZIPU32(h, 0) != 0x04034b50 {
		return mxlZIPFormat("unproved local header")
	}
	n, x := int64(mxlZIPU16(h, 26)), int64(mxlZIPU16(h, 28))
	if n+x > centralStart-e.localOffset-30 {
		return mxlZIPFormat("local variable fields outside local area")
	}
	if err := a.step(30 + n + x); err != nil {
		return err
	}
	if err := a.account.Check(validation.BudgetScalarBytes, n); err != nil {
		return err
	}
	if err := a.account.Check(validation.BudgetScalarBytes, x); err != nil {
		return err
	}
	if err := a.reserve(n + x); err != nil {
		return err
	}
	v := a.data[int(e.localOffset+30):int(e.localOffset+30+n+x)]
	e.local = mxlZIPLocalHeader(h, v, n)
	e.localObserved = true
	if e.local.name != e.central.name || e.local.method != e.central.method || e.local.flags&0x2049 != e.central.flags&0x2049 {
		return mxlZIPFormat("local/central header disagreement")
	}
	if e.local.compressed == math.MaxUint32 || e.local.uncompressed == math.MaxUint32 {
		return &mxlAssessmentZIPCapabilityError{feature: "ZIP64 local sizes"}
	}
	descriptor := e.central.flags&8 != 0
	if descriptor {
		if e.local.crc != 0 && e.local.crc != e.central.crc || e.local.compressed != 0 && e.local.compressed != e.central.compressed || e.local.uncompressed != 0 && e.local.uncompressed != e.central.uncompressed {
			return mxlZIPFormat("inconsistent descriptor placeholders")
		}
	} else if e.local.crc != e.central.crc || e.local.compressed != e.central.compressed || e.local.uncompressed != e.central.uncompressed {
		return mxlZIPFormat("local/central sizes or CRC disagree")
	}
	e.bodyStart = e.localOffset + 30 + n + x
	if e.central.compressed > centralStart-e.bodyStart {
		return mxlZIPFormat("entry body outside local area")
	}
	e.bodyEnd = e.bodyStart + e.central.compressed
	e.end = e.bodyEnd
	if descriptor {
		d, ok := mxlZIPRange(a.data, e.bodyEnd, 12)
		if !ok || e.bodyEnd+12 > centralStart {
			return mxlZIPFormat("truncated descriptor")
		}
		signed := mxlZIPU32(d, 0) == 0x08074b50
		if signed && e.central.crc == 0x08074b50 && int64(mxlZIPU32(d, 4)) == e.central.compressed && int64(mxlZIPU32(d, 8)) == e.central.uncompressed {
			// A valid unsigned descriptor can have the signature as its CRC. Go's
			// descriptor reader cannot distinguish that case; name the capability
			// rather than pretending the independently valid raw framing is corrupt.
			e.end += 12
			return &mxlAssessmentZIPCapabilityError{feature: "unsigned descriptor CRC collides with signature"}
		}
		if signed {
			d, ok = mxlZIPRange(a.data, e.bodyEnd+4, 12)
			if !ok || e.bodyEnd+16 > centralStart {
				return mxlZIPFormat("truncated signed descriptor")
			}
			e.end += 16
		} else {
			e.end += 12
		}
		if mxlZIPU32(d, 0) != e.central.crc || int64(mxlZIPU32(d, 4)) != e.central.compressed || int64(mxlZIPU32(d, 8)) != e.central.uncompressed {
			return mxlZIPFormat("descriptor/central disagreement")
		}
	}
	e.associated = true
	return nil
}
func (a *mxlAssessmentArchive) checkExtra(extra string) error {
	for p := 0; p+4 <= len(extra); {
		if err := a.step(1); err != nil {
			return err
		}
		tag := uint16(extra[p]) | uint16(extra[p+1])<<8
		n := int(extra[p+2]) | int(extra[p+3])<<8
		if tag == 1 {
			return &mxlAssessmentZIPCapabilityError{feature: "ZIP64 extra field"}
		}
		if n > len(extra)-p-4 {
			return nil
		} // Preserve uninterpreted malformed extras as raw facts.
		p += 4 + n
	}
	return nil
}

// Iterative heap sort needs no extra allocation and polls/charges every
// comparison/swap. Physical position never comes from central-directory order.
func (a *mxlAssessmentArchive) sortPhysical() error {
	sift := func(root, end int) error {
		for root < end/2 {
			child := root*2 + 1
			if err := a.step(1); err != nil {
				return err
			}
			if child+1 < end && a.entries[a.physical[child]].localOffset < a.entries[a.physical[child+1]].localOffset {
				child++
			}
			if a.entries[a.physical[root]].localOffset >= a.entries[a.physical[child]].localOffset {
				return nil
			}
			a.physical[root], a.physical[child] = a.physical[child], a.physical[root]
			root = child
		}
		return nil
	}
	for start := len(a.physical) / 2; start > 0; {
		start--
		if err := sift(start, len(a.physical)); err != nil {
			return err
		}
	}
	for end := len(a.physical) - 1; end > 0; end-- {
		if err := a.step(1); err != nil {
			return err
		}
		a.physical[0], a.physical[end] = a.physical[end], a.physical[0]
		if err := sift(0, end); err != nil {
			return err
		}
	}
	return nil
}

// Inventory arithmetic is checked before either multiplication and host-int
// conversion. It is testable without trying an impossible large allocation.
func mxlZIPInventoryAllowance(count, variable int64) (int64, error) {
	if count < 0 || variable < 0 {
		return 0, &validation.ConfigurationError{Field: "ZIP inventory", Reason: "negative measure"}
	}
	overflow := func() error {
		return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Limit: math.MaxInt64, Requested: math.MaxInt64}
	}
	if variable > (math.MaxInt64-mxlZIPReaderScratch)/4 {
		return 0, overflow()
	}
	allowance := variable*4 + mxlZIPReaderScratch
	if count > (math.MaxInt64-allowance)/mxlZIPEntryAllowance {
		return 0, overflow()
	}
	allowance += count * mxlZIPEntryAllowance
	if _, err := validation.AllocationSize(allowance); err != nil {
		return 0, err
	}
	return allowance, nil
}
