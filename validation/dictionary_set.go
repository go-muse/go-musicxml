package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// SMuFLGlyph records the primary codepoint of one exact canonical name.
type SMuFLGlyph struct {
	Name      string
	Codepoint uint32
}

// SMuFLRange records an inclusive source range and its exact enumerated names.
type SMuFLRange struct {
	ID         string
	Start, End uint32
	Names      []string
}

// SMuFLDictionaryData holds one manifest's canonical or range records. Only the
// branch corresponding to the selected manifest may be populated.
type SMuFLDictionaryData struct {
	Glyphs []SMuFLGlyph
	Ranges []SMuFLRange
}

// StandardSoundsDictionaryData contains the exact standard MusicXML identifiers.
// Nonmembership does not imply MusicXML invalidity.
type StandardSoundsDictionaryData struct{ IDs []string }

// IPASymbolEncoding is one reviewed positive atomic-encoding record.
type IPASymbolEncoding struct{ Encoding, Category, RowID string }

// IPAAttestedExample is a literal reviewed example, not a general sequence rule.
type IPAAttestedExample struct{ Encoding, Category, RowID string }

// IPACanonicalEquivalent records an explicitly reviewed canonical equivalence.
// It never authorizes compatibility normalization or arbitrary decomposition.
type IPACanonicalEquivalent struct{ Encoding, EquivalentToEncoding, RowID string }

// IPAPositiveDictionaryData is an explicitly partial positive-evidence table.
// Neither absent rows nor full atomic coverage prove sequence validity.
type IPAPositiveDictionaryData struct {
	Symbols      []IPASymbolEncoding
	Examples     []IPAAttestedExample
	Equivalences []IPACanonicalEquivalent
}

// DictionaryImport supplies typed records or a capability gap. Known imports
// must match the compiled transformed digest independently of any certificate.
// Without Certificate, successful data establishes reviewed derived lineage only.
// The caller must not mutate any supplied data while construction is in progress.
type DictionaryImport struct {
	Manifest    DictionaryManifestID
	State       EvidenceState
	Reason      Code
	Certificate *DictionaryComponentCertificate
	SMuFL       *SMuFLDictionaryData
	Sounds      *StandardSoundsDictionaryData
	IPA         *IPAPositiveDictionaryData
}

type dictionaryEntry struct {
	manifest    DictionaryManifest
	relations   []DictionaryRelation
	state       EvidenceState
	reason      Code
	certificate bool
	smufl       *SMuFLDictionaryData
	sounds      *StandardSoundsDictionaryData
	ipa         *IPAPositiveDictionaryData
}

// DictionarySet owns immutable verified dictionary records admitted to one
// Account. Do not copy a set after first use. Independent sessions may borrow it
// concurrently. Close blocks new borrows immediately; admitted sessions retain
// their data until finalization. The final owner/borrow release frees storage
// exactly once. Reports never retain dictionary borrows.
type DictionarySet struct {
	mu          sync.Mutex
	initialized bool
	closed      bool
	borrowers   int64
	account     *Account
	retained    int64
	entries     []dictionaryEntry
	fingerprint [32]byte
}

type dictionaryLease struct {
	set      *DictionarySet
	retained int64
}

func dictionaryConfiguration(reason string) error { return contract("dictionary", reason) }

// The base reservation applies even to an empty set: 512 bytes for the set and
// constructor bookkeeping, 1024 for reusable sequential SHA state/sum copies,
// the encoder, its 256-byte text buffer and bounded hex/codepoint temporaries,
// and 512 bytes of conservative fixed-work headroom. Hashing never retains a
// full serialized corpus. Per-entry admission separately covers the finite
// entry/manifest descriptors, owned relation slices, reason and branch headers;
// variable record arrays and strings have their own measured reservations.
const (
	dictionarySetFixedBytes   int64 = 2048
	dictionaryEntryFixedBytes int64 = 2048
)

