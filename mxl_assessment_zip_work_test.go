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

func TestZIPReviewBulkWorkUsesBoundedUnits(t *testing.T) {
	for _, method := range []uint16{0, 8} {
		data := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: strings.Repeat("x", 12<<20), method: method}}, nil, "")
		account := assessmentZIPAccount(t, validation.Budgets{})
		a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(data), MXLOptions{}, account)
		if err == nil {
			var body []byte
			body, err = a.readEntry(0, mxlAssessmentPrimaryDocument)
			if err == nil && len(body) != 12<<20 {
				t.Errorf("method %d: body size %d", method, len(body))
			}
		}
		work, _ := account.Usage(validation.BudgetRuleSteps)
		external, _ := account.Usage(validation.BudgetExternalBytes)
		assessmentZIPClose(t, a, account)
		if err != nil || external != 12<<20 || work > 50000 {
			t.Errorf("method %d: work=%d expanded=%d error=%v", method, work, external, err)
		}
	}
}

func TestZIPReviewReadRelocationUnitsAndPeak(t *testing.T) {
	const size = 9000
	// Three nonempty 4096-byte-or-smaller reads, one terminal probe, three
	// appends, plus 4096 and 8192 bytes relocated as one and two bounded copies.
	const work = 4 + 3 + 1 + 2
	// Third growth keeps the old 8192-byte backing array live with the new one.
	const peak = mxlZIPOwnerBytes + mxlZIPReadScratch + 8192 + size
	for _, tc := range []struct {
		name          string
		steps, buffer int64
		want          validation.BudgetKind
	}{
		{"exact", work, peak, ""}, {"one-under-work", work - 1, peak, validation.BudgetRuleSteps},
		{"one-under-peak", work, peak - 1, validation.BudgetBufferedBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := assessmentZIPAccount(t, validation.Budgets{MaxRuleSteps: tc.steps, MaxBufferedBytes: tc.buffer})
			a, err := newMXLAssessmentArchive(context.Background(), MXLOptions{}, account)
			if err != nil {
				t.Fatal(err)
			}
			defer assessmentZIPClose(t, a, account)
			got, err := a.readBounded(strings.NewReader(strings.Repeat("x", size)), size, validation.BudgetInputBytes, false)
			used, _ := account.Usage(validation.BudgetRuleSteps)
			if tc.want == "" {
				if err != nil || len(got) != size || used != work {
					t.Fatalf("work=%d bytes=%d err=%v", used, len(got), err)
				}
			} else {
				var resource *validation.ResourceError
				if !errors.As(err, &resource) || resource.Kind != tc.want {
					t.Fatalf("want %s, got %v", tc.want, err)
				}
			}
		})
	}
}

type reviewZIPReadSizes struct {
	data    []byte
	largest int
	empty   bool
	reads   int
}

func (r *reviewZIPReadSizes) Read(p []byte) (int, error) {
	r.reads++
	r.largest = max(r.largest, len(p))
	if r.empty {
		return 0, nil
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}
func (r *reviewZIPReadSizes) Close() error { return nil }

func TestZIPReviewExpandedReadsAreBounded(t *testing.T) {
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, err := newMXLAssessmentArchive(context.Background(), MXLOptions{}, account)
	if err != nil {
		t.Fatal(err)
	}
	defer assessmentZIPClose(t, a, account)
	body := &reviewZIPReadSizes{data: bytes.Repeat([]byte("x"), 8192)}
	r := &mxlAssessmentExpandedReader{archive: a, body: body, limit: 8192}
	var out [8192]byte
	n, err := r.Read(out[:])
	if err != nil || n != 4096 || body.largest != 4096 {
		t.Fatalf("decompressor read not bounded: %d max=%d %v", n, body.largest, err)
	}
	before, _ := account.Usage(validation.BudgetRuleSteps)
	if n, err = r.Read(nil); n != 0 || err != nil {
		t.Fatal(n, err)
	}
	after, _ := account.Usage(validation.BudgetRuleSteps)
	if after != before+1 {
		t.Fatalf("empty call uncharged: %d -> %d", before, after)
	}
}

func TestZIPReviewEmptyReadsAndProbesConsumeWork(t *testing.T) {
	for _, empty := range []bool{false, true} {
		account := assessmentZIPAccount(t, validation.Budgets{MaxRuleSteps: 3})
		a, err := newMXLAssessmentArchive(context.Background(), MXLOptions{}, account)
		if err != nil {
			t.Fatal(err)
		}
		source := &reviewZIPReadSizes{empty: empty}
		_, err = a.readBounded(source, 1, validation.BudgetInputBytes, false)
		work, _ := account.Usage(validation.BudgetRuleSteps)
		input, _ := account.Usage(validation.BudgetInputBytes)
		assessmentZIPClose(t, a, account)
		if empty {
			var resource *validation.ResourceError
			if !errors.As(err, &resource) || resource.Kind != validation.BudgetRuleSteps || work != 3 || source.reads != 3 {
				t.Fatalf("empty reads work=%d calls=%d: %v", work, source.reads, err)
			}
		} else if err != nil || work != 1 || source.reads != 1 {
			t.Fatalf("EOF probe work=%d calls=%d: %v", work, source.reads, err)
		}
		if input != 0 {
			t.Fatal("empty read charged bytes", input)
		}
	}
}

func TestZIPReviewDefaultMidSizedArchive(t *testing.T) {
	data := packageGraphBytes(`<opus/>`, "main.opus", "", assessmentZIPFixture{name: "pad.bin", body: strings.Repeat("x", 12<<20)})
	bundle, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
	if err != nil || !bundle.EnumerationComplete() || !bundle.PackageReport().InputComplete || bundle.ResourceCount() != 2 {
		t.Fatalf("default archive failed transport/enumeration: %v", err)
	}
}

func TestZIPReviewRelocationFailureKeepsOldOwner(t *testing.T) {
	account := assessmentZIPAccount(t, validation.Budgets{MaxRuleSteps: 8})
	a, err := newMXLAssessmentArchive(context.Background(), MXLOptions{}, account)
	if err != nil {
		t.Fatal(err)
	}
	defer assessmentZIPClose(t, a, account)
	data, err := a.readBounded(strings.NewReader(strings.Repeat("x", 9000)), 9000, validation.BudgetInputBytes, false)
	var resource *validation.ResourceError
	if !errors.As(err, &resource) || resource.Kind != validation.BudgetRuleSteps || len(data) != 8192 || cap(data) != 8192 {
		t.Fatalf("interrupted relocation lost old owner: len=%d cap=%d %v", len(data), cap(data), err)
	}
	live, _ := account.Usage(validation.BudgetBufferedBytes)
	input, _ := account.Usage(validation.BudgetInputBytes)
	if live != mxlZIPOwnerBytes+8192 || input != 9000 {
		t.Fatalf("failure ledgers: buffered=%d input=%d", live, input)
	}
}
