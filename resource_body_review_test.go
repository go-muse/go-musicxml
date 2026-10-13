package musicxml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"testing"
)

func TestResourceBodyReviewedChunkWork(t *testing.T) {
	for _, tc := range []struct {
		size  int
		steps int64
	}{{0, 1}, {1, 2}, {4096, 2}, {4097, 4}, {8193, 7}, {65537, 49}} {
		t.Run(fmt.Sprint(tc.size), func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{})
			body := &resourceTestBody{Reader: bytes.NewReader(make([]byte, tc.size))}
			out, err := readAssessmentResourceBody(context.Background(), a, body)
			if err != nil {
				t.Fatal(err)
			}
			defer out.close()
			steps, _ := a.Usage(validation.BudgetRuleSteps)
			external, _ := a.Usage(validation.BudgetExternalBytes)
			if steps != tc.steps || external != int64(tc.size) || body.closed != 1 {
				t.Fatalf("steps=%d want=%d bytes=%d close=%d", steps, tc.steps, external, body.closed)
			}
			out.close()
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
				t.Fatalf("live=%d", live)
			}
		})
	}
}

func TestResourceBodyReviewedExactWorkAdmission(t *testing.T) {
	for _, limit := range []int64{4, 6, 7} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{MaxRuleSteps: limit})
			body := &resourceTestBody{Reader: bytes.NewReader(make([]byte, 8193))}
			out, err := readAssessmentResourceBody(context.Background(), a, body)
			if limit == 7 && err != nil {
				t.Fatal(err)
			}
			if limit < 7 && !errors.Is(err, validation.ErrResourceLimit) {
				t.Fatalf("err=%v", err)
			}
			if out != nil {
				out.close()
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 || body.closed != 1 {
				t.Fatalf("live=%d close=%d", live, body.closed)
			}
		})
	}
}

type resourceWorkCancelContext struct {
	context.Context
	account *validation.Account
	at      int64
}

func (c resourceWorkCancelContext) Err() error {
	n, _ := c.account.Usage(validation.BudgetRuleSteps)
	if n >= c.at {
		return context.Canceled
	}
	return nil
}
func TestResourceBodyReviewedCancellationDuringRelocation(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	ctx := resourceWorkCancelContext{context.Background(), a, 5}
	body := &resourceTestBody{Reader: bytes.NewReader(make([]byte, 8193))}
	out, err := readAssessmentResourceBody(ctx, a, body)
	if out != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 || body.closed != 1 {
		t.Fatalf("live=%d close=%d", live, body.closed)
	}
}

func TestResourceBodyChunkEndNativeBoundary(t *testing.T) {
	max := int(^uint(0) >> 1)
	for _, at := range []int{max - 8192, max - 4096, max - 4095, max - 1, max} {
		end := resourceBodyChunkEnd(at, max)
		if end < at || end > max || end-at > 4096 || end != at+min(4096, max-at) {
			t.Fatalf("at=%d end=%d", at, end)
		}
	}
}

type resourceBodyZeroReader struct{}

func (resourceBodyZeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
func TestResourceBodyDocumentedLiveCapacityLimit(t *testing.T) {
	for _, size := range []int64{10 << 20, 33 << 20} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{})
			body := &resourceTestBody{Reader: io.LimitReader(resourceBodyZeroReader{}, size)}
			out, err := readAssessmentResourceBody(context.Background(), a, body)
			if size == 10<<20 {
				if err != nil {
					t.Fatal(err)
				}
				if out.retained != (16<<20)+256 {
					t.Fatalf("retained=%d", out.retained)
				}
				out.close()
			} else {
				var e *validation.ResourceError
				if out != nil || !errors.As(err, &e) || e.Kind != validation.BudgetBufferedBytes || e.Limit != 64<<20 || e.Used != (32<<20)+4608 || e.Requested != 64<<20 {
					t.Fatalf("out=%v err=%v detail=%+v", out, err, e)
				}
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 || body.closed != 1 {
				t.Fatalf("live=%d close=%d", live, body.closed)
			}
		})
	}
}
