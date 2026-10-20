package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/go-muse/go-musicxml/validation"
)

func TestIndependentResourceURIRFCAndBoundaries(t *testing.T) {
	cases := []struct{ ref, want string }{
		{"g:h", "g:h"}, {"g", "http://a/b/c/g"}, {"./g", "http://a/b/c/g"}, {"g/", "http://a/b/c/g/"},
		{"/g", "http://a/g"}, {"//g", "http://g"}, {"?y", "http://a/b/c/d;p?y"},
		{"g?y", "http://a/b/c/g?y"}, {"#s", "http://a/b/c/d;p?q"}, {"g#s", "http://a/b/c/g"},
		{";x", "http://a/b/c/;x"}, {"g;x?y#s", "http://a/b/c/g;x?y"}, {"", "http://a/b/c/d;p?q"},
		{".", "http://a/b/c/"}, {"..", "http://a/b/"}, {"../g", "http://a/b/g"}, {"../../g", "http://a/g"},
		{"../../../g", "http://a/g"}, {"/./g", "http://a/g"}, {"g/./h", "http://a/b/c/g/h"},
		{"g/../h", "http://a/b/c/h"}, {"g?y/./x", "http://a/b/c/g?y/./x"},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			a := adapterAccount(t, validation.Budgets{})
			got, err := resolveAssessmentResourceURI(context.Background(), a, resourceURIBase{RetrievalURI: "http://a/b/c/d;p?q"}, tc.ref)
			if err != nil || got.RetrievalURI != tc.want {
				t.Fatalf("got %+v err %v want %s", got, err, tc.want)
			}
			adapterClean(t, a)
		})
	}
	for _, tc := range []struct {
		base resourceURIBase
		ref  string
		want resolvedResourceURI
		bad  bool
	}{
		{resourceURIBase{Archive: true, Entry: "score.xml"}, "urn:vendor:child?x#f", resolvedResourceURI{RetrievalURI: "urn:vendor:child?x", Fragment: "f", FragmentPresent: true}, false},
		{resourceURIBase{RetrievalURI: "custom://host/a/b"}, "../c#", resolvedResourceURI{RetrievalURI: "custom://host/c", FragmentPresent: true}, false},
		{resourceURIBase{RetrievalURI: "urn:vendor:score?q"}, "#f", resolvedResourceURI{RetrievalURI: "urn:vendor:score?q", Fragment: "f", FragmentPresent: true, SameEntity: true}, false},
		{resourceURIBase{RetrievalURI: "urn:vendor:score"}, "relative", resolvedResourceURI{}, true},
		{resourceURIBase{Archive: true, Entry: "dir/score.xml"}, "./score.xml#%66", resolvedResourceURI{ArchivePath: "dir/score.xml", Fragment: "f", FragmentPresent: true, SameEntity: true}, false},
		{resourceURIBase{Archive: true, Entry: "dir/score.xml"}, "part%23x.xml", resolvedResourceURI{ArchivePath: "dir/part#x.xml"}, false},
		{resourceURIBase{Archive: true, Entry: "dir/score.xml"}, "?", resolvedResourceURI{}, true},
		{resourceURIBase{Archive: true, Entry: "../score.xml"}, "#", resolvedResourceURI{}, true},
		{resourceURIBase{Archive: true, Entry: "dir/score.xml", RetrievalURI: "https://outer.test/root.mxl"}, "//else.test/child", resolvedResourceURI{RetrievalURI: "https://else.test/child"}, false},
		{resourceURIBase{RetrievalURI: "https://example.test/a"}, " \n #a\t ", resolvedResourceURI{RetrievalURI: "https://example.test/a", Fragment: "a", FragmentPresent: true, SameEntity: true}, false},
	} {
		a := adapterAccount(t, validation.Budgets{})
		got, err := resolveAssessmentResourceURI(context.Background(), a, tc.base, tc.ref)
		if tc.bad {
			if !errors.Is(err, validation.ErrIncompleteAssessment) {
				t.Fatalf("%+v %q: got %+v %v", tc.base, tc.ref, got, err)
			}
		} else if err != nil || got != tc.want {
			t.Fatalf("%+v %q: got %+v %v want %+v", tc.base, tc.ref, got, err, tc.want)
		}
		adapterClean(t, a)
	}
	base, ref := resourceURIBase{RetrievalURI: "https://e.test/a"}, "b#f"
	n := int64(len(base.RetrievalURI) + len(ref))
	for _, tc := range []struct {
		b  validation.Budgets
		ok bool
	}{
		{validation.Budgets{MaxRuleSteps: n + 1, MaxBufferedBytes: 16*n + 1024, MaxScalarBytes: int64(len(base.RetrievalURI))}, true},
		{validation.Budgets{MaxRuleSteps: n}, false}, {validation.Budgets{MaxBufferedBytes: 16*n + 1023}, false},
		{validation.Budgets{MaxScalarBytes: int64(len(base.RetrievalURI)) - 1}, false},
	} {
		a := adapterAccount(t, tc.b)
		_, err := resolveAssessmentResourceURI(context.Background(), a, base, ref)
		if tc.ok && err != nil || !tc.ok && !errors.Is(err, validation.ErrResourceLimit) {
			t.Fatalf("budget %+v: %v", tc, err)
		}
		adapterClean(t, a)
	}
}

