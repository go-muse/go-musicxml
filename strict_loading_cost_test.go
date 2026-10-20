package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
			result, err := DecodeAssessed(context.Background(), strings.NewReader(mandatoryDictionaryScore()), DecodeAssessmentOptions{Assessment: &SourceOptions{Context: validation.ContextFamilyAll}})
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
		stride = 80
	}
	compared := 0
	for i := 0; i < len(names); i += stride {
		data, err := os.ReadFile(names[i])
		if err != nil {
			t.Fatal(err)
		}
		for _, lenient := range []bool{false, true} {
			cached, cachedErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Assessment: &SourceOptions{Context: validation.ContextFamilyAll}, Lenient: lenient})
			fresh, freshErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: lenient, Assessment: &SourceOptions{Context: validation.ContextFamilyAll, Dictionaries: &DictionaryOptions{Source: &bundledPassThroughSource{}, Selections: requiredDictionarySelections()}}})
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
			result, err := DecodeAssessed(context.Background(), strings.NewReader(tc.input), DecodeAssessmentOptions{Lenient: tc.lenient, Assessment: &SourceOptions{Context: validation.ContextFamilyAll, Dictionaries: &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()}}})
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
		result, err := DecodeAssessed(ctx, strings.NewReader(mandatoryDictionaryScore()), DecodeAssessmentOptions{Lenient: lenient, Assessment: &SourceOptions{Context: validation.ContextFamilyAll, Dictionaries: &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()}}})
		if !errors.Is(err, context.Canceled) || result.Document != nil || result.Accepted || result.ModelLine.Requested || !result.SourceLine.Satisfied() {
			t.Fatalf("lenient=%v: document=%T accepted=%v model requested=%v source satisfied=%v error=%v", lenient, result.Document, result.Accepted, result.ModelLine.Requested, result.SourceLine.Satisfied(), err)
		}
		ctx, cancel = context.WithCancel(context.Background())
		source.open = func(context.Context, DictionaryComponentRequest) { cancel() }
	}
	cancel()
}

// invalidDictionaryScore fails line 2 on its pitch step and still carries the
// SMuFL and IPA values that line 3 checks against the dictionaries.
func invalidDictionaryScore() string {
	return strings.Replace(mandatoryDictionaryScore(), "<step>C</step>", "<step>H</step>", 1)
}

// eofReader records whether the source reached its end.
type eofReader struct {
	io.Reader
	eof atomic.Bool
}

func (r *eofReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		r.eof.Store(true)
	}
	return n, err
}

// An ordinary provider failure after line 2 leaves a set whose components are
// unavailable: line 3 still runs on it and reports the unavailable evidence,
// and the failure rejects the document in both modes. The provider is opened
// once per component, only after line 2 read the whole source, and never
// after a line-2 rejection in the strict mode.
func TestStrictLoadingNonTerminalImportFailureAfterLineTwo(t *testing.T) {
	providerFailure := errors.New("dictionary provider failed")
	invalid := invalidDictionaryScore()
	for _, tc := range []struct {
		name, input    string
		lenient        bool
		calls          int
		sourceAccepted bool
	}{
		{"strict accepted", mandatoryDictionaryScore(), false, 2, true},
		{"lenient accepted", mandatoryDictionaryScore(), true, 2, true},
		{"strict rejected", invalid, false, 0, false},
		{"lenient rejected", invalid, true, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &eofReader{Reader: strings.NewReader(tc.input)}
			var calls atomic.Int32
			var openedBeforeEOF atomic.Bool
			source := dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls.Add(1)
				if !reader.eof.Load() {
					openedBeforeEOF.Store(true)
				}
				return DictionaryComponentResult{}, providerFailure
			})
			result, err := DecodeAssessed(context.Background(), reader, DecodeAssessmentOptions{Lenient: tc.lenient, Assessment: &SourceOptions{Context: validation.ContextFamilyAll, Dictionaries: &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()}}})
			if int(calls.Load()) != tc.calls || openedBeforeEOF.Load() {
				t.Fatalf("provider calls=%d want %d; opened before the source end=%v", calls.Load(), tc.calls, openedBeforeEOF.Load())
			}
			if err == nil || result.Document != nil || result.Accepted || result.SourceLine.Satisfied() != tc.sourceAccepted || !result.SourceLine.InputComplete {
				t.Fatalf("document=%T accepted=%v source satisfied=%v complete=%v error=%v", result.Document, result.Accepted, result.SourceLine.Satisfied(), result.SourceLine.InputComplete, err)
			}
			if tc.calls == 0 {
				if result.ModelLine.Requested {
					t.Fatal("line 3 ran after a strict line-2 rejection")
				}
				return
			}
			var importErr *DictionaryImportError
			if !errors.Is(err, providerFailure) || !errors.As(err, &importErr) || !errors.Is(err, ErrDictionaryUnavailable) {
				t.Fatalf("import failure lost: %v", err)
			}
			if !result.ModelLine.Requested || !result.ModelLine.InputComplete || result.ModelLine.Satisfied() {
				t.Fatalf("line 3 did not report the unavailable set: requested=%v complete=%v satisfied=%v", result.ModelLine.Requested, result.ModelLine.InputComplete, result.ModelLine.Satisfied())
			}
			for _, property := range []validation.ContextProperty{validation.ContextDictionaryCanonical, validation.ContextDictionaryIPARepresentation} {
				evidence := dictionaryResults(result.ModelLine, property)
				if len(evidence) != 1 || evidence[0].ComponentVerified || evidence[0].Result.State == validation.EvidenceKnown {
					t.Fatalf("unavailable %s evidence: %+v", property, evidence)
				}
			}
		})
	}
}

