package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestMXLAssessmentZIPBudgetsExactAndOneUnder(t *testing.T) {
	body := "expanded bytes"
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: body}, {name: "b", body: "second"}}, nil, "")
	run := func(budget validation.Budgets) (*validation.Account, error) {
		account := assessmentZIPAccount(t, budget)
		a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
		if e == nil {
			_, e = a.readEntry(0, mxlAssessmentResource)
		}
		assessmentZIPClose(t, a, account)
		return account, e
	}
	account, e := run(validation.Budgets{})
	if e != nil {
		t.Fatal(e)
	}
	work, _ := account.Usage(validation.BudgetRuleSteps)
	cases := []struct {
		name  string
		kind  validation.BudgetKind
		limit int64
		set   func(*validation.Budgets, int64)
	}{
		{"input", validation.BudgetInputBytes, int64(len(b)), func(x *validation.Budgets, n int64) { x.MaxInputBytes = n }},
		{"external", validation.BudgetExternalBytes, int64(len(body)), func(x *validation.Budgets, n int64) { x.MaxExternalBytes = n }},
		{"work", validation.BudgetRuleSteps, work, func(x *validation.Budgets, n int64) { x.MaxRuleSteps = n }},
		{"nodes", validation.BudgetNodes, 2, func(x *validation.Budgets, n int64) { x.MaxNodes = n }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, delta := range []int64{0, -1} {
				var budgets validation.Budgets
				tc.set(&budgets, tc.limit+delta)
				_, e := run(budgets)
				if delta == 0 && e != nil {
					t.Fatalf("exact: %v", e)
				}
				if delta < 0 {
					var resource *validation.ResourceError
					if !errors.As(e, &resource) || resource.Kind != tc.kind {
						t.Fatalf("one-under %s: %v", tc.kind, e)
					}
				}
			}
		})
	}
	// Find the logical live peak, then prove the exact threshold rather than
	// comparing heap allocations or silently raising default limits.
	low, high := int64(1), int64(2<<20)
	for low < high {
		mid := low + (high-low)/2
		_, e = run(validation.Budgets{MaxBufferedBytes: mid})
		if e == nil {
			high = mid
		} else {
			var resource *validation.ResourceError
			if !errors.As(e, &resource) || resource.Kind != validation.BudgetBufferedBytes {
				t.Fatal(e)
			}
			low = mid + 1
		}
	}
	if _, e = run(validation.Budgets{MaxBufferedBytes: low}); e != nil {
		t.Fatalf("peak exact: %v", e)
	}
	if _, e = run(validation.Budgets{MaxBufferedBytes: low - 1}); e == nil {
		t.Fatal("peak one-under accepted")
	}
	t.Logf("logical peak=%d bytes, work=%d, encoded=%d, expanded=%d", low, work, len(b), len(body))
}
func TestMXLAssessmentZIPScalarCaps(t *testing.T) {
	for _, f := range []assessmentZIPFixture{{name: "longname", body: "x"}, {name: "a", body: "x", localExtra: "\xfe\xca\x04\x00abcd"}, {name: "a", body: "x", centralExtra: "\xfe\xca\x04\x00abcd"}} {
		for _, limit := range []int64{8, 7} {
			b := assessmentZIPBytes([]assessmentZIPFixture{f}, nil, "")
			account := assessmentZIPAccount(t, validation.Budgets{MaxScalarBytes: limit})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			assessmentZIPClose(t, a, account)
			if limit == 8 && e != nil {
				t.Fatal(e)
			}
			if limit == 7 {
				var resource *validation.ResourceError
				if !errors.As(e, &resource) || resource.Kind != validation.BudgetScalarBytes {
					t.Fatalf("scalar one-under: %v", e)
				}
			}
		}
	}
	for _, limit := range []int64{8, 7} {
		b := assessmentZIPBytes(nil, nil, "comment8")
		account := assessmentZIPAccount(t, validation.Budgets{MaxScalarBytes: limit})
		a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
		assessmentZIPClose(t, a, account)
		if limit == 8 && e != nil || limit == 7 && e == nil {
			t.Fatalf("comment scalar %d: %v", limit, e)
		}
	}
}
func TestMXLAssessmentZIPFiveLegacyCaps(t *testing.T) {
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "abc"}}, nil, "")
	tests := []struct {
		name  string
		role  mxlAssessmentEntryRole
		exact int64
		set   func(*MXLOptions, int64)
	}{
		{"archive", mxlAssessmentResource, int64(len(b)), func(o *MXLOptions, n int64) { o.MaxArchiveBytes = n }},
		{"metadata", mxlAssessmentMetadata, 3, func(o *MXLOptions, n int64) { o.MaxMetadataBytes = n }},
		{"document", mxlAssessmentDocument, 3, func(o *MXLOptions, n int64) { o.MaxDocumentBytes = n }},
		{"resource", mxlAssessmentResource, 3, func(o *MXLOptions, n int64) { o.MaxResourceBytes = n }},
		{"expanded-total", mxlAssessmentMetadata, 3, func(o *MXLOptions, n int64) { o.MaxResourcesBytes = n }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, delta := range []int64{0, -1} {
				account := assessmentZIPAccount(t, validation.Budgets{})
				var o MXLOptions
				tc.set(&o, tc.exact+delta)
				a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), o, account)
				if e == nil {
					_, e = a.readEntry(0, tc.role)
				}
				assessmentZIPClose(t, a, account)
				if delta == 0 && e != nil {
					t.Fatalf("exact: %v", e)
				}
				if delta < 0 && !errors.Is(e, ErrMXLTooLarge) {
					t.Fatalf("one-under: %v", e)
				}
			}
		})
	}
	b = assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: "ab"}, {name: "main", body: "cd"}}, nil, "")
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{MaxResourcesBytes: 3}, account)
	defer assessmentZIPClose(t, a, account)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.readEntry(0, mxlAssessmentMetadata); e != nil {
		t.Fatal(e)
	}
	if _, e = a.readEntry(1, mxlAssessmentDocument); !errors.Is(e, ErrMXLTooLarge) {
		t.Fatalf("metadata not included in expanded total: %v", e)
	}
}
func TestMXLAssessmentZIPAdmissionBeforeRead(t *testing.T) {
	good := assessmentZIPAccount(t, validation.Budgets{})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		o    MXLOptions
		a    *validation.Account
	}{
		{"nil-context", nil, MXLOptions{}, good}, {"canceled", canceled, MXLOptions{}, good}, {"nil-account", context.Background(), MXLOptions{}, nil}, {"zero-account", context.Background(), MXLOptions{}, &validation.Account{}},
		{"negative", context.Background(), MXLOptions{MaxArchiveBytes: -1}, good}, {"depth", context.Background(), MXLOptions{MaxXMLDepth: 4097}, good},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &assessmentZIPReader{data: []byte("do not read")}
			a, e := openMXLAssessmentArchive(tc.ctx, r, tc.o, tc.a)
			if a != nil {
				a.Close()
			}
			if e == nil || r.reads != 0 || r.closed != 0 {
				t.Fatalf("invalid admission touched caller: %v %+v", e, r)
			}
		})
	}
	for _, o := range []MXLOptions{{MaxMetadataBytes: -1}, {MaxDocumentBytes: -1}, {MaxResourceBytes: -1}, {MaxResourcesBytes: -1}, {MaxXMLDepth: -1}} {
		r := &assessmentZIPReader{data: []byte("do not read")}
		a, e := openMXLAssessmentArchive(context.Background(), r, o, good)
		if a != nil {
			a.Close()
		}
		if e == nil || r.reads != 0 {
			t.Fatalf("bad options admitted: %v", e)
		}
	}
	used, _ := good.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("configuration retained %d", used)
	}
}
func TestMXLAssessmentZIPCallerReaderOwnership(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("junk"), assessmentZIPBytes(nil, nil, "")} {
		for _, cause := range []error{nil, io.ErrUnexpectedEOF} {
			r := &assessmentZIPReader{data: data, err: cause}
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, _ := openMXLAssessmentArchive(context.Background(), r, MXLOptions{}, account)
			assessmentZIPClose(t, a, account)
			if r.closed != 0 {
				t.Fatal("caller reader closed")
			}
		}
	}
}
func TestMXLAssessmentZIPBorrowedByteOwnership(t *testing.T) {
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "abc"}}, nil, "")
	account := assessmentZIPAccount(t, validation.Budgets{})
	if e := account.Charge(validation.BudgetExternalBytes, int64(len(b))); e != nil {
		t.Fatal(e)
	}
	if e := account.Charge(validation.BudgetBufferedBytes, int64(cap(b))); e != nil {
		t.Fatal(e)
	}
	a, e := borrowMXLAssessmentArchive(context.Background(), b, MXLOptions{}, account)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.readEntry(0, mxlAssessmentResource); e != nil {
		t.Fatal(e)
	}
	a.Close()
	a.Close()
	input, _ := account.Usage(validation.BudgetInputBytes)
	external, _ := account.Usage(validation.BudgetExternalBytes)
	buffered, _ := account.Usage(validation.BudgetBufferedBytes)
	if input != 0 || external != int64(len(b)+3) || buffered != int64(cap(b)) {
		t.Fatalf("borrowed ownership changed input=%d external=%d buffer=%d", input, external, buffered)
	}
	_ = account.Release(validation.BudgetBufferedBytes, int64(cap(b)))
	assessmentZIPClose(t, a, account)
}
func TestMXLAssessmentZIPCancellationAtEveryPoll(t *testing.T) {
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: "\xef\xbb\xbf" + mxlMIMEType, localExtra: "\xfe\xca\x00\x00"}, {name: "b", body: "two"}, {name: "a", body: "three"}}, []int{2, 0, 1}, "tenbytes!!")
	full := &assessmentZIPCountingContext{}
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(full, bytes.NewReader(b), MXLOptions{}, account)
	if e == nil {
		_, e = a.readEntry(1, mxlAssessmentMetadata)
	}
	assessmentZIPClose(t, a, account)
	if e != nil {
		t.Fatal(e)
	}
	for at := 1; at <= full.polls; at++ {
		ctx := &assessmentZIPCountingContext{fail: at}
		account := assessmentZIPAccount(t, validation.Budgets{})
		a, e := openMXLAssessmentArchive(ctx, bytes.NewReader(b), MXLOptions{}, account)
		if e == nil {
			_, e = a.readEntry(1, mxlAssessmentMetadata)
		}
		assessmentZIPClose(t, a, account)
		if !errors.Is(e, context.Canceled) {
			t.Fatalf("poll %d/%d: %v", at, full.polls, e)
		}
	}
	t.Logf("cancellation checked at all %d polls", full.polls)
}
func TestMXLAssessmentZIPOverflowAndHostInt(t *testing.T) {
	for _, tc := range [][2]int64{{-1, 0}, {0, -1}, {math.MaxInt64, 0}, {0, math.MaxInt64}, {math.MaxInt64 / 2048, 1}} {
		if _, e := mxlZIPInventoryAllowance(tc[0], tc[1]); e == nil {
			t.Fatalf("unchecked measure %v", tc)
		}
	}
	if _, e := mxlZIPInventoryAllowance(0, 0); e != nil {
		t.Fatal(e)
	}
	if ^uint(0)>>63 == 0 {
		if _, e := mxlZIPInventoryAllowance(1<<22, 0); e == nil {
			t.Fatal("32-bit host overflow accepted")
		}
	}
	b := assessmentZIPBytes(nil, nil, "")
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{MaxArchiveBytes: math.MaxInt64, MaxMetadataBytes: math.MaxInt64, MaxDocumentBytes: math.MaxInt64, MaxResourceBytes: math.MaxInt64, MaxResourcesBytes: math.MaxInt64}, account)
	defer assessmentZIPClose(t, a, account)
	if e != nil {
		t.Fatalf("finite large limits overflowed: %v", e)
	}
}

