package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type dictionaryImportSourceFunc func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error)

func (f dictionaryImportSourceFunc) Open(ctx context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
	return f(ctx, r)
}

type dictionaryImportNilSource struct{}

func (*dictionaryImportNilSource) Open(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
	panic("typed nil source invoked")
}

func dictionaryImportSelection(id validation.DictionaryManifestID) validation.DictionarySelection {
	m, ok := validation.LookupDictionaryManifest(id)
	if !ok {
		panic("unknown test manifest")
	}
	return validation.DictionarySelection{Manifest: id, Relations: m.Relations}
}
func dictionaryImportFixture(t *testing.T, id validation.DictionaryManifestID) []byte {
	t.Helper()
	paths := map[validation.DictionaryManifestID]string{
		validation.DictionaryManifestSMuFL14Canonical:         "smufl-1.4/glyphnames.json",
		validation.DictionaryManifestSMuFL14Ranges:            "smufl-1.4/ranges.json",
		validation.DictionaryManifestStandardSounds40:         "musicxml-4.0/sounds.xml",
		validation.DictionaryManifestIPAPositive2015Unicode13: "ipa-positive/runtime-positive-bundle.json",
	}
	raw, err := os.ReadFile("testdata/dictionaries/" + paths[id])
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func dictionaryImportAccount(t *testing.T, b validation.Budgets) *validation.Account {
	t.Helper()
	a, e := validation.NewBudgetAccount(b)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func dictionaryImportNoLive(t *testing.T, a *validation.Account) {
	t.Helper()
	if n, e := a.Usage(validation.BudgetBufferedBytes); e != nil || n != 0 {
		t.Fatalf("live=%d err=%v", n, e)
	}
}
func dictionaryImportFixtureSource(t *testing.T, calls *[]DictionaryComponentRequest) DictionarySource {
	return dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
		*calls = append(*calls, r)
		m, ok := validation.LookupDictionaryManifest(r.Manifest)
		if !ok || r.Component != m.Component {
			t.Fatalf("unexpected request %+v", r)
		}
		return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: io.NopCloser(bytes.NewReader(dictionaryImportFixture(t, r.Manifest)))}, nil
	})
}