// A resource refusal during the import is terminal: line 3 does not start, the
// document is not returned, and the error is the resource limit itself rather
// than a dictionary failure. The import draws on the same account as line 2,
// which has already completed and keeps its report.
func TestStrictLoadingTerminalImportFailureAfterLineTwo(t *testing.T) {
	invalid := invalidDictionaryScore()
	for _, tc := range []struct {
		name, input    string
		lenient        bool
		calls          int
		sourceAccepted bool
	}{
		{"strict accepted", mandatoryDictionaryScore(), false, 1, true},
		{"lenient accepted", mandatoryDictionaryScore(), true, 1, true},
		{"strict rejected", invalid, false, 0, false},
		{"lenient rejected", invalid, true, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &eofReader{Reader: strings.NewReader(tc.input)}
			source := &bundledPassThroughSource{open: func(context.Context, DictionaryComponentRequest) {
				if !reader.eof.Load() {
					t.Error("dictionary opened before the source end")
				}
			}}
			// Line 2 charges no external bytes; the first dictionary body
			// exceeds this limit.
			result, err := DecodeAssessed(context.Background(), reader, DecodeAssessmentOptions{Lenient: tc.lenient, Assessment: &SourceOptions{
				Context:      validation.ContextFamilyAll,
				Budgets:      validation.Budgets{MaxExternalBytes: 1024},
				Dictionaries: &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()},
			}})
			if source.count() != tc.calls {
				t.Fatalf("provider calls=%d want %d; error=%v", source.count(), tc.calls, err)
			}
			if err == nil || result.Document != nil || result.Accepted || result.ModelLine.Requested || !result.SourceLine.Requested || !result.SourceLine.InputComplete || result.SourceLine.Satisfied() != tc.sourceAccepted {
				t.Fatalf("document=%T accepted=%v model requested=%v source satisfied=%v complete=%v error=%v", result.Document, result.Accepted, result.ModelLine.Requested, result.SourceLine.Satisfied(), result.SourceLine.InputComplete, err)
			}
			if tc.calls == 0 {
				return
			}
			var limit *validation.ResourceError
			if !errors.As(err, &limit) || limit.Kind != validation.BudgetExternalBytes || limit.Limit != 1024 {
				t.Fatalf("import did not refuse on the external-byte limit: %v", err)
			}
			var importErr *DictionaryImportError
			if errors.As(err, &importErr) || errors.Is(err, ErrDictionaryUnavailable) {
				t.Fatalf("terminal refusal reported as an unavailable dictionary: %v", err)
			}
		})
	}
}

// cancelAtImportContext cancels on the first context check made by the line-3
// import, which is the boundary between the two lines.
type cancelAtImportContext struct {
	context.Context
	cancel context.CancelFunc
	fired  atomic.Bool
}

