package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func finalQAOptions(r validation.ResourceResolver) LinkedAssessmentOptions {
	return LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://final-qa.test/root.opus"}, Resolver: r, Dependencies: []ResourceDependency{ResourceDocuments}}
}
func finalQARun(t *testing.T, mode, source string, ctx context.Context, opts LinkedAssessmentOptions) (validation.AssessmentBundle, error) {
	t.Helper()
	switch mode {
	case "xml":
		return InspectXMLWithResources(ctx, strings.NewReader(source), opts)
	case "model":
		d, e := Decode(strings.NewReader(source))
		if e != nil {
			t.Fatal(e)
		}
		return AssessModelWithResources(ctx, d, opts)
	case "raw-mxl":
		o := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource, Budgets: opts.Graph.Budgets}, Identity: opts.Identity, Resolver: opts.Resolver, Dependencies: opts.Dependencies}
		return InspectMXL(ctx, bytes.NewReader(packageGraphBytes(source, "root.opus", "")), o)
	case "current-mxl":
		d, e := Decode(strings.NewReader(source))
		if e != nil {
			t.Fatal(e)
		}
		o := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel, Budgets: opts.Graph.Budgets}, Identity: opts.Identity, Resolver: opts.Resolver, Dependencies: opts.Dependencies}
		return AssessMXLPackage(ctx, &MXLPackage{Document: d, RootFiles: []MXLRootFile{{FullPath: "root.opus"}}}, o)
	default:
		t.Fatal(mode)
		return validation.AssessmentBundle{}, nil
	}
}
func finalQAWrongKind(b validation.AssessmentBundle) bool {
	for _, f := range b.PackageReport().Findings {
		if f.Reason == "wrong-linked-document-kind" && f.Outcome == validation.OutcomeFail {
			return true
		}
	}
	return false
}

// Refused assembly evidence retains the original intrinsic result through the
// bounded failure channel, without creating a canonical resource or kind proof.
func finalQAOriginalSourceFailure(t *testing.T, b validation.AssessmentBundle, source string) {
	t.Helper()
	original, err := InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceLinks})
	if err == nil || original.Terminal == nil {
		t.Fatal("fixture needs an original terminal intrinsic result", err)
	}
	f, ok := b.Failure()
	if !ok || f.Target != original.Target || f.Profile != original.Profile.ID || f.CatalogDigest != original.Profile.CatalogDigest || f.Counts != original.Counts || f.Conformance != original.Conformance || f.Terminal != *original.Terminal || !f.MetadataTruncated || !f.DiagnosticsTruncated {
		t.Fatalf("unadmitted original child result lost: got=%+v want=%+v", f, original)
	}
	if _, ok := b.FailureReport(); ok {
		t.Fatal("terminal assembly refusal admitted a new full report")
	}
}

func TestFinalQAPartialTargetAcrossEntrypoints(t *testing.T) {
	for _, mode := range []string{"xml", "model", "raw-mxl", "current-mxl"} {
		for _, tc := range []struct {
			name, body        string
			wrong, unadmitted bool
		}{
			{"unsupported-local", `<sounds/>`, false, true},
			{"unsupported-qname", `<score-partwise xmlns="urn:unsupported"/>`, false, true},
			{"partial-syntax-wrong", `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://final-qa.test/forbidden.opus"/>`, true, false},
			{"partial-syntax-score", `<score-partwise>`, false, false},
			{"empty-root", ``, false, true},
		} {
			for _, fragment := range []string{"", "#", "#missing"} {
				t.Run(mode+"/"+tc.name+"/"+fragment, func(t *testing.T) {
					calls := 0
					body := &resourceTestBody{Reader: strings.NewReader(tc.body)}
					r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
						calls++
						return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "target", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(tc.body)))}, Body: body, RetrievalURI: "https://final-qa.test/target.xml"}, nil
					})
					source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://final-qa.test/target.xml` + fragment + `"/><score xlink:href="https://final-qa.test/never.xml"/></opus>`
					b, e := finalQARun(t, mode, source, context.Background(), finalQAOptions(r))
					if e == nil || errors.Is(e, validation.ErrInvalidConfiguration) != tc.unadmitted || calls != 1 || body.closed != 1 || b.Satisfied() || b.EnumerationComplete() {
						t.Fatalf("err=%v calls=%d closes=%d satisfied=%v complete=%v", e, calls, body.closed, b.Satisfied(), b.EnumerationComplete())
					}
					if finalQAWrongKind(b) != tc.wrong {
						t.Fatalf("wrong-kind=%v expected=%v: %+v", finalQAWrongKind(b), tc.wrong, b.PackageReport().Findings)
					}
					if tc.unadmitted {
						finalQAOriginalSourceFailure(t, b, tc.body)
						if tc.name != "empty-root" && !errors.Is(e, ErrUnsupportedRoot) {
							t.Fatal("original unsupported-root error lost", e)
						}
					}
					wantResources := 2
					wantAdoptions := 2
					if tc.unadmitted {
						wantResources--
						wantAdoptions--
					}
					if mode == "raw-mxl" {
						wantResources++
						wantAdoptions++
					}
					if b.ResourceCount() != wantResources || b.AdoptionCount() != wantAdoptions {
						t.Fatalf("resources=%d/%d adoptions=%d/%d err=%v", b.ResourceCount(), wantResources, b.AdoptionCount(), wantAdoptions, e)
					}
					for i := 0; i < b.AdoptionCount(); i++ {
						a, _ := b.AdoptionContext(i)
						if a.ArchiveEntry != mxlContainerPath && a.ExpansionComplete.State == validation.EvidenceKnown && a.ExpansionComplete.Value {
							t.Fatalf("partial traversal promoted completion %+v", a)
						}
					}
				})
			}
		}
	}
}