func TestDictionaryImportCertifiedComponents(t *testing.T) {
	ids := []validation.DictionaryManifestID{validation.DictionaryManifestSMuFL14Canonical, validation.DictionaryManifestSMuFL14Ranges, validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestIPAPositive2015Unicode13}
	a := dictionaryImportAccount(t, validation.Budgets{})
	var calls []DictionaryComponentRequest
	o := DictionaryOptions{Source: dictionaryImportFixtureSource(t, &calls)}
	var wantBytes int64
	for _, id := range ids {
		o.Selections = append(o.Selections, dictionaryImportSelection(id))
		m, _ := validation.LookupDictionaryManifest(id)
		wantBytes += m.Component.Bytes
	}
	set, err := loadDictionarySet(context.Background(), a, o)
	if err != nil || set == nil {
		t.Fatalf("set=%v err=%v", set, err)
	}
	if len(calls) != 4 {
		t.Fatalf("calls=%d", len(calls))
	}
	if n, _ := a.Usage(validation.BudgetExternalBytes); n != wantBytes {
		t.Fatalf("external=%d want=%d", n, wantBytes)
	}
	if _, err = set.Fingerprint(); err != nil {
		t.Fatal(err)
	}
	if err = set.Close(); err != nil {
		t.Fatal(err)
	}
	dictionaryImportNoLive(t, a)
}
func TestDictionaryImportSelectionBeforeCallbacks(t *testing.T) {
	good := dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)
	cases := []struct {
		name       string
		selections []validation.DictionarySelection
	}{
		{"unknown manifest", []validation.DictionarySelection{{Manifest: "latest", Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName}}}},
		{"duplicate manifest", []validation.DictionarySelection{good, good}},
		{"empty relations", []validation.DictionarySelection{{Manifest: good.Manifest}}},
		{"unknown relation", []validation.DictionarySelection{{Manifest: good.Manifest, Relations: []validation.DictionaryRelation{"classes"}}}},
		{"duplicate relation", []validation.DictionarySelection{{Manifest: good.Manifest, Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName, validation.DictionaryRelationCanonicalName}}}},
		{"too many manifests", []validation.DictionarySelection{good, good, good, good, good}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			calls := 0
			o := DictionaryOptions{Selections: tc.selections, Source: dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				panic("invalid selection called source")
			})}
			if err := validateDictionaryOptions(o); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatal(err)
			}
			set, err := loadDictionarySet(context.Background(), a, o)
			if set != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || calls != 0 {
				t.Fatalf("set=%v err=%v calls=%d", set, err, calls)
			}
			dictionaryImportNoLive(t, a)
		})
	}
}
func TestDictionaryImportEmptyAndMissingSources(t *testing.T) {
	var typedNil *dictionaryImportNilSource
	for _, tc := range []struct {
		name string
		o    DictionaryOptions
	}{
		{"empty", DictionaryOptions{Source: dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
			panic("empty selection called source")
		})}},
		{"nil source", DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40)}}},
		{"typed nil source", DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40)}, Source: typedNil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			set, err := loadDictionarySet(context.Background(), a, tc.o)
			if err != nil || set == nil {
				t.Fatalf("set=%v err=%v", set, err)
			}
			set.Close()
			dictionaryImportNoLive(t, a)
			if n, _ := a.Usage(validation.BudgetExternalBytes); n != 0 {
				t.Fatal(n)
			}
		})
	}
}
func TestDictionaryImportProviderFaultOwnership(t *testing.T) {
	transport, closing := errors.New("provider failed"), errors.New("close failed")
	for _, name := range []string{"provider error", "wrong echo", "nil body", "typed nil body", "bad state", "known reason", "unknown with body", "invalid unknown reason"} {
		t.Run(name, func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			reader := bytes.NewReader([]byte("never read"))
			body := &resourceTestBody{Reader: reader, closeErr: closing}
			hasBody := true
			o := DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40)}, Source: dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				out := DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}
				switch name {
				case "provider error":
					return out, transport
				case "wrong echo":
					out.Request.Component.SHA256 = "wrong"
				case "nil body":
					out.Body = nil
					hasBody = false
				case "typed nil body":
					var p *resourceTestBody
					out.Body = p
					hasBody = false
				case "bad state":
					out.State = validation.EvidenceInvalid
				case "known reason":
					out.Reason = "contradiction"
				case "unknown with body":
					out.State = validation.EvidenceUnknown
					out.Reason = "unavailable"
				case "invalid unknown reason":
					out.State = validation.EvidenceUnknown
					out.Reason = "bad reason"
					out.Body = nil
					hasBody = false
				}
				return out, nil
			})}
			set, err := loadDictionarySet(context.Background(), a, o)
			if set == nil || err == nil {
				t.Fatalf("set=%v err=%v", set, err)
			}
			if name == "provider error" && !errors.Is(err, transport) {
				t.Fatal(err)
			}
			if hasBody && (!errors.Is(err, closing) || body.closed != 1) {
				t.Fatalf("err=%v closed=%d", err, body.closed)
			}
			if !hasBody && body.closed != 0 {
				t.Fatal(body.closed)
			}
			if reader.Len() != len("never read") {
				t.Fatal("fault body read")
			}
			set.Close()
			dictionaryImportNoLive(t, a)
		})
	}
}
func TestDictionaryImportIntegrityAndReadFaults(t *testing.T) {
	raw := dictionaryImportFixture(t, validation.DictionaryManifestStandardSounds40)
	for _, name := range []string{"short", "oversize", "changed digest", "normalized line endings", "read error", "close error"} {
		t.Run(name, func(t *testing.T) {
			data := append([]byte(nil), raw...)
			sentinel := errors.New("stream fault")
			var read io.Reader
			var closeErr error
			switch name {
			case "short":
				data = data[:len(data)-1]
			case "oversize":
				data = append(data, make([]byte, 10000)...)
			case "changed digest":
				data[len(data)-4] ^= 1
			case "normalized line endings":
				data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
			case "read error":
				read = &resourceReadError{data, sentinel}
			case "close error":
				closeErr = sentinel
			}
			if read == nil {
				read = bytes.NewReader(data)
			}
			body := &resourceTestBody{Reader: read, closeErr: closeErr}
			a := dictionaryImportAccount(t, validation.Budgets{})
			o := DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40)}, Source: dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}, nil
			})}
			set, err := loadDictionarySet(context.Background(), a, o)
			if err == nil || set == nil || body.closed != 1 {
				t.Fatalf("set=%v err=%v closes=%d", set, err, body.closed)
			}
			if n, _ := a.Usage(validation.BudgetExternalBytes); n > int64(len(raw)+1) {
				t.Fatal(n)
			}
			set.Close()
			dictionaryImportNoLive(t, a)
		})
	}
}
func TestDictionaryImportPartialOptionalComponent(t *testing.T) {
	a := dictionaryImportAccount(t, validation.Budgets{})
	boom := errors.New("optional ranges unavailable")
	var calls []DictionaryComponentRequest
	source := dictionaryImportFixtureSource(t, &calls)
	selections := []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical), dictionaryImportSelection(validation.DictionaryManifestSMuFL14Ranges)}
	set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: selections, Source: dictionaryImportSourceFunc(func(ctx context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
		if r.Manifest == validation.DictionaryManifestSMuFL14Ranges {
			return DictionaryComponentResult{Request: r}, boom
		}
		return source.Open(ctx, r)
	})})
	if set == nil || !errors.Is(err, boom) || len(calls) != 1 {
		t.Fatalf("set=%v err=%v calls=%d", set, err, len(calls))
	}
	fp, err := set.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	missing, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: selections})
	if err != nil {
		t.Fatal(err)
	}
	emptyFP, _ := missing.Fingerprint()
	if fp == emptyFP {
		t.Fatal("successful canonical component erased")
	}
	set.Close()
	missing.Close()
	dictionaryImportNoLive(t, a)
}
func TestDictionaryImportTerminalCancellationAndBudgets(t *testing.T) {
	id := validation.DictionaryManifestStandardSounds40
	selection := dictionaryImportSelection(id)
	raw := dictionaryImportFixture(t, id)
	load := func(b validation.Budgets) (*validation.Account, error) {
		a := dictionaryImportAccount(t, b)
		var calls []DictionaryComponentRequest
		set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{selection}, Source: dictionaryImportFixtureSource(t, &calls)})
		if set != nil {
			set.Close()
		}
		dictionaryImportNoLive(t, a)
		return a, err
	}
	a, err := load(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	work, _ := a.Usage(validation.BudgetRuleSteps)
	for _, tc := range []struct {
		name string
		b    validation.Budgets
		ok   bool
	}{
		{"exact bytes", validation.Budgets{MaxExternalBytes: int64(len(raw))}, true}, {"one under bytes", validation.Budgets{MaxExternalBytes: int64(len(raw) - 1)}, false},
		{"exact work", validation.Budgets{MaxRuleSteps: work}, true}, {"one under work", validation.Budgets{MaxRuleSteps: work - 1}, false},
		{"tiny live budget", validation.Budgets{MaxBufferedBytes: 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := load(tc.b)
			if (err == nil) != tc.ok || (!tc.ok && !errors.Is(err, validation.ErrResourceLimit)) {
				t.Fatal(err)
			}
		})
	}
	// Find the concrete peak-admission boundary and verify both sides, rather than
	// treating final retained bytes as the peak while raw and typed data overlap.
	low, high := int64(1), int64(1<<20)
	for low < high {
		mid := low + (high-low)/2
		_, err := load(validation.Budgets{MaxBufferedBytes: mid})
		if err == nil {
			high = mid
		} else if errors.Is(err, validation.ErrResourceLimit) {
			low = mid + 1
		} else {
			t.Fatal(err)
		}
	}
	if _, err := load(validation.Budgets{MaxBufferedBytes: low}); err != nil {
		t.Fatal(err)
	}
	if _, err := load(validation.Budgets{MaxBufferedBytes: low - 1}); !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	body := &resourceTestBody{Reader: bytes.NewReader(raw)}
	a = dictionaryImportAccount(t, validation.Budgets{})
	set, err := loadDictionarySet(ctx, a, DictionaryOptions{Selections: []validation.DictionarySelection{selection}, Source: dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
		cancel()
		return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}, nil
	})})
	if set != nil || !errors.Is(err, context.Canceled) || body.closed != 1 {
		t.Fatalf("set=%v err=%v closes=%d", set, err, body.closed)
	}
	dictionaryImportNoLive(t, a)
}