func (d *DictionarySet) retain(n int64) error {
	total, err := rationalCheckedAdd(BudgetBufferedBytes, d.retained, n)
	if err != nil {
		return err
	}
	if _, err = AllocationSize(n); err != nil {
		return err
	}
	if err = d.account.Charge(BudgetBufferedBytes, n); err != nil {
		return err
	}
	d.retained = total
	return nil
}

// NewDictionarySet validates bounded typed input, owns independent immutable
// copies, and verifies each known relation against its fixed reviewed manifest.
// Missing imports remain explicit unavailable capabilities. Failure releases
// only storage acquired here and never refunds work or resets the caller Account.
func NewDictionarySet(ctx context.Context, a *Account, selections []DictionarySelection, imports []DictionaryImport) (_ *DictionarySet, err error) {
	if err = dictionaryStep(ctx, a, 1); err != nil {
		return nil, err
	}
	if len(selections) > 4 || len(imports) > 4 {
		return nil, dictionaryConfiguration("too many selections or imports")
	}
	retained := dictionarySetFixedBytes + dictionaryEntryFixedBytes*int64(len(selections))
	if err = a.Charge(BudgetBufferedBytes, retained); err != nil {
		return nil, err
	}
	d := &DictionarySet{initialized: true, account: a, retained: retained}
	defer func() {
		if err != nil {
			_ = a.Release(BudgetBufferedBytes, d.retained)
			d.retained = 0
			d.entries = nil
		}
	}()
	d.entries = make([]dictionaryEntry, len(selections))
	for i, selection := range selections {
		if err = dictionaryStep(ctx, a, 1024); err != nil {
			return nil, err
		}
		manifest, ok := dictionaryManifest(selection.Manifest)
		if !ok {
			return nil, dictionaryConfiguration("unsupported manifest")
		}
		if len(selection.Relations) == 0 || len(selection.Relations) > len(manifest.Relations) {
			return nil, dictionaryConfiguration("invalid relation count")
		}
		for j := 0; j < i; j++ {
			if d.entries[j].manifest.ID == selection.Manifest {
				return nil, dictionaryConfiguration("duplicate manifest selection")
			}
		}
		entry := &d.entries[i]
		*entry = dictionaryEntry{manifest: manifest, state: EvidenceUnknown, reason: "dictionary-not-supplied", relations: make([]DictionaryRelation, len(selection.Relations))}
		for j, relation := range selection.Relations {
			if err = dictionaryStep(ctx, a, 1); err != nil {
				return nil, err
			}
			if !slices.Contains(manifest.Relations, relation) {
				return nil, dictionaryConfiguration("unsupported manifest relation")
			}
			for k := 0; k < j; k++ {
				if entry.relations[k] == relation {
					return nil, dictionaryConfiguration("duplicate relation selection")
				}
			}
			entry.relations[j] = relation
		}
		slices.Sort(entry.relations)
	}
	// A four-entry bounded insertion sort needs no dynamic comparison workspace.
	for i := 1; i < len(d.entries); i++ {
		for j := i; j > 0 && d.entries[j].manifest.ID < d.entries[j-1].manifest.ID; j-- {
			d.entries[j], d.entries[j-1] = d.entries[j-1], d.entries[j]
		}
	}
	var imported [4]bool
	for _, input := range imports {
		if err = dictionaryStep(ctx, a, 1); err != nil {
			return nil, err
		}
		at := -1
		for i := range d.entries {
			if d.entries[i].manifest.ID == input.Manifest {
				at = i
				break
			}
		}
		if at < 0 {
			return nil, dictionaryConfiguration("unselected import")
		}
		if imported[at] {
			return nil, dictionaryConfiguration("duplicate import")
		}
		imported[at] = true
		entry := &d.entries[at]
		if err = d.importEntry(ctx, entry, input); err != nil {
			return nil, err
		}
	}
	if err = dictionaryStep(ctx, a, 1); err != nil {
		return nil, err
	}
	d.fingerprint, err = dictionaryFingerprint(ctx, a, d.entries)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DictionarySet) importEntry(ctx context.Context, entry *dictionaryEntry, input DictionaryImport) error {
	branches := 0
	if input.SMuFL != nil {
		branches++
	}
	if input.Sounds != nil {
		branches++
	}
	if input.IPA != nil {
		branches++
	}
	switch input.State {
	case EvidenceUnknown, EvidenceUnsupported:
		if branches != 0 || input.Certificate != nil {
			return dictionaryConfiguration("unavailable import contains data or certificate")
		}
		if err := dictionaryString(ctx, d.account, string(input.Reason), 128); err != nil {
			return err
		}
		if !validID(string(input.Reason)) {
			return dictionaryConfiguration("unavailable import requires a bounded reason")
		}
		if err := dictionaryStep(ctx, d.account, int64(len(input.Reason))+1); err != nil {
			return err
		}
		entry.state = input.State
		entry.reason = Code(dictionaryCopyString(string(input.Reason)))
		return nil
	case EvidenceKnown:
		if branches != 1 || input.Reason != "" {
			return dictionaryConfiguration("known import requires exactly one payload and no reason")
		}
	default:
		return dictionaryConfiguration("invalid import evidence state")
	}
	m := entry.manifest
	if c := input.Certificate; c != nil {
		if !c.verified || c.account != d.account || c.component != m.Component.ID || hex.EncodeToString(c.digest[:]) != m.Component.SHA256 {
			return dictionaryConfiguration("invalid or foreign component certificate")
		}
		entry.certificate = true
	}
	switch m.ID {
	case DictionaryManifestSMuFL14Canonical, DictionaryManifestSMuFL14Ranges:
		if input.SMuFL == nil {
			return dictionaryConfiguration("wrong SMuFL payload branch")
		}
		if err := d.copySMuFL(ctx, entry, input.SMuFL); err != nil {
			return err
		}
	case DictionaryManifestStandardSounds40:
		if input.Sounds == nil {
			return dictionaryConfiguration("wrong sounds payload branch")
		}
		if err := d.copySounds(ctx, entry, input.Sounds); err != nil {
			return err
		}
	case DictionaryManifestIPAPositive2015Unicode13:
		if input.IPA == nil {
			return dictionaryConfiguration("wrong IPA payload branch")
		}
		if err := d.copyIPA(ctx, entry, input.IPA); err != nil {
			return err
		}
	}
	digest, bytes, err := dictionaryDataDigest(ctx, d.account, entry)
	if err != nil {
		return err
	}
	if bytes != m.DataBytes || hex.EncodeToString(digest[:]) != m.DataSHA256 {
		return dictionaryConfiguration("transformed data identity mismatch")
	}
	entry.state = EvidenceKnown
	entry.reason = ""
	return nil
}