type finalQAReadError struct {
	data []byte
	err  error
}

func (r *finalQAReadError) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}
func finalQAMeasuredXML(t *testing.T, ctx context.Context, source string, opts LinkedAssessmentOptions) (validation.AssessmentBundle, error) {
	t.Helper()
	g, e := startResourceGraph(ctx, opts.internal(), ProfileResourceGraph.ID())
	if e != nil {
		t.Fatal(e)
	}
	engine, e := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
	if e != nil {
		t.Fatal(e)
	}
	id := g.next("source")
	r := assessResourceSource(ctx, strings.NewReader(source), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
	b, e := g.finishPrimary(r, g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(r.Digest[:])), id)
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, x := g.options.account.Usage(kind)
		if x != nil || used != 0 {
			t.Errorf("gauge %v=%d err=%v", kind, used, x)
		}
	}
	return b, e
}
func TestFinalQAPartialTargetAdmissionGuardsAndCleanup(t *testing.T) {
	boom := errors.New("verified-read-error")
	for _, mode := range []string{"partial-syntax", "unsupported-qname", "empty-root", "read-error", "byte-budget", "node-budget", "cancel", "deadline", "wrong-digest", "close-error"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			target := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="never.opus"/>`
			if mode == "unsupported-qname" {
				target = `<unexpected xmlns="urn:test"/>`
			}
			if mode == "empty-root" {
				target = ""
			}
			var reader io.Reader = strings.NewReader(target)
			if mode == "read-error" {
				reader = &finalQAReadError{data: []byte(target), err: boom}
			}
			body := &resourceTestBody{Reader: reader}
			if mode == "close-error" {
				body.closeErr = boom
			}
			calls := 0
			r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				if mode == "cancel" {
					cancel()
				}
				digest := fmt.Sprintf("%x", sha256.Sum256([]byte(target)))
				if mode == "wrong-digest" {
					digest = strings.Repeat("0", 64)
				}
				out := validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "target", Representation: validation.ResourceSourceXML, Digest: digest}, Body: body, RetrievalURI: "https://final-qa.test/target.xml"}
				if mode == "deadline" {
					return out, context.DeadlineExceeded
				}
				return out, nil
			})
			opts := finalQAOptions(r)
			if mode == "byte-budget" {
				opts.Graph.Budgets.MaxExternalBytes = int64(len(target) - 1)
			}
			if mode == "node-budget" {
				opts.Graph.Budgets.MaxNodes = 4
			}
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="target.xml#"/><score xlink:href="never.xml"/></opus>`
			b, e := finalQAMeasuredXML(t, ctx, source, opts)
			if e == nil || calls != 1 || body.closed != 1 || b.Satisfied() || b.EnumerationComplete() {
				t.Fatalf("err=%v calls=%d closed=%d bundle=%+v", e, calls, body.closed, b)
			}
			allow := mode == "partial-syntax"
			want := 1
			if allow {
				want = 2
			}
			if mode == "unsupported-qname" || mode == "empty-root" {
				finalQAOriginalSourceFailure(t, b, target)
				if !errors.Is(e, validation.ErrInvalidConfiguration) {
					t.Fatal("unknown assembly was not refused", e)
				}
			}
			if b.AdoptionCount() != want {
				t.Fatalf("adopted after %s: %d want %d err=%v", mode, b.AdoptionCount(), want, e)
			}
			if (mode == "read-error" || mode == "close-error") && !errors.Is(e, boom) {
				t.Fatalf("lost I/O cause: %v", e)
			}
			if (mode == "byte-budget" || mode == "node-budget") && !errors.Is(e, validation.ErrResourceLimit) {
				t.Fatalf("lost resource cause: %v", e)
			}
			if mode == "cancel" && !errors.Is(e, context.Canceled) {
				t.Fatalf("lost canceled: %v", e)
			}
			if mode == "deadline" && !errors.Is(e, context.DeadlineExceeded) {
				t.Fatalf("lost deadline: %v", e)
			}
			if !allow && finalQAWrongKind(b) {
				t.Fatalf("unverified/interrupted target supplied root proof %+v", b.PackageReport().Findings)
			}
		})
	}
}