type independentBadResourceReader struct {
	n   int
	err error
}

func (r independentBadResourceReader) Read([]byte) (int, error) { return r.n, r.err }

type independentCancelResourceReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (r independentCancelResourceReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.cancel()
	return n, e
}

func TestIndependentResourceBodyExactAndOneUnder(t *testing.T) {
	// Each <=4096-byte read/append/digest attempt costs one unit, including
	// bytes.Reader's final EOF read. Capacity growth also costs one unit per
	// <=4096 bytes relocated; these totals are independent contract expectations.
	for _, budget := range []struct {
		size  int
		steps int64
	}{{0, 1}, {1, 2}, {4096, 2}, {4097, 4}, {8192, 4}, {8193, 7}, {65537, 49}} {
		size, steps := budget.size, budget.steps
		data := bytes.Repeat([]byte{'x'}, size)
		peak := int64(4608)
		capn := 0
		for seen := 0; seen < size; {
			n := min(4096, size-seen)
			need := seen + n
			if need > capn {
				next := max(need, capn*2)
				peak = max(peak, int64(4608+capn+next))
				capn = next
			}
			seen += n
		}
		for _, tc := range []struct {
			name string
			b    validation.Budgets
			ok   bool
			kind validation.BudgetKind
		}{
			{"exact", validation.Budgets{MaxExternalBytes: max(1, int64(size)), MaxRuleSteps: steps, MaxBufferedBytes: peak}, true, ""},
			{"buffer one under", validation.Budgets{MaxBufferedBytes: peak - 1}, false, validation.BudgetBufferedBytes},
			{"work one under", validation.Budgets{MaxRuleSteps: max(1, steps-1)}, size == 0, validation.BudgetRuleSteps},
			{"external one under", validation.Budgets{MaxExternalBytes: max(1, int64(size)-1)}, size <= 1, validation.BudgetExternalBytes},
		} {
			t.Run(fmt.Sprintf("%d/%s", size, tc.name), func(t *testing.T) {
				a := adapterAccount(t, tc.b)
				body := &resourceTestBody{Reader: bytes.NewReader(data)}
				out, err := readAssessmentResourceBody(context.Background(), a, body)
				if tc.ok {
					if err != nil || out == nil || !bytes.Equal(out.bytes, data) || out.digest != fmt.Sprintf("%x", sha256.Sum256(data)) {
						t.Fatalf("got %v err %v", out, err)
					}
					if n := adapterUsage(t, a, validation.BudgetExternalBytes); n != int64(size) {
						t.Fatalf("external bytes %d", n)
					}
					if n := adapterUsage(t, a, validation.BudgetRuleSteps); n != steps {
						t.Fatalf("work %d want %d", n, steps)
					}
					if n := adapterUsage(t, a, validation.BudgetBufferedBytes); n != 256+int64(cap(out.bytes)) {
						t.Fatalf("owned storage %d cap %d", n, cap(out.bytes))
					}
					out.close()
					out.close()
					if out.bytes != nil || out.digest != "" || out.account != nil {
						t.Fatal("lease not invalidated")
					}
				} else {
					var limit *validation.ResourceError
					if out != nil || !errors.Is(err, validation.ErrResourceLimit) || !errors.As(err, &limit) || limit.Kind != tc.kind {
						t.Fatalf("out present %v err %v want %s refusal", out != nil, err, tc.kind)
					}
				}
				if body.closed != 1 {
					t.Fatalf("closes %d", body.closed)
				}
				adapterClean(t, a)
			})
		}
	}
}