type assessmentZIPCloseError struct {
	io.Reader
	closed   int
	closeErr error
}

func (r *assessmentZIPCloseError) Close() error { r.closed++; return r.closeErr }
func TestMXLAssessmentZIPOpenedStreamClosedOnce(t *testing.T) {
	readErr := errors.New("read failure")
	closeErr := errors.New("close failure")
	for _, cause := range []error{nil, readErr, context.Canceled} {
		account := assessmentZIPAccount(t, validation.Budgets{})
		a, e := newMXLAssessmentArchive(context.Background(), MXLOptions{}, account)
		if e != nil {
			t.Fatal(e)
		}
		var source io.Reader = strings.NewReader("abc")
		if cause != nil {
			source = &assessmentZIPReader{err: cause}
		}
		r := &assessmentZIPCloseError{Reader: source, closeErr: closeErr}
		meter := &mxlAssessmentExpandedReader{archive: a, body: r, limit: 100}
		_, e = a.readOpenedEntry(meter, 100)
		if r.closed != 1 || !errors.Is(e, closeErr) || cause != nil && !errors.Is(e, cause) {
			t.Fatalf("stream close=%d err=%v", r.closed, e)
		}
		assessmentZIPClose(t, a, account)
	}
}

func TestMXLAssessmentZIPCompressedWorkWithoutOutput(t *testing.T) {
	account := assessmentZIPAccount(t, validation.Budgets{MaxRuleSteps: 4})
	a, e := newMXLAssessmentArchive(context.Background(), MXLOptions{}, account)
	if e != nil {
		t.Fatal(e)
	}
	defer assessmentZIPClose(t, a, account)
	r := &mxlAssessmentCompressedReader{archive: a, raw: strings.NewReader(strings.Repeat("x", 8192))}
	var buffer [8192]byte
	for i := 0; i < 2; i++ {
		n, err := r.Read(buffer[:])
		if err != nil || n != 4096 {
			t.Fatalf("bounded compressed read %d: %d %v", i, n, err)
		}
	}
	n, e := r.Read(buffer[:])
	var resource *validation.ResourceError
	if n != 0 || !errors.As(e, &resource) || resource.Kind != validation.BudgetRuleSteps {
		t.Fatalf("compressed work bypassed quota: %d %v", n, e)
	}
}

