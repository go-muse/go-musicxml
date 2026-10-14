package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Every failure here originates after valid options reach the provider. None
// should look like a preflight configuration rejection to an assessment caller.
func TestDictionaryRuntimeFailuresKeepUsableReport(t *testing.T) {
	id := validation.DictionaryManifestSMuFL14Canonical
	transport, closing := errors.New("provider transport failed"), errors.New("provider close failed")
	for _, which := range []string{"open", "wrong echo", "nil body", "typed nil body", "state", "known reason", "unknown body", "empty reason", "spaced reason", "long reason", "invalid UTF-8 reason", "short", "oversize", "digest", "read", "close", "negative read count", "excessive read count"} {
		t.Run(which, func(t *testing.T) {
			var body *resourceTestBody
			var wantCause error
			opts := DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(id)}}}
			opts.Dictionaries.Source = dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
				out := DictionaryComponentResult{Request: r, State: validation.EvidenceKnown}
				raw := dictionaryImportFixture(t, id)
				var reader io.Reader = bytes.NewReader(raw)
				var openErr, closeErr error
				switch which {
				case "open":
					openErr, wantCause = transport, transport
				case "wrong echo":
					out.Request.Manifest = validation.DictionaryManifestStandardSounds40
				case "nil body":
					return out, nil
				case "typed nil body":
					out.Body = (*resourceTestBody)(nil)
					return out, nil
				case "state":
					out.State = validation.EvidenceInvalid
				case "known reason":
					out.Reason = "unexpected"
				case "unknown body":
					out.State, out.Reason = validation.EvidenceUnknown, "offline"
				case "empty reason", "spaced reason", "long reason", "invalid UTF-8 reason":
					out.State = validation.EvidenceUnknown
					out.Reason = validation.Code(map[string]string{"empty reason": "", "spaced reason": "bad reason", "long reason": strings.Repeat("x", 129), "invalid UTF-8 reason": "\xff"}[which])
					return out, nil
				case "short":
					reader = bytes.NewReader(raw[:len(raw)-1])
				case "oversize":
					reader = bytes.NewReader(append(raw, 'x'))
				case "digest":
					raw[0] ^= 1
				case "read":
					reader, wantCause = &resourceReadError{raw, transport}, transport
				case "close":
					closeErr, wantCause = closing, closing
				case "negative read count":
					reader, wantCause = dictionaryInvalidCountReader{-1, transport}, transport
				case "excessive read count":
					reader, wantCause = dictionaryInvalidCountReader{4097, transport}, transport
				}
				body = &resourceTestBody{Reader: reader, closeErr: closeErr}
				out.Body = body
				return out, openErr
			})
			report, err := InspectXMLWithDictionaries(context.Background(), strings.NewReader(`<opus/>`), opts)
			if err == nil || errors.Is(err, validation.ErrInvalidConfiguration) || !report.Requested || !report.InputComplete || !report.Satisfied() {
				t.Fatalf("report requested=%v complete=%v satisfied=%v error=%v", report.Requested, report.InputComplete, report.Satisfied(), err)
			}
			if !errors.Is(err, ErrDictionaryUnavailable) {
				t.Fatalf("missing runtime sentinel: %v", err)
			}
			var fault *DictionaryImportError
			if !errors.As(err, &fault) || fault.Manifest != id || fault.Reason == "" || fault.Err == nil {
				t.Fatalf("missing manifest/reason/cause: %+v", fault)
			}
			if wantCause != nil && !errors.Is(err, wantCause) {
				t.Fatalf("lost provider cause %v: %v", wantCause, err)
			}
			if body != nil && body.closed != 1 {
				t.Fatalf("closed %d times", body.closed)
			}
		})
	}
}

type dictionaryInvalidCountReader struct {
	n   int
	err error
}

func (r dictionaryInvalidCountReader) Read([]byte) (int, error) { return r.n, r.err }

func TestDictionaryRuntimeTerminalProviderFaultsStopImport(t *testing.T) {
	for _, terminal := range []error{context.Canceled, context.DeadlineExceeded, &validation.ResourceError{Kind: validation.BudgetRuleSteps, Limit: 1, Used: 1, Requested: 1}} {
		for _, where := range []string{"open", "read", "close", "invalid response close"} {
			t.Run(terminal.Error()+"/"+where, func(t *testing.T) {
				a := dictionaryImportAccount(t, validation.Budgets{})
				closing := errors.New("independent cleanup failure")
				var body *resourceTestBody
				calls := 0
				source := dictionaryImportSourceFunc(func(_ context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
					calls++
					var reader io.Reader = bytes.NewReader(dictionaryImportFixture(t, r.Manifest))
					body = &resourceTestBody{Reader: reader}
					out := DictionaryComponentResult{Request: r, State: validation.EvidenceKnown, Body: body}
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
				set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40), dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}, Source: source})
				if set != nil || !errors.Is(err, terminal) || !errors.Is(err, closing) || errors.Is(err, ErrDictionaryUnavailable) || calls != 1 || body.closed != 1 {
					t.Fatalf("set=%v err=%v calls=%d closes=%d", set, err, calls, body.closed)
				}
				dictionaryImportNoLive(t, a)
			})
		}
	}
}

