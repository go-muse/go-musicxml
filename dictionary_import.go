package musicxml

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

// dictionaryImported owns temporary decoded rows until NewDictionarySet finishes
// its independently admitted copy. Raw bytes are separately owned and released.
type dictionaryImported struct {
	data     validation.DictionaryImport
	account  *validation.Account
	retained int64
}

func (d *dictionaryImported) close() {
	if d != nil && d.account != nil {
		_ = d.account.Release(validation.BudgetBufferedBytes, d.retained)
		d.data = validation.DictionaryImport{}
		d.retained = 0
		d.account = nil
	}
}
func dictionaryImportError(reason string) error {
	return &validation.ConfigurationError{Field: "dictionary import", Reason: reason}
}
func dictionaryImportTerminal(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, validation.ErrResourceLimit)
}
func dictionaryImportStep(ctx context.Context, a *validation.Account, n int64) error {
	if ctx == nil {
		return dictionaryImportError("non-nil context required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Charge(validation.BudgetRuleSteps, n)
}

// loadDictionarySet preserves ordinary component failures as unavailable facts
// while returning their errors alongside a usable set. Only invalid selection,
// invalid account/context or terminal resource/cancellation failure stops import.
func loadDictionarySet(ctx context.Context, a *validation.Account, o DictionaryOptions) (*validation.DictionarySet, error) {
	if err := validateDictionaryOptions(o); err != nil {
		return nil, err
	}
	if err := dictionaryImportStep(ctx, a, 1); err != nil {
		return nil, err
	}
	if len(o.Selections) == 0 {
		return validation.NewDictionarySet(ctx, a, nil, nil)
	}
	// At most four metadata values, selection copies, 12 relation string headers,
	// imports/certificates, cleanup records and bounded error-join bookkeeping.
	// 8192 is a fixed logical envelope, independent of provider-controlled text.
	const controlBytes int64 = 8192
	if err := a.Charge(validation.BudgetBufferedBytes, controlBytes); err != nil {
		return nil, err
	}
	defer a.Release(validation.BudgetBufferedBytes, controlBytes)
	selections := make([]validation.DictionarySelection, len(o.Selections))
	for i, s := range o.Selections {
		selections[i] = validation.DictionarySelection{Manifest: s.Manifest, Relations: append([]validation.DictionaryRelation(nil), s.Relations...)}
	}
	if dictionaryNil(o.Source) {
		return validation.NewDictionarySet(ctx, a, selections, nil)
	}
	imports := make([]validation.DictionaryImport, 0, len(selections))
	owned := make([]*dictionaryImported, 0, len(selections))
	defer func() {
		for _, d := range owned {
			d.close()
		}
	}()
	var ordinary error
	for _, selection := range selections {
		if err := dictionaryImportStep(ctx, a, 1); err != nil {
			return nil, errors.Join(ordinary, err)
		}
		manifest, _ := validation.LookupDictionaryManifest(selection.Manifest)
		request := DictionaryComponentRequest{Manifest: selection.Manifest, Component: manifest.Component}
		parsed, err := importDictionaryComponent(ctx, a, o.Source, request)
		if err != nil {
			if dictionaryImportTerminal(err) {
				return nil, errors.Join(ordinary, err)
			}
			ordinary = errors.Join(ordinary, err)
			imports = append(imports, validation.DictionaryImport{Manifest: selection.Manifest, State: validation.EvidenceUnknown, Reason: "dictionary-import-unavailable"})
			continue
		}
		owned = append(owned, parsed)
		imports = append(imports, parsed.data)
	}
	set, err := validation.NewDictionarySet(ctx, a, selections, imports)
	return set, errors.Join(ordinary, err)
}

func importDictionaryComponent(ctx context.Context, a *validation.Account, source DictionarySource, request DictionaryComponentRequest) (*dictionaryImported, error) {
	result, err := source.Open(ctx, request)
	closeResult := func(cause error) error {
		if !dictionaryNil(result.Body) {
			cause = errors.Join(cause, result.Body.Close())
			result.Body = nil
		}
		return errors.Join(cause, ctx.Err())
	}
	if err != nil {
		return nil, closeResult(err)
	}
	if err = ctx.Err(); err != nil {
		return nil, closeResult(err)
	}
	if result.Request != request {
		return nil, closeResult(dictionaryImportError("component response must echo the exact request"))
	}
	switch result.State {
	case validation.EvidenceUnknown, validation.EvidenceUnsupported:
		reason := string(result.Reason)
		if !dictionaryNil(result.Body) || len(reason) == 0 || len(reason) > 128 || !utf8.ValidString(reason) || strings.IndexFunc(reason, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return nil, closeResult(dictionaryImportError("unavailable component requires a bounded reason and no body"))
		}
		// Copy provider text; the control envelope admits this <=128-byte value.
		return &dictionaryImported{data: validation.DictionaryImport{Manifest: request.Manifest, State: result.State, Reason: validation.Code(strings.Clone(reason))}}, nil
	case validation.EvidenceKnown:
		if result.Reason != "" || dictionaryNil(result.Body) {
			return nil, closeResult(dictionaryImportError("known component requires a usable body and no reason"))
		}
	default:
		return nil, closeResult(dictionaryImportError("unsupported component evidence state"))
	}
	body, err := readDictionaryComponentBody(ctx, a, result.Body, request.Component.Bytes)
	result.Body = nil
	if err != nil {
		return nil, err
	}
	defer body.close()
	certificate, err := validation.VerifyDictionaryComponent(ctx, a, request.Component.ID, body.bytes)
	if err != nil {
		return nil, err
	}
	parsed, err := decodeDictionaryComponent(ctx, a, request.Manifest, body.bytes)
	if err != nil {
		return nil, err
	}
	parsed.data.Certificate = &certificate
	return parsed, nil
}