func TestMXLAssessmentZIPEmptyDeflateBlocksBounded(t *testing.T) {
	// Empty nonfinal stored DEFLATE blocks produce no expanded bytes. Processing
	// must still poll and spend work on their compressed bytes.
	packed := bytes.Repeat([]byte{0, 0, 0, 255, 255}, 5000)
	packed = append(packed, 1, 0, 0, 255, 255)
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", method: 8, packed: packed}}, nil, "")
	// First measure only archive admission, then allow four more work units.
	probe := assessmentZIPAccount(t, validation.Budgets{})
	observed, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, probe)
	if err != nil {
		t.Fatal(err)
	}
	admission, _ := probe.Usage(validation.BudgetRuleSteps)
	assessmentZIPClose(t, observed, probe)
	account := assessmentZIPAccount(t, validation.Budgets{MaxRuleSteps: admission + 4})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.readEntry(0, mxlAssessmentResource)
	var resource *validation.ResourceError
	if !errors.As(e, &resource) || resource.Kind != validation.BudgetRuleSteps {
		t.Fatalf("empty blocks not work bounded: %v", e)
	}
	external, _ := account.Usage(validation.BudgetExternalBytes)
	if external != 0 {
		t.Fatalf("empty blocks fabricated expanded bytes: %d", external)
	}
}
