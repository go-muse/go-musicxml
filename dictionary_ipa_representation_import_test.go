package musicxml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

const dictionaryRepresentationTestID validation.DictionaryManifestID = "ipa-2015-unicode13-representation-v1"

func dictionaryRepresentationTestSelection() validation.DictionarySelection {
	return validation.DictionarySelection{Manifest: dictionaryRepresentationTestID, Relations: []validation.DictionaryRelation{"ipa-symbol-representation"}}
}

func TestDictionaryIPARepresentationFiveSelections(t *testing.T) {
	selections := []validation.DictionarySelection{
		dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical),
		dictionaryImportSelection(validation.DictionaryManifestSMuFL14Ranges),
		dictionaryImportSelection(validation.DictionaryManifestStandardSounds40),
		dictionaryImportSelection(validation.DictionaryManifestIPAPositive2015Unicode13),
		dictionaryRepresentationTestSelection(),
	}
	if err := validateDictionaryOptions(DictionaryOptions{Selections: selections}); err != nil {
		t.Fatalf("five independent fixed components must fit: %v", err)
	}
	a := dictionaryImportAccount(t, validation.Budgets{})
	set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: selections})
	if set == nil || err != nil {
		t.Fatalf("five missing capabilities should remain usable: set=%v err=%v", set, err)
	}
	if set != nil {
		set.Close()
	}
	dictionaryImportNoLive(t, a)
	selections = append(selections, dictionaryRepresentationTestSelection())
	if err := validateDictionaryOptions(DictionaryOptions{Selections: selections}); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("six selections accepted: %v", err)
	}
}

func TestDictionaryIPARepresentationMalformedResponseIsConfiguration(t *testing.T) {
	for _, which := range []string{"wrong echo", "nil body", "typed nil body", "state", "known reason", "unknown body", "empty reason", "spaced reason", "long reason", "invalid UTF-8 reason"} {
		t.Run(which, func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			closing := errors.New("independent close failure")
			reader := bytes.NewReader([]byte("unread"))
			body := &resourceTestBody{Reader: reader, closeErr: closing}
			hasBody := true
			request := DictionaryComponentRequest{Manifest: dictionaryRepresentationTestID}
			source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				out := DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}
				switch which {
				case "wrong echo":
					out.Request.Manifest = "wrong"
				case "nil body":
					out.Body, hasBody = nil, false
				case "typed nil body":
					out.Body, hasBody = (*resourceTestBody)(nil), false
				case "state":
					out.State = validation.EvidenceInvalid
				case "known reason":
					out.Reason = "contradiction"
				case "unknown body":
					out.State, out.Reason = validation.EvidenceUnknown, "offline"
				default:
					out.State, out.Body, hasBody = validation.EvidenceUnknown, nil, false
					out.Reason = validation.Code(map[string]string{"empty reason": "", "spaced reason": "bad reason", "long reason": strings.Repeat("x", 129), "invalid UTF-8 reason": "\xff"}[which])
				}
				return out, nil
			})
			out, err := importDictionaryComponent(context.Background(), a, source, request)
			var configuration *validation.ConfigurationError
			if out != nil || !errors.As(err, &configuration) || errors.Is(err, ErrDictionaryUnavailable) {
				t.Fatalf("malformed representation response became capability gap: out=%v err=%v", out, err)
			}
			if hasBody && (body.closed != 1 || !errors.Is(err, closing)) {
				t.Fatalf("close cause or ownership lost: closes=%d err=%v", body.closed, err)
			}
			if !hasBody && body.closed != 0 || reader.Len() != len("unread") {
				t.Fatalf("invalid response body used: closes=%d remaining=%d", body.closed, reader.Len())
			}
			dictionaryImportNoLive(t, a)
		})
	}
}

