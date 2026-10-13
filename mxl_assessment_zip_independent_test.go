package musicxml

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"hash/crc32"
	"io"
	"math"
	"strings"
	"sync"
	"testing"
)

// Independent wire encoder. No production observation helper or author fixture is used.
type auditZIPMember struct {
	name                              string
	body                              []byte
	method                            uint16
	descriptor                        int
	localExtra, centralExtra, comment []byte
}

func auditZIPEncode(members []auditZIPMember, comment []byte) []byte {
	var out bytes.Buffer
	type wireMember struct {
		m                              auditZIPMember
		off, crc, compressed, expanded uint32
	}
	var records []wireMember
	put16 := func(b []byte, off int, n uint16) { binary.LittleEndian.PutUint16(b[off:], n) }
	put32 := func(b []byte, off int, n uint32) { binary.LittleEndian.PutUint32(b[off:], n) }
	for _, m := range members {
		packed := m.body
		if m.method == 8 {
			var dst bytes.Buffer
			w, _ := flate.NewWriter(&dst, flate.BestSpeed)
			_, _ = w.Write(m.body)
			_ = w.Close()
			packed = dst.Bytes()
		}
		r := wireMember{m: m, off: uint32(out.Len()), crc: crc32.ChecksumIEEE(m.body), compressed: uint32(len(packed)), expanded: uint32(len(m.body))}
		h := make([]byte, 30)
		put32(h, 0, 0x04034b50)
		put16(h, 4, 20)
		put16(h, 8, m.method)
		if m.descriptor == 0 {
			put32(h, 14, r.crc)
			put32(h, 18, r.compressed)
			put32(h, 22, r.expanded)
		} else {
			put16(h, 6, 8)
		}
		put16(h, 26, uint16(len(m.name)))
		put16(h, 28, uint16(len(m.localExtra)))
		out.Write(h)
		out.WriteString(m.name)
		out.Write(m.localExtra)
		out.Write(packed)
		if m.descriptor != 0 {
			d := make([]byte, m.descriptor)
			base := 0
			if len(d) == 16 {
				put32(d, 0, 0x08074b50)
				base = 4
			}
			put32(d, base, r.crc)
			put32(d, base+4, r.compressed)
			put32(d, base+8, r.expanded)
			out.Write(d)
		}
		records = append(records, r)
	}
	start := out.Len()
	for _, r := range records {
		m := r.m
		h := make([]byte, 46)
		put32(h, 0, 0x02014b50)
		put16(h, 4, 20)
		put16(h, 6, 20)
		put16(h, 10, m.method)
		if m.descriptor != 0 {
			put16(h, 8, 8)
		}
		put32(h, 16, r.crc)
		put32(h, 20, r.compressed)
		put32(h, 24, r.expanded)
		put16(h, 28, uint16(len(m.name)))
		put16(h, 30, uint16(len(m.centralExtra)))
		put16(h, 32, uint16(len(m.comment)))
		put32(h, 42, r.off)
		out.Write(h)
		out.WriteString(m.name)
		out.Write(m.centralExtra)
		out.Write(m.comment)
	}
	end := make([]byte, 22)
	put32(end, 0, 0x06054b50)
	put16(end, 8, uint16(len(members)))
	put16(end, 10, uint16(len(members)))
	put32(end, 12, uint32(out.Len()-start))
	put32(end, 16, uint32(start))
	put16(end, 20, uint16(len(comment)))
	out.Write(end)
	out.Write(comment)
	return out.Bytes()
}
func auditZIPAccount(t *testing.T, b validation.Budgets) *validation.Account {
	t.Helper()
	a, e := validation.NewBudgetAccount(b)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func auditZIPFinish(t *testing.T, a *mxlAssessmentArchive, ac *validation.Account) {
	t.Helper()
	a.Close()
	a.Close()
	n, e := ac.Usage(validation.BudgetBufferedBytes)
	if e != nil || n != 0 {
		t.Errorf("live gauge after cleanup: %d %v", n, e)
	}
}

type auditZIPFinalError struct {
	b      []byte
	err    error
	closes int
}

func (r *auditZIPFinalError) Read(p []byte) (int, error) {
	n := copy(p, r.b)
	r.b = r.b[n:]
	if len(r.b) == 0 {
		return n, r.err
	}
	return n, nil
}
func (r *auditZIPFinalError) Close() error { r.closes++; return nil }
func TestIndependentZIPJoinedEOFPreservesTransportFailure(t *testing.T) {
	expected := errors.New("independent transport failure")
	r := &auditZIPFinalError{b: auditZIPEncode([]auditZIPMember{{name: "a", body: []byte("abc")}}, nil), err: errors.Join(io.EOF, expected)}
	ac := auditZIPAccount(t, validation.Budgets{})
	a, err := openMXLAssessmentArchive(context.Background(), r, MXLOptions{}, ac)
	defer auditZIPFinish(t, a, ac)
	if !errors.Is(err, expected) {
		t.Errorf("transport error disappeared: %v; ready=%v", err, a.ready)
	}
	if r.closes != 0 {
		t.Errorf("caller closed %d times", r.closes)
	}
}

func TestIndependentZIPActualOutputAndCachedRoles(t *testing.T) {
	for _, method := range []uint16{0, 8} {
		t.Run(fmt.Sprint(method), func(t *testing.T) {
			body := []byte(strings.Repeat("q", 10000))
			b := auditZIPEncode([]auditZIPMember{{name: "a", body: body, method: method}, {name: "b", body: []byte("two")}}, nil)
			ac := auditZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{MaxMetadataBytes: 9999}, ac)
			defer auditZIPFinish(t, a, ac)
			if e != nil {
				t.Fatal(e)
			}
			first, e := a.readEntry(0, mxlAssessmentResource)
			if e != nil || !bytes.Equal(first, body) {
				t.Fatalf("first read %d %v", len(first), e)
			}
			before, _ := ac.Usage(validation.BudgetExternalBytes)
			narrow, e := a.readEntry(0, mxlAssessmentMetadata)
			after, _ := ac.Usage(validation.BudgetExternalBytes)
			if !errors.Is(e, ErrMXLTooLarge) || narrow != nil || before != 10000 || after != before {
				t.Fatalf("narrow role escaped: bytes=%d before=%d after=%d err=%v", len(narrow), before, after, e)
			}
			again, e := a.readEntry(0, mxlAssessmentDocument)
			if e != nil || &again[0] != &first[0] {
				t.Fatalf("immutable cache identity lost: %v", e)
			}
			v, e := a.readEntry(1, mxlAssessmentResource)
			after, _ = ac.Usage(validation.BudgetExternalBytes)
			if string(v) != "two" || e != nil || after != 10003 {
				t.Fatalf("entry identity reused wrongly %q %v %d", v, e, after)
			}
		})
	}
}

