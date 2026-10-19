package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// bundledPassThroughSource opens the bundled components through a caller
// source, which the cache does not recognize, and counts the calls.
type bundledPassThroughSource struct {
	mu    sync.Mutex
	calls int
	open  func(context.Context, DictionaryComponentRequest)
}

func (s *bundledPassThroughSource) Open(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.open != nil {
		s.open(ctx, request)
	}
	return BundledDictionarySource().Open(ctx, request)
}

func (s *bundledPassThroughSource) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func dictionaryLoadUsage(t *testing.T, o DictionaryOptions) (set *validation.DictionarySet, account *validation.Account, steps, buffered, external int64) {
	t.Helper()
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	set, err = loadValidatedDictionarySet(context.Background(), account, o)
	if err != nil || set == nil {
		t.Fatalf("load: set=%v error=%v", set != nil, err)
	}
	usage := func(kind validation.BudgetKind) int64 {
		used, err := account.Usage(kind)
		if err != nil {
			t.Fatal(err)
		}
		return used
	}
	return set, account, usage(validation.BudgetRuleSteps), usage(validation.BudgetBufferedBytes), usage(validation.BudgetExternalBytes)
}

// The required bundled selections are imported once per process: every load
// reads the same verified data, charges the caller its retained storage but
// not the import work, and returns the charge on Close.
func TestBundledDictionaryCacheSharesOneVerifiedSet(t *testing.T) {
	fresh, freshAccount, freshSteps, freshBuffered, freshExternal := dictionaryLoadUsage(t, DictionaryOptions{Source: &bundledPassThroughSource{}, Selections: requiredDictionarySelections()})
	defer fresh.Close()
	want, err := fresh.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range []DictionaryOptions{requiredDictionaryOptions(), {Source: BundledDictionarySource(), Selections: mandatoryDictionarySelections()}} {
		set, account, steps, buffered, external := dictionaryLoadUsage(t, options)
		got, err := set.Fingerprint()
		if err != nil || got != want {
			t.Fatalf("cached set differs from a fresh import: %x want %x error=%v", got, want, err)
		}
		if steps >= 100 || freshSteps < 1_000_000 || external != 0 || freshExternal == 0 || buffered != freshBuffered {
			t.Fatalf("cached load charged steps=%d buffered=%d external=%d; fresh import steps=%d buffered=%d external=%d", steps, buffered, external, freshSteps, freshBuffered, freshExternal)
		}
		if err := set.Close(); err != nil {
			t.Fatal(err)
		}
		if used, err := account.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
			t.Fatalf("closed view kept %d bytes: %v", used, err)
		}
	}
	if err := fresh.Close(); err != nil {
		t.Fatal(err)
	}
	if used, err := freshAccount.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
		t.Fatalf("closed fresh set kept %d bytes: %v", used, err)
	}
}

// Only the bundled source with exactly the required selections is cached: a
// caller source and other bundled selections still import on the caller's
// account, and a caller source is always opened.
func TestBundledDictionaryCacheIgnoresOtherOptions(t *testing.T) {
	soundsToo := append(requiredDictionarySelections(), validation.DictionarySelection{Manifest: validation.DictionaryManifestStandardSounds40, Relations: []validation.DictionaryRelation{validation.DictionaryRelationStandardSound}})
	reversed := requiredDictionarySelections()
	reversed[0], reversed[1] = reversed[1], reversed[0]
	source := &bundledPassThroughSource{}
	for name, options := range map[string]DictionaryOptions{
		"caller source":    {Source: source, Selections: requiredDictionarySelections()},
		"more selections":  {Source: BundledDictionarySource(), Selections: soundsToo},
		"selection order":  {Source: BundledDictionarySource(), Selections: reversed},
		"one selection":    {Source: BundledDictionarySource(), Selections: requiredDictionarySelections()[:1]},
		"typed-nil source": {Source: (*dictionaryImportNilSource)(nil), Selections: requiredDictionarySelections()},
	} {
		if bundledRequiredDictionaries(options) {
			t.Errorf("%s: recognized as the cached selection", name)
		}
	}
	set, _, steps, _, _ := dictionaryLoadUsage(t, DictionaryOptions{Source: source, Selections: requiredDictionarySelections()})
	defer set.Close()
	if source.count() != 2 || steps < 1_000_000 {
		t.Fatalf("caller source bypassed: calls=%d steps=%d", source.count(), steps)
	}
	set2, _, steps, _, _ := dictionaryLoadUsage(t, DictionaryOptions{Source: BundledDictionarySource(), Selections: soundsToo})
	defer set2.Close()
	if steps < 1_000_000 {
		t.Fatalf("other bundled selections served from the cache: steps=%d", steps)
	}
}