func dictionaryString(ctx context.Context, a *Account, s string, maximum int) error {
	if len(s) == 0 || len(s) > maximum {
		return dictionaryConfiguration("empty or excessive record string")
	}
	if err := a.Check(BudgetScalarBytes, int64(len(s))); err != nil {
		return err
	}
	if err := dictionaryStep(ctx, a, int64(len(s))+1); err != nil {
		return err
	}
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return dictionaryConfiguration("invalid record string")
	}
	return nil
}

func dictionaryCodepoint(cp uint32) bool { return cp <= 0x10ffff && !(cp >= 0xd800 && cp <= 0xdfff) }

func (d *DictionarySet) copySMuFL(ctx context.Context, e *dictionaryEntry, in *SMuFLDictionaryData) error {
	size := int64(256)
	if e.manifest.ID == DictionaryManifestSMuFL14Canonical {
		if len(in.Glyphs) != 2932 || len(in.Ranges) != 0 {
			return dictionaryConfiguration("incorrect canonical record count or branch")
		}
		for _, g := range in.Glyphs {
			if err := dictionaryString(ctx, d.account, g.Name, 44); err != nil {
				return err
			}
			if !dictionaryCodepoint(g.Codepoint) {
				return dictionaryConfiguration("invalid glyph codepoint")
			}
			size += 64 + int64(len(g.Name))
		}
		if err := d.retain(size); err != nil {
			return err
		}
		e.smufl = &SMuFLDictionaryData{Glyphs: make([]SMuFLGlyph, len(in.Glyphs))}
		for i, g := range in.Glyphs {
			if err := dictionaryStep(ctx, d.account, int64(len(g.Name))+1); err != nil {
				return err
			}
			e.smufl.Glyphs[i] = SMuFLGlyph{dictionaryCopyString(g.Name), g.Codepoint}
		}
		return dictionarySort(ctx, d.account, e.smufl.Glyphs, func(g SMuFLGlyph) string { return g.Name })
	}
	if len(in.Ranges) != 131 || len(in.Glyphs) != 0 {
		return dictionaryConfiguration("incorrect range record count or branch")
	}
	members := 0
	for _, r := range in.Ranges {
		if err := dictionaryString(ctx, d.account, r.ID, 63); err != nil {
			return err
		}
		if !dictionaryCodepoint(r.Start) || !dictionaryCodepoint(r.End) || r.Start > r.End {
			return dictionaryConfiguration("invalid range bounds")
		}
		if len(r.Names) == 0 || len(r.Names) > 128 {
			return dictionaryConfiguration("invalid range member count")
		}
		members += len(r.Names)
		if members > 2932 {
			return dictionaryConfiguration("too many range members")
		}
		size += 128 + int64(len(r.ID))
		for _, name := range r.Names {
			if err := dictionaryString(ctx, d.account, name, 44); err != nil {
				return err
			}
			size += 32 + int64(len(name))
		}
	}
	if members != 2932 {
		return dictionaryConfiguration("incorrect range member count")
	}
	if err := d.retain(size); err != nil {
		return err
	}
	e.smufl = &SMuFLDictionaryData{Ranges: make([]SMuFLRange, len(in.Ranges))}
	for i, r := range in.Ranges {
		if err := dictionaryStep(ctx, d.account, int64(len(r.ID))+1); err != nil {
			return err
		}
		out := SMuFLRange{ID: dictionaryCopyString(r.ID), Start: r.Start, End: r.End, Names: make([]string, len(r.Names))}
		for j, name := range r.Names {
			if err := dictionaryStep(ctx, d.account, int64(len(name))+1); err != nil {
				return err
			}
			out.Names[j] = dictionaryCopyString(name)
		}
		if err := dictionarySort(ctx, d.account, out.Names, func(s string) string { return s }); err != nil {
			return err
		}
		e.smufl.Ranges[i] = out
	}
	return dictionarySort(ctx, d.account, e.smufl.Ranges, func(r SMuFLRange) string { return r.ID })
}

