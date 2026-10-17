package validation

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

// Keep the pre-reuse primitive implementation independent of production. Each
// Write snapshot owns its bytes: retaining the caller's slice would violate
// io.Writer's contract and would conceal or invent scratch aliasing failures.
type dictionaryScratchTrace struct {
	hash.Hash
	writes [][]byte
}

func (h *dictionaryScratchTrace) Write(p []byte) (int, error) {
	h.writes = append(h.writes, append([]byte(nil), p...))
	return h.Hash.Write(p)
}

type dictionaryScratchReference struct {
	ctx     context.Context
	account *Account
	hash    hash.Hash
	bytes   int64
	err     error
}

func (w *dictionaryScratchReference) write(p []byte) {
	if w.err != nil {
		return
	}
	if w.err = dictionaryStep(w.ctx, w.account, int64(len(p))+1); w.err != nil {
		return
	}
	_, _ = w.hash.Write(p)
	w.bytes += int64(len(p))
}
func (w *dictionaryScratchReference) count(n uint32) {
	var p [4]byte
	binary.BigEndian.PutUint32(p[:], n)
	w.write(p[:])
}
func (w *dictionaryScratchReference) text(s string) {
	w.count(uint32(len(s)))
	var p [256]byte
	for len(s) > 0 && w.err == nil {
		n := copy(p[:], s)
		w.write(p[:n])
		s = s[n:]
	}
}
func (w *dictionaryScratchReference) row(kind string, fields ...string) {
	w.text(kind)
	w.count(uint32(len(fields)))
	for _, field := range fields {
		w.text(field)
	}
}

type dictionaryScratchPrimitives interface {
	count(uint32)
	text(string)
	row(string, ...string)
}

func dictionaryScratchExercise(w dictionaryScratchPrimitives) {
	w.count(0)
	w.count(0x10203040)
	w.text("")
	w.text(strings.Repeat("x", 255))
	w.text(strings.Repeat("y", 256))
	w.text(strings.Repeat("z", 257))
	w.text(strings.Repeat("a", 255) + "é") // Split UTF-8 between byte chunks.
	w.row("many", "", strings.Repeat("界", 300), "a\x00b")
	w.text("final")
}

type dictionaryScratchResult struct {
	writes               [][]byte
	digest               []byte
	bytes, steps, checks int64
	err                  error
}

func dictionaryScratchRun(t *testing.T, reference bool, limit, cancelAt int64) dictionaryScratchResult {
	t.Helper()
	a, err := NewBudgetAccount(Budgets{MaxRuleSteps: limit})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &dictionaryCancelContext{Context: context.Background(), cancelAt: cancelAt}
	h := &dictionaryScratchTrace{Hash: sha256.New()}
	var out dictionaryScratchResult
	if reference {
		w := dictionaryScratchReference{ctx: ctx, account: a, hash: h}
		dictionaryScratchExercise(&w)
		if w.err != nil {
			w.text("sticky failure must prevent this write")
		}
		out.bytes, out.err = w.bytes, w.err
	} else {
		w := dictionaryEncoder{ctx: ctx, account: a, hash: h}
		dictionaryScratchExercise(&w)
		if w.err != nil {
			w.text("sticky failure must prevent this write")
		}
		out.bytes, out.err = w.bytes, w.err
	}
	out.steps, _ = a.Usage(BudgetRuleSteps)
	out.checks = ctx.checks.Load()
	out.writes, out.digest = h.writes, h.Sum(nil)
	return out
}

func TestDictionaryEncoderScratchTraceAndInterruption(t *testing.T) {
	const neverCancel = int64(1 << 60)
	check := func(limit, cancelAt int64) dictionaryScratchResult {
		t.Helper()
		want := dictionaryScratchRun(t, true, limit, cancelAt)
		got := dictionaryScratchRun(t, false, limit, cancelAt)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("limit=%d cancel=%d: got bytes/work/polls=%d/%d/%d err=%v; want %d/%d/%d err=%v; chunk stream, digest or accounting differs", limit, cancelAt, got.bytes, got.steps, got.checks, got.err, want.bytes, want.steps, want.checks, want.err)
		}
		return want
	}
	full := check(0, neverCancel)
	if full.err != nil {
		t.Fatal(full.err)
	}
	for at := int64(1); at <= full.checks; at++ {
		check(0, at)
	}
	var work int64
	for _, p := range full.writes {
		work += int64(len(p)) + 1
		check(work-1, neverCancel)
		check(work, neverCancel)
	}
}