func (c *cancelAtImportContext) Err() error {
	if !c.fired.Load() && calledFrom(".dictionaryAssessmentConfigAccount") {
		c.fired.Store(true)
		c.cancel()
	}
	return c.Context.Err()
}

func calledFrom(suffix string) bool {
	pcs := make([]uintptr, 8)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, suffix) {
			return true
		}
		if !more {
			return false
		}
	}
}

// Cancellation after line 2 and before the import opens nothing, starts no
// line 3 and returns no document; the line-2 report stays. The default bundled
// set, which is shared rather than imported, stops at the same boundary.
func TestStrictLoadingCancellationBetweenLines(t *testing.T) {
	invalid := invalidDictionaryScore()
	for _, tc := range []struct {
		name, input    string
		lenient        bool
		caller         bool
		sourceAccepted bool
	}{
		{"strict caller source", mandatoryDictionaryScore(), false, true, true},
		{"lenient caller source", mandatoryDictionaryScore(), true, true, true},
		{"lenient rejected caller source", invalid, true, true, false},
		{"strict bundled", mandatoryDictionaryScore(), false, false, true},
		{"lenient bundled", mandatoryDictionaryScore(), true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &cancelAtImportContext{Context: base, cancel: cancel}
			source := &bundledPassThroughSource{}
			assessment := &SourceOptions{Context: validation.ContextFamilyAll}
			if tc.caller {
				assessment.Dictionaries = &DictionaryOptions{Source: source, Selections: requiredDictionarySelections()}
			}
			result, err := DecodeAssessed(ctx, strings.NewReader(tc.input), DecodeAssessmentOptions{Lenient: tc.lenient, Assessment: assessment})
			if !ctx.fired.Load() {
				t.Fatalf("the import boundary was never reached: %v", err)
			}
			var incomplete *validation.IncompleteError
			if !errors.Is(err, context.Canceled) || !errors.As(err, &incomplete) || incomplete.Reason != "canceled-before-dictionaries" {
				t.Fatalf("cancellation between the lines lost its boundary: %v", err)
			}
			if source.count() != 0 || result.Document != nil || result.Accepted || result.ModelLine.Requested || !result.SourceLine.InputComplete || result.SourceLine.Satisfied() != tc.sourceAccepted {
				t.Fatalf("provider calls=%d document=%T accepted=%v model requested=%v source satisfied=%v complete=%v", source.count(), result.Document, result.Accepted, result.ModelLine.Requested, result.SourceLine.Satisfied(), result.SourceLine.InputComplete)
			}
		})
	}
}

// selectedContext keeps the records of the complete projection whose family
// families selects, in order.
func selectedContext(r validation.Report, families validation.ContextFamilies) validation.Report {
	var kept []validation.ContextResult
	for _, c := range r.Context {
		if c.Family()&families != 0 {
			kept = append(kept, c)
		}
	}
	r.Context = kept
	return r
}