func TestIndependentZIPHiddenOutputStillCharged(t *testing.T) {
	for _, method := range []uint16{0, 8} {
		t.Run(fmt.Sprint(method), func(t *testing.T) {
			body := []byte(strings.Repeat("a", 6000))
			b := auditZIPEncode([]auditZIPMember{{name: "a", body: body, method: method}}, nil)
			central := int(binary.LittleEndian.Uint32(b[len(b)-6:]))
			binary.LittleEndian.PutUint32(b[22:], 5999)
			binary.LittleEndian.PutUint32(b[central+24:], 5999)
			ac := auditZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, ac)
			defer auditZIPFinish(t, a, ac)
			if e != nil {
				t.Fatal(e)
			}
			v, e := a.readEntry(0, mxlAssessmentResource)
			n, _ := ac.Usage(validation.BudgetExternalBytes)
			if !errors.Is(e, zip.ErrFormat) || n != 6000 {
				t.Fatalf("underlying output disappeared: payload=%d count=%d err=%v", len(v), n, e)
			}
			_, e2 := a.readEntry(0, mxlAssessmentResource)
			n2, _ := ac.Usage(validation.BudgetExternalBytes)
			if e2 != e || n2 != n {
				t.Fatalf("failure retried %v %v %d %d", e, e2, n, n2)
			}
		})
	}
}