func TestIndependentResourceBodyShortReadsWorkAdmission(t *testing.T) {
	// Short reads are separately admitted, even when many fit in one scratch
	// block. One-byte reads grow at powers of two: relocations through 4096
	// bytes each cost one unit, then 8192/16384/32768/65536 cost 2/4/8/16.
	for _, tc := range []struct {
		size  int
		steps int64
	}{{0, 1}, {1, 2}, {4096, 4109}, {4097, 4111}, {8192, 8206}, {8193, 8209}, {65537, 65581}} {
		for _, oneUnder := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/one_under_%v", tc.size, oneUnder), func(t *testing.T) {
				steps := tc.steps
				if oneUnder {
					steps--
				}
				// A zero configured limit means the default, so the empty-body
				// one-under case can only exercise the minimum positive limit.
				a := adapterAccount(t, validation.Budgets{MaxRuleSteps: max(1, steps)})
				data := bytes.Repeat([]byte{'x'}, tc.size)
				reader := &independentRecordingReader{Reader: iotest.OneByteReader(bytes.NewReader(data))}
				body := &resourceTestBody{Reader: reader}
				out, err := readAssessmentResourceBody(context.Background(), a, body)
				if !oneUnder || tc.size == 0 {
					if err != nil || out == nil || !bytes.Equal(out.bytes, data) || out.digest != fmt.Sprintf("%x", sha256.Sum256(data)) {
						t.Fatalf("out present %v err %v", out != nil, err)
					}
					if reader.reads != tc.size+1 || adapterUsage(t, a, validation.BudgetRuleSteps) != tc.steps {
						t.Fatalf("reads %d work %d want reads %d work %d", reader.reads, adapterUsage(t, a, validation.BudgetRuleSteps), tc.size+1, tc.steps)
					}
					out.close()
					out.close()
				} else {
					var limit *validation.ResourceError
					if out != nil || !errors.Is(err, validation.ErrResourceLimit) || !errors.As(err, &limit) || limit.Kind != validation.BudgetRuleSteps {
						t.Fatalf("out present %v err %v", out != nil, err)
					}
					if reader.reads != tc.size || adapterUsage(t, a, validation.BudgetRuleSteps) != steps || limit.Requested != 1 {
						t.Fatalf("reads %d work %d want reads %d work %d; refusal %+v", reader.reads, adapterUsage(t, a, validation.BudgetRuleSteps), tc.size, steps, limit)
					}
				}
				if body.closed != 1 || !bytes.Equal(reader.data, data) || adapterUsage(t, a, validation.BudgetExternalBytes) != int64(tc.size) {
					t.Fatalf("closes %d read bytes %d charged bytes %d", body.closed, len(reader.data), adapterUsage(t, a, validation.BudgetExternalBytes))
				}
				adapterClean(t, a)
			})
		}
	}
}