func TestFinalQAEmptyFragmentCannotInventRoot(t *testing.T) {
	for _, target := range []string{"", "<!-- no root -->", "<sounds/>", "<opus>", "<opus/>"} {
		t.Run(target, func(t *testing.T) {
			ctx := context.Background()
			g, e := startResourceGraph(ctx, finalQAOptions(nil).internal(), ProfileResourceGraph.ID())
			if e != nil {
				t.Fatal(e)
			}
			engine, e := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
			if e != nil {
				t.Fatal(e)
			}
			id := g.next("source")
			r := assessResourceSource(ctx, strings.NewReader(target), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
			if r.Snapshot == nil {
				t.Fatal("partial retained snapshot missing", r.Err)
			}
			pin := g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(r.Digest[:]))
			if e = validationPackageSessionOpenPackage(g.session, ctx, validation.PackageIdentity{ID: "root", Representation: pin.Representation, Pin: pin}); e != nil {
				t.Fatal(e)
			}
			header := r.Snapshot.Header()
			rid, e := validationPackageSessionAddResource(g.session, ctx, pin, r.Snapshot, true)
			if header.Assembly.State != validation.EvidenceKnown {
				if !errors.Is(e, validation.ErrInvalidConfiguration) || rid != "" {
					t.Fatalf("unknown assembly admitted: id=%q error=%v", rid, e)
				}
				if got := r.Snapshot.Header(); got.Root != header.Root || got.Assembly.State != header.Assembly.State {
					t.Fatal("rejected transfer changed original header")
				}
				e = g.retainUntransferredIntrinsic(r, pin, e)
				b, _ := g.finish(e)
				finalQAOriginalSourceFailure(t, b, target)
				if b.ResourceCount() != 0 || b.AdoptionCount() != 0 || b.Satisfied() {
					t.Fatal("unknown assembly invented an adopted fragment root")
				}
				for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
					if used, _ := g.options.account.Usage(kind); used != 0 {
						t.Errorf("refused snapshot gauge %v=%d", kind, used)
					}
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			aid, e := validationPackageSessionAdoptResource(g.session, ctx, rid, validation.ResourceAdoptionOptions{DocumentID: id, Root: true, Package: "root"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := validationPackageSessionAdoption(g.session, ctx, aid)
			if e != nil {
				t.Fatal(e)
			}
			h := a.Header()
			fragment, e := g.resolveFragment(aid, validation.Evidence[string]{State: validation.EvidenceKnown, Value: ""})
			if e != nil {
				t.Fatal(e)
			}
			if h.Root.Node == 0 {
				if fragment.State != validation.EvidenceUnknown {
					t.Fatalf("unobserved root promoted: %+v", fragment)
				}
			} else if fragment.State != validation.EvidenceKnown || fragment.Value != h.Root {
				t.Fatalf("actual root lost: header=%+v fragment=%+v", h.Root, fragment)
			}
			_, _ = g.finish(r.Err)
			for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
				used, _ := g.options.account.Usage(kind)
				if used != 0 {
					t.Errorf("gauge %v=%d", kind, used)
				}
			}
		})
	}
}

func TestFinalQAKnownWrongKindSurvivesLaterFailure(t *testing.T) {
	for _, mode := range []string{"xml", "model", "raw-mxl", "current-mxl"} {
		for _, failure := range []string{"malformed", "read-error", "cancel"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				var bodies []*resourceTestBody
				boom := errors.New("late-read")
				r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
					calls++
					text := `<opus/>`
					if calls == 2 {
						text = `<score-partwise>`
					}
					var reader io.Reader = strings.NewReader(text)
					if calls == 2 && failure == "read-error" {
						reader = &finalQAReadError{data: []byte(text), err: boom}
					}
					if calls == 2 && failure == "cancel" {
						cancel()
					}
					b := &resourceTestBody{Reader: reader}
					bodies = append(bodies, b)
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: fmt.Sprint(calls), Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(text)))}, Body: b, RetrievalURI: "https://final-qa.test/target.xml"}, nil
				})
				source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://final-qa.test/first"/><score xlink:href="https://final-qa.test/second"/><score xlink:href="https://final-qa.test/never"/></opus>`
				b, e := finalQARun(t, mode, source, ctx, finalQAOptions(r))
				if e == nil || calls != 2 || !finalQAWrongKind(b) || b.PackageReport().Conformance != validation.ConformanceViolated || b.EnumerationComplete() || b.Satisfied() {
					t.Fatalf("prior established failure lost: err=%v calls=%d report=%+v", e, calls, b.PackageReport())
				}
				for _, b := range bodies {
					if b.closed != 1 {
						t.Fatal("close", b.closed)
					}
				}
			})
		}
	}
}