// A context selection keeps exactly the records of the complete projection
// whose family it names and changes nothing else: on corpus files strict and
// lenient loading, source inspection (including the source origins of the
// internal-DTD profile) and model assessment give the same decisions, counts
// and findings with the complete projection, with none (the default) and with
// a rotating single family.
func TestContextSelectionKeepsEveryVerdict(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "corpus", "musicxml-test-suite", "*.xml"))
	if err != nil || len(names) < 100 {
		t.Fatalf("corpus: %d files, %v", len(names), err)
	}
	stride := 2
	if raceEnabled || testing.Short() {
		stride = 75
	}
	projected, partial := 0, 0
	same := func(all validation.Report, families validation.ContextFamilies, got validation.Report) bool {
		want := selectedContext(all, families)
		if len(want.Context) == 0 && len(got.Context) == 0 {
			want.Context, got.Context = nil, nil
		}
		if families != 0 && len(want.Context) != 0 && len(want.Context) != len(all.Context) {
			partial++
		}
		return reflect.DeepEqual(want, got)
	}
	for i := 0; i < len(names); i += stride {
		data, err := os.ReadFile(names[i])
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(names[i])
		one := validation.ContextFamilies(1) << (i / stride % 11)
		for _, lenient := range []bool{false, true} {
			all, allErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: lenient, Assessment: &SourceOptions{Context: validation.ContextFamilyAll}})
			projected += len(all.SourceLine.Context) + len(all.ModelLine.Context)
			for _, c := range append(slices.Clip(all.SourceLine.Context), all.ModelLine.Context...) {
				if f := c.Family(); f == 0 || f&(f-1) != 0 {
					t.Fatalf("%s: property %q has family %b", name, c.Property, f)
				}
			}
			for _, families := range []validation.ContextFamilies{0, one} {
				var assessment *SourceOptions
				if families != 0 {
					assessment = &SourceOptions{Context: families}
				}
				got, gotErr := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: lenient, Assessment: assessment})
				if (allErr == nil) != (gotErr == nil) || all.Accepted != got.Accepted || !same(all.SourceLine, families, got.SourceLine) || !same(all.ModelLine, families, got.ModelLine) || !reflect.DeepEqual(all.Conversion, got.Conversion) {
					t.Errorf("%s lenient=%v context=%b: reports differ from the selected projection: errors %v / %v", name, lenient, families, allErr, gotErr)
				}
			}
			if lenient && all.Document != nil {
				for _, profile := range []validation.ProfileID{"", ProfileModelXSD.ID()} {
					a, aErr := AssessModel(context.Background(), all.Document, ModelOptions{Profile: ModelProfile(profile), Context: validation.ContextFamilyAll})
					for _, families := range []validation.ContextFamilies{0, one} {
						b, bErr := AssessModel(context.Background(), all.Document, ModelOptions{Profile: ModelProfile(profile), Context: families})
						if (aErr == nil) != (bErr == nil) || !same(a, families, b) {
							t.Errorf("%s AssessModel %q context=%b: reports differ from the selected projection: errors %v / %v", name, profile, families, aErr, bErr)
						}
					}
				}
			}
		}
		for _, profile := range []validation.ProfileID{ProfileSourceInternalDTD.ID(), ""} {
			a, aErr := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: SourceProfile(profile), Context: validation.ContextFamilyAll})
			for _, families := range []validation.ContextFamilies{0, one} {
				b, bErr := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: SourceProfile(profile), Context: families})
				if (aErr == nil) != (bErr == nil) || !same(a, families, b) {
					t.Errorf("%s InspectXML %q context=%b: reports differ from the selected projection: errors %v / %v", name, profile, families, aErr, bErr)
				}
			}
		}
	}
	if projected == 0 || partial == 0 && !testing.Short() && !raceEnabled {
		t.Fatalf("no projection to select: %d records, %d partial selections", projected, partial)
	}
}

// The default empty selection roughly halves the buffered storage strict
// loading needs: the smallest buffer that accepts 01a without the projection
// is at most 55% of the one with the complete projection.
func TestEmptyContextSelectionLowersTheBufferedStorage(t *testing.T) {
	if raceEnabled {
		t.Skip("binary search is slow under the race detector")
	}
	data, err := os.ReadFile(filepath.Join("testdata", "corpus", "musicxml-test-suite", "01a-Pitches-Pitches.xml"))
	if err != nil {
		t.Fatal(err)
	}
	minimal := func(families validation.ContextFamilies) int64 {
		lo, hi := int64(0), int64(256<<20)
		for hi-lo > 64<<10 {
			mid := lo + (hi-lo)/2
			_, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Assessment: &SourceOptions{Context: families, Budgets: validation.Budgets{MaxBufferedBytes: mid}}})
			if err == nil {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi
	}
	with, without := minimal(validation.ContextFamilyAll), minimal(0)
	t.Logf("minimal buffer: %d with the projection, %d without", with, without)
	if without*100 > with*55 {
		t.Fatalf("leaving out the projection saved too little: %d with, %d without", with, without)
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
		bundledDictionaryImport = importDictionarySet
		bundledDictionaryCache.mu.Unlock()
	})
	// The hook runs under the cache mutex, which also guards failures.
	failures := 0
	bundledDictionaryCache.mu.Lock()
	bundledDictionaryImport = func(ctx context.Context, a *validation.Account, o DictionaryOptions) (*validation.DictionarySet, error) {
		if failures == 0 {
			failures++
			return nil, errors.New("process-wide import failed")
		}
		return importDictionarySet(ctx, a, o)
	}
	bundledDictionaryCache.mu.Unlock()
	cached := func() bool {
		bundledDictionaryCache.mu.Lock()
		defer bundledDictionaryCache.mu.Unlock()
		return bundledDictionaryCache.set != nil
	}
	set, _, steps, _, _ := dictionaryLoadUsage(t, requiredDictionaryOptions())
	if steps < 1_000_000 || cached() {
		t.Fatalf("failed shared import: caller steps=%d cached=%v", steps, cached())
	}
	_ = set.Close()
	set, _, steps, _, _ = dictionaryLoadUsage(t, requiredDictionaryOptions())
	if steps >= 100 || !cached() {
		t.Fatalf("retried shared import: caller steps=%d cached=%v", steps, cached())
	}
	_ = set.Close()
}

