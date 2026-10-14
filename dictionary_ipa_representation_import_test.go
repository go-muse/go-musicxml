package musicxml

import (
	"bytes"
	"context"
	"errors"
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