func TestIndependentResourceBodyCauseAndCleanupMatrix(t *testing.T) {
	readFailure, closeFailure := errors.New("independent read cause"), errors.New("independent close cause")
	for _, tc := range []struct {
		name string
		r    io.Reader
		ctx  context.Context
		want error
	}{
		{"EOF data", &resourceReadError{[]byte("abc"), io.EOF}, context.Background(), nil},
		{"EOF joined other", &resourceReadError{[]byte("abc"), errors.Join(io.EOF, readFailure)}, context.Background(), readFailure},
		{"read only", &resourceReadError{nil, readFailure}, context.Background(), readFailure},
		{"no progress", resourceEmptyReader{}, context.Background(), io.ErrNoProgress},
		{"nil context", strings.NewReader("abc"), nil, validation.ErrInvalidConfiguration},
		{"bad negative", independentBadResourceReader{-1, nil}, context.Background(), validation.ErrInvalidConfiguration},
		{"bad too large", independentBadResourceReader{4097, nil}, context.Background(), validation.ErrInvalidConfiguration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := adapterAccount(t, validation.Budgets{})
			body := &resourceTestBody{Reader: tc.r, closeErr: closeFailure}
			out, err := readAssessmentResourceBody(tc.ctx, a, body)
			if out != nil || !errors.Is(err, closeFailure) || (tc.want != nil && !errors.Is(err, tc.want)) || body.closed != 1 {
				t.Fatalf("out %v err %v close %d", out, err, body.closed)
			}
			adapterClean(t, a)
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := adapterAccount(t, validation.Budgets{})
	body := &resourceTestBody{Reader: independentCancelResourceReader{&resourceReadError{[]byte("abc"), readFailure}, cancel}, closeErr: closeFailure}
	out, err := readAssessmentResourceBody(ctx, a, body)
	if out != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, readFailure) || !errors.Is(err, closeFailure) || body.closed != 1 {
		t.Fatalf("cancel result %v %v %d", out, err, body.closed)
	}
	if n := adapterUsage(t, a, validation.BudgetExternalBytes); n != 3 {
		t.Fatalf("unaccounted canceled bytes %d", n)
	}
	adapterClean(t, a)
}

func TestIndependentResourceBodyBadReaderPreservesCauses(t *testing.T) {
	readFailure, closeFailure := errors.New("independent read cause"), errors.New("independent close cause")
	for _, n := range []int{-1, 4097} {
		a := adapterAccount(t, validation.Budgets{})
		body := &resourceTestBody{Reader: independentBadResourceReader{n, errors.Join(io.EOF, readFailure)}, closeErr: closeFailure}
		out, err := readAssessmentResourceBody(context.Background(), a, body)
		if out != nil || !errors.Is(err, validation.ErrInvalidConfiguration) || !errors.Is(err, readFailure) || !errors.Is(err, io.EOF) || !errors.Is(err, closeFailure) || body.closed != 1 {
			t.Errorf("bad reader n=%d lost causes: out=%v err=%v closes=%d", n, out, err, body.closed)
		}
		adapterClean(t, a)
	}
}

func TestIndependentResourceIntrinsicActualDigestAndErrors(t *testing.T) {
	text := "<opus version=\"4.0\"><title>é</title></opus>\r\n"
	inputs := [][]byte{[]byte(text), append([]byte{239, 187, 191}, []byte(text)...), encodeUTF16(text, binary.LittleEndian), encodeUTF16(text, binary.BigEndian)}
	for _, data := range inputs {
		a := adapterAccount(t, validation.Budgets{})
		r := &adapterReadCloser{Reader: bytes.NewReader(data)}
		got := assessResourceSource(context.Background(), r, adapterSourceConfig(t, a, "digest-original", "src/opus.xml", ProfileSourceLinks.ID(), sourceBytesPrimary))
		if got.Err != nil || got.Snapshot == nil || !got.DigestComplete || got.Digest != sha256.Sum256(data) {
			t.Fatalf("digest got %+v", got)
		}
		if r.closes != 0 || adapterUsage(t, a, validation.BudgetInputBytes) != int64(len(data)) {
			t.Fatal("source ownership/accounting")
		}
		got.Snapshot.Close()
		adapterClean(t, a)
	}
	boom := errors.New("independent source error")
	for _, tc := range []struct {
		name string
		r    io.Reader
		want error
	}{
		{"read", &resourceReadError{[]byte(`<opus/>`), boom}, boom},
		{"joined EOF", &resourceReadError{[]byte(`<opus/>`), errors.Join(io.EOF, boom)}, boom},
		{"no progress", resourceEmptyReader{}, io.ErrNoProgress},
		{"partial malformed", strings.NewReader(`<opus>`), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := adapterAccount(t, validation.Budgets{})
			r := &adapterReadCloser{Reader: tc.r}
			got := assessResourceSource(context.Background(), r, adapterSourceConfig(t, a, "source-errors", "src/opus.xml", ProfileSourceLinks.ID(), sourceBytesPrimary))
			if !got.Report.Requested || got.Report.Profile.ID != ProfileSourceLinks.ID() || got.Report.Target != validation.TargetSource || got.Report.Profile.CatalogDigest == "" || got.Report.Satisfied() || got.DigestComplete || r.closes != 0 {
				t.Fatalf("partial report %+v closes %d", got, r.closes)
			}
			if tc.want != nil && !errors.Is(got.Err, tc.want) {
				t.Fatalf("lost cause %v want %v", got.Err, tc.want)
			}
			if got.Snapshot != nil {
				got.Snapshot.Close()
			}
			adapterClean(t, a)
		})
	}
}

