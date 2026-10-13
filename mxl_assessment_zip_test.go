package musicxml

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// This builder writes records directly. In particular archive/zip.Writer is
// never the oracle for local/central disagreement or descriptor framing.
type assessmentZIPFixture struct {
	name, body, localExtra, centralExtra string
	localName                            string
	packed                               []byte
	method, flags                        uint16
	localMethod, localFlags              *uint16
	descriptor                           int // 12 (unsigned) or 16 (signed)
}

func assessmentZIPBytes(entries []assessmentZIPFixture, order []int, comment string) []byte {
	var b bytes.Buffer
	u16 := func(v uint16) { _ = binary.Write(&b, binary.LittleEndian, v) }
	u32 := func(v uint32) { _ = binary.Write(&b, binary.LittleEndian, v) }
	offsets := make([]uint32, len(entries))
	compressed := make([][]byte, len(entries))
	for i, e := range entries {
		compressed[i] = []byte(e.body)
		if e.packed != nil {
			compressed[i] = e.packed
			continue
		}
		if e.method == zip.Deflate {
			var packed bytes.Buffer
			w, _ := flate.NewWriter(&packed, flate.DefaultCompression)
			_, _ = w.Write(compressed[i])
			_ = w.Close()
			compressed[i] = packed.Bytes()
		}
	}
	for i, e := range entries {
		offsets[i] = uint32(b.Len())
		name := e.name
		if e.localName != "" {
			name = e.localName
		}
		method, flags := e.method, e.flags
		if e.localMethod != nil {
			method = *e.localMethod
		}
		if e.localFlags != nil {
			flags = *e.localFlags
		}
		u32(0x04034b50)
		u16(20)
		u16(flags)
		u16(method)
		u16(0)
		u16(0)
		if e.descriptor != 0 {
			u32(0)
			u32(0)
			u32(0)
		} else {
			u32(crc32.ChecksumIEEE([]byte(e.body)))
			u32(uint32(len(compressed[i])))
			u32(uint32(len(e.body)))
		}
		u16(uint16(len(name)))
		u16(uint16(len(e.localExtra)))
		b.WriteString(name)
		b.WriteString(e.localExtra)
		b.Write(compressed[i])
		if e.descriptor != 0 {
			if e.descriptor == 16 {
				u32(0x08074b50)
			}
			u32(crc32.ChecksumIEEE([]byte(e.body)))
			u32(uint32(len(compressed[i])))
			u32(uint32(len(e.body)))
		}
	}
	start := b.Len()
	if order == nil {
		for i := range entries {
			order = append(order, i)
		}
	}
	for _, i := range order {
		e := entries[i]
		u32(0x02014b50)
		u16(20)
		u16(20)
		u16(e.flags)
		u16(e.method)
		u16(0)
		u16(0)
		u32(crc32.ChecksumIEEE([]byte(e.body)))
		u32(uint32(len(compressed[i])))
		u32(uint32(len(e.body)))
		u16(uint16(len(e.name)))
		u16(uint16(len(e.centralExtra)))
		u16(0)
		u16(0)
		u16(0)
		u32(0)
		u32(offsets[i])
		b.WriteString(e.name)
		b.WriteString(e.centralExtra)
	}
	size := b.Len() - start
	u32(0x06054b50)
	u16(0)
	u16(0)
	u16(uint16(len(order)))
	u16(uint16(len(order)))
	u32(uint32(size))
	u32(uint32(start))
	u16(uint16(len(comment)))
	b.WriteString(comment)
	return b.Bytes()
}
func assessmentZIPAccount(t *testing.T, b validation.Budgets) *validation.Account {
	t.Helper()
	a, e := validation.NewBudgetAccount(b)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func assessmentZIPClose(t *testing.T, a *mxlAssessmentArchive, account *validation.Account) {
	t.Helper()
	if a != nil {
		a.Close()
		a.Close()
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		v, e := account.Usage(kind)
		if e != nil || v != 0 {
			t.Errorf("cleanup %s = %d, %v", kind, v, e)
		}
	}
}
func TestMXLAssessmentZIPRawHeaders(t *testing.T) {
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: mxlMIMEType, localExtra: "\xfe\xca\x00\x00"}, {name: "score.xml", body: "x"}}, []int{1, 0}, "")
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	if e != nil {
		t.Fatal(e)
	}
	entry := a.entries[1]
	if entry.local.name != "mimetype" || len(entry.local.extra) != 4 || len(entry.central.extra) != 0 || !entry.associated || entry.physicalOrder != 0 {
		t.Fatalf("raw evidence lost: %+v", entry)
	}
	data, e := a.readEntry(1, mxlAssessmentMetadata)
	if e != nil || string(data) != mxlMIMEType {
		t.Fatalf("read %q %v", data, e)
	}
	used, _ := account.Usage(validation.BudgetExternalBytes)
	_, e = a.readEntry(1, mxlAssessmentMetadata)
	again, _ := account.Usage(validation.BudgetExternalBytes)
	if e != nil || used != int64(len(mxlMIMEType)) || again != used {
		t.Fatalf("double charge %d %d %v", used, again, e)
	}
}
func TestMXLAssessmentZIPDescriptors(t *testing.T) {
	for _, size := range []int{12, 16} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "abc", flags: 8, descriptor: size}}, nil, "")
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			if e != nil {
				t.Fatal(e)
			}
			v, e := a.readEntry(0, mxlAssessmentResource)
			if e != nil || string(v) != "abc" || a.entries[0].end-a.entries[0].bodyEnd != int64(size) {
				t.Fatalf("descriptor %q %v %+v", v, e, a.entries[0])
			}
		})
	}
}
func TestMXLAssessmentZIPMismatchIsTechnical(t *testing.T) {
	method := uint16(8)
	for _, f := range []assessmentZIPFixture{{name: "a", body: "x", localName: "b"}, {name: "a", body: "x", localMethod: &method}} {
		b := assessmentZIPBytes([]assessmentZIPFixture{f}, nil, "")
		account := assessmentZIPAccount(t, validation.Budgets{})
		a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
		defer assessmentZIPClose(t, a, account)
		if !errors.Is(e, zip.ErrFormat) {
			t.Fatalf("wanted technical error, got %v", e)
		}
	}
}
