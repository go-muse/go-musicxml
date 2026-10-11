package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceCursorPositions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   []byte
		offsets []int64
	}{
		{"utf8", []byte("Aé\r\n𝄞Z"), []int64{0, 1, 3, 5, 9}},
		{"utf8-bom", append([]byte{239, 187, 191}, []byte("Aé\r\n𝄞Z")...), []int64{3, 4, 6, 8, 12}},
		{"utf16be", encodeUTF16("Aé\r\n𝄞Z", binary.BigEndian), []int64{2, 4, 6, 10, 14}},
		{"utf16le", encodeUTF16("Aé\r\n𝄞Z", binary.LittleEndian), []int64{2, 4, 6, 10, 14}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newSourceXMLCursor(iotest.OneByteReader(bytes.NewReader(tc.input)), context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.close()
			for i, want := range []rune{'A', 'é', '\n', '𝄞', 'Z'} {
				u, err := c.take()
				if err != nil {
					t.Fatal(err)
				}
				if u.Rune != want || u.Start.ByteOffset != tc.offsets[i] {
					t.Fatalf("unit %d: %+v", i, u)
				}
				if i == 3 && (u.Start.Line != 2 || u.Start.Column != 1) {
					t.Fatalf("line after CRLF: %+v", u)
				}
			}
			if _, err := c.take(); err != io.EOF {
				t.Fatal(err)
			}
			if c.readDigest() != sha256.Sum256(tc.input) {
				t.Fatal("digest excludes original bytes")
			}
		})
	}
}

func TestSourceCursorBudgetAndCancellation(t *testing.T) {
	for _, n := range []int{4, 5} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxInputBytes: 4})
		c, err := newSourceXMLCursor(strings.NewReader(strings.Repeat("a", n)), context.Background(), a)
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = c.take()
		}
		if n == 4 && err != io.EOF {
			t.Fatal(err)
		}
		if n == 5 {
			var resource *validation.ResourceError
			if !errors.As(err, &resource) {
				t.Fatalf("want resource, got %v", err)
			}
		}
		c.close()
		used, _ := a.Usage(validation.BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("buffer leak %d", used)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &sourceCursorReadSpy{}
	if _, err := newSourceXMLCursor(r, ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if r.calls != 0 {
		t.Fatal("read canceled source")
	}
}

type sourceCursorReadSpy struct{ calls int }

func (r *sourceCursorReadSpy) Read(p []byte) (int, error) { r.calls++; return 0, io.EOF }

func TestSourceCursorUTF16LookaheadAllocations(t *testing.T) {
	input := encodeUTF16(strings.Repeat("ASCII é 𝄞\r\n", 256), binary.LittleEndian)
	allocations := testing.AllocsPerRun(3, func() {
		cursor, err := newSourceXMLCursor(bytes.NewReader(input), context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cursor.close()
		for {
			_, err = cursor.take()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	if allocations > 20 {
		t.Fatalf("UTF-16 code units allocate: %.0f allocations", allocations)
	}
}

func TestSourceCursorCachedUnitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cursor, err := newSourceXMLCursor(strings.NewReader("abcde"), ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.close()
	if _, err = cursor.peek(); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err = cursor.peek(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached peek: %v", err)
	}
	if _, err = cursor.take(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached take: %v", err)
	}
	if cursor.position.ByteOffset != 0 {
		t.Fatalf("canceled unit consumed: %+v", cursor.position)
	}
}