func TestIndependentResourceIntrinsicSharedLimitsAndNoEncoding(t *testing.T) {
	a := adapterAccount(t, validation.Budgets{MaxNodes: 2, MaxInputBytes: 1, MaxExternalBytes: 7})
	body, err := readAssessmentResourceBody(context.Background(), a, &resourceTestBody{Reader: strings.NewReader(`<opus/>`)})
	if err != nil {
		t.Fatal(err)
	}
	source := assessResourceSource(context.Background(), bytes.NewReader(body.bytes), adapterSourceConfig(t, a, "first-source", "", ProfileSourceLinks.ID(), sourceBytesAccounted))
	if source.Err != nil || source.Snapshot == nil {
		t.Fatalf("source %+v", source)
	}
	if adapterUsage(t, a, validation.BudgetInputBytes) != 0 || adapterUsage(t, a, validation.BudgetExternalBytes) != 7 {
		t.Fatal("duplicate body charge")
	}
	source.Snapshot.Close()
	body.close()
	model := assessResourceModel(context.Background(), NewOpusDocument(), adapterModelConfig(t, a, "second-model", "", ProfileModelLinks.ID()))
	if model.Err != nil || model.Snapshot == nil || !model.Report.Satisfied() {
		t.Fatalf("model %+v", model)
	}
	model.Snapshot.Close()
	third := assessResourceSource(context.Background(), strings.NewReader(`<opus/>`), adapterSourceConfig(t, a, "third-source", "", ProfileSourceLinks.ID(), sourceBytesAccounted))
	if !errors.Is(third.Err, validation.ErrResourceLimit) || third.Report.Satisfied() {
		t.Fatalf("shared limit bypass %+v", third)
	}
	if third.Snapshot != nil {
		third.Snapshot.Close()
	}
	adapterClean(t, a)
	a = adapterAccount(t, validation.Budgets{MaxInputBytes: 1, MaxExternalBytes: 1})
	cyclic := NewOpusDocument()
	cyclic.AddOpus(cyclic)
	got := assessResourceModel(context.Background(), cyclic, adapterModelConfig(t, a, "direct-model", "model.xml", ProfileModelLinks.ID()))
	if got.Err != nil || got.Report.Satisfied() || got.Snapshot == nil || got.DigestComplete || got.Digest != ([32]byte{}) || got.Report.Syntax != validation.SyntaxNotAssessed {
		t.Fatalf("cyclic direct model %+v", got)
	}
	got.Snapshot.Close()
	if adapterUsage(t, a, validation.BudgetInputBytes) != 0 || adapterUsage(t, a, validation.BudgetExternalBytes) != 0 {
		t.Fatal("model serialization bytes")
	}
	adapterClean(t, a)
}

