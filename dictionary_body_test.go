package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDictionaryComponentBodyExactLengthAndProbe(t *testing.T) {
	for _, tc := range []struct {
		name           string
		size, expected int
		valid          bool
	}{
		{"exact", 4097, 4097, true},
		{"short", 4096, 4097, false},
		{"overflow probe", 1 << 20, 4097, false},
		{"empty", 0, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{})
			reader := bytes.NewReader(make([]byte, tc.size))
			body := &resourceTestBody{Reader: reader}
			out, err := readDictionaryComponentBody(context.Background(), a, body, int64(tc.expected))
			if tc.valid {
				if err != nil || out == nil || len(out.bytes) != tc.expected {
					t.Fatalf("out=%v err=%v", out, err)
				}
				out.close()
			} else if out != nil || err == nil || errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("out=%v err=%v", out, err)
			}
			wantRead := min(tc.size, tc.expected+1)
			used, _ := a.Usage(validation.BudgetExternalBytes)
			if used != int64(wantRead) || reader.Len() != tc.size-wantRead {
				t.Fatalf("read=%d remaining=%d", used, reader.Len())
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 || body.closed != 1 {
				t.Fatalf("live=%d closes=%d", live, body.closed)
			}
		})
	}
}

func TestDictionaryComponentBodyFailureOwnership(t *testing.T) {
	transport, closing := errors.New("transport"), errors.New("close")
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		reader  io.Reader
		length  int64
		budgets validation.Budgets
		want    error
	}{
		{"invalid length", context.Background(), bytes.NewReader([]byte("a")), 0, validation.Budgets{}, validation.ErrInvalidConfiguration},
		{"negative length", context.Background(), bytes.NewReader([]byte("a")), -1, validation.Budgets{}, validation.ErrInvalidConfiguration},
		{"length overflow", context.Background(), bytes.NewReader([]byte("a")), math.MaxInt64, validation.Budgets{}, validation.ErrInvalidConfiguration},
		{"nil context", nil, bytes.NewReader([]byte("a")), 1, validation.Budgets{}, validation.ErrInvalidConfiguration},
		{"bytes with read error", context.Background(), &resourceReadError{[]byte("a"), transport}, 1, validation.Budgets{}, transport},
		{"joined EOF", context.Background(), &resourceReadError{[]byte("a"), errors.Join(io.EOF, transport)}, 1, validation.Budgets{}, transport},
		{"external admission", context.Background(), bytes.NewReader([]byte("ab")), 2, validation.Budgets{MaxExternalBytes: 1}, validation.ErrResourceLimit},
		{"buffer admission", context.Background(), bytes.NewReader([]byte("a")), 1, validation.Budgets{MaxBufferedBytes: 1}, validation.ErrResourceLimit},
		{"work admission", context.Background(), bytes.NewReader([]byte("a")), 1, validation.Budgets{MaxRuleSteps: 1}, validation.ErrResourceLimit},
		{"no progress", context.Background(), resourceEmptyReader{}, 1, validation.Budgets{}, io.ErrNoProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(tc.budgets)
			body := &resourceTestBody{Reader: tc.reader, closeErr: closing}
			out, err := readDictionaryComponentBody(tc.ctx, a, body, tc.length)
			if out != nil || !errors.Is(err, tc.want) || !errors.Is(err, closing) || body.closed != 1 {
				t.Fatalf("out=%v err=%v closes=%d", out, err, body.closed)
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
				t.Fatalf("live=%d", live)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	body := &resourceTestBody{Reader: bytes.NewReader([]byte("a"))}
	out, err := readDictionaryComponentBody(ctx, a, body, 1)
	if out != nil || !errors.Is(err, context.Canceled) || body.closed != 1 {
		t.Fatalf("out=%v err=%v closes=%d", out, err, body.closed)
	}
}

type dictionaryCancelReader struct{ cancel context.CancelFunc }

func (r dictionaryCancelReader) Read(p []byte) (int, error) {
	r.cancel()
	p[0] = 'a'
	return 1, nil
}

func TestDictionaryComponentBodyReviewOwnership(t *testing.T) {
	t.Run("close-only error", func(t *testing.T) {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		boom := errors.New("close-only")
		body := &resourceTestBody{Reader: bytes.NewReader([]byte("a")), closeErr: boom}
		out, err := readDictionaryComponentBody(context.Background(), a, body, 1)
		if out != nil || !errors.Is(err, boom) || body.closed != 1 {
			t.Fatalf("out=%v err=%v closes=%d", out, err, body.closed)
		}
		if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
			t.Fatal(live)
		}
	})
	t.Run("nil account", func(t *testing.T) {
		body := &resourceTestBody{Reader: bytes.NewReader([]byte("a"))}
		out, err := readDictionaryComponentBody(context.Background(), nil, body, 1)
		if out != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || body.closed != 1 {
			t.Fatalf("out=%v err=%v closes=%d", out, err, body.closed)
		}
	})
	t.Run("canceled by read", func(t *testing.T) {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		body := &resourceTestBody{Reader: dictionaryCancelReader{cancel}}
		out, err := readDictionaryComponentBody(ctx, a, body, 1)
		if out != nil || !errors.Is(err, context.Canceled) || body.closed != 1 {
			t.Fatalf("out=%v err=%v closes=%d", out, err, body.closed)
		}
		if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
			t.Fatal(live)
		}
		if used, _ := a.Usage(validation.BudgetExternalBytes); used != 1 {
			t.Fatal(used)
		}
	})
	t.Run("typed nil", func(t *testing.T) {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		var body *resourceTestBody
		out, err := readDictionaryComponentBody(context.Background(), a, body, 1)
		if out != nil || !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Fatalf("out=%v err=%v", out, err)
		}
		if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
			t.Fatal(live)
		}
	})
}