func TestDictionaryIPARepresentationCallbackErrorsRemainUnavailable(t *testing.T) {
	configuration := &validation.ConfigurationError{Field: "caller configuration", Reason: "transport initialization"}
	for _, where := range []string{"open", "read", "close"} {
		for _, cause := range []error{errors.New("transport unavailable"), configuration, dictionaryImportError("component length does not match the selected pin")} {
			t.Run(where+"/"+cause.Error(), func(t *testing.T) {
				a := dictionaryImportAccount(t, validation.Budgets{})
				body := &resourceTestBody{Reader: bytes.NewReader([]byte("x"))}
				request := DictionaryComponentRequest{Manifest: dictionaryRepresentationTestID, Component: validation.DictionaryComponentPin{Bytes: 1}}
				source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
					out := DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}
					switch where {
					case "open":
						return out, cause
					case "read":
						body.Reader = &resourceReadError{[]byte("x"), cause}
					case "close":
						body.closeErr = cause
					}
					return out, nil
				})
				out, err := importDictionaryComponent(context.Background(), a, source, request)
				if out != nil || !errors.Is(err, cause) || !errors.Is(err, ErrDictionaryUnavailable) || body.closed != 1 {
					t.Fatalf("provider cause reclassified or lost: out=%v err=%v closes=%d", out, err, body.closed)
				}
				dictionaryImportNoLive(t, a)
			})
		}
	}
}

func TestDictionaryIPARepresentationMissingSourceAndExplicitGap(t *testing.T) {
	for _, source := range []DictionarySource{nil, (*dictionaryImportNilSource)(nil), dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
		return DictionaryComponentResult{Request: request, State: validation.EvidenceUnknown, Reason: "offline"}, nil
	})} {
		a := dictionaryImportAccount(t, validation.Budgets{})
		set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: source})
		if set == nil || err != nil {
			t.Fatalf("missing representation became an invalid configuration: set=%v err=%v", set, err)
		}
		if set != nil {
			set.Close()
		}
		dictionaryImportNoLive(t, a)
	}
}

func TestDictionaryIPARepresentationDecoderLengthIsConfiguration(t *testing.T) {
	a := dictionaryImportAccount(t, validation.Budgets{})
	out, err := decodeDictionaryComponent(context.Background(), a, dictionaryRepresentationTestID, []byte("{}"))
	if out != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable) {
		t.Fatalf("out=%v err=%v", out, err)
	}
	dictionaryImportNoLive(t, a)
}

func TestDictionaryIPARepresentationTransportGapKeepsUsableSet(t *testing.T) {
	for _, where := range []string{"open", "read", "close"} {
		t.Run(where, func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			cause := &validation.ConfigurationError{Field: "caller transport", Reason: "offline"}
			var body *resourceTestBody
			source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				body = &resourceTestBody{Reader: bytes.NewReader([]byte("x"))}
				out := DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}
				switch where {
				case "open":
					return out, cause
				case "read":
					body.Reader = &resourceReadError{[]byte("x"), cause}
				case "close":
					body.closeErr = cause
				}
				return out, nil
			})
			set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: source})
			if set == nil || !errors.Is(err, cause) || !errors.Is(err, ErrDictionaryUnavailable) || body == nil || body.closed != 1 {
				t.Fatalf("set=%v err=%v body=%v", set, err, body)
			}
			if set != nil {
				set.Close()
			}
			dictionaryImportNoLive(t, a)
		})
	}
}

func TestDictionaryIPARepresentationLengthMismatchIsConfiguration(t *testing.T) {
	for _, size := range []int{0, 1, 3, 10000} {
		a := dictionaryImportAccount(t, validation.Budgets{})
		body := &resourceTestBody{Reader: bytes.NewReader(make([]byte, size))}
		request := DictionaryComponentRequest{Manifest: dictionaryRepresentationTestID, Component: validation.DictionaryComponentPin{Bytes: 2}}
		source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
			return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}, nil
		})
		out, err := importDictionaryComponent(context.Background(), a, source, request)
		if out != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable) || body.closed != 1 {
			t.Fatalf("size=%d out=%v err=%v closes=%d", size, out, err, body.closed)
		}
		if external, _ := a.Usage(validation.BudgetExternalBytes); external != int64(min(size, 3)) {
			t.Fatalf("size=%d external bytes=%d", size, external)
		}
		dictionaryImportNoLive(t, a)
	}
}

func TestDictionaryIPARepresentationMalformedImportStopsLaterProviders(t *testing.T) {
	for _, earlierGap := range []bool{false, true} {
		a := dictionaryImportAccount(t, validation.Budgets{})
		selections := []validation.DictionarySelection{dictionaryRepresentationTestSelection(), dictionaryImportSelection(validation.DictionaryManifestSMuFL14Ranges)}
		if earlierGap {
			selections = append([]validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40)}, selections...)
		}
		transport := errors.New("earlier component offline")
		calls := 0
		source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
			calls++
			if r.Manifest == validation.DictionaryManifestStandardSounds40 {
				return DictionaryComponentResult{}, transport
			}
			return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown}, nil
		})
		set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: selections, Source: source})
		wantCalls := 1
		if earlierGap {
			wantCalls++
		}
		if set != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || calls != wantCalls || errors.Is(err, ErrDictionaryUnavailable) != earlierGap || errors.Is(err, transport) != earlierGap {
			t.Fatalf("earlier gap=%v set=%v calls=%d err=%v", earlierGap, set, calls, err)
		}
		dictionaryImportNoLive(t, a)
	}
}

func dictionaryRepresentationFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/dictionaries/ipa-representation/runtime-representation-bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDictionaryIPARepresentationCertifiedComponent(t *testing.T) {
	raw := dictionaryRepresentationFixture(t)
	a := dictionaryImportAccount(t, validation.Budgets{})
	parsed, err := decodeDictionaryComponent(context.Background(), a, dictionaryRepresentationTestID, raw)
	if err != nil || parsed == nil {
		t.Fatalf("certified representation decode: parsed=%v err=%v", parsed, err)
	}
	if parsed.data.IPA == nil || parsed.data.SMuFL != nil || parsed.data.Sounds != nil || len(parsed.data.IPA.Symbols) != 295 || len(parsed.data.IPA.Examples) != 0 || len(parsed.data.IPA.Equivalences) != 158 {
		t.Fatalf("unexpected representation branches or counts: %+v", parsed.data)
	}
	parsed.close()
	dictionaryImportNoLive(t, a)
	calls := 0
	body := &resourceTestBody{Reader: bytes.NewReader(raw)}
	source := dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		manifest, ok := validation.LookupDictionaryManifest(dictionaryRepresentationTestID)
		if !ok || request.Manifest != dictionaryRepresentationTestID || request.Component != manifest.Component {
			t.Fatalf("unexpected component request: %+v", request)
		}
		return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: body}, nil
	})
	set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: source})
	if err != nil || set == nil || calls != 1 || body.closed != 1 {
		t.Fatalf("certified representation import: set=%v calls=%d closes=%d err=%v", set, calls, body.closed, err)
	}
	set.Close()
	dictionaryImportNoLive(t, a)
	if external, _ := a.Usage(validation.BudgetExternalBytes); external != int64(len(raw)) {
		t.Fatalf("external bytes=%d want=%d", external, len(raw))
	}
}

func TestDictionaryIPARepresentationDigestMismatchIsConfiguration(t *testing.T) {
	raw := dictionaryRepresentationFixture(t)
	raw[0] ^= 1
	a := dictionaryImportAccount(t, validation.Budgets{})
	body := &resourceTestBody{Reader: bytes.NewReader(raw)}
	source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
		return DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}, nil
	})
	set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: source})
	if set != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable) || body.closed != 1 {
		t.Fatalf("set=%v err=%v closes=%d", set, err, body.closed)
	}
	dictionaryImportNoLive(t, a)
}

func TestDictionaryIPARepresentationMalformedCertifiedRecords(t *testing.T) {
	raw := dictionaryRepresentationFixture(t)
	opener := []byte(`"records": [`)
	at := bytes.Index(raw, opener)
	if at < 0 {
		t.Fatal("missing records opener")
	}
	prefix := raw[:at+len(opener)]
	for _, tc := range []struct {
		name, records, reason string
	}{
		{"duplicate field", `{"record_type":"symbol","record_type":"symbol","fields":["a","letter","A"]}`, "duplicate corpus field"},
		{"unknown field", `{"record_type":"symbol","other":1,"fields":["a","letter","A"]}`, "unknown IPA representation record field"},
		{"unknown record", `{"record_type":"example-sequence","fields":["a","letter","A"]}`, "unsupported IPA representation record kind"},
		{"missing field", `{"record_type":"symbol"}`, "incomplete IPA representation record"},
		{"extra string", `{"record_type":"symbol","fields":["a","letter","A","extra"]}`, "unexpected certified corpus token"},
		{"empty encoding", `{"record_type":"symbol","fields":["","letter","A"]}`, "empty IPA representation encoding"},
		{"duplicate encoding", `{"record_type":"symbol","fields":["a","letter","A"]},{"record_type":"symbol","fields":["a","letter","B"]}`, "duplicate or out-of-order IPA representation record"},
		{"out of order", `{"record_type":"symbol","fields":["b","letter","B"]},{"record_type":"symbol","fields":["a","letter","A"]}`, "duplicate or out-of-order IPA representation record"},
		{"bad escape", `{"record_type":"symbol","fields":["\uD800","letter","A"]}`, "unpaired JSON surrogate"},
		{"invalid UTF-8", "{\"record_type\":\"symbol\",\"fields\":[\"\xff\",\"letter\",\"A\"]}", "invalid UTF-8 corpus string"},
		{"missing rows", ``, "incomplete IPA representation record counts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutant := append(append([]byte(nil), prefix...), tc.records...)
			mutant = append(mutant, ']', '}')
			if len(mutant) > len(raw) {
				t.Fatal("test mutation exceeds exact component length")
			}
			mutant = append(mutant, bytes.Repeat([]byte(" "), len(raw)-len(mutant))...)
			a := dictionaryImportAccount(t, validation.Budgets{})
			if err := a.Charge(validation.BudgetBufferedBytes, 17); err != nil {
				t.Fatal(err)
			}
			parsed, err := decodeDictionaryComponent(context.Background(), a, dictionaryRepresentationTestID, mutant)
			if parsed != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable) || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("parsed=%v err=%v; want %q", parsed, err, tc.reason)
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 17 {
				t.Fatalf("decoder lost caller lease or leaked temporary rows: %d", live)
			}
			if err := a.Release(validation.BudgetBufferedBytes, 17); err != nil {
				t.Fatal(err)
			}
			dictionaryImportNoLive(t, a)
		})
	}
}

