package musicxml

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type byteOwnerPauseReader struct {
	io.Reader
	pauses, reads int
	cancel        context.CancelFunc
}

func (r *byteOwnerPauseReader) Read(p []byte) (int, error) {
	r.reads++
	if r.pauses > 0 {
		r.pauses--
		return 0, nil
	}
	n, err := r.Reader.Read(p)
	if r.cancel != nil {
		r.cancel()
	}
	return n, err
}

func TestSourceByteOwnerExactAndExcessIncludingEventualReads(t *testing.T) {
	for _, owner := range []sourceByteOwner{sourceBytesPrimary, sourceBytesExternal, sourceBytesAccounted} {
		for _, pauses := range []int{0, 3, 99} {
			for _, length := range []int{4, 5} {
				t.Run(fmt.Sprintf("owner-%d/pause-%d/bytes-%d", owner, pauses, length), func(t *testing.T) {
					a := adapterAccount(t, validation.Budgets{MaxInputBytes: 4, MaxExternalBytes: 4})
					if owner == sourceBytesAccounted {
						if err := a.Charge(validation.BudgetExternalBytes, 4); err != nil {
							t.Fatal(err)
						}
					}
					input := strings.Repeat("a", length)
					reader := &byteOwnerPauseReader{Reader: strings.NewReader(input), pauses: pauses}
					cursor, err := newSourceXMLCursorWithByteOwner(reader, context.Background(), a, owner)
					if err != nil {
						t.Fatal(err)
					}
					for err == nil {
						_, err = cursor.take()
					}
					overflow := length == 5 && owner != sourceBytesAccounted
					if overflow {
						var resource *validation.ResourceError
						if !errors.As(err, &resource) {
							t.Fatalf("one over: %v", err)
						}
						wantKind := validation.BudgetInputBytes
						if owner == sourceBytesExternal {
							wantKind = validation.BudgetExternalBytes
						}
						if resource.Kind != wantKind || resource.Limit != 4 || resource.Used != 4 || resource.Requested != 1 {
							t.Errorf("wrong overflow: %+v", resource)
						}
					} else if err != io.EOF {
						t.Fatalf("exact/accounted: %v", err)
					}
					if cursor.reads != int64(length) || cursor.readDigest() != sha256.Sum256([]byte(input)) {
						t.Fatalf("original bytes not retained in read summary: %d", cursor.reads)
					}
					inputUsed, externalUsed := adapterUsage(t, a, validation.BudgetInputBytes), adapterUsage(t, a, validation.BudgetExternalBytes)
					if owner == sourceBytesPrimary && (inputUsed != 4 || externalUsed != 0) {
						t.Errorf("primary %d/%d", inputUsed, externalUsed)
					}
					if owner != sourceBytesPrimary && (inputUsed != 0 || externalUsed != 4) {
						t.Errorf("external/accounted %d/%d", inputUsed, externalUsed)
					}
					cursor.close()
					cursor.close()
					adapterClean(t, a)
				})
			}
		}
	}
}

func TestSourceByteOwnerInvalidBeforeAnyReadOrCharge(t *testing.T) {
	for _, owner := range []sourceByteOwner{3, 255} {
		a := adapterAccount(t, validation.Budgets{})
		reader := &sourceCursorReadSpy{}
		cursor, err := newSourceXMLCursorWithByteOwner(reader, context.Background(), a, owner)
		if cursor != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || reader.calls != 0 {
			t.Fatalf("invalid owner escaped: %v %v reads=%d", cursor, err, reader.calls)
		}
		adapterClean(t, a)
	}
	for _, owner := range []sourceByteOwner{sourceBytesExternal, sourceBytesAccounted} {
		reader := &sourceCursorReadSpy{}
		if _, err := newSourceXMLCursorWithByteOwner(reader, context.Background(), nil, owner); !errors.Is(err, validation.ErrInvalidConfiguration) || reader.calls != 0 {
			t.Fatalf("missing owner account: %v reads=%d", err, reader.calls)
		}
	}
}

func TestSourceByteOwnerNoProgressCancellationAndReaderErrors(t *testing.T) {
	for _, owner := range []sourceByteOwner{sourceBytesPrimary, sourceBytesExternal, sourceBytesAccounted} {
		t.Run(fmt.Sprint(owner), func(t *testing.T) {
			a := adapterAccount(t, validation.Budgets{})
			reader := &byteOwnerPauseReader{Reader: strings.NewReader("a"), pauses: 1000}
			cursor, err := newSourceXMLCursorWithByteOwner(reader, context.Background(), a, owner)
			if err != nil {
				t.Fatal(err)
			}
			_, err = cursor.take()
			if !errors.Is(err, io.ErrNoProgress) || reader.reads != 100 {
				t.Fatalf("unbounded no-progress: %v reads=%d", err, reader.reads)
			}
			cursor.close()
			adapterClean(t, a)
			if adapterUsage(t, a, validation.BudgetInputBytes) != 0 || adapterUsage(t, a, validation.BudgetExternalBytes) != 0 {
				t.Fatal("empty read charged")
			}

			ctx, cancel := context.WithCancel(context.Background())
			reader = &byteOwnerPauseReader{Reader: strings.NewReader("abc"), pauses: 3, cancel: cancel}
			cursor, err = newSourceXMLCursorWithByteOwner(reader, ctx, a, owner)
			if err != nil {
				t.Fatal(err)
			}
			_, err = cursor.take()
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("eventual read cancellation lost: %v", err)
			}
			if cursor.reads != 3 {
				t.Fatalf("read bytes lost: %d", cursor.reads)
			}
			if owner != sourceBytesAccounted {
				kind := validation.BudgetInputBytes
				if owner == sourceBytesExternal {
					kind = validation.BudgetExternalBytes
				}
				if adapterUsage(t, a, kind) != 3 {
					t.Fatal("canceled eventual read bypassed accounting")
				}
			}
			cursor.close()
			adapterClean(t, a)
		})
	}
}