func TestDictionaryEncoderScratchAllocationBound(t *testing.T) {
	a, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(10, func() {
		w := dictionaryEncoder{ctx: context.Background(), account: a, hash: sha256.New()}
		for i := 0; i < 32; i++ {
			w.count(uint32(i))
			w.text("independent owned scratch")
		}
		if w.err != nil {
			panic(w.err)
		}
	})
	// One encoder and one hash may escape. Leave portable fixed headroom, but
	// never permit a new 4/256-byte allocation for every primitive invocation.
	if allocations > 4 {
		t.Fatalf("repeated count/text allocated %g times; want <=4", allocations)
	}
}

func TestDictionaryEncoderScratchFitsExistingAdmission(t *testing.T) {
	// The unchanged 1024-byte sub-envelope within dictionarySetFixedBytes owns
	// the encoder, sequential SHA state/sum and bounded codepoint/hex scratch.
	// Size the active standard SHA implementation without assuming its layout.
	hashBytes := reflect.TypeOf(sha256.New()).Elem().Size()
	used := unsafe.Sizeof(dictionaryEncoder{}) + hashBytes + 2*sha256.Size + 128
	if used > 1024 || dictionarySetFixedBytes != 2048 {
		t.Fatalf("encoder/SHA/sum/temporary bound=%d, fixed admission=%d", used, dictionarySetFixedBytes)
	}
	t.Logf("encoder=%d SHA=%d measured fixed components=%d <=1024", unsafe.Sizeof(dictionaryEncoder{}), hashBytes, used)
}

func TestDictionaryEncoderScratchPinnedAndIndependentConcurrentAccounts(t *testing.T) {
	ids := []DictionaryManifestID{DictionaryManifestSMuFL14Canonical, DictionaryManifestSMuFL14Ranges, DictionaryManifestStandardSounds40, DictionaryManifestIPAPositive2015Unicode13, DictionaryManifestIPARepresentation2015Unicode13}
	for _, id := range ids {
		t.Run(string(id), func(t *testing.T) {
			var selection DictionarySelection
			var input DictionaryImport
			if id == DictionaryManifestIPARepresentation2015Unicode13 {
				selection, input, _ = ipaRepresentationFixture(t)
			} else {
				selection, input, _ = dictionaryFixture(t, id)
			}
			a := dictionaryCoreAccount(t)
			set, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selection}, []DictionaryImport{input})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := set.Close(); err != nil {
					t.Error(err)
				}
				if dictionaryCoreUsage(t, a) != 17 {
					t.Error("owned dictionary admission leaked")
				}
			}()
			entry := &set.entries[0]
			var wg sync.WaitGroup
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					account, err := NewBudgetAccount(Budgets{})
					if err != nil {
						t.Error(err)
						return
					}
					if err := account.Charge(BudgetBufferedBytes, dictionarySetFixedBytes); err != nil {
						t.Error(err)
						return
					}
					defer account.Release(BudgetBufferedBytes, dictionarySetFixedBytes)
					digest, count, err := dictionaryDataDigest(context.Background(), account, entry)
					if err != nil || count != entry.manifest.DataBytes || hex.EncodeToString(digest[:]) != entry.manifest.DataSHA256 {
						t.Errorf("independent digest=%x bytes=%d err=%v", digest, count, err)
					}
					fingerprint, err := dictionaryFingerprint(context.Background(), account, set.entries)
					if err != nil || fingerprint != set.fingerprint {
						t.Errorf("fingerprint=%x err=%v", fingerprint, err)
					}
					canceled, cancel := context.WithCancel(context.Background())
					cancel()
					fingerprint, err = dictionaryFingerprint(canceled, account, set.entries)
					if err != context.Canceled || fingerprint != ([32]byte{}) {
						t.Errorf("canceled fingerprint=%x err=%v", fingerprint, err)
					}
				}()
			}
			wg.Wait()
		})
	}
}