func TestIndependentZIPExtraAndCommentScalarCaps(t *testing.T) {
	for _, field := range []string{"name", "local-extra", "central-extra", "file-comment", "archive-comment"} {
		t.Run(field, func(t *testing.T) {
			m := auditZIPMember{name: "a", body: []byte("a")}
			var comment []byte
			switch field {
			case "name":
				m.name = "12345678"
			case "local-extra":
				m.localExtra = []byte{0x42, 0x42, 4, 0, 1, 2, 3, 4}
			case "central-extra":
				m.centralExtra = []byte{0x42, 0x42, 4, 0, 1, 2, 3, 4}
			case "file-comment":
				m.comment = []byte("12345678")
			case "archive-comment":
				comment = []byte("12345678")
			}
			b := auditZIPEncode([]auditZIPMember{m}, comment)
			for _, limit := range []int64{8, 7} {
				ac := auditZIPAccount(t, validation.Budgets{MaxScalarBytes: limit})
				a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, ac)
				if limit == 8 && e != nil {
					t.Errorf("exact: %v", e)
				}
				if limit == 7 {
					var re *validation.ResourceError
					if !errors.As(e, &re) || re.Kind != validation.BudgetScalarBytes || a.reader != nil {
						t.Errorf("pre-admission one-under: %v", e)
					}
				}
				auditZIPFinish(t, a, ac)
			}
		})
	}
}

func TestIndependentZIPConcurrentArchives(t *testing.T) {
	data := auditZIPEncode([]auditZIPMember{{name: "mimetype", body: []byte("application/vnd.recordare.musicxml")}, {name: "part", body: []byte(strings.Repeat("staff", 10000)), method: 8, descriptor: 16}}, nil)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Go(func() {
			ac := auditZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(data), MXLOptions{}, ac)
			defer auditZIPFinish(t, a, ac)
			if e != nil {
				t.Error(e)
				return
			}
			for j := range a.entries {
				v, e := a.readEntry(j, mxlAssessmentResource)
				if e != nil || len(v) == 0 {
					t.Errorf("parallel bytes=%d err=%v", len(v), e)
				}
			}
		})
	}
	wg.Wait()
}

func TestIndependentZIPArithmetic(t *testing.T) {
	for _, n := range []int64{math.MaxInt64, math.MaxInt64 / 2048, math.MaxInt64 / 4} {
		if _, e := mxlZIPInventoryAllowance(n, n); e == nil {
			t.Errorf("overflow admitted %d", n)
		}
	}
}

func TestIndependentZIPConcreteBudgetBoundaries(t *testing.T) {
	encoded := auditZIPEncode([]auditZIPMember{{name: "a", body: []byte("abc")}}, nil)
	if len(encoded) != 103 {
		t.Fatal("independent fixture layout changed")
	}
	cases := []struct {
		name  string
		limit int64
		kind  validation.BudgetKind
		set   func(*validation.Budgets, int64)
	}{
		{"input", 103, validation.BudgetInputBytes, func(b *validation.Budgets, n int64) { b.MaxInputBytes = n }},
		{"external", 3, validation.BudgetExternalBytes, func(b *validation.Budgets, n int64) { b.MaxExternalBytes = n }},
		// 1024 owner + 4096 compressed capacity + 262144 ZIP-reader allowance +
		// 2048 entry allowance + 4 central name bytes + 1 local name byte +
		// 524288 decompressor allowance + 4096 scratch + 4096 expanded capacity.
		{"buffered", 801797, validation.BudgetBufferedBytes, func(b *validation.Budgets, n int64) { b.MaxBufferedBytes = n }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, delta := range []int64{0, -1} {
				var budgets validation.Budgets
				tc.set(&budgets, tc.limit+delta)
				ac := auditZIPAccount(t, budgets)
				a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(encoded), MXLOptions{}, ac)
				if e == nil {
					_, e = a.readEntry(0, mxlAssessmentResource)
				}
				if delta == 0 && e != nil {
					t.Errorf("exact: %v", e)
				}
				if delta < 0 {
					var re *validation.ResourceError
					if !errors.As(e, &re) || re.Kind != tc.kind {
						t.Errorf("one-under: %v", e)
					}
				}
				auditZIPFinish(t, a, ac)
			}
		})
	}
}