func TestDictionaryRuntimeClassificationPreservesCoreAndPreflightErrors(t *testing.T) {
	id := validation.DictionaryManifestStandardSounds40
	pin, _ := validation.LookupDictionaryManifest(id)
	a := dictionaryImportAccount(t, validation.Budgets{})
	raw := dictionaryImportFixture(t, id)
	raw[0] ^= 1
	_, err := validation.VerifyDictionaryComponent(context.Background(), a, pin.Component.ID, raw)
	if !errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable) {
		t.Fatal(err)
	}
	dictionaryImportNoLive(t, a)
	for _, account := range []*validation.Account{nil, {}} {
		set, err := loadDictionarySet(context.Background(), account, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(id)}, Source: dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
			t.Fatal("invalid account reached provider")
			return DictionaryComponentResult{}, nil
		})})
		if set != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable) {
			t.Fatalf("set=%v error=%v", set, err)
		}
	}
}

func TestDictionaryRuntimePreservesProviderConfigurationAndTypedCauses(t *testing.T) {
	id := validation.DictionaryManifestStandardSounds40
	configuration := &validation.ConfigurationError{Field: "provider-owned configuration", Reason: "caller callback failure"}
	cause := &dictionaryProviderRuntimeCause{message: "original typed cause"}
	for _, where := range []string{"open", "read", "close"} {
		t.Run(where, func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			body := &resourceTestBody{Reader: bytes.NewReader(dictionaryImportFixture(t, id))}
			joined := errors.Join(configuration, cause)
			set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(id)}, Source: dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
				result := DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: body}
				switch where {
				case "open":
					return result, joined
				case "read":
					body.Reader = &resourceReadError{[]byte("x"), joined}
				case "close":
					body.closeErr = joined
				}
				return result, nil
			})})
			var got *dictionaryProviderRuntimeCause
			if set == nil || !errors.Is(err, ErrDictionaryUnavailable) || !errors.Is(err, configuration) || !errors.Is(err, validation.ErrInvalidConfiguration) || !errors.As(err, &got) || got != cause || body.closed != 1 {
				t.Fatalf("set=%v err=%v cause=%v closes=%d", set, err, got, body.closed)
			}
			set.Close()
			dictionaryImportNoLive(t, a)
		})
	}
}

type dictionaryProviderRuntimeCause struct{ message string }

func (e *dictionaryProviderRuntimeCause) Error() string { return e.message }

func TestDictionaryRuntimeEarlierGapDoesNotHideLaterTerminal(t *testing.T) {
	for _, terminal := range []error{context.Canceled, &validation.ResourceError{Kind: validation.BudgetRuleSteps, Limit: 1, Used: 1, Requested: 1}} {
		t.Run(terminal.Error(), func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			ordinary := errors.New("first component unavailable")
			calls := 0
			set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40), dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical), dictionaryImportSelection(validation.DictionaryManifestSMuFL14Ranges)}, Source: dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				if calls == 1 {
					return DictionaryComponentResult{}, ordinary
				}
				return DictionaryComponentResult{}, errors.Join(errors.New("second provider fault"), terminal)
			})})
			if set != nil || calls != 2 || !dictionaryImportTerminal(err) || !errors.Is(err, ordinary) || !errors.Is(err, terminal) || !errors.Is(err, ErrDictionaryUnavailable) {
				t.Fatalf("set=%v calls=%d err=%v", set, calls, err)
			}
			dictionaryImportNoLive(t, a)
		})
	}
}

func TestDictionaryRuntimeErrorMethods(t *testing.T) {
	var absent *DictionaryImportError
	if absent.Error() == "" || absent.Unwrap() != nil || absent.Is(ErrDictionaryUnavailable) {
		t.Fatal("invalid nil receiver behavior")
	}
	empty := &DictionaryImportError{}
	if empty.Error() != ErrDictionaryUnavailable.Error() || empty.Unwrap() != nil || !errors.Is(empty, ErrDictionaryUnavailable) || errors.Is(empty, validation.ErrInvalidConfiguration) {
		t.Fatal(empty)
	}
	cause := errors.New("cause")
	fault := &DictionaryImportError{Manifest: "component", Reason: "reason", Err: cause}
	if !strings.Contains(fault.Error(), "component: reason: cause") || !errors.Is(fault, cause) {
		t.Fatal(fault)
	}
}

func TestDictionaryRuntimeProviderUnavailableJoinedWithTerminal(t *testing.T) {
	for _, terminal := range []error{context.Canceled, &validation.ResourceError{Kind: validation.BudgetRuleSteps, Limit: 1, Used: 1, Requested: 1}} {
		a := dictionaryImportAccount(t, validation.Budgets{})
		calls := 0
		set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40), dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}, Source: dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
			calls++
			return DictionaryComponentResult{}, errors.Join(ErrDictionaryUnavailable, terminal)
		})})
		var addedWrapper *DictionaryImportError
		if set != nil || calls != 1 || !errors.Is(err, terminal) || !errors.Is(err, ErrDictionaryUnavailable) || errors.As(err, &addedWrapper) {
			t.Fatalf("set=%v calls=%d err=%v wrapper=%v", set, calls, err, addedWrapper)
		}
		dictionaryImportNoLive(t, a)
	}
}