func (d *DictionarySet) copySounds(ctx context.Context, e *dictionaryEntry, in *StandardSoundsDictionaryData) error {
	if len(in.IDs) != 894 {
		return dictionaryConfiguration("incorrect standard sound count")
	}
	size := int64(256)
	for _, id := range in.IDs {
		if err := dictionaryString(ctx, d.account, id, 39); err != nil {
			return err
		}
		size += 32 + int64(len(id))
	}
	if err := d.retain(size); err != nil {
		return err
	}
	e.sounds = &StandardSoundsDictionaryData{IDs: make([]string, len(in.IDs))}
	for i, id := range in.IDs {
		if err := dictionaryStep(ctx, d.account, int64(len(id))+1); err != nil {
			return err
		}
		e.sounds.IDs[i] = dictionaryCopyString(id)
	}
	return dictionarySort(ctx, d.account, e.sounds.IDs, func(s string) string { return s })
}

func (d *DictionarySet) copyIPA(ctx context.Context, e *dictionaryEntry, in *IPAPositiveDictionaryData) error {
	if len(in.Symbols) != 170 || len(in.Examples) != 18 || len(in.Equivalences) != 9 {
		return dictionaryConfiguration("incorrect IPA positive record counts")
	}
	size := int64(256)
	inspect := func(a, b, c string) error {
		for _, v := range []struct {
			s   string
			max int
		}{{a, 16}, {b, 64}, {c, 64}} {
			if err := dictionaryString(ctx, d.account, v.s, v.max); err != nil {
				return err
			}
			size += int64(len(v.s))
		}
		size += 128
		return nil
	}
	for _, r := range in.Symbols {
		if err := inspect(r.Encoding, r.Category, r.RowID); err != nil {
			return err
		}
	}
	for _, r := range in.Examples {
		if err := inspect(r.Encoding, r.Category, r.RowID); err != nil {
			return err
		}
	}
	for _, r := range in.Equivalences {
		if err := inspect(r.Encoding, r.EquivalentToEncoding, r.RowID); err != nil {
			return err
		}
	}
	if err := d.retain(size); err != nil {
		return err
	}
	e.ipa = &IPAPositiveDictionaryData{Symbols: make([]IPASymbolEncoding, len(in.Symbols)), Examples: make([]IPAAttestedExample, len(in.Examples)), Equivalences: make([]IPACanonicalEquivalent, len(in.Equivalences))}
	copyFields := func(a, b, c string) (string, string, string, error) {
		if err := dictionaryStep(ctx, d.account, int64(len(a)+len(b)+len(c))+1); err != nil {
			return "", "", "", err
		}
		return dictionaryCopyString(a), dictionaryCopyString(b), dictionaryCopyString(c), nil
	}
	for i, r := range in.Symbols {
		a, b, c, err := copyFields(r.Encoding, r.Category, r.RowID)
		if err != nil {
			return err
		}
		e.ipa.Symbols[i] = IPASymbolEncoding{a, b, c}
	}
	for i, r := range in.Examples {
		a, b, c, err := copyFields(r.Encoding, r.Category, r.RowID)
		if err != nil {
			return err
		}
		e.ipa.Examples[i] = IPAAttestedExample{a, b, c}
	}
	for i, r := range in.Equivalences {
		a, b, c, err := copyFields(r.Encoding, r.EquivalentToEncoding, r.RowID)
		if err != nil {
			return err
		}
		e.ipa.Equivalences[i] = IPACanonicalEquivalent{a, b, c}
	}
	if err := dictionarySort(ctx, d.account, e.ipa.Symbols, func(r IPASymbolEncoding) string { return r.Encoding }); err != nil {
		return err
	}
	if err := dictionarySort(ctx, d.account, e.ipa.Examples, func(r IPAAttestedExample) string { return r.Encoding }); err != nil {
		return err
	}
	return dictionarySort(ctx, d.account, e.ipa.Equivalences, func(r IPACanonicalEquivalent) string { return r.Encoding })
}