func TestIndependentZIPLegacyCaps(t *testing.T) {
	b := auditZIPEncode([]auditZIPMember{{name: "a", body: []byte("abc")}}, nil)
	cases := []struct {
		name string
		n    int64
		role mxlAssessmentEntryRole
		set  func(*MXLOptions, int64)
	}{
		{"archive", 103, mxlAssessmentResource, func(o *MXLOptions, n int64) { o.MaxArchiveBytes = n }},
		{"metadata", 3, mxlAssessmentMetadata, func(o *MXLOptions, n int64) { o.MaxMetadataBytes = n }},
		{"document", 3, mxlAssessmentDocument, func(o *MXLOptions, n int64) { o.MaxDocumentBytes = n }},
		{"resource", 3, mxlAssessmentResource, func(o *MXLOptions, n int64) { o.MaxResourceBytes = n }},
		{"resources", 3, mxlAssessmentResource, func(o *MXLOptions, n int64) { o.MaxResourcesBytes = n }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, d := range []int64{0, -1} {
				o := MXLOptions{}
				tc.set(&o, tc.n+d)
				ac := auditZIPAccount(t, validation.Budgets{})
				a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), o, ac)
				if e == nil {
					_, e = a.readEntry(0, tc.role)
				}
				if d == 0 && e != nil || d < 0 && !errors.Is(e, ErrMXLTooLarge) {
					t.Errorf("delta=%d: %v", d, e)
				}
				auditZIPFinish(t, a, ac)
			}
		})
	}
}

func TestIndependentZIPBorrowedAccount(t *testing.T) {
	b := auditZIPEncode([]auditZIPMember{{name: "a", body: []byte("abc")}}, nil)
	ac := auditZIPAccount(t, validation.Budgets{})
	if e := ac.Charge(validation.BudgetExternalBytes, int64(len(b))); e != nil {
		t.Fatal(e)
	}
	if e := ac.Charge(validation.BudgetBufferedBytes, int64(cap(b))); e != nil {
		t.Fatal(e)
	}
	a, e := borrowMXLAssessmentArchive(context.Background(), b, MXLOptions{}, ac)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.readEntry(0, mxlAssessmentDocument); e != nil {
		t.Fatal(e)
	}
	a.Close()
	a.Close()
	input, _ := ac.Usage(validation.BudgetInputBytes)
	external, _ := ac.Usage(validation.BudgetExternalBytes)
	live, _ := ac.Usage(validation.BudgetBufferedBytes)
	if input != 0 || external != 106 || live != int64(cap(b)) {
		t.Fatalf("wrong borrower charge: input=%d external=%d live=%d", input, external, live)
	}
	if e := ac.Release(validation.BudgetBufferedBytes, int64(cap(b))); e != nil {
		t.Fatal(e)
	}
}

func TestIndependentZIPWrappedCountAndPreallocation(t *testing.T) {
	members := make([]auditZIPMember, 65536)
	for i := range members {
		members[i].name = fmt.Sprintf("%05x", i)
	}
	b := auditZIPEncode(members, nil)
	for _, nodes := range []int64{65535, 65536} {
		ac := auditZIPAccount(t, validation.Budgets{MaxNodes: nodes, MaxBufferedBytes: 256 << 20})
		a, e := borrowMXLAssessmentArchive(context.Background(), b, MXLOptions{}, ac)
		if a.reader != nil || a.ready {
			t.Fatal("archive/zip constructed across count gate")
		}
		if nodes == 65535 {
			var re *validation.ResourceError
			if !errors.As(e, &re) || re.Kind != validation.BudgetNodes || len(a.entries) != 0 {
				t.Errorf("node admission not before entry inventory: %v", e)
			}
		} else {
			var ce *mxlAssessmentZIPCapabilityError
			if !errors.As(e, &ce) || a.actualEntryCount != 65536 || a.declaredEntryCount != 0 || a.countConsistent || !a.inventoryComplete || !a.centralDirectoryComplete {
				t.Errorf("wrapped count laundered: %v", e)
			}
		}
		auditZIPFinish(t, a, ac)
	}
}