// A view's retained storage is admitted to the caller's account: a buffer too
// small for it fails as a resource limit and leaves nothing charged.
func TestBundledDictionaryCacheChargesTheCallerBuffer(t *testing.T) {
	_, _, _, buffered, _ := dictionaryLoadUsage(t, requiredDictionaryOptions())
	account, err := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: buffered - 1})
	if err != nil {
		t.Fatal(err)
	}
	set, err := loadValidatedDictionarySet(context.Background(), account, requiredDictionaryOptions())
	if set != nil || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("view admitted beyond the buffer: set=%v error=%v", set != nil, err)
	}
	if used, err := account.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
		t.Fatalf("refused view kept %d bytes: %v", used, err)
	}
}

// Concurrent loaders share the cached set and each accepts its document.
func TestBundledDictionaryCacheConcurrentLoaders(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := DecodeAssessed(context.Background(), strings.NewReader(mandatoryDictionaryScore()), DecodeAssessmentOptions{})
			if err == nil && !result.Accepted {
				err = errors.New("not accepted")
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// The cached set gives byte-identical reports: strict and lenient loading of
// corpus files match a load whose caller source imports the same bundled
// components afresh.
func TestBundledDictionaryCacheReportsMatchAFreshImport(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "corpus", "musicxml-test-suite", "*.xml"))
	if err != nil || len(names) < 100 {
		t.Fatalf("corpus: %d files, %v", len(names), err)
	}
	stride := 1
	if raceEnabled || testing.Short() {
		stride = 40
	}
	compared := 0
	for i := 0; i < len(names); i += stride {
		data, err := os.ReadFile(names[i])
		if err != nil {
			t.Fatal(err)
		}
		for _, lenient := range []bool{false, true} {
			cached, cachedErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: lenient})
			fresh, freshErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: lenient, Assessment: &AssessmentOptions{Dictionaries: &DictionaryOptions{Source: &bundledPassThroughSource{}, Selections: requiredDictionarySelections()}}})
			if (cachedErr == nil) != (freshErr == nil) || cached.Accepted != fresh.Accepted || !reflect.DeepEqual(cached.SourceLine, fresh.SourceLine) || !reflect.DeepEqual(cached.ModelLine, fresh.ModelLine) || !reflect.DeepEqual(cached.Conversion, fresh.Conversion) {
				t.Errorf("%s lenient=%v: cached and fresh loads differ: errors %v / %v", filepath.Base(names[i]), lenient, cachedErr, freshErr)
			}
			compared++
		}
	}
	t.Logf("compared %d loads", compared)
}