func TestIndependentResourceIntrinsicSnapshotAdmissionKeepsBestReport(t *testing.T) {
	for _, model := range []bool{false, true} {
		a := adapterAccount(t, validation.Budgets{})
		input := `<opus invalid="yes"/>`
		var steps int64
		if model {
			// The invalid fixed XLink type produces a known intrinsic failure without source serialization.
			doc := NewOpusDocument()
			badType := "extended"
			doc.AddScore(&OpusScore{Type: &badType})
			_, _, _ = assessModelWithConfig(context.Background(), doc, adapterModelConfig(t, a, "best-model", "", ProfileModelLinks.ID()), modelWalkOptions{})
			steps = adapterUsage(t, a, validation.BudgetRuleSteps)
			a = adapterAccount(t, validation.Budgets{MaxRuleSteps: steps})
			got := assessResourceModel(context.Background(), doc, adapterModelConfig(t, a, "best-model", "", ProfileModelLinks.ID()))
			if !errors.Is(got.Err, validation.ErrResourceLimit) || got.Snapshot != nil || !got.Report.Requested || got.Report.Conformance != validation.ConformanceViolated {
				t.Fatalf("model best %+v budget %d", got, steps)
			}
		} else {
			_, _ = adapterInspect(t, strings.NewReader(input), adapterSourceConfig(t, a, "best-source", "", ProfileSourceLinks.ID(), sourceBytesPrimary))
			steps = adapterUsage(t, a, validation.BudgetRuleSteps)
			a = adapterAccount(t, validation.Budgets{MaxRuleSteps: steps})
			got := assessResourceSource(context.Background(), strings.NewReader(input), adapterSourceConfig(t, a, "best-source", "", ProfileSourceLinks.ID(), sourceBytesPrimary))
			if !errors.Is(got.Err, validation.ErrResourceLimit) || got.Snapshot != nil || !got.Report.Requested || got.Report.Conformance != validation.ConformanceViolated || !got.DigestComplete || got.Digest != sha256.Sum256([]byte(input)) {
				t.Fatalf("source best %+v budget %d", got, steps)
			}
		}
		adapterClean(t, a)
	}
}

type independentCountingContext struct {
	context.Context
	calls, at int
	canceled  chan struct{}
}

func (c *independentCountingContext) Done() <-chan struct{} { return c.canceled }
func (c *independentCountingContext) Err() error {
	c.calls++
	if c.calls >= c.at {
		select {
		case <-c.canceled:
		default:
			close(c.canceled)
		}
		return context.Canceled
	}
	return nil
}
func independentContext(at int) *independentCountingContext {
	return &independentCountingContext{Context: context.Background(), at: at, canceled: make(chan struct{})}
}

type independentRecordingReader struct {
	io.Reader
	data  []byte
	reads int
}

func (r *independentRecordingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.reads++
	if n > 0 {
		r.data = append(r.data, p[:n]...)
	}
	return n, err
}