func TestDictionaryImportExplicitUnavailableStates(t *testing.T) {
	for _, state := range []validation.EvidenceState{validation.EvidenceUnknown, validation.EvidenceUnsupported} {
		t.Run(string(state), func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			calls := 0
			set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Ranges)}, Source: dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				if r.Manifest != validation.DictionaryManifestSMuFL14Ranges {
					t.Fatal("implicit canonical request")
				}
				return DictionaryComponentResult{Request: r, State: state, Reason: "caller-offline"}, nil
			})})
			if err != nil || set == nil || calls != 1 {
				t.Fatalf("set=%v err=%v calls=%d", set, err, calls)
			}
			set.Close()
			dictionaryImportNoLive(t, a)
		})
	}
}
func TestDictionaryImportPreservesCallerAccountAndStopsCallbacks(t *testing.T) {
	ids := []validation.DictionaryManifestID{validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestSMuFL14Canonical}
	for _, which := range []string{"already canceled", "provider canceled", "resource exhausted", "nil context", "zero account"} {
		t.Run(which, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			budgets := validation.Budgets{}
			if which == "resource exhausted" {
				budgets.MaxRuleSteps = 3
			}
			a := dictionaryImportAccount(t, budgets)
			if err := a.Charge(validation.BudgetBufferedBytes, 17); err != nil {
				t.Fatal(err)
			}
			calls := 0
			selections := []validation.DictionarySelection{dictionaryImportSelection(ids[0]), dictionaryImportSelection(ids[1])}
			if which == "already canceled" {
				cancel()
			}
			if which == "nil context" {
				ctx = nil
			}
			runAccount := a
			if which == "zero account" {
				runAccount = &validation.Account{}
			}
			set, err := loadDictionarySet(ctx, runAccount, DictionaryOptions{Selections: selections, Source: dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				if which == "provider canceled" {
					cancel()
					return DictionaryComponentResult{Request: r}, context.Canceled
				}
				return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: io.NopCloser(bytes.NewReader(dictionaryImportFixture(t, r.Manifest)))}, nil
			})})
			if set != nil || err == nil {
				t.Fatalf("set=%v err=%v", set, err)
			}
			if calls > 1 || ((which == "already canceled" || which == "nil context" || which == "zero account") && calls != 0) {
				t.Fatal(calls)
			}
			if n, _ := a.Usage(validation.BudgetBufferedBytes); n != 17 {
				t.Fatalf("caller lease changed: %d", n)
			}
			a.Release(validation.BudgetBufferedBytes, 17)
		})
	}
}
func TestDictionaryImportSpecializedJSONGuards(t *testing.T) {
	for _, raw := range []string{`"\uD800"`, `"\uDC00"`, `"\uD800\u0061"`, `"\q"`, "\"\xff\"", `"unterminated`, `"\u0000"`} {
		t.Run(raw, func(t *testing.T) {
			c := dictionaryJSON{ctx: context.Background(), raw: []byte(raw)}
			got := c.str(40)
			if raw == `"\u0000"` {
				if c.err != nil || len(got) != 1 || got[0] != 0 {
					t.Fatalf("got=%q err=%v", got, c.err)
				}
			} else if c.err == nil {
				t.Fatalf("accepted %q", raw)
			}
		})
	}
	for _, raw := range []string{`{"gClef":{"codepoint":"U+E050","codepoint":"U+E050","description":"x"}}`, `{"gClef":{"codepoint":"U+E050","description":"x"},"g\u0043lef":{"codepoint":"U+E050","description":"x"}}`, `{"gClef":{"codepoint":"U+e050","description":"x"}}`, `{"gClef":{"codepoint":"U+D800","description":"x"}}`, `{"gClef":{"codepoint":"U+E050","description":"x","extra":"x"}}`} {
		c := dictionaryJSON{ctx: context.Background(), raw: []byte(raw), copyLimit: 58476}
		if _, err := decodeDictionaryGlyphs(&c); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestDictionaryImportDerivedBundleSourceKind(t *testing.T) {
	for _, id := range []validation.DictionaryManifestID{validation.DictionaryManifestSMuFL14Canonical, validation.DictionaryManifestSMuFL14Ranges, validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestIPAPositive2015Unicode13} {
		a := dictionaryImportAccount(t, validation.Budgets{})
		m, _ := validation.LookupDictionaryManifest(id)
		var calls []DictionaryComponentRequest
		out, err := importDictionaryComponent(context.Background(), a, dictionaryImportFixtureSource(t, &calls), DictionaryComponentRequest{Manifest: id, Component: m.Component})
		if err != nil {
			t.Fatal(err)
		}
		if out.data.Certificate == nil {
			t.Fatal("missing raw certificate")
		}
		if id == validation.DictionaryManifestIPAPositive2015Unicode13 {
			if m.Component.SourceKind != validation.DictionarySourceReviewedDerivedBundle || out.data.IPA == nil {
				t.Fatal("IPA must retain derived-bundle lineage")
			}
		} else if m.Component.SourceKind != validation.DictionarySourceOriginalComponent {
			t.Fatal("lost original-component lineage")
		}
		set, err := validation.NewDictionarySet(context.Background(), a, []validation.DictionarySelection{dictionaryImportSelection(id)}, []validation.DictionaryImport{out.data})
		if err != nil {
			t.Fatal(err)
		}
		out.close()
		set.Close()
		dictionaryImportNoLive(t, a)
	}
}

func TestDictionaryImportSuccessfulComponentBeforeTerminalStop(t *testing.T) {
	firstID := validation.DictionaryManifestStandardSounds40
	secondID := validation.DictionaryManifestSMuFL14Canonical
	firstRaw := dictionaryImportFixture(t, firstID)
	secondRaw := dictionaryImportFixture(t, secondID)
	for _, terminal := range []string{"cancellation", "external quota"} {
		t.Run(terminal, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			budgets := validation.Budgets{}
			if terminal == "external quota" {
				budgets.MaxExternalBytes = int64(len(firstRaw))
			}
			a := dictionaryImportAccount(t, budgets)
			if err := a.Charge(validation.BudgetBufferedBytes, 17); err != nil {
				t.Fatal(err)
			}
			firstBody := &resourceTestBody{Reader: bytes.NewReader(firstRaw)}
			secondReader := bytes.NewReader(secondRaw)
			secondBody := &resourceTestBody{Reader: secondReader}
			calls := 0
			source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				var body io.ReadCloser = firstBody
				if calls == 2 {
					if firstBody.closed != 1 {
						t.Fatal("first component was not completed before second callback")
					}
					if live, _ := a.Usage(validation.BudgetBufferedBytes); live <= 8192+17 {
						t.Fatalf("first typed component has no retained lease: %d", live)
					}
					body = secondBody
					if terminal == "cancellation" {
						cancel()
					}
				}
				if calls > 2 {
					t.Fatal("unexpected provider call")
				}
				return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}, nil
			})
			set, err := loadDictionarySet(ctx, a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(firstID), dictionaryImportSelection(secondID)}, Source: source})
			want := validation.ErrResourceLimit
			if terminal == "cancellation" {
				want = context.Canceled
			}
			if set != nil || !errors.Is(err, want) || calls != 2 || firstBody.closed != 1 || secondBody.closed != 1 {
				t.Fatalf("set=%v err=%v calls=%d closes=%d/%d", set, err, calls, firstBody.closed, secondBody.closed)
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 17 {
				t.Fatalf("partial import leaked or released caller lease: %d", live)
			}
			if external, _ := a.Usage(validation.BudgetExternalBytes); external != int64(len(firstRaw)) {
				t.Fatalf("unexpected cumulative external usage: %d", external)
			}
			if terminal == "cancellation" && secondReader.Len() != len(secondRaw) {
				t.Fatal("canceled body read")
			}
			a.Release(validation.BudgetBufferedBytes, 17)
		})
	}
}
