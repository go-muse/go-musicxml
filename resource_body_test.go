package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"testing"
)

type resourceTestBody struct {
	io.Reader
	closed   int
	closeErr error
}

func (r *resourceTestBody) Close() error { r.closed++; return r.closeErr }

type resourceReadError struct {
	data []byte
	err  error
}

func (r *resourceReadError) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, r.err
}
func TestResourceBodyOwnedVerifiedAndClosed(t *testing.T) {
	data := bytes.Repeat([]byte("<opus/>"), 10000)
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	b := &resourceTestBody{Reader: bytes.NewReader(data)}
	out, err := readAssessmentResourceBody(context.Background(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	if b.closed != 1 || !bytes.Equal(out.bytes, data) || out.digest != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal("body/identity/close mismatch")
	}
	used, _ := a.Usage(validation.BudgetExternalBytes)
	if used != int64(len(data)) {
		t.Fatal(used)
	}
	live, _ := a.Usage(validation.BudgetBufferedBytes)
	if live < int64(len(data)) {
		t.Fatal(live)
	}
	out.close()
	out.close()
	live, _ = a.Usage(validation.BudgetBufferedBytes)
	if live != 0 {
		t.Fatal(live)
	}
}
func TestResourceBodyErrorOwnershipAndQuotas(t *testing.T) {
	boom := errors.New("transport")
	closeBoom := errors.New("close")
	for _, test := range []struct {
		name     string
		reader   io.Reader
		budget   validation.Budgets
		want     error
		canceled bool
	}{
		{"transport", &resourceReadError{[]byte("a"), boom}, validation.Budgets{}, boom, false},
		{"joined EOF", &resourceReadError{[]byte("a"), errors.Join(io.EOF, boom)}, validation.Budgets{}, boom, false},
		{"external cap", bytes.NewReader([]byte("ab")), validation.Budgets{MaxExternalBytes: 1}, validation.ErrResourceLimit, false},
		{"buffer cap", bytes.NewReader([]byte("ab")), validation.Budgets{MaxBufferedBytes: 1}, validation.ErrResourceLimit, false},
		{"work cap", bytes.NewReader([]byte("ab")), validation.Budgets{MaxRuleSteps: 1}, validation.ErrResourceLimit, false},
		{"canceled", bytes.NewReader([]byte("ab")), validation.Budgets{}, context.Canceled, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.canceled {
				cancel()
			}
			a, _ := validation.NewBudgetAccount(test.budget)
			b := &resourceTestBody{Reader: test.reader, closeErr: closeBoom}
			out, err := readAssessmentResourceBody(ctx, a, b)
			if out != nil || !errors.Is(err, test.want) || !errors.Is(err, closeBoom) || b.closed != 1 {
				t.Fatalf("%v %v %d", out, err, b.closed)
			}
			live, _ := a.Usage(validation.BudgetBufferedBytes)
			if live != 0 {
				t.Fatal(live)
			}
		})
	}
}
func TestResourceBodyEveryReturnedStreamCharged(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{MaxExternalBytes: 3})
	first, err := readAssessmentResourceBody(context.Background(), a, &resourceTestBody{Reader: bytes.NewBufferString("ab")})
	if err != nil {
		t.Fatal(err)
	}
	defer first.close()
	second, err := readAssessmentResourceBody(context.Background(), a, &resourceTestBody{Reader: bytes.NewBufferString("ab")})
	if second != nil || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal(second, err)
	}
}

type resourceEmptyReader struct{}

func (resourceEmptyReader) Read([]byte) (int, error) { return 0, nil }
func TestResourceBodyEmptyCloseFailureAndNoProgress(t *testing.T) {
	closeFailure := errors.New("close failed")
	for _, tc := range []struct {
		name     string
		r        io.Reader
		ce, want error
	}{
		{"empty", bytes.NewReader(nil), nil, nil},
		{"close", bytes.NewReader([]byte("x")), closeFailure, closeFailure},
		{"no progress", resourceEmptyReader{}, nil, io.ErrNoProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{})
			b := &resourceTestBody{Reader: tc.r, closeErr: tc.ce}
			out, err := readAssessmentResourceBody(context.Background(), a, b)
			if !errors.Is(err, tc.want) || b.closed != 1 {
				t.Fatal(err, b.closed)
			}
			if tc.want == nil {
				if out == nil || out.digest != fmt.Sprintf("%x", sha256.Sum256(nil)) {
					t.Fatal(out)
				}
				out.close()
			} else if out != nil {
				t.Fatal("failed body escaped")
			}
			live, _ := a.Usage(validation.BudgetBufferedBytes)
			if live != 0 {
				t.Fatal(live)
			}
		})
	}
}