// Line 3 alone reads the dictionaries, so strict loading opens a caller source
// only after line 2 accepts the source; the lenient mode opens it after any
// line 2 without an operational failure, and no mode opens it after one.
func TestStrictLoadingImportsDictionariesAfterLineTwo(t *testing.T) {
	invalid := mandatoryScore("1", `<note><pitch><step>H</step><octave>4</octave></pitch><duration>1</duration></note>`)
	malformed := mandatoryScore("1", `<note>`)
	for _, tc := range []struct {
		name, input string
		lenient     bool
		calls       int
		model       bool
	}{
		{"strict accepted", mandatoryDictionaryScore(), false, 2, true},
		{"strict rejected", invalid, false, 0, false},
		{"lenient rejected", invalid, true, 2, true},
		{"strict malformed", malformed, false, 0, false},
		{"lenient malformed", malformed, true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &bundledPassThroughSource{}
			result, err := DecodeAssessed(context.Background(), strings.NewReader(tc.input), DecodeAssessmentOptions{Lenient: tc.lenient, Assessment: &AssessmentOptions{Dictionaries: &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()}}})
			if source.count() != tc.calls || result.ModelLine.Requested != tc.model || !result.SourceLine.Requested {
				t.Fatalf("provider calls=%d want %d; model requested=%v want %v; source requested=%v; error=%v", source.count(), tc.calls, result.ModelLine.Requested, tc.model, result.SourceLine.Requested, err)
			}
			if tc.calls == 0 && err == nil {
				t.Fatal("rejected document returned no error")
			}
		})
	}
}

// Cancellation during the import, after line 2, ends the load as an
// operational failure that keeps the line-2 report and returns no document.
func TestStrictLoadingCancellationDuringImport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &bundledPassThroughSource{open: func(context.Context, DictionaryComponentRequest) { cancel() }}
	for _, lenient := range []bool{false, true} {
		result, err := DecodeAssessed(ctx, strings.NewReader(mandatoryDictionaryScore()), DecodeAssessmentOptions{Lenient: lenient, Assessment: &AssessmentOptions{Dictionaries: &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()}}})
		if !errors.Is(err, context.Canceled) || result.Document != nil || result.Accepted || result.ModelLine.Requested || !result.SourceLine.Satisfied() {
			t.Fatalf("lenient=%v: document=%T accepted=%v model requested=%v source satisfied=%v error=%v", lenient, result.Document, result.Accepted, result.ModelLine.Requested, result.SourceLine.Satisfied(), err)
		}
		ctx, cancel = context.WithCancel(context.Background())
		source.open = func(context.Context, DictionaryComponentRequest) { cancel() }
	}
	cancel()
}

func withoutContext(r validation.Report) validation.Report {
	r.Context = nil
	return r
}

// OmitContext leaves Report.Context empty and changes nothing else: on corpus
// files strict and lenient loading, source inspection (including the source
// origins of the internal-DTD profile) and model assessment give the same
// decisions, counts and findings with and without the projection.
func TestOmitContextKeepsEveryVerdict(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "corpus", "musicxml-test-suite", "*.xml"))
	if err != nil || len(names) < 100 {
		t.Fatalf("corpus: %d files, %v", len(names), err)
	}
	stride := 2
	if raceEnabled || testing.Short() {
		stride = 30
	}
	projected := 0
	for i := 0; i < len(names); i += stride {
		data, err := os.ReadFile(names[i])
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(names[i])
		for _, lenient := range []bool{false, true} {
			with, withErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: lenient})
			without, withoutErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: lenient, Assessment: &AssessmentOptions{OmitContext: true}})
			if len(without.SourceLine.Context) != 0 || len(without.ModelLine.Context) != 0 {
				t.Fatalf("%s: omitted projection kept %d/%d records", name, len(without.SourceLine.Context), len(without.ModelLine.Context))
			}
			if (withErr == nil) != (withoutErr == nil) || with.Accepted != without.Accepted || !reflect.DeepEqual(withoutContext(with.SourceLine), without.SourceLine) || !reflect.DeepEqual(withoutContext(with.ModelLine), without.ModelLine) || !reflect.DeepEqual(with.Conversion, without.Conversion) {
				t.Errorf("%s lenient=%v: verdicts differ without the projection: errors %v / %v", name, lenient, withErr, withoutErr)
			}
			projected += len(with.SourceLine.Context) + len(with.ModelLine.Context)
			if lenient && with.Document != nil {
				for _, profile := range []validation.ProfileID{"", ProfileModelXSD} {
					a, aErr := AssessModel(context.Background(), with.Document, AssessmentOptions{Profile: profile})
					b, bErr := AssessModel(context.Background(), with.Document, AssessmentOptions{Profile: profile, OmitContext: true})
					if (aErr == nil) != (bErr == nil) || len(b.Context) != 0 || !reflect.DeepEqual(withoutContext(a), b) {
						t.Errorf("%s AssessModel %q: verdicts differ without the projection: errors %v / %v", name, profile, aErr, bErr)
					}
				}
			}
		}
		for _, profile := range []validation.ProfileID{ProfileSourceInternalDTD, ""} {
			a, aErr := InspectXML(context.Background(), bytes.NewReader(data), AssessmentOptions{Profile: profile})
			b, bErr := InspectXML(context.Background(), bytes.NewReader(data), AssessmentOptions{Profile: profile, OmitContext: true})
			if (aErr == nil) != (bErr == nil) || len(b.Context) != 0 || !reflect.DeepEqual(withoutContext(a), b) {
				t.Errorf("%s InspectXML %q: verdicts differ without the projection: errors %v / %v", name, profile, aErr, bErr)
			}
		}
	}
	if projected == 0 {
		t.Fatal("no projection to omit")
	}
}

