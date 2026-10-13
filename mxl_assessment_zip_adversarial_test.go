package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-muse/go-musicxml/validation"
)

func assessmentZIPCentral(b []byte) int {
	return int(binary.LittleEndian.Uint32(b[len(b)-6 : len(b)-2]))
}
func assessmentZIPSet16(b []byte, p int, v uint16) { binary.LittleEndian.PutUint16(b[p:p+2], v) }
func assessmentZIPSet32(b []byte, p int, v uint32) { binary.LittleEndian.PutUint32(b[p:p+4], v) }

func TestMXLAssessmentZIPHeaderMatrix(t *testing.T) {
	extra := "\xfe\xca\x00\x00"
	for _, tc := range []struct{ name, local, central string }{{"neither", "", ""}, {"local", extra, ""}, {"central", "", extra}, {"both", extra, extra}} {
		t.Run(tc.name, func(t *testing.T) {
			b := assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: mxlMIMEType, localExtra: tc.local, centralExtra: tc.central}}, nil, "")
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			if e != nil {
				t.Fatal(e)
			}
			if a.entries[0].local.extra != tc.local || a.entries[0].central.extra != tc.central {
				t.Fatalf("independent extras lost: %+v", a.entries[0])
			}
		})
	}
}
func TestMXLAssessmentZIPDeflateAndExactPrefixes(t *testing.T) {
	bodies := []string{mxlMIMEType, "", mxlMIMEType + "\n", "arbitrary", "\x00" + mxlMIMEType, " " + mxlMIMEType, "\t" + mxlMIMEType, "\n" + mxlMIMEType, "\v" + mxlMIMEType, "\f" + mxlMIMEType, "\r" + mxlMIMEType, "\xef\xbb\xbf" + mxlMIMEType, "\xfe\xff" + mxlMIMEType, "\xff\xfe" + mxlMIMEType, "\x00\x00\xfe\xff" + mxlMIMEType, "\xff\xfe\x00\x00" + mxlMIMEType}
	for i, body := range bodies {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			b := assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: body, method: zip.Deflate, flags: 8, descriptor: 16}}, nil, "")
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			if e != nil {
				t.Fatal(e)
			}
			v, e := a.readEntry(0, mxlAssessmentMetadata)
			if e != nil || string(v) != body {
				t.Fatalf("raw bytes changed: %x %v", v, e)
			}
			if string(a.contents[0].prefix[:a.contents[0].prefixLength]) != body {
				t.Fatal("prefix changed")
			}
		})
	}
}
func TestMXLAssessmentZIPEnvelopeAndPolicy(t *testing.T) {
	base := func() []byte {
		return assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "x"}, {name: "b", body: "y"}}, nil, "")
	}
	cases := []struct {
		name   string
		change func([]byte) []byte
		kind   string
	}{
		{"truncated-central", func(b []byte) []byte { assessmentZIPSet16(b, assessmentZIPCentral(b)+28, 65535); return b }, "technical"},
		{"bad-count", func(b []byte) []byte {
			assessmentZIPSet16(b, len(b)-12, 1)
			assessmentZIPSet16(b, len(b)-14, 1)
			return b
		}, "technical"},
		{"local-name", func(b []byte) []byte { b[30] = 'z'; return b }, "technical"},
		{"local-flags", func(b []byte) []byte { assessmentZIPSet16(b, 6, 1); return b }, "technical"},
		{"duplicate", func(b []byte) []byte { p := assessmentZIPCentral(b); b[p+47+46] = 'a'; return b }, "policy"},
		{"reused-offset", func(b []byte) []byte {
			p := assessmentZIPCentral(b)
			assessmentZIPSet32(b, p+47+42, 0)
			b[p+47+46] = 'a'
			return b
		}, "policy"},
		{"symlink", func(b []byte) []byte {
			p := assessmentZIPCentral(b)
			assessmentZIPSet16(b, p+4, 3<<8|20)
			assessmentZIPSet32(b, p+38, 0120777<<16)
			return b
		}, "policy"},
		{"multi-disk", func(b []byte) []byte { assessmentZIPSet16(b, len(b)-18, 1); return b }, "capability"},
		{"zip64", func(b []byte) []byte { assessmentZIPSet32(b, len(b)-10, math.MaxUint32); return b }, "capability"},
		{"trailing", func(b []byte) []byte { return append(b, 0) }, "capability"},
		{"preamble", func(b []byte) []byte { return append([]byte("junk"), b...) }, "capability"},
		{"unfamiliar-method", func(b []byte) []byte {
			p := assessmentZIPCentral(b)
			assessmentZIPSet16(b, 8, 99)
			assessmentZIPSet16(b, p+10, 99)
			return b
		}, "capability"},
		{"encrypted", func(b []byte) []byte {
			p := assessmentZIPCentral(b)
			assessmentZIPSet16(b, 6, 1)
			assessmentZIPSet16(b, p+8, 1)
			return b
		}, "capability"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(tc.change(base())), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			var capability *mxlAssessmentZIPCapabilityError
			var policy *mxlAssessmentZIPPolicyError
			ok := tc.kind == "technical" && errors.Is(e, zip.ErrFormat) || tc.kind == "capability" && errors.As(e, &capability) || tc.kind == "policy" && errors.As(e, &policy)
			if !ok {
				t.Fatalf("wanted %s, got %T %v", tc.kind, e, e)
			}
			if a != nil && a.ready {
				t.Fatal("unsafe archive ready")
			}
		})
	}
	for _, name := range []string{"../a", "/a", "a\\b", "a//b", "a/./b", "a/../b", ""} {
		t.Run("path-"+name, func(t *testing.T) {
			b := assessmentZIPBytes([]assessmentZIPFixture{{name: name, body: "x"}}, nil, "")
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			var policy *mxlAssessmentZIPPolicyError
			if !errors.As(e, &policy) {
				t.Fatalf("wanted path policy, got %v", e)
			}
		})
	}
}
func TestMXLAssessmentZIPCommentsAndFakeSignatures(t *testing.T) {
	for _, comment := range []string{"", "PK\x05\x06", strings.Repeat("x", 65535)} {
		account := assessmentZIPAccount(t, validation.Budgets{})
		b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "PK\x05\x06"}}, nil, comment)
		a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
		if e != nil {
			t.Errorf("comment size %d: %v", len(comment), e)
		}
		assessmentZIPClose(t, a, account)
	}
	// Go picks this forged zero-length EOCD inside the real comment. Admission
	// rejects the differential before giving archive/zip any allocation authority.
	fake := make([]byte, 22)
	assessmentZIPSet32(fake, 0, 0x06054b50)
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "x"}}, nil, string(fake))
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	var capability *mxlAssessmentZIPCapabilityError
	if !errors.As(e, &capability) || a.reader != nil {
		t.Fatalf("fake EOCD admitted: %v", e)
	}
}
func TestMXLAssessmentZIPPartialFactsAndNoAlias(t *testing.T) {
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: mxlMIMEType, localExtra: "\xfe\xca\x00\x00"}}, nil, "")
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := borrowMXLAssessmentArchive(context.Background(), b, MXLOptions{}, account)
	if e != nil {
		t.Fatal(e)
	}
	entry := a.entries[0]
	b[30] = 'X'
	if entry.local.name != "mimetype" || entry.central.name != "mimetype" {
		t.Fatal("raw facts alias input")
	}
	assessmentZIPClose(t, a, account)
	b = assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: mxlMIMEType, localExtra: "\xfe\xca\x00\x00"}}, nil, "")
	b = b[:len(b)-22]
	account = assessmentZIPAccount(t, validation.Budgets{})
	a, e = openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	if e == nil || !a.prefix.framed || len(a.prefix.header.extra) != 4 || a.inventoryComplete || len(a.entries) != 0 {
		t.Fatalf("partial local framing lost or promoted: %+v %v", a.prefix, e)
	}
}
func TestMXLAssessmentZIPCRCAndDecompressionAreTechnical(t *testing.T) {
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		for _, corrupt := range []string{"crc-zero", "short-size", "broken-body"} {
			t.Run(fmt.Sprintf("%d-%s", method, corrupt), func(t *testing.T) {
				body := "contents to check"
				b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: body, method: method}}, nil, "")
				p := assessmentZIPCentral(b)
				switch corrupt {
				case "crc-zero":
					assessmentZIPSet32(b, 14, 0)
					assessmentZIPSet32(b, p+16, 0)
				case "short-size":
					assessmentZIPSet32(b, 22, uint32(len(body)+1))
					assessmentZIPSet32(b, p+24, uint32(len(body)+1))
				case "broken-body":
					b[31] ^= 0xff
				}
				account := assessmentZIPAccount(t, validation.Budgets{})
				a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
				defer assessmentZIPClose(t, a, account)
				if e != nil {
					t.Fatal(e)
				}
				_, e = a.readEntry(0, mxlAssessmentResource)
				if e == nil {
					t.Fatal("corruption accepted")
				}
				if corrupt != "broken-body" {
					used, _ := account.Usage(validation.BudgetExternalBytes)
					if used != int64(len(body)) {
						t.Fatalf("checksum wrapper hid bytes: %d, want %d", used, len(body))
					}
					if string(a.contents[0].prefix[:a.contents[0].prefixLength]) != body {
						t.Fatal("known raw prefix lost on checksum failure")
					}
				}
			})
		}
	}
}
func TestMXLAssessmentZIPActualCountBeforeReader(t *testing.T) {
	entries := make([]assessmentZIPFixture, 65536)
	for i := range entries {
		entries[i].name = fmt.Sprintf("%05x", i)
	}
	b := assessmentZIPBytes(entries, nil, "")
	account := assessmentZIPAccount(t, validation.Budgets{MaxNodes: 65535, MaxBufferedBytes: 256 << 20})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	var resource *validation.ResourceError
	if !errors.As(e, &resource) || resource.Kind != validation.BudgetNodes || a.reader != nil || len(a.entries) != 0 {
		t.Fatalf("actual count not preadmitted: %v", e)
	}
	account2 := assessmentZIPAccount(t, validation.Budgets{MaxNodes: 65536, MaxBufferedBytes: 256 << 20})
	a2, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account2)
	defer assessmentZIPClose(t, a2, account2)
	var capability *mxlAssessmentZIPCapabilityError
	if !errors.As(e, &capability) || len(a2.entries) != 65536 || a2.ready || a2.reader != nil || !a2.inventoryComplete || !a2.centralDirectoryComplete || a2.countConsistent || a2.actualEntryCount != 65536 || a2.declaredEntryCount != 0 {
		t.Fatalf("wrapped count not explicit: %v", e)
	}
}

