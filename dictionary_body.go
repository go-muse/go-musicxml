package musicxml

import (
	"context"
	"errors"
	"io"
	"math"
	"reflect"

	"github.com/go-muse/go-musicxml/validation"
)

// dictionaryLimitedBody preserves the transferred stream's single close while
// limiting reads to the certified component length plus one overflow byte.
type dictionaryLimitedBody struct {
	*io.LimitedReader
	io.Closer
}

// Read rejects provider-owned reader contract faults before the shared reader
// can classify them as caller configuration errors. Preserve any returned cause.
func (b *dictionaryLimitedBody) Read(p []byte) (int, error) {
	allowed := min(int64(len(p)), b.N)
	n, err := b.LimitedReader.Read(p)
	if n < 0 || int64(n) > allowed {
		return 0, errors.Join(dictionaryImportError("invalid component read count"), err)
	}
	return n, err
}

// A typed-nil provider or stream transfers no usable object. Do not invoke
// methods (including Close) on it: those methods may dereference the nil value.
func dictionaryNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// readDictionaryComponentBody owns body on entry. The existing resource reader
// accounts each actual external byte, bounded read/hash/copy work and live
// capacity. The component cap is independent of the aggregate external quota;
// it prevents an oversized component from consuming the rest of that quota.
// Matching length alone is not a certificate: callers must verify the exact
// reviewed digest before parsing or publishing any authoritative dictionary.
func readDictionaryComponentBody(ctx context.Context, a *validation.Account, body io.ReadCloser, expected int64) (*assessmentResourceBody, error) {
	bad := func(reason string) error {
		return &validation.ConfigurationError{Field: "dictionary component body", Reason: reason}
	}
	if dictionaryNil(body) {
		return nil, bad("non-nil owned stream required")
	}
	if ctx == nil || a == nil || expected <= 0 || expected == math.MaxInt64 {
		return nil, errors.Join(bad("context, root account and bounded positive component length required"), body.Close())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, body.Close())
	}
	// Admit both the LimitedReader and transferred-close wrapper before they
	// escape to the shared reader; this lease ends after its close/cleanup.
	const wrapperBytes int64 = 128
	if err := a.Charge(validation.BudgetBufferedBytes, wrapperBytes); err != nil {
		return nil, errors.Join(err, body.Close())
	}
	defer a.Release(validation.BudgetBufferedBytes, wrapperBytes)
	out, err := readAssessmentResourceBody(ctx, a, &dictionaryLimitedBody{LimitedReader: &io.LimitedReader{R: body, N: expected + 1}, Closer: body})
	if err != nil {
		return nil, err
	}
	if int64(len(out.bytes)) != expected {
		out.close()
		return nil, dictionaryImportError("component length does not match the selected pin")
	}
	return out, nil
}