// Omitting the projection lowers the buffered storage strict loading needs:
// the smallest buffer that accepts 01a with the projection exceeds the one
// without it by at least a third.
func TestOmitContextLowersTheBufferedStorage(t *testing.T) {
	if raceEnabled {
		t.Skip("binary search is slow under the race detector")
	}
	data, err := os.ReadFile(filepath.Join("testdata", "corpus", "musicxml-test-suite", "01a-Pitches-Pitches.xml"))
	if err != nil {
		t.Fatal(err)
	}
	minimal := func(omit bool) int64 {
		lo, hi := int64(0), int64(256<<20)
		for hi-lo > 64<<10 {
			mid := lo + (hi-lo)/2
			_, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Assessment: &AssessmentOptions{OmitContext: omit, Budgets: validation.Budgets{MaxBufferedBytes: mid}}})
			if err == nil {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi
	}
	with, without := minimal(false), minimal(true)
	t.Logf("minimal buffer: %d with the projection, %d without", with, without)
	if without*3 > with*2 {
		t.Fatalf("omitting the projection saved too little: %d with, %d without", with, without)
	}
}

// A failed process-wide import is not cached: the call imports on its own
// account, and the next call builds the shared set. The test replaces global
// state, so it must not run in parallel.
func TestBundledDictionaryCacheDoesNotCacheFailure(t *testing.T) {
	bundledDictionaryCache.mu.Lock()
	saved := bundledDictionaryCache.set
	bundledDictionaryCache.set = nil
	bundledDictionaryCache.mu.Unlock()
	t.Cleanup(func() {
		// The set built here is left open: a view may still read it.
		bundledDictionaryCache.mu.Lock()
		bundledDictionaryCache.set = saved
		bundledDictionaryCache.mu.Unlock()
		bundledDictionaryImport = importDictionarySet
	})
	failures := 0
	bundledDictionaryImport = func(ctx context.Context, a *validation.Account, o DictionaryOptions) (*validation.DictionarySet, error) {
		if failures == 0 {
			failures++
			return nil, errors.New("process-wide import failed")
		}
		return importDictionarySet(ctx, a, o)
	}
	set, _, steps, _, _ := dictionaryLoadUsage(t, requiredDictionaryOptions())
	if steps < 1_000_000 || bundledDictionaryCache.set != nil {
		t.Fatalf("failed shared import: caller steps=%d cached=%v", steps, bundledDictionaryCache.set != nil)
	}
	_ = set.Close()
	set, _, steps, _, _ = dictionaryLoadUsage(t, requiredDictionaryOptions())
	if steps >= 100 || bundledDictionaryCache.set == nil {
		t.Fatalf("retried shared import: caller steps=%d cached=%v", steps, bundledDictionaryCache.set != nil)
	}
	_ = set.Close()
}