// This checks the preserved external fixture without making a repository test
// depend on an untracked absolute path. It is an optional local evidence probe.
func TestMXLAssessmentZIPExternalLegacyGap(t *testing.T) {
	name := os.Getenv("MUSICXML_BATCH8_LOCAL_EXTRA_FIXTURE")
	if name == "" {
		t.Skip("independent external fixture not selected")
	}
	b, e := os.ReadFile(name)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = DecodeMXL(bytes.NewReader(b)); e != nil {
		t.Fatalf("legacy compatibility changed: %v", e)
	}
	if _, e = DecodeMXLPackage(bytes.NewReader(b)); e != nil {
		t.Fatalf("legacy package compatibility changed: %v", e)
	}
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	if e != nil {
		t.Fatal(e)
	}
	i, ok := a.index["mimetype"]
	if !ok || len(a.entries[i].local.extra) != 4 || len(a.entries[i].central.extra) != 0 {
		t.Fatal("independently built local-only extra missing")
	}
}

type assessmentZIPReader struct {
	data          []byte
	reads, closed int
	err           error
}

func (r *assessmentZIPReader) Read(p []byte) (int, error) {
	r.reads++
	if len(r.data) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}
func (r *assessmentZIPReader) Close() error { r.closed++; return nil }

type assessmentZIPCountingContext struct{ polls, fail int }