func TestDictionaryIPARepresentationAllFiveCertifiedImports(t *testing.T) {
	ids := []validation.DictionaryManifestID{validation.DictionaryManifestSMuFL14Canonical, validation.DictionaryManifestSMuFL14Ranges, validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestIPAPositive2015Unicode13, dictionaryRepresentationTestID}
	selections := make([]validation.DictionarySelection, 0, len(ids))
	for _, id := range ids {
		if id == dictionaryRepresentationTestID {
			selections = append(selections, dictionaryRepresentationTestSelection())
		} else {
			selections = append(selections, dictionaryImportSelection(id))
		}
	}
	calls := make(map[validation.DictionaryManifestID]int)
	var wantBytes int64
	source := dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls[request.Manifest]++
		var raw []byte
		if request.Manifest == dictionaryRepresentationTestID {
			raw = dictionaryRepresentationFixture(t)
		} else {
			raw = dictionaryImportFixture(t, request.Manifest)
		}
		wantBytes += int64(len(raw))
		return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: &resourceTestBody{Reader: bytes.NewReader(raw)}}, nil
	})
	a := dictionaryImportAccount(t, validation.Budgets{})
	set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: selections, Source: source})
	if set == nil || err != nil {
		t.Fatalf("all-five import: set=%v err=%v", set, err)
	}
	for _, id := range ids {
		if calls[id] != 1 {
			t.Fatalf("manifest %s opened %d times", id, calls[id])
		}
	}
	if external, _ := a.Usage(validation.BudgetExternalBytes); external != wantBytes {
		t.Fatalf("external bytes=%d want=%d", external, wantBytes)
	}
	set.Close()
	dictionaryImportNoLive(t, a)
}

func TestDictionaryIPARepresentationTerminalImportFailures(t *testing.T) {
	raw := dictionaryRepresentationFixture(t)
	for _, terminal := range []error{context.Canceled, context.DeadlineExceeded, &validation.ResourceError{Kind: validation.BudgetRuleSteps, Limit: 1, Used: 1, Requested: 1}} {
		for _, where := range []string{"open", "read", "close", "invalid response close"} {
			t.Run(where+"/"+terminal.Error(), func(t *testing.T) {
				a := dictionaryImportAccount(t, validation.Budgets{})
				closing := errors.New("independent cleanup cause")
				body := &resourceTestBody{Reader: bytes.NewReader(raw)}
				source := dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
					out := DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: body}
					switch where {
					case "open":
						body.closeErr = closing
						return out, terminal
					case "read":
						body.Reader = &resourceReadError{[]byte("x"), terminal}
						body.closeErr = closing
					case "close":
						body.closeErr = errors.Join(terminal, closing)
					case "invalid response close":
						out.State = validation.EvidenceInvalid
						body.closeErr = errors.Join(terminal, closing)
					}
					return out, nil
				})
				set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: source})
				if set != nil || !errors.Is(err, terminal) || !errors.Is(err, closing) || errors.Is(err, ErrDictionaryUnavailable) || body.closed != 1 {
					t.Fatalf("set=%v err=%v closes=%d", set, err, body.closed)
				}
				dictionaryImportNoLive(t, a)
			})
		}
	}
}