func TestIndependentResourceIntrinsicPrefixDigestAndStartup(t *testing.T) {
	input := `<opus><title>more data than allowed</title></opus>`
	for _, owner := range []sourceByteOwner{sourceBytesPrimary, sourceBytesExternal} {
		a := adapterAccount(t, validation.Budgets{MaxInputBytes: 5, MaxExternalBytes: 5})
		r := &independentRecordingReader{Reader: strings.NewReader(input)}
		got := assessResourceSource(context.Background(), r, adapterSourceConfig(t, a, "prefix", "", ProfileSourceLinks.ID(), owner))
		if !errors.Is(got.Err, validation.ErrResourceLimit) || !got.Report.Requested || got.DigestComplete || got.Digest != sha256.Sum256(r.data) {
			t.Fatalf("prefix digest %+v read %x", got, r.data)
		}
		if len(r.data) != 6 {
			t.Fatalf("overshoot %d", len(r.data))
		}
		if got.Snapshot != nil {
			got.Snapshot.Close()
		}
		adapterClean(t, a)
	}
	a := adapterAccount(t, validation.Budgets{})
	engine, _ := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
	session, err := engine.Begin(context.Background(), validation.RunOptions{DocumentID: "startup", Profile: ProfileSourceLinks.ID(), Account: a})
	if err != nil {
		t.Fatal(err)
	}
	setup := adapterUsage(t, a, validation.BudgetBufferedBytes)
	session.Finish(context.Background(), validation.Completion{Syntax: validation.SyntaxIncomplete})
	for _, limit := range []int64{setup + sourceXMLCursorBytes + 511, setup + sourceXMLCursorBytes + 512 + sourceXMLReaderBytes - 1, setup + sourceXMLCursorBytes + 512 + sourceXMLReaderBytes + 255} {
		a = adapterAccount(t, validation.Budgets{MaxBufferedBytes: limit})
		r := &adapterReadCloser{Reader: strings.NewReader(`<opus/>`)}
		got := assessResourceSource(context.Background(), r, adapterSourceConfig(t, a, "startup", "", ProfileSourceLinks.ID(), sourceBytesPrimary))
		if !errors.Is(got.Err, validation.ErrResourceLimit) || !got.Report.Requested || got.Report.Profile.ID != ProfileSourceLinks.ID() || got.Snapshot != nil || r.reads != 0 || r.closes != 0 {
			t.Fatalf("startup %+v reads %d closes %d", got, r.reads, r.closes)
		}
		adapterClean(t, a)
	}
}

func TestIndependentResourceCancellationBoundaries(t *testing.T) {
	for _, kind := range []string{"uri", "body", "source", "model"} {
		run := func(ctx context.Context) {
			a := adapterAccount(t, validation.Budgets{})
			var err error
			switch kind {
			case "uri":
				_, err = resolveAssessmentResourceURI(ctx, a, resourceURIBase{RetrievalURI: "https://test.invalid/one.xml"}, "two.xml#fragment")
			case "body":
				body := &resourceTestBody{Reader: strings.NewReader("data")}
				var out *assessmentResourceBody
				out, err = readAssessmentResourceBody(ctx, a, body)
				if out != nil {
					out.close()
				}
				if body.closed != 1 {
					t.Fatal("body close count")
				}
			case "source":
				got := assessResourceSource(ctx, strings.NewReader(`<opus/>`), adapterSourceConfig(t, a, "canceled-source", "", ProfileSourceLinks.ID(), sourceBytesPrimary))
				err = got.Err
				if got.Snapshot != nil {
					got.Snapshot.Close()
				}
				if err != nil && adapterUsage(t, a, validation.BudgetInputBytes) > 0 && !got.Report.Requested {
					t.Fatal("source lost best report")
				}
			case "model":
				got := assessResourceModel(ctx, NewOpusDocument(), adapterModelConfig(t, a, "canceled-model", "", ProfileModelLinks.ID()))
				err = got.Err
				if got.Snapshot != nil {
					got.Snapshot.Close()
				}
				if err != nil && adapterUsage(t, a, validation.BudgetNodes) > 0 && !got.Report.Requested {
					t.Fatal("model lost best report")
				}
			}
			if ctx.Err() != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("%s lost cancel: %v", kind, err)
			}
			adapterClean(t, a)
		}
		probe := independentContext(1 << 30)
		run(probe)
		calls := probe.calls - 1
		boundaries := map[int]bool{}
		for n := 1; n <= calls; n++ {
			if n <= 25 || n > calls-25 || n%max(1, calls/25) == 0 {
				boundaries[n] = true
			}
		}
		t.Logf("%s: %d cancellation sites, %d selected boundaries", kind, calls, len(boundaries))
		for at := range boundaries {
			t.Run(fmt.Sprintf("%s/%d", kind, at), func(t *testing.T) { run(independentContext(at)) })
		}
	}
}