func (*assessmentZIPCountingContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*assessmentZIPCountingContext) Done() <-chan struct{}       { return nil }
func (c *assessmentZIPCountingContext) Err() error {
	c.polls++
	if c.fail > 0 && c.polls >= c.fail {
		return context.Canceled
	}
	return nil
}
func (*assessmentZIPCountingContext) Value(any) any { return nil }

func TestMXLAssessmentZIPOverlappingRanges(t *testing.T) {
	inner := assessmentZIPBytes([]assessmentZIPFixture{{name: "b", body: "y"}}, nil, "")
	embedded := inner[:assessmentZIPCentral(inner)]
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: string(embedded)}, {name: "b", body: "y"}}, nil, "")
	assessmentZIPSet32(b, assessmentZIPCentral(b)+47+42, 31)
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	var policy *mxlAssessmentZIPPolicyError
	if !errors.As(e, &policy) || !strings.Contains(policy.reason, "overlapping") || a.reader != nil {
		t.Fatalf("overlap not rejected before reader: %v", e)
	}
}
func TestMXLAssessmentZIPLiteralNamesAndDirectory(t *testing.T) {
	for _, name := range []string{"%2e%2e/score.musicxml", "literal%20space.musicxml", "a b.musicxml", "a\x00b", "nested/"} {
		b := assessmentZIPBytes([]assessmentZIPFixture{{name: name}}, nil, "")
		account := assessmentZIPAccount(t, validation.Budgets{})
		a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := a.index[name]; !ok {
			t.Fatalf("literal name transformed: %q", name)
		}
		if strings.HasSuffix(name, "/") {
			if _, e = a.readEntry(0, mxlAssessmentResource); e == nil {
				t.Fatal("directory used as resource")
			}
		}
		assessmentZIPClose(t, a, account)
	}
}
func TestMXLAssessmentZIPPhysicalSorting(t *testing.T) {
	entries := make([]assessmentZIPFixture, 33)
	order := make([]int, len(entries))
	for i := range entries {
		entries[i] = assessmentZIPFixture{name: fmt.Sprint(i), body: strings.Repeat("x", i)}
		order[i] = (i * 17) % len(entries)
	}
	b := assessmentZIPBytes(entries, order, "")
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	if e != nil {
		t.Fatal(e)
	}
	for rank, index := range a.physical {
		if a.entries[index].central.name != fmt.Sprint(rank) || a.entries[index].physicalOrder != rank {
			t.Fatalf("bad physical order at %d", rank)
		}
	}
}
func TestMXLAssessmentZIPDescriptorCorruptionAndPlaceholders(t *testing.T) {
	for _, signed := range []int{12, 16} {
		for _, change := range []string{"nonzero-placeholder", "wrong-descriptor", "wrong-local-size", "truncated-descriptor"} {
			t.Run(fmt.Sprintf("%d-%s", signed, change), func(t *testing.T) {
				b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "abc", flags: 8, descriptor: signed}}, nil, "")
				p := assessmentZIPCentral(b)
				switch change {
				case "nonzero-placeholder":
					copy(b[14:26], b[p+16:p+28])
				case "wrong-descriptor":
					b[34+signed-1] ^= 1
				case "wrong-local-size":
					assessmentZIPSet32(b, 22, 4)
				case "truncated-descriptor":
					assessmentZIPSet32(b, p+20, 100)
					assessmentZIPSet32(b, 18, 100)
				}
				account := assessmentZIPAccount(t, validation.Budgets{})
				a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
				defer assessmentZIPClose(t, a, account)
				if change == "nonzero-placeholder" {
					if e != nil {
						t.Fatal(e)
					}
					if _, e = a.readEntry(0, mxlAssessmentResource); e != nil {
						t.Fatal(e)
					}
				} else if !errors.Is(e, zip.ErrFormat) {
					t.Fatalf("descriptor corruption not technical: %v", e)
				}
			})
		}
	}
}
func TestMXLAssessmentZIPEmptyAndTruncations(t *testing.T) {
	complete := assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: mxlMIMEType, localExtra: "\xfe\xca\x00\x00"}}, nil, "")
	for n := 0; n < len(complete); n++ {
		account := assessmentZIPAccount(t, validation.Budgets{})
		a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(complete[:n]), MXLOptions{}, account)
		if e == nil {
			t.Fatalf("truncation %d accepted", n)
		}
		if a.ready || a.inventoryComplete || len(a.entries) != 0 {
			t.Fatalf("unproved entry identity on prefix length %d", n)
		}
		assessmentZIPClose(t, a, account)
	}
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(assessmentZIPBytes(nil, nil, "")), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	if e != nil || !a.inventoryComplete || len(a.entries) != 0 {
		t.Fatalf("empty ZIP observation: %v", e)
	}
}

func TestMXLAssessmentZIPUnsignedDescriptorSignatureCollision(t *testing.T) {
	b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "abc", flags: 8, descriptor: 12}}, nil, "")
	assessmentZIPSet32(b, 34, 0x08074b50)
	assessmentZIPSet32(b, assessmentZIPCentral(b)+16, 0x08074b50)
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	var capability *mxlAssessmentZIPCapabilityError
	if !errors.As(e, &capability) || !strings.Contains(capability.feature, "CRC collides") || a.reader != nil {
		t.Fatalf("descriptor ambiguity misclassified: %v", e)
	}
}