type dictionaryRepresentationCancelOnCloseBody struct {
	*bytes.Reader
	cancel   context.CancelFunc
	closed   int
	closeErr error
}

func (b *dictionaryRepresentationCancelOnCloseBody) Close() error {
	b.closed++
	b.cancel()
	return b.closeErr
}

func TestDictionaryIPARepresentationLengthMismatchPreservesCloseCancellation(t *testing.T) {
	for _, id := range []validation.DictionaryManifestID{validation.DictionaryManifestIPAPositive2015Unicode13, dictionaryRepresentationTestID} {
		for _, length := range []string{"short", "long"} {
			t.Run(string(id)+"/"+length, func(t *testing.T) {
				var raw []byte
				if id == dictionaryRepresentationTestID {
					raw = dictionaryRepresentationFixture(t)
				} else {
					raw = dictionaryImportFixture(t, id)
				}
				if length == "short" {
					raw = raw[:len(raw)-1]
				} else {
					raw = append(raw, 'x')
				}
				a := dictionaryImportAccount(t, validation.Budgets{})
				if err := a.Charge(validation.BudgetBufferedBytes, 17); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				body := &dictionaryRepresentationCancelOnCloseBody{Reader: bytes.NewReader(raw), cancel: cancel}
				calls := 0
				source := dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
					calls++
					return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: body}, nil
				})
				set, err := loadDictionarySet(ctx, a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(id), dictionaryImportSelection(validation.DictionaryManifestStandardSounds40)}, Source: source})
				if set != nil || !errors.Is(err, context.Canceled) || body.closed != 1 || calls != 1 {
					t.Fatalf("close cancellation lost: set=%v err=%v closes=%d calls=%d", set, err, body.closed, calls)
				}
				if id == dictionaryRepresentationTestID && (!errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable)) {
					t.Fatalf("malformed representation classification changed: %v", err)
				}
				if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 17 {
					t.Fatalf("lost caller lease or leaked failed import: %d", live)
				}
				if err := a.Release(validation.BudgetBufferedBytes, 17); err != nil {
					t.Fatal(err)
				}
				dictionaryImportNoLive(t, a)
			})
		}
	}
}

func TestDictionaryIPARepresentationCloseCancellationStopsDocumentReads(t *testing.T) {
	for _, length := range []string{"exact", "short", "long"} {
		for _, closeFailure := range []bool{false, true} {
			for _, earlierGap := range []bool{false, true} {
				name := fmt.Sprintf("%s/close-error=%v/earlier-gap=%v", length, closeFailure, earlierGap)
				t.Run(name, func(t *testing.T) {
					raw := dictionaryRepresentationFixture(t)
					if length == "short" {
						raw = raw[:len(raw)-1]
					} else if length == "long" {
						raw = append(raw, 'x')
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					closing := errors.New("caller close failure")
					ordinary := errors.New("earlier component unavailable")
					body := &dictionaryRepresentationCancelOnCloseBody{Reader: bytes.NewReader(raw), cancel: cancel}
					if closeFailure {
						body.closeErr = closing
					}
					selections := []validation.DictionarySelection{dictionaryRepresentationTestSelection()}
					if earlierGap {
						selections = append([]validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}, selections...)
					}
					calls := 0
					source := dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
						calls++
						if request.Manifest == validation.DictionaryManifestSMuFL14Canonical {
							return DictionaryComponentResult{}, ordinary
						}
						return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: body}, nil
					})
					report, err := inspectXMLWithDictionaries(ctx, &dictionaryUnexpectedRead{t}, dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: DictionaryOptions{Selections: selections, Source: source}})
					wantCalls := 1
					if earlierGap {
						wantCalls++
					}
					wantConfiguration := length != "exact" && !closeFailure
					if !errors.Is(err, context.Canceled) || errors.Is(err, validation.ErrInvalidConfiguration) != wantConfiguration || errors.Is(err, ordinary) != earlierGap || errors.Is(err, closing) != closeFailure || body.closed != 1 || calls != wantCalls || report.InputComplete {
						t.Fatalf("cancellation or prior cause lost: err=%v closes=%d calls=%d complete=%v", err, body.closed, calls, report.InputComplete)
					}
				})
			}
		}
	}
}