func dictionarySort[T any](ctx context.Context, a *Account, rows []T, key func(T) string) error {
	var first error
	compare := func(x, y T) int {
		if first != nil {
			return 0
		}
		left, right := key(x), key(y)
		if first = dictionaryComparisonStep(ctx, a, int64(len(left)), int64(len(right))); first != nil {
			return 0
		}
		return strings.Compare(left, right)
	}
	slices.SortFunc(rows, compare)
	if first != nil {
		return first
	}
	for i := 1; i < len(rows); i++ {
		if compare(rows[i-1], rows[i]) == 0 {
			if first != nil {
				return first
			}
			return dictionaryConfiguration("duplicate record key")
		}
	}
	return first
}

// Keep lengths wide before adding: scalar limits may exceed the host int range
// on another supported architecture. The trailing unit admits the comparison.
func dictionaryComparisonWork(left, right int64) (int64, error) {
	return rationalSum(BudgetRuleSteps, left, right, 1)
}

func dictionaryComparisonStep(ctx context.Context, a *Account, left, right int64) error {
	if ctx == nil {
		return contract("dictionary context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	work, err := dictionaryComparisonWork(left, right)
	if err != nil {
		return err
	}
	return dictionaryStep(ctx, a, work)
}

func dictionarySearch(ctx context.Context, a *Account, count int, key string, keyAt func(int) string) (int, bool, error) {
	if err := a.Check(BudgetScalarBytes, int64(len(key))); err != nil {
		return 0, false, err
	}
	if err := dictionaryComparisonStep(ctx, a, int64(len(key)), 0); err != nil {
		return 0, false, err
	}
	lo, hi := 0, count
	for lo < hi {
		mid := lo + (hi-lo)/2
		candidate := keyAt(mid)
		if err := dictionaryComparisonStep(ctx, a, int64(len(key)), int64(len(candidate))); err != nil {
			return 0, false, err
		}
		compared := strings.Compare(candidate, key)
		if compared < 0 {
			lo = mid + 1
		} else if compared > 0 {
			hi = mid
		} else {
			return mid, true, nil
		}
	}
	return lo, false, nil
}

func (d *DictionarySet) borrow(a *Account) (*dictionaryLease, error) {
	if d == nil {
		return nil, dictionaryConfiguration("nil set")
	}
	// Admit the lease before allocation, without holding either lifecycle or
	// account mutex while acquiring the other. A rejected borrow refunds only it.
	const leaseBytes = 64
	if err := a.Charge(BudgetBufferedBytes, leaseBytes); err != nil {
		return nil, err
	}
	d.mu.Lock()
	var err error
	switch {
	case !d.initialized || d.closed:
		err = dictionaryConfiguration("uninitialized or closed set")
	case d.account != a:
		err = dictionaryConfiguration("set requires identical account")
	case d.borrowers == int64(^uint64(0)>>1):
		err = dictionaryConfiguration("too many dictionary borrowers")
	}
	if err != nil {
		d.mu.Unlock()
		_ = a.Release(BudgetBufferedBytes, leaseBytes)
		return nil, err
	}
	d.borrowers++
	d.mu.Unlock()
	return &dictionaryLease{set: d, retained: leaseBytes}, nil
}

func (l *dictionaryLease) release() {
	if l == nil || l.set == nil {
		return
	}
	d := l.set
	l.set = nil
	leaseBytes := l.retained
	l.retained = 0
	d.mu.Lock()
	d.borrowers--
	var retained int64
	if d.closed && d.borrowers == 0 {
		retained = d.retained
		d.retained = 0
		d.entries = nil
	}
	d.mu.Unlock()
	if retained != 0 {
		_ = d.account.Release(BudgetBufferedBytes, retained)
	}
	if leaseBytes != 0 {
		_ = d.account.Release(BudgetBufferedBytes, leaseBytes)
	}
}

// Close is concurrent-safe and idempotent on an initialized set. It rejects
// an uninitialized set and defers retained-storage release while sessions borrow.
func (d *DictionarySet) Close() error {
	if d == nil {
		return dictionaryConfiguration("nil set")
	}
	d.mu.Lock()
	if !d.initialized {
		d.mu.Unlock()
		return dictionaryConfiguration("uninitialized set")
	}
	d.closed = true
	var retained int64
	if d.borrowers == 0 {
		retained = d.retained
		d.retained = 0
		d.entries = nil
	}
	d.mu.Unlock()
	if retained != 0 {
		return d.account.Release(BudgetBufferedBytes, retained)
	}
	return nil
}

// Fingerprint returns the immutable capability identity, including selected
// relations, actual availability, fixed data identity and verification lineage.
// Uninitialized or closed owners reject access; admitted sessions remain valid.
func (d *DictionarySet) Fingerprint() ([32]byte, error) {
	if d == nil {
		return [32]byte{}, dictionaryConfiguration("nil set")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.initialized || d.closed {
		return [32]byte{}, dictionaryConfiguration("uninitialized or closed set")
	}
	return d.fingerprint, nil
}

// BeginWithDictionaries borrows immutable same-account data without changing
// RunOptions or ordinary Begin behavior. A nil set delegates to Begin. A nonnil
// set requires the exact root Account and zero Budgets. Failed initialization
// releases the borrow; successful sessions release it through either finalizer.
func (e *Engine) BeginWithDictionaries(ctx context.Context, opts RunOptions, set *DictionarySet) (*Session, error) {
	if set == nil {
		return e.Begin(ctx, opts)
	}
	// Preserve ordinary Begin's context precedence before a lease can consume
	// quota. Other initialization remains in Begin, including its second check.
	if ctx == nil {
		return nil, contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, &IncompleteError{Reason: "canceled-before-begin", Cause: err}
	}
	if opts.Budgets != (Budgets{}) {
		return nil, dictionaryConfiguration("nonzero budgets cannot accompany dictionaries")
	}
	lease, err := set.borrow(opts.Account)
	if err != nil {
		return nil, err
	}
	s, err := e.Begin(ctx, opts)
	if err != nil {
		lease.release()
		return nil, err
	}
	s.dictionaries = lease
	return s, nil
}

func (s *Session) dictionaryCapability(ctx context.Context, id DictionaryManifestID, relation DictionaryRelation) (*dictionaryEntry, Evidence[bool], error) {
	gap := Evidence[bool]{State: EvidenceUnknown, Reason: "dictionary-unavailable"}
	if err := dictionaryStep(ctx, s.account, 1); err != nil {
		return nil, gap, err
	}
	if s.dictionaries == nil || s.dictionaries.set == nil {
		return nil, gap, nil
	}
	for i := range s.dictionaries.set.entries {
		entry := &s.dictionaries.set.entries[i]
		if entry.manifest.ID != id {
			continue
		}
		if !slices.Contains(entry.relations, relation) {
			gap.Reason = "dictionary-relation-not-selected"
			return nil, gap, nil
		}
		if entry.state != EvidenceKnown {
			return entry, Evidence[bool]{State: entry.state, Reason: entry.reason}, nil
		}
		return entry, Evidence[bool]{State: EvidenceKnown, Value: true}, nil
	}
	return nil, gap, nil
}

func dictionaryFingerprint(ctx context.Context, a *Account, entries []dictionaryEntry) ([32]byte, error) {
	h := sha256.New()
	w := dictionaryEncoder{ctx: ctx, account: a, hash: h}
	w.text("go-musicxml.dictionary-capability/v1")
	w.count(uint32(len(entries)))
	for _, e := range entries {
		for _, s := range []string{string(e.manifest.ID), e.manifest.Edition, e.manifest.TransformRevision, e.manifest.PolicyRevision, e.manifest.DataSHA256, string(e.state), string(e.reason), string(e.manifest.Component.SourceKind)} {
			w.text(s)
		}
		if e.certificate {
			w.text(e.manifest.Component.SHA256)
		} else {
			w.text("")
		}
		w.count(uint32(len(e.relations)))
		for _, r := range e.relations {
			w.text(string(r))
		}
	}
	var digest [32]byte
	if w.err != nil {
		return digest, w.err
	}
	h.Sum(digest[:0])
	return digest, ctx.Err()
}

func (s *Session) dictionaryFingerprint() [32]byte {
	if s == nil || s.dictionaries == nil || s.dictionaries.set == nil {
		return [32]byte{}
	}
	return s.dictionaries.set.fingerprint
}