// bundleView is everything an assessment bundle returns except resource IDs,
// which are random per run.
func bundleView(b validation.AssessmentBundle) []any {
	view := []any{b.Satisfied(), b.EnumerationComplete(), b.AdoptionCount(), b.PackageReport()}
	for _, report := range b.Resources() {
		view = append(view, report)
	}
	if failure, ok := b.FailureReport(); ok {
		view = append(view, failure)
	}
	return view
}

// narrowView narrows the projection of every report in view to families and
// replaces the random run identity in its document IDs.
func narrowView(view []any, families validation.ContextFamilies) []any {
	out := append([]any{}, view...)
	for i, v := range out {
		if report, ok := v.(validation.Report); ok {
			report = selectedContext(report, families)
			if len(report.Context) == 0 {
				report.Context = nil
			}
			scrubRunIdentity(reflect.ValueOf(&report).Elem())
			out[i] = report
		}
	}
	return out
}

var runIdentity = regexp.MustCompile(`resource-run-[0-9a-f]{32}`)

// scrubRunIdentity rewrites, in place, every settable string under v.
func scrubRunIdentity(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		if v.CanSet() {
			v.SetString(runIdentity.ReplaceAllString(v.String(), "resource-run"))
		}
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			scrubRunIdentity(v.Elem())
		}
	case reflect.Struct:
		for i := range v.NumField() {
			scrubRunIdentity(v.Field(i))
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			scrubRunIdentity(v.Index(i))
		}
	}
}

// The graph entry points honour the selection too: InspectMXL, both lines of
// DecodeMXLAssessed and AssessMXLPackage return the bundles of the complete
// projection, narrowed to the selected families, although their resource
// snapshots keep the records their package rules read.
func TestGraphContextSelectionKeepsEveryVerdict(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "corpus", "musicxml-test-suite", "90a-Compressed-MusicXML.mxl"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(families validation.ContextFamilies) map[string][]any {
		options := MXLAssessmentOptions{Graph: MXLGraphOptions{Context: families}}
		bundle, err := InspectMXL(context.Background(), bytes.NewReader(data), options)
		if err != nil {
			t.Fatalf("context=%b: InspectMXL: %v", families, err)
		}
		result, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(data), DecodeMXLAssessmentOptions{Assessment: options, Lenient: true})
		if err != nil || result.Package == nil {
			t.Fatalf("context=%b: DecodeMXLAssessed: %v", families, err)
		}
		current, err := AssessMXLPackage(context.Background(), result.Package, options)
		if err != nil {
			t.Fatalf("context=%b: AssessMXLPackage: %v", families, err)
		}
		return map[string][]any{"InspectMXL": bundleView(bundle), "source line": bundleView(result.SourceLine), "model line": bundleView(result.ModelLine), "AssessMXLPackage": bundleView(current)}
	}
	all := run(validation.ContextFamilyAll)
	projected := 0
	for _, view := range all {
		for _, v := range view {
			if report, ok := v.(validation.Report); ok {
				projected += len(report.Context)
			}
		}
	}
	if projected == 0 {
		t.Fatal("no projection to select")
	}
	for _, families := range []validation.ContextFamilies{0, validation.ContextFamilyLayout, validation.ContextFamilyAll &^ validation.ContextFamilyLayout} {
		for name, view := range run(families) {
			if !reflect.DeepEqual(narrowView(all[name], families), narrowView(view, validation.ContextFamilyAll)) {
				t.Errorf("context=%b: %s differs from the selected projection", families, name)
			}
		}
	}
}