func TestIndependentResourceBodyExternalBudgetBoundsActualRead(t *testing.T) {
	for _, prior := range []int64{0, 1} {
		a := adapterAccount(t, validation.Budgets{MaxExternalBytes: 1})
		if err := a.Charge(validation.BudgetExternalBytes, prior); err != nil {
			t.Fatal(err)
		}
		r := &independentRecordingReader{Reader: bytes.NewReader(bytes.Repeat([]byte{'x'}, 8192))}
		body := &resourceTestBody{Reader: r}
		out, err := readAssessmentResourceBody(context.Background(), a, body)
		if out != nil || !errors.Is(err, validation.ErrResourceLimit) || body.closed != 1 {
			t.Fatalf("out %v error %v close %d", out, err, body.closed)
		}
		if len(r.data) > int(1-prior+1) {
			t.Errorf("remaining %d: consumed %d actual bytes rather than at most remaining plus one", 1-prior, len(r.data))
		}
		if used := adapterUsage(t, a, validation.BudgetExternalBytes); used != 1 {
			t.Errorf("consumed %d actual bytes but quota usage is %d, want exhausted limit 1", len(r.data), used)
		}
		adapterClean(t, a)
	}
}

func TestIndependentResourceOptionsRejectUnusablePinBeforeIO(t *testing.T) {
	for _, identity := range []ResourceIdentity{
		{CanonicalID: "line\nbreak"}, {CanonicalID: "line\rbreak"}, {CanonicalID: "tab\tname"}, {Revision: strings.Repeat("r", 1025)},
	} {
		_, err := validateLinkedAssessmentOptions(context.Background(), LinkedAssessmentOptions{Graph: GraphOptions{Profile: "test-resource"}, Identity: identity}.internal(), "test-resource")
		if !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Errorf("accepted primary identity incompatible with core ResourcePin: canonical %q revision length %d error %v", identity.CanonicalID, len(identity.Revision), err)
		}
	}
}

func TestIndependentResourceIntrinsicBufferAdmissionKeepsKnownFailure(t *testing.T) {
	for _, model := range []bool{false, true} {
		input := `<opus invalid="yes"/>`
		doc := NewOpusDocument()
		badType := "extended"
		doc.AddScore(&OpusScore{Type: &badType})
		ordinary := func(limit int64) (validation.Report, error) {
			a := adapterAccount(t, validation.Budgets{MaxBufferedBytes: limit})
			var report validation.Report
			var err error
			if model {
				report, _, err = assessModelWithConfig(context.Background(), doc, adapterModelConfig(t, a, "buffer-admission", "", ProfileModelLinks.ID()), modelWalkOptions{})
			} else {
				report, err = adapterInspect(t, strings.NewReader(input), adapterSourceConfig(t, a, "buffer-admission", "", ProfileSourceLinks.ID(), sourceBytesPrimary))
			}
			adapterClean(t, a)
			return report, err
		}
		low, high := int64(1), int64(1<<20)
		for low < high {
			mid := (low + high) / 2
			_, err := ordinary(mid)
			if err != nil {
				low = mid + 1
			} else {
				high = mid
			}
		}
		prior, err := ordinary(high)
		if err != nil || prior.Conformance != validation.ConformanceViolated {
			t.Fatalf("ordinary model %v limit %d error %v report %v", model, high, err, prior.Conformance)
		}
		a := adapterAccount(t, validation.Budgets{MaxBufferedBytes: high})
		var got resourceIntrinsicResult
		if model {
			got = assessResourceModel(context.Background(), doc, adapterModelConfig(t, a, "buffer-admission", "", ProfileModelLinks.ID()))
		} else {
			got = assessResourceSource(context.Background(), strings.NewReader(input), adapterSourceConfig(t, a, "buffer-admission", "", ProfileSourceLinks.ID(), sourceBytesPrimary))
		}
		if got.Snapshot != nil {
			got.Snapshot.Close()
		}
		if !errors.Is(got.Err, validation.ErrResourceLimit) || got.Snapshot != nil || got.Report.Conformance != validation.ConformanceViolated || got.Report.Counts.Fail != prior.Counts.Fail {
			t.Fatalf("model %v buffer %d lost report: error %v snapshot %v conformance %v fails %d want %d", model, high, got.Err, got.Snapshot, got.Report.Conformance, got.Report.Counts.Fail, prior.Counts.Fail)
		}
		adapterClean(t, a)
	}
}
